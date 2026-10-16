package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
)

func TestBackupCreationVerificationAndPruning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wedrink_backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	liveDBPath := filepath.Join(tempDir, "live.db")
	backupDir := filepath.Join(tempDir, "backups")

	// 1. Initialize live SQLite database
	sqliteDB, err := db.ConnectSQLite(liveDBPath)
	if err != nil {
		t.Fatalf("Failed to initialize live DB: %v", err)
	}
	defer sqliteDB.Close()

	ctx := context.Background()
	reportRepo := repository.NewSQLiteReportRepository(sqliteDB.DB)
	userRepo := repository.NewSQLiteUserRepository(sqliteDB.DB)

	// Seed 1 user and 3 reports
	_ = userRepo.SeedDefaultUsers(ctx)
	for i := 1; i <= 3; i++ {
		_ = reportRepo.Create(ctx, &models.EODReport{
			ReportID:      fmt.Sprintf("rep_%d", i),
			ReportDate:    fmt.Sprintf("2026-07-%02d", i),
			TotalSale:     50000,
			CreditSale:    10000,
			BankTransfer:  5000,
			OtherPayments: 2000,
			ExpectedCash:  33000,
			CounterCash:   33000,
			SubmittedBy:   "staff",
			Expenses: []models.ExpenseItem{
				{Description: "Lemons", Amount: 2000},
			},
		})
	}

	// 2. Test verified backup creation
	info, err := CreateBackup(ctx, sqliteDB.DB, backupDir, 3)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	if info.ReportCount != 3 {
		t.Errorf("Expected 3 reports in backup info, got %d", info.ReportCount)
	}
	if info.UserCount < 1 {
		t.Errorf("Expected at least 1 user in backup info, got %d", info.UserCount)
	}

	// Verify file exists on disk
	if _, err := os.Stat(info.Path); err != nil {
		t.Fatalf("Backup file does not exist on disk: %v", err)
	}

	// 3. Test retention pruning:
	// We want retention = 3.
	// Create 4 more backups (total 5 created).
	for i := 2; i <= 5; i++ {
		time.Sleep(1050 * time.Millisecond) // distinct timestamp filename
		_, err := CreateBackup(ctx, sqliteDB.DB, backupDir, 3)
		if err != nil {
			t.Fatalf("Failed to create backup %d: %v", i, err)
		}
	}

	// Verify that ONLY 3 backups exist on disk!
	remainingBackups, err := ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(remainingBackups) != 3 {
		t.Fatalf("Retention pruning failed! Expected exactly 3 backups remaining, got %d", len(remainingBackups))
	}

	// 4. Test RestoreBackup:
	// Delete all reports in live DB
	_ = reportRepo.Delete(ctx, "2026-07-01", "admin")
	_ = reportRepo.Delete(ctx, "2026-07-02", "admin")
	_ = reportRepo.Delete(ctx, "2026-07-03", "admin")

	activeAfterDel, _ := reportRepo.FindAll(ctx, 0)
	if len(activeAfterDel) != 0 {
		t.Fatalf("Expected 0 active reports after deletion, got %d", len(activeAfterDel))
	}

	// Restore from the latest remaining backup
	_ = sqliteDB.Close()
	if err := RestoreBackup(remainingBackups[0].Path, liveDBPath); err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Reopen live DB and verify data is back
	reopenedDB, err := db.ConnectSQLite(liveDBPath)
	if err != nil {
		t.Fatalf("Failed to reopen live DB: %v", err)
	}
	defer reopenedDB.Close()

	reopenedRepo := repository.NewSQLiteReportRepository(reopenedDB.DB)
	restoredReports, err := reopenedRepo.FindAll(ctx, 0)
	if err != nil {
		t.Fatalf("Failed to fetch restored reports: %v", err)
	}
	if len(restoredReports) != 3 {
		t.Fatalf("Restore check failed! Expected 3 reports restored, got %d", len(restoredReports))
	}
}
