package rush

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHumanRejected001_005_FunctionallySimilarDetected(t *testing.T) {
	dir := filepath.Join("output", "pilot")
	ids := []string{"Candidate_001", "Candidate_002", "Candidate_003", "Candidate_004", "Candidate_005"}
	sigs, err := LoadHumanRejectedPilotSignatures(dir, ids)
	if err != nil {
		t.Skipf("pilot refs unavailable: %v", err)
	}
	high := 0
	pairs := 0
	for i := 0; i < len(sigs); i++ {
		for j := i + 1; j < len(sigs); j++ {
			pairs++
			s := FunctionalSimilarity(sigs[i], sigs[j])
			if s >= 0.70 {
				high++
			}
		}
	}
	if high < pairs/2 {
		t.Fatalf("expected majority of human-rejected pairs >=0.70, got %d/%d", high, pairs)
	}
	// Mean should be clearly above random (~0.5).
	var sum float64
	n := 0
	for i := 0; i < len(sigs); i++ {
		for j := i + 1; j < len(sigs); j++ {
			sum += FunctionalSimilarity(sigs[i], sigs[j])
			n++
		}
	}
	if sum/float64(n) < 0.70 {
		t.Fatalf("mean functional similarity too low: %.3f", sum/float64(n))
	}
}

func TestSameSolutionDifferentIds_HighFunctionalSimilarity(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable {
		t.Fatal(sol)
	}
	a := BuildPuzzleSignature(board, sol)
	b := board.Copy()
	for i := range b.Labels {
		b.Labels[i] = "X" + string(rune('A'+i))
	}
	c := BuildPuzzleSignature(b, sol)
	if a.SequenceDigest != c.SequenceDigest {
		t.Fatalf("sequence changed after relabel: %v vs %v", a.SequenceDigest, c.SequenceDigest)
	}
	if FunctionalSimilarity(a, c) < 0.95 {
		t.Fatalf("relabel should keep high functional similarity, got %.3f", FunctionalSimilarity(a, c))
	}
}

func TestDifferentDependencyPattern_LowerFunctionalSimilarity(t *testing.T) {
	easy, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	easySol := easy.Solve()
	easySig := BuildPuzzleSignature(easy, easySol)

	dir := filepath.Join("output", "pilot")
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, "Candidate_001.json"))
	if err != nil {
		t.Skip(err)
	}
	hard, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	hardSol := hard.SolveWithBudget(SolveBudget{TimeLimit: 15 * time.Second, MaxVisited: 4_000_000})
	if !hardSol.Solvable {
		t.Skip("hard unsolved")
	}
	hardSig := BuildPuzzleSignature(hard, hardSol)
	sim := FunctionalSimilarity(easySig, hardSig)
	if sim >= 0.85 {
		t.Fatalf("trivial vs RUSH005-001 should not be near-identical: %.3f", sim)
	}
}

func TestSameInventoryDifferentLogic_NotAutomaticallyIdentical(t *testing.T) {
	// Two signatures with same inventory string but different sequences.
	a := PuzzleSignature{
		InventorySignature:           "1x1:4,Target:1",
		OptimalSolutionClassSequence: []string{"1x1-left-1", "Target-exit"},
		MovedPieceClassHistogram:     map[string]int{"1x1": 1},
		DependencyDepth:              1,
		InitialTargetBlockerCount:    1,
		DirectTargetBlockerClasses:   []string{"1x1"},
		OptimalGestures:              2,
	}
	b := PuzzleSignature{
		InventorySignature:           "1x1:4,Target:1",
		OptimalSolutionClassSequence: []string{"1x2H-right-2", "1x3V-down-1", "1x2H-left-1", "Target-up-1", "Target-exit"},
		MovedPieceClassHistogram:     map[string]int{"1x2H": 2, "1x3V": 1},
		DependencyDepth:              4,
		InitialTargetBlockerCount:    2,
		DirectTargetBlockerClasses:   []string{"1x2H", "1x3V"},
		OptimalGestures:              5,
		TargetMoveCount:              2,
		NonTargetMovesBeforeFirstTargetMove: 3,
	}
	if FunctionalSimilarity(a, b) >= 0.72 {
		t.Fatalf("different logic should stay below functional threshold, got %.3f", FunctionalSimilarity(a, b))
	}
}

func TestArchetypeConstraintsValidated(t *testing.T) {
	sig := PuzzleSignature{
		InitialTargetBlockerCount:  2,
		DirectTargetBlockerClasses: []string{"1x2H", "1x1"},
		MovedPieceClassHistogram:   map[string]int{"1x2H": 1, "1x1": 1},
		DependencyDepth:            4,
		DistinctNonTargetMovedPieces: 3,
	}
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 5*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	if ValidateArchetype(ArchetypeCrossLock, sig, board) != "" {
		t.Fatal("crosslock should pass")
	}
	if ValidateArchetype(ArchetypeSideChain, sig, board) != "" {
		t.Fatal("sidechain should pass")
	}
}

func TestArchetypeMismatchRejected(t *testing.T) {
	sig := PuzzleSignature{
		InitialTargetBlockerCount:  1,
		DirectTargetBlockerClasses: []string{"1x1"},
		MovedPieceClassHistogram:   map[string]int{"1x1": 1},
		DependencyDepth:            1,
	}
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 5*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	if ValidateArchetype(ArchetypeCrossLock, sig, board) != RejectArchetypeMismatch {
		t.Fatal("expected mismatch")
	}
	if ValidateArchetype(ArchetypeSideChain, sig, board) != RejectArchetypeMismatch {
		t.Fatal("expected sidechain mismatch")
	}
}

func TestBatchContainsMultipleArchetypes(t *testing.T) {
	cfg := DefaultDiverseCargoFlowGenerationConfig(777)
	cfg.TargetAccepted = 4
	cfg.MaxAttempts = 80
	cfg.MaxPerArchetype = 2
	cfg.SolveTimeLimit = 2 * time.Second
	cfg.MaxVisitedStates = 800_000
	cfg.HumanRejectedSignatures = nil
	res := NewCargoFlowGenerator(cfg).Generate()
	if len(res.Accepted) == 0 {
		t.Skip("no accepted under light budget")
	}
	set := map[PuzzleArchetype]bool{}
	for _, c := range res.Accepted {
		set[c.Archetype] = true
	}
	if len(set) < 2 && len(res.Accepted) >= 2 {
		t.Fatalf("expected multiple archetypes, got %v", set)
	}
}

func TestPerArchetypeQuotaRespected(t *testing.T) {
	cfg := DefaultDiverseCargoFlowGenerationConfig(12)
	cfg.TargetAccepted = 6
	cfg.MaxAttempts = 60
	cfg.MaxPerArchetype = 1
	cfg.SolveTimeLimit = 1500 * time.Millisecond
	cfg.MaxVisitedStates = 500_000
	cfg.HumanRejectedSignatures = nil
	res := NewCargoFlowGenerator(cfg).Generate()
	counts := map[PuzzleArchetype]int{}
	for _, c := range res.Accepted {
		counts[c.Archetype]++
		if counts[c.Archetype] > 1 {
			t.Fatalf("quota exceeded for %s", c.Archetype)
		}
	}
}

func TestHumanRejectedReferenceFilterWorks(t *testing.T) {
	dir := filepath.Join("output", "pilot")
	sigs, err := LoadHumanRejectedPilotSignatures(dir, []string{"Candidate_001", "Candidate_002"})
	if err != nil {
		t.Skip(err)
	}
	cfg := DefaultDiverseCargoFlowGenerationConfig(99)
	cfg.HumanRejectedSignatures = sigs
	cfg.HumanRejectedFunctionalThreshold = 0.50 // aggressive for test
	cfg.TargetAccepted = 1
	cfg.MaxAttempts = 5
	cfg.SolveTimeLimit = 800 * time.Millisecond
	cfg.MaxVisitedStates = 300_000
	// Force SmallBlockShuttle only — closest to human-rejected pattern.
	cfg.Archetypes = []PuzzleArchetype{ArchetypeSmallBlockShuttle}
	res := NewCargoFlowGenerator(cfg).Generate()
	if res.Rejected[RejectTooSimilarToHumanRejectedPilot] == 0 && len(res.Accepted) > 0 {
		// May accept if somehow dissimilar; ensure filter path exists by checking threshold logic.
		s := FunctionalSimilarity(res.Accepted[0].Signature, sigs[0])
		if s >= 0.50 {
			t.Fatal("accepted candidate above human-rejected threshold without reject")
		}
	}
}

func TestExactComplexityFloorStillWorks(t *testing.T) {
	cfg := DefaultDiverseCargoFlowGenerationConfig(5)
	cfg.TargetAccepted = 1
	cfg.MaxAttempts = 40
	cfg.MinOptimalGestures = 10
	cfg.SolveTimeLimit = 2 * time.Second
	cfg.MaxVisitedStates = 800_000
	cfg.HumanRejectedSignatures = nil
	res := NewCargoFlowGenerator(cfg).Generate()
	for _, c := range res.Accepted {
		if c.OptimalGestures < 10 {
			t.Fatalf("below floor: %d", c.OptimalGestures)
		}
	}
}

func TestReplayStillPASS(t *testing.T) {
	cfg := DefaultDiverseCargoFlowGenerationConfig(8)
	cfg.TargetAccepted = 1
	cfg.MaxAttempts = 40
	cfg.SolveTimeLimit = 2 * time.Second
	cfg.MaxVisitedStates = 800_000
	cfg.HumanRejectedSignatures = nil
	res := NewCargoFlowGenerator(cfg).Generate()
	if len(res.Accepted) == 0 {
		t.Skip("no accepted")
	}
	c := res.Accepted[0]
	b, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Replay(c.Solution.Moves); err != nil {
		t.Fatal(err)
	}
	if !c.ReplayVerified {
		t.Fatal("ReplayVerified")
	}
}
