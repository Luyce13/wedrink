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

	clientOpts := options.Client().ApplyURI(cfg.MongoURI)

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

	// Ensure unique index on report_date for EOD reports
	go func() {
		idxCtx, idxCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer idxCancel()

		reportsCol := db.Collection("eod_reports")
		_, _ = reportsCol.Indexes().CreateOne(idxCtx, mongo.IndexModel{
			Keys:    bson.D{{Key: "report_date", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_unique_report_date"),
		})

		usersCol := db.Collection("users")
		_, _ = usersCol.Indexes().CreateOne(idxCtx, mongo.IndexModel{
			Keys: bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_unique_username").SetCollation(&options.Collation{
				Locale:   "en",
				Strength: 2,
			}),
		})
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
