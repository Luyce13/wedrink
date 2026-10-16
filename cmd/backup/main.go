package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"wedrink/internal/backup"
	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/utils"
)

func printHelp() {
	fmt.Print(`
Wedrink SQLite Backup Utility

Usage:
  go run ./cmd/backup <command> [options]

Commands:
  create              Create a verified online backup and automatically delete older backups
  list                List all existing backups with timestamps and sizes
  restore --file=...  Restore live database from a chosen backup file
  verify              Run integrity checks across all existing backup files

Examples:
  go run ./cmd/backup create
  go run ./cmd/backup list
  go run ./cmd/backup verify
  go run ./cmd/backup restore --file="wedrink_backup_20261016_234500.db"
`)
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]
	cfg := config.LoadConfig()
	utils.InitLogger("production")

	backupDir := cfg.BackupDir
	if backupDir == "" {
		backupDir = "data/backups"
	}
	retention := cfg.BackupRetention
	if retention <= 0 {
		retention = 7
	}

	switch command {
	case "create":
		sqliteDB, err := db.ConnectSQLite(cfg.SQLitePath)
		if err != nil {
			slog.Error("Failed to connect to SQLite database", "error", err)
			os.Exit(1)
		}
		defer sqliteDB.Close()

		ctx := context.Background()
		info, err := backup.CreateBackup(ctx, sqliteDB.DB, backupDir, retention)
		if err != nil {
			fmt.Printf("✗ Backup failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n==================================================================")
		fmt.Println(" BACKUP CREATED & VERIFIED SUCCESSFULLY")
		fmt.Println("==================================================================")
		fmt.Printf(" File:            %s\n", info.Filename)
		fmt.Printf(" Created At:      %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf(" Size:            %.2f KB\n", float64(info.SizeBytes)/1024.0)
		fmt.Printf(" Reports:         %d\n", info.ReportCount)
		fmt.Printf(" Users:           %d\n", info.UserCount)
		fmt.Printf(" Audit Records:   %d\n", info.AuditCount)
		fmt.Printf(" Retention Limit: Last %d backups kept\n", retention)
		fmt.Println("==================================================================")
		fmt.Println()

	case "list":
		backups, err := backup.ListBackups(backupDir)
		if err != nil {
			fmt.Printf("✗ Failed to list backups: %v\n", err)
			os.Exit(1)
		}

		if len(backups) == 0 {
			fmt.Printf("No backups found in '%s'. Run 'go run ./cmd/backup create' to generate one.\n", backupDir)
			return
		}

		fmt.Println("\n================================================================================")
		fmt.Printf(" WEDRINK DATABASE BACKUPS (%d available, keeping latest %d)\n", len(backups), retention)
		fmt.Printf(" Directory: %s\n", backupDir)
		fmt.Println("================================================================================")
		fmt.Printf("%-36s | %-20s | %s\n", "Backup Filename", "Created At", "Size")
		fmt.Println(strings.Repeat("-", 75))

		for _, b := range backups {
			fmt.Printf("%-36s | %-20s | %.1f KB\n",
				b.Filename,
				b.CreatedAt.Format("2006-01-02 15:04:05"),
				float64(b.SizeBytes)/1024.0,
			)
		}
		fmt.Println(strings.Repeat("-", 75))
		fmt.Println()

	case "restore":
		restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
		fileStr := restoreCmd.String("file", "", "Backup filename or path to restore")
		_ = restoreCmd.Parse(os.Args[2:])

		if *fileStr == "" {
			fmt.Println("Error: must specify --file='wedrink_backup_...db'")
			os.Exit(1)
		}

		filePath := *fileStr
		if !strings.Contains(filePath, "/") {
			filePath = backupDir + "/" + filePath
		}

		if err := backup.RestoreBackup(filePath, cfg.SQLitePath); err != nil {
			fmt.Printf("✗ Restore failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n==================================================================")
		fmt.Println(" DATABASE RESTORED SUCCESSFULLY")
		fmt.Println("==================================================================")
		fmt.Printf(" Restored From: %s\n", filePath)
		fmt.Printf(" Target Live DB: %s\n", cfg.SQLitePath)
		fmt.Println(" Integrity verification passed: OK")
		fmt.Println("==================================================================")
		fmt.Println()

	case "verify":
		backups, err := backup.ListBackups(backupDir)
		if err != nil {
			fmt.Printf("✗ Failed to list backups: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nVerifying integrity of all backup files...")
		for _, b := range backups {
			testDB, err := db.ConnectSQLite(b.Path)
			if err != nil {
				fmt.Printf("✗ %s: Failed to open (%v)\n", b.Filename, err)
				continue
			}
			var integrity string
			err = testDB.DB.QueryRow("PRAGMA integrity_check;").Scan(&integrity)
			testDB.Close()

			if err != nil || integrity != "ok" {
				fmt.Printf("✗ %s: Corrupt (%s)\n", b.Filename, integrity)
			} else {
				fmt.Printf("✓ %s: Integrity Verified [OK]\n", b.Filename)
			}
		}
		fmt.Println("Verification complete.")
		fmt.Println()

	default:
		printHelp()
		os.Exit(1)
	}
}
