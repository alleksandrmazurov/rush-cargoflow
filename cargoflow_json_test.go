package rush

import (
	"strings"
	"testing"
)

func sampleLevelJSON() *LevelJSON {
	return &LevelJSON{
		SchemaVersion:   CargoFlowJSONSchemaVersion,
		LevelID:         "test_level",
		Width:           7,
		Height:          8,
		CoordinateSpace: CoordinateSpaceUnity,
		Exit:            ExitJSON{Side: "top", Column: 3},
		Pieces: []PieceJSON{
			{ID: "target", Type: "target", X: 3, Y: 0, Width: 1, Height: 2, Movable: true},
			{ID: "a", Type: "movable1x1", X: 3, Y: 2, Width: 1, Height: 1, Movable: true},
			{ID: "H", Type: "movable1x2", X: 4, Y: 2, Width: 2, Height: 1, Movable: true},
			{ID: "V", Type: "movable1x3", X: 0, Y: 0, Width: 1, Height: 3, Movable: true},
			{ID: "S1", Type: "static1x1", X: 6, Y: 7, Width: 1, Height: 1, Movable: false},
		},
	}
}

func TestValidJson_Imports(t *testing.T) {
	level := sampleLevelJSON()
	if err := level.Validate(); err != nil {
		t.Fatal(err)
	}
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	if board.Rules != RulesCargoFlow || board.ExitCol != 3 {
		t.Fatalf("board meta: rules=%v exit=%d", board.Rules, board.ExitCol)
	}
}

func TestWrongSchema_Rejects(t *testing.T) {
	level := sampleLevelJSON()
	level.SchemaVersion = 99
	if err := level.Validate(); err == nil {
		t.Fatal("expected schema rejection")
	}
}

func TestOverlap_Rejects(t *testing.T) {
	level := sampleLevelJSON()
	level.Pieces = append(level.Pieces, PieceJSON{ID: "dup", Type: "movable1x1", X: 3, Y: 2, Width: 1, Height: 1, Movable: true})
	if err := level.Validate(); err == nil {
		t.Fatal("expected overlap rejection")
	}
}

func TestOutOfBoard_Rejects(t *testing.T) {
	level := sampleLevelJSON()
	level.Pieces[1].X = 7
	if err := level.Validate(); err == nil {
		t.Fatal("expected OOB rejection")
	}
}

func TestMissingTarget_Rejects(t *testing.T) {
	level := sampleLevelJSON()
	level.Pieces = level.Pieces[1:]
	if err := level.Validate(); err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected missing target, got %v", err)
	}
}

func TestMultipleTargets_Rejects(t *testing.T) {
	level := sampleLevelJSON()
	level.Pieces = append(level.Pieces, PieceJSON{ID: "t2", Type: "target", X: 0, Y: 5, Width: 1, Height: 2, Movable: true})
	if err := level.Validate(); err == nil {
		t.Fatal("expected multiple targets rejection")
	}
}

func TestStaticBlock_Imports(t *testing.T) {
	level := sampleLevelJSON()
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Walls) != 1 {
		t.Fatalf("walls=%d", len(board.Walls))
	}
}

func TestMovable1x1_Imports(t *testing.T) {
	board, err := BoardFromLevelJSON(sampleLevelJSON())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range board.Pieces {
		if p.Kind == PieceUnit {
			found = true
		}
	}
	if !found {
		t.Fatal("missing unit")
	}
}

func TestLongPieces_Import(t *testing.T) {
	board, err := BoardFromLevelJSON(sampleLevelJSON())
	if err != nil {
		t.Fatal(err)
	}
	var hasH, hasV bool
	for _, p := range board.Pieces {
		if p.Kind != PieceNormal {
			continue
		}
		if p.Orientation == Horizontal && p.Size == 2 {
			hasH = true
		}
		if p.Orientation == Vertical && p.Size == 3 {
			hasV = true
		}
	}
	if !hasH || !hasV {
		t.Fatalf("long pieces H=%v V=%v", hasH, hasV)
	}
}

func TestUnityExport_KnownLevel_ExpectedCoordinates(t *testing.T) {
	// Level 01 target at Unity (3,2) 1x2 → Rush origin row = 8-2-2=4, col=3
	rx, ry := UnityToRushOrigin(3, 2, 1, 2, 8)
	if rx != 3 || ry != 4 {
		t.Fatalf("mapping got (%d,%d)", rx, ry)
	}
	ux, uy := RushToUnityOrigin(rx, ry, 1, 2, 8)
	if ux != 3 || uy != 2 {
		t.Fatalf("roundtrip unity got (%d,%d)", ux, uy)
	}
}

func TestRushImport_PreservesPieceCount(t *testing.T) {
	level := sampleLevelJSON()
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	movables := 0
	statics := 0
	for _, p := range level.Pieces {
		if p.Type == "static1x1" {
			statics++
		} else {
			movables++
		}
	}
	if len(board.Pieces) != movables {
		t.Fatalf("pieces %d want %d", len(board.Pieces), movables)
	}
	if len(board.Walls) != statics {
		t.Fatalf("walls %d want %d", len(board.Walls), statics)
	}
}

func TestRushImport_PreservesFootprints(t *testing.T) {
	level := sampleLevelJSON()
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	// Target Unity (3,0) 1x2 → Rush origin row = 8-0-2=6, col=3
	tpos := board.Pieces[0].Position
	if tpos != 6*7+3 {
		t.Fatalf("target pos %d", tpos)
	}
}

func TestRushImport_PreservesTarget(t *testing.T) {
	board, err := BoardFromLevelJSON(sampleLevelJSON())
	if err != nil {
		t.Fatal(err)
	}
	if board.Pieces[0].Kind != PieceTarget {
		t.Fatal("piece0 not target")
	}
}

func TestRushImport_PreservesStatics(t *testing.T) {
	level := sampleLevelJSON()
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	// Unity (6,7) → Rush row = 8-7-1=0, col=6 → index 6
	want := 0*7 + 6
	if board.Walls[0] != want {
		t.Fatalf("wall=%d want %d", board.Walls[0], want)
	}
}

func TestSolutionExport_ReplayPASS(t *testing.T) {
	level := sampleLevelJSON()
	// Clear path after moving the 1x1: make solvable in few moves.
	level.Pieces = []PieceJSON{
		{ID: "target", Type: "target", X: 3, Y: 5, Width: 1, Height: 2, Movable: true},
		{ID: "a", Type: "movable1x1", X: 3, Y: 7, Width: 1, Height: 1, Movable: true},
	}
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.SolveWithBudget(DefaultCargoFlowSolveBudget())
	if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
		t.Fatalf("solve failed: %+v", sol)
	}
	b2, _ := BoardFromLevelJSON(level)
	if err := b2.Replay(sol.Moves); err != nil {
		t.Fatal(err)
	}
	doc := ExportSolutionJSON(level, board, sol, 0, true)
	if !doc.Optimal || !doc.ReplayPass || len(doc.Moves) == 0 {
		t.Fatalf("bad export: %+v", doc)
	}
}

func TestProductionLevel01_RoundtripSolve(t *testing.T) {
	level, err := LoadCargoFlowLevelJSONFile("testdata/cargoflow/levels/level_01.json")
	if err != nil {
		t.Skip(err)
	}
	fp := level.FingerprintUnity()
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Pieces) != 9 {
		t.Fatalf("piece count %d", len(board.Pieces))
	}
	sol := board.SolveWithBudget(DefaultCargoFlowSolveBudget())
	if !sol.Solvable || sol.NumMoves != 4 {
		t.Fatalf("expected optimal 4, got %+v", sol)
	}
	b2, _ := BoardFromLevelJSON(level)
	if err := b2.Replay(sol.Moves); err != nil {
		t.Fatal(err)
	}
	if fp == "" || !strings.Contains(fp, "exit=3") {
		t.Fatal("fingerprint")
	}
}
