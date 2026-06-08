package overlap_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestFileCollisions(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go", "shared.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:qa", "branch-b", []string{"b.go", "shared.go"}, overlap.SourceActual)
	hermes2 := overlap.NewClaim("hermes", "mod:qa", "branch-c", []string{"c.go", "shared.go"}, overlap.SourceActual)
	cronos := overlap.NewClaim("cronos", "mod:data", "branch-d", []string{"d.go", "shared.go"}, overlap.SourceActual)
	iris := overlap.NewClaim("iris", "mod:ui", "branch-e", []string{"e.go"}, overlap.SourceActual)
	// Same agent, different branches — must NOT produce file collision.
	atlasExtra := overlap.NewClaim("atlas", "mod:core", "branch-f", []string{"a.go", "shared.go"}, overlap.SourceActual)

	cases := []struct {
		name       string
		claims     []overlap.Claim
		wantCount  int
		wantFiles  []string
	}{
		{
			name:      "distinct agents share file → collision",
			claims:    []overlap.Claim{atlas, hermes},
			wantCount: 1,
			wantFiles: []string{"shared.go"},
		},
		{
			name:      "same agent different branches → no collision (REQ-VERDICT-2, REQ-TEST-8)",
			claims:    []overlap.Claim{atlas, atlasExtra},
			wantCount: 0,
		},
		{
			name:      "disjoint files → no collision",
			claims:    []overlap.Claim{atlas, iris},
			wantCount: 0,
		},
		{
			name:      "three agents all share same file → pairwise collisions",
			claims:    []overlap.Claim{atlas, hermes, cronos},
			wantCount: 3,
		},
		{
			name:      "single claim → no pairs → no collision",
			claims:    []overlap.Claim{atlas},
			wantCount: 0,
		},
		{
			name:      "empty claims → no collision",
			claims:    []overlap.Claim{},
			wantCount: 0,
		},
		{
			name:      "two hermes branches sharing file → no collision (same agent)",
			claims:    []overlap.Claim{hermes, hermes2},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			collisions := overlap.FileCollisions(tc.claims)

			if len(collisions) != tc.wantCount {
				t.Errorf("FileCollisions count = %d, want %d; collisions: %v", len(collisions), tc.wantCount, collisions)
				return
			}
			if len(tc.wantFiles) > 0 && len(collisions) > 0 {
				if collisions[0].File != tc.wantFiles[0] {
					t.Errorf("first collision file = %q, want %q", collisions[0].File, tc.wantFiles[0])
				}
			}
		})
	}
}

func TestFileCollisions_DeterministicOrder(t *testing.T) {
	t.Parallel()

	a := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"z.go", "a.go"}, overlap.SourceActual)
	b := overlap.NewClaim("hermes", "mod:qa", "branch-b", []string{"z.go", "a.go"}, overlap.SourceActual)

	collisions1 := overlap.FileCollisions([]overlap.Claim{a, b})
	collisions2 := overlap.FileCollisions([]overlap.Claim{b, a})

	if len(collisions1) != len(collisions2) {
		t.Fatalf("collision count differs: %d vs %d", len(collisions1), len(collisions2))
	}
	for i := range collisions1 {
		if collisions1[i].File != collisions2[i].File {
			t.Errorf("collision[%d].File differs: %q vs %q", i, collisions1[i].File, collisions2[i].File)
		}
	}
}

func TestModuleOverlaps(t *testing.T) {
	t.Parallel()

	// Same module, different files, different agents → SERIALIZE (ModuleOverlap only, no FileCollision)
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"b.go"}, overlap.SourceActual)
	// Different module → no overlap
	cronos := overlap.NewClaim("cronos", "mod:data", "branch-c", []string{"c.go"}, overlap.SourceActual)
	// File collision case — should be EXCLUDED from ModuleOverlaps
	atlasConflict := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"shared.go"}, overlap.SourceActual)
	hermesConflict := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"shared.go"}, overlap.SourceActual)

	cases := []struct {
		name      string
		claims    []overlap.Claim
		wantCount int
	}{
		{
			name:      "same module, different files, different agents → one module overlap",
			claims:    []overlap.Claim{atlas, hermes},
			wantCount: 1,
		},
		{
			name:      "different modules → no module overlap",
			claims:    []overlap.Claim{atlas, cronos},
			wantCount: 0,
		},
		{
			name:      "file collision pair excluded from module overlaps",
			claims:    []overlap.Claim{atlasConflict, hermesConflict},
			wantCount: 0,
		},
		{
			name:      "empty → no overlap",
			claims:    []overlap.Claim{},
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			overlaps := overlap.ModuleOverlaps(tc.claims)
			if len(overlaps) != tc.wantCount {
				t.Errorf("ModuleOverlaps count = %d, want %d; got %v", len(overlaps), tc.wantCount, overlaps)
			}
		})
	}
}
