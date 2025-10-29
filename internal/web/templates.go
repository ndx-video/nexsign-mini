package web

import (
	"fmt"
	"html/template"
	"time"

	"nexsign.mini/nsm/internal/types"
)

func parseTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"healthStatus": func(host types.Host) types.HealthStatus {
			thresholds := types.DefaultHealthThresholds()
			return types.DetermineHealth(host.LastSeen, host.AnthiasStatus, thresholds)
		},
		"healthColor": func(status types.HealthStatus) string {
			return types.HealthColor(status)
		},
		"healthDesc": func(status types.HealthStatus) string {
			return types.HealthDescription(status)
		},
		"timeSince": func(t time.Time) string {
			if t.IsZero() {
				return "never"
			}
			duration := time.Since(t)
			if duration < time.Minute {
				return "just now"
			} else if duration < time.Hour {
				mins := int(duration.Minutes())
				return fmt.Sprintf("%dm ago", mins)
			} else if duration < 24*time.Hour {
				hours := int(duration.Hours())
				return fmt.Sprintf("%dh ago", hours)
			}
			days := int(duration.Hours() / 24)
			return fmt.Sprintf("%dd ago", days)
		},
	}

	tmpl := template.New("").Funcs(funcMap)
	return tmpl.ParseGlob("internal/web/*.html")
}
