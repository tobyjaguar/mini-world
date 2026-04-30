// Direct-Sat bump helper for INV-11 saturation observability.
//
// Per INV-11 closure (2026-04-23): when an upstream/adjacent need (Safety,
// Belonging, Esteem, or Purpose) is at the [0,1] clamp ceiling at the moment
// a direct Wellbeing.Satisfaction bump fires, the EMA equilibrium formula
// `currentSat ≈ satTarget + daily_bumps/14.4` significantly under-predicts
// the steady-state Sat shift — by ~5–15× empirically. The R70-2 case was
// canonical: predicted +0.005 Sat shift, observed +0.064 (~13× amplification).
//
// This helper makes the saturation co-occurrence visible at runtime via a
// per-source counter exposed on /api/v1/metrics. It does NOT change behavior;
// the architectural decision (legitimate first-class mechanic vs technical
// debt) is deferred to roadmap Phase 3.3.
//
// Usage at each direct-Sat bump call site (Section 9 of
// docs/10-mood-revision-proposal.md enumerates them):
//
//	a.ApplyDirectSatBump(+0.05, "eat")
//	a.ApplyDirectSatBump(-0.2, "crime.caught")
//	a.ApplyDirectSatBump(+0.08, "stipend")
//	... etc.

package agents

import "sync"

// satBumpCounter is a per-source accumulator of direct-Sat bump events that
// fired while at least one of (Safety, Belonging, Esteem, Purpose) was ≥0.99.
// Mutex-guarded for safe concurrent access from per-tick agent loops.
type satBumpCounter struct {
	mu     sync.Mutex
	counts map[string]uint64
}

// SatBumpSaturatedCounter is the global counter. Read snapshots via Snapshot();
// expose on /api/v1/metrics.
var SatBumpSaturatedCounter = &satBumpCounter{counts: make(map[string]uint64)}

func (c *satBumpCounter) add(source string) {
	c.mu.Lock()
	c.counts[source]++
	c.mu.Unlock()
}

// Snapshot returns a point-in-time copy of all per-source counts. Safe to call
// from any goroutine; callers may mutate the returned map without affecting
// the live counter.
func (c *satBumpCounter) Snapshot() map[string]uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]uint64, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out
}

// ApplyDirectSatBump applies a delta to Wellbeing.Satisfaction and increments
// the saturation counter for `source` if any non-Survival need is at or above
// 0.99 at the moment of the bump. `source` should be a stable, low-cardinality
// tag identifying the call site (e.g. "eat", "stipend", "crime.caught").
//
// Survival is intentionally excluded from the saturation check — it almost
// never reaches 1.0 due to the eat-cycle equilibrium (INV-3, ~0.39 mean).
func (a *Agent) ApplyDirectSatBump(delta float32, source string) {
	a.Wellbeing.Satisfaction += delta
	if a.Needs.Safety >= 0.99 ||
		a.Needs.Belonging >= 0.99 ||
		a.Needs.Esteem >= 0.99 ||
		a.Needs.Purpose >= 0.99 {
		SatBumpSaturatedCounter.add(source)
	}
}
