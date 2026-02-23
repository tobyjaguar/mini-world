// Package persistence provides SQLite-based world state storage.
// See design doc Section 8.3.
package persistence

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/engine"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// DB wraps a SQLite connection for world state persistence.
type DB struct {
	conn *sqlx.DB
}

// Open opens or creates a SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sqlx.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER NOT NULL,
		sex INTEGER NOT NULL,
		health REAL NOT NULL,
		pos_q INTEGER NOT NULL,
		pos_r INTEGER NOT NULL,
		home_settlement_id INTEGER,
		occupation INTEGER NOT NULL,
		wealth INTEGER NOT NULL,
		tier INTEGER NOT NULL,
		mood REAL NOT NULL,
		alive INTEGER NOT NULL,
		born_tick INTEGER NOT NULL,
		role INTEGER NOT NULL,
		faction_id INTEGER,
		archetype TEXT,
		skills_json TEXT NOT NULL,
		needs_json TEXT NOT NULL,
		soul_json TEXT NOT NULL,
		inventory_json TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settlements (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		pos_q INTEGER NOT NULL,
		pos_r INTEGER NOT NULL,
		population INTEGER NOT NULL,
		governance INTEGER NOT NULL,
		tax_rate REAL NOT NULL,
		treasury INTEGER NOT NULL,
		governance_score REAL NOT NULL,
		cultural_memory REAL NOT NULL,
		culture_tradition REAL NOT NULL,
		culture_openness REAL NOT NULL,
		culture_militarism REAL NOT NULL,
		wall_level INTEGER NOT NULL,
		road_level INTEGER NOT NULL,
		market_level INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tick INTEGER NOT NULL,
		description TEXT NOT NULL,
		category TEXT NOT NULL,
		narrated TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS world_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS memories (
		agent_id INTEGER NOT NULL,
		tick INTEGER NOT NULL,
		content TEXT NOT NULL,
		importance REAL NOT NULL
	);

	CREATE TABLE IF NOT EXISTS stats_history (
		tick INTEGER PRIMARY KEY,
		population INTEGER NOT NULL,
		total_wealth INTEGER NOT NULL,
		avg_mood REAL NOT NULL,
		avg_survival REAL NOT NULL,
		births INTEGER NOT NULL,
		deaths INTEGER NOT NULL,
		trade_volume INTEGER NOT NULL,
		avg_coherence REAL NOT NULL,
		settlement_count INTEGER NOT NULL,
		gini REAL NOT NULL
	);

	CREATE TABLE IF NOT EXISTS relationships (
		agent_id INTEGER NOT NULL,
		target_id INTEGER NOT NULL,
		sentiment REAL NOT NULL,
		trust REAL NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_tick ON events(tick);
	CREATE INDEX IF NOT EXISTS idx_agents_settlement ON agents(home_settlement_id);
	CREATE INDEX IF NOT EXISTS idx_agents_alive ON agents(alive);
	CREATE INDEX IF NOT EXISTS idx_memories_agent ON memories(agent_id);
	CREATE INDEX IF NOT EXISTS idx_relationships_agent ON relationships(agent_id);
	`
	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Factions table (added in Phase 5 tuning).
	_, err = db.conn.Exec(`
	CREATE TABLE IF NOT EXISTS factions (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		kind INTEGER NOT NULL,
		leader_id INTEGER,
		treasury INTEGER NOT NULL,
		tax_preference REAL NOT NULL,
		trade_preference REAL NOT NULL,
		military_preference REAL NOT NULL,
		influence_json TEXT NOT NULL,
		relations_json TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	// Add columns that may not exist in older databases.
	migrations := []string{
		"ALTER TABLE events ADD COLUMN narrated TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE settlements ADD COLUMN abandoned INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN satisfaction REAL NOT NULL DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN alignment REAL NOT NULL DEFAULT 0",
		"ALTER TABLE stats_history ADD COLUMN avg_satisfaction REAL NOT NULL DEFAULT 0",
		"ALTER TABLE stats_history ADD COLUMN avg_alignment REAL NOT NULL DEFAULT 0",
	}
	for _, m := range migrations {
		db.conn.Exec(m) // Ignore errors — column may already exist.
	}

	return nil
}

// SaveAgents writes all agents to the database (full replace).
func (db *DB) SaveAgents(agentList []*agents.Agent) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM agents"); err != nil {
		return err
	}

	stmt, err := tx.Preparex(`INSERT INTO agents
		(id, name, age, sex, health, pos_q, pos_r, home_settlement_id,
		 occupation, wealth, tier, mood, alive, born_tick, role, faction_id, archetype,
		 skills_json, needs_json, soul_json, inventory_json, satisfaction, alignment)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range agentList {
		skillsJSON, _ := json.Marshal(a.Skills)
		needsJSON, _ := json.Marshal(a.Needs)
		soulJSON, _ := json.Marshal(a.Soul)
		invJSON, _ := json.Marshal(a.Inventory)

		alive := 0
		if a.Alive {
			alive = 1
		}

		_, err := stmt.Exec(
			a.ID, a.Name, a.Age, a.Sex, a.Health,
			a.Position.Q, a.Position.R, a.HomeSettID,
			a.Occupation, a.Wealth, a.Tier, a.Wellbeing.EffectiveMood,
			alive, a.BornTick, a.Role, a.FactionID, a.Archetype,
			string(skillsJSON), string(needsJSON), string(soulJSON), string(invJSON),
			a.Wellbeing.Satisfaction, a.Wellbeing.Alignment,
		)
		if err != nil {
			return fmt.Errorf("insert agent %d: %w", a.ID, err)
		}
	}

	return tx.Commit()
}

// SaveSettlements writes all settlements to the database.
func (db *DB) SaveSettlements(settlements []*social.Settlement) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM settlements"); err != nil {
		return err
	}

	for _, s := range settlements {
		_, err := tx.Exec(`INSERT INTO settlements
			(id, name, pos_q, pos_r, population, governance, tax_rate, treasury,
			 governance_score, cultural_memory, culture_tradition, culture_openness,
			 culture_militarism, wall_level, road_level, market_level)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.Name, s.Position.Q, s.Position.R, s.Population,
			s.Governance, s.TaxRate, s.Treasury, s.GovernanceScore,
			s.CulturalMemory, s.CultureTradition, s.CultureOpenness,
			s.CultureMilitarism, s.WallLevel, s.RoadLevel, s.MarketLevel,
		)
		if err != nil {
			return fmt.Errorf("insert settlement %d: %w", s.ID, err)
		}
	}

	return tx.Commit()
}

// SaveFactions writes all factions to the database (full replace).
func (db *DB) SaveFactions(factions []*social.Faction) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM factions"); err != nil {
		return err
	}

	for _, f := range factions {
		influenceJSON, _ := json.Marshal(f.Influence)
		relationsJSON, _ := json.Marshal(f.Relations)

		_, err := tx.Exec(`INSERT INTO factions
			(id, name, kind, leader_id, treasury,
			 tax_preference, trade_preference, military_preference,
			 influence_json, relations_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID, f.Name, f.Kind, f.LeaderID, f.Treasury,
			f.TaxPreference, f.TradePreference, f.MilitaryPreference,
			string(influenceJSON), string(relationsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert faction %d: %w", f.ID, err)
		}
	}

	return tx.Commit()
}

// LoadFactions reads all factions from the database.
func (db *DB) LoadFactions() ([]*social.Faction, error) {
	type factionRow struct {
		ID                 uint64  `db:"id"`
		Name               string  `db:"name"`
		Kind               uint8   `db:"kind"`
		LeaderID           *uint64 `db:"leader_id"`
		Treasury           uint64  `db:"treasury"`
		TaxPreference      float64 `db:"tax_preference"`
		TradePreference    float64 `db:"trade_preference"`
		MilitaryPreference float64 `db:"military_preference"`
		InfluenceJSON      string  `db:"influence_json"`
		RelationsJSON      string  `db:"relations_json"`
	}

	var rows []factionRow
	if err := db.conn.Select(&rows, "SELECT * FROM factions"); err != nil {
		return nil, fmt.Errorf("load factions: %w", err)
	}

	result := make([]*social.Faction, 0, len(rows))
	for _, r := range rows {
		f := &social.Faction{
			ID:                 social.FactionID(r.ID),
			Name:               r.Name,
			Kind:               social.FactionKind(r.Kind),
			LeaderID:           r.LeaderID,
			Treasury:           r.Treasury,
			TaxPreference:      r.TaxPreference,
			TradePreference:    r.TradePreference,
			MilitaryPreference: r.MilitaryPreference,
		}

		var influence map[uint64]float64
		json.Unmarshal([]byte(r.InfluenceJSON), &influence)
		if influence == nil {
			influence = make(map[uint64]float64)
		}
		f.Influence = influence

		var relations map[social.FactionID]float64
		json.Unmarshal([]byte(r.RelationsJSON), &relations)
		if relations == nil {
			relations = make(map[social.FactionID]float64)
		}
		f.Relations = relations

		result = append(result, f)
	}

	return result, nil
}

// HasFactions returns true if the database contains saved factions.
func (db *DB) HasFactions() bool {
	var count int
	err := db.conn.Get(&count, "SELECT COUNT(*) FROM factions")
	return err == nil && count > 0
}

// SaveEvents appends events to the database.
func (db *DB) SaveEvents(events []engine.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, e := range events {
		_, err := tx.Exec(
			"INSERT INTO events (tick, description, category, narrated) VALUES (?, ?, ?, ?)",
			e.Tick, e.Description, e.Category, e.NarratedDescription,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TrimOldEvents removes events older than keepTicks from the database.
func (db *DB) TrimOldEvents(currentTick uint64, keepTicks uint64) (int64, error) {
	if currentTick <= keepTicks {
		return 0, nil
	}
	cutoff := currentTick - keepTicks
	result, err := db.conn.Exec("DELETE FROM events WHERE tick < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// SaveMeta stores a key-value pair in world metadata.
func (db *DB) SaveMeta(key, value string) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO world_meta (key, value) VALUES (?, ?)",
		key, value,
	)
	return err
}

// GetMeta retrieves a metadata value.
func (db *DB) GetMeta(key string) (string, error) {
	var value string
	err := db.conn.Get(&value, "SELECT value FROM world_meta WHERE key = ?", key)
	return value, err
}

// SaveWorldState performs a full save of all world state.
func (db *DB) SaveWorldState(sim *engine.Simulation) error {
	slog.Info("saving world state", "agents", len(sim.Agents), "settlements", len(sim.Settlements))

	if err := db.SaveAgents(sim.Agents); err != nil {
		return fmt.Errorf("save agents: %w", err)
	}
	if err := db.SaveSettlements(sim.Settlements); err != nil {
		return fmt.Errorf("save settlements: %w", err)
	}
	if err := db.SaveFactions(sim.Factions); err != nil {
		return fmt.Errorf("save factions: %w", err)
	}
	if err := db.SaveMemories(sim.Agents); err != nil {
		return fmt.Errorf("save memories: %w", err)
	}
	if err := db.SaveRelationships(sim.Agents); err != nil {
		return fmt.Errorf("save relationships: %w", err)
	}
	if err := db.SaveEvents(sim.Events); err != nil {
		return fmt.Errorf("save events: %w", err)
	}
	if err := db.SaveMeta("last_tick", fmt.Sprintf("%d", sim.CurrentTick())); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}
	if err := db.SaveMeta("season", fmt.Sprintf("%d", sim.CurrentSeason)); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	// Persist settlement viability tracking maps so they survive restarts.
	if len(sim.NonViableWeeks) > 0 {
		nvJSON, _ := json.Marshal(sim.NonViableWeeks)
		if err := db.SaveMeta("non_viable_weeks", string(nvJSON)); err != nil {
			return fmt.Errorf("save non_viable_weeks: %w", err)
		}
	}
	if len(sim.AbandonedWeeks) > 0 {
		awJSON, _ := json.Marshal(sim.AbandonedWeeks)
		if err := db.SaveMeta("abandoned_weeks", string(awJSON)); err != nil {
			return fmt.Errorf("save abandoned_weeks: %w", err)
		}
	}

	slog.Info("world state saved")
	return nil
}

// HasWorldState returns true if the database contains saved agents.
func (db *DB) HasWorldState() bool {
	var count int
	err := db.conn.Get(&count, "SELECT COUNT(*) FROM agents")
	return err == nil && count > 0
}

// LoadAgents reads all agents from the database.
func (db *DB) LoadAgents() ([]*agents.Agent, error) {
	type agentRow struct {
		ID               uint64  `db:"id"`
		Name             string  `db:"name"`
		Age              uint16  `db:"age"`
		Sex              uint8   `db:"sex"`
		Health           float32 `db:"health"`
		PosQ             int     `db:"pos_q"`
		PosR             int     `db:"pos_r"`
		HomeSettlementID *uint64 `db:"home_settlement_id"`
		Occupation       uint8   `db:"occupation"`
		Wealth           uint64  `db:"wealth"`
		Tier             uint8   `db:"tier"`
		Mood             float32 `db:"mood"`
		Alive            int     `db:"alive"`
		BornTick         uint64  `db:"born_tick"`
		Role             uint8   `db:"role"`
		FactionID        *uint64 `db:"faction_id"`
		Archetype        *string `db:"archetype"`
		SkillsJSON       string  `db:"skills_json"`
		NeedsJSON        string  `db:"needs_json"`
		SoulJSON         string  `db:"soul_json"`
		InventoryJSON    string  `db:"inventory_json"`
		Satisfaction     float32 `db:"satisfaction"`
		Alignment        float32 `db:"alignment"`
	}

	var rows []agentRow
	if err := db.conn.Select(&rows, "SELECT * FROM agents"); err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}

	result := make([]*agents.Agent, 0, len(rows))
	for _, r := range rows {
		// Seed wellbeing from stored values. Old data: satisfaction/alignment
		// default to 0; mood seeds EffectiveMood. Alignment recomputes on first tick.
		sat := r.Satisfaction
		if sat == 0 && r.Mood != 0 {
			sat = r.Mood // Seed from old mood column on first load
		}
		a := &agents.Agent{
			ID:         agents.AgentID(r.ID),
			Name:       r.Name,
			Age:        r.Age,
			Sex:        agents.Sex(r.Sex),
			Health:     r.Health,
			Position:   world.HexCoord{Q: r.PosQ, R: r.PosR},
			HomeSettID: r.HomeSettlementID,
			Occupation: agents.Occupation(r.Occupation),
			Wealth:     r.Wealth,
			Tier:       agents.CognitionTier(r.Tier),
			Wellbeing: agents.WellbeingState{
				Satisfaction:  sat,
				Alignment:     r.Alignment,
				EffectiveMood: r.Mood,
			},
			Alive:    r.Alive != 0,
			BornTick: r.BornTick,
			Role:     agents.SocialRole(r.Role),
			FactionID: r.FactionID,
		}
		if r.Archetype != nil {
			a.Archetype = *r.Archetype
		}

		json.Unmarshal([]byte(r.SkillsJSON), &a.Skills)
		json.Unmarshal([]byte(r.NeedsJSON), &a.Needs)
		json.Unmarshal([]byte(r.SoulJSON), &a.Soul)

		var inv map[agents.GoodType]int
		json.Unmarshal([]byte(r.InventoryJSON), &inv)
		if inv == nil {
			inv = make(map[agents.GoodType]int)
		}
		a.Inventory = inv

		result = append(result, a)
	}

	return result, nil
}

// LoadSettlements reads all settlements from the database.
func (db *DB) LoadSettlements() ([]*social.Settlement, error) {
	type settRow struct {
		ID                uint64  `db:"id"`
		Name              string  `db:"name"`
		PosQ              int     `db:"pos_q"`
		PosR              int     `db:"pos_r"`
		Population        uint32  `db:"population"`
		Governance        uint8   `db:"governance"`
		TaxRate           float64 `db:"tax_rate"`
		Treasury          uint64  `db:"treasury"`
		GovernanceScore   float64 `db:"governance_score"`
		CulturalMemory    float64 `db:"cultural_memory"`
		CultureTradition  float32 `db:"culture_tradition"`
		CultureOpenness   float32 `db:"culture_openness"`
		CultureMilitarism float32 `db:"culture_militarism"`
		WallLevel         uint8   `db:"wall_level"`
		RoadLevel         uint8   `db:"road_level"`
		MarketLevel       uint8   `db:"market_level"`
		Abandoned         int     `db:"abandoned"`
	}

	var rows []settRow
	if err := db.conn.Select(&rows, "SELECT * FROM settlements"); err != nil {
		return nil, fmt.Errorf("load settlements: %w", err)
	}

	result := make([]*social.Settlement, 0, len(rows))
	for _, r := range rows {
		result = append(result, &social.Settlement{
			ID:                r.ID,
			Name:              r.Name,
			Position:          world.HexCoord{Q: r.PosQ, R: r.PosR},
			Population:        r.Population,
			Governance:        social.GovernanceType(r.Governance),
			TaxRate:           r.TaxRate,
			Treasury:          r.Treasury,
			GovernanceScore:   r.GovernanceScore,
			CulturalMemory:    r.CulturalMemory,
			CultureTradition:  r.CultureTradition,
			CultureOpenness:   r.CultureOpenness,
			CultureMilitarism: r.CultureMilitarism,
			WallLevel:         r.WallLevel,
			RoadLevel:         r.RoadLevel,
			MarketLevel:       r.MarketLevel,
		})
	}

	return result, nil
}

// SaveMemories writes all agent memories to the database (full replace).
func (db *DB) SaveMemories(agentList []*agents.Agent) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM memories"); err != nil {
		return err
	}

	stmt, err := tx.Preparex("INSERT INTO memories (agent_id, tick, content, importance) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range agentList {
		for _, m := range a.Memories {
			if _, err := stmt.Exec(a.ID, m.Tick, m.Content, m.Importance); err != nil {
				return fmt.Errorf("insert memory for agent %d: %w", a.ID, err)
			}
		}
	}

	return tx.Commit()
}

// LoadMemories reads all memories and attaches them to agents by ID.
func (db *DB) LoadMemories(agentIndex map[agents.AgentID]*agents.Agent) error {
	type memRow struct {
		AgentID    uint64  `db:"agent_id"`
		Tick       uint64  `db:"tick"`
		Content    string  `db:"content"`
		Importance float32 `db:"importance"`
	}

	var rows []memRow
	if err := db.conn.Select(&rows, "SELECT agent_id, tick, content, importance FROM memories"); err != nil {
		// Table might not exist yet in old databases — not fatal.
		return nil
	}

	for _, r := range rows {
		a, ok := agentIndex[agents.AgentID(r.AgentID)]
		if !ok {
			continue
		}
		a.Memories = append(a.Memories, agents.Memory{
			Tick:       r.Tick,
			Content:    r.Content,
			Importance: r.Importance,
		})
	}
	return nil
}

// SaveRelationships writes all agent relationships to the database (full replace).
func (db *DB) SaveRelationships(agentList []*agents.Agent) error {
	tx, err := db.conn.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM relationships"); err != nil {
		return err
	}

	stmt, err := tx.Preparex("INSERT INTO relationships (agent_id, target_id, sentiment, trust) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range agentList {
		for _, r := range a.Relationships {
			if _, err := stmt.Exec(a.ID, r.TargetID, r.Sentiment, r.Trust); err != nil {
				return fmt.Errorf("insert relationship for agent %d: %w", a.ID, err)
			}
		}
	}

	return tx.Commit()
}

// LoadRelationships reads all relationships and attaches them to agents by ID.
func (db *DB) LoadRelationships(agentIndex map[agents.AgentID]*agents.Agent) error {
	type relRow struct {
		AgentID   uint64  `db:"agent_id"`
		TargetID  uint64  `db:"target_id"`
		Sentiment float32 `db:"sentiment"`
		Trust     float32 `db:"trust"`
	}

	var rows []relRow
	if err := db.conn.Select(&rows, "SELECT agent_id, target_id, sentiment, trust FROM relationships"); err != nil {
		// Table might not exist yet in old databases — not fatal.
		return nil
	}

	for _, r := range rows {
		a, ok := agentIndex[agents.AgentID(r.AgentID)]
		if !ok {
			continue
		}
		a.Relationships = append(a.Relationships, agents.Relationship{
			TargetID:  agents.AgentID(r.TargetID),
			Sentiment: r.Sentiment,
			Trust:     r.Trust,
		})
	}
	return nil
}

// RecentEvents returns the most recent N events.
func (db *DB) RecentEvents(limit int) ([]engine.Event, error) {
	var events []engine.Event
	err := db.conn.Select(&events,
		"SELECT tick, description, category, narrated FROM events ORDER BY id DESC LIMIT ?",
		limit,
	)
	return events, err
}

// StatsRow represents a single historical stats snapshot.
type StatsRow struct {
	Tick            uint64  `json:"tick" db:"tick"`
	Population      int     `json:"population" db:"population"`
	TotalWealth     uint64  `json:"total_wealth" db:"total_wealth"`
	AvgMood         float64 `json:"avg_mood" db:"avg_mood"`
	AvgSurvival     float64 `json:"avg_survival" db:"avg_survival"`
	Births          int     `json:"births" db:"births"`
	Deaths          int     `json:"deaths" db:"deaths"`
	TradeVolume     uint64  `json:"trade_volume" db:"trade_volume"`
	AvgCoherence    float64 `json:"avg_coherence" db:"avg_coherence"`
	SettlementCount int     `json:"settlement_count" db:"settlement_count"`
	Gini            float64 `json:"gini" db:"gini"`
	AvgSatisfaction float64 `json:"avg_satisfaction" db:"avg_satisfaction"`
	AvgAlignment    float64 `json:"avg_alignment" db:"avg_alignment"`
}

// SaveStatsSnapshot records a daily statistics snapshot.
func (db *DB) SaveStatsSnapshot(row StatsRow) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO stats_history
		(tick, population, total_wealth, avg_mood, avg_survival, births, deaths,
		 trade_volume, avg_coherence, settlement_count, gini, avg_satisfaction, avg_alignment)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Tick, row.Population, row.TotalWealth, row.AvgMood, row.AvgSurvival,
		row.Births, row.Deaths, row.TradeVolume, row.AvgCoherence,
		row.SettlementCount, row.Gini, row.AvgSatisfaction, row.AvgAlignment,
	)
	return err
}

// LoadStatsHistory returns stats snapshots within a tick range.
func (db *DB) LoadStatsHistory(fromTick, toTick uint64, limit int) ([]StatsRow, error) {
	var rows []StatsRow
	if limit <= 0 {
		limit = 30
	}
	err := db.conn.Select(&rows,
		`SELECT tick, population, total_wealth, avg_mood, avg_survival, births, deaths,
		 trade_volume, avg_coherence, settlement_count, gini, avg_satisfaction, avg_alignment
		 FROM stats_history WHERE tick >= ? AND tick <= ?
		 ORDER BY tick DESC LIMIT ?`,
		fromTick, toTick, limit,
	)
	return rows, err
}
