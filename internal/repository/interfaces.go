package repository

import (
	"context"

	"wedrink/internal/models"
)

// ReportRepo defines the contract for EOD report persistence.
// Implemented by ReportRepository (MongoDB).
type ReportRepo interface {
	Create(ctx context.Context, report *models.EODReport) error
	Update(ctx context.Context, report *models.EODReport) error
	UpsertByDate(ctx context.Context, report *models.EODReport) error
	FindByID(ctx context.Context, idStr string) (*models.EODReport, error)
	FindByDate(ctx context.Context, dateStr string) (*models.EODReport, error)
	FindByMonth(ctx context.Context, yearMonth string) ([]models.EODReport, error)
	GetSubmittedDatesByMonth(ctx context.Context, yearMonth string) ([]string, error)
	FindByDateRange(ctx context.Context, startDate, endDate string) ([]models.EODReport, error)
	FindWithParams(ctx context.Context, params ReportQueryParams) ([]models.EODReport, error)
	FindAll(ctx context.Context, limit int) ([]models.EODReport, error)
	Delete(ctx context.Context, idStr string) error
	CalculateMonthlySummary(ctx context.Context, yearMonth string) (*models.MonthlySummary, error)
}

// UserRepo defines the contract for user persistence.
// Implemented by UserRepository (MongoDB).
type UserRepo interface {
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	FindByID(ctx context.Context, idStr string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	FindAll(ctx context.Context) ([]models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, username string) error
	SeedDefaultUsers(ctx context.Context) error
}

// NotificationRepo defines the contract for notification persistence.
// Implemented by NotificationRepository (MongoDB).
type NotificationRepo interface {
	Create(ctx context.Context, notif *models.Notification) error
	FindByReportID(ctx context.Context, reportID string) (*models.Notification, error)
	FindUnread(ctx context.Context) ([]models.Notification, error)
	FindWithParams(ctx context.Context, params NotificationQueryParams) (*NotificationQueryResult, error)
	GetUnreadCount(ctx context.Context) (int64, error)
	MarkAsRead(ctx context.Context, idStr string) error
	Update(ctx context.Context, notif *models.Notification) error
	DeleteByReportID(ctx context.Context, reportID string) error
}
