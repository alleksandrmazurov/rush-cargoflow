package rush

import (
	"fmt"
	"math"
	"sort"
)

// BoardMixSelectReport explains pool vs final diversity selection.
type BoardMixSelectReport struct {
	PoolSize                 int            `json:"poolSize"`
	PoolShapeDist            map[string]int `json:"poolBoardShapeClass"`
	PoolInventoryDist        map[string]int `json:"poolInventoryClass"`
	PoolOuterZoneRelevant    int            `json:"poolOuterZoneRelevant"`
	FinalShapeDist           map[string]int `json:"finalBoardShapeClass"`
	FinalInventoryDist       map[string]int `json:"finalInventoryClass"`
	FinalOuterZoneRelevant   int            `json:"finalOuterZoneRelevant"`
	DistinctBoardShapes      int            `json:"distinctBoardShapes"`
	SelectionSkipCounters    map[string]int `json:"selectionSkipCounters"`
	QuotaRelaxations         []string       `json:"quotaRelaxations"`
	DiversityTargetUnmet     bool           `json:"diversityTargetUnmet"`
	DiversityUnmetReasons    []string       `json:"diversityUnmetReasons,omitempty"`
}

// SelectBoardMixShortlist picks a diversity-aware final shortlist from a solved pool.
// Deterministic for a fixed pool order + config (same seed ⇒ same generation order ⇒ same pool).
func SelectBoardMixShortlist(pool []BoardMixAccepted, cfg BoardMixConfig) ([]BoardMixAccepted, BoardMixSelectReport) {
	rep := BoardMixSelectReport{
		PoolSize:              len(pool),
		PoolShapeDist:         map[string]int{},
		PoolInventoryDist:     map[string]int{},
		FinalShapeDist:        map[string]int{},
		FinalInventoryDist:    map[string]int{},
		SelectionSkipCounters: map[string]int{},
		QuotaRelaxations:      []string{},
	}
	for _, c := range pool {
		rep.PoolShapeDist[string(c.BoardUtil.BoardShapeClass)]++
		rep.PoolInventoryDist[string(c.InventoryClass)]++
		if c.BoardUtil.OuterZoneRelevant {
			rep.PoolOuterZoneRelevant++
		}
	}

	target := cfg.TargetAccepted
	if target <= 0 {
		target = 12
	}
	minShapes := cfg.MinDistinctBoardShapes
	if minShapes <= 0 {
		minShapes = 3
	}
	maxShapeFrac := cfg.MaxBoardShapeFraction
	if maxShapeFrac <= 0 {
		maxShapeFrac = 0.40
	}
	maxInvFrac := cfg.MaxInventoryClassFraction
	if maxInvFrac <= 0 {
		maxInvFrac = 0.40
	}
	minOuter := cfg.MinOuterZoneRelevant
	if minOuter <= 0 {
		minOuter = 4
	}
	maxShapeCount := int(math.Floor(float64(target)*maxShapeFrac + 1e-9))
	if maxShapeCount < 1 {
		maxShapeCount = 1
	}
	maxInvCount := int(math.Floor(float64(target)*maxInvFrac + 1e-9))
	if maxInvCount < 1 {
		maxInvCount = 1
	}

	shapeNeed := copyQuotaMap(cfg.ShapeQuotas)
	invNeed := copyQuotaMap(cfg.InventoryQuotas)
	if len(invNeed) == 0 {
		invNeed = map[string]int{
			string(InvNo1x1): 3, string(InvOne1x1): 3,
			string(InvTwo1x1): 3, string(InvThree1x1): 3,
		}
	}

	// Stable order: prefer outer-relevant, then underfilled shape/inv, then lower candidate key.
	idx := make([]int, len(pool))
	for i := range pool {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := pool[idx[i]], pool[idx[j]]
		if a.BoardUtil.OuterZoneRelevant != b.BoardUtil.OuterZoneRelevant {
			return a.BoardUtil.OuterZoneRelevant
		}
		// Prefer rarer inventory/shape in pool (inverse frequency).
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

	canTake := func(c BoardMixAccepted, relaxShape, relaxInv, relaxOuter bool) (bool, string) {
		shape := string(c.BoardUtil.BoardShapeClass)
		inv := string(c.InventoryClass)
		if cfg.UniqueFamily && usedFam[c.FamilyID] {
			return false, "FamilyDuplicate"
		}
		if shapeCount[shape] >= maxShapeCount && !relaxShape {
			return false, "BoardShapeBalance"
		}
		if invCount[inv] >= maxInvCount && !relaxInv {
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
		for _, s := range selected {
			if s.FamilyID == c.FamilyID {
				return true
			}
		}
		return false
	}

	tryPick := func(pred func(BoardMixAccepted) bool, relaxShape, relaxInv, relaxOuter bool) bool {
		for _, i := range idx {
			c := pool[i]
			if pred != nil && !pred(c) {
				continue
			}
			if alreadySelected(c) {
				rep.SelectionSkipCounters["FamilyDuplicate"]++
				continue
			}
			ok, reason := canTake(c, relaxShape, relaxInv, relaxOuter)
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

	// Pass 1: quota round-robin inventory × shape, prefer outer.
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
				}, false, false, false) {
					progress = true
				}
			}
			// Same inventory, any shape.
			if len(selected) < target && invNeed[string(inv)] > 0 {
				if tryPick(func(c BoardMixAccepted) bool { return c.InventoryClass == inv }, false, false, false) {
					progress = true
				}
			}
		}
		// Outer-relevant fill.
		if len(selected) < target && outerCount < minOuter {
			if tryPick(func(c BoardMixAccepted) bool { return c.BoardUtil.OuterZoneRelevant }, false, false, false) {
				progress = true
			}
		}
		// Any underfilled shape.
		if len(selected) < target {
			for _, shape := range shapeOrder {
				if shapeNeed[string(shape)] <= 0 {
					continue
				}
				if tryPick(func(c BoardMixAccepted) bool { return c.BoardUtil.BoardShapeClass == shape }, false, false, false) {
					progress = true
					break
				}
			}
		}
		if !progress {
			break
		}
	}

	// Pass 2: controlled relaxations.
	if len(selected) < target {
		rep.QuotaRelaxations = append(rep.QuotaRelaxations, "relax:fill-any-unique-family-with-fraction-caps")
		for len(selected) < target {
			if !tryPick(nil, false, false, true) {
				break
			}
		}
	}
	if len(selected) < target {
		rep.QuotaRelaxations = append(rep.QuotaRelaxations, "relax:inventory-fraction-cap")
		for len(selected) < target {
			if !tryPick(nil, false, true, true) {
				break
			}
		}
	}
	if len(selected) < target {
		rep.QuotaRelaxations = append(rep.QuotaRelaxations, "relax:boardshape-fraction-cap")
		for len(selected) < target {
			if !tryPick(nil, true, true, true) {
				break
			}
		}
	}

	// Log shape quota shortfalls.
	for shape, need := range cfg.ShapeQuotas {
		got := shapeCount[shape]
		want := need
		if got < want {
			rep.QuotaRelaxations = append(rep.QuotaRelaxations,
				fmt.Sprintf("Requested %s = %d; Selected = %d; QuotaRelaxed = true", shape, want, got))
		}
	}
	for inv, need := range invNeed {
		// compare against original targets
		_ = inv
		_ = need
	}
	for inv, want := range cfg.InventoryQuotas {
		got := invCount[inv]
		if got < want {
			rep.QuotaRelaxations = append(rep.QuotaRelaxations,
				fmt.Sprintf("Requested Inventory %s = %d; Selected = %d; QuotaRelaxed = true", inv, want, got))
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

	if rep.DistinctBoardShapes < minShapes {
		rep.DiversityTargetUnmet = true
		rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons,
			fmt.Sprintf("distinct BoardShapeClass=%d < min=%d", rep.DistinctBoardShapes, minShapes))
	}
	if rep.FinalOuterZoneRelevant < minOuter {
		rep.DiversityTargetUnmet = true
		rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons,
			fmt.Sprintf("OuterZoneRelevant=%d < min=%d", rep.FinalOuterZoneRelevant, minOuter))
	}
	if len(rep.FinalInventoryDist) < 2 && target >= 8 {
		// Soft signal when inventory collapsed.
		if rep.PoolInventoryDist[string(InvOne1x1)]+rep.PoolInventoryDist[string(InvTwo1x1)]+rep.PoolInventoryDist[string(InvThree1x1)] > 0 {
			rep.DiversityTargetUnmet = true
			rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons, "inventory diversity collapsed despite pool alternatives")
		}
	}
	// Max fraction hard check on final.
	for shape, n := range rep.FinalShapeDist {
		if n > maxShapeCount && rep.PoolShapeDist[shape] < len(pool) {
			// only flag if alternatives existed in other shapes
			other := 0
			for s, m := range rep.PoolShapeDist {
				if s != shape {
					other += m
				}
			}
			if other > 0 && n > maxShapeCount {
				rep.DiversityTargetUnmet = true
				rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons,
					fmt.Sprintf("%s fraction %d/%d exceeds max %d despite alternatives", shape, n, target, maxShapeCount))
			}
		}
	}

	// Assign sequential IDs for final shortlist.
	for i := range selected {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		selected[i].CandidateID = id
		finalizeBoardMixCandidate(&selected[i], id)
	}
	return selected, rep
}
