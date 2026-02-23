// Command gardener runs the autonomous world steward for Crossworlds.
// It observes world state, decides on interventions via Claude Haiku,
// and acts via the admin intervention API.
package main

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/talgya/mini-world/internal/gardener"
	"github.com/talgya/mini-world/internal/llm"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Configuration from environment.
	apiURL := envOrDefault("WORLDSIM_API_URL", "http://localhost")
	adminKey := os.Getenv("WORLDSIM_ADMIN_KEY")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	intervalMin := envIntOrDefault("GARDENER_INTERVAL", 6)

	if adminKey == "" {
		slog.Error("WORLDSIM_ADMIN_KEY is required")
		os.Exit(1)
	}
	if anthropicKey == "" {
		slog.Error("ANTHROPIC_API_KEY is required")
		os.Exit(1)
	}

	interval := time.Duration(intervalMin) * time.Minute

	slog.Info("Crossworlds Gardener starting",
		"api_url", apiURL,
		"interval", interval,
	)

	observer := gardener.NewObserver(apiURL)
	actor := gardener.NewActor(apiURL, adminKey)
	llmClient := llm.NewClient(anthropicKey)
	memory := gardener.LoadMemory()

	// Wait for worldsim API to be ready before first cycle.
	// systemd After= only ensures process start, not HTTP readiness.
	slog.Info("waiting for worldsim API...")
	waitForAPI(apiURL)

	// Run first cycle immediately.
	runCycle(observer, actor, llmClient, memory)

	// Timer loop.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runCycle(observer, actor, llmClient, memory)
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
			fmt.Println("Gardener stopped.")
			return
		}
	}
}

// runCycle executes one observe → triage → decide → act → record cycle.
func runCycle(observer *gardener.Observer, actor *gardener.Actor, llmClient *llm.Client, memory *gardener.CycleMemory) {
	slog.Info("gardener cycle starting")

	// 1. Observe.
	snap, err := observer.Observe()
	if err != nil {
		slog.Error("observation failed", "error", err)
		return
	}
	slog.Info("observation complete",
		"population", snap.Status.Population,
		"settlements", snap.Status.Settlements,
		"market_health", fmt.Sprintf("%.2f", snap.Economy.AvgMarketHealth),
		"avg_mood", fmt.Sprintf("%.2f", snap.Status.AvgMood),
		"avg_satisfaction", fmt.Sprintf("%.3f", snap.Status.AvgSatisfaction),
		"avg_alignment", fmt.Sprintf("%.3f", snap.Status.AvgAlignment),
	)

	// 2. Triage — deterministic health check.
	health := gardener.Triage(snap)
	dbRatioStr := fmt.Sprintf("%.2f", health.DeathBirthRatio)
	if math.IsInf(health.DeathBirthRatio, 1) {
		dbRatioStr = "INF"
	}
	slog.Info("triage complete",
		"crisis_level", health.CrisisLevel,
		"death_birth_ratio", dbRatioStr,
		"satisfaction", fmt.Sprintf("%.3f", health.AvgSatisfaction),
		"alignment", fmt.Sprintf("%.3f", health.AvgAlignment),
		"tiny_settlements", health.TinySettlements,
		"trade_per_capita", fmt.Sprintf("%.4f", health.TradePerCapita),
	)

	// 3. Decide.
	decision, err := gardener.Decide(llmClient, snap, health, memory)
	if err != nil {
		slog.Error("decision failed", "error", err)
		return
	}
	slog.Info("decision made",
		"action", decision.Action,
		"interventions", len(decision.Interventions),
		"rationale", decision.Rationale,
	)

	// 4. Act — execute all interventions.
	if decision.Action == "none" || len(decision.Interventions) == 0 {
		slog.Info("gardener cycle complete — no intervention")
	} else {
		for i, iv := range decision.Interventions {
			result, err := actor.Act(iv)
			if err != nil {
				slog.Error("intervention failed", "index", i, "type", iv.Type, "error", err)
				continue
			}
			slog.Info("intervention executed",
				"index", i,
				"type", iv.Type,
				"settlement", iv.Settlement,
				"success", result.Success,
				"details", result.Details,
			)
		}
	}

	// 5. Record — save cycle to memory.
	settlement := ""
	if len(decision.Interventions) > 0 {
		settlement = decision.Interventions[0].Settlement
	}
	record := gardener.CycleRecord{
		Tick:         snap.Status.Tick,
		Action:       decision.Action,
		DeathBirth:   health.DeathBirthRatio,
		Satisfaction: health.AvgSatisfaction,
		Alignment:    health.AvgAlignment,
		CrisisLevel:  health.CrisisLevel,
		Settlement:   settlement,
		Rationale:    decision.Rationale,
	}
	if math.IsInf(record.DeathBirth, 1) {
		record.DeathBirth = 999.0 // JSON can't encode Inf
	}
	memory.Record(record)
	memory.Save()

	slog.Info("gardener cycle complete")
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
