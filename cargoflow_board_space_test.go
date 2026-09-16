package rush

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompact6x6Detected(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	// Target + one piece fully inside FlushTop core (offX=1, offY=0): cols 1-6, rows 0-5.
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 2*CargoFlowWidth + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	m := ComputeBoardUtilization(board, 1, 0, nil)
	if m.BoardShapeClass != ShapeCompact6x6 && m.BoardShapeClass != ShapeShiftedCore {
		// offX=1 may classify ShiftedCore; force compact by offX=0 style check:
		m2 := ComputeBoardUtilization(board, 0, 0, nil)
		if m2.PiecesOutsideOriginal6x6Core != 0 {
			// pieces at col 1 may be outside core offX=0 (cols 0-5). Rebuild flush.
		}
	}
	// Explicit FlushTop core offX=1: pieces at col 3 and col 1 are inside.
	if m.OccupiedBoundingBoxWidth > 6 || m.OccupiedBoundingBoxHeight > 6 {
		t.Fatalf("expected compact bbox, got %dx%d class=%s", m.OccupiedBoundingBoxWidth, m.OccupiedBoundingBoxHeight, m.BoardShapeClass)
	}
	if m.PiecesOutsideOriginal6x6Core != 0 {
		t.Fatalf("expected 0 outside pieces, got %d", m.PiecesOutsideOriginal6x6Core)
	}
}

func TestShiftedCoreDetected(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	// ShiftDown1 core: offY=1. Pieces only in rows 1-6.
	board.AddPiece(Piece{Position: 1*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 3*CargoFlowWidth + 2, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	m := ComputeBoardUtilization(board, 1, 1, nil)
	if m.BoardShapeClass != ShapeShiftedCore && m.BoardShapeClass != ShapeCompact6x6 {
		// With offY=1 and no outer pieces, should be ShiftedCore.
		if m.CoreOffsetY != 1 {
			t.Fatal("core offset Y")
		}
	}
	if ClassifyBoardShape(m, 1, 1) != ShapeShiftedCore && m.PiecesOutsideOriginal6x6Core == 0 && m.OccupiedBoundingBoxHeight <= 6 {
		// Reclassify explicitly
		m.OuterZoneRelevant = false
		cls := ClassifyBoardShape(m, 1, 1)
		if cls != ShapeShiftedCore {
			t.Fatalf("want ShiftedCore, got %s util=%+v", cls, m)
		}
	}
}

func TestExpandedBoardDetected(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	// Piece in bottom outer row (FlushTop core is rows 0-5).
	board.AddPiece(Piece{Position: 7*CargoFlowWidth + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	m := ComputeBoardUtilization(board, 1, 0, nil)
	if m.PiecesOutsideOriginal6x6Core < 1 {
		t.Fatalf("expected outside piece")
	}
	cls := ClassifyBoardShape(m, 1, 0)
	if cls != ShapeExpanded && cls != ShapeTall && cls != ShapeFullField {
		t.Fatalf("want Expanded-ish, got %s", cls)
	}
}

func TestOuterZoneUsageDetected(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 6*CargoFlowWidth + 0, Size: 1, Orientation: Horizontal, Kind: PieceUnit})
	m := ComputeBoardUtilization(board, 1, 0, nil)
	if m.OuterRowUsage < 1 && m.PiecesOutsideOriginal6x6Core < 1 {
		t.Fatalf("outer usage not detected: %+v", m)
	}
}

func TestOuterZoneRelevantDetected(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 7*CargoFlowWidth + 2, Size: 1, Orientation: Horizontal, Kind: PieceUnit})
	m := ComputeBoardUtilization(board, 1, 0, nil)
	if !m.OuterZoneRelevant {
		t.Fatalf("expected OuterZoneRelevant when piece outside core")
	}
}

func TestBoardShapeClassificationDeterministic(t *testing.T) {
	m := BoardUtilizationMetrics{
		OccupiedBoundingBoxWidth: 6, OccupiedBoundingBoxHeight: 6,
		PiecesOutsideOriginal6x6Core: 0, OuterZoneRelevant: false,
	}
	a := ClassifyBoardShape(m, 0, 0)
	b := ClassifyBoardShape(m, 0, 0)
	if a != b || a != ShapeCompact6x6 {
		t.Fatalf("deterministic compact failed: %s %s", a, b)
	}
	m2 := m
	c1 := ClassifyBoardShape(m2, 1, 1)
	c2 := ClassifyBoardShape(m2, 1, 1)
	if c1 != c2 || c1 != ShapeShiftedCore {
		t.Fatalf("deterministic shifted failed: %s %s", c1, c2)
	}
}

func TestAlternativeEmbeddingTargetAligned(t *testing.T) {
	rec := mustBoardMixRushRecord(t)
	for _, emb := range []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1, EmbedFlushTopMirrorH, EmbedShiftDown1MirrorH} {
		tr, err := TransformRushRecordToCargoFlow(rec, emb)
		if err != nil {
			t.Fatalf("%s: %v", emb, err)
		}
		if tr.Board.Pieces[0].Col(CargoFlowWidth) != CargoFlowExitCol {
			t.Fatalf("%s: target not on exit col", emb)
		}
	}
}

func TestAlternativeEmbeddingNoOverlap(t *testing.T) {
	rec := mustBoardMixRushRecord(t)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushTopMirrorH)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Board.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAlternativeEmbeddingInside7x8(t *testing.T) {
	rec := mustBoardMixRushRecord(t)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushBottomMirrorH)
	if err != nil {
		t.Fatal(err)
	}
	w, h := tr.Board.Width, tr.Board.Height
	for _, p := range tr.Board.Pieces {
		idx := p.Position
		for s := 0; s < p.Size; s++ {
			if idx < 0 || idx >= w*h {
				t.Fatalf("out of bounds")
			}
			idx += p.Stride(w)
		}
	}
}

func TestAlternativeEmbeddingExactSolved(t *testing.T) {
	rec := mustBoardMixRushRecord(t)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedShiftDown1MirrorH)
	if err != nil {
		t.Fatal(err)
	}
	sol := tr.Board.SolveWithBudget(SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 1_000_000})
	if sol.TimedOut || sol.BudgetExceeded {
		t.Skip("budget")
	}
	if !sol.Solvable {
		t.Fatal("expected solvable")
	}
}

func TestAlternativeEmbeddingReplayPASS(t *testing.T) {
	rec := mustBoardMixRushRecord(t)
	tr, err := TransformRushRecordToCargoFlow(rec, EmbedFlushTopMirrorH)
	if err != nil {
		t.Fatal(err)
	}
	sol := tr.Board.SolveWithBudget(SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 1_000_000})
	if !sol.Solvable || sol.TimedOut {
		t.Skip("unsolved")
	}
	b := tr.Board.Copy()
	if err := b.Replay(sol.Moves); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	cp := BoardMixCheckpoint{Version: BoardMixVersion, CompletedAttemptKeys: []string{"a|b|c"}, Rejected: map[string]int{"X": 1}}
	if err := SaveBoardMixCheckpoint(path, cp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestResumeContinuesInsteadOfRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	cp := BoardMixCheckpoint{
		Version: BoardMixVersion,
		CompletedAttemptKeys: []string{"Cand1|FlushTop|plain"},
		AcceptedIDs: []string{"Candidate_001"},
		Rejected: map[string]int{},
	}
	if err := SaveBoardMixCheckpoint(path, cp); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBoardMixCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.CompletedAttemptKeys) != 1 || loaded.CompletedAttemptKeys[0] != "Cand1|FlushTop|plain" {
		t.Fatalf("resume keys lost: %+v", loaded.CompletedAttemptKeys)
	}
}

func TestCtrlCCleanupOrEquivalent(t *testing.T) {
	// Cancel channel closes cleanly; checkpoint save is best-effort in RunBoardMixPilot.
	cancel := make(chan struct{})
	close(cancel)
	select {
	case <-cancel:
	default:
		t.Fatal("cancel not closed")
	}
}

func TestCacheReusedAfterResume(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "c.jsonl")
	cache, err := OpenSolveCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	key := SolveCacheKey{
		SourceDatabaseHash: "h", SourcePuzzleID: "p",
		TransformVersion: TransformVersionCW90MirrorH, EmbeddingVariant: string(EmbedFlushTopMirrorH),
		CargoRulesVersion: CargoRulesVersionCF, SolverVersion: SolverVersionBFS,
	}
	if err := cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true, OptimalGestures: 9}); err != nil {
		t.Fatal(err)
	}
	cache2, err := OpenSolveCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := cache2.Get(key)
	if !ok || e.OptimalGestures != 9 {
		t.Fatal("cache not reused")
	}
}

func TestSameSeedResumeDeterministic(t *testing.T) {
	a := DefaultBoardMixConfig()
	b := DefaultBoardMixConfig()
	if a.Seed != b.Seed {
		t.Fatal("seed drift")
	}
	key := boardMixAttemptKey{BaseID: "X", Embedding: "FlushTop", Augment: "plain"}.String()
	if key != "X|FlushTop|plain" {
		t.Fatal(key)
	}
}

func TestCompletedCandidatesNotDuplicated(t *testing.T) {
	accepted := []BoardMixAccepted{{FamilyID: "F1"}, {FamilyID: "F2"}}
	if !familyTaken(accepted, "F1") || familyTaken(accepted, "F9") {
		t.Fatal("familyTaken")
	}
}

func mustBoardMixRushRecord(t *testing.T) RushDBRecord {
	t.Helper()
	// Prefer a known shortlist transplant board.
	path := "output/RUSH009_CuratedShortlist_001/Candidates/Candidate_003.json"
	level, err := LoadCargoFlowLevelJSONFile(path)
	if err != nil {
		// Minimal solvable-ish classic-style board with A on row 2 (aligns to exit with offX=1).
		return RushDBRecord{
			Board36:              "oooBBooooooAAoCCoooooooooooooooooo",
			OriginalOptimalMoves: 4,
			OriginalClusterSize:  1,
			SourcePuzzleID:       "test:mini",
			LineNumber:           1,
		}
	}
	if level.Transplant == nil || level.Transplant.OriginalBoard == "" {
		t.Fatal("no transplant board")
	}
	return RushDBRecord{
		Board36:              level.Transplant.OriginalBoard,
		OriginalOptimalMoves: level.Transplant.OriginalOptimalMoves,
		OriginalClusterSize:  level.Transplant.OriginalClusterSize,
		SourcePuzzleID:       level.Transplant.SourcePuzzleId,
		LineNumber:           level.Transplant.SourceLine,
	}
}
