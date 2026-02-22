// Package api provides the HTTP API for querying world state.
// GET endpoints are public (read-only observation).
// POST endpoints require a bearer token (admin control plane).
// See design doc Section 8.4.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/engine"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/persistence"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// Server serves the world state over HTTP.
type Server struct {
	Sim      *engine.Simulation
	Eng      *engine.Engine
	LLM      *llm.Client
	DB       *persistence.DB
	Port     int
	AdminKey string // Bearer token for POST endpoints. Empty = POST disabled.

	// Cached newspaper (regenerated at most once per sim-day).
	newspaperMu    sync.Mutex
	cachedPaper    *llm.Newspaper
	lastPaperTick  uint64

	// Cached biographies (agent ID → cached bio).
	bioMu    sync.Mutex
	bioCache map[agents.AgentID]cachedBio
}

type cachedBio struct {
	Biography   string `json:"biography"`
	GeneratedAt string `json:"generated_at"`
}

// Start begins serving the HTTP API in a goroutine.
func (s *Server) Start() {
	// Rate limiters for LLM-consuming endpoints.
	storyLimiter := NewRateLimiter(10, time.Hour)
	newspaperLimiter := NewRateLimiter(30, time.Hour)

	mux := http.NewServeMux()

	// Public endpoints (GET, read-only — anyone can check in on the world).
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/settlements", s.handleSettlements)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agent/", s.handleAgentRoutes(storyLimiter))
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/newspaper", RateLimitMiddleware(newspaperLimiter, s.handleNewspaper))
	mux.HandleFunc("/api/v1/factions", s.handleFactions)
	mux.HandleFunc("/api/v1/economy", s.handleEconomy)
	mux.HandleFunc("/api/v1/social", s.handleSocial)

	// Detail endpoints.
	mux.HandleFunc("/api/v1/settlement/", s.handleSettlementDetail)
	mux.HandleFunc("/api/v1/faction/", s.handleFactionDetail)
	mux.HandleFunc("/api/v1/map", s.handleMapRoutes)
	mux.HandleFunc("/api/v1/stats/history", s.handleStatsHistory)

	// Admin endpoints (POST, require bearer token).
	mux.HandleFunc("/api/v1/speed", s.adminOnly(s.handleSpeed))
	mux.HandleFunc("/api/v1/snapshot", s.adminOnly(s.handleSnapshot))
	mux.HandleFunc("/api/v1/intervention", s.adminOnly(s.handleIntervention))

	addr := fmt.Sprintf(":%d", s.Port)
	slog.Info("HTTP API starting", "addr", addr, "admin_auth", s.AdminKey != "")

	go func() {
		handler := corsMiddleware(mux)
		if err := http.ListenAndServe(addr, handler); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()
}

// corsMiddleware adds CORS headers for allowed frontend origins.
// Set CORS_ORIGINS env var to a comma-separated list of allowed origins
// (e.g. "https://crossworlds.example.com,https://crossworlds-ui.vercel.app").
// Localhost dev servers are always allowed.
func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost:5173": true,
		"http://localhost:4173": true,
		"http://localhost:3000": true,
	}
	if env := os.Getenv("CORS_ORIGINS"); env != "" {
		for _, origin := range strings.Split(env, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins[origin] = true
			}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleMapRoutes dispatches between bulk map (GET /api/v1/map) and hex detail (GET /api/v1/map/:q/:r).
func (s *Server) handleMapRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/map")
	if path == "" || path == "/" {
		s.handleBulkMap(w, r)
		return
	}
	s.handleHexDetail(w, r)
}

// handleBulkMap returns all hexes for the hex map renderer.
func (s *Server) handleBulkMap(w http.ResponseWriter, r *http.Request) {
	type hexEntry struct {
		Q            int     `json:"q"`
		R            int     `json:"r"`
		Terrain      uint8   `json:"terrain"`
		Elevation    float64 `json:"elevation"`
		SettlementID *uint64 `json:"settlement_id,omitempty"`
	}

	type settlementEntry struct {
		ID         uint64 `json:"id"`
		Name       string `json:"name"`
		Q          int    `json:"q"`
		R          int    `json:"r"`
		Population uint32 `json:"population"`
	}

	hexes := make([]hexEntry, 0, len(s.Sim.WorldMap.Hexes))
	for _, h := range s.Sim.WorldMap.Hexes {
		hexes = append(hexes, hexEntry{
			Q:            h.Coord.Q,
			R:            h.Coord.R,
			Terrain:      uint8(h.Terrain),
			Elevation:    h.Elevation,
			SettlementID: h.SettlementID,
		})
	}

	settlements := make([]settlementEntry, 0, len(s.Sim.Settlements))
	for _, st := range s.Sim.Settlements {
		settlements = append(settlements, settlementEntry{
			ID:         st.ID,
			Name:       st.Name,
			Q:          st.Position.Q,
			R:          st.Position.R,
			Population: st.Population,
		})
	}

	writeJSON(w, map[string]any{
		"radius":      s.Sim.WorldMap.Radius,
		"hexes":       hexes,
		"settlements": settlements,
	})
}

// checkBearerToken returns true if the request has a valid admin bearer token.
func (s *Server) checkBearerToken(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == s.AdminKey
}

// adminOnly wraps a handler to require bearer token auth on POST requests.
// GET requests pass through (for endpoints that support both GET and POST).
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if s.AdminKey == "" {
				http.Error(w, "admin endpoints disabled (no WORLDSIM_ADMIN_KEY set)", http.StatusForbidden)
				return
			}

			if !s.checkBearerToken(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	weatherInfo := map[string]any{
		"description":   s.Sim.CurrentWeather.Description,
		"temp_modifier": s.Sim.CurrentWeather.TempModifier,
	}

	status := map[string]any{
		"name":         "Crossworlds",
		"tick":         s.Sim.CurrentTick(),
		"sim_time":     engine.SimTime(s.Sim.CurrentTick()),
		"season":       engine.SeasonName(s.Sim.CurrentSeason),
		"speed":        s.Eng.Speed,
		"running":      s.Eng.Running,
		"population":   s.Sim.Stats.TotalPopulation,
		"deaths":       s.Sim.Stats.Deaths,
		"births":       s.Sim.Stats.Births,
		"settlements":  len(s.Sim.Settlements),
		"factions":     len(s.Sim.Factions),
		"avg_mood":     s.Sim.Stats.AvgMood,
		"total_wealth": s.Sim.Stats.TotalWealth,
		"weather":      weatherInfo,
	}
	writeJSON(w, status)
}

func (s *Server) handleSettlements(w http.ResponseWriter, r *http.Request) {
	type settlementSummary struct {
		ID         uint64  `json:"id"`
		Name       string  `json:"name"`
		Q          int     `json:"q"`
		R          int     `json:"r"`
		Population uint32  `json:"population"`
		Governance string  `json:"governance"`
		Treasury   uint64  `json:"treasury"`
		Health     float64 `json:"health"`
	}

	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}

	var result []settlementSummary
	for _, st := range s.Sim.Settlements {
		result = append(result, settlementSummary{
			ID:         st.ID,
			Name:       st.Name,
			Q:          st.Position.Q,
			R:          st.Position.R,
			Population: st.Population,
			Governance: govNames[uint8(st.Governance)],
			Treasury:   st.Treasury,
			Health:     st.Health(),
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	tier := r.URL.Query().Get("tier")

	type agentSummary struct {
		ID         agents.AgentID `json:"id"`
		Name       string         `json:"name"`
		Age        uint16         `json:"age"`
		Occupation string         `json:"occupation"`
		Tier       int            `json:"tier"`
		Coherence  float32        `json:"coherence"`
		Mood       float32        `json:"mood"`
		Wealth     uint64         `json:"wealth"`
		Alive      bool           `json:"alive"`
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}

	var result []agentSummary
	for _, a := range s.Sim.Agents {
		if tier != "" {
			t, _ := strconv.Atoi(tier)
			if int(a.Tier) != t {
				continue
			}
		} else if a.Tier < agents.Tier2 {
			continue
		}

		occName := "Unknown"
		if int(a.Occupation) < len(occNames) {
			occName = occNames[a.Occupation]
		}

		result = append(result, agentSummary{
			ID:         a.ID,
			Name:       a.Name,
			Age:        a.Age,
			Occupation: occName,
			Tier:       int(a.Tier),
			Coherence:  a.Soul.CittaCoherence,
			Mood:       a.Mood,
			Wealth:     a.Wealth,
			Alive:      a.Alive,
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleAgentRoutes(storyLimiter *RateLimiter) http.HandlerFunc {
	rateLimitedStory := RateLimitMiddleware(storyLimiter, func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		id, _ := strconv.ParseUint(parts[4], 10, 64)
		agent := s.Sim.AgentIndex[agents.AgentID(id)]
		s.handleAgentStory(w, r, agent)
	})

	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 5 {
			http.Error(w, "missing agent id", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseUint(parts[4], 10, 64)
		if err != nil {
			http.Error(w, "invalid agent id", http.StatusBadRequest)
			return
		}

		agent, ok := s.Sim.AgentIndex[agents.AgentID(id)]
		if !ok {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		_ = agent

		// Route to /agent/:id/story if requested.
		if len(parts) >= 6 && parts[5] == "story" {
			rateLimitedStory(w, r)
			return
		}

		writeJSON(w, s.Sim.AgentIndex[agents.AgentID(id)])
	}
}

func (s *Server) handleAgentStory(w http.ResponseWriter, r *http.Request, agent *agents.Agent) {
	refresh := r.URL.Query().Get("refresh") == "true"

	// Refresh requires admin auth (LLM-consuming operation).
	if refresh {
		if s.AdminKey == "" || !s.checkBearerToken(r) {
			http.Error(w, "refresh requires admin authorization", http.StatusUnauthorized)
			return
		}
	}

	// Check cache.
	s.bioMu.Lock()
	if s.bioCache == nil {
		s.bioCache = make(map[agents.AgentID]cachedBio)
	}
	cached, hasCached := s.bioCache[agent.ID]
	s.bioMu.Unlock()

	if hasCached && !refresh {
		writeJSON(w, map[string]any{
			"name":         agent.Name,
			"biography":    cached.Biography,
			"generated_at": cached.GeneratedAt,
		})
		return
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}
	stateNames := map[agents.StateOfBeing]string{
		agents.Embodied: "Embodied", agents.Centered: "Centered", agents.Liberated: "Liberated",
	}
	elementNames := map[agents.ElementType]string{
		agents.ElementHelium: "Helium", agents.ElementHydrogen: "Hydrogen",
		agents.ElementGold: "Gold", agents.ElementUranium: "Uranium",
	}

	occName := "Unknown"
	if int(agent.Occupation) < len(occNames) {
		occName = occNames[agent.Occupation]
	}

	ctx := llm.BiographyContext{
		Name:      agent.Name,
		Age:       agent.Age,
		Occupation: occName,
		Wealth:    agent.Wealth,
		Coherence: agent.Soul.CittaCoherence,
		State:     stateNames[agent.Soul.State],
		Element:   elementNames[agent.Soul.Element()],
		Archetype: agent.Archetype,
		Mood:      agent.Mood,
	}

	// Settlement name.
	if agent.HomeSettID != nil {
		if sett, ok := s.Sim.SettlementIndex[*agent.HomeSettID]; ok {
			ctx.Settlement = sett.Name
		}
	}

	// Faction name.
	if agent.FactionID != nil {
		for _, f := range s.Sim.Factions {
			if uint64(f.ID) == *agent.FactionID {
				ctx.Faction = f.Name
				break
			}
		}
	}

	// Top relationships.
	for i, rel := range agent.Relationships {
		if i >= 5 {
			break
		}
		if target, ok := s.Sim.AgentIndex[rel.TargetID]; ok {
			sentiment := "neutral toward"
			if rel.Sentiment > 0.5 {
				sentiment = "close to"
			} else if rel.Sentiment < -0.3 {
				sentiment = "hostile toward"
			}
			ctx.Relationships = append(ctx.Relationships, fmt.Sprintf("%s %s", sentiment, target.Name))
		}
	}

	// Top memories by importance.
	if len(agent.Memories) > 0 {
		sorted := make([]agents.Memory, len(agent.Memories))
		copy(sorted, agent.Memories)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Importance > sorted[j].Importance })
		for i, m := range sorted {
			if i >= 10 {
				break
			}
			ctx.Memories = append(ctx.Memories, m.Content)
		}
	}

	bio, err := llm.GenerateBiography(s.LLM, ctx)
	if err != nil {
		slog.Error("biography generation failed", "error", err, "agent", agent.Name)
		http.Error(w, "biography generation failed", http.StatusInternalServerError)
		return
	}

	genTime := engine.SimTime(s.Sim.CurrentTick())
	s.bioMu.Lock()
	s.bioCache[agent.ID] = cachedBio{Biography: bio, GeneratedAt: genTime}
	s.bioMu.Unlock()

	writeJSON(w, map[string]any{
		"name":         agent.Name,
		"biography":    bio,
		"generated_at": genTime,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	events := s.Sim.Events
	start := 0
	if len(events) > limit {
		start = len(events) - limit
	}

	writeJSON(w, events[start:])
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Sim.Stats)
}

func (s *Server) handleSpeed(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Speed float64 `json:"speed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Speed < 0 || req.Speed > 1000 {
			http.Error(w, "speed must be 0-1000", http.StatusBadRequest)
			return
		}
		s.Eng.Speed = req.Speed
		slog.Info("speed changed", "speed", req.Speed)
	}

	writeJSON(w, map[string]float64{"speed": s.Eng.Speed})
}

func (s *Server) handleNewspaper(w http.ResponseWriter, r *http.Request) {
	s.newspaperMu.Lock()
	defer s.newspaperMu.Unlock()

	currentTick := s.Sim.CurrentTick()
	currentDay := currentTick / engine.TicksPerSimDay

	// Return cached newspaper if still from today.
	if s.cachedPaper != nil && s.lastPaperTick/engine.TicksPerSimDay == currentDay {
		writeJSON(w, s.cachedPaper)
		return
	}

	// Build newspaper data from current world state.
	data := s.buildNewspaperData()

	paper, err := llm.GenerateNewspaper(s.LLM, data)
	if err != nil {
		slog.Error("newspaper generation failed", "error", err)
		http.Error(w, "newspaper generation failed", http.StatusInternalServerError)
		return
	}

	s.cachedPaper = paper
	s.lastPaperTick = currentTick
	writeJSON(w, paper)
}

func (s *Server) buildNewspaperData() *llm.NewspaperData {
	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}
	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}
	goodNames := map[agents.GoodType]string{
		agents.GoodGrain: "Grain", agents.GoodTimber: "Timber", agents.GoodIronOre: "Iron Ore",
		agents.GoodStone: "Stone", agents.GoodFish: "Fish", agents.GoodHerbs: "Herbs",
		agents.GoodGems: "Gems", agents.GoodFurs: "Furs", agents.GoodCoal: "Coal",
		agents.GoodExotics: "Exotics", agents.GoodTools: "Tools", agents.GoodWeapons: "Weapons",
		agents.GoodClothing: "Clothing", agents.GoodMedicine: "Medicine", agents.GoodLuxuries: "Luxuries",
	}
	stateNames := map[agents.StateOfBeing]string{
		agents.Embodied: "Embodied", agents.Centered: "Centered", agents.Liberated: "Liberated",
	}
	elementNames := map[agents.ElementType]string{
		agents.ElementHelium: "Helium", agents.ElementHydrogen: "Hydrogen",
		agents.ElementGold: "Gold", agents.ElementUranium: "Uranium",
	}
	kindNames := []string{"Political", "Economic", "Military", "Religious", "Criminal"}

	data := &llm.NewspaperData{
		SimTime:     engine.SimTime(s.Sim.CurrentTick()),
		Season:      engine.SeasonName(s.Sim.CurrentSeason),
		Population:  s.Sim.Stats.TotalPopulation,
		Settlements: len(s.Sim.Settlements),
		TotalWealth: s.Sim.Stats.TotalWealth,
		AvgMood:     s.Sim.Stats.AvgMood,
	}

	// Weather.
	data.Weather = s.Sim.CurrentWeather.Description

	// Collect recent events by category.
	for _, e := range s.Sim.Events {
		switch e.Category {
		case "death":
			data.Deaths = append(data.Deaths, e.Description)
		case "birth":
			data.Births = append(data.Births, e.Description)
		case "crime":
			data.Crimes = append(data.Crimes, e.Description)
		case "social":
			data.Social = append(data.Social, e.Description)
		case "economy":
			data.Economy = append(data.Economy, e.Description)
		case "political":
			data.Political = append(data.Political, e.Description)
		}
	}

	// Top 5 settlements by population (sort first).
	sorted := make([]*social.Settlement, len(s.Sim.Settlements))
	copy(sorted, s.Sim.Settlements)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Population > sorted[j].Population
	})
	for i, st := range sorted {
		if i >= 5 {
			break
		}
		data.TopSettlements = append(data.TopSettlements, llm.SettlementSummary{
			Name:       st.Name,
			Population: st.Population,
			Treasury:   st.Treasury,
			Governance: govNames[uint8(st.Governance)],
			Health:     st.Health(),
		})
	}

	// Market prices — collect all entries, find the most notable (furthest from base price).
	type priceEntry struct {
		good       string
		settlement string
		price      float64
		ratio      float64
	}
	var allPrices []priceEntry
	for _, st := range s.Sim.Settlements {
		if st.Market == nil {
			continue
		}
		for goodType, entry := range st.Market.Entries {
			if entry.BasePrice <= 0 {
				continue
			}
			ratio := entry.Price / entry.BasePrice
			gn := goodNames[goodType]
			if gn == "" {
				gn = fmt.Sprintf("Good#%d", goodType)
			}
			allPrices = append(allPrices, priceEntry{
				good:       gn,
				settlement: st.Name,
				price:      entry.Price,
				ratio:      ratio,
			})
		}
	}
	// Sort by distance from 1.0 (most notable deviations first).
	sort.Slice(allPrices, func(i, j int) bool {
		return math.Abs(allPrices[i].ratio-1.0) > math.Abs(allPrices[j].ratio-1.0)
	})
	for i, pe := range allPrices {
		if i >= 8 {
			break
		}
		data.MarketPrices = append(data.MarketPrices, llm.MarketPriceSummary{
			Good:       pe.good,
			Settlement: pe.settlement,
			Price:      pe.price,
			PriceRatio: pe.ratio,
		})
	}

	// Faction news.
	for _, f := range s.Sim.Factions {
		kindName := "Unknown"
		if int(f.Kind) < len(kindNames) {
			kindName = kindNames[f.Kind]
		}

		// Find top settlement by influence.
		var topSett string
		var topInf float64
		for settID, inf := range f.Influence {
			if inf > topInf {
				if sett, ok := s.Sim.SettlementIndex[settID]; ok {
					topSett = sett.Name
					topInf = inf
				}
			}
		}

		// Build faction summary line.
		line := fmt.Sprintf("%s (%s): treasury %d crowns", f.Name, kindName, f.Treasury)
		if topSett != "" {
			line += fmt.Sprintf(", strongest in %s (influence %.0f)", topSett, topInf)
		}

		// Note any strong inter-faction tensions or alliances.
		for otherID, rel := range f.Relations {
			if rel > 50 {
				for _, other := range s.Sim.Factions {
					if other.ID == otherID {
						line += fmt.Sprintf("; allied with %s", other.Name)
						break
					}
				}
			} else if rel < -50 {
				for _, other := range s.Sim.Factions {
					if other.ID == otherID {
						line += fmt.Sprintf("; hostile toward %s", other.Name)
						break
					}
				}
			}
		}

		data.FactionNews = append(data.FactionNews, line)
	}

	// Wheeler coherence — count states and compute average.
	var totalCoherence float32
	aliveCount := 0
	for _, a := range s.Sim.Agents {
		if !a.Alive {
			continue
		}
		aliveCount++
		totalCoherence += a.Soul.CittaCoherence
		switch a.Soul.State {
		case agents.Embodied:
			data.CoherenceCounts.Embodied++
		case agents.Centered:
			data.CoherenceCounts.Centered++
		case agents.Liberated:
			data.CoherenceCounts.Liberated++
		}
	}
	if aliveCount > 0 {
		data.AvgCoherence = totalCoherence / float32(aliveCount)
	}

	// Notable agents (Tier 2) with Wheeler descriptions.
	for _, a := range s.Sim.Agents {
		if a.Tier >= agents.Tier2 && a.Alive {
			occName := "Unknown"
			if int(a.Occupation) < len(occNames) {
				occName = occNames[a.Occupation]
			}

			stateName := stateNames[a.Soul.State]
			elemName := elementNames[a.Soul.Element()]

			data.NotableAgents = append(data.NotableAgents, llm.AgentSummary{
				Name:       a.Name,
				Age:        a.Age,
				Occupation: occName,
				Wealth:     a.Wealth,
				Mood:       fmt.Sprintf("%.2f", a.Mood),
				State:      stateName,
				Element:    elemName,
				Coherence:  a.Soul.CittaCoherence,
			})
		}
	}

	return data
}

func (s *Server) handleFactions(w http.ResponseWriter, r *http.Request) {
	if s.Sim.Factions == nil {
		writeJSON(w, []any{})
		return
	}

	type factionSummary struct {
		ID        uint64             `json:"id"`
		Name      string             `json:"name"`
		Kind      string             `json:"kind"`
		Treasury  uint64             `json:"treasury"`
		Influence map[string]float64 `json:"top_influence"` // settlement name → influence
	}

	kindNames := []string{"Political", "Economic", "Military", "Religious", "Criminal"}

	var result []factionSummary
	for _, f := range s.Sim.Factions {
		kindName := "Unknown"
		if int(f.Kind) < len(kindNames) {
			kindName = kindNames[f.Kind]
		}

		// Convert settlement ID influence to settlement names (top 5).
		topInf := make(map[string]float64)
		for settID, inf := range f.Influence {
			if sett, ok := s.Sim.SettlementIndex[settID]; ok {
				if len(topInf) < 5 || inf > 5 {
					topInf[sett.Name] = inf
				}
			}
		}

		result = append(result, factionSummary{
			ID:        uint64(f.ID),
			Name:      f.Name,
			Kind:      kindName,
			Treasury:  f.Treasury,
			Influence: topInf,
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleEconomy(w http.ResponseWriter, r *http.Request) {
	// Total crowns: agent wealth + settlement treasuries.
	totalAgentWealth := uint64(0)
	totalTreasury := uint64(0)
	aliveCount := 0

	// Collect all agent wealths for distribution calculation.
	var wealths []uint64
	for _, a := range s.Sim.Agents {
		if a.Alive {
			aliveCount++
			totalAgentWealth += a.Wealth
			wealths = append(wealths, a.Wealth)
		}
	}
	for _, st := range s.Sim.Settlements {
		totalTreasury += st.Treasury
	}

	// Wealth distribution: sort and compute shares.
	sort.Slice(wealths, func(i, j int) bool { return wealths[i] < wealths[j] })

	poorest50Share := 0.0
	richest10Share := 0.0
	if len(wealths) > 0 && totalAgentWealth > 0 {
		mid := len(wealths) / 2
		top := len(wealths) - len(wealths)/10

		poorSum := uint64(0)
		for _, w := range wealths[:mid] {
			poorSum += w
		}
		richSum := uint64(0)
		for _, w := range wealths[top:] {
			richSum += w
		}
		poorest50Share = float64(poorSum) / float64(totalAgentWealth)
		richest10Share = float64(richSum) / float64(totalAgentWealth)
	}

	// Market health and price deviations.
	goodNames := map[agents.GoodType]string{
		agents.GoodGrain: "Grain", agents.GoodTimber: "Timber", agents.GoodIronOre: "Iron Ore",
		agents.GoodStone: "Stone", agents.GoodFish: "Fish", agents.GoodHerbs: "Herbs",
		agents.GoodGems: "Gems", agents.GoodFurs: "Furs", agents.GoodCoal: "Coal",
		agents.GoodExotics: "Exotics", agents.GoodTools: "Tools", agents.GoodWeapons: "Weapons",
		agents.GoodClothing: "Clothing", agents.GoodMedicine: "Medicine", agents.GoodLuxuries: "Luxuries",
	}

	type priceDeviation struct {
		Good       string  `json:"good"`
		Settlement string  `json:"settlement"`
		Price      float64 `json:"price"`
		BasePrice  float64 `json:"base_price"`
		Ratio      float64 `json:"ratio"`
	}

	var inflated, deflated []priceDeviation
	totalHealth := 0.0
	marketCount := 0

	for _, st := range s.Sim.Settlements {
		if st.Market == nil {
			continue
		}
		marketCount++
		// Average conjugate field health for this market.
		settHealth := 0.0
		entryCount := 0
		for goodType, entry := range st.Market.Entries {
			if entry.BasePrice <= 0 {
				continue
			}
			ratio := entry.Price / entry.BasePrice
			entryCount++

			gn := goodNames[goodType]
			if gn == "" {
				gn = fmt.Sprintf("Good#%d", goodType)
			}

			pd := priceDeviation{
				Good:       gn,
				Settlement: st.Name,
				Price:      entry.Price,
				BasePrice:  entry.BasePrice,
				Ratio:      ratio,
			}
			if ratio > 1.0 {
				inflated = append(inflated, pd)
			} else if ratio < 1.0 {
				deflated = append(deflated, pd)
			}

			// Health: how close ratio is to 1.0 (perfect equilibrium).
			dev := math.Abs(ratio - 1.0)
			settHealth += 1.0 - dev
		}
		if entryCount > 0 {
			totalHealth += settHealth / float64(entryCount)
		}
	}

	avgMarketHealth := 0.0
	if marketCount > 0 {
		avgMarketHealth = totalHealth / float64(marketCount)
	}

	// Sort inflated descending by ratio, deflated ascending by ratio.
	sort.Slice(inflated, func(i, j int) bool { return inflated[i].Ratio > inflated[j].Ratio })
	sort.Slice(deflated, func(i, j int) bool { return deflated[i].Ratio < deflated[j].Ratio })

	// Take top 5 of each.
	if len(inflated) > 5 {
		inflated = inflated[:5]
	}
	if len(deflated) > 5 {
		deflated = deflated[:5]
	}

	result := map[string]any{
		"total_crowns":       totalAgentWealth + totalTreasury,
		"agent_wealth":       totalAgentWealth,
		"treasury_wealth":    totalTreasury,
		"avg_market_health":  avgMarketHealth,
		"trade_volume":       s.Sim.Stats.TradeVolume,
		"most_inflated":      inflated,
		"most_deflated":      deflated,
		"wealth_distribution": map[string]any{
			"poorest_50_pct_share": poorest50Share,
			"richest_10_pct_share": richest10Share,
		},
	}

	writeJSON(w, result)
}

func (s *Server) handleSocial(w http.ResponseWriter, r *http.Request) {
	// Faction summaries.
	type factionInfo struct {
		Name          string             `json:"name"`
		Treasury      uint64             `json:"treasury"`
		TotalMembers  int                `json:"total_members"`
		TopSettlements map[string]float64 `json:"top_settlements"`
	}

	factionMembers := make(map[uint64]int)
	for _, a := range s.Sim.Agents {
		if a.Alive && a.FactionID != nil {
			factionMembers[*a.FactionID]++
		}
	}

	var factions []factionInfo
	for _, f := range s.Sim.Factions {
		topSetts := make(map[string]float64)
		// Get top 3 settlements by influence.
		type infEntry struct {
			name string
			inf  float64
		}
		var entries []infEntry
		for settID, inf := range f.Influence {
			if sett, ok := s.Sim.SettlementIndex[settID]; ok {
				entries = append(entries, infEntry{sett.Name, inf})
			}
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].inf > entries[j].inf })
		for i, e := range entries {
			if i >= 3 {
				break
			}
			topSetts[e.name] = e.inf
		}

		factions = append(factions, factionInfo{
			Name:          f.Name,
			Treasury:      f.Treasury,
			TotalMembers:  factionMembers[uint64(f.ID)],
			TopSettlements: topSetts,
		})
	}

	// Governance health.
	totalGovScore := 0.0
	var atRiskSettlements []string
	for _, st := range s.Sim.Settlements {
		totalGovScore += st.GovernanceScore
		if st.GovernanceScore < 0.3 {
			atRiskSettlements = append(atRiskSettlements, st.Name)
		}
	}
	avgGovScore := 0.0
	if len(s.Sim.Settlements) > 0 {
		avgGovScore = totalGovScore / float64(len(s.Sim.Settlements))
	}

	// Relationship stats.
	totalSentiment := float32(0)
	relCount := 0
	families := 0
	rivalries := 0
	for _, a := range s.Sim.Agents {
		if !a.Alive {
			continue
		}
		for _, rel := range a.Relationships {
			totalSentiment += rel.Sentiment
			relCount++
			if rel.Sentiment > 0.7 && rel.Trust > 0.5 {
				families++
			}
			if rel.Sentiment < -0.5 {
				rivalries++
			}
		}
	}
	avgSentiment := float32(0)
	if relCount > 0 {
		avgSentiment = totalSentiment / float32(relCount)
	}
	// Families are double-counted (both sides), so halve.
	families /= 2

	// Tier distribution.
	tier0, tier1, tier2 := 0, 0, 0
	embodied, centered, liberated := 0, 0, 0
	for _, a := range s.Sim.Agents {
		if !a.Alive {
			continue
		}
		switch a.Tier {
		case agents.Tier0:
			tier0++
		case agents.Tier1:
			tier1++
		case agents.Tier2:
			tier2++
		}
		switch a.Soul.State {
		case agents.Embodied:
			embodied++
		case agents.Centered:
			centered++
		case agents.Liberated:
			liberated++
		}
	}

	// Recent political events.
	var politicalEvents []engine.Event
	for _, e := range s.Sim.Events {
		if e.Category == "political" {
			politicalEvents = append(politicalEvents, e)
		}
	}
	// Keep only last 20.
	if len(politicalEvents) > 20 {
		politicalEvents = politicalEvents[len(politicalEvents)-20:]
	}

	result := map[string]any{
		"factions": factions,
		"governance": map[string]any{
			"avg_score":            avgGovScore,
			"at_risk_settlements":  atRiskSettlements,
		},
		"relationships": map[string]any{
			"avg_sentiment": avgSentiment,
			"families":      families,
			"rivalries":     rivalries,
		},
		"tier_distribution": map[string]int{
			"tier_0": tier0,
			"tier_1": tier1,
			"tier_2": tier2,
		},
		"coherence_distribution": map[string]int{
			"embodied":  embodied,
			"centered":  centered,
			"liberated": liberated,
		},
		"recent_political_events": politicalEvents,
	}

	writeJSON(w, result)
}

func (s *Server) handleStatsHistory(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	fromTick := uint64(0)
	toTick := uint64(1<<63 - 1) // Max int64 — avoids uint64 high-bit SQLite driver issue.
	limit := 30

	if f := r.URL.Query().Get("from"); f != "" {
		if v, err := strconv.ParseUint(f, 10, 64); err == nil {
			fromTick = v
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if v, err := strconv.ParseUint(t, 10, 64); err == nil {
			toTick = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}

	rows, err := s.DB.LoadStatsHistory(fromTick, toTick, limit)
	if err != nil {
		slog.Error("stats history query failed", "error", err)
		// Return empty array instead of error — table may not have data yet.
		writeJSON(w, []persistence.StatsRow{})
		return
	}
	if rows == nil {
		rows = []persistence.StatsRow{}
	}
	writeJSON(w, rows)
}

func (s *Server) handleSettlementDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "missing settlement id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		http.Error(w, "invalid settlement id", http.StatusBadRequest)
		return
	}

	sett, ok := s.Sim.SettlementIndex[id]
	if !ok {
		http.Error(w, "settlement not found", http.StatusNotFound)
		return
	}

	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}

	// Market data.
	goodNames := map[agents.GoodType]string{
		agents.GoodGrain: "Grain", agents.GoodTimber: "Timber", agents.GoodIronOre: "Iron Ore",
		agents.GoodStone: "Stone", agents.GoodFish: "Fish", agents.GoodHerbs: "Herbs",
		agents.GoodGems: "Gems", agents.GoodFurs: "Furs", agents.GoodCoal: "Coal",
		agents.GoodExotics: "Exotics", agents.GoodTools: "Tools", agents.GoodWeapons: "Weapons",
		agents.GoodClothing: "Clothing", agents.GoodMedicine: "Medicine", agents.GoodLuxuries: "Luxuries",
	}

	type marketEntry struct {
		Good   string  `json:"good"`
		Price  float64 `json:"price"`
		Supply float64 `json:"supply"`
		Demand float64 `json:"demand"`
	}
	var market []marketEntry
	if sett.Market != nil {
		for goodType, entry := range sett.Market.Entries {
			gn := goodNames[goodType]
			if gn == "" {
				gn = fmt.Sprintf("Good#%d", goodType)
			}
			market = append(market, marketEntry{
				Good:   gn,
				Price:  entry.Price,
				Supply: entry.Supply,
				Demand: entry.Demand,
			})
		}
	}

	// Top 5 agents by wealth.
	type agentBrief struct {
		ID     agents.AgentID `json:"id"`
		Name   string         `json:"name"`
		Wealth uint64         `json:"wealth"`
	}
	settAgents := s.Sim.SettlementAgents[id]
	var aliveAgents []*agents.Agent
	for _, a := range settAgents {
		if a.Alive {
			aliveAgents = append(aliveAgents, a)
		}
	}
	sort.Slice(aliveAgents, func(i, j int) bool { return aliveAgents[i].Wealth > aliveAgents[j].Wealth })
	var topAgents []agentBrief
	for i, a := range aliveAgents {
		if i >= 5 {
			break
		}
		topAgents = append(topAgents, agentBrief{ID: a.ID, Name: a.Name, Wealth: a.Wealth})
	}

	// Faction presence.
	factionCounts := make(map[string]int)
	for _, a := range settAgents {
		if a.Alive && a.FactionID != nil {
			for _, f := range s.Sim.Factions {
				if uint64(f.ID) == *a.FactionID {
					factionCounts[f.Name]++
					break
				}
			}
		}
	}

	// Recent events mentioning this settlement.
	var recentEvents []engine.Event
	for _, e := range s.Sim.Events {
		if strings.Contains(e.Description, sett.Name) {
			recentEvents = append(recentEvents, e)
		}
	}
	if len(recentEvents) > 20 {
		recentEvents = recentEvents[len(recentEvents)-20:]
	}

	result := map[string]any{
		"id":               sett.ID,
		"name":             sett.Name,
		"q":                sett.Position.Q,
		"r":                sett.Position.R,
		"population":       sett.Population,
		"governance":       govNames[uint8(sett.Governance)],
		"treasury":         sett.Treasury,
		"health":           sett.Health(),
		"governance_score": sett.GovernanceScore,
		"tax_rate":         sett.TaxRate,
		"culture": map[string]any{
			"tradition":  sett.CultureTradition,
			"openness":   sett.CultureOpenness,
			"militarism": sett.CultureMilitarism,
			"memory":     sett.CulturalMemory,
		},
		"infrastructure": map[string]any{
			"wall_level":   sett.WallLevel,
			"road_level":   sett.RoadLevel,
			"market_level": sett.MarketLevel,
		},
		"market":          market,
		"top_agents":      topAgents,
		"faction_presence": factionCounts,
		"recent_events":   recentEvents,
	}
	writeJSON(w, result)
}

func (s *Server) handleFactionDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "missing faction id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		http.Error(w, "invalid faction id", http.StatusBadRequest)
		return
	}

	var faction *social.Faction
	for _, f := range s.Sim.Factions {
		if uint64(f.ID) == id {
			faction = f
			break
		}
	}
	if faction == nil {
		http.Error(w, "faction not found", http.StatusNotFound)
		return
	}

	kindNames := []string{"Political", "Economic", "Military", "Religious", "Criminal"}
	kindName := "Unknown"
	if int(faction.Kind) < len(kindNames) {
		kindName = kindNames[faction.Kind]
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}

	// Member list.
	type memberInfo struct {
		ID         agents.AgentID `json:"id"`
		Name       string         `json:"name"`
		Tier       int            `json:"tier"`
		Occupation string         `json:"occupation"`
	}
	var members []memberInfo
	for _, a := range s.Sim.Agents {
		if a.Alive && a.FactionID != nil && *a.FactionID == id {
			occName := "Unknown"
			if int(a.Occupation) < len(occNames) {
				occName = occNames[a.Occupation]
			}
			members = append(members, memberInfo{
				ID:         a.ID,
				Name:       a.Name,
				Tier:       int(a.Tier),
				Occupation: occName,
			})
		}
	}

	// Top influence settlements.
	type infEntry struct {
		Name      string  `json:"name"`
		Influence float64 `json:"influence"`
	}
	var topInfluence []infEntry
	for settID, inf := range faction.Influence {
		if sett, ok := s.Sim.SettlementIndex[settID]; ok {
			topInfluence = append(topInfluence, infEntry{Name: sett.Name, Influence: inf})
		}
	}
	sort.Slice(topInfluence, func(i, j int) bool { return topInfluence[i].Influence > topInfluence[j].Influence })
	if len(topInfluence) > 10 {
		topInfluence = topInfluence[:10]
	}

	// Relations.
	type relEntry struct {
		Name     string  `json:"name"`
		Relation float64 `json:"relation"`
	}
	var relations []relEntry
	for otherID, rel := range faction.Relations {
		for _, other := range s.Sim.Factions {
			if other.ID == otherID {
				relations = append(relations, relEntry{Name: other.Name, Relation: rel})
				break
			}
		}
	}

	// Recent faction events.
	var recentEvents []engine.Event
	for _, e := range s.Sim.Events {
		if strings.Contains(e.Description, faction.Name) {
			recentEvents = append(recentEvents, e)
		}
	}
	if len(recentEvents) > 20 {
		recentEvents = recentEvents[len(recentEvents)-20:]
	}

	result := map[string]any{
		"id":             faction.ID,
		"name":           faction.Name,
		"kind":           kindName,
		"treasury":       faction.Treasury,
		"members":        members,
		"member_count":   len(members),
		"top_influence":  topInfluence,
		"relations":      relations,
		"policies": map[string]any{
			"tax_preference":      faction.TaxPreference,
			"trade_preference":    faction.TradePreference,
			"military_preference": faction.MilitaryPreference,
		},
		"recent_events": recentEvents,
	}
	writeJSON(w, result)
}

func (s *Server) handleHexDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	// /api/v1/map/:q/:r → parts[0]="" [1]="api" [2]="v1" [3]="map" [4]=q [5]=r
	if len(parts) < 6 {
		http.Error(w, "usage: /api/v1/map/:q/:r", http.StatusBadRequest)
		return
	}
	q, err1 := strconv.Atoi(parts[4])
	rr, err2 := strconv.Atoi(parts[5])
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	coord := world.HexCoord{Q: q, R: rr}
	hex := s.Sim.WorldMap.Get(coord)
	if hex == nil {
		http.Error(w, "hex not found", http.StatusNotFound)
		return
	}

	terrainNames := []string{
		"Plains", "Forest", "Mountain", "Coast", "River",
		"Desert", "Swamp", "Tundra", "Ocean",
	}
	terrainName := "Unknown"
	if int(hex.Terrain) < len(terrainNames) {
		terrainName = terrainNames[hex.Terrain]
	}

	// Resources.
	resNames := map[world.ResourceType]string{
		world.ResourceGrain: "Grain", world.ResourceTimber: "Timber",
		world.ResourceIronOre: "Iron Ore", world.ResourceStone: "Stone",
		world.ResourceFish: "Fish", world.ResourceHerbs: "Herbs",
		world.ResourceGems: "Gems", world.ResourceFurs: "Furs",
		world.ResourceCoal: "Coal", world.ResourceExotics: "Exotics",
	}
	resources := make(map[string]float64)
	for rt, amount := range hex.Resources {
		name := resNames[rt]
		if name == "" {
			name = fmt.Sprintf("Resource#%d", rt)
		}
		resources[name] = amount
	}

	// Settlement on hex.
	var settlement *map[string]any
	if hex.SettlementID != nil {
		if sett, ok := s.Sim.SettlementIndex[*hex.SettlementID]; ok {
			m := map[string]any{
				"id":         sett.ID,
				"name":       sett.Name,
				"population": sett.Population,
			}
			settlement = &m
		}
	}

	// Agents on hex.
	var agentCount int
	type agentBrief struct {
		ID   agents.AgentID `json:"id"`
		Name string         `json:"name"`
	}
	var agentsOnHex []agentBrief
	for _, a := range s.Sim.Agents {
		if a.Alive && a.Position.Q == q && a.Position.R == rr {
			agentCount++
			if len(agentsOnHex) < 20 {
				agentsOnHex = append(agentsOnHex, agentBrief{ID: a.ID, Name: a.Name})
			}
		}
	}

	// Neighbors.
	type neighborInfo struct {
		Q       int    `json:"q"`
		R       int    `json:"r"`
		Terrain string `json:"terrain"`
	}
	var neighbors []neighborInfo
	for _, nc := range coord.Neighbors() {
		nh := s.Sim.WorldMap.Get(nc)
		if nh == nil {
			continue
		}
		tn := "Unknown"
		if int(nh.Terrain) < len(terrainNames) {
			tn = terrainNames[nh.Terrain]
		}
		neighbors = append(neighbors, neighborInfo{Q: nc.Q, R: nc.R, Terrain: tn})
	}

	result := map[string]any{
		"q":           q,
		"r":           rr,
		"terrain":     terrainName,
		"elevation":   hex.Elevation,
		"rainfall":    hex.Rainfall,
		"temperature": hex.Temperature,
		"resources":   resources,
		"settlement":  settlement,
		"agent_count": agentCount,
		"agents":      agentsOnHex,
		"neighbors":   neighbors,
	}
	writeJSON(w, result)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.DB.SaveWorldState(s.Sim); err != nil {
		slog.Error("snapshot save failed", "error", err)
		http.Error(w, "snapshot failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"tick":    s.Sim.CurrentTick(),
		"message": "snapshot saved",
	})
}

func (s *Server) handleIntervention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
		Category    string `json:"category,omitempty"`
		Settlement  string `json:"settlement,omitempty"`
		Amount      int64  `json:"amount,omitempty"`
		Count       int    `json:"count,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	tick := s.Sim.CurrentTick()

	switch req.Type {
	case "event":
		if req.Description == "" {
			http.Error(w, "description required for event type", http.StatusBadRequest)
			return
		}
		cat := req.Category
		if cat == "" {
			cat = "intervention"
		}
		s.Sim.Events = append(s.Sim.Events, engine.Event{
			Tick:        tick,
			Description: req.Description,
			Category:    cat,
		})
		writeJSON(w, map[string]any{"success": true, "details": "event injected"})

	case "wealth":
		if req.Settlement == "" {
			http.Error(w, "settlement required for wealth type", http.StatusBadRequest)
			return
		}
		var found *social.Settlement
		for _, st := range s.Sim.Settlements {
			if st.Name == req.Settlement {
				found = st
				break
			}
		}
		if found == nil {
			http.Error(w, "settlement not found", http.StatusNotFound)
			return
		}
		if req.Amount < 0 && uint64(-req.Amount) > found.Treasury {
			found.Treasury = 0
		} else {
			found.Treasury = uint64(int64(found.Treasury) + req.Amount)
		}
		writeJSON(w, map[string]any{
			"success": true,
			"details": fmt.Sprintf("treasury of %s adjusted by %d (now %d)", found.Name, req.Amount, found.Treasury),
		})

	case "spawn":
		if req.Settlement == "" || req.Count <= 0 {
			http.Error(w, "settlement and count required for spawn type", http.StatusBadRequest)
			return
		}
		if req.Count > 100 {
			http.Error(w, "max 100 agents per spawn", http.StatusBadRequest)
			return
		}
		var found *social.Settlement
		for _, st := range s.Sim.Settlements {
			if st.Name == req.Settlement {
				found = st
				break
			}
		}
		if found == nil {
			http.Error(w, "settlement not found", http.StatusNotFound)
			return
		}
		if s.Sim.Spawner == nil {
			http.Error(w, "spawner not available", http.StatusServiceUnavailable)
			return
		}
		hex := s.Sim.WorldMap.Get(found.Position)
		terrain := world.TerrainPlains
		if hex != nil {
			terrain = hex.Terrain
		}
		immigrants := s.Sim.Spawner.SpawnPopulation(uint32(req.Count), found.Position, found.ID, terrain)
		for _, a := range immigrants {
			a.BornTick = tick
			s.Sim.Agents = append(s.Sim.Agents, a)
			s.Sim.AgentIndex[a.ID] = a
			if a.HomeSettID != nil {
				s.Sim.SettlementAgents[*a.HomeSettID] = append(s.Sim.SettlementAgents[*a.HomeSettID], a)
			}
		}
		found.Population += uint32(req.Count)
		writeJSON(w, map[string]any{
			"success": true,
			"details": fmt.Sprintf("%d immigrants arrived in %s", req.Count, found.Name),
		})

	default:
		http.Error(w, "unknown intervention type (use: event, wealth, spawn)", http.StatusBadRequest)
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}
