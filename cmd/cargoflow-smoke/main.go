package main

import (
	"fmt"
	"log"
	"time"

	"github.com/fogleman/rush"
)

func main() {
	fmt.Println("Cargo Flow POC")
	fmt.Printf("Board: %dx%d\n", rush.CargoFlowWidth, rush.CargoFlowHeight)
	fmt.Printf("Exit column (0-based): %d\n", rush.CargoFlowExitCol)
	fmt.Println()

	board, err := rush.NewCargoFlowBoard(rush.CargoFlowPOCFixture())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(board)
	fmt.Println()

	start := time.Now()
	solution := board.Solve()
	elapsed := time.Since(start)

	fmt.Printf("Solved: %v\n", solution.Solvable)
	fmt.Printf("Optimal gestures: %d\n", solution.NumMoves)
	fmt.Printf("Visited states: %d\n", solution.MemoSize)
	fmt.Printf("Elapsed: %s\n", elapsed)

	if !solution.Solvable {
		log.Fatal("smoke failed: not solved")
	}

	replay, err := rush.NewCargoFlowBoard(rush.CargoFlowPOCFixture())
	if err != nil {
		log.Fatal(err)
	}
	if err := replay.Replay(solution.Moves); err != nil {
		log.Fatalf("Replay: FAIL (%v)", err)
	}
	fmt.Println("Replay: PASS")
	fmt.Println()
	fmt.Println("Solution:")
	fmt.Print(rush.FormatCargoSolution(board, solution.Moves))

	if solution.NumMoves != 4 {
		log.Fatalf("smoke failed: expected 4 optimal gestures, got %d", solution.NumMoves)
	}
}
