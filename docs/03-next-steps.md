# Next Steps — Future Development

## Current State (Phases 1–5 + Tuning — Complete)

The world is live and tuned. ~100,000 agents across 714 settlements on a hex grid continent, running 24/7 on DreamCompute. The API serves at `api.crossworlds.xyz` (Cloudflare proxy) and the Next.js frontend is deployed on Vercel at `crossworlds.xyz`.

### What's working:
- **World**: Simplex noise terrain, rivers, coast detection, hex grid (~2,000 hexes)
- **Agents**: Tier 0 rule-based (95%), Tier 1 archetype-guided (4%), Tier 2 LLM-powered (<1%, ~30 named characters)
- **Economy**: Per-settlement markets with supply/demand pricing, merchant trade routes, crafting (single-recipe demand), goods decay, seasonal price modifiers, economic circuit breaker, tax collection
- **Social**: Agent relationships (family, mentorship, rivalry), crime/theft, faction recruitment
- **Political**: 5 factions with per-settlement influence, 4 governance types, leader succession, revolution mechanics, governance-aligned faction bonuses
- **LLM**: Haiku-powered Tier 2 decisions, weekly archetype updates, newspaper generation, event narration, agent biographies
- **Weather**: Real OpenWeatherMap data mapped to sim modifiers (food decay, travel penalty)
- **Entropy**: random.org true randomness with crypto/rand fallback, weekly random events (disasters, discoveries, breakthroughs)
- **Population**: Births, aging, natural death, migration, anti-collapse safeguards, famine relief
- **Settlement lifecycle**: Overmass diaspora founding, abandonment after 2 weeks of zero population
- **Resources**: Seasonal hex regeneration, mine depletion, discovery events inject hex resources, hunter production scales with combat skill
- **Observability**: 15 public GET endpoints (status, settlements, agents, events, stats, newspaper, factions, economy, social, settlement detail, faction detail, hex detail, stats history, agent biography), 3 admin POST endpoints (speed, snapshot, intervention)
- **Persistence**: SQLite with WAL mode, daily auto-save, stats history time-series with Gini coefficient, faction persistence (treasury, influence, relations survive restart)

### Tuning Fixes Applied

Five issues were diagnosed from observing the live world and fixed:

1. **Fisher mood bug** (critical): Fallback work paths (depleted hex, nil hex) now replenish esteem, safety, and belonging instead of only giving a wage. Fishers on depleted coast hexes no longer have all needs decay to 0.

2. **Raw material inflation** (critical): Crafters now pick the single recipe they're closest to completing and demand only its materials (max 2 goods), instead of demanding all 5 raw materials simultaneously. Hunters now scale fur production with combat skill (`Combat * 2`, min 1) instead of flat 1.

3. **Needs decay spiral** (medium): Work now provides belonging (+0.003) and purpose (+0.002). Wealthy agents (>30 crowns) with low belonging (<0.4) socialize instead of defaulting to more work. Socializing now provides safety (+0.003) and purpose (+0.002). This breaks the cycle where safety was always the priority and agents never socialized.

4. **Faction treasury reset** (medium): Added `factions` table to SQLite. Faction state (treasury, influence, relations, preferences) now persists across restarts via `SaveFactions`/`LoadFactions`, wired into the daily auto-save. Old databases without the table fall back to `InitFactions()`.

5. **Crown faction irrelevant** (low): `factionForAgent` now considers settlement governance type — agents in monarchies with wealth>50 or coherence>0.3 lean Crown; agents in merchant republics with trade>0.1 or wealth>80 lean Merchant's Compact. `updateFactionInfluence` adds governance alignment bonuses: Crown +15 in monarchies, Merchant's Compact +15 in merchant republics, Verdant Circle +10 in councils.

## Future Work

### Web Frontend (Deployed — `crossworlds.xyz`)
Next.js frontend deployed on Vercel. Separate repo: `github.com/tobyjaguar/crossworlds` (private).

- [x] **Hex map renderer**: Canvas-based interactive hex map with pan/zoom/click, terrain colors, settlement markers
- [x] **Settlement views**: List (sortable) and detail (market, agents, factions, culture, events)
- [x] **Agent profiles**: Notable Tier 2 characters with biographies, memories, relationships
- [x] **Newspaper page**: Styled newspaper layout rendering Haiku-generated content
- [x] **Economy overview**: Wealth distribution, inflation/deflation, market health
- [x] **Stats dashboard**: Time-series charts for population, wealth, mood, trade volume
- [x] **API hardening**: Rate limiting on LLM endpoints (story: 10/hr, newspaper: 30/hr), CORS (env-var driven), admin auth on biography refresh
- [ ] **Factions page**: Faction list, detail, influence per settlement (API exists, no UI yet)
- [ ] **Social graph page**: Relationship network visualization (API exists, no UI yet)
- [ ] **Admin control panel**: UI for speed/snapshot/intervention endpoints
- [ ] **Faction influence heatmap**: Overlay faction influence on hex map
- [ ] **Trade route visualization**: Show merchant paths between settlements

### Claude Gardener (Deployed + Upgraded)
Autonomous steward agent that observes world health and nudges conditions to prevent collapse/stagnation. Upgraded from blind observer to effective steward with deterministic triage, cycle memory, 7 action types (event, spawn, wealth, provision, cultivate, consolidate), compound interventions, and crisis-aware reasoning. Cycles every 6 real minutes (~4.25 sim-days). See `docs/05-claude-gardener.md` for full design and `docs/14-gardener-assessment.md` for the diagnosis that prompted the upgrade.

### Deeper Emergence (Medium Priority)
New mechanics that would make the world more interesting.

- [x] **Infrastructure growth**: Settlements invest treasury into roads (pop >= 50) and walls (pop >= 100) weekly, raising overmass capacity. See `docs/05-settlement-fragmentation-fixes.md`.
- [ ] **Infrastructure effects**: Roads should reduce travel time, walls should reduce crime, market level should improve trade efficiency.
- [ ] **Inter-settlement diplomacy**: Formal alliances and trade agreements between settlements, influenced by faction politics. Currently settlements are economically connected but politically isolated.
- [ ] **Agent life events**: Marriage ceremonies, apprenticeships, coming-of-age — notable life milestones that generate events and affect relationships. Currently relationships form but aren't celebrated.
- [ ] **Warfare**: Settlement-vs-settlement conflict driven by faction tensions and resource competition. The Iron Brotherhood exists but has nothing to fight over.
- [ ] **Religion and philosophy**: The Verdant Circle is "religious" but has no doctrine. Emanationist philosophy could manifest as in-world beliefs that affect agent behavior and coherence growth.

### Observability Enhancements
- [ ] **Agent timeline**: `GET /agent/:id/history` — chronological events involving this agent. Currently only current state is visible, not history.
- [ ] **Settlement history**: Track population, treasury, governance changes over time. Stats history exists globally but not per-settlement.
- [ ] **Alternate timelines / fork**: `POST /fork` — snapshot the world and run a divergent copy. Interesting for "what if" experiments.
- [ ] **Event webhook / streaming**: Push notable events to a webhook (Discord, Slack) so the world can announce itself.

### Robustness & Operations
- [ ] **Graceful degradation under memory pressure**: The server has 1GB RAM. As the world grows (births, new settlements, event log), memory usage will climb. Consider capping agent count or archiving dead agents.
- [ ] **Event log rotation**: Events are trimmed to 1,000 in memory but the DB table grows unbounded. Add periodic cleanup or archival.
- [ ] **Metrics endpoint**: Prometheus-compatible `/metrics` for monitoring tick rate, API latency, LLM call success rate, memory usage.
- [ ] **Backup strategy**: Daily DB snapshots to off-server storage. Currently only one copy exists on the DreamCompute instance.

### Tuning — Areas to Watch (from post-fix observations)
The tuning fixes are working (mood +0.64, faction treasuries accumulating), but new dynamics have emerged. See `docs/summaries/2026-02-21-post-tuning.md` for full data.

- [x] **Settlement fragmentation**: 714 settlements with 45% under 25 pop. Fixed: raised founding min to 25, added infrastructure growth, non-viable settlement tracking disables refugee spawning after 4 weeks, enhanced migration absorbs tiny settlements. See `docs/05-settlement-fragmentation-fixes.md`.
- [x] **Unclosed money supply**: Market sells, fallback wages, Tier 2 trade, and merchants all minted crowns from nothing. Fixed: order-matched market engine, treasury-paid merchant/Tier 2 trade, fallback wages removed, remaining mints throttled 60x. See `docs/07-closed-economy-implementation.md` and `docs/08-closed-economy-changelog.md`.
- [x] **Fisher skill bug**: `productionAmount()` now uses `max(Farming, Combat, 0.5) * 5` for fishers. Proper `Skills.Fishing` field still a possible future schema change but current fix is effective.

### Post-Closed-Economy Issues (observed 2026-02-22)

Diagnosed via `/observe` after deploying the closed economy. All P0 issues from the initial closed-economy transition have been resolved through waves 1-9. See the wave descriptions above for details.

**RESOLVED — Trade volume near zero:** Fixed by price ratchet fix (wave 3), food buying action (wave 7).
**RESOLVED — Zero births:** Fixed by belonging restore on failed production, birth threshold 0.4→0.3 (wave 2), sigmoid birth curve (tuning round 11).
**RESOLVED — Grain inflation (431%):** Fixed by price ratchet (wave 3) — grain now within Phi bounds.
**RESOLVED — Merchant death spiral:** Fixed by throttled wage + consignment buying. See `docs/08-closed-economy-changelog.md`.
**RESOLVED — Fisher mood spiral:** Fixed by fisher skill bug fix (tuning round 11), food buying action (wave 7).
**RESOLVED — Producer doom loop:** Fixed in tuning round 12 (wave 9). See below.

### Recently Resolved Issues

**FIXED — NonViableWeeks resets on deploy (234 tiny settlements frozen):**
Persisted both `NonViableWeeks` and `AbandonedWeeks` to `world_meta` as JSON. Restored on startup. Deployed at tick ~222K. Accumulating — expect tiny settlements to start consolidating over 2-4 sim-weeks.

**FIXED — Satisfaction frozen at 0.126 (producer doom loop):**
Avg satisfaction climbed from 0.126 → 0.187 → **0.300** (+138% total) after wave 9 doom loop fix matured. Tier 2 farmer satisfaction improved from -0.45 → -0.10. Stable at 0.300 as of tick 229,763.

**FIXED — Merchant extinction / no Tier 2 replenishment:**
Added `processWeeklyTier2Replenishment()` — promotes up to 2 Tier 0 adults per week to fill Tier 2 vacancies (target: 30). Deployed at tick ~225K.

**FIXED — Birth/trade counter resets on deploy:**
Persisted `births` and `trade_volume` counters to `world_meta`. Restored on startup. Eliminates counter reset noise from deploys.

### Current Issues (observed 2026-02-23, tick 229,763)

**P2 — Farmer satisfaction still negative (-0.10):**
Farmer Tier 2 satisfaction improved from -0.45 to -0.10 but remains the lowest occupation. Structural cause: hex depletion means farmers often can't produce even without punishment. Weekly micro-regen (~4.7%) means fully depleted hexes take ~21 weeks to recover. If satisfaction plateaus, consider faster regen.

**P3 — Faction imbalance (Crown dominates):**
Crown faction influence significantly outweighs other factions in monarchies. Not blocking anything but could be explored for deeper political dynamics. The Merchant's Compact and Iron Brotherhood are relatively weak.

**P3 — 234 tiny settlements awaiting consolidation:**
NonViableWeeks persistence deployed — viability counters are now accumulating. These settlements should start consolidating as the 2-week grace period expires. Monitoring.

**P3 — Fisher skill alias:**
`productionAmount()` still uses `max(Farming, Combat, 0.5) * 5` for fishers instead of a dedicated `Skills.Fishing` field. Works correctly but technically wrong. Low priority schema change.

## Roadmap

### Step 1 (Current): Monitor & Stabilize
All critical fixes deployed. World is healthy: population 100K+, satisfaction 0.300, D:B ratio 0.07, market health 96.8%, Gini 0.575. Monitor tiny settlement consolidation (NonViableWeeks now persistent) and Tier 2 replenishment over the next few sim-weeks.

### Step 2: Factions + Social UI
Add the missing frontend pages for factions (list + detail with influence per settlement) and social graph (relationship network visualization). API endpoints already exist.

### Step 3: Infrastructure Effects
Make walls/roads/markets mechanically meaningful. Roads reduce travel time for merchants, walls reduce crime/theft, market level improves trade efficiency. Currently these exist as numbers but have no gameplay effect.

### Step 4: Deeper Emergence
Inter-settlement diplomacy, warfare, religion/philosophy, agent life events. See "Deeper Emergence" section above.
