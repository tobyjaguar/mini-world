# 11 — Ant-Farm: Settlement Activity Visualization

## Overview

A new frontend page (`/settlement/:id/live` or a tab within the settlement detail page) that renders a **living, animated "public square"** for a single settlement. Think ant farm through glass — you see agents going about their day, clustering, trading, socializing, eating, working — without intruding on their inner lives.

The visualization conveys **behavioral texture**: what does daily life feel like in this settlement? Is it bustling or sleepy? Are people congregating or isolated? Is the mood bright or heavy?

## Design Principles

1. **World server is sacred.** The world must not be slowed down to power a UI feature. All heavy lifting happens on the frontend (Vercel) or in the browser.
2. **Public square, not surveillance.** Show what an observer standing in the town square would see — not private thoughts, memories, or relationship internals. Agents are in public space by choice.
3. **Emergent, not scripted.** The visualization should reflect real simulation data, not pre-baked animations. If the settlement is starving, it should look different from a prosperous one.
4. **Ambient, not demanding.** This is a fish tank you glance at, not a dashboard you study. Motion and mood over numbers and labels.

---

## Architecture: Load Budget

### The Problem
The world server is a 1GB RAM Debian box running a continuous simulation tick every second. It serves a REST API with no streaming. Every API call competes with simulation work for CPU and memory.

### The Solution: Frontend Caching Proxy

```
Browser  →  Vercel (Next.js API routes)  →  World Server
              ↑                               ↑
         Cache + enrich               Minimal polling
         Animate in browser           Only structured data
```

**Vercel API route** (`/api/settlement/[id]/pulse`):
- Polls the world server on a **30-second interval** (not per-user — one server-side fetch, cached)
- Aggregates data from 2-3 world API calls into a single enriched payload
- Caches the result for 30 seconds (ISR / in-memory cache)
- Serves all connected browsers from cache — 1,000 users = still 1 world API call per 30s

**Browser**:
- Fetches the Vercel pulse endpoint every 30 seconds
- All animation, interpolation, and visual simulation runs client-side
- Agents move smoothly between state snapshots (lerp positions over 30s)
- No direct calls to the world server from the browser for this feature

### World Server Load
- **2-3 additional GET requests per 30 seconds** (settlement detail + events + optionally agents)
- These endpoints already exist and are lightweight
- This is negligible compared to the existing frontend polling

---

## Data Model: The Pulse

The Vercel caching proxy assembles a **Pulse** from existing world API endpoints:

### Source Endpoints (called by Vercel, not browser)

1. `GET /api/v1/settlement/:id` → population, governance, treasury, market prices, top agents, faction presence, recent events
2. `GET /api/v1/events?limit=20` → recent world events (filter client-side to this settlement)
3. `GET /api/v1/status` → world clock, season, weather

### Assembled Pulse Payload

```typescript
interface SettlementPulse {
  // Identity
  settlement_id: number;
  name: string;
  population: number;
  governance: string;
  season: string;
  sim_time: string;
  weather: string;

  // Mood of the square (aggregate, not individual)
  avg_mood: number;          // from settlement health
  treasury: number;
  market_health: number;

  // Occupational breakdown (for crowd composition)
  occupations: Record<string, number>;  // e.g. { "Farmer": 34, "Merchant": 8, ... }

  // Faction presence (for visual groupings)
  factions: Record<string, number>;     // e.g. { "Iron Covenant": 12, ... }

  // Recent public events (the town crier)
  events: Array<{
    tick: number;
    description: string;
    category: string;       // "economy", "social", "birth", "death", "political", "crime"
  }>;

  // Top visible agents (named characters in the square)
  notable_agents: Array<{
    id: number;
    name: string;
    occupation: string;
    tier: number;
    mood: number;           // effective mood (public demeanor)
    role: string;           // leader, merchant, soldier, scholar, etc.
  }>;

  // Economic pulse (market stall activity)
  recent_trades: number;     // trade volume this period
  most_traded_good: string;  // what's hot at the market
  price_trend: "rising" | "stable" | "falling";  // overall direction
}
```

### What's NOT in the Pulse (privacy boundary)
- Individual agent memories
- Relationship details (sentiment, trust scores)
- Soul coherence / spiritual state
- Inventory contents
- Wealth of individuals (only treasury and aggregate)
- Tier 2 LLM decision reasoning

---

## Backend Changes Required

### Minimal — Enrich Settlement Detail Endpoint

The existing `GET /api/v1/settlement/:id` response should be extended with a few fields that are already computed but not exposed:

```go
// In the settlement detail API response, ADD:
type SettlementDetailResponse struct {
    // ... existing fields ...

    // NEW: Occupational breakdown of living agents in settlement
    Occupations map[string]int `json:"occupations"`

    // NEW: Recent trade volume for this settlement (from market resolution)
    RecentTradeVolume int `json:"recent_trade_volume"`

    // NEW: Most traded good (by volume) in last market resolution
    MostTradedGood string `json:"most_traded_good,omitempty"`

    // NEW: Average effective mood of agents in settlement
    AvgMood float64 `json:"avg_mood"`
}
```

**Implementation notes:**
- `Occupations`: Loop over `sim.SettlementAgents[settID]`, count by `agent.Occupation`. O(n) where n = settlement population. Cheap.
- `RecentTradeVolume`: Already tracked per market resolution. Store `lastTradeVolume` on Market struct, expose it.
- `MostTradedGood`: Track during `resolveSettlementMarket()`. Store alongside volume.
- `AvgMood`: Loop over settlement agents, average `Wellbeing.EffectiveMood`. O(n), cheap.

**These are all read-only aggregations of data that already exists.** No new simulation logic. No new storage. No new LLM calls.

### Optional — Settlement Events Filter

Currently `/api/v1/events` returns global events. Adding a `?settlement=ID` query filter would avoid transferring irrelevant events to the frontend:

```go
// In api/server.go, events handler:
if settID := r.URL.Query().Get("settlement"); settID != "" {
    // Filter events whose Description mentions the settlement name
    // (events are already text — simple string match)
}
```

This is a nice-to-have. The frontend can filter client-side if this isn't implemented.

### Optional — Agent Occupation in Agent List

The existing `GET /api/v1/agents` and settlement top_agents don't include occupation as a string. Add `occupation_name` to the response for named agents. (Currently only the raw enum index is in AgentDetail.)

---

## Frontend Implementation

### Page Structure

New route: `/settlements/[id]/live` (or a "Live" tab on the existing settlement detail page).

```
┌──────────────────────────────────────────┐
│  Settlement Name           Season  Clock │  ← thin header
├──────────────────────────────────────────┤
│                                          │
│            ┌─────────┐                   │
│    ○  ○    │ MARKET  │   ○              │
│   ○ ○  ○  │  STALL  │  ○ ○ ○           │
│            └─────────┘     ○             │
│                                          │
│  ○ ○                         ┌────────┐ │
│   ○    ○  ○                  │ TOWN   │ │
│                ○  ○   ○      │ HALL   │ │
│     ○                        └────────┘ │
│         ○  ○    ○                        │
│                       ○  ○              │
│  ○        ○                    ○        │
│                                          │
├──────────────────────────────────────────┤
│  📜 Town Crier: "A merchant arrived..." │  ← scrolling event ticker
└──────────────────────────────────────────┘
```

### Canvas Visualization (HTML Canvas or SVG)

The main area is a **2D canvas** rendering an abstract public square. Not a literal map — a **behavioral space**.

#### Zones
The square has implicit zones that agents gravitate toward based on current activity:

| Zone | Location | Who gathers here |
|------|----------|-------------------|
| **Market** | Center-left | Merchants, traders, anyone buying/selling |
| **Town Hall** | Right | Leader, soldiers, political agents, governance events |
| **Well / Commons** | Center | Socializing agents, idle agents, new arrivals |
| **Workshop Row** | Top | Crafters, miners, laborers (working agents) |
| **Fields Edge** | Bottom-left | Farmers, fishers, hunters (just returned or heading out) |
| **Tavern** | Bottom-right | Resting agents, high-belonging agents |

Zones are **soft attractors**, not hard boundaries. Agents drift between them based on the pulse data.

#### Agent Dots

Each agent is a **small colored dot** (4-8px radius) with subtle ambient motion:

- **Color**: Derived from occupation
  - Farmer: green
  - Merchant: gold
  - Soldier: red
  - Scholar: blue
  - Crafter: orange
  - Miner: grey
  - Fisher: teal
  - Hunter: brown
  - Alchemist: purple
  - Laborer: tan

- **Size**: Base 4px. Tier 1 agents: 6px. Tier 2 (named): 8px with a subtle glow.

- **Opacity**: Maps to mood. Happy agents are fully opaque. Low-mood agents are semi-transparent (fading into the background, withdrawn).

- **Motion**: Brownian drift within their zone attractor. Speed varies:
  - Working agents: purposeful movement toward their zone
  - Socializing: gentle clustering (2-3 dots drift together, linger, drift apart)
  - Idle: slow wandering
  - Eating: stationary for a few seconds

- **Named agents** (Tier 2): Show a tiny name label on hover. Clicking navigates to their agent detail page.

#### Crowd Dynamics (Client-Side Simulation)

The browser maintains a **soft particle simulation** of `N` dots where `N = population`. Each dot has:

```typescript
interface Dot {
  x: number;
  y: number;
  vx: number;
  vy: number;
  occupation: string;
  zone: Zone;          // current attractor
  mood: number;
  tier: number;
  agentId?: number;    // only for notable agents
  name?: string;       // only for notable agents
}
```

**On each animation frame** (~60fps, all client-side):
1. Apply zone attractor force: `F = k * (zoneCenter - position)`, weak spring
2. Apply separation force: dots repel within 10px (avoid overlap)
3. Apply clustering force: same-zone dots attract gently within 30px
4. Add Brownian noise: `v += random(-0.3, 0.3)` per frame
5. Damping: `v *= 0.95`
6. Clamp to canvas bounds

**On pulse update** (every 30s):
1. Reassign dots to zones based on occupational mix and events
2. If population changed: add/remove dots with fade in/out
3. If a significant event occurred: trigger a visual ripple (see below)

#### Event Ripples

When the pulse includes a new event, trigger a visual effect in the relevant zone:

| Category | Zone | Visual Effect |
|----------|------|---------------|
| `economy` / trade | Market | Gold shimmer, dots cluster briefly |
| `social` | Commons | Warm pulse, dots draw together |
| `birth` | Commons | New dot fades in with a soft glow |
| `death` | — | A dot fades out slowly over 5 seconds |
| `political` | Town Hall | Dots converge on hall, brief red/blue flash |
| `crime` | Random | A dot darts across the square, brief red flash |
| `disaster` | Everywhere | All dots scatter outward, then regroup |

#### Ambient Environmental Layer

Behind the dots, render a subtle **ambient background** that reflects settlement state:

- **Season**: Background tint (spring green warmth, summer golden, autumn amber, winter blue-grey)
- **Weather**: Particle effects — rain drops, snow flakes, sun rays (very subtle, CSS/canvas)
- **Time of day** (derived from sim_time): Dawn/dusk gradient shifts. Night = darker, fewer dots visible (agents sleeping). Midday = brightest, most active.
- **Prosperity**: Background warmth/saturation scales with treasury and market health. A poor settlement looks muted. A rich one glows.

### Event Ticker (Town Crier)

A horizontal scrolling bar at the bottom showing recent events as they arrive:

```
📜 "Kael the merchant arrived from Thornhaven with iron ore"  •  "A child was born to the Ashford family"  •  "Council voted to raise taxes"
```

- New events slide in from the right
- Events fade out after 60 seconds
- Clicking an event that mentions a named agent navigates to their page
- Category icon prefix: 💰 economy, 👶 birth, ⚔️ crime, 🏛️ political, 💀 death, 🤝 social

### Header Bar

Minimal info strip at top:

```
Thornhaven  •  Pop: 234  •  Council Republic  •  ☀️ Summer  •  Mood: 72%
```

On mobile this wraps to two lines (already handled by the responsive refactor pattern).

### Interaction

- **Hover on a dot**: If Tier 1/2, show name tooltip. If Tier 0, show occupation.
- **Click a named dot**: Navigate to `/agents/:id`
- **Click Market zone**: Navigate to settlement economy/market detail
- **Click Town Hall zone**: Navigate to settlement governance detail
- **Keyboard**: `Space` to pause/resume animation. `+`/`-` to zoom.

---

## Performance Budget

### Browser
- Canvas renders up to **500 dots** at 60fps on mobile (tested heuristic for 2D particle systems)
- Settlements above 500 pop: render 500 representative dots (sampled proportionally by occupation)
- RequestAnimationFrame with delta time — no fixed-step assumptions
- Pause animation when tab is hidden (`document.hidden`)

### Vercel
- One API route (`/api/settlement/[id]/pulse`) with 30s ISR cache
- Calls 2-3 world API endpoints per cache miss
- Response size: ~2-5KB JSON per pulse
- No server-side rendering of the canvas — pure client component

### World Server
- **Net new load: 2-3 GET requests per 30 seconds per actively-viewed settlement**
- These are lightweight read-only endpoints that already exist
- With Vercel caching, 1,000 simultaneous viewers of the same settlement = still 2-3 requests/30s
- If 10 different settlements are being viewed simultaneously: 20-30 requests/30s = ~1 req/sec additional. Negligible.

---

## Implementation Plan

### Phase 1: Backend (mini-world) — COMPLETE
1. ✅ Add `occupations` map to settlement detail response
2. ✅ Add `avg_mood`, `avg_satisfaction`, `avg_alignment` to settlement detail response
3. ✅ Add `recent_trade_volume` and `most_traded_good` to settlement detail response
4. ✅ Add `?settlement=NAME` filter to events endpoint

**~53 lines of Go. No simulation logic changes. No new storage.**

Changes:
- `internal/economy/goods.go` — `TradeCount` and `MostTradedGood` fields on `Market` struct
- `internal/engine/market.go` — Accumulate trade stats in `resolveSettlementMarket`
- `internal/api/server.go` — Settlement detail enrichment (single-pass occupation/wellbeing aggregation) + events settlement filter

### Phase 2: Frontend Caching Proxy (crossworlds)
1. Create Next.js API route `/api/settlement/[id]/pulse`
2. Fetch from world API, assemble Pulse, cache 30s
3. Return enriched JSON

### Phase 3: Frontend Visualization (crossworlds)
1. Create `/settlements/[id]/live` page (or tab)
2. Implement canvas with zone layout
3. Implement dot particle system (positions, forces, rendering)
4. Wire pulse data → dot state transitions
5. Implement event ticker
6. Implement ambient background layer

### Phase 4: Polish
1. Event ripple effects
2. Hover/click interactions on dots
3. Mobile responsive layout (canvas scales, ticker wraps)
4. Performance profiling on low-end devices
5. Pause when tab hidden
6. Add link from settlement detail page and sidebar

---

## Open Questions

1. **Tab vs. separate page?** Adding a "Live" tab to the existing settlement detail page keeps navigation simple. A separate `/live` route allows fullscreen immersion. Preference?

2. **Sound?** Ambient audio (market chatter, birds, rain) could add a lot to the fish-tank feel. But it's a separate effort and some users hate auto-playing sound. Consider as a Phase 5 toggle.

3. **Multiple settlements?** A "world overview" mode showing all settlements as mini fish tanks in a grid could be compelling but is a larger scope. Park for later.

4. **Agent names for Tier 0?** Currently Tier 0 agents are anonymous in the API. We could generate procedural names client-side (no backend cost) to make the square feel more alive. Or leave them as anonymous dots — the unnamed masses.

---

## File Inventory

### mini-world (backend) changes — DONE
- `internal/economy/goods.go` — `TradeCount`, `MostTradedGood` on Market struct
- `internal/engine/market.go` — Accumulate trade stats in `resolveSettlementMarket`
- `internal/api/server.go` — Settlement detail: `occupations`, `avg_mood`, `avg_satisfaction`, `avg_alignment`, `recent_trade_volume`, `most_traded_good`. Events: `?settlement=NAME` filter.

### crossworlds (frontend) new/changed files
- `src/app/api/settlement/[id]/pulse/route.ts` — Vercel caching proxy
- `src/app/settlements/[id]/live/page.tsx` — Main page component
- `src/components/SettlementCanvas.tsx` — Canvas rendering + particle system
- `src/components/EventTicker.tsx` — Scrolling event bar
- `src/lib/pulse.ts` — Pulse type definitions and fetch logic
- `src/lib/particles.ts` — Dot physics simulation (zone attractors, separation, Brownian motion)
