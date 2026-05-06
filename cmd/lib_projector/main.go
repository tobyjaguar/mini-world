// Liberation Redesign Projector — agent-based simulation of coherence dynamics
// to validate Doc 25 Layer 1 + Layer 2 constants before committing to R88+.
//
// What this does (and doesn't do):
//
//   DOES model — at sim-week granularity:
//     • Cultural drift (perpetuation.go:77, ages 14-25)
//     • Faction doctrine boost (factions.go:710-712)
//     • Scholar work coherence (behavior.go:291)
//     • Tier 1 archetype growth (archetype.go:222-225)
//     • Mentorship growth (relationships.go:217-218)
//     • Witness ordinary death gain (population.go:206-208 + simulation.go:374-378)
//     • Witness liberation death ripple (population.go:195-197)
//     • Baseline coherence drift (simulation.go:990-993)
//     • Universal trauma decay (NEW, Layer 1)
//     • Active practice / WisdomEffort (NEW, Layer 2)
//     • Births / deaths (replacement at population cap)
//
//   DOES NOT model (would over-engineer):
//     • Individual settlement state (uses world-average conflict/governance)
//     • Per-agent eligibility for practice based on Survival/Safety/Belonging
//       (assumes a configurable fraction of agents meet the four-foundations
//       gate at any given week)
//     • Reincarnation (Layer 3) — separate spreadsheet exercise
//     • Migration / monastic settlements (Layer 4) — out of scope for this run
//     • LLM-driven Tier 2 contemplation choice (uses class-weighted probability)
//
// Runs comparison scenarios (see runScenarios()):
//
//   "current"   — current rules, baseline against production
//   "layer1"    — Layer 1 cuts only (stop the candy machine)
//   "layer1+2"  — Layer 1 + Layer 2 active practice with split fields
//
// Outputs a summary table to stdout. Population at convergence is what matters
// — relative comparison of the three scenarios validates Layer 1 + Layer 2
// magnitudes against the targets in Doc 25 §3.
//
// Usage:
//   go run ./cmd/lib_projector
//   go run ./cmd/lib_projector -seed=42 -weeks=520 -agents=10000
//   go run ./cmd/lib_projector -tune  # iterate over Layer 1 magnitudes
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/talgya/mini-world/internal/phi"
)

const (
	NumOccupations = 10
	NumClasses     = 4
	NumFactions    = 5
)

// Occupation indices (match agents.Occupation enum)
const (
	OccFarmer = iota
	OccMiner
	OccCrafter
	OccMerchant
	OccSoldier
	OccScholar
	OccAlchemist
	OccLaborer
	OccFisher
	OccHunter
)

// Class indices (match agents.AgentClass enum)
const (
	ClassDevotionalist = iota
	ClassRitualist
	ClassNihilist
	ClassTranscendentalist
)

// Faction IDs (match social.FactionID; 0 = unaffiliated)
const (
	FactionNone = iota
	FactionCrown
	FactionMerchant
	FactionIron
	FactionVerdant
	FactionAshen
)

var occNames = []string{"Farmer", "Miner", "Crafter", "Merchant", "Soldier", "Scholar", "Alchemist", "Laborer", "Fisher", "Hunter"}
var classNames = []string{"Devotionalist", "Ritualist", "Nihilist", "Transcendentalist"}

// Production distribution (pulled from /api/v1/status, 2026-05-06).
var occupationDistribution = [NumOccupations]float64{
	OccFarmer:    0.4742,
	OccMiner:     0.0139,
	OccCrafter:   0.0297,
	OccMerchant:  0.0557,
	OccSoldier:   0.0626,
	OccScholar:   0.0588,
	OccAlchemist: 0.1362,
	OccLaborer:   0.0586,
	OccFisher:    0.0937,
	OccHunter:    0.0168,
}

var classDistribution = [NumClasses]float64{
	ClassDevotionalist:     0.45,
	ClassRitualist:         0.35,
	ClassNihilist:          0.17,
	ClassTranscendentalist: 0.03,
}

// Approximate faction membership per the live world (from /api/v1/factions).
// Children get faction at birth so this distribution applies at all ages.
var factionDistribution = [NumFactions + 1]float64{
	FactionNone:     0.10, // unaffiliated children
	FactionCrown:    0.23,
	FactionMerchant: 0.05,
	FactionIron:     0.07,
	FactionVerdant:  0.33,
	FactionAshen:    0.12,
}

// SimAgent — minimal agent state for the projector.
type SimAgent struct {
	Age          int
	Occupation   int
	Class        int
	FactionID    int
	Coherence    float64
	WisdomEffort uint32
	Tier         int     // 0, 1, or 2
	Wealth       float64 // for Ashen doctrine eligibility check
	HexHealth    float64 // for VC doctrine eligibility check
}

// IsLiberated under the proposed split-fields criterion (Layer 2).
func (a *SimAgent) IsLiberated(useSplitFields bool, gate uint32) bool {
	if !useSplitFields {
		return a.Coherence >= 0.7
	}
	return a.Coherence >= 0.7 && a.WisdomEffort >= gate
}

// FulfillsDoctrine — simplified version of agentFulfillsDoctrine().
// Crown: settlement well-governed (assumed true ~75% of weeks)
// Merchant: active merchant (true if Occupation == Merchant ~80% of weeks)
// Iron: soldier in governed settlement (Occupation == Soldier ~75% of weeks)
// Verdant: worked recently + healthy hex (true ~50% of weeks for producers)
// Ashen: wealth < 30 OR belonging > Matter (children always; adults often)
func (a *SimAgent) FulfillsDoctrine(rng *rand.Rand) bool {
	switch a.FactionID {
	case FactionCrown:
		return rng.Float64() < 0.75
	case FactionMerchant:
		return a.Occupation == OccMerchant && rng.Float64() < 0.80
	case FactionIron:
		return a.Occupation == OccSoldier && rng.Float64() < 0.75
	case FactionVerdant:
		producers := a.Occupation == OccFarmer || a.Occupation == OccFisher ||
			a.Occupation == OccHunter || a.Occupation == OccMiner ||
			a.Occupation == OccAlchemist || a.Occupation == OccLaborer
		return producers && rng.Float64() < 0.50 && a.HexHealth > phi.Psyche
	case FactionAshen:
		// Children always (wealth < 30); adults sometimes
		if a.Age < 16 {
			return true
		}
		return a.Wealth < 30 || rng.Float64() < 0.40 // belonging > Matter approx
	}
	return false
}

// Practice eligibility (Layer 2): four-foundations gate.
// Approximation: about 60% of adults have all four foundations met at any given week.
// Children excluded. Note: a.Age is in WEEKS, not years.
func (a *SimAgent) PracticeEligible(rng *rand.Rand) bool {
	if a.Age < 16*52 {
		return false
	}
	return rng.Float64() < 0.60
}

// Mortality probability per sim-week, age-dependent.
// Calibrated against production's observed ~80 deaths/week per 400K agents
// (0.02%/week ≈ 1%/year average), with sigmoid acceleration after 50.
// Agents reach age 70-100 in production, matching this curve.
func mortalityWeekly(age int) float64 {
	base := 0.0002 // ~1% / year baseline
	if age < 16 {
		base *= 0.5 // children slightly safer (no occupational hazards)
	}
	if age >= 50 {
		// Sigmoid acceleration: at 70 → ~5%/yr, at 90 → ~30%/yr
		x := float64(age-50) / 15.0
		s := 1.0 / (1.0 + expNeg(x))
		return base + 0.006*s*s
	}
	return base
}

func expNeg(x float64) float64 {
	return math.Exp(-x)
}

// Config — all the magnitudes we want to tune.
type Config struct {
	Name string

	// Layer 1 magnitudes (per sim-week unless noted)
	CulturalDriftRate     float64 // per week, ages 14-CulturalDriftEndAge
	CulturalDriftEndAge   int
	DoctrineBoostRate     float64 // per week
	DoctrineMinAge        int
	ScholarWorkRatePerWk  float64 // per week (continuous-time approximation)
	Tier1ArchetypeRate    float64 // per week, average across archetypes
	MentorshipPerEvent    float64 // per pairing event
	MentorshipPerYear     float64 // pairings per agent per year
	WitnessDeathGain      float64 // per witnessed death
	WitnessDeathsPerYear  float64 // ordinary deaths witnessed per agent per year
	SageDeathRipple       float64 // per witnessed sage death (negative)
	BaselineDriftRate     float64 // per week, satisfied adults
	BaselineEligible      float64 // fraction of adults satisfied enough
	OracleBlessRate       float64 // per agent per year
	OracleBlessAmount     float64
	OracleBlessCap        float64

	// Layer 1 NEW — universal trauma decay
	TraumaPerEvent    float64 // base intensity
	TraumaLibMult     float64 // multiplier for liberated agents
	TraumaPerYear     float64 // events per agent per year (war/plague/famine/theft)
	TraumaCap         float64

	// Layer 2 — active practice
	UseSplitFields       bool
	UseActivePractice    bool
	WisdomEffortGate     uint32
	BasePracticeProb     float64    // per HOUR when eligible
	OccupationConducive  [NumOccupations]float64
	ClassIntention       [NumClasses]float64
	InsightProb          float64
	InsightCoherenceGain float64

	// Architectural choice: cap all natural inflows at Matter.
	// Only practice insight (Layer 2) can bridge from Matter to Liberation.
	// Trauma decay can still push below Matter; reincarnation can bypass.
	NaturalCap float64 // 0 = no cap; Matter = Layer 1+ cap

	// Birth coherence model
	BirthBaseCoherence float64 // mean
	BirthBaseStdev     float64
	ParentBoostRatio   float64 // multiplier on parent coherence
	BirthHardCap       float64 // 0 = no cap (current); Matter = Layer 1+ cap

	// Liberation threshold (kept at 0.7 unchanged across scenarios)
	LiberationThreshold float64
}

// CurrentRulesConfig — production behavior as of 2026-05-06.
func CurrentRulesConfig() Config {
	return Config{
		Name:                  "current",
		CulturalDriftRate:     phi.Agnosis * 0.005,         // +0.00118/week
		CulturalDriftEndAge:   25,
		DoctrineBoostRate:     phi.Agnosis * phi.Agnosis * 0.1, // +0.00557/week
		DoctrineMinAge:        0, // no age gate
		ScholarWorkRatePerWk:  phi.Agnosis * 0.000001 * 60 * 24 * 7, // tick→week (60min/hr × 24hr × 7d)
		Tier1ArchetypeRate:    phi.Agnosis * 0.03 * 7, // average archetype, daily → weekly
		MentorshipPerEvent:    phi.Agnosis * 0.05,
		MentorshipPerYear:     8,
		WitnessDeathGain:      phi.Agnosis * 0.05, // both paths combined approx
		WitnessDeathsPerYear:  20,
		SageDeathRipple:       phi.Agnosis * 0.05,
		BaselineDriftRate:     phi.Agnosis * 0.001 * 7, // daily → weekly
		BaselineEligible:      0.75,
		OracleBlessRate:       0.5, // ~rare per agent per year
		OracleBlessAmount:     phi.Agnosis * 0.1,
		OracleBlessCap:        0.7,

		TraumaPerEvent: 0,
		TraumaLibMult:  1.0,
		TraumaPerYear:  0, // no universal trauma decay in current rules

		UseSplitFields:    false,
		UseActivePractice: false,

		BirthBaseCoherence:  phi.Agnosis,
		BirthBaseStdev:      phi.Agnosis * 0.5,
		ParentBoostRatio:    phi.Agnosis,
		LiberationThreshold: 0.7,
	}
}

// Layer1Config — Doc 25 §3.1 proposed cuts.
// Revised after first projector run (2026-05-06): aggregate effect was too
// strong; relaxed individual cuts to match the rare-by-design target band
// (1-3%) without over-correcting. Birth coherence cap added — newborns of
// liberated parents otherwise inherit liberation status (147 children
// observed liberated under the over-aggressive Layer 1 v0).
func Layer1Config() Config {
	c := CurrentRulesConfig()
	c.Name = "layer1"
	c.CulturalDriftRate = phi.Agnosis * 0.0017   // ~3× cut (was 10× over-aggressive)
	c.CulturalDriftEndAge = 22                    // narrowed (unchanged)
	c.DoctrineBoostRate = phi.Agnosis * phi.Agnosis * 0.040 // 2.5× cut (was 4×)
	c.DoctrineMinAge = 16                         // age gate
	c.ScholarWorkRatePerWk = phi.Agnosis * 0.0000003 * 60 * 24 * 7 // ~3× cut
	c.Tier1ArchetypeRate = phi.Agnosis * 0.010 * 7 // 3× cut (mostly defensive — only 1 Tier 1 in production)
	c.MentorshipPerEvent = phi.Agnosis * 0.025     // 2× cut
	c.WitnessDeathGain = phi.Agnosis * 0.025       // 2× cut
	c.SageDeathRipple = phi.Agnosis * 0.05         // unchanged

	// Trauma decay: rare but meaningful. Modeled as 1 event per ~3 years average
	// (most agents in peaceful settlements rarely face existential trauma; war
	// zones bring it higher but average across world is low).
	c.TraumaPerEvent = phi.Agnosis * 0.05 // ~0.012 base coherence cost per event
	c.TraumaLibMult = 2.0                 // liberated agents take 2× (extraction paradox)
	c.TraumaPerYear = 0.33                // ~1 event per 3 years for the average agent
	c.TraumaCap = phi.Agnosis * 0.5       // single-event cap ≈ 0.118

	// NEW: cap birth coherence at Matter — newborns can no longer inherit
	// liberation status from parents. Critical: this is the dominant child-
	// liberation path under Layer 1 v0 (untouched original birth formula).
	c.ParentBoostRatio = phi.Agnosis * 0.5 // half the original parent boost
	c.BirthHardCap = phi.Matter             // hard cap at Matter — no birth-liberation

	// NEW (architectural): natural inflows cap at Matter. The Awakening
	// Valley (Matter to 0.7) cannot be crossed by drift. Only Layer 2
	// active practice (insight events) can bridge.
	c.NaturalCap = phi.Matter
	return c
}

// Layer1Plus2Config — Layer 1 + active practice.
// Architectural choice: NaturalCap at Matter prevents drift-liberation
// (so the AND-condition is unnecessary). WisdomEffort still accumulates
// but is a metadata counter, not a gate. Liberation criterion stays
// `c >= 0.7`, which now requires bridging the Awakening Valley via
// practice insights that bypass the natural cap.
func Layer1Plus2Config() Config {
	c := Layer1Config()
	c.Name = "layer1+2"
	c.UseSplitFields = false // unnecessary under cap architecture
	c.UseActivePractice = true
	c.WisdomEffortGate = 0 // not used as gate; counter only

	c.BasePracticeProb = phi.Agnosis * phi.Agnosis * phi.Agnosis // ≈0.013/hour
	// Practice produces both gradual deepening (samatha — every tick) and
	// punctuated insights (vipassanā — rare, larger). Calibrated so a
	// Scholar × Transcendentalist combo bridges Matter→Liberation in
	// ~14 sim-years of consistent practice; Hunter × Devotionalist takes
	// ~50+ years (rare lifetime achievement); Laborer × Nihilist effectively
	// never (>200 years required).
	c.InsightProb = 0.01                                          // 1% of practice ticks produce insight
	c.InsightCoherenceGain = 0.025                                // size of insight bump
	// Effective per-tick gain (gradual + insight) ≈ 0.000_350 for the
	// Scholar/Transcendentalist multiplier 2.62 — see comment in applyPaths.

	c.OccupationConducive = [NumOccupations]float64{
		OccFarmer:    phi.Agnosis,
		OccMiner:     phi.Agnosis,
		OccCrafter:   phi.Agnosis,
		OccMerchant:  phi.Matter,
		OccSoldier:   phi.Psyche,
		OccScholar:   phi.Being,
		OccAlchemist: phi.Being / phi.Phi,
		OccLaborer:   phi.Agnosis,
		OccFisher:    phi.Psyche,
		OccHunter:    phi.Matter,
	}
	c.ClassIntention = [NumClasses]float64{
		ClassDevotionalist:     phi.Matter,
		ClassRitualist:         phi.Psyche,
		ClassNihilist:          phi.Agnosis,
		ClassTranscendentalist: phi.Being,
	}
	return c
}

// Run — execute the projection and return weekly liberation %.
type RunResult struct {
	WeeklyLiberatedPct []float64
	FinalAgents        []SimAgent
	TotalDeaths        int
	TotalBirths        int
}

func runProjection(cfg Config, seed int64, weeks int, agentCount int, ageDistribution string) RunResult {
	rng := rand.New(rand.NewSource(seed))
	agents := generatePopulation(rng, agentCount, ageDistribution, cfg)

	result := RunResult{
		WeeklyLiberatedPct: make([]float64, 0, weeks),
	}

	for w := 0; w < weeks; w++ {
		// 1. Apply each coherence path
		applyPaths(rng, agents, cfg)

		// 2. Births / deaths (maintain population stability)
		alive := agents[:0]
		for i := range agents {
			a := &agents[i]
			a.Age++ // sim-week aging is too granular; use 1 year per 52 weeks
			// Actually simpler: stochastic mortality per week
			pDie := mortalityWeekly(a.Age / 52)
			if rng.Float64() < pDie {
				result.TotalDeaths++
				continue
			}
			alive = append(alive, *a)
		}
		// Replenish to target population
		for len(alive) < agentCount {
			alive = append(alive, spawnNewborn(rng, alive, cfg))
			result.TotalBirths++
		}
		agents = alive

		// 3. Tick liberation %
		libCount := 0
		for i := range agents {
			if agents[i].IsLiberated(cfg.UseSplitFields, cfg.WisdomEffortGate) {
				libCount++
			}
		}
		result.WeeklyLiberatedPct = append(result.WeeklyLiberatedPct, float64(libCount)/float64(len(agents)))
	}

	result.FinalAgents = agents
	return result
}

func applyPaths(rng *rand.Rand, agents []SimAgent, cfg Config) {
	for i := range agents {
		a := &agents[i]

		// PASS 1: Natural drift inflows. These can only fill the band BELOW
		// NaturalCap. If the agent is already above NaturalCap (via practice
		// or reincarnation), drift contributes 0 — natural processes can't
		// ascend past Matter; only practice can.
		startCoherence := a.Coherence
		ageY := a.Age / 52

		if cfg.NaturalCap == 0 || a.Coherence < cfg.NaturalCap {
			// Cultural drift (ages 14-CulturalDriftEndAge)
			if ageY >= 14 && ageY < cfg.CulturalDriftEndAge {
				a.Coherence += cfg.CulturalDriftRate
			}

			// Doctrine boost
			if a.FactionID != FactionNone && ageY >= cfg.DoctrineMinAge {
				if a.FulfillsDoctrine(rng) {
					a.Coherence += cfg.DoctrineBoostRate
				}
			}

			// Scholar work
			if a.Occupation == OccScholar {
				a.Coherence += cfg.ScholarWorkRatePerWk
			}

			// Tier 1 archetype
			if a.Tier == 1 {
				a.Coherence += cfg.Tier1ArchetypeRate
			}

			// Mentorship — Poisson-approximation
			mentorshipExpected := cfg.MentorshipPerYear / 52.0
			if rng.Float64() < mentorshipExpected {
				a.Coherence += cfg.MentorshipPerEvent
			}

			// Witness ordinary deaths
			witnessExpected := cfg.WitnessDeathsPerYear / 52.0
			nWitnesses := poisson(rng, witnessExpected)
			for n := 0; n < nWitnesses; n++ {
				gain := cfg.WitnessDeathGain * (1 - 0.5*a.Coherence)
				a.Coherence += gain
			}

			// Baseline drift
			if ageY >= 20 && rng.Float64() < cfg.BaselineEligible {
				a.Coherence += cfg.BaselineDriftRate
			}

			// Oracle bless (rare; capped at OracleBlessCap which is below 0.7)
			if rng.Float64() < cfg.OracleBlessRate/52.0 {
				newC := a.Coherence + cfg.OracleBlessAmount
				if newC > cfg.OracleBlessCap {
					newC = cfg.OracleBlessCap
				}
				if newC > a.Coherence {
					a.Coherence = newC
				}
			}

			// Apply natural cap to any drift inflows above (clamp drift only,
			// preserve any practice/reincarnation gains the agent already had).
			if cfg.NaturalCap > 0 && startCoherence < cfg.NaturalCap && a.Coherence > cfg.NaturalCap {
				a.Coherence = cfg.NaturalCap
			}
		}

		// Universal trauma decay (Layer 1 NEW)
		if cfg.TraumaPerYear > 0 {
			traumaExpected := cfg.TraumaPerYear / 52.0
			nTrauma := poisson(rng, traumaExpected)
			for n := 0; n < nTrauma; n++ {
				intensity := 0.3 + rng.Float64()*0.7
				decay := cfg.TraumaPerEvent * intensity
				if a.Coherence >= 0.7 {
					decay *= cfg.TraumaLibMult
				}
				if decay > cfg.TraumaCap {
					decay = cfg.TraumaCap
				}
				a.Coherence -= decay
			}
		}

		// Active practice (Layer 2)
		// Practice has two effects:
		//   (1) Gradual deepening (samatha): every tick adds a tiny coherence
		//       gain. This is the slow, steady cultivation.
		//   (2) Punctuated insights (vipassanā): rare ticks produce a larger
		//       bump. This is the breakthrough moment.
		// Both BYPASS the natural cap at Matter — practice is the only path
		// across the Awakening Valley to Liberation.
		if cfg.UseActivePractice {
			if a.PracticeEligible(rng) {
				occW := cfg.OccupationConducive[a.Occupation]
				clsW := cfg.ClassIntention[a.Class]
				// Hours per week of eligibility (simplified): 20 hours of "free time"
				const eligibleHours = 20.0
				const samathaPerTick = 0.0005 // gradual deepening per practice tick (calibrated 2026-05-06 to land mean ~1.5% liberation in 80yr run)
				practiceProb := cfg.BasePracticeProb * occW * clsW
				expectedTicks := practiceProb * eligibleHours
				ticks := poisson(rng, expectedTicks)
				a.WisdomEffort += uint32(ticks)
				for n := 0; n < ticks; n++ {
					a.Coherence += samathaPerTick // gradual
					if rng.Float64() < cfg.InsightProb {
						a.Coherence += cfg.InsightCoherenceGain // breakthrough
					}
				}
			}
		}

		// Floor + ceiling clamp (separate from NaturalCap; agents can't go above 1.0)
		if a.Coherence < 0 {
			a.Coherence = 0
		}
		if a.Coherence > 1 {
			a.Coherence = 1
		}
	}
}

func poisson(rng *rand.Rand, lambda float64) int {
	if lambda < 0.001 {
		if rng.Float64() < lambda {
			return 1
		}
		return 0
	}
	// Knuth's algorithm
	L := expNeg(lambda)
	k := 0
	p := 1.0
	for {
		k++
		p *= rng.Float64()
		if p <= L {
			return k - 1
		}
		if k > 50 { // bail out
			return k
		}
	}
}

func generatePopulation(rng *rand.Rand, n int, ageDistribution string, cfg Config) []SimAgent {
	agents := make([]SimAgent, n)
	for i := range agents {
		agents[i] = generateAgent(rng, ageDistribution, cfg)
	}
	return agents
}

func generateAgent(rng *rand.Rand, ageDistribution string, cfg Config) SimAgent {
	var age int
	switch ageDistribution {
	case "uniform":
		age = rng.Intn(80 * 52)
	case "production": // skewed older, matches live world
		// Approximation: 25% under 16, 50% 16-50, 25% 50+
		r := rng.Float64()
		switch {
		case r < 0.25:
			age = rng.Intn(16 * 52)
		case r < 0.75:
			age = (16 + rng.Intn(34)) * 52
		default:
			age = (50 + rng.Intn(40)) * 52
		}
	default:
		age = 0
	}

	occ := pickWeighted(rng, occupationDistribution[:])
	class := pickWeighted(rng, classDistribution[:])
	faction := pickWeighted(rng, factionDistribution[:])

	a := SimAgent{
		Age:        age,
		Occupation: occ,
		Class:      class,
		FactionID:  faction,
		Tier:       0,
		Wealth:     50 + rng.Float64()*200,
		HexHealth:  0.3 + rng.Float64()*0.6,
	}

	// Initial coherence: spawner generates from Normal(Agnosis, Agnosis*0.5) clamped [0.01, Matter]
	c := cfg.BirthBaseCoherence + rng.NormFloat64()*cfg.BirthBaseStdev
	if c < 0.01 {
		c = 0.01
	}
	if c > phi.Matter {
		c = phi.Matter
	}
	a.Coherence = c

	// 1 in 25 agents is Tier 1
	if rng.Float64() < 0.04 {
		a.Tier = 1
	}

	return a
}

func spawnNewborn(rng *rand.Rand, currentAgents []SimAgent, cfg Config) SimAgent {
	occ := pickWeighted(rng, occupationDistribution[:])
	class := pickWeighted(rng, classDistribution[:])
	faction := pickWeighted(rng, factionDistribution[:])

	parentCoherence := phi.Agnosis
	if len(currentAgents) > 0 {
		// Pick a random adult parent
		for tries := 0; tries < 5; tries++ {
			p := currentAgents[rng.Intn(len(currentAgents))]
			if p.Age >= 16*52 {
				parentCoherence = p.Coherence
				break
			}
		}
	}

	c := cfg.BirthBaseCoherence + rng.NormFloat64()*cfg.BirthBaseStdev
	if c < 0.01 {
		c = 0.01
	}
	if c > phi.Matter {
		c = phi.Matter
	}
	c += parentCoherence * cfg.ParentBoostRatio
	// Layer 1 hard cap: newborns cannot be born above Matter (no inherited
	// liberation status). Pre-Embodied is the natural state of birth.
	if cfg.BirthHardCap > 0 && c > cfg.BirthHardCap {
		c = cfg.BirthHardCap
	} else if c > 1 {
		c = 1
	}

	return SimAgent{
		Age:        0,
		Occupation: occ,
		Class:      class,
		FactionID:  faction,
		Coherence:  c,
		Tier:       0,
		Wealth:     0,
		HexHealth:  0.3 + rng.Float64()*0.6,
	}
}

func pickWeighted(rng *rand.Rand, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := rng.Float64() * total
	for i, w := range weights {
		r -= w
		if r <= 0 {
			return i
		}
	}
	return len(weights) - 1
}

// Summary stats for end-of-run analysis.
type Summary struct {
	FinalLiberated   float64
	MedianAge        int
	LiberatedByOcc   map[string]float64
	LiberatedByClass map[string]float64
	LiberatedUnder16 int
	TotalAgents      int
}

func summarize(result RunResult, cfg Config) Summary {
	s := Summary{
		LiberatedByOcc:   make(map[string]float64),
		LiberatedByClass: make(map[string]float64),
	}
	if len(result.WeeklyLiberatedPct) > 0 {
		s.FinalLiberated = result.WeeklyLiberatedPct[len(result.WeeklyLiberatedPct)-1]
	}
	s.TotalAgents = len(result.FinalAgents)

	libByOcc := make(map[int]int)
	libByClass := make(map[int]int)
	occCount := make(map[int]int)
	classCount := make(map[int]int)
	libAges := []int{}

	for i := range result.FinalAgents {
		a := &result.FinalAgents[i]
		ageY := a.Age / 52
		occCount[a.Occupation]++
		classCount[a.Class]++
		if a.IsLiberated(cfg.UseSplitFields, cfg.WisdomEffortGate) {
			libByOcc[a.Occupation]++
			libByClass[a.Class]++
			libAges = append(libAges, ageY)
			if ageY < 16 {
				s.LiberatedUnder16++
			}
		}
	}

	for occ, cnt := range libByOcc {
		s.LiberatedByOcc[occNames[occ]] = float64(cnt) / float64(occCount[occ])
	}
	for cls, cnt := range libByClass {
		s.LiberatedByClass[classNames[cls]] = float64(cnt) / float64(classCount[cls])
	}

	if len(libAges) > 0 {
		sort.Ints(libAges)
		s.MedianAge = libAges[len(libAges)/2]
	}

	return s
}

func main() {
	seed := flag.Int64("seed", 42, "RNG seed")
	weeks := flag.Int("weeks", 520, "sim-weeks to project (520 = 10 sim-years)")
	agents := flag.Int("agents", 10000, "synthetic population size")
	mode := flag.String("mode", "compare", "compare | tune")
	startAge := flag.String("start", "production", "uniform | production")
	flag.Parse()

	switch *mode {
	case "compare":
		runCompare(*seed, *weeks, *agents, *startAge)
	case "tune":
		runTune(*seed, *weeks, *agents, *startAge)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func debugTopAgents(result RunResult, cfg Config) {
	// Sort by coherence descending and print top 10
	agents := make([]SimAgent, len(result.FinalAgents))
	copy(agents, result.FinalAgents)
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Coherence > agents[j].Coherence
	})
	limit := 10
	if len(agents) < limit {
		limit = len(agents)
	}
	fmt.Printf("--- %s top %d agents by coherence ---\n", cfg.Name, limit)
	fmt.Printf("%-3s %-10s %-12s %-18s %-10s %-12s\n", "#", "Age(yr)", "Occupation", "Class", "Coherence", "WisdomEffort")
	for i := 0; i < limit; i++ {
		a := agents[i]
		fmt.Printf("%-3d %-10d %-12s %-18s %-10.4f %-12d\n",
			i+1, a.Age/52, occNames[a.Occupation], classNames[a.Class],
			a.Coherence, a.WisdomEffort)
	}
}

func runCompare(seed int64, weeks, agents int, startAge string) {
	configs := []Config{CurrentRulesConfig(), Layer1Config(), Layer1Plus2Config()}

	fmt.Printf("\n=== Liberation Projection (seed=%d weeks=%d agents=%d start=%s) ===\n\n", seed, weeks, agents, startAge)
	fmt.Printf("%-12s %-10s %-12s %-12s %-12s\n", "Scenario", "Final %", "Median age", "Under 16", "Total agents")
	fmt.Printf("%s\n", "------------------------------------------------------------")

	for _, cfg := range configs {
		result := runProjection(cfg, seed, weeks, agents, startAge)
		s := summarize(result, cfg)
		fmt.Printf("%-12s %-10.2f %-12d %-12d %-12d\n",
			cfg.Name, s.FinalLiberated*100, s.MedianAge, s.LiberatedUnder16, s.TotalAgents)
	}

	fmt.Println()
	for _, cfg := range configs {
		result := runProjection(cfg, seed, weeks, agents, startAge)
		s := summarize(result, cfg)
		debugTopAgents(result, cfg)
		fmt.Println()
		fmt.Printf("--- %s by occupation (frac liberated within occ) ---\n", cfg.Name)
		// stable order
		ordered := []string{"Scholar", "Alchemist", "Hunter", "Merchant", "Fisher", "Soldier", "Farmer", "Crafter", "Miner", "Laborer"}
		for _, name := range ordered {
			fmt.Printf("  %-10s %.2f%%\n", name, s.LiberatedByOcc[name]*100)
		}
		fmt.Printf("--- %s by class ---\n", cfg.Name)
		for _, name := range []string{"Transcendentalist", "Devotionalist", "Ritualist", "Nihilist"} {
			fmt.Printf("  %-18s %.2f%%\n", name, s.LiberatedByClass[name]*100)
		}
		fmt.Println()
	}

	// Trajectory snapshot every 10% of run
	fmt.Println("--- Trajectory: liberated % over time (snapshot every 10% of run) ---")
	fmt.Printf("%-8s ", "Week")
	for _, cfg := range configs {
		fmt.Printf("%-12s ", cfg.Name)
	}
	fmt.Println()
	results := make([]RunResult, len(configs))
	for i, cfg := range configs {
		results[i] = runProjection(cfg, seed, weeks, agents, startAge)
	}
	for w := 0; w < weeks; w += weeks / 10 {
		fmt.Printf("%-8d ", w)
		for _, r := range results {
			if w < len(r.WeeklyLiberatedPct) {
				fmt.Printf("%-12.2f ", r.WeeklyLiberatedPct[w]*100)
			}
		}
		fmt.Println()
	}
	fmt.Printf("%-8d ", weeks-1)
	for _, r := range results {
		if len(r.WeeklyLiberatedPct) > 0 {
			fmt.Printf("%-12.2f ", r.WeeklyLiberatedPct[len(r.WeeklyLiberatedPct)-1]*100)
		}
	}
	fmt.Println()
}

func runTune(seed int64, weeks, agents int, startAge string) {
	fmt.Println("=== Tuning Layer 1 cultural drift cut magnitude ===")
	for _, cut := range []float64{1, 2, 5, 10, 20, 50} {
		c := Layer1Config()
		c.CulturalDriftRate = phi.Agnosis * 0.005 / cut
		c.Name = fmt.Sprintf("layer1-cut-%vx", cut)
		result := runProjection(c, seed, weeks, agents, startAge)
		s := summarize(result, c)
		fmt.Printf("  cut=%-4v  final-liberated=%5.2f%%  median-age=%-4d  under-16-count=%d\n",
			cut, s.FinalLiberated*100, s.MedianAge, s.LiberatedUnder16)
	}
}
