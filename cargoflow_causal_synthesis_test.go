package rush

import (
	"fmt"
	"path/filepath"
	"testing"
)

// causalFixture builds:
// Lower unit L → vertical side gate M → upper corridor blocker U → Target exit.
func causalFixture(includeLower bool) *Board {
	b := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	b.Rules = RulesCargoFlow
	b.ExitCol = CargoFlowExitCol
	_ = b.AddPiece(Piece{Position: 3*7 + 3, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	_ = b.AddPiece(Piece{Position: 2*7 + 2, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	_ = b.AddPiece(Piece{Position: 0*7 + 5, Size: 3, Orientation: Vertical, Kind: PieceNormal})
	b.Labels = []string{"target", "U", "M"}
	_ = b.AddWall(2*7 + 1) // prevents U from escaping left
	if includeLower {
		_ = b.AddPiece(Piece{Position: 5*7 + 5, Size: 1, Orientation: Horizontal, Kind: PieceUnit})
		b.Labels = append(b.Labels, "L")
	}
	return b
}

func causalFixtureWithLowerSize(size int) *Board {
	b := causalFixture(false)
	kind := PieceNormal
	if size == 1 {
		kind = PieceUnit
	}
	_ = b.AddPiece(Piece{
		Position: 5*7 + (6 - size), Size: size,
		Orientation: Horizontal, Kind: kind,
	})
	b.Labels = append(b.Labels, fmt.Sprintf("L%d", size))
	return b
}

func solveCausalFixture(t *testing.T, includeLower bool) (*Board, Solution, SolveBudget) {
	t.Helper()
	b := causalFixture(includeLower)
	if err := b.Validate(); err != nil {
		t.Fatal(err)
	}
	budget := DefaultCargoFlowSolveBudget()
	sol := b.SolveWithBudget(budget)
	if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
		t.Fatalf("fixture solve failed: %+v", sol)
	}
	return b, sol, budget
}

func TestCausalTemplateCreatesRequiredDependency(t *testing.T) {
	base, baseSol, budget := solveCausalFixture(t, false)
	cfg := DefaultCausalSynthesisConfig()
	cfg.MaxProposalsPerBoard = 12
	cfg.MaxAcceptedPerBoard = 2
	got, rejected := GenerateCausalSynthesisCandidates(base, baseSol.NumMoves, budget, cfg)
	if len(got) == 0 {
		t.Fatalf("expected causal synthesis; rejected=%v", rejected)
	}
	found := false
	for _, c := range got {
		if c.Proof.Valid && c.Proof.RequiredLowerPieceCount > 0 && c.Proof.CrossRegionDependencyDepth >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no required lower-chain proof: %+v", got)
	}
}

func TestCausalSynthesisKnownLevelFixture(t *testing.T) {
	level, err := LoadCargoFlowLevelJSONFile(
		filepath.Join("testdata", "cargoflow", "levels", "level_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	budget := DefaultCargoFlowSolveBudget()
	baseSol := base.SolveWithBudget(budget)
	cfg := DefaultCausalSynthesisConfig()
	cfg.MaxAcceptedPerBoard = 1
	candidates, rejected := GenerateCausalSynthesisCandidates(
		base, baseSol.NumMoves, budget, cfg)
	if len(candidates) != 1 || !candidates[0].Proof.Valid {
		t.Fatalf("known-level causal synthesis failed: rejected=%v candidates=%+v", rejected, candidates)
	}
	p := candidates[0].Proof
	if len(p.Interventions) == 0 ||
		(!p.Interventions[0].FreezeUnsolvable && p.Interventions[0].OptimalIncrease <= 0) {
		t.Fatalf("known-level prerequisite is not necessary: %+v", p.Interventions)
	}
	t.Logf("level_01 proof: template=%s optimal=%d→%d cross=%d depth=%d freezeUnsolvable=%v edges=%v",
		p.Template, p.BaseOptimal, p.NativeOptimal,
		p.CrossRegionDependencyEdgeCount, p.CrossRegionDependencyDepth,
		p.Interventions[0].FreezeUnsolvable, p.EdgeTypeDistribution)
}

func TestCausalPieceLengthDiversityHasRealRoles(t *testing.T) {
	budget := DefaultCargoFlowSolveBudget()
	templates := map[int]CausalTemplate{
		1: CausalLower1x1SpaceMaker,
		2: CausalLowerParkingUnlock,
		3: CausalCrossRegionLongGate,
	}
	for _, size := range []int{1, 2, 3} {
		board := causalFixtureWithLowerSize(size)
		sol := board.SolveWithBudget(budget)
		if !sol.Solvable {
			t.Fatalf("size %d fixture unsolved", size)
		}
		proof := ProveCausalSynthesis(board, sol, 3, []int{3}, templates[size], budget)
		if !proof.Valid || proof.RequiredLowerPieceCount != 1 {
			t.Fatalf("size %d lacks required causal role: %+v", size, proof)
		}
	}
	vertical := causalFixture(false)
	_ = vertical.AddPiece(Piece{
		Position: 5*7 + 5, Size: 2,
		Orientation: Vertical, Kind: PieceNormal,
	})
	vertical.Labels = append(vertical.Labels, "LV2")
	sol := vertical.SolveWithBudget(budget)
	proof := ProveCausalSynthesis(
		vertical, sol, 3, []int{3}, CausalLowerParkingUnlock, budget)
	if !proof.Valid || proof.RequiredLowerPieceCount != 1 {
		t.Fatalf("vertical 1x2 lacks required causal role: %+v", proof)
	}
}

func TestRemovingLowerPrerequisiteBreaksOrChangesSolution(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	if !proof.Valid {
		t.Fatalf("proof invalid: %+v", proof)
	}
	if len(proof.Interventions) != 1 {
		t.Fatalf("interventions=%+v", proof.Interventions)
	}
	e := proof.Interventions[0]
	if !e.FreezeUnsolvable && e.OptimalIncrease <= 0 {
		t.Fatalf("lower prerequisite not necessary: %+v", e)
	}
}

func TestLowerToUpperEdgeDetected(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	if proof.LowerToUpperDependencyEdges+proof.LowerToCorridorDependencyEdges == 0 &&
		proof.CrossRegionDependencyDepth < 2 {
		t.Fatalf("lower→upper/corridor chain not detected: %+v", proof)
	}
}

func TestSideToUpperEdgeDetected(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	if proof.SideToUpperDependencyEdges+proof.SideToCorridorDependencyEdges == 0 {
		t.Fatalf("side→upper/corridor edge not detected: %+v", proof.EdgeTypeDistribution)
	}
}

func TestCrossRegionChainDetected(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalSideToLowerToUpper, budget)
	if !proof.Valid || !proof.MultiRegionChain || proof.CausalRegionCount < 3 {
		t.Fatalf("multi-region chain not detected: %+v", proof)
	}
	t.Logf("fixture proof: optimal=%d delta=%d crossEdges=%d depth=%d requiredLower=%d requiredSide=%d regions=%d edges=%v",
		proof.NativeOptimal, proof.OptimalDelta, proof.CrossRegionDependencyEdgeCount,
		proof.CrossRegionDependencyDepth, proof.RequiredLowerPieceCount,
		proof.RequiredSidePieceCount, proof.CausalRegionCount, proof.EdgeTypeDistribution)
}

func TestDecorativeOuterPieceRejected(t *testing.T) {
	base, _, budget := solveCausalFixture(t, false)
	if !base.AddPiece(Piece{Position: 7 * 7, Size: 1, Orientation: Horizontal, Kind: PieceUnit}) {
		t.Fatal("add decorative unit")
	}
	base.Labels = append(base.Labels, "decorative")
	sol := base.SolveWithBudget(budget)
	if !sol.Solvable {
		t.Fatal("decorative fixture should solve")
	}
	proof := ProveCausalSynthesis(base, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	if proof.Valid || proof.RejectionReason != "DecorativePrerequisite" {
		t.Fatalf("decorative piece accepted: %+v", proof)
	}
}

func TestReplayVerified(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	if !proof.Valid || !proof.ReplayVerified || !sol.Solvable {
		t.Fatalf("proof=%+v sol=%+v", proof, sol)
	}
}

func TestExactSolvePass(t *testing.T) {
	_, sol, _ := solveCausalFixture(t, true)
	if !sol.Solvable || sol.NumMoves != 4 {
		t.Fatalf("expected exact optimum 4, got %+v", sol)
	}
}

func syntheticCausalCandidate(family string, score float64, template CausalTemplate) BoardMixAccepted {
	proof := &CausalProof{
		Valid: true, ReplayVerified: true, Template: template,
		CrossRegionDependencyEdgeCount: 2, CrossRegionDependencyDepth: 2,
		RequiredLowerPieceCount: 1, DistributedCausalityScore: score,
		EdgeTypeDistribution: map[string]int{"Lower→Side": 1, "Side→Corridor": 1},
	}
	return BoardMixAccepted{
		FamilyID: family, InventoryClass: InvOne1x1,
		BoardUtil:      BoardUtilizationMetrics{BoardShapeClass: ShapeWide, OuterZoneRelevant: true},
		CausalTemplate: template, CausalProof: proof, ReplayVerified: true,
	}
}

func TestCausalSelectorTargetEightAndUniqueFamily(t *testing.T) {
	pool := []BoardMixAccepted{}
	templates := []CausalTemplate{CausalLower1x1SpaceMaker, CausalLowerParkingUnlock, CausalCrossRegionLongGate, CausalSideToLowerToUpper}
	for i := 0; i < 10; i++ {
		pool = append(pool, syntheticCausalCandidate(fmt.Sprintf("C%02d", i), float64(20-i), templates[i%len(templates)]))
	}
	for i := 0; i < 8; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("D%02d", i), InventoryClass: InvTwo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeTall, OuterZoneRelevant: true},
		})
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.TargetCausalExpanded = 8
	cfg.TargetGenuineCoreExpanded = 0
	cfg.UniqueFamily = true
	cfg.MinDistinctBoardShapes = 1
	cfg.MinDistinctInventoryClasses = 1
	cfg.MinOuterZoneRelevant = 1
	cfg.ShapeQuotas = map[string]int{}
	cfg.InventoryQuotas = map[string]int{}
	selected, report := SelectBoardMixShortlist(pool, cfg)
	if len(selected) != 12 || report.FinalCausalExpanded < 8 {
		t.Fatalf("selected=%d causal=%d report=%+v", len(selected), report.FinalCausalExpanded, report)
	}
	if len(report.CausalTemplateDistribution) < 4 || len(report.HumanValidationCandidates) < 8 {
		t.Fatalf("causal reporting incomplete: templates=%v validation=%d",
			report.CausalTemplateDistribution, len(report.HumanValidationCandidates))
	}
	families := map[string]bool{}
	for _, c := range selected {
		if families[c.FamilyID] {
			t.Fatalf("duplicate family %s", c.FamilyID)
		}
		families[c.FamilyID] = true
	}
}

func TestCausalGenerationModeScheduled(t *testing.T) {
	cfg := DefaultBoardMixConfig()
	cfg.TryCausalSynthesis = true
	found := false
	for _, mode := range boardMixModes(cfg) {
		if mode == "causal" {
			found = true
		}
	}
	if !found {
		t.Fatal("causal boardmix mode was not scheduled")
	}
}

func TestUniqueFamilySelection(t *testing.T) {
	pool := []BoardMixAccepted{}
	for i := 0; i < 10; i++ {
		pool = append(pool, syntheticCausalCandidate(
			fmt.Sprintf("UF%02d", i), float64(i), CausalLower1x1SpaceMaker))
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 8
	cfg.TargetCausalExpanded = 8
	cfg.TargetGenuineCoreExpanded = 0
	cfg.UniqueFamily = true
	cfg.MinDistinctBoardShapes = 1
	cfg.MinDistinctInventoryClasses = 1
	cfg.MinOuterZoneRelevant = 1
	cfg.ShapeQuotas = map[string]int{}
	cfg.InventoryQuotas = map[string]int{}
	selected, _ := SelectBoardMixShortlist(pool, cfg)
	seen := map[string]bool{}
	for _, candidate := range selected {
		if seen[candidate.FamilyID] {
			t.Fatalf("duplicate family %s", candidate.FamilyID)
		}
		seen[candidate.FamilyID] = true
	}
}

func TestNoZeroSolutionExport(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	level, err := LevelJSONFromBoard(board, "Candidate_001", "CausalFixture")
	if err != nil {
		t.Fatal(err)
	}
	candidate := BoardMixAccepted{
		CandidateID: "Candidate_001", FamilyID: "fixture",
		OptimalGestures: sol.NumMoves, CausalTemplate: proof.Template,
		CausalProof: &proof, ReplayVerified: true, Level: level, Board: board, Solution: sol,
	}
	finalizeBoardMixCandidate(&candidate, candidate.CandidateID)
	if candidate.SolutionDoc.SchemaVersion == 0 || !candidate.SolutionDoc.Solved {
		t.Fatalf("zero/invalid solution doc: %+v", candidate.SolutionDoc)
	}
}

func TestUnityBatchValidationPass(t *testing.T) {
	board, sol, budget := solveCausalFixture(t, true)
	proof := ProveCausalSynthesis(board, sol, 3, []int{3}, CausalLower1x1SpaceMaker, budget)
	level, err := LevelJSONFromBoard(board, "Candidate_001", "CausalFixture")
	if err != nil {
		t.Fatal(err)
	}
	candidate := BoardMixAccepted{
		CandidateID: "Candidate_001", FamilyID: "fixture",
		InventoryClass:  BuildCargoInventorySignature(board).InventoryClass,
		BoardUtil:       ComputeBoardUtilization(board, 0, 0, &sol),
		OptimalGestures: sol.NumMoves, CausalTemplate: proof.Template,
		CausalProof: &proof, ReplayVerified: true, Level: level, Board: board, Solution: sol,
	}
	finalizeBoardMixCandidate(&candidate, candidate.CandidateID)
	dir := t.TempDir()
	report := BoardMixSelectReport{FinalTarget: 1, FinalAccepted: 1}
	result := BoardMixResult{
		Config:   BoardMixConfig{SolveTimeLimitMs: 5000, MaxVisitedStates: 500_000},
		Accepted: []BoardMixAccepted{candidate}, Pool: []BoardMixAccepted{candidate},
		SelectReport: report,
	}
	if err := WriteBoardMixBatch(dir, result); err != nil {
		t.Fatal(err)
	}
	if candidate.SolutionDoc.SchemaVersion == 0 {
		t.Fatal("zero SolutionDoc")
	}
	validation, err := ValidateUnityBatchReport(filepath.Clean(dir))
	if err != nil || !validation.OK {
		t.Fatalf("validation=%+v err=%v", validation, err)
	}
}
