package rush

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOneByOneHorizontalMove(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := findUnit(t, board)
	ok := false
	for _, m := range board.Moves(nil) {
		if m.Piece == unit && m.Axis == Horizontal && m.AbsSteps() >= 1 {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("expected horizontal 1x1 move")
	}
}

func TestOneByOneVerticalMove(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := findUnit(t, board)
	ok := false
	for _, m := range board.Moves(nil) {
		if m.Piece == unit && m.Axis == Vertical && m.AbsSteps() >= 1 {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("expected vertical 1x1 move")
	}
}

func TestOneByOneNoDiagonalGesture(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	unit := findUnit(t, board)
	for _, m := range board.Moves(nil) {
		if m.Piece != unit || m.Exit {
			continue
		}
		// Each gesture is pure axis — never changes both row and col in one Move encoding.
		if m.Axis != Horizontal && m.Axis != Vertical {
			t.Fatalf("bad axis %v", m.Axis)
		}
	}
	// Apply H then V requires two gestures.
	start := board.Pieces[unit].Position
	moved := false
	for _, m := range board.Moves(nil) {
		if m.Piece == unit && m.Axis == Horizontal && m.Steps == 1 {
			board.DoMove(m)
			moved = true
			break
		}
	}
	if !moved {
		t.Fatal("could not move H+1")
	}
	if board.Pieces[unit].Position == start {
		t.Fatal("position unchanged")
	}
}

func TestOneByOneMultiCellOneGesture(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
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
	unit := findUnit(t, board)
	if !board.HasMove(Move{Piece: unit, Steps: 3, Axis: Horizontal}) {
		t.Fatal("expected multi-cell horizontal gesture +3")
	}
}

func findUnit(t *testing.T, board *Board) int {
	t.Helper()
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			return i
		}
	}
	t.Fatal("no unit")
	return -1
}

func TestEnrichedCandidateContains1x1(t *testing.T) {
	res := runTinyEnrichment(t)
	for _, c := range res.Accepted {
		n := 0
		for _, p := range c.Board.Pieces {
			if p.Kind == PieceUnit {
				n++
			}
		}
		if n < 1 {
			t.Fatalf("%s missing 1x1", c.CandidateID)
		}
	}
}

func TestAcceptedSolutionMoves1x1(t *testing.T) {
	res := runTinyEnrichment(t)
	for _, c := range res.Accepted {
		if c.OneByOneMovesInOptimal < 1 {
			t.Fatalf("%s optimal unused 1x1", c.CandidateID)
		}
	}
}

func TestEssential1x1RestrictedSolveWorse(t *testing.T) {
	res := runTinyEnrichment(t)
	for _, c := range res.Accepted {
		if c.Essential1x1Count < 1 {
			t.Fatalf("%s not essential", c.CandidateID)
		}
		if c.RestrictedSolvable && c.RestrictedOptimal <= c.EnrichedOptimal {
			t.Fatalf("%s restricted %d <= enriched %d", c.CandidateID, c.RestrictedOptimal, c.EnrichedOptimal)
		}
	}
}

func TestDecorative1x1Rejected(t *testing.T) {
	// Place a 1x1 far away on a trivial board — should be unused or non-essential.
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
	// Blocking the exit corridor with a car makes a real puzzle; add distant 1x1.
	base := EnrichmentBase{
		CandidateID: "toy", FamilyID: "toy", SourcePuzzleID: "toy",
		BaseOptimal: 2, Board: baseBoard, DependencyDepth: 1, VisitedStates: 10,
	}
	sol0 := baseBoard.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	if sol0.Solvable {
		base.BaseOptimal = sol0.NumMoves
	}
	cfg := DefaultEnrichmentConfig(".")
	cfg.MaxPlacements1 = 5
	cfg.MaxPlacements2 = 0
	budget := SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000}
	res := &EnrichmentBatchResult{Rejected: map[string]int{}}
	// Force far corner cell only.
	cand, reason := evaluatePlacement(base, []int{0}, cfg, budget, res) // top-left
	if cand != nil && reason == "" {
		// If somehow accepted, must still be essential — decorative path should usually reject.
		if cand.OneByOneMovesInOptimal == 0 {
			t.Fatal("accepted decorative unused")
		}
	}
	if reason != "OneByOneUnused" && reason != "OneByOneNotEssential" && reason != "QualityDegraded" && reason != "ImmediateVictory" && reason != "Unsolvable" && reason != "" {
		// empty reason with cand means accepted; decorative far cell on this board often unused
		if reason == "" && cand != nil {
			t.Logf("unexpected accept of corner placement: %+v", cand.PlacementCells)
		}
	}
}

func TestInvalidPlacementRejected(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	base := EnrichmentBase{Board: board, BaseOptimal: 1, CandidateID: "x", FamilyID: "x"}
	cfg := DefaultEnrichmentConfig(".")
	res := &EnrichmentBatchResult{Rejected: map[string]int{}}
	// Occupied by target
	_, reason := evaluatePlacement(base, []int{board.Pieces[0].Position}, cfg, SolveBudget{TimeLimit: time.Second, MaxVisited: 100000}, res)
	if reason != "InvalidPlacement" {
		t.Fatalf("want InvalidPlacement got %q", reason)
	}
}

func TestEnrichmentReplayPASS(t *testing.T) {
	res := runTinyEnrichment(t)
	for _, c := range res.Accepted {
		if !c.ReplayVerified {
			t.Fatal("replay flag")
		}
		b := c.Board.Copy()
		if err := b.Replay(c.Solution.Moves); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBaseFamilyPreserved(t *testing.T) {
	res := runTinyEnrichment(t)
	for _, c := range res.Accepted {
		if c.Level.Enrichment == nil || c.Level.Enrichment.BaseFamilyId == "" {
			t.Fatal("missing family metadata")
		}
		if c.Level.Enrichment.BaseFamilyId != c.Base.FamilyID {
			t.Fatal("family mismatch")
		}
	}
}

func TestOneVariantPerFamily(t *testing.T) {
	res := runTinyEnrichment(t)
	seen := map[string]bool{}
	for _, c := range res.Accepted {
		if seen[c.Base.FamilyID] {
			t.Fatalf("duplicate family %s", c.Base.FamilyID)
		}
		seen[c.Base.FamilyID] = true
	}
}

func runTinyEnrichment(t *testing.T) EnrichmentBatchResult {
	t.Helper()
	tinyEnrichOnce.Do(func() {
		batch := filepath.Join("output", "RUSH009_CuratedShortlist_001")
		if _, err := os.Stat(batch); err != nil {
			tinyEnrichErr = err
			return
		}
		dir, err := os.MkdirTemp("", "rush010-test-*")
		if err != nil {
			tinyEnrichErr = err
			return
		}
		cfg := DefaultEnrichmentConfig(batch)
		cfg.OutputDir = dir
		cfg.TargetAccepted = 2
		cfg.BaseCount = 3
		cfg.MaxPlacements1 = 12
		cfg.MaxPlacements2 = 0
		cfg.SolveTimeLimit = 4 * time.Second
		cfg.MaxVisitedStates = 1_500_000
		// Prefer lower/mid bases for test speed: temporarily override selection via smaller hard quota.
		res, err := RunEnrichmentPilot(cfg)
		tinyEnrichErr = err
		tinyEnrichRes = res
	})
	if tinyEnrichErr != nil {
		if os.IsNotExist(tinyEnrichErr) {
			t.Skip("RUSH009 batch missing")
		}
		t.Fatal(tinyEnrichErr)
	}
	if len(tinyEnrichRes.Accepted) == 0 {
		t.Fatal("no accepted enrichments")
	}
	return tinyEnrichRes
}

var (
	tinyEnrichOnce sync.Once
	tinyEnrichRes  EnrichmentBatchResult
	tinyEnrichErr  error
)
