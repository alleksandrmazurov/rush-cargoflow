package rush

import (
	"fmt"
	"sort"
)

// TransformRotation documents the Rush→Cargo Flow geometric map.
// Clockwise 90° maps classic right-exit (+column) to Cargo Flow top-exit (−row).
const TransformRotationCW90 = "CW90"

// EmbeddingVariant places a rotated 6x6 block inside 7x8 with Target on exit col 3.
type EmbeddingVariant string

const (
	EmbedFlushTop    EmbeddingVariant = "FlushTop"    // offsetY = 0
	EmbedShiftDown1  EmbeddingVariant = "ShiftDown1"  // offsetY = 1
	EmbedFlushBottom EmbeddingVariant = "FlushBottom" // offsetY = 2
)

// DefaultEmbeddingVariants returns the POC embedding set.
func DefaultEmbeddingVariants() []EmbeddingVariant {
	return []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1, EmbedFlushBottom}
}

func embeddingOffsetY(v EmbeddingVariant) (int, error) {
	switch v {
	case EmbedFlushTop:
		return 0, nil
	case EmbedShiftDown1:
		return 1, nil
	case EmbedFlushBottom:
		return 2, nil
	default:
		return 0, fmt.Errorf("unknown embedding %q", v)
	}
}

// TransplantResult is one geometric conversion before Cargo Flow solving.
type TransplantResult struct {
	Source              RushDBRecord
	Rotation            string
	Embedding           EmbeddingVariant
	OffsetX             int
	OffsetY             int
	Board               *Board
	Level               *LevelJSON
	ASCIIPreview        string
	PrimaryColAfterRot  int
}

// RotateRushCellCW90 maps (r,c) on n×n to clockwise-90 destination.
// Right edge (r, n-1) → top edge (0, r): classic +column becomes −row (toward exit).
func RotateRushCellCW90(r, c, n int) (nr, nc int) {
	return n - 1 - c, r
}

// TransformRushRecordToCargoFlow rotates CW90 and embeds into 7x8 Cargo Flow.
func TransformRushRecordToCargoFlow(rec RushDBRecord, embed EmbeddingVariant) (*TransplantResult, error) {
	src, err := BoardFromRushDBRecord(rec)
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}
	if src.Width != RushDBWidth || src.Height != RushDBHeight {
		return nil, fmt.Errorf("source must be %dx%d", RushDBWidth, RushDBHeight)
	}

	offY, err := embeddingOffsetY(embed)
	if err != nil {
		return nil, err
	}

	// Locate primary before rotation (label A / first piece after NewBoard sort is "A").
	prim := src.Pieces[0]
	if prim.Orientation != Horizontal {
		return nil, fmt.Errorf("primary must be horizontal in classic Rush")
	}
	primRow := prim.Row(src.Width)
	// After CW90, primary becomes vertical in column = old primary row.
	primColAfter := primRow
	offX := CargoFlowExitCol - primColAfter
	if offX < 0 || offX > CargoFlowWidth-RushDBWidth {
		return nil, fmt.Errorf("primary col after rot=%d cannot align to exit col %d (offX=%d)",
			primColAfter, CargoFlowExitCol, offX)
	}
	if offY < 0 || offY > CargoFlowHeight-RushDBHeight {
		return nil, fmt.Errorf("invalid offsetY %d", offY)
	}

	cf := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	cf.Rules = RulesCargoFlow
	cf.ExitCol = CargoFlowExitCol

	type placed struct {
		label string
		cells []int // CF positions
	}
	var groups []placed

	// Collect cells per piece label from source board string.
	rows, err := RushDBBoardRows(rec.Board36)
	if err != nil {
		return nil, err
	}
	labelCells := map[string][][2]int{} // label -> list of (r,c)
	wallCells := [][2]int{}
	for r := 0; r < RushDBHeight; r++ {
		for c := 0; c < RushDBWidth; c++ {
			ch := rows[r][c]
			switch ch {
			case 'o', '.':
			case 'x':
				wallCells = append(wallCells, [2]int{r, c})
			default:
				lab := string(ch)
				labelCells[lab] = append(labelCells[lab], [2]int{r, c})
			}
		}
	}

	mapCell := func(r, c int) int {
		nr, nc := RotateRushCellCW90(r, c, RushDBWidth)
		cr := nr + offY
		cc := nc + offX
		return cr*CargoFlowWidth + cc
	}

	for _, wc := range wallCells {
		cf.AddWall(mapCell(wc[0], wc[1]))
	}

	labels := make([]string, 0, len(labelCells))
	for lab := range labelCells {
		labels = append(labels, lab)
	}
	sort.Strings(labels)

	for _, lab := range labels {
		cellsRC := labelCells[lab]
		cfCells := make([]int, len(cellsRC))
		for i, rc := range cellsRC {
			cfCells[i] = mapCell(rc[0], rc[1])
		}
		sort.Ints(cfCells)
		groups = append(groups, placed{label: lab, cells: cfCells})
	}

	// Build pieces: Target (A) first.
	var targetPiece *Piece
	var otherPieces []Piece
	var otherLabels []string
	for _, g := range groups {
		p, err := pieceFromSortedCells(g.cells, CargoFlowWidth)
		if err != nil {
			return nil, fmt.Errorf("piece %s: %w", g.label, err)
		}
		if g.label == "A" {
			if p.Size != 2 || p.Orientation != Vertical {
				return nil, fmt.Errorf("primary after transform must be vertical 1x2, got size=%d ori=%v", p.Size, p.Orientation)
			}
			if p.Col(CargoFlowWidth) != CargoFlowExitCol {
				return nil, fmt.Errorf("target col %d want %d", p.Col(CargoFlowWidth), CargoFlowExitCol)
			}
			p.Kind = PieceTarget
			targetPiece = &p
		} else {
			if p.Size < 2 || p.Size > 3 {
				return nil, fmt.Errorf("piece %s unsupported size %d", g.label, p.Size)
			}
			p.Kind = PieceNormal
			otherPieces = append(otherPieces, p)
			otherLabels = append(otherLabels, g.label)
		}
	}
	if targetPiece == nil {
		return nil, fmt.Errorf("missing primary A after transform")
	}

	cf.Pieces = append([]Piece{*targetPiece}, otherPieces...)
	labelsOut := make([]string, len(cf.Pieces))
	labelsOut[0] = "target"
	for i := range otherPieces {
		labelsOut[i+1] = fmt.Sprintf("P%02d", i+1)
	}
	cf.Labels = labelsOut
	// Rebuild occupancy from pieces+walls.
	cf.occupied = make([]bool, CargoFlowWidth*CargoFlowHeight)
	for _, w := range cf.Walls {
		cf.occupied[w] = true
	}
	for _, p := range cf.Pieces {
		cf.setOccupied(p, true)
	}
	cf.memoKey = MakeMemoKey(cf.Pieces)

	if err := cf.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	level, err := LevelJSONFromBoard(cf, "pending", "RushDatabaseTransplant")
	if err != nil {
		return nil, err
	}
	preview := asciiCargoBoard(cf)

	return &TransplantResult{
		Source:             rec,
		Rotation:           TransformRotationCW90,
		Embedding:          embed,
		OffsetX:            offX,
		OffsetY:            offY,
		Board:              cf,
		Level:              level,
		ASCIIPreview:       preview,
		PrimaryColAfterRot: primColAfter,
	}, nil
}

func pieceFromSortedCells(cells []int, w int) (Piece, error) {
	if len(cells) < 2 {
		return Piece{}, fmt.Errorf("need >=2 cells")
	}
	sort.Ints(cells)
	stride := cells[1] - cells[0]
	if stride != 1 && stride != w {
		return Piece{}, fmt.Errorf("invalid stride %d", stride)
	}
	for i := 2; i < len(cells); i++ {
		if cells[i]-cells[i-1] != stride {
			return Piece{}, fmt.Errorf("non-contiguous")
		}
	}
	ori := Horizontal
	if stride == w {
		ori = Vertical
	}
	return Piece{Position: cells[0], Size: len(cells), Orientation: ori}, nil
}

func asciiCargoBoard(board *Board) string {
	w, h := board.Width, board.Height
	grid := make([][]byte, h)
	for y := 0; y < h; y++ {
		grid[y] = make([]byte, w)
		for x := 0; x < w; x++ {
			grid[y][x] = '.'
		}
	}
	for _, wi := range board.Walls {
		grid[wi/w][wi%w] = 'x'
	}
	for i, p := range board.Pieces {
		ch := byte('A' + i)
		if p.Kind == PieceTarget {
			ch = 'T'
		}
		idx := p.Position
		stride := p.Stride(w)
		for s := 0; s < p.Size; s++ {
			grid[idx/w][idx%w] = ch
			idx += stride
		}
	}
	out := ""
	for y := 0; y < h; y++ {
		out += string(grid[y]) + "\n"
	}
	return out
}
