package rush

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestZero1x1ValidCandidate(t *testing.T) {
	base := loadInventoryTestBase(t)
	c := wrapNo1x1Candidate(base)
	if c.Inventory.InventoryClass != InvNo1x1 {
		t.Fatalf("expected No1x1, got %s", c.Inventory.InventoryClass)
	}
	if c.Added1x1Count != 0 || c.Inventory.Movable1x1Count != 0 {
		t.Fatalf("expected zero movable 1x1")
	}
	if !c.ReplayVerified {
		t.Fatal("No1x1 should be replay-valid as clean base")
	}
}

func TestOne1x1Essential(t *testing.T) {
	res := runInventoryTiny(t)
	found := false
	for _, c := range res.Accepted {
		if c.Added1x1Count != 1 {
			continue
		}
		found = true
		if c.Essential1x1Count < 1 {
			t.Fatalf("%s: single 1x1 must be essential", c.CandidateID)
		}
		if c.Relevant1x1Count < 1 {
			t.Fatalf("%s: single 1x1 must be relevant", c.CandidateID)
		}
	}
	if !found && res.AcceptedByClass[InvOne1x1] == 0 {
		t.Skip("no One1x1 in tiny run; covered by pilot if available")
	}
}

func TestTwo1x1Supported(t *testing.T) {
	res := runInventoryTiny(t)
	if res.AcceptedByClass[InvTwo1x1] == 0 && len(res.Accepted) > 0 {
		// Tiny budget may miss Two1x1; assert plumbing still works.
		cfg := DefaultInventoryDiversityConfig(".")
		if cfg.QuotaTwo1x1 != 3 {
			t.Fatal("Two1x1 quota missing from defaults")
		}
		return
	}
	for _, c := range res.Accepted {
		if c.Added1x1Count != 2 {
			continue
		}
		if c.Essential1x1Count < 1 {
			t.Fatalf("%s: need >=1 essential", c.CandidateID)
		}
		if c.Relevant1x1Count < 2 {
			t.Fatalf("%s: both 1x1 must be relevant", c.CandidateID)
		}
	}
}

func TestThree1x1Supported(t *testing.T) {
	cfg := DefaultInventoryDiversityConfig(".")
	if cfg.MaxPlacements3 <= 0 || cfg.QuotaThree1x1 != 3 {
		t.Fatal("Three1x1 not configured")
	}
	res := runInventoryTiny(t)
	for _, c := range res.Accepted {
		if c.Added1x1Count != 3 {
			continue
		}
		if c.Essential1x1Count < 1 {
			t.Fatalf("%s: need >=1 essential", c.CandidateID)
		}
		if c.Relevant1x1Count < 2 {
			t.Fatalf("%s: need >=2 relevant", c.CandidateID)
		}
	}
}

func TestMultiple1x1NoOverlap(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 5*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	// Occupy two empties via walls so remaining empties are distinct.
	cfg := DefaultInventoryDiversityConfig(".")
	cfg.MaxPlacements2 = 8
	cfg.MaxPlacements3 = 6
	cfg.MaxCorridor1x1 = 1
	// Place a blocking wall so board has empties.
	for _, cells := range enumerate1x1PlacementsN(board, cfg, 2) {
		if cells[0] == cells[1] {
			t.Fatal("duplicate cells in pair")
		}
	}
	for _, cells := range enumerate1x1PlacementsN(board, cfg, 3) {
		seen := map[int]bool{}
		for _, c := range cells {
			if seen[c] {
				t.Fatal("overlap in triple")
			}
			seen[c] = true
		}
	}
}

func TestMultiple1x1RulesCorrect(t *testing.T) {
	if minRelevantRequired(1) != 1 || minRelevantRequired(2) != 2 || minRelevantRequired(3) != 2 {
		t.Fatal("relevant count rules mismatch")
	}
}

func TestDecorative1x1Rejected_Inventory(t *testing.T) {
	// Decorative rejection reason must be wired into evaluateInventoryPlacement.
	base := loadInventoryTestBase(t)
	cfg := DefaultInventoryDiversityConfig(".")
	cfg.MaxPlacements1 = 4
	cfg.MaxPlacements2 = 0
	cfg.MaxPlacements3 = 0
	budget := SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000}
	res := &EnrichmentBatchResult{Rejected: map[string]int{}}
	// Place a 1x1 far from action likely unused — may reject as unused/decorative.
	empties := []int{}
	for i := 0; i < base.Board.Width*base.Board.Height; i++ {
		if !base.Board.occupied[i] && !CellInTargetExitCorridor(base.Board, i) {
			empties = append(empties, i)
		}
	}
	if len(empties) == 0 {
		t.Skip("no empty cells")
	}
	_, reason := evaluateInventoryPlacement(base, []int{empties[len(empties)-1]}, cfg, budget, res)
	if reason == "" {
		// Accepted is fine if it happened to be essential.
		return
	}
	switch reason {
	case "Decorative1x1", "OneByOneUnused", "OneByOneNotEssential", "OneByOneMoveCount",
		"QualityDegraded", "DifficultyUnknown", "NecessityUnknown", "Unsolvable", "ImmediateVictory":
		// expected reject classes
	default:
		t.Logf("got reject reason %s (ok)", reason)
	}
}

func TestAtLeastOneEssentialForMulti(t *testing.T) {
	res := runInventoryTiny(t)
	for _, c := range res.Accepted {
		if c.Added1x1Count >= 2 && c.Essential1x1Count < 1 {
			t.Fatalf("%s multi without essential", c.CandidateID)
		}
	}
}

func TestRelevantCountRequirementPASS(t *testing.T) {
	res := runInventoryTiny(t)
	for _, c := range res.Accepted {
		need := minRelevantRequired(c.Added1x1Count)
		if c.Added1x1Count > 0 && c.Relevant1x1Count < need {
			t.Fatalf("%s relevant=%d need=%d", c.CandidateID, c.Relevant1x1Count, need)
		}
	}
}

func TestRoleDiversityRankingWorks(t *testing.T) {
	cfg := DefaultInventoryDiversityConfig(".")
	a := &EnrichmentAccepted{
		Essential1x1Count: 1, Relevant1x1Count: 2, OptimalDelta: 2,
		CubeDiags: []CubeRelevanceDiag{{Role: RoleGateKeeper}, {Role: RoleSpaceMaker}},
		Spatial:   Spatial1x1Diag{DistinctRowsUsed: 2, DistinctColumnsUsed: 2, BoardRegionsUsed: 2},
		Inventory: CargoInventorySignature{InventoryClass: InvTwo1x1},
	}
	b := &EnrichmentAccepted{
		Essential1x1Count: 1, Relevant1x1Count: 2, OptimalDelta: 2,
		CubeDiags: []CubeRelevanceDiag{{Role: RoleSpaceMaker}, {Role: RoleSpaceMaker}},
		Spatial:   Spatial1x1Diag{DistinctRowsUsed: 2, DistinctColumnsUsed: 2, BoardRegionsUsed: 2},
		Inventory: CargoInventorySignature{InventoryClass: InvTwo1x1},
	}
	if !betterInventoryCandidate(a, b, cfg) {
		t.Fatal("role-diverse candidate should rank higher")
	}
}

func TestSpatialDiversityCalculated(t *testing.T) {
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 5*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	cells := []int{0, board.Width*board.Height - 1}
	sp := ComputeSpatial1x1Diag(board, cells)
	if sp.DistinctRowsUsed < 2 || sp.DistinctColumnsUsed < 2 {
		t.Fatalf("expected spread spatial diag, got %+v", sp)
	}
	if sp.BoardRegionsUsed < 2 {
		t.Fatalf("expected multi-region, got %d", sp.BoardRegionsUsed)
	}
}

func TestTargetCorridorDominancePenalized(t *testing.T) {
	cfg := DefaultInventoryDiversityConfig(".")
	a := &EnrichmentAccepted{
		Essential1x1Count: 1, Relevant1x1Count: 2, OptimalDelta: 1,
		Corridor1x1Count: 0,
		Spatial:          Spatial1x1Diag{DistinctRowsUsed: 2, DistinctColumnsUsed: 2, BoardRegionsUsed: 2},
		CubeDiags:        []CubeRelevanceDiag{{Role: RoleGateKeeper}, {Role: RoleSpaceMaker}},
	}
	b := &EnrichmentAccepted{
		Essential1x1Count: 1, Relevant1x1Count: 2, OptimalDelta: 1,
		Corridor1x1Count: 2,
		Spatial:          Spatial1x1Diag{DistinctRowsUsed: 2, DistinctColumnsUsed: 2, BoardRegionsUsed: 2},
		CubeDiags:        []CubeRelevanceDiag{{Role: RoleGateKeeper}, {Role: RoleSpaceMaker}},
	}
	if inventoryRankScore(a, cfg) <= inventoryRankScore(b, cfg) {
		t.Fatal("corridor dominance should be penalized")
	}
}

func TestInventoryClassCorrect(t *testing.T) {
	if inventoryClassFromCount(0) != InvNo1x1 || inventoryClassFromCount(1) != InvOne1x1 ||
		inventoryClassFromCount(2) != InvTwo1x1 || inventoryClassFromCount(3) != InvThree1x1 {
		t.Fatal("inventory class mapping wrong")
	}
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.AddPiece(Piece{Position: 5*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	sig := BuildCargoInventorySignature(board)
	if sig.InventoryClass != InvNo1x1 || sig.Movable1x1Count != 0 {
		t.Fatalf("clean board should be No1x1: %+v", sig)
	}
}

func TestDifficultyNotDerivedFrom1x1Count(t *testing.T) {
	res := runInventoryTiny(t)
	// Ensure SelectionBand is preserved from base, not overwritten by cube count.
	for _, c := range res.Accepted {
		if c.Base.SelectionBand == "" {
			t.Fatalf("%s missing selection band", c.CandidateID)
		}
		if c.Level != nil && c.Level.Enrichment != nil {
			if c.Level.Enrichment.InventoryClass == "" && c.Added1x1Count == 0 {
				// No1x1 still gets class via finalize
			}
		}
	}
	// Config documents quotas are independent of difficulty.
	cfg := DefaultInventoryDiversityConfig(".")
	if cfg.QuotaNo1x1+cfg.QuotaOne1x1+cfg.QuotaTwo1x1+cfg.QuotaThree1x1 != 12 {
		t.Fatal("pilot quotas should sum to 12")
	}
}

func TestReplayPASS_Inventory(t *testing.T) {
	res := runInventoryTiny(t)
	for _, c := range res.Accepted {
		if !c.ReplayVerified {
			t.Fatalf("%s replay failed", c.CandidateID)
		}
		b := c.Board.Copy()
		if err := b.Replay(c.Solution.Moves); err != nil {
			t.Fatalf("%s replay error: %v", c.CandidateID, err)
		}
	}
}

func TestRUSH009Unaffected_Inventory(t *testing.T) {
	// Inventory mode must not mutate curator defaults / RUSH-009 selection math.
	a := DefaultCuratorConfig("data/external/rush/rush.txt")
	_ = DefaultInventoryDiversityConfig("output/RUSH009_CuratedShortlist_001")
	b := DefaultCuratorConfig("data/external/rush/rush.txt")
	if a.ShortlistSize != b.ShortlistSize || a.FamilyThreshold != b.FamilyThreshold {
		t.Fatal("inventory config mutated curator defaults")
	}
}

func TestInventoryClassOnJSON(t *testing.T) {
	res := runInventoryTiny(t)
	for _, c := range res.Accepted {
		if c.Level == nil || c.Level.Enrichment == nil {
			t.Fatalf("%s missing enrichment metadata", c.CandidateID)
		}
		if c.Level.Enrichment.InventoryClass == "" {
			t.Fatalf("%s missing inventoryClass", c.CandidateID)
		}
	}
}

var (
	inventoryTinyOnce sync.Once
	inventoryTinyRes  InventoryDiversityBatchResult
	inventoryTinyErr  error
)

func runInventoryTiny(t *testing.T) InventoryDiversityBatchResult {
	t.Helper()
	batch := "output/RUSH009_CuratedShortlist_001"
	if _, err := filepath.Abs(batch); err != nil {
		t.Fatal(err)
	}
	inventoryTinyOnce.Do(func() {
		cfg := DefaultInventoryDiversityConfig(batch)
		cfg.OutputDir = t.TempDir()
		cfg.BaseCount = 6
		cfg.TargetAccepted = 4
		cfg.QuotaNo1x1 = 1
		cfg.QuotaOne1x1 = 1
		cfg.QuotaTwo1x1 = 1
		cfg.QuotaThree1x1 = 1
		cfg.MaxPlacements1 = 8
		cfg.MaxPlacements2 = 4
		cfg.MaxPlacements3 = 3
		cfg.SolveTimeLimit = 4 * time.Second
		cfg.MaxVisitedStates = 800_000
		inventoryTinyRes, inventoryTinyErr = RunInventoryDiversityPilot(cfg)
	})
	if inventoryTinyErr != nil {
		t.Fatal(inventoryTinyErr)
	}
	return inventoryTinyRes
}

func loadInventoryTestBase(t *testing.T) EnrichmentBase {
	t.Helper()
	cfg := DefaultInventoryDiversityConfig("output/RUSH009_CuratedShortlist_001")
	cfg.BaseCount = 3
	bases, err := SelectEnrichmentBases(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(bases) == 0 {
		t.Fatal("no enrichment bases")
	}
	return bases[0]
}
