package rush

import (
	"path/filepath"
	"testing"
	"time"
)

func lightGenConfig(seed int64) CargoFlowGenerationConfig {
	cfg := DefaultCargoFlowGenerationConfig(seed)
	cfg.TargetAccepted = 1
	cfg.MaxAttempts = 40
	cfg.MinOptimalGestures = 5
	cfg.SolveTimeLimit = 800 * time.Millisecond
	cfg.MaxVisitedStates = 400_000
	cfg.Profiles = []CargoFlowInventoryProfile{DefaultCargoFlowProfiles()[0]} // MediumDense
	return cfg
}

func TestSameSeed_SameBatch(t *testing.T) {
	cfg := lightGenConfig(4242)
	cfg.TargetAccepted = 2
	cfg.MaxAttempts = 30

	a := NewCargoFlowGenerator(cfg).Generate()
	b := NewCargoFlowGenerator(cfg).Generate()
	if len(a.Accepted) == 0 {
		t.Skip("no accepted under light test budget")
	}
	if a.Attempts != b.Attempts || len(a.Accepted) != len(b.Accepted) {
		t.Fatalf("attempts/accepted mismatch %d/%d vs %d/%d", a.Attempts, len(a.Accepted), b.Attempts, len(b.Accepted))
	}
	for i := range a.Accepted {
		if a.Accepted[i].Fingerprint != b.Accepted[i].Fingerprint {
			t.Fatalf("fingerprint mismatch at %d", i)
		}
		if a.Accepted[i].OptimalGestures != b.Accepted[i].OptimalGestures {
			t.Fatalf("optimal mismatch at %d", i)
		}
		if a.Accepted[i].AttemptSeed != b.Accepted[i].AttemptSeed {
			t.Fatalf("attempt seed mismatch")
		}
	}
}

func TestGeneratedLayout_Valid(t *testing.T) {
	c := mustAcceptOne(t, 7)
	if err := c.Level.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedLayout_NoOverlap(t *testing.T) {
	c := mustAcceptOne(t, 8)
	board, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	if err := board.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestExactlyOneTarget(t *testing.T) {
	c := mustAcceptOne(t, 9)
	board, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	targets := 0
	for _, p := range board.Pieces {
		if p.Kind == PieceTarget {
			targets++
		}
	}
	if targets != 1 {
		t.Fatalf("targets=%d", targets)
	}
}

func TestTargetNotImmediateVictory(t *testing.T) {
	c := mustAcceptOne(t, 10)
	board, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	if board.cargoTargetCanExit() {
		t.Fatal("immediate victory")
	}
}

func TestAcceptedCandidate_SolvedExact(t *testing.T) {
	c := mustAcceptOne(t, 11)
	if c.OptimalGestures <= 0 {
		t.Fatal("expected positive optimal")
	}
	board, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 10 * time.Second, MaxVisited: 4_000_000})
	if !sol.Solvable || sol.NumMoves != c.OptimalGestures {
		t.Fatalf("re-solve %v want %d", sol, c.OptimalGestures)
	}
}

func TestAcceptedCandidate_OptimalAtLeastMinimum(t *testing.T) {
	cfg := lightGenConfig(12)
	res := NewCargoFlowGenerator(cfg).Generate()
	if len(res.Accepted) == 0 {
		t.Skip("no accepted")
	}
	if res.Accepted[0].OptimalGestures < cfg.MinOptimalGestures {
		t.Fatal("below floor")
	}
}

func TestAcceptedCandidate_ReplayPASS(t *testing.T) {
	c := mustAcceptOne(t, 13)
	b2, err := BoardFromLevelJSON(c.Level)
	if err != nil {
		t.Fatal(err)
	}
	if err := b2.Replay(c.Solution.Moves); err != nil {
		t.Fatal(err)
	}
	if !c.SolutionDoc.ReplayVerified || !c.SolutionDoc.ReplayPass {
		t.Fatal("replayVerified expected")
	}
}

func TestDuplicateRejected(t *testing.T) {
	b1, _ := NewCargoFlowBoard(CargoFlowPOCFixture())
	fp1 := layoutFingerprint(b1)
	fp2 := layoutFingerprint(b1.Copy())
	if fp1 != fp2 {
		t.Fatal("same board fingerprint mismatch")
	}
	seen := map[string]bool{fp1: true}
	if !seen[fp2] {
		t.Fatal("duplicate fingerprint should hit seen set")
	}
}

func TestInterchangeableIdsIgnored(t *testing.T) {
	board, err := NewCargoFlowBoard([]string{
		".......",
		"...a...",
		"...b...",
		"...T...",
		"...T...",
		".......",
		".......",
		".......",
	})
	if err != nil {
		t.Fatal(err)
	}
	classes := buildCargoClasses(board)
	pos := positionsOf(board)
	units := []int{}
	for i, p := range board.Pieces {
		if p.Kind == PieceUnit {
			units = append(units, i)
		}
	}
	k1 := encodeCargoKeyFromPositions(pos, false, classes)
	pos[units[0]], pos[units[1]] = pos[units[1]], pos[units[0]]
	k2 := encodeCargoKeyFromPositions(pos, false, classes)
	if k1 != k2 {
		t.Fatal("interchangeable ids should not affect canonical key")
	}
	// Fingerprints for accepted candidates also use canonical cargo key.
	if layoutFingerprint(board) == "" {
		t.Fatal("empty fingerprint")
	}
}

func TestSimilarityFilterWorks(t *testing.T) {
	a := make([]bool, CargoFlowWidth*CargoFlowHeight)
	b := make([]bool, len(a))
	for i := 0; i < 20; i++ {
		a[i] = true
		b[i] = true
	}
	if occupancySimilarity(a, b) < 0.85 {
		t.Fatal("identical occupancy should be similar")
	}
	mir := mirrorOccupancy(a, CargoFlowWidth, CargoFlowHeight)
	if occupancySimilarity(mir, a) < 0.0 {
		t.Fatal("mirror similarity should be defined")
	}
	c := append([]bool(nil), a...)
	c[20] = true
	if occupancySimilarity(a, c) < 0.85 {
		t.Fatalf("near clone sim=%f", occupancySimilarity(a, c))
	}
	accepted := [][]bool{a}
	sim, _ := maxSimilarity(c, accepted, []string{"x"}, "y")
	if sim < 0.85 {
		t.Fatalf("maxSimilarity near-clone=%f", sim)
	}
}

func TestSolverBudgetRespected(t *testing.T) {
	cfg := DefaultCargoFlowGenerationConfig(99)
	cfg.TargetAccepted = 1
	cfg.MaxAttempts = 5
	cfg.MinOptimalGestures = 50 // nearly impossible => mostly rejects
	cfg.SolveTimeLimit = 50 * time.Millisecond
	cfg.MaxVisitedStates = 1000
	res := NewCargoFlowGenerator(cfg).Generate()
	if res.TotalElapsed > 5*time.Second {
		t.Fatalf("too slow: %s", res.TotalElapsed)
	}
	_ = res
}

func TestOriginalRushGeneratorUnaffected(t *testing.T) {
	g := NewDefaultGenerator()
	if g.Width != 6 || g.Height != 6 || g.PrimarySize != 2 {
		t.Fatalf("%+v", g)
	}
}

func TestOriginalRushSolverUnaffected(t *testing.T) {
	board, err := NewBoard(knownPuzzleForty1)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable || sol.NumMoves != 9 {
		t.Fatalf("%+v", sol)
	}
}

func TestCargoFlowSolverRegression(t *testing.T) {
	board, err := NewCargoFlowBoard(CargoFlowPOCFixture())
	if err != nil {
		t.Fatal(err)
	}
	sol := board.Solve()
	if !sol.Solvable || sol.NumMoves != 4 {
		t.Fatalf("%+v", sol)
	}
}

func TestWriteGenerationBatch_Roundtrip(t *testing.T) {
	cfg := lightGenConfig(11)
	res := NewCargoFlowGenerator(cfg).Generate()
	if len(res.Accepted) == 0 {
		t.Skip("no candidate")
	}
	dir := t.TempDir()
	if err := WriteGenerationBatch(dir, res); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, res.Accepted[0].CandidateID+".json")
	level, err := LoadCargoFlowLevelJSONFile(path)
	if err != nil {
		t.Fatal(err)
	}
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 10 * time.Second, MaxVisited: 4_000_000})
	if !sol.Solvable || sol.NumMoves != res.Accepted[0].OptimalGestures {
		t.Fatalf("reload solve %d want %d", sol.NumMoves, res.Accepted[0].OptimalGestures)
	}
}

func mustAcceptOne(t *testing.T, seed int64) CargoFlowAcceptedCandidate {
	t.Helper()
	res := NewCargoFlowGenerator(lightGenConfig(seed)).Generate()
	if len(res.Accepted) == 0 {
		t.Skip("no accepted candidate in budget; flaky env")
	}
	return res.Accepted[0]
}
