package handlers

import (
	"net/http"
	"time"

	"wedrink/internal/middleware"
	"wedrink/internal/render"
	"wedrink/internal/services"
	"wedrink/internal/utils"
)

type DashboardHandler struct {
	service  *services.ReportService
	renderer *render.Renderer
}

func NewDashboardHandler(service *services.ReportService, renderer *render.Renderer) *DashboardHandler {
	return &DashboardHandler{
		service:  service,
		renderer: renderer,
	}
}

func (h *DashboardHandler) RenderDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	ctx := r.Context()

	todayStr := utils.PKTodayStr()
	oldestDate, err := h.service.GetOldestReportDate(ctx)
	if err != nil || oldestDate == "" {
		oldestDate = utils.MinDateStr
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		yesterday := utils.PKYesterdayStr()
		if yesterday >= oldestDate {
			dateStr = yesterday
		} else {
			dateStr = todayStr
		}
	}

	// Boundary guards: clamp to [oldestDate, todayStr]
	if utils.IsFutureDate(dateStr) {
		dateStr = todayStr
	} else if dateStr < oldestDate {
		dateStr = oldestDate
	}

	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		targetDate, _ = time.Parse("2006-01-02", todayStr)
		dateStr = todayStr
	}

	prevDate := targetDate.AddDate(0, 0, -1).Format("2006-01-02")
	if prevDate < oldestDate {
		prevDate = oldestDate
	}

	nextDate := targetDate.AddDate(0, 0, 1).Format("2006-01-02")
	if nextDate > todayStr {
		nextDate = todayStr
	}

	report, _ := h.service.GetReportByDate(ctx, dateStr)

	data := map[string]any{
		"Title":      "Dashboard - Wedrink EOD",
		"User":       user,
		"ReportDate": dateStr,
		"PrevDate":   prevDate,
		"NextDate":   nextDate,
		"TodayStr":   todayStr,
		"OldestDate": oldestDate,
		"Report":     report,
		"ActiveTab":  "dashboard",
	}

	if isHTMX(r) && r.URL.Query().Get("partial") == "true" {
		renderPartial(w, h.renderer, "dashboard_content.html", data)
		return
	}

	renderPage(w, h.renderer, "dashboard.html", data)
}
