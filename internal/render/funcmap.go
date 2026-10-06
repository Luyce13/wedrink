package render

import (
	"fmt"
	"html/template"
	"math"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/utils"
)

// DefaultFuncMap returns the canonical template functions for all HTML pages and components.
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"mathAbs": func(val float64) float64 {
			return math.Abs(val)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"mod": func(a, b int) int {
			return a % b
		},
		// fmtNum formats numeric values as comma-separated integers.
		"fmtNum": func(val any) string {
			switch v := val.(type) {
			case float64:
				return utils.FormatNumber(v)
			case int64:
				return utils.FormatNumber(float64(v))
			case int:
				return utils.FormatNumber(float64(v))
			case float32:
				return utils.FormatNumber(float64(v))
			default:
				return fmt.Sprint(val)
			}
		},
		"not": func(v any) bool {
			if v == nil {
				return true
			}
			switch val := v.(type) {
			case bool:
				return !val
			case string:
				return val == ""
			case int:
				return val == 0
			case int64:
				return val == 0
			case float64:
				return val == 0
			default:
				return false
			}
		},
		"fmtTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(utils.PKLocation).Format("02 Jan 2006, 03:04:05 PM")
		},
		"fmtTimeShort": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(utils.PKLocation).Format("02 Jan 2006, 03:04 PM")
		},
		"timeAgo": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			d := time.Since(t)
			if d < time.Minute {
				return "just now"
			}
			if d < time.Hour {
				return fmt.Sprintf("%dm ago", int(d.Minutes()))
			}
			if d < 24*time.Hour {
				return fmt.Sprintf("%dh ago", int(d.Hours()))
			}
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		},
		"actionBadgeClass": func(action string) string {
			switch action {
			case "report.submit", "user.create":
				return "bg-emerald-950/80 border-emerald-700/60 text-emerald-300"
			case "report.overwrite", "user.update":
				return "bg-amber-950/80 border-amber-700/60 text-amber-300"
			case "report.delete", "user.delete":
				return "bg-rose-950/80 border-rose-700/60 text-rose-300"
			case "auth.login_success":
				return "bg-cyan-950/80 border-cyan-700/60 text-cyan-300"
			case "auth.login_failed":
				return "bg-red-950 border-red-600 text-red-200"
			default:
				return "bg-slate-800 border-slate-700 text-slate-300"
			}
		},
		"cleanActionName": func(action string) string {
			switch action {
			case "report.submit":
				return "Report Submitted"
			case "report.overwrite":
				return "Report Overwritten"
			case "report.delete":
				return "Report Deleted"
			case "auth.login_success":
				return "Login Success"
			case "auth.login_failed":
				return "Login Failed"
			case "user.create":
				return "User Created"
			case "user.update":
				return "User Updated"
			case "user.delete":
				return "User Deleted"
			default:
				return action
			}
		},
		"hasState": func(l models.AuditLog) bool {
			return l.OldState != nil || l.NewState != nil
		},
	}
}
