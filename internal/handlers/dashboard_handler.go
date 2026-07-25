package handlers

import (
	"net/http"
	"time"

	"wedrink/internal/middleware"
	"wedrink/internal/render"
	"wedrink/internal/services"
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

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	targetDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		targetDate = time.Now()
		dateStr = targetDate.Format("2006-01-02")
	}

	prevDate := targetDate.AddDate(0, 0, -1).Format("2006-01-02")
	nextDate := targetDate.AddDate(0, 0, 1).Format("2006-01-02")
	todayStr := time.Now().Format("2006-01-02")

	report, _ := h.service.GetReportByDate(ctx, dateStr)

	data := map[string]any{
		"Title":      "Dashboard - Wedrink EOD",
		"User":       user,
		"ReportDate": dateStr,
		"PrevDate":   prevDate,
		"NextDate":   nextDate,
		"TodayStr":   todayStr,
		"Report":     report,
		"ActiveTab":  "dashboard",
	}

	if r.Header.Get("HX-Request") == "true" && r.URL.Query().Get("partial") == "true" {
		_ = h.renderer.RenderPartial(w, "dashboard_content.html", data)
		return
	}

	_ = h.renderer.RenderPage(w, "dashboard.html", data)
}
