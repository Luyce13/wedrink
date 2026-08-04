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

type NotificationRepository struct {
	collection *mongo.Collection
}

func NewNotificationRepository(db *mongo.Database) *NotificationRepository {
	return &NotificationRepository{
		collection: db.Collection("notifications"),
	}
}

func (r *NotificationRepository) Create(ctx context.Context, notif *models.Notification) error {
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}
	res, err := r.collection.InsertOne(ctx, notif)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		notif.ID = oid
	}
	return nil
}

func (r *NotificationRepository) FindUnread(ctx context.Context) ([]models.Notification, error) {
	filter := bson.M{"is_read": false}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query unread notifications: %w", err)
	}
	defer cursor.Close(ctx)

	var notifs []models.Notification
	if err := cursor.All(ctx, &notifs); err != nil {
		return nil, fmt.Errorf("failed to decode unread notifications: %w", err)
	}

	return notifs, nil
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context) (int64, error) {
	filter := bson.M{"is_read": false}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, idStr string) error {
	oid, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		return fmt.Errorf("invalid notification ID format: %w", err)
	}

	filter := bson.M{"_id": oid}
	update := bson.M{"$set": bson.M{"is_read": true}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) FindByReportID(ctx context.Context, reportID string) (*models.Notification, error) {
	filter := bson.M{"report_id": reportID}
	var notif models.Notification
	err := r.collection.FindOne(ctx, filter).Decode(&notif)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find notification by report_id: %w", err)
	}
	return &notif, nil
}

func (r *NotificationRepository) FindByReportDate(ctx context.Context, reportDate string) (*models.Notification, error) {
	filter := bson.M{"report_date": reportDate}
	var notif models.Notification
	err := r.collection.FindOne(ctx, filter).Decode(&notif)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find notification by report_date: %w", err)
	}
	return &notif, nil
}

func (r *NotificationRepository) Update(ctx context.Context, notif *models.Notification) error {
	filter := bson.M{"_id": notif.ID}
	update := bson.M{
		"$set": bson.M{
			"notes":        notif.Notes,
			"submitted_by": notif.SubmittedBy,
			"is_read":      notif.IsRead,
			"created_at":   notif.CreatedAt,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}
	return nil
}
