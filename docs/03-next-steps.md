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
- [ ] **Fisher skill bug**: `productionAmount()` uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Low priority — works but technically wrong.

### Post-Closed-Economy Issues (observed 2026-02-22)

Diagnosed via `/observe` after deploying the closed economy. Two P0 issues need immediate attention:

**P0 — Trade volume near zero (104 trades for 51K agents):**
The order-matched market engine may be too restrictive. Sell orders use `price * Matter` (~0.38x) as min ask, buy orders use `price * Being` (~1.618x) as max bid. These bands overlap in theory, but if most goods are stuck at the Agnosis price floor (0.236x base), sell asks may be too high relative to what buyers can afford. Symptoms: most goods at price floor, grain at 431% inflation (universal demand, insufficient supply reaching market). Likely cause: the price bands work relative to `entry.Price`, but when prices are already at floor, the sell ask (`floor * Matter`) produces sub-1-crown prices that round to 1, while buyers' max (`floor * Being`) is also very low. Agents with >10K wealth should be able to buy, so the issue may be that sellers aren't listing (surplus threshold too high?) or clearing prices aren't updating correctly.

**P0 — Zero births:**
Stats show 2,073 deaths and 0 births. Population declining toward extinction. The closed economy removed fallback wages — if births require minimum agent wealth, agents on depleted hexes who now get nothing may have dropped below the birth threshold. Alternatively, the birth system may have a separate issue (tick gating, population cap, or a bug introduced by the `ApplyAction` signature change).

**P1 — Grain inflation (431%):**
Grain is the universal food need. At 8.6x base price, most agents can't afford to buy grain even when farmers produce it. This may be a symptom of the trade volume issue (farmers have grain surplus but sell orders aren't matching) or a separate supply problem.

**P1 — Merchant death spiral:**
All 6 dead Tier 2 agents are merchants, all at 0 wealth. Merchants need wealth to buy cargo at home market, but the closed economy means they must spend crowns to buy. If they ran out of capital, they can't trade, can't earn, and starve. May need: a minimum-wealth safety net for merchants, or allowing merchants to take settlement goods on consignment.

**P2 — Fisher mood (-0.30 for all Tier 2 fishers):**
Systematic negative mood across all Tier 2 fishers. May be related to the fisher skill bug (`Skills.Farming` instead of fishing), fish price deflation (fish at floor price = low trade income), or a needs decay issue from the production.go changes.

**P2 — 180+ settlements below viability threshold (pop < 25):**
A third of settlements are sub-viable. The absorption migration (mood threshold 0.0 for pop < 25) should be consolidating these, but with 714 settlements and only 51K agents (avg 71/settlement), fragmentation is severe. Monitor whether consolidation is happening or if these settlements persist indefinitely.

## Roadmap

### Step 1 (URGENT): Fix Post-Closed-Economy P0s
Diagnose and fix zero births and near-zero trade volume. These will collapse the world if left unaddressed. See "Post-Closed-Economy Issues" above.

### Step 2: Factions + Social UI
Add the missing frontend pages for factions (list + detail with influence per settlement) and social graph (relationship network visualization). API endpoints already exist.

### Step 3: Infrastructure Effects
Make walls/roads/markets mechanically meaningful. Roads reduce travel time for merchants, walls reduce crime/theft, market level improves trade efficiency. Currently these exist as numbers but have no gameplay effect.

### Step 4: Deeper Emergence
Inter-settlement diplomacy, warfare, religion/philosophy, agent life events. See "Deeper Emergence" section above.
