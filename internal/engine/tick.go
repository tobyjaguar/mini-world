// Package engine provides the tick-based simulation loop.
// See design doc Section 3.4 and Section 8.2.
package engine

import (
	"fmt"
	"log/slog"
	"time"
)

// TickSchedule defines when each system runs relative to the tick counter.
const (
	TicksPerSimHour   = 60       // 60 ticks = 1 sim-hour
	TicksPerSimDay    = 1440     // 24 hours × 60
	TicksPerSimWeek   = 10080    // 7 days × 1440
	TicksPerSimSeason = 90000    // ~62.5 days
)

// Engine drives the simulation forward.
type Engine struct {
	Tick     uint64        // Current tick counter (monotonic, never resets)
	Speed    float64       // Multiplier: 1.0 = real-time, 0 = paused
	Interval time.Duration // Base tick interval (default 1 second)
	Running  bool

	// Callbacks for each tick layer — populated during setup.
	OnTick       func(tick uint64) // Every tick (sim-minute)
	OnHour       func(tick uint64) // Every 60 ticks
	OnDay        func(tick uint64) // Every 1440 ticks
	OnWeek       func(tick uint64) // Every 10080 ticks
	OnSeason     func(tick uint64) // Every ~90000 ticks
}

// NewEngine creates a simulation engine with default settings.
func NewEngine() *Engine {
	return &Engine{
		Tick:     0,
		Speed:    1.0,
		Interval: time.Second,
		Running:  false,
	}
}

// Run starts the simulation loop. Blocks until Stop() is called.
func (e *Engine) Run() {
	e.Running = true
	slog.Info("simulation engine started", "tick", e.Tick, "speed", e.Speed)

	for e.Running {
		if e.Speed <= 0 {
			// Paused — sleep briefly and check again.
			time.Sleep(100 * time.Millisecond)
			continue
		}

		start := time.Now()

		e.step()

		// Sleep for the remainder of the tick interval, adjusted for speed.
		elapsed := time.Since(start)
		target := time.Duration(float64(e.Interval) / e.Speed)
		if elapsed < target {
			time.Sleep(target - elapsed)
		}
	}

	slog.Info("simulation engine stopped", "tick", e.Tick)
}

// Stop halts the simulation loop.
func (e *Engine) Stop() {
	e.Running = false
}

// step advances the simulation by one tick.
func (e *Engine) step() {
	e.Tick++

	// Every tick: fast rule-based updates.
	if e.OnTick != nil {
		e.OnTick(e.Tick)
	}

	// Every sim-hour: market resolution, weather, event checks.
	if e.Tick%TicksPerSimHour == 0 && e.OnHour != nil {
		e.OnHour(e.Tick)
	}

	// Every sim-day: daily summaries, births/deaths, building progress.
	if e.Tick%TicksPerSimDay == 0 && e.OnDay != nil {
		e.OnDay(e.Tick)
	}

	// Every sim-week: diplomatic cycles, faction updates, LLM decisions.
	if e.Tick%TicksPerSimWeek == 0 && e.OnWeek != nil {
		e.OnWeek(e.Tick)
	}

	// Every sim-season: harvests, seasonal shifts, major narrative arcs.
	if e.Tick%TicksPerSimSeason == 0 && e.OnSeason != nil {
		e.OnSeason(e.Tick)
	}
}

// SimTime returns a human-readable simulation time string from a tick number.
func SimTime(tick uint64) string {
	totalMinutes := tick
	minutes := totalMinutes % 60
	totalHours := totalMinutes / 60
	hours := totalHours % 24
	totalDays := totalHours / 24
	days := totalDays%90 + 1
	seasons := totalDays / 90
	season := seasons % 4
	years := seasons/4 + 1

	seasonNames := [4]string{"Spring", "Summer", "Autumn", "Winter"}

	return fmt.Sprintf("%s Day %d, %d:%02d Year %d",
		seasonNames[season], days, hours, minutes, years)
}
