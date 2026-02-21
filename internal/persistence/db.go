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
		category TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS world_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_tick ON events(tick);
	CREATE INDEX IF NOT EXISTS idx_agents_settlement ON agents(home_settlement_id);
	CREATE INDEX IF NOT EXISTS idx_agents_alive ON agents(alive);
	`
	_, err := db.conn.Exec(schema)
	return err
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
		 skills_json, needs_json, soul_json, inventory_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
			a.Occupation, a.Wealth, a.Tier, a.Mood,
			alive, a.BornTick, a.Role, a.FactionID, a.Archetype,
			string(skillsJSON), string(needsJSON), string(soulJSON), string(invJSON),
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
			"INSERT INTO events (tick, description, category) VALUES (?, ?, ?)",
			e.Tick, e.Description, e.Category,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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
	if err := db.SaveEvents(sim.Events); err != nil {
		return fmt.Errorf("save events: %w", err)
	}
	if err := db.SaveMeta("last_tick", fmt.Sprintf("%d", sim.CurrentTick())); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	slog.Info("world state saved")
	return nil
}

// RecentEvents returns the most recent N events.
func (db *DB) RecentEvents(limit int) ([]engine.Event, error) {
	var events []engine.Event
	err := db.conn.Select(&events,
		"SELECT tick, description, category FROM events ORDER BY id DESC LIMIT ?",
		limit,
	)
	return events, err
}
