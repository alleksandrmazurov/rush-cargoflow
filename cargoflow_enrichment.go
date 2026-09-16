package rush

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const EnrichmentVersion = "enrich-1x1-poc-1"
const EnrichmentRoleDiversityVersion = "enrich-1x1-roles-v1"

// EnrichmentConfig bounds the RUSH-010 / RUSH-010.1 / RUSH-010.2 enrichment pilots.
type EnrichmentConfig struct {
	BatchDir             string
	CuratorReportPath    string
	OutputDir            string
	TargetAccepted       int
	BaseCount            int
	MaxPlacements1       int
	MaxPlacements2       int
	MaxPlacements3       int
	Min1x1Moves          int
	Max1x1Moves          int
	MinOptimalDelta      int
	MaxOptimalDelta      int
	SoftMinDelta         int
	SoftMaxDelta         int
	SolveTimeLimit       time.Duration
	MaxVisitedStates     int
	Seed                 int64
	// RUSH-010.1 role diversity controls.
	RoleDiversity        bool
	PreferOffCorridor    bool
	MaxDirectBlockerFrac float64 // e.g. 0.30
	MinOffCorridor       int
	MinDistinctRoles     int
	// RUSH-010.2 inventory diversity controls.
	InventoryDiversity bool
	QuotaNo1x1         int
	QuotaOne1x1        int
	QuotaTwo1x1        int
	QuotaThree1x1      int
	MaxCorridor1x1     int // default max movable 1x1 initially in Target corridor
}

func DefaultEnrichmentConfig(batchDir string) EnrichmentConfig {
	return EnrichmentConfig{
		BatchDir:             batchDir,
		CuratorReportPath:    filepath.Join(batchDir, "CuratorReport.json"),
		OutputDir:            "output/RUSH010_OneByOneEnrichment_001",
		TargetAccepted:       8,
		BaseCount:            8,
		MaxPlacements1:       24,
		MaxPlacements2:       8,
		Min1x1Moves:          1,
		Max1x1Moves:          3,
		MinOptimalDelta:      -2,
		MaxOptimalDelta:      12,
		SoftMinDelta:         0,
		SoftMaxDelta:         6,
		SolveTimeLimit:       10 * time.Second,
		MaxVisitedStates:     4_000_000,
		Seed:                 20260915,
		RoleDiversity:        false,
		PreferOffCorridor:    false,
		MaxDirectBlockerFrac: 1.0,
		MinOffCorridor:       0,
		MinDistinctRoles:     0,
	}
}

// DefaultRoleDiversityConfig is RUSH-010.1 defaults.
func DefaultRoleDiversityConfig(batchDir string) EnrichmentConfig {
	cfg := DefaultEnrichmentConfig(batchDir)
	cfg.OutputDir = "output/RUSH0101_OneByOneRoleDiversity_001"
	cfg.BaseCount = 20
	cfg.TargetAccepted = 8
	cfg.MaxPlacements1 = 28
	cfg.MaxPlacements2 = 4
	cfg.SoftMaxDelta = 4
	cfg.MaxOptimalDelta = 6
	cfg.RoleDiversity = true
	cfg.PreferOffCorridor = true
	cfg.MaxDirectBlockerFrac = 0.30
	cfg.MinOffCorridor = 5
	cfg.MinDistinctRoles = 3
	return cfg
}

// EnrichmentBase is one RUSH-009 shortlist level selected for enrichment.
type EnrichmentBase struct {
	CandidateID       string
	FamilyID          string
	SourcePuzzleID    string
	BaseOptimal       int
	EstDifficulty     float64
	DependencyDepth   int
	VisitedStates     int
	Board             *Board
	Level             *LevelJSON
	Solution          Solution
	BaseMetrics       CandidateMetrics
	BaseSignals       DifficultySignals
	SelectionBand     string // lower-mid | medium | hard
}

// EnrichmentAccepted is one validated enriched candidate.
type EnrichmentAccepted struct {
	CandidateID             string
	Base                    EnrichmentBase
	Board                   *Board
	Level                   *LevelJSON
	Solution                Solution
	ElapsedMs               int64
	ReplayVerified          bool
	PlacementCells          []int
	Added1x1Count           int
	OneByOneMovesInOptimal  int
	Essential1x1Count       int
	RestrictedOptimal       int
	RestrictedSolvable      bool
	OptimalWithout1x1       int
	Without1x1Solvable      bool
	EnrichedOptimal         int
	OptimalDelta            int
	BaseDependencyDepth     int
	EnrichedDependencyDepth int
	BaseVisited             int
	EnrichedVisited         int
	EnrichmentImpact        float64
	ASCIIPreview            string
	SolutionDoc             SolutionJSON
	RoleEvidence            OneByOneRoleEvidence
	OffCorridorPlacement    bool
	// RUSH-010.2 inventory fields.
	Inventory        CargoInventorySignature
	Spatial          Spatial1x1Diag
	CubeDiags        []CubeRelevanceDiag
	Relevant1x1Count int
	Corridor1x1Count int
}

// EnrichmentBatchResult summarizes a RUSH-010 / RUSH-010.1 run.
type EnrichmentBatchResult struct {
	Config                   EnrichmentConfig
	BasesAttempted           int
	PlacementsEvaluated      int
	DirectPlacementsEval     int
	OffCorridorPlacementsEval int
	ExactSolves              int
	NecessitySolves          int
	Accepted                 []EnrichmentAccepted
	Rejected                 map[string]int
	TotalElapsed             time.Duration
	PerBaseMs                []int64
}

type curatorShortlistRow struct {
	CandidateID              string  `json:"candidateId"`
	FamilyID                 string  `json:"familyId"`
	SourcePuzzleID           string  `json:"sourcePuzzleId"`
	CargoOptimalGestures     int     `json:"cargoOptimalGestures"`
	EstimatedHumanDifficulty float64 `json:"estimatedHumanDifficulty"`
	DependencyDepth          int     `json:"dependencyDepth"`
	VisitedStates            int     `json:"visitedStates"`
}

type curatorReportFile struct {
	Shortlist []curatorShortlistRow `json:"shortlist"`
}

// SelectEnrichmentBases picks diverse RUSH-009 families with medium/hard bias.
func SelectEnrichmentBases(cfg EnrichmentConfig) ([]EnrichmentBase, error) {
	data, err := os.ReadFile(cfg.CuratorReportPath)
	if err != nil {
		return nil, err
	}
	var report curatorReportFile
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	if len(report.Shortlist) == 0 {
		return nil, fmt.Errorf("empty shortlist in %s", cfg.CuratorReportPath)
	}

	type scored struct {
		row   curatorShortlistRow
		score float64
		band  string
	}
	items := make([]scored, 0, len(report.Shortlist))
	for _, row := range report.Shortlist {
		band := enrichmentBand(row.EstimatedHumanDifficulty, row.CargoOptimalGestures)
		score := enrichmentSelectScore(row)
		items = append(items, scored{row: row, score: score, band: band})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].row.CandidateID < items[j].row.CandidateID
	})

	quotas := map[string]int{"lower-mid": 2, "medium": 3, "hard": 3}
	picked := []scored{}
	usedFam := map[string]bool{}
	for _, band := range []string{"hard", "medium", "lower-mid"} {
		for _, it := range items {
			if quotas[band] <= 0 {
				break
			}
			if it.band != band || usedFam[it.row.FamilyID] {
				continue
			}
			picked = append(picked, it)
			usedFam[it.row.FamilyID] = true
			quotas[band]--
		}
	}
	// Fill remaining slots from any band.
	for _, it := range items {
		if len(picked) >= cfg.BaseCount {
			break
		}
		if usedFam[it.row.FamilyID] {
			continue
		}
		picked = append(picked, it)
		usedFam[it.row.FamilyID] = true
	}
	if len(picked) > cfg.BaseCount {
		picked = picked[:cfg.BaseCount]
	}

	out := make([]EnrichmentBase, 0, len(picked))
	for _, it := range picked {
		path := filepath.Join(cfg.BatchDir, "Candidates", it.row.CandidateID+".json")
		level, err := LoadCargoFlowLevelJSONFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return nil, err
		}
		sol := board.SolveWithBudget(SolveBudget{TimeLimit: cfg.SolveTimeLimit, MaxVisited: cfg.MaxVisitedStates})
		if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
			continue
		}
		metrics := BuildCandidateMetrics(board, sol)
		signals := BuildDifficultySignals(metrics)
		out = append(out, EnrichmentBase{
			CandidateID:     it.row.CandidateID,
			FamilyID:        it.row.FamilyID,
			SourcePuzzleID:  it.row.SourcePuzzleID,
			BaseOptimal:     sol.NumMoves,
			EstDifficulty:   it.row.EstimatedHumanDifficulty,
			DependencyDepth: metrics.DependencyDepth,
			VisitedStates:   sol.MemoSize,
			Board:           board,
			Level:           level,
			Solution:        sol,
			BaseMetrics:     metrics,
			BaseSignals:     signals,
			SelectionBand:   it.band,
		})
	}
	return out, nil
}

func enrichmentBand(est float64, opt int) string {
	switch {
	case est >= 78 || opt >= 17:
		return "hard"
	case est >= 60 || opt >= 11:
		return "medium"
	default:
		return "lower-mid"
	}
}

func enrichmentSelectScore(row curatorShortlistRow) float64 {
	normOpt := clamp01(float64(row.CargoOptimalGestures) / 25.0)
	normEst := clamp01(row.EstimatedHumanDifficulty / 100.0)
	normDep := clamp01(float64(row.DependencyDepth) / 15.0)
	search := 0.0
	if row.VisitedStates > 0 {
		search = clamp01(math.Log(float64(row.VisitedStates)) / 14.0)
	}
	// Prefer interesting search, but avoid ultra-expensive solver nightmares for enrichment.
	score := 0.25*normOpt + 0.35*normEst + 0.20*normDep + 0.20*search
	if row.VisitedStates > 200_000 {
		score *= 0.55
	}
	if row.VisitedStates > 500_000 {
		score *= 0.5
	}
	return score
}

// RunEnrichmentPilot enriches selected RUSH-009 bases with essential movable 1x1.
func RunEnrichmentPilot(cfg EnrichmentConfig) (EnrichmentBatchResult, error) {
	start := time.Now()
	res := EnrichmentBatchResult{
		Config:   cfg,
		Rejected: map[string]int{},
	}
	bases, err := SelectEnrichmentBases(cfg)
	if err != nil {
		return res, err
	}
	budget := SolveBudget{TimeLimit: cfg.SolveTimeLimit, MaxVisited: cfg.MaxVisitedStates}
	usedFamilies := map[string]bool{}
	directCount := 0
	maxDirect := cfg.TargetAccepted
	if cfg.RoleDiversity && cfg.MaxDirectBlockerFrac < 1 {
		maxDirect = int(float64(cfg.TargetAccepted) * cfg.MaxDirectBlockerFrac)
		if maxDirect < 1 {
			maxDirect = 1
		}
	}

	// Phase A: gather one best candidate per base (off-corridor preferred).
	type baseHit struct {
		base EnrichmentBase
		cand *EnrichmentAccepted
		ms   int64
		rej  map[string]int
	}
	hits := []baseHit{}

	for _, base := range bases {
		if usedFamilies[base.FamilyID] {
			continue
		}
		res.BasesAttempted++
		t0 := time.Now()
		best, rej := enrichOneBase(base, cfg, budget, &res)
		ms := time.Since(t0).Milliseconds()
		res.PerBaseMs = append(res.PerBaseMs, ms)
		for k, v := range rej {
			res.Rejected[k] += v
		}
		if best == nil {
			continue
		}
		hits = append(hits, baseHit{base: base, cand: best, ms: ms, rej: rej})
		usedFamilies[base.FamilyID] = true
	}

	// Phase B: accept with role/corridor quotas + role diversity (deterministic).
	sort.SliceStable(hits, func(i, j int) bool {
		return betterEnrichmentRoleAware(hits[i].cand, hits[j].cand, cfg)
	})

	roleCount := map[OneByOneRole]int{}
	pickHit := func(requireNewRole bool) int {
		best := -1
		for i, h := range hits {
			if h.cand == nil {
				continue
			}
			c := h.cand
			isDirect := c.RoleEvidence.Role == RoleDirectTargetBlocker || c.RoleEvidence.InitiallyInTargetCorridor
			if cfg.RoleDiversity && isDirect && directCount >= maxDirect {
				continue
			}
			if requireNewRole && roleCount[c.RoleEvidence.Role] > 0 {
				continue
			}
			if best < 0 || betterEnrichmentRoleAware(c, hits[best].cand, cfg) {
				// Prefer underrepresented roles when not requiring new.
				if best >= 0 && !requireNewRole && cfg.RoleDiversity {
					if roleCount[c.RoleEvidence.Role] > roleCount[hits[best].cand.RoleEvidence.Role] {
						continue
					}
					if roleCount[c.RoleEvidence.Role] == roleCount[hits[best].cand.RoleEvidence.Role] &&
						!betterEnrichmentRoleAware(c, hits[best].cand, cfg) {
						continue
					}
					if roleCount[c.RoleEvidence.Role] < roleCount[hits[best].cand.RoleEvidence.Role] {
						best = i
						continue
					}
				}
				best = i
			}
		}
		return best
	}

	acceptIdx := func(i int) {
		h := hits[i]
		c := h.cand
		isDirect := c.RoleEvidence.Role == RoleDirectTargetBlocker || c.RoleEvidence.InitiallyInTargetCorridor
		fillRemovalDiagnostic(c, budget, &res)
		id := fmt.Sprintf("Candidate_%03d", len(res.Accepted)+1)
		c.CandidateID = id
		opt := c.EnrichedOptimal
		c.Level.LevelID = id
		c.Level.Source = "RushDatabaseEnriched"
		c.Level.CanonicalGestures = &opt
		if h.base.Level.Transplant != nil {
			cp := *h.base.Level.Transplant
			c.Level.Transplant = &cp
		}
		c.Level.Enrichment = &EnrichmentJSON{
			BaseCandidateId:                   h.base.CandidateID,
			BaseFamilyId:                      h.base.FamilyID,
			BaseSourcePuzzleId:                h.base.SourcePuzzleID,
			Added1x1Count:                     c.Added1x1Count,
			Essential1x1Count:                 c.Essential1x1Count,
			OneByOneMovesInOptimal:            c.OneByOneMovesInOptimal,
			BaseOptimal:                       h.base.BaseOptimal,
			EnrichedOptimal:                   c.EnrichedOptimal,
			OptimalDelta:                      c.OptimalDelta,
			EnrichmentImpact:                  c.EnrichmentImpact,
			PlacementCells:                    append([]int(nil), c.PlacementCells...),
			OneByOneRole:                      string(c.RoleEvidence.Role),
			OneByOneInitiallyInTargetCorridor: c.RoleEvidence.InitiallyInTargetCorridor,
			RoleEvidenceSummary:               c.RoleEvidence.Summary,
			ReleasedCells:                     append([]int(nil), c.RoleEvidence.ReleasedCells...),
			SubsequentPieceClass:              c.RoleEvidence.SubsequentPieceClass,
		}
		c.SolutionDoc = ExportSolutionJSON(c.Level, c.Board, c.Solution, time.Duration(c.ElapsedMs)*time.Millisecond, true)
		res.Accepted = append(res.Accepted, *c)
		roleCount[c.RoleEvidence.Role]++
		if isDirect {
			directCount++
		}
		hits[i].cand = nil // consumed
	}

	// First pass: seed distinct roles.
	if cfg.RoleDiversity {
		for len(roleCount) < cfg.MinDistinctRoles && len(res.Accepted) < cfg.TargetAccepted {
			i := pickHit(true)
			if i < 0 {
				break
			}
			acceptIdx(i)
		}
	}
	// Fill remaining with underrepresented roles.
	for len(res.Accepted) < cfg.TargetAccepted {
		i := pickHit(false)
		if i < 0 {
			break
		}
		acceptIdx(i)
	}

	res.TotalElapsed = time.Since(start)
	return res, nil
}

func enrichOneBase(base EnrichmentBase, cfg EnrichmentConfig, budget SolveBudget, res *EnrichmentBatchResult) (*EnrichmentAccepted, map[string]int) {
	rej := map[string]int{}
	local := cfg
	if base.VisitedStates > 100_000 {
		local.MaxPlacements1 = minInt(local.MaxPlacements1, 10)
		local.MaxPlacements2 = minInt(local.MaxPlacements2, 2)
	}
	if base.VisitedStates > 250_000 {
		local.MaxPlacements1 = minInt(local.MaxPlacements1, 6)
		local.MaxPlacements2 = 0
	}
	placements := enumerate1x1Placements(base.Board, local)
	var bestOff, bestAny *EnrichmentAccepted
	offSeen := 0
	for _, cells := range placements {
		off := true
		for _, cell := range cells {
			if CellInTargetExitCorridor(base.Board, cell) {
				off = false
				break
			}
		}
		if off {
			res.OffCorridorPlacementsEval++
			offSeen++
		} else {
			res.DirectPlacementsEval++
		}
		cand, reason := evaluatePlacement(base, cells, local, budget, res)
		res.PlacementsEvaluated++
		if reason != "" {
			rej[reason]++
			continue
		}
		if bestAny == nil || betterEnrichmentRoleAware(cand, bestAny, local) {
			bestAny = cand
		}
		if cand.OffCorridorPlacement {
			if bestOff == nil || betterEnrichmentRoleAware(cand, bestOff, local) {
				bestOff = cand
			}
			// Early stop only after exploring several off-corridor hits,
			// or when we found a non-SpaceMaker soft-delta role.
			if local.PreferOffCorridor && bestOff != nil &&
				bestOff.OptimalDelta >= local.SoftMinDelta && bestOff.OptimalDelta <= local.SoftMaxDelta &&
				bestOff.Essential1x1Count >= 1 {
				if !local.RoleDiversity {
					return bestOff, rej
				}
				if bestOff.RoleEvidence.Role != RoleSpaceMaker || offSeen >= 8 {
					return bestOff, rej
				}
			}
		}
	}
	if local.PreferOffCorridor && bestOff != nil {
		return bestOff, rej
	}
	return bestAny, rej
}

func enumerate1x1Placements(board *Board, cfg EnrichmentConfig) [][]int {
	empty := []int{}
	n := board.Width * board.Height
	for i := 0; i < n; i++ {
		if !board.occupied[i] {
			empty = append(empty, i)
		}
	}
	off := []int{}
	on := []int{}
	for _, cell := range empty {
		if CellInTargetExitCorridor(board, cell) {
			on = append(on, cell)
		} else {
			off = append(off, cell)
		}
	}
	sortCells := func(cells []int) {
		sort.SliceStable(cells, func(i, j int) bool {
			si, sj := placementPriority(board, cells[i]), placementPriority(board, cells[j])
			if cfg.PreferOffCorridor {
				// Off-corridor: prefer near pieces / mid-board interactions, not exit column.
				si, sj = offCorridorPriority(board, cells[i]), offCorridorPriority(board, cells[j])
			}
			if si != sj {
				return si < sj
			}
			return cells[i] < cells[j]
		})
	}
	sortCells(off)
	sortCells(on)

	ordered := []int{}
	if cfg.PreferOffCorridor {
		ordered = append(ordered, off...)
		ordered = append(ordered, on...)
	} else {
		// Legacy: corridor-near first.
		all := append(append([]int{}, on...), off...)
		sort.SliceStable(all, func(i, j int) bool {
			si, sj := placementPriority(board, all[i]), placementPriority(board, all[j])
			if si != sj {
				return si < sj
			}
			return all[i] < all[j]
		})
		ordered = all
	}

	out := [][]int{}
	limit1 := cfg.MaxPlacements1
	if limit1 > len(ordered) {
		limit1 = len(ordered)
	}
	for i := 0; i < limit1; i++ {
		out = append(out, []int{ordered[i]})
	}
	pairs := 0
	pool := ordered
	if cfg.PreferOffCorridor && len(off) > 0 {
		pool = off
	}
	for i := 0; i < len(pool) && pairs < cfg.MaxPlacements2; i++ {
		for j := i + 1; j < len(pool) && pairs < cfg.MaxPlacements2; j++ {
			if cellManhattan(board.Width, pool[i], pool[j]) > 3 {
				continue
			}
			out = append(out, []int{pool[i], pool[j]})
			pairs++
		}
	}
	return out
}

func offCorridorPriority(board *Board, cell int) int {
	w := board.Width
	r, c := cell/w, cell%w
	tr := board.Pieces[0].Row(w)
	tc := board.Pieces[0].Col(w)
	// Prefer cells near target/pieces but NOT on exit corridor (already filtered).
	distTarget := absInt(r-tr) + absInt(c-tc)
	// Mild preference for side columns / adjacency to occupied cells.
	adj := 0
	for _, d := range []int{-1, 1, -w, w} {
		nb := cell + d
		if nb >= 0 && nb < w*board.Height && board.occupied[nb] {
			adj++
		}
	}
	return distTarget*5 - adj*3 + absInt(c-tc)
}

func placementPriority(board *Board, cell int) int {
	w := board.Width
	r, c := cell/w, cell%w
	tr := board.Pieces[0].Row(w)
	tc := board.Pieces[0].Col(w)
	// Lower is better: near exit column and above/near target.
	distExitCol := absInt(c - board.ExitCol)
	distTarget := absInt(r-tr) + absInt(c-tc)
	return distExitCol*10 + distTarget
}

func cellManhattan(w, a, b int) int {
	return absInt(a/w-b/w) + absInt(a%w-b%w)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func evaluatePlacement(base EnrichmentBase, cells []int, cfg EnrichmentConfig, budget SolveBudget, res *EnrichmentBatchResult) (*EnrichmentAccepted, string) {
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
	sol := board.SolveWithBudget(budget)
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

	// Strong necessity: freeze added 1x1s.
	restricted := board.Copy()
	restricted.ImmobilePieces = make([]bool, len(restricted.Pieces))
	for _, idx := range addedIdx {
		restricted.ImmobilePieces[idx] = true
	}
	t1 := time.Now()
	rsol := restricted.SolveWithBudget(budget)
	_ = time.Since(t1)
	res.NecessitySolves++
	if rsol.TimedOut || rsol.BudgetExceeded {
		return nil, "NecessityUnknown"
	}
	restrictedOptimal := -1
	if rsol.Solvable {
		restrictedOptimal = rsol.NumMoves
	}
	essential := 0
	if !rsol.Solvable || (rsol.Solvable && rsol.NumMoves > sol.NumMoves) {
		essential = 1
	} else {
		return nil, "OneByOneNotEssential"
	}

	b2 := board.Copy()
	if err := b2.Replay(sol.Moves); err != nil {
		return nil, "ReplayFailed"
	}

	metrics := BuildCandidateMetrics(board, sol)
	impact := enrichmentImpact(delta, base.DependencyDepth, metrics.DependencyDepth, unitMoves, base.VisitedStates, sol.MemoSize)
	role := ClassifyOneByOneRole(board, sol, addedIdx)
	off := !role.InitiallyInTargetCorridor
	for _, cell := range cells {
		if CellInTargetExitCorridor(base.Board, cell) {
			off = false
			break
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
		Essential1x1Count:       essential,
		RestrictedOptimal:       restrictedOptimal,
		RestrictedSolvable:      rsol.Solvable,
		OptimalWithout1x1:       -1,
		Without1x1Solvable:      false,
		EnrichedOptimal:         sol.NumMoves,
		OptimalDelta:            delta,
		BaseDependencyDepth:     base.DependencyDepth,
		EnrichedDependencyDepth: metrics.DependencyDepth,
		BaseVisited:             base.VisitedStates,
		EnrichedVisited:         sol.MemoSize,
		EnrichmentImpact:        impact,
		ASCIIPreview:            asciiCargoBoard(board),
		RoleEvidence:            role,
		OffCorridorPlacement:    off,
	}, ""
}

func fillRemovalDiagnostic(c *EnrichmentAccepted, budget SolveBudget, res *EnrichmentBatchResult) {
	fresh, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		return
	}
	idxs := []int{}
	for i, p := range fresh.Pieces {
		if p.Kind != PieceUnit {
			continue
		}
		for _, cell := range c.PlacementCells {
			if p.Position == cell {
				idxs = append(idxs, i)
				break
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(idxs)))
	for _, idx := range idxs {
		fresh.RemovePiece(idx)
		if idx < len(fresh.Labels) {
			fresh.Labels = append(fresh.Labels[:idx], fresh.Labels[idx+1:]...)
		}
	}
	wsol := fresh.SolveWithBudget(budget)
	res.NecessitySolves++
	if !wsol.TimedOut && !wsol.BudgetExceeded && wsol.Solvable {
		c.OptimalWithout1x1 = wsol.NumMoves
		c.Without1x1Solvable = true
	}
}

func countUnitMoves(board *Board, sol Solution, addedIdx []int) int {
	added := map[int]bool{}
	for _, i := range addedIdx {
		added[i] = true
	}
	n := 0
	work := board.Copy()
	for _, m := range sol.Moves {
		if added[m.Piece] && work.Pieces[m.Piece].Kind == PieceUnit {
			n++
		}
		work.DoMove(m)
	}
	return n
}

func enrichmentImpact(delta, baseDep, enrDep, unitMoves, baseVis, enrVis int) float64 {
	depDelta := float64(enrDep - baseDep)
	visDelta := 0.0
	if baseVis > 0 && enrVis > 0 {
		visDelta = math.Log(float64(enrVis)+1) - math.Log(float64(baseVis)+1)
	}
	score := 0.35*float64(delta) + 0.25*depDelta + 0.25*float64(unitMoves) + 0.15*visDelta
	return round3(score)
}

func betterEnrichment(a, b *EnrichmentAccepted, cfg EnrichmentConfig) bool {
	return betterEnrichmentRoleAware(a, b, cfg)
}

func betterEnrichmentRoleAware(a, b *EnrichmentAccepted, cfg EnrichmentConfig) bool {
	if cfg.PreferOffCorridor {
		if a.OffCorridorPlacement != b.OffCorridorPlacement {
			return a.OffCorridorPlacement
		}
		aDirect := a.RoleEvidence.Role == RoleDirectTargetBlocker
		bDirect := b.RoleEvidence.Role == RoleDirectTargetBlocker
		if aDirect != bDirect {
			return !aDirect
		}
	}
	if cfg.RoleDiversity {
		ra, rb := roleDiversityRank(a.RoleEvidence.Role), roleDiversityRank(b.RoleEvidence.Role)
		if ra != rb {
			return ra < rb
		}
	}
	aSoft := softDeltaScore(a.OptimalDelta, cfg)
	bSoft := softDeltaScore(b.OptimalDelta, cfg)
	if aSoft != bSoft {
		return aSoft > bSoft
	}
	if a.Essential1x1Count != b.Essential1x1Count {
		return a.Essential1x1Count > b.Essential1x1Count
	}
	if a.RoleEvidence.SubsequentUsesReleasedCell != b.RoleEvidence.SubsequentUsesReleasedCell {
		return a.RoleEvidence.SubsequentUsesReleasedCell
	}
	if a.OneByOneMovesInOptimal != b.OneByOneMovesInOptimal {
		return a.OneByOneMovesInOptimal < b.OneByOneMovesInOptimal
	}
	depA := a.EnrichedDependencyDepth - a.BaseDependencyDepth
	depB := b.EnrichedDependencyDepth - b.BaseDependencyDepth
	if depA != depB {
		return depA > depB
	}
	if a.EnrichedVisited != b.EnrichedVisited {
		return a.EnrichedVisited < b.EnrichedVisited
	}
	if a.Added1x1Count != b.Added1x1Count {
		return a.Added1x1Count < b.Added1x1Count
	}
	return placementKey(a.PlacementCells) < placementKey(b.PlacementCells)
}

func roleDiversityRank(r OneByOneRole) int {
	switch r {
	case RoleGateKeeper:
		return 0
	case RoleSideShuttle:
		return 1
	case RoleIndirectBlocker:
		return 2
	case RoleSpaceMaker:
		return 3
	case RoleDirectTargetBlocker:
		return 4
	default:
		return 5
	}
}

func softDeltaScore(delta int, cfg EnrichmentConfig) int {
	if delta >= cfg.SoftMinDelta && delta <= cfg.SoftMaxDelta {
		return 2
	}
	if delta > cfg.SoftMaxDelta {
		return 1
	}
	return 0
}

func placementKey(cells []int) string {
	c := append([]int(nil), cells...)
	sort.Ints(c)
	return fmt.Sprintf("%v", c)
}

// WriteEnrichmentBatch writes Unity-compatible enriched shortlist.
func WriteEnrichmentBatch(dir string, res EnrichmentBatchResult) error {
	candDir := filepath.Join(dir, "Candidates")
	solDir := filepath.Join(dir, "Solutions")
	if err := os.MkdirAll(candDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(solDir, 0o755); err != nil {
		return err
	}
	for _, c := range res.Accepted {
		if err := WriteJSONFile(filepath.Join(candDir, c.CandidateID+".json"), c.Level); err != nil {
			return err
		}
		if err := WriteJSONFile(filepath.Join(solDir, c.CandidateID+".solution.json"), c.SolutionDoc); err != nil {
			return err
		}
		if c.ASCIIPreview != "" {
			_ = os.WriteFile(filepath.Join(candDir, c.CandidateID+".ascii.txt"), []byte(c.ASCIIPreview), 0o644)
		}
	}
	if err := WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), res.ToManifest()); err != nil {
		return err
	}
	if err := WriteJSONFile(filepath.Join(dir, "EnrichmentReport.json"), res.ToReport()); err != nil {
		return err
	}
	return writeEnrichmentHumanReviewCSV(filepath.Join(dir, "HumanReview.csv"), res.Accepted)
}

func writeEnrichmentHumanReviewCSV(path string, accepted []EnrichmentAccepted) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{
		"CandidateId", "BaseCandidateId", "FamilyId", "BaseOptimal", "EnrichedOptimal",
		"OneByOneRole", "InitiallyInTargetCorridor",
		"HumanMoves", "HumanDifficulty", "Did1x1FeelNatural", "Did1x1RoleFeelDifferent", "Keep", "Notes",
	})
	for _, c := range accepted {
		_ = w.Write([]string{
			c.CandidateID,
			c.Base.CandidateID,
			c.Base.FamilyID,
			fmt.Sprintf("%d", c.Base.BaseOptimal),
			fmt.Sprintf("%d", c.EnrichedOptimal),
			string(c.RoleEvidence.Role),
			fmt.Sprintf("%v", c.RoleEvidence.InitiallyInTargetCorridor),
			"", "", "", "", "", "",
		})
	}
	w.Flush()
	return w.Error()
}

func (r EnrichmentBatchResult) RoleDistribution() map[string]int {
	out := map[string]int{
		string(RoleDirectTargetBlocker): 0,
		string(RoleIndirectBlocker):     0,
		string(RoleSpaceMaker):          0,
		string(RoleSideShuttle):         0,
		string(RoleGateKeeper):          0,
	}
	for _, c := range r.Accepted {
		if len(c.CubeDiags) > 0 {
			for _, d := range c.CubeDiags {
				out[string(d.Role)]++
			}
			continue
		}
		if c.RoleEvidence.Role != "" {
			out[string(c.RoleEvidence.Role)]++
		}
	}
	return out
}

func (r EnrichmentBatchResult) CorridorFlags() (inN, outN int) {
	for _, c := range r.Accepted {
		if c.RoleEvidence.InitiallyInTargetCorridor {
			inN++
		} else {
			outN++
		}
	}
	return
}

func (r EnrichmentBatchResult) ToManifest() map[string]interface{} {
	ver := EnrichmentVersion
	pipe := "CargoFlowOneByOneEnrichment"
	if r.Config.RoleDiversity {
		ver = EnrichmentRoleDiversityVersion
		pipe = "CargoFlowOneByOneRoleDiversity"
	}
	cands := []map[string]interface{}{}
	for _, c := range r.Accepted {
		cands = append(cands, map[string]interface{}{
			"candidateId":                     c.CandidateID,
			"baseCandidateId":                 c.Base.CandidateID,
			"baseFamilyId":                    c.Base.FamilyID,
			"baseSourcePuzzleId":              c.Base.SourcePuzzleID,
			"baseOptimal":                     c.Base.BaseOptimal,
			"enrichedOptimal":                 c.EnrichedOptimal,
			"optimalDelta":                    c.OptimalDelta,
			"added1x1Count":                   c.Added1x1Count,
			"oneByOneMoves":                   c.OneByOneMovesInOptimal,
			"essential1x1Count":               c.Essential1x1Count,
			"oneByOneRole":                    string(c.RoleEvidence.Role),
			"oneByOneInitiallyInTargetCorridor": c.RoleEvidence.InitiallyInTargetCorridor,
			"levelFile":                       "Candidates/" + c.CandidateID + ".json",
			"solutionFile":                    "Solutions/" + c.CandidateID + ".solution.json",
			"replayVerified":                  c.ReplayVerified,
		})
	}
	return map[string]interface{}{
		"generatorVersion": ver,
		"pipeline":         pipe,
		"accepted":         len(r.Accepted),
		"roleDistribution": r.RoleDistribution(),
		"candidates":       cands,
	}
}

func (r EnrichmentBatchResult) ToReport() map[string]interface{} {
	rows := []map[string]interface{}{}
	for _, c := range r.Accepted {
		rows = append(rows, map[string]interface{}{
			"candidate":                   c.CandidateID,
			"baseCandidate":               c.Base.CandidateID,
			"familyId":                    c.Base.FamilyID,
			"baseOptimal":                 c.Base.BaseOptimal,
			"enrichedOptimal":             c.EnrichedOptimal,
			"delta":                       c.OptimalDelta,
			"oneByOneRole":                string(c.RoleEvidence.Role),
			"initiallyInTargetCorridor":   c.RoleEvidence.InitiallyInTargetCorridor,
			"oneByOneMoves":               c.OneByOneMovesInOptimal,
			"essential":                   c.Essential1x1Count,
			"dependencyEvidence":          c.RoleEvidence.Summary,
			"releasedCells":               c.RoleEvidence.ReleasedCells,
			"subsequentPieceClass":        c.RoleEvidence.SubsequentPieceClass,
			"dependencyBefore":            c.BaseDependencyDepth,
			"dependencyAfter":             c.EnrichedDependencyDepth,
			"replay":                      c.ReplayVerified,
			"placementCells":              c.PlacementCells,
		})
	}
	inN, outN := r.CorridorFlags()
	avg, med, maxV := 0.0, 0.0, 0.0
	if len(r.PerBaseMs) > 0 {
		avg = meanInt64(r.PerBaseMs)
		med = medianInt64(r.PerBaseMs)
		maxV = float64(maxInt64(r.PerBaseMs))
	}
	return map[string]interface{}{
		"performance": map[string]interface{}{
			"basesAttempted":              r.BasesAttempted,
			"placementsEvaluated":         r.PlacementsEvaluated,
			"directPlacementsEvaluated":   r.DirectPlacementsEval,
			"offCorridorPlacementsEvaluated": r.OffCorridorPlacementsEval,
			"exactSolves":                 r.ExactSolves,
			"necessitySolves":             r.NecessitySolves,
			"accepted":                    len(r.Accepted),
			"totalElapsedMs":              r.TotalElapsed.Milliseconds(),
			"avgPerBaseMs":                avg,
			"medianPerBaseMs":             med,
			"maxPerBaseMs":                maxV,
		},
		"roleDistribution": r.RoleDistribution(),
		"corridorFlags": map[string]int{
			"initiallyInTargetCorridor": inN,
			"outsideTargetCorridor":     outN,
		},
		"rejected":  r.Rejected,
		"accepted":  rows,
		"limitations": []string{
			"DirectTargetBlocker remains valid but is quota-capped in role-diversity mode.",
			"Role labels are explainable heuristics from solution occupancy, not ML.",
			"EnrichmentImpact is diagnostic, not human difficulty.",
		},
	}
}
