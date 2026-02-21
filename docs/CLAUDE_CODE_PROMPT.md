# SYNTHESIS / WORLDSIM — Claude Code Project Prompt

## What This Project Is

You are building **SYNTHESIS** (also called WORLDSIM / Crossroads) — a **continuously running, persistent synthetic world simulation** populated by autonomous agents who form economies, relationships, factions, and emergent history. The world ticks forward in real time (or accelerated time), and an external observer can query the world via API to receive a "newspaper" of recent events, economic reports, and emergent narratives.

This is NOT a game. It is a **petri dish** — a simulation designed for emergence, observation, and surprise. The goal is a world you want to check in on.

## What Has Been Done

The project has gone through extensive design and theoretical integration. The design document (`worldsim-design.md`) is the **single source of truth** and contains everything you need:

### Sections 1–15: Core Architecture (the "engineering" layer)
These sections define the conventional simulation architecture:
- **Rust-based tick engine** with hex grid world, terrain, resources
- **Tiered agent cognition**: 95% rule-based (Tier 0), 4% archetype-guided (Tier 1), <1% individual LLM-powered (Tier 2, Haiku API)
- **Robust economic system**: scarcity, sinks/faucets, multi-settlement trade, currency, crafting chains, price discovery
- **Social/political systems**: factions, governance, relationships, crime, conflict
- **Technical stack**: Rust core, SQLite persistence, HTTP API (Axum), Haiku API for cognition
- **Five-phase implementation roadmap** from MVP through LLM integration to polish
- **File structure**, crate recommendations, configuration, and API examples

### Section 16: Emanationist Cosmology — The Wheeler Integration (the "soul" layer)
This is what makes SYNTHESIS philosophically distinctive. Section 16 translates Ken Wheeler's emanationist metaphysics (a synthesis of Neoplatonism, Pythagorean number theory, Buddhist psychology, and field theory) into **concrete simulation architecture**. It provides:

- **Ontological stack**: Each simulation layer (tick loop → LLM → hex grid → resources → agents) maps to a Neoplatonic emanation hierarchy
- **Agent coherence model**: Replaces generic personality traits with a "laser vs. light bulb" coherence system based on the via negativa (agents gain power through subtraction of attachments, not accumulation)
- **Four-variable agent classification**: Agents classified on Mass × Gauss axes (Helium/Hydrogen/Gold/Uranium types) replacing arbitrary randomization
- **Economic field theory**: Markets modeled as conjugate charge/discharge pairs. Economic "gravity" is an anti-field (pressure-null resolution), not an attractive force
- **Phase transition mechanics**: Three energy barriers — information propagates (C<), matter persists (C>), settlements collapse when overmassed (M>)
- **Information contagion**: Dual propagation — beliefs spread virally (fast, broad, parasitic), wisdom spreads point-source (slow, narrow, permanent)
- **Corruption as privation**: Evil/corruption is never a positive force — it's the absence of governance, wisdom, and coherence. You don't code corruption; you code the withdrawal of ordering forces.
- **Complete Φ-derived constants**: ALL simulation tuning values (error rates, decay, growth, thresholds) derived from the golden ratio rather than arbitrary magic numbers. The `EmanationConstants` struct centralizes these.
- **Holographic self-similarity**: The same `ConjugateField` trait is implemented identically at agent, settlement, and world-economy scales

## How to Use This

### Read the design document first
`worldsim-design.md` is ~1,500 lines. Read it. Sections 1–15 give you architecture. Section 16 gives you the deeper logic. Together they form a complete blueprint.

### Implementation philosophy
1. **Start with Sections 1–15** for the engineering foundation. Phase 1 (MVP) should work with pure rule-based agents, basic economy, hex grid, and tick engine.
2. **Integrate Section 16 progressively**. The Φ constants can be added immediately (they're just numbers). The coherence model replaces personality during Phase 3. The economic field theory enriches the economy in Phase 2. The information contagion and corruption mechanics come in Phase 3–4.
3. **The Wheeler layer is not decoration** — it provides the mathematical backbone that prevents arbitrary tuning. When you need a decay rate, a threshold, or a balance ratio, derive it from Φ (Section 16.15) instead of guessing.

### Key architectural decisions already made
- **Rust** for the core engine (performance, safety, concurrency)
- **Hex grid** (~2,000 hexes, continental shape)
- **SQLite** for persistence (simple, embedded, sufficient for single-server)
- **Axum** for HTTP API
- **Haiku API** for LLM cognition (rate-limited, tiered — not every agent, not every tick)
- **Emergence over scripting** — never hard-code storylines or outcomes
- **Economic robustness as heartbeat** — if the economy works, everything else follows

### Critical Rust structs to implement early
From the design doc, the core data structures are:
- `Agent` (Section 4.1) with `AgentSoul` (Section 16.2)
- `Settlement`, `Hex`, `Market`, `Faction`
- `EmanationConstants` (Section 16.15) — define this first, reference it everywhere
- `ConjugateField` trait (Section 16.6) — implement for Agent, Settlement, WorldEconomy

### What "done" looks like
A working SYNTHESIS produces a world where:
- Agents are born, work, trade, form relationships, age, and die
- Settlements grow, compete, sometimes collapse and scatter refugees who found new ones
- Markets discover prices through supply/demand pressure mediation
- Information (rumors, prices, beliefs) propagates across the map at varying speeds
- High-coherence agents emerge rarely and disproportionately shape history
- An observer can query the API and receive a "newspaper" that reads like a living world
- The whole thing runs 24/7 without human intervention and doesn't collapse or stagnate

## File Provided
- `worldsim-design.md` — The complete design specification (v2.0, ~1,500 lines, 16 sections)
