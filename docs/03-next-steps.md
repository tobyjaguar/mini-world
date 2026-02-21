# Next Steps — Future Development

## Current State (Phases 1–5 — Complete)

The world is live and feature-complete through the first pass. 27,500+ agents across 73 settlements on a hex grid continent, running 24/7 on DreamCompute.

### What's working:
- **World**: Simplex noise terrain, rivers, coast detection, hex grid (~2,000 hexes)
- **Agents**: Tier 0 rule-based (95%), Tier 1 archetype-guided (4%), Tier 2 LLM-powered (<1%, ~30 named characters)
- **Economy**: Per-settlement markets with supply/demand pricing, merchant trade routes, crafting, goods decay, seasonal price modifiers, economic circuit breaker, tax collection
- **Social**: Agent relationships (family, mentorship, rivalry), crime/theft, faction recruitment
- **Political**: 5 factions with per-settlement influence, 4 governance types, leader succession, revolution mechanics
- **LLM**: Haiku-powered Tier 2 decisions, weekly archetype updates, newspaper generation, event narration, agent biographies
- **Weather**: Real OpenWeatherMap data mapped to sim modifiers (food decay, travel penalty)
- **Entropy**: random.org true randomness with crypto/rand fallback, weekly random events (disasters, discoveries, breakthroughs)
- **Population**: Births, aging, natural death, migration, anti-collapse safeguards, famine relief
- **Settlement lifecycle**: Overmass diaspora founding, abandonment after 2 weeks of zero population
- **Resources**: Seasonal hex regeneration, mine depletion, discovery events inject hex resources
- **Observability**: 15 public GET endpoints (status, settlements, agents, events, stats, newspaper, factions, economy, social, settlement detail, faction detail, hex detail, stats history, agent biography), 3 admin POST endpoints (speed, snapshot, intervention)
- **Persistence**: SQLite with WAL mode, daily auto-save, stats history time-series with Gini coefficient

## Observations from the Live World

After running for ~8 sim-days (Year 1, Spring Day 8):

1. **Raw material scarcity is the defining economic problem.** Timber, iron ore, gems, and furs are 4.2x inflated across most settlements. Crafted goods (clothing, tools, luxuries) are deflated to 0.24x. The production→consumption chain may be unbalanced — too many consumers, not enough resource producers, or resource extraction rate is too low relative to demand.

2. **Fishers are universally miserable** (mood ~-0.63) while crafters hover around +0.35. Something in the fisher occupation loop is generating sustained negative mood. Worth investigating.

3. **All faction treasuries are 0.** Dues collection may not be generating enough, or there's a sink draining them immediately.

4. **The Merchant's Compact dominates** (present in 66/73 settlements) while The Crown is nearly irrelevant. Faction influence balance may need tuning.

5. **Average mood is slightly negative (-0.07)** — the world starts gloomy. This could be fine (the world is young, material scarcity is real) or could indicate systematic mood depression.

6. **1,353 deaths in 8 days** out of ~29,000 — roughly 4.7% mortality. This is high for early days before old age kicks in. Likely starvation deaths from the resource scarcity.

## Future Work

### Tuning & Balance (High Priority)
These don't require new systems — they're adjustments to existing mechanics that would improve emergent behavior.

- [ ] **Resource production rate audit**: Check if farmers/miners/fishers produce enough goods per tick relative to consumption. The 4.2x raw material inflation suggests production is too low or demand scaling is off.
- [ ] **Fisher mood investigation**: Why are all fishers deeply unhappy? Is it the occupation loop, the settlement they live in, or a needs satisfaction bug?
- [ ] **Faction treasury flow**: Trace where faction dues go. Are factions collecting and spending, or is the flow broken?
- [ ] **Faction influence rebalancing**: The Crown should be stronger in monarchies, not uniformly weak. Governance type should amplify matching faction influence.
- [ ] **Initial mood calibration**: Consider starting agents with slightly higher mood or faster mood recovery to avoid universal early gloom.
- [ ] **Death rate check**: 4.7% in 8 days is steep. Verify starvation deaths aren't cascading from the resource scarcity.

### Deeper Emergence (Medium Priority)
New mechanics that would make the world more interesting.

- [ ] **Crafting recipes**: Explicit recipe system (iron + timber → tools, herbs + knowledge → medicine) rather than implicit occupation-based production. Would create real supply chain dependencies.
- [ ] **Infrastructure effects**: Wall/road/market levels exist but are cosmetic. Roads should reduce travel time, walls should reduce crime, market level should improve trade efficiency.
- [ ] **Inter-settlement diplomacy**: Formal alliances and trade agreements between settlements, influenced by faction politics. Currently settlements are economically connected but politically isolated.
- [ ] **Agent life events**: Marriage ceremonies, apprenticeships, coming-of-age — notable life milestones that generate events and affect relationships. Currently relationships form but aren't celebrated.
- [ ] **Warfare**: Settlement-vs-settlement conflict driven by faction tensions and resource competition. The Iron Brotherhood exists but has nothing to fight over.
- [ ] **Religion and philosophy**: The Verdant Circle is "religious" but has no doctrine. Emanationist philosophy could manifest as in-world beliefs that affect agent behavior and coherence growth.

### Observability & Interface (Lower Priority)
Making the world easier to watch and understand.

- [ ] **Web frontend**: A simple page that renders the hex map, settlement details, agent profiles, and the newspaper. Currently API-only (JSON responses).
- [ ] **Agent timeline**: `GET /agent/:id/history` — chronological events involving this agent. Currently only current state is visible, not history.
- [ ] **Settlement history**: Track population, treasury, governance changes over time. Stats history exists globally but not per-settlement.
- [ ] **Alternate timelines / fork**: `POST /fork` — snapshot the world and run a divergent copy. Interesting for "what if" experiments.
- [ ] **Event webhook / streaming**: Push notable events to a webhook (Discord, Slack) so the world can announce itself.
- [ ] **Map visualization**: Render the hex grid as an image or SVG, showing terrain, settlements, trade routes, faction influence heatmaps.

### Robustness & Operations
- [ ] **Graceful degradation under memory pressure**: The server has 1GB RAM. As the world grows (births, new settlements, event log), memory usage will climb. Consider capping agent count or archiving dead agents.
- [ ] **Event log rotation**: Events are trimmed to 1,000 in memory but the DB table grows unbounded. Add periodic cleanup or archival.
- [ ] **Metrics endpoint**: Prometheus-compatible `/metrics` for monitoring tick rate, API latency, LLM call success rate, memory usage.
- [ ] **Backup strategy**: Daily DB snapshots to off-server storage. Currently only one copy exists on the DreamCompute instance.

## Suggested Next Session

**Start with tuning.** The systems are all in place, but the live world is showing imbalances (resource scarcity, fisher misery, dead faction treasuries). A tuning pass would:

1. Audit resource production rates vs consumption — fix the 4.2x inflation
2. Investigate fisher mood bug
3. Fix faction treasury flow
4. Tie governance type to faction influence bonuses

After tuning, the best next feature is probably a **web frontend** — it would make checking in on Crossroads much more engaging than reading JSON, and would give the project a public face for the GitHub repo.
