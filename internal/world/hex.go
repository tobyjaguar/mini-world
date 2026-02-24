// Package world provides the hex grid, terrain, and spatial data structures.
// Uses axial coordinates (q, r) for the hex grid.
// See design doc Section 3.
package world

// HexCoord represents a position on the hex grid using axial coordinates.
// The third cube coordinate s is derived: s = -q - r.
type HexCoord struct {
	Q int `json:"q"`
	R int `json:"r"`
}

// S returns the implicit third cube coordinate.
func (h HexCoord) S() int {
	return -h.Q - h.R
}

// Terrain types for hex tiles.
type Terrain uint8

const (
	TerrainPlains    Terrain = iota // Fertile plains — high agricultural yield
	TerrainForest                   // Timber, herbs, game
	TerrainMountain                 // Minerals, gems, defensive positions
	TerrainCoast                    // Fishing, port potential
	TerrainRiver                    // Freshwater, irrigation, trade arteries
	TerrainDesert                   // Rare minerals, harsh conditions
	TerrainSwamp                    // Alchemical ingredients, disease risk
	TerrainTundra                   // Furs, ice minerals, extreme conditions
	TerrainOcean                    // Impassable except by ship
)

// Hex represents a single tile on the world map.
type Hex struct {
	Coord   HexCoord `json:"coord"`
	Terrain Terrain  `json:"terrain"`

	// Resource yields (base values, modified by season and exploitation).
	// Each resource type maps to a remaining quantity.
	Resources map[ResourceType]float64 `json:"resources"`

	// Elevation and climate data (set during world generation).
	Elevation   float64 `json:"elevation"`   // 0.0 (sea level) to 1.0 (peak)
	Rainfall    float64 `json:"rainfall"`    // 0.0 (arid) to 1.0 (tropical)
	Temperature float64 `json:"temperature"` // 0.0 (frozen) to 1.0 (hot)

	// Settlement on this hex, if any.
	SettlementID *uint64 `json:"settlement_id,omitempty"`

	// Land health: 0.0 (degraded) to 1.0 (pristine).
	// Extraction degrades health; fallow hexes recover.
	Health            float64 `json:"health"`
	LastExtractedTick uint64  `json:"last_extracted_tick"`
}

// ResourceType enumerates primary resources harvestable from terrain.
type ResourceType uint8

const (
	ResourceGrain    ResourceType = iota // From plains
	ResourceTimber                       // From forests
	ResourceIronOre                      // From mountains
	ResourceStone                        // From mountains/quarries
	ResourceFish                         // From coast/rivers
	ResourceHerbs                        // From forests/swamps
	ResourceGems                         // Rare, from deep mines
	ResourceFurs                         // From hunting
	ResourceCoal                         // From mines
	ResourceExotics                      // Rare spawns — alchemical reagents, strange metals
)

// HexNeighborDirections defines the six neighbor offsets in axial coordinates.
var HexNeighborDirections = [6]HexCoord{
	{Q: 1, R: 0},
	{Q: 1, R: -1},
	{Q: 0, R: -1},
	{Q: -1, R: 0},
	{Q: -1, R: 1},
	{Q: 0, R: 1},
}

// Neighbors returns the six adjacent hex coordinates.
func (h HexCoord) Neighbors() [6]HexCoord {
	var result [6]HexCoord
	for i, dir := range HexNeighborDirections {
		result[i] = HexCoord{Q: h.Q + dir.Q, R: h.R + dir.R}
	}
	return result
}

// Distance returns the hex distance between two coordinates.
func Distance(a, b HexCoord) int {
	dq := a.Q - b.Q
	dr := a.R - b.R
	ds := a.S() - b.S()
	if dq < 0 {
		dq = -dq
	}
	if dr < 0 {
		dr = -dr
	}
	if ds < 0 {
		ds = -ds
	}
	// Max of the three absolute differences in cube coordinates.
	max := dq
	if dr > max {
		max = dr
	}
	if ds > max {
		max = ds
	}
	return max
}
