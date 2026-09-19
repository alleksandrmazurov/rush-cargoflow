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
	configPath := flag.String("config", "configs/RUSH01071_DeepTargetPilot_001.json", "boardmix config JSON")
	output := flag.String("output", "", "override output directory")
	resume := flag.Bool("resume", true, "resume from checkpoint if present")
	smoke := flag.Bool("smoke", false, "tiny smoke: few bases, short budget (~30-60s)")
	buildInfo := flag.Bool("version", false, "print version and exit")
	reexport := flag.String("reexport-existing", "", "re-export Unity batch from existing dir (no regeneration)")
	validate := flag.String("validate-batch", "", "validate Unity BatchManifest/Candidates/Solutions contract")
	calibrate := flag.String("calibrate-core-space", "", "calibrate 6x6 meaningful-space metrics on existing batch dir (no generation)")
	calibrateTargetDepth := flag.String("calibrate-target-depth", "", "analyze target depth and vertical dependencies in an existing batch (no generation)")
	analyzeSpatial := flag.String("analyze-spatial-dependencies", "", "write RUSH01041/RUSH0105 spatial dependency reports to this directory (no generation)")
	analyzeDeepTarget := flag.String("analyze-deep-target-routing", "", "write RUSH0107 source-column/embedding funnel reports to this directory")
	reselect := flag.String("reselect-existing", "", "reselect final 12 from checkpoint pool (no generation)")
	flag.Parse()

	if *buildInfo {
		fmt.Println(rush.BoardMixVersion)
		return
	}

	if *analyzeSpatial != "" {
		report, err := rush.AnalyzeSpatialDependencies(
			"output/RUSH01041_NativeFamilyCoveragePilot_001",
			"output/RUSH0105_NativeCoreExpansionPilot_001",
			*analyzeSpatial,
			rush.DefaultCargoFlowSolveBudget(),
		)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := rush.MarshalSpatialReport(report)
		fmt.Println(string(b))
		fmt.Printf("Wrote %s\n", filepath.Join(*analyzeSpatial, "SpatialDependencyAnalysisReport.json"))
		fmt.Printf("Wrote %s\n", filepath.Join(*analyzeSpatial, "SpatialDependencyAnalysisReport.md"))
		return
	}

	if *analyzeDeepTarget != "" {
		report, err := rush.AnalyzeDeepTargetRouting(
			"output/RUSH009_CuratedShortlist_001",
			"output/RUSH0107_CrossRegionCausalPilot_001/checkpoint.json",
			"data/external/rush/rush.txt",
			*analyzeDeepTarget,
			rush.DefaultCargoFlowSolveBudget(),
		)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		fmt.Printf("Wrote %s\n", filepath.Join(*analyzeDeepTarget, "DeepTargetRoutingAnalysis.json"))
		fmt.Printf("Wrote %s\n", filepath.Join(*analyzeDeepTarget, "DeepTargetRoutingAnalysis.md"))
		return
	}

	if *calibrateTargetDepth != "" {
		labels := map[string]string(nil)
		if filepath.Base(filepath.Clean(*calibrateTargetDepth)) == "RUSH01041_NativeFamilyCoveragePilot_001" {
			labels = rush.RUSH01041HumanCoreLabels
		}
		rows, summary, err := rush.CalibrateTargetDepthOnBatch(
			*calibrateTargetDepth, labels, rush.DefaultCargoFlowSolveBudget())
		if err != nil {
			log.Fatal(err)
		}
		rep := map[string]interface{}{
			"batchDir": *calibrateTargetDepth,
			"rows":     rows,
			"summary":  summary,
		}
		outPath := filepath.Join(*calibrateTargetDepth, "TargetDepthCalibrationReport.json")
		if err := rush.WriteJSONFile(outPath, rep); err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		fmt.Printf("Wrote %s\n", outPath)
		return
	}

	if *reselect != "" {
		cfg, err := rush.LoadBoardMixConfigJSON(*configPath)
		if err != nil {
			log.Fatal(err)
		}
		if *output != "" {
			cfg.OutputDir = *output
		}
		fmt.Printf("Reselect from existing pool (no generation): %s\n", *reselect)
		fmt.Printf("config=%s targetGenuine=%d targetCausal=%d targetAccepted=%d\n",
			*configPath, cfg.TargetGenuineCoreExpanded, cfg.TargetCausalExpanded, cfg.TargetAccepted)
		rep, err := rush.ReselectBoardMixFromExisting(*reselect, cfg)
		if err != nil {
			b, _ := json.MarshalIndent(rep, "", "  ")
			fmt.Println(string(b))
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		fmt.Printf("OK — FinalAccepted=%d Genuine=%d/%d Causal=%d/%d PoolPreserved=%v\n",
			rep.SelectReport.FinalAccepted,
			rep.SelectReport.FinalGenuineCoreExpanded,
			rep.SelectReport.FinalGenuineCoreExpandedTarget,
			rep.SelectReport.FinalCausalExpanded,
			rep.SelectReport.FinalCausalExpandedTarget,
			rep.PoolPreserved)
		fmt.Printf("Import folder:\n  %s\n", filepath.Clean(*reselect))
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
		cfg.OutputDir = "output/RUSH01071_DeepTargetSmoke"
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
		cfg.TargetAccepted = 2
		cfg.TargetCausalExpanded = 1
		cfg.SourceAwareEmbeddingRouting = true
		cfg.TargetDeepRows5Plus = 1
		cfg.TargetBottomRow6 = 1
		cfg.TargetTopRowQuotas = map[string]int{"6": 1, "4": 1}
		cfg.TargetGenuineCoreExpanded = 0
		cfg.BaseCount = 2
		cfg.MaxPoolSize = 8
		cfg.MaxAttempts = 8
		cfg.TryOuterAugment = false
		cfg.TryNativeAugment = false
		cfg.TryCoreExpansion = false
		cfg.TryCausalSynthesis = true
		cfg.TryInventoryEnrichment = false
		cfg.MaxCausalAcceptedPerEmbed = 1
		cfg.MaxCausalProposalsPerEmbed = 4
		cfg.CoreSpaceFitThreshold = rush.Default6x6FitThreshold
		cfg.RequireCoreNotFitIn6x6 = false
		cfg.SolveTimeLimitMs = 1500
		cfg.MaxVisitedStates = 250_000
		cfg.MinDistinctBoardShapes = 1
		cfg.MinOuterZoneRelevant = 1
		cfg.MaxBoardShapeFraction = 1.0
		cfg.MaxInventoryClassFraction = 1.0
		cfg.Embeddings = []string{
			string(rush.EmbedFlushTop),
			string(rush.EmbedShiftDown1),
			string(rush.EmbedFlushBottom),
		}
		cfg.ShapeQuotas = map[string]int{}
		cfg.InventoryQuotas = map[string]int{}
		cfg.CheckpointEvery = 1
		cfg.ProgressEvery = 1
		cfg.Workers = 2
		cfg.FamilyFirstExploration = true
		cfg.PerFamilyPoolCap = 2
		cfg.MinUniqueFamiliesInPool = 3
		fmt.Println("Cargo Flow Deep-Target Causal SMOKE (RUSH-010.7.1)")
	} else {
		fmt.Println("Cargo Flow Boardmix (RUSH-010.7.1 Deep-Target Source Routing)")
		fmt.Println("Ctrl+C safe; --resume continues. Use -calibrate-core-space / -reexport-existing / -validate-batch.")
	}

	fmt.Printf("config=%s output=%s resume=%v workers=%d target=%d maxPool=%d causal=%v\n",
		*configPath, cfg.OutputDir, cfg.Resume, cfg.Workers, cfg.TargetAccepted, cfg.MaxPoolSize, cfg.TryCausalSynthesis)

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
