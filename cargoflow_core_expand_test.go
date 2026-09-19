package rush

import (
	"testing"
	"time"
)

func TestCoreExpansionProposalsNonEmpty(t *testing.T) {
	b := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	b.Rules = RulesCargoFlow
	b.ExitCol = CargoFlowExitCol
	if !b.AddPiece(Piece{Position: 3, Size: 2, Orientation: Horizontal, Kind: PieceTarget}) {
		t.Fatal("target")
	}
	if !b.AddPiece(Piece{Position: 1*7 + 3, Size: 2, Orientation: Vertical, Kind: PieceNormal}) {
		t.Fatal("blocker")
	}
	if !b.AddPiece(Piece{Position: 2*7 + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal}) {
		t.Fatal("side")
	}
	if !b.AddPiece(Piece{Position: 4*7 + 5, Size: 3, Orientation: Vertical, Kind: PieceNormal}) {
		t.Fatal("long")
	}
	budget := SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000}
	sol := b.SolveWithBudget(budget)
	if !sol.Solvable {
		t.Skip("synthetic board unsolved — skip proposal smoke")
	}
	props := proposeCoreExpansions(b, sol, 1, 0, DefaultCoreExpansionConfig())
	if len(props) == 0 {
		t.Fatal("expected at least one core-expansion proposal")
	}
	seen := map[CoreExpansionClass]bool{}
	for _, p := range props {
		seen[p.Class] = true
		if len(p.Relocate) == 0 {
			t.Fatalf("%s: relocate empty", p.Class)
		}
	}
	t.Logf("proposal classes: %v (n=%d)", seen, len(props))
}

func TestGenerateCoreExpansionRejectsInvalid(t *testing.T) {
	b := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	b.Rules = RulesCargoFlow
	b.ExitCol = CargoFlowExitCol
	_ = b.AddPiece(Piece{Position: 3, Size: 2, Orientation: Horizontal, Kind: PieceTarget})
	_ = b.AddPiece(Piece{Position: 1*7 + 4, Size: 2, Orientation: Vertical, Kind: PieceNormal})
	budget := SolveBudget{TimeLimit: 2 * time.Second, MaxVisited: 200_000}
	cfg := DefaultCoreExpansionConfig()
	cfg.MaxProposalsPerBoard = 4
	cfg.MaxAcceptedPerBoard = 1
	_, rej := GenerateCoreExpansionCandidates(b, 1, 0, 0, budget, cfg)
	if rej == nil {
		t.Fatal("nil reject map")
	}
}

func TestCoreExpansionConfigDefaults(t *testing.T) {
	cfg := DefaultCoreExpansionConfig()
	if cfg.SoftDeltaMin != -1 || cfg.SoftDeltaMax != 8 {
		t.Fatalf("soft delta bounds: %+v", cfg)
	}
	if cfg.FitThreshold != Default6x6FitThreshold {
		t.Fatalf("fit threshold %v", cfg.FitThreshold)
	}
}
