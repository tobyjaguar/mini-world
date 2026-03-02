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

## Call Budget (March 2026)

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

**Estimated daily cost:** ~$0.43/day (~$13/month) at Haiku 4.5 pricing ($1/M input, $5/M output).

### Before optimization (pre-R31)

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
| 2026-03-02 | Gardener interval 6→15 min | -144 calls/day |
| 2026-03-02 | Newspaper cache: sim-day→3 real hours | -16 calls/day |
| 2026-03-02 | Usage tracking + `/api/v1/llm-usage` | Visibility |
