package rush

import (
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
		t.Fatalf("want 12, got %d", len(sel))
	}
	// Must NOT be first 12 ShiftedCore No1x1.
	shiftedNo := 0
	for _, c := range sel {
		if c.BoardUtil.BoardShapeClass == ShapeShiftedCore && c.InventoryClass == InvNo1x1 {
			shiftedNo++
		}
	}
	if shiftedNo > 4 {
		t.Fatalf("selector collapsed to ShiftedCore/No1x1: %d/12 (pool report=%+v)", shiftedNo, rep.FinalShapeDist)
	}
	if rep.DistinctBoardShapes < 3 {
		t.Fatalf("distinct shapes %d", rep.DistinctBoardShapes)
	}
}

func TestBoardShapeQuotaApplied(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MaxBoardShapeFraction = 0.40
	sel, _ := SelectBoardMixShortlist(pool, cfg)
	counts := map[BoardShapeClass]int{}
	for _, c := range sel {
		counts[c.BoardUtil.BoardShapeClass]++
	}
	for shape, n := range counts {
		if n > 4 {
			t.Fatalf("%s count %d exceeds max 4", shape, n)
		}
	}
}

func TestInventoryQuotaApplied(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MaxInventoryClassFraction = 0.40
	sel, rep := SelectBoardMixShortlist(pool, cfg)
	for inv, n := range rep.FinalInventoryDist {
		if n > 4 {
			t.Fatalf("%s count %d exceeds max 4 (selected=%d)", inv, n, len(sel))
		}
	}
	if len(rep.FinalInventoryDist) < 3 {
		t.Fatalf("expected multiple inventory classes, got %v", rep.FinalInventoryDist)
	}
}

func TestMinDistinctBoardShapesPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.MinDistinctBoardShapes = 3
	_, rep := SelectBoardMixShortlist(pool, cfg)
	if rep.DistinctBoardShapes < 3 {
		t.Fatalf("got %d", rep.DistinctBoardShapes)
	}
}

func TestOuterZoneRelevantQuotaPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.MinOuterZoneRelevant = 4
	_, rep := SelectBoardMixShortlist(pool, cfg)
	if rep.FinalOuterZoneRelevant < 4 {
		t.Fatalf("outer=%d", rep.FinalOuterZoneRelevant)
	}
}

func TestUniqueFamilyPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	cfg.UniqueFamily = true
	sel, _ := SelectBoardMixShortlist(pool, cfg)
	seen := map[string]bool{}
	for _, c := range sel {
		if seen[c.FamilyID] {
			t.Fatalf("duplicate family %s", c.FamilyID)
		}
		seen[c.FamilyID] = true
	}
}

func TestQuotaRelaxationReportedPASS(t *testing.T) {
	// Only ShiftedCore available → relaxation notes expected.
	pool := []BoardMixAccepted{}
	for i := 0; i < 15; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID:       string(rune('A'+i)),
			InventoryClass: InvNo1x1,
			BoardUtil:      BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore},
			SelectionBand:  "medium",
		})
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.MinDistinctBoardShapes = 3
	cfg.MinOuterZoneRelevant = 4
	_, rep := SelectBoardMixShortlist(pool, cfg)
	if !rep.DiversityTargetUnmet {
		t.Fatal("expected DiversityTargetUnmet when pool lacks shapes")
	}
	if len(rep.DiversityUnmetReasons) == 0 {
		t.Fatal("expected unmet reasons")
	}
}

func TestSameSeedSameSelectionPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	cfg := DefaultBoardMixConfig()
	a, _ := SelectBoardMixShortlist(pool, cfg)
	b, _ := SelectBoardMixShortlist(pool, cfg)
	if len(a) != len(b) {
		t.Fatal("length mismatch")
	}
	for i := range a {
		if a[i].FamilyID != b[i].FamilyID || a[i].InventoryClass != b[i].InventoryClass {
			t.Fatalf("nondeterministic at %d", i)
		}
	}
}

func TestCandidatePoolDistributionReportedPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	_, rep := SelectBoardMixShortlist(pool, DefaultBoardMixConfig())
	if rep.PoolSize != len(pool) || len(rep.PoolShapeDist) == 0 || len(rep.PoolInventoryDist) == 0 {
		t.Fatalf("pool distribution missing: %+v", rep)
	}
}

func TestFinalDistributionReportedPASS(t *testing.T) {
	pool := syntheticDiversityPool()
	_, rep := SelectBoardMixShortlist(pool, DefaultBoardMixConfig())
	if len(rep.FinalShapeDist) == 0 || len(rep.FinalInventoryDist) == 0 {
		t.Fatal("final distribution missing")
	}
}

func syntheticDiversityPool() []BoardMixAccepted {
	pool := []BoardMixAccepted{}
	// 20 ShiftedCore No1x1 (tempting first-N trap)
	for i := 0; i < 20; i++ {
		pool = append(pool, BoardMixAccepted{
			FamilyID:       "S" + string(rune('A'+i)),
			InventoryClass: InvNo1x1,
			BoardUtil: BoardUtilizationMetrics{
				BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: false,
			},
			SelectionBand: "medium",
			Embedding:     EmbedShiftDown1,
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
