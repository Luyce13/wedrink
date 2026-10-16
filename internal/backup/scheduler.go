package backup

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

type SchedulerConfig struct {
	Enabled   bool
	Interval  time.Duration
	BackupDir string
	Retention int
}

// StartScheduler starts a background cron goroutine that periodically generates point-in-time snapshots.
func StartScheduler(ctx context.Context, db *sql.DB, cfg SchedulerConfig) {
	if !cfg.Enabled {
		slog.Info("Automated SQLite backup scheduler is disabled (BACKUP_ENABLED=false)")
		return
	}

	interval := cfg.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	backupDir := cfg.BackupDir
	if backupDir == "" {
		backupDir = "data/backups"
	}

	retention := cfg.Retention
	if retention <= 0 {
		retention = 7
	}

	slog.Info("Starting automated SQLite backup scheduler",
		"interval", interval.String(),
		"dir", backupDir,
		"retention", retention,
	)

	go func() {
		// Run initial backup 30 seconds after server startup if no backups exist yet
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
			backups, _ := ListBackups(backupDir)
			if len(backups) == 0 {
				slog.Info("No existing backups found. Running startup snapshot...")
				_, _ = CreateBackup(ctx, db, backupDir, retention)
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Automated SQLite backup scheduler stopped")
				return
			case <-ticker.C:
				slog.Info("Scheduled cron triggered: creating database snapshot...")
				_, err := CreateBackup(ctx, db, backupDir, retention)
				if err != nil {
					slog.Error("Scheduled backup failed", "error", err)
				}
			}
		}
	}()
}
