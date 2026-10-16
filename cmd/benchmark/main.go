package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

type BenchMetric struct {
	Name        string
	MongoAvg    time.Duration
	SQLiteAvg   time.Duration
	Speedup     float64
	Iterations  int
	Description string
}

func main() {
	cfg := config.LoadConfig()
	utils.InitLogger("production") // quiet info logging for cleaner bench output

	fmt.Println("==================================================================================")
	fmt.Println(" WEDRINK DATABASE PERFORMANCE BENCHMARK: SQLITE vs MONGODB ATLAS")
	fmt.Println(" Dataset: 92 Reports | 420 Itemized Expenses | 263 Audit Logs")
	fmt.Println("==================================================================================")

	// 1. Connect to SQLite
	sqliteDB, err := db.ConnectSQLite(cfg.SQLitePath)
	if err != nil {
		slog.Error("Failed to connect to SQLite", "error", err)
		os.Exit(1)
	}
	defer sqliteDB.Close()

	sqliteReportRepo := repository.NewSQLiteReportRepository(sqliteDB.DB)
	sqliteAuditRepo := repository.NewSQLiteAuditRepository(sqliteDB.DB)
	sqliteNotifRepo := repository.NewSQLiteNotificationRepository(sqliteDB.DB)

	// 2. Connect to MongoDB Atlas (Optional comparison)
	mongoAvailable := false
	var mongoReportRepo repository.ReportRepo
	var mongoAuditRepo repository.AuditRepo
	var mongoNotifRepo repository.NotificationRepo

	ctxPing, cancelPing := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelPing()

	mongoDB, err := db.Connect(cfg)
	if err == nil && mongoDB.Client.Ping(ctxPing, nil) == nil {
		mongoAvailable = true
		mongoReportRepo = repository.NewReportRepository(mongoDB.Database)
		mongoAuditRepo = repository.NewAuditRepository(mongoDB.Database)
		mongoNotifRepo = repository.NewNotificationRepository(mongoDB.Database)
		defer func() {
			ctxClose, cancelClose := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelClose()
			_ = mongoDB.Close(ctxClose)
		}()
	} else {
		fmt.Println(" (!) Note: MongoDB Atlas not reachable from current network. Benchmarking SQLite directly.")
	}

	ctx := context.Background()
	var metrics []BenchMetric

	// Benchmark 1: FindByDate (Single Report Read)
	{
		const iters = 100
		targetDate := "2026-09-15"

		// SQLite
		start := time.Now()
		for i := 0; i < iters; i++ {
			_, _ = sqliteReportRepo.FindByDate(ctx, targetDate)
		}
		sqliteDur := time.Since(start) / time.Duration(iters)

		// Mongo
		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 20); i++ {
				_, _ = mongoReportRepo.FindByDate(ctx, targetDate)
			}
			mongoDur = time.Since(start) / time.Duration(min(iters, 20))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "FindByDate (Single Report)",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Point lookup by report_date with itemized expenses",
		})
	}

	// Benchmark 2: CalculateMonthlySummary
	{
		const iters = 100
		yearMonth := "2026-09"

		start := time.Now()
		for i := 0; i < iters; i++ {
			_, _ = sqliteReportRepo.CalculateMonthlySummary(ctx, yearMonth)
		}
		sqliteDur := time.Since(start) / time.Duration(iters)

		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 20); i++ {
				_, _ = mongoReportRepo.CalculateMonthlySummary(ctx, yearMonth)
			}
			mongoDur = time.Since(start) / time.Duration(min(iters, 20))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "Monthly Summary Aggregation",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Aggregates sales, card, transfer, expenses, cash, diff for month",
		})
	}

	// Benchmark 3: FindWithParams (Filter + Sort)
	{
		const iters = 50
		params := repository.ReportQueryParams{
			StartDate: "2026-08-01",
			EndDate:   "2026-10-31",
			SortBy:    "total_sale",
			SortOrder: "desc",
			Limit:     25,
		}

		start := time.Now()
		for i := 0; i < iters; i++ {
			_, _ = sqliteReportRepo.FindWithParams(ctx, params)
		}
		sqliteDur := time.Since(start) / time.Duration(iters)

		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 15); i++ {
				_, _ = mongoReportRepo.FindWithParams(ctx, params)
			}
			mongoDur = time.Since(start) / time.Duration(min(iters, 15))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "Filter & Sort (FindWithParams)",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Date range filter + sort by total_sale DESC (Limit 25)",
		})
	}

	// Benchmark 4: AuditLog Stats Aggregation
	{
		const iters = 100

		start := time.Now()
		for i := 0; i < iters; i++ {
			_, _ = sqliteAuditRepo.GetStats(ctx)
		}
		sqliteDur := time.Since(start) / time.Duration(iters)

		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 20); i++ {
				_, _ = mongoAuditRepo.GetStats(ctx)
			}
			mongoDur = time.Since(start) / time.Duration(min(iters, 20))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "Audit Metrics Aggregation",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Aggregates total, security, mutation, and unique actors across audit logs",
		})
	}

	// Benchmark 5: Unread Notification Count
	{
		const iters = 200

		start := time.Now()
		for i := 0; i < iters; i++ {
			_, _ = sqliteNotifRepo.GetUnreadCount(ctx)
		}
		sqliteDur := time.Since(start) / time.Duration(iters)

		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 20); i++ {
				_, _ = mongoNotifRepo.GetUnreadCount(ctx)
			}
			mongoDur = time.Since(start) / time.Duration(min(iters, 20))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "Notification Unread Count",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Indexed count of unread store alerts",
		})
	}

	// Benchmark 6: Write Transaction (Insert + Cleanup)
	{
		const iters = 50
		testDate := "2099-12-31"

		start := time.Now()
		for i := 0; i < iters; i++ {
			_ = sqliteReportRepo.Delete(ctx, testDate, "bench")
			_ = sqliteReportRepo.Create(ctx, &models.EODReport{
				ReportID:      fmt.Sprintf("eod_bench_%d", i),
				ReportDate:    testDate,
				TotalSale:     60000,
				CreditSale:    10000,
				BankTransfer:  5000,
				OtherPayments: 2000,
				ExpectedCash:  43000,
				CounterCash:   43000,
				SubmittedBy:   "bench",
				Expenses: []models.ExpenseItem{
					{Description: "Cups", Amount: 1000},
					{Description: "Straws", Amount: 1000},
				},
			})
		}
		_ = sqliteReportRepo.Delete(ctx, testDate, "bench")
		sqliteDur := time.Since(start) / time.Duration(iters)

		mongoDur := time.Duration(0)
		if mongoAvailable {
			start = time.Now()
			for i := 0; i < min(iters, 10); i++ {
				_ = mongoReportRepo.Delete(ctx, testDate, "bench")
				_ = mongoReportRepo.Create(ctx, &models.EODReport{
					ReportID:      fmt.Sprintf("eod_bench_%d", i),
					ReportDate:    testDate,
					TotalSale:     60000,
					CreditSale:    10000,
					BankTransfer:  5000,
					OtherPayments: 2000,
					ExpectedCash:  43000,
					CounterCash:   43000,
					SubmittedBy:   "bench",
					Expenses: []models.ExpenseItem{
						{Description: "Cups", Amount: 1000},
						{Description: "Straws", Amount: 1000},
					},
				})
			}
			_ = mongoReportRepo.Delete(ctx, testDate, "bench")
			mongoDur = time.Since(start) / time.Duration(min(iters, 10))
		}

		speedup := 0.0
		if mongoDur > 0 {
			speedup = float64(mongoDur) / float64(sqliteDur)
		}

		metrics = append(metrics, BenchMetric{
			Name:        "Report Write Transaction",
			SQLiteAvg:   sqliteDur,
			MongoAvg:    mongoDur,
			Speedup:     speedup,
			Iterations:  iters,
			Description: "Full report insert with relational expenses & cascade delete",
		})
	}

	// Print Results Table
	fmt.Println()
	if mongoAvailable {
		fmt.Printf("%-32s | %-14s | %-14s | %-10s | %s\n", "Operation", "SQLite WAL", "MongoDB Atlas", "Speedup", "Throughput (SQLite)")
		fmt.Println(strings.Repeat("-", 100))
		for _, m := range metrics {
			opsPerSec := 1.0 / m.SQLiteAvg.Seconds()
			speedupStr := fmt.Sprintf("%.1fx faster", m.Speedup)
			if m.Speedup == 0 {
				speedupStr = "N/A"
			}
			fmt.Printf("%-32s | %-14s | %-14s | %-10s | %s\n",
				m.Name,
				formatDuration(m.SQLiteAvg),
				formatDuration(m.MongoAvg),
				speedupStr,
				formatOps(opsPerSec),
			)
		}
	} else {
		fmt.Printf("%-34s | %-16s | %-20s | %s\n", "Operation", "SQLite Latency", "Throughput", "Details")
		fmt.Println(strings.Repeat("-", 100))
		for _, m := range metrics {
			opsPerSec := 1.0 / m.SQLiteAvg.Seconds()
			fmt.Printf("%-34s | %-16s | %-20s | %s\n",
				m.Name,
				formatDuration(m.SQLiteAvg),
				formatOps(opsPerSec),
				m.Description,
			)
		}
	}
	fmt.Println(strings.Repeat("-", 100))
	fmt.Println()
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Nanoseconds())/1000.0)
	}
	return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000.0)
}

func formatOps(ops float64) string {
	if ops > 1000000 {
		return fmt.Sprintf("%.1fM ops/sec", ops/1000000.0)
	}
	if ops > 1000 {
		return fmt.Sprintf("%.1fk ops/sec", ops/1000.0)
	}
	return fmt.Sprintf("%.0f ops/sec", math.Round(ops))
}
