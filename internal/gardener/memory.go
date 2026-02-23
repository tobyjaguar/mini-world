package gardener

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	memoryFile    = "gardener_memory.json"
	maxRecords    = 10
	promptRecords = 5 // how many recent records to include in the Haiku prompt
)

// CycleRecord captures what happened in a single gardener cycle.
type CycleRecord struct {
	Tick         uint64  `json:"tick"`
	Action       string  `json:"action"`
	DeathBirth   float64 `json:"death_birth_ratio"`
	Satisfaction float64 `json:"satisfaction"`
	Alignment    float64 `json:"alignment"`
	CrisisLevel  string  `json:"crisis_level"`
	Settlement   string  `json:"settlement,omitempty"`
	Rationale    string  `json:"rationale,omitempty"`
}

// CycleMemory manages a ring of recent gardener cycle records.
type CycleMemory struct {
	Records []CycleRecord `json:"records"`
}

// LoadMemory reads the memory file from disk. Returns empty memory if not found.
func LoadMemory() *CycleMemory {
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		return &CycleMemory{}
	}
	var mem CycleMemory
	if err := json.Unmarshal(data, &mem); err != nil {
		slog.Warn("gardener memory corrupted, starting fresh", "error", err)
		return &CycleMemory{}
	}
	return &mem
}

// Save writes the memory to disk.
func (m *CycleMemory) Save() {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		slog.Error("failed to marshal gardener memory", "error", err)
		return
	}
	if err := os.WriteFile(memoryFile, data, 0644); err != nil {
		slog.Error("failed to write gardener memory", "error", err)
	}
}

// Record adds a cycle record, trimming to maxRecords.
func (m *CycleMemory) Record(r CycleRecord) {
	m.Records = append(m.Records, r)
	if len(m.Records) > maxRecords {
		m.Records = m.Records[len(m.Records)-maxRecords:]
	}
}

// FormatForPrompt returns a string summarizing the last N cycles for inclusion in the Haiku prompt.
func (m *CycleMemory) FormatForPrompt() string {
	if len(m.Records) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Recent Gardener Cycles\n")

	start := 0
	if len(m.Records) > promptRecords {
		start = len(m.Records) - promptRecords
	}
	for _, r := range m.Records[start:] {
		fmt.Fprintf(&b, "- Tick %d: action=%s, D:B=%.2f, satisfaction=%.3f, alignment=%.3f, crisis=%s",
			r.Tick, r.Action, r.DeathBirth, r.Satisfaction, r.Alignment, r.CrisisLevel)
		if r.Settlement != "" {
			fmt.Fprintf(&b, ", settlement=%s", r.Settlement)
		}
		b.WriteString("\n")
	}
	return b.String()
}
