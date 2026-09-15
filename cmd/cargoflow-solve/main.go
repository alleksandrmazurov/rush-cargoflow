package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fogleman/rush"
)

func main() {
	input := flag.String("input", "", "Cargo Flow level JSON path")
	output := flag.String("output", "", "optional solution JSON path")
	timeLimit := flag.Duration("time-limit", 8*time.Second, "solve time limit")
	maxVisited := flag.Int("max-visited", 1_500_000, "max visited memo states")
	maxDepth := flag.Int("max-depth", 64, "max gesture depth")
	flag.Parse()

	if *input == "" {
		fmt.Fprintf(os.Stderr, "usage: cargoflow-solve --input level.json [--output solution.json]\n")
		os.Exit(2)
	}

	level, err := rush.LoadCargoFlowLevelJSONFile(*input)
	if err != nil {
		log.Fatalf("import: %v", err)
	}
	board, err := rush.BoardFromLevelJSON(level)
	if err != nil {
		log.Fatalf("board: %v", err)
	}

	fmt.Println("Cargo Flow Solve")
	fmt.Printf("Level: %s\n", level.LevelID)
	fmt.Printf("Board: %dx%d\n", level.Width, level.Height)
	fmt.Printf("Exit: %s column %d\n", level.Exit.Side, level.Exit.Column)
	fmt.Printf("Pieces: %d (movables+target; statics as walls=%d)\n", len(board.Pieces), len(board.Walls))
	if level.CanonicalGestures != nil {
		fmt.Printf("Canonical/Human gestures: %d\n", *level.CanonicalGestures)
	}
	fmt.Println()
	fmt.Println("Unity fingerprint (exit at top of dump):")
	fmt.Print(level.FingerprintUnity())
	fmt.Println()

	budget := rush.SolveBudget{
		TimeLimit:  *timeLimit,
		MaxVisited: *maxVisited,
		MaxDepth:   *maxDepth,
	}
	start := time.Now()
	sol := board.SolveWithBudget(budget)
	elapsed := time.Since(start)
	stats := rush.LastCargoSolveStats()

	replayPass := false
	if sol.Solvable {
		replay := board.Copy()
		// Fresh board for replay
		b2, err := rush.BoardFromLevelJSON(level)
		if err != nil {
			log.Fatalf("replay board: %v", err)
		}
		if err := b2.Replay(sol.Moves); err != nil {
			fmt.Printf("Replay: FAIL (%v)\n", err)
		} else {
			replayPass = true
			fmt.Println("Replay: PASS")
		}
		_ = replay
	} else {
		fmt.Println("Replay: SKIP")
	}

	fmt.Printf("Solved: %v\n", sol.Solvable)
	fmt.Printf("Optimal: %v\n", sol.Solvable && !sol.TimedOut && !sol.BudgetExceeded)
	fmt.Printf("Optimal Gestures: %d\n", sol.NumMoves)
	fmt.Printf("Visited States: %d\n", sol.MemoSize)
	fmt.Printf("Expanded: %d\n", stats.ExpandedStates)
	fmt.Printf("Generated successors: %d\n", stats.GeneratedSuccessors)
	fmt.Printf("Duplicates rejected: %d\n", stats.DuplicateRejected)
	fmt.Printf("Peak frontier: %d\n", stats.PeakFrontier)
	fmt.Printf("Alloc bytes Δ: %d\n", stats.AllocatedBytes)
	fmt.Printf("Mallocs Δ: %d\n", stats.Mallocs)
	fmt.Printf("GC Δ: %d\n", stats.NumGC)
	fmt.Printf("Elapsed: %s (%d ms)\n", elapsed, elapsed.Milliseconds())
	if sol.TimedOut {
		fmt.Println("Status: TIMEOUT")
	} else if sol.BudgetExceeded {
		fmt.Println("Status: BUDGET_EXCEEDED")
	} else if sol.Solvable {
		fmt.Println("Status: OK")
	} else {
		fmt.Println("Status: UNSOLVED")
	}
	fmt.Println()
	if sol.Solvable {
		fmt.Println("Solution:")
		fmt.Print(rush.FormatCargoSolution(board, sol.Moves))
	}

	if *output != "" {
		doc := rush.ExportSolutionJSON(level, board, sol, elapsed, replayPass)
		if err := rush.WriteJSONFile(*output, doc); err != nil {
			log.Fatalf("write solution: %v", err)
		}
		fmt.Printf("\nWrote %s\n", *output)
	}

	if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded || !replayPass {
		os.Exit(1)
	}
}
