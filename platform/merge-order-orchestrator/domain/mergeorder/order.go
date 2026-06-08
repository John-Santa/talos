package mergeorder

import "sort"

// Order computes a deterministic, safe merge order for the given candidates.
//
// Priority: (1) topological layers from deps (back-edge → ErrDependencyCycle),
// (2) within a layer: ascending ChangedFiles count, (3) ascending CreatedAt,
// (4) ascending Branch lexicographic.
func Order(candidates []Candidate, deps map[string][]string) ([]Candidate, error) {
	if len(candidates) == 0 {
		return nil, &ErrNoCandidates{}
	}

	layers, err := topoLayers(candidates, deps)
	if err != nil {
		return nil, err
	}

	result := make([]Candidate, 0, len(candidates))
	for _, layer := range layers {
		sorted := make([]Candidate, len(layer))
		copy(sorted, layer)
		sort.SliceStable(sorted, func(i, j int) bool {
			ci, cj := sorted[i], sorted[j]
			if len(ci.ChangedFiles) != len(cj.ChangedFiles) {
				return len(ci.ChangedFiles) < len(cj.ChangedFiles)
			}
			if !ci.CreatedAt.Equal(cj.CreatedAt) {
				return ci.CreatedAt.Before(cj.CreatedAt)
			}
			return ci.Branch < cj.Branch
		})
		result = append(result, sorted...)
	}
	return result, nil
}

// topoLayers performs a Kahn-style topological sort returning layers of candidates.
// A back-edge (cycle) produces ErrDependencyCycle.
func topoLayers(candidates []Candidate, deps map[string][]string) ([][]Candidate, error) {
	byBranch := make(map[string]Candidate, len(candidates))
	for _, c := range candidates {
		byBranch[c.Branch] = c
	}

	// Build adjacency: for each candidate, who must come before it?
	// deps["feat/b"] = ["feat/a"] means feat/a must precede feat/b.
	// inDegree[b] = number of predecessors of b that are in the candidate set.
	inDegree := make(map[string]int, len(candidates))
	successors := make(map[string][]string, len(candidates))

	for _, c := range candidates {
		if _, ok := inDegree[c.Branch]; !ok {
			inDegree[c.Branch] = 0
		}
	}

	for branch, needs := range deps {
		if _, inSet := byBranch[branch]; !inSet {
			continue
		}
		for _, prereq := range needs {
			if _, inSet := byBranch[prereq]; !inSet {
				continue
			}
			inDegree[branch]++
			successors[prereq] = append(successors[prereq], branch)
		}
	}

	// Collect initial layer (zero in-degree), sorted for determinism.
	var queue []string
	for _, c := range candidates {
		if inDegree[c.Branch] == 0 {
			queue = append(queue, c.Branch)
		}
	}
	sort.Strings(queue)

	var layers [][]Candidate
	visited := 0

	for len(queue) > 0 {
		layer := make([]Candidate, 0, len(queue))
		for _, b := range queue {
			layer = append(layer, byBranch[b])
		}
		layers = append(layers, layer)
		visited += len(queue)

		var nextQueue []string
		for _, b := range queue {
			for _, succ := range successors[b] {
				inDegree[succ]--
				if inDegree[succ] == 0 {
					nextQueue = append(nextQueue, succ)
				}
			}
		}
		sort.Strings(nextQueue)
		queue = nextQueue
	}

	if visited != len(candidates) {
		// Cycle exists: collect participants (those still with in-degree > 0).
		var cycleParticipants []string
		for _, c := range candidates {
			if inDegree[c.Branch] > 0 {
				cycleParticipants = append(cycleParticipants, c.Branch)
			}
		}
		sort.Strings(cycleParticipants)
		return nil, &ErrDependencyCycle{Branches: cycleParticipants}
	}

	return layers, nil
}
