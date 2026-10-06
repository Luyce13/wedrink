package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/repository"
	"wedrink/internal/services"
	"wedrink/internal/utils"
)

type AuditHandler struct {
	service  *services.AuditService
	renderer *render.Renderer
}

func NewAuditHandler(service *services.AuditService, renderer *render.Renderer) *AuditHandler {
	return &AuditHandler{
		service:  service,
		renderer: renderer,
	}
}

type FieldDiff struct {
	Field    string
	OldValue string
	NewValue string
	Changed  bool
}

func (h *AuditHandler) RenderAuditList(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.IsSuperAdmin() {
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		action = "reports"
	}
	actor := strings.TrimSpace(r.URL.Query().Get("actor"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	startDate := strings.TrimSpace(r.URL.Query().Get("startDate"))
	endDate := strings.TrimSpace(r.URL.Query().Get("endDate"))
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	isAppend := r.URL.Query().Get("append") == "true"
	isPartial := isHTMX(r) && r.URL.Query().Get("partial") == "true"

	limit := 25
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	params := repository.AuditQueryParams{
		Actor:      actor,
		ResourceID: resource,
		Action:     action,
		StartDate:  startDate,
		EndDate:    endDate,
		Cursor:     cursor,
		Limit:      limit,
	}

	result, err := h.service.GetAuditLogs(r.Context(), params)
	if err != nil {
		slog.Error("Failed to query audit logs", "error", err)
		result = &repository.AuditQueryResult{Logs: []models.AuditLog{}}
	}

	data := map[string]any{
		"Title":      "Audit Trail & Security Logs",
		"User":       user,
		"ActiveTab":  "audit",
		"Logs":       result.Logs,
		"NextCursor": result.NextCursor,
		"HasMore":    result.HasMore,
		"TriggerIdx": result.TriggerIdx,
		"Action":     action,
		"Actor":      actor,
		"Resource":   resource,
		"StartDate":  startDate,
		"EndDate":    endDate,
		"Limit":      limit,
	}

	if isAppend {
		renderPartial(w, h.renderer, "audit_rows.html", data)
		return
	}

	if isPartial {
		renderPartial(w, h.renderer, "audit_table.html", data)
		return
	}

	renderPage(w, h.renderer, "audit.html", data)
}

func (h *AuditHandler) RenderAuditDetailModal(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.IsSuperAdmin() {
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		renderHTMXError(w, h.renderer, "Invalid audit log ID.")
		return
	}

	log, err := h.service.GetAuditLogByID(r.Context(), id)
	if err != nil || log == nil {
		renderHTMXError(w, h.renderer, "Audit record not found.")
		return
	}

	diffs := ExtractAuditDiff(log)
	oldJSON := toPrettyJSON(log.OldState)
	newJSON := toPrettyJSON(log.NewState)

	data := map[string]any{
		"Log":     log,
		"Diffs":   diffs,
		"OldJSON": oldJSON,
		"NewJSON": newJSON,
		"HasDiff": len(diffs) > 0,
	}

	renderPartial(w, h.renderer, "audit_modal.html", data)
}

func toCleanMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func sanitizeState(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	res := make(map[string]any)
	for k, v := range m {
		kLower := strings.ToLower(k)
		if strings.Contains(kLower, "password") || strings.Contains(kLower, "hash") || strings.Contains(kLower, "secret") {
			if v != nil && v != "" {
				res[k] = "[REDACTED]"
			} else {
				res[k] = ""
			}
		} else {
			res[k] = v
		}
	}
	return res
}

func toPrettyJSON(v any) string {
	if v == nil {
		return ""
	}
	m := toCleanMap(v)
	if m == nil {
		return ""
	}
	sanitized := sanitizeState(m)
	b, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func ExtractAuditDiff(log *models.AuditLog) []FieldDiff {
	if log == nil {
		return nil
	}

	oldMap := sanitizeState(toCleanMap(log.OldState))
	newMap := sanitizeState(toCleanMap(log.NewState))

	if oldMap == nil && newMap == nil {
		return nil
	}

	fieldOrder := []string{
		"report_date", "total_sale", "credit_sale", "bank_transfer",
		"other_payments", "counter_cash", "expected_cash", "difference",
		"expenses", "notes", "username", "full_name", "role", "is_deleted",
	}

	fieldLabels := map[string]string{
		"report_date":    "Report Date",
		"total_sale":     "Total Sales (PKR)",
		"credit_sale":    "Card Sales (PKR)",
		"bank_transfer":  "Bank Transfer (PKR)",
		"other_payments": "Total Expenses (PKR)",
		"counter_cash":   "Counter Cash (PKR)",
		"expected_cash":  "Expected Cash (PKR)",
		"difference":     "Discrepancy (PKR)",
		"expenses":       "Itemized Expenses",
		"notes":          "Store Remarks",
		"username":       "Username",
		"full_name":      "Full Name",
		"role":           "Access Role",
		"is_deleted":     "Deleted Status",
	}

	monetaryFields := map[string]bool{
		"total_sale":     true,
		"credit_sale":    true,
		"bank_transfer":  true,
		"other_payments": true,
		"counter_cash":   true,
		"expected_cash":  true,
		"difference":     true,
	}

	ignoredFields := map[string]bool{
		"_id": true, "id": true, "created_at": true, "updated_at": true,
		"report_id": true, "submitted_by_id": true,
	}

	allKeys := make(map[string]bool)
	for k := range oldMap {
		if !ignoredFields[strings.ToLower(k)] {
			allKeys[k] = true
		}
	}
	for k := range newMap {
		if !ignoredFields[strings.ToLower(k)] {
			allKeys[k] = true
		}
	}

	formatVal := func(k string, val any) string {
		if val == nil {
			return "-"
		}
		if monetaryFields[strings.ToLower(k)] {
			switch v := val.(type) {
			case float64:
				return utils.FormatNumber(v)
			case int:
				return utils.FormatNumber(float64(v))
			case int64:
				return utils.FormatNumber(float64(v))
			}
		}
		if sliceVal, ok := val.([]any); ok {
			if len(sliceVal) == 0 {
				return "None"
			}
			parts := make([]string, 0, len(sliceVal))
			for _, item := range sliceVal {
				if itemMap, isMap := item.(map[string]any); isMap {
					desc := fmt.Sprint(itemMap["description"])
					amt := 0.0
					if a, ok := itemMap["amount"].(float64); ok {
						amt = a
					}
					parts = append(parts, fmt.Sprintf("%s: %s", desc, utils.FormatNumber(amt)))
				} else {
					parts = append(parts, fmt.Sprint(item))
				}
			}
			return strings.Join(parts, "; ")
		}
		s := fmt.Sprint(val)
		if strings.TrimSpace(s) == "" {
			return "-"
		}
		return s
	}

	var orderedKeys []string
	for _, k := range fieldOrder {
		if allKeys[k] {
			orderedKeys = append(orderedKeys, k)
			delete(allKeys, k)
		}
	}
	var restKeys []string
	for k := range allKeys {
		restKeys = append(restKeys, k)
	}
	sort.Strings(restKeys)
	orderedKeys = append(orderedKeys, restKeys...)

	diffs := make([]FieldDiff, 0, len(orderedKeys))
	for _, k := range orderedKeys {
		oldVal := formatVal(k, oldMap[k])
		newVal := formatVal(k, newMap[k])

		label := fieldLabels[strings.ToLower(k)]
		if label == "" {
			label = strings.Title(strings.ReplaceAll(k, "_", " "))
		}

		changed := oldVal != newVal
		diffs = append(diffs, FieldDiff{
			Field:    label,
			OldValue: oldVal,
			NewValue: newVal,
			Changed:  changed,
		})
	}

	return diffs
}
