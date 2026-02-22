# Next Steps — Future Development

## Current State (Phases 1–5 + Tuning — Complete)

The world is live and tuned. ~50,000 agents across 714 settlements on a hex grid continent, running 24/7 on DreamCompute. The API serves at `api.crossworlds.xyz` (Cloudflare proxy) and the Next.js frontend is deployed on Vercel at `crossworlds.xyz`.

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

### Claude Gardener (Future)
Autonomous steward agent that observes world health and nudges conditions to prevent collapse/stagnation. See `docs/05-claude-gardener.md` for full design.

### Deeper Emergence (Medium Priority)
New mechanics that would make the world more interesting.

- [ ] **Infrastructure effects**: Wall/road/market levels exist but are cosmetic. Roads should reduce travel time, walls should reduce crime, market level should improve trade efficiency.
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
The tuning fixes are working (mood +0.64, births outpacing deaths, faction treasuries accumulating), but new dynamics have emerged. See `docs/summaries/2026-02-21-post-tuning.md` for full data.

- [ ] **Settlement explosion**: 73 → 468 settlements in ~20 sim-days. Overmass diaspora threshold may be too aggressive, fragmenting population into tiny settlements that lack critical mass for healthy markets. Consider raising the overmass threshold or adding a minimum founding population requirement.
- [ ] **Persistent raw material inflation**: Furs and iron ore still at 4.2x ceiling in some settlements. Single-recipe demand helped but didn't eliminate it. May need: increased hex resource regen rate, more hunter/miner occupations in spawner, or higher base supply floors in markets.
- [ ] **Clothing oversupply**: Clothing stuck at 0.24x floor everywhere. No demand driver for clothing. Consider: agents needing clothing (weather/cold), clothing decay, or rebalancing recipe selection weights.
- [ ] **Population growth rate**: 5,652 births vs 1,874 deaths — healthy but may accelerate unsustainably across 468 settlements. Monitor memory usage on the 1GB server.
- [ ] **Market health still low (0.35)**: More than half of goods significantly mispriced. May need more sim-time for price discovery, or supply/demand mechanics need further tuning.
- [ ] **Fisher skill bug**: `productionAmount()` uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Low priority — works but technically wrong.

## Suggested Next Session

**Verify the frontend end-to-end** now that DNS propagates. Then focus on either:
1. **Claude Gardener** — autonomous steward to keep the world healthy as it scales
2. **Factions/Social UI** — add the missing frontend pages for political and social systems
3. **Infrastructure effects** — make walls/roads/markets mechanically meaningful
4. **World tuning** — run `/observe` to diagnose current world health and address imbalances
