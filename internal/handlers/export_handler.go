package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/services"
)

type ExportHandler struct {
	reportService *services.ReportService
}

func NewExportHandler(reportService *services.ReportService) *ExportHandler {
	return &ExportHandler{reportService: reportService}
}

func (h *ExportHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	selectedIDs := r.URL.Query().Get("ids")
	exportMode := r.URL.Query().Get("export") // "summary", "expenses", "all_combined"
	if exportMode == "" {
		exportMode = "summary"
	}

	var reports []models.EODReport
	var err error

	if selectedIDs != "" {
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
	} else if startDate != "" || endDate != "" {
		reports, err = h.serviceGetReportsForRange(ctx, startDate, endDate)
	} else {
		month := r.URL.Query().Get("month")
		if r.URL.Query().Get("type") == "all" || month == "all" {
			reports, err = h.reportService.GetReportsForRange(ctx, "", "")
		} else {
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			reports, err = h.reportService.GetReportsForMonth(ctx, month)
		}
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Export database query error: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("wedrink_%s_%s.csv", exportMode, time.Now().Format("20060102_150405"))

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	switch exportMode {
	case "expenses":
		// Export itemized expenses tab matching legacy Google Sheets "EOD Expenses"
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
		// Export detailed combined sheet (Summary row followed by itemized expense lines)
		header := []string{
			"Record Type", "Report Date", "Total Sale", "Credit Card Sale", "Bank Transfer", 
			"Other Payments Total", "Expected Cash", "Counter Cash", "Difference", 
			"Expense Description", "Expense Amount", "Submitted By", "Notes", "Report ID",
		}
		_ = writer.Write(header)

		for _, rep := range reports {
			// Write summary record row
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

			// Write individual itemized expense rows for this report
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
		// Export EOD Summary matching legacy Google Sheets "EOD Summary"
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
}

func (h *ExportHandler) serviceGetReportsForRange(ctx context.Context, startDate, endDate string) ([]models.EODReport, error) {
	return h.reportService.GetReportsForRange(ctx, startDate, endDate)
}
