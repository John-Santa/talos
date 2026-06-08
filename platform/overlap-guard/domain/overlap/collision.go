package overlap

import "sort"

// FileCollision is the same file claimed by two distinct agents — the hard BLOCK case.
type FileCollision struct {
	File string
	A, B Claim
}

// FileCollisions returns every same-file collision across distinct-agent claims (pairwise).
// Same-agent pairs are never counted as collisions (REQ-VERDICT-2).
// Output is sorted deterministically by file path then agent-pair (REQ-VERDICT-3).
func FileCollisions(claims []Claim) []FileCollision {
	var result []FileCollision

	for i := 0; i < len(claims); i++ {
		for j := i + 1; j < len(claims); j++ {
			a, b := claims[i], claims[j]
			if a.Agent == b.Agent {
				continue
			}
			for _, shared := range sharedFiles(a.Files, b.Files) {
				result = append(result, FileCollision{File: shared, A: a, B: b})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		if result[i].A.Agent != result[j].A.Agent {
			return result[i].A.Agent < result[j].A.Agent
		}
		return result[i].B.Agent < result[j].B.Agent
	})

	return result
}

// ModuleOverlap is two distinct agents touching the same module without a file collision — the soft SERIALIZE case.
type ModuleOverlap struct {
	Module string
	A, B   Claim
}

// ModuleOverlaps returns module-level overlaps among distinct agents, excluding pairs already in FileCollisions.
func ModuleOverlaps(claims []Claim) []ModuleOverlap {
	colliding := fileCollidingPairs(claims)
	var result []ModuleOverlap

	for i := 0; i < len(claims); i++ {
		for j := i + 1; j < len(claims); j++ {
			a, b := claims[i], claims[j]
			if a.Agent == b.Agent {
				continue
			}
			if a.Module == "" || b.Module == "" || a.Module != b.Module {
				continue
			}
			key := pairKey(a.Agent, b.Agent)
			if colliding[key] {
				continue
			}
			result = append(result, ModuleOverlap{Module: a.Module, A: a, B: b})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Module != result[j].Module {
			return result[i].Module < result[j].Module
		}
		if result[i].A.Agent != result[j].A.Agent {
			return result[i].A.Agent < result[j].A.Agent
		}
		return result[i].B.Agent < result[j].B.Agent
	})

	return result
}

func sharedFiles(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, f := range a {
		set[f] = struct{}{}
	}
	var shared []string
	seen := make(map[string]struct{})
	for _, f := range b {
		if _, ok := set[f]; ok {
			if _, dup := seen[f]; !dup {
				shared = append(shared, f)
				seen[f] = struct{}{}
			}
		}
	}
	sort.Strings(shared)
	return shared
}

func fileCollidingPairs(claims []Claim) map[string]bool {
	result := make(map[string]bool)
	for i := 0; i < len(claims); i++ {
		for j := i + 1; j < len(claims); j++ {
			a, b := claims[i], claims[j]
			if a.Agent == b.Agent {
				continue
			}
			if len(sharedFiles(a.Files, b.Files)) > 0 {
				result[pairKey(a.Agent, b.Agent)] = true
			}
		}
	}
	return result
}

func pairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}
