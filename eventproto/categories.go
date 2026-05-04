// Package eventproto is the source of truth for the worldsim event category
// taxonomy (R84, Phase 5.2 — Schema Sharing). The string values must match
// what currently flows through the SSE pipeline; renaming any of them is a
// breaking wire-protocol change.
//
// The relay imports this package via a local replace directive in
// crossworlds-relay/go.mod and derives its allowlist from Categories().
//
// The frontend's TypeScript discriminated union is generated from this file
// by mini-world/cmd/typegen — do not edit
// `crossworlds/src/lib/eventproto.generated.ts` by hand.
//
// Synthetic relay-emitted categories (baby_boom, crime_wave, trade_burst,
// regime_change, activity) live in `crossworlds-relay/eventproto/` because
// they originate in the relay, not worldsim.
package eventproto

// Category is the type tag attached to every world event. New categories
// MUST be declared here and rebuilt with `go generate ./...` before any
// emit site can use them.
type Category string

const (
	CategoryAgent     Category = "agent"     // individual-agent actions surfaced to Tier 1+
	CategoryBirth     Category = "birth"     // new agent born
	CategoryCrime     Category = "crime"     // theft / outlaw events
	CategoryDeath     Category = "death"     // agent died (natural, age, illness, battle)
	CategoryDisaster  Category = "disaster"  // storms, droughts, plagues, region-affecting
	CategoryDiscovery Category = "discovery" // medicinal springs, hidden routes, etc.
	CategoryEconomy   Category = "economy"   // markets, prices, infrastructure investment
	CategoryGardener  Category = "gardener"  // gardener interventions
	CategoryOracle    Category = "oracle"    // Liberated agent visions and oracle actions
	CategoryPolitical Category = "political" // governance transitions, faction politics
	CategorySocial    Category = "social"    // marriages, mentorship, recruitment, friendship
	CategorySpiritual Category = "spiritual" // doctrine awakenings, coherence transitions
	CategoryWarfare   Category = "warfare"   // raids, casualties, plunder
)

// Categories returns every declared worldsim category in declaration order.
// Used by the relay to derive its allowlist and by cmd/typegen to emit
// the TypeScript const tuple. Stable order matters for the generator's
// output to be deterministic.
func Categories() []Category {
	return []Category{
		CategoryAgent,
		CategoryBirth,
		CategoryCrime,
		CategoryDeath,
		CategoryDisaster,
		CategoryDiscovery,
		CategoryEconomy,
		CategoryGardener,
		CategoryOracle,
		CategoryPolitical,
		CategorySocial,
		CategorySpiritual,
		CategoryWarfare,
	}
}

// String exists so a Category prints as its string value in fmt %s, slog
// attrs, and similar contexts without an explicit cast.
func (c Category) String() string { return string(c) }
