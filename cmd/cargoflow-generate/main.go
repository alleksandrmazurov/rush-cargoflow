package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/fogleman/rush"
)

func main() {
	count := flag.Int("count", 10, "target accepted candidates")
	minOptimal := flag.Int("min-optimal", 10, "minimum ExactOptimalGestures")
	seed := flag.Int64("seed", 12345, "master seed")
	maxAttempts := flag.Int("max-attempts", 300, "hard attempt cap")
	output := flag.String("output", "output/pilot", "output directory")
	timeLimit := flag.Duration("time-limit", 10*time.Second, "per-candidate solve time limit")
	maxVisited := flag.Int("max-visited", 4_000_000, "per-candidate max visited states")
	flag.Parse()

	cfg := rush.DefaultCargoFlowGenerationConfig(*seed)
	cfg.TargetAccepted = *count
	cfg.MinOptimalGestures = *minOptimal
	cfg.MaxAttempts = *maxAttempts
	cfg.SolveTimeLimit = *timeLimit
	cfg.MaxVisitedStates = *maxVisited

	fmt.Println("Cargo Flow Generator POC")
	fmt.Printf("version=%s seed=%d count=%d minOptimal=%d maxAttempts=%d\n",
		cfg.GeneratorVersion, cfg.MasterSeed, cfg.TargetAccepted, cfg.MinOptimalGestures, cfg.MaxAttempts)
	fmt.Printf("solveBudget: time=%s visited=%d\n", cfg.SolveTimeLimit, cfg.MaxVisitedStates)
	fmt.Println()

	gen := rush.NewCargoFlowGenerator(cfg)
	result := gen.Generate()

	if err := rush.WriteGenerationBatch(*output, result); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Attempts: %d\n", result.Attempts)
	fmt.Printf("Accepted: %d\n", len(result.Accepted))
	fmt.Printf("Accepted/Attempts: %.3f\n", float64(len(result.Accepted))/mathMax1(result.Attempts))
	fmt.Printf("Total elapsed: %s\n", result.TotalElapsed)
	if stats, ok := result.SolveTimeStats(); ok {
		fmt.Printf("Solve times ms: avg=%.1f median=%.1f max=%d (n=%d)\n",
			stats.Average, stats.Median, stats.Max, stats.Count)
	}
	fmt.Println("Rejected:")
	keys := make([]string, 0, len(result.Rejected))
	for k := range result.Rejected {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, result.Rejected[rush.RejectionReason(k)])
	}
	fmt.Println()
	fmt.Println("Candidate\tProfile\tSeed\tPieces\tOptimal\tVisited\tElapsedMs\tSim\tReplay")
	for _, c := range result.Accepted {
		fmt.Printf("%s\t%s\t%d\t%d\t%d\t%d\t%d\t%.3f\tPASS\n",
			c.CandidateID, c.Profile, c.AttemptSeed, len(c.Level.Pieces),
			c.OptimalGestures, c.VisitedStates, c.ElapsedMs, c.SimilarityToNearest)
	}

	buckets := map[string]int{"10-12": 0, "13-15": 0, "16-20": 0, "21+": 0}
	for _, c := range result.Accepted {
		switch {
		case c.OptimalGestures <= 12:
			buckets["10-12"]++
		case c.OptimalGestures <= 15:
			buckets["13-15"]++
		case c.OptimalGestures <= 20:
			buckets["16-20"]++
		default:
			buckets["21+"]++
		}
	}
	fmt.Println()
	fmt.Println("Optimal distribution:")
	for _, k := range []string{"10-12", "13-15", "16-20", "21+"} {
		fmt.Printf("  %s: %d\n", k, buckets[k])
	}

	fmt.Printf("\nWrote batch to %s\n", *output)
	if len(result.Accepted) == 0 {
		os.Exit(1)
	}
}

func mathMax1(n int) float64 {
	if n <= 0 {
		return 1
	}
	return float64(n)
}
