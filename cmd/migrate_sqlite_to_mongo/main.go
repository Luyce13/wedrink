package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	utils.InitLogger(cfg.Env)

	slog.Info("Starting SQLite -> MongoDB Reverse Migration Utility...")

	// 1. Connect to SQLite
	sqliteDB, err := db.ConnectSQLite(cfg.SQLitePath)
	if err != nil {
		slog.Error("Failed to connect to SQLite", "error", err, "path", cfg.SQLitePath)
		os.Exit(1)
	}
	defer func() {
		_ = sqliteDB.Close()
	}()

	// 2. Connect to MongoDB Atlas
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

	ctx := context.Background()

	sqliteUserRepo := repository.NewSQLiteUserRepository(sqliteDB.DB)
	sqliteReportRepo := repository.NewSQLiteReportRepository(sqliteDB.DB)

	userColl := mongoDB.Database.Collection("users")
	reportColl := mongoDB.Database.Collection("eod_reports")
	notifColl := mongoDB.Database.Collection("notifications")
	auditColl := mongoDB.Database.Collection("audit_logs")

	fmt.Println("\n==================================================================")
	fmt.Println(" WEDRINK REVERSE MIGRATION: SQLITE -> MONGODB ATLAS")
	fmt.Printf(" SQLite Source File: %s\n", cfg.SQLitePath)
	fmt.Printf(" MongoDB Database:   %s\n", cfg.MongoDBName)
	fmt.Println("==================================================================")

	// 1. Migrate Users
	users, err := sqliteUserRepo.FindAll(ctx)
	if err != nil {
		slog.Error("Failed to fetch users from SQLite", "error", err)
	} else {
		userCount := 0
		for _, u := range users {
			opts := options.Replace().SetUpsert(true)
			filter := bson.M{"username": u.Username}
			if _, err := userColl.ReplaceOne(ctx, filter, u, opts); err == nil {
				userCount++
			} else {
				slog.Warn("Failed to upsert user to MongoDB", "username", u.Username, "error", err)
			}
		}
		fmt.Printf("✓ Synced %d users to MongoDB\n", userCount)
	}

	// 2. Migrate Reports & Expenses
	reports, err := sqliteReportRepo.FindAll(ctx, 0)
	if err != nil {
		slog.Error("Failed to fetch reports from SQLite", "error", err)
	} else {
		reportCount := 0
		expenseCount := 0
		for _, r := range reports {
			opts := options.Replace().SetUpsert(true)
			filter := bson.M{"report_date": r.ReportDate}
			if _, err := reportColl.ReplaceOne(ctx, filter, r, opts); err == nil {
				reportCount++
				expenseCount += len(r.Expenses)
			} else {
				slog.Warn("Failed to upsert report to MongoDB", "date", r.ReportDate, "error", err)
			}
		}
		fmt.Printf("✓ Synced %d reports (with %d itemized expenses) to MongoDB\n", reportCount, expenseCount)
	}

	// 3. Migrate Notifications
	rows, err := sqliteDB.DB.QueryContext(ctx, `
		SELECT id, report_id, report_date, submitted_by, notes, is_read, created_at
		FROM notifications
	`)
	if err != nil {
		slog.Error("Failed to query notifications from SQLite", "error", err)
	} else {
		defer rows.Close()
		notifCount := 0
		for rows.Next() {
			var (
				idStr, reportID, reportDate, submittedBy, notes, createdAtStr string
				isReadInt                                                     int
			)
			if err := rows.Scan(&idStr, &reportID, &reportDate, &submittedBy, &notes, &isReadInt, &createdAtStr); err == nil {
				objID, _ := bson.ObjectIDFromHex(idStr)
				t, _ := time.Parse(time.RFC3339, createdAtStr)
				notif := models.Notification{
					ID:          objID,
					ReportID:    reportID,
					ReportDate:  reportDate,
					SubmittedBy: submittedBy,
					Notes:       notes,
					IsRead:      isReadInt == 1,
					CreatedAt:   t,
				}
				opts := options.Replace().SetUpsert(true)
				filter := bson.M{"report_id": notif.ReportID}
				if _, err := notifColl.ReplaceOne(ctx, filter, notif, opts); err == nil {
					notifCount++
				}
			}
		}
		if err := rows.Err(); err != nil {
			slog.Warn("Notification rows scan error", "error", err)
		}
		fmt.Printf("✓ Synced %d notifications to MongoDB\n", notifCount)
	}

	// 4. Migrate Audit Logs
	auditRows, err := sqliteDB.DB.QueryContext(ctx, `
		SELECT id, timestamp, actor_id, actor, role, action, resource_id, ip_address, user_agent, old_state, new_state
		FROM audit_logs
	`)
	if err != nil {
		slog.Error("Failed to query audit logs from SQLite", "error", err)
	} else {
		defer auditRows.Close()
		auditCount := 0
		for auditRows.Next() {
			var (
				idStr, tsStr, actorID, actor, role, action, resourceID, ip, ua string
				oldJSON, newJSON                                               sql.NullString
			)
			if err := auditRows.Scan(&idStr, &tsStr, &actorID, &actor, &role, &action, &resourceID, &ip, &ua, &oldJSON, &newJSON); err == nil {
				objID, _ := bson.ObjectIDFromHex(idStr)
				ts, _ := time.Parse(time.RFC3339Nano, tsStr)
				if ts.IsZero() {
					ts, _ = time.Parse(time.RFC3339, tsStr)
				}
				var oldState, newState map[string]any
				if oldJSON.Valid && oldJSON.String != "" {
					_ = json.Unmarshal([]byte(oldJSON.String), &oldState)
				}
				if newJSON.Valid && newJSON.String != "" {
					_ = json.Unmarshal([]byte(newJSON.String), &newState)
				}
				auditLog := models.AuditLog{
					ID:         objID,
					Timestamp:  ts,
					ActorID:    actorID,
					Actor:      actor,
					Role:       role,
					Action:     action,
					ResourceID: resourceID,
					IPAddress:  ip,
					UserAgent:  ua,
					OldState:   oldState,
					NewState:   newState,
				}
				opts := options.Replace().SetUpsert(true)
				filter := bson.M{"_id": auditLog.ID}
				if _, err := auditColl.ReplaceOne(ctx, filter, auditLog, opts); err == nil {
					auditCount++
				}
			}
		}
		if err := auditRows.Err(); err != nil {
			slog.Warn("Audit log rows scan error", "error", err)
		}
		fmt.Printf("✓ Synced %d audit logs to MongoDB\n", auditCount)
	}

	fmt.Println("==================================================================")
	fmt.Println(" Reverse migration complete! MongoDB Atlas is now in sync with SQLite.")
	fmt.Println("==================================================================")
	fmt.Println()
}
