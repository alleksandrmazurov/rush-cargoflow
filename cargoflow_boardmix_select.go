package rush

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type CausalCandidateReport struct {
	CandidateID                    string          `json:"candidateId"`
	FamilyID                       string          `json:"familyId"`
	CausalTemplate                 CausalTemplate  `json:"causalTemplate"`
	OptimalGestures                int             `json:"optimalGestures"`
	BaseOptimal                    int             `json:"baseOptimal"`
	CrossRegionDependencyEdgeCount int             `json:"crossRegionDependencyEdgeCount"`
	LowerToUpperDependencyEdges    int             `json:"lowerToUpperDependencyEdges"`
	SideToUpperDependencyEdges     int             `json:"sideToUpperDependencyEdges"`
	LowerToCorridorDependencyEdges int             `json:"lowerToCorridorDependencyEdges"`
	SideToCorridorDependencyEdges  int             `json:"sideToCorridorDependencyEdges"`
	CrossRegionDependencyDepth     int             `json:"crossRegionDependencyDepth"`
	RequiredLowerPieceCount        int             `json:"requiredLowerPieceCount"`
	RequiredSidePieceCount         int             `json:"requiredSidePieceCount"`
	InventoryClass                 InventoryClass  `json:"inventoryClass"`
	BoardShapeClass                BoardShapeClass `json:"boardShapeClass"`
}

type CausalMatchedFamilyComparison struct {
	FamilyID                   string  `json:"familyId"`
	BaseOptimal                int     `json:"baseOptimal"`
	CausalOptimal              int     `json:"causalOptimal"`
	BaseGraphEdges             int     `json:"baseGraphEdges"`
	CausalGraphEdges           int     `json:"causalGraphEdges"`
	BaseGraphDepth             int     `json:"baseGraphDepth"`
	CausalGraphDepth           int     `json:"causalGraphDepth"`
	BaseCrossRegionEdges       int     `json:"baseCrossRegionEdges"`
	CausalCrossRegionEdges     int     `json:"causalCrossRegionEdges"`
	BaseDependencyRowSpan      int     `json:"baseDependencyRowSpan"`
	CausalDependencyRowSpan    int     `json:"causalDependencyRowSpan"`
	BaseDependencyColumnSpan   int     `json:"baseDependencyColumnSpan"`
	CausalDependencyColumnSpan int     `json:"causalDependencyColumnSpan"`
	BaseContainment            float64 `json:"baseContainment"`
	CausalContainment          float64 `json:"causalContainment"`
	PullLeftAvailable          bool    `json:"pullLeftAvailable"`
	PullLeftOptimal            int     `json:"pullLeftOptimal,omitempty"`
	PullLeftGraphEdges         int     `json:"pullLeftGraphEdges,omitempty"`
	PullLeftGraphDepth         int     `json:"pullLeftGraphDepth,omitempty"`
	PullLeftCrossRegionEdges   int     `json:"pullLeftCrossRegionEdges,omitempty"`
	PullLeftDependencyRowSpan  int     `json:"pullLeftDependencyRowSpan,omitempty"`
	PullLeftDependencyColSpan  int     `json:"pullLeftDependencyColumnSpan,omitempty"`
	PullLeftContainment        float64 `json:"pullLeftContainment,omitempty"`
}

// BoardMixSelectReport explains pool vs final diversity selection and feasibility.
type BoardMixSelectReport struct {
	FinalTarget             int                       `json:"finalTarget"`
	FinalAccepted           int                       `json:"finalAccepted"`
	MissingCount            int                       `json:"missingCount"`
	PoolSize                int                       `json:"poolSize"`
	PoolUniqueFamilies      int                       `json:"poolUniqueFamilies"`
	PoolShapeDist           map[string]int            `json:"poolBoardShapeClass"`
	PoolInventoryDist       map[string]int            `json:"poolInventoryClass"`
	PoolOuterZoneRelevant   int                       `json:"poolOuterZoneRelevant"`
	PoolDifficultyBands     map[string]int            `json:"poolDifficultyBands"`
	PoolShapeInventoryCross map[string]map[string]int `json:"poolShapeInventoryCross"`
	PoolCrossAvailability   map[string]int            `json:"poolCrossAvailability"`
	FinalShapeDist          map[string]int            `json:"finalBoardShapeClass"`
	FinalInventoryDist      map[string]int            `json:"finalInventoryClass"`
	FinalOuterZoneRelevant  int                       `json:"finalOuterZoneRelevant"`
	DistinctBoardShapes     int                       `json:"distinctBoardShapes"`
	SelectionSkipCounters   map[string]int            `json:"selectionSkipCounters"`
	QuotaRelaxations        []string                  `json:"quotaRelaxations"`
	UnmetRequirements       []string                  `json:"unmetRequirements"`
	WhyFinalShort           []string                  `json:"whyFinalShort,omitempty"`
	DiversityTargetUnmet    bool                      `json:"diversityTargetUnmet"`
	DiversityUnmetReasons   []string                  `json:"diversityUnmetReasons,omitempty"`
	ExpandedFullFieldDiag   BoardShapeFeasibilityDiag `json:"expandedFullFieldDiagnostic"`
	HardConstraints         []string                  `json:"hardConstraints"`
	SoftConstraints         []string                  `json:"softConstraints"`
	// RUSH-010.5.1 genuine core-expansion selection diagnostics.
	PoolGenuineCoreExpanded               int            `json:"poolGenuineCoreExpanded"`
	PoolGenuineCoreExpandedUniqueFamilies int            `json:"poolGenuineCoreExpandedUniqueFamilies"`
	FinalGenuineCoreExpanded              int            `json:"finalGenuineCoreExpanded"`
	FinalGenuineCoreExpandedTarget        int            `json:"finalGenuineCoreExpandedTarget"`
	FinalCoreExpansionClass               map[string]int `json:"finalCoreExpansionClass,omitempty"`
	FinalContainmentValues                []float64      `json:"finalContainmentValues,omitempty"`
	FinalFits6x6False                     int            `json:"finalFits6x6False"`
	FinalIsolated1x1Suspect               int            `json:"finalIsolated1x1Suspect"`
	GenuineCoreExpandedMonocultureNote    string         `json:"genuineCoreExpandedMonocultureNote,omitempty"`
	// RUSH-010.7 causal synthesis diagnostics.
	PoolCausalExpanded               int                             `json:"poolCausalExpanded"`
	PoolCausalExpandedUniqueFamilies int                             `json:"poolCausalExpandedUniqueFamilies"`
	FinalCausalExpanded              int                             `json:"finalCausalExpanded"`
	FinalCausalExpandedTarget        int                             `json:"finalCausalExpandedTarget"`
	CausalTemplateDistribution       map[string]int                  `json:"causalTemplateDistribution,omitempty"`
	CrossRegionEdgeDistribution      map[string]int                  `json:"crossRegionEdgeDistribution,omitempty"`
	LowerToUpperCount                int                             `json:"lowerToUpperCount"`
	SideToUpperCount                 int                             `json:"sideToUpperCount"`
	MultiRegionChainCount            int                             `json:"multiRegionChainCount"`
	MatchedFamilyComparison          []CausalMatchedFamilyComparison `json:"matchedFamilyComparison,omitempty"`
	HumanValidationCandidates        []CausalCandidateReport         `json:"humanValidationCandidates,omitempty"`
}

// BoardShapeFeasibilityDiag explains missing Expanded/FullField/Compact classes.
type BoardShapeFeasibilityDiag struct {
	Summary             string         `json:"summary"`
	ClassifierNotes     []string       `json:"classifierNotes"`
	PoolShapeCounts     map[string]int `json:"poolShapeCounts"`
	LikelyRootCause     string         `json:"likelyRootCause"`
	Native7x8Limitation string         `json:"native7x8Limitation"`
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
			"MinDistinctInventoryClasses if pool supports it",
			"FinalAccepted == FinalTarget for DiversityUnmet=false",
		},
		SoftConstraints: []string{
			"InventoryTargets preferred (Three1x1 is rare/soft, not hard 3/12)",
			"MaxBoardShapeFraction 0.40→0.50→0.60 (staged)",
			"MaxInventoryClassFraction (relaxable)",
			"Exact shape quota counts (relaxable)",
			"Compact6x6 not required from native generator",
		},
	}

	famSet := map[string]bool{}
	genuineFam := map[string]bool{}
	causalFam := map[string]bool{}
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
		if IsGenuineCoreExpanded(c) {
			rep.PoolGenuineCoreExpanded++
			genuineFam[c.FamilyID] = true
		}
		if IsCausalExpanded(c) {
			rep.PoolCausalExpanded++
			causalFam[c.FamilyID] = true
		}
	}
	rep.PoolUniqueFamilies = len(famSet)
	rep.PoolGenuineCoreExpandedUniqueFamilies = len(genuineFam)
	rep.PoolCausalExpandedUniqueFamilies = len(causalFam)
	rep.ExpandedFullFieldDiag = DiagnoseExpandedFullFieldAvailability(rep.PoolShapeDist)

	target := cfg.TargetAccepted
	if target <= 0 {
		target = 12
	}
	rep.FinalTarget = target
	targetGenuine := cfg.TargetGenuineCoreExpanded
	if targetGenuine < 0 {
		targetGenuine = 0
	}
	if targetGenuine > target {
		targetGenuine = target
	}
	rep.FinalGenuineCoreExpandedTarget = targetGenuine
	targetCausal := cfg.TargetCausalExpanded
	if targetCausal < 0 {
		targetCausal = 0
	}
	if targetCausal > target {
		targetCausal = target
	}
	rep.FinalCausalExpandedTarget = targetCausal
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
			string(InvTwo1x1): 3, // Three1x1 intentionally omitted as hard preferred quota
		}
	}
	minInvClasses := cfg.MinDistinctInventoryClasses
	if minInvClasses <= 0 {
		minInvClasses = 3
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

	takeCandidate := func(c BoardMixAccepted) {
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
			takeCandidate(c)
			return true
		}
		return false
	}

	// RUSH-010.7: reserve proof-backed causal slots before geometric balancing.
	if targetCausal > 0 {
		rep.HardConstraints = append(rep.HardConstraints,
			fmt.Sprintf("TargetCausalExpanded=%d (proof-backed, unique FamilyId)", targetCausal))
		usedTemplate := map[CausalTemplate]bool{}
		usedEdgeSignature := map[string]bool{}
		usedCausalInventory := map[InventoryClass]bool{}
		for len(selected) < targetCausal {
			bestIndex, bestScore := -1, math.Inf(-1)
			for i, c := range pool {
				if !IsCausalExpanded(c) || usedFam[c.FamilyID] {
					continue
				}
				score := c.CausalProof.DistributedCausalityScore
				if c.CausalProof.OptimalDelta >= 0 && c.CausalProof.OptimalDelta <= 10 {
					score += 25
				} else if c.CausalProof.OptimalDelta < 0 {
					score -= 50
				}
				if !usedTemplate[c.CausalTemplate] {
					score += 1000
				}
				signature := causalEdgeSignature(c.CausalProof)
				if !usedEdgeSignature[signature] {
					score += 500
				}
				if !usedCausalInventory[c.InventoryClass] {
					score += 100
				}
				if c.CausalProof.MultiRegionChain {
					score += 50
				}
				if score > bestScore || (score == bestScore && c.FamilyID < pool[bestIndex].FamilyID) {
					bestIndex, bestScore = i, score
				}
			}
			if bestIndex < 0 {
				break
			}
			candidate := pool[bestIndex]
			takeCandidate(candidate)
			usedTemplate[candidate.CausalTemplate] = true
			usedEdgeSignature[causalEdgeSignature(candidate.CausalProof)] = true
			usedCausalInventory[candidate.InventoryClass] = true
		}
	}

	// RUSH-010.5.1: reserve genuine core-expanded slots first (unique FamilyId).
	if targetGenuine > 0 {
		rep.HardConstraints = append(rep.HardConstraints,
			fmt.Sprintf("TargetGenuineCoreExpanded=%d (unique FamilyId)", targetGenuine))
		rep.SoftConstraints = append(rep.SoftConstraints,
			"Genuine ranking: lower Best6x6Containment, higher DepRows/Cols, higher OuterDependencyMoves")
		genuineIdx := []int{}
		for i, c := range pool {
			if IsGenuineCoreExpanded(c) {
				genuineIdx = append(genuineIdx, i)
			}
		}
		sort.SliceStable(genuineIdx, func(i, j int) bool {
			return compareGenuineCoreExpanded(pool[genuineIdx[i]], pool[genuineIdx[j]]) < 0
		})
		for _, i := range genuineIdx {
			if len(selected) >= targetGenuine {
				break
			}
			c := pool[i]
			if usedFam[c.FamilyID] {
				continue
			}
			takeCandidate(c)
		}
		noteGenuineMonoculture(&rep, selected)
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

	for selectedIndex, c := range selected {
		rep.FinalShapeDist[string(c.BoardUtil.BoardShapeClass)]++
		rep.FinalInventoryDist[string(c.InventoryClass)]++
		if c.BoardUtil.OuterZoneRelevant {
			rep.FinalOuterZoneRelevant++
		}
		cls := string(c.CoreExpansionClass)
		if cls == "" {
			cls = string(CoreExpNone)
		}
		if rep.FinalCoreExpansionClass == nil {
			rep.FinalCoreExpansionClass = map[string]int{}
		}
		rep.FinalCoreExpansionClass[cls]++
		if c.CoreSpace != nil {
			rep.FinalContainmentValues = append(rep.FinalContainmentValues, c.CoreSpace.Best6x6MeaningfulContainmentRatio)
			if !c.CoreSpace.CanMeaningfulStructureFitInAny6x6 {
				rep.FinalFits6x6False++
			}
			if c.CoreSpace.IsolatedAddon1x1Suspect {
				rep.FinalIsolated1x1Suspect++
			}
		}
		if IsGenuineCoreExpanded(c) {
			rep.FinalGenuineCoreExpanded++
		}
		if IsCausalExpanded(c) {
			rep.FinalCausalExpanded++
			if rep.CausalTemplateDistribution == nil {
				rep.CausalTemplateDistribution = map[string]int{}
			}
			if rep.CrossRegionEdgeDistribution == nil {
				rep.CrossRegionEdgeDistribution = map[string]int{}
			}
			rep.CausalTemplateDistribution[string(c.CausalTemplate)]++
			for edgeType, count := range c.CausalProof.EdgeTypeDistribution {
				rep.CrossRegionEdgeDistribution[edgeType] += count
			}
			if c.CausalProof.RequiredLowerPieceCount > 0 {
				rep.LowerToUpperCount++
			}
			if c.CausalProof.RequiredSidePieceCount > 0 ||
				c.CausalProof.SideToUpperDependencyEdges+c.CausalProof.SideToCorridorDependencyEdges > 0 {
				rep.SideToUpperCount++
			}
			if c.CausalProof.MultiRegionChain {
				rep.MultiRegionChainCount++
			}
			rep.HumanValidationCandidates = append(rep.HumanValidationCandidates, CausalCandidateReport{
				CandidateID: fmt.Sprintf("Candidate_%03d", selectedIndex+1),
				FamilyID:    c.FamilyID, CausalTemplate: c.CausalTemplate,
				OptimalGestures: c.OptimalGestures, BaseOptimal: c.CausalProof.BaseOptimal,
				CrossRegionDependencyEdgeCount: c.CausalProof.CrossRegionDependencyEdgeCount,
				LowerToUpperDependencyEdges:    c.CausalProof.LowerToUpperDependencyEdges,
				SideToUpperDependencyEdges:     c.CausalProof.SideToUpperDependencyEdges,
				LowerToCorridorDependencyEdges: c.CausalProof.LowerToCorridorDependencyEdges,
				SideToCorridorDependencyEdges:  c.CausalProof.SideToCorridorDependencyEdges,
				CrossRegionDependencyDepth:     c.CausalProof.CrossRegionDependencyDepth,
				RequiredLowerPieceCount:        c.CausalProof.RequiredLowerPieceCount,
				RequiredSidePieceCount:         c.CausalProof.RequiredSidePieceCount,
				InventoryClass:                 c.InventoryClass, BoardShapeClass: c.BoardUtil.BoardShapeClass,
			})
			rep.MatchedFamilyComparison = append(rep.MatchedFamilyComparison, CausalMatchedFamilyComparison{
				FamilyID: c.FamilyID, BaseOptimal: c.CausalProof.BaseOptimal,
				CausalOptimal:  c.CausalProof.NativeOptimal,
				BaseGraphEdges: c.CausalProof.BaseGraphEdges, CausalGraphEdges: c.CausalProof.GraphEdges,
				BaseGraphDepth: c.CausalProof.BaseGraphDepth, CausalGraphDepth: c.CausalProof.GraphDepth,
				BaseCrossRegionEdges:       c.CausalProof.BaseCrossRegionEdges,
				CausalCrossRegionEdges:     c.CausalProof.CrossRegionDependencyEdgeCount,
				BaseDependencyRowSpan:      c.CausalProof.BaseDependencyRowSpan,
				CausalDependencyRowSpan:    c.CausalProof.DependencyRowSpan,
				BaseDependencyColumnSpan:   c.CausalProof.BaseDependencyColumnSpan,
				CausalDependencyColumnSpan: c.CausalProof.DependencyColumnSpan,
				BaseContainment:            c.CausalProof.BaseContainment,
				CausalContainment:          c.CausalProof.Best6x6Containment,
			})
		}
	}
	attachPullLeftMatchedComparisons(&rep, pool)
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
	if targetGenuine > 0 &&
		rep.PoolGenuineCoreExpandedUniqueFamilies >= targetGenuine &&
		rep.FinalGenuineCoreExpanded < targetGenuine {
		rep.DiversityTargetUnmet = true
		msg := fmt.Sprintf("TargetGenuineCoreExpanded: need %d, got %d (pool unique genuine families %d)",
			targetGenuine, rep.FinalGenuineCoreExpanded, rep.PoolGenuineCoreExpandedUniqueFamilies)
		rep.UnmetRequirements = append(rep.UnmetRequirements, msg)
		rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons, msg)
	}
	if targetCausal > 0 &&
		rep.PoolCausalExpandedUniqueFamilies >= targetCausal &&
		rep.FinalCausalExpanded < targetCausal {
		rep.DiversityTargetUnmet = true
		msg := fmt.Sprintf("TargetCausalExpanded: need %d, got %d (pool unique causal families %d)",
			targetCausal, rep.FinalCausalExpanded, rep.PoolCausalExpandedUniqueFamilies)
		rep.UnmetRequirements = append(rep.UnmetRequirements, msg)
		rep.DiversityUnmetReasons = append(rep.DiversityUnmetReasons, msg)
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
			msg := fmt.Sprintf("Inventory %s: need %d, got %d, poolAvailable %d", inv, want, got, avail)
			if inv == string(InvThree1x1) {
				msg += " (soft/rare — not a hard pilot quota)"
			}
			rep.UnmetRequirements = append(rep.UnmetRequirements, msg)
		}
	}
	invClassCount := countPositiveKeys(rep.FinalInventoryDist)
	if invClassCount < minInvClasses {
		poolInvClasses := countPositiveKeys(rep.PoolInventoryDist)
		rep.UnmetRequirements = append(rep.UnmetRequirements,
			fmt.Sprintf("MinDistinctInventoryClasses: need %d, got %d (pool had %d)", minInvClasses, invClassCount, poolInvClasses))
		if poolInvClasses >= minInvClasses && rep.FinalAccepted >= rep.FinalTarget {
			rep.DiversityTargetUnmet = true
		} else if poolInvClasses >= minInvClasses && invClassCount < minInvClasses {
			rep.DiversityTargetUnmet = true
		}
	}
	for shape, want := range cfg.ShapeQuotas {
		got := rep.FinalShapeDist[shape]
		avail := rep.PoolShapeDist[shape]
		if want > 0 && avail == 0 {
			msg := fmt.Sprintf("BoardShape %s: need %d, poolAvailable 0", shape, want)
			if shape == string(ShapeCompact6x6) {
				msg += " (Compact not required from native generator)"
			}
			rep.UnmetRequirements = append(rep.UnmetRequirements, msg)
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

// IsGenuineCoreExpanded reports whether a candidate is a non-decorative structural core expansion
// whose meaningful structure does not fit in any 6×6 window and is not an isolated-1x1 suspect.
func IsGenuineCoreExpanded(c BoardMixAccepted) bool {
	if c.CoreExpansionClass == "" || c.CoreExpansionClass == CoreExpNone {
		return false
	}
	if c.CoreSpace == nil {
		return false
	}
	if c.CoreSpace.CanMeaningfulStructureFitInAny6x6 {
		return false
	}
	if c.CoreSpace.IsolatedAddon1x1Suspect {
		return false
	}
	return true
}

// compareGenuineCoreExpanded returns <0 if a should rank before b (preferred).
func compareGenuineCoreExpanded(a, b BoardMixAccepted) int {
	ra, rb := 1.0, 1.0
	da, db := 0, 0
	oa, ob := 0, 0
	if a.CoreSpace != nil {
		ra = a.CoreSpace.Best6x6MeaningfulContainmentRatio
		da = a.CoreSpace.DependencyColumnsUsed + a.CoreSpace.DependencyRowsUsed
		oa = a.CoreSpace.OuterDependencyMoves
	}
	if b.CoreSpace != nil {
		rb = b.CoreSpace.Best6x6MeaningfulContainmentRatio
		db = b.CoreSpace.DependencyColumnsUsed + b.CoreSpace.DependencyRowsUsed
		ob = b.CoreSpace.OuterDependencyMoves
	}
	if ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	if da != db {
		if da > db {
			return -1
		}
		return 1
	}
	if oa != ob {
		if oa > ob {
			return -1
		}
		return 1
	}
	if a.FamilyID != b.FamilyID {
		if a.FamilyID < b.FamilyID {
			return -1
		}
		return 1
	}
	return 0
}

func noteGenuineMonoculture(rep *BoardMixSelectReport, selected []BoardMixAccepted) {
	classes, shapes, invs := map[string]int{}, map[string]int{}, map[string]int{}
	n := 0
	for _, c := range selected {
		if !IsGenuineCoreExpanded(c) {
			continue
		}
		n++
		classes[string(c.CoreExpansionClass)]++
		shapes[string(c.BoardUtil.BoardShapeClass)]++
		invs[string(c.InventoryClass)]++
	}
	if n == 0 {
		return
	}
	if len(classes) == 1 && len(shapes) == 1 && len(invs) == 1 {
		var cls, shape, inv string
		for k := range classes {
			cls = k
		}
		for k := range shapes {
			shape = k
		}
		for k := range invs {
			inv = k
		}
		rep.GenuineCoreExpandedMonocultureNote = fmt.Sprintf(
			"All %d reserved genuine-expanded slots are %s / %s / %s — adequate for human validation of the visual 6x6-in-7x8 pattern, but NOT sufficient template diversity for a mass factory (defer broader CoreExpansion templates to a later milestone).",
			n, cls, shape, inv)
	}
}

func causalEdgeSignature(proof *CausalProof) string {
	if proof == nil {
		return "none"
	}
	parts := []string{}
	if proof.LowerToUpperDependencyEdges > 0 {
		parts = append(parts, "L>U")
	}
	if proof.LowerToCorridorDependencyEdges > 0 {
		parts = append(parts, "L>C")
	}
	if proof.SideToUpperDependencyEdges > 0 {
		parts = append(parts, "S>U")
	}
	if proof.SideToCorridorDependencyEdges > 0 {
		parts = append(parts, "S>C")
	}
	if proof.MultiRegionChain {
		parts = append(parts, "multi")
	}
	if len(parts) == 0 {
		return "cross-other"
	}
	return strings.Join(parts, "+")
}

func attachPullLeftMatchedComparisons(rep *BoardMixSelectReport, pool []BoardMixAccepted) {
	if rep == nil || len(rep.MatchedFamilyComparison) == 0 {
		return
	}
	pullByFamily := map[string]BoardMixAccepted{}
	for _, candidate := range pool {
		if candidate.CoreExpansionClass != CoreExpDependencyPullLeft ||
			candidate.Board == nil || !candidate.Solution.Solvable {
			continue
		}
		if _, exists := pullByFamily[candidate.FamilyID]; !exists {
			pullByFamily[candidate.FamilyID] = candidate
		}
	}
	for i := range rep.MatchedFamilyComparison {
		matched := &rep.MatchedFamilyComparison[i]
		pull, ok := pullByFamily[matched.FamilyID]
		if !ok {
			continue
		}
		spatial := ComputeSpatialDependencyMetrics(
			pull.Board, pull.Solution, DefaultCargoFlowSolveBudget())
		core := pull.CoreSpace
		if core == nil {
			computed := ComputeCoreSpaceMetrics(
				pull.Board, pull.OffsetX, pull.OffsetY, pull.Solution, DefaultCargoFlowSolveBudget())
			core = &computed
		}
		matched.PullLeftAvailable = true
		matched.PullLeftOptimal = pull.Solution.NumMoves
		matched.PullLeftGraphEdges = spatial.DependencyGraphEdgeCount
		matched.PullLeftGraphDepth = spatial.DependencyGraphDepth
		matched.PullLeftCrossRegionEdges = spatial.CrossTargetDependencies
		matched.PullLeftDependencyRowSpan = spatial.DependencyRowSpan
		matched.PullLeftDependencyColSpan = spatial.DependencyColumnSpan
		matched.PullLeftContainment = core.Best6x6MeaningfulContainmentRatio
	}
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
		d.LikelyRootCause = "A: pre-native pipeline was 6x6-core-centric (CW90 embed + sparse outer1x1). RUSH-010.4 native structural templates (SideGate/OuterParking/LongPiece/…) are required to produce Expanded/FullField with essential outer interaction."
		d.Summary = "Without native augmentation, pool lacks Compact6x6/Expanded/FullField; enable tryNativeAugment to generate interacting pieces in free 7x8 space."
	case !hasExp && !hasFull:
		d.LikelyRootCause = "A+C: Tall/Wide cover single-axis outer growth; Expanded/FullField need both-axis native templates + essential outer."
		d.Summary = "Expanded/FullField absent; prefer DoubleCorridor/CrossDependency/SideGate native templates."
	default:
		d.LikelyRootCause = "partial availability"
		d.Summary = "Some rare shapes present in pool."
	}
	d.Native7x8Limitation = "Rush-derived core remains the logical seed; native augmentation must add interacting Cargo pieces so final geometry is a true 7x8 structure, not a shifted 6x6 block."
	return d
}
