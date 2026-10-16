package main

import (
	"context"
	"fmt"
	"log"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/repository"
)

func main() {
	cfg := config.LoadConfig()
	if cfg.DBDriver == "sqlite" {
		sqliteDB, err := db.ConnectSQLite(cfg.SQLitePath)
		if err != nil {
			log.Fatalf("Failed to connect SQLite: %v", err)
		}
		defer sqliteDB.Close()

		repo := repository.NewSQLiteReportRepository(sqliteDB.DB)
		ctx := context.Background()

		reports, err := repo.FindWithParams(ctx, repository.ReportQueryParams{
			Limit: 100,
		})
		if err != nil {
			fmt.Printf("FindWithParams error: %v\n", err)
			return
		}
		fmt.Printf("Found %d reports in SQLite\n", len(reports))
		for i, r := range reports {
			if i < 5 {
				fmt.Printf("  Date: %s, TotalSale: %.0f, Expenses: %d\n", r.ReportDate, r.TotalSale, len(r.Expenses))
			}
		}
	} else {
		mongoDB, err := db.Connect(cfg)
		if err != nil {
			log.Fatalf("Failed to connect MongoDB: %v", err)
		}
		defer func() {
			_ = mongoDB.Close(context.Background())
		}()

		repo := repository.NewReportRepository(mongoDB.Database)
		ctx := context.Background()

		reports, err := repo.FindWithParams(ctx, repository.ReportQueryParams{
			Limit: 100,
		})
		if err != nil {
			fmt.Printf("FindWithParams error: %v\n", err)
			return
		}
		fmt.Printf("Found %d reports in MongoDB\n", len(reports))
		for i, r := range reports {
			if i < 5 {
				fmt.Printf("  Date: %s, TotalSale: %.0f, Expenses: %d\n", r.ReportDate, r.TotalSale, len(r.Expenses))
			}
		}
	}
}
