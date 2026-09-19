package rush

import (
	"path/filepath"
	"testing"
)

func TestCellDistribution(t *testing.T) {
	row, col, rowSpan, colSpan := cellDistribution([]int{0, 6, 7 * 7, 7*7 + 6}, 7)
	if row != 3.5 || col != 3 || rowSpan != 8 || colSpan != 7 {
		t.Fatalf("got centroid=(%v,%v) span=(%d,%d)", row, col, rowSpan, colSpan)
	}
}

func TestSpatialDependencyMetricsFixture(t *testing.T) {
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join("testdata", "cargoflow", "levels", "level_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	budget := DefaultCargoFlowSolveBudget()
	sol := board.SolveWithBudget(budget)
	if !sol.Solvable {
		t.Fatal("fixture must solve")
	}
	m := ComputeSpatialDependencyMetrics(board, sol, budget)
	if len(m.DependencyDensityByRow) != 8 || len(m.DependencyDensityByColumn) != 7 {
		t.Fatalf("density dimensions rows=%d cols=%d", len(m.DependencyDensityByRow), len(m.DependencyDensityByColumn))
	}
	if len(m.OptimalMoveActivityByRow) != 8 || len(m.OptimalMoveActivityByColumn) != 7 {
		t.Fatalf("activity dimensions rows=%d cols=%d", len(m.OptimalMoveActivityByRow), len(m.OptimalMoveActivityByColumn))
	}
	if m.DependencyGraphNodeCount != len(sol.Moves) {
		t.Fatalf("graph nodes=%d moves=%d", m.DependencyGraphNodeCount, len(sol.Moves))
	}
	if m.DependencyGraphDepth < 1 {
		t.Fatalf("graph depth=%d", m.DependencyGraphDepth)
	}
}

func TestRushSourceSpatialMetricsFixture(t *testing.T) {
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join("testdata", "cargoflow", "levels", "level_01.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Generic fixture may have no transplant provenance; absence must be explicit and safe.
	m := ComputeRushSourceSpatialMetrics(level, nil, DefaultCargoFlowSolveBudget())
	if level.Transplant == nil && m.Available {
		t.Fatal("source metrics should be unavailable without transplant")
	}
}

func TestSpatialSeparatorsRanked(t *testing.T) {
	compact := []SpatialDependencyRow{
		{Spatial: SpatialDependencyMetrics{CrossTargetDependencies: 0, DependencyGraphDepth: 1}},
		{Spatial: SpatialDependencyMetrics{CrossTargetDependencies: 0, DependencyGraphDepth: 2}},
	}
	native := []SpatialDependencyRow{
		{Spatial: SpatialDependencyMetrics{CrossTargetDependencies: 3, DependencyGraphDepth: 4}},
		{Spatial: SpatialDependencyMetrics{CrossTargetDependencies: 2, DependencyGraphDepth: 5}},
	}
	got := rankSpatialSeparators(compact, native)
	if len(got) == 0 || got[0].StandardizedGap <= 0 {
		t.Fatalf("unexpected separators: %+v", got)
	}
}

func TestVariantClassification(t *testing.T) {
	if got := classifyVariant("", ""); got != "Plain" {
		t.Fatal(got)
	}
	if got := classifyVariant("", string(AugSideGate)); got != "NativeAugment" {
		t.Fatal(got)
	}
	if got := classifyVariant(string(CoreExpDependencyPullLeft), string(AugNone)); got != "CoreExpansion" {
		t.Fatal(got)
	}
}
