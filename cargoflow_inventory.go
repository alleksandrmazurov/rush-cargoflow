package rush

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const EnrichmentInventoryVersion = "enrich-inventory-v1"

// InventoryClass labels movable-1x1 count buckets (not difficulty).
type InventoryClass string

const (
	InvNo1x1    InventoryClass = "No1x1"
	InvOne1x1   InventoryClass = "One1x1"
	InvTwo1x1   InventoryClass = "Two1x1"
	InvThree1x1 InventoryClass = "Three1x1"
)

// CargoInventorySignature is an explainable piece-count fingerprint.
type CargoInventorySignature struct {
	Movable1x1Count  int    `json:"movable1x1Count"`
	Movable1x2HCount int    `json:"movable1x2HCount"`
	Movable1x2VCount int    `json:"movable1x2VCount"`
	Movable1x3HCount int    `json:"movable1x3HCount"`
	Movable1x3VCount int    `json:"movable1x3VCount"`
	Static1x1Count   int    `json:"static1x1Count"`
	TotalMovableCount int   `json:"totalMovableCount"`
	TotalPieceCount  int    `json:"totalPieceCount"`
	Signature        string `json:"signature"`
	InventoryClass   InventoryClass `json:"inventoryClass"`
}

// Spatial1x1Diag is a cheap spatial spread diagnostic for added 1x1s.
type Spatial1x1Diag struct {
	DistinctRowsUsed    int `json:"distinctRowsUsed"`
	DistinctColumnsUsed int `json:"distinctColumnsUsed"`
	BoardRegionsUsed    int `json:"boardRegionsUsed"` // 2x2 quadrants occupied
	MaxPairManhattan    int `json:"maxPairManhattan"`
	MinPairManhattan    int `json:"minPairManhattan"`
	Clustered           bool `json:"clustered"` // all pairs manhattan <= 1
}

// CubeRelevanceDiag is per-added-1x1 gameplay relevance.
type CubeRelevanceDiag struct {
	Cell        int          `json:"cell"`
	PieceIndex  int          `json:"pieceIndex"`
	InOptimal   bool         `json:"inOptimal"`
	Essential   bool         `json:"essential"`
	Relevant    bool         `json:"relevant"`
	InCorridor  bool         `json:"inCorridor"`
	Role        OneByOneRole `json:"role"`
	Evidence    string       `json:"evidence"`
}

// BuildCargoInventorySignature counts piece types on a Cargo Flow board.
func BuildCargoInventorySignature(board *Board) CargoInventorySignature {
	s := CargoInventorySignature{}
	for _, p := range board.Pieces {
		switch {
		case p.Kind == PieceUnit || (p.Size == 1 && p.Kind != PieceTarget):
			s.Movable1x1Count++
		case p.Size == 2 && p.Orientation == Horizontal:
			s.Movable1x2HCount++
		case p.Size == 2 && p.Orientation == Vertical:
			s.Movable1x2VCount++
		case p.Size == 3 && p.Orientation == Horizontal:
			s.Movable1x3HCount++
		case p.Size == 3 && p.Orientation == Vertical:
			s.Movable1x3VCount++
		}
		s.TotalMovableCount++
	}
	s.Static1x1Count = len(board.Walls)
	s.TotalPieceCount = len(board.Pieces) + len(board.Walls)
	s.InventoryClass = inventoryClassFromCount(s.Movable1x1Count)
	s.Signature = fmt.Sprintf("1x1:%d,1x2H:%d,1x2V:%d,1x3H:%d,1x3V:%d,Static:%d,Target:1",
		s.Movable1x1Count, s.Movable1x2HCount, s.Movable1x2VCount,
		s.Movable1x3HCount, s.Movable1x3VCount, s.Static1x1Count)
	return s
}

func inventoryClassFromCount(n int) InventoryClass {
	switch n {
	case 0:
		return InvNo1x1
	case 1:
		return InvOne1x1
	case 2:
		return InvTwo1x1
	default:
		if n >= 3 {
			return InvThree1x1
		}
		return InvNo1x1
	}
}

// ComputeSpatial1x1Diag summarizes spread of placement cells.
func ComputeSpatial1x1Diag(board *Board, cells []int) Spatial1x1Diag {
	w := board.Width
	rows, cols := map[int]bool{}, map[int]bool{}
	regions := map[int]bool{}
	d := Spatial1x1Diag{MinPairManhattan: 999}
	for _, cell := range cells {
		r, c := cell/w, cell%w
		rows[r] = true
		cols[c] = true
		rq, cq := 0, 0
		if r >= board.Height/2 {
			rq = 1
		}
		if c >= w/2 {
			cq = 1
		}
		regions[rq*2+cq] = true
	}
	d.DistinctRowsUsed = len(rows)
	d.DistinctColumnsUsed = len(cols)
	d.BoardRegionsUsed = len(regions)
	if len(cells) < 2 {
		d.MinPairManhattan = 0
		d.MaxPairManhattan = 0
		return d
	}
	clustered := true
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			m := cellManhattan(w, cells[i], cells[j])
			if m > d.MaxPairManhattan {
				d.MaxPairManhattan = m
			}
			if m < d.MinPairManhattan {
				d.MinPairManhattan = m
			}
			if m > 1 {
				clustered = false
			}
		}
	}
	d.Clustered = clustered
	return d
}

// DefaultInventoryDiversityConfig is RUSH-010.2 defaults.
func DefaultInventoryDiversityConfig(batchDir string) EnrichmentConfig {
	cfg := DefaultEnrichmentConfig(batchDir)
	cfg.OutputDir = "output/RUSH0102_InventoryDiversity_001"
	cfg.BaseCount = 24
	cfg.TargetAccepted = 12
	cfg.MaxPlacements1 = 12
	cfg.MaxPlacements2 = 8
	cfg.MaxPlacements3 = 4
	cfg.SoftMaxDelta = 6
	cfg.MaxOptimalDelta = 8
	cfg.Min1x1Moves = 1
	cfg.Max1x1Moves = 9 // total unit gestures across all cubes
	cfg.SolveTimeLimit = 10 * time.Second
	cfg.MaxVisitedStates = 4_000_000
	cfg.RoleDiversity = true
	cfg.PreferOffCorridor = true
	cfg.MaxDirectBlockerFrac = 0.30
	cfg.InventoryDiversity = true
	cfg.QuotaNo1x1 = 3
	cfg.QuotaOne1x1 = 3
	cfg.QuotaTwo1x1 = 3
	cfg.QuotaThree1x1 = 3
	cfg.MaxCorridor1x1 = 1
	return cfg
}

// InventoryDiversityBatchResult extends enrichment result with inventory stats.
type InventoryDiversityBatchResult struct {
	EnrichmentBatchResult
	AcceptedByClass map[InventoryClass]int `json:"acceptedByClass"`
	PatternNotes    []string               `json:"patternNotes"`
}

// RunInventoryDiversityPilot builds a mixed 0/1/2/3 movable-1x1 shortlist.
func RunInventoryDiversityPilot(cfg EnrichmentConfig) (InventoryDiversityBatchResult, error) {
	start := time.Now()
	out := InventoryDiversityBatchResult{
		EnrichmentBatchResult: EnrichmentBatchResult{
			Config:   cfg,
			Rejected: map[string]int{},
		},
		AcceptedByClass: map[InventoryClass]int{
			InvNo1x1: 0, InvOne1x1: 0, InvTwo1x1: 0, InvThree1x1: 0,
		},
	}
	bases, err := SelectEnrichmentBases(cfg)
	if err != nil {
		return out, err
	}
	budget := SolveBudget{TimeLimit: cfg.SolveTimeLimit, MaxVisited: cfg.MaxVisitedStates}
	usedFam := map[string]bool{}

	pool := []inventoryPoolItem{}

	// --- No1x1: clean RUSH-009 bases ---
	for _, base := range bases {
		if countClass(pool, InvNo1x1) >= cfg.QuotaNo1x1+2 {
			break
		}
		c := wrapNo1x1Candidate(base)
		pool = append(pool, inventoryPoolItem{class: InvNo1x1, cand: c, band: base.SelectionBand})
	}

	poolCap := map[InventoryClass]int{
		InvOne1x1:   maxInt(cfg.QuotaOne1x1*3, cfg.QuotaOne1x1+4),
		InvTwo1x1:   maxInt(cfg.QuotaTwo1x1*3, cfg.QuotaTwo1x1+4),
		InvThree1x1: maxInt(cfg.QuotaThree1x1*3, cfg.QuotaThree1x1+4),
	}

	// --- Enriched 1/2/3 with early stop once pool caps are filled ---
	for _, base := range bases {
		if countClass(pool, InvOne1x1) >= poolCap[InvOne1x1] &&
			countClass(pool, InvTwo1x1) >= poolCap[InvTwo1x1] &&
			countClass(pool, InvThree1x1) >= poolCap[InvThree1x1] {
			break
		}
		out.BasesAttempted++
		t0 := time.Now()
		// Prefer scarcer classes first so Two/Three get attempts before One saturates budget.
		// Three-from-scratch is rarely fruitful; defer to Two→Three extension.
		counts := []int{2, 1}
		for _, n := range counts {
			cls := inventoryClassFromCount(n)
			if countClass(pool, cls) >= poolCap[cls] {
				continue
			}
			// Multi-cube enrichment is expensive; skip already-heavy bases in the first pass.
			if n >= 2 && base.VisitedStates > 200_000 {
				continue
			}
			best, rej := enrichOneBaseCount(base, n, cfg, budget, &out.EnrichmentBatchResult)
			for k, v := range rej {
				out.Rejected[k] += v
			}
			if best == nil {
				continue
			}
			pool = append(pool, inventoryPoolItem{class: cls, cand: best, band: base.SelectionBand})
		}
		out.PerBaseMs = append(out.PerBaseMs, time.Since(t0).Milliseconds())
	}

	// Fill-pass for scarce Two/Three using easier bases + modest placement budget.
	needTwo := cfg.QuotaTwo1x1 - countClass(pool, InvTwo1x1)
	needThree := cfg.QuotaThree1x1 - countClass(pool, InvThree1x1)
	if needTwo > 0 || needThree > 0 {
		easy := append([]EnrichmentBase(nil), bases...)
		sort.SliceStable(easy, func(i, j int) bool {
			if easy[i].VisitedStates != easy[j].VisitedStates {
				return easy[i].VisitedStates < easy[j].VisitedStates
			}
			return easy[i].BaseOptimal < easy[j].BaseOptimal
		})
		if len(easy) > 12 {
			easy = easy[:12]
		}
		fillCfg := cfg
		fillCfg.MaxPlacements2 = 8
		fillCfg.MaxPlacements3 = 6
		fillCfg.MaxOptimalDelta = maxInt(cfg.MaxOptimalDelta, 10)
		fillBudget := SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 1_500_000}
		for _, base := range easy {
			if needTwo <= 0 && needThree <= 0 {
				break
			}
			if base.VisitedStates > 80_000 {
				continue
			}
			out.BasesAttempted++
			if needTwo > 0 {
				best, rej := enrichOneBaseCount(base, 2, fillCfg, fillBudget, &out.EnrichmentBatchResult)
				for k, v := range rej {
					out.Rejected[k] += v
				}
				if best != nil {
					pool = append(pool, inventoryPoolItem{class: InvTwo1x1, cand: best, band: base.SelectionBand})
					needTwo--
				}
			}
		}
	}

	// Over-generate Three1x1 on distinct families; final selection enforces uniqueness.
	needThree = (cfg.QuotaThree1x1 + 2) - countClass(pool, InvThree1x1)
	if needThree > 0 {
		claimed := map[string]bool{}
		for _, p := range pool {
			if p.class == InvThree1x1 && p.cand != nil {
				claimed[p.cand.Base.FamilyID] = true
			}
		}
		extBudget := SolveBudget{TimeLimit: 8 * time.Second, MaxVisited: 3_000_000}
		easy := append([]EnrichmentBase(nil), bases...)
		sort.SliceStable(easy, func(i, j int) bool {
			return easy[i].VisitedStates < easy[j].VisitedStates
		})
		for _, base := range easy {
			if needThree <= 0 {
				break
			}
			if claimed[base.FamilyID] {
				continue
			}
			if base.VisitedStates > 250_000 {
				continue
			}
			twoCfg := cfg
			twoCfg.MaxPlacements2 = maxInt(cfg.MaxPlacements2, 10)
			two, rej := enrichOneBaseCount(base, 2, twoCfg, extBudget, &out.EnrichmentBatchResult)
			for k, v := range rej {
				out.Rejected[k] += v
			}
			if two == nil {
				continue
			}
			best := extendTwoToThree(base, two.PlacementCells, cfg, extBudget, &out.EnrichmentBatchResult)
			if best == nil {
				continue
			}
			pool = append(pool, inventoryPoolItem{class: InvThree1x1, cand: best, band: base.SelectionBand})
			claimed[base.FamilyID] = true
			needThree--
		}
	}

	// Select with quotas + family uniqueness + difficulty mix.
	selected := selectInventoryPilot(pool, cfg, usedFam)
	for i, c := range selected {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		finalizeInventoryCandidate(c, id, budget, &out.EnrichmentBatchResult)
		out.Accepted = append(out.Accepted, *c)
		out.AcceptedByClass[c.Inventory.InventoryClass]++
	}
	out.PatternNotes = inventoryPatternSanity(out.Accepted)
	out.TotalElapsed = time.Since(start)
	return out, nil
}

type inventoryPoolItem struct {
	class InventoryClass
	cand  *EnrichmentAccepted
	band  string
}

func countClass(pool []inventoryPoolItem, cls InventoryClass) int {
	n := 0
	for _, p := range pool {
		if p.class == cls {
			n++
		}
	}
	return n
}

func wrapNo1x1Candidate(base EnrichmentBase) *EnrichmentAccepted {
	inv := BuildCargoInventorySignature(base.Board)
	level := base.Level
	if level != nil {
		cp := *level
		level = &cp
	}
	return &EnrichmentAccepted{
		Base:                    base,
		Board:                   base.Board.Copy(),
		Level:                   level,
		Solution:                base.Solution,
		ReplayVerified:          true,
		Added1x1Count:           0,
		Essential1x1Count:       0,
		EnrichedOptimal:         base.BaseOptimal,
		OptimalDelta:            0,
		BaseDependencyDepth:     base.DependencyDepth,
		EnrichedDependencyDepth: base.DependencyDepth,
		BaseVisited:             base.VisitedStates,
		EnrichedVisited:         base.VisitedStates,
		ASCIIPreview:            asciiCargoBoard(base.Board),
		OffCorridorPlacement:    true,
		Inventory:               inv,
		Spatial:                 Spatial1x1Diag{},
		CubeDiags:               nil,
		Relevant1x1Count:        0,
	}
}

func enrichOneBaseCount(base EnrichmentBase, n int, cfg EnrichmentConfig, budget SolveBudget, res *EnrichmentBatchResult) (*EnrichmentAccepted, map[string]int) {
	rej := map[string]int{}
	local := cfg
	if base.VisitedStates > 100_000 {
		local.MaxPlacements1 = minInt(local.MaxPlacements1, 10)
		local.MaxPlacements2 = minInt(local.MaxPlacements2, 6)
		local.MaxPlacements3 = minInt(local.MaxPlacements3, 4)
	}
	placements := enumerate1x1PlacementsN(base.Board, local, n)
	solveBudget := budget
	if n >= 2 {
		// Multi-cube boards need a bit more BFS room; still capped.
		if solveBudget.MaxVisited < 1_500_000 {
			solveBudget.MaxVisited = 1_500_000
		}
		if solveBudget.TimeLimit < 5*time.Second {
			solveBudget.TimeLimit = 5 * time.Second
		}
	}
	var best *EnrichmentAccepted
	for _, cells := range placements {
		corridorN := 0
		for _, cell := range cells {
			if CellInTargetExitCorridor(base.Board, cell) {
				corridorN++
			}
			res.PlacementsEvaluated++
			if CellInTargetExitCorridor(base.Board, cell) {
				res.DirectPlacementsEval++
			} else {
				res.OffCorridorPlacementsEval++
			}
		}
		if corridorN > local.MaxCorridor1x1 {
			rej["CorridorDominance"]++
			continue
		}
		cand, reason := evaluateInventoryPlacement(base, cells, local, solveBudget, res)
		if reason != "" {
			rej[reason]++
			continue
		}
		if best == nil || betterInventoryCandidate(cand, best, local) {
			best = cand
		}
		// Accept first solid hit — pool ranking compares across bases.
		if best != nil && best.Essential1x1Count >= 1 &&
			best.Relevant1x1Count >= minRelevantRequired(n) &&
			best.OptimalDelta <= local.SoftMaxDelta {
			if n == 1 || best.Relevant1x1Count >= n || len(best.roleSet()) >= 2 {
				break
			}
			if n >= 2 && best.OptimalDelta >= local.SoftMinDelta {
				break
			}
		}
	}
	return best, rej
}

func extendTwoToThree(base EnrichmentBase, twoCells []int, cfg EnrichmentConfig, budget SolveBudget, res *EnrichmentBatchResult) *EnrichmentAccepted {
	occupied := map[int]bool{}
	corridorTwo := 0
	for _, c := range twoCells {
		occupied[c] = true
		if CellInTargetExitCorridor(base.Board, c) {
			corridorTwo++
		}
	}
	type scoredCell struct {
		cell  int
		score int
	}
	cands := []scoredCell{}
	for i := 0; i < base.Board.Width*base.Board.Height; i++ {
		if base.Board.occupied[i] || occupied[i] {
			continue
		}
		inCorr := CellInTargetExitCorridor(base.Board, i)
		if inCorr && corridorTwo >= cfg.MaxCorridor1x1 {
			continue
		}
		sp := ComputeSpatial1x1Diag(base.Board, []int{twoCells[0], twoCells[1], i})
		score := sp.DistinctRowsUsed*4 + sp.DistinctColumnsUsed*4 + sp.BoardRegionsUsed*6
		// Prefer action-adjacent / corridor (likely essential) over empty corners.
		if inCorr {
			score += 40
		}
		adj := 0
		w := base.Board.Width
		for _, d := range []int{-1, 1, -w, w} {
			nb := i + d
			if nb >= 0 && nb < w*base.Board.Height && base.Board.occupied[nb] {
				adj++
			}
		}
		score += adj * 8
		// Mild penalty for adjacency to existing 1x1s (avoid blob).
		for _, c := range twoCells {
			if cellManhattan(w, i, c) == 1 {
				score -= 10
			}
		}
		score -= offCorridorPriority(base.Board, i) / 2
		cands = append(cands, scoredCell{cell: i, score: score})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].cell < cands[j].cell
	})
	limit := 12
	if len(cands) > limit {
		cands = cands[:limit]
	}
	var best *EnrichmentAccepted
	for _, sc := range cands {
		cells := []int{twoCells[0], twoCells[1], sc.cell}
		res.PlacementsEvaluated++
		cand, reason := evaluateInventoryPlacement(base, cells, cfg, budget, res)
		if reason != "" {
			res.Rejected[reason]++
			continue
		}
		if best == nil || betterInventoryCandidate(cand, best, cfg) {
			best = cand
		}
		if best != nil && best.Essential1x1Count >= 1 && best.Relevant1x1Count >= 2 {
			break
		}
	}
	return best
}

func minRelevantRequired(n int) int {
	switch n {
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 2
	default:
		return 1
	}
}

func enumerate1x1PlacementsN(board *Board, cfg EnrichmentConfig, n int) [][]int {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		all := enumerate1x1Placements(board, cfg)
		out := [][]int{}
		for _, p := range all {
			if len(p) == 1 {
				out = append(out, p)
			}
		}
		return out
	}
	// n >= 2: prefer spatially spread placements (not adjacent clusters).
	empty := []int{}
	for i := 0; i < board.Width*board.Height; i++ {
		if !board.occupied[i] {
			empty = append(empty, i)
		}
	}
	off, on := []int{}, []int{}
	for _, c := range empty {
		if CellInTargetExitCorridor(board, c) {
			on = append(on, c)
		} else {
			off = append(off, c)
		}
	}
	sort.SliceStable(off, func(i, j int) bool {
		si, sj := offCorridorPriority(board, off[i]), offCorridorPriority(board, off[j])
		if si != sj {
			return si < sj
		}
		return off[i] < off[j]
	})
	// Bound pool to keep placement search deterministic and finite.
	maxPool := 14
	if len(off) > maxPool {
		off = off[:maxPool]
	}
	pool := append([]int{}, off...)
	if len(on) > 0 {
		sort.SliceStable(on, func(i, j int) bool { return on[i] < on[j] })
		pool = append(pool, on[0]) // at most one corridor seed
	}
	out := [][]int{}
	if n == 2 {
		limit := cfg.MaxPlacements2
		if limit <= 0 {
			limit = 12
		}
		type scoredPair struct {
			cells []int
			score int
		}
		cands := []scoredPair{}
		for i := 0; i < len(pool); i++ {
			for j := i + 1; j < len(pool); j++ {
				cells := []int{pool[i], pool[j]}
				sp := ComputeSpatial1x1Diag(board, cells)
				man := sp.MinPairManhattan
				if man < 1 {
					continue // same cell
				}
				// Prefer non-adjacent, but allow man==1 only if not both corridor.
				if man == 1 && sp.Clustered {
					continue
				}
				corridorN := 0
				for _, c := range cells {
					if CellInTargetExitCorridor(board, c) {
						corridorN++
					}
				}
				if corridorN > cfg.MaxCorridor1x1 {
					continue
				}
				// Mix interactive (near) and spread (far) placements.
				score := sp.DistinctRowsUsed*6 + sp.DistinctColumnsUsed*6 + sp.BoardRegionsUsed*8
				if man >= 2 && man <= 4 {
					score += 20 // interactive sweet spot
				} else if man > 4 {
					score += 12 + minInt(man, 8) // spread bonus
				}
				if man == 1 {
					score -= 15
				}
				cands = append(cands, scoredPair{cells: cells, score: score})
			}
		}
		sort.SliceStable(cands, func(i, j int) bool {
			if cands[i].score != cands[j].score {
				return cands[i].score > cands[j].score
			}
			return placementKey(cands[i].cells) < placementKey(cands[j].cells)
		})
		for _, c := range cands {
			if len(out) >= limit {
				break
			}
			out = append(out, c.cells)
		}
		return out
	}
	// n == 3
	limit := cfg.MaxPlacements3
	if limit <= 0 {
		limit = 10
	}
	type scoredTrip struct {
		cells []int
		score int
	}
	cands := []scoredTrip{}
	for i := 0; i < len(pool); i++ {
		for j := i + 1; j < len(pool); j++ {
			if cellManhattan(board.Width, pool[i], pool[j]) < 2 {
				continue
			}
			for k := j + 1; k < len(pool); k++ {
				cells := []int{pool[i], pool[j], pool[k]}
				sp := ComputeSpatial1x1Diag(board, cells)
				if sp.MinPairManhattan < 1 {
					continue
				}
				// Allow moderately close triples; reject only fully clustered adjacent packs.
				if sp.Clustered && sp.MaxPairManhattan <= 1 {
					continue
				}
				if sp.DistinctRowsUsed < 2 && sp.DistinctColumnsUsed < 2 {
					continue
				}
				corridorN := 0
				for _, c := range cells {
					if CellInTargetExitCorridor(board, c) {
						corridorN++
					}
				}
				if corridorN > cfg.MaxCorridor1x1 {
					continue
				}
				score := sp.DistinctRowsUsed*6 + sp.DistinctColumnsUsed*6 + sp.BoardRegionsUsed*10
				if sp.MinPairManhattan >= 2 {
					score += 15
				}
				if sp.MaxPairManhattan >= 4 {
					score += 8
				}
				cands = append(cands, scoredTrip{cells: cells, score: score})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return placementKey(cands[i].cells) < placementKey(cands[j].cells)
	})
	for _, c := range cands {
		if len(out) >= limit {
			break
		}
		out = append(out, c.cells)
	}
	return out
}

func evaluateInventoryPlacement(base EnrichmentBase, cells []int, cfg EnrichmentConfig, budget SolveBudget, res *EnrichmentBatchResult) (*EnrichmentAccepted, string) {
	board := base.Board.Copy()
	addedIdx := []int{}
	for _, cell := range cells {
		if board.occupied[cell] {
			return nil, "InvalidPlacement"
		}
		p := Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
		if !board.AddPiece(p) {
			return nil, "InvalidPlacement"
		}
		addedIdx = append(addedIdx, len(board.Pieces)-1)
		board.Labels = append(board.Labels, fmt.Sprintf("U%02d", len(addedIdx)))
	}
	if board.cargoTargetCanExit() {
		return nil, "ImmediateVictory"
	}

	t0 := time.Now()
	// Fast-fail screen for single-cube; multi-cube uses the caller budget more fully.
	screen := budget
	if len(cells) == 1 {
		if screen.MaxVisited <= 0 || screen.MaxVisited > 800_000 {
			screen.MaxVisited = 800_000
		}
		if screen.TimeLimit <= 0 || screen.TimeLimit > 4*time.Second {
			screen.TimeLimit = 4 * time.Second
		}
	}
	sol := board.SolveWithBudget(screen)
	elapsed := time.Since(t0)
	res.ExactSolves++
	if sol.TimedOut || sol.BudgetExceeded {
		return nil, "DifficultyUnknown"
	}
	if !sol.Solvable {
		return nil, "Unsolvable"
	}

	unitMoves := countUnitMoves(board, sol, addedIdx)
	if unitMoves == 0 {
		return nil, "OneByOneUnused"
	}
	if unitMoves < cfg.Min1x1Moves || unitMoves > cfg.Max1x1Moves {
		return nil, "OneByOneMoveCount"
	}
	delta := sol.NumMoves - base.BaseOptimal
	if delta < cfg.MinOptimalDelta || delta > cfg.MaxOptimalDelta {
		return nil, "QualityDegraded"
	}

	// Per-cube diagnostics (bounded): mark InOptimal first; restricted only as needed.
	cubeDiags := make([]CubeRelevanceDiag, 0, len(addedIdx))
	essentialCount := 0
	relevantCount := 0
	movedSet := map[int]bool{}
	workScan := board.Copy()
	for _, m := range sol.Moves {
		for _, ai := range addedIdx {
			if m.Piece == ai {
				movedSet[ai] = true
			}
		}
		workScan.DoMove(m)
	}

	for i, idx := range addedIdx {
		cell := cells[i]
		d := CubeRelevanceDiag{
			Cell:       cell,
			PieceIndex: idx,
			InOptimal:  movedSet[idx],
			InCorridor: CellInTargetExitCorridor(base.Board, cell),
		}
		role := ClassifyOneByOneRole(board, sol, []int{idx})
		d.Role = role.Role

		// Restricted solve: skip if we already have an essential and this cube is clearly relevant via optimal.
		needRestricted := essentialCount < 1 || !d.InOptimal
		if needRestricted {
			restricted := board.Copy()
			restricted.ImmobilePieces = make([]bool, len(restricted.Pieces))
			restricted.ImmobilePieces[idx] = true
			rb := budget
			if len(cells) >= 2 {
				if rb.MaxVisited > 1_200_000 {
					rb.MaxVisited = 1_200_000
				}
				if rb.TimeLimit > 5*time.Second {
					rb.TimeLimit = 5 * time.Second
				}
			} else {
				if rb.MaxVisited > 600_000 {
					rb.MaxVisited = 600_000
				}
				if rb.TimeLimit > 3*time.Second {
					rb.TimeLimit = 3 * time.Second
				}
			}
			rsol := restricted.SolveWithBudget(rb)
			res.NecessitySolves++
			if rsol.TimedOut || rsol.BudgetExceeded {
				if !d.InOptimal && essentialCount < 1 {
					return nil, "NecessityUnknown"
				}
			} else if !rsol.Solvable || (rsol.Solvable && rsol.NumMoves > sol.NumMoves) {
				d.Essential = true
				essentialCount++
				d.Evidence = "restricted-worse"
			}
		}

		if d.InOptimal || d.Essential || role.SubsequentUsesReleasedCell ||
			role.Role == RoleDirectTargetBlocker || role.Role == RoleGateKeeper {
			d.Relevant = true
			relevantCount++
			if d.Evidence == "" {
				switch {
				case d.InOptimal:
					d.Evidence = "in-optimal"
				case role.Role == RoleDirectTargetBlocker || role.Role == RoleGateKeeper:
					d.Evidence = string(role.Role)
				default:
					d.Evidence = role.Summary
				}
			}
		}
		if !d.Relevant {
			return nil, "Decorative1x1"
		}
		cubeDiags = append(cubeDiags, d)
	}
	if essentialCount < 1 {
		// Last chance: freeze all added cubes together (legacy strong test).
		allRest := board.Copy()
		allRest.ImmobilePieces = make([]bool, len(allRest.Pieces))
		for _, idx := range addedIdx {
			allRest.ImmobilePieces[idx] = true
		}
		rb := budget
		if rb.MaxVisited > 600_000 {
			rb.MaxVisited = 600_000
		}
		if rb.TimeLimit > 3*time.Second {
			rb.TimeLimit = 3 * time.Second
		}
		rsol := allRest.SolveWithBudget(rb)
		res.NecessitySolves++
		if rsol.TimedOut || rsol.BudgetExceeded {
			return nil, "NecessityUnknown"
		}
		if !rsol.Solvable || (rsol.Solvable && rsol.NumMoves > sol.NumMoves) {
			essentialCount = 1
			cubeDiags[0].Essential = true
			if cubeDiags[0].Evidence == "" {
				cubeDiags[0].Evidence = "group-restricted-worse"
			}
		} else {
			return nil, "OneByOneNotEssential"
		}
	}
	needRel := minRelevantRequired(len(cells))
	if relevantCount < needRel {
		return nil, "RelevantCount"
	}
	// Single cube must be essential (RUSH-010 semantics).
	if len(cells) == 1 && !cubeDiags[0].Essential {
		return nil, "OneByOneNotEssential"
	}

	b2 := board.Copy()
	if err := b2.Replay(sol.Moves); err != nil {
		return nil, "ReplayFailed"
	}

	metrics := BuildCandidateMetrics(board, sol)
	inv := BuildCargoInventorySignature(board)
	spatial := ComputeSpatial1x1Diag(board, cells)
	rolePrimary := cubeDiags[0].Role
	roleEv := ClassifyOneByOneRole(board, sol, addedIdx)
	if len(cubeDiags) > 0 {
		rolePrimary = cubeDiags[0].Role
	}
	_ = rolePrimary
	impact := enrichmentImpact(delta, base.DependencyDepth, metrics.DependencyDepth, unitMoves, base.VisitedStates, sol.MemoSize)
	corridorN := 0
	for _, d := range cubeDiags {
		if d.InCorridor {
			corridorN++
		}
	}
	level, err := LevelJSONFromBoard(board, "pending", "RushDatabaseEnriched")
	if err != nil {
		return nil, "ExportFailed"
	}

	return &EnrichmentAccepted{
		Base:                    base,
		Board:                   board,
		Level:                   level,
		Solution:                sol,
		ElapsedMs:               elapsed.Milliseconds(),
		ReplayVerified:          true,
		PlacementCells:          append([]int(nil), cells...),
		Added1x1Count:           len(cells),
		OneByOneMovesInOptimal:  unitMoves,
		Essential1x1Count:       essentialCount,
		RestrictedOptimal:       -1,
		RestrictedSolvable:      true,
		OptimalWithout1x1:       -1,
		EnrichedOptimal:         sol.NumMoves,
		OptimalDelta:            delta,
		BaseDependencyDepth:     base.DependencyDepth,
		EnrichedDependencyDepth: metrics.DependencyDepth,
		BaseVisited:             base.VisitedStates,
		EnrichedVisited:         sol.MemoSize,
		EnrichmentImpact:        impact,
		ASCIIPreview:            asciiCargoBoard(board),
		RoleEvidence:            roleEv,
		OffCorridorPlacement:    corridorN == 0,
		Inventory:               inv,
		Spatial:                 spatial,
		CubeDiags:               cubeDiags,
		Relevant1x1Count:        relevantCount,
		Corridor1x1Count:        corridorN,
	}, ""
}

func (c *EnrichmentAccepted) roleSet() map[OneByOneRole]bool {
	s := map[OneByOneRole]bool{}
	for _, d := range c.CubeDiags {
		s[d.Role] = true
	}
	if len(s) == 0 && c.RoleEvidence.Role != "" {
		s[c.RoleEvidence.Role] = true
	}
	return s
}

func betterInventoryCandidate(a, b *EnrichmentAccepted, cfg EnrichmentConfig) bool {
	sa, sb := inventoryRankScore(a, cfg), inventoryRankScore(b, cfg)
	if sa != sb {
		return sa > sb
	}
	return placementKey(a.PlacementCells) < placementKey(b.PlacementCells)
}

func inventoryRankScore(c *EnrichmentAccepted, cfg EnrichmentConfig) float64 {
	score := 0.0
	// Puzzle quality preserved (prefer soft delta).
	score += float64(softDeltaScore(c.OptimalDelta, cfg)) * 20
	score += float64(c.Essential1x1Count) * 15
	score += float64(c.Relevant1x1Count) * 10
	score += float64(len(c.roleSet())) * 12
	score += float64(c.Spatial.DistinctRowsUsed+c.Spatial.DistinctColumnsUsed+c.Spatial.BoardRegionsUsed) * 3
		// Soft clustered penalty only when all pairs are adjacent.
	if c.Spatial.Clustered && c.Spatial.MaxPairManhattan <= 1 {
		score -= 25
	}
	if c.Corridor1x1Count > cfg.MaxCorridor1x1 {
		score -= 40
	} else if c.Corridor1x1Count > 0 {
		score -= 5 // mild penalty
	}
	dep := c.EnrichedDependencyDepth - c.BaseDependencyDepth
	score += float64(dep) * 2
	// Prefer cheaper solves.
	if c.EnrichedVisited > 0 {
		score -= mathLog10(c.EnrichedVisited) * 2
	}
	return score
}

func mathLog10(n int) float64 {
	if n <= 1 {
		return 0
	}
	return float64(len(fmt.Sprintf("%d", n))) // cheap digit-length proxy
}

func selectInventoryPilot(pool []inventoryPoolItem, cfg EnrichmentConfig, usedFam map[string]bool) []*EnrichmentAccepted {
	quotas := map[InventoryClass]int{
		InvNo1x1: cfg.QuotaNo1x1, InvOne1x1: cfg.QuotaOne1x1,
		InvTwo1x1: cfg.QuotaTwo1x1, InvThree1x1: cfg.QuotaThree1x1,
	}
	bandNeed := map[string]int{"lower-mid": 4, "medium": 4, "hard": 4}
	out := []*EnrichmentAccepted{}

	// Sort pool: higher inventory score first within class.
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].class != pool[j].class {
			return pool[i].class < pool[j].class
		}
		return betterInventoryCandidate(pool[i].cand, pool[j].cand, cfg)
	})

	var pick func(cls InventoryClass, preferBand string) bool
	pick = func(cls InventoryClass, preferBand string) bool {
		best := -1
		for i, p := range pool {
			if p.cand == nil || p.class != cls {
				continue
			}
			if usedFam[p.cand.Base.FamilyID] {
				continue
			}
			if preferBand != "" && p.band != preferBand {
				continue
			}
			if best < 0 || betterInventoryCandidate(p.cand, pool[best].cand, cfg) {
				best = i
			}
		}
		if best < 0 && preferBand != "" {
			return pick(cls, "")
		}
		if best < 0 {
			return false
		}
		c := pool[best].cand
		usedFam[c.Base.FamilyID] = true
		out = append(out, c)
		quotas[cls]--
		bandNeed[pool[best].band]--
		pool[best].cand = nil
		return true
	}

	classes := []InventoryClass{InvThree1x1, InvTwo1x1, InvOne1x1, InvNo1x1}
	bands := []string{"hard", "medium", "lower-mid"}
	// Round-robin band × class; Three before Two so multi-cube families aren't stolen.
	for _, band := range bands {
		for _, cls := range classes {
			if quotas[cls] <= 0 {
				continue
			}
			_ = pick(cls, band)
		}
	}
	for _, cls := range classes {
		for quotas[cls] > 0 {
			if !pick(cls, "") {
				break
			}
		}
	}
	return out
}

func finalizeInventoryCandidate(c *EnrichmentAccepted, id string, budget SolveBudget, res *EnrichmentBatchResult) {
	if c.Added1x1Count > 0 {
		fillRemovalDiagnostic(c, budget, res)
	} else if len(c.Solution.Moves) == 0 && c.Base.Solution.Solvable {
		c.Solution = c.Base.Solution
		c.EnrichedOptimal = c.Base.BaseOptimal
		c.EnrichedVisited = c.Base.VisitedStates
		b2 := c.Board.Copy()
		c.ReplayVerified = b2.Replay(c.Solution.Moves) == nil
	} else if len(c.Solution.Moves) > 0 {
		b2 := c.Board.Copy()
		c.ReplayVerified = b2.Replay(c.Solution.Moves) == nil
	}
	c.CandidateID = id
	opt := c.EnrichedOptimal
	if c.Level == nil {
		level, _ := LevelJSONFromBoard(c.Board, id, "RushDatabase")
		c.Level = level
	}
	c.Level.LevelID = id
	if c.Added1x1Count == 0 {
		c.Level.Source = "RushDatabaseCurator"
	} else {
		c.Level.Source = "RushDatabaseEnriched"
	}
	c.Level.CanonicalGestures = &opt
	if c.Base.Level != nil && c.Base.Level.Transplant != nil {
		cp := *c.Base.Level.Transplant
		c.Level.Transplant = &cp
	}
	roles := []string{}
	for _, d := range c.CubeDiags {
		roles = append(roles, string(d.Role))
	}
	if len(roles) == 0 && c.Added1x1Count == 0 {
		roles = []string{"None"}
	}
	c.Level.Enrichment = &EnrichmentJSON{
		BaseCandidateId:                   c.Base.CandidateID,
		BaseFamilyId:                      c.Base.FamilyID,
		BaseSourcePuzzleId:                c.Base.SourcePuzzleID,
		Added1x1Count:                     c.Added1x1Count,
		Essential1x1Count:                 c.Essential1x1Count,
		OneByOneMovesInOptimal:            c.OneByOneMovesInOptimal,
		BaseOptimal:                       c.Base.BaseOptimal,
		EnrichedOptimal:                   c.EnrichedOptimal,
		OptimalDelta:                      c.OptimalDelta,
		EnrichmentImpact:                  c.EnrichmentImpact,
		PlacementCells:                    append([]int(nil), c.PlacementCells...),
		OneByOneRole:                      strings.Join(roles, "+"),
		OneByOneInitiallyInTargetCorridor: c.Corridor1x1Count > 0,
		RoleEvidenceSummary:               c.RoleEvidence.Summary,
		ReleasedCells:                     append([]int(nil), c.RoleEvidence.ReleasedCells...),
		SubsequentPieceClass:              c.RoleEvidence.SubsequentPieceClass,
		InventoryClass:                    string(c.Inventory.InventoryClass),
		InventorySignature:                c.Inventory.Signature,
		Relevant1x1Count:                  c.Relevant1x1Count,
		Corridor1x1Count:                  c.Corridor1x1Count,
		DistinctRowsUsed:                  c.Spatial.DistinctRowsUsed,
		DistinctColumnsUsed:               c.Spatial.DistinctColumnsUsed,
		BoardRegionsUsed:                  c.Spatial.BoardRegionsUsed,
	}
	c.SolutionDoc = ExportSolutionJSON(c.Level, c.Board, c.Solution, time.Duration(c.ElapsedMs)*time.Millisecond, c.ReplayVerified)
}

func inventoryPatternSanity(accepted []EnrichmentAccepted) []string {
	notes := []string{}
	oneCenter := 0
	twoCornerish := 0
	threeSameRow := 0
	for _, c := range accepted {
		switch c.Inventory.InventoryClass {
		case InvOne1x1:
			if len(c.PlacementCells) == 1 {
				cell := c.PlacementCells[0]
				w, h := c.Board.Width, c.Board.Height
				r, col := cell/w, cell%w
				if absInt(r-h/2) <= 1 && absInt(col-w/2) <= 1 {
					oneCenter++
				}
			}
		case InvTwo1x1:
			if c.Spatial.MaxPairManhattan >= 5 && c.Spatial.BoardRegionsUsed >= 2 {
				twoCornerish++
			}
		case InvThree1x1:
			if c.Spatial.DistinctRowsUsed == 1 {
				threeSameRow++
			}
		}
	}
	n1 := 0
	for _, c := range accepted {
		if c.Inventory.InventoryClass == InvOne1x1 {
			n1++
		}
	}
	if n1 > 0 && oneCenter == n1 {
		notes = append(notes, "PATTERN: all One1x1 near board center")
	}
	n3 := 0
	for _, c := range accepted {
		if c.Inventory.InventoryClass == InvThree1x1 {
			n3++
		}
	}
	if n3 > 0 && threeSameRow == n3 {
		notes = append(notes, "PATTERN: all Three1x1 share a single row")
	}
	if len(notes) == 0 {
		notes = append(notes, "No obvious fixed spatial inventory pattern detected")
	}
	_ = twoCornerish
	return notes
}

// WriteInventoryDiversityBatch writes the RUSH-010.2 pilot outputs.
func WriteInventoryDiversityBatch(dir string, res InventoryDiversityBatchResult) error {
	if err := WriteEnrichmentBatch(dir, res.EnrichmentBatchResult); err != nil {
		return err
	}
	// Override CSV / reports with inventory-specific fields.
	if err := writeInventoryHumanReviewCSV(filepath.Join(dir, "HumanReview.csv"), res.Accepted); err != nil {
		return err
	}
	report := res.ToInventoryReport()
	return WriteJSONFile(filepath.Join(dir, "InventoryDiversityReport.json"), report)
}

func writeInventoryHumanReviewCSV(path string, accepted []EnrichmentAccepted) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{
		"CandidateId", "FamilyId", "InventoryClass", "1x1Count", "Roles",
		"HumanDifficulty", "HumanMoves", "InventoryFeelsNatural", "InventoryFeelsRepetitive", "Keep", "Notes",
	})
	for _, c := range accepted {
		roles := []string{}
		for _, d := range c.CubeDiags {
			roles = append(roles, string(d.Role))
		}
		if len(roles) == 0 {
			roles = []string{"None"}
		}
		_ = w.Write([]string{
			c.CandidateID,
			c.Base.FamilyID,
			string(c.Inventory.InventoryClass),
			fmt.Sprintf("%d", c.Added1x1Count),
			strings.Join(roles, "+"),
			"", "", "", "", "", "",
		})
	}
	w.Flush()
	return w.Error()
}

func (r InventoryDiversityBatchResult) ToInventoryReport() map[string]interface{} {
	rows := []map[string]interface{}{}
	for _, c := range r.Accepted {
		roles := []string{}
		for _, d := range c.CubeDiags {
			roles = append(roles, string(d.Role))
		}
		rows = append(rows, map[string]interface{}{
			"candidateId":           c.CandidateID,
			"baseCandidate":         c.Base.CandidateID,
			"familyId":              c.Base.FamilyID,
			"inventoryClass":        string(c.Inventory.InventoryClass),
			"movable1x1Count":       c.Added1x1Count,
			"roles":                 roles,
			"corridor1x1Count":      c.Corridor1x1Count,
			"relevant1x1Count":      c.Relevant1x1Count,
			"essential1x1Count":     c.Essential1x1Count,
			"baseOptimal":           c.Base.BaseOptimal,
			"enrichedOptimal":       c.EnrichedOptimal,
			"optimalDelta":          c.OptimalDelta,
			"dependencyDepth":       c.EnrichedDependencyDepth,
			"inventorySignature":    c.Inventory.Signature,
			"selectionBand":         c.Base.SelectionBand,
			"spatial":               c.Spatial,
			"replay":                c.ReplayVerified,
		})
	}
	bandDist := map[string]int{}
	sigDist := map[string]int{}
	for _, c := range r.Accepted {
		bandDist[c.Base.SelectionBand]++
		sigDist[c.Inventory.Signature]++
	}
	avg := 0.0
	if len(r.Accepted) > 0 && r.TotalElapsed > 0 {
		avg = float64(r.TotalElapsed.Milliseconds()) / float64(len(r.Accepted))
	}
	return map[string]interface{}{
		"generatorVersion": EnrichmentInventoryVersion,
		"pipeline":         "CargoFlowInventoryDiversity",
		"performance": map[string]interface{}{
			"basesAttempted":                 r.BasesAttempted,
			"placementCombinationsEvaluated": r.PlacementsEvaluated,
			"exactSolves":                    r.ExactSolves,
			"restrictedSolves":               r.NecessitySolves,
			"acceptedByClass":                r.AcceptedByClass,
			"totalElapsedMs":                 r.TotalElapsed.Milliseconds(),
			"avgPerAcceptedMs":               avg,
		},
		"distribution": map[string]interface{}{
			"inventoryClass": r.AcceptedByClass,
			"role":           r.RoleDistribution(),
			"difficultyBand": bandDist,
			"inventorySignature": sigDist,
		},
		"patternSanity": r.PatternNotes,
		"accepted":      rows,
		"rejected":      r.Rejected,
		"limitations": []string{
			"InventoryClass is not a difficulty label.",
			"Per-cube restricted solves are used; full 2^N subset search is intentionally avoided.",
			"No1x1 candidates are clean RUSH-009 curated levels.",
		},
	}
}
