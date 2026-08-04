package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/services"
)

func main() {
	slog.Info("Starting Wedrink Notifications Backfill Tool...")

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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Fetch all reports
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

		existing, err := notifRepo.FindByReportID(ctx, rep.ReportID)
		if err != nil {
			slog.Error("Backfill error checking report notification", "reportID", rep.ReportID, "error", err)
			continue
		}

		if existing != nil {
			skippedCount++
			continue
		}

		notif := &models.Notification{
			ReportID:    rep.ReportID,
			ReportDate:  rep.ReportDate,
			SubmittedBy: rep.SubmittedBy,
			Notes:       cleanNotes,
			IsRead:      false,
			CreatedAt:   rep.CreatedAt,
		}

		if err := notifRepo.Create(ctx, notif); err != nil {
			slog.Error("Failed to backfill notification", "reportID", rep.ReportID, "error", err)
		} else {
			backfilledCount++
			slog.Info("Backfilled notification for report", "date", rep.ReportDate, "submittedBy", rep.SubmittedBy)
		}
	}

	unreadCount, _ := notifService.GetUnreadCount(ctx)

	fmt.Printf("\n=========================================\n")
	fmt.Printf("BACKFILL COMPLETED SUCCESSFULLY\n")
	fmt.Printf("Total Reports Scanned : %d\n", len(reports))
	fmt.Printf("Reports without Notes : %d\n", emptyCount)
	fmt.Printf("Already Notified      : %d\n", skippedCount)
	fmt.Printf("Newly Backfilled      : %d\n", backfilledCount)
	fmt.Printf("Total Unread Notes    : %d\n", unreadCount)
	fmt.Printf("=========================================\n\n")
}
