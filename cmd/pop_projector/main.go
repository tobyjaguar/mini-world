// Command pop_projector is a demographic/mortality projector for Crossworlds.
//
// Unlike cmd/lib_projector (which models liberation/coherence trajectories),
// this projector models the POPULATION dynamics that drive the W-19 winter
// illness die-off and the W-20 birth freeze / age-structure collapse:
//
//   - age structure (seeded from the real 2026-05-26 histogram, tick 6.21M)
//   - agent Health (seeded from a live API sample, n=500)
//   - the survival↔rest↔health coupling (health self-regulates near the 0.30
//     rest trigger, but survival-stressed agents can't rest and ratchet down)
//   - winter hardship (−0.05 Health/winter to warmth-less agents, once per
//     mechanical season via OnSeason)
//   - illness death (Health<0.15 → −0.01/day → death; NOT age-gated in prod)
//   - coherence/age/over-capacity natural mortality (population.go math)
//   - births (processBirths gate: ≥2 co-located adults 18-45, Health>0.5,
//     Survival>0.3) with newborns at Health 1.0 / Survival 0.8
//
// It enforces the ACTUAL production gate math (the lib_projector lesson:
// approximating gates as flat probabilities hid the R89 dormancy). All
// constants mirror internal/engine and internal/phi.
//
// The baseline scenario is calibrated to reproduce the two observables:
// health equilibrium ~0.29 and a winter die-off in the tens of thousands.
// Scenarios then show the RELATIVE effect of each candidate counter-measure.
//
// Usage:
//
//	go run ./cmd/pop_projector -scenario baseline -years 20
//	go run ./cmd/pop_projector -scenario all -years 20
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
)

// ---- Φ constants (mirror internal/phi) ----
const (
	Agnosis = 0.2360679774997897 // φ⁻¹·... (1/φ² roughly; matches phi.Agnosis)
	Psyche  = 0.3819660112501051 // 1/φ²·... (phi.Psyche)
	Matter  = 0.6180339887498949 // 1/φ (phi.Matter)
	Nous    = 0.7639320225002102 // phi.Nous
)

// ---- engine constants (mirror internal/engine) ----
const (
	TicksPerSimDay     = 1440
	TicksPerSimSeason  = 90000 // ~62.5 sim-days
	SimDaysPerSeason   = TicksPerSimSeason / TicksPerSimDay
	MaxWorldPopulation = 400_000
	seasonWinter       = 3
)

// Seed scale: model SCALE× fewer agents for speed, then report scaled-up
// population numbers. 100K agents representing ~307K (scale ~3.07).
const targetSample = 100_000

// Sensitivity knobs (set from flags in main). Defaults mirror production
// post-R98. To reproduce the 2026-05-26 R97 validation runs exactly, use
// `-bgpow 4 -kidcoh 0.85` (pre-R98 mortality + the May 26 coherence estimate).
var (
	kidCohMean = 0.53 // boom-cohort seed coherence mean (live 2026-06-09)
	bgPower    = 6    // background scatter scale = Agnosis^bgPower (R98)
	bgOnsetAge = 16   // background mortality onset age
)

type Agent struct {
	age       int     // sim-years
	months    int     // month counter (age++ every 12)
	health    float64 // 0..1 vitality
	survival  float64 // Needs.Survival
	coherence float64 // CittaCoherence — drives background mortality
	warmth    bool    // has Clothing/Furs (suppresses winter hardship)
	sett      int     // settlement id
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
		// The systemic combo I hypothesize as the long-term fix:
		{name: "COMBO_regen+healthgate", passiveRegen: 0.01, birthHealthGate: 0.3, smoothBirthCap: true},
		// R97 as actually implemented in internal/engine (full structural fix):
		{name: "R97_full", coherenceRegen: true, softCap: true, protectRepro: true, birthHealthGate: 0.5},
	}
}

// realAgeHistogram is the full-population age histogram pulled 2026-05-26
// (tick 6,213,048) from /api/v1/agents?tier=0. Index = age, value = count.
var realAgeHistogram = map[int]int{
	0: 113, 1: 1105, 2: 263, 3: 186, 4: 116, 5: 331, 6: 919, 7: 2127,
	8: 3679, 9: 6358, 10: 28282, 11: 69548, 12: 51226, 13: 56899,
	14: 52858, 15: 28358, 16: 4738, 17: 29, 18: 12, 19: 11, 20: 4,
	21: 3, 32: 1,
}

// healthBands is the sampled Health distribution (n=500, 2026-05-26):
// {upper, fraction}. Agents are seeded uniformly within each band.
var healthBands = []struct {
	lo, hi, frac float64
}{
	{0.15, 0.20, 0.390},
	{0.20, 0.30, 0.484},
	{0.30, 0.50, 0.048},
	{0.50, 0.70, 0.004},
	{0.70, 1.00, 0.074},
}

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

func seedPopulation(rng *rand.Rand) []Agent {
	total := 0
	for _, c := range realAgeHistogram {
		total += c
	}
	scale := float64(targetSample) / float64(total)
	var agents []Agent
	id := 0
	// 200 settlements keeps ~500 agents/settlement (close to the real
	// ~393/settlement ratio so birth co-location behaves realistically).
	const numSett = 200
	ages := make([]int, 0, len(realAgeHistogram))
	for a := range realAgeHistogram {
		ages = append(ages, a)
	}
	sort.Ints(ages)
	for _, age := range ages {
		n := int(float64(realAgeHistogram[age])*scale + 0.5)
		for i := 0; i < n; i++ {
			h := seedHealth(rng)
			// Newborns/young start healthier in reality; the sampled bands
			// already reflect the aged-down equilibrium, so use them directly.
			coh := 0.7 + rng.NormFloat64()*0.15
			if age < 16 {
				// boom cohort coherence: default 0.85 (2026-05-26 estimate);
				// override with -kidcoh to match later live observations
				// (2026-06-09 live mean: 0.53 and falling — sage-ripple drain).
				coh = kidCohMean + rng.NormFloat64()*0.10
			}
			coh = clamp(coh, 0.05, 1.0)
			// Survival: designed-scarcity equilibrium ~0.39 with spread; a
			// chronic-stress tail (survival 0.1-0.3) cannot rest.
			surv := clamp(0.39+rng.NormFloat64()*0.10, 0.02, 0.95)
			agents = append(agents, Agent{
				age:       age,
				months:    rng.Intn(12),
				health:    h,
				survival:  surv,
				coherence: coh,
				warmth:    rng.Float64() < 0.02, // ~2% can afford warmth
				sett:      id % numSett,
				alive:     true,
			})
			id++
		}
	}
	return agents
}

// dailyMortality mirrors agentDailyMortalityChance (population.go).
func dailyMortality(a *Agent, pop int, protectRepro bool) float64 {
	if a.age < 16 {
		// children exempt from natural mortality...
		// (over-capacity pressure below still applies, weighted down)
	}
	var chance float64
	if a.age >= bgOnsetAge {
		// Background scatter scale: Agnosis^bgPower (prod post-R98: 6;
		// pre-R98: 4). R98 applied the R51 design comment's own tuning note
		// ("if too aggressive, reduce background by one Φ power") twice.
		// Floor stays one power below the base.
		ag4 := math.Pow(Agnosis, float64(bgPower))
		ag5 := ag4 * Agnosis
		scatter := 1.0 - a.coherence
		chance = ag5 + ag4*scatter
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
	maxAge                 int
}

func runScenario(sc Scenario, years int, scaleUp float64, rng *rand.Rand) []yearStat {
	agents := seedPopulation(rng)
	var stats []yearStat

	tick := uint64(6_213_048) // start at the real current tick (mechanical Summer)
	prevSeason := int((tick / TicksPerSimSeason) % 4)

	totalDays := years * 360 // age-system year = 360 sim-days
	var illnessDeaths, naturalDeaths, births int

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
			// approximate: damage a random ~6.5% of the population
			for i := range agents {
				if agents[i].alive && rng.Float64() < 0.065 {
					agents[i].health -= severity * 0.3
					if agents[i].health < 0 {
						agents[i].health = 0
					}
				}
			}
		}

		// --- daily: aging, mortality, illness, survival/rest/health ---
		for i := range agents {
			a := &agents[i]
			if !a.alive {
				continue
			}

			// natural + over-capacity mortality
			if rng.Float64() < dailyMortality(a, scaledPop, sc.protectRepro) {
				a.alive = false
				naturalDeaths++
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
			// group eligible parents by settlement
			perSett := map[int][]*Agent{}
			for i := range agents {
				a := &agents[i]
				if a.alive && a.age >= 18 && a.age <= 45 &&
					a.health > sc.birthHealthGate && a.survival > 0.3 {
					perSett[a.sett] = append(perSett[a.sett], a)
				}
			}
			for _, parents := range perSett {
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
					sett := parents[0].sett
					agents = append(agents, Agent{
						age: 0, months: 0, health: 1.0, survival: 0.8,
						coherence: clamp(Agnosis+rng.NormFloat64()*Agnosis*0.5, 0.05, Matter),
						warmth:    rng.Float64() < 0.02, sett: sett, alive: true,
					})
					births++
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
	stats = append(stats, snapshot(agents, years, scaleUp, illnessDeaths, naturalDeaths, births, sc.birthHealthGate))
	return stats
}

func snapshot(agents []Agent, year int, scaleUp float64, ill, nat, b int, bhg float64) yearStat {
	var ys yearStat
	ys.year = year
	var hsum float64
	for i := range agents {
		a := &agents[i]
		if !a.alive {
			continue
		}
		ys.pop++
		hsum += a.health
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
	}
	su := func(x int) int { return int(float64(x) * scaleUp) }
	ys.pop, ys.kids, ys.adults = su(ys.pop), su(ys.kids), su(ys.adults)
	ys.repro, ys.eligibleParents = su(ys.repro), su(ys.eligibleParents)
	ys.deathsIllness, ys.deathsNatural, ys.births = su(ill), su(nat), su(b)
	return ys
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

func main() {
	scenarioFlag := flag.String("scenario", "all", "baseline|agegate_illness|passive_regen_0.01|warmth_provision|birth_healthgate_0.3|COMBO_regen+healthgate|all")
	years := flag.Int("years", 20, "sim-years to project")
	seed := flag.Int64("seed", 42, "RNG seed")
	flag.Float64Var(&kidCohMean, "kidcoh", 0.53, "boom-cohort (age<16) seed coherence mean (live 2026-06-09: 0.53)")
	flag.IntVar(&bgPower, "bgpow", 6, "background scatter mortality Φ-power (prod post-R98: Agnosis^6; pre-R98: 4; floor one power below)")
	flag.IntVar(&bgOnsetAge, "onset", 16, "background mortality onset age (prod: 16)")
	flag.Parse()

	var total int
	for _, c := range realAgeHistogram {
		total += c
	}
	scaleUp := float64(total) / float64(targetSample)

	all := scenarios()
	var toRun []Scenario
	if *scenarioFlag == "all" {
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

	fmt.Printf("=== Crossworlds Demographic Projector ===\n")
	fmt.Printf("Seed: real age histogram (tick 6.21M, 2026-05-26) + sampled Health (n=500)\n")
	fmt.Printf("Modeled agents: %d (scale ×%.2f → ~%d world pop)\n", targetSample, scaleUp, total)
	fmt.Printf("Projection: %d sim-years\n\n", *years)

	for _, sc := range toRun {
		rng := rand.New(rand.NewSource(*seed))
		stats := runScenario(sc, *years, scaleUp, rng)
		fmt.Printf("┌─ Scenario: %s\n", sc.name)
		fmt.Printf("│ %-4s %10s %10s %9s %8s %9s %9s %7s %6s\n",
			"yr", "pop", "kids", "repro18-45", "eligPar", "ill_d", "nat_d", "births", "maxAge")
		for _, s := range stats {
			fmt.Printf("│ %-4d %10d %10d %9d %8d %9d %9d %7d %6d  mh=%.3f\n",
				s.year, s.pop, s.kids, s.repro, s.eligibleParents,
				s.deathsIllness, s.deathsNatural, s.births, s.maxAge, s.meanHealth)
		}
		final := stats[len(stats)-1]
		first := stats[0]
		verdict := "STABLE"
		if final.pop < first.pop/10 {
			verdict = "COLLAPSE"
		} else if final.pop < first.pop/2 {
			verdict = "DECLINE"
		} else if final.pop > first.pop*2 {
			verdict = "REBOUND"
		}
		fmt.Printf("└─ verdict: %s  (pop %d → %d, eligible parents %d → %d)\n\n",
			verdict, first.pop, final.pop, first.eligibleParents, final.eligibleParents)
	}
}
