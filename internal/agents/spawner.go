// Agent spawning — creates the initial population with demographics,
// occupations, soul state, and needs.
// See design doc Sections 4.1, 4.2, 16.2–16.4.
package agents

import (
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

// SetNextID sets the next agent ID to be issued (used when restoring from DB).
func (s *Spawner) SetNextID(id AgentID) {
	s.nextID = id
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
		Wealth:     wealth,
		Skills:     skills,
		Role:       RoleCommoner,
		Tier:       Tier0,
		Wellbeing: WellbeingState{
			Satisfaction: s.rng.Float32()*0.6 - 0.1, // Slightly positive on average
		},
		Soul:     soul,
		Needs:    needs,
		BornTick: 0,
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
		if r < 0.45 {
			return OccupationFarmer
		} else if r < 0.62 {
			return OccupationLaborer
		} else if r < 0.77 {
			return OccupationCrafter
		} else if r < 0.88 {
			return OccupationMerchant
		} else if r < 0.94 {
			return OccupationSoldier
		} else {
			return OccupationScholar
		}
	case world.TerrainMountain:
		if r < 0.40 {
			return OccupationMiner
		} else if r < 0.60 {
			return OccupationLaborer
		} else if r < 0.75 {
			return OccupationCrafter
		} else if r < 0.85 {
			return OccupationMerchant
		} else if r < 0.95 {
			return OccupationSoldier
		} else {
			return OccupationScholar
		}
	case world.TerrainCoast:
		if r < 0.35 {
			return OccupationFisher
		} else if r < 0.55 {
			return OccupationMerchant
		} else if r < 0.72 {
			return OccupationCrafter
		} else if r < 0.85 {
			return OccupationLaborer
		} else if r < 0.93 {
			return OccupationSoldier
		} else {
			return OccupationScholar
		}
	case world.TerrainForest:
		if r < 0.28 {
			return OccupationHunter
		} else if r < 0.45 {
			return OccupationFarmer
		} else if r < 0.60 {
			return OccupationLaborer
		} else if r < 0.74 {
			return OccupationCrafter
		} else if r < 0.82 {
			return OccupationAlchemist
		} else if r < 0.92 {
			return OccupationSoldier
		} else {
			return OccupationScholar
		}
	case world.TerrainSwamp:
		if r < 0.35 {
			return OccupationAlchemist
		} else if r < 0.60 {
			return OccupationHunter
		} else if r < 0.80 {
			return OccupationLaborer
		} else if r < 0.92 {
			return OccupationSoldier
		} else {
			return OccupationScholar
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
	// Coherence: centered on Agnosis (Φ⁻³ ≈ 0.236) — the natural entropy of embodiment.
	// Normal distribution gives range within Embodied: some deeply scattered,
	// some approaching Centered, but nobody spawns enlightened.
	coherence := float32(phi.Agnosis + s.rng.NormFloat64()*phi.Agnosis*0.5)
	coherence = clamp32(coherence, 0.01, float32(phi.Matter)) // 0.01 to 0.618 max

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

// SpawnChild creates a newborn agent in a settlement, inheriting some traits from a parent.
func (s *Spawner) SpawnChild(position world.HexCoord, settlementID uint64, terrain world.Terrain, tick uint64, parent *Agent) *Agent {
	id := s.nextID
	s.nextID++

	sex := SexMale
	if s.rng.Float32() < 0.5 {
		sex = SexFemale
	}

	// Child inherits parent's occupation tendency but with variation.
	occ := s.occupationForTerrain(terrain)
	if s.rng.Float32() < 0.4 {
		occ = parent.Occupation // 40% chance to follow parent's trade
	}

	skills := SkillSet{
		Farming:  0.05 + s.rng.Float32()*0.05,
		Mining:   0.05 + s.rng.Float32()*0.05,
		Crafting: 0.05 + s.rng.Float32()*0.05,
		Combat:   0.02 + s.rng.Float32()*0.05,
		Trade:    0.05 + s.rng.Float32()*0.05,
	}

	// Soul: centered on Agnosis with slight influence from parent (cultural transmission).
	coherence := float32(phi.Agnosis + s.rng.NormFloat64()*phi.Agnosis*0.5)
	coherence = clamp32(coherence, 0.01, float32(phi.Matter))
	// Small boost from parent wisdom.
	coherence += parent.Soul.CittaCoherence * float32(phi.Agnosis)
	if coherence > 1 {
		coherence = 1
	}

	soul := AgentSoul{
		CittaCoherence: coherence,
		Mass:           float32(s.rng.NormFloat64()*0.2 + 0.35),
		Gauss:          float32(s.rng.NormFloat64()*0.2 + 0.35),
		Class:          s.randomClass(),
	}
	soul.Mass = clamp32(soul.Mass, 0, 1)
	soul.Gauss = clamp32(soul.Gauss, 0, 1)
	soul.UpdateState()

	sid := settlementID
	return &Agent{
		ID:         id,
		Name:       s.generateName(sex),
		Age:        0,
		Sex:        sex,
		Health:     1.0,
		Position:   position,
		HomeSettID: &sid,
		Occupation: occ,
		Wealth:     0,
		Skills:     skills,
		Role:       RoleCommoner,
		Tier:       Tier0,
		Wellbeing: WellbeingState{
			Satisfaction: 0.3 + s.rng.Float32()*0.3, // Babies start happy
		},
		Soul: soul,
		Needs: NeedsState{
			Survival:  0.8,
			Safety:    0.7,
			Belonging: 0.8, // High belonging (family)
			Esteem:    0.1,
			Purpose:   0.1,
		},
		BornTick: tick,
		Alive:    true,
	}
}

// PromoteToTier2 upgrades the most notable agents in a population to Tier 2.
// Selects based on coherence and gauss (ambition) — inner qualities, not wealth.
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
		// Notability = coherence (soul depth) + gauss (ambition).
		// Wealth removed — a poor fisherman with deep coherence
		// is more notable than a rich merchant with a scattered soul.
		s := float64(a.Soul.CittaCoherence)*phi.Nous +
			float64(a.Soul.Gauss)*phi.Being
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

// PromoteToTier1 upgrades a fraction of Tier 0 agents to Tier 1 (archetype-guided).
// Selects agents with medium coherence (0.3–0.6) who are adults.
func PromoteToTier1(agents []*Agent, fraction float64) {
	if fraction <= 0 || len(agents) == 0 {
		return
	}

	// Collect eligible Tier 0 adults with medium coherence.
	var eligible []*Agent
	for _, a := range agents {
		if a.Tier == Tier0 && a.Alive && a.Age >= 16 &&
			a.Soul.CittaCoherence >= 0.15 && a.Soul.CittaCoherence <= 0.6 {
			eligible = append(eligible, a)
		}
	}

	count := int(float64(len(eligible)) * fraction)
	if count <= 0 {
		return
	}
	if count > len(eligible) {
		count = len(eligible)
	}

	// Score by coherence distance from 0.4 (prefer mid-range) + gauss (ambition).
	type scored struct {
		agent *Agent
		score float64
	}
	var scorable []scored
	for _, a := range eligible {
		// Prefer agents near 0.4 coherence with some drive.
		dist := float64(a.Soul.CittaCoherence - 0.4)
		if dist < 0 {
			dist = -dist
		}
		s := (1.0 - dist) + float64(a.Soul.Gauss)*0.5
		scorable = append(scorable, scored{a, s})
	}

	// Sort descending by score.
	for i := 0; i < len(scorable)-1; i++ {
		for j := i + 1; j < len(scorable); j++ {
			if scorable[j].score > scorable[i].score {
				scorable[i], scorable[j] = scorable[j], scorable[i]
			}
		}
	}

	for i := 0; i < count; i++ {
		a := scorable[i].agent
		a.Tier = Tier1
		a.Archetype = AssignArchetype(a)
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
