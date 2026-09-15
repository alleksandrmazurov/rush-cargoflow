package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
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
	diverse := flag.Bool("diverse", false, "enable puzzle archetypes + functional diversity (RUSH-007)")
	archetypes := flag.String("archetypes", "all", "comma list or 'all'")
	humanRef := flag.String("human-rejected-ref", "", "dir with RUSH-005 Candidate_001..005 for functional anti-clone filter")
	flag.Parse()

	var cfg rush.CargoFlowGenerationConfig
	if *diverse {
		cfg = rush.DefaultDiverseCargoFlowGenerationConfig(*seed)
		if *archetypes != "all" && *archetypes != "" {
			parts := strings.Split(*archetypes, ",")
			cfg.Archetypes = nil
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cfg.Archetypes = append(cfg.Archetypes, rush.PuzzleArchetype(p))
				}
			}
		}
		refDir := *humanRef
		if refDir == "" {
			refDir = "output/pilot"
		}
		ids := []string{"Candidate_001", "Candidate_002", "Candidate_003", "Candidate_004", "Candidate_005"}
		sigs, err := rush.LoadHumanRejectedPilotSignatures(refDir, ids)
		if err != nil {
			log.Printf("warning: human-rejected refs not loaded (%v); continuing without ref filter", err)
		} else {
			cfg.HumanRejectedSignatures = sigs
			fmt.Printf("Loaded %d human-rejected reference signatures from %s\n", len(sigs), refDir)
		}
	} else {
		cfg = rush.DefaultCargoFlowGenerationConfig(*seed)
	}
	cfg.TargetAccepted = *count
	cfg.MinOptimalGestures = *minOptimal
	cfg.MaxAttempts = *maxAttempts
	cfg.SolveTimeLimit = *timeLimit
	cfg.MaxVisitedStates = *maxVisited

	fmt.Println("Cargo Flow Generator")
	fmt.Printf("version=%s seed=%d count=%d minOptimal=%d maxAttempts=%d diverse=%v\n",
		cfg.GeneratorVersion, cfg.MasterSeed, cfg.TargetAccepted, cfg.MinOptimalGestures, cfg.MaxAttempts, cfg.DiverseMode)
	fmt.Printf("solveBudget: time=%s visited=%d\n", cfg.SolveTimeLimit, cfg.MaxVisitedStates)
	if cfg.DiverseMode {
		fmt.Printf("functionalThreshold=%.2f layoutThreshold=%.2f maxPerArchetype=%d\n",
			cfg.FunctionalSimilarityThreshold, cfg.SimilarityThreshold, cfg.MaxPerArchetype)
	}
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

	if cfg.DiverseMode {
		fmt.Println()
		fmt.Println("Candidate\tArchetype\tSeed\tPieces\tOptimal\tDepDepth\tMoved\tBlockers\tLaySim\tFunSim\tNearest\tReplay")
		for _, c := range result.Accepted {
			fmt.Printf("%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%.3f\t%.3f\t%s\tPASS\n",
				c.CandidateID, c.Archetype, c.AttemptSeed, len(c.Level.Pieces),
				c.OptimalGestures, c.Signature.DependencyDepth, c.Signature.DistinctMovedPieces,
				c.Signature.InitialTargetBlockerCount, c.SimilarityToNearest,
				c.FunctionalSimilarityNearest, c.NearestCandidateID)
		}
		fmt.Println()
		fmt.Println("Archetype distribution:")
		dist := map[string]int{}
		for _, c := range result.Accepted {
			dist[string(c.Archetype)]++
		}
		for _, a := range rush.AllPuzzleArchetypes() {
			fmt.Printf("  %s: %d\n", a, dist[string(a)])
		}
		if len(result.Accepted) > 1 {
			fmt.Println()
			fmt.Println("FunctionalSimilarity matrix:")
			fmt.Print("      ")
			for i := range result.Accepted {
				fmt.Printf(" %03d", i+1)
			}
			fmt.Println()
			for i := range result.Accepted {
				fmt.Printf("%03d  ", i+1)
				for j := range result.Accepted {
					s := rush.FunctionalSimilarity(result.Accepted[i].Signature, result.Accepted[j].Signature)
					fmt.Printf(" %.2f", s)
				}
				fmt.Println()
			}
		}
	} else {
		fmt.Println()
		fmt.Println("Candidate\tProfile\tSeed\tPieces\tOptimal\tVisited\tElapsedMs\tSim\tReplay")
		for _, c := range result.Accepted {
			fmt.Printf("%s\t%s\t%d\t%d\t%d\t%d\t%d\t%.3f\tPASS\n",
				c.CandidateID, c.Profile, c.AttemptSeed, len(c.Level.Pieces),
				c.OptimalGestures, c.VisitedStates, c.ElapsedMs, c.SimilarityToNearest)
		}
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
