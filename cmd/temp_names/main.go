package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"wedrink/internal/config"
	"wedrink/internal/db"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DeductionResult struct {
	StaffName   string
	YearMonth   string
	Items       []ItemDetail
	TotalAmount float64
	TotalCount  int
}

type ItemDetail struct {
	Date        string  `json:"date"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
}

// QueryDeductions runs an aggregation pipeline for an employee in a given month.
// It filters solely for advances and orders, and distinguishes between Senior and Junior.
func QueryDeductions(ctx context.Context, coll *mongo.Collection, staffKey string, yearMonth string) (*DeductionResult, error) {
	var pattern string

	switch strings.ToLower(staffKey) {
	case "faizan_j", "faizan j", "faizan (j)":
		// Must have faizan AND (j or (j)) AND (order or advance)
		pattern = `(?=.*faizan)(?=.*(\bj\b|\(j\)))(?=.*(order|advance))`
	case "faizan_s", "faizan s", "faizan (s)":
		// Must have faizan AND (s or (s)) AND (order or advance) - strictly Senior
		pattern = `(?=.*faizan)(?=.*(\bs\b|\(s\)))(?=.*(order|advance))`
	case "faizan_ambiguous":
		// Has faizan AND (order or advance) but NO 's' or 'j' qualifier
		pattern = `(?=.*faizan)(?!.*(\b[sj]\b|\([sj]\)))(?=.*(order|advance))`
	default:
		// General staff member (e.g. "adnan", "bisma", "abdullah")
		pattern = fmt.Sprintf(`(?=.*%s)(?=.*(order|advance))`, strings.ToLower(staffKey))
	}

	pipeline := bson.A{
		// 1. Month filter & ignore deleted reports (uses report_date index)
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "report_date", Value: bson.D{{Key: "$regex", Value: fmt.Sprintf("^%s", yearMonth)}}},
			{Key: "is_deleted", Value: bson.D{{Key: "$ne", Value: true}}},
		}}},
		// 2. Unwind expenses
		bson.D{{Key: "$unwind", Value: "$expenses"}},
		// 3. Match only target staff deductions
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "expenses.description", Value: bson.D{
				{Key: "$regex", Value: pattern},
				{Key: "$options", Value: "i"},
			}},
		}}},
		// 4. Project and sort chronologically
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "date", Value: "$report_date"},
			{Key: "description", Value: "$expenses.description"},
			{Key: "amount", Value: "$expenses.amount"},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "date", Value: 1}}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rawItems []struct {
		Date        string  `bson:"date"`
		Description string  `bson:"description"`
		Amount      float64 `bson:"amount"`
	}
	if err := cursor.All(ctx, &rawItems); err != nil {
		return nil, err
	}

	res := &DeductionResult{
		StaffName: staffKey,
		YearMonth: yearMonth,
	}

	for _, it := range rawItems {
		cat := "Advance"
		lower := strings.ToLower(it.Description)
		if strings.Contains(lower, "order") {
			cat = "Order"
		}
		res.Items = append(res.Items, ItemDetail{
			Date:        it.Date,
			Description: it.Description,
			Amount:      it.Amount,
			Category:    cat,
		})
		res.TotalAmount += it.Amount
		res.TotalCount++
	}

	return res, nil
}

func printResult(title string, res *DeductionResult) {
	fmt.Println("================================================================================")
	fmt.Printf(" %s (Month: %s)\n", title, res.YearMonth)
	fmt.Println("================================================================================")
	if len(res.Items) == 0 {
		fmt.Println(" (No deduction records found)")
		fmt.Println()
		return
	}
	fmt.Printf("%-12s | %-8s | %-12s | %s\n", "Date", "Type", "Amount (PKR)", "Description in DB")
	fmt.Println(strings.Repeat("-", 80))
	for _, it := range res.Items {
		fmt.Printf("%-12s | %-8s | %-12.0f | %s\n", it.Date, it.Category, it.Amount, it.Description)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("TOTAL DEDUCTIONS: PKR %.0f (%d items)\n\n", res.TotalAmount, res.TotalCount)
}

func main() {
	cfg := config.LoadConfig()
	mongoDB, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoDB.Close(ctx)
	}()

	coll := mongoDB.Database.Collection("eod_reports")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println("\n>>> RUNNING TEST: MONTHLY SALARY DEDUCTIONS TEST HARNESS <<<")

	// Test 1: Faizan (Junior) in September 2026
	jRes, err := QueryDeductions(ctx, coll, "faizan_j", "2026-09")
	if err != nil {
		log.Fatalf("Test 1 error: %v", err)
	}
	printResult("FAIZAN (JUNIOR) - SEPTEMBER 2026 DEDUCTIONS", jRes)

	// Test 2: Faizan (Senior) in September 2026
	sRes, err := QueryDeductions(ctx, coll, "faizan_s", "2026-09")
	if err != nil {
		log.Fatalf("Test 2 error: %v", err)
	}
	printResult("FAIZAN (SENIOR) - SEPTEMBER 2026 DEDUCTIONS", sRes)

	// Test 3: Ambiguous Faizan Entries in September 2026 (Missing S or J)
	ambRes, err := QueryDeductions(ctx, coll, "faizan_ambiguous", "2026-09")
	if err != nil {
		log.Fatalf("Test 3 error: %v", err)
	}
	printResult("AMBIGUOUS ENTRIES (CASHIER WROTE 'FAIZAN' WITHOUT S OR J)", ambRes)

	// Test 4: Faizan in August 2026 (Before the second Faizan joined)
	augRes, err := QueryDeductions(ctx, coll, "faizan", "2026-08")
	if err != nil {
		log.Fatalf("Test 4 error: %v", err)
	}
	printResult("FAIZAN (ALL) - AUGUST 2026 DEDUCTIONS", augRes)

	// Test 5: Another Staff Member - Adnan in August 2026
	adnanRes, err := QueryDeductions(ctx, coll, "adnan", "2026-08")
	if err != nil {
		log.Fatalf("Test 5 error: %v", err)
	}
	printResult("ADNAN - AUGUST 2026 DEDUCTIONS", adnanRes)
}
