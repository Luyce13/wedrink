package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/models"
)

type SQLiteNotificationRepository struct {
	db *sql.DB
}

func NewSQLiteNotificationRepository(db *sql.DB) *SQLiteNotificationRepository {
	return &SQLiteNotificationRepository{db: db}
}

func (r *SQLiteNotificationRepository) Create(ctx context.Context, notif *models.Notification) error {
	if notif.ID.IsZero() {
		notif.ID = bson.NewObjectID()
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}

	isReadInt := 0
	if notif.IsRead {
		isReadInt = 1
	}

	query := `
		INSERT INTO notifications (id, report_id, report_date, submitted_by, notes, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := r.db.ExecContext(ctx, query,
		notif.ID.Hex(),
		notif.ReportID,
		notif.ReportDate,
		notif.SubmittedBy,
		notif.Notes,
		isReadInt,
		notif.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

func (r *SQLiteNotificationRepository) FindByReportID(ctx context.Context, reportID string) (*models.Notification, error) {
	cleanID := strings.TrimSpace(reportID)
	if cleanID == "" {
		return nil, nil
	}

	query := `
		SELECT id, report_id, report_date, submitted_by, notes, is_read, created_at
		FROM notifications
		WHERE report_id = ?
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, cleanID)

	notif, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find notification by report id: %w", err)
	}
	return notif, nil
}

func (r *SQLiteNotificationRepository) FindUnread(ctx context.Context) ([]models.Notification, error) {
	query := `
		SELECT id, report_id, report_date, submitted_by, notes, is_read, created_at
		FROM notifications
		WHERE is_read = 0
		ORDER BY created_at DESC;
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find unread notifications: %w", err)
	}
	defer rows.Close()

	notifs := make([]models.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifs = append(notifs, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unread notifications iteration error: %w", err)
	}
	return notifs, nil
}

func (r *SQLiteNotificationRepository) FindWithParams(ctx context.Context, params NotificationQueryParams) (*NotificationQueryResult, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}

	var whereClauses []string
	var args []any

	if params.Filter == "unread" {
		whereClauses = append(whereClauses, "is_read = 0")
	}

	if params.Cursor != "" {
		whereClauses = append(whereClauses, "id < ?")
		args = append(args, params.Cursor)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, report_id, report_date, submitted_by, notes, is_read, created_at
		FROM notifications
		%s
		ORDER BY id DESC
		LIMIT ?;
	`, whereSQL)

	args = append(args, params.Limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications with params: %w", err)
	}
	defer rows.Close()

	notifs := make([]models.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifs = append(notifs, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notifications iteration error: %w", err)
	}

	result := &NotificationQueryResult{
		Notifications: notifs,
		HasMore:       false,
	}

	if len(notifs) > params.Limit {
		result.HasMore = true
		result.NextCursor = notifs[params.Limit-1].ID.Hex()
		result.TriggerIdx = params.Limit
		result.Notifications = notifs[:params.Limit]
	}

	return result, nil
}

func (r *SQLiteNotificationRepository) GetUnreadCount(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE is_read = 0;`
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread notification count: %w", err)
	}
	return count, nil
}

func (r *SQLiteNotificationRepository) MarkAsRead(ctx context.Context, idStr string) error {
	cleanID := strings.TrimSpace(idStr)
	query := `UPDATE notifications SET is_read = 1 WHERE id = ?;`
	_, err := r.db.ExecContext(ctx, query, cleanID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	return nil
}

func (r *SQLiteNotificationRepository) Update(ctx context.Context, notif *models.Notification) error {
	isReadInt := 0
	if notif.IsRead {
		isReadInt = 1
	}

	query := `
		UPDATE notifications
		SET report_date = ?, submitted_by = ?, notes = ?, is_read = ?
		WHERE id = ?;
	`
	res, err := r.db.ExecContext(ctx, query,
		notif.ReportDate,
		notif.SubmittedBy,
		notif.Notes,
		isReadInt,
		notif.ID.Hex(),
	)
	if err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *SQLiteNotificationRepository) DeleteByReportID(ctx context.Context, reportID string) error {
	cleanID := strings.TrimSpace(reportID)
	query := `DELETE FROM notifications WHERE report_id = ?;`
	_, err := r.db.ExecContext(ctx, query, cleanID)
	if err != nil {
		return fmt.Errorf("failed to delete notification by report id: %w", err)
	}
	return nil
}

func scanNotification(scanner rowScanner) (*models.Notification, error) {
	var (
		idStr        string
		reportID     string
		reportDate   string
		submittedBy  string
		notes        string
		isReadInt    int
		createdAtStr string
	)

	err := scanner.Scan(
		&idStr,
		&reportID,
		&reportDate,
		&submittedBy,
		&notes,
		&isReadInt,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	objID, _ := bson.ObjectIDFromHex(idStr)
	createdAt, _ := time.Parse(time.RFC3339, createdAtStr)

	return &models.Notification{
		ID:          objID,
		ReportID:    reportID,
		ReportDate:  reportDate,
		SubmittedBy: submittedBy,
		Notes:       notes,
		IsRead:      isReadInt == 1,
		CreatedAt:   createdAt,
	}, nil
}
