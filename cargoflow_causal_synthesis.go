package rush

import (
	"fmt"
	"sort"
)

type CausalTemplate string

const (
	CausalNone                CausalTemplate = "None"
	CausalLower1x1SpaceMaker  CausalTemplate = "Lower1x1SpaceMaker"
	CausalLowerParkingUnlock  CausalTemplate = "LowerParkingUnlock"
	CausalCrossRegionLongGate CausalTemplate = "CrossRegionLongGate"
	CausalSideToLowerToUpper  CausalTemplate = "SideToLowerToUpper"
)

type CausalInterventionEvidence struct {
	PieceIndex       int    `json:"pieceIndex"`
	Region           string `json:"region"`
	FreezeUnsolvable bool   `json:"freezeUnsolvable"`
	FreezeOptimal    int    `json:"freezeOptimal,omitempty"`
	OptimalIncrease  int    `json:"optimalIncrease,omitempty"`
	CausalPathToExit bool   `json:"causalPathToExit"`
}

type CausalProof struct {
	Valid                          bool                         `json:"valid"`
	Template                       CausalTemplate               `json:"template"`
	BaseOptimal                    int                          `json:"baseOptimal"`
	NativeOptimal                  int                          `json:"nativeOptimal"`
	OptimalDelta                   int                          `json:"optimalDelta"`
	BaseGraphEdges                 int                          `json:"baseGraphEdges"`
	BaseGraphDepth                 int                          `json:"baseGraphDepth"`
	BaseCrossRegionEdges           int                          `json:"baseCrossRegionEdges"`
	BaseDependencyRowSpan          int                          `json:"baseDependencyRowSpan"`
	BaseDependencyColumnSpan       int                          `json:"baseDependencyColumnSpan"`
	BaseContainment                float64                      `json:"baseContainment"`
	GraphEdges                     int                          `json:"graphEdges"`
	GraphDepth                     int                          `json:"graphDepth"`
	DependencyRowSpan              int                          `json:"dependencyRowSpan"`
	DependencyColumnSpan           int                          `json:"dependencyColumnSpan"`
	Best6x6Containment             float64                      `json:"best6x6Containment"`
	AddedPieceIndexes              []int                        `json:"addedPieceIndexes"`
	CrossRegionDependencyEdgeCount int                          `json:"crossRegionDependencyEdgeCount"`
	LowerToUpperDependencyEdges    int                          `json:"lowerToUpperDependencyEdges"`
	SideToUpperDependencyEdges     int                          `json:"sideToUpperDependencyEdges"`
	LowerToCorridorDependencyEdges int                          `json:"lowerToCorridorDependencyEdges"`
	SideToCorridorDependencyEdges  int                          `json:"sideToCorridorDependencyEdges"`
	CrossRegionDependencyDepth     int                          `json:"crossRegionDependencyDepth"`
	RequiredLowerPieceCount        int                          `json:"requiredLowerPieceCount"`
	RequiredSidePieceCount         int                          `json:"requiredSidePieceCount"`
	CausalRegionCount              int                          `json:"causalRegionCount"`
	DistributedCausalityScore      float64                      `json:"distributedCausalityScore"`
	MultiRegionChain               bool                         `json:"multiRegionChain"`
	ReplayVerified                 bool                         `json:"replayVerified"`
	Interventions                  []CausalInterventionEvidence `json:"interventions"`
	EdgeTypeDistribution           map[string]int               `json:"edgeTypeDistribution"`
	RejectionReason                string                       `json:"rejectionReason,omitempty"`
}

type CausalSynthesisConfig struct {
	MaxProposalsPerBoard int
	MaxAcceptedPerBoard  int
	HardShortcutDelta    int
	SoftDeltaMax         int
	AllowTwoPieceChains  bool
	CoreOffsetX          int
	CoreOffsetY          int
}

func DefaultCausalSynthesisConfig() CausalSynthesisConfig {
	return CausalSynthesisConfig{
		MaxProposalsPerBoard: 16,
		MaxAcceptedPerBoard:  2,
		HardShortcutDelta:    -2,
		SoftDeltaMax:         10,
		AllowTwoPieceChains:  true,
	}
}

type causalSynthesisProposal struct {
	Template  CausalTemplate
	AddPieces []Piece
	Labels    []string
}

type CausalSynthesisCandidate struct {
	Board *Board
	Sol   Solution
	Proof CausalProof
}

// GenerateCausalSynthesisCandidates inserts bounded blockers into cells used by
// the base optimal exit-clearing sequence. Acceptance is proof-driven: exact
// solve, replay, intervention necessity, and a causal path to upper/corridor.
func GenerateCausalSynthesisCandidates(base *Board, baseOptimal int, budget SolveBudget, cfg CausalSynthesisConfig) ([]CausalSynthesisCandidate, map[string]int) {
	rejected := map[string]int{}
	if base == nil {
		rejected["NilBase"]++
		return nil, rejected
	}
	if cfg.MaxProposalsPerBoard <= 0 {
		cfg = DefaultCausalSynthesisConfig()
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	baseSol := base.SolveWithBudget(budget)
	if !baseSol.Solvable || baseSol.TimedOut || baseSol.BudgetExceeded {
		rejected["BaseUnsolved"]++
		return nil, rejected
	}
	if baseOptimal <= 0 {
		baseOptimal = baseSol.NumMoves
	}
	baseSpatial := ComputeSpatialDependencyMetrics(base, baseSol, budget)
	baseCore := ComputeCoreSpaceMetrics(base, cfg.CoreOffsetX, cfg.CoreOffsetY, baseSol, budget)
	proposals := proposeCausalSynthesis(base, baseSol, cfg)
	if len(proposals) > cfg.MaxProposalsPerBoard {
		proposals = proposals[:cfg.MaxProposalsPerBoard]
	}
	out := []CausalSynthesisCandidate{}
	seen := map[string]bool{}
	for _, proposal := range proposals {
		if len(out) >= cfg.MaxAcceptedPerBoard {
			break
		}
		board, reason := applyCausalProposal(base, proposal)
		if board == nil {
			rejected[reason]++
			continue
		}
		fp := nativeVariantFingerprint(board, AugmentationClass(proposal.Template))
		if seen[fp] {
			rejected["DuplicateFingerprint"]++
			continue
		}
		seen[fp] = true
		if board.cargoTargetCanExit() {
			rejected["ImmediateVictory"]++
			continue
		}
		sol := board.SolveWithBudget(budget)
		if sol.TimedOut || sol.BudgetExceeded {
			rejected["DifficultyUnknown"]++
			continue
		}
		if !sol.Solvable {
			rejected["Unsolvable"]++
			continue
		}
		work := board.Copy()
		if err := work.Replay(sol.Moves); err != nil {
			rejected["ReplayFailed"]++
			continue
		}
		if sol.NumMoves-baseOptimal < cfg.HardShortcutDelta {
			rejected["ShortcutDegradation"]++
			continue
		}
		if sol.NumMoves <= 1 || (baseOptimal >= 6 && sol.NumMoves*2 < baseOptimal) {
			rejected["TrivialCollapse"]++
			continue
		}
		added := make([]int, 0, len(proposal.AddPieces))
		for i := range proposal.AddPieces {
			added = append(added, len(base.Pieces)+i)
		}
		proof := ProveCausalSynthesis(board, sol, baseOptimal, added, proposal.Template, budget)
		if !proof.Valid {
			reason := proof.RejectionReason
			if reason == "" {
				reason = "CausalProofFailed"
			}
			rejected[reason]++
			continue
		}
		proof.BaseGraphEdges = baseSpatial.DependencyGraphEdgeCount
		proof.BaseGraphDepth = baseSpatial.DependencyGraphDepth
		proof.BaseCrossRegionEdges = baseSpatial.CrossTargetDependencies
		proof.BaseDependencyRowSpan = baseSpatial.DependencyRowSpan
		proof.BaseDependencyColumnSpan = baseSpatial.DependencyColumnSpan
		proof.BaseContainment = baseCore.Best6x6MeaningfulContainmentRatio
		nativeSpatial := ComputeSpatialDependencyMetrics(board, sol, budget)
		nativeCore := ComputeCoreSpaceMetrics(board, cfg.CoreOffsetX, cfg.CoreOffsetY, sol, budget)
		proof.GraphEdges = nativeSpatial.DependencyGraphEdgeCount
		proof.GraphDepth = nativeSpatial.DependencyGraphDepth
		proof.DependencyRowSpan = nativeSpatial.DependencyRowSpan
		proof.DependencyColumnSpan = nativeSpatial.DependencyColumnSpan
		proof.Best6x6Containment = nativeCore.Best6x6MeaningfulContainmentRatio
		out = append(out, CausalSynthesisCandidate{Board: board, Sol: sol, Proof: proof})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Proof, out[j].Proof
		ar := causalDifficultyRank(a.OptimalDelta, cfg.SoftDeltaMax)
		br := causalDifficultyRank(b.OptimalDelta, cfg.SoftDeltaMax)
		if ar != br {
			return ar < br
		}
		if a.DistributedCausalityScore != b.DistributedCausalityScore {
			return a.DistributedCausalityScore > b.DistributedCausalityScore
		}
		ad, bd := absInt(a.OptimalDelta), absInt(b.OptimalDelta)
		return ad < bd
	})
	return out, rejected
}

func causalDifficultyRank(delta, softMax int) int {
	if delta >= 0 && (softMax <= 0 || delta <= softMax) {
		return 0
	}
	if delta > softMax {
		return 1
	}
	return 2
}

func proposeCausalSynthesis(base *Board, sol Solution, cfg CausalSynthesisConfig) []causalSynthesisProposal {
	critical := criticalLandingCells(base, sol)
	singles := []causalSynthesisProposal{}
	for _, cell := range critical {
		for _, candidate := range causalPiecesCoveringCell(base, cell) {
			template := CausalLowerParkingUnlock
			label := "CP"
			switch candidate.Size {
			case 1:
				template, label = CausalLower1x1SpaceMaker, "C1"
			case 3:
				template, label = CausalCrossRegionLongGate, "C3"
			}
			p := causalSynthesisProposal{Template: template, AddPieces: []Piece{candidate}, Labels: []string{label}}
			singles = append(singles, p)
			if len(singles) >= cfg.MaxProposalsPerBoard*4 {
				break
			}
		}
		if len(singles) >= cfg.MaxProposalsPerBoard*4 {
			break
		}
	}

	out := []causalSynthesisProposal{}
	seen := map[string]bool{}
	appendProposal := func(proposal causalSynthesisProposal) {
		if len(out) >= cfg.MaxProposalsPerBoard {
			return
		}
		key := causalProposalKey(proposal)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, proposal)
	}
	// Put one proposal per single-piece template at the front of the bounded search.
	for _, template := range []CausalTemplate{
		CausalLower1x1SpaceMaker, CausalLowerParkingUnlock, CausalCrossRegionLongGate,
	} {
		for _, proposal := range singles {
			if proposal.Template == template {
				appendProposal(proposal)
				break
			}
		}
	}
	if cfg.AllowTwoPieceChains {
		pairAdded := false
		for i := 0; i < len(singles); i++ {
			for j := i + 1; j < len(singles); j++ {
				a, b := singles[i].AddPieces[0], singles[j].AddPieces[0]
				if piecesOverlap(a, b, base.Width) {
					continue
				}
				ra := initialPieceRegion(a, base)
				rb := initialPieceRegion(b, base)
				if ra == rb {
					continue
				}
				appendProposal(causalSynthesisProposal{
					Template:  CausalSideToLowerToUpper,
					AddPieces: []Piece{a, b},
					Labels:    []string{"CS", "CL"},
				})
				pairAdded = true
				break
			}
			if pairAdded {
				break
			}
		}
	}
	for _, proposal := range singles {
		appendProposal(proposal)
	}
	return out
}

func causalProposalKey(proposal causalSynthesisProposal) string {
	key := string(proposal.Template)
	for _, p := range proposal.AddPieces {
		key += fmt.Sprintf("|%d:%d:%d:%d", p.Position, p.Size, p.Orientation, p.Kind)
	}
	return key
}

func criticalLandingCells(base *Board, sol Solution) []int {
	initialOccupied := append([]bool{}, base.occupied...)
	work := base.Copy()
	seen := map[int]bool{}
	out := []int{}
	for _, mv := range sol.Moves {
		if mv.Exit {
			work.DoMove(mv)
			continue
		}
		before := work.Pieces[mv.Piece]
		work.DoMove(mv)
		after := work.Pieces[mv.Piece]
		old := cellsSet(pieceOccupancyCells(before, base.Width))
		for _, cell := range pieceOccupancyCells(after, base.Width) {
			if old[cell] || initialOccupied[cell] || seen[cell] {
				continue
			}
			seen[cell] = true
			out = append(out, cell)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := out[i]/base.Width, out[j]/base.Width
		return ri > rj // lower cells first
	})
	return out
}

func causalPiecesCoveringCell(base *Board, cell int) []Piece {
	w, h := base.Width, base.Height
	row, col := cell/w, cell%w
	target := base.Pieces[0]
	top, bottom := target.Row(w), target.Row(w)+target.Size-1
	out := []Piece{}
	try := func(p Piece) {
		if !pieceFitsBoard(p, w, h) || pieceTouchesOccupied(base, p) {
			return
		}
		region := classifyPieceRelativeToTarget(p, w, top, bottom)
		if region != "Below" && region != "Side" {
			return
		}
		out = append(out, p)
	}
	try(Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit})
	for _, size := range []int{2, 3} {
		for start := col - size + 1; start <= col; start++ {
			if start >= 0 && start+size <= w {
				try(Piece{Position: row*w + start, Size: size, Orientation: Horizontal, Kind: PieceNormal})
			}
		}
		for start := row - size + 1; start <= row; start++ {
			if start >= 0 && start+size <= h {
				try(Piece{Position: start*w + col, Size: size, Orientation: Vertical, Kind: PieceNormal})
			}
		}
	}
	return out
}

func pieceTouchesOccupied(board *Board, p Piece) bool {
	for _, cell := range pieceOccupancyCells(p, board.Width) {
		if board.occupied[cell] {
			return true
		}
	}
	return false
}

func piecesOverlap(a, b Piece, width int) bool {
	ac := cellsSet(pieceOccupancyCells(a, width))
	for _, cell := range pieceOccupancyCells(b, width) {
		if ac[cell] {
			return true
		}
	}
	return false
}

func applyCausalProposal(base *Board, proposal causalSynthesisProposal) (*Board, string) {
	board := base.Copy()
	for i, p := range proposal.AddPieces {
		if !board.AddPiece(p) {
			return nil, "AddPieceOverlap"
		}
		label := fmt.Sprintf("C%02d", i+1)
		if i < len(proposal.Labels) {
			label = proposal.Labels[i]
		}
		board.Labels = append(board.Labels, label)
	}
	if err := board.Validate(); err != nil {
		return nil, "ValidateFailed"
	}
	return board, ""
}

type causalProofGraph struct {
	eventPiece  map[int]int
	eventRegion map[int]string
	adj         map[int][]int
	edges       map[string]bool
	edgeTypes   map[string]int
}

func buildCausalProofGraph(board *Board, sol Solution) causalProofGraph {
	g := causalProofGraph{
		eventPiece: map[int]int{}, eventRegion: map[int]string{},
		adj: map[int][]int{}, edges: map[string]bool{}, edgeTypes: map[string]int{},
	}
	if board == nil || len(board.Pieces) == 0 {
		return g
	}
	target := board.Pieces[0]
	top, bottom := target.Row(board.Width), target.Row(board.Width)+target.Size-1
	lastVacated := map[int]int{}
	work := board.Copy()
	for event, mv := range sol.Moves {
		g.eventPiece[event] = mv.Piece
		required := []int{}
		region := "Corridor"
		if mv.Exit {
			for row := 0; row < work.Pieces[0].Row(work.Width); row++ {
				required = append(required, row*work.Width+work.exitColumn())
			}
		} else {
			before := work.Pieces[mv.Piece]
			after := before
			stride := before.Stride(work.Width)
			if before.Kind == PieceUnit {
				if mv.Axis == Vertical {
					stride = work.Width
				} else {
					stride = 1
				}
			}
			after.Position += mv.Steps * stride
			old := cellsSet(pieceOccupancyCells(before, work.Width))
			for _, cell := range pieceOccupancyCells(after, work.Width) {
				if !old[cell] {
					required = append(required, cell)
				}
			}
			cells := unionCells(pieceOccupancyCells(before, work.Width), pieceOccupancyCells(after, work.Width))
			region = causalEventRegion(cells, work.Width, work.exitColumn(), top, bottom)
		}
		g.eventRegion[event] = region
		for _, cell := range required {
			parent, ok := lastVacated[cell]
			if !ok {
				continue
			}
			key := fmt.Sprintf("%d>%d", parent, event)
			if g.edges[key] {
				continue
			}
			g.edges[key] = true
			g.adj[parent] = append(g.adj[parent], event)
			g.edgeTypes[g.eventRegion[parent]+"→"+region]++
		}
		if mv.Exit {
			work.DoMove(mv)
			continue
		}
		before := work.Pieces[mv.Piece]
		old := cellsSet(pieceOccupancyCells(before, work.Width))
		work.DoMove(mv)
		newCells := cellsSet(pieceOccupancyCells(work.Pieces[mv.Piece], work.Width))
		for cell := range old {
			if !newCells[cell] {
				lastVacated[cell] = event
			}
		}
	}
	return g
}

func causalEventRegion(cells []int, width, exitCol, top, bottom int) string {
	for _, cell := range cells {
		row, col := cell/width, cell%width
		if col == exitCol && row < top {
			return "Corridor"
		}
	}
	return normalizeCausalRegion(classifyCellsRelativeToTarget(cells, width, top, bottom))
}

func normalizeCausalRegion(region string) string {
	switch region {
	case "Above":
		return "Upper"
	case "Below":
		return "Lower"
	default:
		return region
	}
}

func ProveCausalSynthesis(board *Board, sol Solution, baseOptimal int, added []int, template CausalTemplate, budget SolveBudget) CausalProof {
	proof := CausalProof{
		Template: template, BaseOptimal: baseOptimal, NativeOptimal: sol.NumMoves,
		OptimalDelta: sol.NumMoves - baseOptimal, AddedPieceIndexes: append([]int{}, added...),
		ReplayVerified: true, EdgeTypeDistribution: map[string]int{},
	}
	if board == nil || !sol.Solvable {
		proof.RejectionReason = "Unsolved"
		return proof
	}
	work := board.Copy()
	if err := work.Replay(sol.Moves); err != nil {
		proof.ReplayVerified = false
		proof.RejectionReason = "ReplayFailed"
		return proof
	}
	graph := buildCausalProofGraph(board, sol)
	for key, count := range graph.edgeTypes {
		proof.EdgeTypeDistribution[key] = count
		if regionEdgeCrosses(key) {
			proof.CrossRegionDependencyEdgeCount += count
		}
	}
	proof.LowerToUpperDependencyEdges = graph.edgeTypes["Lower→Upper"]
	proof.SideToUpperDependencyEdges = graph.edgeTypes["Side→Upper"]
	proof.LowerToCorridorDependencyEdges = graph.edgeTypes["Lower→Corridor"]
	proof.SideToCorridorDependencyEdges = graph.edgeTypes["Side→Corridor"]

	addedSet := map[int]bool{}
	for _, index := range added {
		addedSet[index] = true
	}
	requiredPieces := map[int]bool{}
	regions := map[string]bool{"Corridor": true}
	for _, index := range added {
		if index <= 0 || index >= len(board.Pieces) {
			proof.RejectionReason = "BadAddedIndex"
			return proof
		}
		region := initialPieceRegion(board.Pieces[index], board)
		evidence := CausalInterventionEvidence{PieceIndex: index, Region: region}
		restricted := board.Copy()
		restricted.ImmobilePieces = make([]bool, len(restricted.Pieces))
		restricted.ImmobilePieces[index] = true
		frozen := restricted.SolveWithBudget(budget)
		if frozen.TimedOut || frozen.BudgetExceeded {
			proof.RejectionReason = "InterventionUnknown"
			return proof
		}
		evidence.FreezeUnsolvable = !frozen.Solvable
		if frozen.Solvable {
			evidence.FreezeOptimal = frozen.NumMoves
			evidence.OptimalIncrease = frozen.NumMoves - sol.NumMoves
		}
		if !evidence.FreezeUnsolvable && evidence.OptimalIncrease <= 0 {
			proof.Interventions = append(proof.Interventions, evidence)
			proof.RejectionReason = "DecorativePrerequisite"
			return proof
		}
		requiredPieces[index] = true
		if region == "Lower" {
			proof.RequiredLowerPieceCount++
		} else if region == "Side" {
			proof.RequiredSidePieceCount++
		}
		path, pathRegions := causalPathFromPieceToExit(graph, index)
		evidence.CausalPathToExit = path > 0
		if path == 0 {
			proof.Interventions = append(proof.Interventions, evidence)
			proof.RejectionReason = "NoCausalPathToExit"
			return proof
		}
		if path > proof.CrossRegionDependencyDepth {
			proof.CrossRegionDependencyDepth = path
		}
		for r := range pathRegions {
			regions[r] = true
		}
		proof.Interventions = append(proof.Interventions, evidence)
	}
	if len(requiredPieces) == 0 || proof.CrossRegionDependencyEdgeCount == 0 {
		proof.RejectionReason = "NoCrossRegionDependency"
		return proof
	}
	proof.CausalRegionCount = len(regions)
	proof.MultiRegionChain = proof.CausalRegionCount >= 3 && proof.CrossRegionDependencyDepth >= 2
	proof.DistributedCausalityScore = round3(
		float64(proof.CrossRegionDependencyEdgeCount*2 +
			proof.CrossRegionDependencyDepth*2 +
			proof.RequiredLowerPieceCount*4 +
			proof.RequiredSidePieceCount*3 +
			proof.CausalRegionCount))
	proof.Valid = true
	return proof
}

func initialPieceRegion(p Piece, board *Board) string {
	target := board.Pieces[0]
	return normalizeCausalRegion(classifyPieceRelativeToTarget(
		p, board.Width, target.Row(board.Width), target.Row(board.Width)+target.Size-1))
}

func regionEdgeCrosses(edge string) bool {
	switch edge {
	case "Upper→Upper", "Lower→Lower", "Side→Side", "Corridor→Corridor":
		return false
	default:
		return true
	}
}

func causalPathFromPieceToExit(graph causalProofGraph, piece int) (int, map[string]bool) {
	best := 0
	bestRegions := map[string]bool{}
	for event, eventPiece := range graph.eventPiece {
		if eventPiece != piece {
			continue
		}
		type nodeDepth struct{ node, depth int }
		queue := []nodeDepth{{event, 0}}
		seen := map[int]bool{}
		regions := map[string]bool{graph.eventRegion[event]: true}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if seen[current.node] {
				continue
			}
			seen[current.node] = true
			regions[graph.eventRegion[current.node]] = true
			if current.depth > 0 && graph.eventRegion[current.node] == "Corridor" {
				if current.depth > best {
					best = current.depth
					bestRegions = copyBoolMap(regions)
				}
			}
			for _, next := range graph.adj[current.node] {
				queue = append(queue, nodeDepth{next, current.depth + 1})
			}
		}
	}
	return best, bestRegions
}

func copyBoolMap(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

// IsCausalExpanded is the selector-facing hard predicate.
func IsCausalExpanded(candidate BoardMixAccepted) bool {
	return candidate.CausalProof != nil &&
		candidate.CausalProof.Valid &&
		candidate.CausalProof.ReplayVerified &&
		candidate.CausalProof.CrossRegionDependencyEdgeCount > 0
}
