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
	configPath := flag.String("config", "configs/RUSH0105_NativeCoreExpansionPilot_001.json", "boardmix config JSON")
	output := flag.String("output", "", "override output directory")
	resume := flag.Bool("resume", true, "resume from checkpoint if present")
	smoke := flag.Bool("smoke", false, "tiny smoke: few bases, short budget (~30-60s)")
	buildInfo := flag.Bool("version", false, "print version and exit")
	reexport := flag.String("reexport-existing", "", "re-export Unity batch from existing dir (no regeneration)")
	validate := flag.String("validate-batch", "", "validate Unity BatchManifest/Candidates/Solutions contract")
	calibrate := flag.String("calibrate-core-space", "", "calibrate 6x6 meaningful-space metrics on existing batch dir (no generation)")
	flag.Parse()

	if *buildInfo {
		fmt.Println(rush.BoardMixVersion)
		return
	}

	if *calibrate != "" {
		budget := rush.DefaultCargoFlowSolveBudget()
		rows, summary, err := rush.CalibrateCoreSpaceOnBatch(*calibrate, budget)
		if err != nil {
			log.Fatal(err)
		}
		rep := map[string]interface{}{
			"batchDir": *calibrate,
			"rows":     rows,
			"summary":  summary,
		}
		outPath := filepath.Join(*calibrate, "CoreSpaceCalibrationReport.json")
		if err := rush.WriteJSONFile(outPath, rep); err != nil {
			log.Printf("warn: could not write %s: %v", outPath, err)
		} else {
			fmt.Printf("Wrote %s\n", outPath)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return
	}

	if *validate != "" {
		rep, err := rush.ValidateUnityBatchReport(*validate)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		if !rep.OK {
			os.Exit(1)
		}
		return
	}

	if *reexport != "" {
		fmt.Printf("Re-exporting Unity batch (no generation): %s\n", *reexport)
		rep, err := rush.ReexportBoardMixBatchFromDir(*reexport)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		if !rep.OK {
			os.Exit(1)
		}
		fmt.Printf("OK — select this folder in Unity Import Rush Batch:\n  %s\n", filepath.Clean(*reexport))
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
		cfg.OutputDir = "output/RUSH0105_CoreExpandSmoke"
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
		cfg.TargetAccepted = 4
		cfg.BaseCount = 4
		cfg.MaxPoolSize = 16
		cfg.MaxAttempts = 24
		cfg.TryOuterAugment = true
		cfg.TryNativeAugment = true
		cfg.TryCoreExpansion = true
		cfg.TryInventoryEnrichment = false
		cfg.MaxNativeAcceptedPerEmbed = 1
		cfg.MaxNativeProposalsPerEmbed = 4
		cfg.MaxCoreExpansionAccepted = 1
		cfg.MaxCoreExpansionProposals = 6
		cfg.CoreSpaceFitThreshold = rush.Default6x6FitThreshold
		cfg.RequireCoreNotFitIn6x6 = false
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
		fmt.Println("Cargo Flow Core Expansion SMOKE (RUSH-010.5)")
	} else {
		fmt.Println("Cargo Flow Boardmix (RUSH-010.5 Native Core Expansion)")
		fmt.Println("Ctrl+C safe; --resume continues. Use -calibrate-core-space / -reexport-existing / -validate-batch.")
	}

	fmt.Printf("config=%s output=%s resume=%v workers=%d target=%d maxPool=%d coreExpand=%v\n",
		*configPath, cfg.OutputDir, cfg.Resume, cfg.Workers, cfg.TargetAccepted, cfg.MaxPoolSize, cfg.TryCoreExpansion)

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
	fmt.Printf("DiversityUnmet: %v  Missing: %d\n",
		res.SelectReport.DiversityTargetUnmet, res.SelectReport.MissingCount)
	fmt.Printf("Family coverage: poolUnique=%d earlyStop=%v\n",
		res.FamilyCoverage.PoolUniqueFamilyIds, res.FamilyCoverage.EarlyStopBeforeAllFamilies)
	fmt.Printf("Unity batch OK. Import folder:\n  %s\n", cfg.OutputDir)
}
