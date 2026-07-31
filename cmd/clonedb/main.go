package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/config"
	"wedrink/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	utils.InitLogger(cfg.Env)

	srcDBName := "wedrink"
	dstDBName := "wedrink_test"

	if envDst := os.Getenv("TARGET_DB_NAME"); envDst != "" {
		dstDBName = envDst
	}

	slog.Info("Starting MongoDB Database Cloning Process...", "src", srcDBName, "dst", dstDBName, "uri", cfg.MongoURI)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	srcDB := client.Database(srcDBName)
	dstDB := client.Database(dstDBName)

	collections := []string{"users", "eod_reports"}

	for _, colName := range collections {
		slog.Info(fmt.Sprintf("Cloning collection '%s' from '%s' to '%s'...", colName, srcDBName, dstDBName))

		srcCol := srcDB.Collection(colName)
		dstCol := dstDB.Collection(colName)

		// Drop target collection to start fresh
		_ = dstCol.Drop(ctx)

		cursor, err := srcCol.Find(ctx, bson.M{})
		if err != nil {
			slog.Error("Failed to query source collection", "collection", colName, "error", err)
			continue
		}

		var docs []bson.M
		if err := cursor.All(ctx, &docs); err != nil {
			slog.Error("Failed to decode documents", "collection", colName, "error", err)
			_ = cursor.Close(ctx)
			continue
		}
		_ = cursor.Close(ctx)

		if len(docs) > 0 {
			docsInterface := make([]interface{}, len(docs))
			for i, d := range docs {
				docsInterface[i] = d
			}

			_, err := dstCol.InsertMany(ctx, docsInterface)
			if err != nil {
				slog.Error("Failed to insert documents into target collection", "collection", colName, "error", err)
			} else {
				slog.Info(fmt.Sprintf("Successfully cloned %d documents into '%s.%s'", len(docs), dstDBName, colName))
			}
		} else {
			slog.Info(fmt.Sprintf("Source collection '%s' was empty. Created empty target collection.", colName))
		}
	}

	// Re-create indexes on target database
	reportsCol := dstDB.Collection("eod_reports")
	_, _ = reportsCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "report_date", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_unique_report_date"),
	})

	usersCol := dstDB.Collection("users")
	_, _ = usersCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_unique_username"),
	})

	slog.Info("Database cloning & indexing completed successfully!", "targetDB", dstDBName)
}
