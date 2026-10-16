package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	utils.InitLogger(cfg.Env)

	slog.Info("Starting MongoDB -> SQLite Migration Utility...")

	// 1. Connect to MongoDB Atlas
	mongoDB, err := db.Connect(cfg)
	if err != nil {
		slog.Error("Failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoDB.Close(ctx)
	}()

	// 2. Connect to SQLite
	sqliteDB, err := db.ConnectSQLite(cfg.SQLitePath)
	if err != nil {
		slog.Error("Failed to connect to SQLite", "error", err, "path", cfg.SQLitePath)
		os.Exit(1)
	}
	defer func() {
		_ = sqliteDB.Close()
	}()

	ctx := context.Background()

	// Instantiate repositories
	mongoUserRepo := repository.NewUserRepository(mongoDB.Database)
	mongoReportRepo := repository.NewReportRepository(mongoDB.Database)

	sqliteUserRepo := repository.NewSQLiteUserRepository(sqliteDB.DB)
	sqliteReportRepo := repository.NewSQLiteReportRepository(sqliteDB.DB)
	sqliteNotifRepo := repository.NewSQLiteNotificationRepository(sqliteDB.DB)
	sqliteAuditRepo := repository.NewSQLiteAuditRepository(sqliteDB.DB)

	fmt.Println("\n==================================================================")
	fmt.Println(" WEDRINK MIGRATION: MONGODB ATLAS -> SQLITE")
	fmt.Printf(" SQLite Target File: %s\n", cfg.SQLitePath)
	fmt.Println("==================================================================")

	// 1. Migrate Users
	users, err := mongoUserRepo.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to fetch users from MongoDB", "error", err)
	} else {
		userCount := 0
		for _, u := range users {
			existing, _ := sqliteUserRepo.FindByUsername(ctx, u.Username)
			if existing == nil {
				if err := sqliteUserRepo.Create(ctx, &u); err == nil {
					userCount++
				}
			}
		}
		fmt.Printf("✓ Migrated %d users\n", userCount)
	}

	// 2. Migrate Reports & Expenses
	reports, err := mongoReportRepo.FindAll(ctx, 0)
	if err != nil {
		slog.Error("Failed to fetch reports from MongoDB", "error", err)
	} else {
		reportCount := 0
		expenseCount := 0
		for _, r := range reports {
			existing, _ := sqliteReportRepo.FindByDate(ctx, r.ReportDate)
			if existing == nil {
				if err := sqliteReportRepo.Create(ctx, &r); err == nil {
					reportCount++
					expenseCount += len(r.Expenses)
				}
			}
		}
		fmt.Printf("✓ Migrated %d reports (with %d itemized expenses)\n", reportCount, expenseCount)
	}

	// 3. Migrate Notifications
	notifColl := mongoDB.Database.Collection("notifications")
	curNotifs, err := notifColl.Find(ctx, bson.M{})
	if err != nil {
		slog.Error("Failed to fetch notifications from MongoDB", "error", err)
	} else {
		var notifs []models.Notification
		_ = curNotifs.All(ctx, &notifs)
		notifCount := 0
		for _, n := range notifs {
			existing, _ := sqliteNotifRepo.FindByReportID(ctx, n.ReportID)
			if existing == nil {
				if err := sqliteNotifRepo.Create(ctx, &n); err == nil {
					notifCount++
				}
			}
		}
		fmt.Printf("✓ Migrated %d notifications\n", notifCount)
	}

	// 4. Migrate Audit Logs
	auditColl := mongoDB.Database.Collection("audit_logs")
	curAudit, err := auditColl.Find(ctx, bson.M{})
	if err != nil {
		slog.Error("Failed to fetch audit logs from MongoDB", "error", err)
	} else {
		var auditLogs []models.AuditLog
		_ = curAudit.All(ctx, &auditLogs)
		auditCount := 0
		for _, a := range auditLogs {
			existing, _ := sqliteAuditRepo.FindByID(ctx, a.ID.Hex())
			if existing == nil {
				if err := sqliteAuditRepo.Create(ctx, &a); err == nil {
					auditCount++
				}
			}
		}
		fmt.Printf("✓ Migrated %d audit logs\n", auditCount)
	}

	fmt.Println("==================================================================")
	fmt.Println(" Migration complete! You can now run with DB_DRIVER=sqlite.")
	fmt.Println("==================================================================")
	fmt.Println()
}
