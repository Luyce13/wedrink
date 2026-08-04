package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"wedrink/internal/models"
	"wedrink/internal/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// CreateNotificationAsync asynchronously creates or updates an unread notification when a report has remarks/notes.
// Bound directly to the EOD report's MongoDB ObjectID (_id).
func (s *NotificationService) CreateNotificationAsync(reportID bson.ObjectID, reportDate, submittedBy, notes string) {
	cleanNotes := strings.TrimSpace(notes)
	if cleanNotes == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		existing, err := s.repo.FindByReportID(ctx, reportID)
		if err != nil {
			slog.Error("CreateNotificationAsync: failed to check existing notification", "reportID", reportID.Hex(), "error", err)
			return
		}
		if existing != nil {
			existing.Notes = cleanNotes
			existing.SubmittedBy = submittedBy
			existing.IsRead = false
			existing.CreatedAt = time.Now()
			if err := s.repo.Update(ctx, existing); err != nil {
				slog.Error("CreateNotificationAsync: failed to update notification", "reportID", reportID.Hex(), "error", err)
			} else {
				slog.Info("CreateNotificationAsync: updated existing notification for report _id", "reportID", reportID.Hex())
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
			slog.Error("CreateNotificationAsync: failed to create notification", "reportID", reportID.Hex(), "error", err)
		} else {
			slog.Info("CreateNotificationAsync: unread notification created successfully", "reportID", reportID.Hex(), "date", reportDate)
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
