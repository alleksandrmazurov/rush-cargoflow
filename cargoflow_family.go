package rush

import (
	"fmt"
	"sort"
)

// PuzzleFamily is a greedy cluster of similar Cargo Flow candidates.
type PuzzleFamily struct {
	FamilyID           string
	MemberIndexes      []int
	RepresentativeIdx  int
	AvgOptimal         float64
	InventoryProfile   string
	DominantSequence   string
}

// ClusterByFamilyDistance greedily clusters candidates. Deterministic.
// threshold: max distance to join an existing family (e.g. 0.28).
func ClusterByFamilyDistance(sigs []PuzzleSignature, threshold float64) []PuzzleFamily {
	n := len(sigs)
	assigned := make([]int, n)
	for i := range assigned {
		assigned[i] = -1
	}
	families := []PuzzleFamily{}
	for i := 0; i < n; i++ {
		if assigned[i] >= 0 {
			continue
		}
		fid := fmt.Sprintf("F%03d", len(families)+1)
		members := []int{i}
		assigned[i] = len(families)
		for j := i + 1; j < n; j++ {
			if assigned[j] >= 0 {
				continue
			}
			if PuzzleFamilyDistance(sigs[i], sigs[j]) <= threshold {
				assigned[j] = len(families)
				members = append(members, j)
			}
		}
		families = append(families, PuzzleFamily{
			FamilyID:      fid,
			MemberIndexes: members,
		})
	}
	for fi := range families {
		f := &families[fi]
		best := f.MemberIndexes[0]
		bestScore := -1.0
		sumOpt := 0
		seqCount := map[string]int{}
		invCount := map[string]int{}
		for _, mi := range f.MemberIndexes {
			s := sigs[mi]
			sumOpt += s.OptimalGestures
			seqCount[s.SequenceDigest]++
			invCount[s.InventorySignature]++
			// Prefer mid-high complexity + deeper dependency as representative.
			score := float64(s.OptimalGestures)*1.2 + float64(s.DependencyDepth) + float64(s.DistinctMovedPieces)*0.5
			if score > bestScore {
				bestScore = score
				best = mi
			}
		}
		f.RepresentativeIdx = best
		f.AvgOptimal = float64(sumOpt) / float64(len(f.MemberIndexes))
		f.DominantSequence = maxKey(seqCount)
		f.InventoryProfile = maxKey(invCount)
	}
	return families
}

func maxKey(m map[string]int) string {
	best, bestN := "", -1
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if m[k] > bestN {
			bestN = m[k]
			best = k
		}
	}
	return best
}

// FarthestPointShortlist picks diverse representatives across difficulty bands.
func FarthestPointShortlist(
	cands []CuratorSolvedCandidate,
	families []PuzzleFamily,
	target int,
	bandQuotas map[string]int,
	seed int64,
) []int {
	if target <= 0 || len(cands) == 0 {
		return nil
	}
	// Start from family representatives, sorted by estimated difficulty desc then seed key.
	type candIdx struct {
		idx int
		fam string
	}
	pool := make([]candIdx, 0, len(families))
	for _, f := range families {
		pool = append(pool, candIdx{idx: f.RepresentativeIdx, fam: f.FamilyID})
		// Optionally add second member if difficulty differs a lot.
		repOpt := cands[f.RepresentativeIdx].Metrics.OptimalGestures
		for _, mi := range f.MemberIndexes {
			if mi == f.RepresentativeIdx {
				continue
			}
			d := cands[mi].Metrics.OptimalGestures - repOpt
			if d < 0 {
				d = -d
			}
			if d >= 4 {
				pool = append(pool, candIdx{idx: mi, fam: f.FamilyID})
				break
			}
		}
	}
	sort.SliceStable(pool, func(i, j int) bool {
		ai, aj := cands[pool[i].idx], cands[pool[j].idx]
		if ai.Signals.EstimatedHumanDifficultyScore != aj.Signals.EstimatedHumanDifficultyScore {
			return ai.Signals.EstimatedHumanDifficultyScore > aj.Signals.EstimatedHumanDifficultyScore
		}
		return sampleKey(seed, ai.Source) < sampleKey(seed, aj.Source)
	})

	selected := []int{}
	selectedSet := map[int]bool{}
	bandCount := map[string]int{}
	usedSource := map[string]bool{}
	usedFamBand := map[string]bool{} // family+band

	pickOK := func(idx int, fam string) bool {
		c := cands[idx]
		if usedSource[c.Source.SourcePuzzleID] {
			return false
		}
		b := cargoOptBand(c.Metrics.OptimalGestures)
		if q := bandQuotas[b]; q > 0 && bandCount[b] >= q {
			return false
		}
		key := fam + "|" + b
		if usedFamBand[key] {
			return false
		}
		return true
	}

	// Seed: best candidate that fits any open band.
	for _, p := range pool {
		if len(selected) >= target {
			break
		}
		if !pickOK(p.idx, p.fam) {
			continue
		}
		selected = append(selected, p.idx)
		selectedSet[p.idx] = true
		c := cands[p.idx]
		usedSource[c.Source.SourcePuzzleID] = true
		b := cargoOptBand(c.Metrics.OptimalGestures)
		bandCount[b]++
		usedFamBand[p.fam+"|"+b] = true
		break
	}

	for len(selected) < target {
		bestIdx := -1
		bestFam := ""
		bestDist := -1.0
		for _, p := range pool {
			if selectedSet[p.idx] || !pickOK(p.idx, p.fam) {
				continue
			}
			minD := 1e9
			for _, si := range selected {
				d := PuzzleFamilyDistance(cands[p.idx].Metrics.Signature, cands[si].Metrics.Signature)
				if d < minD {
					minD = d
				}
			}
			if minD > bestDist {
				bestDist = minD
				bestIdx = p.idx
				bestFam = p.fam
			}
		}
		if bestIdx < 0 {
			break
		}
		selected = append(selected, bestIdx)
		selectedSet[bestIdx] = true
		c := cands[bestIdx]
		usedSource[c.Source.SourcePuzzleID] = true
		b := cargoOptBand(c.Metrics.OptimalGestures)
		bandCount[b]++
		usedFamBand[bestFam+"|"+b] = true
	}
	return selected
}

func cargoOptBand(n int) string {
	switch {
	case n >= 6 && n <= 8:
		return "6-8"
	case n >= 9 && n <= 11:
		return "9-11"
	case n >= 12 && n <= 14:
		return "12-14"
	case n >= 15 && n <= 17:
		return "15-17"
	case n >= 18:
		return "18+"
	default:
		return "other"
	}
}
