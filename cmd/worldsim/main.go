// Command worldsim runs the SYNTHESIS autonomous world simulation.
package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/api"
	"github.com/talgya/mini-world/internal/engine"
	"github.com/talgya/mini-world/internal/persistence"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("SYNTHESIS / Crossroads — Autonomous World Simulation")
	slog.Info("emanation constants",
		"phi", phi.Phi,
		"agnosis", fmt.Sprintf("%.5f", phi.Agnosis),
		"matter", fmt.Sprintf("%.5f", phi.Matter),
		"being", fmt.Sprintf("%.5f", phi.Being),
		"totality", fmt.Sprintf("%.5f", phi.Totality),
	)

	seed := int64(42)
	dbPath := "data/crossroads.db"
	apiPort := 80

	// ── Database ──────────────────────────────────────────────────────
	os.MkdirAll("data", 0755)
	db, err := persistence.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database opened", "path", dbPath)

	// ── World Generation ──────────────────────────────────────────────
	slog.Info("generating world...")
	cfg := world.DefaultGenConfig()
	cfg.Seed = seed
	worldMap := world.Generate(cfg)

	counts := world.TerrainCounts(worldMap)
	landHexes := 0
	for t, c := range counts {
		if t != world.TerrainOcean {
			landHexes += c
		}
		slog.Info("terrain", "type", world.TerrainName(t), "count", c)
	}
	slog.Info("world generated",
		"total_hexes", worldMap.HexCount(),
		"land_hexes", landHexes,
		"radius", worldMap.Radius,
	)

	// ── Settlement Placement ──────────────────────────────────────────
	slog.Info("placing settlements...")
	settlementSeeds := world.PlaceSettlements(worldMap, seed)

	// ── Create Settlements & Spawn Agents ─────────────────────────────
	rng := rand.New(rand.NewSource(seed + 400))
	spawner := agents.NewSpawner(seed)

	var allSettlements []*social.Settlement
	var allAgents []*agents.Agent

	for i, ss := range settlementSeeds {
		pop := world.PopulationForSize(ss.Size, rng)

		var gov social.GovernanceType
		switch ss.Size {
		case world.SizeCity:
			gov = social.GovMonarchy
		case world.SizeTown:
			if rng.Float32() < 0.5 {
				gov = social.GovCouncil
			} else {
				gov = social.GovMerchantRepublic
			}
		default:
			gov = social.GovCommune
		}

		sid := uint64(i + 1)
		settlement := &social.Settlement{
			ID:              sid,
			Name:            ss.Name,
			Position:        ss.Coord,
			Population:      pop,
			Governance:      gov,
			TaxRate:         0.10,
			Treasury:        uint64(pop) * 5,
			GovernanceScore: 0.5 + rng.Float64()*0.3,
			MarketLevel:     1,
		}

		hex := worldMap.Get(ss.Coord)
		if hex != nil {
			hex.SettlementID = &sid
		}

		allSettlements = append(allSettlements, settlement)

		terrain := world.TerrainPlains
		if hex != nil {
			terrain = hex.Terrain
		}
		popAgents := spawner.SpawnPopulation(pop, ss.Coord, sid, terrain)
		allAgents = append(allAgents, popAgents...)
	}

	agents.PromoteToTier2(allAgents, 30)

	for _, a := range allAgents {
		if a.Tier == agents.Tier2 {
			slog.Info("notable character",
				"name", a.Name,
				"age", a.Age,
				"occupation", a.Occupation,
				"coherence", fmt.Sprintf("%.3f", a.Soul.CittaCoherence),
				"element", a.Soul.Element(),
				"wealth", a.Wealth,
			)
		}
	}

	slog.Info("world ready",
		"agents", len(allAgents),
		"settlements", len(allSettlements),
		"hexes", worldMap.HexCount(),
	)

	// ── Simulation ────────────────────────────────────────────────────
	sim := engine.NewSimulation(worldMap, allAgents, allSettlements)

	// Initial save.
	if err := db.SaveWorldState(sim); err != nil {
		slog.Error("initial save failed", "error", err)
	}

	eng := engine.NewEngine()
	eng.Speed = 1

	// Wire tick callbacks — auto-save every sim-day.
	eng.OnTick = sim.TickMinute
	eng.OnHour = sim.TickHour
	eng.OnDay = func(tick uint64) {
		sim.TickDay(tick)
		// Auto-save daily.
		if err := db.SaveWorldState(sim); err != nil {
			slog.Error("daily save failed", "error", err)
		}
	}
	eng.OnWeek = sim.TickWeek
	eng.OnSeason = sim.TickSeason

	// ── HTTP API ──────────────────────────────────────────────────────
	adminKey := os.Getenv("WORLDSIM_ADMIN_KEY")
	if adminKey == "" {
		slog.Warn("WORLDSIM_ADMIN_KEY not set — admin POST endpoints will be disabled")
	}

	apiServer := &api.Server{
		Sim:      sim,
		Eng:      eng,
		Port:     apiPort,
		AdminKey: adminKey,
	}
	apiServer.Start()

	// ── Start ─────────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		eng.Stop()
	}()

	fmt.Printf("\nCrossroads is alive: %d souls across %d settlements on %d land hexes.\n",
		len(allAgents), len(allSettlements), landHexes)
	fmt.Printf("API: http://localhost:%d/api/v1/status\n", apiPort)
	fmt.Println("Starting simulation... (Ctrl+C to stop)")

	eng.Run()

	// Final save on shutdown.
	slog.Info("final save...")
	if err := db.SaveWorldState(sim); err != nil {
		slog.Error("final save failed", "error", err)
	}

	fmt.Println("Simulation stopped. World state saved.")
}
