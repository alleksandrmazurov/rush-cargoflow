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
	batch := flag.String("batch", "output/RUSH009_CuratedShortlist_001", "RUSH-009 curated batch directory")
	output := flag.String("output", "", "output directory (default depends on mode)")
	count := flag.Int("count", 8, "target accepted enriched candidates")
	bases := flag.Int("bases", 0, "number of base families to attempt (0=mode default)")
	seed := flag.Int64("seed", 20260915, "deterministic seed")
	roleDiversity := flag.Bool("role-diversity", false, "RUSH-010.1: prefer off-corridor 1x1 roles + DirectBlocker quota")
	inventoryDiversity := flag.Bool("inventory-diversity", false, "RUSH-010.2: mix No1x1/One/Two/Three movable 1x1 inventory classes")
	flag.Parse()

	if *inventoryDiversity {
		runInventory(*batch, *output, *count, *bases, *seed)
		return
	}

	var cfg rush.EnrichmentConfig
	if *roleDiversity {
		cfg = rush.DefaultRoleDiversityConfig(*batch)
		fmt.Println("Cargo Flow 1x1 Role Diversity POC (RUSH-010.1)")
	} else {
		cfg = rush.DefaultEnrichmentConfig(*batch)
		fmt.Println("Cargo Flow 1x1 Enrichment POC (RUSH-010)")
	}
	if *output != "" {
		cfg.OutputDir = *output
	}
	cfg.TargetAccepted = *count
	if *bases > 0 {
		cfg.BaseCount = *bases
	}
	cfg.Seed = *seed

	fmt.Printf("batch=%s bases=%d target=%d roleDiversity=%v preferOffCorridor=%v\n",
		cfg.BatchDir, cfg.BaseCount, cfg.TargetAccepted, cfg.RoleDiversity, cfg.PreferOffCorridor)

	res, err := rush.RunEnrichmentPilot(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := rush.WriteEnrichmentBatch(cfg.OutputDir, res); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nBases attempted: %d\n", res.BasesAttempted)
	fmt.Printf("Placements evaluated: %d (off=%d direct=%d)\n",
		res.PlacementsEvaluated, res.OffCorridorPlacementsEval, res.DirectPlacementsEval)
	fmt.Printf("Exact solves: %d\n", res.ExactSolves)
	fmt.Printf("Necessity solves: %d\n", res.NecessitySolves)
	fmt.Printf("Accepted: %d\n", len(res.Accepted))
	fmt.Printf("Total elapsed: %s\n", res.TotalElapsed)
	inN, outN := res.CorridorFlags()
	fmt.Printf("InitiallyInTargetCorridor: %d | Outside: %d\n", inN, outN)
	fmt.Println("Role distribution:")
	rd := res.RoleDistribution()
	keys := make([]string, 0, len(rd))
	for k := range rd {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, rd[k])
	}
	fmt.Println("Rejected:")
	rkeys := make([]string, 0, len(res.Rejected))
	for k := range res.Rejected {
		rkeys = append(rkeys, k)
	}
	sort.Strings(rkeys)
	for _, k := range rkeys {
		fmt.Printf("  %s: %d\n", k, res.Rejected[k])
	}
	fmt.Println("\nCandidate\tBase\tFamily\tBaseOpt\tEnrOpt\tRole\tInCorr\tMoves\tEss\tReplay")
	for _, c := range res.Accepted {
		fmt.Printf("%s\t%s\t%s\t%d\t%d\t%s\t%v\t%d\t%d\tPASS\n",
			c.CandidateID, c.Base.CandidateID, c.Base.FamilyID,
			c.Base.BaseOptimal, c.EnrichedOptimal, c.RoleEvidence.Role,
			c.RoleEvidence.InitiallyInTargetCorridor,
			c.OneByOneMovesInOptimal, c.Essential1x1Count)
	}
	fmt.Printf("\nWrote %s\n", cfg.OutputDir)
	if len(res.Accepted) == 0 {
		os.Exit(1)
	}
}

func runInventory(batch, output string, count, bases int, seed int64) {
	fmt.Println("Cargo Flow Inventory Diversity POC (RUSH-010.2)")
	cfg := rush.DefaultInventoryDiversityConfig(batch)
	if output != "" {
		cfg.OutputDir = output
	}
	if count > 0 {
		cfg.TargetAccepted = count
	}
	if bases > 0 {
		cfg.BaseCount = bases
	}
	cfg.Seed = seed

	fmt.Printf("batch=%s bases=%d target=%d quotas=No:%d One:%d Two:%d Three:%d\n",
		cfg.BatchDir, cfg.BaseCount, cfg.TargetAccepted,
		cfg.QuotaNo1x1, cfg.QuotaOne1x1, cfg.QuotaTwo1x1, cfg.QuotaThree1x1)

	res, err := rush.RunInventoryDiversityPilot(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := rush.WriteInventoryDiversityBatch(cfg.OutputDir, res); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nBases attempted: %d\n", res.BasesAttempted)
	fmt.Printf("Placement combinations evaluated: %d\n", res.PlacementsEvaluated)
	fmt.Printf("Exact solves: %d\n", res.ExactSolves)
	fmt.Printf("Restricted solves: %d\n", res.NecessitySolves)
	fmt.Printf("Accepted: %d\n", len(res.Accepted))
	fmt.Printf("Accepted by class: No1x1=%d One1x1=%d Two1x1=%d Three1x1=%d\n",
		res.AcceptedByClass[rush.InvNo1x1], res.AcceptedByClass[rush.InvOne1x1],
		res.AcceptedByClass[rush.InvTwo1x1], res.AcceptedByClass[rush.InvThree1x1])
	fmt.Printf("Total elapsed: %s\n", res.TotalElapsed)
	if len(res.Accepted) > 0 {
		avg := res.TotalElapsed / time.Duration(len(res.Accepted))
		fmt.Printf("Average per accepted: %s\n", avg)
	}
	fmt.Println("Pattern sanity:")
	for _, n := range res.PatternNotes {
		fmt.Printf("  %s\n", n)
	}
	fmt.Println("Rejected:")
	rkeys := make([]string, 0, len(res.Rejected))
	for k := range res.Rejected {
		rkeys = append(rkeys, k)
	}
	sort.Strings(rkeys)
	for _, k := range rkeys {
		fmt.Printf("  %s: %d\n", k, res.Rejected[k])
	}
	fmt.Println("\nCandidate\tFamily\tClass\t1x1\tEss\tRel\tCorr\tBaseOpt\tEnrOpt\tDelta\tReplay")
	for _, c := range res.Accepted {
		replay := "PASS"
		if !c.ReplayVerified {
			replay = "FAIL"
		}
		fmt.Printf("%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%+d\t%s\n",
			c.CandidateID, c.Base.FamilyID, c.Inventory.InventoryClass,
			c.Added1x1Count, c.Essential1x1Count, c.Relevant1x1Count, c.Corridor1x1Count,
			c.Base.BaseOptimal, c.EnrichedOptimal, c.OptimalDelta, replay)
	}
	fmt.Printf("\nWrote %s\n", cfg.OutputDir)
	if len(res.Accepted) < cfg.TargetAccepted {
		fmt.Printf("WARNING: accepted %d < target %d\n", len(res.Accepted), cfg.TargetAccepted)
	}
	if len(res.Accepted) == 0 {
		os.Exit(1)
	}
}
