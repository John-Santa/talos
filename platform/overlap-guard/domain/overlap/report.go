package overlap

// Verdict is the precedence-ordered overlap outcome: BLOCK > SERIALIZE > OK.
type Verdict int

const (
	// VerdictOK means no file or module overlap was found.
	VerdictOK Verdict = iota
	// VerdictSerialize means a module-level overlap (but no file collision) was found.
	VerdictSerialize
	// VerdictBlock means at least one file collision between distinct agents was found.
	VerdictBlock
)

// Report is the full overlap evaluation: verdict, the two collision kinds, the HG6 collision rate, and observability advisories.
type Report struct {
	Verdict        Verdict
	FileCollisions []FileCollision
	ModuleOverlaps []ModuleOverlap
	Advisories     []string
	ClaimCount     int
	CollisionRate  float64
	OverThreshold  bool
}

// NewReport evaluates claims, derives the Verdict by precedence, and computes CollisionRate over distinct-agent pairs.
// OverThreshold is true when CollisionRate is STRICTLY greater than threshold (mirrors mo's 0.15 boundary).
func NewReport(claims []Claim, threshold float64) Report {
	fc := FileCollisions(claims)
	mo := ModuleOverlaps(claims)

	verdict := VerdictOK
	if len(fc) > 0 {
		verdict = VerdictBlock
	} else if len(mo) > 0 {
		verdict = VerdictSerialize
	}

	pairs, colliding := countDistinctAgentPairs(claims)
	rate := 0.0
	if pairs > 0 {
		rate = float64(colliding) / float64(pairs)
	}

	return Report{
		Verdict:        verdict,
		FileCollisions: fc,
		ModuleOverlaps: mo,
		ClaimCount:     len(claims),
		CollisionRate:  rate,
		OverThreshold:  rate > threshold,
	}
}

func countDistinctAgentPairs(claims []Claim) (total, colliding int) {
	seen := make(map[string]bool)

	for i := 0; i < len(claims); i++ {
		for j := i + 1; j < len(claims); j++ {
			a, b := claims[i], claims[j]
			if a.Agent == b.Agent {
				continue
			}
			key := pairKey(a.Agent, b.Agent)
			if seen[key] {
				continue
			}
			seen[key] = true
			total++
			if len(sharedFiles(a.Files, b.Files)) > 0 {
				colliding++
			}
		}
	}
	return total, colliding
}
