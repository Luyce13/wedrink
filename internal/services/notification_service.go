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

// SaveNotification creates, updates, or cleans up a notification for a report.
// Guarantees zero failure impact on report submission by catching and logging all DB errors internally.
func (s *NotificationService) SaveNotification(ctx context.Context, reportID bson.ObjectID, reportDate, submittedBy, notes string) {
	cleanNotes := strings.TrimSpace(notes)

	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	existing, err := s.repo.FindByReportID(bgCtx, reportID)
	if err != nil {
		slog.Error("SaveNotification: failed to check existing notification", "reportID", reportID.Hex(), "error", err)
		return
	}

	if cleanNotes == "" {
		if existing != nil {
			_ = s.repo.DeleteByReportID(bgCtx, reportID)
			slog.Info("SaveNotification: removed empty notification for report", "reportID", reportID.Hex())
		}
		return
	}

	if existing != nil {
		existing.Notes = cleanNotes
		existing.SubmittedBy = submittedBy
		existing.IsRead = false
		existing.CreatedAt = time.Now()
		if err := s.repo.Update(bgCtx, existing); err != nil {
			slog.Error("SaveNotification: failed to update notification", "reportID", reportID.Hex(), "error", err)
		} else {
			slog.Info("SaveNotification: updated notification", "reportID", reportID.Hex(), "date", reportDate)
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

	if err := s.repo.Create(bgCtx, notif); err != nil {
		slog.Error("SaveNotification: failed to create notification", "reportID", reportID.Hex(), "error", err)
	} else {
		slog.Info("SaveNotification: created notification", "reportID", reportID.Hex(), "date", reportDate)
	}
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
