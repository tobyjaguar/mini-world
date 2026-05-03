package agents

import (
	"testing"
)

// makeAgents constructs a slice of agents from a compact (id, alive) spec.
// Helper to keep the table-driven cases readable.
func makeAgents(spec ...struct {
	id    AgentID
	alive bool
}) []*Agent {
	out := make([]*Agent, 0, len(spec))
	for _, s := range spec {
		if s.id == 0 {
			out = append(out, nil)
			continue
		}
		out = append(out, &Agent{ID: s.id, Alive: s.alive})
	}
	return out
}

func TestAlive(t *testing.T) {
	type entry = struct {
		id    AgentID
		alive bool
	}

	cases := []struct {
		name    string
		input   []*Agent
		wantIDs []AgentID
	}{
		{
			name:    "empty slice yields nothing",
			input:   nil,
			wantIDs: nil,
		},
		{
			name:    "all alive yields all",
			input:   makeAgents(entry{1, true}, entry{2, true}, entry{3, true}),
			wantIDs: []AgentID{1, 2, 3},
		},
		{
			name:    "all dead yields nothing",
			input:   makeAgents(entry{1, false}, entry{2, false}, entry{3, false}),
			wantIDs: nil,
		},
		{
			name:    "mixed yields only alive in order",
			input:   makeAgents(entry{1, true}, entry{2, false}, entry{3, true}, entry{4, false}, entry{5, true}),
			wantIDs: []AgentID{1, 3, 5},
		},
		{
			name:    "nil entries skipped",
			input:   makeAgents(entry{1, true}, entry{0, false}, entry{2, true}, entry{0, false}),
			wantIDs: []AgentID{1, 2},
		},
		{
			name:    "single alive agent",
			input:   makeAgents(entry{42, true}),
			wantIDs: []AgentID{42},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []AgentID
			for a := range Alive(tc.input) {
				got = append(got, a.ID)
			}
			if !equalIDs(got, tc.wantIDs) {
				t.Fatalf("Alive() = %v, want %v", got, tc.wantIDs)
			}
		})
	}
}

// TestAliveEarlyExit ensures the iterator honors `break` from the body
// and does not continue iterating after the consumer signals stop.
// This is critical for short-circuiting search loops over agents.
func TestAliveEarlyExit(t *testing.T) {
	agents := []*Agent{
		{ID: 1, Alive: true},
		{ID: 2, Alive: true},
		{ID: 3, Alive: true},
		{ID: 4, Alive: true},
	}
	visited := 0
	for a := range Alive(agents) {
		visited++
		if a.ID == 2 {
			break
		}
	}
	if visited != 2 {
		t.Fatalf("expected to visit 2 agents before break, visited %d", visited)
	}
}

func equalIDs(a, b []AgentID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
