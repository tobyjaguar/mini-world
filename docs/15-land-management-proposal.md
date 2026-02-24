# Land, Resources, and Carrying Capacity in Synthetic Worlds
## A Research Direction for SYNTHESIS / Crossworlds

*February 2026*

---

## The Problem as It Stands

Crossworlds has a fixed hex grid (~2,000 hexes) with resources that deplete and regenerate. The regen rate has been tuned twice (4.7% → 9.4% per season, plus weekly micro-regen at 4.7%) as band-aids for farmer/fisher satisfaction crashes. The current system is a simple flow: agents extract, hexes deplete, hexes slowly recover. There is no concept of land quality degradation, investment in land improvement, crop rotation, carrying capacity, or the commons problem. The result is that resource management creates suffering without creating *decisions*.

The fundamental question: **How do you make land constraints produce emergent decisions rather than just emergent suffering?**

---

## Research Lineage 1: Elinor Ostrom — Governing the Commons

**Core work:** *Governing the Commons: The Evolution of Institutions for Collective Action* (Cambridge, 1990). Nobel Prize in Economics, 2009.

**Why it matters for SYNTHESIS:** Ostrom empirically disproved the "Tragedy of the Commons" by documenting 800+ real-world cases where communities successfully self-governed shared resources for centuries — Swiss alpine pastures, Japanese mountain commons, Philippine irrigation systems, Turkish fisheries. She identified **eight design principles** that distinguish successful commons from failed ones.

**The Eight Principles (adapted for SYNTHESIS):**

1. **Clearly defined boundaries.** Who can harvest from which hexes? Right now, any agent can extract from any hex. Ostrom found that successful commons always define who belongs and who doesn't.
   - *Implementation idea:* Settlement hex claims. A settlement "owns" adjacent hexes and can regulate access.

2. **Congruence between rules and local conditions.** Extraction rules should match the specific ecology of each hex.
   - *Implementation idea:* Per-terrain-type regen curves and extraction limits, not a flat global rate.

3. **Collective-choice arrangements.** Those affected by the rules participate in modifying them.
   - *Implementation idea:* Settlement governance decides extraction rates. Democracies might vote for conservation; autocracies might strip-mine for short-term gain.

4. **Monitoring.** Someone watches whether rules are being followed.
   - *Implementation idea:* Settlement "wardens" who enforce extraction limits. Corruption means less monitoring, more over-extraction.

5. **Graduated sanctions.** Rule-breakers face escalating penalties, not instant banishment.
   - *Implementation idea:* Agents who over-extract face fines, reputation hits, eventual exile.

6. **Conflict resolution mechanisms.** Disputes over resources need accessible resolution.
   - *Implementation idea:* When two settlements claim the same hex, a dispute resolution mechanic determines outcome.

7. **Minimal recognition of rights to organize.** External authorities don't undermine local governance.
   - *Implementation idea:* Factions shouldn't override settlement resource decisions without political consequences.

8. **Nested enterprises.** For larger systems, governance is layered.
   - *Implementation idea:* Individual hex management nests within settlement policy, which nests within faction ideology.

**Key reading:**
- Ostrom, E. (1990). *Governing the Commons*. Cambridge University Press.
- Ostrom, E. (2005). *Understanding Institutional Diversity*. Princeton University Press.
- Ostrom, E., Walker, J., Gardner, R. (1994). *Rules, Games, and Common-Pool Resources*. University of Michigan Press.

---

## Research Lineage 2: Castronova & Lehdonvirta — Virtual Economy Design

**Core work:** *Virtual Economies: Design and Analysis* (MIT Press, 2014). Also: *Synthetic Worlds* (University of Chicago Press, 2005).

**Why it matters:** Castronova established that synthetic world economies follow real economic principles. His key insight for resource management: **scarcity is fundamental, but scarcity must create choices, not just suffering.**

**Key principles for SYNTHESIS land management:**

- **Sinks and faucets must balance, but the balance should shift.** Static balance creates flat-line. The interesting world has oscillating resource availability.
- **Multiple interlinked resource types create strategic depth.** The question is whether land management creates meaningful trade-offs between resource types.
- **The economy regulates power.** Resource-rich hexes should be politically contested.
- **Infrastructure investment should be the primary growth mechanism.** Settlements should invest treasury in land improvement — irrigation, roads, mines, conservation.

**Key reading:**
- Lehdonvirta, V. & Castronova, E. (2014). *Virtual Economies: Design and Analysis*. MIT Press.
- Castronova, E. (2005). *Synthetic Worlds*. University of Chicago Press.

---

## Research Lineage 3: Agent-Based Land-Use Modeling (LUCC)

**Core work:** Parker et al. (2003). "Multi-Agent Systems for the Simulation of Land-Use and Land-Cover Change: A Review." *Annals of the AAG.*

**Key concepts for SYNTHESIS:**

- **Land has state, not just quantity.** A hex should have soil fertility, degradation level, improvement level, and ecological health.
- **Heterogeneous agents make different land decisions.** Agent coherence should directly affect land-use strategy.
- **Spatial heterogeneity is the driver.** The *differences* between hexes create the interesting dynamics.
- **Technology adoption changes the game.** When agents can adopt new practices, carrying capacity increases.

**Key reading:**
- Parker, D.C. et al. (2003). "Multi-Agent Systems for the Simulation of Land-Use and Land-Cover Change." *Annals of the AAG*, 93(2), 314-337.
- Berger, T. (2001). "Agent-based spatial models applied to agriculture." *Simulation Modelling Practice and Theory*, 9(6-8), 489-501.

---

## Research Lineage 4: Tragedy of the Commons — ABM Approaches

**The Wheeler-Ostrom synthesis for SYNTHESIS:**

- **Low-coherence agents over-extract.** They maximize short-term gain, ignore sustainability, free-ride on shared resources.
- **High-coherence agents conserve.** They factor in long-term consequences, value sustainability.
- **The tragedy IS the interesting dynamic.** Don't prevent the tragedy — let it happen in low-coherence settlements while high-coherence settlements thrive.
- **Graduated sanctions and monitoring = governance coherence.** Ostrom's principles 4 and 5 map to settlement governance quality, which maps to average agent coherence.

**Key reading:**
- Schindler, J. (2012). "A Simple Agent-Based Model of the Tragedy of the Commons." *ECMS 2012.*
- "Beyond the Tragedy of the Commons: Building a Reputation System for Generative Multi-agent Systems." arXiv:2505.05029 (2025).

---

## Research Lineage 5: Ecological Economics — Carrying Capacity

Every environment has a **carrying capacity** — the maximum population it can sustain given resource regeneration rates, technology, and management practices.

**The Φ-derived carrying capacity model:**

```
hex_capacity = base_yield(terrain) × (1 + improvement_level) × regen_health
settlement_capacity = sum(claimed_hex_capacities) × cooperative_multiplier(pop, coherence)
population_pressure = population / settlement_capacity

When pressure < Φ⁻¹ (0.618): abundance — growth, surplus, trade export
When pressure ≈ 1.0: equilibrium — stable, needs met, no surplus
When pressure > Φ (1.618): scarcity — rationing, migration pressure, conflict
When pressure > Φ³ (4.236): collapse — famine, diaspora
```

**Key reading:**
- Cohen, J.E. (1995). "Population Growth and Earth's Human Carrying Capacity." *Science*, 269(5222), 341-346.
- Arrow, K. et al. (1995). "Economic Growth, Carrying Capacity, and the Environment." *Science*, 268, 520-521.

---

## The Wheeler Integration: Land as Mirror of Coherence

**Land = Matter (Hyle) = furthest from the One.** Land is the material substrate, subject to decay. It doesn't have agency — it reflects the quality of the agents acting upon it.

**The coherence-land feedback loop:**

```
High settlement coherence → sustainable extraction → hex health improves →
    more resources → prosperity → coherence maintained (virtuous cycle)

Low settlement coherence → over-extraction → hex degrades →
    scarcity → suffering → coherence drops further (vicious cycle)
```

**The via negativa of land management:** The best land policy isn't the one that adds the most rules — it's the one that removes the most obstacles to natural regeneration. Hexes want to regenerate (rest seeks rest). The question is whether agents let them.

---

## Implementation Phases

### Phase A: Hex Health Foundation (implemented)

1. **Hex health field** — single float per hex (0.0–1.0) that degrades with extraction and recovers with rest.
2. **Health-scaled regen** — replaces flat regen rate with health-proportional regen. Creates positive feedback loop.
3. **Desertification threshold** — below Agnosis (0.236), no regen occurs. Stakes for mismanagement.
4. **Fallow recovery** — un-extracted hexes regain health weekly. Rest seeks rest.
5. **Carrying capacity metric** — settlement population / hex capacity ratio. Makes pressure visible.
6. **Hex health persistence** — survives restarts via world_meta.
7. **API exposure** — health visible in hex detail, bulk map, and settlement detail endpoints.

### Phase B: Land Governance (pending observation of Phase A)

1. **Settlement hex claims** — Ostrom principle 1: boundaries and property rights.
2. **Infrastructure investment** — treasury spending on hex improvements. Solves treasury-hoarding problem.
3. **Fallow/rotation as agent behavior** — agents can choose to leave hexes fallow. Creates temporal strategy.
4. **Coherence-based extraction policy** — governance type + coherence affects land stewardship.
5. **Carrying capacity as strategic metric** — settlements decide between expand vs. intensify.

### Phase C: External Data Integration

Three external data sources exist but are underutilized. Connecting them to the hex health system would make the world responsive to outside information — the land breathes with real weather, real randomness, and LLM-driven intelligence.

**Weather API → Land Health**

Currently: OpenWeatherMap fetched hourly, produces `TempModifier` (unused), `FoodDecayMod` (inventory only), `TravelPenalty` (travel only). Weather description is cosmetic — passed to oracles and newspaper but has no physical effect.

Proposed:
- **Drought stress**: When `TempModifier > 0.5` (hot) and no rain, hex health degradation accelerates by `TempModifier * Agnosis * 0.005` per tick for plains/river terrain. Farmers on drought-stressed hexes see yields drop.
- **Rain recovery**: When `IsRain == true`, fallow recovery rate doubles. The hex `Rainfall` field (set at generation) modulates sensitivity — high-rainfall hexes benefit more from rain, low-rainfall hexes are drought-prone.
- **Storm damage**: When `IsStorm == true`, 1-3 random settlement hexes take a health hit (`-Agnosis * 0.1`). Flooding erodes topsoil.
- **Seasonal weather patterns**: Spring rain → plains/river regen boost. Summer heat → coast/swamp regen boost (algae/herbs). Winter frost → tundra/mountain regen suppressed. Autumn → universal mild boost (decomposition returning nutrients).
- **Weather as signal**: Tier 2 agents and oracles can perceive weather trends and adapt — plant different crops, time extraction, advocate for conservation. Weather becomes information that drives decisions, not just flavor text.

**random.org → Land Events**

Currently: True randomness used only in `processRandomEvents()` for weekly disaster/discovery/alchemy rolls. No connection to land.

Proposed:
- **Regional weather extremes**: True-random drought or flood events affecting 3-5 contiguous hexes. Drought: health drops `Agnosis * 0.2` across the region. Flood: resources wash away but health recovers faster afterward (silt deposit).
- **Blight and plague**: Random crop/animal disease events that degrade specific resource types in an area. Grain blight destroys grain resources on plains hexes near a settlement. Creates urgency to diversify food sources (fish, furs/hunting).
- **Mineral discovery**: Random events reveal hidden resources in degraded hexes — a silver lining for over-extracted land. Incentivizes exploration of damaged territory.
- **Volcanic/tectonic events**: Rare (0.5%/week) events that permanently alter a hex — mountain hex gains gems, coast hex gains fish, but nearby hexes take health damage.

**Oracle Visions → Land Stewardship**

Currently: Oracles receive world context and choose from {trade, advocate, invest, speak, bless}. No action affects the physical world — they're social/economic only. Oracles see weather and Gini but not hex health.

Proposed:
- **New oracle action: "tend"** — Oracle spends the week restoring a degraded hex near their settlement. Direct health boost (`Agnosis * 0.1`). High coherence means the oracle sees what the land needs.
- **"Advocate" affects land policy** — Oracle advocacy influences settlement extraction rate for the week. In Phase B, this maps to governance decisions about conservation vs. exploitation.
- **Hex health in oracle context** — Pass average hex health of settlement's area to the oracle prompt. Liberated agents would perceive degradation as spiritual sickness in the land — "the earth groans under our extraction." Their prophecies would naturally shift toward sustainability themes.
- **"Invest" targets hex improvement** — Oracle directs personal wealth toward hex infrastructure (Phase B prerequisite). Agent wealth → hex improvement level. Makes oracle economic actions physically meaningful.
- **Prophecy as early warning** — Oracles who perceive low hex health generate warning prophecies that become settlement memories. Tier 2 agents in the settlement receive the warning and may shift behavior. This is the LLM intelligence → physical world pipeline.

**The integration thesis:** Currently, external data is cosmetic — weather is flavor text, randomness drives isolated events, and oracle visions are social theater. Connecting all three to hex health creates a feedback triangle: real weather shapes the land → randomness creates crises → oracles perceive and respond → settlements adapt. The world becomes responsive to outside reality while maintaining emergent internal dynamics.

---

## Reading List Summary

### Must-Read (Core Framework)
1. Ostrom, E. (1990). *Governing the Commons.* Cambridge University Press.
2. Lehdonvirta, V. & Castronova, E. (2014). *Virtual Economies: Design and Analysis.* MIT Press.
3. Castronova, E. (2005). *Synthetic Worlds.* University of Chicago Press.

### Should-Read (Technical Depth)
4. Parker et al. (2003). "Multi-Agent Systems for the Simulation of Land-Use and Land-Cover Change." *Annals of the AAG.*
5. Schindler, J. (2012). "A Simple Agent-Based Model of the Tragedy of the Commons." *ECMS 2012.*
6. Schlüter et al. (2017). "A framework for mapping and comparing behavioral theories in models of social-ecological systems." *Ecological Economics.*

### Worth Reading (Extended Context)
7. Hardin, G. (1968). "The Tragedy of the Commons." *Science.*
8. "Beyond the Tragedy of the Commons" (2025). arXiv:2505.05029.
9. Berger, T. (2001). "Agent-based spatial models applied to agriculture."
10. Cohen, J.E. (1995). "Population Growth and Earth's Human Carrying Capacity." *Science.*
