package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

type ReportService struct {
	repo *repository.ReportRepository
}

func NewReportService(repo *repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
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
	SubmittedBy     string
	SubmittedByRole string
	AllowOverwrite  bool
}

func (s *ReportService) ProcessAndSaveReport(ctx context.Context, input SubmitReportInput) (*models.EODReport, error) {
	if strings.TrimSpace(input.ReportDate) == "" {
		return nil, fmt.Errorf("report date is required")
	}

	if utils.IsBeforeMinDate(input.ReportDate) {
		return nil, fmt.Errorf("date cannot be prior to July 2026 (%s)", utils.MinDateStr)
	}

	if utils.IsFutureDate(input.ReportDate) {
		return nil, fmt.Errorf("cannot submit EOD report for future date %s", input.ReportDate)
	}

	totalSale, err := parseAmount(input.TotalSale)
	if err != nil {
		return nil, fmt.Errorf("invalid Total Sale amount")
	}

	creditSale, err := parseAmount(input.CreditSale)
	if err != nil {
		return nil, fmt.Errorf("invalid Credit Card Sale amount")
	}

	bankTransfer, err := parseAmount(input.BankTransfer)
	if err != nil {
		return nil, fmt.Errorf("invalid Bank Transfer amount")
	}

	counterCash, err := parseAmount(input.CounterCash)
	if err != nil {
		return nil, fmt.Errorf("invalid Counter Cash amount")
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
			if err == nil && amt > 0 {
				posAmt := math.Round(math.Abs(amt))
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
		return nil, fmt.Errorf("a report for date %s already exists. Contact a Manager to edit.", input.ReportDate)
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
		report.CreatedAt = existing.CreatedAt
		err = s.repo.Update(ctx, report)
	} else {
		err = s.repo.Create(ctx, report)
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

func (s *ReportService) DeleteReport(ctx context.Context, idStr string) error {
	return s.repo.Delete(ctx, idStr)
}

func parseAmount(val string) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, nil
	}
	// Remove commas or currency formatting if any
	clean := strings.ReplaceAll(val, ",", "")
	return strconv.ParseFloat(clean, 64)
}
