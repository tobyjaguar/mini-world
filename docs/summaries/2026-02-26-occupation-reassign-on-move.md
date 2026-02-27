# Refactor: Reassign Occupation at Movement Source — 2026-02-26

## Problem

When agents move between settlements (migration, diaspora, consolidation), their occupation is preserved even if the new terrain doesn't support it. A fisher moving to a plains hex can never produce fish — they sit unproductive until a weekly `reassignMismatchedProducers()` sweep catches them. That's up to 7 sim-days of zero productivity per move.

Additionally, `bestProductionHex()` never returns nil (it falls back to `sett.Position`), so the neighborhood resource check in the weekly sweep silently passed agents that actually had no viable resource nearby.

## Solution

New `reassignIfMismatched(a, settID)` method on `*Simulation` checks occupation viability at the point of movement, not in a periodic sweep. Called at all 4 movement sites:

| Site | File | Context |
|------|------|---------|
| `foundSettlement()` | `settlement_lifecycle.go` | Diaspora founders loop |
| `processSeasonalMigration()` | `perpetuation.go` | Desperate agent migration |
| `processViabilityCheck()` | `settlement_lifecycle.go` | Force-migration of non-viable settlements |
| `ConsolidateSettlement()` | `intervention.go` | Gardener consolidation intervention |

### How it works

1. Look up agent's required resource via `occupationResource` map
2. Special-case Alchemist (needs `ResourceHerbs` but not in `occupationResource`)
3. Skip non-resource occupations (Crafter, Merchant, Soldier, Scholar)
4. Scan 7-hex neighborhood (settlement position + 6 neighbors) for required resource
5. If no resource found, reassign via `bestOccupationForHex()` on settlement position hex

### Safety net

The existing `reassignMismatchedProducers()` is demoted to a weekly safety net:
- Comment updated to explain its new role
- Log level changed from `slog.Info` → `slog.Warn`
- If it fires with count > 0, we missed a movement path
- After weeks of count=0 in production, can be removed in a follow-up

## Files Modified

- `internal/engine/perpetuation.go` — new helper + migration call site + safety-net demotion
- `internal/engine/settlement_lifecycle.go` — diaspora + viability check call sites
- `internal/engine/intervention.go` — consolidation call site

## Verification

- `go build ./cmd/worldsim` — compiles cleanly
- Watch logs for `"reassigned occupation on move"` (debug) — confirms helper fires
- Watch for `"safety-net reassigned mismatched producers"` (warn) — should show count=0
