package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/models"
)

type SQLiteReportRepository struct {
	db *sql.DB
}

func NewSQLiteReportRepository(db *sql.DB) *SQLiteReportRepository {
	return &SQLiteReportRepository{db: db}
}

const reportSelectBase = `
	SELECT 
		r.id, r.report_id, r.report_date, r.total_sale, r.credit_sale, r.bank_transfer,
		r.other_payments, r.expected_cash, r.counter_cash, r.difference,
		r.submitted_by, r.submitted_by_role, r.notes,
		r.is_deleted, r.deleted_at, r.deleted_by, r.created_at, r.updated_at,
		COALESCE(
			json_group_array(
				json_object('id', e.id, 'description', e.description, 'amount', e.amount)
			) FILTER (WHERE e.id IS NOT NULL),
			'[]'
		) AS expenses_json
	FROM eod_reports r
	LEFT JOIN (
		SELECT id, report_id, description, amount, sort_order 
		FROM report_expenses 
		ORDER BY sort_order ASC
	) e ON r.id = e.report_id
`

func (r *SQLiteReportRepository) Create(ctx context.Context, report *models.EODReport) error {
	if err := validateNonNegativeReport(report); err != nil {
		return err
	}

	now := time.Now()
	report.CreatedAt = now
	report.UpdatedAt = now
	if report.ID.IsZero() {
		report.ID = bson.NewObjectID()
	}

	// Financial rounding invariant before database persistence
	report.TotalSale = math.Round(report.TotalSale)
	report.CreditSale = math.Round(report.CreditSale)
	report.BankTransfer = math.Round(report.BankTransfer)
	report.OtherPayments = math.Round(report.OtherPayments)
	report.ExpectedCash = math.Round(report.ExpectedCash)
	report.CounterCash = math.Round(report.CounterCash)
	report.Difference = math.Round(report.Difference)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queryReport := `
		INSERT INTO eod_reports (
			id, report_id, report_date, total_sale, credit_sale, bank_transfer,
			other_payments, expected_cash, counter_cash, difference,
			submitted_by, submitted_by_role, notes, is_deleted, deleted_at,
			deleted_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, NULL, ?, ?);
	`
	_, err = tx.ExecContext(ctx, queryReport,
		report.ID.Hex(),
		report.ReportID,
		report.ReportDate,
		report.TotalSale,
		report.CreditSale,
		report.BankTransfer,
		report.OtherPayments,
		report.ExpectedCash,
		report.CounterCash,
		report.Difference,
		report.SubmittedBy,
		report.SubmittedByRole,
		report.Notes,
		report.CreatedAt.Format(time.RFC3339),
		report.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("a report for date %s already exists", report.ReportDate)
		}
		return fmt.Errorf("failed to save report: %w", err)
	}

	// Insert itemized expenses into relational table (single source of truth)
	if len(report.Expenses) > 0 {
		stmtExp, err := tx.PrepareContext(ctx, `
			INSERT INTO report_expenses (id, report_id, description, amount, sort_order)
			VALUES (?, ?, ?, ?, ?);
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare expense insert: %w", err)
		}
		defer stmtExp.Close()

		for idx, exp := range report.Expenses {
			expID := exp.ID
			if expID == "" {
				expID = fmt.Sprintf("exp_%s_%d", report.ID.Hex(), idx)
			}
			_, err = stmtExp.ExecContext(ctx, expID, report.ID.Hex(), exp.Description, math.Round(exp.Amount), idx)
			if err != nil {
				return fmt.Errorf("failed to insert expense item: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit report transaction: %w", err)
	}
	return nil
}

func (r *SQLiteReportRepository) Update(ctx context.Context, report *models.EODReport) error {
	if err := validateNonNegativeReport(report); err != nil {
		return err
	}

	report.UpdatedAt = time.Now()
	report.TotalSale = math.Round(report.TotalSale)
	report.CreditSale = math.Round(report.CreditSale)
	report.BankTransfer = math.Round(report.BankTransfer)
	report.OtherPayments = math.Round(report.OtherPayments)
	report.ExpectedCash = math.Round(report.ExpectedCash)
	report.CounterCash = math.Round(report.CounterCash)
	report.Difference = math.Round(report.Difference)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queryReport := `
		UPDATE eod_reports
		SET report_id = ?, report_date = ?, total_sale = ?, credit_sale = ?,
		    bank_transfer = ?, other_payments = ?, expected_cash = ?,
		    counter_cash = ?, difference = ?,
		    submitted_by = ?, submitted_by_role = ?, notes = ?, updated_at = ?
		WHERE (id = ? OR report_id = ? OR report_date = ?) AND is_deleted = 0;
	`
	res, err := tx.ExecContext(ctx, queryReport,
		report.ReportID,
		report.ReportDate,
		report.TotalSale,
		report.CreditSale,
		report.BankTransfer,
		report.OtherPayments,
		report.ExpectedCash,
		report.CounterCash,
		report.Difference,
		report.SubmittedBy,
		report.SubmittedByRole,
		report.Notes,
		report.UpdatedAt.Format(time.RFC3339),
		report.ID.Hex(),
		report.ReportID,
		report.ReportDate,
	)
	if err != nil {
		return fmt.Errorf("failed to update report: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("report not found")
	}

	// Replace itemized expenses in relational table
	_, err = tx.ExecContext(ctx, "DELETE FROM report_expenses WHERE report_id = ?;", report.ID.Hex())
	if err != nil {
		return fmt.Errorf("failed to clear old expenses: %w", err)
	}

	if len(report.Expenses) > 0 {
		stmtExp, err := tx.PrepareContext(ctx, `
			INSERT INTO report_expenses (id, report_id, description, amount, sort_order)
			VALUES (?, ?, ?, ?, ?);
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare expense insert: %w", err)
		}
		defer stmtExp.Close()

		for idx, exp := range report.Expenses {
			expID := exp.ID
			if expID == "" {
				expID = fmt.Sprintf("exp_%s_%d", report.ID.Hex(), idx)
			}
			_, err = stmtExp.ExecContext(ctx, expID, report.ID.Hex(), exp.Description, math.Round(exp.Amount), idx)
			if err != nil {
				return fmt.Errorf("failed to insert expense item: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit report update: %w", err)
	}
	return nil
}

func (r *SQLiteReportRepository) UpsertByDate(ctx context.Context, report *models.EODReport) error {
	if err := validateNonNegativeReport(report); err != nil {
		return err
	}

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

func (r *SQLiteReportRepository) FindByID(ctx context.Context, idStr string) (*models.EODReport, error) {
	cleanID := strings.TrimSpace(idStr)
	if cleanID == "" {
		return nil, nil
	}

	query := fmt.Sprintf(`
		%s
		WHERE (r.id = ? OR r.report_id = ? OR r.report_date = ?) AND r.is_deleted = 0
		GROUP BY r.id
		LIMIT 1;
	`, reportSelectBase)

	row := r.db.QueryRowContext(ctx, query, cleanID, cleanID, cleanID)

	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch report by id: %w", err)
	}
	return report, nil
}

func (r *SQLiteReportRepository) FindByDate(ctx context.Context, dateStr string) (*models.EODReport, error) {
	cleanDate := strings.TrimSpace(dateStr)
	if cleanDate == "" {
		return nil, nil
	}

	query := fmt.Sprintf(`
		%s
		WHERE r.report_date = ? AND r.is_deleted = 0
		GROUP BY r.id
		LIMIT 1;
	`, reportSelectBase)

	row := r.db.QueryRowContext(ctx, query, cleanDate)

	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch report by date: %w", err)
	}
	return report, nil
}

func (r *SQLiteReportRepository) FindByMonth(ctx context.Context, yearMonth string) ([]models.EODReport, error) {
	query := fmt.Sprintf(`
		%s
		WHERE r.report_date LIKE ? AND r.is_deleted = 0
		GROUP BY r.id
		ORDER BY r.report_date DESC;
	`, reportSelectBase)

	rows, err := r.db.QueryContext(ctx, query, yearMonth+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch monthly reports: %w", err)
	}
	defer rows.Close()

	reports := make([]models.EODReport, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monthly report: %w", err)
		}
		reports = append(reports, *rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("monthly reports iteration error: %w", err)
	}
	return reports, nil
}

func (r *SQLiteReportRepository) GetSubmittedDatesByMonth(ctx context.Context, yearMonth string) ([]string, error) {
	query := `
		SELECT report_date
		FROM eod_reports
		WHERE report_date LIKE ? AND is_deleted = 0
		ORDER BY report_date ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, yearMonth+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch submitted dates: %w", err)
	}
	defer rows.Close()

	dates := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("failed to scan date: %w", err)
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dates iteration error: %w", err)
	}
	return dates, nil
}

func (r *SQLiteReportRepository) FindByDateRange(ctx context.Context, startDate, endDate string) ([]models.EODReport, error) {
	var whereClauses []string
	var args []any

	whereClauses = append(whereClauses, "r.is_deleted = 0")

	if startDate != "" && endDate != "" {
		whereClauses = append(whereClauses, "r.report_date >= ? AND r.report_date <= ?")
		args = append(args, startDate, endDate)
	} else if startDate != "" {
		whereClauses = append(whereClauses, "r.report_date >= ?")
		args = append(args, startDate)
	} else if endDate != "" {
		whereClauses = append(whereClauses, "r.report_date <= ?")
		args = append(args, endDate)
	}

	query := fmt.Sprintf(`
		%s
		WHERE %s
		GROUP BY r.id
		ORDER BY r.report_date DESC;
	`, reportSelectBase, strings.Join(whereClauses, " AND "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch date range reports: %w", err)
	}
	defer rows.Close()

	reports := make([]models.EODReport, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan range report: %w", err)
		}
		reports = append(reports, *rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("range reports iteration error: %w", err)
	}
	return reports, nil
}

func (r *SQLiteReportRepository) FindWithParams(ctx context.Context, params ReportQueryParams) ([]models.EODReport, error) {
	var whereClauses []string
	var args []any

	whereClauses = append(whereClauses, "r.is_deleted = 0")

	if params.StartDate != "" && params.EndDate != "" {
		whereClauses = append(whereClauses, "r.report_date >= ? AND r.report_date <= ?")
		args = append(args, params.StartDate, params.EndDate)
	} else if params.StartDate != "" {
		whereClauses = append(whereClauses, "r.report_date >= ?")
		args = append(args, params.StartDate)
	} else if params.EndDate != "" {
		whereClauses = append(whereClauses, "r.report_date <= ?")
		args = append(args, params.EndDate)
	}

	var sortKey string
	switch params.SortBy {
	case "total_sale":
		sortKey = "r.total_sale"
	case "credit_sale":
		sortKey = "r.credit_sale"
	case "bank_transfer":
		sortKey = "r.bank_transfer"
	case "other_payments", "expenses":
		sortKey = "r.other_payments"
	case "difference":
		sortKey = "r.difference"
	default:
		sortKey = "r.report_date"
	}

	sortDir := "DESC"
	if strings.ToLower(params.SortOrder) == "asc" {
		sortDir = "ASC"
	}

	if params.Cursor != "" {
		if sortKey == "r.report_date" {
			if sortDir == "DESC" {
				whereClauses = append(whereClauses, "r.report_date < ?")
			} else {
				whereClauses = append(whereClauses, "r.report_date > ?")
			}
			args = append(args, params.Cursor)
		} else {
			parts := strings.Split(params.Cursor, "|")
			if len(parts) == 2 {
				valStr, cursorDate := parts[0], parts[1]
				val, _ := strconv.ParseFloat(valStr, 64)
				if sortDir == "DESC" {
					whereClauses = append(whereClauses, fmt.Sprintf("(%s < ? OR (%s = ? AND r.report_date < ?))", sortKey, sortKey))
					args = append(args, val, val, cursorDate)
				} else {
					whereClauses = append(whereClauses, fmt.Sprintf("(%s > ? OR (%s = ? AND r.report_date > ?))", sortKey, sortKey))
					args = append(args, val, val, cursorDate)
				}
			} else {
				if sortDir == "DESC" {
					whereClauses = append(whereClauses, "r.report_date < ?")
				} else {
					whereClauses = append(whereClauses, "r.report_date > ?")
				}
				args = append(args, params.Cursor)
			}
		}
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	orderClause := fmt.Sprintf("%s %s", sortKey, sortDir)
	if sortKey != "r.report_date" {
		orderClause += fmt.Sprintf(", r.report_date %s", sortDir)
	}

	query := fmt.Sprintf(`
		%s
		WHERE %s
		GROUP BY r.id
		ORDER BY %s
		LIMIT ?;
	`, reportSelectBase, strings.Join(whereClauses, " AND "), orderClause)

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reports with params: %w", err)
	}
	defer rows.Close()

	reports := make([]models.EODReport, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan report with params: %w", err)
		}
		reports = append(reports, *rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reports iteration error: %w", err)
	}
	return reports, nil
}

func (r *SQLiteReportRepository) FindAll(ctx context.Context, limit int) ([]models.EODReport, error) {
	query := fmt.Sprintf(`
		%s
		WHERE r.is_deleted = 0
		GROUP BY r.id
		ORDER BY r.report_date DESC
	`, reportSelectBase)

	var rows *sql.Rows
	var err error
	if limit > 0 {
		query += " LIMIT ?;"
		rows, err = r.db.QueryContext(ctx, query, limit)
	} else {
		query += ";"
		rows, err = r.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch all reports: %w", err)
	}
	defer rows.Close()

	reports := make([]models.EODReport, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan report: %w", err)
		}
		reports = append(reports, *rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("all reports iteration error: %w", err)
	}
	return reports, nil
}

func (r *SQLiteReportRepository) Delete(ctx context.Context, idStr string, actor string) error {
	cleanID := strings.TrimSpace(idStr)
	if cleanID == "" {
		return fmt.Errorf("report id required")
	}

	now := time.Now().Format(time.RFC3339)
	query := `
		UPDATE eod_reports
		SET is_deleted = 1, deleted_at = ?, deleted_by = ?, updated_at = ?
		WHERE (id = ? OR report_id = ? OR report_date = ?) AND is_deleted = 0;
	`
	res, err := r.db.ExecContext(ctx, query, now, actor, now, cleanID, cleanID, cleanID)
	if err != nil {
		return fmt.Errorf("failed to delete report: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("report not found")
	}
	return nil
}

func (r *SQLiteReportRepository) CalculateMonthlySummary(ctx context.Context, yearMonth string) (*models.MonthlySummary, error) {
	query := `
		SELECT 
			COUNT(*),
			COALESCE(SUM(total_sale), 0),
			COALESCE(SUM(credit_sale), 0),
			COALESCE(SUM(bank_transfer), 0),
			COALESCE(SUM(other_payments), 0),
			COALESCE(SUM(expected_cash), 0),
			COALESCE(SUM(counter_cash), 0),
			COALESCE(SUM(difference), 0)
		FROM eod_reports
		WHERE report_date LIKE ? AND is_deleted = 0;
	`
	row := r.db.QueryRowContext(ctx, query, yearMonth+"%")

	var (
		count           int
		totalSale       float64
		totalCredit     float64
		totalBank       float64
		totalExpenses   float64
		expectedCash    float64
		counterCash     float64
		totalDifference float64
	)

	err := row.Scan(
		&count,
		&totalSale,
		&totalCredit,
		&totalBank,
		&totalExpenses,
		&expectedCash,
		&counterCash,
		&totalDifference,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate monthly summary: %w", err)
	}

	return &models.MonthlySummary{
		YearMonth:       yearMonth,
		TotalSale:       math.Round(totalSale),
		TotalCredit:     math.Round(totalCredit),
		TotalBank:       math.Round(totalBank),
		TotalExpenses:   math.Round(totalExpenses),
		ExpectedCash:    math.Round(expectedCash),
		CounterCash:     math.Round(counterCash),
		TotalDifference: math.Round(totalDifference),
		ReportCount:     count,
	}, nil
}

func scanReport(scanner rowScanner) (*models.EODReport, error) {
	var (
		idStr           string
		reportID        string
		reportDate      string
		totalSale       float64
		creditSale      float64
		bankTransfer    float64
		otherPayments   float64
		expectedCash    float64
		counterCash     float64
		difference      float64
		submittedBy     string
		submittedByRole string
		notes           string
		isDeletedInt    int
		deletedAtStr    sql.NullString
		deletedBy       sql.NullString
		createdAtStr    string
		updatedAtStr    string
		expensesJSON    string
	)

	err := scanner.Scan(
		&idStr,
		&reportID,
		&reportDate,
		&totalSale,
		&creditSale,
		&bankTransfer,
		&otherPayments,
		&expectedCash,
		&counterCash,
		&difference,
		&submittedBy,
		&submittedByRole,
		&notes,
		&isDeletedInt,
		&deletedAtStr,
		&deletedBy,
		&createdAtStr,
		&updatedAtStr,
		&expensesJSON,
	)
	if err != nil {
		return nil, err
	}

	objID, _ := bson.ObjectIDFromHex(idStr)
	createdAt, _ := time.Parse(time.RFC3339, createdAtStr)
	updatedAt, _ := time.Parse(time.RFC3339, updatedAtStr)

	var deletedAt *time.Time
	if deletedAtStr.Valid {
		if t, err := time.Parse(time.RFC3339, deletedAtStr.String); err == nil {
			deletedAt = &t
		}
	}

	expenses := make([]models.ExpenseItem, 0)
	if strings.TrimSpace(expensesJSON) != "" && expensesJSON != "[]" {
		_ = json.Unmarshal([]byte(expensesJSON), &expenses)
	}

	return &models.EODReport{
		ID:              objID,
		ReportID:        reportID,
		ReportDate:      reportDate,
		TotalSale:       totalSale,
		CreditSale:      creditSale,
		BankTransfer:    bankTransfer,
		OtherPayments:   otherPayments,
		ExpectedCash:    expectedCash,
		CounterCash:     counterCash,
		Difference:      difference,
		Expenses:        expenses,
		SubmittedBy:     submittedBy,
		SubmittedByRole: submittedByRole,
		Notes:           notes,
		IsDeleted:       isDeletedInt == 1,
		DeletedAt:       deletedAt,
		DeletedBy:       deletedBy.String,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

func (r *SQLiteReportRepository) GetOldestReportDate(ctx context.Context) (string, error) {
	var dateStr string
	err := r.db.QueryRowContext(ctx, "SELECT report_date FROM eod_reports WHERE is_deleted = 0 ORDER BY report_date ASC LIMIT 1").Scan(&dateStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return dateStr, nil
}
