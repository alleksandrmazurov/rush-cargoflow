package rush

import (
	"path/filepath"
	"testing"
)

func TestClassifyTargetDepth(t *testing.T) {
	cases := map[int]TargetDepthClass{
		1: TargetDepthShallow,
		2: TargetDepthShallow,
		3: TargetDepthMedium,
		4: TargetDepthDeep,
		5: TargetDepthDeep,
		0: TargetDepthUnknown,
		6: TargetDepthUnknown,
	}
	for row, want := range cases {
		if got := ClassifyTargetDepth(row); got != want {
			t.Fatalf("row %d: got %s want %s", row, got, want)
		}
	}
}

func TestTargetDepthMetricsOnFixture(t *testing.T) {
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
	m := ComputeTargetDepthMetrics(board, sol, budget)
	if m.TargetTopRow != board.Pieces[0].Row(board.Width) {
		t.Fatalf("target row got %d want %d", m.TargetTopRow, board.Pieces[0].Row(board.Width))
	}
	if m.ExitCorridorLength != m.TargetTopRow {
		t.Fatalf("corridor length=%d top=%d", m.ExitCorridorLength, m.TargetTopRow)
	}
	if m.AboveBelowDependencyBalance < 0 || m.AboveBelowDependencyBalance > 1 {
		t.Fatalf("balance out of range: %v", m.AboveBelowDependencyBalance)
	}
}

func TestTargetDepthCalibrationHypothesisDecision(t *testing.T) {
	rows := []TargetDepthCalibrationRow{
		{HumanLabel: "visible-6x6", TargetDepthMetrics: TargetDepthMetrics{TargetTopRow: 4, BelowTargetDependencyMoves: 2}},
		{HumanLabel: "visible-6x6", TargetDepthMetrics: TargetDepthMetrics{TargetTopRow: 4, BelowTargetDependencyMoves: 3}},
		{HumanLabel: "better-native", TargetDepthMetrics: TargetDepthMetrics{TargetTopRow: 4, BelowTargetDependencyMoves: 3}},
		{HumanLabel: "better-native", TargetDepthMetrics: TargetDepthMetrics{TargetTopRow: 3, BelowTargetDependencyMoves: 2}},
	}
	summary := summarizeTargetDepthCalibration(rows)
	if got, _ := summary["targetDepthHypothesisSupported"].(bool); got {
		t.Fatal("near-identical depth/lower use must not support hypothesis")
	}
}
