package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/models"
)

type AuditQueryParams struct {
	Actor      string
	ResourceID string
	Action     string
	Limit      int64
}

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
	filter := bson.M{}
	if params.Actor != "" {
		filter["actor"] = params.Actor
	}
	if params.ResourceID != "" {
		filter["resource_id"] = params.ResourceID
	}
	if params.Action != "" {
		filter["action"] = params.Action
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(limit)

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer cursor.Close(ctx)

	logs := make([]models.AuditLog, 0)
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode audit logs: %w", err)
	}
	return logs, nil
}
