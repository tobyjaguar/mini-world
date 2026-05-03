package engine

import (
	"math"
	"testing"

	"github.com/talgya/mini-world/internal/phi"
)

// TestRecruitmentProbabilityEndpoints locks in the four documented
// endpoints of the recruitment formula. If anyone changes the
// constants (Psyche, Being) or the formula shape, this test fires.
func TestRecruitmentProbabilityEndpoints(t *testing.T) {
	cases := []struct {
		name           string
		bestScore      float64
		totalInfluence float64
		want           float64
		desc           string
	}{
		{
			name:           "fully dominated with affinity match",
			bestScore:      100 * phi.Being, // single-faction settlement, agent has natural affinity
			totalInfluence: 100,
			want:           phi.Being * phi.Psyche, // ≈ Matter (~0.618)
			desc:           "the strongest possible recruitment signal in the design",
		},
		{
			name:           "fully dominated no affinity",
			bestScore:      100, // single-faction settlement, agent has no natural affinity
			totalInfluence: 100,
			want:           phi.Psyche, // ~0.382
			desc:           "even no-affinity recruits in a one-faction town are likely",
		},
		{
			name:           "50% share with affinity",
			bestScore:      50 * phi.Being,
			totalInfluence: 100,
			want:           0.5 * phi.Being * phi.Psyche, // ~0.309
			desc:           "typical contested settlement, affinity match",
		},
		{
			name:           "50% share no affinity",
			bestScore:      50,
			totalInfluence: 100,
			want:           0.5 * phi.Psyche, // ~0.191
			desc:           "typical contested settlement, no match",
		},
		{
			name:           "minority faction with affinity barely competes",
			bestScore:      10 * phi.Being,
			totalInfluence: 100,
			want:           0.1 * phi.Being * phi.Psyche, // ~0.062
			desc:           "small faction can still recruit affinity matches",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := recruitmentProbability(tc.bestScore, tc.totalInfluence)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("recruitmentProbability(%g, %g) = %g, want %g  (%s)",
					tc.bestScore, tc.totalInfluence, got, tc.want, tc.desc)
			}
		})
	}
}

// TestRecruitmentProbabilityZeroInfluenceGuard ensures we don't hit a
// division-by-zero in degenerate settlements where every faction has zero
// influence (rare but possible during world setup or after weekly decay).
// Without the guard, the formula returns NaN, which would crash the
// downstream `if hash >= prob` comparison silently.
func TestRecruitmentProbabilityZeroInfluenceGuard(t *testing.T) {
	cases := []struct {
		name           string
		bestScore      float64
		totalInfluence float64
	}{
		{"both zero", 0, 0},
		{"total zero", 50, 0},
		{"total negative (defensive)", 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := recruitmentProbability(tc.bestScore, tc.totalInfluence)
			if got != 0 {
				t.Errorf("recruitmentProbability(%g, %g) = %g, want 0 (degenerate guard)",
					tc.bestScore, tc.totalInfluence, got)
			}
		})
	}
}

// TestRecruitmentProbabilityMonotonicInBestScore is a property test:
// for fixed totalInfluence, recruiting probability must be monotonic
// in the winning faction's score. If anyone changes the formula in a
// way that breaks monotonicity, recruitment dynamics would behave
// nonsensically (a stronger faction less likely to recruit than a
// weaker one).
func TestRecruitmentProbabilityMonotonicInBestScore(t *testing.T) {
	const total = 100.0
	prev := -1.0
	for score := 0.0; score <= total*phi.Being; score += 5 {
		got := recruitmentProbability(score, total)
		if got < prev {
			t.Errorf("non-monotonic: at score=%g, prob=%g < prev=%g", score, got, prev)
		}
		prev = got
	}
}
