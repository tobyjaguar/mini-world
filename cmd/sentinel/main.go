// Command sentinel runs the structural health monitor for Crossworlds.
// It observes world state via the public API, runs 8 health checks,
// detects trends, and raises alerts on state transitions.
// It never modifies the simulation — it observes and reports.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/talgya/mini-world/internal/sentinel"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Configuration from environment.
	apiURL := envOrDefault("SENTINEL_API_URL", "http://localhost:8080")
	dataDir := envOrDefault("SENTINEL_DATA_DIR", "/opt/worldsim")
	intervalMin := envIntOrDefault("SENTINEL_INTERVAL", 30)

	interval := time.Duration(intervalMin) * time.Minute

	slog.Info("Crossworlds Sentinel starting",
		"api_url", apiURL,
		"data_dir", dataDir,
		"interval", interval,
	)

	observer := sentinel.NewObserver(apiURL)
	state := sentinel.LoadState(dataDir)

	// Wait for worldsim API to be ready before first cycle.
	slog.Info("waiting for worldsim API...")
	waitForAPI(apiURL)

	// Run first cycle immediately.
	runCycle(observer, state)

	// Timer loop.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runCycle(observer, state)
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
			fmt.Println("Sentinel stopped.")
			return
		}
	}
}

// runCycle executes one observe → check → trend → alert → report cycle.
func runCycle(observer *sentinel.Observer, state *sentinel.SentinelState) {
	slog.Info("sentinel cycle starting")
	state.CycleNum++

	// 1. Observe.
	snap, err := observer.Observe()
	if err != nil {
		slog.Error("observation failed", "error", err)
		return
	}
	slog.Info("observation complete",
		"tick", snap.Status.Tick,
		"population", snap.Status.Population,
		"satisfaction", fmt.Sprintf("%.3f", snap.Status.AvgSatisfaction),
	)

	// 2. Run checks.
	checks, healthSnap := sentinel.RunChecks(snap, state)

	// 3. Add snapshot to ring buffer for trend analysis.
	state.AddSnapshot(healthSnap)

	// 4. Compute trends and attach to check results.
	for i := range checks {
		checks[i].Trend = string(sentinel.ComputeTrend(state.Snapshots, checks[i].Name))
	}

	// 5. Detect alerts (state transitions).
	alerts := sentinel.DetectAlerts(checks, state)

	// 6. Build and save report.
	report := sentinel.BuildReport(snap, checks, alerts)
	state.SaveReport(report)

	// 7. Log report.
	sentinel.LogReport(report)

	// 8. Persist state.
	state.Save()
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// waitForAPI polls the worldsim status endpoint with exponential backoff
// until it responds. Exits after 5 minutes if the API never becomes ready.
func waitForAPI(apiURL string) {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second
	deadline := time.Now().Add(5 * time.Minute)

	for {
		resp, err := http.Get(apiURL + "/api/v1/status")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				slog.Info("worldsim API is ready")
				return
			}
		}
		if time.Now().After(deadline) {
			slog.Error("worldsim API did not become ready within 5 minutes")
			os.Exit(1)
		}
		slog.Info("worldsim not ready, retrying...", "backoff", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
