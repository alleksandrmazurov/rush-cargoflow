package rush

import "testing"

func TestExactStartStateFingerprintIgnoresPieceIDsAndOrder(t *testing.T) {
	a := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	a.Rules = RulesCargoFlow
	a.ExitCol = CargoFlowExitCol
	_ = a.AddPiece(Piece{Position: 4*7 + 3, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	_ = a.AddPiece(Piece{Position: 2*7 + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	_ = a.AddPiece(Piece{Position: 6*7 + 5, Size: 1, Orientation: Horizontal, Kind: PieceUnit})
	_ = a.AddWall(7)
	a.Labels = []string{"target", "A", "unit-a"}

	b := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	b.Rules = RulesCargoFlow
	b.ExitCol = CargoFlowExitCol
	_ = b.AddPiece(Piece{Position: 4*7 + 3, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	_ = b.AddPiece(Piece{Position: 6*7 + 5, Size: 1, Orientation: Vertical, Kind: PieceUnit})
	_ = b.AddPiece(Piece{Position: 2*7 + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	_ = b.AddWall(7)
	b.Labels = []string{"renamed-target", "unit-b", "B"}

	if ExactStartStateFingerprint(a) != ExactStartStateFingerprint(b) {
		t.Fatal("piece IDs/order or unit storage orientation changed exact-state fingerprint")
	}
}

func TestExactStartStateFingerprintRetainsMovementRulesAndWalls(t *testing.T) {
	base := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	base.Rules = RulesCargoFlow
	base.ExitCol = CargoFlowExitCol
	_ = base.AddPiece(Piece{Position: 4*7 + 3, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	_ = base.AddPiece(Piece{Position: 2*7 + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})

	changedOrientation := base.Copy()
	changedOrientation.Pieces[1].Orientation = Vertical
	if ExactStartStateFingerprint(base) == ExactStartStateFingerprint(changedOrientation) {
		t.Fatal("long-piece movement axis was ignored")
	}

	changedWalls := base.Copy()
	_ = changedWalls.AddWall(0)
	if ExactStartStateFingerprint(base) == ExactStartStateFingerprint(changedWalls) {
		t.Fatal("walls were ignored")
	}
}
