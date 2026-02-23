package gardener

import (
	"math"

	"github.com/talgya/mini-world/internal/phi"
)

// WorldHealth holds derived diagnostic signals computed from a WorldSnapshot.
// Runs before Haiku — deterministic and free.
type WorldHealth struct {
	DeathBirthRatio float64  // from last 2 history snapshots (per-snapshot delta, not cumulative)
	BirthTrend      []int    // last N births from history
	DeathTrend      []int    // last N deaths from history
	AvgSatisfaction float64
	AvgAlignment    float64
	SmallSettlements int     // pop < 50
	TinySettlements  int     // pop < 25
	TotalSettlements int
	TradePerCapita  float64
	CrisisLevel     string  // "CRITICAL", "WARNING", "WATCH", "HEALTHY"
}

// Triage computes a WorldHealth from the snapshot's data.
func Triage(snap *WorldSnapshot) *WorldHealth {
	h := &WorldHealth{
		AvgSatisfaction:  float64(snap.Status.AvgSatisfaction),
		AvgAlignment:     float64(snap.Status.AvgAlignment),
		TotalSettlements: len(snap.Settlements),
	}

	// Settlement size histogram.
	for _, s := range snap.Settlements {
		if s.Population < 25 {
			h.TinySettlements++
		}
		if s.Population < 50 {
			h.SmallSettlements++
		}
	}

	// Trade per capita.
	if snap.Status.Population > 0 {
		h.TradePerCapita = float64(snap.Economy.TradeVolume) / float64(snap.Status.Population)
	}

	// Death:Birth ratio from consecutive history snapshots (delta of cumulative values).
	// The /status endpoint returns cumulative totals — dividing those is meaningless.
	// Instead we use per-snapshot deltas from /stats/history.
	//
	// Counter reset detection: worldsim restarts reset cumulative birth/death
	// counters to 0. If newer.Births < older.Births, a restart happened between
	// those two snapshots. We skip that pair and scan backwards for a usable one.
	if len(snap.History) >= 2 {
		// History is sorted by tick DESC, so [0] is newest.
		for i := len(snap.History) - 1; i >= 0; i-- {
			h.BirthTrend = append(h.BirthTrend, snap.History[i].Births)
			h.DeathTrend = append(h.DeathTrend, snap.History[i].Deaths)
		}

		// Find a consecutive pair without a counter reset between them.
		foundPair := false
		for i := 0; i < len(snap.History)-1; i++ {
			newer := snap.History[i]
			older := snap.History[i+1]

			// Counter reset: cumulative values went backwards.
			if newer.Births < older.Births || newer.Deaths < older.Deaths {
				continue
			}

			deltaBirths := newer.Births - older.Births
			deltaDeaths := newer.Deaths - older.Deaths

			if deltaBirths > 0 {
				h.DeathBirthRatio = float64(deltaDeaths) / float64(deltaBirths)
			} else if deltaDeaths > 0 {
				h.DeathBirthRatio = math.Inf(1) // births genuinely stalled
			} else {
				h.DeathBirthRatio = 1.0 // both zero — neutral
			}
			foundPair = true
			break
		}

		if !foundPair {
			// All pairs span a restart — use population delta as fallback.
			newest := snap.History[0]
			oldest := snap.History[len(snap.History)-1]
			if newest.Population < oldest.Population {
				// Population shrinking — estimate crisis from decline rate.
				declinePct := float64(oldest.Population-newest.Population) / float64(oldest.Population)
				if declinePct > 0.1 {
					h.DeathBirthRatio = phi.Totality + 1 // signal CRITICAL
				} else {
					h.DeathBirthRatio = phi.Being // signal WARNING
				}
			} else {
				h.DeathBirthRatio = 1.0 // population growing — assume healthy
			}
		}

		// Capture satisfaction/alignment from newest history if available.
		newest := snap.History[0]
		if newest.AvgSatisfaction > 0 {
			h.AvgSatisfaction = newest.AvgSatisfaction
		}
		if newest.AvgAlignment > 0 {
			h.AvgAlignment = newest.AvgAlignment
		}
	}

	// Crisis level thresholds (Phi-derived).
	h.CrisisLevel = "HEALTHY"
	tinyFraction := 0.0
	if h.TotalSettlements > 0 {
		tinyFraction = float64(h.TinySettlements) / float64(h.TotalSettlements)
	}

	switch {
	case h.DeathBirthRatio > phi.Totality: // D:B > 4.236
		h.CrisisLevel = "CRITICAL"
	case h.TradePerCapita < 0.01 && snap.Status.Population > 100:
		h.CrisisLevel = "CRITICAL"
	case tinyFraction > 0.4:
		h.CrisisLevel = "CRITICAL"
	case h.DeathBirthRatio > phi.Being: // D:B > 1.618
		h.CrisisLevel = "WARNING"
	case h.DeathBirthRatio > phi.Monad: // D:B > 1.0
		h.CrisisLevel = "WATCH"
	}

	return h
}
