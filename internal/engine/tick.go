// Package engine provides the tick-based simulation loop.
// See design doc Section 3.4 and Section 8.2.
package engine

import (
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"
)

// TickSchedule defines when each system runs relative to the tick counter.
const (
	TicksPerSimHour   = 60    // 60 ticks = 1 sim-hour
	TicksPerSimDay    = 1440  // 24 hours × 60
	TicksPerSimWeek   = 10080 // 7 days × 1440
	TicksPerSimSeason = 90000 // ~62.5 days
)

// Engine drives the simulation forward.
type Engine struct {
	Tick     uint64        // Current tick counter (monotonic, never resets; tick-loop goroutine only)
	Interval time.Duration // Base tick interval (default 1 second)

	// speed and running are read by the tick loop and read/written by HTTP
	// handler goroutines (handleSpeed / handleStatus / metrics), so they are
	// atomic. Use Speed()/SetSpeed() and IsRunning()/Stop(). speed holds a
	// float64 multiplier via math.Float64bits (0 = paused).
	speed   atomic.Uint64
	running atomic.Bool

	// Callbacks for each tick layer — populated during setup.
	OnTick   func(tick uint64) // Every tick (sim-minute)
	OnHour   func(tick uint64) // Every 60 ticks
	OnDay    func(tick uint64) // Every 1440 ticks
	OnWeek   func(tick uint64) // Every 10080 ticks
	OnSeason func(tick uint64) // Every ~90000 ticks

	// loopTasks carries functions to run synchronously inside the tick-loop
	// goroutine at a safe point between ticks (no agent mutation in flight).
	// Used for consistent full-state snapshots that must not race the loop.
	// See SubmitLoopTask / drainLoopTasks (W-21 fix).
	loopTasks chan func()
}

// NewEngine creates a simulation engine with default settings.
func NewEngine() *Engine {
	e := &Engine{
		Interval:  time.Second,
		loopTasks: make(chan func(), 8),
	}
	e.SetSpeed(1.0)
	return e
}

// Run starts the simulation loop. Blocks until Stop() is called.
func (e *Engine) Run() {
	e.running.Store(true)
	slog.Info("simulation engine started", "tick", e.Tick, "speed", e.Speed())

	for e.running.Load() {
		sp := e.Speed()
		if sp <= 0 {
			// Paused — service queued loop tasks (the sim is idle, so this is a
			// safe, mutation-free point), then sleep briefly and check again.
			e.drainLoopTasks()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		start := time.Now()

		e.step()

		// Safe point: the tick's mutations are complete and the next tick has
		// not begun, so queued tasks (e.g. snapshots) observe consistent state.
		e.drainLoopTasks()

		// Sleep for the remainder of the tick interval, adjusted for speed.
		elapsed := time.Since(start)
		target := time.Duration(float64(e.Interval) / sp)
		if elapsed < target {
			time.Sleep(target - elapsed)
		}
	}

	slog.Info("simulation engine stopped", "tick", e.Tick)
}

// Stop halts the simulation loop. Safe to call from any goroutine.
func (e *Engine) Stop() {
	e.running.Store(false)
}

// IsRunning reports whether the simulation loop is active. Safe from any goroutine.
func (e *Engine) IsRunning() bool {
	return e.running.Load()
}

// Speed returns the current speed multiplier (1.0 = real-time, 0 = paused).
// Safe from any goroutine.
func (e *Engine) Speed() float64 {
	return math.Float64frombits(e.speed.Load())
}

// SetSpeed sets the speed multiplier (1.0 = real-time, 0 = paused).
// Safe from any goroutine.
func (e *Engine) SetSpeed(s float64) {
	e.speed.Store(math.Float64bits(s))
}

// SubmitLoopTask enqueues fn to run synchronously inside the tick-loop
// goroutine at the next safe point between ticks (where no agent mutation is in
// flight). Returns false if the queue cannot accept the task within a short
// window (engine stalled or stopped). Callers needing the result should have fn
// signal completion (e.g. via a channel) and apply their own timeout.
//
// This exists so consistency-sensitive operations — notably a full-state
// snapshot — run without racing the tick loop. Saving from an HTTP goroutine
// while the loop mutated sim.Agents produced transient duplicate ids and a
// "UNIQUE constraint failed: agents.id" save failure (W-21).
func (e *Engine) SubmitLoopTask(fn func()) bool {
	select {
	case e.loopTasks <- fn:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

// drainLoopTasks runs all currently-queued loop tasks. Called from the tick-loop
// goroutine between ticks, so tasks observe a consistent, mutation-free state.
func (e *Engine) drainLoopTasks() {
	for {
		select {
		case fn := <-e.loopTasks:
			fn()
		default:
			return
		}
	}
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
//
// The season MUST be derived from TicksPerSimSeason — the same basis the engine
// uses in processSeason (`(tick / TicksPerSimSeason) % 4`) — so the displayed
// season always matches the season that actually drives mechanics (winter
// hardship, seasonal regen, crop yields). The old implementation assumed a
// 90-day season (129,600 ticks); with TicksPerSimSeason at 90,000 ticks
// (~62.5 days) the two clocks drifted apart, so the string reported "Winter"
// while the engine was mechanically in "Summer". Day-in-season and year are now
// expressed on the same TicksPerSimSeason basis for internal consistency.
func SimTime(tick uint64) string {
	minutes := tick % 60
	hours := (tick / 60) % 24

	seasonIndex := tick / TicksPerSimSeason
	season := seasonIndex % 4
	years := seasonIndex/4 + 1

	// Day within the current mechanical season (1-based).
	ticksIntoSeason := tick % TicksPerSimSeason
	dayInSeason := ticksIntoSeason/TicksPerSimDay + 1

	seasonNames := [4]string{"Spring", "Summer", "Autumn", "Winter"}

	return fmt.Sprintf("%s Day %d, %d:%02d Year %d",
		seasonNames[season], dayInSeason, hours, minutes, years)
}
