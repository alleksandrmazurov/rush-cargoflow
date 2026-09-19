package rush

import (
	"fmt"
	"strings"
	"testing"
)

func sourceBoardWithTargetAt(left int) string {
	row := []byte("oooooo")
	row[left], row[left+1] = 'A', 'A'
	return string(row) + strings.Repeat("oooooo", 5)
}

func TestSourceTargetLeftColumnAndExpectedRows(t *testing.T) {
	for left := 0; left <= 3; left++ {
		col, err := SourceTargetLeftColumn(sourceBoardWithTargetAt(left))
		if err != nil || col != left {
			t.Fatalf("left=%d col=%d err=%v", left, col, err)
		}
		for embedding, offset := range map[EmbeddingVariant]int{
			EmbedFlushTop: 0, EmbedShiftDown1: 1, EmbedFlushBottom: 2,
		} {
			row, err := ExpectedTargetTopRow(left, embedding)
			want := 4 - left + offset
			if err != nil || row != want {
				t.Fatalf("left=%d embedding=%s row=%d want=%d err=%v",
					left, embedding, row, want, err)
			}
		}
	}
}

func TestSourceAwareEmbeddingRouting(t *testing.T) {
	embeddings := []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1, EmbedFlushBottom}
	cases := []struct {
		col  int
		want EmbeddingVariant
	}{
		{0, EmbedFlushBottom},
		{1, EmbedFlushBottom},
		{2, EmbedFlushBottom},
		{3, EmbedShiftDown1},
	}
	for _, tc := range cases {
		base := EnrichmentBase{
			CandidateID: fmt.Sprintf("C%d", tc.col),
			FamilyID:    fmt.Sprintf("F%d", tc.col),
			Level: &LevelJSON{Transplant: &TransplantJSON{
				OriginalBoard: sourceBoardWithTargetAt(tc.col),
			}},
		}
		cfg := DefaultBoardMixConfig()
		cfg.FamilyFirstExploration = true
		cfg.SourceAwareEmbeddingRouting = true
		cfg.TryOuterAugment = false
		cfg.TryNativeAugment = false
		cfg.TryCoreExpansion = false
		cfg.TryCausalSynthesis = false
		jobs := buildBoardMixJobs([]EnrichmentBase{base}, embeddings, cfg)
		if len(jobs) == 0 || jobs[0].embed != tc.want || jobs[0].pass != 1 {
			t.Fatalf("col%d jobs=%+v want primary %s", tc.col, jobs, tc.want)
		}
	}
}

func deepTargetSelectorCandidate(index, row int, causal bool) BoardMixAccepted {
	candidate := BoardMixAccepted{
		FamilyID:               fmt.Sprintf("DT%02d", index),
		TargetTopRow:           row,
		SourceTargetLeftColumn: 4 - minInt(row, 4),
		ExpectedTargetTopRow:   row,
		Embedding:              EmbedFlushBottom,
		InventoryClass: InventoryClass([]InventoryClass{
			InvNo1x1, InvOne1x1, InvTwo1x1,
		}[index%3]),
		BoardUtil: BoardUtilizationMetrics{
			BoardShapeClass:   []BoardShapeClass{ShapeTall, ShapeWide, ShapeExpanded}[index%3],
			OuterZoneRelevant: true,
		},
		ReplayVerified: true,
	}
	if causal {
		candidate.CausalTemplate = CausalTemplate([]CausalTemplate{
			CausalLower1x1SpaceMaker, CausalLowerParkingUnlock,
			CausalCrossRegionLongGate, CausalSideToLowerToUpper,
		}[index%4])
		candidate.CausalProof = &CausalProof{
			Valid: true, ReplayVerified: true, Template: candidate.CausalTemplate,
			CrossRegionDependencyEdgeCount: 2, CrossRegionDependencyDepth: 2,
			DistributedCausalityScore: float64(30 - index),
			EdgeTypeDistribution:      map[string]int{"Side→Corridor": 1},
		}
	}
	return candidate
}

func TestDeepTargetSelectorPreservesCausalQuality(t *testing.T) {
	pool := []BoardMixAccepted{}
	index := 0
	for _, row := range []int{6, 6, 6, 5, 5, 5, 5} {
		pool = append(pool, deepTargetSelectorCandidate(index, row, index < 5))
		index++
	}
	for _, row := range []int{4, 4, 4, 3, 3, 2, 2, 2, 1, 1, 1, 1, 1} {
		pool = append(pool, deepTargetSelectorCandidate(index, row, index < 12))
		index++
	}
	cfg := DefaultBoardMixConfig()
	cfg.TargetAccepted = 12
	cfg.TargetCausalExpanded = 8
	cfg.TargetGenuineCoreExpanded = 0
	cfg.TargetDeepRows5Plus = 5
	cfg.TargetBottomRow6 = 2
	cfg.TargetTopRowQuotas = map[string]int{"6": 3, "5": 4, "4": 3, "3": 1, "2": 1}
	cfg.UniqueFamily = true
	cfg.MinDistinctBoardShapes = 1
	cfg.MinDistinctInventoryClasses = 1
	cfg.MinOuterZoneRelevant = 1
	cfg.ShapeQuotas = map[string]int{}
	cfg.InventoryQuotas = map[string]int{}
	selected, report := SelectBoardMixShortlist(pool, cfg)
	if len(selected) != 12 {
		t.Fatalf("selected=%d", len(selected))
	}
	if report.FinalTargetRow5PlusCount < 5 || report.FinalTargetRow6Count < 2 {
		t.Fatalf("deep targets not met: %+v", report)
	}
	if report.FinalCausalExpanded < 8 {
		t.Fatalf("causal target regressed: %d", report.FinalCausalExpanded)
	}
	if report.DeepTargetPoolUniqueFamilies != 7 || report.BottomTargetPoolUniqueFamilies != 3 {
		t.Fatalf("pool deep reporting wrong: %+v", report)
	}
}
