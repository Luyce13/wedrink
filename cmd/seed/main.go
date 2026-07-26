package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var sampleDescs = []string{
	"Milk & Dairy",
	"Ice Bags",
	"Tea Leaves & Boba",
	"Cups & Lids",
	"Cleaning Supplies",
	"Fruit / Lemons",
	"Syrup & Flavors",
	"Minor Equipment Repair",
	"Staff Refreshments",
	"Packaging Boxes",
}

var sampleNotes = []string{
	"Balanced successfully.",
	"Regular dayend closure.",
	"Slight shortage due to coin change.",
	"High card volume today.",
	"Everything verified by manager.",
	"Smooth closing shift.",
	"",
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

	collection := mongoDB.Database.Collection("eod_reports")
	ctx := context.Background()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Target 200 reports spanning 200 days going backwards from today
	startDate := time.Now().AddDate(0, 0, -200)
	var docs []any

	fmt.Println("Generating 200 reports for bulk insert...")

	for i := 0; i < 200; i++ {
		currDate := startDate.AddDate(0, 0, i)
		dateStr := currDate.Format("2006-01-02")

		totalSale := float64((r.Intn(160) + 70) * 500) // 35,000 to 115,000
		creditSale := float64((r.Intn(40) + 10) * 500) // 5,000 to 25,000
		bankTransfer := float64((r.Intn(20) + 2) * 500) // 1,000 to 11,000

		// Expenses (1 to 4 items)
		expCount := r.Intn(4) + 1
		var expenses []models.ExpenseItem
		var totalExpenses float64 = 0

		for j := 0; j < expCount; j++ {
			desc := sampleDescs[r.Intn(len(sampleDescs))]
			amt := float64((r.Intn(30) + 2) * 100) // 200 to 3,200
			totalExpenses += amt
			expenses = append(expenses, models.ExpenseItem{
				ID:          fmt.Sprintf("exp_%d_%d", currDate.Unix(), j),
				Description: desc,
				Amount:      amt,
			})
		}

		expectedCash := totalSale - creditSale - bankTransfer - totalExpenses

		// Occasional minor discrepancy
		disc := 0.0
		switch r.Intn(10) {
		case 1:
			disc = -500.0
		case 2:
			disc = -1000.0
		case 3:
			disc = 500.0
		}
		counterCash := expectedCash + disc
		difference := counterCash - expectedCash

		submitter := "System Administrator"
		role := "super_admin"
		if r.Intn(3) == 0 {
			submitter = "Staff Member"
			role = "staff"
		}

		notes := sampleNotes[r.Intn(len(sampleNotes))]

		report := models.EODReport{
			ID:              bson.NewObjectID(),
			ReportID:        fmt.Sprintf("eod_%s_%d", dateStr, currDate.Unix()),
			ReportDate:      dateStr,
			TotalSale:       totalSale,
			CreditSale:      creditSale,
			BankTransfer:    bankTransfer,
			OtherPayments:   totalExpenses,
			ExpectedCash:    expectedCash,
			CounterCash:     counterCash,
			Difference:      difference,
			Expenses:        expenses,
			SubmittedBy:     submitter,
			SubmittedByRole: role,
			Notes:           notes,
			CreatedAt:       currDate.Add(23 * time.Hour),
			UpdatedAt:       currDate.Add(23 * time.Hour),
		}

		docs = append(docs, report)
	}

	count := 0
	for _, d := range docs {
		rep := d.(models.EODReport)
		var existing models.EODReport
		err := collection.FindOne(ctx, bson.M{"report_date": rep.ReportDate}).Decode(&existing)
		if err == nil {
			continue
		}
		_, err = collection.InsertOne(ctx, rep)
		if err == nil {
			count++
		}
	}

	fmt.Printf("Successfully seeded %d reports into MongoDB!\n", count)
}
