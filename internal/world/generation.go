// World generation using layered simplex noise.
// Generates elevation, rainfall, and temperature maps, then derives terrain and resources.
// See design doc Section 3.2.
package world

import (
	"math"
	"math/rand"

	opensimplex "github.com/ojrac/opensimplex-go"
)

// GenConfig holds world generation parameters.
type GenConfig struct {
	Radius      int     // Hex grid radius (~22 for ~2000 hexes)
	Seed        int64   // Random seed (0 = random)
	SeaLevel    float64 // Elevation threshold for ocean (0.0–1.0)
	MountainLvl float64 // Elevation threshold for mountains (0.0–1.0)
}

// DefaultGenConfig returns a reasonable starting configuration.
// Uses a smaller radius for initial development/testing.
func DefaultGenConfig() GenConfig {
	return GenConfig{
		Radius:      22,
		Seed:        0,
		SeaLevel:    0.25,
		MountainLvl: 0.72,
	}
}

// SmallTestConfig returns a tiny world for rapid iteration.
func SmallTestConfig() GenConfig {
	return GenConfig{
		Radius:      5,
		Seed:        42,
		SeaLevel:    0.30,
		MountainLvl: 0.75,
	}
}

// Generate creates a complete world map with terrain and resources.
func Generate(cfg GenConfig) *Map {
	seed := cfg.Seed
	if seed == 0 {
		seed = rand.Int63()
	}

	// Three noise generators for independent layers.
	elevNoise := opensimplex.NewNormalized(seed)
	rainNoise := opensimplex.NewNormalized(seed + 1)
	tempNoise := opensimplex.NewNormalized(seed + 2)

	m := NewMap(cfg.Radius)

	// Generate each hex within radius.
	for q := -cfg.Radius; q <= cfg.Radius; q++ {
		for r := -cfg.Radius; r <= cfg.Radius; r++ {
			s := -q - r
			// Cube coordinate constraint: max(|q|,|r|,|s|) <= radius
			aq, ar, as := abs(q), abs(r), abs(s)
			maxCoord := aq
			if ar > maxCoord {
				maxCoord = ar
			}
			if as > maxCoord {
				maxCoord = as
			}
			if maxCoord > cfg.Radius {
				continue
			}

			coord := HexCoord{Q: q, R: r}

			// Convert hex coords to continuous space for noise sampling.
			// Hex axial → cartesian: x = q + r*0.5, y = r * sqrt(3)/2
			x := float64(q) + float64(r)*0.5
			y := float64(r) * math.Sqrt(3.0) / 2.0

			// Multi-octave noise for natural-looking terrain.
			elev := octaveNoise(elevNoise, x, y, 4, 0.08, 0.5)
			rain := octaveNoise(rainNoise, x, y, 3, 0.06, 0.5)
			temp := octaveNoise(tempNoise, x, y, 3, 0.05, 0.5)

			// Continental shaping: reduce elevation near edges to create ocean border.
			distFromCenter := math.Sqrt(x*x+y*y) / float64(cfg.Radius)
			edgeFalloff := 1.0 - math.Pow(distFromCenter, 3.5)
			if edgeFalloff < 0 {
				edgeFalloff = 0
			}
			elev *= edgeFalloff

			// Temperature decreases with elevation and distance from equator.
			temp = temp*0.6 + (1.0-math.Abs(y)/float64(cfg.Radius))*0.3 + (1.0-elev)*0.1

			// Derive terrain type.
			terrain := deriveTerrain(elev, rain, temp, cfg)

			hex := &Hex{
				Coord:       coord,
				Terrain:     terrain,
				Elevation:   elev,
				Rainfall:    rain,
				Temperature: temp,
				Resources:   makeResources(terrain, elev, rain),
			Health:      1.0, // Pristine land at world generation
			}

			m.Set(hex)
		}
	}

	// Post-pass: mark coastal hexes (land hexes adjacent to ocean).
	markCoastalHexes(m)

	// Post-pass: place rivers flowing from high elevation to coast.
	placeRivers(m, seed)

	return m
}

// deriveTerrain determines terrain type from environmental parameters.
func deriveTerrain(elev, rain, temp float64, cfg GenConfig) Terrain {
	if elev < cfg.SeaLevel {
		return TerrainOcean
	}
	if elev > cfg.MountainLvl {
		return TerrainMountain
	}
	if temp < 0.25 {
		return TerrainTundra
	}
	if rain < 0.25 && temp > 0.5 {
		return TerrainDesert
	}
	if rain > 0.7 && elev < 0.45 {
		return TerrainSwamp
	}
	if rain > 0.45 && elev > 0.45 {
		return TerrainForest
	}
	return TerrainPlains
}

// makeResources populates initial resource yields based on terrain.
func makeResources(terrain Terrain, elev, rain float64) map[ResourceType]float64 {
	res := make(map[ResourceType]float64)

	switch terrain {
	case TerrainPlains:
		res[ResourceGrain] = 80 + rain*40   // Rainfall boosts yield
	case TerrainForest:
		res[ResourceTimber] = 100
		res[ResourceHerbs] = 30
		res[ResourceFurs] = 20
	case TerrainMountain:
		res[ResourceIronOre] = 60 + elev*30
		res[ResourceStone] = 80
		res[ResourceCoal] = 40
		if elev > 0.85 {
			res[ResourceGems] = 10 // Rare at high elevations
		}
	case TerrainCoast:
		res[ResourceFish] = 80
	case TerrainRiver:
		res[ResourceFish] = 50
		res[ResourceGrain] = 40 // Irrigation bonus
	case TerrainSwamp:
		res[ResourceHerbs] = 60
		res[ResourceExotics] = 5 // Rare alchemical ingredients
	case TerrainTundra:
		res[ResourceFurs] = 40
	case TerrainDesert:
		res[ResourceStone] = 30
		if elev > 0.5 {
			res[ResourceGems] = 8
		}
	}

	return res
}

// markCoastalHexes converts land hexes adjacent to ocean into coast terrain.
func markCoastalHexes(m *Map) {
	var toMark []HexCoord

	for coord, hex := range m.Hexes {
		if hex.Terrain == TerrainOcean {
			continue
		}
		for _, neighbor := range coord.Neighbors() {
			nh := m.Get(neighbor)
			if nh != nil && nh.Terrain == TerrainOcean {
				toMark = append(toMark, coord)
				break
			}
		}
	}

	for _, coord := range toMark {
		hex := m.Get(coord)
		// Only convert plains/forest at low elevation to coast.
		if hex.Terrain == TerrainPlains || hex.Terrain == TerrainForest {
			if hex.Elevation < 0.5 {
				hex.Terrain = TerrainCoast
				hex.Resources = makeResources(TerrainCoast, hex.Elevation, hex.Rainfall)
				// Coast also gets some of its original resources.
				if hex.Rainfall > 0.4 {
					hex.Resources[ResourceGrain] = 20
				}
			}
		}
	}
}

// placeRivers traces paths from high elevation to coast/ocean, marking hexes as river.
func placeRivers(m *Map, seed int64) {
	rng := rand.New(rand.NewSource(seed + 100))

	// Find mountain/highland hexes as river sources.
	var sources []HexCoord
	for coord, hex := range m.Hexes {
		if hex.Elevation > 0.65 && hex.Terrain != TerrainOcean {
			sources = append(sources, coord)
		}
	}

	// Only create a handful of rivers — not every mountain needs one.
	numRivers := len(sources) / 8
	if numRivers < 2 {
		numRivers = 2
	}
	if numRivers > 10 {
		numRivers = 10
	}

	// Shuffle and pick.
	rng.Shuffle(len(sources), func(i, j int) {
		sources[i], sources[j] = sources[j], sources[i]
	})
	if len(sources) > numRivers {
		sources = sources[:numRivers]
	}

	for _, start := range sources {
		traceRiver(m, start)
	}
}

// traceRiver follows the steepest descent from a source hex until reaching
// ocean or running out of downhill path.
func traceRiver(m *Map, start HexCoord) {
	current := start
	visited := make(map[HexCoord]bool)
	maxSteps := 50

	for step := 0; step < maxSteps; step++ {
		visited[current] = true
		hex := m.Get(current)
		if hex == nil {
			break
		}

		// Stop at ocean.
		if hex.Terrain == TerrainOcean {
			break
		}

		// Mark as river (unless it's a mountain peak or coast).
		if hex.Terrain != TerrainMountain && hex.Terrain != TerrainCoast {
			hex.Terrain = TerrainRiver
			hex.Resources[ResourceFish] = 50
			hex.Resources[ResourceGrain] += 20 // Irrigation bonus
		}

		// Find lowest neighbor.
		var bestNeighbor *HexCoord
		bestElev := hex.Elevation

		for _, nc := range current.Neighbors() {
			if visited[nc] {
				continue
			}
			nh := m.Get(nc)
			if nh == nil {
				continue
			}
			if nh.Elevation < bestElev {
				bestElev = nh.Elevation
				c := nc // capture
				bestNeighbor = &c
			}
		}

		if bestNeighbor == nil {
			break // No downhill path — river ends (lake would form in reality)
		}
		current = *bestNeighbor
	}
}

// octaveNoise generates fractal noise by layering multiple frequencies.
func octaveNoise(noise opensimplex.Noise, x, y float64, octaves int, frequency, persistence float64) float64 {
	total := 0.0
	amplitude := 1.0
	maxVal := 0.0

	for i := 0; i < octaves; i++ {
		total += noise.Eval2(x*frequency, y*frequency) * amplitude
		maxVal += amplitude
		amplitude *= persistence
		frequency *= 2
	}

	return total / maxVal
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TerrainCounts returns a summary of terrain type distribution.
func TerrainCounts(m *Map) map[Terrain]int {
	counts := make(map[Terrain]int)
	for _, hex := range m.Hexes {
		counts[hex.Terrain]++
	}
	return counts
}

// TerrainName returns a human-readable name for a terrain type.
func TerrainName(t Terrain) string {
	switch t {
	case TerrainPlains:
		return "Plains"
	case TerrainForest:
		return "Forest"
	case TerrainMountain:
		return "Mountain"
	case TerrainCoast:
		return "Coast"
	case TerrainRiver:
		return "River"
	case TerrainDesert:
		return "Desert"
	case TerrainSwamp:
		return "Swamp"
	case TerrainTundra:
		return "Tundra"
	case TerrainOcean:
		return "Ocean"
	default:
		return "Unknown"
	}
}
