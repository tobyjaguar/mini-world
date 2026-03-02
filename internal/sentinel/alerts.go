package sentinel

// AlertState tracks the alert history for a single check.
type AlertState struct {
	LastLevel    Level `json:"last_level"`
	LastAlertAt  int   `json:"last_alert_at"`  // cycle number of last alert
	LastClearedAt int  `json:"last_cleared_at"` // cycle number when last cleared
}

// Alert represents a state transition event.
type Alert struct {
	Check   string `json:"check"`
	From    Level  `json:"from"`
	To      Level  `json:"to"`
	Message string `json:"message"`
}

// cooldownCycles is the minimum cycles between re-alerting on the same check.
const cooldownCycles = 3

// DetectAlerts compares current check results against previous alert state
// and returns any state transition alerts.
func DetectAlerts(checks []CheckResult, state *SentinelState) []Alert {
	var alerts []Alert

	// First cycle: baseline only, no alerts.
	if state.CycleNum <= 1 {
		for _, c := range checks {
			state.Alerts[c.Name] = &AlertState{
				LastLevel:   c.Status,
				LastAlertAt: state.CycleNum,
			}
		}
		return nil
	}

	for _, c := range checks {
		prev, exists := state.Alerts[c.Name]
		if !exists {
			// New check, set baseline.
			state.Alerts[c.Name] = &AlertState{
				LastLevel:   c.Status,
				LastAlertAt: state.CycleNum,
			}
			continue
		}

		// No state transition → no alert.
		if c.Status == prev.LastLevel {
			continue
		}

		// Cooldown check: don't re-alert within cooldownCycles.
		if state.CycleNum-prev.LastAlertAt < cooldownCycles {
			continue
		}

		// State transition detected.
		alert := Alert{
			Check:   c.Name,
			From:    prev.LastLevel,
			To:      c.Status,
			Message: c.Detail,
		}
		alerts = append(alerts, alert)

		// Update alert state.
		prev.LastLevel = c.Status
		prev.LastAlertAt = state.CycleNum
		if levelSeverity(c.Status) < levelSeverity(prev.LastLevel) {
			prev.LastClearedAt = state.CycleNum
		}
	}

	return alerts
}
