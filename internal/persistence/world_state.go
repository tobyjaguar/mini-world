// Persistence registry for world state stored in the world_meta key/value
// table. Each piece of persistent state appears as a single PersistedField
// entry below — co-locating its save and load logic so it's impossible to
// ship one half without the other.
//
// The previous "wire-it-and-pray" pattern (Save block in db.go +
// independent Load block in cmd/worldsim/main.go) silently broke twice in
// project history: R41 forgot to save hex_resources for weeks (artificial
// post-deploy work-rate spikes), and R71 shipped HeatStreakHours as
// transient before R72 retroactively persisted it. Both bugs were invisible
// until a deploy revealed missing state.
//
// SCOPE: this registry owns the LATE persistent fields — the ones loaded
// AFTER `engine.NewSimulation()` is constructed. The four EARLY fields
// (last_tick, season, hex_health, hex_resources) remain as inline calls in
// SaveWorldState and main.go because their load order is interleaved with
// world-map initialization logic that doesn't cleanly factor into the
// registry pattern. New persistent state should default to the registry;
// only add to the early-inline path if load-order genuinely requires it.

package persistence

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/talgya/mini-world/internal/engine"
)

// PersistedField represents a single piece of world-meta-backed state with
// custom save and load logic. Both halves are required at construction; the
// Go compiler enforces co-location.
type PersistedField struct {
	Name string
	// Save reads from sim and writes a string value via db.SaveMeta.
	// Returns nil even when there's nothing meaningful to write (the field
	// stays absent from world_meta until non-empty).
	Save func(*engine.Simulation, *DB) error
	// Load reads a string value via db.GetMeta and applies it to sim. Called
	// only when the meta key exists; safe to leave sim unchanged if the
	// stored value is malformed (logged at warn level).
	Load func(*engine.Simulation, *DB)
}

// persistedFields is the single source of truth for late-restoration world
// state. Add a new entry here to persist new state; you cannot accidentally
// ship a Save without a Load (or vice versa).
var persistedFields = []PersistedField{
	{Name: "non_viable_weeks", Save: saveNonViableWeeks, Load: loadNonViableWeeks},
	{Name: "abandoned_weeks", Save: saveAbandonedWeeks, Load: loadAbandonedWeeks},
	{Name: "settlement_relations", Save: saveSettlementRelations, Load: loadSettlementRelations},
	{Name: "trade_routes", Save: saveTradeRoutes, Load: loadTradeRoutes},
	{Name: "trade_tracker", Save: saveTradeTracker, Load: loadTradeTracker},
	{Name: "agreements", Save: saveAgreements, Load: loadAgreements},
	{Name: "peace_treaties", Save: savePeaceTreaties, Load: loadPeaceTreaties},
	{Name: "raid_counts", Save: saveRaidCounts, Load: loadRaidCounts},
	{Name: "births", Save: saveBirths, Load: loadBirths},
	{Name: "trade_volume", Save: saveTradeVolume, Load: loadTradeVolume},
	{Name: "deaths", Save: saveDeaths, Load: loadDeaths},
	{Name: "heat_streak_hours", Save: saveHeatStreakHours, Load: loadHeatStreakHours},
}

// SaveLatePersisted iterates the registry and saves every late field. Called
// from SaveWorldState after the inline early fields. Returns the first
// non-nil save error (consistent with prior behavior).
func (db *DB) SaveLatePersisted(sim *engine.Simulation) error {
	for _, f := range persistedFields {
		if err := f.Save(sim, db); err != nil {
			return fmt.Errorf("save %s: %w", f.Name, err)
		}
	}
	return nil
}

// RestoreLatePersisted iterates the registry and loads every late field that
// has a value in world_meta. Called once on startup after sim creation. Each
// field's Load handles its own missing-key/malformed-value behavior; one
// field's failure does not block others.
func (db *DB) RestoreLatePersisted(sim *engine.Simulation) {
	for _, f := range persistedFields {
		f.Load(sim, db)
	}
}

// ─── Save closures ────────────────────────────────────────────────────────

func saveNonViableWeeks(sim *engine.Simulation, db *DB) error {
	if len(sim.NonViableWeeks) == 0 {
		return nil
	}
	b, _ := json.Marshal(sim.NonViableWeeks)
	return db.SaveMeta("non_viable_weeks", string(b))
}

func saveAbandonedWeeks(sim *engine.Simulation, db *DB) error {
	if len(sim.AbandonedWeeks) == 0 {
		return nil
	}
	b, _ := json.Marshal(sim.AbandonedWeeks)
	return db.SaveMeta("abandoned_weeks", string(b))
}

type relEntry struct {
	A uint64  `json:"a"`
	B uint64  `json:"b"`
	S float64 `json:"s"` // sentiment
	T float64 `json:"t"` // trade
}

func saveSettlementRelations(sim *engine.Simulation, db *DB) error {
	if len(sim.Relations) == 0 {
		return nil
	}
	rels := make([]relEntry, 0, len(sim.Relations))
	for key, rel := range sim.Relations {
		rels = append(rels, relEntry{A: key.A, B: key.B, S: rel.Sentiment, T: rel.Trade})
	}
	b, _ := json.Marshal(rels)
	if err := db.SaveMeta("settlement_relations", string(b)); err != nil {
		return err
	}
	slog.Info("settlement relations persisted", "pairs", len(rels))
	return nil
}

type routeEntry struct {
	A  uint64  `json:"a"`
	B  uint64  `json:"b"`
	L  uint8   `json:"l"`
	N  string  `json:"n"`
	SW int     `json:"sw"`
	DW int     `json:"dw"`
	WT float64 `json:"wt"`
}

func saveTradeRoutes(sim *engine.Simulation, db *DB) error {
	if len(sim.TradeRoutes) == 0 {
		return nil
	}
	routes := make([]routeEntry, 0, len(sim.TradeRoutes))
	for key, route := range sim.TradeRoutes {
		routes = append(routes, routeEntry{
			A: key.A, B: key.B, L: route.Level, N: route.Name,
			SW: route.SustainedWeeks, DW: route.DormantWeeks, WT: route.WeeklyTrade,
		})
	}
	b, _ := json.Marshal(routes)
	if err := db.SaveMeta("trade_routes", string(b)); err != nil {
		return err
	}
	slog.Info("trade routes persisted", "count", len(routes))
	return nil
}

type ttEntry struct {
	A uint64  `json:"a"`
	B uint64  `json:"b"`
	V float64 `json:"v"`
}

func saveTradeTracker(sim *engine.Simulation, db *DB) error {
	if len(sim.TradeTracker) == 0 {
		return nil
	}
	entries := make([]ttEntry, 0, len(sim.TradeTracker))
	for key, vol := range sim.TradeTracker {
		entries = append(entries, ttEntry{A: key.A, B: key.B, V: vol})
	}
	b, _ := json.Marshal(entries)
	return db.SaveMeta("trade_tracker", string(b))
}

type agreeEntry struct {
	A  uint64 `json:"a"`
	B  uint64 `json:"b"`
	T  uint8  `json:"t"`
	SW int    `json:"sw"`
	FT uint64 `json:"ft"`
}

func saveAgreements(sim *engine.Simulation, db *DB) error {
	if len(sim.Agreements) == 0 {
		return nil
	}
	agrees := make([]agreeEntry, 0, len(sim.Agreements))
	for key, a := range sim.Agreements {
		if a.Type > 0 {
			agrees = append(agrees, agreeEntry{
				A: key.A, B: key.B, T: uint8(a.Type), SW: a.SustainedWeeks, FT: a.FormedAtTick,
			})
		}
	}
	if len(agrees) == 0 {
		return nil
	}
	b, _ := json.Marshal(agrees)
	if err := db.SaveMeta("agreements", string(b)); err != nil {
		return err
	}
	slog.Info("agreements persisted", "count", len(agrees))
	return nil
}

type peaceEntry struct {
	A  uint64 `json:"a"`
	B  uint64 `json:"b"`
	RW int    `json:"rw"`
	RC int    `json:"rc"`
	FT uint64 `json:"ft"`
}

func savePeaceTreaties(sim *engine.Simulation, db *DB) error {
	if len(sim.PeaceTreaties) == 0 {
		return nil
	}
	entries := make([]peaceEntry, 0, len(sim.PeaceTreaties))
	for key, p := range sim.PeaceTreaties {
		entries = append(entries, peaceEntry{
			A: key.A, B: key.B, RW: p.RemainingWeeks, RC: p.RaidCount, FT: p.FormedAtTick,
		})
	}
	b, _ := json.Marshal(entries)
	return db.SaveMeta("peace_treaties", string(b))
}

type raidEntry struct {
	A uint64 `json:"a"`
	B uint64 `json:"b"`
	C int    `json:"c"`
}

func saveRaidCounts(sim *engine.Simulation, db *DB) error {
	if len(sim.RaidCounts) == 0 {
		return nil
	}
	entries := make([]raidEntry, 0, len(sim.RaidCounts))
	for key, c := range sim.RaidCounts {
		if c > 0 {
			entries = append(entries, raidEntry{A: key.A, B: key.B, C: c})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	b, _ := json.Marshal(entries)
	return db.SaveMeta("raid_counts", string(b))
}

func saveBirths(sim *engine.Simulation, db *DB) error {
	return db.SaveMeta("births", fmt.Sprintf("%d", sim.Stats.Births))
}

func saveTradeVolume(sim *engine.Simulation, db *DB) error {
	return db.SaveMeta("trade_volume", fmt.Sprintf("%d", sim.Stats.TradeVolume))
}

func saveDeaths(sim *engine.Simulation, db *DB) error {
	return db.SaveMeta("deaths", fmt.Sprintf("%d", sim.Stats.Deaths))
}

func saveHeatStreakHours(sim *engine.Simulation, db *DB) error {
	return db.SaveMeta("heat_streak_hours", fmt.Sprintf("%d", sim.HeatStreakHours))
}

// ─── Load closures ────────────────────────────────────────────────────────

func loadNonViableWeeks(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("non_viable_weeks")
	if err != nil {
		return
	}
	var nv map[uint64]int
	if json.Unmarshal([]byte(s), &nv) == nil && len(nv) > 0 {
		sim.NonViableWeeks = nv
		slog.Info("non-viable weeks restored", "settlements", len(nv))
	}
}

func loadAbandonedWeeks(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("abandoned_weeks")
	if err != nil {
		return
	}
	var aw map[uint64]int
	if json.Unmarshal([]byte(s), &aw) == nil && len(aw) > 0 {
		sim.AbandonedWeeks = aw
		slog.Info("abandoned weeks restored", "settlements", len(aw))
	}
}

func loadSettlementRelations(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("settlement_relations")
	if err != nil {
		return
	}
	var rels []relEntry
	if json.Unmarshal([]byte(s), &rels) == nil && len(rels) > 0 {
		sim.Relations = make(map[engine.SettRelKey]*engine.SettlementRelation, len(rels))
		for _, r := range rels {
			key := engine.SettRelKey{A: r.A, B: r.B}
			sim.Relations[key] = &engine.SettlementRelation{Sentiment: r.S, Trade: r.T}
		}
		slog.Info("settlement relations restored", "pairs", len(rels))
	}
}

func loadTradeRoutes(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("trade_routes")
	if err != nil {
		return
	}
	var routes []routeEntry
	if json.Unmarshal([]byte(s), &routes) == nil && len(routes) > 0 {
		sim.TradeRoutes = make(map[engine.SettRelKey]*engine.TradeRoute, len(routes))
		for _, r := range routes {
			key := engine.SettRelKey{A: r.A, B: r.B}
			sim.TradeRoutes[key] = &engine.TradeRoute{
				Level: r.L, Name: r.N, SustainedWeeks: r.SW,
				DormantWeeks: r.DW, WeeklyTrade: r.WT,
			}
		}
		slog.Info("trade routes restored", "count", len(routes))
	}
}

func loadTradeTracker(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("trade_tracker")
	if err != nil {
		return
	}
	var entries []ttEntry
	if json.Unmarshal([]byte(s), &entries) == nil && len(entries) > 0 {
		sim.TradeTracker = make(map[engine.SettRelKey]float64, len(entries))
		for _, e := range entries {
			key := engine.SettRelKey{A: e.A, B: e.B}
			sim.TradeTracker[key] = e.V
		}
		slog.Info("trade tracker restored", "pairs", len(entries))
	}
}

func loadAgreements(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("agreements")
	if err != nil {
		return
	}
	var agrees []agreeEntry
	if json.Unmarshal([]byte(s), &agrees) == nil && len(agrees) > 0 {
		sim.Agreements = make(map[engine.SettRelKey]*engine.Agreement, len(agrees))
		for _, a := range agrees {
			key := engine.SettRelKey{A: a.A, B: a.B}
			sim.Agreements[key] = &engine.Agreement{
				Type: engine.AgreementType(a.T), SustainedWeeks: a.SW, FormedAtTick: a.FT,
			}
		}
		slog.Info("agreements restored", "count", len(agrees))
	}
}

func loadPeaceTreaties(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("peace_treaties")
	if err != nil {
		return
	}
	var entries []peaceEntry
	if json.Unmarshal([]byte(s), &entries) == nil && len(entries) > 0 {
		sim.PeaceTreaties = make(map[engine.SettRelKey]*engine.PeaceTreaty, len(entries))
		for _, e := range entries {
			key := engine.SettRelKey{A: e.A, B: e.B}
			sim.PeaceTreaties[key] = &engine.PeaceTreaty{
				RemainingWeeks: e.RW, RaidCount: e.RC, FormedAtTick: e.FT,
			}
		}
		slog.Info("peace treaties restored", "count", len(entries))
	}
}

func loadRaidCounts(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("raid_counts")
	if err != nil {
		return
	}
	var entries []raidEntry
	if json.Unmarshal([]byte(s), &entries) == nil && len(entries) > 0 {
		sim.RaidCounts = make(map[engine.SettRelKey]int, len(entries))
		for _, e := range entries {
			key := engine.SettRelKey{A: e.A, B: e.B}
			sim.RaidCounts[key] = e.C
		}
		slog.Info("raid counts restored", "count", len(entries))
	}
}

func loadBirths(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("births")
	if err != nil {
		return
	}
	if v, err := strconv.Atoi(s); err == nil {
		sim.Stats.Births = v
		slog.Info("births counter restored", "births", v)
	}
}

func loadTradeVolume(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("trade_volume")
	if err != nil {
		return
	}
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		sim.Stats.TradeVolume = v
		slog.Info("trade volume counter restored", "volume", v)
	}
}

func loadDeaths(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("deaths")
	if err != nil {
		return
	}
	if v, err := strconv.Atoi(s); err == nil {
		sim.Stats.Deaths = v
		slog.Info("deaths counter restored", "deaths", v)
	}
}

func loadHeatStreakHours(sim *engine.Simulation, db *DB) {
	s, err := db.GetMeta("heat_streak_hours")
	if err != nil {
		return
	}
	if v, err := strconv.Atoi(s); err == nil {
		sim.HeatStreakHours = v
		slog.Info("heat streak counter restored", "hours", v)
	}
}
