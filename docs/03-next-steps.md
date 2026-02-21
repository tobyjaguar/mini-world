# Next Steps — Phase 2 and Beyond

## Current State (Phase 1 — Complete)

The world is live. 28,912 agents across 73 settlements on a hex grid continent, running 24/7 on DreamCompute. Agents have needs-driven Tier 0 behavior (eat, work, forage, rest, socialize), Wheeler coherence model souls, and daily auto-save to SQLite.

What works:
- World generation (simplex noise terrain, rivers, coast detection)
- Settlement placement (cities, towns, villages scored by location)
- Agent spawning with demographics, occupations, skills, soul state
- Tick engine with layered schedule (minute/hour/day/week/season)
- Needs decay and satisfaction loop (starvation → death)
- SQLite persistence (save on shutdown, daily auto-save)
- HTTP API (public GET, auth-gated POST)
- Production deployment with systemd, UFW, fail2ban

## Phase 2: Economy & Trade

The economy is the heartbeat. If trade works, everything else follows.

### High Priority
- [ ] **Market per settlement**: Each settlement has a local market with supply/demand price discovery
- [ ] **Agent work produces goods**: Farmers produce food, miners produce ore, etc. — currently work just satisfies a need
- [ ] **Inventory consumption**: Agents consume food from inventory, buy from market when low
- [ ] **Trade routes between settlements**: Merchants travel between settlements, buying low and selling high
- [ ] **Currency system**: Transition from barter to coin-based economy
- [ ] **Crafting chains**: Raw materials → finished goods (ore → tools, grain → bread)

### Economic Balance (from Section 16.6)
- [ ] **Sinks and faucets**: Goods decay, tools break, food spoils — prevents infinite accumulation
- [ ] **ConjugateField for markets**: Implement charging (production) vs discharging (consumption) pressure
- [ ] **Price discovery**: Prices emerge from supply/demand, not set arbitrarily

## Phase 3: Social & Political

### Relationships
- [ ] **Agent-to-agent relationships**: Track affinity between agents (family, friends, rivals)
- [ ] **Family formation**: Marriage, children (population growth beyond initial spawn)
- [ ] **Social needs satisfaction**: Socializing targets specific agents, builds relationships

### Governance & Factions
- [ ] **Settlement governance**: Leaders emerge from high-coherence agents
- [ ] **Factions**: Political/religious/trade groups with shared goals
- [ ] **Tier 1 archetypes**: 4% of agents get archetype-guided behavior (weekly Haiku templates)
- [ ] **Crime and conflict**: Theft, disputes — corruption as privation (absence of governance)

## Phase 4: LLM Integration

### Tier 2 Cognition
- [ ] **Haiku API client**: Rate-limited client for Claude Haiku calls
- [ ] **Tier 2 agent decisions**: Named characters make individual LLM-powered decisions
- [ ] **Agent memory**: Summarized memory of recent events, relationships, goals
- [ ] **Prompt templates**: TOML-based prompt templates for different decision types

### Newspaper / Event Narration
- [ ] **Event detection**: Identify notable events (battles, marriages, market crashes, deaths of important figures)
- [ ] **Newspaper endpoint**: `GET /api/v1/newspaper` — Haiku generates a prose summary of recent events
- [ ] **Event classification**: Categorize events by type and significance

## Phase 5: Polish & Perpetuation

### Anti-Stagnation
- [ ] **Population dynamics**: Birth rate, death rate, migration — population shouldn't flatline
- [ ] **Resource regeneration**: Forests regrow, mines deplete and new ones are discovered
- [ ] **Settlement lifecycle**: New settlements can be founded by migrants; failed ones are abandoned
- [ ] **Seasonal effects**: Weather affects crop yields, travel, mood

### External Entropy
- [ ] **Weather API integration**: Real weather data mapped to in-world weather
- [ ] **random.org integration**: True randomness for critical stochastic events
- [ ] **World events**: Droughts, plagues, discoveries — rare but impactful

### Observability
- [ ] **Richer event log**: More event types, better categorization
- [ ] **Agent history endpoint**: `GET /api/v1/agent/:id/history` — timeline of an agent's life
- [ ] **Settlement history**: Track founding, growth, decline
- [ ] **Snapshot/fork**: Save world state at a point, fork into alternate timelines

### Admin Endpoints
- [ ] `POST /api/v1/intervention` — Inject events (drought, discovery, migration wave)
- [ ] `POST /api/v1/snapshot` — Save named snapshot
- [ ] `POST /api/v1/fork` — Create alternate timeline from snapshot

## Suggested Next Session

**Start with Phase 2 economy** — it has the highest leverage. A good order:
1. Make agent work produce actual goods into inventory
2. Add a market to each settlement with supply/demand pricing
3. Make agents buy food from market when hungry (instead of abstract "eat" action)
4. Add goods decay (food spoils, tools degrade) as the first economic sink
5. Add merchant agents who travel between settlements for inter-settlement trade

This gives the simulation its economic pulse. Once goods flow between agents and settlements, the world starts feeling alive.
