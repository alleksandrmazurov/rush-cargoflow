package rush

import (
	"reflect"
	"testing"
	"time"
)

func TestEquivalent1x1Permutation_SameCanonicalKey(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		"...a...",
		"...b...",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	classes := buildCargoClasses(board)
	pos := positionsOf(board)
	units := []int{}
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			units = append(units, i)
		}
	}
	if len(units) != 2 {
		t.Fatalf("want 2 units, got %d", len(units))
	}
	k1 := encodeCargoKeyFromPositions(pos, false, classes)
	pos[units[0]], pos[units[1]] = pos[units[1]], pos[units[0]]
	k2 := encodeCargoKeyFromPositions(pos, false, classes)
	if k1 != k2 {
		t.Fatalf("1x1 permutation should share key")
	}
}

func TestEquivalent1x2Permutation_SameCanonicalKey(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		"AA.BB..",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	classes := buildCargoClasses(board)
	pos := positionsOf(board)
	// Two horizontal size-2 normals: swap their positions in the key encoding input.
	idxs := []int{}
	for i, p := range board.Pieces {
		if p.Kind == PieceNormal && p.Orientation == Horizontal && p.Size == 2 {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) != 2 {
		t.Fatalf("expected 2 H1x2, got %d", len(idxs))
	}
	k1 := encodeCargoKeyFromPositions(pos, false, classes)
	pos[idxs[0]], pos[idxs[1]] = pos[idxs[1]], pos[idxs[0]]
	k2 := encodeCargoKeyFromPositions(pos, false, classes)
	if k1 != k2 {
		t.Fatal("1x2 permutation key mismatch")
	}
}

func TestDifferentLogicalState_DifferentKey(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	k1 := CanonicalCargoKey(board)
	unit := 0
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
			break
		}
	}
	board.DoMove(Move{Piece: unit, Steps: 1, Axis: Horizontal})
	k2 := CanonicalCargoKey(board)
	if k1 == k2 {
		t.Fatal("moved unit must change key")
	}
}

func TestTargetDifference_DifferentKey(t *testing.T) {
	a, _ := NewCargoFlowBoard([]string{
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
		".......",
	})
	b, _ := NewCargoFlowBoard([]string{
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	if CanonicalCargoKey(a) == CanonicalCargoKey(b) {
		t.Fatal("different target rows must differ")
	}
}

func TestStaticDefinitionDifference_NotMixed(t *testing.T) {
	a, _ := NewCargoFlowBoard([]string{
		".......",
		"...x...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	b, _ := NewCargoFlowBoard([]string{
		".......",
		".......",
		"...x...",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	// Keys only encode movable positions; statics are part of PuzzleDefinition.
	// Same movable layout => same dynamic key, but boards are different puzzles.
	if CanonicalCargoKey(a) != CanonicalCargoKey(b) {
		// Target and empty movables identical — keys should match.
		t.Fatal("dynamic key should ignore static cell identity (definition-level)")
	}
	if len(a.Walls) == 0 || a.Walls[0] == b.Walls[0] {
		t.Fatal("statics should differ between boards")
	}
}

func TestCompactKey_Deterministic(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	k1 := CanonicalCargoKey(board)
	k2 := CanonicalCargoKey(board)
	if k1 != k2 {
		t.Fatal("nondeterministic key")
	}
}

func TestOptimizedMoveGraph_EqualsBaseline(t *testing.T) {
	// Baseline successor multiset: cargoMoves on a fresh board.
	// Optimized path uses the same Moves() generator; compare sorted move lists
	// before/after a round-trip applyPositions encoding.
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	base := append([]Move(nil), board.Moves(nil)...)
	pos := positionsOf(board)
	work := board.Copy()
	applyPositions(work, pos, false)
	opt := work.Moves(nil)
	if !reflect.DeepEqual(base, opt) {
		t.Fatalf("move graph mismatch\nbase=%v\nopt=%v", base, opt)
	}
}

func TestOptimalityRegression_KnownExact(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"testdata/cargoflow/levels/level_01.json", 4},
		{"testdata/cargoflow/levels/level_10.json", 6},
		{"testdata/cargoflow/levels/level_20.json", 9},
		{"testdata/cargoflow/levels/level_30.json", 21},
	}
	for _, tc := range cases {
		level, err := LoadCargoFlowLevelJSONFile(tc.path)
		if err != nil {
			t.Skip(err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			t.Fatal(err)
		}
		sol := board.SolveWithBudget(SolveBudget{TimeLimit: 60 * time.Second, MaxVisited: 16000000})
		if !sol.Solvable || sol.BudgetExceeded || sol.TimedOut {
			t.Fatalf("%s not exact: %+v", tc.path, sol)
		}
		if sol.NumMoves != tc.want {
			t.Fatalf("%s gestures=%d want %d", tc.path, sol.NumMoves, tc.want)
		}
		b2, _ := BoardFromLevelJSON(level)
		if err := b2.Replay(sol.Moves); err != nil {
			t.Fatalf("%s replay: %v", tc.path, err)
		}
	}
}
