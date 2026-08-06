package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/repository"
	"wedrink/internal/services"
	"wedrink/internal/utils"
)

type ReportHandler struct {
	service      *services.ReportService
	authService  *services.AuthService
	notifService *services.NotificationService
	renderer     *render.Renderer
}

func NewReportHandler(service *services.ReportService, authService *services.AuthService, notifService *services.NotificationService, renderer *render.Renderer) *ReportHandler {
	return &ReportHandler{
		service:      service,
		authService:  authService,
		notifService: notifService,
		renderer:     renderer,
	}
}

func (h *ReportHandler) RenderSubmitForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	todayStr := utils.PKTodayStr()
	yesterdayStr := utils.PKYesterdayStr()
	if utils.IsBeforeMinDate(yesterdayStr) {
		yesterdayStr = todayStr
	}

	existing, _ := h.service.GetReportByDate(r.Context(), todayStr)

	data := map[string]any{
		"Title":          "Submit End of Day Report",
		"User":           user,
		"Today":          todayStr,
		"Yesterday":      yesterdayStr,
		"ExistingReport": existing,
		"ActiveTab":      "submit",
		"IsEditMode":     false,
		"TotalExpenses":  0.0,
		"ExpectedCash":   0.0,
		"CounterCash":    0.0,
		"Difference":     0.0,
	}

	renderPage(w, h.renderer, "submit.html", data)
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

	renderPage(w, h.renderer, "submit.html", data)
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

	// Save or update notification if remarks are present (completely safe, error-swallowed)
	if h.notifService != nil && report != nil {
		h.notifService.SaveNotification(r.Context(), report.ID.Hex(), report.ReportDate, report.SubmittedBy, report.Notes)
	}

	if isHTMX(r) {
		w.Header().Set("HX-Trigger", "reportSaved")
		msg := fmt.Sprintf("Report for %s successfully saved!", report.ReportDate)
		if input.AllowOverwrite {
			msg = fmt.Sprintf("Report for %s updated successfully by Super Admin!", report.ReportDate)
		}
		w.WriteHeader(http.StatusOK)
		data := map[string]any{
			"Report":  report,
			"Message": msg,
		}
		renderPartial(w, h.renderer, "alert_success.html", data)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?date=%s", report.ReportDate), http.StatusSeeOther)
}

func (h *ReportHandler) RenderReportsList(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	ctx := r.Context()

	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	sortBy := r.URL.Query().Get("sortBy")
	if sortBy == "" {
		sortBy = "date"
	}
	sortOrder := r.URL.Query().Get("sortOrder")
	if sortOrder == "" {
		sortOrder = "desc"
	}
	cursor := r.URL.Query().Get("cursor")
	isAppend := r.URL.Query().Get("append") == "true"

	limit := 100
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	params := repository.ReportQueryParams{
		StartDate: startDate,
		EndDate:   endDate,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Cursor:    cursor,
		Limit:     limit,
	}

	reports, err := h.service.GetReportsWithParams(ctx, params)
	if err != nil {
		reports = []models.EODReport{}
	}

	var nextCursor string
	if len(reports) == limit {
		last := reports[len(reports)-1]
		if sortBy == "date" {
			nextCursor = last.ReportDate
		} else {
			var val float64
			switch sortBy {
			case "total_sale":
				val = last.TotalSale
			case "credit_sale":
				val = last.CreditSale
			case "bank_transfer":
				val = last.BankTransfer
			case "other_payments", "expenses":
				val = last.OtherPayments
			case "difference":
				val = last.Difference
			}
			nextCursor = fmt.Sprintf("%.0f|%s", val, last.ReportDate)
		}
	}

	// Trigger next batch fetch when user reaches row (limit - 25), e.g. row 75 for 100 items
	triggerIdx := limit - 25
	if triggerIdx <= 0 {
		triggerIdx = limit / 2
	}

	data := map[string]any{
		"Title":      "Historical EOD Reports",
		"User":       user,
		"Reports":    reports,
		"StartDate":  startDate,
		"EndDate":    endDate,
		"SortBy":     sortBy,
		"SortOrder":  sortOrder,
		"NextCursor": nextCursor,
		"TriggerIdx": triggerIdx,
		"ActiveTab":  "reports",
		"Success":    r.URL.Query().Get("success"),
		"Error":      r.URL.Query().Get("error"),
	}

	if isHTMX(r) {
		if isAppend {
			renderPartial(w, h.renderer, "report_rows.html", data)
			return
		}
		if r.URL.Query().Get("partial") == "true" {
			renderPartial(w, h.renderer, "report_table.html", data)
			return
		}
	}

	renderPage(w, h.renderer, "reports.html", data)
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

	renderPartial(w, h.renderer, "report_modal.html", data)
}

// RenderDeleteConfirmModal (GET /reports/delete-confirm) — renders the password modal partial
func (h *ReportHandler) RenderDeleteConfirmModal(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := r.URL.Query().Get("id")
	date := r.URL.Query().Get("date")
	if date == "" {
		date = id
	}

	data := map[string]any{
		"ReportID":   id,
		"ReportDate": date,
	}
	renderPartial(w, h.renderer, "delete_confirm_modal.html", data)
}

func (h *ReportHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden: Super Admin role required to delete reports.", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id := r.FormValue("id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/reports/delete/")
	}

	// Password verification — required before any deletion
	adminPassword := strings.TrimSpace(r.FormValue("adminPassword"))
	if adminPassword == "" {
		errMsg := "Password is required to delete a report."
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)))
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)), http.StatusSeeOther)
		return
	}

	_, authErr := h.authService.Authenticate(r.Context(), user.Username, adminPassword)
	if authErr != nil {
		errMsg := "Incorrect password. Deletion cancelled."
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)))
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)), http.StatusSeeOther)
		return
	}

	if delErr := h.service.DeleteReport(r.Context(), id, user.Username); delErr != nil {
		slog.Error("HandleDelete: failed to delete report from DB", "id", id, "error", delErr)
		errMsg := "Failed to delete report from database."
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)))
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/reports?error=%s", url.QueryEscape(errMsg)), http.StatusSeeOther)
		return
	}

	slog.Info("HandleDelete: report deleted from DB", "id", id)

	successMsg := "Report deleted successfully."
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/reports?success=%s", url.QueryEscape(successMsg)))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/reports?success=%s", url.QueryEscape(successMsg)), http.StatusSeeOther)
}

func (h *ReportHandler) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	if isHTMX(r) {
		renderHTMXError(w, h.renderer, msg)
		return
	}
	http.Error(w, msg, http.StatusBadRequest)
}


func (h *ReportHandler) GetSubmittedDates(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	if month == "" {
		month = utils.PKNow().Format("2006-01")
	}

	dates, err := h.service.GetSubmittedDatesForMonth(r.Context(), month)
	if err != nil {
		dates = []string{}
	}

	roleStr := ""
	if user != nil {
		roleStr = string(user.Role)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"month":    month,
		"dates":    dates,
		"userRole": roleStr,
		"canEdit":  user != nil && user.CanEditReports(),
	})
}

func (h *ReportHandler) CheckReportDate(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	dateStr := strings.TrimSpace(r.URL.Query().Get("reportDate"))
	if dateStr == "" {
		dateStr = strings.TrimSpace(r.URL.Query().Get("date"))
	}
	if dateStr == "" {
		dateStr = utils.PKTodayStr()
	}

	isBeforeMin := utils.IsBeforeMinDate(dateStr)
	isFuture := utils.IsFutureDate(dateStr)

	var report *models.EODReport
	var reportJSON string
	if !isBeforeMin && !isFuture {
		report, _ = h.service.GetReportByDate(r.Context(), dateStr)
		if report != nil {
			b, err := json.Marshal(report)
			if err == nil {
				reportJSON = string(b)
			}
		}
	}

	isEditMode := r.FormValue("isEditMode") == "true" || r.URL.Query().Get("isEditMode") == "true"

	data := map[string]any{
		"Date":           dateStr,
		"MinDate":        utils.MinDateStr,
		"IsBeforeMin":    isBeforeMin,
		"IsFuture":       isFuture,
		"ExistingReport": report,
		"ReportJSON":     reportJSON,
		"User":           user,
		"CanEdit":        user != nil && user.CanEditReports(),
		"IsEditMode":     isEditMode,
	}

	renderPartial(w, h.renderer, "date_status.html", data)
}
