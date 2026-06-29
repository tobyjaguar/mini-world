# Health Report — 2026-06-29 — Equilibrium at the Cap

**Tick 8,939,621 — Winter Day 21, Year 25** (+11 days after R98 validation and the
DeepSeek provider-chain deploy)

## Verdict

The world has reached a healthy steady state. Population is parked at the 400K soft
cap (~399,200) with births and deaths balanced at ~30–45/day each — the R97-2 soft
cap's first real boundary test, passed cleanly (no overshoot, no synchronized-cohort
cliff). **Winter produced no death spike** — the W-19/W-22 illness die-off is gone;
winters are now non-events. The reproductive pool grew 15K → **93K** as the boom
cohort came of age. Cognition has run **100% on DeepSeek for 11 days at ~$10/month**.
The only declining numbers (coherence/alignment) are the by-design dilution effect at
their new equilibrium, not pathology.

## Key metrics (vs 2026-06-18)

| Metric | 06-18 | 06-29 | Note |
|---|---|---|---|
| Population | 360,201 | 399,204 | at the 400K cap |
| Net/day | +150–250 | ~0 (±15) | equilibrium |
| Deaths/day | ~20 | ~30–45 | low; **no winter spike** |
| Births/day | ~200 | ~30–45 | soft-cap tapered |
| Repro pool 18-45 | 15,466 | 93,073 | cohort matured |
| Coherence | 0.480 | 0.471 | decel, near floor |
| Liberated % | 12.4% | 10.7% | stabilizing |
| Satisfaction | 0.697 | 0.698 | flat (material life solid) |
| Alignment | 0.300 | 0.256 | Awakening-Valley, by design |
| Market health | 0.917 | 0.903 | healthy |
| Work rate | 0.507 | 0.514 | up |
| Crowns | 1.4069B | 1.4028B | conserved |
| Trade routes | 697 | 736 | up |
| Council gov | 92.2% | 91.7% | stable |
| LLM | DeepSeek (new) | DeepSeek 100% | ~$10/mo, 1 transient 500/24h |

## Why max age tops out at 43 (and is *falling*: 59 → 47 → 43)

The user's hypothesis is correct and the fine-grained data sharpens it. The age tail:

```
age 18-23  : ~93,000  (the boom cohort — the mass)
age 24     : 5     age 30 : 10     age 36 : 3
age 25     : 8     age 31 : 4      age 38 : 1
age 26     : 5     age 32 : 4      age 43 : 1
age 27     : 2     age 33 : 3      (24-29 total = 33)
age 28     : 3     age 34 : 1      (30-39 total = 27)
age 29     : 10    age 35 : 1      (40+    total = 1)
```

So the population is a **single boom cohort whose leading edge is ~23**, with a
near-extinct older tail — only ~60 agents in the entire world are older than 23, out
of 399,000 (0.015%). Max age 43 is literally one agent.

**Mechanism.** Through early 2026 the pre-R98 background mortality (`Agnosis⁴`, ~0.22%/day
from age 16) plus the W-19/W-22 illness die-offs annihilated essentially everyone who
aged past ~20. R98 (deployed 2026-06-09) stopped the killing — but only in time to save
the cohort that was 16-17 at deploy, who are now 18-23. The handful of agents aged 24-43
are the last survivors of the pre-boom generation, and they are dying of old age faster
than the young cohort ages into their range — which is why max age has *fallen* 59 → 47
→ 43 across the last three observes, rather than risen.

**Forward prediction (trackable).** Under R98's survivable mortality the 18-23 mass will
now age *upward* into territory that used to be a death zone. Max age should bottom out
around now (~40-43), then **climb** as the cohort's leading edge passes the dying old
survivors — the first sign the age pyramid is building from the bottom. Full smoothing
into a real pyramid takes ~20-40 sim-years (the demographic echo). Watch max age and the
30-45 bucket: when 30-45 starts filling (from today's 28) and max age rises past 43,
the cohort is successfully aging — the cure is not just stopping death but restoring a
normal lifespan distribution.

## Coherence / alignment are at their by-design floor

Coherence has nearly flattened at ~0.47 (slide decelerated to −0.0008/day), *higher*
than the projector's ~0.26 long-run guess — baseline drift + practice inflows are
balancing newborn dilution at a healthier equilibrium. With most agents in the
Awakening Valley (coherence 0.38-0.7), `ComputeAlignment` suppresses alignment by
design (0.256), which pulls EffectiveMood to 0.517. Satisfaction (material, 0.698) is
the real-economy gauge and is rock-solid. **These low alignment/Liberated numbers are
the new normal — a sentinel `coherence_drift` alert here would be a false positive.**

## LLM provider chain — confirmed on DeepSeek

11 days in: all four cognition paths (biography, tier2, oracle, narration) running on
DeepSeek, ~33 calls/hr, **~$10/month** (vs Anthropic's ~$90/mo run-rate — ~9× cheaper).
One transient DeepSeek 500 in 24h, auto-handled (`consecutive_failures=1`, recovered,
no cooldown since a 500 is not a cap error). Zero cap-driven chain advances. cache
tokens 0 (expected — OpenAI path, no Anthropic caching). **Anthropic auto-resumes
2026-07-01; with `LLM_PROVIDERS=deepseek,anthropic`, DeepSeek stays primary regardless.**

Biography is the dominant tag (~20/hr ≈ 60% of calls). See the cost-reduction plan in
`docs/26-llm-cost-reduction-plan.md`.

## Monitor next session

- **Cap stability:** population should keep hugging ~399K. Sustained net-negative = the
  first hint of an eventual aging-cohort die-off (decades out — the canary).
- **Max age climbing past 43** + the 30-45 bucket filling = the pyramid forming.
- **Coherence holds ~0.47** rather than resuming a slide.
- **DeepSeek cost/reliability:** first full-day token total ≈ ~$10/mo; watch for repeat 500s.
- **2026-07-01 Anthropic reset:** confirm calls stay on DeepSeek (no unexpected revert).
