package rush

import (
	"fmt"
	"math"
	"sort"
)

// BoardMixSelectReport explains pool vs final diversity selection and feasibility.
type BoardMixSelectReport struct {
	FinalTarget              int               `json:"finalTarget"`
	FinalAccepted            int               `json:"finalAccepted"`
	MissingCount             int               `json:"missingCount"`
	PoolSize                 int               `json:"poolSize"`
	PoolUniqueFamilies       int               `json:"poolUniqueFamilies"`
	PoolShapeDist            map[string]int    `json:"poolBoardShapeClass"`
	PoolInventoryDist        map[string]int    `json:"poolInventoryClass"`
	PoolOuterZoneRelevant    int               `json:"poolOuterZoneRelevant"`
	PoolDifficultyBands      map[string]int    `json:"poolDifficultyBands"`
	PoolShapeInventoryCross  map[string]map[string]int `json:"poolShapeInventoryCross"`
	PoolCrossAvailability    map[string]int    `json:"poolCrossAvailability"`
	FinalShapeDist           map[string]int    `json:"finalBoardShapeClass"`
	FinalInventoryDist       map[string]int    `json:"finalInventoryClass"`
	FinalOuterZoneRelevant   int               `json:"finalOuterZoneRelevant"`
	DistinctBoardShapes      int               `json:"distinctBoardShapes"`
	SelectionSkipCounters    map[string]int    `json:"selectionSkipCounters"`
	QuotaRelaxations         []string          `json:"quotaRelaxations"`
	UnmetRequirements        []string          `json:"unmetRequirements"`
	WhyFinalShort            []string          `json:"whyFinalShort,omitempty"`
	DiversityTargetUnmet     bool              `json:"diversityTargetUnmet"`
	DiversityUnmetReasons    []string          `json:"diversityUnmetReasons,omitempty"`
	ExpandedFullFieldDiag    BoardShapeFeasibilityDiag `json:"expandedFullFieldDiagnostic"`
	HardConstraints          []string          `json:"hardConstraints"`
	SoftConstraints          []string          `json:"softConstraints"`
}

// BoardShapeFeasibilityDiag explains missing Expanded/FullField/Compact classes.
type BoardShapeFeasibilityDiag struct {
	Summary              string         `json:"summary"`
	ClassifierNotes      []string       `json:"classifierNotes"`
	PoolShapeCounts      map[string]int `json:"poolShapeCounts"`
	LikelyRootCause      string         `json:"likelyRootCause"`
	Native7x8Limitation  string         `json:"native7x8Limitation"`
}

// SelectBoardMixShortlist picks a diversity-aware final shortlist from a solved pool.
func SelectBoardMixShortlist(pool []BoardMixAccepted, cfg BoardMixConfig) ([]BoardMixAccepted, BoardMixSelectReport) {
	rep := BoardMixSelectReport{
		PoolSize:                len(pool),
		PoolShapeDist:           map[string]int{},
		PoolInventoryDist:       map[string]int{},
		PoolDifficultyBands:     map[string]int{},
		PoolShapeInventoryCross: map[string]map[string]int{},
		PoolCrossAvailability:   map[string]int{},
		FinalShapeDist:          map[string]int{},
		FinalInventoryDist:      map[string]int{},
		SelectionSkipCounters:   map[string]int{},
		QuotaRelaxations:        []string{},
		UnmetRequirements:       []string{},
		WhyFinalShort:           []string{},
		HardConstraints: []string{
			"UniqueFamily (one final slot per FamilyId)",
			"MinDistinctBoardShapes if pool supports it",
			"FinalAccepted == FinalTarget for DiversityUnmet=false",
		},
		SoftConstraints: []string{
			"InventoryTargets 3/3/3/3 (preferred, relaxable)",
			"MaxBoardShapeFraction 0.40→0.50→0.60 (staged)",
			"MaxInventoryClassFraction (relaxable)",
			"Exact shape quota counts (relaxable)",
		},
	}

	famSet := map[string]bool{}
	for _, c := range pool {
		shape := string(c.BoardUtil.BoardShapeClass)
		inv := string(c.InventoryClass)
		rep.PoolShapeDist[shape]++
		rep.PoolInventoryDist[inv]++
		if c.BoardUtil.OuterZoneRelevant {
			rep.PoolOuterZoneRelevant++
		}
		band := c.SelectionBand
		if band == "" {
			band = "unknown"
		}
		rep.PoolDifficultyBands[band]++
		famSet[c.FamilyID] = true
		if rep.PoolShapeInventoryCross[shape] == nil {
			rep.PoolShapeInventoryCross[shape] = map[string]int{}
		}
		rep.PoolShapeInventoryCross[shape][inv]++
		// Cross-constraint probes
		if c.InventoryClass == InvThree1x1 && c.BoardUtil.OuterZoneRelevant {
			rep.PoolCrossAvailability["Three1x1+OuterZoneRelevant"]++
		}
		if c.InventoryClass == InvTwo1x1 {
			rep.PoolCrossAvailability["Two1x1"]++
		}
		if c.BoardUtil.BoardShapeClass == ShapeExpanded {
			rep.PoolCrossAvailability["Expanded"]++
		}
		if c.BoardUtil.BoardShapeClass == ShapeFullField {
			rep.PoolCrossAvailability["FullField"]++
		}
		if c.BoardUtil.BoardShapeClass == ShapeCompact6x6 {
			rep.PoolCrossAvailability["Compact6x6"]++
		}
	}
	rep.PoolUniqueFamilies = len(famSet)
	rep.ExpandedFullFieldDiag = DiagnoseExpandedFullFieldAvailability(rep.PoolShapeDist)

	target := cfg.TargetAccepted
	if target <= 0 {
		target = 12
	}
	rep.FinalTarget = target
	minShapes := cfg.MinDistinctBoardShapes
	if minShapes <= 0 {
		minShapes = 3
	}
	minOuter := cfg.MinOuterZoneRelevant
	if minOuter <= 0 {
		minOuter = 4
	}
	maxInvFrac := cfg.MaxInventoryClassFraction
	if maxInvFrac <= 0 {
		maxInvFrac = 0.40
	}

	shapeNeed := copyQuotaMap(cfg.ShapeQuotas)
	invNeed := copyQuotaMap(cfg.InventoryQuotas)
	if len(invNeed) == 0 {
		invNeed = map[string]int{
			string(InvNo1x1): 3, string(InvOne1x1): 3,
			string(InvTwo1x1): 3, string(InvThree1x1): 3,
		}
	}

	idx := make([]int, len(pool))
	for i := range pool {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := pool[idx[i]], pool[idx[j]]
		if a.BoardUtil.OuterZoneRelevant != b.BoardUtil.OuterZoneRelevant {
			return a.BoardUtil.OuterZoneRelevant
		}
		ia, ib := rep.PoolInventoryDist[string(a.InventoryClass)], rep.PoolInventoryDist[string(b.InventoryClass)]
		if ia != ib {
			return ia < ib
		}
		sa, sb := rep.PoolShapeDist[string(a.BoardUtil.BoardShapeClass)], rep.PoolShapeDist[string(b.BoardUtil.BoardShapeClass)]
		if sa != sb {
			return sa < sb
		}
		if a.FamilyID != b.FamilyID {
			return a.FamilyID < b.FamilyID
		}
		return string(a.Embedding) < string(b.Embedding)
	})

	selected := []BoardMixAccepted{}
	usedFam := map[string]bool{}
	shapeCount := map[string]int{}
	invCount := map[string]int{}
	outerCount := 0

	maxShapeCountFor := func(frac float64) int {
		n := int(math.Floor(float64(target)*frac + 1e-9))
		if n < 1 {
			return 1
		}
		return n
	}
	maxInvCountFor := func(frac float64) int {
		n := int(math.Floor(float64(target)*frac + 1e-9))
		if n < 1 {
			return 1
		}
		return n
	}

	canTake := func(c BoardMixAccepted, maxShapeCount, maxInvCount int, relaxOuter bool) (bool, string) {
		shape := string(c.BoardUtil.BoardShapeClass)
		inv := string(c.InventoryClass)
		if cfg.UniqueFamily && usedFam[c.FamilyID] {
			return false, "FamilyDuplicate"
		}
		if maxShapeCount > 0 && shapeCount[shape] >= maxShapeCount {
			return false, "BoardShapeBalance"
		}
		if maxInvCount > 0 && invCount[inv] >= maxInvCount {
			return false, "InventoryBalance"
		}
		needOuter := minOuter - outerCount
		remain := target - len(selected)
		if !relaxOuter && needOuter > 0 && !c.BoardUtil.OuterZoneRelevant && remain <= needOuter {
			return false, "OuterZoneBalance"
		}
		return true, ""
	}

	alreadySelected := func(c BoardMixAccepted) bool {
		return usedFam[c.FamilyID]
	}

	tryPick := func(pred func(BoardMixAccepted) bool, maxShapeCount, maxInvCount int, relaxOuter bool) bool {
		for _, i := range idx {
			c := pool[i]
			if pred != nil && !pred(c) {
				continue
			}
			if alreadySelected(c) {
				rep.SelectionSkipCounters["FamilyDuplicate"]++
				continue
			}
			ok, reason := canTake(c, maxShapeCount, maxInvCount, relaxOuter)
			if !ok {
				rep.SelectionSkipCounters[reason]++
				continue
			}
			selected = append(selected, c)
			usedFam[c.FamilyID] = true
			shape := string(c.BoardUtil.BoardShapeClass)
			inv := string(c.InventoryClass)
			shapeCount[shape]++
			invCount[inv]++
			if c.BoardUtil.OuterZoneRelevant {
				outerCount++
			}
			decQuota(shapeNeed, shape)
			decQuota(invNeed, inv)
			return true
		}
		return false
	}

	inventoryOrder := []InventoryClass{InvNo1x1, InvOne1x1, InvTwo1x1, InvThree1x1}
	shapeOrder := []BoardShapeClass{
		ShapeCompact6x6, ShapeShiftedCore, ShapeExpanded, ShapeTall, ShapeWide, ShapeFullField,
	}

	runQuotaPass := func(maxShapeCount, maxInvCount int, relaxOuter bool) {
		for len(selected) < target {
			progress := false
			for _, inv := range inventoryOrder {
				if invNeed[string(inv)] <= 0 && sumQuota(invNeed) > 0 {
					continue
				}
				for _, shape := range shapeOrder {
					if len(selected) >= target {
						break
					}
					if shapeNeed[string(shape)] <= 0 && sumQuota(shapeNeed) > 0 {
						continue
					}
					if tryPick(func(c BoardMixAccepted) bool {
						return c.InventoryClass == inv && c.BoardUtil.BoardShapeClass == shape
					}, maxShapeCount, maxInvCount, relaxOuter) {
						progress = true
					}
				}
				if len(selected) < target && invNeed[string(inv)] > 0 {
					if tryPick(func(c BoardMixAccepted) bool { return c.InventoryClass == inv }, maxShapeCount, maxInvCount, relaxOuter) {
						progress = true
					}
				}
			}
			if len(selected) < target && outerCount < minOuter {
				if tryPick(func(c BoardMixAccepted) bool { return c.BoardUtil.OuterZoneRelevant }, maxShapeCount, maxInvCount, relaxOuter) {
					progress = true
				}
			}
			if len(selected) < target {
				for _, shape := range shapeOrder {
					if shapeNeed[string(shape)] <= 0 {
						continue
					}
					if tryPick(func(c BoardMixAccepted) bool { return c.BoardUtil.BoardShapeClass == shape }, maxShapeCount, maxInvCount, relaxOuter) {
						progress = true
						break
					}
				}
			}
			if !progress {
				break
			}
		}
	}

	// Stage 0: preferred fractions.
	shapeFracStages := []float64{0.40, 0.50, 0.60}
	invFrac := maxInvFrac
	runQuotaPass(maxShapeCountFor(shapeFracStages[0]), maxInvCountFor(invFrac), false)

	// Staged shape fraction relaxation to reach FinalTarget.
	for _, frac := range shapeFracStages[1:] {
		if len(selected) >= target {
			break
		}
		rep.QuotaRelaxations = append(rep.QuotaRelaxations,
			fmt.Sprintf("relax:MaxBoardShapeFraction→%.2f (selected=%d/%d)", frac, len(selected), target))
		runQuotaPass(maxShapeCountFor(frac), maxInvCountFor(invFrac), true)
		// Also fill any unique family under new cap.
		for len(selected) < target {
			if !tryPick(nil, maxShapeCountFor(frac), maxInvCountFor(invFrac), true) {
				break
			}
		}
	}

	// Inventory fraction relaxation.
	if len(selected) < target {
		rep.QuotaRelaxations = append(rep.QuotaRelaxations, "relax:MaxInventoryClassFraction→1.0")
		for len(selected) < target {
			if !tryPick(nil, maxShapeCountFor(0.60), 0, true) { // 0 = no inv cap
				break
			}
		}
	}

	// Final fill: only UniqueFamily hard (+ optional shape cap off).
	if len(selected) < target {
		rep.QuotaRelaxations = append(rep.QuotaRelaxations, "relax:fill-unique-families-only-hard-constraint")
		for len(selected) < target {
			if !tryPick(nil, 0, 0, true) {
				break
			}
		}
	}

	for shape, want := range cfg.ShapeQuotas {
		got := shapeCount[shape]
		if got < want {
			avail := rep.PoolShapeDist[shape]
			rep.QuotaRelaxations = append(rep.QuotaRelaxations,
				fmt.Sprintf("Requested %s=%d; poolAvailable=%d; Selected=%d; QuotaRelaxed=true", shape, want, avail, got))
		}
	}
	for inv, want := range cfg.InventoryQuotas {
		got := invCount[inv]
		if got < want {
			avail := rep.PoolInventoryDist[inv]
			rep.QuotaRelaxations = append(rep.QuotaRelaxations,
				fmt.Sprintf("Requested Inventory %s=%d; poolAvailable=%d; Selected=%d; QuotaRelaxed=true", inv, want, avail, got))
		}
	}

	for _, c := range selected {
		rep.FinalShapeDist[string(c.BoardUtil.BoardShapeClass)]++
		rep.FinalInventoryDist[string(c.InventoryClass)]++
		if c.BoardUtil.OuterZoneRelevant {
			rep.FinalOuterZoneRelevant++
		}
	}
	rep.DistinctBoardShapes = len(rep.FinalShapeDist)
	rep.FinalAccepted = len(selected)
	rep.MissingCount = target - len(selected)
	if rep.MissingCount < 0 {
		rep.MissingCount = 0
	}

	// --- Success / unmet semantics ---
	if rep.FinalAccepted < rep.FinalTarget {
		rep.DiversityTargetUnmet = true
		rep.UnmetRequirements = append(rep.UnmetRequirements,
			fmt.Sprintf("FinalTarget: need %d, got %d", rep.FinalTarget, rep.FinalAccepted))
		rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons,
			fmt.Sprintf("FinalAccepted %d < FinalTarget %d", rep.FinalAccepted, rep.FinalTarget))
	}
	if rep.DistinctBoardShapes < minShapes && countPositiveKeys(rep.PoolShapeDist) >= minShapes {
		rep.DiversityTargetUnmet = true
		rep.UnmetRequirements = append(rep.UnmetRequirements,
			fmt.Sprintf("MinDistinctShapes: need %d, got %d (pool had %d)", minShapes, rep.DistinctBoardShapes, countPositiveKeys(rep.PoolShapeDist)))
	} else if rep.DistinctBoardShapes < minShapes {
		rep.DiversityTargetUnmet = true
		rep.UnmetRequirements = append(rep.UnmetRequirements,
			fmt.Sprintf("MinDistinctShapes: need %d, got %d (pool only has %d shapes — generation limit)", minShapes, rep.DistinctBoardShapes, countPositiveKeys(rep.PoolShapeDist)))
	}
	if rep.FinalOuterZoneRelevant < minOuter {
		rep.UnmetRequirements = append(rep.UnmetRequirements,
			fmt.Sprintf("OuterZoneRelevant: need %d, got %d (pool had %d)", minOuter, rep.FinalOuterZoneRelevant, rep.PoolOuterZoneRelevant))
		if rep.PoolOuterZoneRelevant >= minOuter {
			rep.DiversityTargetUnmet = true
		}
	}
	for inv, want := range cfg.InventoryQuotas {
		got := rep.FinalInventoryDist[inv]
		avail := rep.PoolInventoryDist[inv]
		if got < want {
			rep.UnmetRequirements = append(rep.UnmetRequirements,
				fmt.Sprintf("Inventory %s: need %d, got %d, poolAvailable %d", inv, want, got, avail))
		}
	}
	for shape, want := range cfg.ShapeQuotas {
		got := rep.FinalShapeDist[shape]
		avail := rep.PoolShapeDist[shape]
		if want > 0 && avail == 0 {
			rep.UnmetRequirements = append(rep.UnmetRequirements,
				fmt.Sprintf("BoardShape %s: need %d, poolAvailable 0", shape, want))
		} else if got < want && avail > 0 {
			rep.UnmetRequirements = append(rep.UnmetRequirements,
				fmt.Sprintf("BoardShape %s: need %d, got %d, poolAvailable %d", shape, want, got, avail))
		}
	}

	// Preferred MaxBoardShapeFraction note (soft).
	prefFrac := cfg.MaxBoardShapeFraction
	if prefFrac <= 0 {
		prefFrac = 0.40
	}
	prefCap := maxShapeCountFor(prefFrac)
	for shape, n := range rep.FinalShapeDist {
		actualFrac := float64(n) / float64(target)
		if n > prefCap {
			rep.UnmetRequirements = append(rep.UnmetRequirements,
				fmt.Sprintf("BoardShape MaxFraction preferred: %s requested<=%.2f (cap %d/%d) actual=%.2f (%d/%d) — relaxed",
					shape, prefFrac, prefCap, target, actualFrac, n, target))
		}
	}

	// Explain why short of target.
	if rep.FinalAccepted < rep.FinalTarget {
		if rep.PoolUniqueFamilies < rep.FinalTarget {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				fmt.Sprintf("UniqueFamily hard cap: pool has only %d unique FamilyId < FinalTarget %d",
					rep.PoolUniqueFamilies, rep.FinalTarget))
		}
		if rep.PoolInventoryDist[string(InvThree1x1)] == 0 {
			rep.WhyFinalShort = append(rep.WhyFinalShort, "No Three1x1 candidates in pool (generation/enrichment did not produce any)")
		} else if rep.FinalInventoryDist[string(InvThree1x1)] == 0 {
			rep.WhyFinalShort = append(rep.WhyFinalShort, "Three1x1 existed in pool but none selected (family/shape conflict)")
		}
		if rep.PoolInventoryDist[string(InvTwo1x1)] > 0 && rep.PoolInventoryDist[string(InvTwo1x1)] < 3 {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				fmt.Sprintf("Only %d Two1x1 candidate(s) available in pool", rep.PoolInventoryDist[string(InvTwo1x1)]))
		}
		if rep.PoolInventoryDist[string(InvTwo1x1)] == 0 {
			rep.WhyFinalShort = append(rep.WhyFinalShort, "No Two1x1 candidates in pool")
		}
		// Shape monoculture / fraction wall analysis (pilot: ShiftedCore=4 Tall=1 Wide=2 under 0.40).
		for shape, n := range rep.FinalShapeDist {
			if n >= prefCap && rep.PoolShapeDist[shape] > n {
				rep.WhyFinalShort = append(rep.WhyFinalShort,
					fmt.Sprintf("Shape quota/fraction limited %s to %d (preferred frac %.2f); pool had %d more unused of that shape",
						shape, n, prefFrac, rep.PoolShapeDist[shape]-n))
			}
		}
		if rep.PoolShapeDist[string(ShapeExpanded)] == 0 && rep.PoolShapeDist[string(ShapeFullField)] == 0 {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				"No Expanded/FullField in pool — cannot fill remaining slots with those shape quotas")
		}
		remainFam := 0
		for fam := range famSet {
			if !usedFam[fam] {
				remainFam++
			}
		}
		if remainFam == 0 && rep.FinalAccepted < rep.FinalTarget {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				"After controlled relaxations, no unused unique FamilyId remained in pool")
		} else if remainFam > 0 && rep.FinalAccepted < rep.FinalTarget {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				fmt.Sprintf("BUG-or-constraint: %d unused families remain but selector could not take them; skips=%v",
					remainFam, rep.SelectionSkipCounters))
		}
		if len(rep.WhyFinalShort) == 0 {
			rep.WhyFinalShort = append(rep.WhyFinalShort,
				fmt.Sprintf("Selector stopped at %d/%d; skipCounters=%v", rep.FinalAccepted, rep.FinalTarget, rep.SelectionSkipCounters))
		}
	}

	for i := range selected {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		selected[i].CandidateID = id
		finalizeBoardMixCandidate(&selected[i], id)
	}
	return selected, rep
}

func countPositiveKeys(m map[string]int) int {
	n := 0
	for _, v := range m {
		if v > 0 {
			n++
		}
	}
	return n
}

// DiagnoseExpandedFullFieldAvailability explains missing Expanded/FullField/Compact in pools.
func DiagnoseExpandedFullFieldAvailability(poolShapeDist map[string]int) BoardShapeFeasibilityDiag {
	d := BoardShapeFeasibilityDiag{
		PoolShapeCounts: copyIntMap(poolShapeDist),
		ClassifierNotes: []string{
			"Compact6x6 requires bbox≤6x6, OuterZoneRelevant=false, and CoreOffsetY=0 with CoreOffsetX=0; but Target→exit alignment usually forces offsetX=0 or 1 — offsetX=1 yields ShiftedCore.",
			"Expanded requires PiecesOutsideOriginal6x6Core>0 or multi-row/col outer usage / OuterZoneRelevant with spread — a single outer seal-relevant board can still stay ShiftedCore if bbox stays within the 6x6 core.",
			"Tall = bbox height≥7 & width≤6; Wide = bbox width≥7 & height≤6 — these fire when outer pieces extend one axis.",
			"FullField needs ~7x8 bbox AND utilization≥0.35 — rare from a transplanted 6x6 block plus sparse outer augment.",
		},
		Native7x8Limitation: "Pipeline still primarily CW90-embeds / mirrors a Rush 6x6 block into 7x8. Shifted/Tall/Wide diversify placement of that block (and sparse outer pieces); they are not native 7x8 compositions that fill the frame by design.",
	}
	hasExp := poolShapeDist[string(ShapeExpanded)] > 0
	hasFull := poolShapeDist[string(ShapeFullField)] > 0
	hasCompact := poolShapeDist[string(ShapeCompact6x6)] > 0
	switch {
	case !hasExp && !hasFull && !hasCompact:
		d.LikelyRootCause = "C+A: classifier maps typical flush/shifted 6x6 embeds to ShiftedCore (offsetX≠0 or offsetY≥1); Expanded/FullField transforms are not produced by current embed+light-augment generation (not merely rejected by exact solver)."
		d.Summary = "Pool lacks Compact6x6/Expanded/FullField because generation is still 6x6-core-centric and classification treats offsetX=1 / offsetY≥1 cores as ShiftedCore; outer augment more often yields Tall/Wide than Expanded/FullField."
	case !hasExp && !hasFull:
		d.LikelyRootCause = "A+C: no FullField/Expanded instances generated; Tall/Wide cover most outer-axis growth."
		d.Summary = "Expanded/FullField absent from pool; outer usage classified as Tall/Wide instead."
	default:
		d.LikelyRootCause = "partial availability"
		d.Summary = "Some rare shapes present in pool."
	}
	return d
}
