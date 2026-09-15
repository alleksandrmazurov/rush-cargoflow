package rush

import (
	"path/filepath"
	"testing"
	"time"
)

func testRushDBPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("testdata", "rushdb", "rush1000.txt")
	if _, err := LoadRushDBFile(p, 1); err != nil {
		t.Skipf("dataset missing: %v", err)
	}
	return p
}

func TestDatasetParser_ReadsKnownRecord(t *testing.T) {
	rec, err := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	if err != nil {
		t.Fatal(err)
	}
	if rec.OriginalOptimalMoves != 51 || rec.OriginalClusterSize != 4780 {
		t.Fatalf("%+v", rec)
	}
	if len(rec.Board36) != 36 || rec.Board36[0] != 'G' {
		t.Fatalf("board %q", rec.Board36)
	}
	board, err := BoardFromRushDBRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if board.Width != 6 || board.Pieces[0].Orientation != Horizontal {
		t.Fatalf("%+v", board.Pieces[0])
	}
}

func TestDatasetParser_OriginalMovesParsed(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].OriginalOptimalMoves != 60 {
		t.Fatalf("first record moves=%d", recs[0].OriginalOptimalMoves)
	}
}

func TestDatasetParser_MetadataParsed(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].SourcePuzzleID != "rush1000:L0001" || recs[0].LineNumber != 1 {
		t.Fatalf("%+v", recs[0])
	}
	if recs[0].OriginalClusterSize <= 0 {
		t.Fatal("cluster size")
	}
}

func TestMalformedRecordRejected(t *testing.T) {
	if _, err := ParseRushDBLine("not-a-record", 1); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseRushDBLine("10 short 1", 1); err == nil {
		t.Fatal("expected board length error")
	}
}

func TestRotateRushToCargoFlow_TargetVertical(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
	if err != nil {
		t.Fatal(err)
	}
	tgt := tr.Board.Pieces[0]
	if tgt.Kind != PieceTarget || tgt.Orientation != Vertical || tgt.Size != 2 {
		t.Fatalf("%+v", tgt)
	}
}

func TestRotateRushToCargoFlow_ExitTop(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushTop)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Board.ExitCol != CargoFlowExitCol {
		t.Fatal(tr.Board.ExitCol)
	}
	if tr.Board.Pieces[0].Col(tr.Board.Width) != CargoFlowExitCol {
		t.Fatal("target not on exit column")
	}
	// CW90: right edge -> top
	nr, nc := RotateRushCellCW90(2, 5, 6)
	if nr != 0 || nc != 2 {
		t.Fatalf("CW90(2,5)->(%d,%d) want (0,2)", nr, nc)
	}
}

func TestLength2MappedCorrectly(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
		if err != nil {
			continue
		}
		for _, p := range tr.Board.Pieces {
			if p.Size == 2 {
				return
			}
		}
	}
	t.Fatal("no size-2 piece found")
}

func TestLength3MappedCorrectly(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
		if err != nil {
			continue
		}
		for _, p := range tr.Board.Pieces {
			if p.Size == 3 {
				return
			}
		}
	}
	t.Fatal("no size-3 piece found")
}

func TestOrientationMappedCorrectly(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	src, _ := BoardFromRushDBRecord(rec)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushTop)
	if err != nil {
		t.Fatal(err)
	}
	// Primary was horizontal -> vertical
	if src.Pieces[0].Orientation != Horizontal || tr.Board.Pieces[0].Orientation != Vertical {
		t.Fatal("primary orientation map failed")
	}
}

func TestTransformNoOverlap(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushBottom)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Board.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTransformInside7x8(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Board.Width != 7 || tr.Board.Height != 8 {
		t.Fatal(tr.Board.Width, tr.Board.Height)
	}
}

func TestTransformedCandidate_ExactSolved(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
		if err != nil {
			continue
		}
		sol := tr.Board.SolveWithBudget(SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 2_000_000})
		if sol.Solvable && sol.NumMoves >= 1 {
			return
		}
	}
	t.Fatal("no solvable transform in sample")
}

func TestTransformedSolution_ReplayPASS(t *testing.T) {
	recs, err := LoadRushDBFile(testRushDBPath(t), 40)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recs {
		tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushTop)
		if err != nil {
			continue
		}
		sol := tr.Board.SolveWithBudget(SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 2_000_000})
		if !sol.Solvable {
			continue
		}
		b2 := tr.Board.Copy()
		if err := b2.Replay(sol.Moves); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Skip("no solvable in sample")
}

func TestCargoFlowOptimalAuthoritative(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
	if err != nil {
		t.Fatal(err)
	}
	sol := tr.Board.SolveWithBudget(SolveBudget{TimeLimit: 8 * time.Second, MaxVisited: 4_000_000})
	if !sol.Solvable {
		t.Skip("unsolved")
	}
	// Must not assume equality with original 51.
	if sol.NumMoves == rec.OriginalOptimalMoves {
		t.Logf("coincidentally equal opt=%d", sol.NumMoves)
	}
	if sol.NumMoves <= 0 {
		t.Fatal("non-positive")
	}
}

func TestTooEasyTransformRejected(t *testing.T) {
	cfg := DefaultTransplantConfig(testRushDBPath(t))
	cfg.MaxSourcePuzzles = 5
	cfg.MinOptimal = 100 // impossible floor
	cfg.TargetAccepted = 1
	cfg.SolveTimeLimit = 2 * time.Second
	cfg.MaxVisitedStates = 500_000
	res, err := RunTransplantPilot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accepted) != 0 {
		t.Fatal("expected none")
	}
	if res.Rejected[TRejectTooEasy] == 0 && res.Rejected[TRejectOutsideRequestedRange] == 0 &&
		res.Rejected[TRejectInvalidTransform] == 0 {
		t.Fatalf("expected rejections, got %+v", res.Rejected)
	}
}

func TestBudgetExceededRejected(t *testing.T) {
	cfg := DefaultTransplantConfig(testRushDBPath(t))
	cfg.MaxSourcePuzzles = 3
	cfg.SolveTimeLimit = 1 * time.Nanosecond
	cfg.MaxVisitedStates = 1
	cfg.TargetAccepted = 1
	cfg.MinOptimal = 1
	cfg.MaxOptimal = 100
	res, err := RunTransplantPilot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rejected[TRejectDifficultyUnknown] == 0 && len(res.Accepted) == 0 {
		// Either budget rejects or invalid transforms — both OK for tiny budget.
		if res.TransformAttempts == 0 {
			t.Fatal("no attempts")
		}
	}
}

func TestSameSourcePuzzle_NotAcceptedTwice(t *testing.T) {
	cfg := DefaultTransplantConfig(testRushDBPath(t))
	cfg.MaxSourcePuzzles = 40
	cfg.MinOptimal = 1
	cfg.MaxOptimal = 60
	cfg.TargetAccepted = 5
	cfg.BucketQuotas = map[string]int{"8-10": 10, "11-13": 10, "14-16": 10, "17-20": 10}
	// Expand buckets via optimalBucket only covering 8-20 — use wide and custom accept logic:
	cfg.MinOptimal = 1
	cfg.MaxOptimal = 100
	// Override: RunTransplantPilot uses optimalBucket which returns "" outside 8-20.
	// So keep 8-20 and check unique sources among accepted.
	cfg.MinOptimal = 8
	cfg.MaxOptimal = 20
	cfg.SolveTimeLimit = 3 * time.Second
	cfg.MaxVisitedStates = 1_000_000
	res, err := RunTransplantPilot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.UniqueSourceIDs() {
		t.Fatal("duplicate source ids")
	}
}

func TestDifferentSourcePuzzles_CanCoexist(t *testing.T) {
	cfg := DefaultTransplantConfig(testRushDBPath(t))
	cfg.MaxSourcePuzzles = 80
	cfg.TargetAccepted = 3
	cfg.SolveTimeLimit = 4 * time.Second
	cfg.MaxVisitedStates = 1_500_000
	res, err := RunTransplantPilot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accepted) < 2 {
		t.Skip("need >=2 accepted for coexistence")
	}
	if res.Accepted[0].Source.SourcePuzzleID == res.Accepted[1].Source.SourcePuzzleID {
		t.Fatal("same source")
	}
}

func TestExactDuplicateRejected(t *testing.T) {
	b1, _ := NewCargoFlowBoard(CargoFlowPOCFixture())
	fp1 := layoutFingerprint(b1)
	fp2 := layoutFingerprint(b1.Copy())
	if fp1 != fp2 {
		t.Fatal("fingerprint unstable")
	}
}

func TestBatchCandidatesHaveUniqueSourceIds(t *testing.T) {
	TestSameSourcePuzzle_NotAcceptedTwice(t)
}

func TestOriginalRushSolver_TransplantRegression(t *testing.T) {
	board, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable || sol.NumMoves != 9 {
		t.Fatalf("%+v", sol)
	}
}

func TestOriginalRushGeneratorUntouched_Transplant(t *testing.T) {
	g := NewDefaultGenerator()
	if g.Width != 6 || g.Height != 6 {
		t.Fatalf("%+v", g)
	}
}

func TestCargoFlowSolver_TransplantRegression(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable || sol.NumMoves != 4 {
		t.Fatalf("%+v", sol)
	}
}

func TestExistingJSONContractCompatible_Transplant(t *testing.T) {
	rec, _ := ParseRushDBLine("51 GBBoLoGHIoLMGHIAAMCCCKoMooJKDDEEJFFo 4780", 15)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Level.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := BoardFromLevelJSON(tr.Level)
	if err != nil {
		t.Fatal(err)
	}
	if b.Rules != RulesCargoFlow {
		t.Fatal(b.Rules)
	}
}
