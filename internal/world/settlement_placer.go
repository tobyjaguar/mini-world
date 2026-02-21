// Settlement placement — finds suitable locations and seeds initial settlements.
// See design doc Section 3.3.
package world

import (
	"math"
	"math/rand"
	"sort"
)

// SettlementSeed holds the parameters for an initial settlement placement.
type SettlementSeed struct {
	Coord      HexCoord
	Size       SettlementSize
	Score      float64 // Desirability score
	Name       string
}

// SettlementSize categorizes settlement scale.
type SettlementSize uint8

const (
	SizeVillage SettlementSize = iota // 20–200 agents
	SizeTown                          // 200–2,000 agents
	SizeCity                          // 2,000–10,000 agents
)

// PlaceSettlements finds optimal locations for initial settlements on the map.
// Returns a list of settlement seeds sorted by desirability.
func PlaceSettlements(m *Map, seed int64) []SettlementSeed {
	rng := rand.New(rand.NewSource(seed + 200))

	// Score every land hex for settlement desirability.
	type scored struct {
		coord HexCoord
		score float64
	}
	var candidates []scored

	for coord, hex := range m.Hexes {
		if hex.Terrain == TerrainOcean {
			continue
		}
		s := settlementScore(m, coord, hex)
		if s > 0 {
			candidates = append(candidates, scored{coord, s})
		}
	}

	// Sort by score descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	var seeds []SettlementSeed

	// Place cities at the best locations, enforcing minimum distance.
	taken := make(map[HexCoord]bool)
	minCityDist := 8
	minTownDist := 4
	minVillageDist := 2

	// Cities: top 3–5 locations.
	numCities := 3 + rng.Intn(3)
	for _, c := range candidates {
		if len(seeds) >= numCities {
			break
		}
		if tooClose(c.coord, seeds, minCityDist) {
			continue
		}
		taken[c.coord] = true
		seeds = append(seeds, SettlementSeed{
			Coord: c.coord,
			Size:  SizeCity,
			Score: c.score,
		})
	}

	// Towns: next 10–20 locations.
	numTowns := 10 + rng.Intn(11)
	for _, c := range candidates {
		if countBySize(seeds, SizeTown) >= numTowns {
			break
		}
		if taken[c.coord] || tooClose(c.coord, seeds, minTownDist) {
			continue
		}
		taken[c.coord] = true
		seeds = append(seeds, SettlementSeed{
			Coord: c.coord,
			Size:  SizeTown,
			Score: c.score,
		})
	}

	// Villages: scatter 30–50 across remaining good land.
	numVillages := 30 + rng.Intn(21)
	for _, c := range candidates {
		if countBySize(seeds, SizeVillage) >= numVillages {
			break
		}
		if taken[c.coord] || tooClose(c.coord, seeds, minVillageDist) {
			continue
		}
		taken[c.coord] = true
		seeds = append(seeds, SettlementSeed{
			Coord: c.coord,
			Size:  SizeVillage,
			Score: c.score,
		})
	}

	// Assign procedural names.
	names := generateNames(rng, len(seeds))
	for i := range seeds {
		seeds[i].Name = names[i]
	}

	return seeds
}

// settlementScore evaluates how desirable a hex is for a settlement.
// Prefers: coast (trade), rivers (water+trade), fertile plains, nearby mountains (resources).
func settlementScore(m *Map, coord HexCoord, hex *Hex) float64 {
	score := 0.0

	switch hex.Terrain {
	case TerrainPlains:
		score += 3.0
	case TerrainCoast:
		score += 4.0 // Harbors are prime locations
	case TerrainRiver:
		score += 3.5 // Freshwater + trade arteries
	case TerrainForest:
		score += 1.5
	case TerrainDesert, TerrainSwamp, TerrainTundra:
		score += 0.5
	case TerrainMountain:
		score += 0.3 // Mining outposts, not ideal for settlements
	default:
		return 0
	}

	// Bonus for nearby terrain diversity (economic complexity).
	terrainTypes := make(map[Terrain]bool)
	for _, nc := range coord.Neighbors() {
		nh := m.Get(nc)
		if nh != nil && nh.Terrain != TerrainOcean {
			terrainTypes[nh.Terrain] = true
		}
	}
	score += float64(len(terrainTypes)) * 0.3

	// Bonus for nearby river or coast (water access).
	for _, nc := range coord.Neighbors() {
		nh := m.Get(nc)
		if nh == nil {
			continue
		}
		if nh.Terrain == TerrainRiver || nh.Terrain == TerrainCoast {
			score += 0.5
			break
		}
	}

	// Bonus for total nearby resources.
	totalRes := 0.0
	for _, v := range hex.Resources {
		totalRes += v
	}
	score += math.Log1p(totalRes) * 0.2

	return score
}

func tooClose(coord HexCoord, existing []SettlementSeed, minDist int) bool {
	for _, s := range existing {
		if Distance(coord, s.Coord) < minDist {
			return true
		}
	}
	return false
}

func countBySize(seeds []SettlementSeed, size SettlementSize) int {
	n := 0
	for _, s := range seeds {
		if s.Size == size {
			n++
		}
	}
	return n
}

// generateNames produces procedural settlement names by combining syllables.
func generateNames(rng *rand.Rand, count int) []string {
	prefixes := []string{
		"Iron", "Green", "Ash", "Stone", "Mill", "Cross", "Black",
		"Silver", "Red", "White", "Dark", "Bright", "High", "Low",
		"Old", "New", "Far", "Deep", "Long", "Broad", "Gold", "Frost",
		"Storm", "Thorn", "Elm", "Oak", "Pine", "Copper", "River",
	}
	suffixes := []string{
		"haven", "ford", "hollow", "wick", "bridge", "gate", "keep",
		"stead", "wood", "field", "dale", "crest", "vale", "port",
		"town", "bury", "marsh", "well", "brook", "cliff", "moor",
		"ridge", "watch", "fall", "rest", "point", "reach", "helm",
	}

	used := make(map[string]bool)
	names := make([]string, 0, count)

	for len(names) < count {
		name := prefixes[rng.Intn(len(prefixes))] + suffixes[rng.Intn(len(suffixes))]
		if !used[name] {
			used[name] = true
			names = append(names, name)
		}
	}

	return names
}

// PopulationForSize returns the initial population range for a settlement size.
func PopulationForSize(size SettlementSize, rng *rand.Rand) uint32 {
	switch size {
	case SizeCity:
		return 2000 + uint32(rng.Intn(3000))
	case SizeTown:
		return 200 + uint32(rng.Intn(800))
	case SizeVillage:
		return 20 + uint32(rng.Intn(80))
	default:
		return 50
	}
}
