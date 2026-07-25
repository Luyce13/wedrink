package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/services"
)

type ReportHandler struct {
	service  *services.ReportService
	renderer *render.Renderer
}

func NewReportHandler(service *services.ReportService, renderer *render.Renderer) *ReportHandler {
	return &ReportHandler{
		service:  service,
		renderer: renderer,
	}
}

func (h *ReportHandler) RenderSubmitForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	todayStr := time.Now().Format("2006-01-02")

	existing, _ := h.service.GetReportByDate(r.Context(), todayStr)

	data := map[string]any{
		"Title":          "Submit End of Day Report",
		"User":           user,
		"Today":          todayStr,
		"ExistingReport": existing,
		"ActiveTab":      "submit",
		"IsEditMode":     false,
		"TotalExpenses":  0.0,
		"ExpectedCash":   0.0,
		"CounterCash":    0.0,
		"Difference":     0.0,
	}

	_ = h.renderer.RenderPage(w, "submit.html", data)
}

func (h *ReportHandler) RenderEditForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden: Super Admin access required to edit reports.", http.StatusForbidden)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/reports/edit/")
	}

	report, err := h.service.GetReportByID(r.Context(), id)
	if err != nil || report == nil {
		http.Error(w, "Report not found for editing", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Title":          fmt.Sprintf("Edit Report - %s", report.ReportDate),
		"User":           user,
		"Today":          report.ReportDate,
		"Report":         report,
		"ExistingReport": report,
		"ActiveTab":      "submit",
		"IsEditMode":     true,
		"TotalSale":      report.TotalSale,
		"CreditSale":     report.CreditSale,
		"BankTransfer":   report.BankTransfer,
		"TotalExpenses":  report.OtherPayments,
		"ExpectedCash":   report.ExpectedCash,
		"CounterCash":    report.CounterCash,
		"Difference":     report.Difference,
	}

	_ = h.renderer.RenderPage(w, "submit.html", data)
}

func (h *ReportHandler) RenderAddExpenseRow(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("index")
	idx, _ := strconv.Atoi(idxStr)
	if idx <= 0 {
		idx = time.Now().Nanosecond()
	}

	data := map[string]any{
		"Index": idx,
	}
	_ = h.renderer.RenderPartial(w, "expense_row.html", data)
}

func (h *ReportHandler) RenderCalculationPreview(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	totalSale, _ := strconv.ParseFloat(r.FormValue("totalSale"), 64)
	creditSale, _ := strconv.ParseFloat(r.FormValue("creditSale"), 64)
	bankTransfer, _ := strconv.ParseFloat(r.FormValue("bankTransfer"), 64)
	counterCash, _ := strconv.ParseFloat(r.FormValue("counterCash"), 64)

	descs := r.Form["expenseDesc[]"]
	amts := r.Form["expenseAmount[]"]
	if len(descs) == 0 {
		descs = r.Form["expenseDesc"]
		amts = r.Form["expenseAmount"]
	}

	var totalExpenses float64 = 0
	for i := 0; i < len(amts); i++ {
		val, err := strconv.ParseFloat(amts[i], 64)
		if err == nil && val > 0 {
			totalExpenses += math.Abs(val)
		}
	}

	expectedCash := totalSale - totalExpenses - creditSale - bankTransfer
	difference := counterCash - expectedCash

	data := map[string]any{
		"TotalSale":     totalSale,
		"CreditSale":    creditSale,
		"BankTransfer":  bankTransfer,
		"TotalExpenses": totalExpenses,
		"ExpectedCash":  expectedCash,
		"CounterCash":   counterCash,
		"Difference":    difference,
	}

	_ = h.renderer.RenderPartial(w, "calculation_preview.html", data)
}

func (h *ReportHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, "Invalid form data submission")
		return
	}

	descs := r.Form["expenseDesc[]"]
	amts := r.Form["expenseAmount[]"]
	if len(descs) == 0 {
		descs = r.Form["expenseDesc"]
		amts = r.Form["expenseAmount"]
	}

	allowOverwrite := r.FormValue("allowOverwrite") == "true" || r.FormValue("isEditMode") == "true"
	if allowOverwrite && !user.CanEditReports() {
		h.renderError(w, r, "Only Super Admins can edit or overwrite submitted reports.")
		return
	}

	input := services.SubmitReportInput{
		ReportDate:      r.FormValue("reportDate"),
		TotalSale:       r.FormValue("totalSale"),
		CreditSale:      r.FormValue("creditSale"),
		BankTransfer:    r.FormValue("bankTransfer"),
		CounterCash:     r.FormValue("counterCash"),
		ExpenseDescs:    descs,
		ExpenseAmounts:  amts,
		Notes:           r.FormValue("notes"),
		SubmittedBy:     user.FullName,
		SubmittedByRole: string(user.Role),
		AllowOverwrite:  allowOverwrite,
	}

	report, err := h.service.ProcessAndSaveReport(r.Context(), input)
	if err != nil {
		h.renderError(w, r, err.Error())
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Reswap", "outerHTML")
		msg := fmt.Sprintf("Report for %s successfully saved!", report.ReportDate)
		if input.AllowOverwrite {
			msg = fmt.Sprintf("Report for %s updated successfully by Super Admin!", report.ReportDate)
		}
		data := map[string]any{
			"Report":  report,
			"Message": msg,
		}
		_ = h.renderer.RenderPartial(w, "alert_success.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?date=%s", report.ReportDate), http.StatusSeeOther)
}

func (h *ReportHandler) RenderReportsList(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	ctx := r.Context()

	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var reports []models.EODReport
	var err error

	if startDate != "" || endDate != "" {
		reports, err = h.service.GetReportsForRange(ctx, startDate, endDate)
	} else {
		reports, err = h.service.GetReportsForMonth(ctx, month)
	}

	if err != nil {
		reports = []models.EODReport{}
	}

	summary, _ := h.service.GetMonthlySummary(ctx, month)

	data := map[string]any{
		"Title":          "Historical EOD Reports",
		"User":           user,
		"Reports":        reports,
		"Month":          month,
		"StartDate":      startDate,
		"EndDate":        endDate,
		"MonthlySummary": summary,
		"ActiveTab":      "reports",
	}

	if r.Header.Get("HX-Request") == "true" && r.URL.Query().Get("partial") == "true" {
		_ = h.renderer.RenderPartial(w, "report_table.html", data)
		return
	}

	_ = h.renderer.RenderPage(w, "reports.html", data)
}

func (h *ReportHandler) RenderDetailModal(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/reports/")
	}

	report, err := h.service.GetReportByID(r.Context(), id)
	if err != nil || report == nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	user := middleware.GetUser(r)

	data := map[string]any{
		"Report": report,
		"User":   user,
	}

	_ = h.renderer.RenderPartial(w, "report_modal.html", data)
}

func (h *ReportHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden: Super Admin role required to delete reports.", http.StatusForbidden)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/reports/delete/")
	}

	err := h.service.DeleteReport(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "refreshReportsList")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<div class="p-3 bg-slate-900 border border-emerald-500/50 text-emerald-300 text-xs font-semibold rounded-lg">Report deleted successfully.</div>`))
		return
	}

	http.Redirect(w, r, "/reports", http.StatusSeeOther)
}

func (h *ReportHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Reswap", "outerHTML")
		w.WriteHeader(http.StatusOK) // Allow HTMX to swap alert_error.html component onto page
		data := map[string]any{
			"Error": msg,
		}
		_ = h.renderer.RenderPartial(w, "alert_error.html", data)
		return
	}
	http.Error(w, msg, http.StatusBadRequest)
}
