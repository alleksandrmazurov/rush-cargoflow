package rush

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// NewCargoFlowBoard parses a 7x8 ASCII Cargo Flow puzzle.
//
// Legend:
//
//	.     empty
//	x     static blocker (immovable)
//	T     Target (must form a vertical 1x2)
//	A-Z   long movable block (size >= 2, axis-aligned; not T)
//	a-z   movable 1x1 (each distinct letter is one piece)
//
// Coordinates: row 0 is the TOP (exit side), col 0 is LEFT.
// Position index = row*Width + col.
// Exit is a 1-cell gate above the board at column CargoFlowExitCol.
func NewCargoFlowBoard(desc []string) (*Board, error) {
	if len(desc) != CargoFlowHeight {
		return nil, fmt.Errorf("cargo flow board height must be %d", CargoFlowHeight)
	}
	for i, row := range desc {
		if len(row) != CargoFlowWidth {
			return nil, fmt.Errorf("cargo flow row %d width must be %d", i, CargoFlowWidth)
		}
	}

	w, h := CargoFlowWidth, CargoFlowHeight
	occupied := make([]bool, w*h)
	positions := make(map[rune][]int)
	var walls []int

	for y, row := range desc {
		for x, ch := range row {
			i := y*w + x
			switch {
			case ch == '.':
				continue
			case ch == 'x':
				occupied[i] = true
				walls = append(walls, i)
			default:
				occupied[i] = true
				positions[ch] = append(positions[ch], i)
			}
		}
	}

	targetCells, ok := positions['T']
	if !ok {
		return nil, fmt.Errorf("cargo flow board requires Target 'T'")
	}
	delete(positions, 'T')

	target, err := cargoPieceFromCells('T', targetCells, w, PieceTarget)
	if err != nil {
		return nil, err
	}
	if target.Size != CargoFlowTargetSize || target.Orientation != Vertical {
		return nil, fmt.Errorf("Target must be vertical 1x%d", CargoFlowTargetSize)
	}

	labelRunes := make([]rune, 0, len(positions))
	for label := range positions {
		labelRunes = append(labelRunes, label)
	}
	sort.Slice(labelRunes, func(i, j int) bool { return labelRunes[i] < labelRunes[j] })

	pieces := []Piece{target}
	display := []string{"T"}
	for _, label := range labelRunes {
		kind := PieceNormal
		if unicode.IsLower(label) {
			kind = PieceUnit
		}
		piece, err := cargoPieceFromCells(label, positions[label], w, kind)
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, piece)
		display = append(display, string(label))
	}

	board := &Board{
		Width:    w,
		Height:   h,
		Pieces:   pieces,
		Walls:    walls,
		Labels:   display,
		occupied: occupied,
		memoKey:  MakeMemoKey(pieces),
		Rules:    RulesCargoFlow,
	}
	return board, board.Validate()
}

func cargoPieceFromCells(label rune, cells []int, w int, kind PieceKind) (Piece, error) {
	if len(cells) == 0 {
		return Piece{}, fmt.Errorf("piece %c has no cells", label)
	}
	sort.Ints(cells)

	if kind == PieceUnit {
		if len(cells) != 1 {
			return Piece{}, fmt.Errorf("unit piece %c must be 1x1", label)
		}
		return Piece{Position: cells[0], Size: 1, Orientation: Horizontal, Kind: PieceUnit}, nil
	}

	if len(cells) < 2 {
		return Piece{}, fmt.Errorf("piece %c length must be >= 2", label)
	}
	stride := cells[1] - cells[0]
	if stride != 1 && stride != w {
		return Piece{}, fmt.Errorf("piece %c has invalid shape", label)
	}
	for i := 2; i < len(cells); i++ {
		if cells[i]-cells[i-1] != stride {
			return Piece{}, fmt.Errorf("piece %c has invalid shape", label)
		}
	}
	dir := Horizontal
	if stride != 1 {
		dir = Vertical
	}
	return Piece{Position: cells[0], Size: len(cells), Orientation: dir, Kind: kind}, nil
}

func (board *Board) validateCargoFlow() error {
	w, h := board.Width, board.Height
	if w != CargoFlowWidth || h != CargoFlowHeight {
		return fmt.Errorf("cargo flow board must be %dx%d", CargoFlowWidth, CargoFlowHeight)
	}
	if len(board.Pieces) < 1 {
		return fmt.Errorf("board must have at least one piece")
	}
	if len(board.Pieces) > MaxPieces {
		return fmt.Errorf("board must have <= %d pieces", MaxPieces)
	}

	occupied := make([]bool, w*h)
	for _, i := range board.Walls {
		if i < 0 || i >= w*h {
			return fmt.Errorf("a wall is outside of the grid")
		}
		if occupied[i] {
			return fmt.Errorf("a wall intersects another wall")
		}
		occupied[i] = true
	}

	target := board.Pieces[0]
	if target.Kind != PieceTarget {
		return fmt.Errorf("piece 0 must be Cargo Flow Target")
	}
	if target.Orientation != Vertical || target.Size != CargoFlowTargetSize {
		return fmt.Errorf("Target must be vertical size %d", CargoFlowTargetSize)
	}

	for i, piece := range board.Pieces {
		label := cargoLabel(i, piece)
		switch piece.Kind {
		case PieceTarget:
			if i != 0 {
				return fmt.Errorf("only piece 0 may be Target")
			}
		case PieceUnit:
			if piece.Size != 1 {
				return fmt.Errorf("piece %s unit size must be 1", label)
			}
		case PieceNormal:
			if piece.Size < 2 {
				return fmt.Errorf("piece %s size must be >= 2", label)
			}
		default:
			return fmt.Errorf("piece %s has unknown kind", label)
		}

		row, col := piece.Row(w), piece.Col(w)
		if piece.Orientation == Horizontal {
			if row < 0 || row >= h || col < 0 || col+piece.Size > w {
				return fmt.Errorf("piece %s is outside of the grid", label)
			}
		} else {
			if col < 0 || col >= w || row < 0 || row+piece.Size > h {
				return fmt.Errorf("piece %s is outside of the grid", label)
			}
		}

		idx := piece.Position
		stride := piece.Stride(w)
		if piece.Kind == PieceUnit {
			stride = 1
		}
		for j := 0; j < piece.Size; j++ {
			if occupied[idx] {
				return fmt.Errorf("piece %s intersects with another piece", label)
			}
			occupied[idx] = true
			idx += stride
		}
	}
	return nil
}

func cargoLabel(i int, piece Piece) string {
	if piece.Kind == PieceTarget {
		return "T"
	}
	return string(rune('A' + i))
}

func (board *Board) cargoMoves(buf []Move) []Move {
	moves := buf[:0]
	if board.won {
		return moves
	}
	for i, piece := range board.Pieces {
		switch piece.Kind {
		case PieceTarget:
			moves = board.appendAxisMoves(moves, i, piece, Vertical)
			if board.cargoTargetCanExit() {
				moves = append(moves, Move{Piece: i, Exit: true})
			}
		case PieceUnit:
			moves = board.appendAxisMoves(moves, i, piece, Horizontal)
			moves = board.appendAxisMoves(moves, i, piece, Vertical)
		case PieceNormal:
			moves = board.appendAxisMoves(moves, i, piece, piece.Orientation)
		}
	}
	return moves
}

// appendAxisMoves adds every on-board sliding destination along axis as its own
// 1-gesture transition (multi-cell drag == one move).
func (board *Board) appendAxisMoves(moves []Move, index int, piece Piece, axis Orientation) []Move {
	w, h := board.Width, board.Height
	var stride, reverseSteps, forwardSteps int
	if axis == Vertical {
		y := piece.Position / w
		reverseSteps = -y
		forwardSteps = h - piece.Size - y
		stride = w
	} else {
		x := piece.Position % w
		reverseSteps = -x
		forwardSteps = w - piece.Size - x
		stride = 1
	}

	idx := piece.Position - stride
	for steps := -1; steps >= reverseSteps; steps-- {
		if board.occupied[idx] {
			break
		}
		moves = append(moves, Move{Piece: index, Steps: steps, Axis: axis})
		idx -= stride
	}
	idx = piece.Position + piece.Size*stride
	for steps := 1; steps <= forwardSteps; steps++ {
		if board.occupied[idx] {
			break
		}
		moves = append(moves, Move{Piece: index, Steps: steps, Axis: axis})
		idx += stride
	}
	return moves
}

func (board *Board) cargoTargetCanExit() bool {
	target := board.Pieces[0]
	if target.Kind != PieceTarget || target.Orientation != Vertical {
		return false
	}
	w := board.Width
	if target.Col(w) != CargoFlowExitCol {
		return false
	}
	// Cells strictly above the target in the exit column must be empty.
	for row := 0; row < target.Row(w); row++ {
		if board.occupied[row*w+CargoFlowExitCol] {
			return false
		}
	}
	return true
}

func (board *Board) cargoIsSolved() bool {
	return board.won
}

func (board *Board) moveStride(piece Piece, move Move) int {
	if piece.Kind == PieceUnit {
		if move.Axis == Vertical {
			return board.Width
		}
		return 1
	}
	return piece.Stride(board.Width)
}

// HasMove reports whether move is currently legal.
func (board *Board) HasMove(move Move) bool {
	for _, m := range board.Moves(nil) {
		if m == move {
			return true
		}
	}
	return false
}

// ReplayApplies solution moves from a fresh copy semantics: caller provides the
// starting board; each move is checked then applied. Final state must be solved.
func (board *Board) Replay(moves []Move) error {
	startRules := board.Rules
	for i, move := range moves {
		if !board.HasMove(move) {
			return fmt.Errorf("replay step %d: move %v is not legal", i+1, move)
		}
		if err := board.validateOccupancyInvariant(); err != nil {
			return fmt.Errorf("replay step %d before apply: %w", i+1, err)
		}
		board.DoMove(move)
		if err := board.validateOccupancyInvariant(); err != nil {
			return fmt.Errorf("replay step %d after apply: %w", i+1, err)
		}
	}
	solved := board.Rules == RulesCargoFlow && board.cargoIsSolved()
	if board.Rules == RulesOriginalRush {
		solved = board.Pieces[0].Position == board.Target()
	}
	if !solved {
		return fmt.Errorf("replay finished without victory")
	}
	_ = startRules
	return nil
}

func (board *Board) validateOccupancyInvariant() error {
	if board.won {
		return nil
	}
	w, h := board.Width, board.Height
	occ := make([]bool, w*h)
	for _, i := range board.Walls {
		if i < 0 || i >= len(occ) {
			return fmt.Errorf("wall out of range")
		}
		if occ[i] {
			return fmt.Errorf("wall overlap")
		}
		occ[i] = true
	}
	for i, piece := range board.Pieces {
		stride := piece.Stride(w)
		if piece.Kind == PieceUnit {
			stride = 1
		}
		idx := piece.Position
		for j := 0; j < piece.Size; j++ {
			if idx < 0 || idx >= len(occ) {
				return fmt.Errorf("piece %d outside board", i)
			}
			if occ[idx] {
				return fmt.Errorf("piece %d overlap", i)
			}
			occ[idx] = true
			idx += stride
		}
	}
	for i := range occ {
		if occ[i] != board.occupied[i] {
			return fmt.Errorf("occupied bitmap mismatch at %d", i)
		}
	}
	return nil
}

// CargoFlowPOCFixture is a deterministic 7x8 puzzle for the POC.
// Optimal solution length is discovered by the solver and locked in tests.
func CargoFlowPOCFixture() []string {
	return []string{
		".......", // 0 top / exit side
		"BBBaHH.", // 1 a trapped on exit column between B and H
		"...b...", // 2 second 1x1 on exit column
		"...T...", // 3 Target
		"...T...", // 4 Target
		".VVV...", // 5 horizontal long block
		"...C...", // 6 vertical long
		"...C.x.", // 7 static blocker
	}
}

// FormatCargoSolution renders human-readable gesture list.
func FormatCargoSolution(board *Board, moves []Move) string {
	var b strings.Builder
	for i, move := range moves {
		label := pieceDisplayLabel(board, move.Piece)
		if move.Exit {
			fmt.Fprintf(&b, "%d. %s -> Exit\n", i+1, label)
			continue
		}
		axis := "H"
		p := board.Pieces[move.Piece]
		if p.Kind == PieceUnit {
			if move.Axis == Vertical {
				axis = "V"
			}
		} else if p.Orientation == Vertical {
			axis = "V"
		}
		fmt.Fprintf(&b, "%d. %s %s%+d\n", i+1, label, axis, move.Steps)
	}
	return b.String()
}

func pieceDisplayLabel(board *Board, index int) string {
	if index >= 0 && index < len(board.Labels) && board.Labels[index] != "" {
		return board.Labels[index]
	}
	if index >= 0 && index < len(board.Pieces) && board.Pieces[index].Kind == PieceTarget {
		return "T"
	}
	if index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("P%d", index)
}
