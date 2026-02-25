# Live View Animation Model — Stack-Wide Design

## Stack Architecture Principle

Crossworlds runs across three repos, three processes, and two servers:

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   mini-world     │     │ crossworlds-relay │     │   crossworlds    │
│   (worldsim)     │────▶│   (SSE relay)     │────▶│   (frontend)     │
│   Go · SQLite    │ SSE │   Go · stdlib     │ SSE │   Next.js · Vercel│
│   DreamCompute   │     │   DreamCompute    │     │   Vercel CDN     │
└──────────────────┘     └──────────────────┘     └──────────────────┘
     sacred                 workhorse              presentation
```

**The world server is sacred.** It runs a 164K-agent simulation on a 1GB RAM machine. Every change to worldsim must justify itself against that constraint. Anyone should be able to run their own worldsim cheaply.

**The relay is the workhorse.** It's the right place for enrichment, aggregation, derived state, and any computation that serves the viewer experience. It has no database, no simulation state — just events flowing through it. It can be as smart as we need without touching the world.

**The frontend is the presentation.** All animation, physics, interpolation, and rendering runs client-side. The browser does the expensive work (60fps canvas), not the servers.

**Rule: push complexity outward.** Worldsim emits raw facts. The relay enriches them. The frontend renders them. Never pull presentation concerns into the world server.

## The Problem with the Current Live View

The ant-farm canvas has dots that drift with Brownian motion inside zone attractors. It's a screensaver, not a window into a living world. Specifically:

1. **Nothing happens.** Every farmer drifts identically. Every merchant drifts identically. There's no visible difference between a thriving settlement and a dying one.
2. **SSE events are wasted.** The stream carries births, deaths, crimes, trade, governance changes — but they only show up in a tiny text ticker at the bottom. The canvas ignores them entirely.
3. **No sense of time.** The world has day/night, seasons, weather. The canvas is always the same dark background.
4. **No sense of place.** Zones are floating text labels. There's no spatial structure that makes the scene legible.
5. **Dots don't *do* anything.** In a real ant farm, you watch ants carry food, dig tunnels, interact. Here, dots jiggle.

## Design: Event-Driven Behavioral Animation

### Layer 1: Structured Events (worldsim)

**Current:** Events are `{tick, description, category}` where `description` is human-readable prose like `"Kira has died of old age at 72"`. The relay and frontend can't extract structured data without parsing English.

**Change:** Add optional structured metadata to events. Keep the description for humans, add machine-readable fields for the stream.

```go
type Event struct {
    Tick                uint64            `json:"tick" db:"tick"`
    Description         string            `json:"description" db:"description"`
    NarratedDescription string            `json:"narrated_description,omitempty" db:"narrated"`
    Category            string            `json:"category" db:"category"`
    Meta                map[string]any    `json:"meta,omitempty"`  // NEW: structured data
}
```

`Meta` is a free-form map that carries whatever's relevant. Not persisted to SQLite (events table schema stays the same). Only flows through the SSE stream. Zero performance cost — it's just extra fields on a JSON payload that was already being serialized.

Examples of what Meta carries at each emission site:

| Event | Meta fields |
|-------|-------------|
| Birth | `settlement_id`, `settlement_name`, `parent_ids` |
| Death | `agent_id`, `agent_name`, `settlement_id`, `cause` ("age", "illness", "starvation") |
| Crime (caught) | `agent_id`, `settlement_id`, `crime_type` ("theft") |
| Infrastructure | `settlement_id`, `settlement_name`, `type` ("roads", "walls"), `level` |
| Governance change | `settlement_id`, `governance` ("Council", "Monarchy", etc.) |
| Diaspora | `source_settlement_id`, `target_settlement_name`, `count` |
| Abandonment | `settlement_id`, `settlement_name` |
| Migration | `source_settlement_id`, `target_settlement_id`, `count` |
| Tier 2 decision | `agent_id`, `agent_name`, `occupation`, `action`, `settlement_id` |
| Oracle vision | `agent_id`, `agent_name`, `settlement_id` |
| Faction change | `faction_id`, `faction_name`, `settlement_id` |

This is the *only* worldsim change. Adding `Meta` fields to existing `EmitEvent()` calls. No new endpoints, no new queries, no new goroutines.

**Cost to worldsim:** ~2-3 extra map allocations per event (events fire a few per sim-minute, not per tick). Negligible.

### Layer 2: Relay Enrichment (crossworlds-relay)

The relay transforms from a dumb pipe into a light processing layer. It stays stdlib-only, no database, no persistence beyond memory.

#### 2a. Settlement Activity Tracker

The relay maintains a rolling window of event counts per settlement. Every event with `meta.settlement_id` increments a counter. Every 10 seconds, the relay emits a synthetic `settlement_activity` event:

```json
{
  "event": "activity",
  "data": {
    "tick": 380000,
    "category": "activity",
    "settlements": {
      "42": { "births": 2, "deaths": 0, "trades": 5, "crimes": 1, "migrations": 0 },
      "17": { "births": 0, "deaths": 1, "trades": 12, "crimes": 0, "migrations": 3 }
    },
    "window_seconds": 30
  }
}
```

The frontend uses this to adjust the *energy level* of the canvas — a settlement with high trade volume has dots moving faster, more traveling between zones. A settlement with deaths has dimmer ambient light. This replaces the 30-second pulse poll for activity data.

#### 2b. Settlement Filtering

Currently all events fan out to all clients. With `meta.settlement_id`, the relay can support per-settlement streams:

```
GET /stream?settlement=42
```

The frontend connects to the settlement-specific stream when viewing a live view. This means a viewer watching settlement 42 only gets events relevant to that settlement, plus the periodic activity summary. Reduces noise, improves relevance.

#### 2c. Behavioral Event Synthesis

The relay detects patterns and emits synthetic behavioral hints:

| Pattern | Synthetic Event |
|---------|----------------|
| 3+ trades in 30s for same settlement | `trade_burst` — Market zone gets busy |
| 2+ crimes in 30s | `crime_wave` — dots scatter nervously |
| 5+ births in 30s | `baby_boom` — warm glow at Commons |
| Death of Tier 2 agent | `notable_death` — all dots pause briefly |
| Governance change | `regime_change` — dots converge on Town Hall |

These are higher-level signals the frontend can map directly to animation sequences without parsing individual events.

**Cost to relay:** A few maps in memory, a 10-second ticker goroutine, string matching on event categories. The relay already runs a goroutine per client + hub + upstream. This adds one more. Well within budget for a process that currently uses ~6MB RAM.

### Layer 3: Behavioral Animation (frontend)

#### 3a. Dot Behaviors

Each dot gets a `behavior` state that determines its movement pattern:

```typescript
interface Dot {
    // ... existing fields ...
    behavior: 'idle' | 'working' | 'traveling' | 'gathering' | 'fleeing' | 'celebrating';
    behaviorTimer: number;     // seconds remaining in current behavior
    targetX?: number;          // destination for traveling
    targetY?: number;
    originX?: number;          // return point after traveling
    originY?: number;
}
```

**Movement by behavior:**

| Behavior | Physics | Duration |
|----------|---------|----------|
| `idle` | Current Brownian drift in zone | 2-5s, then transition |
| `working` | Small rhythmic oscillation (±0.005) at fixed point | 3-8s |
| `traveling` | Lerp toward target point, then return | 2-4s |
| `gathering` | Move to commons zone center, cluster tightly | 2-3s |
| `fleeing` | Fast velocity away from a repulsion point | 1-2s |
| `celebrating` | Brief brightness pulse + upward drift | 1-2s |

**Behavior assignment:**

Dots cycle through behaviors on a timer. The *default cycle* for each occupation creates the baseline rhythm:

- **Farmers/Fishers/Hunters/Miners**: 60% working, 20% idle, 15% traveling (fields↔market), 5% gathering
- **Crafters/Laborers**: 50% working, 25% idle, 15% traveling (workshop↔market), 10% gathering
- **Merchants**: 30% traveling (market↔workshop, market↔fields), 30% idle, 20% working, 20% gathering
- **Soldiers/Scholars**: 40% idle (patrol/study), 30% working, 20% gathering, 10% traveling
- **Taverners**: 70% idle, 20% working, 10% gathering

When SSE events arrive, they *override* the timer cycle:

- `trade_burst` → 5 random merchant + producer dots switch to `traveling` between market and their home zone
- `crime_wave` → 3-5 random dots near the crime zone switch to `fleeing`
- `birth` → a dot at commons switches to `celebrating`
- `notable_death` → the named dot (if visible) fades out slowly over 5s, nearby dots pause
- `regime_change` → 10-15 dots converge on Town Hall (`gathering`)

#### 3b. Trail Rendering

When a dot is `traveling`, it leaves a fading trail — a series of points rendered as a thin line with decreasing opacity. Trail fades over 3 seconds.

Over time, the most-traveled routes become visible as persistent ghostly paths. Fields→Market will glow brighter than Workshop→Tavern because more dots travel that route. This is the "pheromone trail" effect from the ant-farm design.

Implementation: each dot has a `trail: {x, y, age}[]` ring buffer (max 20 points). The canvas draws polylines between trail points with opacity `1 - (age / 3.0)`. Trail points older than 3s are discarded.

#### 3c. Event Ripples

Visual effects triggered by SSE events, rendered as expanding rings on the canvas:

```typescript
interface Ripple {
    x: number;           // center (normalized)
    y: number;
    radius: number;      // current radius (expanding)
    maxRadius: number;   // fade-out radius
    color: string;       // category-based
    opacity: number;     // decreasing
    age: number;         // seconds since spawn
}
```

| Category | Ripple | Location |
|----------|--------|----------|
| `economy` / `trade_burst` | Gold ring | Market zone center |
| `birth` / `baby_boom` | Warm white glow | Commons |
| `death` | Dark purple ring | Agent's last zone |
| `crime` / `crime_wave` | Red flash | Random edge of canvas |
| `social` | Soft teal pulse | Commons |
| `political` / `regime_change` | Blue-white ring | Town Hall |
| `disaster` | Orange shockwave | Canvas center |

Ripples expand over 2-3 seconds, fade linearly. Max 5 active ripples (older ones discarded).

#### 3d. Ambient Environment

**Time of day** (from `pulse.simTime`):

The background gradient shifts with sim time. One sim-day = 1440 ticks = ~85 real seconds at 17 ticks/sec. So a full day/night cycle happens roughly every 1.5 minutes of viewing.

| Sim hour | Background | Dot brightness |
|----------|-----------|---------------|
| 0-5 (night) | Deep navy-black | 60% base opacity, mostly `idle` |
| 5-7 (dawn) | Warm amber gradient from bottom | Dots begin `working` |
| 7-17 (day) | Brighter warm brown | Full brightness, active behaviors |
| 17-19 (dusk) | Orange-purple gradient | Dots shift toward `idle` and `gathering` |
| 19-24 (night) | Deep navy-black | Dots dim, mostly `idle` |

**Season** (from pulse data or tick math):

Subtle color tint overlay:
- Spring: faint green cast
- Summer: warm golden
- Autumn: amber-brown
- Winter: cool blue-grey

#### 3e. Zone Architecture

Replace text labels with minimal line-art structures at 15% opacity:

```
Market:     ┌─┐ ┌─┐ ┌─┐    (three stall outlines)
Town Hall:  ╱▔▔▔▔╲           (triangle roof on rectangle)
                ┌────┐
Workshop:   ┌──┐ ▏             (rectangle with chimney line)
            └──┘
Fields:     ─ ─ ─ ─ ─        (dashed horizontal lines)
            ─ ─ ─ ─ ─
Tavern:     ┌──┐ ○            (rectangle with hanging sign)
            └──┘
Commons:    ╭──╮              (well/fountain circle)
            ╰──╯
```

Drawn as canvas line segments, not text. Maybe 8-15 line calls per zone. Gives spatial meaning so the dots moving between recognizable structures reads as purposeful navigation through a settlement.

## Implementation Plan

### Phase 1: Structured Events (worldsim — minimal)

Add `Meta map[string]any` to Event struct. Update ~37 EmitEvent call sites to include relevant metadata. No schema changes, no new endpoints, no performance impact.

**Files:** `internal/engine/simulation.go` (struct), `population.go`, `crime.go`, `governance.go`, `settlement_lifecycle.go`, `cognition.go`, `factions.go`, `relationships.go`, `seasons.go`

**Risk:** Minimal. Meta is `omitempty` — if a field isn't set, nothing changes in the JSON output. Backwards compatible.

### Phase 2: Relay Enrichment (crossworlds-relay) — COMPLETE

Settlement activity tracker, settlement filtering (`?settlement=ID`), and behavioral event synthesis. All in-memory, no external dependencies. Deployed 2026-02-25.

**Files changed:** `enrich.go` (new — activity tracker + pattern detection), `hub.go` (SettlementID on Event, settlementFilter on Client, filtered fan-out + catch-up), `upstream.go` (zero-alloc `extractSettlementID()`), `handler.go` (`?settlement` query param parsing), `main.go` (Enricher wiring)

**Synthetic event types:** `activity` (10s interval, 30s window), `baby_boom` (5+ births/30s), `crime_wave` (2+ crimes/30s), `trade_burst` (3+ economy/30s), `regime_change` (governance changes)

**Not implemented:** `notable_death` — worldsim death events don't include `tier` in meta. Can be added later by enriching death events with tier info in worldsim, or by having the relay track known Tier 2 agent IDs.

### Phase 3: Frontend Animation (crossworlds)

Behavioral states on dots, trail rendering, event ripples, time-of-day cycle, zone architecture. All canvas changes, no new API calls.

**Files:** `lib/particles.ts` (behaviors, trails), `lib/pulse.ts` (ripple types, zone geometry), `components/SettlementCanvas.tsx` (drawing), `app/settlements/[id]/live/page.tsx` (thread events to canvas)

**Risk:** Low. Client-side only. If something breaks, the page reloads.

### Phase 4: Polish

- Tune behavior timings and transition probabilities
- Adjust trail opacity and length
- Calibrate day/night cycle brightness
- Test on mobile (500 dots + trails + ripples performance budget)
- Add keyboard controls (pause, speed, zoom)

## Data Flow After Implementation

```
worldsim emits:
  { tick: 380001, description: "Kira born in Millhaven", category: "birth",
    meta: { settlement_id: 42, settlement_name: "Millhaven" } }

relay receives, tracks activity for settlement 42, forwards to clients.
relay detects 5 births in 30s for settlement 42, emits:
  { category: "baby_boom", meta: { settlement_id: 42 } }

frontend (viewing settlement 42) receives "birth":
  - spawns ripple at Commons (warm white glow)
  - picks random dot at Commons, sets behavior = "celebrating"
  - adds new dot with fade-in at Commons

frontend receives "baby_boom":
  - amplifies: 3 more dots celebrate, ripple is larger
  - Commons zone briefly brightens

viewer sees: a warm pulse of light at the town well, several dots
  brightening and drifting upward, a new dot appearing. They don't need
  to read the ticker to know something good just happened.
```

## What This Doesn't Do

- **No per-agent tracking from worldsim.** We don't stream individual agent actions (eat, work, buy food) — that would be 164K agents × ~17 ticks/sec = millions of events. Behavioral animation is *representative*, not literal. A farmer dot traveling to market represents the aggregate pattern, not a specific agent.
- **No worldsim API polling from the relay.** The relay only consumes the SSE stream. It doesn't call `/api/v1/settlement/:id` or any other endpoint. This keeps the relay simple and the world server untouched.
- **No persistence in the relay.** Activity windows are in memory. Relay restart = clean slate. The ring buffer (100 events) already handles catch-up for brief restarts.
- **No new worldsim endpoints.** Everything flows through the existing `/api/v1/stream` SSE endpoint.

## Appendix: Event Metadata Reference

Complete list of metadata fields to add per emission site:

### population.go
- Death (old age): `{agent_id, agent_name, settlement_id, cause: "age", age}`
- Death (illness): `{agent_id, agent_name, settlement_id, cause: "illness"}`
- Death (starvation): `{agent_id, agent_name, settlement_id, cause: "starvation"}`
- Birth: `{settlement_id, settlement_name, parent_ids}`
- Migration: `{agent_id, agent_name, source_settlement_id, target_settlement_id, target_settlement_name}`
- Tier 2 death: same as death + `{tier: 2, occupation}`
- Tier 2 replenishment: `{agent_id, agent_name, occupation, settlement_id}`

### crime.go
- Caught stealing: `{agent_id, agent_name, settlement_id, crime_type: "theft"}`

### governance.go
- Leader elected/appointed: `{settlement_id, settlement_name, leader_id, leader_name, governance}`
- Revolution: `{settlement_id, settlement_name, old_governance, new_governance}`

### settlement_lifecycle.go
- Diaspora founding: `{source_settlement_id, target_settlement_name, count}`
- Abandonment: `{settlement_id, settlement_name}`
- Treasury redistribution: `{settlement_id, settlement_name, amount}`
- Infrastructure upgrade: `{settlement_id, settlement_name, type, level}`
- Force migration: `{source_settlement_id, target_settlement_id, count}`

### cognition.go (Tier 2)
- Decision: `{agent_id, agent_name, occupation, action, settlement_id}`
- Oracle vision: `{agent_id, agent_name, settlement_id}`

### factions.go
- Faction event: `{faction_id, faction_name, settlement_id}`

### relationships.go
- Marriage/mentorship/rivalry: `{agent_ids, settlement_id, type}`

### seasons.go
- Season change: `{season, year}`
- Disaster: `{settlement_id, settlement_name, type}`
