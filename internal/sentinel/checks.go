package sentinel

import (
	"fmt"
	"math"

	"github.com/talgya/mini-world/internal/phi"
)

// Level represents a health check severity.
type Level string

const (
	LevelHealthy  Level = "HEALTHY"
	LevelWatch    Level = "WATCH"
	LevelWarning  Level = "WARNING"
	LevelCritical Level = "CRITICAL"
)

// CheckResult holds the output of a single health check.
type CheckResult struct {
	Name      string  `json:"name"`
	Status    Level   `json:"status"`
	Value     float64 `json:"value"`
	Threshold string  `json:"threshold"`
	Trend     string  `json:"trend"`
	Detail    string  `json:"detail"`
}

// HealthSnapshot captures the key metrics from one cycle for trend analysis.
type HealthSnapshot struct {
	Tick            uint64  `json:"tick"`
	DeathBirthRatio float64 `json:"death_birth_ratio"`
	CrafterPct      float64 `json:"crafter_pct"`
	ProducerPct     float64 `json:"producer_pct"`
	WorkRate        float64 `json:"work_rate"`
	MarketHealth    float64 `json:"market_health"`
	TreasuryShare   float64 `json:"treasury_share"`
	GovScore        float64 `json:"gov_score"`
	Satisfaction    float64 `json:"satisfaction"`
	AvgSettHealth   float64 `json:"avg_sett_health"`
	Population      int     `json:"population"`
}

// The 8 main occupations for Tier 2 vitality check.
var mainOccupations = []string{
	"Farmer", "Miner", "Crafter", "Merchant",
	"Soldier", "Scholar", "Fisher", "Hunter",
}

// RunChecks runs all 8 health checks against a snapshot and returns the results
// plus a HealthSnapshot for trend storage.
func RunChecks(snap *WorldSnapshot, state *SentinelState) ([]CheckResult, HealthSnapshot) {
	hs := extractMetrics(snap)

	results := []CheckResult{
		checkPopulationVitality(snap, hs),
		checkOccupationBalance(hs),
		checkProducerHealth(snap, hs),
		checkEconomicCircuit(hs),
		checkGovernanceDiversity(snap, hs),
		checkTier2Vitality(snap),
		checkSatisfactionTrend(hs, state),
		checkLandHealth(hs),
	}

	// Guard: replace +Inf/-Inf/NaN values that break JSON marshaling.
	for i := range results {
		if math.IsInf(results[i].Value, 0) || math.IsNaN(results[i].Value) {
			results[i].Value = 0
		}
	}
	if math.IsInf(hs.DeathBirthRatio, 0) || math.IsNaN(hs.DeathBirthRatio) {
		hs.DeathBirthRatio = 0
	}

	return results, hs
}

// extractMetrics computes derived metrics from the raw snapshot.
func extractMetrics(snap *WorldSnapshot) HealthSnapshot {
	hs := HealthSnapshot{
		Tick:         snap.Status.Tick,
		WorkRate:     snap.Economy.ProducerHealth.WorkRate,
		MarketHealth: snap.Economy.AvgMarketHealth,
		GovScore:     snap.Social.Governance.AvgScore,
		Satisfaction: snap.Status.AvgSatisfaction,
		Population:   snap.Status.Population,
	}

	// Treasury share.
	if snap.Economy.TotalCrowns > 0 {
		hs.TreasuryShare = float64(snap.Economy.TreasuryWealth) / float64(snap.Economy.TotalCrowns)
	}

	// Occupation percentages.
	totalPop := 0
	crafterCount := 0
	producerCount := 0
	for name, occ := range snap.Status.Occupations {
		totalPop += occ.Count
		switch name {
		case "Crafter":
			crafterCount = occ.Count
		case "Farmer", "Miner", "Fisher", "Hunter", "Laborer", "Alchemist":
			producerCount += occ.Count
		}
	}
	if totalPop > 0 {
		hs.CrafterPct = float64(crafterCount) / float64(totalPop)
		hs.ProducerPct = float64(producerCount) / float64(totalPop)
	}

	// D:B ratio from stats history deltas.
	hs.DeathBirthRatio = computeDeathBirthRatio(snap.History)

	// Average settlement health.
	if len(snap.Settlements) > 0 {
		totalHealth := 0.0
		for _, s := range snap.Settlements {
			totalHealth += s.Health
		}
		hs.AvgSettHealth = totalHealth / float64(len(snap.Settlements))
	}

	return hs
}

// populationCap is the world population ceiling (MaxWorldPopulation in engine).
// The sentinel is read-only and doesn't import the engine package, so we
// define this locally. Kept in sync manually.
const populationCap = 400_000

// populationNearCap returns true when population is within 1% of the cap.
func populationNearCap(pop int) bool {
	return pop >= int(float64(populationCap)*0.99)
}

// computeDeathBirthRatio calculates the D:B ratio from history deltas.
// History is sorted by tick DESC, so [0] is newest.
func computeDeathBirthRatio(history []StatsHistoryRow) float64 {
	if len(history) < 2 {
		return 1.0
	}

	for i := 0; i < len(history)-1; i++ {
		newer := history[i]
		older := history[i+1]

		// Counter reset detection.
		if newer.Births < older.Births || newer.Deaths < older.Deaths {
			continue
		}

		deltaBirths := newer.Births - older.Births
		deltaDeaths := newer.Deaths - older.Deaths

		if deltaBirths > 0 {
			return float64(deltaDeaths) / float64(deltaBirths)
		} else if deltaDeaths > 0 {
			// Births gated — return a sentinel value that won't break JSON.
			return 0
		}
		return 1.0
	}

	// All pairs span a restart.
	return 1.0
}

// --- Individual checks ---

func checkPopulationVitality(snap *WorldSnapshot, hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name:  "population_vitality",
		Value: hs.DeathBirthRatio,
	}

	// Births gated at population cap — zero births is expected, not a crisis.
	birthsGated := hs.DeathBirthRatio == 0 && populationNearCap(hs.Population)
	if birthsGated {
		cr.Status = LevelHealthy
		cr.Value = 0
		cr.Threshold = fmt.Sprintf("Births gated at population cap (%dK)", populationCap/1000)
		cr.Detail = fmt.Sprintf("Births gated at population cap, population %dK", hs.Population/1000)
		return cr
	}

	dbStr := fmt.Sprintf("%.3f", hs.DeathBirthRatio)

	// Check for population decline trend.
	popDeclining := false
	if len(snap.History) >= 4 {
		declining := 0
		for i := 0; i < len(snap.History)-1 && i < 3; i++ {
			if snap.History[i].Population < snap.History[i+1].Population {
				declining++
			}
		}
		popDeclining = declining >= 3
	}

	switch {
	case hs.DeathBirthRatio > phi.Totality:
		cr.Status = LevelCritical
		cr.Threshold = fmt.Sprintf("D:B > %.3f (Totality)", phi.Totality)
	case hs.DeathBirthRatio > phi.Being || popDeclining:
		cr.Status = LevelWarning
		if popDeclining {
			cr.Threshold = "Population declining 3+ snapshots"
		} else {
			cr.Threshold = fmt.Sprintf("D:B > %.3f (Being)", phi.Being)
		}
	case hs.DeathBirthRatio > phi.Monad:
		cr.Status = LevelWatch
		cr.Threshold = fmt.Sprintf("D:B > %.3f (Monad)", phi.Monad)
	default:
		cr.Status = LevelHealthy
		cr.Threshold = fmt.Sprintf("D:B <= %.3f (Monad)", phi.Monad)
	}

	cr.Detail = fmt.Sprintf("D:B %s, population %dK", dbStr, hs.Population/1000)
	return cr
}

func checkOccupationBalance(hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name: "occupation_balance",
	}

	crafterLevel := LevelHealthy
	producerLevel := LevelHealthy

	// Crafter thresholds (higher is worse).
	// WATCH > 61.8% (Matter), WARNING > 73.6%, CRITICAL > 85.4%
	switch {
	case hs.CrafterPct > (phi.Matter + phi.Psyche) / 2 + phi.Matter: // ~85.4%
		crafterLevel = LevelCritical
	case hs.CrafterPct > phi.Matter+(phi.Matter-phi.Psyche)/2: // ~73.6%
		crafterLevel = LevelWarning
	case hs.CrafterPct > phi.Matter: // 61.8%
		crafterLevel = LevelWatch
	}

	// Producer thresholds (lower is worse).
	// WATCH < 23.6% (Agnosis), WARNING < 9.0%, CRITICAL < 5.6%
	switch {
	case hs.ProducerPct < phi.Agnosis*phi.Agnosis: // ~5.6%
		producerLevel = LevelCritical
	case hs.ProducerPct < phi.Agnosis*phi.Psyche: // ~9.0%
		producerLevel = LevelWarning
	case hs.ProducerPct < phi.Agnosis: // 23.6%
		producerLevel = LevelWatch
	}

	// Use the worse of the two.
	cr.Status = worstLevel(crafterLevel, producerLevel)
	cr.Value = hs.CrafterPct
	cr.Threshold = fmt.Sprintf("Crafter <= %.1f%% (Matter), Producer >= %.1f%% (Agnosis)",
		phi.Matter*100, phi.Agnosis*100)
	cr.Detail = fmt.Sprintf("Crafter %.1f%%, Producer %.1f%%",
		hs.CrafterPct*100, hs.ProducerPct*100)
	return cr
}

func checkProducerHealth(snap *WorldSnapshot, hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name: "producer_health",
	}

	workLevel := LevelHealthy
	switch {
	case hs.WorkRate < phi.Agnosis*phi.Agnosis: // ~5.6%
		workLevel = LevelCritical
	case hs.WorkRate < phi.Agnosis*phi.Matter: // ~14.6%
		workLevel = LevelWarning
	case hs.WorkRate < phi.Agnosis: // 23.6%
		workLevel = LevelWatch
	}

	// Satisfaction gap: max - min across occupations.
	satGapLevel := LevelHealthy
	satGap := occupationSatisfactionGap(snap)
	switch {
	case satGap > phi.Monad: // 1.0
		satGapLevel = LevelCritical
	case satGap > phi.Matter: // 0.618
		satGapLevel = LevelWarning
	case satGap > phi.Psyche: // 0.382
		satGapLevel = LevelWatch
	}

	cr.Status = worstLevel(workLevel, satGapLevel)
	cr.Value = hs.WorkRate
	cr.Threshold = fmt.Sprintf("Work rate >= %.1f%% (Agnosis), Sat gap <= %.3f (Psyche)",
		phi.Agnosis*100, phi.Psyche)
	cr.Detail = fmt.Sprintf("Work rate %.1f%%, satisfaction gap %.3f",
		hs.WorkRate*100, satGap)
	return cr
}

func checkEconomicCircuit(hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name: "economic_circuit",
	}

	// Market health thresholds.
	// Use Φ-derived: 1 - Agnosis*0.236 = ~94.4%, 1 - Agnosis*0.382 = ~91.0%, Matter = 61.8%
	marketLevel := LevelHealthy
	switch {
	case hs.MarketHealth < phi.Matter: // 61.8%
		marketLevel = LevelCritical
	case hs.MarketHealth < 1-phi.Agnosis*phi.Psyche: // ~91.0%
		marketLevel = LevelWarning
	case hs.MarketHealth < 1-phi.Agnosis*phi.Agnosis: // ~94.4%
		marketLevel = LevelWatch
	}

	// Treasury share thresholds.
	treasuryLevel := LevelHealthy
	switch {
	case hs.TreasuryShare > phi.Matter+(phi.Matter-phi.Psyche)/2: // ~73.6%
		treasuryLevel = LevelWarning
	case hs.TreasuryShare > phi.Matter: // 61.8%
		treasuryLevel = LevelWatch
	}

	cr.Status = worstLevel(marketLevel, treasuryLevel)
	cr.Value = hs.MarketHealth
	cr.Threshold = fmt.Sprintf("Market >= %.1f%%, Treasury <= %.1f%% (Matter)",
		(1-phi.Agnosis*phi.Agnosis)*100, phi.Matter*100)
	cr.Detail = fmt.Sprintf("Market health %.1f%%, treasury share %.1f%%",
		hs.MarketHealth*100, hs.TreasuryShare*100)
	return cr
}

func checkGovernanceDiversity(snap *WorldSnapshot, hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name: "governance_diversity",
	}

	// Governance score thresholds (higher score = less diverse, worse).
	scoreLevel := LevelHealthy
	switch {
	case hs.GovScore > 1-phi.Agnosis*phi.Agnosis: // ~94.4%
		scoreLevel = LevelWarning
	case hs.GovScore > phi.Matter+(phi.Matter-phi.Psyche)/2: // ~73.6%
		scoreLevel = LevelWatch
	}

	// Dominant governance type percentage.
	govCounts := make(map[string]int)
	for _, s := range snap.Settlements {
		govCounts[s.Governance]++
	}
	dominantPct := 0.0
	dominantType := ""
	total := len(snap.Settlements)
	if total > 0 {
		for gov, count := range govCounts {
			pct := float64(count) / float64(total)
			if pct > dominantPct {
				dominantPct = pct
				dominantType = gov
			}
		}
	}

	domLevel := LevelHealthy
	switch {
	case dominantPct > 1-phi.Agnosis*phi.Agnosis: // ~94.4%
		domLevel = LevelWarning
	case dominantPct > phi.Matter+(phi.Matter-phi.Psyche)/2: // ~73.6%
		domLevel = LevelWatch
	}

	cr.Status = worstLevel(scoreLevel, domLevel)
	cr.Value = hs.GovScore
	cr.Threshold = fmt.Sprintf("Score <= %.1f%%, Dominant <= %.1f%%",
		(phi.Matter+(phi.Matter-phi.Psyche)/2)*100, (phi.Matter+(phi.Matter-phi.Psyche)/2)*100)
	cr.Detail = fmt.Sprintf("Avg score %.3f, %s %.1f%% dominant",
		hs.GovScore, dominantType, dominantPct*100)
	return cr
}

func checkTier2Vitality(snap *WorldSnapshot) CheckResult {
	cr := CheckResult{
		Name: "tier2_vitality",
	}

	// Count alive Tier 2 agents by occupation.
	aliveByOcc := make(map[string]int)
	for _, a := range snap.Agents {
		if a.Alive {
			aliveByOcc[a.Occupation]++
		}
	}

	missing := 0
	var missingOccs []string
	for _, occ := range mainOccupations {
		if aliveByOcc[occ] == 0 {
			missing++
			missingOccs = append(missingOccs, occ)
		}
	}

	cr.Value = float64(missing)
	switch {
	case missing >= 3:
		cr.Status = LevelCritical
	case missing >= 2:
		cr.Status = LevelWarning
	case missing >= 1:
		cr.Status = LevelWatch
	default:
		cr.Status = LevelHealthy
	}

	cr.Threshold = "All 8 main occupations represented in Tier 2"
	if missing > 0 {
		cr.Detail = fmt.Sprintf("%d missing: %v", missing, missingOccs)
	} else {
		cr.Detail = "All 8 occupations have alive Tier 2 agents"
	}
	return cr
}

func checkSatisfactionTrend(hs HealthSnapshot, state *SentinelState) CheckResult {
	cr := CheckResult{
		Name:  "satisfaction_trend",
		Value: hs.Satisfaction,
	}

	// Absolute thresholds.
	absLevel := LevelHealthy
	switch {
	case hs.Satisfaction < 0:
		absLevel = LevelCritical
	case hs.Satisfaction < phi.Agnosis: // 0.236
		absLevel = LevelWarning
	case hs.Satisfaction < phi.Psyche: // 0.382
		absLevel = LevelWatch
	}

	// Consecutive decline count from ring buffer.
	declines := ConsecutiveDeclines(state.Snapshots, "satisfaction")
	trendLevel := LevelHealthy
	switch {
	case declines >= 5:
		trendLevel = LevelWarning
	case declines >= 3:
		trendLevel = LevelWatch
	}

	cr.Status = worstLevel(absLevel, trendLevel)
	cr.Threshold = fmt.Sprintf("Satisfaction >= %.3f (Psyche), no 3+ decline streak", phi.Psyche)
	cr.Detail = fmt.Sprintf("Satisfaction %.3f, %d consecutive declines", hs.Satisfaction, declines)
	return cr
}

func checkLandHealth(hs HealthSnapshot) CheckResult {
	cr := CheckResult{
		Name: "land_health",
	}

	// Primary metric: work rate — the direct measure of whether land supports production.
	// Settlement health from the API is organizational health (always ~1.0), not hex health.
	workLevel := LevelHealthy
	switch {
	case hs.WorkRate < phi.Agnosis*phi.Agnosis: // ~5.6%
		workLevel = LevelCritical
	case hs.WorkRate < phi.Agnosis*phi.Matter: // ~14.6%
		workLevel = LevelWarning
	case hs.WorkRate < phi.Agnosis: // 23.6%
		workLevel = LevelWatch
	}

	// Secondary: settlement health (organizational, useful if it ever degrades).
	healthLevel := LevelHealthy
	switch {
	case hs.AvgSettHealth < phi.Psyche: // 0.382
		healthLevel = LevelWarning
	case hs.AvgSettHealth < phi.Matter: // 0.618
		healthLevel = LevelWatch
	}

	cr.Status = worstLevel(workLevel, healthLevel)
	cr.Value = hs.WorkRate
	cr.Threshold = fmt.Sprintf("Work rate >= %.1f%% (Agnosis), avg health >= %.3f (Matter)",
		phi.Agnosis*100, phi.Matter)
	cr.Detail = fmt.Sprintf("Work rate %.1f%%, avg settlement health %.3f",
		hs.WorkRate*100, hs.AvgSettHealth)
	return cr
}

// --- Helpers ---

func occupationSatisfactionGap(snap *WorldSnapshot) float64 {
	if len(snap.Status.Occupations) == 0 {
		return 0
	}
	minSat := math.MaxFloat64
	maxSat := -math.MaxFloat64
	for _, occ := range snap.Status.Occupations {
		if occ.Count == 0 {
			continue
		}
		if occ.AvgSatisfaction < minSat {
			minSat = occ.AvgSatisfaction
		}
		if occ.AvgSatisfaction > maxSat {
			maxSat = occ.AvgSatisfaction
		}
	}
	if minSat == math.MaxFloat64 {
		return 0
	}
	return maxSat - minSat
}

func worstLevel(a, b Level) Level {
	if levelSeverity(a) > levelSeverity(b) {
		return a
	}
	return b
}

func levelSeverity(l Level) int {
	switch l {
	case LevelCritical:
		return 3
	case LevelWarning:
		return 2
	case LevelWatch:
		return 1
	default:
		return 0
	}
}
