package rush

// Ruleset selects puzzle semantics for a Board.
type Ruleset int

const (
	// RulesOriginalRush is classic Rush Hour (default zero value).
	RulesOriginalRush Ruleset = iota
	// RulesCargoFlow is the Cargo Flow POC ruleset (7x8, top exit, 1x1, etc).
	RulesCargoFlow
)

func (r Ruleset) String() string {
	switch r {
	case RulesOriginalRush:
		return "OriginalRush"
	case RulesCargoFlow:
		return "CargoFlow"
	default:
		return "Unknown"
	}
}

// PieceKind distinguishes Cargo Flow piece roles.
// Zero value PieceNormal preserves original Rush Hour vehicles.
type PieceKind int

const (
	PieceNormal PieceKind = iota // Rush vehicle / Cargo Flow long block
	PieceTarget                  // Cargo Flow target (vertical 1x2)
	PieceUnit                    // Cargo Flow movable 1x1
)

// Cargo Flow board geometry (matches intended Unity playfield for this POC).
const (
	CargoFlowWidth   = 7
	CargoFlowHeight  = 8
	CargoFlowExitCol = 3 // center column on a 7-wide board (0-based)
	CargoFlowTargetSize = 2
)
