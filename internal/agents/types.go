// Package agents provides the agent data model, needs system, and cognition tiers.
// See design doc Sections 4 and 16.2–16.4.
package agents

import (
	"github.com/talgya/mini-world/internal/world"
)

// AgentID is a unique identifier for an agent.
type AgentID uint64

// Sex represents biological sex for demographic simulation.
type Sex uint8

const (
	SexMale   Sex = 0
	SexFemale Sex = 1
)

// CognitionTier determines how an agent makes decisions.
type CognitionTier uint8

const (
	Tier0 CognitionTier = 0 // Automaton — pure rule-based (95% of agents)
	Tier1 CognitionTier = 1 // Archetype-guided — behavioral templates (4%)
	Tier2 CognitionTier = 2 // LLM-powered individual — Haiku API calls (<1%)
)

// Occupation represents an agent's primary economic activity.
type Occupation uint8

const (
	OccupationFarmer    Occupation = iota
	OccupationMiner
	OccupationCrafter
	OccupationMerchant
	OccupationSoldier
	OccupationScholar
	OccupationAlchemist
	OccupationLaborer
	OccupationFisher
	OccupationHunter
)

// SocialRole represents an agent's position in the social hierarchy.
type SocialRole uint8

const (
	RoleCommoner  SocialRole = iota
	RoleMerchant
	RoleSoldier
	RoleNoble
	RoleLeader
	RoleOutlaw
	RoleScholar
)

// Agent is the core entity representing a person in the simulation.
type Agent struct {
	ID   AgentID `json:"id"`
	Name string  `json:"name"`

	// Demographics
	Age    uint16  `json:"age"`    // Sim-years
	Sex    Sex     `json:"sex"`
	Health float32 `json:"health"` // 0.0–1.0

	// Location
	Position     world.HexCoord  `json:"position"`
	HomeSettID   *uint64         `json:"home_settlement_id,omitempty"`
	Destination  *world.HexCoord `json:"destination,omitempty"`

	// Economic
	Occupation Occupation         `json:"occupation"`
	Inventory  map[GoodType]int   `json:"inventory"`
	Wealth     uint64             `json:"wealth"`  // Crowns
	Skills     SkillSet           `json:"skills"`

	// Social
	Relationships []Relationship `json:"relationships"`
	FactionID     *uint64        `json:"faction_id,omitempty"`
	Role          SocialRole     `json:"role"`

	// Cognition
	Tier      CognitionTier `json:"tier"`
	Archetype string        `json:"archetype,omitempty"` // For Tier 1
	Mood      float32       `json:"mood"`                // -1.0 (despair) to 1.0 (elation)

	// Soul — Wheeler coherence model (Section 16.2)
	Soul AgentSoul `json:"soul"`

	// Needs — evaluated bottom-up (Section 4.3)
	Needs NeedsState `json:"needs"`

	// Trade (merchants only)
	TradeDestSett  *uint64          `json:"trade_dest_sett,omitempty"`  // Destination settlement ID
	TradeCargo     map[GoodType]int `json:"trade_cargo,omitempty"`     // Goods being transported
	TravelTicksLeft uint16          `json:"travel_ticks_left,omitempty"` // Ticks remaining to reach destination

	// Metadata
	BornTick uint64 `json:"born_tick"`
	Alive    bool   `json:"alive"`
}

// GoodType enumerates manufactured/tradeable goods.
type GoodType uint8

const (
	GoodGrain    GoodType = iota // Food staple
	GoodTimber                   // Construction
	GoodIronOre                  // Raw material
	GoodStone                    // Construction
	GoodFish                     // Food
	GoodHerbs                    // Medicine/alchemy
	GoodGems                     // Luxury
	GoodFurs                     // Clothing/luxury
	GoodCoal                     // Fuel
	GoodExotics                  // Alchemical
	GoodTools                    // Iron + Timber
	GoodWeapons                  // Iron + Timber/Leather
	GoodClothing                 // Furs/Fibers + Labor
	GoodMedicine                 // Herbs + Knowledge
	GoodLuxuries                 // Gems + Crafting
)

// SkillSet tracks an agent's capabilities.
type SkillSet struct {
	Farming  float32 `json:"farming"`  // 0.0–1.0
	Mining   float32 `json:"mining"`
	Crafting float32 `json:"crafting"`
	Combat   float32 `json:"combat"`
	Trade    float32 `json:"trade"`
}

// Relationship represents a social bond between two agents.
type Relationship struct {
	TargetID  AgentID `json:"target_id"`
	Sentiment float32 `json:"sentiment"` // -1.0 (hatred) to 1.0 (love)
	Trust     float32 `json:"trust"`     // 0.0 to 1.0
}
