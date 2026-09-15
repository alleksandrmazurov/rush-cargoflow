package rush

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testRush1000(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "rushdb", "rush1000.txt")
}

func TestFullDatasetStreaming_NoHugeMemoryGrowth(t *testing.T) {
	path := testRush1000(t)
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	before := ms.HeapAlloc
	n := 0
	_, err := StreamRushDBFile(path, "rush1000", func(rec RushDBRecord) error {
		n++
		_, _ = ComputeSourceFeatures(rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	after := ms.HeapAlloc
	if n < 100 {
		t.Fatalf("scanned %d", n)
	}
	// Streaming must not retain all records; allow generous slack for GC noise.
	if after > before+50*1024*1024 {
		t.Fatalf("heap grew too much: before=%d after=%d", before, after)
	}
}

func TestStratifiedSampling_Deterministic(t *testing.T) {
	cfg := DefaultStratifiedSampleConfig()
	cfg.TargetPoolSize = 40
	cfg.PerStratumCap = 8
	cfg.Seed = 42
	cfg.DatasetTag = "rush1000"
	a, _, err := StratifiedSampleSources(testRush1000(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := StratifiedSampleSources(testRush1000(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("len %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].SourcePuzzleID != b[i].SourcePuzzleID {
			t.Fatalf("mismatch at %d: %s vs %s", i, a[i].SourcePuzzleID, b[i].SourcePuzzleID)
		}
	}
}

func TestDifferentDifficultyStrataRepresented(t *testing.T) {
	cfg := DefaultStratifiedSampleConfig()
	cfg.TargetPoolSize = 100
	cfg.PerStratumCap = 25
	cfg.DatasetTag = "rush1000"
	// rush1000 is top-heavy; still expect very-high + maybe high.
	pool, _, err := StratifiedSampleSources(testRush1000(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, f := range pool {
		seen[f.MovesStratum]++
	}
	if seen["very-high"] == 0 {
		t.Fatalf("expected very-high stratum, got %v", seen)
	}
}

func TestInventoryDiversityRepresented(t *testing.T) {
	cfg := DefaultStratifiedSampleConfig()
	cfg.TargetPoolSize = 80
	cfg.PerStratumCap = 20
	cfg.DatasetTag = "rush1000"
	pool, _, err := StratifiedSampleSources(testRush1000(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	inv := map[string]bool{}
	for _, f := range pool {
		inv[f.InventorySignature] = true
	}
	if len(inv) < 5 {
		t.Fatalf("inventory diversity too low: %d", len(inv))
	}
}

func TestSameSolve_CacheHit(t *testing.T) {
	dir := t.TempDir()
	cache, err := OpenSolveCache(filepath.Join(dir, "c.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	key := SolveCacheKey{
		SourceDatabaseHash: "abc",
		SourcePuzzleID:     "x",
		TransformVersion:   TransformVersionCW90,
		EmbeddingVariant:   "FlushTop",
		CargoRulesVersion:  CargoRulesVersionCF,
		SolverVersion:      SolverVersionBFS,
	}
	if err := cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true, OptimalGestures: 9}); err != nil {
		t.Fatal(err)
	}
	cache2, err := OpenSolveCache(filepath.Join(dir, "c.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := cache2.Get(key)
	if !ok || e.OptimalGestures != 9 {
		t.Fatalf("cache miss %+v ok=%v", e, ok)
	}
}

func TestSolverVersionChange_InvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	cache, _ := OpenSolveCache(filepath.Join(dir, "c.jsonl"))
	key := SolveCacheKey{SourceDatabaseHash: "h", SourcePuzzleID: "p", TransformVersion: "t", EmbeddingVariant: "e", CargoRulesVersion: "r", SolverVersion: "v1"}
	_ = cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true})
	key2 := key
	key2.SolverVersion = "v2"
	if _, ok := cache.Get(key2); ok {
		t.Fatal("different solver version should miss")
	}
}

func TestTransformVersionChange_InvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	cache, _ := OpenSolveCache(filepath.Join(dir, "c.jsonl"))
	key := SolveCacheKey{SourceDatabaseHash: "h", SourcePuzzleID: "p", TransformVersion: "t1", EmbeddingVariant: "e", CargoRulesVersion: "r", SolverVersion: "v"}
	_ = cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true})
	key2 := key
	key2.TransformVersion = "t2"
	if _, ok := cache.Get(key2); ok {
		t.Fatal("different transform version should miss")
	}
}

func TestInterruptedRun_ResumeUsesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.jsonl")
	cache, _ := OpenSolveCache(path)
	key := SolveCacheKey{SourceDatabaseHash: "h", SourcePuzzleID: "p", TransformVersion: TransformVersionCW90, EmbeddingVariant: "FlushTop", CargoRulesVersion: CargoRulesVersionCF, SolverVersion: SolverVersionBFS}
	_ = cache.Put(SolveCacheEntry{Key: key, Valid: true, Solved: true, OptimalGestures: 11, ReplayVerified: true})
	// Simulate restart
	cache2, err := OpenSolveCache(path)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := cache2.Get(key)
	if !ok || !e.Solved {
		t.Fatal("resume cache miss")
	}
}

func loadRUSH008Sigs(t *testing.T) map[string]PuzzleSignature {
	t.Helper()
	dir := filepath.Join("output", "RUSH008_DatabaseTransplant_Pilot")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("RUSH008 pilot output missing")
	}
	out := map[string]PuzzleSignature{}
	for i := 1; i <= 12; i++ {
		id := filepath.Join(dir, fmt.Sprintf("Candidate_%03d.json", i))
		level, err := LoadCargoFlowLevelJSONFile(id)
		if err != nil {
			t.Fatal(err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			t.Fatal(err)
		}
		sol := board.SolveWithBudget(SolveBudget{TimeLimit: 8 * time.Second, MaxVisited: 4_000_000})
		if !sol.Solvable {
			t.Fatalf("%s unsolvable", id)
		}
		out[fmt.Sprintf("Candidate_%03d", i)] = BuildPuzzleSignature(board, sol)
	}
	return out
}

func assertNear(t *testing.T, sigs map[string]PuzzleSignature, a, b string, maxDist float64) {
	t.Helper()
	d := PuzzleFamilyDistance(sigs[a], sigs[b])
	if d > maxDist {
		t.Fatalf("%s vs %s distance %.3f > %.3f", a, b, d, maxDist)
	}
}

func TestHumanFamily_001_003DetectedAsNear(t *testing.T) {
	sigs := loadRUSH008Sigs(t)
	assertNear(t, sigs, "Candidate_001", "Candidate_002", 0.35)
	assertNear(t, sigs, "Candidate_001", "Candidate_003", 0.35)
	assertNear(t, sigs, "Candidate_002", "Candidate_003", 0.35)
}

func TestHumanFamily_004_005DetectedAsNear(t *testing.T) {
	sigs := loadRUSH008Sigs(t)
	assertNear(t, sigs, "Candidate_004", "Candidate_005", 0.35)
}

func TestHumanFamily_008_012DetectedAsNear(t *testing.T) {
	sigs := loadRUSH008Sigs(t)
	assertNear(t, sigs, "Candidate_008", "Candidate_012", 0.40)
}

func TestHumanFamily_010_011DetectedAsNear(t *testing.T) {
	sigs := loadRUSH008Sigs(t)
	assertNear(t, sigs, "Candidate_010", "Candidate_011", 0.35)
}

func TestDifferentHumanFamilies_NotAllCollapsed(t *testing.T) {
	sigs := loadRUSH008Sigs(t)
	// 006/007 should not be nearly identical to family A (001)
	d6 := PuzzleFamilyDistance(sigs["Candidate_001"], sigs["Candidate_006"])
	d7 := PuzzleFamilyDistance(sigs["Candidate_001"], sigs["Candidate_007"])
	d9 := PuzzleFamilyDistance(sigs["Candidate_001"], sigs["Candidate_009"])
	far := 0
	for _, d := range []float64{d6, d7, d9} {
		if d >= 0.20 {
			far++
		}
	}
	if far < 2 {
		t.Fatalf("expected distinct families; d6=%.3f d7=%.3f d9=%.3f", d6, d7, d9)
	}
}

func TestShortlistUniqueSourceIds(t *testing.T) {
	res := runTinyCurator(t)
	seen := map[string]bool{}
	for _, c := range res.Shortlisted {
		if seen[c.Source.SourcePuzzleID] {
			t.Fatalf("duplicate source %s", c.Source.SourcePuzzleID)
		}
		seen[c.Source.SourcePuzzleID] = true
	}
}

func TestShortlistMultipleFamilies(t *testing.T) {
	res := runTinyCurator(t)
	fams := map[string]bool{}
	for _, c := range res.Shortlisted {
		fams[c.FamilyID] = true
	}
	if len(fams) < 2 && len(res.Shortlisted) >= 2 {
		t.Fatalf("expected multiple families, got %v", fams)
	}
}

func TestShortlistMultipleDifficultyBands(t *testing.T) {
	res := runTinyCurator(t)
	bands := map[string]bool{}
	for _, c := range res.Shortlisted {
		bands[cargoOptBand(c.Metrics.OptimalGestures)] = true
	}
	if len(bands) < 2 && len(res.Shortlisted) >= 3 {
		t.Fatalf("expected multiple bands, got %v", bands)
	}
}

func TestShortlistExactSolved(t *testing.T) {
	res := runTinyCurator(t)
	for _, c := range res.Shortlisted {
		if !c.Solution.Solvable || c.Metrics.OptimalGestures <= 0 {
			t.Fatalf("%s not exact solved", c.Level.LevelID)
		}
	}
}

func TestShortlistReplayPASS(t *testing.T) {
	res := runTinyCurator(t)
	for _, c := range res.Shortlisted {
		if !c.ReplayVerified {
			t.Fatalf("%s replay failed", c.Level.LevelID)
		}
		b := c.Board.Copy()
		if err := b.Replay(c.Solution.Moves); err != nil {
			t.Fatal(err)
		}
	}
}

func TestShortlistUnityJsonCompatible(t *testing.T) {
	res := runTinyCurator(t)
	dir := t.TempDir()
	if err := WriteCuratorShortlist(dir, res); err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Shortlisted {
		path := filepath.Join(dir, "Candidates", c.Level.LevelID+".json")
		level, err := LoadCargoFlowLevelJSONFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if level.CoordinateSpace != CoordinateSpaceUnity {
			t.Fatal("coordinate space")
		}
		if err := level.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSameSeedSameShortlist(t *testing.T) {
	a := runTinyCurator(t)
	b := runTinyCurator(t)
	if len(a.Shortlisted) != len(b.Shortlisted) {
		t.Fatalf("len %d vs %d", len(a.Shortlisted), len(b.Shortlisted))
	}
	for i := range a.Shortlisted {
		if a.Shortlisted[i].Source.SourcePuzzleID != b.Shortlisted[i].Source.SourcePuzzleID {
			t.Fatalf("order mismatch at %d", i)
		}
	}
}

func runTinyCurator(t *testing.T) CuratorResult {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultCuratorConfig(testRush1000(t))
	cfg.DatasetTag = "rush1000"
	cfg.SourcePoolSize = 40
	cfg.MaxSolveAttempts = 60
	cfg.ShortlistSize = 6
	cfg.Workers = 2
	cfg.Seed = 20260915
	cfg.CachePath = filepath.Join(dir, "cache.jsonl")
	cfg.OutputDir = filepath.Join(dir, "out")
	cfg.CalibrationPath = filepath.Join("data", "calibration", "rush008_human_review.json")
	cfg.MinOptimal = 6
	cfg.MaxOptimal = 40
	cfg.BandQuotas = map[string]int{"6-8": 2, "9-11": 2, "12-14": 2, "15-17": 2, "18+": 2}
	res, err := RunCurator(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Shortlisted) == 0 {
		t.Fatal("empty shortlist")
	}
	return res
}
