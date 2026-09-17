package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fogleman/rush"
)

func main() {
	configPath := flag.String("config", "configs/RUSH01041_NativeFamilyCoveragePilot_001.json", "boardmix config JSON")
	output := flag.String("output", "", "override output directory")
	resume := flag.Bool("resume", true, "resume from checkpoint if present")
	smoke := flag.Bool("smoke", false, "tiny smoke: few bases, short budget (~30-60s)")
	buildInfo := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *buildInfo {
		fmt.Println(rush.BoardMixVersion)
		return
	}

	cfg, err := rush.LoadBoardMixConfigJSON(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = rush.DefaultBoardMixConfig()
			fmt.Printf("config not found (%s); using defaults\n", *configPath)
		} else {
			log.Fatal(err)
		}
	}
	if *output != "" {
		cfg.OutputDir = *output
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
	}
	cfg.Resume = *resume
	if cfg.CheckpointPath == "" {
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
	}

	if *smoke {
		cfg.OutputDir = "output/RUSH0104_NativeSmoke"
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
		cfg.TargetAccepted = 4
		cfg.BaseCount = 4
		cfg.MaxPoolSize = 12
		cfg.MaxAttempts = 20
		cfg.TryOuterAugment = true
		cfg.TryNativeAugment = true
		cfg.TryInventoryEnrichment = false
		cfg.MaxNativeAcceptedPerEmbed = 1
		cfg.MaxNativeProposalsPerEmbed = 6
		cfg.SolveTimeLimitMs = 2500
		cfg.MaxVisitedStates = 400_000
		cfg.MinDistinctBoardShapes = 2
		cfg.MinOuterZoneRelevant = 1
		cfg.MaxBoardShapeFraction = 0.75
		cfg.MaxInventoryClassFraction = 1.0
		cfg.Embeddings = []string{
			string(rush.EmbedFlushTop),
			string(rush.EmbedShiftDown1),
		}
		cfg.ShapeQuotas = map[string]int{
			string(rush.ShapeShiftedCore): 1,
			string(rush.ShapeExpanded):    1,
			string(rush.ShapeTall):        1,
			string(rush.ShapeWide):        1,
		}
		cfg.InventoryQuotas = map[string]int{string(rush.InvNo1x1): 4}
		cfg.CheckpointEvery = 1
		cfg.ProgressEvery = 1
		cfg.Workers = 2
		cfg.FamilyFirstExploration = true
		cfg.PerFamilyPoolCap = 2
		cfg.MinUniqueFamiliesInPool = 3
		fmt.Println("Cargo Flow Native Family Coverage SMOKE (RUSH-010.4.1)")
	} else {
		fmt.Println("Cargo Flow Native Family Coverage (RUSH-010.4.1)")
		fmt.Println("Family-first pool → diversity select. Ctrl+C safe; --resume continues.")
	}

	fmt.Printf("config=%s output=%s resume=%v workers=%d target=%d maxPool=%d\n",
		*configPath, cfg.OutputDir, cfg.Resume, cfg.Workers, cfg.TargetAccepted, cfg.MaxPoolSize)

	cancel := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nCtrl+C — finishing current work and saving checkpoint...")
		close(cancel)
	}()

	start := time.Now()
	res, err := rush.RunBoardMixPilot(cfg, cancel)
	if err != nil {
		log.Fatal(err)
	}
	if err := rush.WriteBoardMixBatch(cfg.OutputDir, res); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nPool: %d (unique families %d)  Final accepted: %d / target %d\n",
		len(res.Pool), res.SelectReport.PoolUniqueFamilies, len(res.Accepted), res.SelectReport.FinalTarget)
	fmt.Printf("Attempts: %d  Exact: %d  CacheHits: %d\n",
		res.Stats.Attempts, res.Stats.ExactSolves, res.Stats.CacheHits)
	fmt.Printf("Wall: %s\n", time.Since(start).Round(time.Second))
	fmt.Println("Pool inventory distribution:")
	b, _ := json.MarshalIndent(res.SelectReport.PoolInventoryDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Println("Pool shape × inventory:")
	b, _ = json.MarshalIndent(res.SelectReport.PoolShapeInventoryCross, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Println("Final shape distribution:")
	b, _ = json.MarshalIndent(res.SelectReport.FinalShapeDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Println("Final inventory distribution:")
	b, _ = json.MarshalIndent(res.SelectReport.FinalInventoryDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Printf("OuterZoneRelevant final: %d  DistinctShapes: %d  DiversityUnmet: %v  Missing: %d\n",
		res.SelectReport.FinalOuterZoneRelevant, res.SelectReport.DistinctBoardShapes,
		res.SelectReport.DiversityTargetUnmet, res.SelectReport.MissingCount)
	if len(res.SelectReport.WhyFinalShort) > 0 {
		fmt.Println("Why final short:")
		for _, w := range res.SelectReport.WhyFinalShort {
			fmt.Printf("  - %s\n", w)
		}
	}
	if len(res.SelectReport.UnmetRequirements) > 0 {
		fmt.Println("Unmet requirements:")
		for _, u := range res.SelectReport.UnmetRequirements {
			fmt.Printf("  - %s\n", u)
		}
	}
	fmt.Printf("Expanded/FullField diag: %s\n", res.SelectReport.ExpandedFullFieldDiag.Summary)
	if len(res.Stats.AcceptedByAugmentationClass) > 0 {
		fmt.Println("Final augmentation classes:")
		b, _ = json.MarshalIndent(res.Stats.AcceptedByAugmentationClass, "  ", "  ")
		fmt.Println(" ", string(b))
	}
	fmt.Printf("Native stats: baseFamilies=%d embeddings=%d augmentationsProposed=%d restrictedSolves=%d\n",
		res.Stats.BaseFamiliesTried, res.Stats.EmbeddingsTried, res.Stats.AugmentationsProposed, res.Stats.RestrictedSolves)
	fmt.Printf("Family coverage: requested=%d distinct=%d poolUnique=%d earlyStop=%v\n",
		res.FamilyCoverage.RequestedBaseCandidates, res.FamilyCoverage.DistinctRequestedBaseFamilyIds,
		res.FamilyCoverage.PoolUniqueFamilyIds, res.FamilyCoverage.EarlyStopBeforeAllFamilies)
	fmt.Printf("Family root cause: %s\n", res.FamilyCoverage.RootCauseSummary)
	if res.FamilyCoverage.ExpandedZeroExplanation != "" {
		fmt.Printf("Expanded=0: %s\n", res.FamilyCoverage.ExpandedZeroExplanation)
	}
	if len(res.FamilyCoverage.PoolAugmentationClass) > 0 {
		fmt.Println("Pool augmentation classes:")
		b, _ = json.MarshalIndent(res.FamilyCoverage.PoolAugmentationClass, "  ", "  ")
		fmt.Println(" ", string(b))
	}
	fmt.Printf("\nWrote %s\n", cfg.OutputDir)
	fmt.Printf("Checkpoint: %s\n", cfg.CheckpointPath)
	if len(res.Accepted) == 0 {
		os.Exit(1)
	}
}
