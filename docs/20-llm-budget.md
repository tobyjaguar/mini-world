# LLM Budget & Usage Architecture

## Design Principle: World-Driven LLM Calls

All LLM calls originate from the simulation's internal processes. No external user action can trigger unbounded LLM usage. The world is insulated — an outsider hitting public API endpoints cannot drive up Haiku costs.

| Call Site | Triggered By | Rate Control | Can External User Trigger? |
|-----------|-------------|--------------|---------------------------|
| Gardener decisions | Timer (every 15 real min) | Fixed interval | No — separate process |
| Tier 2 cognition | Sim tick (weekly per agent) | ~5/sim-day, population-capped | No — tick engine internal |
| Oracle visions | Sim tick (weekly per Liberated) | ~5/sim-week, Liberated count | No — tick engine internal |
| Archetype templates | Sim tick (weekly per archetype) | ~8/sim-week, fixed archetypes | No — tick engine internal |
| Event narration | Sim tick (major events) | 0-5/sim-week, event frequency | No — tick engine internal |
| Newspaper | User request (GET) | **Wall-clock cache (3h)** + rate limiter (30/h) | Capped — at most 8 calls/day |
| Biographies | User request (GET) | Rate limiter (10/h) | Capped — at most 10 calls/hour |

The newspaper and biography endpoints are the only user-facing LLM paths. Both are double-gated:
1. **Rate limiter** — HTTP-level cap on requests per hour
2. **Cache** — newspaper uses wall-clock caching (default 3 real hours); even if the rate limiter allows the request through, a cached response is returned without an LLM call

## Call Budget — Historical & Current

### March 2026 (R31 baseline — STALE)

| Call Site | Calls/Real Day | Input Tok/Call | Output Tok/Call |
|-----------|----------------|----------------|-----------------|
| Gardener | ~96 | ~2,000 | ~200 |
| Tier 2 cognition | ~58 | ~800 | ~150 |
| Newspaper | ~8 | ~1,500 | ~800 |
| Oracle visions | ~3 | ~800 | ~200 |
| Archetype templates | ~5 | ~500 | ~150 |
| Event narration | ~2 | ~400 | ~150 |
| Biographies | 0-10 | ~500 | ~300 |
| **Total** | **~170** | | |

Estimated at the time: ~$0.43/day (~$13/month). **This estimate became stale by 2026-05.** Prompts grew substantially (oracle added LandHealth/Conflicts/TradeLinks/WorkforceData context blocks; tier2 added ResourceAvailability/SkillSummary/OccupationSatisfaction; merchant got TradeContext). World-state growth (29 Liberated Tier 2 oracles vs ~5 in March) compounded.

### May 2026 (pre-R95 — empirical from journalctl hourly summaries)

Measured from May 14 (full active day, 486 calls / 1.196M input tokens / 109.9K output tokens):

| Call Site | Calls/Real Day | Input Tok/Call | Output Tok/Call | $/Call |
|-----------|----------------|----------------|-----------------|--------|
| Tier 2 cognition | ~150 (10/hr) | ~1,600 | ~230 | $0.00275 |
| Oracle visions | ~210 (28/burst × ~8/day) | ~2,860 | ~200 | $0.00386 |
| Archetype templates | ~60 (8/TickWeek × ~8 TickWeeks/day) | small | small | low |
| Event narration | ~40 (5/TickWeek × ~8/day, capped) | small | small | low |
| Newspaper | ≤8 (3hr cache) | ~1,500 | ~800 | $0.0055 |
| Biographies | rare | small | small | negligible |
| **Total** | **~486** | weighted avg ~2,460 | ~226 | weighted **~$0.0036** |

**Empirical cost: ~$1.75/day → ~$52/month.** Spend cap (console-configured at ~$30) tripped 2026-05-15 ~13:16 UTC, suspending API access until 2026-06-01 (W-15 incident).

### Post-R95 architecture (committed 2026-05-16, deploys 2026-05-17)

R95 (mini-world `804925a`) restructures LLM call paths to amortize cost via Anthropic prompt caching + deduplication. Six numbered fixes (#279-284); see ROADMAP.md "Closed: R95" section for the full design.

| Mechanism | Change | Saving |
|---|---|---|
| Prompt caching (#279, #281, #282) | System prompts moved to byte-stable `const` strings, marked `cache_control: ephemeral`. Cache hits bill ~10% of base input rate. Effective on within-batch repeated calls (oracle batch in seconds, tier2 batch in TickDay). | ~30-50% input token cost reduction |
| Per-settlement oracle dedup (#283) | Group oracles by `HomeSettID`; one LLM call per settlement-with-oracles instead of one per oracle. Prophecy memory spreads to all liberated in settlement (existing mechanic). | 28 → 10-12 oracle calls/TickWeek |
| 2-week state-hash gate (#283) | Skip LLM if `(season, treasury bucket, pop bucket, hostile pair count, peace count)` hash matches last week; reuse cached vision; cap reuse at 2 weeks. | Further halves oracle volume → ~5-8 calls/TickWeek |
| Season-gated archetype (#284) | `updateArchetypeTemplates` moved from TickWeek (~20 calls/day) to season-transition (~1 call/season). | -19 calls/day |
| Circuit breaker (#280) | First failure of streak emits `slog.Warn`; every 100th re-warns; success resets. **Not a cost saving — a visibility fix for W-15.** | Prevents next silent outage |

**Expected post-R95 cost: ~$0.70-0.85/day → ~$22-25/month** (target band $25 ± $3).

### Pre-March 2026 (pre-R31)

~330 calls/day, ~$0.80/day (~$24/month). Gardener alone was 73% of all calls (240/day at 6-min intervals). Newspaper regenerated every sim-day (~85 real seconds) for ~1,000 calls/day when actively queried.

## Usage Tracking

All LLM calls go through `internal/llm/client.go`. The client tracks per-tag call counts and token usage.

### Tags

| Tag | File | What |
|-----|------|------|
| `gardener` | `internal/gardener/decide.go` | Gardener intervention decisions |
| `tier2` | `internal/llm/cognition.go` | Tier 2 agent weekly decisions |
| `newspaper` | `internal/llm/newspaper.go` | Newspaper generation |
| `oracle` | `internal/llm/oracle.go` | Oracle visions for Liberated agents |
| `archetype` | `internal/llm/archetypes.go` | Tier 1 archetype template updates |
| `narration` | `internal/llm/narration.go` | Major event narration |
| `biography` | `internal/llm/biography.go` | Agent biography generation |

### Monitoring

- **Hourly log:** The LLM client logs a usage summary every real hour to the worldsim journal:
  ```
  llm usage summary  period=1h  total_calls=42  gardener=6  tier2=28  newspaper=1  ...  input_tokens=84200  output_tokens=12300
  ```
  Counters reset after each hourly log.

- **API endpoint:** `GET /api/v1/llm-usage` returns the current tracking period as JSON:
  ```json
  {
    "period_start": "2026-03-02T14:00:00Z",
    "period_duration": "32m15s",
    "total_calls": 18,
    "total_input_tokens": 31200,
    "total_output_tokens": 4800,
    "by_tag": {
      "gardener": {"calls": 2, "input_tokens": 4000, "output_tokens": 400},
      "tier2": {"calls": 14, "input_tokens": 11200, "output_tokens": 2100},
      ...
    }
  }
  ```

- **Journal inspection:** `journalctl -u worldsim --grep "llm usage summary"` shows hourly totals.

## Environment Variables

| Variable | Default | What |
|----------|---------|------|
| `GARDENER_INTERVAL` | `15` | Gardener cycle interval in real minutes |
| `NEWSPAPER_CACHE_HOURS` | `3` | Newspaper cache duration in real hours |

During active tuning sessions, temporarily lower `GARDENER_INTERVAL` to 6 for faster feedback.

## Global Rate Limit

`internal/llm/client.go` enforces a global rate limit of **20 calls/minute** across all call sites. This is a safety net — at 170 calls/day (~0.12/min average), the world is well below this ceiling. The limit protects against runaway loops or bugs that could spam the API.

## Scaling Considerations

If the world grows significantly, monitor these cost drivers:

- **Tier 2 cognition** scales with Tier 2 population (currently ~30 agents, target 30). If raised, costs scale linearly.
- **Oracle visions** scale with Liberated agent count (currently ~5). Liberated agents are rare by design.
- **Gardener** is fixed-interval, independent of world size.
- **Newspaper** is fixed-rate (wall-clock cache), independent of world size. Content richness increases with more events but token count stays bounded by the prompt structure.

The `/api/v1/llm-usage` endpoint and hourly logs provide early warning if any tag's usage grows unexpectedly.

## History

| Date | Change | Impact |
|------|--------|--------|
| 2026-03-02 | Gardener interval 6→15 min (R31 #152) | -144 calls/day |
| 2026-03-02 | Newspaper cache: sim-day→3 real hours (R31 #153) | -16 calls/day |
| 2026-03-02 | Usage tracking + `/api/v1/llm-usage` (R31 #154) | Visibility |
| 2026-05-15 | Anthropic spend cap tripped → API suspended (W-15) | $0 spend, narrative paused |
| 2026-05-16 | R95 — prompt caching + oracle dedup + state-hash gate + season-gated archetype + circuit breaker | ~$52/mo → ~$22-25/mo + Warn-level failure visibility |

## W-15 Visibility Lesson (2026-05-16)

The May 15 spend-cap event was silent for ~30 wall-hours. Three failures of observability compounded:

1. **All per-call LLM failures logged at `slog.Debug`** — invisible at the default INFO log level. The journal showed `level=INFO msg="llm usage summary" total_calls=0` every hour for 29 hours straight, with no error context.
2. **`processOracleVisions` (engine/cognition.go:459)** logged `count: len(oracles)` — the count of CANDIDATE oracles iterated, not the count of SUCCESSFUL LLM calls. The log line `"oracle visions processed" count=28` continued firing every TickWeek even though every `GenerateOracleVision` call was returning a 400 error.
3. **Worldsim OOM-restart at 04:36 UTC May 16** (separately caused by backup contention) did not recover because the cap is at Anthropic, not at worldsim. The boot log `"LLM client enabled (Haiku)"` was misleading — it only confirmed the API key was non-empty, not that calls would succeed.

**R95 #280 fixes #1**: the client `recordFailure` helper emits `slog.Warn` on the first failure of a streak and every 100th thereafter. A future cap event will be loud.

**Lesson for new LLM call sites**: success-path summary logs MUST count successes, not candidates. The pattern `"X processed", count: len(candidates)` masks failure. Prefer `"X processed", attempted: N, succeeded: M, failed: K`.

## Failure Modes Worth Knowing About

| Symptom | Likely cause | Where it shows up |
|---|---|---|
| All tags drop to 0 in hourly summary, restart doesn't help | Anthropic console-configured spend cap tripped | API returns 400 with `error.message` naming the reset date. Post-R95, also visible as `slog.Warn "LLM call failed"`. Direct test: `curl -H "x-api-key: $KEY" https://api.anthropic.com/v1/messages -d '{...}'` |
| One tag drops to 0 but others active | Trigger predicate not firing (e.g. no Tier 2 Liberated agents → oracle silent; sentinel `tier2_vitality WATCH "1 missing: Hunter"` is a related signal) | `/api/v1/agents?tier=2` and filter by State |
| Total calls 0 but client.Enabled() = true | API key revoked or invalid | `slog.Warn "LLM call failed"` with 401 message (post-R95) |
| Rate limit error `(20 calls/min)` | Burst exceeded internal rate limit | Increase `maxPerMin` in `client.go` if intentional |
