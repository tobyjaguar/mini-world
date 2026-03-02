package sentinel

// Trend describes the direction of a metric.
type Trend string

const (
	TrendImproving Trend = "improving"
	TrendStable    Trend = "stable"
	TrendDeclining Trend = "declining"
)

// trendThreshold is the percentage change needed to register as non-stable.
const trendThreshold = 0.05 // 5%

// metricPolarity defines whether higher is better or lower is better.
type metricPolarity int

const (
	higherIsBetter metricPolarity = iota
	lowerIsBetter
)

// checkMetrics maps check names to the snapshot field extractor and polarity.
var checkMetrics = map[string]struct {
	extract  func(HealthSnapshot) float64
	polarity metricPolarity
}{
	"population_vitality": {func(s HealthSnapshot) float64 { return s.DeathBirthRatio }, lowerIsBetter},
	"occupation_balance":  {func(s HealthSnapshot) float64 { return s.CrafterPct }, lowerIsBetter},
	"producer_health":     {func(s HealthSnapshot) float64 { return s.WorkRate }, higherIsBetter},
	"economic_circuit":    {func(s HealthSnapshot) float64 { return s.MarketHealth }, higherIsBetter},
	"governance_diversity": {func(s HealthSnapshot) float64 { return s.GovScore }, lowerIsBetter},
	"satisfaction_trend":  {func(s HealthSnapshot) float64 { return s.Satisfaction }, higherIsBetter},
	"land_health":         {func(s HealthSnapshot) float64 { return s.AvgSettHealth }, higherIsBetter},
}

// ComputeTrend compares the current value to the rolling average of the last 5 snapshots.
func ComputeTrend(snapshots []HealthSnapshot, checkName string) Trend {
	m, ok := checkMetrics[checkName]
	if !ok || len(snapshots) < 2 {
		return TrendStable
	}

	// Current value is the last snapshot.
	current := m.extract(snapshots[len(snapshots)-1])

	// Rolling average of up to 5 previous snapshots (excluding the current one).
	count := 0
	sum := 0.0
	start := len(snapshots) - 6 // 5 previous + skip current
	if start < 0 {
		start = 0
	}
	for i := start; i < len(snapshots)-1; i++ {
		sum += m.extract(snapshots[i])
		count++
	}
	if count == 0 {
		return TrendStable
	}
	avg := sum / float64(count)

	if avg == 0 {
		return TrendStable
	}

	change := (current - avg) / abs(avg)

	switch m.polarity {
	case higherIsBetter:
		if change > trendThreshold {
			return TrendImproving
		} else if change < -trendThreshold {
			return TrendDeclining
		}
	case lowerIsBetter:
		if change < -trendThreshold {
			return TrendImproving
		} else if change > trendThreshold {
			return TrendDeclining
		}
	}

	return TrendStable
}

// ConsecutiveDeclines counts how many consecutive snapshots show declining values
// for the given check, walking backwards from the most recent.
func ConsecutiveDeclines(snapshots []HealthSnapshot, checkName string) int {
	m, ok := checkMetrics[checkName]
	if !ok || len(snapshots) < 2 {
		return 0
	}

	count := 0
	for i := len(snapshots) - 1; i > 0; i-- {
		curr := m.extract(snapshots[i])
		prev := m.extract(snapshots[i-1])

		var declining bool
		switch m.polarity {
		case higherIsBetter:
			declining = curr < prev
		case lowerIsBetter:
			declining = curr > prev
		}

		if declining {
			count++
		} else {
			break
		}
	}
	return count
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
