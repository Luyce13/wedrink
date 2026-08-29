package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

// parseAmount cleans and parses a monetary string value to float64.
// Empty strings are treated as zero. Commas are stripped from formatted inputs.
// Negative values are rejected.
func parseAmount(val string) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}
	clean := strings.ReplaceAll(val, ",", "")
	amt, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	if amt < 0 {
		return 0, models.ErrNegativeAmount
	}
	return amt, nil
}

type ReportService struct {
	repo         repository.ReportRepo
	auditService *AuditService
}

func NewReportService(repo repository.ReportRepo, auditService *AuditService) *ReportService {
	return &ReportService{repo: repo, auditService: auditService}
}

type SubmitReportInput struct {
	ReportDate      string
	TotalSale       string
	CreditSale      string
	BankTransfer    string
	CounterCash     string
	ExpenseDescs    []string
	ExpenseAmounts  []string
	Notes           string
	SubmittedByID   string
	SubmittedBy     string
	SubmittedByRole string
	AllowOverwrite  bool
	Req             *http.Request
}

func (s *ReportService) ProcessAndSaveReport(ctx context.Context, input SubmitReportInput) (*models.EODReport, error) {
	if strings.TrimSpace(input.ReportDate) == "" {
		return nil, models.ErrReportDateRequired
	}

	if utils.IsBeforeMinDate(input.ReportDate) {
		return nil, fmt.Errorf("%w: %s (minimum: %s)", models.ErrDateBeforeMinimum, input.ReportDate, utils.MinDateStr)
	}

	if utils.IsFutureDate(input.ReportDate) {
		return nil, fmt.Errorf("%w: %s", models.ErrFutureDate, input.ReportDate)
	}

	totalSale, err := parseAmount(input.TotalSale)
	if err != nil {
		if errors.Is(err, models.ErrNegativeAmount) {
			return nil, fmt.Errorf("%w: Total Sale cannot be negative", models.ErrNegativeAmount)
		}
		return nil, fmt.Errorf("%w: Total Sale", models.ErrInvalidAmount)
	}

	creditSale, err := parseAmount(input.CreditSale)
	if err != nil {
		if errors.Is(err, models.ErrNegativeAmount) {
			return nil, fmt.Errorf("%w: Credit Card Sale cannot be negative", models.ErrNegativeAmount)
		}
		return nil, fmt.Errorf("%w: Credit Card Sale", models.ErrInvalidAmount)
	}

	bankTransfer, err := parseAmount(input.BankTransfer)
	if err != nil {
		if errors.Is(err, models.ErrNegativeAmount) {
			return nil, fmt.Errorf("%w: Bank Transfer cannot be negative", models.ErrNegativeAmount)
		}
		return nil, fmt.Errorf("%w: Bank Transfer", models.ErrInvalidAmount)
	}

	counterCash, err := parseAmount(input.CounterCash)
	if err != nil {
		if errors.Is(err, models.ErrNegativeAmount) {
			return nil, fmt.Errorf("%w: Counter Cash cannot be negative", models.ErrNegativeAmount)
		}
		return nil, fmt.Errorf("%w: Counter Cash", models.ErrInvalidAmount)
	}

	// Process expenses
	expenses := make([]models.ExpenseItem, 0)
	var totalExpenses float64 = 0

	for i := 0; i < len(input.ExpenseDescs); i++ {
		desc := strings.TrimSpace(input.ExpenseDescs[i])
		amtStr := ""
		if i < len(input.ExpenseAmounts) {
			amtStr = input.ExpenseAmounts[i]
		}
		if desc != "" && amtStr != "" {
			amt, err := parseAmount(amtStr)
			if err != nil {
				if errors.Is(err, models.ErrNegativeAmount) {
					return nil, fmt.Errorf("%w: Expense amount cannot be negative (%s)", models.ErrNegativeAmount, desc)
				}
				return nil, fmt.Errorf("%w: Expense amount (%s)", models.ErrInvalidAmount, desc)
			}
			if amt > 0 {
				posAmt := math.Round(amt)
				totalExpenses += posAmt
				expenses = append(expenses, models.ExpenseItem{
					ID:          fmt.Sprintf("exp_%d", time.Now().UnixNano()+int64(i)),
					Description: desc,
					Amount:      posAmt,
				})
			}
		}
	}

	totalSale = math.Round(totalSale)
	creditSale = math.Round(creditSale)
	bankTransfer = math.Round(bankTransfer)
	counterCash = math.Round(counterCash)

	expectedCash := totalSale - totalExpenses - creditSale - bankTransfer
	difference := counterCash - expectedCash

	// Check if a report for this date already exists
	existing, err := s.repo.FindByDate(ctx, input.ReportDate)
	if err != nil {
		return nil, err
	}

	if existing != nil && !input.AllowOverwrite {
		return nil, fmt.Errorf("%w: %s. Contact a Manager to edit", models.ErrDuplicateReport, input.ReportDate)
	}

	report := &models.EODReport{
		ReportID:        fmt.Sprintf("eod_%s_%d", input.ReportDate, time.Now().Unix()),
		ReportDate:      input.ReportDate,
		TotalSale:       totalSale,
		CreditSale:      creditSale,
		BankTransfer:    bankTransfer,
		OtherPayments:   totalExpenses,
		ExpectedCash:    expectedCash,
		CounterCash:     counterCash,
		Difference:      difference,
		Expenses:        expenses,
		SubmittedBy:     input.SubmittedBy,
		SubmittedByRole: input.SubmittedByRole,
		Notes:           strings.TrimSpace(input.Notes),
	}

	if existing != nil && input.AllowOverwrite {
		report.ID = existing.ID
		report.ReportID = existing.ReportID
		report.CreatedAt = existing.CreatedAt
		err = s.repo.Update(ctx, report)
		if err == nil && s.auditService != nil {
			s.auditService.Record(ctx, RecordAuditInput{
				ActorID:    input.SubmittedByID,
				Actor:      input.SubmittedBy,
				Role:       input.SubmittedByRole,
				Action:     "report.overwrite",
				ResourceID: report.ReportDate,
				Req:        input.Req,
				OldState:   existing,
				NewState:   report,
			})
		}
	} else {
		err = s.repo.Create(ctx, report)
		if err == nil && s.auditService != nil {
			s.auditService.Record(ctx, RecordAuditInput{
				ActorID:    input.SubmittedByID,
				Actor:      input.SubmittedBy,
				Role:       input.SubmittedByRole,
				Action:     "report.submit",
				ResourceID: report.ReportDate,
				Req:        input.Req,
				OldState:   nil,
				NewState:   report,
			})
		}
	}

	if err != nil {
		return nil, err
	}

	return report, nil
}

func (s *ReportService) GetReportByDate(ctx context.Context, dateStr string) (*models.EODReport, error) {
	return s.repo.FindByDate(ctx, dateStr)
}

func (s *ReportService) GetReportByID(ctx context.Context, idStr string) (*models.EODReport, error) {
	return s.repo.FindByID(ctx, idStr)
}

func (s *ReportService) GetReportsForMonth(ctx context.Context, yearMonth string) ([]models.EODReport, error) {
	return s.repo.FindByMonth(ctx, yearMonth)
}

func (s *ReportService) GetSubmittedDatesForMonth(ctx context.Context, yearMonth string) ([]string, error) {
	return s.repo.GetSubmittedDatesByMonth(ctx, yearMonth)
}

func (s *ReportService) GetReportsForRange(ctx context.Context, startDate, endDate string) ([]models.EODReport, error) {
	return s.repo.FindByDateRange(ctx, startDate, endDate)
}

func (s *ReportService) GetAllReports(ctx context.Context) ([]models.EODReport, error) {
	return s.repo.FindAll(ctx, 0) // 0 = no limit
}

func (s *ReportService) GetReportsWithParams(ctx context.Context, params repository.ReportQueryParams) ([]models.EODReport, error) {
	return s.repo.FindWithParams(ctx, params)
}


func (s *ReportService) GetMonthlySummary(ctx context.Context, yearMonth string) (*models.MonthlySummary, error) {
	return s.repo.CalculateMonthlySummary(ctx, yearMonth)
}

func (s *ReportService) DeleteReport(ctx context.Context, idStr string, actorUsername string, actorRole string, actorID string, req *http.Request) error {
	oldReport, _ := s.repo.FindByID(ctx, idStr)
	err := s.repo.Delete(ctx, idStr, actorUsername)
	if err == nil && s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			ActorID:    actorID,
			Actor:      actorUsername,
			Role:       actorRole,
			Action:     "report.delete",
			ResourceID: idStr,
			Req:        req,
			OldState:   oldReport,
			NewState:   nil,
		})
	}
	return err
}


