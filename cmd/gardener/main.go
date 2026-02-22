// Command gardener runs the autonomous world steward for Crossworlds.
// It observes world state, decides on interventions via Claude Haiku,
// and acts via the admin intervention API.
package main

import (
	"fmt"
	"log/slog"
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
	intervalMin := envIntOrDefault("GARDENER_INTERVAL", 360)

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

	// Run first cycle immediately.
	runCycle(observer, actor, llmClient)

	// Timer loop.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runCycle(observer, actor, llmClient)
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
			fmt.Println("Gardener stopped.")
			return
		}
	}
}

// runCycle executes one observe → decide → act cycle.
func runCycle(observer *gardener.Observer, actor *gardener.Actor, llmClient *llm.Client) {
	slog.Info("gardener cycle starting")

	// Observe.
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
	)

	// Decide.
	decision, err := gardener.Decide(llmClient, snap)
	if err != nil {
		slog.Error("decision failed", "error", err)
		return
	}
	slog.Info("decision made",
		"action", decision.Action,
		"rationale", decision.Rationale,
	)

	// Act.
	if decision.Action == "none" || decision.Intervention == nil {
		slog.Info("gardener cycle complete — no intervention")
		return
	}

	result, err := actor.Act(decision.Intervention)
	if err != nil {
		slog.Error("intervention failed", "error", err)
		return
	}

	slog.Info("intervention executed",
		"type", decision.Intervention.Type,
		"settlement", decision.Intervention.Settlement,
		"success", result.Success,
		"details", result.Details,
	)
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
