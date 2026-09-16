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
	configPath := flag.String("config", "configs/RUSH01031_DiversityQuotaPilot_001.json", "boardmix config JSON")
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
		cfg.OutputDir = "output/RUSH01031_BoardMixSmoke"
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
		cfg.TargetAccepted = 4
		cfg.BaseCount = 4
		cfg.MaxPoolSize = 16
		cfg.MaxAttempts = 24
		cfg.TryOuterAugment = true
		cfg.TryInventoryEnrichment = false // keep smoke fast
		cfg.SolveTimeLimitMs = 3000
		cfg.MaxVisitedStates = 500_000
		cfg.MinDistinctBoardShapes = 2
		cfg.MinOuterZoneRelevant = 1
		cfg.MaxBoardShapeFraction = 0.75
		cfg.MaxInventoryClassFraction = 1.0
		cfg.Embeddings = []string{
			string(rush.EmbedFlushTop),
			string(rush.EmbedShiftDown1),
			string(rush.EmbedFlushTopMirrorH),
		}
		cfg.ShapeQuotas = map[string]int{
			string(rush.ShapeCompact6x6):  1,
			string(rush.ShapeShiftedCore): 1,
			string(rush.ShapeExpanded):    1,
			string(rush.ShapeTall):        1,
		}
		cfg.InventoryQuotas = map[string]int{string(rush.InvNo1x1): 4}
		cfg.CheckpointEvery = 1
		cfg.ProgressEvery = 1
		cfg.Workers = 2
		fmt.Println("Cargo Flow Board Mix SMOKE (RUSH-010.3.1)")
	} else {
		fmt.Println("Cargo Flow Board Space Diversity Quota Fix (RUSH-010.3.1)")
		fmt.Println("Pool → diversity select. Ctrl+C safe; --resume continues.")
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

	fmt.Printf("\nPool: %d  Final accepted: %d\n", len(res.Pool), len(res.Accepted))
	fmt.Printf("Attempts: %d  Exact: %d  CacheHits: %d\n",
		res.Stats.Attempts, res.Stats.ExactSolves, res.Stats.CacheHits)
	fmt.Printf("Wall: %s\n", time.Since(start).Round(time.Second))
	fmt.Println("Pool shape distribution:")
	b, _ := json.MarshalIndent(res.SelectReport.PoolShapeDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Println("Final shape distribution:")
	b, _ = json.MarshalIndent(res.SelectReport.FinalShapeDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Println("Final inventory distribution:")
	b, _ = json.MarshalIndent(res.SelectReport.FinalInventoryDist, "  ", "  ")
	fmt.Println(" ", string(b))
	fmt.Printf("OuterZoneRelevant final: %d  DistinctShapes: %d  DiversityUnmet: %v\n",
		res.SelectReport.FinalOuterZoneRelevant, res.SelectReport.DistinctBoardShapes,
		res.SelectReport.DiversityTargetUnmet)
	fmt.Printf("\nWrote %s\n", cfg.OutputDir)
	fmt.Printf("Checkpoint: %s\n", cfg.CheckpointPath)
	if len(res.Accepted) == 0 {
		os.Exit(1)
	}
}
