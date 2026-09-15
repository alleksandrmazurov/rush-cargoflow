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
	database := flag.String("database", "data/external/rush/rush.txt", "full Rush database path (.txt or .gz)")
	output := flag.String("output", "output/RUSH009_CuratedShortlist_001", "output directory")
	cache := flag.String("cache", "data/cache/curator/solve_cache.jsonl", "persistent solve cache")
	calibration := flag.String("calibration", "data/calibration/rush008_human_review.json", "human calibration corpus")
	shortlist := flag.Int("shortlist", 24, "target shortlist size")
	seed := flag.Int64("seed", 20260915, "master seed")
	workers := flag.Int("workers", 2, "solver worker count")
	pool := flag.Int("source-pool", 1500, "stratified source candidate pool")
	maxAttempts := flag.Int("max-solves", 3000, "max transform+solve attempts")
	resume := flag.Bool("resume", true, "reuse persistent solve cache")
	flag.Parse()

	if _, err := os.Stat(*database); err != nil {
		log.Fatalf("database not found: %s\nFetch with: go run ./cmd/rush-db-fetch --out data/external/rush", *database)
	}

	cfg := rush.DefaultCuratorConfig(*database)
	cfg.OutputDir = *output
	cfg.CachePath = *cache
	cfg.CalibrationPath = *calibration
	cfg.ShortlistSize = *shortlist
	cfg.Seed = *seed
	cfg.Workers = *workers
	cfg.SourcePoolSize = *pool
	cfg.MaxSolveAttempts = *maxAttempts
	cfg.Resume = *resume

	fmt.Println("Cargo Flow Full Database Curator (RUSH-009)")
	fmt.Printf("database=%s pool=%d maxSolves=%d shortlist=%d seed=%d workers=%d\n",
		cfg.DatabasePath, cfg.SourcePoolSize, cfg.MaxSolveAttempts, cfg.ShortlistSize, cfg.Seed, cfg.Workers)

	res, err := rush.RunCurator(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := rush.WriteCuratorShortlist(*output, res); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nSource scanned: %d\n", res.SourceRecordsScanned)
	fmt.Printf("Cheap retained: %d\n", res.CheapSourcesRetained)
	fmt.Printf("Transforms: %d\n", res.TransformsAttempted)
	fmt.Printf("Exact attempts: %d (cache hits %d)\n", res.ExactSolvesAttempted, res.CacheHits)
	fmt.Printf("Exact solved: %d\n", res.ExactSolved)
	fmt.Printf("Pool after filter: %d\n", res.PoolAfterFilter)
	fmt.Printf("Families: %d\n", res.FamiliesFound)
	fmt.Printf("Shortlisted: %d\n", len(res.Shortlisted))
	fmt.Printf("Wall: %s (solver %s)\n", res.TotalWallTime, res.SolverWallTime)
	fmt.Println("Rejects:")
	keys := make([]string, 0, len(res.RejectCounts))
	for k := range res.RejectCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, res.RejectCounts[k])
	}
	fmt.Println("\nCandidate\tSourceId\tOrigOpt\tCFOpt\tFamily\tEstDiff\tVisited")
	for _, c := range res.Shortlisted {
		fmt.Printf("%s\t%s\t%d\t%d\t%s\t%.1f\t%d\n",
			c.Level.LevelID, c.Source.SourcePuzzleID, c.Source.OriginalOptimalMoves,
			c.Metrics.OptimalGestures, c.FamilyID, c.Signals.EstimatedHumanDifficultyScore,
			c.Metrics.VisitedStates)
	}
	fmt.Printf("\nWrote %s\n", *output)
	if len(res.Shortlisted) == 0 {
		os.Exit(1)
	}
}
