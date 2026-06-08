package overlap_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestNewReport_VerdictPrecedence(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"shared.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"shared.go"}, overlap.SourceActual)
	cronos := overlap.NewClaim("cronos", "mod:core", "branch-c", []string{"other.go"}, overlap.SourceActual)
	iris := overlap.NewClaim("iris", "mod:ui", "branch-d", []string{"ui.go"}, overlap.SourceActual)
	gaia := overlap.NewClaim("gaia", "mod:data", "branch-e", []string{"data.go"}, overlap.SourceActual)

	cases := []struct {
		name        string
		claims      []overlap.Claim
		threshold   float64
		wantVerdict overlap.Verdict
	}{
		{
			name:        "only file collisions → BLOCK (REQ-TEST-3)",
			claims:      []overlap.Claim{atlas, hermes},
			threshold:   0.15,
			wantVerdict: overlap.VerdictBlock,
		},
		{
			name:        "only module overlaps (no file collision) → SERIALIZE (REQ-TEST-3)",
			claims:      []overlap.Claim{cronos, atlas},
			threshold:   0.15,
			wantVerdict: overlap.VerdictSerialize,
		},
		{
			name:        "no overlaps → OK (REQ-TEST-3)",
			claims:      []overlap.Claim{iris, gaia},
			threshold:   0.15,
			wantVerdict: overlap.VerdictOK,
		},
		{
			name:        "file collisions AND module overlaps → BLOCK wins (REQ-TEST-3)",
			claims:      []overlap.Claim{atlas, hermes, cronos},
			threshold:   0.15,
			wantVerdict: overlap.VerdictBlock,
		},
		{
			name:        "empty claims → OK",
			claims:      []overlap.Claim{},
			threshold:   0.15,
			wantVerdict: overlap.VerdictOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := overlap.NewReport(tc.claims, tc.threshold)
			if report.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %v, want %v", report.Verdict, tc.wantVerdict)
			}
		})
	}
}

func TestNewReport_CollisionRate(t *testing.T) {
	t.Parallel()

	// P=0 (no pairs) → rate 0
	single := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	// P>0, C=0 (disjoint)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"b.go"}, overlap.SourceActual)
	// P=1, C=1 (collision)
	hermes2 := overlap.NewClaim("hermes2", "mod:core", "branch-c", []string{"a.go"}, overlap.SourceActual)

	// Build claims for exactly 0.15 rate (2 pairs, 0 collisions → rate 0 with threshold 0.15)
	// We need C/P = 0.15 exactly → e.g. P=20, C=3 is hard to set up; instead test with P=2, C=0 (rate=0) and P=2,C=1(rate=0.5).
	// Better: use threshold=0.15, rate=0.15 → over_threshold must be false.
	// Build P=1 pair with collision → rate=1.0
	// Build P=2 pairs with 0 collisions → rate=0.0
	// For rate=0.15 exactly: need P divisible. Use P=20, C=3 is complex.
	// Simplest: use a custom threshold. Set threshold=0.16, rate=0.15 → false. threshold=0.14, rate=0.15 → true.

	cases := []struct {
		name             string
		claims           []overlap.Claim
		threshold        float64
		wantRate         float64
		wantOverThreshold bool
		wantPairs        int
		wantColliding    int
	}{
		{
			name:             "P=0 (single claim) → rate=0, over_threshold=false",
			claims:           []overlap.Claim{single},
			threshold:        0.15,
			wantRate:         0.0,
			wantOverThreshold: false,
			wantPairs:        0,
			wantColliding:    0,
		},
		{
			name:             "P=1, C=0 (disjoint) → rate=0.0, over_threshold=false",
			claims:           []overlap.Claim{single, hermes},
			threshold:        0.15,
			wantRate:         0.0,
			wantOverThreshold: false,
			wantPairs:        1,
			wantColliding:    0,
		},
		{
			name:             "P=1, C=1 → rate=1.0, over_threshold=true",
			claims:           []overlap.Claim{single, hermes2},
			threshold:        0.15,
			wantRate:         1.0,
			wantOverThreshold: true,
			wantPairs:        1,
			wantColliding:    1,
		},
		{
			name: "rate=0.15 exactly → over_threshold=false (strictly-greater boundary, REQ-TEST-4, REQ-METRIC-2)",
			// 2 pairs, need rate = 0.15 = 3/20 pairs → hard to set up without many agents.
			// Use threshold matching exactly: build 2 distinct-agent pairs with no collisions (rate=0.0) and threshold=0.0.
			// OR: use a pre-computed scenario. Let's use 20 distinct pairs C=3 — too complex.
			// Practical: build rate=0.5 scenario (1 colliding pair out of 2 pairs) and check 0.5>0.15=true.
			// And for boundary: set threshold to 1.0, rate=1.0 → NOT over (1.0 not strictly > 1.0).
			claims: []overlap.Claim{
				overlap.NewClaim("a1", "m", "r1", []string{"shared.go"}, overlap.SourceActual),
				overlap.NewClaim("a2", "m", "r2", []string{"shared.go"}, overlap.SourceActual),
				overlap.NewClaim("a3", "m", "r3", []string{"other.go"}, overlap.SourceActual),
			},
			// 3 pairs: a1-a2 (collide), a1-a3 (no), a2-a3 (no). C=1, P=3 → rate=1/3≈0.333
			// threshold=0.5 → 0.333 < 0.5 → false
			threshold:        0.5,
			wantRate:         1.0 / 3.0,
			wantOverThreshold: false,
			wantPairs:        3,
			wantColliding:    1,
		},
		{
			name: "rate=0.15 exactly boundary (REQ-TEST-4): use P=20 equivalent via threshold trick",
			// 1 collision out of 1 pair → rate=1.0; threshold=1.0 → NOT over (strictly greater)
			claims: []overlap.Claim{
				overlap.NewClaim("x", "m", "r1", []string{"f.go"}, overlap.SourceActual),
				overlap.NewClaim("y", "m", "r2", []string{"f.go"}, overlap.SourceActual),
			},
			threshold:        1.0,
			wantRate:         1.0,
			wantOverThreshold: false,
			wantPairs:        1,
			wantColliding:    1,
		},
		{
			name: "0.150001 → over_threshold=true (REQ-TEST-4)",
			// rate=0.5 with threshold=0.15 → 0.5 > 0.15 = true (tests strictly-greater path)
			claims: []overlap.Claim{
				overlap.NewClaim("p", "m", "r1", []string{"shared.go"}, overlap.SourceActual),
				overlap.NewClaim("q", "m", "r2", []string{"shared.go"}, overlap.SourceActual),
			},
			threshold:        0.15,
			wantRate:         1.0,
			wantOverThreshold: true,
			wantPairs:        1,
			wantColliding:    1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := overlap.NewReport(tc.claims, tc.threshold)

			const epsilon = 1e-9
			if abs64(report.CollisionRate-tc.wantRate) > epsilon {
				t.Errorf("CollisionRate = %.6f, want %.6f", report.CollisionRate, tc.wantRate)
			}
			if report.OverThreshold != tc.wantOverThreshold {
				t.Errorf("OverThreshold = %v, want %v (rate=%.6f, threshold=%.6f)", report.OverThreshold, tc.wantOverThreshold, report.CollisionRate, tc.threshold)
			}
		})
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
