package sentinel

import (
	"fmt"
	"log/slog"
	"time"
)

// Report is the JSON structure written to sentinel_report.json each cycle.
type Report struct {
	Tick          uint64        `json:"tick"`
	SimTime       string        `json:"sim_time"`
	Timestamp     string        `json:"timestamp"`
	OverallHealth Level         `json:"overall_health"`
	Checks        []CheckResult `json:"checks"`
	Alerts        []Alert       `json:"alerts,omitempty"`
}

// BuildReport creates a Report from the check results and alerts.
func BuildReport(snap *WorldSnapshot, checks []CheckResult, alerts []Alert) *Report {
	overall := LevelHealthy
	for _, c := range checks {
		if levelSeverity(c.Status) > levelSeverity(overall) {
			overall = c.Status
		}
	}

	return &Report{
		Tick:          snap.Status.Tick,
		SimTime:       snap.Status.SimTime,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		OverallHealth: overall,
		Checks:        checks,
		Alerts:        alerts,
	}
}

// LogReport outputs the report via structured logging.
// INFO for HEALTHY/WATCH, WARN for WARNING, ERROR for CRITICAL.
func LogReport(report *Report) {
	// Log each check.
	for _, c := range report.Checks {
		attrs := []any{
			"check", c.Name,
			"status", c.Status,
			"value", fmt.Sprintf("%.3f", c.Value),
			"trend", c.Trend,
			"detail", c.Detail,
		}
		switch c.Status {
		case LevelCritical:
			slog.Error("health check", attrs...)
		case LevelWarning:
			slog.Warn("health check", attrs...)
		default:
			slog.Info("health check", attrs...)
		}
	}

	// Log alerts.
	for _, a := range report.Alerts {
		attrs := []any{
			"check", a.Check,
			"from", a.From,
			"to", a.To,
			"message", a.Message,
		}
		if levelSeverity(a.To) > levelSeverity(a.From) {
			slog.Warn("ALERT escalation", attrs...)
		} else {
			slog.Info("ALERT improvement", attrs...)
		}
	}

	// Log overall summary.
	attrs := []any{
		"tick", report.Tick,
		"sim_time", report.SimTime,
		"overall", report.OverallHealth,
		"checks", len(report.Checks),
		"alerts", len(report.Alerts),
	}
	switch report.OverallHealth {
	case LevelCritical:
		slog.Error("sentinel cycle complete", attrs...)
	case LevelWarning:
		slog.Warn("sentinel cycle complete", attrs...)
	default:
		slog.Info("sentinel cycle complete", attrs...)
	}
}
