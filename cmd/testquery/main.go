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
	mongoDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer mongoDB.Close(context.Background())

	repo := repository.NewReportRepository(mongoDB.Database)
	ctx := context.Background()

	reports, err := repo.FindWithParams(ctx, repository.ReportQueryParams{
		SortBy:    "total_sale",
		SortOrder: "desc",
		Limit:     5,
	})
	if err != nil {
		fmt.Printf("FindWithParams error: %v\n", err)
		return
	}
	fmt.Printf("Top 5 TotalSale desc reports:\n")
	for _, r := range reports {
		fmt.Printf("  Date: %s, TotalSale: %.0f, Expenses: %.0f\n", r.ReportDate, r.TotalSale, r.OtherPayments)
	}
}
