package rush

import (
	"path/filepath"
	"testing"
)

func TestBest6x6ContainmentFullBoard(t *testing.T) {
	// All meaningful cells inside a 6×6 corner ⇒ ratio 1.0
	cells := map[int]bool{}
	w, h := 7, 8
	for r := 0; r < 6; r++ {
		for c := 0; c < 6; c++ {
			cells[r*w+c] = true
		}
	}
	best, _, _ := best6x6Containment(cells, w, h)
	if best < 0.999 {
		t.Fatalf("expected ~1.0 for compact 6x6, got %v", best)
	}
	_ = h
}

func TestBest6x6ContainmentSpread(t *testing.T) {
	// Cells at opposite corners of 7×8 cannot all fit in any 6×6.
	w := 7
	cells := map[int]bool{
		0:       true, // (0,0)
		6:       true, // (0,6)
		7*w + 0: true, // (7,0)
		7*w + 6: true, // (7,6)
		3*w + 3: true,
	}
	best, _, _ := best6x6Containment(cells, w, 8)
	if best >= 0.90 {
		t.Fatalf("spread cells should not fit well in 6x6, got %v", best)
	}
	if best > 0.8 {
		t.Fatalf("expected clearly <0.8 for corner spread, got %v", best)
	}
}

func TestApplyCoreSpaceThreshold(t *testing.T) {
	m := CoreSpaceMetrics{Best6x6MeaningfulContainmentRatio: 0.85}
	ApplyCoreSpaceThreshold(&m, 0.97)
	if m.CanMeaningfulStructureFitInAny6x6 {
		t.Fatal("0.85 < 0.97 ⇒ should not fit")
	}
	ApplyCoreSpaceThreshold(&m, 0.80)
	if !m.CanMeaningfulStructureFitInAny6x6 {
		t.Fatal("0.85 >= 0.80 ⇒ should fit")
	}
}

func TestCalibrateCoreSpaceOnBatchRUSH01041(t *testing.T) {
	dir := filepath.Join("output", "RUSH01041_NativeFamilyCoveragePilot_001")
	if _, err := listCandidateIDsFromDir(filepath.Join(dir, "Candidates")); err != nil {
		t.Skip("RUSH01041 batch not present:", err)
	}
	budget := DefaultCargoFlowSolveBudget()
	budget.TimeLimit = budget.TimeLimit // keep default
	rows, summary, err := CalibrateCoreSpaceOnBatch(dir, budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 8 {
		t.Fatalf("expected >=8 candidates, got %d", len(rows))
	}
	labelled := 0
	for _, r := range rows {
		if r.HumanLabel == "visible-6x6" || r.HumanLabel == "better-native" {
			labelled++
		}
		if r.Best6x6MeaningfulContainmentRatio < 0 || r.Best6x6MeaningfulContainmentRatio > 1.001 {
			t.Fatalf("%s: bad ratio %v", r.CandidateID, r.Best6x6MeaningfulContainmentRatio)
		}
	}
	if labelled < 8 {
		t.Fatalf("expected 8 human labels, got %d", labelled)
	}
	if summary["metric"] == nil {
		t.Fatal("missing summary metric")
	}
}
