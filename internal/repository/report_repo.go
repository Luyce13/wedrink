package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"wedrink/internal/models"
)

type ReportRepository struct {
	collection *mongo.Collection
}

func NewReportRepository(db *mongo.Database) *ReportRepository {
	return &ReportRepository{
		collection: db.Collection("eod_reports"),
	}
}

func (r *ReportRepository) Create(ctx context.Context, report *models.EODReport) error {
	now := time.Now()
	report.CreatedAt = now
	report.UpdatedAt = now
	if report.ID.IsZero() {
		report.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, report)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("a report for date %s already exists", report.ReportDate)
		}
		return fmt.Errorf("failed to save report: %w", err)
	}
	return nil
}

func (r *ReportRepository) Update(ctx context.Context, report *models.EODReport) error {
	report.UpdatedAt = time.Now()
	filter := bson.M{"_id": report.ID}
	update := bson.M{"$set": report}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update report: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}

func (r *ReportRepository) UpsertByDate(ctx context.Context, report *models.EODReport) error {
	report.UpdatedAt = time.Now()
	
	existing, err := r.FindByDate(ctx, report.ReportDate)
	if err != nil {
		return err
	}
	
	if existing != nil {
		report.ID = existing.ID
		report.CreatedAt = existing.CreatedAt
		return r.Update(ctx, report)
	}
	
	return r.Create(ctx, report)
}

func (r *ReportRepository) FindByID(ctx context.Context, idStr string) (*models.EODReport, error) {
	if idStr == "" {
		return nil, nil
	}

	orConditions := []bson.M{
		{"report_id": idStr},
		{"report_date": idStr},
	}
	if objID, err := bson.ObjectIDFromHex(idStr); err == nil {
		orConditions = append(orConditions, bson.M{"_id": objID})
	}

	var report models.EODReport
	err := r.collection.FindOne(ctx, bson.M{"$or": orConditions}).Decode(&report)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch report by id: %w", err)
	}
	return &report, nil
}

func (r *ReportRepository) FindByDate(ctx context.Context, dateStr string) (*models.EODReport, error) {
	var report models.EODReport
	err := r.collection.FindOne(ctx, bson.M{"report_date": dateStr}).Decode(&report)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch report by date: %w", err)
	}
	return &report, nil
}

func (r *ReportRepository) FindByMonth(ctx context.Context, yearMonth string) ([]models.EODReport, error) {
	// yearMonth is format YYYY-MM
	filter := bson.M{
		"report_date": bson.M{
			"$regex": "^" + yearMonth,
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "report_date", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch monthly reports: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []models.EODReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("failed to decode monthly reports: %w", err)
	}
	return reports, nil
}

func (r *ReportRepository) GetSubmittedDatesByMonth(ctx context.Context, yearMonth string) ([]string, error) {
	filter := bson.M{
		"report_date": bson.M{
			"$regex": "^" + yearMonth,
		},
	}
	opts := options.Find().SetProjection(bson.M{"report_date": 1, "_id": 0})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submitted dates: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		ReportDate string `bson:"report_date"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode submitted dates: %w", err)
	}

	dates := make([]string, 0, len(results))
	for _, res := range results {
		if res.ReportDate != "" {
			dates = append(dates, res.ReportDate)
		}
	}
	return dates, nil
}

func (r *ReportRepository) FindByDateRange(ctx context.Context, startDate, endDate string) ([]models.EODReport, error) {
	filter := bson.M{}
	if startDate != "" && endDate != "" {
		filter["report_date"] = bson.M{
			"$gte": startDate,
			"$lte": endDate,
		}
	} else if startDate != "" {
		filter["report_date"] = bson.M{"$gte": startDate}
	} else if endDate != "" {
		filter["report_date"] = bson.M{"$lte": endDate}
	}

	opts := options.Find().SetSort(bson.D{{Key: "report_date", Value: -1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch date range reports: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []models.EODReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("failed to decode range reports: %w", err)
	}
	return reports, nil
}

type ReportQueryParams struct {
	StartDate string
	EndDate   string
	SortBy    string
	SortOrder string
	Cursor    string
	Limit     int
}

func (r *ReportRepository) FindWithParams(ctx context.Context, params ReportQueryParams) ([]models.EODReport, error) {
	filter := bson.M{}

	if params.StartDate != "" && params.EndDate != "" {
		filter["report_date"] = bson.M{
			"$gte": params.StartDate,
			"$lte": params.EndDate,
		}
	} else if params.StartDate != "" {
		filter["report_date"] = bson.M{"$gte": params.StartDate}
	} else if params.EndDate != "" {
		filter["report_date"] = bson.M{"$lte": params.EndDate}
	}

	sortKey := "report_date"
	switch params.SortBy {
	case "total_sale":
		sortKey = "total_sale"
	case "credit_sale":
		sortKey = "credit_sale"
	case "bank_transfer":
		sortKey = "bank_transfer"
	case "other_payments", "expenses":
		sortKey = "other_payments"
	case "difference":
		sortKey = "difference"
	default:
		sortKey = "report_date"
	}

	sortDir := -1
	if params.SortOrder == "asc" {
		sortDir = 1
	}

	if params.Cursor != "" {
		if sortKey == "report_date" {
			if sortDir == -1 {
				filter["report_date"] = bson.M{"$lt": params.Cursor}
			} else {
				filter["report_date"] = bson.M{"$gt": params.Cursor}
			}
		} else {
			parts := strings.Split(params.Cursor, "|")
			if len(parts) == 2 {
				valStr, cursorDate := parts[0], parts[1]
				val, _ := strconv.ParseFloat(valStr, 64)
				if sortDir == -1 {
					filter["$or"] = []bson.M{
						{sortKey: bson.M{"$lt": val}},
						{sortKey: val, "report_date": bson.M{"$lt": cursorDate}},
					}
				} else {
					filter["$or"] = []bson.M{
						{sortKey: bson.M{"$gt": val}},
						{sortKey: val, "report_date": bson.M{"$gt": cursorDate}},
					}
				}
			} else {
				if sortDir == -1 {
					filter["report_date"] = bson.M{"$lt": params.Cursor}
				} else {
					filter["report_date"] = bson.M{"$gt": params.Cursor}
				}
			}
		}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	sortFields := bson.D{{Key: sortKey, Value: sortDir}}
	if sortKey != "report_date" {
		sortFields = append(sortFields, bson.E{Key: "report_date", Value: sortDir})
	}

	opts := options.Find().
		SetSort(sortFields).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reports with params: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []models.EODReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("failed to decode reports with params: %w", err)
	}

	return reports, nil
}

func (r *ReportRepository) FindAll(ctx context.Context, limit int) ([]models.EODReport, error) {
	opts := options.Find().SetSort(bson.D{{Key: "report_date", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reports: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []models.EODReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("failed to decode reports: %w", err)
	}
	return reports, nil
}

func (r *ReportRepository) Delete(ctx context.Context, idStr string) error {
	if idStr == "" {
		return fmt.Errorf("report id required")
	}

	orConditions := []bson.M{
		{"report_id": idStr},
		{"report_date": idStr},
	}
	if objID, err := bson.ObjectIDFromHex(idStr); err == nil {
		orConditions = append(orConditions, bson.M{"_id": objID})
	}

	res, err := r.collection.DeleteOne(ctx, bson.M{"$or": orConditions})
	if err != nil {
		return fmt.Errorf("failed to delete report: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}

func (r *ReportRepository) CalculateMonthlySummary(ctx context.Context, yearMonth string) (*models.MonthlySummary, error) {
	reports, err := r.FindByMonth(ctx, yearMonth)
	if err != nil {
		return nil, err
	}

	summary := &models.MonthlySummary{
		YearMonth:   yearMonth,
		ReportCount: len(reports),
	}

	for _, rep := range reports {
		summary.TotalSale += rep.TotalSale
		summary.TotalCredit += rep.CreditSale
		summary.TotalBank += rep.BankTransfer
		summary.TotalExpenses += rep.OtherPayments
		summary.ExpectedCash += rep.ExpectedCash
		summary.CounterCash += rep.CounterCash
		summary.TotalDifference += rep.Difference
	}

	return summary, nil
}
