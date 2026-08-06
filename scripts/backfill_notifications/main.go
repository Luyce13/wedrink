package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/services"
)

func main() {
	slog.Info("Starting Wedrink Notifications Backfill & Deduplication Tool...")

	cfg := config.LoadConfig()

	mongoDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		_ = mongoDB.Close(context.Background())
	}()

	reportRepo := repository.NewReportRepository(mongoDB.Database)
	notifRepo := repository.NewNotificationRepository(mongoDB.Database)
	notifService := services.NewNotificationService(notifRepo)
	notifCollection := mongoDB.Database.Collection("notifications")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Deduplicate existing notifications in DB by report_date
	var allNotifs []models.Notification
	cur, err := notifCollection.Find(ctx, bson.M{})
	if err == nil {
		_ = cur.All(ctx, &allNotifs)
		_ = cur.Close(ctx)
	}

	seenDates := make(map[string]bson.ObjectID)
	removedDups := 0

	for _, n := range allNotifs {
		if firstID, seen := seenDates[n.ReportDate]; seen {
			// Delete duplicate
			_, _ = notifCollection.DeleteOne(ctx, bson.M{"_id": n.ID})
			removedDups++
			slog.Info("Removed duplicate notification", "date", n.ReportDate, "dupID", n.ID.Hex(), "keptID", firstID.Hex())
		} else {
			seenDates[n.ReportDate] = n.ID
		}
	}

	// 2. Backfill missing notifications for reports with notes
	reports, err := reportRepo.FindWithParams(ctx, repository.ReportQueryParams{
		Limit: 5000,
	})
	if err != nil {
		log.Fatalf("Failed to fetch reports for backfill: %v", err)
	}

	slog.Info("Found reports in DB", "count", len(reports))

	backfilledCount := 0
	skippedCount := 0
	emptyCount := 0

	for _, rep := range reports {
		cleanNotes := strings.TrimSpace(rep.Notes)
		if cleanNotes == "" {
			emptyCount++
			continue
		}

		existing, err := notifRepo.FindByReportID(ctx, rep.ID.Hex())
		if err != nil {
			slog.Error("Backfill error checking report notification", "reportID", rep.ID.Hex(), "error", err)
			continue
		}

		if existing != nil {
			skippedCount++
			continue
		}

		notif := &models.Notification{
			ReportID:    rep.ID.Hex(),
			ReportDate:  rep.ReportDate,
			SubmittedBy: rep.SubmittedBy,
			Notes:       cleanNotes,
			IsRead:      false,
			CreatedAt:   rep.CreatedAt,
		}

		if err := notifRepo.Create(ctx, notif); err != nil {
			slog.Error("Failed to backfill notification", "reportDate", rep.ReportDate, "error", err)
		} else {
			backfilledCount++
			slog.Info("Backfilled notification for report", "date", rep.ReportDate, "submittedBy", rep.SubmittedBy)
		}
	}

	unreadCount, _ := notifService.GetUnreadCount(ctx)

	fmt.Printf("\n=========================================\n")
	fmt.Printf("BACKFILL & DEDUPLICATION COMPLETED\n")
	fmt.Printf("Total Reports Scanned : %d\n", len(reports))
	fmt.Printf("Reports without Notes : %d\n", emptyCount)
	fmt.Printf("Duplicates Removed    : %d\n", removedDups)
	fmt.Printf("Already Notified      : %d\n", skippedCount)
	fmt.Printf("Newly Backfilled      : %d\n", backfilledCount)
	fmt.Printf("Total Unread Notes    : %d\n", unreadCount)
	fmt.Printf("=========================================\n\n")
}
