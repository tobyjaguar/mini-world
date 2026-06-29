# LLM Cost-Reduction Plan — Biography Generation (2026-06-29)

## Goal

Roughly halve LLM call volume. Biography is the dominant tag (~20 calls/hr ≈ 60%
of all LLM calls; tier2 ~10/hr is core cognition and stays). On DeepSeek the whole
chain is ~$10/mo, so biography ≈ ~$6/mo; this is a quality/efficiency cleanup more
than a budget emergency.

## Important correction: there is no biography TTL to "raise"

The original framing was "raise the cache TTL." Inspecting the code, **biographies
have no TTL** — the lever is different from the newspaper's:

- `handleAgentStory` (`internal/api/server.go:669`) generates a biography on a cache
  **miss** (first view of an agent) and stores it in `bioCache` (`server.go:54`), an
  **in-memory `map[AgentID]cachedBio` with no expiry**. Subsequent views of the same
  agent return the cached copy with **zero** LLM cost (`server.go:688`). Regeneration
  happens only on an explicit admin `?refresh=true`.
- So biography LLM calls come from **cache misses (new/never-viewed agents)** and from
  the **cache being wiped on every restart/deploy** (it is in-memory, not persisted) —
  **not** from a TTL expiring entries.

Therefore *adding* a TTL would be counterproductive — it would expire cached bios and
cause **more** regeneration. The correct levers reduce cache misses and stop paying
twice. (The newspaper TTL — `NEWSPAPER_CACHE_HOURS`, `server.go:65` — is already fine;
newspaper barely appears in usage.)

## Options (ranked by leverage)

### 1. Tier-gate LLM biographies — RECOMMENDED (~80-90% cut)

~95% of agents are Tier 0 (ordinary, near-identical state machines). Generating bespoke
Haiku prose for a randomly-browsed Tier 0 farmer is the cost sink. Reserve LLM bios for
agents people actually care about; template the rest.

- In `handleAgentStory`, before calling `llm.GenerateBiography`, branch on `agent.Tier`
  (and/or `agent.Soul.State == Liberated`, or a non-empty `agent.Archetype`):
  - **Tier ≥ 1 / notable** → existing LLM path (bespoke biography).
  - **Tier 0** → a deterministic templated blurb assembled from struct fields already in
    `BiographyContext` (name, age, occupation, settlement, faction, coherence state,
    top relationship). No LLM call. Cache it the same way (or skip caching since it's
    cheap to rebuild).
- Effect: only the ~5% notable cohort (~30 Tier 2 + Tier 1) consume LLM bios; Tier 0
  browsing costs nothing. Cuts biography LLM calls ~80-90% on its own — comfortably past
  the "halve" target.
- Effort: ~30 lines + a template function. Risk: low — Tier 0 visitors still get a
  readable bio, just not unique LLM prose. UX-visible only to someone comparing many
  Tier 0 bios.

### 2. Persist `bioCache` across restarts — COMPLEMENT (eliminates deploy re-gen)

The cache is in-memory, so every deploy/restart wipes thousands of already-generated
bios; re-viewing them regenerates. Persist it like the rest of world state.

- Add a `biographies` table (or a `world_meta` JSON blob keyed by agent ID) in
  `internal/persistence`; save on the daily/shutdown save, load on startup into
  `bioCache`. Follows the R76 persistence-registry pattern.
- Effect: "don't pay twice" — a deployed worldsim keeps all prior bios. Helps most
  around deploys; does not reduce steady-state cost of genuinely new agents being viewed.
- Effort: ~40 lines. Risk: low. Combines naturally with #1 (only notable bios need
  persisting if #1 lands).

### 3. Lower the per-IP story rate limit — QUICK PARTIAL (~halves if IP-bound)

`storyLimiter := NewRateLimiter(10, time.Hour)` (`server.go:74`) is per-IP
(`ratelimit.go:94`, keyed by `r.RemoteAddr`). Lowering `10 → 5` halves the per-visitor
LLM-generation ceiling.

- One-line change.
- Effect: direct throttle *if* biography volume is dominated by a few heavy IPs hitting
  the cap. If volume is spread across many light IPs, impact is smaller.
- Risk: legitimate heavy browsers get 429s on uncached bios. Low UX cost (bios are a
  nice-to-have). Reversible.

## Recommendation

Ship **#1 (tier-gate) + #2 (persist)** together: #1 cuts steady-state cost the most and
matches the intent ("stop generating throwaway bios"), #2 stops the deploy-driven
re-gen wave. That removes ~80-90% of biography calls — total LLM volume drops from
~33/hr toward ~13/hr (tier2 + oracle + occasional notable bios), well past halving. #3
is the zero-effort partial if you want an immediate trim before the larger change.

No deploy urgency — at ~$10/mo nothing is on fire. Bundle into the next TickWeek-boundary
deploy.
