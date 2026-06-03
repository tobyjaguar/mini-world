# Health Report — Second Winter Clean, Coherence Trough Deepening, LLM Cache Root-Caused

**Date:** 2026-06-02
**Tick:** 6,803,340 · Winter Day 38, Year 19
**Population:** 294,095 · 784 settlements
**Author:** `/observe` + LLM-cache code trace

---

## TL;DR

1. **R97 is weathering a SECOND mechanical winter (season 75) cleanly** — deaths flat ~69/sim-day, no spike. The winter-cull pattern is decisively broken.
2. **Births accelerating** — ~19/sim-day (was ~9 on 06-01, ~1/day a week ago). Net population change down to ~−50/day and closing.
3. **Material economy at record health:** work rate 0.547 (new high), Gini compressing to 0.616, trade volume rising to 505M, wealth conserved at 840M.
4. **Coherence trough deepening, not floored.** Avg coherence 0.653 → **0.627** (now at the Matter line), liberated cohort 151K → **137K** (−14K/day), alignment 0.541 and effective mood 0.659 softening in lockstep. The earlier "floors at Matter (0.618)" estimate looks optimistic — the low-coherence mass is mostly age-gated children, so the average may keep falling below Matter until the cohort matures.
5. **LLM prompt caching root-caused: a no-op, but harmless.** Cached system prompts (~1,150–1,400 tokens) are below Haiku's 2048-token minimum cacheable size, so `cache_control` is silently ignored → cache reads/writes always 0. Uncached cost is already ~$12/mo at ~170 calls/day. Not worth fixing.

---

## 1. Key Metrics

| Metric | 2026-06-01 | 2026-06-02 | Trend |
|---|---|---|---|
| Population | 297,814 | 294,095 | ➡️ gentle decline (−3.7K) |
| Season | Autumn | **Winter** (Day 38) | 🧊 2nd R97 winter, in progress |
| Deaths/sim-day | ~65 | **~69** | ✅ flat in winter, no spike |
| Births/sim-day | ~9 | **~19** | ⬆️ accelerating |
| Avg satisfaction | 0.6970 | 0.6974 | ➡️ flat (material steady) |
| Avg coherence | 0.653 | **0.627** | ⬇️ −0.026 (at Matter 0.618) |
| Avg alignment | 0.5851 | 0.5414 | ⬇️ −0.044 (Awakening valley) |
| Avg mood (effective) | 0.6869 | 0.6587 | ⬇️ tracks coherence |
| Liberated count | 151,359 | **136,884** | ⬇️ −14,475/day |
| Producer work rate | 0.515 | **0.547** | ⬆️ improving |
| Gini | 0.6185 | 0.6163 | ✅ compressing |
| Bottom-50% share | 8.60% | 8.71% | ✅ improving |
| Trade volume | 497M | 505M | ⬆️ rising |
| Trade routes | 818 | 673 | 🔁 churn (Matthew consolidation) |
| Total wealth | 840.3M | 839.9M | ➡️ conserved |
| Avg survival | 0.388 | 0.390 | ➡️ INV-3 equilibrium |

**Factions** (all solvent): Verdant Circle 92,890 (↓ from 98.8K — R62 defection working) · Crown 78,893 · Ashen Path 41,504 · Iron Brotherhood 24,326 · Merchant's Compact 17,771. Unaffiliated adults 537, children 38,174.

---

## 2. Second Winter Validation

Mechanical season 75 (winter) began at tick ~6,750,000; we are ~53K ticks in (Winter Day 38). The death counter advanced linearly at **~69/sim-day** straight through, with survival pinned at the 0.390 INV-3 equilibrium — no illness-band signature, no spike. This is the **second** winter R97 has carried cleanly, confirming the fix is structural, not a one-winter fluke.

Births continue to accelerate (~19/sim-day) as the 16–17 cohort crosses the age-18 reproductive gate. Net population is ~−50/day and closing; replacement is plausibly weeks away.

---

## 3. Coherence Trough — Tracking the Audited Decline

The 2026-06-01 inflow/outflow audit (see prior report) root-caused the coherence decline to **reincarnation feedstock starvation**: `LiberatedSpiritsPool` only refills when a liberated agent dies at age ≥30, but live max age among liberated agents is 17 (zero ≥18). The high-coherence birth pipeline (reincarnated children seeded 0.7–0.85) is switched off, while liberation-death ripples + ordinary low-coherence births + the lost winter-cull survivor-bias drain the liberated remnant.

This session confirms the decline is **persistent and not yet floored**:
- Avg coherence 0.653 → 0.627 (−0.020/day), now at the Matter line.
- Liberated count 151,359 → 136,884 (−14,475/day) — the leading indicator, still falling fast.
- Alignment 0.585 → 0.541 (the average agent has entered the dual-register Awakening-valley dip); effective mood follows.

**Revised floor expectation:** the "asymptotes at ~0.62–0.64" estimate was optimistic. The low-coherence mass is dominated by *children* (38K+ unaffiliated children alone), who are age-gated out of baseline drift (gated `age > 20`) — so there is no drift floor holding them at Matter. The average can continue toward the non-liberated mean (back-of-envelope ~0.43–0.5) until the boom cohort ages enough to practice (contemplation opens at 16) and drift (age 20). Expect coherence to cross **below** Matter shortly.

**Action:** still **watch, do not patch** (project lesson against demographic/coherence mutations). R-round trigger remains "avg coherence sustained below Matter (0.618) for >1 sim-week." It is about to cross; start the sim-week clock if it does. A projector run seeded from the live age × coherence histogram would quantify how deep the trough goes before the maturing cohort reverses it.

---

## 4. LLM Prompt Caching — Root Cause (code trace)

**Trigger:** >24h post the 2026-06-01 Anthropic cap reset, `/api/v1/llm-usage` still shows `cache_read_tokens=0` AND `cache_write_tokens=0` (only tier2 calls observed in-window). The standing memory directive said to investigate if cache reads stayed 0 past 24h.

**Findings (code trace of `internal/llm/`):**
- Wiring is correct: tier2 (`cognition.go:58`) and oracle (`oracle.go:59`) both call `CompleteTaggedCached`, which sets a byte-stable `const` system prompt with `CacheControl: {Type: "ephemeral"}` (`client.go:159–172`). Prompts are identity-agnostic constants — cache-friendly by design.
- **Root cause: prompt too short for Haiku.** Model is `claude-haiku-4-5-20251001`. Anthropic's minimum cacheable prefix for **Haiku is 2048 tokens** (Sonnet/Opus: 1024). Measured sizes:
  - tier2 general system prompt: ~4,760 chars ≈ **~1,150 tokens** → below 2048.
  - oracle system prompt: ~5,678 chars ≈ **~1,400 tokens** → below 2048.
  - Anthropic **silently ignores `cache_control` below the minimum** → `cache_creation_input_tokens=0` and `cache_read_input_tokens=0` on every call. Exact signature observed.
- **Secondary problem (even if padded):** the ephemeral cache TTL is 5 min. tier2 fires ~5 calls across a sim-day (~24 min) and oracle ~10 across a sim-week — most calls land >5 min apart, so each would be a cache *write* (1.25×), not a read (0.1×). Caching would cost *more* than uncached at this cadence.

**Severity: low.** At ~170 Haiku calls/day of small prompts, uncached cost is already ~$12/mo — the ~$13 budget target holds *without* caching ever working. R95's caching is a no-op, but the budget was never at risk.

**Recommendation:** do **not** invest in fixing caching at this volume (would require padding prompts ≥2048 *and* restructuring each tag's batch into a tight <5-min synchronous burst — disproportionate effort for ~$3–4/mo). Instead: update Doc 20 + memory to record that caching is inert on Haiku-with-short-prompts, stop expecting `cache_read > 0`, and treat ~$12/mo uncached as the true baseline. Optionally drop `CompleteTaggedCached` back to `CompleteTagged` to remove the misleading code path.

---

## 5. Things to Monitor Next Session

- **Coherence vs. Matter (0.618)** — primary watch. Does it cross below, and how far toward the non-liberated mean? Track liberated count (137K) as the leading indicator.
- **Births vs. deaths** — births ~19/day climbing toward deaths ~69/day. When does net turn positive?
- **Winter completion** — confirm deaths stay flat through the end of season 75 (no late-winter acceleration).
- **Effective mood inflection** — as average coherence falls, the alignment weight in EffectiveMood shrinks; mood may bottom and then drift *up* toward satisfaction (0.697). Watch for the turn.
- **Reincarnation pool restart** — first liberated agent reaching age 30, or first earned-liberation (contemplation) events from the maturing 16+ cohort.
- **Governance monoculture (W-16)** — still not re-measured (heavy endpoint); last ~92.4%, escalate at 95%.
- **LLM** — closed for now (root-caused). Only revisit if call volume rises materially.
