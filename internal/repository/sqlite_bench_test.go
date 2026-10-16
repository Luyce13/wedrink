package repository

import (
	"context"
	"fmt"
	"testing"

	"wedrink/internal/db"
	"wedrink/internal/models"
)

func setupBenchSQLite(b *testing.B, numReports int) (*db.SQLiteDB, *SQLiteReportRepository, func()) {
	b.Helper()
	sqliteDB, err := db.ConnectSQLite(":memory:")
	if err != nil {
		b.Fatalf("Failed to open bench sqlite: %v", err)
	}

	repo := NewSQLiteReportRepository(sqliteDB.DB)
	ctx := context.Background()

	// Pre-seed reports
	for i := 1; i <= numReports; i++ {
		dateStr := fmt.Sprintf("2026-09-%02d", (i%30)+1)
		rep := &models.EODReport{
			ReportID:      fmt.Sprintf("eod_%s_%d", dateStr, i),
			ReportDate:    fmt.Sprintf("2026-%02d-%02d", (i/28)+1, (i%28)+1),
			TotalSale:     50000 + float64(i*100),
			CreditSale:    10000,
			BankTransfer:  5000,
			OtherPayments: 2000,
			ExpectedCash:  33000 + float64(i*100),
			CounterCash:   33000 + float64(i*100),
			Difference:    0,
			SubmittedBy:   "cashier",
			Expenses: []models.ExpenseItem{
				{Description: "Milk", Amount: 1200},
				{Description: "Ice", Amount: 800},
			},
		}
		_ = repo.Create(ctx, rep)
	}

	cleanup := func() {
		_ = sqliteDB.Close()
	}
	return sqliteDB, repo, cleanup
}

func BenchmarkSQLite_FindByDate(b *testing.B) {
	_, repo, cleanup := setupBenchSQLite(b, 100)
	defer cleanup()

	ctx := context.Background()

	for b.Loop() {
		_, err := repo.FindByDate(ctx, "2026-01-15")
		if err != nil {
			b.Fatalf("FindByDate failed: %v", err)
		}
	}
}

func BenchmarkSQLite_CalculateMonthlySummary(b *testing.B) {
	_, repo, cleanup := setupBenchSQLite(b, 100)
	defer cleanup()

	ctx := context.Background()

	for b.Loop() {
		_, err := repo.CalculateMonthlySummary(ctx, "2026-01")
		if err != nil {
			b.Fatalf("CalculateMonthlySummary failed: %v", err)
		}
	}
}

func BenchmarkSQLite_FindWithParams_SortAndFilter(b *testing.B) {
	_, repo, cleanup := setupBenchSQLite(b, 100)
	defer cleanup()

	ctx := context.Background()
	params := ReportQueryParams{
		StartDate: "2026-01-01",
		EndDate:   "2026-02-28",
		SortBy:    "total_sale",
		SortOrder: "desc",
		Limit:     25,
	}

	for b.Loop() {
		_, err := repo.FindWithParams(ctx, params)
		if err != nil {
			b.Fatalf("FindWithParams failed: %v", err)
		}
	}
}

func BenchmarkSQLite_CreateReport_Transaction(b *testing.B) {
	sqliteDB, err := db.ConnectSQLite(":memory:")
	if err != nil {
		b.Fatalf("Failed to open bench sqlite: %v", err)
	}
	defer sqliteDB.Close()

	repo := NewSQLiteReportRepository(sqliteDB.DB)
	ctx := context.Background()

	var i int
	for b.Loop() {
		i++
		dateStr := fmt.Sprintf("20%02d-%02d-%02d", (i/365)%99+26, ((i/28)%12)+1, (i%28)+1)
		rep := &models.EODReport{
			ReportID:      fmt.Sprintf("eod_bench_%d", i),
			ReportDate:    dateStr,
			TotalSale:     75000,
			CreditSale:    15000,
			BankTransfer:  5000,
			OtherPayments: 3000,
			ExpectedCash:  52000,
			CounterCash:   52000,
			Difference:    0,
			SubmittedBy:   "benchmark",
			Expenses: []models.ExpenseItem{
				{Description: "Lemons", Amount: 1500},
				{Description: "Packaging", Amount: 1500},
			},
		}
		_ = repo.Create(ctx, rep)
	}
}

func BenchmarkSQLite_ConcurrentReads(b *testing.B) {
	_, repo, cleanup := setupBenchSQLite(b, 100)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = repo.FindByDate(ctx, "2026-01-10")
			_, _ = repo.CalculateMonthlySummary(ctx, "2026-01")
		}
	})
}
