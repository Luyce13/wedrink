package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/config"
)

type Database struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func Connect(cfg *config.Config) (*Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetBSONOptions(&options.BSONOptions{
			ObjectIDAsHexString: true,
		})

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongodb client: %w", err)
	}

	// Ping database to verify connection (important for Atlas)
	if err := client.Ping(ctx, nil); err != nil {
		slog.Warn("MongoDB ping warning (check MONGO_URI string or network connection)", "error", err, "uri", maskURI(cfg.MongoURI))
	} else {
		slog.Info("Successfully connected to MongoDB Atlas / Database", "database", cfg.MongoDBName)
	}

	db := client.Database(cfg.MongoDBName)

	// Startup migration & index setup
	go func() {
		idxCtx, idxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer idxCancel()

		reportsCol := db.Collection("eod_reports")
		usersCol := db.Collection("users")
		auditCol := db.Collection("audit_logs")

		// 1. Backfill migration: ensure all existing records have is_deleted: false if field is un-set
		unflaggedFilter := bson.M{"is_deleted": bson.M{"$exists": false}}
		setFalse := bson.M{"$set": bson.M{"is_deleted": false}}
		if res, err := reportsCol.UpdateMany(idxCtx, unflaggedFilter, setFalse); err == nil && res.ModifiedCount > 0 {
			slog.Info("Backfilled soft-delete field on reports", "modifiedCount", res.ModifiedCount)
		}
		if res, err := usersCol.UpdateMany(idxCtx, unflaggedFilter, setFalse); err == nil && res.ModifiedCount > 0 {
			slog.Info("Backfilled soft-delete field on users", "modifiedCount", res.ModifiedCount)
		}

		// 2. Drop legacy un-partitioned indexes if present so partial unique indexes can be created
		_ = reportsCol.Indexes().DropOne(idxCtx, "idx_unique_report_date")
		_ = usersCol.Indexes().DropOne(idxCtx, "idx_unique_username")

		// 3. Create Partial Unique Indexes (filtering strictly on is_deleted == false)
		partialActiveFilter := bson.D{{Key: "is_deleted", Value: false}}

		if _, err := reportsCol.Indexes().CreateOne(idxCtx, mongo.IndexModel{
			Keys: bson.D{{Key: "report_date", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_unique_active_report_date").
				SetPartialFilterExpression(partialActiveFilter),
		}); err != nil {
			slog.Error("Failed to create idx_unique_active_report_date partial index", "error", err)
		}

		if _, err := usersCol.Indexes().CreateOne(idxCtx, mongo.IndexModel{
			Keys: bson.D{{Key: "username", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_unique_active_username").
				SetPartialFilterExpression(partialActiveFilter).
				SetCollation(&options.Collation{
					Locale:   "en",
					Strength: 2,
				}),
		}); err != nil {
			slog.Error("Failed to create idx_unique_active_username partial index", "error", err)
		}

		// 4. Create Audit Log Indexes
		auditModels := []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "timestamp", Value: -1}},
				Options: options.Index().SetName("idx_audit_timestamp"),
			},
			{
				Keys:    bson.D{{Key: "actor", Value: 1}},
				Options: options.Index().SetName("idx_audit_actor"),
			},
			{
				Keys:    bson.D{{Key: "resource_id", Value: 1}},
				Options: options.Index().SetName("idx_audit_resource_id"),
			},
		}
		if _, err := auditCol.Indexes().CreateMany(idxCtx, auditModels); err != nil {
			slog.Error("Failed to create audit_logs indexes", "error", err)
		}
	}()

	return &Database{
		Client:   client,
		Database: db,
	}, nil
}

func (d *Database) Close(ctx context.Context) error {
	if d != nil && d.Client != nil {
		return d.Client.Disconnect(ctx)
	}
	return nil
}

func maskURI(uri string) string {
	if len(uri) > 20 {
		return uri[:12] + "..." + uri[len(uri)-5:]
	}
	return uri
}
