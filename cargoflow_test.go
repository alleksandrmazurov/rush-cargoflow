package rush

import (
	"testing"
)

func TestCargoFlowBoardDimensions(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	if board.Width != 7 || board.Height != 8 {
		t.Fatalf("got %dx%d", board.Width, board.Height)
	}
	if board.Rules != RulesCargoFlow {
		t.Fatalf("rules: %v", board.Rules)
	}
	if board.Pieces[0].Kind != PieceTarget {
		t.Fatal("piece 0 must be Target")
	}
}

func TestMultiCellDrag_CountsAsOneGesture(t *testing.T) {
	// Horizontal 1x2 with three free cells to the right => +1,+2,+3 each cost 1.
	desc := []string{
		".......",
		".......",
		"..AA...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	var aa int
	for i, p := range board.Pieces {
		if p.Kind == PieceNormal && p.Orientation == Horizontal && p.Size == 2 {
			aa = i
			break
		}
	}
	seen := map[int]bool{}
	for _, m := range board.Moves(nil) {
		if m.Piece == aa && !m.Exit {
			seen[m.Steps] = true
		}
	}
	for _, steps := range []int{1, 2, 3} {
		if !seen[steps] {
			t.Fatalf("expected single-gesture move +%d, got %v", steps, seen)
		}
	}
	// Solver must be allowed to take +3 directly (not forced into +1+1+1).
	if !board.HasMove(Move{Piece: aa, Steps: 3, Axis: Horizontal}) {
		t.Fatal("missing +3 multi-cell gesture")
	}
}

func TestUnit_HorizontalAndVerticalAllowed(t *testing.T) {
	desc := []string{
		".......",
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	unit := 0
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
			break
		}
	}
	var hasH, hasV bool
	for _, m := range board.Moves(nil) {
		if m.Piece != unit {
			continue
		}
		if m.Axis == Horizontal {
			hasH = true
		}
		if m.Axis == Vertical {
			hasV = true
		}
	}
	if !hasH || !hasV {
		t.Fatalf("1x1 must allow both axes (H=%v V=%v)", hasH, hasV)
	}
}

func TestUnit_DiagonalCornerNotAllowed(t *testing.T) {
	desc := []string{
		".......",
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	unit := 0
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
			break
		}
	}
	start := board.Pieces[unit].Position
	// No single move may change both row and col.
	for _, m := range board.Moves(nil) {
		if m.Piece != unit || m.Exit {
			continue
		}
		board.DoMove(m)
		end := board.Pieces[unit].Position
		board.UndoMove(m)
		sr, sc := start/board.Width, start%board.Width
		er, ec := end/board.Width, end%board.Width
		if sr != er && sc != ec {
			t.Fatalf("corner/diagonal gesture not allowed: %v", m)
		}
	}
}

func TestTarget_HorizontalMoveNotAllowed(t *testing.T) {
	desc := []string{
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range board.Moves(nil) {
		if m.Piece == 0 && !m.Exit && m.Axis == Horizontal {
			t.Fatalf("target horizontal move: %v", m)
		}
		if m.Piece == 0 && !m.Exit {
			// Axis may be Vertical; also orientation-driven slides set Axis Vertical in cargoMoves.
			if board.Pieces[0].Orientation != Vertical {
				t.Fatal("target must stay vertical")
			}
		}
	}
	// Explicit: no change of column via any non-exit target move.
	col := board.Pieces[0].Col(board.Width)
	for _, m := range board.Moves(nil) {
		if m.Piece != 0 || m.Exit {
			continue
		}
		board.DoMove(m)
		if board.Pieces[0].Col(board.Width) != col {
			t.Fatal("target changed column")
		}
		board.UndoMove(m)
	}
}

func TestTarget_VerticalMoveAllowed(t *testing.T) {
	desc := []string{
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, m := range board.Moves(nil) {
		if m.Piece == 0 && !m.Exit && m.Steps != 0 {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("expected vertical on-board target moves")
	}
}

func TestTarget_WrongColumnCannotExit(t *testing.T) {
	desc := []string{
		".......",
		".......",
		".......",
		"..T....",
		"..T....",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	if board.Pieces[0].Col(board.Width) == CargoFlowExitCol {
		t.Fatal("fixture should place target off exit column")
	}
	for _, m := range board.Moves(nil) {
		if m.Exit {
			t.Fatal("exit must not be legal off exit column")
		}
	}
}

func TestTarget_BlockedPathCannotExit(t *testing.T) {
	desc := []string{
		".......",
		"...a...",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range board.Moves(nil) {
		if m.Exit {
			t.Fatal("exit must not be legal while blocked")
		}
	}
}

func TestTarget_ClearAlignedPathCanExit(t *testing.T) {
	desc := []string{
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	if !board.HasMove(Move{Piece: 0, Exit: true}) {
		t.Fatal("expected Exit gesture")
	}
}

func TestTarget_ExitCountsAsOneGesture(t *testing.T) {
	desc := []string{
		".......",
		".......",
		".......",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable || sol.NumMoves != 1 || !sol.Moves[0].Exit {
		t.Fatalf("expected single Exit gesture, got %+v", sol)
	}
}

func TestStaticBlock_HasNoMoves(t *testing.T) {
	desc := []string{
		".......",
		".......",
		"...x...",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Walls) != 1 {
		t.Fatalf("expected 1 wall, got %d", len(board.Walls))
	}
	for _, m := range board.Moves(nil) {
		// Walls are not pieces; ensure no piece sits on wall cell as movable.
		if !m.Exit {
			board.DoMove(m)
			if board.occupied[board.Walls[0]] == false {
				t.Fatal("static cell cleared")
			}
			board.UndoMove(m)
		}
	}
}

func TestStaticBlock_BlocksMovement(t *testing.T) {
	desc := []string{
		".......",
		".......",
		"..ax...",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	}
	board, err := NewCargoFlowBoard(desc)
	if err != nil {
		t.Fatal(err)
	}
	unit := 0
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			unit = i
			break
		}
	}
	for _, m := range board.Moves(nil) {
		if m.Piece == unit && m.Axis == Horizontal && m.Steps > 0 {
			t.Fatalf("unit must not slide through static: %v", m)
		}
	}
}

func TestCargoFlowPOCFixtureSolvedOptimally(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	before := board.Hash()
	sol := board.Solve()
	if board.Hash() != before {
		t.Fatal("solve mutated board")
	}
	if !sol.Solvable {
		t.Fatal("expected solvable")
	}
	if sol.NumMoves != 4 {
		t.Fatalf("expected optimal 4 gestures, got %d (%v)", sol.NumMoves, sol.Moves)
	}
	if !sol.Moves[len(sol.Moves)-1].Exit {
		t.Fatal("last move must be Target Exit")
	}
}

func TestCargoFlowPOCReplay(t *testing.T) {
	start, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	sol := start.Solve()
	replay, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := replay.Replay(sol.Moves); err != nil {
		t.Fatal(err)
	}
	if !replay.won {
		t.Fatal("expected victory flag")
	}
}
