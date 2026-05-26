package engine

import (
	"testing"
	"time"
)

// TestSubmitLoopTaskRunsInLoop verifies the W-21 fix: a task submitted via
// SubmitLoopTask executes inside the tick-loop goroutine (between ticks), so
// consistency-sensitive work like a full snapshot never races the loop.
func TestSubmitLoopTaskRunsInLoop(t *testing.T) {
	e := NewEngine()
	e.Interval = time.Millisecond // run fast for the test

	// ranOnTick is set inside the loop goroutine and observed via the
	// channel-synchronized task below, so no separate read race.
	tickCh := make(chan struct{}, 1)
	e.OnTick = func(uint64) {
		select {
		case tickCh <- struct{}{}:
		default:
		}
	}

	go e.Run()
	defer e.Stop()

	done := make(chan struct{})
	if !e.SubmitLoopTask(func() { close(done) }) {
		t.Fatal("SubmitLoopTask returned false (queue unavailable)")
	}

	select {
	case <-done:
		// task ran inside the loop goroutine
	case <-time.After(2 * time.Second):
		t.Fatal("loop task did not run within 2s")
	}

	// Confirm the loop was actually ticking (not just draining a paused queue).
	select {
	case <-tickCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected the engine loop to have ticked")
	}
}

// TestSubmitLoopTaskWhilePaused verifies tasks are still serviced when the
// engine is paused (Speed == 0) — the snapshot endpoint must work even if the
// sim is paused by an operator.
func TestSubmitLoopTaskWhilePaused(t *testing.T) {
	e := NewEngine()
	e.SetSpeed(0) // paused
	e.Interval = time.Millisecond

	go e.Run()
	defer e.Stop()

	ran := make(chan struct{})
	if !e.SubmitLoopTask(func() { close(ran) }) {
		t.Fatal("SubmitLoopTask returned false while paused")
	}

	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("loop task did not run while paused")
	}
}
