package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/services"
)

type ExportHandler struct {
	reportService *services.ReportService
}

func NewExportHandler(reportService *services.ReportService) *ExportHandler {
	return &ExportHandler{reportService: reportService}
}

func (h *ExportHandler) getReportsFromRequest(r *http.Request) ([]models.EODReport, error) {
	ctx := r.Context()

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	selectedIDs := r.URL.Query().Get("ids")

	if selectedIDs != "" {
		var reports []models.EODReport
		idList := strings.Split(selectedIDs, ",")
		for _, idStr := range idList {
			idStr = strings.TrimSpace(idStr)
			if idStr != "" {
				rep, errFetch := h.reportService.GetReportByID(ctx, idStr)
				if errFetch == nil && rep != nil {
					reports = append(reports, *rep)
				}
			}
		}
		return reports, nil
	}

	params := repository.ReportQueryParams{
		StartDate: startDate,
		EndDate:   endDate,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     10000,
	}

	return h.reportService.GetReportsWithParams(ctx, params)
}

func (h *ExportHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	reports, err := h.getReportsFromRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Export database query error: %v", err), http.StatusInternalServerError)
		return
	}

	exportMode := r.URL.Query().Get("export") // "summary", "expenses", "all_combined"
	if exportMode == "" {
		exportMode = "summary"
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	switch exportMode {
	case "expenses":
		header := []string{"Report ID", "Report Date", "Expense Description", "Amount", "Submitted By", "Submitted Role"}
		_ = writer.Write(header)

		for _, rep := range reports {
			for _, exp := range rep.Expenses {
				row := []string{
					rep.ReportID,
					rep.ReportDate,
					exp.Description,
					strconv.FormatFloat(exp.Amount, 'f', 0, 64),
					rep.SubmittedBy,
					rep.SubmittedByRole,
				}
				_ = writer.Write(row)
			}
		}

	case "all_combined":
		header := []string{
			"Record Type", "Report Date", "Total Sale", "Credit Card Sale", "Bank Transfer",
			"Other Payments Total", "Expected Cash", "Counter Cash", "Difference",
			"Expense Description", "Expense Amount", "Submitted By", "Notes", "Report ID",
		}
		_ = writer.Write(header)

		for _, rep := range reports {
			sumRow := []string{
				"SUMMARY",
				rep.ReportDate,
				strconv.FormatFloat(rep.TotalSale, 'f', 0, 64),
				strconv.FormatFloat(rep.CreditSale, 'f', 0, 64),
				strconv.FormatFloat(rep.BankTransfer, 'f', 0, 64),
				strconv.FormatFloat(rep.OtherPayments, 'f', 0, 64),
				strconv.FormatFloat(rep.ExpectedCash, 'f', 0, 64),
				strconv.FormatFloat(rep.CounterCash, 'f', 0, 64),
				strconv.FormatFloat(rep.Difference, 'f', 0, 64),
				"-", "-",
				rep.SubmittedBy,
				rep.Notes,
				rep.ReportID,
			}
			_ = writer.Write(sumRow)

			for _, exp := range rep.Expenses {
				expRow := []string{
					"EXPENSE_ITEM",
					rep.ReportDate,
					"-", "-", "-", "-", "-", "-", "-",
					exp.Description,
					strconv.FormatFloat(exp.Amount, 'f', 0, 64),
					rep.SubmittedBy,
					"",
					rep.ReportID,
				}
				_ = writer.Write(expRow)
			}
		}

	default: // "summary"
		header := []string{
			"Date",
			"Total Sale",
			"Credit Card Sale",
			"Bank Transfer",
			"Other Payments",
			"Expected Cash",
			"Counter Cash",
			"Difference",
			"Report ID",
			"Expenses Summary",
			"Submitted By",
			"Notes",
		}
		_ = writer.Write(header)

		for _, rep := range reports {
			expDetails := []string{}
			for _, exp := range rep.Expenses {
				expDetails = append(expDetails, fmt.Sprintf("%s: %.0f", exp.Description, exp.Amount))
			}
			expStr := strings.Join(expDetails, " | ")

			row := []string{
				rep.ReportDate,
				strconv.FormatFloat(rep.TotalSale, 'f', 0, 64),
				strconv.FormatFloat(rep.CreditSale, 'f', 0, 64),
				strconv.FormatFloat(rep.BankTransfer, 'f', 0, 64),
				strconv.FormatFloat(rep.OtherPayments, 'f', 0, 64),
				strconv.FormatFloat(rep.ExpectedCash, 'f', 0, 64),
				strconv.FormatFloat(rep.CounterCash, 'f', 0, 64),
				strconv.FormatFloat(rep.Difference, 'f', 0, 64),
				rep.ReportID,
				expStr,
				rep.SubmittedBy,
				rep.Notes,
			}
			_ = writer.Write(row)
		}
	}

	writer.Flush()

	filename := fmt.Sprintf("wedrink_%s_%s.csv", exportMode, time.Now().Format("20060102_150405"))

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	_, _ = w.Write(buf.Bytes())
}

// ExportExcel generates a single .xlsx file containing 2 tabs ("EOD Summary" and "EOD Expenses"), matching legacy Google Sheets
func (h *ExportHandler) ExportExcel(w http.ResponseWriter, r *http.Request) {
	reports, err := h.getReportsFromRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Export database query error: %v", err), http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheetSummary := "EOD Summary"
	sheetExpenses := "EOD Expenses"

	// Rename default sheet
	if err := f.SetSheetName("Sheet1", sheetSummary); err != nil {
		http.Error(w, fmt.Sprintf("Excel sheet rename error: %v", err), http.StatusInternalServerError)
		return
	}

	// Create second sheet for Expenses
	_, err = f.NewSheet(sheetExpenses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Excel sheet creation error: %v", err), http.StatusInternalServerError)
		return
	}

	// -------------------------------------------------------------
	// Tab 1: EOD Summary
	// -------------------------------------------------------------
	summaryHeaders := []interface{}{
		"Date", "Total Sale", "Credit Card Sale", "Bank Transfer", "Other Payments",
		"Expected Cash", "Counter Cash", "Difference", "Report ID", "Submitted By", "Notes",
	}

	_ = f.SetSheetRow(sheetSummary, "A1", &summaryHeaders)

	for i, rep := range reports {
		rowNum := i + 2
		rowData := []interface{}{
			rep.ReportDate,
			rep.TotalSale,
			rep.CreditSale,
			rep.BankTransfer,
			rep.OtherPayments,
			rep.ExpectedCash,
			rep.CounterCash,
			rep.Difference,
			rep.ReportID,
			rep.SubmittedBy,
			rep.Notes,
		}
		cellAddr := fmt.Sprintf("A%d", rowNum)
		_ = f.SetSheetRow(sheetSummary, cellAddr, &rowData)
	}

	// -------------------------------------------------------------
	// Tab 2: EOD Expenses
	// -------------------------------------------------------------
	expenseHeaders := []interface{}{
		"Report ID", "Date", "Description", "Amount", "Submitted By", "Submitted Role",
	}

	_ = f.SetSheetRow(sheetExpenses, "A1", &expenseHeaders)

	expRowIdx := 2
	for _, rep := range reports {
		for _, exp := range rep.Expenses {
			expRowData := []interface{}{
				rep.ReportID,
				rep.ReportDate,
				exp.Description,
				exp.Amount,
				rep.SubmittedBy,
				rep.SubmittedByRole,
			}
			cellAddr := fmt.Sprintf("A%d", expRowIdx)
			_ = f.SetSheetRow(sheetExpenses, cellAddr, &expRowData)
			expRowIdx++
		}
	}

	// Auto-fit column widths across both sheets
	autoFitColumns(f, sheetSummary)
	autoFitColumns(f, sheetExpenses)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write excel file: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("wedrink_eod_report_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	_, _ = w.Write(buf.Bytes())
}

func autoFitColumns(f *excelize.File, sheet string) {
	cols, err := f.GetCols(sheet)
	if err != nil {
		return
	}
	for i, col := range cols {
		maxLen := 0
		for _, cell := range col {
			if len(cell) > maxLen {
				maxLen = len(cell)
			}
		}
		colName, err := excelize.ColumnNumberToName(i + 1)
		if err == nil {
			width := float64(maxLen) + 4
			if width < 12 {
				width = 12
			}
			_ = f.SetColWidth(sheet, colName, colName, width)
		}
	}
}
