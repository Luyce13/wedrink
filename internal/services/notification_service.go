package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// CreateNotificationAsync asynchronously creates an unread notification when a report has remarks/notes.
// Runs in a detached background goroutine to guarantee zero impact on submission latency or HTTP response.
func (s *NotificationService) CreateNotificationAsync(reportID, reportDate, submittedBy, notes string) {
	cleanNotes := strings.TrimSpace(notes)
	if cleanNotes == "" {
		return
	}

	// Launch async detached goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Check if notification already exists for this report date to update or create
		existing, err := s.repo.FindByReportDate(ctx, reportDate)
		if err != nil {
			slog.Error("CreateNotificationAsync: failed to check existing notification", "reportDate", reportDate, "error", err)
			return
		}
		if existing != nil {
			existing.Notes = cleanNotes
			existing.SubmittedBy = submittedBy
			existing.IsRead = false
			existing.CreatedAt = time.Now()
			if err := s.repo.Update(ctx, existing); err != nil {
				slog.Error("CreateNotificationAsync: failed to update notification", "reportDate", reportDate, "error", err)
			} else {
				slog.Info("CreateNotificationAsync: updated existing notification for date", "reportDate", reportDate)
			}
			return
		}

		notif := &models.Notification{
			ReportID:    reportID,
			ReportDate:  reportDate,
			SubmittedBy: submittedBy,
			Notes:       cleanNotes,
			IsRead:      false,
			CreatedAt:   time.Now(),
		}

		if err := s.repo.Create(ctx, notif); err != nil {
			slog.Error("CreateNotificationAsync: failed to create notification", "reportDate", reportDate, "error", err)
		} else {
			slog.Info("CreateNotificationAsync: unread notification created successfully", "reportID", reportID, "date", reportDate)
		}
	}()
}

func (s *NotificationService) GetUnreadNotifications(ctx context.Context) ([]models.Notification, error) {
	return s.repo.FindUnread(ctx)
}

func (s *NotificationService) GetUnreadCount(ctx context.Context) (int64, error) {
	return s.repo.GetUnreadCount(ctx)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, idStr string) error {
	if strings.TrimSpace(idStr) == "" {
		return fmt.Errorf("notification ID is required")
	}
	return s.repo.MarkAsRead(ctx, idStr)
}
