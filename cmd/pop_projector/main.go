// Command pop_projector is a demographic/mortality projector for Crossworlds.
//
// Unlike cmd/lib_projector (which models liberation/coherence trajectories),
// this projector models the POPULATION dynamics that drive the W-19 winter
// illness die-off, the W-20 age-structure collapse, and the W-22 adolescent
// mortality wall:
//
//   - age structure (live-seeded via -seed-agents, or the 2026-05-26 histogram)
//   - agent Health (R97-1 recovery toward Matter + c·(Nous−Matter))
//   - the survival↔rest↔health coupling (health self-regulates near the 0.30
//     rest trigger, but survival-stressed agents can't rest and ratchet down)
//   - winter hardship (−0.05 Health/winter to warmth-less agents, once per
//     mechanical season via OnSeason)
//   - illness death (Health<0.15 → −0.01/day → death; NOT age-gated in prod)
//   - coherence/age/over-capacity natural mortality (population.go math)
//   - births (processBirths gate: ≥2 co-located adults 18-45, Health>0.5,
//     Survival>0.3) with newborns at Health 1.0 / Survival 0.8
//   - DYNAMIC COHERENCE (v2): sage-death ripple (−Agnosis·0.05×c to all
//     settlement witnesses), baseline drift toward Matter (age>20), practice
//     (samatha + vipassanā insights, the only path past Matter), newborn
//     coherence with parental inheritance capped at Matter. The W-22 review
//     proved static coherence was the projector's largest forecast error:
//     it seeded the boom cohort at 0.85 while the live ripple spiral pulled
//     0.71→0.53, doubling realized mortality.
//
// It enforces the ACTUAL production gate math (the lib_projector lesson:
// approximating gates as flat probabilities hid the R89 dormancy). All
// constants mirror internal/engine, internal/agents, and internal/phi.
//
// v2 (2026-06-09, W-22/R98 session) is NOT bit-comparable with the archived
// May-26 R97 validation runs: coherence is now dynamic by default, the Nous
// constant was corrected to mirror phi.Nous=Φ² (the old 0.7639 understated
// the R97-1 health target), and legacy-histogram seeding now uses the real
// per-settlement density (~393/sett). Approximate legacy behavior:
// -dyncoh=false -bgpow 4 -kidcoh 0.85 (see git history for exact).
//
// Usage:
//
//	go run ./cmd/pop_projector -scenario R97_full -years 20            # legacy histogram seed
//	go run ./cmd/pop_projector -scenario R97_full -years 20 \
//	    -seed-agents data/projector-seeds/2026-06-09-agents.json \
//	    -seed-setts  data/projector-seeds/2026-06-09-settlements.json  # live full-scale seed
//	go run ./cmd/pop_projector -hindcast                               # 05-26→06-09 backtest vs actuals
//	go run ./cmd/pop_projector -scenario R97_full -runs 7              # multi-seed envelope
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
)

// ---- Φ constants (mirror internal/phi) ----
const (
	Agnosis = 0.2360679774997897 // phi.Agnosis = Φ⁻³
	Psyche  = 0.3819660112501051 // phi.Psyche = Φ⁻²
	Matter  = 0.6180339887498949 // phi.Matter = Φ⁻¹
	// Nous mirrors phi.Nous = Φ² (v2 fix: v1 used 0.7639, which understated
	// the R97-1 health target Matter + c·(Nous−Matter); production clamps
	// health at 1.0, reached for any c ≥ ~0.19).
	Nous = 2.618033988749895
)

// ---- engine constants (mirror internal/engine) ----
const (
	TicksPerSimDay     = 1440
	TicksPerSimSeason  = 90000 // ~62.5 sim-days
	SimDaysPerSeason   = TicksPerSimSeason / TicksPerSimDay
	MaxWorldPopulation = 400_000
	seasonWinter       = 3
)

// ---- coherence dynamics constants (mirror internal/agents) ----
const (
	samathaPerTick  = 0.0005                      // contemplation.go SamathaPerTick (uncapped)
	insightProb     = 0.01                        // InsightProb per practice tick
	insightGain     = 0.025                       // InsightCoherenceGain (uncapped)
	contemplateBase = Agnosis * Agnosis * Agnosis // ContemplationProbability base, per sim-hour
	driftPerDay     = Agnosis * 0.001             // processBaselineCoherence, age>20, NaturalCap at Matter
	rippleScale     = Agnosis * 0.05              // sage-death ripple × witness coherence, un-gated
	liberatedAt     = 0.7                         // StateFromCoherence Liberated threshold
)

// Production satisfaction is flat ~0.70 (well above both the practice gate
// Psyche and the drift gate Matter), so the satisfaction gates are modeled
// as always-pass. Ordinary-death witness gains (relationship-gated, rare)
// are deliberately omitted.

// Practice weight = occupationConducive × classIntention, sampled from the
// live occupation mix (2026-06-09 counts) × uniform class. Expected practice
// ticks/day for an eligible agent = contemplateBase × pw × 24.
var occMix = []struct {
	share float64
	w     float64
}{
	{107200, Agnosis},          // Farmer
	{36005, 1.0},               // Alchemist
	{32535, Psyche},            // Fisher
	{16606, Agnosis},           // Laborer
	{15951, Psyche},            // Soldier
	{15042, 1.618033988749895}, // Scholar (Being)
	{14704, Matter},            // Merchant
	{5571, Matter},             // Hunter
	{4620, Agnosis},            // Crafter
	{4362, Agnosis},            // Miner
}

var classWeights = []float64{Matter, Psyche, Agnosis, 1.618033988749895}

var occMixTotal float64

func init() {
	for _, o := range occMix {
		occMixTotal += o.share
	}
}

func samplePracticeWeight(rng *rand.Rand) float64 {
	r := rng.Float64() * occMixTotal
	cum := 0.0
	occW := Agnosis
	for _, o := range occMix {
		cum += o.share
		if r <= cum {
			occW = o.w
			break
		}
	}
	return occW * classWeights[rng.Intn(len(classWeights))]
}

// Sensitivity knobs (set from flags in main).
var (
	kidCohMean = 0.53 // boom-cohort seed coherence mean (legacy histogram seeding)
	bgPower    = 6    // background scatter scale = Agnosis^bgPower (R98; pre-R98: 4)
	bgOnsetAge = 16   // background mortality onset age (prod: 16)
	dynCoh     = true // dynamic coherence (ripple + drift + practice)
)

type Agent struct {
	age       int     // sim-years (360-day agent-age calendar)
	months    int     // month counter (age++ every 12)
	health    float64 // 0..1 vitality
	survival  float64 // Needs.Survival
	coherence float64 // CittaCoherence — drives background mortality
	pw        float64 // practice weight (occW × clsW)
	warmth    bool    // has Clothing/Furs (suppresses winter hardship)
	sett      int32   // settlement id
	alive     bool
}

// Scenario flags — each candidate counter-measure.
type Scenario struct {
	name            string
	ageGateIllness  bool    // illness death only for age>=16 (protect children)
	passiveRegen    float64 // flat baseline Health regen/day, decoupled from rest
	provideWarmth   bool    // cohort gets warmth → no winter hardship
	birthHealthGate float64 // birth Health threshold (prod 0.5)
	smoothBirthCap  bool    // replacement-rate births near cap (no hard cliff)

	// R97 implemented levers (mirror internal/engine exactly):
	coherenceRegen bool // R97-1: Health drifts toward Matter+c*(Nous-Matter) at rate Agnosis*0.1
	softCap        bool // R97-2: birthChance *= capFactor taper toward MaxWorldPopulation
	protectRepro   bool // R97-3: 18-45 over-capacity weight = Psyche (not 1.0)
}

func scenarios() []Scenario {
	return []Scenario{
		{name: "baseline", birthHealthGate: 0.5},
		{name: "agegate_illness", ageGateIllness: true, birthHealthGate: 0.5},
		{name: "passive_regen_0.01", passiveRegen: 0.01, birthHealthGate: 0.5},
		{name: "warmth_provision", provideWarmth: true, birthHealthGate: 0.5},
		{name: "birth_healthgate_0.3", birthHealthGate: 0.3},
		// The systemic combo hypothesized pre-R97:
		{name: "COMBO_regen+healthgate", passiveRegen: 0.01, birthHealthGate: 0.3, smoothBirthCap: true},
		// R97 as actually implemented in internal/engine (full structural fix):
		{name: "R97_full", coherenceRegen: true, softCap: true, protectRepro: true, birthHealthGate: 0.5},
	}
}

// realAgeHistogram is the full-population age histogram pulled 2026-05-26
// (tick 6,213,048) from /api/v1/agents?tier=0. Index = age, value = count.
// Used by legacy seeding and by -hindcast.
var realAgeHistogram = map[int]int{
	0: 113, 1: 1105, 2: 263, 3: 186, 4: 116, 5: 331, 6: 919, 7: 2127,
	8: 3679, 9: 6358, 10: 28282, 11: 69548, 12: 51226, 13: 56899,
	14: 52858, 15: 28358, 16: 4738, 17: 29, 18: 12, 19: 11, 20: 4,
	21: 3, 32: 1,
}

// healthBands is the sampled Health distribution (n=500, 2026-05-26, PRE-R97):
// {lo, hi, fraction}. Agents are seeded uniformly within each band.
var healthBands = []struct {
	lo, hi, frac float64
}{
	{0.15, 0.20, 0.390},
	{0.20, 0.30, 0.484},
	{0.30, 0.50, 0.048},
	{0.50, 0.70, 0.004},
	{0.70, 1.00, 0.074},
}

// hindcastActuals are the live observations at tick 7,339,822 (2026-06-09),
// 778 sim-days (1440-tick days) after the R97 deploy restore at tick
// 6,219,979. Rates are 7-day trailing averages from stats_history (rows are
// exactly 1440 ticks apart).
var hindcastActuals = struct {
	days                            int
	pop                             int
	u16, a1617, a1829, a3045, a46up int
	cohMean                         float64
	deathsPerDay, birthsPerDay      float64
}{
	days: 778, pop: 270916,
	u16: 225047, a1617: 44821, a1829: 1038, a3045: 121, a46up: 23,
	cohMean:      0.5328,
	deathsPerDay: 155.4, birthsPerDay: 38.6,
}

const targetSample = 100_000

// realSettDensity is agents-per-settlement in production (May 26: 307K/781;
// June 9: 271K/786 — both ≈ 350-395). Legacy/hindcast seeding sizes its
// settlement count so the births co-location gate (≥2 eligible parents in
// one settlement) sees realistic local density at sample scale.
const realSettDensity = 393

func seedHealth(rng *rand.Rand) float64 {
	r := rng.Float64()
	cum := 0.0
	for _, b := range healthBands {
		cum += b.frac
		if r <= cum {
			return b.lo + rng.Float64()*(b.hi-b.lo)
		}
	}
	return 0.25
}

// seedCoherenceHindcast reproduces the 2026-05-26 live coherence distribution:
// Liberated (c≥0.7) = 164,695/307,174 = 53.6%, world mean 0.7075.
func seedCoherenceHindcast(rng *rand.Rand) float64 {
	if rng.Float64() < 0.536 {
		return 0.7 + rng.Float64()*0.3 // U(0.7, 1.0), mean 0.85
	}
	return clamp(0.54+rng.NormFloat64()*0.10, 0.05, 0.699)
}

// seedPopulation seeds from the hardcoded 2026-05-26 histogram at sample
// scale. hindcastCoh switches the coherence seed from the kidCohMean blanket
// to the bimodal May-26 distribution.
func seedPopulation(rng *rand.Rand, hindcastCoh bool) ([]Agent, float64, int) {
	total := 0
	for _, c := range realAgeHistogram {
		total += c
	}
	scale := float64(targetSample) / float64(total)
	scaleUp := float64(total) / float64(targetSample)
	var agents []Agent
	id := 0
	numSett := targetSample / realSettDensity // ~254 — real per-settlement density
	ages := make([]int, 0, len(realAgeHistogram))
	for a := range realAgeHistogram {
		ages = append(ages, a)
	}
	sort.Ints(ages)
	for _, age := range ages {
		n := int(float64(realAgeHistogram[age])*scale + 0.5)
		for i := 0; i < n; i++ {
			h := seedHealth(rng)
			var coh float64
			if hindcastCoh {
				coh = seedCoherenceHindcast(rng)
			} else {
				coh = 0.7 + rng.NormFloat64()*0.15
				if age < 16 {
					coh = kidCohMean + rng.NormFloat64()*0.10
				}
				coh = clamp(coh, 0.05, 1.0)
			}
			// Survival: designed-scarcity equilibrium ~0.39 with spread; a
			// chronic-stress tail (survival 0.1-0.3) cannot rest.
			surv := clamp(0.39+rng.NormFloat64()*0.10, 0.02, 0.95)
			agents = append(agents, Agent{
				age:       age,
				months:    rng.Intn(12),
				health:    h,
				survival:  surv,
				coherence: coh,
				pw:        samplePracticeWeight(rng),
				warmth:    rng.Float64() < 0.02, // ~2% can afford warmth
				sett:      int32(id % numSett),
				alive:     true,
			})
			id++
		}
	}
	return agents, scaleUp, numSett
}

type liveAgentJSON struct {
	Age       int     `json:"age"`
	Coherence float64 `json:"coherence"`
}

type liveSettJSON struct {
	Population int `json:"population"`
}

// seedFromLive seeds one model agent per real agent (full scale, scaleUp=1)
// from /api/v1/agents?tier=0 and /api/v1/settlements dumps. Agents are
// assigned to settlements preserving the real settlement-size distribution
// (the births co-location gate is superlinear in local density — the W-22
// review measured a 4× births overestimate from uniform 500-agent buckets).
// Health is seeded at the R97-1 recovery target (production converges there
// at ~10%/day of the gap); survival at the INV-3 equilibrium.
func seedFromLive(rng *rand.Rand, agentsPath, settsPath string) ([]Agent, float64, int, error) {
	ab, err := os.ReadFile(agentsPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read agents seed: %w", err)
	}
	var live []liveAgentJSON
	if err := json.Unmarshal(ab, &live); err != nil {
		return nil, 0, 0, fmt.Errorf("parse agents seed: %w", err)
	}

	// Settlement size distribution: real if provided, else uniform at real density.
	var pops []int
	if settsPath != "" {
		sb, err := os.ReadFile(settsPath)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read settlements seed: %w", err)
		}
		var setts []liveSettJSON
		if err := json.Unmarshal(sb, &setts); err != nil {
			return nil, 0, 0, fmt.Errorf("parse settlements seed: %w", err)
		}
		for _, s := range setts {
			pops = append(pops, s.Population)
		}
	} else {
		n := len(live) / realSettDensity
		for i := 0; i < n; i++ {
			pops = append(pops, realSettDensity)
		}
	}

	// Quota-fill settlements in shuffled agent order.
	totalPop := 0
	for _, p := range pops {
		totalPop += p
	}
	order := rng.Perm(len(live))
	agents := make([]Agent, 0, len(live))
	sett, filled := 0, 0
	quota := func(s int) int {
		return int(float64(pops[s]) * float64(len(live)) / float64(totalPop))
	}
	for _, idx := range order {
		la := live[idx]
		for sett < len(pops)-1 && filled >= quota(sett) {
			sett++
			filled = 0
		}
		c := clamp(la.Coherence, 0.0, 1.0)
		target := math.Min(1.0, Matter+c*(Nous-Matter))
		agents = append(agents, Agent{
			age:       la.Age,
			months:    rng.Intn(12),
			health:    target,
			survival:  clamp(0.39+rng.NormFloat64()*0.10, 0.02, 0.95),
			coherence: c,
			pw:        samplePracticeWeight(rng),
			warmth:    rng.Float64() < 0.02,
			sett:      int32(sett),
			alive:     true,
		})
		filled++
	}
	return agents, 1.0, len(pops), nil
}

// dailyMortality mirrors agentDailyMortalityChance (population.go).
func dailyMortality(a *Agent, pop int, protectRepro bool) float64 {
	var chance float64
	if a.age >= bgOnsetAge {
		// Background scatter scale: Agnosis^bgPower (prod post-R98: 6;
		// pre-R98: 4). R98 applied the R51 design comment's own tuning note
		// ("if too aggressive, reduce background by one Φ power") twice.
		// Floor stays one power below the base.
		base := math.Pow(Agnosis, float64(bgPower))
		floor := base * Agnosis
		scatter := 1.0 - a.coherence
		chance = floor + base*scatter
		ag3 := Agnosis * Agnosis * Agnosis
		off := float64(a.age) - 50.0
		sig := 1.0 / (1.0 + math.Exp(-off/12.0))
		chance += ag3 * sig * sig
	}
	// Over-capacity pressure (all ages, weighted by life stage).
	if pop > MaxWorldPopulation {
		overshoot := float64(pop)/float64(MaxWorldPopulation) - 1.0
		pressure := Agnosis * Agnosis * overshoot
		var w float64
		switch {
		case a.age < 2:
			w = Agnosis
		case a.age < 16:
			w = Psyche
		case protectRepro && a.age >= 18 && a.age <= 45:
			w = Psyche // R97-3
		default:
			w = 1.0
		}
		chance += pressure * w
	}
	return chance
}

type yearStat struct {
	year                   int
	pop, kids, adults      int
	repro, eligibleParents int
	deathsIllness          int
	deathsNatural          int
	births                 int
	meanHealth             float64
	meanCoh                float64
	maxAge                 int
}

type runResult struct {
	stats                  []yearStat
	final                  []Agent
	tailDeaths, tailBirths int // counts over the trailing tailDays
	tailDays               int
}

func runScenario(sc Scenario, totalDays int, scaleUp float64, numSett int, agents []Agent, rng *rand.Rand) runResult {
	var stats []yearStat

	tick := uint64(6_219_979) // R97 deploy restore tick (mechanical Summer)
	prevSeason := int((tick / TicksPerSimSeason) % 4)

	var illnessDeaths, naturalDeaths, births int
	res := runResult{tailDays: 30}
	if res.tailDays > totalDays {
		res.tailDays = totalDays
	}

	// Settlement membership index (agent indices) for the sage-death ripple.
	settMembers := make([][]int32, numSett)
	for i := range agents {
		s := agents[i].sett
		settMembers[s] = append(settMembers[s], int32(i))
	}

	livePop := func() int {
		n := 0
		for i := range agents {
			if agents[i].alive {
				n++
			}
		}
		return n
	}

	for day := 0; day < totalDays; day++ {
		tick += TicksPerSimDay
		pop := livePop()
		scaledPop := int(float64(pop) * scaleUp)
		inTail := day >= totalDays-res.tailDays

		season := int((tick / TicksPerSimSeason) % 4)
		enteredWinter := season != prevSeason && season == seasonWinter
		prevSeason = season

		// --- season boundary: winter hardship (once) ---
		if enteredWinter {
			for i := range agents {
				a := &agents[i]
				if !a.alive {
					continue
				}
				warm := a.warmth || sc.provideWarmth
				if !warm {
					a.health -= 0.05
					if a.health < 0 {
						a.health = 0
					}
				}
			}
		}

		// --- weekly plague (1%/7d): hits ~6.5% of pop (epicenter+spread) ---
		if day%7 == 0 && rng.Float64() < 0.01 {
			severity := Agnosis + rng.Float64()*Psyche
			for i := range agents {
				if agents[i].alive && rng.Float64() < 0.065 {
					agents[i].health -= severity * 0.3
					if agents[i].health < 0 {
						agents[i].health = 0
					}
				}
			}
		}

		// --- daily: aging, mortality (+ ripple), illness, health, coherence ---
		for i := range agents {
			a := &agents[i]
			if !a.alive {
				continue
			}

			// natural + over-capacity mortality
			if rng.Float64() < dailyMortality(a, scaledPop, sc.protectRepro) {
				a.alive = false
				naturalDeaths++
				if inTail {
					res.tailDeaths++
				}
				// Sage-death ripple (population.go processNaturalDeaths):
				// a Liberated death drains every settlement witness by
				// rippleScale × witness coherence, un-gated. This is the
				// coherence→scatter→mortality spiral W-22 observed live.
				if dynCoh && a.coherence >= liberatedAt {
					for _, wi := range settMembers[a.sett] {
						w := &agents[wi]
						if w.alive && int(wi) != i {
							w.coherence -= rippleScale * w.coherence
						}
					}
				}
				continue
			}

			// illness death path (Health<0.15)
			illnessActive := a.health < 0.15 && a.health > 0
			if sc.ageGateIllness && a.age < 16 {
				illnessActive = false // protect children, like natural mortality
			}
			if illnessActive {
				a.health -= 0.01
				if a.health <= 0 {
					a.alive = false
					illnessDeaths++
					if inTail {
						res.tailDeaths++
					}
					continue
				}
			}

			// survival dynamics: decay + eating keeps survival ~0.39 with a
			// chronic-stress tail. Simplified stochastic model.
			a.survival += (0.39 - a.survival) * 0.05 // mean-revert toward eq
			a.survival += rng.NormFloat64() * 0.03
			a.survival = clamp(a.survival, 0.0, 0.98)
			if a.survival < 0.1 {
				a.health -= 0.01 // starvation health decay (DecayNeeds)
				if a.health <= 0 {
					a.alive = false
					illnessDeaths++
					if inTail {
						res.tailDeaths++
					}
					continue
				}
			}

			// rest: gated behind survival≥0.3 (decideSurvival ordering) and
			// triggered by health<0.30. The survival-stressed tail can't rest.
			if a.survival >= 0.30 && a.health < 0.30 {
				a.health += 0.05
				if a.health > 1.0 {
					a.health = 1.0
				}
			}
			// scenario: passive baseline regen (decoupled from rest action)
			if sc.passiveRegen > 0 {
				a.health += sc.passiveRegen
				if a.health > 1.0 {
					a.health = 1.0
				}
			}
			// R97-1: Matter-floored, coherence-scaled Health recovery (mirrors processHealthRecovery)
			if sc.coherenceRegen {
				target := Matter + a.coherence*(Nous-Matter)
				if a.health < target {
					a.health += (target - a.health) * Agnosis * 0.1
					if a.health > 1.0 {
						a.health = 1.0
					}
				}
			}

			// Coherence dynamics (daily aggregation of per-tick mechanics).
			if dynCoh {
				// Practice (contemplation.go): age≥16, sat gate ~always
				// passes in production. Expected practice ticks/day
				// e = base × pw × 24; samatha applied as e×rate (uncapped),
				// insight as one Bernoulli(e×p) draw per day.
				if a.age >= 16 && a.pw > 0 {
					e := contemplateBase * a.pw * 24
					a.coherence += e * samathaPerTick
					if rng.Float64() < e*insightProb {
						a.coherence += insightGain
					}
					if a.coherence > 1 {
						a.coherence = 1
					}
				}
				// Baseline drift (processBaselineCoherence): age>20,
				// NaturalCap at Matter — fills the Embodied band only.
				if a.age > 20 && a.coherence < Matter {
					a.coherence += driftPerDay
					if a.coherence > Matter {
						a.coherence = Matter
					}
				}
			}

			// aging: months++ every 30 days; age++ every 12 months
			if day%30 == 0 && day > 0 {
				a.months++
				if a.months >= 12 {
					a.months = 0
					a.age++
				}
			}
		}

		// --- births (processBirths): per settlement, ≥2 eligible parents ---
		// cap gate. R97-2 soft cap: taper birthChance toward the cap instead of
		// a hard cliff. capFactor∈[0,1], multiplied into birthChance below.
		capBlocked := scaledPop >= MaxWorldPopulation
		capFactor := 1.0
		if sc.softCap {
			capFactor = float64(MaxWorldPopulation-scaledPop) / (float64(MaxWorldPopulation) * Agnosis)
			capBlocked = capFactor <= 0
			if capFactor > 1.0 {
				capFactor = 1.0
			}
		}
		if !capBlocked {
			// group eligible parents by settlement (indices — appends below
			// reallocate the agents slice, so pointers would go stale)
			perSett := map[int32][]int32{}
			for i := range agents {
				a := &agents[i]
				if a.alive && a.age >= 18 && a.age <= 45 &&
					a.health > sc.birthHealthGate && a.survival > 0.3 {
					perSett[a.sett] = append(perSett[a.sett], int32(i))
				}
			}
			for sett, parents := range perSett {
				if len(parents) < 2 {
					continue
				}
				birthChance := float64(len(parents)) / 30.0 * 0.75 * capFactor // mid prosperity × soft cap
				bc := int(birthChance)
				if rng.Float64() < birthChance-float64(bc) {
					bc++
				}
				if bc > 3 {
					bc = 3 // cap per settlement/day
				}
				for b := 0; b < bc; b++ {
					// Newborn coherence (spawner.go SpawnChild): base around
					// Agnosis + parental cultural transmission, hard-capped
					// at Matter (R88 — no inherited Liberation).
					parent := agents[parents[(day+b)%len(parents)]]
					c := clamp(Agnosis+rng.NormFloat64()*Agnosis*0.5, 0.01, Matter)
					c += parent.coherence * Agnosis * 0.5
					if c > Matter {
						c = Matter
					}
					agents = append(agents, Agent{
						age: 0, months: 0, health: 1.0, survival: 0.8,
						coherence: c,
						pw:        samplePracticeWeight(rng),
						warmth:    rng.Float64() < 0.02, sett: sett, alive: true,
					})
					settMembers[sett] = append(settMembers[sett], int32(len(agents)-1))
					births++
					if inTail {
						res.tailBirths++
					}
				}
			}
		}

		// yearly snapshot
		if day%360 == 0 {
			stats = append(stats, snapshot(agents, day/360, scaleUp, illnessDeaths, naturalDeaths, births, sc.birthHealthGate))
			illnessDeaths, naturalDeaths, births = 0, 0, 0
		}
	}
	// final snapshot
	stats = append(stats, snapshot(agents, (totalDays+359)/360, scaleUp, illnessDeaths, naturalDeaths, births, sc.birthHealthGate))
	res.stats = stats
	res.final = agents
	return res
}

func snapshot(agents []Agent, year int, scaleUp float64, ill, nat, b int, bhg float64) yearStat {
	var ys yearStat
	ys.year = year
	var hsum, csum float64
	for i := range agents {
		a := &agents[i]
		if !a.alive {
			continue
		}
		ys.pop++
		hsum += a.health
		csum += a.coherence
		if a.age > ys.maxAge {
			ys.maxAge = a.age
		}
		if a.age < 16 {
			ys.kids++
		} else {
			ys.adults++
		}
		if a.age >= 18 && a.age <= 45 {
			ys.repro++
			if a.health > bhg && a.survival > 0.3 {
				ys.eligibleParents++
			}
		}
	}
	if ys.pop > 0 {
		ys.meanHealth = hsum / float64(ys.pop)
		ys.meanCoh = csum / float64(ys.pop)
	}
	su := func(x int) int { return int(float64(x) * scaleUp) }
	ys.pop, ys.kids, ys.adults = su(ys.pop), su(ys.kids), su(ys.adults)
	ys.repro, ys.eligibleParents = su(ys.repro), su(ys.eligibleParents)
	ys.deathsIllness, ys.deathsNatural, ys.births = su(ill), su(nat), su(b)
	return ys
}

// hindcastReport compares the model's end state against the 2026-06-09 live
// observations and prints per-metric errors.
func hindcastReport(res runResult, scaleUp float64) {
	var pop, u16, a1617, a1829, a3045, a46 int
	var csum float64
	for i := range res.final {
		a := &res.final[i]
		if !a.alive {
			continue
		}
		pop++
		csum += a.coherence
		switch {
		case a.age < 16:
			u16++
		case a.age < 18:
			a1617++
		case a.age < 30:
			a1829++
		case a.age < 46:
			a3045++
		default:
			a46++
		}
	}
	cohMean := 0.0
	if pop > 0 {
		cohMean = csum / float64(pop)
	}
	su := func(x int) int { return int(float64(x) * scaleUp) }
	dpd := float64(su(res.tailDeaths)) / float64(res.tailDays)
	bpd := float64(su(res.tailBirths)) / float64(res.tailDays)

	act := hindcastActuals
	row := func(name string, model, actual float64) {
		errPct := math.Inf(1)
		if actual != 0 {
			errPct = (model - actual) / actual * 100
		}
		fmt.Printf("│ %-22s %12.1f %12.1f %+9.1f%%\n", name, model, actual, errPct)
	}
	fmt.Printf("┌─ Hindcast vs 2026-06-09 actuals (tick 7,339,822, %d sim-days post-R97)\n", act.days)
	fmt.Printf("│ %-22s %12s %12s %10s\n", "metric", "model", "actual", "err")
	row("population", float64(su(pop)), float64(act.pop))
	row("age <16", float64(su(u16)), float64(act.u16))
	row("age 16-17", float64(su(a1617)), float64(act.a1617))
	row("age 18-29", float64(su(a1829)), float64(act.a1829))
	row("age 30-45", float64(su(a3045)), float64(act.a3045))
	row("age 46+", float64(su(a46)), float64(act.a46up))
	row("mean coherence", cohMean, act.cohMean)
	row("deaths/day (30d tail)", dpd, act.deathsPerDay)
	row("births/day (30d tail)", bpd, act.birthsPerDay)
	fmt.Printf("└─\n\n")
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func verdictOf(first, final yearStat) string {
	switch {
	case final.pop < first.pop/10:
		return "COLLAPSE"
	case final.pop < first.pop/2:
		return "DECLINE"
	case final.pop > first.pop*2:
		return "REBOUND"
	}
	return "STABLE"
}

func main() {
	scenarioFlag := flag.String("scenario", "all", "baseline|agegate_illness|passive_regen_0.01|warmth_provision|birth_healthgate_0.3|COMBO_regen+healthgate|R97_full|all")
	years := flag.Int("years", 20, "sim-years to project")
	seed := flag.Int64("seed", 42, "RNG seed")
	runs := flag.Int("runs", 1, "independent runs (seed, seed+1, ...) — prints min/median/max envelope")
	seedAgentsPath := flag.String("seed-agents", "", "live /api/v1/agents?tier=0 JSON dump — full-scale seeding (age+coherence per agent)")
	seedSettsPath := flag.String("seed-setts", "", "live /api/v1/settlements JSON dump — real settlement-size distribution (with -seed-agents)")
	hindcast := flag.Bool("hindcast", false, "backtest: seed from the 2026-05-26 histogram + bimodal coherence, run 778 days under pre-R98 mortality, compare vs 2026-06-09 actuals")
	flag.Float64Var(&kidCohMean, "kidcoh", 0.53, "boom-cohort (age<16) seed coherence mean (legacy histogram seeding)")
	flag.IntVar(&bgPower, "bgpow", 6, "background scatter mortality Φ-power (prod post-R98: Agnosis^6; pre-R98: 4; floor one power below)")
	flag.IntVar(&bgOnsetAge, "onset", 16, "background mortality onset age (prod: 16)")
	flag.BoolVar(&dynCoh, "dyncoh", true, "dynamic coherence: sage-death ripple + baseline drift + practice")
	flag.Parse()

	all := scenarios()
	var toRun []Scenario
	if *hindcast {
		// Hindcast = what production actually ran: R97 levers + pre-R98 mortality.
		for _, s := range all {
			if s.name == "R97_full" {
				toRun = append(toRun, s)
			}
		}
		if bgPower != 4 {
			fmt.Fprintf(os.Stderr, "hindcast: forcing -bgpow 4 (pre-R98 production mortality)\n")
			bgPower = 4
		}
	} else if *scenarioFlag == "all" {
		toRun = all
	} else {
		for _, s := range all {
			if s.name == *scenarioFlag {
				toRun = append(toRun, s)
			}
		}
		if len(toRun) == 0 {
			fmt.Fprintf(os.Stderr, "unknown scenario %q\n", *scenarioFlag)
			os.Exit(1)
		}
	}

	fmt.Printf("=== Crossworlds Demographic Projector (v2: dynamic coherence) ===\n")
	switch {
	case *hindcast:
		fmt.Printf("Mode: HINDCAST — 2026-05-26 seed (bimodal coherence, mean 0.7075) → %d days, pre-R98 mortality\n", hindcastActuals.days)
	case *seedAgentsPath != "":
		fmt.Printf("Seed: live agent dump %s (full scale)\n", *seedAgentsPath)
	default:
		fmt.Printf("Seed: 2026-05-26 age histogram at 1:%d sample scale, kidcoh=%.2f\n", int(307166/targetSample)+1, kidCohMean)
	}
	fmt.Printf("Mortality: Agnosis^%d background (onset %d), dynamic coherence: %v\n\n", bgPower, bgOnsetAge, dynCoh)

	for _, sc := range toRun {
		type runOut struct {
			res     runResult
			scaleUp float64
		}
		var outs []runOut
		for r := 0; r < *runs; r++ {
			rng := rand.New(rand.NewSource(*seed + int64(r)))
			var agents []Agent
			var scaleUp float64
			var numSett int
			var err error
			if *seedAgentsPath != "" {
				agents, scaleUp, numSett, err = seedFromLive(rng, *seedAgentsPath, *seedSettsPath)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			} else {
				agents, scaleUp, numSett = seedPopulation(rng, *hindcast)
			}
			totalDays := *years * 360
			if *hindcast {
				totalDays = hindcastActuals.days
			}
			outs = append(outs, runOut{runScenario(sc, totalDays, scaleUp, numSett, agents, rng), scaleUp})
		}

		// Print the first run's trajectory in full.
		first := outs[0]
		fmt.Printf("┌─ Scenario: %s (seed %d)\n", sc.name, *seed)
		fmt.Printf("│ %-4s %10s %10s %9s %8s %9s %9s %7s %6s %6s %6s\n",
			"yr", "pop", "kids", "repro18-45", "eligPar", "ill_d", "nat_d", "births", "maxAge", "mh", "coh")
		for _, s := range first.res.stats {
			fmt.Printf("│ %-4d %10d %10d %9d %8d %9d %9d %7d %6d %6.3f %6.3f\n",
				s.year, s.pop, s.kids, s.repro, s.eligibleParents,
				s.deathsIllness, s.deathsNatural, s.births, s.maxAge, s.meanHealth, s.meanCoh)
		}
		f0 := first.res.stats[0]
		fN := first.res.stats[len(first.res.stats)-1]
		fmt.Printf("└─ verdict: %s  (pop %d → %d, eligible parents %d → %d)\n\n",
			verdictOf(f0, fN), f0.pop, fN.pop, f0.eligibleParents, fN.eligibleParents)

		if *hindcast {
			hindcastReport(first.res, first.scaleUp)
		}

		// Multi-run envelope.
		if *runs > 1 {
			finals := make([]int, 0, *runs)
			verdicts := map[string]int{}
			for _, o := range outs {
				s0 := o.res.stats[0]
				sN := o.res.stats[len(o.res.stats)-1]
				finals = append(finals, sN.pop)
				verdicts[verdictOf(s0, sN)]++
			}
			sort.Ints(finals)
			fmt.Printf("   envelope over %d runs: final pop min %d / median %d / max %d — verdicts: %v\n\n",
				*runs, finals[0], finals[len(finals)/2], finals[len(finals)-1], verdicts)
		}
	}
}
