// Agent spawning — creates the initial population with demographics,
// occupations, soul state, and needs.
// See design doc Sections 4.1, 4.2, 16.2–16.4.
package agents

import (
	"math"
	"math/rand"

	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// SpawnConfig controls initial population generation.
type SpawnConfig struct {
	Seed int64
}

// Spawner creates agents for the simulation.
type Spawner struct {
	rng    *rand.Rand
	nextID AgentID
}

// NewSpawner creates an agent spawner with the given seed.
func NewSpawner(seed int64) *Spawner {
	return &Spawner{
		rng:    rand.New(rand.NewSource(seed + 300)),
		nextID: 1,
	}
}

// SpawnPopulation creates a batch of agents for a settlement.
func (s *Spawner) SpawnPopulation(count uint32, position world.HexCoord, settlementID uint64, terrain world.Terrain) []*Agent {
	agents := make([]*Agent, 0, count)

	for i := uint32(0); i < count; i++ {
		agent := s.spawnOne(position, settlementID, terrain)
		agents = append(agents, agent)
	}

	return agents
}

func (s *Spawner) spawnOne(position world.HexCoord, settlementID uint64, terrain world.Terrain) *Agent {
	id := s.nextID
	s.nextID++

	sex := SexMale
	if s.rng.Float32() < 0.5 {
		sex = SexFemale
	}

	// Age: weighted toward working-age adults (16–55), some children and elderly.
	age := s.weightedAge()

	// Occupation based on terrain and some randomness.
	occ := s.occupationForTerrain(terrain)

	// Skills: primary skill matches occupation, others are low.
	skills := s.skillsForOccupation(occ)

	// Soul: most agents are low-coherence Devotionalists or Ritualists (realistic population).
	soul := s.generateSoul()

	// Starting wealth: modest, occupation-dependent.
	wealth := s.startingWealth(occ)

	// Needs: mostly met at world start (stable starting conditions).
	needs := NeedsState{
		Survival:  0.7 + s.rng.Float32()*0.3,
		Safety:    0.6 + s.rng.Float32()*0.3,
		Belonging: 0.5 + s.rng.Float32()*0.3,
		Esteem:    0.3 + s.rng.Float32()*0.3,
		Purpose:   0.2 + s.rng.Float32()*0.3,
	}

	sid := settlementID
	return &Agent{
		ID:         id,
		Name:       s.generateName(sex),
		Age:        age,
		Sex:        sex,
		Health:     0.8 + s.rng.Float32()*0.2,
		Position:   position,
		HomeSettID: &sid,
		Occupation: occ,
		Inventory:  make(map[GoodType]int),
		Wealth:     wealth,
		Skills:     skills,
		Role:       RoleCommoner,
		Tier:       Tier0,
		Mood:       s.rng.Float32()*0.6 - 0.1, // Slightly positive on average
		Soul:       soul,
		Needs:      needs,
		BornTick:   0,
		Alive:      true,
	}
}

func (s *Spawner) weightedAge() uint16 {
	// Bell curve centered around 30, range 5–70.
	age := 30.0 + s.rng.NormFloat64()*12.0
	if age < 5 {
		age = 5
	}
	if age > 70 {
		age = 70
	}
	return uint16(age)
}

func (s *Spawner) occupationForTerrain(terrain world.Terrain) Occupation {
	r := s.rng.Float32()
	switch terrain {
	case world.TerrainPlains, world.TerrainRiver:
		if r < 0.50 {
			return OccupationFarmer
		} else if r < 0.70 {
			return OccupationLaborer
		} else if r < 0.85 {
			return OccupationCrafter
		} else {
			return OccupationMerchant
		}
	case world.TerrainMountain:
		if r < 0.45 {
			return OccupationMiner
		} else if r < 0.70 {
			return OccupationLaborer
		} else if r < 0.85 {
			return OccupationCrafter
		} else {
			return OccupationMerchant
		}
	case world.TerrainCoast:
		if r < 0.40 {
			return OccupationFisher
		} else if r < 0.60 {
			return OccupationMerchant
		} else if r < 0.80 {
			return OccupationCrafter
		} else {
			return OccupationLaborer
		}
	case world.TerrainForest:
		if r < 0.30 {
			return OccupationHunter
		} else if r < 0.55 {
			return OccupationFarmer
		} else if r < 0.75 {
			return OccupationLaborer
		} else {
			return OccupationCrafter
		}
	case world.TerrainSwamp:
		if r < 0.40 {
			return OccupationAlchemist
		} else if r < 0.70 {
			return OccupationHunter
		} else {
			return OccupationLaborer
		}
	default:
		return OccupationLaborer
	}
}

func (s *Spawner) skillsForOccupation(occ Occupation) SkillSet {
	skills := SkillSet{
		Farming:  0.1 + s.rng.Float32()*0.1,
		Mining:   0.1 + s.rng.Float32()*0.1,
		Crafting: 0.1 + s.rng.Float32()*0.1,
		Combat:   0.05 + s.rng.Float32()*0.1,
		Trade:    0.1 + s.rng.Float32()*0.1,
	}

	// Boost primary skill.
	primary := 0.4 + s.rng.Float32()*0.3
	switch occ {
	case OccupationFarmer:
		skills.Farming = primary
	case OccupationMiner:
		skills.Mining = primary
	case OccupationCrafter:
		skills.Crafting = primary
	case OccupationMerchant:
		skills.Trade = primary
	case OccupationSoldier:
		skills.Combat = primary
	case OccupationFisher:
		skills.Farming = primary * 0.8
	case OccupationHunter:
		skills.Combat = primary * 0.7
		skills.Farming = primary * 0.5
	case OccupationScholar, OccupationAlchemist:
		skills.Crafting = primary * 0.6
		skills.Trade = primary * 0.4
	default:
		skills.Farming = primary * 0.5
	}

	return skills
}

func (s *Spawner) generateSoul() AgentSoul {
	// Coherence: most agents are low (Torment or WellBeing).
	// Distribution matches Wheeler: few at extremes.
	coherence := float32(s.rng.Float64() * phi.Matter) // 0–0.618, skewed low
	coherence *= float32(s.rng.Float64())               // Skew further toward low

	// Mass and Gauss: most are moderate (Helium-type).
	mass := float32(s.rng.NormFloat64()*0.2 + 0.35)
	gauss := float32(s.rng.NormFloat64()*0.2 + 0.35)
	mass = clamp32(mass, 0, 1)
	gauss = clamp32(gauss, 0, 1)

	// Agent class: mostly Devotionalist and Ritualist.
	class := s.randomClass()

	soul := AgentSoul{
		CittaCoherence: coherence,
		Mass:           mass,
		Gauss:          gauss,
		Class:          class,
		WisdomScore:    0,
	}
	soul.UpdateState()
	return soul
}

func (s *Spawner) randomClass() AgentClass {
	r := s.rng.Float32()
	switch {
	case r < 0.45:
		return Devotionalist
	case r < 0.80:
		return Ritualist
	case r < 0.97:
		return Nihilist
	default:
		return Transcendentalist // ~3%, extremely rare
	}
}

func (s *Spawner) startingWealth(occ Occupation) uint64 {
	base := uint64(50 + s.rng.Intn(100))
	switch occ {
	case OccupationMerchant:
		base += uint64(s.rng.Intn(200))
	case OccupationCrafter:
		base += uint64(s.rng.Intn(100))
	case OccupationLaborer:
		base -= 20
	}
	return base
}

func (s *Spawner) generateName(sex Sex) string {
	var firsts []string
	if sex == SexMale {
		firsts = maleNames
	} else {
		firsts = femaleNames
	}
	first := firsts[s.rng.Intn(len(firsts))]
	last := lastNames[s.rng.Intn(len(lastNames))]
	return first + " " + last
}

// PromoteToTier2 upgrades the most notable agents in a population to Tier 2.
// Selects based on coherence, wealth, and gauss (ambition).
// Only considers adults (age 16+).
func PromoteToTier2(agents []*Agent, count int) {
	if count <= 0 || len(agents) == 0 {
		return
	}

	// Score each agent for "notability". Only adults qualify.
	type scored struct {
		agent *Agent
		score float64
	}
	var scorable []scored
	for _, a := range agents {
		if a.Age < 16 {
			continue
		}
		s := float64(a.Soul.CittaCoherence)*phi.Nous +
			float64(a.Soul.Gauss)*phi.Being +
			math.Log1p(float64(a.Wealth))*phi.Agnosis
		scorable = append(scorable, scored{a, s})
	}

	// Sort descending.
	for i := 0; i < len(scorable)-1; i++ {
		for j := i + 1; j < len(scorable); j++ {
			if scorable[j].score > scorable[i].score {
				scorable[i], scorable[j] = scorable[j], scorable[i]
			}
		}
	}

	if count > len(scorable) {
		count = len(scorable)
	}
	for i := 0; i < count; i++ {
		scorable[i].agent.Tier = Tier2
	}
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Name pools for procedural generation.
var maleNames = []string{
	"Aldric", "Bram", "Cedric", "Doran", "Erik", "Finn", "Gareth",
	"Halvard", "Ivan", "Jasper", "Kael", "Leif", "Magnus", "Nils",
	"Oswin", "Per", "Quinn", "Rowan", "Stellan", "Theron", "Ulric",
	"Varen", "Wren", "Yorick", "Zander", "Arlen", "Beric", "Cade",
	"Dorian", "Edric", "Falk", "Gunnar", "Hugo", "Ivar", "Jorik",
}

var femaleNames = []string{
	"Astrid", "Brenna", "Calla", "Daria", "Elara", "Freya", "Greta",
	"Helene", "Iris", "Juno", "Kira", "Lena", "Mira", "Nessa",
	"Olwen", "Petra", "Runa", "Senna", "Thea", "Una", "Vera",
	"Willa", "Yara", "Zara", "Ava", "Birgit", "Cora", "Dagny",
	"Eira", "Fern", "Gwen", "Hilde", "Inga", "Johanna", "Katla",
}

var lastNames = []string{
	"Voss", "Thornwood", "Blackwood", "Ashford", "Ironhand", "Dunmore",
	"Greenvale", "Stormcrow", "Frostborn", "Hearthstone", "Millward",
	"Copperfield", "Ravenmoor", "Silverdale", "Wolfsbane", "Stoneheart",
	"Deepwell", "Brightwater", "Oakenshield", "Redforge", "Windholm",
	"Marshwood", "Goldhaven", "Nightingale", "Riverstone", "Steelworth",
	"Embercroft", "Holloway", "Dawnridge", "Farrow", "Wyatt", "Thatcher",
	"Briar", "Caldwell", "Frost", "Harper", "Mercer", "Ward", "Cross",
}
