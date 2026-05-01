// Narrative memory imprinting (R80, Phase 4.5). When relationally significant
// events fire — a sage's death, a marriage, the start of a mentorship — the
// agents who lived through them gain memories that name the others involved.
//
// Prior to R80 the existing memory plumbing fired indiscriminately
// (createSettlementMemories planted the same generic line on every Tier 2 in
// a settlement) or not at all (formFamilies/processMentorship emitted events
// but left the agents themselves with no recollection). The result was that
// "a liberation death produces coherence ripples but no story survivors
// carry" — survivors had nothing to *say* about the dead, the married, or
// the taught. Without those imprints, biography generation, oracle prompts,
// and Tier 2 cognition context all fall back on generic phrases.
//
// These memories are story-bearing: each names a specific other agent, places
// the event, and carries durable importance. They are designed to surface in
// `ImportantMemories` queries and survive the 50-slot ring without being
// evicted by trade-result clutter.
package engine

import (
	"fmt"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
)

// Φ-derived sentiment thresholds for relational memory imprint eligibility.
// `var` rather than `const` because phi.Psyche/phi.Matter are runtime
// math.Pow values, not compile-time constants.
var (
	// Sage deaths reach further: any positive bond above Psyche (0.382)
	// imprints. The settlement losing its wise is everyone's loss; the
	// threshold filters out neutral/forgotten relationships, not friendships.
	sageDeathRelationshipThreshold = float32(phi.Psyche)

	// Ordinary deaths only imprint on close ties (above Matter, 0.618). Most
	// deaths in a 400K world should not flood every Tier 1+ memory stream.
	ordinaryDeathRelationshipThreshold = float32(phi.Matter)
)

// Importance ceilings for the four imprint types. Marriage tops the scale
// because it reshapes the rest of a life; sage-death-of-a-friend is just
// below it. Ordinary deaths and mentorship onsets land lower so they can
// age out under heavy memory pressure. These are explicit narrative weights,
// not Φ-derived (per the R78 magic-number-audit pattern).
const (
	importanceMarriage         float32 = 0.85
	importanceSageDeathClose   float32 = 0.85
	importanceSageDeathDistant float32 = 0.60
	importanceOrdinaryClose    float32 = 0.70
	importanceMentorshipStart  float32 = 0.55
)

// imprintDeathMemories plants per-witness memories for agents who had a
// non-trivial relationship with the deceased. Liberation deaths reach further
// (Psyche threshold, all alive Tier 0+ in the settlement) than ordinary
// deaths (Matter threshold, Tier 1+ only — Tier 0 memory pressure is high).
//
// Settlement-wide generic memories from createSettlementMemories are still
// emitted by the caller; this function adds *personalized* memories on top
// for the specific agents who knew the deceased by name.
func (s *Simulation) imprintDeathMemories(deceased *agents.Agent, isLiberated bool, settName string, tick uint64) {
	if deceased.HomeSettID == nil {
		return
	}
	witnesses := s.SettlementAgents[*deceased.HomeSettID]
	threshold := ordinaryDeathRelationshipThreshold
	if isLiberated {
		threshold = sageDeathRelationshipThreshold
	}

	for _, witness := range witnesses {
		if !witness.Alive || witness.ID == deceased.ID {
			continue
		}
		// Ordinary deaths only imprint Tier 1+ to keep memory streams from
		// flooding with farewell entries.
		if !isLiberated && witness.Tier < agents.Tier1 {
			continue
		}

		sentiment := relationshipSentiment(witness, deceased.ID)
		if sentiment < threshold {
			continue
		}

		var content string
		var importance float32
		switch {
		case isLiberated && sentiment >= 0.7:
			content = fmt.Sprintf("Lost %s, a dear friend. Sage of %s. Their light withdrawn.", deceased.Name, settName)
			importance = importanceSageDeathClose
		case isLiberated:
			content = fmt.Sprintf("Witnessed the passing of %s, sage of %s.", deceased.Name, settName)
			importance = importanceSageDeathDistant
		default:
			content = fmt.Sprintf("Lost %s, a friend, in %s.", deceased.Name, settName)
			importance = importanceOrdinaryClose
		}

		// Importance scales with sentiment but never exceeds the type ceiling.
		scaled := importance * sentiment
		if scaled > importance {
			scaled = importance
		}
		// Floor at half the ceiling so even the bare-minimum-eligible
		// relationship leaves a recognizable trace.
		if scaled < importance*0.5 {
			scaled = importance * 0.5
		}
		agents.AddMemory(witness, tick, content, scaled)
	}
}

// imprintMarriageMemories plants the marriage on both partners. Marriage is
// life-defining — high importance, durable across the 50-slot ring even
// under sustained event pressure.
func (s *Simulation) imprintMarriageMemories(a, partner *agents.Agent, settName string, tick uint64) {
	contentA := fmt.Sprintf("Married %s in %s.", partner.Name, settName)
	contentB := fmt.Sprintf("Married %s in %s.", a.Name, settName)
	agents.AddMemory(a, tick, contentA, importanceMarriage)
	agents.AddMemory(partner, tick, contentB, importanceMarriage)
}

// imprintMentorshipStartMemories plants memories on both parties only on
// the first pairing. The caller must check that no prior relationship
// existed before invoking — subsequent weekly pairings would otherwise
// flood streams.
func (s *Simulation) imprintMentorshipStartMemories(mentor, mentee *agents.Agent, settName string, tick uint64) {
	mentorContent := fmt.Sprintf("Took %s as my student in %s.", mentee.Name, settName)
	menteeContent := fmt.Sprintf("Began studying under %s in %s.", mentor.Name, settName)
	agents.AddMemory(mentor, tick, mentorContent, importanceMentorshipStart)
	agents.AddMemory(mentee, tick, menteeContent, importanceMentorshipStart)
}

// relationshipSentiment returns the sentiment of a's relationship with target,
// or 0 if no relationship exists. Linear scan is acceptable: agents cap at 20
// relationships and this only fires on rare events (deaths, marriages).
func relationshipSentiment(a *agents.Agent, target agents.AgentID) float32 {
	for _, rel := range a.Relationships {
		if rel.TargetID == target {
			return rel.Sentiment
		}
	}
	return 0
}

// hasRelationshipWith reports whether a has any relationship entry for target,
// regardless of sentiment. Used to detect "first time we met" so mentorship
// memories only plant on the initial pairing.
func hasRelationshipWith(a *agents.Agent, target agents.AgentID) bool {
	for _, rel := range a.Relationships {
		if rel.TargetID == target {
			return true
		}
	}
	return false
}
