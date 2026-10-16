package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/models"
	"wedrink/internal/utils"
)

type SQLiteAuditRepository struct {
	db *sql.DB
}

func NewSQLiteAuditRepository(db *sql.DB) *SQLiteAuditRepository {
	return &SQLiteAuditRepository{db: db}
}

func (r *SQLiteAuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	if log.ID.IsZero() {
		log.ID = bson.NewObjectID()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	var oldStateJSON, newStateJSON *string
	if log.OldState != nil {
		if b, err := json.Marshal(log.OldState); err == nil {
			s := string(b)
			oldStateJSON = &s
		}
	}
	if log.NewState != nil {
		if b, err := json.Marshal(log.NewState); err == nil {
			s := string(b)
			newStateJSON = &s
		}
	}

	query := `
		INSERT INTO audit_logs (
			id, timestamp, actor_id, actor, role, action, resource_id,
			ip_address, user_agent, old_state, new_state
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := r.db.ExecContext(ctx, query,
		log.ID.Hex(),
		log.Timestamp.Format(time.RFC3339Nano),
		log.ActorID,
		log.Actor,
		log.Role,
		log.Action,
		log.ResourceID,
		log.IPAddress,
		log.UserAgent,
		oldStateJSON,
		newStateJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

func (r *SQLiteAuditRepository) Query(ctx context.Context, params AuditQueryParams) ([]models.AuditLog, error) {
	result, err := r.FindWithParams(ctx, params)
	if err != nil {
		return nil, err
	}
	return result.Logs, nil
}

func (r *SQLiteAuditRepository) FindWithParams(ctx context.Context, params AuditQueryParams) (*AuditQueryResult, error) {
	var whereClauses []string
	var args []any

	if actor := strings.TrimSpace(params.Actor); actor != "" {
		whereClauses = append(whereClauses, "actor = ?")
		args = append(args, actor)
	}

	if resID := strings.TrimSpace(params.ResourceID); resID != "" {
		whereClauses = append(whereClauses, "resource_id = ?")
		args = append(args, resID)
	}

	if action := strings.TrimSpace(params.Action); action != "" {
		switch action {
		case "reports":
			whereClauses = append(whereClauses, "action IN ('report.submit', 'report.update', 'report.delete')")
		case "auth":
			whereClauses = append(whereClauses, "action IN ('user.login', 'user.logout', 'user.login_failed')")
		case "users":
			whereClauses = append(whereClauses, "action IN ('user.create', 'user.update', 'user.delete')")
		default:
			whereClauses = append(whereClauses, "action = ?")
			args = append(args, action)
		}
	}

	// Date range filtering on timestamp
	if startDate := strings.TrimSpace(params.StartDate); startDate != "" {
		if t, err := time.ParseInLocation("2006-01-02", startDate, utils.PKLocation); err == nil {
			whereClauses = append(whereClauses, "timestamp >= ?")
			args = append(args, t.Format(time.RFC3339Nano))
		}
	}
	if endDate := strings.TrimSpace(params.EndDate); endDate != "" {
		if t, err := time.ParseInLocation("2006-01-02", endDate, utils.PKLocation); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Nanosecond)
			whereClauses = append(whereClauses, "timestamp <= ?")
			args = append(args, endOfDay.Format(time.RFC3339Nano))
		}
	}

	// Cursor pagination
	if cursor := strings.TrimSpace(params.Cursor); cursor != "" {
		whereClauses = append(whereClauses, "id < ?")
		args = append(args, cursor)
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, timestamp, actor_id, actor, role, action, resource_id,
		       ip_address, user_agent, old_state, new_state
		FROM audit_logs
		%s
		ORDER BY id DESC
		LIMIT ?;
	`, whereSQL)

	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.AuditLog, 0)
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, *l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit logs iteration error: %w", err)
	}

	result := &AuditQueryResult{
		Logs:    logs,
		HasMore: false,
	}

	if len(logs) > limit {
		result.HasMore = true
		result.NextCursor = logs[limit-1].ID.Hex()
		result.TriggerIdx = limit
		result.Logs = logs[:limit]
	}

	return result, nil
}

func (r *SQLiteAuditRepository) FindByID(ctx context.Context, idStr string) (*models.AuditLog, error) {
	cleanID := strings.TrimSpace(idStr)
	if cleanID == "" {
		return nil, nil
	}

	query := `
		SELECT id, timestamp, actor_id, actor, role, action, resource_id,
		       ip_address, user_agent, old_state, new_state
		FROM audit_logs
		WHERE id = ?
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, cleanID)

	log, err := scanAuditLog(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find audit log by id: %w", err)
	}
	return log, nil
}

func (r *SQLiteAuditRepository) GetStats(ctx context.Context) (*AuditStats, error) {
	query := `
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN action IN ('user.login', 'user.login_failed', 'user.logout', 'auth.failed') THEN 1 END),
			COUNT(CASE WHEN action NOT IN ('user.login', 'user.login_failed', 'user.logout', 'auth.failed') THEN 1 END),
			COUNT(DISTINCT actor)
		FROM audit_logs;
	`
	row := r.db.QueryRowContext(ctx, query)

	var stats AuditStats
	err := row.Scan(
		&stats.TotalEvents,
		&stats.SecurityEvents,
		&stats.MutationEvents,
		&stats.UniqueActors,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compute audit stats: %w", err)
	}
	return &stats, nil
}

func scanAuditLog(scanner rowScanner) (*models.AuditLog, error) {
	var (
		idStr        string
		timestampStr string
		actorID      sql.NullString
		actor        string
		role         string
		action       string
		resourceID   sql.NullString
		ipAddress    string
		userAgent    string
		oldStateJSON sql.NullString
		newStateJSON sql.NullString
	)

	err := scanner.Scan(
		&idStr,
		&timestampStr,
		&actorID,
		&actor,
		&role,
		&action,
		&resourceID,
		&ipAddress,
		&userAgent,
		&oldStateJSON,
		&newStateJSON,
	)
	if err != nil {
		return nil, err
	}

	objID, _ := bson.ObjectIDFromHex(idStr)
	ts, _ := time.Parse(time.RFC3339Nano, timestampStr)
	if ts.IsZero() {
		ts, _ = time.Parse(time.RFC3339, timestampStr)
	}

	var oldState, newState any
	if oldStateJSON.Valid && strings.TrimSpace(oldStateJSON.String) != "" {
		_ = json.Unmarshal([]byte(oldStateJSON.String), &oldState)
	}
	if newStateJSON.Valid && strings.TrimSpace(newStateJSON.String) != "" {
		_ = json.Unmarshal([]byte(newStateJSON.String), &newState)
	}

	return &models.AuditLog{
		ID:         objID,
		Timestamp:  ts,
		ActorID:    actorID.String,
		Actor:      actor,
		Role:       role,
		Action:     action,
		ResourceID: resourceID.String,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		OldState:   oldState,
		NewState:   newState,
	}, nil
}
