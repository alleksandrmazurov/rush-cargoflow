package rush

import (
	"fmt"
	"time"
)

// BoardShapeClass labels how a puzzle uses the 7×8 field (not difficulty).
type BoardShapeClass string

const (
	ShapeCompact6x6  BoardShapeClass = "Compact6x6"
	ShapeShiftedCore BoardShapeClass = "ShiftedCore"
	ShapeTall        BoardShapeClass = "Tall"
	ShapeWide        BoardShapeClass = "Wide"
	ShapeExpanded    BoardShapeClass = "Expanded"
	ShapeFullField   BoardShapeClass = "FullField"
)

// BoardUtilizationMetrics describes occupied space vs the embedded 6×6 Rush core.
type BoardUtilizationMetrics struct {
	OccupiedBoundingBoxWidth     int             `json:"occupiedBoundingBoxWidth"`
	OccupiedBoundingBoxHeight    int             `json:"occupiedBoundingBoxHeight"`
	OccupiedRows                 int             `json:"occupiedRows"`
	OccupiedColumns              int             `json:"occupiedColumns"`
	OuterRowUsage                int             `json:"outerRowUsage"`
	OuterColumnUsage             int             `json:"outerColumnUsage"`
	PiecesOutsideOriginal6x6Core int             `json:"piecesOutsideOriginal6x6Core"`
	OptimalMovesUsingOuterZone   int             `json:"optimalMovesUsingOuterZone"`
	OuterZoneRelevant            bool            `json:"outerZoneRelevant"`
	BoardUtilizationRatio        float64         `json:"boardUtilizationRatio"`
	CoreOffsetX                  int             `json:"coreOffsetX"`
	CoreOffsetY                  int             `json:"coreOffsetY"`
	BoardShapeClass              BoardShapeClass `json:"boardShapeClass"`
	OuterZoneEvidence            string          `json:"outerZoneEvidence,omitempty"`
}

// Original6x6CoreBounds returns the CF inclusive cell range for the embedded Rush block.
func Original6x6CoreBounds(offX, offY int) (minCol, maxCol, minRow, maxRow int) {
	return offX, offX + RushDBWidth - 1, offY, offY + RushDBHeight - 1
}

func cellInOriginal6x6Core(cell, w, offX, offY int) bool {
	r, c := cell/w, cell%w
	minC, maxC, minR, maxR := Original6x6CoreBounds(offX, offY)
	return c >= minC && c <= maxC && r >= minR && r <= maxR
}

// ComputeBoardUtilization derives geometry metrics from board (+ optional solution).
func ComputeBoardUtilization(board *Board, offX, offY int, sol *Solution) BoardUtilizationMetrics {
	w, h := board.Width, board.Height
	m := BoardUtilizationMetrics{CoreOffsetX: offX, CoreOffsetY: offY}
	if w == 0 || h == 0 {
		return m
	}
	rows, cols := map[int]bool{}, map[int]bool{}
	minR, maxR := h, -1
	minC, maxC := w, -1
	occupiedCells := 0
	outerRows, outerCols := map[int]bool{}, map[int]bool{}
	piecesOutside := 0

	mark := func(cell int) {
		r, c := cell/w, cell%w
		rows[r] = true
		cols[c] = true
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
		if c < minC {
			minC = c
		}
		if c > maxC {
			maxC = c
		}
		occupiedCells++
		if !cellInOriginal6x6Core(cell, w, offX, offY) {
			outerRows[r] = true
			outerCols[c] = true
		}
	}

	for _, wall := range board.Walls {
		mark(wall)
	}
	for _, p := range board.Pieces {
		outside := false
		idx := p.Position
		stride := p.Stride(w)
		for s := 0; s < p.Size; s++ {
			mark(idx)
			if !cellInOriginal6x6Core(idx, w, offX, offY) {
				outside = true
			}
			idx += stride
		}
		if outside {
			piecesOutside++
		}
	}

	if maxR >= minR {
		m.OccupiedBoundingBoxHeight = maxR - minR + 1
		m.OccupiedBoundingBoxWidth = maxC - minC + 1
	}
	m.OccupiedRows = len(rows)
	m.OccupiedColumns = len(cols)
	m.OuterRowUsage = len(outerRows)
	m.OuterColumnUsage = len(outerCols)
	m.PiecesOutsideOriginal6x6Core = piecesOutside
	total := w * h
	if total > 0 {
		m.BoardUtilizationRatio = round3(float64(occupiedCells) / float64(total))
	}

	if sol != nil && sol.Solvable {
		work := board.Copy()
		for _, mv := range sol.Moves {
			p := work.Pieces[mv.Piece]
			fromCells := pieceOccupancyCells(p, w)
			work.DoMove(mv)
			p2 := work.Pieces[mv.Piece]
			toCells := pieceOccupancyCells(p2, w)
			if anyOutsideCore(fromCells, w, offX, offY) || anyOutsideCore(toCells, w, offX, offY) {
				m.OptimalMovesUsingOuterZone++
			}
		}
	}

	m.OuterZoneRelevant, m.OuterZoneEvidence = detectOuterZoneRelevant(board, offX, offY, sol, m)
	m.BoardShapeClass = ClassifyBoardShape(m, offX, offY)
	return m
}

func pieceOccupancyCells(p Piece, w int) []int {
	out := make([]int, p.Size)
	idx := p.Position
	stride := p.Stride(w)
	for i := 0; i < p.Size; i++ {
		out[i] = idx
		idx += stride
	}
	return out
}

func anyOutsideCore(cells []int, w, offX, offY int) bool {
	for _, c := range cells {
		if !cellInOriginal6x6Core(c, w, offX, offY) {
			return true
		}
	}
	return false
}

func detectOuterZoneRelevant(board *Board, offX, offY int, sol *Solution, m BoardUtilizationMetrics) (bool, string) {
	if m.OptimalMovesUsingOuterZone > 0 {
		return true, "optimal-move-touches-outer"
	}
	if m.PiecesOutsideOriginal6x6Core > 0 {
		return true, "piece-occupies-outer"
	}
	if sol == nil || !sol.Solvable {
		return false, ""
	}
	sealed := board.Copy()
	w, h := sealed.Width, sealed.Height
	for cell := 0; cell < w*h; cell++ {
		if cellInOriginal6x6Core(cell, w, offX, offY) {
			continue
		}
		if !sealed.occupied[cell] {
			sealed.AddWall(cell)
		}
	}
	rsol := sealed.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
	if rsol.TimedOut || rsol.BudgetExceeded {
		return false, ""
	}
	if !rsol.Solvable {
		return true, "outer-seal-unsolvable"
	}
	if rsol.NumMoves > sol.NumMoves {
		return true, fmt.Sprintf("outer-seal-worse:%d>%d", rsol.NumMoves, sol.NumMoves)
	}
	return false, ""
}

// ClassifyBoardShape maps metrics to an explainable BoardShapeClass.
func ClassifyBoardShape(m BoardUtilizationMetrics, offX, offY int) BoardShapeClass {
	bboxW, bboxH := m.OccupiedBoundingBoxWidth, m.OccupiedBoundingBoxHeight
	shifted := offY >= 1 || offX != 0
	strongOuter := m.OuterZoneRelevant && (m.OuterRowUsage+m.OuterColumnUsage >= 2 || m.PiecesOutsideOriginal6x6Core >= 1)
	bothAxes := m.OuterRowUsage > 0 && m.OuterColumnUsage > 0
	fullish := bboxW >= CargoFlowWidth && bboxH >= CargoFlowHeight-1 && m.BoardUtilizationRatio >= 0.35

	if fullish && m.OuterZoneRelevant && (strongOuter || m.PiecesOutsideOriginal6x6Core > 0) {
		return ShapeFullField
	}
	if bboxW >= CargoFlowWidth && bboxH <= 6 && (m.OuterColumnUsage > 0 || m.OccupiedColumns >= 7) {
		return ShapeWide
	}
	if bboxH >= 7 && bboxW <= 6 {
		return ShapeTall
	}
	// Expanded: substantial use of BOTH extra dimensions (not just a single-axis stretch).
	if bothAxes && m.OuterZoneRelevant && (bboxW >= 7 && bboxH >= 7 || m.PiecesOutsideOriginal6x6Core >= 2 || strongOuter) {
		return ShapeExpanded
	}
	if strongOuter || m.PiecesOutsideOriginal6x6Core > 0 || (m.OuterRowUsage+m.OuterColumnUsage) >= 2 {
		// Single-axis outer without bothAxes already handled as Tall/Wide; residual → Expanded only if both axes.
		if bothAxes {
			return ShapeExpanded
		}
		if bboxH >= 7 {
			return ShapeTall
		}
		if bboxW >= 7 {
			return ShapeWide
		}
		return ShapeExpanded
	}
	if shifted && bboxW <= 6 && bboxH <= 6 && m.PiecesOutsideOriginal6x6Core == 0 {
		return ShapeShiftedCore
	}
	if bboxW <= 6 && bboxH <= 6 && !m.OuterZoneRelevant {
		return ShapeCompact6x6
	}
	if shifted {
		return ShapeShiftedCore
	}
	return ShapeCompact6x6
}

// BoardSpaceJSON is sequencer-facing board geometry metadata.
type BoardSpaceJSON struct {
	BoardShapeClass              string  `json:"boardShapeClass"`
	BoardUtilizationRatio        float64 `json:"boardUtilizationRatio"`
	OccupiedBoundingBoxWidth     int     `json:"occupiedBoundingBoxWidth"`
	OccupiedBoundingBoxHeight    int     `json:"occupiedBoundingBoxHeight"`
	OccupiedRows                 int     `json:"occupiedRows"`
	OccupiedColumns              int     `json:"occupiedColumns"`
	OuterRowUsage                int     `json:"outerRowUsage"`
	OuterColumnUsage             int     `json:"outerColumnUsage"`
	PiecesOutsideOriginal6x6Core int     `json:"piecesOutsideOriginal6x6Core"`
	OptimalMovesUsingOuterZone   int     `json:"optimalMovesUsingOuterZone"`
	OuterZoneRelevant            bool    `json:"outerZoneRelevant"`
	OuterZoneEvidence            string  `json:"outerZoneEvidence,omitempty"`
	CoreOffsetX                  int     `json:"coreOffsetX"`
	CoreOffsetY                  int     `json:"coreOffsetY"`
	EmbeddingVariant             string  `json:"embeddingVariant,omitempty"`
	// RUSH-010.5 meaningful-space metrics (optional; old importers ignore).
	Best6x6MeaningfulContainmentRatio float64 `json:"best6x6MeaningfulContainmentRatio,omitempty"`
	CanMeaningfulStructureFitInAny6x6 bool    `json:"canMeaningfulStructureFitInAny6x6,omitempty"`
	DependencyRowsUsed                int     `json:"dependencyRowsUsed,omitempty"`
	DependencyColumnsUsed             int     `json:"dependencyColumnsUsed,omitempty"`
	OuterDependencyPieceCount         int     `json:"outerDependencyPieceCount,omitempty"`
	IsolatedAddon1x1Suspect           bool    `json:"isolatedAddon1x1Suspect,omitempty"`
}

func (m BoardUtilizationMetrics) ToJSON(embed EmbeddingVariant) *BoardSpaceJSON {
	return &BoardSpaceJSON{
		BoardShapeClass:              string(m.BoardShapeClass),
		BoardUtilizationRatio:        m.BoardUtilizationRatio,
		OccupiedBoundingBoxWidth:     m.OccupiedBoundingBoxWidth,
		OccupiedBoundingBoxHeight:    m.OccupiedBoundingBoxHeight,
		OccupiedRows:                 m.OccupiedRows,
		OccupiedColumns:              m.OccupiedColumns,
		OuterRowUsage:                m.OuterRowUsage,
		OuterColumnUsage:             m.OuterColumnUsage,
		PiecesOutsideOriginal6x6Core: m.PiecesOutsideOriginal6x6Core,
		OptimalMovesUsingOuterZone:   m.OptimalMovesUsingOuterZone,
		OuterZoneRelevant:            m.OuterZoneRelevant,
		OuterZoneEvidence:            m.OuterZoneEvidence,
		CoreOffsetX:                  m.CoreOffsetX,
		CoreOffsetY:                  m.CoreOffsetY,
		EmbeddingVariant:             string(embed),
	}
}
