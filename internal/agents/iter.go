// Iteration helpers (R82, Phase 5.3). The engine has 81+ `range s.Agents`
// loops, each independently checking `!a.Alive` (or, in cold paths,
// forgetting to). Centralizing the alive-filter into a single iterator
// kills a class of subtle bugs and makes call sites read more cleanly:
//
//   for a := range agents.Alive(s.Agents) { ... }
//
// instead of
//
//   for _, a := range s.Agents {
//       if !a.Alive { continue }
//       ...
//   }
//
// This file deliberately does not refactor the existing call sites — that
// is a separate sweep, intentionally decoupled so the helper can land
// first and be proven, then sites convert one or a few at a time.

package agents

import "iter"

// Alive returns a Go 1.23+ range-over-func iterator that yields only the
// alive agents in the slice, preserving order. Stop early by returning
// false from the loop body (e.g. via `break`); this is honored by the
// iterator and avoids continued iteration once the consumer is done.
func Alive(agents []*Agent) iter.Seq[*Agent] {
	return func(yield func(*Agent) bool) {
		for _, a := range agents {
			if a == nil || !a.Alive {
				continue
			}
			if !yield(a) {
				return
			}
		}
	}
}
