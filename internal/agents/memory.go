// Agent memory stream — records of notable experiences for Tier 2 cognition context.
// See design doc Section 4.2 (Tier 2 needs situational context to make decisions).
package agents

import "sort"

const MaxMemories = 50

// Memory records a notable experience in an agent's life.
type Memory struct {
	Tick       uint64  `json:"tick"`
	Content    string  `json:"content"`
	Importance float32 `json:"importance"` // 0.0–1.0
}

// AddMemory appends a memory to the agent's stream. When full, drops the
// lowest-importance memory to make room.
func AddMemory(a *Agent, tick uint64, content string, importance float32) {
	m := Memory{Tick: tick, Content: content, Importance: importance}

	if len(a.Memories) < MaxMemories {
		a.Memories = append(a.Memories, m)
		return
	}

	// Find the lowest-importance memory and replace it.
	minIdx := 0
	for i := 1; i < len(a.Memories); i++ {
		if a.Memories[i].Importance < a.Memories[minIdx].Importance {
			minIdx = i
		}
	}
	if m.Importance > a.Memories[minIdx].Importance {
		a.Memories[minIdx] = m
	}
}

// RecentMemories returns the most recent N memories ordered by tick descending.
func RecentMemories(a *Agent, count int) []Memory {
	if len(a.Memories) == 0 {
		return nil
	}

	// Copy and sort by tick descending.
	sorted := make([]Memory, len(a.Memories))
	copy(sorted, a.Memories)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Tick > sorted[j].Tick
	})

	if count > len(sorted) {
		count = len(sorted)
	}
	return sorted[:count]
}

// ImportantMemories returns the top N memories by importance.
func ImportantMemories(a *Agent, count int) []Memory {
	if len(a.Memories) == 0 {
		return nil
	}

	sorted := make([]Memory, len(a.Memories))
	copy(sorted, a.Memories)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Importance > sorted[j].Importance
	})

	if count > len(sorted) {
		count = len(sorted)
	}
	return sorted[:count]
}
