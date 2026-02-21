# Next Steps — Future Development

## Current State (Phases 1–5 + Tuning — Complete)

The world is live and tuned. ~28,900 agents across 73 settlements on a hex grid continent, running 24/7 on DreamCompute.

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

### Web Frontend (High Priority)
The world has a rich API but no visual interface. A web frontend would make Crossroads much more engaging to observe and would give the public GitHub repo a proper face.

- [ ] **Hex map renderer**: Render the hex grid showing terrain, settlements, trade routes, faction influence heatmaps
- [ ] **Settlement view**: Population, market prices, governance, factions, recent events
- [ ] **Agent profiles**: Notable Tier 2 characters with biographies, needs, inventory, relationships
- [ ] **Newspaper page**: Render the weekly Haiku-generated newspaper in a readable format
- [ ] **Stats dashboard**: Time-series charts for population, wealth, mood, Gini coefficient

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

### Remaining Tuning (Monitor)
These may need attention after observing the tuned world for a few sim-days:

- [ ] **Initial mood calibration**: Average mood was slightly negative (-0.07) at launch. The belonging/purpose fixes may resolve this; monitor.
- [ ] **Death rate**: 4.7% mortality in the first 8 days was high, likely from resource scarcity cascading into starvation. The inflation fix should reduce this; monitor.
- [ ] **Fisher skill bug**: `productionAmount()` uses `Skills.Farming` for fishers and `applySkillGrowth()` grows `Skills.Farming` for fishers. Low priority — works but should ideally use a dedicated fishing skill.

## Suggested Next Session

**Build the web frontend.** The API is comprehensive (18 endpoints), the world is tuned and running — the biggest gap is that observing it requires reading JSON. A simple web UI with a hex map, settlement detail pages, agent profiles, and the newspaper would transform the experience.
