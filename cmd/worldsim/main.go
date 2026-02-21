// Command worldsim runs the SYNTHESIS autonomous world simulation.
package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/api"
	"github.com/talgya/mini-world/internal/engine"
	"github.com/talgya/mini-world/internal/entropy"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/persistence"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/weather"
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

	// ── World Map (always regenerated — deterministic from seed) ──────
	slog.Info("generating world map...")
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

	// ── Load or Generate World State ─────────────────────────────────
	var allSettlements []*social.Settlement
	var allAgents []*agents.Agent
	var startTick uint64
	var startSeason uint8

	spawner := agents.NewSpawner(seed)

	if db.HasWorldState() {
		// Restore from saved state.
		slog.Info("found saved world state, loading...")

		var loadErr error
		allAgents, loadErr = db.LoadAgents()
		if loadErr != nil {
			slog.Error("failed to load agents", "error", loadErr)
			os.Exit(1)
		}

		allSettlements, loadErr = db.LoadSettlements()
		if loadErr != nil {
			slog.Error("failed to load settlements", "error", loadErr)
			os.Exit(1)
		}

		// Restore tick and season from metadata.
		if tickStr, err := db.GetMeta("last_tick"); err == nil {
			if t, err := strconv.ParseUint(tickStr, 10, 64); err == nil {
				startTick = t
			}
		}
		if seasonStr, err := db.GetMeta("season"); err == nil {
			if s, err := strconv.ParseUint(seasonStr, 10, 8); err == nil {
				startSeason = uint8(s)
			}
		}

		// Update spawner next ID to be above the highest existing agent ID.
		var maxID agents.AgentID
		for _, a := range allAgents {
			if a.ID > maxID {
				maxID = a.ID
			}
		}
		spawner.SetNextID(maxID + 1)

		// Promote Tier 1 agents if none exist yet (backfill for existing worlds).
		tier1Count := 0
		for _, a := range allAgents {
			if a.Tier == agents.Tier1 {
				tier1Count++
			}
		}
		if tier1Count == 0 {
			agents.PromoteToTier1(allAgents, 0.04)
			promoted := 0
			for _, a := range allAgents {
				if a.Tier == agents.Tier1 {
					promoted++
				}
			}
			slog.Info("backfilled Tier 1 agents", "promoted", promoted)
		}

		slog.Info("world state restored",
			"agents", len(allAgents),
			"settlements", len(allSettlements),
			"tick", startTick,
			"season", engine.SeasonName(startSeason),
			"sim_time", engine.SimTime(startTick),
		)
	} else {
		// Fresh world generation.
		slog.Info("no saved state found, generating new world...")

		settlementSeeds := world.PlaceSettlements(worldMap, seed)
		rng := rand.New(rand.NewSource(seed + 400))

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
		agents.PromoteToTier1(allAgents, 0.04) // 4% of remaining Tier 0 agents

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
	}

	// Link settlement hex references (needed for both fresh and loaded worlds).
	for _, st := range allSettlements {
		sid := st.ID
		hex := worldMap.Get(st.Position)
		if hex != nil {
			hex.SettlementID = &sid
		}
	}

	slog.Info("world ready",
		"agents", len(allAgents),
		"settlements", len(allSettlements),
		"hexes", worldMap.HexCount(),
	)

	// ── Simulation ────────────────────────────────────────────────────
	sim := engine.NewSimulation(worldMap, allAgents, allSettlements)
	sim.Spawner = spawner
	sim.LastTick = startTick
	sim.CurrentSeason = startSeason

	// Load agent memories from database (if any exist).
	if startTick > 0 {
		if err := db.LoadMemories(sim.AgentIndex); err != nil {
			slog.Warn("failed to load memories", "error", err)
		}
	}

	// ── LLM Client ───────────────────────────────────────────────────
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	llmClient := llm.NewClient(anthropicKey)
	if llmClient != nil {
		slog.Info("LLM client enabled (Haiku)")
		sim.LLM = llmClient
	} else {
		slog.Warn("ANTHROPIC_API_KEY not set — LLM features disabled (newspaper will use fallback)")
	}

	// ── Weather Client ────────────────────────────────────────────────
	weatherKey := os.Getenv("WEATHER_API_KEY")
	weatherLoc := os.Getenv("WEATHER_LOCATION")
	weatherClient := weather.NewClient(weatherKey, weatherLoc)
	if weatherClient != nil {
		slog.Info("weather client enabled", "location", weatherLoc)
		sim.WeatherClient = weatherClient
	} else {
		slog.Info("WEATHER_API_KEY not set — using seasonal weather defaults")
	}

	// ── Entropy Client ────────────────────────────────────────────────
	randomOrgKey := os.Getenv("RANDOM_ORG_API_KEY")
	entropyClient := entropy.NewClient(randomOrgKey)
	if entropyClient != nil {
		slog.Info("entropy client enabled (random.org)")
		sim.Entropy = entropyClient
	} else {
		slog.Info("RANDOM_ORG_API_KEY not set — using crypto/rand for entropy")
	}

	// Save on fresh generation only (loaded worlds are already saved).
	if startTick == 0 {
		if err := db.SaveWorldState(sim); err != nil {
			slog.Error("initial save failed", "error", err)
		}
	}

	eng := engine.NewEngine()
	eng.Tick = startTick
	eng.Speed = 1

	// Wire tick callbacks — auto-save every sim-day.
	eng.OnTick = sim.TickMinute
	eng.OnHour = sim.TickHour
	eng.OnDay = func(tick uint64) {
		sim.TickDay(tick)
		// Save daily stats snapshot.
		statsRow := persistence.StatsRow{
			Tick:            tick,
			Population:      sim.Stats.TotalPopulation,
			TotalWealth:     sim.Stats.TotalWealth,
			AvgMood:         float64(sim.Stats.AvgMood),
			AvgSurvival:     float64(sim.Stats.AvgSurvival),
			Births:          sim.Stats.Births,
			Deaths:          sim.Stats.Deaths,
			TradeVolume:     sim.Stats.TradeVolume,
			AvgCoherence:    sim.AvgCoherence(),
			SettlementCount: len(sim.Settlements),
			Gini:            sim.GiniCoefficient(),
		}
		if err := db.SaveStatsSnapshot(statsRow); err != nil {
			slog.Error("stats snapshot failed", "error", err)
		}
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
		LLM:      llmClient,
		DB:       db,
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
	if startTick > 0 {
		fmt.Printf("Resuming from tick %d (%s)\n", startTick, engine.SimTime(startTick))
	}
	fmt.Println("Starting simulation... (Ctrl+C to stop)")

	eng.Run()

	// Final save on shutdown.
	slog.Info("final save...")
	if err := db.SaveWorldState(sim); err != nil {
		slog.Error("final save failed", "error", err)
	}

	fmt.Println("Simulation stopped. World state saved.")
}
