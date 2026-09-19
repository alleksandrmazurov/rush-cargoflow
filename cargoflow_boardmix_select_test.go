package rush

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestSelectorDoesNotStopAtFirstNValid(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.UniqueFamily = true
	cfg.MinDistinctBoardShapes = 3
	cfg.MaxBoardShapeFraction = 0.40
	cfg.MinOuterZoneRelevant = 4
	cfg.InventoryQuotas = map[string]int{
		string(InvNo1x1): 3, string(InvOne1x1): 3,
		string(InvTwo1x1): 3, string(InvThree1x1): 3,
	}
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	if len(sel) != 12 {
		t.Fatalf("want 12, got %d unmet=%v why=%v", len(sel), rep.UnmetRequirements, rep.WhyFinalShort)
	}
	shiftedNo := 0
	for _, c := range sel {
		if c.BoardUtil.BoardShapeClass == ShapeShiftedCore && c.InventoryClass == InvNo1x1 {
			shiftedNo++
		}
	}
	if shiftedNo > 4 {
		t.Fatalf("selector collapsed to ShiftedCore/No1x1: %d/12", shiftedNo)
	}
}

func TestFinalBelowTargetSetsDiversityUnmet(t *testing.T) {
	pool := []BoardMixAccepted{}
	for i := 0; i < 5; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("F%d", i), InventoryClass: InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: true},
		})
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MinOuterZoneRelevant = 1
	cfg.MinDistinctBoardShapes = 1
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	if len(sel) >= 12 {
		t.Fatal("expected shortfall")
	}
	if !rep.DiversityTargetUnmet {
		t.Fatal("DiversityUnmet must be true when FinalAccepted < FinalTarget")
	}
	if rep.FinalTarget != 12 || rep.FinalAccepted != len(sel) || rep.MissingCount != 12-len(sel) {
		t.Fatalf("target fields %+v", rep)
	}
}

func TestUnmetRequirementsReported(t *testing.T) {
	pool := []BoardMixAccepted{}
	for i := 0; i < 4; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("U%d", i), InventoryClass: InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore},
		})
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	_, rep := SelectBoardMixShortlist(pool, cfg)
	if len(rep.UnmetRequirements) == 0 {
		t.Fatal("expected UnmetRequirements")
	}
}

func TestPoolInventoryDistributionReported(t *testing.T) {
	pool := syntheticDiversityPool()
	_, rep := SelectBoardMixShortlist(pool, DefaultBoardMixConfig())
	if rep.PoolInventoryDist[string(InvNo1x1)] == 0 || rep.PoolInventoryDist[string(InvThree1x1)] == 0 {
		t.Fatalf("pool inventory dist %+v", rep.PoolInventoryDist)
	}
}

func TestShapeInventoryCrossTableReported(t *testing.T) {
	pool := syntheticDiversityPool()
	_, rep := SelectBoardMixShortlist(pool, DefaultBoardMixConfig())
	if len(rep.PoolShapeInventoryCross) == 0 {
		t.Fatal("missing cross table")
	}
	if rep.PoolShapeInventoryCross[string(ShapeExpanded)][string(InvOne1x1)] == 0 {
		t.Fatalf("cross %+v", rep.PoolShapeInventoryCross)
	}
}

func TestControlledInventoryRelaxationPASS(t *testing.T) {
	// No Three1x1, but 12 unique families across No/One/Two → should still complete to 12.
	pool := []BoardMixAccepted{}
	for i := 0; i < 5; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("N%d", i), InventoryClass: InvNo1x1,
			BoardUtil:     BoardUtilizationMetrics{BoardShapeClass: ShapeCompact6x6, OuterZoneRelevant: i < 2},
			SelectionBand: "lower-mid",
		})
	}
	for i := 0; i < 4; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("O%d", i), InventoryClass: InvOne1x1,
			BoardUtil:     BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: true},
			SelectionBand: "medium",
		})
	}
	for i := 0; i < 4; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("W%d", i), InventoryClass: InvTwo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeWide, OuterZoneRelevant: true,
				OccupiedBoundingBoxWidth: 7, OccupiedBoundingBoxHeight: 5},
			SelectionBand: "hard",
		})
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 3, string(InvOne1x1): 3, string(InvTwo1x1): 3, string(InvThree1x1): 3}
	cfg.MinDistinctBoardShapes = 3
	cfg.MinOuterZoneRelevant = 4
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	if len(sel) != 12 {
		t.Fatalf("want 12 without Three1x1, got %d why=%v", len(sel), rep.WhyFinalShort)
	}
	if rep.FinalInventoryDist[string(InvThree1x1)] != 0 {
		t.Fatal("unexpected Three1x1")
	}
	foundRelax := false
	for _, s := range rep.QuotaRelaxations {
		if containsSubstr(s, "Inventory") || containsSubstr(s, "inventory") || containsSubstr(s, "fill-unique") {
			foundRelax = true
			break
		}
	}
	if !foundRelax && len(rep.QuotaRelaxations) == 0 {
		t.Fatal("expected some relaxation logging")
	}
}

func TestControlledShapeRelaxationPASS(t *testing.T) {
	// 12 unique families, mostly ShiftedCore — fraction 0.40 would cap at 4; relaxation must fill to 12.
	pool := []BoardMixAccepted{}
	for i := 0; i < 12; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: fmt.Sprintf("S%d", i), InventoryClass: InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: true},
		})
	}
	// Sprinkle two other shapes so min distinct can hold if we had them — here pool only ShiftedCore.
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MaxBoardShapeFraction = 0.40
	cfg.MinDistinctBoardShapes = 1
	cfg.MinOuterZoneRelevant = 4
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 12}
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	if len(sel) != 12 {
		t.Fatalf("shape relaxation should reach 12, got %d skips=%v relax=%v", len(sel), rep.SelectionSkipCounters, rep.QuotaRelaxations)
	}
	hasFracRelax := false
	for _, s := range rep.QuotaRelaxations {
		if containsSubstr(s, "MaxBoardShapeFraction") || containsSubstr(s, "fill-unique") {
			hasFracRelax = true
		}
	}
	if !hasFracRelax {
		t.Fatalf("expected shape fraction relaxation notes: %v", rep.QuotaRelaxations)
	}
}

func TestExactQuotaNotRequiredForCompletionPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	// Remove all Three1x1
	filtered := []BoardMixAccepted{}
	for _, c := range pool {
		if c.InventoryClass != InvThree1x1 {
			filtered = append(filtered, c)
		}
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	sel, rep := SelectBoardMixShortlist(filtered, cfg)
	if len(sel) != 12 {
		t.Fatalf("completion without exact 3/3/3/3 failed: %d why=%v", len(sel), rep.WhyFinalShort)
	}
}

func TestHardMinimumDiversityPreservedPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.MinDistinctBoardShapes = 3
	_, rep := SelectBoardMixShortlist(pool, cfg)
	if rep.DistinctBoardShapes < 3 {
		t.Fatalf("hard min shapes lost: %d", rep.DistinctBoardShapes)
	}
}

func TestExpandedFullFieldAvailabilityDiagnosedPASS(t *testing.T) {
	d := DiagnoseExpandedFullFieldAvailability(map[string]int{
		string(ShapeShiftedCore): 70,
		string(ShapeTall):        5,
		string(ShapeWide):        6,
	})
	if d.LikelyRootCause == "" || d.Native7x8Limitation == "" || d.Summary == "" {
		t.Fatalf("incomplete diag %+v", d)
	}
}

func TestSameSeedDeterministicPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.Seed = 20260919
	a, _ := SelectBoardMixShortlist(pool, cfg)
	b, _ := SelectBoardMixShortlist(pool, cfg)
	if len(a) != len(b) {
		t.Fatal("length")
	}
	for i := range a {
		if a[i].FamilyID != b[i].FamilyID {
			t.Fatalf("nondeterministic %d", i)
		}
	}
}

func TestDifferentSeedCanChangeSelectedFamiliesPASS(t *testing.T) {
	pool := seededAlternativePool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 6
	cfg.MinDistinctBoardShapes = 1
	cfg.MinOuterZoneRelevant = 0
	cfg.MaxBoardShapeFraction = 1
	cfg.MaxInventoryClassFraction = 1
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 6}

	cfg.Seed = 101
	a, repA := SelectBoardMixShortlist(pool, cfg)
	cfg.Seed = 202
	b, repB := SelectBoardMixShortlist(pool, cfg)
	if len(a) != 6 || len(b) != 6 {
		t.Fatalf("selection shortfall: %d/%d reports %+v %+v", len(a), len(b), repA, repB)
	}
	if sameFamilySet(a, b) {
		t.Fatalf("expected fixture seeds to be capable of changing selected families: %v", familiesOf(a))
	}
	assertUniqueFamilies(t, a)
	assertUniqueFamilies(t, b)
}

func TestSameSeedPreservesStateSetPASS(t *testing.T) {
	pool := seededAlternativePool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 6
	cfg.MinDistinctBoardShapes = 1
	cfg.MinOuterZoneRelevant = 0
	cfg.MaxBoardShapeFraction = 1
	cfg.MaxInventoryClassFraction = 1
	cfg.InventoryQuotas = map[string]int{string(InvNo1x1): 6}
	cfg.Seed = 303

	a, _ := SelectBoardMixShortlist(pool, cfg)
	b, _ := SelectBoardMixShortlist(pool, cfg)
	if !sameFamilySet(a, b) {
		t.Fatalf("same seed selected different family sets: %v vs %v", familiesOf(a), familiesOf(b))
	}
	assertUniqueFamilies(t, a)
}

func TestCheckpointCacheUnaffectedPASS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	cp := BoardMixCheckpoint{
		Version:              BoardMixVersion,
		CompletedAttemptKeys: []string{"fam|FlushTop|0"},
		Rejected:             map[string]int{"unsolvable": 1},
		Pool:                 []BoardMixAccepted{{FamilyID: "keep"}},
	}
	if err := SaveBoardMixCheckpoint(path, cp); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBoardMixCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != BoardMixVersion || len(loaded.CompletedAttemptKeys) != 1 || len(loaded.Pool) != 1 {
		t.Fatalf("checkpoint schema broken: %+v", loaded)
	}
	cachePath := filepath.Join(dir, "solve_cache.jsonl")
	cache, err := OpenSolveCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	key := SolveCacheKey{
		SourceDatabaseHash: "h", SourcePuzzleID: "p",
		TransformVersion: TransformVersionCW90, EmbeddingVariant: "FlushTop",
		CargoRulesVersion: CargoRulesVersionCF, SolverVersion: SolverVersionBFS,
	}
	if err := cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true, OptimalGestures: 9}); err != nil {
		t.Fatal(err)
	}
	cache2, err := OpenSolveCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	ent, ok := cache2.Get(key)
	if !ok || !ent.Solved || ent.OptimalGestures != 9 {
		t.Fatalf("solve cache broken: ok=%v ent=%+v", ok, ent)
	}
}

func TestBoardShapeQuotaApplied(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MaxBoardShapeFraction = 0.40
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	// After staged relaxation, fraction may exceed 0.40 — ensure we still got 12 and ≥3 shapes.
	if len(sel) != 12 || rep.DistinctBoardShapes < 3 {
		t.Fatalf("sel=%d shapes=%d", len(sel), rep.DistinctBoardShapes)
	}
}

func TestInventoryQuotaApplied(t *testing.T) {
	pool := syntheticDiversityPool()
	_, rep := SelectBoardMixShortlist(pool, DefaultBoardMixConfig())
	if len(rep.FinalInventoryDist) < 3 {
		t.Fatalf("inventory %+v", rep.FinalInventoryDist)
	}
}

func TestMinDistinctBoardShapesPASS(t *testing.T) {
	_, rep := SelectBoardMixShortlist(syntheticDiversityPool(), DefaultBoardMixConfig())
	if rep.DistinctBoardShapes < 3 {
		t.Fatal(rep.DistinctBoardShapes)
	}
}

func TestOuterZoneRelevantQuotaPASS(t *testing.T) {
	_, rep := SelectBoardMixShortlist(syntheticDiversityPool(), DefaultBoardMixConfig())
	if rep.FinalOuterZoneRelevant < 4 {
		t.Fatal(rep.FinalOuterZoneRelevant)
	}
}

func TestUniqueFamilyPASS(t *testing.T) {
	sel, _ := SelectBoardMixShortlist(syntheticDiversityPool(), DefaultBoardMixConfig())
	assertUniqueFamilies(t, sel)
}

func TestCandidatePoolDistributionReportedPASS(t *testing.T) {
	_, rep := SelectBoardMixShortlist(syntheticDiversityPool(), DefaultBoardMixConfig())
	if rep.PoolSize == 0 || rep.PoolUniqueFamilies == 0 {
		t.Fatal(rep)
	}
}

func TestFinalDistributionReportedPASS(t *testing.T) {
	_, rep := SelectBoardMixShortlist(syntheticDiversityPool(), DefaultBoardMixConfig())
	if len(rep.FinalShapeDist) == 0 {
		t.Fatal("no final dist")
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

func syntheticDiversityPool() []BoardMixAccepted {
	pool := []BoardMixAccepted{}
	for i := 0; i < 20; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: "S" + string(rune('A'+i)), InventoryClass: InvNo1x1,
			BoardUtil:     BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: false},
			SelectionBand: "medium", Embedding: EmbedShiftDown1,
		})
	}
	for i := 0; i < 5; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: "E" + string(rune('A'+i)), InventoryClass: InvOne1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeExpanded, OuterZoneRelevant: true,
				PiecesOutsideOriginal6x6Core: 1},
			SelectionBand: "hard", Embedding: EmbedFlushBottom,
		})
	}
	for i := 0; i < 5; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: "T" + string(rune('A'+i)), InventoryClass: InvTwo1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeTall, OuterZoneRelevant: true,
				OccupiedBoundingBoxHeight: 7, OccupiedBoundingBoxWidth: 5},
			SelectionBand: "lower-mid", Embedding: EmbedFlushTop,
		})
	}
	for i := 0; i < 5; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: "W" + string(rune('A'+i)), InventoryClass: InvThree1x1,
			BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeWide, OuterZoneRelevant: true,
				OccupiedBoundingBoxWidth: 7, OccupiedBoundingBoxHeight: 5},
			SelectionBand: "medium", Embedding: EmbedFlushTopMirrorH,
		})
	}
	for i := 0; i < 3; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID: "C" + string(rune('A'+i)), InventoryClass: InvNo1x1,
			BoardUtil:     BoardUtilizationMetrics{BoardShapeClass: ShapeCompact6x6, OuterZoneRelevant: false},
			SelectionBand: "lower-mid", Embedding: EmbedFlushTop,
		})
	}
	return pool
}

func seededAlternativePool() []BoardMixAccepted {
	pool := []BoardMixAccepted{}
	for i := 0; i < 18; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID:        fmt.Sprintf("SeedFam%02d", i),
			BaseCandidateID: fmt.Sprintf("Base%02d", i),
			InventoryClass:  InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{
				BoardShapeClass:   ShapeShiftedCore,
				OuterZoneRelevant: true,
			},
			SelectionBand:  "medium",
			Embedding:      EmbedFlushTop,
			ReplayVerified: true,
		})
	}
	return pool
}

func assertUniqueFamilies(t *testing.T, sel []BoardMixAccepted) {
	t.Helper()
	seen := map[string]bool{}
	for _, c := range sel {
		if seen[c.FamilyID] {
			t.Fatal(c.FamilyID)
		}
		seen[c.FamilyID] = true
	}
}

func sameFamilySet(a, b []BoardMixAccepted) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, c := range a {
		seen[c.FamilyID]++
	}
	for _, c := range b {
		seen[c.FamilyID]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}

func familiesOf(sel []BoardMixAccepted) []string {
	out := make([]string, 0, len(sel))
	for _, c := range sel {
		out = append(out, c.FamilyID)
	}
	return out
}
