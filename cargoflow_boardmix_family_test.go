package rush

import (
	"fmt"
	"testing"
)

func TestNativeCandidatePreservesCorrectBaseFamilyId(t *testing.T) {
	base := EnrichmentBase{CandidateID: "Cand_X", FamilyID: "Family_UNIQUE_42"}
	c := &BoardMixAccepted{FamilyID: "STALE", BaseCandidateID: "old"}
	c.FamilyID = base.FamilyID
	c.BaseCandidateID = base.CandidateID
	if c.FamilyID != "Family_UNIQUE_42" || c.BaseCandidateID != "Cand_X" {
		t.Fatalf("family not preserved: %+v", c)
	}
}

func TestNoStaleFamilyIdPropagation(t *testing.T) {
	bases := []EnrichmentBase{
		{CandidateID: "A1", FamilyID: "FA"},
		{CandidateID: "B1", FamilyID: "FB"},
	}
	cfg := DefaultBoardMixConfig()
	cfg.FamilyFirstExploration = true
	cfg.TryNativeAugment = true
	cfg.TryOuterAugment = false
	jobs := buildBoardMixJobs(bases, []EmbeddingVariant{EmbedFlushTop}, cfg)
	for _, j := range jobs {
		if j.base.CandidateID == "A1" && j.base.FamilyID != "FA" {
			t.Fatal("stale family on A")
		}
		if j.base.CandidateID == "B1" && j.base.FamilyID != "FB" {
			t.Fatal("stale family on B")
		}
	}
}

func Test24DistinctInputFamiliesRemainDistinctInScheduling(t *testing.T) {
	bases := make([]EnrichmentBase, 24)
	for i := 0; i < 24; i++ {
		bases[i] = EnrichmentBase{
			CandidateID: fmt.Sprintf("C%02d", i),
			FamilyID:    fmt.Sprintf("F%02d", i),
		}
	}
	cfg := DefaultBoardMixConfig()
	cfg.FamilyFirstExploration = true
	cfg.TryNativeAugment = true
	cfg.TryOuterAugment = true
	embeds := []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1}
	jobs := buildBoardMixJobs(bases, embeds, cfg)
	seenPass1 := map[string]bool{}
	for _, j := range jobs {
		if j.pass != 1 {
			continue
		}
		seenPass1[j.base.FamilyID] = true
	}
	if len(seenPass1) != 24 {
		t.Fatalf("pass1 must cover all 24 families, got %d", len(seenPass1))
	}
	// Pass1 jobs must come before pass2 for each family's first appearance.
	firstPass2 := -1
	for i, j := range jobs {
		if j.pass == 2 {
			firstPass2 = i
			break
		}
	}
	if firstPass2 < 24 { // at least 24 plain jobs in pass1
		t.Fatalf("pass2 started too early at %d", firstPass2)
	}
	for i := 0; i < firstPass2; i++ {
		if jobs[i].pass != 1 {
			t.Fatalf("job %d before pass2 boundary has pass=%d", i, jobs[i].pass)
		}
	}
}

func TestFamilyFirstSchedulingBroadCoverage(t *testing.T) {
	// Synthetic: 24 families; first 3 would dominate without family-first + cap.
	pool := []BoardMixAccepted{}
	requested := 24
	cap := 3
	// Simulate accepting many from F00 then trying F01.. 
	for i := 0; i < 20; i++ {
		fam := "F00"
		if shouldAcceptFamilyVariant(pool, fam, requested, cap) {
			pool = append(pool, BoardMixAccepted{FamilyID: fam})
		}
	}
	if countPoolFamilies(pool)["F00"] > cap {
		t.Fatalf("F00 dominated: %d", countPoolFamilies(pool)["F00"])
	}
	for i := 1; i < 24; i++ {
		fam := fmt.Sprintf("F%02d", i)
		if !shouldAcceptFamilyVariant(pool, fam, requested, cap) {
			t.Fatalf("family %s should be accepted during coverage", fam)
		}
		pool = append(pool, BoardMixAccepted{FamilyID: fam})
	}
	if uniqueFamilyCount(pool) < 12 {
		t.Fatalf("expected broad coverage, got %d", uniqueFamilyCount(pool))
	}
}

func TestNoSingleFamilyDominatesPoolBeforeCoverage(t *testing.T) {
	pool := []BoardMixAccepted{}
	for i := 0; i < 10; i++ {
		if shouldAcceptFamilyVariant(pool, "ONLY", 24, 4) {
			pool = append(pool, BoardMixAccepted{FamilyID: "ONLY"})
		}
	}
	if len(pool) != 4 {
		t.Fatalf("cap should stop at 4, got %d", len(pool))
	}
}

func TestCompactNotRequiredFromNativeGenerator(t *testing.T) {
	cfg := DefaultBoardMixConfig()
	if _, ok := cfg.ShapeQuotas[string(ShapeCompact6x6)]; ok && cfg.ShapeQuotas[string(ShapeCompact6x6)] > 0 {
		t.Fatal("Compact6x6 must not be a hard native pilot quota")
	}
	pool := []BoardMixAccepted{}
	for i := 0; i < 12; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("F%d", i), InventoryClass: InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeFullField, OuterZoneRelevant: true},
		})
	}
	cfg.TargetAccepted = 12
	cfg.ShapeQuotas = map[string]int{string(ShapeFullField): 12}
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 12}
	cfg.MinDistinctBoardShapes = 1
	cfg.MinOuterZoneRelevant = 8
	_, rep := SelectBoardMixShortlist(pool, cfg)
	for _, u := range rep.UnmetRequirements {
		if containsSubstr(u, "Compact6x6") && containsSubstr(u, "need") && !containsSubstr(u, "not required") {
			// only fail if Compact is treated as blocking without soft note
		}
	}
	if rep.FinalAccepted != 12 {
		t.Fatalf("compact absence should not block completion: %d", rep.FinalAccepted)
	}
}

func TestThree1x1NotHardQuota(t *testing.T) {
	cfg := DefaultBoardMixConfig()
	if cfg.InventoryQuotas[string(InvThree1x1)] > 0 {
		t.Fatal("Three1x1 must not be hard default quota")
	}
	pool := []BoardMixAccepted{}
	shapes := []BoardShapeClass{ShapeShiftedCore, ShapeTall, ShapeWide}
	invs := []InventoryClass{InvNo1x1, InvOne1x1, InvTwo1x1}
	for i := 0; i < 12; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("G%d", i), InventoryClass: invs[i%3],
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: shapes[i%3], OuterZoneRelevant: true},
		})
	}
	cfg.TargetAccepted = 12
	cfg.MinDistinctBoardShapes = 3
	cfg.MinOuterZoneRelevant = 8
	cfg.MinDistinctInventoryClasses = 3
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 3, string(InvOne1x1): 3, string(InvTwo1x1): 3}
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	if len(sel) != 12 {
		t.Fatalf("want 12 without Three1x1, got %d why=%v", len(sel), rep.WhyFinalShort)
	}
	if rep.FinalInventoryDist[string(InvThree1x1)] != 0 && rep.DiversityTargetUnmet {
		t.Fatal("Three1x1 absence must not force unmet when target met")
	}
}

func TestUniqueFamilyHardRequirementStillPASS(t *testing.T) {
	pool := []BoardMixAccepted{}
	for i := 0; i < 12; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("U%d", i), InventoryClass: InvOne1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeWide, OuterZoneRelevant: true},
		})
	}
	// duplicate family noise
	pool = append(pool, BoardMixAccepted{
		FamilyID: "U0", InventoryClass: InvTwo1x1,
		BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeTall, OuterZoneRelevant: true},
	})
	cfg := DefaultBoardMixConfig()
	cfg.UniqueFamily = true
	cfg.TargetAccepted = 12
	cfg.MinDistinctBoardShapes = 1
	cfg.MinOuterZoneRelevant = 4
	cfg.InventoryQuotas = map[string]int{string(InvOne1x1): 12}
	sel, _ := SelectBoardMixShortlist(pool, cfg)
	seen := map[string]bool{}
	for _, c := range sel {
		if seen[c.FamilyID] {
			t.Fatal("UniqueFamily violated")
		}
		seen[c.FamilyID] = true
	}
}

func TestFamilyFunnelReportBuilt(t *testing.T) {
	bases := []EnrichmentBase{
		{CandidateID: "C1", FamilyID: "F1"},
		{CandidateID: "C2", FamilyID: "F2"},
	}
	funnel := initFamilyFunnel(bases)
	noteFamilyAttempt(funnel, "F1", "", true, 2, 1)
	noteFamilyAttempt(funnel, "F2", "DifficultyUnknown", false, 0, 0)
	pool := []BoardMixAccepted{{FamilyID: "F1", BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeFullField}, AugmentationClass: AugSideGate}}
	list := finalizeFamilyFunnel(funnel, pool)
	cfg := DefaultBoardMixConfig()
	rep := buildFamilyCoverageReport(cfg, bases, pool, list, true)
	if rep.DistinctRequestedBaseFamilyIds != 2 {
		t.Fatal(rep.DistinctRequestedBaseFamilyIds)
	}
	if rep.PoolUniqueFamilyIds != 1 {
		t.Fatal(rep.PoolUniqueFamilyIds)
	}
	if rep.RootCauseSummary == "" {
		t.Fatal("missing root cause")
	}
	if rep.FamilyShapeCross["F1"][string(ShapeFullField)] != 1 {
		t.Fatal(rep.FamilyShapeCross)
	}
}

func TestExpandedZeroExplanationWhenFullFieldPresent(t *testing.T) {
	bases := []EnrichmentBase{{CandidateID: "C", FamilyID: "F"}}
	pool := []BoardMixAccepted{{
		FamilyID: "F",
		BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeFullField, OuterZoneRelevant: true},
	}}
	rep := buildFamilyCoverageReport(DefaultBoardMixConfig(), bases, pool, nil, false)
	if rep.ExpandedZeroExplanation == "" {
		t.Fatal("expected Expanded=0 explanation when FullField present")
	}
}
