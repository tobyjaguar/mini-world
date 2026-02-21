# WORLDSIM: A Persistent Agentic World Simulation

## Design Specification for Claude Code Implementation

**Version:** 2.0 — February 2026 (with Wheeler Emanationist Cosmology integration)
**Target Stack:** Rust (core simulation engine), with Anthropic Haiku API for high-level agent cognition, external APIs for entropy and weather data
**Deployment:** Single cloud server (scalable architecture for future horizontal scaling)

---

## 1. Vision and Goals

Build a **continuously running, persistent simulated world** populated by autonomous agents who form economies, relationships, factions, and history. The simulation ticks forward in real time (or accelerated time), and an external observer can query the world via API to receive a "newspaper" of recent events, economic reports, and emergent narratives.

### Core Design Principles

1. **Emergence over scripting.** The world should produce surprising events from simple, well-designed rules interacting. We are not authoring a story — we are building a petri dish.
2. **Economic robustness as the backbone.** Drawing from Castronova's research on synthetic world economies and the SEAS framework's modeling of real economies, the economy is the heartbeat. If the economy works, agents have reasons to act, trade, cooperate, and conflict.
3. **Hybrid cognition.** Most agent decisions use fast, deterministic rule-based systems (written in Rust). Major decisions — leadership changes, diplomatic negotiations, responses to crises, creative acts — invoke Haiku via API for richer, less predictable behavior.
4. **Observability is a first-class feature.** The world exists to be checked in on. Every tick produces observable state changes. A rich event/news system makes the world legible and fun to follow.
5. **Perpetuation by design.** The world must resist collapse. Economic sinks and faucets are carefully balanced. Population dynamics include birth, death, migration. Catastrophes are recoverable.

---

## 2. Research Foundations

This design synthesizes several lineages of work:

### 2.1 Sentient World Simulation / SEAS (Purdue, JFCOM)
The 2006 SWS concept paper describes a "synthetic mirror of the real world" with continuously calibrated agents operating across Political, Military, Economic, Social, Informational, and Infrastructure (PMESII) dimensions. Key architectural ideas we borrow:
- **Fractal representation**: The same agent who experiences fine-grained local events is also influenced by coarse-grained global models. Our world uses a similar multi-granularity approach.
- **Society of Models**: Heterogeneous models operating at different temporal and spatial scales within the same environment. We implement this as a layered tick system.
- **Excursion management**: The ability to fork the world state and run "what if" scenarios. We design for snapshot/fork capability from the start.
- **Continuous knowledge integration**: SWS ingested real-world data streams. We use weather APIs, random.org, and optionally RSS feeds as external entropy sources.

### 2.2 Stanford Generative Agents (Park et al., 2023)
The landmark paper demonstrated that LLM-powered agents with memory streams, reflection, and planning produce believable emergent social behavior (a town of 25 agents autonomously organized a Valentine's Day party). Key ideas we adopt:
- **Memory stream architecture**: Each agent maintains a timestamped log of observations, which are periodically synthesized into higher-level reflections.
- **Retrieval-based cognition**: When making decisions, agents retrieve relevant memories by recency, importance, and relevance.
- **Observation → Planning → Reflection loop**: The cognitive cycle that makes agents feel alive.

However, we depart from the Stanford approach in a critical way: calling an LLM for every agent decision is computationally prohibitive at scale. Instead, we use a **tiered cognition model** (see Section 4).

### 2.3 Synthetic World Economies (Castronova, Lehdonvirta)
Edward Castronova's foundational research established that virtual world economies follow real economic principles — supply and demand, scarcity, rational choice, and market equilibrium all emerge naturally when the rules are right. Key design principles from this work:
- **Scarcity is fundamental.** Resources must be finite and depletable. Without scarcity, there is no economy.
- **Sinks and faucets must balance.** Currency enters the world (faucets: harvesting, mining, trade with NPCs) and leaves it (sinks: consumption, taxes, decay, crafting costs). If faucets exceed sinks, you get hyperinflation. If sinks exceed faucets, you get deflation and stagnation.
- **The economy regulates power.** In synthetic worlds, economic participation is how agents grow in capability and influence.
- **Multiple interlinked markets** create realistic complexity. Raw materials, finished goods, labor, land, and services should all be tradeable.

### 2.4 MIT AgentTorch / LLM Archetypes (Chopra et al., 2025)
Recent AAMAS 2025 work showed that "LLM archetypes" — where representative agent types share LLM-generated behavioral templates — allow scaling from hundreds to millions of agents while preserving adaptive behavior. We adopt a similar approach: agents are grouped into behavioral archetypes, and Haiku generates behavior templates for each archetype rather than for each individual agent.

### 2.5 krABMaga / Rust ABM (Antelmi et al., 2019)
Research evaluating Rust for massive agent-based simulations confirmed that Rust's performance approaches C while providing memory safety and fearless concurrency. The krABMaga framework demonstrated ECS-like architectures for ABM in Rust. We use a similar entity-component approach.

### 2.6 Wheeler Emanationist Cosmology (Wheeler, 2021–2026)
Ken Wheeler's corpus of 11 documents on emanationist metaphysics provides the **philosophical and mathematical foundation** for the simulation's deeper architecture. Rather than using arbitrary tuning constants, the simulation derives its key ratios, balance points, and structural patterns from the golden ratio (Φ = 1.618...) and its Neoplatonic emanation framework. Wheeler's work integrates Pythagorean number theory, Neoplatonic ontology, Buddhist psychology (citta/vinnana distinction), and field theory into a unified cosmology that maps remarkably well onto agent-based simulation architecture. **See Section 16 for the complete integration specification**, including: agent coherence model (replacing flat personality traits), economic field theory (conjugate charge/discharge dynamics), phase transition mechanics (three energy barriers), information contagion models (beliefs vs. wisdom propagation), corruption as privation (not positive force), and a complete set of Φ-derived simulation constants.

---

## 3. World Design

### 3.1 Setting: Crossroads

The world is called **Crossroads** — a continent-scale landmass at a roughly early-industrial technology level (think 1700s–1850s equivalent), but with some fantastical elements to keep things interesting and provide natural economic complexity.

**Why this era?** Early industrial is the sweet spot for emergent economic simulation:
- Complex enough for trade networks, banking, manufacturing, and specialization
- Simple enough that the simulation doesn't need to model electronics, internet, or nuclear weapons
- The tension between agrarian tradition and industrial innovation creates natural factional conflict
- Fantastical elements (alchemy, strange fauna, weather anomalies) add unpredictability and delight

### 3.2 Geography

The world is a **hex grid** (preferred over square grids for more natural movement and adjacency). Each hex represents roughly 10 km².

```
World Size: ~2,000 hexes (roughly 45x45 hex grid, masked to a continental shape)
```

**Terrain types:**
- **Fertile Plains** — High agricultural yield, easy travel
- **Forest** — Timber, herbs, game; moderate travel difficulty
- **Mountains** — Minerals, gems, defensive positions; slow travel
- **Coast** — Fishing, port potential, trade routes
- **River** — Freshwater, irrigation bonus, trade arteries
- **Desert/Badlands** — Rare minerals, harsh conditions, low population
- **Swamp** — Alchemical ingredients, disease risk, difficult terrain
- **Tundra** — Furs, ice minerals, extreme conditions

**World generation** uses Perlin noise for elevation, rainfall, and temperature maps, then derives biomes. Rivers flow from high elevation to coast. The map is generated once at world creation and persists.

### 3.3 Settlements

Settlements emerge organically from agent clustering but are seeded initially:

- **3–5 Cities** (population 2,000–10,000 agents each): Founded at natural harbors, river confluences, or mineral-rich areas
- **10–20 Towns** (population 200–2,000): At crossroads, fertile valleys, mining sites
- **30–50 Villages** (population 20–200): Scattered across arable land
- **Outposts/Camps**: Temporary, emerging from exploration or conflict

Each settlement has:
- A **market** (local supply/demand price discovery)
- **Infrastructure**: roads, walls, wells, warehouses, workshops (buildable, degradable)
- **Governance**: A local leader (elected, hereditary, or seized), tax rate, laws
- A **culture score** across several axes (tradition vs. progress, isolationist vs. cosmopolitan, martial vs. mercantile)

### 3.4 Time System

```
1 real-world second = 1 simulation minute (configurable)
1 real-world minute = 1 simulation hour
1 real-world day = 24 simulation days (roughly 1 sim-month)
```

The simulation runs a **tick** every real-world second. Each tick:
1. Advances the world clock by 1 sim-minute
2. Runs fast rule-based agent updates (movement, consumption, work)
3. Every sim-hour (60 ticks): Runs market resolution, weather updates, event checks
4. Every sim-day (1,440 ticks): Runs daily summaries, births/deaths, building progress, agent reflection
5. Every sim-week (10,080 ticks): Runs diplomatic cycles, faction updates, LLM-powered major decisions
6. Every sim-season (≈90,000 ticks): Runs harvest calculations, seasonal weather shifts, major narrative arcs

---

## 4. Agent Architecture

### 4.1 Agent Data Model

Each agent is an entity with the following components:

```rust
struct Agent {
    id: AgentId,                    // Unique identifier
    name: String,                   // Generated name
    
    // Demographics
    age: u16,                       // In sim-years
    sex: Sex,
    health: f32,                    // 0.0–1.0
    
    // Location
    position: HexCoord,
    home_settlement: Option<SettlementId>,
    destination: Option<HexCoord>,
    
    // Economic
    occupation: Occupation,
    inventory: Inventory,           // Goods carried
    wealth: u64,                    // Currency units ("crowns")
    skills: SkillSet,              // Farming, mining, crafting, combat, trade, etc.
    
    // Social
    relationships: Vec<Relationship>,  // (AgentId, sentiment, trust)
    faction: Option<FactionId>,
    role: SocialRole,              // Commoner, merchant, soldier, leader, etc.
    
    // Cognitive (for LLM-tier agents)
    archetype: ArchetypeId,        // Behavioral template
    memory_stream: Vec<MemoryEntry>,
    personality: PersonalityVector, // Big Five traits as f32[5] — SEE SECTION 16 for Wheeler coherence model upgrade
    goals: Vec<Goal>,
    mood: f32,                     // -1.0 (despair) to 1.0 (elation)
    
    // Emanationist Soul (Wheeler integration — Section 16)
    soul: AgentSoul,               // Citta coherence, state of being, agent class, attachments
    
    // Metadata
    born_tick: u64,
    alive: bool,
}
```

### 4.2 Tiered Cognition Model

This is the core hybrid innovation. Agents don't all think the same way:

**Tier 0 — Automaton (95% of agents)**
Pure rule-based. Farmers farm, miners mine, merchants follow trade routes. Behavior is a state machine driven by needs (hunger, safety, wealth) and environmental conditions. Computationally near-free.

```
IF hungry AND has_food → eat
IF hungry AND NOT has_food → go_to_market OR forage
IF at_work AND work_hours → produce_goods
IF danger_nearby → flee OR fight (based on personality)
```

**Tier 1 — Archetype-Guided (4% of agents)**
These agents belong to behavioral archetypes. Periodically (weekly), Haiku generates updated behavioral templates for each archetype given current world conditions. Individual agents in the archetype then follow the template with personality-based variation.

Example archetypes: "Ambitious Merchant," "Devout Traditionalist," "Frontier Explorer," "Disgruntled Laborer," "Scheming Noble"

**Tier 2 — LLM-Powered Individuals (<1% of agents, ~20–50 total)**
Named characters — faction leaders, legendary merchants, notorious criminals, prophets. These agents get individual Haiku API calls for major decisions. They have full memory streams and can generate surprising, narrative-rich actions.

A Haiku call for a Tier 2 agent might look like:

```
You are Lord Aldric Voss, Governor of Ironhaven. You are 54 years old, 
pragmatic but increasingly paranoid about the growing influence of the 
Merchant's Guild.

CURRENT SITUATION:
- The northern mines report declining yields (iron production down 30%)
- The Merchant's Guild has petitioned for lower tariffs
- Your spymaster reports that the Guild is secretly funding a rival candidate
- A strange plague has appeared in the eastern villages
- Your approval rating among commoners: 42% (down from 68% last season)

RECENT MEMORIES:
- Last month you raised taxes to fund wall repairs
- The Guild leader, Senna Blackwood, publicly criticized your tax policy
- Your daughter's marriage to the Duke of Ashford strengthened your alliance

What do you do this week? Respond with 1-3 actions in JSON format:
[{"action": "...", "target": "...", "reasoning": "..."}]
```

### 4.3 Needs System (Maslow-Inspired, Wheeler-Enhanced)

Every agent has needs that drive behavior, evaluated bottom-up. The Wheeler integration (Section 16) maps these to three States of Being: Torment (trapped at levels 1–2), Well-Being (stable at levels 3–4), and Liberation (transcending level 5).

1. **Survival**: Food, water, shelter, health
2. **Safety**: Physical security, economic stability
3. **Belonging**: Social connections, community, faction membership
4. **Esteem**: Reputation, wealth relative to peers, skill mastery
5. **Purpose**: Goals, legacy, meaning (only evaluated for Tier 1+ agents)

Unmet lower needs dominate behavior. A starving merchant doesn't trade — they forage. A secure, well-fed agent with high esteem might pursue political ambitions.

---

## 5. Economic System

The economy is the simulation's beating heart. It must be robust enough to sustain itself indefinitely while producing interesting dynamics.

### 5.1 Resources and Goods

**Primary resources** (harvested from terrain):
- **Grain** — From fertile plains. Staple food. Seasonal.
- **Timber** — From forests. Construction material.
- **Iron Ore** — From mountains. Smelted into iron.
- **Stone** — From mountains/quarries. Construction.
- **Fish** — From coast/rivers. Food source.
- **Herbs** — From forests/swamps. Medicine and alchemy.
- **Gems** — Rare, from deep mines. Luxury/currency.
- **Pelts/Furs** — From hunting. Clothing, luxury trade.
- **Coal** — From mines. Fuel for smelting and industry.
- **Exotic Materials** — Rare spawns. Alchemical reagents, strange metals. These inject surprise.

**Manufactured goods** (crafted from resources):
- **Tools** — Iron + Timber. Required for efficient production.
- **Weapons** — Iron + Timber/Leather. Military use.
- **Clothing** — Pelts/Fibers + Labor. Basic need.
- **Medicines** — Herbs + Knowledge. Combat disease.
- **Luxury Goods** — Gems + Crafting skill. Status, trade value.
- **Ships** — Timber + Iron + High skill. Enable sea trade.
- **Buildings** — Stone + Timber + Iron. Infrastructure.
- **Alchemical Products** — Exotic materials + Herbs. Unpredictable effects.

### 5.2 Market Mechanics

Each settlement runs a **local market** using a simple but effective price discovery mechanism:

```
Price = BasePrice × (Demand / Supply) × RegionalModifier × SeasonalModifier
```

- **Supply** = quantity available in local market + incoming trade
- **Demand** = consumption needs + export orders + stockpiling
- Prices are bounded by floor (production cost) and ceiling (agent willingness to pay)
- **Arbitrage**: Merchants profit by buying where prices are low and selling where prices are high, naturally equalizing prices across regions (with transport cost friction)

### 5.3 Currency and Banking

- **Crowns**: The universal currency. Minted from gold/silver (rare resources).
- **Barter**: Agents can trade goods directly when currency is scarce.
- **Credit**: Tier 1+ merchants can extend credit. Debt is tracked. Default has social consequences.
- **Taxation**: Settlement governments collect taxes (configurable rate). Tax revenue funds infrastructure, military, public goods.
- **Banking**: Cities may develop banks (emergent institution) that hold deposits and make loans.

### 5.4 Faucets and Sinks (Anti-Collapse Mechanisms)

**Faucets (currency/goods enter the world):**
- Resource harvesting (finite per hex per season, regenerates slowly)
- Mining (depletes over time, new deposits discovered rarely)
- Births (new agents = new producers and consumers)
- Random events (discovery of treasure, rich new veins, bumper harvests)

**Sinks (currency/goods leave the world):**
- Consumption (agents eat food, wear out clothes, use tools)
- Building decay (infrastructure degrades without maintenance)
- Death (agent's unretrieved inventory partially lost)
- Taxation (some tax revenue "evaporates" as administrative overhead)
- Disasters (fires, floods, plagues destroy goods)
- Crafting loss (imperfect conversion ratios)

**Balance monitoring**: The simulation tracks total currency in circulation, total goods by category, and Gini coefficient. If metrics drift outside healthy bands, automatic rebalancing triggers (a new mine discovered, a plague, a trade caravan from "off-map").

### 5.5 Trade Network

Settlements are connected by **trade routes** (roads, rivers, sea lanes). Trade happens via:

1. **Merchant agents** who physically travel between settlements carrying goods
2. **Caravans** (groups of merchants for safety and efficiency)
3. **Sea trade** (coastal cities with ships can trade with distant ports)

Trade route quality affects:
- Travel time
- Bandit risk
- Goods capacity

Roads can be built and improved by settlement investment. River trade is faster. Sea trade has highest capacity but requires ships and ports.

---

## 6. Social and Political Systems

### 6.1 Factions

Factions emerge from shared interests, ideology, or geography:

**Seed factions** (present at world start):
- **The Crown** — Monarchist establishment, order and tradition
- **The Merchant's Compact** — Free trade, low taxes, mercantile power
- **The Iron Brotherhood** — Labor, miners, craftsmen, fair wages
- **The Verdant Circle** — Agrarian, environmental, suspicious of industry
- **The Ashen Path** — Alchemists, scholars, seekers of forbidden knowledge

Factions have:
- **Influence** per settlement (0–100)
- **Relations** with other factions (-100 to +100)
- **Policies** they advocate for
- **Leadership** (Tier 2 agent)
- **Treasury** (funded by member dues and activities)

New factions can emerge. Factions can merge, split, or dissolve.

### 6.2 Governance

Each settlement has a governance model:
- **Monarchy/Autocracy**: One leader, hereditary or seized. Stable but can be oppressive.
- **Council**: Elected representatives. Responsive but slow.
- **Merchant Republic**: Wealthiest citizens govern. Pro-trade but inequality-prone.
- **Commune**: Direct democracy. Egalitarian but vulnerable to manipulation.

Governance affects tax rates, law enforcement, trade policy, military readiness, and public works.

**Revolutions** can occur when approval is very low, a faction's influence is very high, and a charismatic leader (Tier 2 agent) is available.

### 6.3 Conflict

- **Crime**: Agents with unmet needs may steal, smuggle, or form bandit gangs. Law enforcement (town guard) deters crime proportional to funding.
- **Feuds**: Inter-faction or inter-settlement disputes over resources, trade routes, or ideology.
- **War**: Large-scale conflict between settlements or alliances. Resolved through a simplified combat model considering troop numbers, equipment, terrain, leadership, and morale.
- **Diplomacy**: Tier 2 leaders negotiate treaties, trade agreements, alliances, and marriages. These are prime LLM decision points.

---

## 7. External Data Integration

### 7.1 Weather System (API-Driven)

Weather is driven by a **real weather API** (e.g., OpenWeatherMap) mapped onto the simulation world:

- Query a configurable real-world location's weather hourly
- Map real temperature, precipitation, wind to simulation equivalents
- Real-world storms become in-game storms (scaled/filtered)
- Creates genuine unpredictability without computational cost

**Fallback**: If the API is unavailable, use a Perlin noise-based seasonal weather model.

**Mapping example:**
```
Real: San Diego, 72°F, sunny → Sim: Coastal Crossroads, warm, clear skies
Real: Minneapolis, -5°F, blizzard → Sim: Northern Crossroads, brutal cold, snowstorm
```

### 7.2 Random.org for True Entropy

Use random.org's API for critical random events:
- Natural disasters (earthquake, volcanic eruption, tsunami)
- Discovery events (new resource deposits, ancient ruins, strange phenomena)
- Mutation events (a new disease, an alchemical breakthrough, a prophetic vision)

True randomness from atmospheric noise ensures events can't be predicted or pattern-matched, adding genuine surprise.

### 7.3 Optional: Real-World News Injection

As an advanced feature, RSS feeds from real news could be semantically mapped to in-world events:
- Real earthquake → sim earthquake
- Real market crash → sim trade disruption
- Real election → sim political upheaval

This echoes the SWS concept of a "mirror world" calibrated to real events.

---

## 8. Technical Architecture

### 8.1 System Overview

```
┌──────────────────────────────────────────────────────┐
│                   WORLDSIM SERVER                     │
│                                                       │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐   │
│  │  TICK    │  │  WORLD   │  │   EVENT/NEWS      │   │
│  │  ENGINE  │→ │  STATE   │→ │   GENERATOR       │   │
│  │  (Rust)  │  │  (ECS)   │  │   (Rust + Haiku)  │   │
│  └────┬─────┘  └────┬─────┘  └────────┬──────────┘   │
│       │              │                 │               │
│  ┌────┴─────┐  ┌────┴─────┐  ┌───────┴──────────┐   │
│  │ AGENT    │  │ ECONOMY  │  │   PERSISTENCE     │   │
│  │ SYSTEMS  │  │ ENGINE   │  │   (SQLite/Postgres)│   │
│  │ (Rust)   │  │ (Rust)   │  │                    │   │
│  └────┬─────┘  └──────────┘  └───────────────────┘   │
│       │                                               │
│  ┌────┴──────────────────────────────────────────┐   │
│  │          EXTERNAL API LAYER                    │   │
│  │  Weather API │ Random.org │ Haiku API          │   │
│  └────────────────────────────────────────────────┘   │
│                                                       │
│  ┌────────────────────────────────────────────────┐   │
│  │          HTTP API (query interface)             │   │
│  │  GET /news │ GET /economy │ GET /agent/:id     │   │
│  │  GET /map  │ GET /factions │ POST /intervention│   │
│  └────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────┘
```

### 8.2 Core Components (Rust)

**Tick Engine**
- Drives the simulation forward
- Manages the layered tick schedule (minute/hour/day/week/season)
- Handles simulation speed control (pause, 1x, 10x, 100x)
- Runs on a dedicated thread

**World State (ECS-like)**
- Entities: Agents, Settlements, Hexes, Trade Routes, Factions, Items
- Components: Position, Health, Inventory, Economy, Governance, Memory, etc.
- Systems: MovementSystem, ProductionSystem, MarketSystem, CombatSystem, NeedsSystem, etc.
- State is the single source of truth; all systems read from and write to it

**Agent Systems**
- `NeedsSystem`: Evaluate agent needs, set priorities
- `MovementSystem`: Path agents toward destinations
- `ProductionSystem`: Agents at work produce goods
- `ConsumptionSystem`: Agents consume food, wear out tools
- `SocialSystem`: Relationship updates, faction loyalty
- `CognitionSystem`: Trigger LLM calls for Tier 1–2 agents on schedule

**Economy Engine**
- `MarketSystem`: Price discovery per settlement per tick-hour
- `TradeSystem`: Move merchants along routes, execute trades
- `TaxSystem`: Collect taxes, fund public goods
- `BalanceMonitor`: Track macro indicators, trigger rebalancing

**Event/News Generator**
- Monitors world state for newsworthy events
- Classifies events by importance and category
- Generates human-readable news items (Haiku for major events, templates for minor)
- Maintains a persistent news archive queryable by date, topic, location

### 8.3 Persistence

**SQLite** for single-server deployment (upgrade path to Postgres for scaling):

```sql
-- Core tables
agents (id, name, age, sex, health, position_q, position_r, 
        occupation, wealth, faction_id, archetype_id, alive, born_tick)
settlements (id, name, position_q, position_r, population, 
             governance_type, tax_rate, treasury)
hexes (q, r, terrain, resources_json, infrastructure_json)
factions (id, name, leader_id, treasury, ideology_json)

-- Economic tables
market_prices (settlement_id, good_type, price, supply, demand, tick)
trade_routes (id, from_settlement, to_settlement, quality, safety)
inventories (agent_id, good_type, quantity)

-- Event tables  
events (id, tick, type, severity, location_q, location_r, 
        description, actors_json)
news (id, tick, headline, body, category, importance)

-- Memory tables (for Tier 1-2 agents)
memories (agent_id, tick, content, importance, type)
reflections (agent_id, tick, content)

-- Snapshots for excursions/forks
snapshots (id, tick, description, state_blob)
```

**Snapshot system**: Every sim-season, automatically save a full world state snapshot. Supports manual snapshots and world forking for "what if" experiments.

### 8.4 API Endpoints

The HTTP API (using `axum` or `actix-web`) provides:

```
GET  /api/v1/status              → World clock, population, economic summary
GET  /api/v1/news?limit=20       → Latest news headlines and stories
GET  /api/v1/news/:id            → Full news article
GET  /api/v1/newspaper           → Curated daily digest (Haiku-generated)
GET  /api/v1/economy             → Global economic indicators
GET  /api/v1/economy/:settlement → Local market prices, trade volume
GET  /api/v1/map                 → Full hex map with current state
GET  /api/v1/map/:q/:r           → Hex detail (terrain, agents, resources)
GET  /api/v1/agent/:id           → Agent profile, inventory, memories
GET  /api/v1/agent/:id/story     → Haiku-generated biography of agent's life
GET  /api/v1/settlement/:id      → Settlement detail
GET  /api/v1/factions            → All factions with influence maps
GET  /api/v1/faction/:id         → Faction detail, members, policies
GET  /api/v1/history?from=&to=   → Historical events in range
GET  /api/v1/statistics          → Demographic, economic, conflict stats over time

POST /api/v1/speed               → Set simulation speed
POST /api/v1/snapshot            → Create manual snapshot
POST /api/v1/intervention        → Inject an event (god mode: earthquake, gold discovery, etc.)
POST /api/v1/fork/:snapshot_id   → Fork world from snapshot (future feature)
```

The `/newspaper` endpoint is the crown jewel — it calls Haiku to synthesize the day's events into a readable, entertaining newspaper with headlines, editorials, market reports, obituaries, and rumors.

### 8.5 Haiku API Integration

**Rate budgeting**: With ~50 Tier 2 agents making weekly decisions and archetype updates, plus newspaper generation:

```
Weekly LLM calls:
  Tier 2 decisions:     ~50 calls/sim-week
  Archetype updates:    ~15 calls/sim-week  
  Newspaper generation: ~7 calls/sim-week (daily digest)
  Major event narration: ~5 calls/sim-week
  On-demand (API queries): ~20 calls/sim-week
  
  Total: ~100 calls/sim-week
  At 1 sim-week ≈ 7 real-world minutes: ~15 calls/real-minute peak
```

This is very manageable for the Haiku API. Calls are batched and queued to smooth out burst patterns.

**Prompt templates** are stored as configurable TOML files so they can be tuned without recompiling:

```toml
[prompts.tier2_decision]
system = """You are simulating a character in a persistent world. 
Respond ONLY with valid JSON. Be creative but consistent with the 
character's personality and situation."""

[prompts.newspaper]
system = """You are the editor of The Crossroads Chronicle, a daily 
newspaper in a fictional world. Write engaging, in-character journalism 
based on the events provided. Include headlines, a lead story, market 
report, and one human interest piece."""
```

---

## 9. World Perpetuation Mechanics

### 9.1 Population Dynamics
- **Births**: Probability based on settlement prosperity, agent age, relationship status
- **Deaths**: Age, disease, violence, starvation
- **Migration**: Agents move toward prosperity, safety, and opportunity
- **Immigration/Emigration**: "Off-map" populations provide a buffer (people arrive when conditions are good, leave when they're bad)

### 9.2 Resource Regeneration
- Forests regrow over seasons
- Fish populations recover if not overfished
- Farmland fertility cycles (crop rotation benefits)
- Mines deplete but new deposits are discovered (tied to random.org events)

### 9.3 Anti-Stagnation Mechanisms
- **Technology drift**: Occasional breakthroughs (new crops, better smelting, navigation advances) open new economic possibilities
- **Generational change**: Young agents have slightly different values than old ones, creating cultural drift
- **External shocks**: Random.org-triggered events prevent equilibrium from becoming boring
- **The Ashen Path**: The alchemist faction occasionally produces wild card events (transmutation discoveries, strange creatures, prophetic visions)

### 9.4 Anti-Collapse Safeguards
- **Minimum population floor**: If a settlement drops below 10 agents, refugees arrive
- **Economic circuit breaker**: If inflation exceeds 500% or deflation exceeds 80%, emergency adjustments activate
- **Famine relief**: If starvation exceeds 20% of population, emergency food shipments arrive from off-map
- **Peace fatigue**: Extended wars gradually increase war-weariness, pushing toward treaties

---

## 10. Implementation Roadmap

### Phase 1: Foundation (MVP)
- [ ] Hex grid world generation (terrain, resources)
- [ ] Basic agent struct and Tier 0 behavior (needs-driven state machine)
- [ ] Simple economy (production, consumption, local markets)
- [ ] Settlement data model
- [ ] SQLite persistence
- [ ] Tick engine with minute/hour/day cycles
- [ ] Basic HTTP API (status, map, agents)
- [ ] Weather API integration

### Phase 2: Economy and Trade
- [ ] Multi-settlement trade routes
- [ ] Merchant agent behavior
- [ ] Price discovery across settlements
- [ ] Currency system (crowns)
- [ ] Crafting/manufacturing chain
- [ ] Economic monitoring dashboard endpoint

### Phase 3: Social and Political
- [ ] Factions with influence mechanics
- [ ] Settlement governance
- [ ] Relationships between agents
- [ ] Crime and law enforcement
- [ ] Tier 1 archetype system
- [ ] Basic conflict resolution

### Phase 4: LLM Integration
- [ ] Haiku API client with rate limiting and retry
- [ ] Tier 2 agent cognition (memory stream, decision prompts)
- [ ] Archetype template generation for Tier 1
- [ ] Newspaper/news generation endpoint
- [ ] Agent biography generation
- [ ] Event narration

### Phase 5: Polish and Perpetuation
- [ ] Population dynamics (birth, death, migration)
- [ ] Resource regeneration and discovery
- [ ] Anti-stagnation and anti-collapse mechanics
- [ ] Snapshot and fork system
- [ ] Random.org integration
- [ ] Real-time event injection (god mode)
- [ ] Historical statistics and charting endpoint

---

## 11. Configuration

All tuning parameters live in a `worldsim.toml` file:

```toml
[simulation]
tick_rate_ms = 1000              # Real-world ms per tick
sim_minutes_per_tick = 1         # Sim-minutes per tick
max_agents = 50000
starting_agents = 5000

[world]
hex_grid_radius = 22             # ~2000 hexes
sea_level = 0.3                  # Perlin noise threshold
mountain_threshold = 0.75
seed = 0                         # 0 = random seed

[economy]
starting_crowns_per_agent = 100
tax_rate_default = 0.10
inflation_ceiling = 5.0          # Trigger rebalance above this
deflation_floor = 0.2            # Trigger rebalance below this
resource_regen_rate = 0.05       # Per season

[agents]
tier2_count = 30
tier1_fraction = 0.04
needs_eval_interval_ticks = 60   # Every sim-hour
memory_max_entries = 500
reflection_interval_ticks = 1440 # Every sim-day

[api]
haiku_model = "claude-haiku-4-5-20251001"
haiku_max_tokens = 1024
haiku_calls_per_minute_limit = 20
weather_api_key = ""
weather_location = "San Diego, CA"
random_org_api_key = ""

[server]
host = "0.0.0.0"
port = 8080
```

---

## 12. File Structure

```
worldsim/
├── Cargo.toml
├── worldsim.toml                 # Configuration
├── prompts/                      # LLM prompt templates
│   ├── tier2_decision.toml
│   ├── archetype_update.toml
│   ├── newspaper.toml
│   ├── event_narration.toml
│   └── agent_biography.toml
├── src/
│   ├── main.rs                   # Entry point, server setup
│   ├── config.rs                 # Configuration loading
│   ├── tick.rs                   # Tick engine
│   ├── world/
│   │   ├── mod.rs
│   │   ├── hex.rs                # Hex grid, terrain
│   │   ├── generation.rs         # World generation (Perlin noise)
│   │   ├── weather.rs            # Weather system + API client
│   │   └── map.rs                # Map queries and serialization
│   ├── agents/
│   │   ├── mod.rs
│   │   ├── types.rs              # Agent struct, components
│   │   ├── needs.rs              # Needs evaluation
│   │   ├── movement.rs           # Pathfinding and movement
│   │   ├── production.rs         # Work and crafting
│   │   ├── cognition.rs          # Tiered cognition dispatcher
│   │   ├── tier0.rs              # Rule-based behavior
│   │   ├── tier1.rs              # Archetype-guided behavior
│   │   ├── tier2.rs              # LLM-powered decisions
│   │   └── memory.rs             # Memory stream and reflection
│   ├── economy/
│   │   ├── mod.rs
│   │   ├── market.rs             # Price discovery, trading
│   │   ├── goods.rs              # Resource and goods definitions
│   │   ├── trade.rs              # Trade routes and merchants
│   │   ├── currency.rs           # Money supply, banking
│   │   └── balance.rs            # Macro monitoring and rebalancing
│   ├── social/
│   │   ├── mod.rs
│   │   ├── factions.rs           # Faction mechanics
│   │   ├── governance.rs         # Settlement governance
│   │   ├── relationships.rs      # Agent relationships
│   │   ├── conflict.rs           # Crime, feuds, war
│   │   └── culture.rs            # Cultural drift
│   ├── events/
│   │   ├── mod.rs
│   │   ├── detector.rs           # Event detection from state changes
│   │   ├── generator.rs          # Random and scheduled events
│   │   ├── news.rs               # News article generation
│   │   └── newspaper.rs          # Daily digest compilation
│   ├── external/
│   │   ├── mod.rs
│   │   ├── haiku.rs              # Anthropic API client
│   │   ├── weather_api.rs        # Weather API client
│   │   └── random_org.rs         # Random.org API client
│   ├── persistence/
│   │   ├── mod.rs
│   │   ├── db.rs                 # Database operations
│   │   ├── schema.rs             # Table definitions
│   │   └── snapshots.rs          # Snapshot and fork
│   └── api/
│       ├── mod.rs
│       ├── routes.rs             # HTTP route definitions
│       ├── handlers.rs           # Request handlers
│       └── responses.rs          # Response serialization
└── tests/
    ├── economy_tests.rs
    ├── agent_tests.rs
    └── integration_tests.rs
```

---

## 13. Key Rust Crates

```toml
[dependencies]
tokio = { version = "1", features = ["full"] }       # Async runtime
axum = "0.7"                                          # HTTP framework
serde = { version = "1", features = ["derive"] }      # Serialization
serde_json = "1"
sqlx = { version = "0.7", features = ["sqlite", "runtime-tokio"] }
noise = "0.9"                                         # Perlin noise for worldgen
rand = "0.8"                                          # Local RNG
reqwest = { version = "0.11", features = ["json"] }   # HTTP client (APIs)
toml = "0.8"                                          # Config parsing
tracing = "0.1"                                       # Logging
tracing-subscriber = "0.3"
chrono = "0.4"                                        # Time handling
uuid = { version = "1", features = ["v4"] }           # Agent IDs
dashmap = "5"                                         # Concurrent hashmap
pathfinding = "4"                                     # A* for agent movement
```

---

## 14. Example API Response: Daily Newspaper

```json
GET /api/v1/newspaper

{
  "date": "Day 47, Season of Iron, Year 3",
  "tick": 67680,
  "headline": "IRONHAVEN MINES YIELD MYSTERIOUS ORE — Alchemists Flock to Northern City",
  "lead_story": "Miners in the deep shafts beneath Mount Carda struck an unusual vein of shimmering blue-black ore yesterday, sending ripples through the markets and drawing the attention of the Ashen Path...",
  "market_report": {
    "summary": "Iron prices fell 12% on news of the Ironhaven discovery. Grain steady at 3.2 crowns/bushel. Timber rising in Ashford due to new construction boom.",
    "notable_prices": [
      {"good": "Iron Ore", "settlement": "Ironhaven", "price": 2.1, "change": -0.12},
      {"good": "Grain", "settlement": "Millhaven", "price": 3.2, "change": 0.01},
      {"good": "Timber", "settlement": "Ashford", "price": 8.7, "change": 0.15}
    ]
  },
  "faction_news": "The Merchant's Compact has formally protested Governor Voss's new 5% export tariff on processed metals, calling it 'a stranglehold on prosperity.'",
  "human_interest": "Elderly herbalist Mira Thornwood of Greenhollow celebrated her 80th birthday this week — the oldest known resident of Crossroads. When asked her secret, she reportedly said, 'Good soil and bad memories.'",
  "obituaries": ["Sergeant Kael Dunmore, 34, killed in a bandit ambush on the Eastern Road."],
  "rumors": ["Whispers in the taverns suggest the Ashen Path has discovered how to transmute copper into silver. The Guild denies all knowledge."],
  "weather_forecast": "Clear skies expected through mid-week, with a cold front moving in from the northern tundra by weekend."
}
```

---

## 15. Getting Started (Instructions for Claude Code)

1. **Initialize the Rust project**: `cargo init worldsim`
2. **Start with Phase 1**: Get the hex grid, basic agents, and tick engine running
3. **Prioritize the economy**: A working market with supply/demand should be the first "interesting" feature
4. **Add the API early**: Even with minimal features, being able to query the world state via HTTP makes development more fun
5. **Integrate Haiku last**: The world should be interesting with pure rule-based agents first; LLM adds spice, not structure
6. **Test with small worlds first**: Use a 10-hex world with 50 agents to iterate quickly before scaling up
7. **Log everything**: Use `tracing` extensively. The simulation's behavior should be auditable.

**Remember**: The goal is a world we want to check in on. Every design decision should serve that goal. If a feature doesn't make the world more interesting to observe, defer it.


---

## 16. Emanationist Cosmology — The Wheeler Integration

**Source corpus:** 12 documents by Ken Wheeler (atom, gravity, cosmic unity, theurgy, energy/fields, codex universalis, holographic cosmos, fools & lies, codex on evil, golden universal truth, water molecule geometry) plus the Purdue/JFCOM Sentient World Simulation technical paper. This section translates Wheeler's emanationist metaphysics into concrete simulation architecture, replacing arbitrary tuning constants with a unified mathematical framework rooted in the golden ratio (Φ = 1.6180339887).

### 16.1 Ontological Stack — What the Simulation Is Made Of

The simulation's layer model maps directly to the Neoplatonic emanation hierarchy. Each layer emanates from the one above it, with increasing complexity and decreasing unity as you descend.

| Layer | Wheeler Term | Simulation Mapping | Φ Power | Value |
|-------|-------------|-------------------|---------|-------|
| 0 | Aether / Rest / Energy | Runtime/OS — the tick loop itself. Pure potentiality with no Cartesian presence inside the simulation. Agents cannot observe it. Known only by effects. | Φ⁰ | 1.00000 |
| 1 | Monad / The One | Kernel tick function — the single indivisible heartbeat from which all simulation state emanates each tick. The "unmoved mover." | Φ⁰ | 1.00000 |
| 1' | Nous / Citta / Spirit | LLM layer (Haiku API) — intelligence that animates agents but is not *of* the simulation. The "signal" in the radio analogy. Counterspatial — never directly observable through the world API. | Φ² | 2.61803 |
| 2 | Magnitude / Topos | Hex grid / spatial field — the relational ground on which all entities exist. Not "empty space" (space doesn't exist as a thing) but the field of becoming. | — | — |
| 3 | Matter / Hyle | Resources, goods, buildings — the material substrate. Furthest from source, subject to decay, the "wake" of emanation. | Φ⁻¹ | 0.61803 |
| 4 | Time | Tick counter — a measure, not a substance. "Time does not exist in the Fibonacci sequence." Time = 4, absent from 1,1,2,3,5. Implemented as a monotonic u64 counter, never as a simulation entity. | 4 | (absent) |
| 5 | Man / Being | Agents — the living union of all pairs. The "pentad" where matter and spirit meet through the logos of interaction. Consubstantial composite of citta + body = consciousness. | Φ¹ | 1.61803 |

**Implementation note:** The Φ power values (0.23606 through 4.23606) should be defined as named constants in the codebase and used wherever the simulation needs balance ratios, decay rates, or distribution curves instead of arbitrary magic numbers.

```rust
// Core emanation constants derived from Phi
pub mod phi {
    pub const PHI: f64 = 1.6180339887;
    pub const PHI_INV: f64 = 0.6180339887;       // Φ⁻¹: matter/form ratio
    pub const PHI_SQ: f64 = 2.6180339887;        // Φ²: nous/unity ratio
    pub const PHI_CUBE: f64 = 4.2360679775;      // Φ³: totality/completion
    pub const AGNOSIS: f64 = 0.2360679775;        // Φ⁻³: entropy/privation constant
    pub const PSYCHE: f64 = 0.3819660113;         // Φ⁻²: soul/coherence base
    pub const PENTAD: f64 = 5.0;                  // completion constant
    pub const GOLDEN_ANGLE: f64 = 137.5077;       // degrees, optimal growth angle
    pub const LIFE_ANGLE: f64 = 85.0;             // degrees, plane of inertia
    pub const MANIFESTATION_ANGLE: f64 = 108.0;   // degrees, aoristos dyad
}
```

### 16.2 Agent Coherence Model — Replacing Flat Personality

The existing framework uses Big Five personality traits (`PersonalityVector: f32[5]`). The Wheeler integration replaces this with a **coherence model** that is both simpler and richer. Coherence represents how "laser-like" vs. "light-bulb-like" an agent's identity is — their degree of self-similarity and point-source concentration.

**The laser vs. light bulb principle:** A 5-watt light bulb is useless; a 5-watt laser is dangerous. The difference is coherence — self-similarity of output. A scattered agent with many mediocre capabilities has little impact. An agent who has subtracted everything but one mastery becomes a point-source of disproportionate influence.

```rust
struct AgentSoul {
    // Core identity — the "citta" (signal, not the radio)
    citta_coherence: f32,       // 0.0–1.0, the master variable
    citta_vector: [f32; 4],     // Mass, Gauss, Drive, Wisdom — the "DNA"
    
    // Derived state — the "consciousness" (broadcast from the radio)
    state_of_being: StateOfBeing,  // Torment, WellBeing, or Liberation
    agent_class: AgentClass,       // Devotionalist, Ritualist, Nihilist, Transcendentalist
    
    // Via negativa accumulator
    attachments: Vec<Attachment>,  // Things binding the agent to phenomena
    wisdom_score: f32,             // Accumulated through subtraction, not addition
}

enum StateOfBeing {
    /// Low coherence (0.0–0.3). Scattered among phenomena. Reactive, driven by 
    /// immediate desires/fears. Takes on attribution of environment like incoherent 
    /// light takes color from walls. Repeats patterns endlessly.
    Torment,
    
    /// Medium coherence (0.4–0.6). Stable, prosperous, materially successful. 
    /// "Meditator" — calm but not wise. Centered within existential phenomena 
    /// but not transcending them. The "well-being" trap.
    WellBeing,
    
    /// High coherence (0.7–1.0). Extremely rare. "Dead man walking" — citta has 
    /// transcended attachment. Self-similar, point-source. Disproportionate influence 
    /// on world events. Becomes teacher/leader/founder through subtraction.
    Liberation,
}

enum AgentClass {
    /// Driven by loyalty, tradition, social conformity, belief systems.
    /// Follows group consensus. Emotionally reactive. Most common class.
    Devotionalist,
    
    /// Driven by routine, established patterns, "the way things are done."
    /// Stable economic actors. Follow trade routes and established procedures.
    Ritualist,
    
    /// Driven by pure self-interest, accumulation, zero-sum thinking.
    /// Aggressive competitors. Atomistic worldview — no loyalty beyond utility.
    Nihilist,
    
    /// Extremely rare. Wisdom-seeking. Driven by subtraction rather than 
    /// accumulation. High coherence. Disproportionate cultural/institutional influence.
    Transcendentalist,
}
```

**Coherence dynamics — the via negativa:**

Coherence does NOT increase by adding traits, skills, or possessions. It increases by *removing attachments*. This is the core Wheeler principle: "All negatives are affirmations of a prior Subject. All negation of marble chips reveals the statue within."

```rust
fn update_coherence(agent: &mut AgentSoul, event: &WorldEvent) {
    match event {
        // Coherence INCREASES (via negativa — subtraction)
        WorldEvent::LossAccepted { .. } => agent.citta_coherence += AGNOSIS * 0.5,
        WorldEvent::AttachmentReleased { .. } => {
            agent.attachments.retain(|a| a.id != event.target);
            agent.citta_coherence += PSYCHE * 0.3;
        },
        WorldEvent::WisdomGained { .. } => agent.citta_coherence += AGNOSIS * 0.3,
        WorldEvent::MentorEncounter { .. } => agent.citta_coherence += AGNOSIS * 0.8,
        
        // Coherence DECREASES (attachment/dilution — addition of phenomena)
        WorldEvent::GreedEvent { .. } => agent.citta_coherence -= AGNOSIS * 0.4,
        WorldEvent::FearEvent { .. } => agent.citta_coherence -= AGNOSIS * 0.3,
        WorldEvent::TraumaEvent { .. } => agent.citta_coherence -= AGNOSIS * 0.6,
        WorldEvent::ExcessAccumulation { .. } => agent.citta_coherence -= AGNOSIS * 0.5,
        
        _ => {}
    }
    agent.citta_coherence = agent.citta_coherence.clamp(0.0, 1.0);
    agent.state_of_being = StateOfBeing::from_coherence(agent.citta_coherence);
}
```

**The extraction paradox:** High-coherence agents experience MORE internal tension than low-coherence ones. The wise agent sees corruption clearly and suffers from that knowledge. Low-coherence agents are "happier" in the short term. This means `mood` should be partially *inverse* to coherence in many situations — wisdom is a burden that enables liberation, not bliss.

### 16.3 The Four-Variable Agent Classification (Mass × Gauss)

Wheeler's magneto-atomic model classifies all atoms by two orthogonal axes: mass (accumulated substance) and gauss/capacitance (field intensity, drive, ambition). Applied to agents:

```
                    HIGH GAUSS (Ambition/Drive)
                           │
         Hydrogen-Type     │     Uranium-Type
         Low mass,         │     High mass,
         high drive.       │     high drive.
         Volatile,         │     Powerful but
         transformative,   │     unstable,
         reactive.         │     potentially
         Entrepreneurs,    │     destructive.
         revolutionaries.  │     Tyrants, empire-
                           │     builders.
    ───────────────────────┼───────────────────────
                           │
         Helium-Type       │     Gold-Type
         Low mass,         │     High mass,
         low drive.        │     low drive.
         Inert, stable,    │     Wealthy but
         unreactive.       │     passive. Hoards.
         Subsistence       │     Old money,
         farmers,          │     institutional
         hermits.          │     inertia.
                           │
                    LOW GAUSS (Ambition/Drive)
```

```rust
struct CittaVector {
    mass: f32,        // 0.0–1.0: accumulated capability, wealth, social weight
    gauss: f32,       // 0.0–1.0: ambition, drive, field intensity
    // Derived classification
    fn element_type(&self) -> ElementType {
        match (self.mass > 0.5, self.gauss > 0.5) {
            (false, false) => ElementType::Helium,    // inert, stable
            (false, true)  => ElementType::Hydrogen,  // reactive, transformative
            (true, false)  => ElementType::Gold,      // wealthy, passive
            (true, true)   => ElementType::Uranium,   // powerful, unstable
        }
    }
}
```

**Population distribution:** Most agents should be Helium-type (stable, unreactive) or mid-range. Hydrogen-types drive change. Gold-types provide stability and capital. Uranium-types drive dramatic events (both good and catastrophic). Distribution follows a natural curve — very few at the extremes.

### 16.4 Consubstantiality — The Radio Analogy for Agent Life/Death

Agents are consubstantial composites, like radios:

| Radio Component | Agent Component | Persistence |
|----------------|-----------------|-------------|
| Signal from distant station | Citta (core identity vector, coherence) | Persists after death as cultural/institutional memory |
| Antenna (water/dipole) | Interaction protocol — how agent interfaces with world | Exists only while agent lives |
| Radio hardware (body) | Physical state (health, position, inventory, age) | Dies, decays, recycled |
| Broadcast from speaker | Consciousness (observable behavior, decisions) | Ceases at death |

**Death mechanics:** When an agent dies:
1. **Body** → resources return to the world (goods drop, position freed)
2. **Consciousness** → ceases entirely (no ghost agents)
3. **Citta** → partially persists as: cultural norms in their settlement, institutional knowledge in their faction/guild, reputation/legend effects on other agents, wisdom contributions to collective knowledge pool

```rust
fn on_agent_death(world: &mut World, agent: &Agent) {
    // Body returns to matter
    world.drop_inventory(agent.position, &agent.inventory);
    
    // Consciousness ceases — remove from active agent list
    world.agents.remove(agent.id);
    
    // Citta partially persists — knowledge/wisdom/culture survive
    if let Some(settlement) = agent.home_settlement {
        let wisdom_contribution = agent.soul.wisdom_score * PSYCHE;
        world.settlements[settlement].cultural_memory += wisdom_contribution;
        
        // High-coherence agents leave disproportionate cultural imprint
        if agent.soul.citta_coherence > 0.7 {
            world.settlements[settlement].add_legend(agent.name.clone(), agent.soul.clone());
        }
    }
    
    // Knowledge vs. wisdom: knowledge dies, wisdom persists
    // Knowledge = agent-local data (routes known, prices seen) → LOST
    // Wisdom = cultural patterns, governance norms, trade customs → PERSISTS
}
```

### 16.5 Economic Field Theory — Conjugate Dynamics

Every economic system in SYNTHESIS operates as a **conjugate pair** — two inseparable aspects of a unified field, like magnetism and dielectric, torus and hyperboloid.

#### 16.5.1 Charge/Discharge Economics

| Charging (Centripetal) | Discharging (Centrifugal) |
|----------------------|-------------------------|
| Saving, investing, accumulating | Spending, consuming, distributing |
| Capital formation | Capital deployment |
| Compression of value | Expansion into goods/services |
| Anode — gravity vector — toward rest | Cathode — magnetism vector — toward force |

An economy that only charges collapses inward (deflation, hoarding, stagnation). An economy that only discharges explodes outward (hyperinflation, resource depletion). A healthy economy *breathes* — conjugate charge/discharge cycles.

```rust
struct MarketField {
    // Every market is a conjugate pair
    supply_pressure: f64,      // centripetal — goods flowing IN (charging)
    demand_pressure: f64,      // centrifugal — goods flowing OUT (discharging)
    
    // Price emerges from the interference pattern between conjugates
    fn equilibrium_price(&self) -> f64 {
        // Price is NOT set — it EMERGES from pressure mediation
        // Like gravity: not a force, but resolution toward lowest-pressure null
        self.demand_pressure / self.supply_pressure.max(AGNOSIS) // prevent div by zero
    }
    
    // Market health = balance of conjugate pair
    fn health(&self) -> f64 {
        let ratio = self.supply_pressure / self.demand_pressure.max(AGNOSIS);
        // Perfect health at ratio = 1.0 (balanced conjugate)
        // Uses golden ratio as natural tolerance band
        if ratio > PHI_INV && ratio < PHI { 1.0 }       // healthy band
        else if ratio > AGNOSIS && ratio < PHI_CUBE { 0.5 } // stressed
        else { 0.0 }                                      // crisis
    }
}
```

#### 16.5.2 Economic Gravity as Anti-Field

Wheeler's critical insight: gravity is NOT a force pulling things together. It is the *absence of force* — an anti-field, a pressure differential that resolves toward the lowest-pressure null point. Objects don't accelerate toward each other; they accelerate toward the lowest null pressure point *between* them.

**For SYNTHESIS economics:** Agents don't "attract" trade. Trade fills voids. Settlements don't pull resources inward through some force — rather, economic pressure differentials resolve toward null points. When Settlement A has surplus grain and Settlement B has deficit grain, trade doesn't happen because B "pulls" — it happens because the pressure differential between surplus and deficit resolves naturally toward equilibrium, like water seeking its level.

```rust
fn calculate_trade_pressure(from: &Settlement, to: &Settlement, good: Good) -> f64 {
    let surplus = from.inventory_of(good) as f64 - from.demand_for(good) as f64;
    let deficit = to.demand_for(good) as f64 - to.inventory_of(good) as f64;
    
    if surplus <= 0.0 || deficit <= 0.0 { return 0.0; }
    
    let distance = hex_distance(from.position, to.position) as f64;
    
    // Trade pressure = mutual acceleration toward null between two mass-points
    // NOT attraction — resolution of Aether-rarefaction tension
    // Pressure falls off with distance (inverse, not inverse-square — 
    // trade is 2D surface phenomenon on the hex grid)
    (surplus * deficit).sqrt() / distance.max(1.0)
}
```

#### 16.5.3 The Three Energy Barriers (Phase Transitions)

From Wheeler's Cosmic Unity, three energy barriers define phase transitions. Applied to the economy:

**C< (Under-barrier) — Propagating energy: INFORMATION**
Things that travel across the map without physical mass: rumors, prices, cultural memes, religious beliefs, market intelligence, reputation. Propagation speed depends on medium (trade routes = fast, wilderness = slow).

```rust
struct Information {
    content: InfoType,       // Rumor, PriceData, CulturalMeme, etc.
    origin: HexCoord,
    strength: f64,           // Decays with distance (inverse of Phi)
    tick_created: u64,
}

// Information propagates at C_LIGHT speed through trade routes,
// slower through wilderness. Decays by PHI_INV per hop.
fn propagate_information(world: &mut World, info: &Information) {
    for neighbor in world.connected_hexes(info.origin) {
        let decay = if world.has_trade_route(info.origin, neighbor) {
            PHI_INV           // faster decay = further reach on trade routes
        } else {
            PSYCHE            // slower through wilderness
        };
        let new_strength = info.strength * decay;
        if new_strength > AGNOSIS {  // below agnosis threshold = lost to noise
            world.inject_info(neighbor, info.with_strength(new_strength));
        }
    }
}
```

**C> (Over-barrier) — Non-propagating matter: PHYSICAL GOODS**
Things that persist in place without travelling: buildings, stored goods, infrastructure, agents themselves. These are the "matter" of the economy — they don't propagate, they accumulate.

**M> (Over-mass barrier) — Collapsed super-mass: SETTLEMENT COLLAPSE**
When a settlement grows beyond magnetism's ability to hold coherence — too many agents, too much wealth concentrated, governance capacity exceeded — it hits the M> barrier. Like a black hole forming from over-mass, the settlement collapses and ejects a "galactic jet" of refugees, resources, and cultural fragments that seed new settlements elsewhere.

```rust
fn check_settlement_overmass(settlement: &Settlement) -> bool {
    let governance_capacity = settlement.governance_score * PHI_CUBE;
    let actual_load = settlement.population as f64 + 
                      (settlement.total_wealth as f64 * AGNOSIS);
    
    // When load exceeds capacity by Phi ratio, collapse begins
    actual_load > governance_capacity * PHI
}

fn settlement_collapse(world: &mut World, settlement_id: SettlementId) {
    let settlement = &world.settlements[settlement_id];
    
    // "Galactic jet" diaspora — refugees scatter carrying cultural fragments
    let refugees: Vec<AgentId> = settlement.agents
        .iter()
        .filter(|a| rand::random::<f64>() > PSYCHE) // ~62% flee
        .cloned()
        .collect();
    
    for agent_id in &refugees {
        let agent = &mut world.agents[*agent_id];
        // Refugees carry partial cultural memory of collapsed settlement
        agent.soul.wisdom_score += settlement.cultural_memory * AGNOSIS;
        // Scatter in golden-angle directions from collapse point
        let angle = GOLDEN_ANGLE * (rand::random::<f64>() * PENTAD);
        agent.destination = Some(hex_at_angle(settlement.position, angle, 
                                              rand::range(3..15)));
    }
    
    // Settlement doesn't fully die — shrinks to core
    settlement.population = (settlement.population as f64 * AGNOSIS) as u32;
    
    // "Beginning begets end which generates another new beginning"
    // Refugees found new settlements at diaspora landing points
}
```

### 16.6 Holographic Self-Similarity Across Scales

The same conjugate pattern repeats at every scale of the simulation. This is not a metaphor — it is the literal architecture. The same field-pressure logic, the same torus/hyperboloid dynamics, the same charge/discharge breathing appears at:

| Scale | Dielectric (Inner/Rest) | Magnetic (Outer/Force) | Gravity (Anti-field/Null) |
|-------|------------------------|----------------------|-------------------------|
| **Agent** | Citta coherence, inner identity | Observable behavior, trade, social acts | Sleep, rest, saving — return toward rest |
| **Settlement** | Core market, stored wealth, institutions | Trade radius, cultural influence, military reach | Economic "gravity" drawing trade — actually a pressure null |
| **World Economy** | Currency base, total resource reserves | Money velocity, trade volume, information flow | World equilibrium — sum of all pressure nulls |

```rust
// The same trait implemented at every scale
trait ConjugateField {
    fn charging_pressure(&self) -> f64;    // centripetal, accumulation
    fn discharging_pressure(&self) -> f64; // centrifugal, expenditure
    fn null_point(&self) -> f64 {          // gravity = anti-field
        // The "center of gravity" is literally no-thing — a coordinate 
        // where nothing is, toward which all pressures resolve
        (self.charging_pressure() - self.discharging_pressure()).abs()
    }
    fn health_ratio(&self) -> f64 {
        let ratio = self.charging_pressure() / self.discharging_pressure().max(AGNOSIS);
        // Healthy when ratio falls in golden band: PHI_INV to PHI
        if ratio >= PHI_INV && ratio <= PHI { 1.0 }
        else { 1.0 - ((ratio - 1.0).abs() / PHI_CUBE).min(1.0) }
    }
}

// Implemented for agents
impl ConjugateField for Agent {
    fn charging_pressure(&self) -> f64 { self.savings_rate() + self.skill_growth() }
    fn discharging_pressure(&self) -> f64 { self.consumption_rate() + self.social_spending() }
}

// Same interface for settlements
impl ConjugateField for Settlement {
    fn charging_pressure(&self) -> f64 { self.production_rate() + self.tax_revenue() }
    fn discharging_pressure(&self) -> f64 { self.consumption_rate() + self.trade_outflow() }
}

// Same interface for world economy
impl ConjugateField for WorldEconomy {
    fn charging_pressure(&self) -> f64 { self.total_production() + self.total_savings() }
    fn discharging_pressure(&self) -> f64 { self.total_consumption() + self.total_trade() }
}
```

### 16.7 Constructive-Destructive Interference — Emergent Complexity

True holography is generated by the *interference pattern* between conjugate beams — reference beam and object beam, mutually accenting AND destroying in harmonic ratios. Complexity and beauty emerge from this dance, not from added rules.

**For SYNTHESIS:** Emergent phenomena (market cycles, political revolutions, cultural renaissances) should NOT be hard-coded. They arise from the constructive-destructive interference between conjugate dynamics:

- **Market booms** = constructive interference between optimism (information propagation) and real production (material accumulation). When both charge in phase, the combined amplitude exceeds either alone (multiplicative, not additive).
- **Market busts** = destructive interference when optimism and reality fall out of phase. The chasm between expectation and reality IS the crash.
- **Cultural golden ages** = constructive interference between high settlement coherence, surplus resources, and concentrated wisdom agents.
- **Societal collapse** = cascading destructive interference when multiple conjugate pairs fall out of phase simultaneously.

**Unification is multiplicative, not additive:** "The whole is greater than the sum of the parts." When agents cooperate in a settlement, output is multiplicatively greater than the sum of individual outputs. A settlement of 10 agents doesn't produce 10× a solo agent — the cooperative multiplier should follow Φ ratios:

```rust
fn cooperative_multiplier(agent_count: u32, avg_coherence: f64) -> f64 {
    // Base: logarithmic scaling (diminishing returns)
    let base = (agent_count as f64).ln() * PHI;
    // Coherence bonus: high-coherence groups get multiplicative bonus
    // (laser principle — concentrated coherence amplifies power)
    let coherence_bonus = avg_coherence.powf(PHI);
    base * (1.0 + coherence_bonus)
}
```

### 16.8 Information Contagion — Beliefs vs. Wisdom

Wheeler's epistemology provides two distinct propagation models:

**Beliefs/lies/fads** spread like parasites — centrifugally, fast, low-cost, seeking to replicate and assimilate. "Anything that tries to duplicate, spread & assimilate is always the opposite of facts & wisdom." Viral propagation model.

**Wisdom/truth** propagates slowly, point-source, through direct transmission (master-apprentice, lived experience). "Nothing true is popular, & nothing popular is true."

```rust
enum InformationKind {
    /// Viral propagation: fast, broad, decays slowly, low threshold to accept
    Belief { virality: f64, content: String },
    /// Point-source propagation: slow, narrow, requires direct contact, 
    /// high threshold but permanent once accepted
    Wisdom { depth: f64, content: String },
}

fn propagation_speed(kind: &InformationKind) -> f64 {
    match kind {
        InformationKind::Belief { virality, .. } => virality * PHI_CUBE,
        InformationKind::Wisdom { .. } => AGNOSIS, // slow — inverse of viral
    }
}

fn acceptance_threshold(agent: &Agent, kind: &InformationKind) -> f64 {
    match kind {
        // Low-coherence agents accept beliefs easily
        InformationKind::Belief { .. } => 1.0 - agent.soul.citta_coherence,
        // High-coherence agents can receive wisdom; low-coherence cannot
        InformationKind::Wisdom { .. } => agent.soul.citta_coherence,
    }
}
```

**The contrarian advantage:** Agents who act against popular consensus (high coherence, Transcendentalist class) should have long-term economic advantage but short-term social cost. The crowd follows beliefs; the wise follow pressure-null points that the crowd cannot see.

### 16.9 Evil as Privation — Corruption and Decay Mechanics

Evil/corruption in SYNTHESIS is NEVER a positive force. It is always a *privation* — the absence of the Good (governance, wisdom, abundance, coherence). You don't code evil; you code the conditions under which ordering forces weaken, and disorder fills the vacuum.

**Primary evil mechanics:**
- **Excess** (>5, beyond the pentad): Agents or settlements accumulating beyond need generate instability. Hoarding, monopoly, over-extraction — the M> barrier triggers collapse.
- **Agnosis** (Φ⁻³): Ignorance, not malice, is the root of most "evil" behavior. Most harmful agent actions stem from incomplete information and low coherence, not conscious malice.
- **Parasitic feeding**: Low-coherence agents under stress (famine, war, loss) become sources of instability that propagate outward — a negative pressure field that attracts predatory behavior.

```rust
fn corruption_score(settlement: &Settlement) -> f64 {
    // Corruption = ABSENCE of good governance, not a positive force
    let governance_deficit = 1.0 - settlement.governance_score;
    let wisdom_deficit = 1.0 - settlement.avg_agent_coherence();
    let excess = (settlement.gini_coefficient() - PHI_INV).max(0.0); // inequality beyond golden ratio
    
    // "At the center of evil there is no evil" — corruption has no autonomous core
    // It's always traceable to a deficit
    (governance_deficit * PSYCHE + wisdom_deficit * PSYCHE + excess * AGNOSIS)
        .min(1.0)
}
```

**Evil agent distribution (from the Codex on Evil):**
- Most "bad" behavior = **agnosis** (ignorance, poor decisions from incomplete information). Not truly evil.
- Some agents participate in exploitative systems because profitable = **secondary evil** (participation without love of harm).
- Extremely rare agents actively seek destruction = **primary evil** (embracing Self-unlikeness, feeding on agitation).

This creates a realistic distribution: ~85% ignorant mistakes, ~14% opportunistic exploitation, ~1% genuine malice.

### 16.10 The Fibonacci Trinity and Sacred Numbers

The first five digits of the Fibonacci sequence (1, 1, 2, 3, 5) encode the simulation's structural framework:

| Number | Wheeler Meaning | Simulation Mapping |
|--------|----------------|-------------------|
| **1** | Principle (the One, the Absolute) | Tick loop kernel |
| **1** | Attribute (Ananke, the One's extrinsic face) | LLM/Nous layer |
| **2** | Matter (force, the material substrate) | Resource/goods system |
| **3** | Magnitude (volume, spatial extent) | Hex grid, three spatial dimensions of toroidal field |
| **5** | Being/Life (pentad, completion) | Agents — the living union of all above |
| **4** | Time (ABSENT from sequence — doesn't truly exist) | Tick counter (measure, not substance) |
| **6** | Excess/Evil (beyond the pentad) | Corruption/collapse trigger threshold |

**Implementation:** The number 5 (pentad) should be a meaningful limit in agent systems — five core needs, five skill categories, five resource tiers. When anything exceeds five, it enters the domain of excess (six = evil). Settlements with more than 5× their governance capacity are in overmass territory. Agents with more than 5× the average wealth are generating instability.

### 16.11 The Three Zones of Economic Activity

Wheeler describes three zones plus a fourth transcendent:

**Zone 1 — Centrifugal Force (Expansion/Frontier):**
Exploration, new trade routes, frontier settlement, speculation, innovation. Agents moving OUTWARD from established centers. High risk, high potential, volatile. This is the domain of Hydrogen-type agents.

**Zone 2 — Centripetal Inertia (Accumulation/Core):**
City cores, warehouses, banks, institutions, stored wealth. Agents moving INWARD toward concentration. Low risk, stable, conservative. Domain of Gold-type agents.

**Zone 3 — Plane of Inertia (Living Rest):**
The lowest-pressure zone BETWEEN centrifugal expansion and centripetal contraction. Where most life exists. Where settlements form and markets operate. The "habitable zone" of economic activity. This is where the 85° angle of life manifests — the sweet spot between expansion and contraction.

**Zone 4 — The Nexus (Transcendence):**
The center/inversion of all three zones. Without measure. In the simulation: the tick loop itself, the unobservable kernel. The place from which all emanates and to which all returns. Not a location on the hex grid — it's the process that computes the hex grid.

```rust
fn classify_economic_zone(hex: &Hex, world: &World) -> EconomicZone {
    let nearest_settlement = world.nearest_settlement(hex.position);
    let distance = hex_distance(hex.position, nearest_settlement.position);
    let frontier_radius = nearest_settlement.trade_radius() * PHI;
    
    if distance > frontier_radius {
        EconomicZone::Centrifugal  // Zone 1: frontier/expansion
    } else if distance < nearest_settlement.core_radius() {
        EconomicZone::Centripetal  // Zone 2: core/accumulation
    } else {
        EconomicZone::PlaneOfInertia  // Zone 3: habitable zone
    }
}
```

### 16.12 Cosmic Recycling — The Eternal Return

"Beginning begets end which generates another new beginning." Nothing is lost — everything transforms. The simulation must embody this principle at every scale:

- **Agent death** → knowledge/wealth partially returns to commons → seeds next generation
- **Settlement collapse** → refugee diaspora → new settlement founding (galactic jets)
- **Economic bust** → resources freed from failed enterprises → new innovation cycle
- **Faction dissolution** → cultural fragments scatter → new movements synthesize from fragments

**No straight lines — all dynamics are curvilinear.** Economic growth curves, population dynamics, trade patterns — none should be linear. Everything curves back toward rest, toward the Aether/null-point, following the toroidal geometry of force.

```rust
// Population follows logistic curve modified by Phi ratios
fn population_growth(current: u32, capacity: u32) -> f64 {
    let ratio = current as f64 / capacity as f64;
    // Growth rate peaks at PHI_INV of capacity, not 0.5
    // This matches Wheeler's golden ratio as the natural balance point
    let growth = PHI * ratio * (1.0 - ratio / PHI_INV);
    growth.max(-AGNOSIS) // can't shrink faster than entropy constant
}
```

### 16.13 The Golden Ratio as Universal Calibration Constant

Rather than arbitrary tuning values scattered through the codebase, SYNTHESIS derives its constants from Φ:

| Parameter | Derivation | Value | Meaning |
|-----------|-----------|-------|---------|
| Agent decision error rate | Φ⁻³ | 0.23606 | ~24% of decisions deviate from optimal — agnosis constant |
| Resource decay per tick | Φ⁻³ | 0.23606 | ~24% of perishables degrade per season |
| Market price noise band | ±Φ⁻³ | ±0.23606 | Prices fluctuate within ~24% of equilibrium |
| Healthy wealth inequality (Gini) | Φ⁻¹ | 0.618 | Golden ratio Gini — balanced but not equal |
| Settlement growth angle | 137.5° | — | Optimal expansion direction for new districts/trade routes |
| Knowledge transfer efficiency | Φ⁻² | 0.382 | ~38% of a teacher's knowledge transfers to student |
| Cultural memory persistence | Φ⁻¹ | 0.618 | ~62% of culture survives generational transfer |
| Cooperative production bonus | Φ | 1.618 | Groups produce Φ× what individuals would |
| Overmass collapse threshold | Φ³ | 4.236 | Settlement collapses when load exceeds capacity by 4.24× |
| Diaspora scatter fraction | 1-Φ⁻² | 0.618 | ~62% of population flees during collapse |

### 16.14 The Triad of Non-Things

Space, shadow, and gravity are identical in nature — they are *absences*, not things. For SYNTHESIS:

- **Unexplored hex** = not "empty space" but absence of agent observation. It doesn't "contain nothing" — it's simply unresolved potential (Aether).
- **Economic shadow** = unmet demand, untapped resources, latent opportunity. The chasm where no trade occurs IS the signal for where trade should flow.
- **Economic gravity** = not a force pulling agents toward markets, but the absence of resistance along trade routes. Agents don't "seek wealth" — they flow along paths of least pressure, and wealth accumulates at null-points.

### 16.15 Summary of Wheeler Constants for Implementation

```rust
/// All simulation constants derived from the Phi emanation series.
/// No arbitrary magic numbers — everything traces back to the golden ratio.
pub struct EmanationConstants {
    // === The Phi Series (Φ⁻³ through Φ³) ===
    pub agnosis: f64,          // 0.23606 — entropy, error, privation, noise
    pub psyche: f64,           // 0.38197 — soul base, coherence seed, transfer rate
    pub matter: f64,           // 0.61803 — material ratio, decay, mortality
    pub monad: f64,            // 1.00000 — unity, the One, baseline
    pub being: f64,            // 1.61803 — life, cooperation bonus, growth factor
    pub nous: f64,             // 2.61803 — intelligence multiplier, wisdom threshold
    pub totality: f64,         // 4.23606 — completion, overmass threshold, max ratio
    
    // === The Three Golden Angles ===
    pub life_angle: f64,       // 85.0° — habitable zone, plane of inertia
    pub manifestation: f64,    // 108.0° — market cycle period base, structural angle
    pub growth_angle: f64,     // 137.5° — optimal expansion, phyllotaxis
    
    // === The Pentad and Excess ===
    pub completion: f64,       // 5.0 — max healthy categories/tiers/accumulation
    pub excess: f64,           // 6.0 — threshold beyond which corruption/evil begins
}

impl Default for EmanationConstants {
    fn default() -> Self {
        let phi = 1.6180339887498948_f64;
        Self {
            agnosis: phi.powi(-3),
            psyche: phi.powi(-2),
            matter: phi.powi(-1),
            monad: 1.0,
            being: phi,
            nous: phi.powi(2),
            totality: phi.powi(3),
            life_angle: 85.0,
            manifestation: 108.0,
            growth_angle: 137.5077,
            completion: 5.0,
            excess: 6.0,
        }
    }
}
```

### 16.16 Source Corpus Reference

| # | Document | Key Contributions to Architecture |
|---|----------|----------------------------------|
| 1 | Magneto-Dielectric Atom Model | Four-variable classification (Mass × Gauss), cosmic recycling, all matter as compounded hydrogen |
| 2 | Codex of Gravity | 18 axioms, gravity as anti-field/pressure mediation, mutual acceleration to torsion-null |
| 3 | Cosmic Unity | Three energy barriers (C<, C>, M>), torus/hyperboloid geometries, three energy classes |
| 4 | Theurgy of Liberation | Three Muses (Aoide/Melete/Mneme), via negativa, laser/lightbulb coherence, radio analogy, three states of being |
| 5 | Energy, Fields & Medium | Precise definitions lexicon, triad of non-things (space/shadow/gravity), field = Aether perturbation |
| 6 | Codex Universalis | Master codex, Φ secret ratio, Fibonacci trinity, charge/discharge conjugates, four classes of humans |
| 7 | Holographic Cosmos of Projection | Constructive-destructive interference as complexity engine, multiplicative (not additive) unification, inseparability of conjugate pairs, water as logos |
| 8 | Laws of Fools & Lies | Belief contagion vs. wisdom propagation, contrarian advantage, social dynamics of low-coherence populations |
| 9 | Codex on Evil | Evil as privation (not autonomous force), parasitic dynamics, excess as corruption seed, primary/secondary evil taxonomy |
| 10 | Auream Veritatem Universalem | Two forms of sentience, divine growth angle (137.5°), action vs. contemplation lives, magnetism:consciousness as dielectric:spirit |
| 11 | Water Molecule Geometry | Complete Φ emanation series (Φ⁻³ through Φ³), three golden angles (85°/108°/137.5°), mathematical derivation of all constants, Pythagorean triangle as life's geometry |
| 12 | Sentient World Simulation (Purdue/JFCOM) | Fractal architecture validation, Society of Models integration pattern, excursion management, continuous calibration, PMESII framework, agent DNA/memory model |
