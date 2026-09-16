package rush

import (
	"fmt"
	"sort"
	"strings"
)

// OneByOneRole is an explainable gameplay role for an enriched movable 1x1.
type OneByOneRole string

const (
	RoleDirectTargetBlocker OneByOneRole = "DirectTargetBlocker"
	RoleIndirectBlocker     OneByOneRole = "IndirectBlocker"
	RoleSpaceMaker          OneByOneRole = "SpaceMaker"
	RoleSideShuttle         OneByOneRole = "SideShuttle"
	RoleGateKeeper          OneByOneRole = "GateKeeper"
)

// OneByOneRoleEvidence explains why a role was assigned.
type OneByOneRoleEvidence struct {
	Role                        OneByOneRole `json:"role"`
	InitiallyInTargetCorridor   bool         `json:"initiallyInTargetCorridor"`
	ReleasedCells               []int        `json:"releasedCells"`
	SubsequentPieceClass         string       `json:"subsequentPieceClass,omitempty"`
	SubsequentPieceStepsLater   int          `json:"subsequentPieceStepsLater,omitempty"`
	SubsequentUsesReleasedCell  bool         `json:"subsequentUsesReleasedCell"`
	LinksToTargetCorridor       bool         `json:"linksToTargetCorridor"`
	UnitMoveCount               int          `json:"unitMoveCount"`
	InitialLegalMoveCount       int          `json:"initialLegalMoveCount"`
	Summary                     string       `json:"summary"`
}

// TargetExitCorridorCells returns Rush cells on the exit column strictly above the Target.
func TargetExitCorridorCells(board *Board) []int {
	if len(board.Pieces) == 0 || board.Pieces[0].Kind != PieceTarget {
		return nil
	}
	w := board.Width
	col := board.exitColumn()
	tgtRow := board.Pieces[0].Row(w)
	out := make([]int, 0, tgtRow)
	for row := 0; row < tgtRow; row++ {
		out = append(out, row*w+col)
	}
	return out
}

// CellInTargetExitCorridor reports whether cell is on the Target exit path (above Target).
func CellInTargetExitCorridor(board *Board, cell int) bool {
	w := board.Width
	if cell < 0 || cell >= w*board.Height {
		return false
	}
	if cell%w != board.exitColumn() {
		return false
	}
	if len(board.Pieces) == 0 {
		return false
	}
	return cell/w < board.Pieces[0].Row(w)
}

// ClassifyOneByOneRole derives an explainable role for added 1x1 pieces from board+solution.
func ClassifyOneByOneRole(board *Board, sol Solution, addedIdx []int) OneByOneRoleEvidence {
	ev := OneByOneRoleEvidence{
		ReleasedCells: []int{},
	}
	if len(addedIdx) == 0 {
		ev.Role = RoleIndirectBlocker
		ev.Summary = "no added 1x1"
		return ev
	}
	primary := addedIdx[0]
	startPos := board.Pieces[primary].Position
	ev.InitiallyInTargetCorridor = CellInTargetExitCorridor(board, startPos)
	ev.InitialLegalMoveCount = countPieceLegalMoves(board, primary)

	// Replay solution: track cells freed by added 1x1 and later occupancy by other pieces.
	work := board.Copy()
	type releaseEvent struct {
		step int
		cell int
	}
	releases := []releaseEvent{}
	unitMoveCount := 0
	corridorCells := map[int]bool{}
	for _, c := range TargetExitCorridorCells(board) {
		corridorCells[c] = true
	}

	for step, m := range sol.Moves {
		isAdded := false
		for _, ai := range addedIdx {
			if m.Piece == ai {
				isAdded = true
				break
			}
		}
		if isAdded && work.Pieces[m.Piece].Kind == PieceUnit {
			unitMoveCount++
			before := pieceCells(work, m.Piece)
			path := movePathCells(work, m)
			work.DoMove(m)
			after := map[int]bool{}
			if !m.Exit {
				for c := range pieceCells(work, m.Piece) {
					after[c] = true
				}
			}
			for c := range before {
				if !after[c] {
					releases = append(releases, releaseEvent{step: step, cell: c})
					ev.ReleasedCells = append(ev.ReleasedCells, c)
				}
			}
			for c := range path {
				if !after[c] {
					releases = append(releases, releaseEvent{step: step, cell: c})
				}
			}
			continue
		}
		work.DoMove(m)
		if m.Exit || ev.SubsequentUsesReleasedCell {
			continue
		}
		occ := pieceCells(work, m.Piece)
		for _, rel := range releases {
			if step <= rel.step {
				continue
			}
			if !occ[rel.cell] {
				continue
			}
			ev.SubsequentUsesReleasedCell = true
			ev.SubsequentPieceClass = pieceClass(work.Pieces[m.Piece])
			ev.SubsequentPieceStepsLater = step - rel.step
			if corridorCells[rel.cell] || pieceTouchesCorridor(work, m.Piece, corridorCells) {
				ev.LinksToTargetCorridor = true
			}
			break
		}
	}
	ev.UnitMoveCount = unitMoveCount
	ev.ReleasedCells = uniqueSortedInts(ev.ReleasedCells)

	// Role decision (explainable priority). Direct corridor always wins.
	if ev.InitiallyInTargetCorridor {
		ev.Role = RoleDirectTargetBlocker
		ev.LinksToTargetCorridor = true
		ev.Summary = fmt.Sprintf("1x1 starts on Target exit corridor cell %d and must move", startPos)
		return ev
	}

	// Constrained side passage controlling local traffic.
	if ev.InitialLegalMoveCount > 0 && ev.InitialLegalMoveCount <= 3 && isConstrainedSideCell(board, startPos) {
		ev.Role = RoleGateKeeper
		ev.Summary = fmt.Sprintf("1x1 in constrained side passage (legalMoves=%d) controlling local traffic",
			ev.InitialLegalMoveCount)
		return ev
	}

	// Multiple off-corridor moves → side shuttle choreography.
	if unitMoveCount >= 2 {
		ev.Role = RoleSideShuttle
		ev.Summary = fmt.Sprintf("1x1 performs %d off-corridor moves enabling sequential manoeuvres", unitMoveCount)
		return ev
	}

	longUsesSpace := ev.SubsequentUsesReleasedCell && isLongPieceClass(ev.SubsequentPieceClass)
	if longUsesSpace {
		ev.Role = RoleSpaceMaker
		ev.Summary = fmt.Sprintf("1x1 releases cells %v; later %s uses freed space (+%d steps)",
			ev.ReleasedCells, ev.SubsequentPieceClass, ev.SubsequentPieceStepsLater)
		return ev
	}

	if ev.SubsequentUsesReleasedCell {
		ev.Role = RoleIndirectBlocker
		ev.Summary = fmt.Sprintf("1x1 off-corridor; frees cells for %s which continues dependency chain",
			ev.SubsequentPieceClass)
		return ev
	}

	// Fallback: essential off-corridor without clear freed-cell reuse.
	ev.Role = RoleIndirectBlocker
	ev.Summary = "1x1 off-corridor essential (restricted solve worse) without clear freed-cell reuse"
	return ev
}

func countPieceLegalMoves(board *Board, index int) int {
	n := 0
	for _, m := range board.Moves(nil) {
		if m.Piece == index && !m.Exit {
			n++
		}
	}
	return n
}

func isLongPieceClass(cls string) bool {
	return strings.Contains(cls, "1x2") || strings.Contains(cls, "1x3")
}

func pieceTouchesCorridor(board *Board, index int, corridor map[int]bool) bool {
	for c := range pieceCells(board, index) {
		if corridor[c] {
			return true
		}
		// Adjacent to corridor counts as linked.
		w := board.Width
		for _, d := range []int{-1, 1, -w, w} {
			nb := c + d
			if corridor[nb] {
				return true
			}
		}
	}
	return false
}

func isConstrainedSideCell(board *Board, cell int) bool {
	w, h := board.Width, board.Height
	r, c := cell/w, cell%w
	// Side columns or near walls/pieces on 2+ sides.
	if c == 0 || c == w-1 || r == h-1 {
		return true
	}
	blocked := 0
	for _, d := range []int{-1, 1, -w, w} {
		nb := cell + d
		if nb < 0 || nb >= w*h {
			blocked++
			continue
		}
		// Prevent wrap on horizontal.
		if d == -1 && c == 0 {
			blocked++
			continue
		}
		if d == 1 && c == w-1 {
			blocked++
			continue
		}
		if board.occupied[nb] {
			blocked++
		}
	}
	return blocked >= 2
}

func uniqueSortedInts(in []int) []int {
	m := map[int]bool{}
	for _, v := range in {
		m[v] = true
	}
	out := make([]int, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
