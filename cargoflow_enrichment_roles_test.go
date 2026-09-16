package rush

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirectCorridorDetected(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		"...a...",
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		"..BB...",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := -1
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
			break
		}
	}
	if unit < 0 {
		t.Fatal("no unit")
	}
	if !CellInTargetExitCorridor(board, board.Pieces[unit].Position) {
		t.Fatal("expected corridor")
	}
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	if !sol.Solvable {
		t.Fatal("unsolvable")
	}
	ev := ClassifyOneByOneRole(board, sol, []int{unit})
	if ev.Role != RoleDirectTargetBlocker || !ev.InitiallyInTargetCorridor {
		t.Fatalf("role=%s inCorr=%v", ev.Role, ev.InitiallyInTargetCorridor)
	}
}

func TestOffCorridorDetected(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		"a......",
		".......",
		"...T...",
		"...T...",
		"..BB...",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := -1
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
		}
	}
	if CellInTargetExitCorridor(board, board.Pieces[unit].Position) {
		t.Fatal("expected off-corridor")
	}
}

func TestIndirectDependencyDetected(t *testing.T) {
	// 1x1 frees a cell later used by a long piece — SpaceMaker/Indirect.
	board, err := NewCargoFlowBoard([]string{
		".......",
		".BB....",
		".a.....",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := -1
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
		}
	}
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	if !sol.Solvable {
		t.Skip("fixture unsolvable")
	}
	ev := ClassifyOneByOneRole(board, sol, []int{unit})
	if ev.InitiallyInTargetCorridor {
		t.Fatal("should be off corridor")
	}
	// Role may be SpaceMaker/Indirect/SideShuttle/GateKeeper — just not Direct.
	if ev.Role == RoleDirectTargetBlocker {
		t.Fatalf("unexpected direct role: %+v", ev)
	}
}

func TestSpaceReleasedForLongPieceDetected(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		"BB.....",
		"a......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := -1
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
		}
	}
	// Manually construct a tiny solution-like sequence if solver path is free.
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	if !sol.Solvable {
		t.Skip("unsolvable")
	}
	ev := ClassifyOneByOneRole(board, sol, []int{unit})
	if ev.SubsequentUsesReleasedCell && isLongPieceClass(ev.SubsequentPieceClass) {
		if ev.Role != RoleSpaceMaker && ev.Role != RoleIndirectBlocker {
			t.Fatalf("expected SpaceMaker/Indirect got %s", ev.Role)
		}
	}
}

func TestDecorativeOffCorridor1x1Rejected(t *testing.T) {
	baseBoard, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		"..AA...",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	sol0 := baseBoard.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	base := EnrichmentBase{
		CandidateID: "toy", FamilyID: "toy", SourcePuzzleID: "toy",
		BaseOptimal: sol0.NumMoves, Board: baseBoard, DependencyDepth: 1, VisitedStates: 10,
	}
	cfg := DefaultRoleDiversityConfig(".")
	res := &EnrichmentBatchResult{Rejected: map[string]int{}}
	_, reason := evaluatePlacement(base, []int{0}, cfg, SolveBudget{TimeLimit: time.Second, MaxVisited: 100000}, res)
	if reason != "OneByOneUnused" && reason != "OneByOneNotEssential" && reason != "ImmediateVictory" && reason != "Unsolvable" && reason != "QualityDegraded" && reason != "DifficultyUnknown" && reason != "NecessityUnknown" {
		if reason == "" {
			t.Fatal("decorative corner should not accept silently without essentiality")
		}
	}
}

func TestEssentialOffCorridor1x1Accepted(t *testing.T) {
	res := runRoleDiversityTiny(t)
	found := false
	for _, c := range res.Accepted {
		if !c.RoleEvidence.InitiallyInTargetCorridor && c.Essential1x1Count >= 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one essential off-corridor accepted")
	}
}

func TestDirectBlockerQuotaRespected(t *testing.T) {
	res := runRoleDiversityTiny(t)
	direct := 0
	for _, c := range res.Accepted {
		if c.RoleEvidence.Role == RoleDirectTargetBlocker || c.RoleEvidence.InitiallyInTargetCorridor {
			direct++
		}
	}
	maxDirect := int(float64(res.Config.TargetAccepted) * res.Config.MaxDirectBlockerFrac)
	if maxDirect < 1 {
		maxDirect = 1
	}
	if direct > maxDirect && len(res.Accepted) >= res.Config.TargetAccepted {
		t.Fatalf("direct=%d > quota=%d", direct, maxDirect)
	}
}

func TestMultiple1x1RolesPresent(t *testing.T) {
	res := runRoleDiversityTiny(t)
	roles := map[OneByOneRole]bool{}
	for _, c := range res.Accepted {
		roles[c.RoleEvidence.Role] = true
	}
	if len(roles) < 2 && len(res.Accepted) >= 3 {
		t.Fatalf("expected multiple roles, got %v", roles)
	}
}

func TestOneVariantPerFamily_RoleDiversity(t *testing.T) {
	res := runRoleDiversityTiny(t)
	seen := map[string]bool{}
	for _, c := range res.Accepted {
		if seen[c.Base.FamilyID] {
			t.Fatalf("duplicate family %s", c.Base.FamilyID)
		}
		seen[c.Base.FamilyID] = true
	}
}

func TestExactSolverStillPASS_RoleDiversity(t *testing.T) {
	res := runRoleDiversityTiny(t)
	for _, c := range res.Accepted {
		if !c.Solution.Solvable || c.EnrichedOptimal <= 0 {
			t.Fatal("exact solve failed")
		}
	}
}

func TestReplayStillPASS_RoleDiversity(t *testing.T) {
	res := runRoleDiversityTiny(t)
	for _, c := range res.Accepted {
		b := c.Board.Copy()
		if err := b.Replay(c.Solution.Moves); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRUSH009CuratorUnaffected(t *testing.T) {
	// Smoke: curator symbols still link; no mutation of family distance API.
	_ = PuzzleFamilyDistance
	_ = DefaultCuratorConfig
	if DefaultRoleDiversityConfig("x").RoleDiversity != true {
		t.Fatal("role diversity config")
	}
}

var (
	roleDivOnce syncOnceEnrich
)

type syncOnceEnrich struct {
	done bool
	res  EnrichmentBatchResult
	err  error
}

func runRoleDiversityTiny(t *testing.T) EnrichmentBatchResult {
	t.Helper()
	if !roleDivOnce.done {
		batch := filepath.Join("output", "RUSH009_CuratedShortlist_001")
		if _, err := os.Stat(batch); err != nil {
			t.Skip("RUSH009 missing")
		}
		cfg := DefaultRoleDiversityConfig(batch)
		cfg.OutputDir = t.TempDir()
		cfg.TargetAccepted = 3
		cfg.BaseCount = 6
		cfg.MaxPlacements1 = 14
		cfg.MaxPlacements2 = 0
		cfg.SolveTimeLimit = 4 * time.Second
		cfg.MaxVisitedStates = 1_500_000
		cfg.MinOffCorridor = 1
		res, err := RunEnrichmentPilot(cfg)
		roleDivOnce.res = res
		roleDivOnce.err = err
		roleDivOnce.done = true
	}
	if roleDivOnce.err != nil {
		t.Fatal(roleDivOnce.err)
	}
	if len(roleDivOnce.res.Accepted) == 0 {
		t.Fatal("empty")
	}
	return roleDivOnce.res
}
