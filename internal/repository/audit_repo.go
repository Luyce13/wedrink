package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/models"
	"wedrink/internal/utils"
)

type AuditRepository struct {
	collection *mongo.Collection
}

func NewAuditRepository(db *mongo.Database) *AuditRepository {
	return &AuditRepository{
		collection: db.Collection("audit_logs"),
	}
}

func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	if log == nil {
		return fmt.Errorf("audit log cannot be nil")
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.ID.IsZero() {
		log.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to save audit log: %w", err)
	}
	return nil
}

func (r *AuditRepository) Query(ctx context.Context, params AuditQueryParams) ([]models.AuditLog, error) {
	res, err := r.FindWithParams(ctx, params)
	if err != nil {
		return nil, err
	}
	return res.Logs, nil
}

func (r *AuditRepository) FindWithParams(ctx context.Context, params AuditQueryParams) (*AuditQueryResult, error) {
	filter := bson.M{}

	if strings.TrimSpace(params.Actor) != "" {
		filter["actor"] = bson.M{"$regex": fmt.Sprintf("(?i)%s", strings.TrimSpace(params.Actor))}
	}
	if strings.TrimSpace(params.ResourceID) != "" {
		filter["resource_id"] = bson.M{"$regex": fmt.Sprintf("(?i)%s", strings.TrimSpace(params.ResourceID))}
	}

	action := strings.TrimSpace(params.Action)
	if action != "" && action != "all" {
		switch action {
		case "reports":
			filter["action"] = bson.M{"$in": []string{"report.submit", "report.overwrite", "report.delete"}}
		case "auth":
			filter["action"] = bson.M{"$in": []string{"auth.login_success", "auth.login_failed"}}
		case "users":
			filter["action"] = bson.M{"$in": []string{"user.create", "user.update", "user.delete"}}
		default:
			filter["action"] = action
		}
	}

	// Date range filtering on timestamp
	tsFilter := bson.M{}
	if strings.TrimSpace(params.StartDate) != "" {
		if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(params.StartDate), utils.PKLocation); err == nil {
			tsFilter["$gte"] = t
		}
	}
	if strings.TrimSpace(params.EndDate) != "" {
		if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(params.EndDate), utils.PKLocation); err == nil {
			tsFilter["$lte"] = t.Add(24*time.Hour - time.Nanosecond)
		}
	}
	if len(tsFilter) > 0 {
		filter["timestamp"] = tsFilter
	}

	// Cursor pagination
	if strings.TrimSpace(params.Cursor) != "" {
		if oid, err := bson.ObjectIDFromHex(strings.TrimSpace(params.Cursor)); err == nil {
			filter["_id"] = bson.M{"$lt": oid}
		}
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(limit + 1))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer cursor.Close(ctx)

	logs := make([]models.AuditLog, 0)
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode audit logs: %w", err)
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

func (r *AuditRepository) FindByID(ctx context.Context, idStr string) (*models.AuditLog, error) {
	oid, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid audit log ID: %w", err)
	}

	var log models.AuditLog
	err = r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&log)
	if err != nil {
		return nil, fmt.Errorf("audit log not found: %w", err)
	}
	return &log, nil
}

func (r *AuditRepository) GetStats(ctx context.Context) (*AuditStats, error) {
	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to count audit logs: %w", err)
	}

	secFilter := bson.M{"action": bson.M{"$in": []string{"auth.login_success", "auth.login_failed"}}}
	security, _ := r.collection.CountDocuments(ctx, secFilter)

	mutFilter := bson.M{"action": bson.M{"$in": []string{"report.submit", "report.overwrite", "report.delete", "user.create", "user.update", "user.delete"}}}
	mutations, _ := r.collection.CountDocuments(ctx, mutFilter)

	var distinctActors []string
	_ = r.collection.Distinct(ctx, "actor", bson.M{}).Decode(&distinctActors)

	return &AuditStats{
		TotalEvents:    total,
		SecurityEvents: security,
		MutationEvents: mutations,
		UniqueActors:   int64(len(distinctActors)),
	}, nil
}


