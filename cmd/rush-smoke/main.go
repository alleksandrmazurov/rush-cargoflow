// Minimal developer smoke command for Rush Hour baseline validation.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/fogleman/rush"
)

// Known beginner puzzle (cmd/forty level 1). Baseline: 9 moves / 21 steps.
var knownPuzzle = []string{
	"..B.CC",
	"..B...",
	"AAB...",
	"DDD..E",
	".....E",
	".....E",
}

func main() {
	board, err := rush.NewBoard(knownPuzzle)
	if err != nil {
		log.Fatal(err)
	}

	before := board.Hash()
	start := time.Now()
	solution := board.Solve()
	elapsed := time.Since(start)
	after := board.Hash()

	fmt.Printf("solved: %v\n", solution.Solvable)
	fmt.Printf("moves: %d\n", solution.NumMoves)
	fmt.Printf("steps: %d\n", solution.NumSteps)
	fmt.Printf("visited states: %d\n", solution.MemoSize)
	fmt.Printf("memo hits: %d\n", solution.MemoHits)
	fmt.Printf("elapsed: %s\n", elapsed)
	fmt.Printf("board mutated: %v\n", before != after)
	if solution.Solvable {
		fmt.Printf("path: %v\n", solution.Moves)
	}

	if !solution.Solvable {
		log.Fatal("smoke failed: puzzle not solved")
	}
	if solution.NumMoves != 9 || solution.NumSteps != 21 {
		log.Fatalf("smoke failed: unexpected baseline (moves=%d steps=%d)", solution.NumMoves, solution.NumSteps)
	}
	if before != after {
		log.Fatal("smoke failed: solver mutated input board")
	}
}
