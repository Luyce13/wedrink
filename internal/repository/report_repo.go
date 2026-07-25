package repository

import (
	"context"
	"errors"
	"fmt"
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
	objID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		// Try report_id fallback
		var rep models.EODReport
		err2 := r.collection.FindOne(ctx, bson.M{"report_id": idStr}).Decode(&rep)
		if err2 != nil {
			if errors.Is(err2, mongo.ErrNoDocuments) {
				return nil, nil
			}
			return nil, err2
		}
		return &rep, nil
	}

	var report models.EODReport
	err = r.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&report)
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
	objID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		res, err2 := r.collection.DeleteOne(ctx, bson.M{"report_id": idStr})
		if err2 != nil {
			return err2
		}
		if res.DeletedCount == 0 {
			return fmt.Errorf("report not found")
		}
		return nil
	}

	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": objID})
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
