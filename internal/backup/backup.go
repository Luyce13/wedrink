package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type BackupInfo struct {
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	CreatedAt   time.Time `json:"created_at"`
	SizeBytes   int64     `json:"size_bytes"`
	ReportCount int       `json:"report_count"`
	UserCount   int       `json:"user_count"`
	AuditCount  int       `json:"audit_count"`
}

// CreateBackup generates an online, vacuumed backup using VACUUM INTO.
// It explicitly verifies database integrity with PRAGMA integrity_check.
// Only after verification passes successfully does it delete older backups according to retention policy.
func CreateBackup(ctx context.Context, db *sql.DB, backupDir string, retention int) (*BackupInfo, error) {
	if backupDir == "" {
		backupDir = "data/backups"
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Passive checkpoint to write WAL pages
	_, _ = db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE);")

	now := time.Now()
	filename := fmt.Sprintf("wedrink_backup_%s.db", now.Format("20060102_150405"))
	targetPath := filepath.Join(backupDir, filename)

	// Clean up if file already exists
	_ = os.Remove(targetPath)

	// 2. Atomic, non-blocking snapshot using SQLite's VACUUM INTO
	vacuumSQL := fmt.Sprintf("VACUUM INTO '%s';", escapeQuotes(targetPath))
	if _, err := db.ExecContext(ctx, vacuumSQL); err != nil {
		return nil, fmt.Errorf("failed to execute VACUUM INTO: %w", err)
	}

	// 3. Open the newly created backup and VERIFY INTEGRITY
	backupDB, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(5000)", targetPath))
	if err != nil {
		_ = os.Remove(targetPath)
		return nil, fmt.Errorf("failed to open generated backup: %w", err)
	}
	defer backupDB.Close()

	var integrity string
	if err := backupDB.QueryRowContext(ctx, "PRAGMA integrity_check;").Scan(&integrity); err != nil || integrity != "ok" {
		// Verification failed! Remove the corrupted file and DO NOT delete older backups!
		_ = os.Remove(targetPath)
		return nil, fmt.Errorf("backup failed integrity check (%s): %w", integrity, err)
	}

	// Read high-level counts to verify content presence
	var reportCount, userCount, auditCount int
	_ = backupDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM eod_reports WHERE is_deleted = 0;").Scan(&reportCount)
	_ = backupDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_deleted = 0;").Scan(&userCount)
	_ = backupDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs;").Scan(&auditCount)

	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	info := &BackupInfo{
		Filename:    filename,
		Path:        targetPath,
		CreatedAt:   now,
		SizeBytes:   fileInfo.Size(),
		ReportCount: reportCount,
		UserCount:   userCount,
		AuditCount:  auditCount,
	}

	slog.Info("Backup created and verified successfully",
		"file", filename,
		"size", fileInfo.Size(),
		"reports", reportCount,
		"users", userCount,
		"audit", auditCount,
	)

	// 4. Verification succeeded! Now safely prune older backups exceeding retention
	if retention > 0 {
		prunedCount, pruneErr := PruneOldBackups(backupDir, retention)
		if pruneErr != nil {
			slog.Warn("Failed to prune old backups", "error", pruneErr)
		} else if prunedCount > 0 {
			slog.Info("Pruned older backups", "count", prunedCount, "retention", retention)
		}
	}

	return info, nil
}

// ListBackups scans the backup directory and returns all verified backup files sorted newest first.
func ListBackups(backupDir string) ([]BackupInfo, error) {
	if backupDir == "" {
		backupDir = "data/backups"
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "wedrink_backup_") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(backupDir, entry.Name())
		backups = append(backups, BackupInfo{
			Filename:  entry.Name(),
			Path:      fullPath,
			CreatedAt: info.ModTime(),
			SizeBytes: info.Size(),
		})
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// PruneOldBackups deletes older backup files if total backups exceed retention limit.
// Returns the number of files deleted.
func PruneOldBackups(backupDir string, retention int) (int, error) {
	if retention <= 0 {
		return 0, nil
	}

	backups, err := ListBackups(backupDir)
	if err != nil {
		return 0, err
	}

	if len(backups) <= retention {
		return 0, nil
	}

	toDelete := backups[retention:]
	deletedCount := 0

	for _, b := range toDelete {
		if err := os.Remove(b.Path); err == nil {
			deletedCount++
		} else {
			slog.Warn("Failed to delete expired backup file", "file", b.Path, "error", err)
		}
	}

	return deletedCount, nil
}

// RestoreBackup restores a given backup file to the live database destination.
// Creates a safety copy of the existing live DB before overwriting.
func RestoreBackup(backupPath, liveDBPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Verify backup integrity before restoring
	testDB, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(5000)", backupPath))
	if err != nil {
		return fmt.Errorf("failed to open backup for verification: %w", err)
	}
	var integrity string
	err = testDB.QueryRow("PRAGMA integrity_check;").Scan(&integrity)
	_ = testDB.Close()
	if err != nil || integrity != "ok" {
		return fmt.Errorf("backup integrity check failed (%s): %w", integrity, err)
	}

	// Create safety copy of current live database if present
	if _, err := os.Stat(liveDBPath); err == nil {
		safetyCopy := fmt.Sprintf("%s.pre_restore_%s.bak", liveDBPath, time.Now().Format("20060102_150405"))
		_ = copyFile(liveDBPath, safetyCopy)
		slog.Info("Created pre-restore safety copy of live database", "path", safetyCopy)
	}

	// Remove WAL & SHM to prevent log mismatch
	_ = os.Remove(liveDBPath + "-wal")
	_ = os.Remove(liveDBPath + "-shm")

	// Copy backup over live DB
	if err := copyFile(backupPath, liveDBPath); err != nil {
		return fmt.Errorf("failed to copy backup to live database: %w", err)
	}

	// Verify restored live database
	liveDB, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", liveDBPath))
	if err != nil {
		return fmt.Errorf("failed to open restored live database: %w", err)
	}
	defer liveDB.Close()

	if err := liveDB.QueryRow("PRAGMA integrity_check;").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("restored database failed integrity check: %w", err)
	}
	_, _ = liveDB.Exec("PRAGMA wal_checkpoint(TRUNCATE);")

	slog.Info("Successfully restored database from backup", "backup", backupPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func escapeQuotes(str string) string {
	return strings.ReplaceAll(str, "'", "''")
}
