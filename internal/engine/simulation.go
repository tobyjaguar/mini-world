// Simulation ties together all world systems and runs them each tick.
package engine

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/entropy"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/weather"
	"github.com/talgya/mini-world/internal/world"
)

// Simulation holds the complete world state and wires systems together.
type Simulation struct {
	WorldMap    *world.Map
	Agents      []*agents.Agent
	AgentIndex  map[agents.AgentID]*agents.Agent
	Settlements []*social.Settlement
	Events      []Event // Recent events (ring buffer in production)
	LastTick    uint64  // Most recent tick processed

	// Settlement lookups.
	SettlementIndex  map[uint64]*social.Settlement   // ID → settlement
	SettlementAgents map[uint64][]*agents.Agent       // settlement ID → agents

	// Agent spawner for births and immigration.
	Spawner *agents.Spawner

	// Faction system.
	Factions []*social.Faction

	// Season tracking (0=Spring, 1=Summer, 2=Autumn, 3=Winter).
	CurrentSeason uint8

	// LLM client for Tier 2 cognition and narration.
	LLM *llm.Client

	// Weather system.
	WeatherClient  *weather.Client
	CurrentWeather SimWeather

	// Entropy source (random.org or crypto/rand fallback).
	Entropy *entropy.Client

	// Settlement abandonment tracking (settlement ID → consecutive weeks with 0 pop).
	AbandonedWeeks map[uint64]int

	// Non-viable settlement tracking (settlement ID → consecutive weeks with pop < 15).
	// After 4 weeks, refugee spawning is disabled so the settlement can naturally decline.
	NonViableWeeks map[uint64]int

	// Active production boosts from gardener "cultivate" interventions.
	ActiveBoosts []ProductionBoost

	// Event streaming support.
	eventSubMu sync.RWMutex
	eventSubs  map[int]chan Event
	nextSubID  int

	// Statistics tracked per day.
	Stats SimStats
}

// CurrentTick returns the most recently processed tick number.
func (s *Simulation) CurrentTick() uint64 {
	return s.LastTick
}

// Subscribe returns a subscriber ID and a buffered channel that receives events.
func (s *Simulation) Subscribe() (int, chan Event) {
	s.eventSubMu.Lock()
	defer s.eventSubMu.Unlock()
	if s.eventSubs == nil {
		s.eventSubs = make(map[int]chan Event)
	}
	id := s.nextSubID
	s.nextSubID++
	ch := make(chan Event, 64)
	s.eventSubs[id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (s *Simulation) Unsubscribe(id int) {
	s.eventSubMu.Lock()
	defer s.eventSubMu.Unlock()
	if ch, ok := s.eventSubs[id]; ok {
		close(ch)
		delete(s.eventSubs, id)
	}
}

// EmitEvent appends an event to the stored slice and broadcasts to all subscribers.
func (s *Simulation) EmitEvent(e Event) {
	s.Events = append(s.Events, e)
	s.eventSubMu.RLock()
	defer s.eventSubMu.RUnlock()
	for _, ch := range s.eventSubs {
		select {
		case ch <- e:
		default:
			// Subscriber buffer full — drop event for slow consumers.
		}
	}
}

// Event is a notable occurrence in the world.
type Event struct {
	Tick                 uint64 `json:"tick" db:"tick"`
	Description          string `json:"description" db:"description"`
	NarratedDescription  string `json:"narrated_description,omitempty" db:"narrated"` // LLM-narrated prose (major events only)
	Category             string `json:"category" db:"category"` // "economy", "social", "death", "birth", "political", "disaster", "discovery", etc.
}

// SimWeather holds the current weather conditions for the simulation.
type SimWeather struct {
	TempModifier float32 `json:"temp_modifier"` // -1 cold to +1 hot
	FoodDecayMod float32 `json:"food_decay_mod"` // Multiplier on food spoilage
	TravelPenalty float32 `json:"travel_penalty"` // Multiplier on travel time
	Description  string  `json:"description"`
}

// SimStats tracks aggregate world statistics.
type SimStats struct {
	TotalPopulation int     `json:"total_population"`
	TotalWealth     uint64  `json:"total_wealth"`
	Deaths          int     `json:"deaths"`
	Births          int     `json:"births"`
	AvgMood         float32 `json:"avg_mood"`         // Effective mood (blended)
	AvgSatisfaction float32 `json:"avg_satisfaction"`  // Material satisfaction
	AvgAlignment    float32 `json:"avg_alignment"`     // Coherence-derived alignment
	AvgSurvival     float32 `json:"avg_survival"`
	TradeVolume     uint64  `json:"trade_volume"` // Merchant trade completions
}

// NewSimulation creates a Simulation from generated components.
func NewSimulation(m *world.Map, ag []*agents.Agent, setts []*social.Settlement) *Simulation {
	index := make(map[agents.AgentID]*agents.Agent, len(ag))
	for _, a := range ag {
		index[a.ID] = a
	}

	// Build settlement index and initialize markets.
	settIndex := make(map[uint64]*social.Settlement, len(setts))
	for _, s := range setts {
		settIndex[s.ID] = s
		s.Market = economy.NewMarket(s.ID)
	}

	// Build reverse index: settlement ID → agents.
	settAgents := make(map[uint64][]*agents.Agent)
	for _, a := range ag {
		if a.HomeSettID != nil {
			settAgents[*a.HomeSettID] = append(settAgents[*a.HomeSettID], a)
		}
	}

	sim := &Simulation{
		WorldMap:         m,
		Agents:           ag,
		AgentIndex:       index,
		Settlements:      setts,
		SettlementIndex:  settIndex,
		SettlementAgents: settAgents,
		AbandonedWeeks:   make(map[uint64]int),
		NonViableWeeks:   make(map[uint64]int),
	}
	sim.updateStats()
	return sim
}

// TickMinute runs every tick (1 sim-minute): agent decisions and need decay.
func (s *Simulation) TickMinute(tick uint64) {
	s.LastTick = tick
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		// Decay needs (passage of time).
		agents.DecayNeeds(a)

		// Agent decides and acts.
		action := agents.Decide(a)

		var events []string

		// Buy food: agent purchases from settlement market (direct transaction).
		if action.Kind == agents.ActionBuyFood {
			s.resolveBuyFood(a)
		} else {
			// Resource-producing occupations draw from hex resources.
			hex := s.WorldMap.Get(a.Position)
			boostMul := 1.0
			if a.HomeSettID != nil {
				boostMul = s.GetSettlementBoost(*a.HomeSettID)
			}
			events = ResolveWork(a, action, hex, tick, boostMul)
		}

		// Record notable events.
		for _, desc := range events {
			s.EmitEvent(Event{
				Tick:        tick,
				Description: desc,
				Category:    "agent",
			})
		}

		// Check for death (starvation).
		if !a.Alive {
			deathDesc := fmt.Sprintf("%s has died", a.Name)
			s.EmitEvent(Event{
				Tick:        tick,
				Description: deathDesc,
				Category:    "death",
			})
			s.inheritWealth(a, tick)

			if a.HomeSettID != nil {
				// Create memories for nearby Tier 2 agents.
				s.createSettlementMemories(*a.HomeSettID, tick, deathDesc, 0.6)

				// Via negativa: witnessing death strips attachment, increasing coherence.
				// "Loss removes dilution" — Wheeler's subtraction principle.
				for _, witness := range s.SettlementAgents[*a.HomeSettID] {
					if witness.Alive && witness.ID != a.ID {
						witness.Soul.AdjustCoherence(float32(phi.Agnosis * 0.05))
					}
				}
			}
		}
	}
}

// TickHour runs every sim-hour: market updates, weather checks.
func (s *Simulation) TickHour(tick uint64) {
	s.resolveMarkets(tick)
	s.resolveMerchantTrade(tick)
	s.decayInventories()
	s.updateWeather()
}

// updateWeather fetches real weather and maps it to simulation modifiers.
func (s *Simulation) updateWeather() {
	if s.WeatherClient == nil {
		// Use seasonal defaults.
		sw := weather.MapToSim(nil, s.CurrentSeason)
		s.CurrentWeather = SimWeather{
			TempModifier:  sw.TempModifier,
			FoodDecayMod:  sw.FoodDecayMod,
			TravelPenalty: sw.TravelPenalty,
			Description:   sw.Description,
		}
		return
	}

	conditions, err := s.WeatherClient.Fetch()
	if err != nil {
		slog.Debug("weather fetch failed", "error", err)
		return
	}

	sw := weather.MapToSim(conditions, s.CurrentSeason)
	s.CurrentWeather = SimWeather{
		TempModifier:  sw.TempModifier,
		FoodDecayMod:  sw.FoodDecayMod,
		TravelPenalty: sw.TravelPenalty,
		Description:   sw.Description,
	}
}

// TickDay runs every sim-day: statistics, daily summary.
func (s *Simulation) TickDay(tick uint64) {
	s.CleanExpiredBoosts(tick)
	s.collectTaxes(tick)
	s.decayWealth()
	s.paySettlementWages()
	s.processPopulation(tick)
	s.processRelationships(tick)
	s.processCrime(tick)
	s.processTier1Growth()
	s.processBaselineCoherence()
	s.processGovernance(tick)
	s.processTier2Decisions(tick)
	s.updateStats()

	// Count events by category since last report.
	eventCounts := make(map[string]int)
	for _, e := range s.Events {
		eventCounts[e.Category]++
	}

	slog.Info("daily report",
		"tick", tick,
		"time", SimTime(tick),
		"alive", s.Stats.TotalPopulation,
		"deaths", s.Stats.Deaths,
		"births", s.Stats.Births,
		"avg_mood", fmt.Sprintf("%.3f", s.Stats.AvgMood),
		"avg_survival", fmt.Sprintf("%.3f", s.Stats.AvgSurvival),
		"total_wealth", s.Stats.TotalWealth,
		"weather", s.CurrentWeather.Description,
		"events_death", eventCounts["death"],
		"events_birth", eventCounts["birth"],
		"events_crime", eventCounts["crime"],
		"events_social", eventCounts["social"],
		"events_economy", eventCounts["economy"],
	)

	// Log recent notable events (deaths, crimes, social).
	recentStart := 0
	if len(s.Events) > 20 {
		recentStart = len(s.Events) - 20
	}
	for _, e := range s.Events[recentStart:] {
		if e.Category == "death" || e.Category == "crime" || e.Category == "social" {
			slog.Info("event", "category", e.Category, "description", e.Description)
		}
	}
}

// TickWeek runs every sim-week: faction updates, diplomatic cycles, LLM updates.
func (s *Simulation) TickWeek(tick uint64) {
	s.processWeeklyFactions(tick)
	s.processAntiStagnation(tick)
	s.weeklyResourceRegen()
	s.processSeasonalMigration(tick)
	s.processViabilityCheck(tick)
	s.processInfrastructureGrowth(tick)
	s.processSettlementOvermass(tick)
	s.processSettlementAbandonment(tick)
	s.processWeeklyTier2Replenishment()
	s.updateArchetypeTemplates(tick)
	s.processOracleVisions(tick)
	s.processRandomEvents(tick)
	s.narrateRecentMajorEvents(tick)

	slog.Info("weekly summary",
		"tick", tick,
		"time", SimTime(tick),
		"events_this_week", len(s.Events),
	)
	// Trim old events to prevent unbounded growth (keep last 1000).
	if len(s.Events) > 1000 {
		s.Events = s.Events[len(s.Events)-1000:]
	}
}

// TickSeason runs every sim-season: harvests, seasonal effects.
func (s *Simulation) TickSeason(tick uint64) {
	s.processSeason(tick)
}

// inheritWealth distributes a dead agent's wealth and inventory.
// 50% of wealth goes to the settlement treasury, 50% to a living settlement member.
// Inventory goods are added to the settlement market supply.
func (s *Simulation) inheritWealth(a *agents.Agent, tick uint64) {
	if a.Wealth == 0 && len(a.Inventory) == 0 {
		return
	}

	var sett *social.Settlement
	if a.HomeSettID != nil {
		sett = s.SettlementIndex[*a.HomeSettID]
	}

	if sett != nil {
		// Split wealth: 50% to treasury, 50% to a living agent.
		treasuryShare := a.Wealth / 2
		agentShare := a.Wealth - treasuryShare
		sett.Treasury += treasuryShare

		// Find a living agent in the same settlement to inherit.
		settAgents := s.SettlementAgents[sett.ID]
		for _, heir := range settAgents {
			if heir.Alive && heir.ID != a.ID {
				heir.Wealth += agentShare
				agentShare = 0
				break
			}
		}
		// If no heir found, treasury gets everything.
		if agentShare > 0 {
			sett.Treasury += agentShare
		}

		// Inventory goods go to settlement market supply.
		if sett.Market != nil {
			for good, qty := range a.Inventory {
				if qty > 0 {
					if entry, ok := sett.Market.Entries[good]; ok {
						entry.Supply += float64(qty)
					}
				}
			}
		}
	}

	// Zero out the dead agent's wealth and inventory.
	a.Wealth = 0
	for good := range a.Inventory {
		a.Inventory[good] = 0
	}
}

// processRandomEvents uses true randomness to trigger rare world events.
func (s *Simulation) processRandomEvents(tick uint64) {
	randFloat := func() float64 {
		return entropy.FloatFromSource(s.Entropy)
	}

	// Natural disaster: 2% chance per week.
	if randFloat() < 0.02 && len(s.Settlements) > 0 {
		idx := int(randFloat() * float64(len(s.Settlements)))
		if idx >= len(s.Settlements) {
			idx = len(s.Settlements) - 1
		}
		sett := s.Settlements[idx]

		disasters := []string{"a fierce storm", "an earthquake", "a flood"}
		dIdx := int(randFloat() * float64(len(disasters)))
		if dIdx >= len(disasters) {
			dIdx = 0
		}

		// Damage: reduce treasury and health.
		damage := sett.Treasury / 5
		sett.Treasury -= damage
		for _, a := range s.SettlementAgents[sett.ID] {
			if a.Alive {
				a.Health -= 0.1
				a.Wellbeing.Satisfaction -= 0.2
				if a.Health < 0 {
					a.Health = 0
				}
			}
		}

		desc := fmt.Sprintf("%s strikes %s! Treasury loses %d crowns", disasters[dIdx], sett.Name, damage)
		s.EmitEvent(Event{
			Tick:        tick,
			Description: desc,
			Category:    "disaster",
		})
		s.createSettlementMemories(sett.ID, tick, desc, 0.9)
		slog.Info("random event: disaster", "settlement", sett.Name, "type", disasters[dIdx])
	}

	// Discovery: 5% chance per week.
	if randFloat() < 0.05 && len(s.Settlements) > 0 {
		idx := int(randFloat() * float64(len(s.Settlements)))
		if idx >= len(s.Settlements) {
			idx = len(s.Settlements) - 1
		}
		sett := s.Settlements[idx]

		discoveries := []string{
			"a rich mineral deposit", "ancient ruins", "a medicinal spring",
			"a hidden trade route", "a vein of precious gems",
		}
		dIdx := int(randFloat() * float64(len(discoveries)))
		if dIdx >= len(discoveries) {
			dIdx = 0
		}

		// Benefit: add resources to the hex and boost treasury.
		bonus := uint64(50 + int(randFloat()*100))
		sett.Treasury += bonus

		hex := s.WorldMap.Get(sett.Position)
		var desc string
		switch discoveries[dIdx] {
		case "a rich mineral deposit":
			amount := 50.0 + randFloat()*50.0
			if hex != nil {
				if hex.Resources == nil {
					hex.Resources = make(map[world.ResourceType]float64)
				}
				hex.Resources[world.ResourceStone] += amount
				hex.Resources[world.ResourceIronOre] += amount * 0.6
			}
			desc = fmt.Sprintf("Discovery near %s: %s found! +%.0f stone, +%.0f iron ore, treasury gains %d crowns",
				sett.Name, discoveries[dIdx], amount, amount*0.6, bonus)
		case "a medicinal spring":
			amount := 20.0 + randFloat()*30.0
			if hex != nil {
				if hex.Resources == nil {
					hex.Resources = make(map[world.ResourceType]float64)
				}
				hex.Resources[world.ResourceHerbs] += amount
			}
			desc = fmt.Sprintf("Discovery near %s: %s found! +%.0f herbs, treasury gains %d crowns",
				sett.Name, discoveries[dIdx], amount, bonus)
		case "a vein of precious gems":
			amount := 30.0 + randFloat()*30.0
			if hex != nil {
				if hex.Resources == nil {
					hex.Resources = make(map[world.ResourceType]float64)
				}
				hex.Resources[world.ResourceGems] += amount
			}
			desc = fmt.Sprintf("Discovery near %s: %s found! +%.0f gems, treasury gains %d crowns",
				sett.Name, discoveries[dIdx], amount, bonus)
		default:
			desc = fmt.Sprintf("Discovery near %s: %s found! Treasury gains %d crowns", sett.Name, discoveries[dIdx], bonus)
		}

		s.EmitEvent(Event{
			Tick:        tick,
			Description: desc,
			Category:    "discovery",
		})
		s.createSettlementMemories(sett.ID, tick, desc, 0.7)
		slog.Info("random event: discovery", "settlement", sett.Name, "type", discoveries[dIdx])
	}

	// Alchemical breakthrough: 3% chance per week.
	if randFloat() < 0.03 {
		// Find an alchemist to credit.
		var alchemist *agents.Agent
		for _, a := range s.Agents {
			if a.Alive && a.Occupation == agents.OccupationAlchemist {
				alchemist = a
				break
			}
		}
		if alchemist != nil {
			alchemist.Soul.AdjustCoherence(0.05)
			alchemist.Inventory[agents.GoodExotics] += 5
			desc := fmt.Sprintf("Alchemical breakthrough: %s discovers a new transmutation technique", alchemist.Name)
			s.EmitEvent(Event{
				Tick:        tick,
				Description: desc,
				Category:    "discovery",
			})
			if alchemist.Tier == agents.Tier2 {
				agents.AddMemory(alchemist, tick, desc, 0.8)
			}
		}
	}
}

// narrateRecentMajorEvents narrates major events (disasters, discoveries, political) via LLM.
// Caps at 5 narrations per call to stay within the call budget.
func (s *Simulation) narrateRecentMajorEvents(tick uint64) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return
	}

	worldContext := fmt.Sprintf("Season: %s. Population: %d. Weather: %s.",
		SeasonName(s.CurrentSeason), s.Stats.TotalPopulation, s.CurrentWeather.Description)

	narrated := 0
	for i := len(s.Events) - 1; i >= 0 && narrated < 5; i-- {
		e := &s.Events[i]
		if e.Tick != tick || e.NarratedDescription != "" {
			continue
		}

		// Only narrate major event categories.
		switch e.Category {
		case "disaster", "discovery", "political":
			text, err := llm.NarrateEvent(s.LLM, e.Description, worldContext)
			if err != nil {
				slog.Debug("narration failed", "error", err)
				continue
			}
			e.NarratedDescription = text
			narrated++
		}
	}
}

// updateArchetypeTemplates uses LLM to regenerate behavioral templates weekly.
func (s *Simulation) updateArchetypeTemplates(tick uint64) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return
	}

	worldSummary := fmt.Sprintf(
		"Season: %s. Population: %d. Average mood: %.2f. Average survival: %.2f. Total wealth: %d crowns. Settlements: %d.",
		SeasonName(s.CurrentSeason), s.Stats.TotalPopulation,
		s.Stats.AvgMood, s.Stats.AvgSurvival, s.Stats.TotalWealth, len(s.Settlements),
	)

	for _, archetype := range agents.AllArchetypes() {
		update, err := llm.GenerateArchetypeUpdate(s.LLM, archetype, worldSummary)
		if err != nil {
			slog.Debug("archetype update failed, keeping defaults", "archetype", archetype, "error", err)
			continue
		}
		agents.UpdateArchetypeTemplate(archetype, update.PriorityShifts, update.PreferredAction)
		slog.Debug("archetype updated", "archetype", archetype, "motto", update.Motto)
	}
}

// createSettlementMemories adds a memory to all Tier 2 agents in a settlement.
func (s *Simulation) createSettlementMemories(settID uint64, tick uint64, content string, importance float32) {
	for _, a := range s.SettlementAgents[settID] {
		if a.Alive && a.Tier == agents.Tier2 {
			agents.AddMemory(a, tick, content, importance)
		}
	}
}

// processTier1Growth applies daily coherence growth for Tier 1 agents.
func (s *Simulation) processTier1Growth() {
	for _, a := range s.Agents {
		agents.ApplyTier1CoherenceGrowth(a)
	}
}

// processBaselineCoherence gives all agents a tiny coherence drift when
// their lives are stable. "Rest seeks rest" — a stable life naturally resolves
// toward slightly less scatter. Over a full lifespan, some agents drift from
// deep Embodied toward the upper edge. See Wheeler: attachments naturally
// fall away as you age.
func (s *Simulation) processBaselineCoherence() {
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		satisfaction := a.Needs.OverallSatisfaction()
		if satisfaction > 0.7 && a.Age > 30 {
			a.Soul.AdjustCoherence(float32(phi.Agnosis * 0.001))
		}
	}
}

// GiniCoefficient computes wealth inequality from agent wealth distribution.
// Uses sorted formula: G = (2*Σ(i*wᵢ))/(n*Σwᵢ) - (n+1)/n
func (s *Simulation) GiniCoefficient() float64 {
	var wealths []uint64
	for _, a := range s.Agents {
		if a.Alive {
			wealths = append(wealths, a.Wealth)
		}
	}
	n := len(wealths)
	if n < 2 {
		return 0
	}
	sort.Slice(wealths, func(i, j int) bool { return wealths[i] < wealths[j] })
	totalWealth := uint64(0)
	weightedSum := uint64(0)
	for i, w := range wealths {
		totalWealth += w
		weightedSum += uint64(i+1) * w
	}
	if totalWealth == 0 {
		return 0
	}
	return (2.0*float64(weightedSum))/(float64(n)*float64(totalWealth)) - float64(n+1)/float64(n)
}

// AvgCoherence computes average citta coherence of living agents.
func (s *Simulation) AvgCoherence() float64 {
	total := float64(0)
	count := 0
	for _, a := range s.Agents {
		if a.Alive {
			total += float64(a.Soul.CittaCoherence)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func (s *Simulation) updateStats() {
	alive := 0
	totalWealth := uint64(0)
	totalMood := float32(0)
	totalSatisfaction := float32(0)
	totalAlignment := float32(0)
	totalSurvival := float32(0)
	deaths := 0

	for _, a := range s.Agents {
		if a.Alive {
			alive++
			totalWealth += a.Wealth
			totalMood += a.Wellbeing.EffectiveMood
			totalSatisfaction += a.Wellbeing.Satisfaction
			totalAlignment += a.Wellbeing.Alignment
			totalSurvival += a.Needs.Survival
		} else {
			deaths++
		}
	}

	s.Stats.TotalPopulation = alive
	s.Stats.TotalWealth = totalWealth
	s.Stats.Deaths = deaths
	if alive > 0 {
		s.Stats.AvgMood = totalMood / float32(alive)
		s.Stats.AvgSatisfaction = totalSatisfaction / float32(alive)
		s.Stats.AvgAlignment = totalAlignment / float32(alive)
		s.Stats.AvgSurvival = totalSurvival / float32(alive)
	}
}

// SettlementCarryingCapacity computes the carrying capacity for a settlement
// based on the health-weighted resource cap of its hex and neighboring hexes.
func (s *Simulation) SettlementCarryingCapacity(settID uint64) (capacity float64, pressure float64) {
	sett, ok := s.SettlementIndex[settID]
	if !ok {
		return 0, 0
	}

	hex := s.WorldMap.Get(sett.Position)
	if hex == nil {
		return 0, 0
	}

	// Sum health-weighted resource caps for settlement hex and neighbors.
	addHexCapacity := func(h *world.Hex) {
		for res, _ := range h.Resources {
			cap := ResourceCap(h.Terrain, res)
			capacity += cap * h.Health
		}
	}

	addHexCapacity(hex)
	for _, nc := range sett.Position.Neighbors() {
		nh := s.WorldMap.Get(nc)
		if nh != nil && nh.Terrain != world.TerrainOcean {
			addHexCapacity(nh)
		}
	}

	if capacity > 0 {
		pressure = float64(sett.Population) / capacity
	}
	return capacity, pressure
}

// rebuildSettlementAgents reconstructs the settlement→agents map from agent HomeSettIDs.
// Must be called after any operation that changes agent HomeSettID (e.g. migration, diaspora).
func (s *Simulation) rebuildSettlementAgents() {
	newMap := make(map[uint64][]*agents.Agent, len(s.Settlements))
	for _, a := range s.Agents {
		if a.HomeSettID != nil {
			newMap[*a.HomeSettID] = append(newMap[*a.HomeSettID], a)
		}
	}
	s.SettlementAgents = newMap
	s.updateSettlementPopulations()
}
