package rush

import (
	"reflect"
	"testing"
)

// Classic beginner Rush Hour layout (also used by cmd/forty level 1).
var knownPuzzleForty1 = []string{
	"..B.CC",
	"..B...",
	"AAB...",
	"DDD..E",
	".....E",
	".....E",
}

// Minimal clear path: move B out of the way, then drive A to the exit.
var knownPuzzleSimple = []string{
	"..B...",
	"..B...",
	"AAB...",
	"......",
	"......",
	"......",
}

func TestNewBoardCreation(t *testing.T) {
	board, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	if board.Width != 6 || board.Height != 6 {
		t.Fatalf("expected 6x6 board, got %dx%d", board.Width, board.Height)
	}
	if len(board.Pieces) != 5 {
		t.Fatalf("expected 5 pieces, got %d", len(board.Pieces))
	}
	if board.Pieces[0].Orientation != Horizontal || board.Pieces[0].Size != 2 {
		t.Fatalf("primary piece: %+v", board.Pieces[0])
	}
	if err := board.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestNewBoardFromString(t *testing.T) {
	desc := "......" +
		"......" +
		"AA...." +
		"......" +
		"......" +
		"......"
	board, err := NewBoardFromString(desc)
	if err != nil {
		t.Fatalf("NewBoardFromString: %v", err)
	}
	if board.Width != 6 || board.Height != 6 {
		t.Fatalf("expected 6x6 board, got %dx%d", board.Width, board.Height)
	}
	if len(board.Pieces) != 1 {
		t.Fatalf("expected 1 piece, got %d", len(board.Pieces))
	}
}

func TestLegalMoveGeneration(t *testing.T) {
	board, err := NewBoard(knownPuzzleSimple)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	moves := board.Moves(nil)
	// B is vertical size-3 on the exit row; only B can move initially (+1..+3).
	if len(moves) != 3 {
		t.Fatalf("expected 3 legal moves, got %d: %v", len(moves), moves)
	}
	want := []Move{{Piece: 1, Steps: 1}, {Piece: 1, Steps: 2}, {Piece: 1, Steps: 3}}
	if !reflect.DeepEqual(moves, want) {
		t.Fatalf("moves: got %v want %v", moves, want)
	}
}

func TestSolverSolvesKnownSimplePuzzle(t *testing.T) {
	board, err := NewBoard(knownPuzzleSimple)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	solution := board.Solve()
	if !solution.Solvable {
		t.Fatal("expected solvable")
	}
	if solution.NumMoves != 2 {
		t.Fatalf("expected 2 moves, got %d", solution.NumMoves)
	}
	if solution.NumSteps != 7 {
		t.Fatalf("expected 7 steps, got %d", solution.NumSteps)
	}
	want := []Move{{Piece: 1, Steps: 3}, {Piece: 0, Steps: 4}}
	if !reflect.DeepEqual(solution.Moves, want) {
		t.Fatalf("moves: got %v want %v", solution.Moves, want)
	}
}

func TestSolverSolvesKnownForty1Puzzle(t *testing.T) {
	board, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	solution := board.Solve()
	if !solution.Solvable {
		t.Fatal("expected solvable")
	}
	if solution.NumMoves != 9 {
		t.Fatalf("expected 9 moves, got %d", solution.NumMoves)
	}
	if solution.NumSteps != 21 {
		t.Fatalf("expected 21 steps, got %d", solution.NumSteps)
	}
}

func TestSolverDoesNotMutateBoard(t *testing.T) {
	board, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	beforeHash := board.Hash()
	beforePieces := append([]Piece(nil), board.Pieces...)
	beforeOccupied := append([]bool(nil), board.occupied...)

	_ = board.Solve()

	if board.Hash() != beforeHash {
		t.Fatal("board Hash changed after Solve")
	}
	if !reflect.DeepEqual(board.Pieces, beforePieces) {
		t.Fatal("board Pieces changed after Solve")
	}
	if !reflect.DeepEqual(board.occupied, beforeOccupied) {
		t.Fatal("board occupied changed after Solve")
	}
}

func TestSolverDeterministic(t *testing.T) {
	board1, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	board2, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	a := board1.Solve()
	b := board2.Solve()
	if a.NumMoves != b.NumMoves || a.NumSteps != b.NumSteps {
		t.Fatalf("non-deterministic metrics: %+v vs %+v", a, b)
	}
	if !reflect.DeepEqual(a.Moves, b.Moves) {
		t.Fatalf("non-deterministic moves: %v vs %v", a.Moves, b.Moves)
	}
}

func TestVictoryDetection(t *testing.T) {
	// Primary already at the rightmost exit cell of its row.
	desc := []string{
		"......",
		"......",
		"....AA",
		"......",
		"......",
		"......",
	}
	board, err := NewBoard(desc)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	if board.Pieces[0].Position != board.Target() {
		t.Fatalf("expected primary at target: pos=%d target=%d", board.Pieces[0].Position, board.Target())
	}
	solution := board.Solve()
	if !solution.Solvable {
		t.Fatal("already-solved board should be solvable")
	}
	if solution.NumMoves != 0 {
		t.Fatalf("expected 0 moves, got %d", solution.NumMoves)
	}
}

func TestApplySolutionReachesVictory(t *testing.T) {
	board, err := NewBoard(knownPuzzleSimple)
	if err != nil {
		t.Fatalf("NewBoard: %v", err)
	}
	solution := board.Solve()
	working := board.Copy()
	for _, move := range solution.Moves {
		working.DoMove(move)
	}
	if working.Pieces[0].Position != working.Target() {
		t.Fatalf("after solution primary at %d, target %d", working.Pieces[0].Position, working.Target())
	}
}
