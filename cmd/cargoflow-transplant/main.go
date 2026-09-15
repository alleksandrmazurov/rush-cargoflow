package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/fogleman/rush"
)

func main() {
	dataset := flag.String("dataset", "testdata/rushdb/rush1000.txt", "path to rush1000.txt")
	output := flag.String("output", "output/RUSH008_DatabaseTransplant_Pilot", "output directory")
	maxSource := flag.Int("max-source", 1000, "max source puzzles to scan")
	minOptimal := flag.Int("min-optimal", 8, "min Cargo Flow OptimalGestures")
	maxOptimal := flag.Int("max-optimal", 20, "max Cargo Flow OptimalGestures")
	count := flag.Int("count", 12, "target accepted candidates")
	flag.Parse()

	cfg := rush.DefaultTransplantConfig(*dataset)
	cfg.OutputDir = *output
	cfg.MaxSourcePuzzles = *maxSource
	cfg.MinOptimal = *minOptimal
	cfg.MaxOptimal = *maxOptimal
	cfg.TargetAccepted = *count

	fmt.Println("Cargo Flow Database Transplant POC (RUSH-008)")
	fmt.Printf("dataset=%s maxSource=%d range=%d..%d target=%d\n",
		cfg.DatasetPath, cfg.MaxSourcePuzzles, cfg.MinOptimal, cfg.MaxOptimal, cfg.TargetAccepted)
	fmt.Println()

	result, err := rush.RunTransplantPilot(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := rush.WriteTransplantBatch(*output, result); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Source scanned: %d\n", result.SourceScanned)
	fmt.Printf("Transform attempts: %d\n", result.TransformAttempts)
	fmt.Printf("Valid transforms: %d\n", result.ValidTransforms)
	fmt.Printf("Exact solved: %d\n", result.ExactSolved)
	fmt.Printf("Accepted: %d\n", len(result.Accepted))
	fmt.Printf("Total elapsed: %s\n", result.TotalElapsed)
	if st, ok := result.SolveTimeStats(); ok {
		fmt.Printf("Solve times ms: avg=%.1f median=%.1f max=%d (n=%d)\n",
			st.Average, st.Median, st.Max, st.Count)
	}
	fmt.Println("Rejected:")
	keys := make([]string, 0, len(result.Rejected))
	for k := range result.Rejected {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, result.Rejected[rush.TransplantRejection(k)])
	}
	fmt.Println()
	fmt.Println("Buckets:")
	for _, k := range []string{"8-10", "11-13", "14-16", "17-20"} {
		fmt.Printf("  %s: %d\n", k, result.BucketCounts[k])
	}
	fmt.Println()
	fmt.Println("Candidate\tSourceId\tOrigOpt\tCluster\tEmbed\tPieces\tCFOpt\tVisited\tElapsedMs\tReplay")
	for _, c := range result.Accepted {
		fmt.Printf("%s\t%s\t%d\t%d\t%s\t%d\t%d\t%d\t%d\tPASS\n",
			c.CandidateID, c.Source.SourcePuzzleID, c.OriginalOptimalMoves, c.OriginalClusterSize,
			c.Embedding, len(c.Level.Pieces), c.OptimalGestures, c.VisitedStates, c.ElapsedMs)
	}
	fmt.Println()
	fmt.Println("Original vs Cargo Flow:")
	for _, c := range result.Accepted {
		fmt.Printf("  %s orig=%d => cf=%d (delta %+d)\n",
			c.Source.SourcePuzzleID, c.OriginalOptimalMoves, c.OptimalGestures,
			c.OptimalGestures-c.OriginalOptimalMoves)
	}
	fmt.Printf("\nUnique source IDs: %v\n", result.UniqueSourceIDs())
	fmt.Printf("Wrote batch to %s\n", *output)
	if len(result.Accepted) == 0 {
		os.Exit(1)
	}
}
