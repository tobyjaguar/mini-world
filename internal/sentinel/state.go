package sentinel

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	stateFile  = "sentinel_state.json"
	reportFile = "sentinel_report.json"
)

// SentinelState holds the persistent state across cycles.
type SentinelState struct {
	Snapshots []HealthSnapshot       `json:"snapshots"`
	Alerts    map[string]*AlertState `json:"alerts"`
	CycleNum  int                    `json:"cycle_num"`
	dataDir   string
}

const maxSnapshots = 20

// LoadState reads the state file from disk. Returns fresh state if not found.
func LoadState(dataDir string) *SentinelState {
	st := &SentinelState{
		Alerts:  make(map[string]*AlertState),
		dataDir: dataDir,
	}

	path := filepath.Join(dataDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if err := json.Unmarshal(data, st); err != nil {
		slog.Warn("sentinel state corrupted, starting fresh", "error", err)
		return &SentinelState{
			Alerts:  make(map[string]*AlertState),
			dataDir: dataDir,
		}
	}
	st.dataDir = dataDir
	if st.Alerts == nil {
		st.Alerts = make(map[string]*AlertState)
	}
	return st
}

// Save writes the state to disk.
func (s *SentinelState) Save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		slog.Error("failed to marshal sentinel state", "error", err)
		return
	}
	path := filepath.Join(s.dataDir, stateFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("failed to write sentinel state", "error", err)
	}
}

// AddSnapshot appends a snapshot, trimming to maxSnapshots.
func (s *SentinelState) AddSnapshot(snap HealthSnapshot) {
	s.Snapshots = append(s.Snapshots, snap)
	if len(s.Snapshots) > maxSnapshots {
		s.Snapshots = s.Snapshots[len(s.Snapshots)-maxSnapshots:]
	}
}

// SaveReport writes the report JSON to disk.
func (s *SentinelState) SaveReport(report *Report) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		slog.Error("failed to marshal sentinel report", "error", err)
		return
	}
	path := filepath.Join(s.dataDir, reportFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("failed to write sentinel report", "error", err)
	}
}
