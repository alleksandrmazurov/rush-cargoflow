package rush

import (
	"runtime"
	"sort"
	"time"
)

// cargoKey is a compact, collision-free canonical Cargo Flow state identity.
// Positions are packed after sorting interchangeable pieces within each class.
// Unused slots are 0xFF. won is the Target-exit terminal flag.
type cargoKey struct {
	pos [MaxPieces]uint8
	won bool
}

type cargoClass struct {
	kind        PieceKind
	size        int
	orientation Orientation
	indexes     []int
}

type cargoBFSStats struct {
	Expanded         int
	Generated        int
	Duplicates       int
	PeakFrontier     int
	Unique           int
	AllocBytesBefore uint64
	AllocBytesAfter  uint64
	MallocsBefore    uint64
	MallocsAfter     uint64
	NumGCBefore      uint32
	NumGCAfter       uint32
}

// SolveStats is instrumentation from the most recent Cargo Flow BFS solve.
type SolveStats struct {
	ExpandedStates      int
	GeneratedSuccessors int
	DuplicateRejected   int
	UniqueStates        int
	PeakFrontier        int
	AllocatedBytes      uint64
	Mallocs             uint64
	NumGC               uint32
}

var lastCargoStats SolveStats

// LastCargoSolveStats returns instrumentation from the most recent Cargo Flow BFS.
func LastCargoSolveStats() SolveStats { return lastCargoStats }

func (st cargoBFSStats) toSolveStats() SolveStats {
	return SolveStats{
		ExpandedStates:      st.Expanded,
		GeneratedSuccessors: st.Generated,
		DuplicateRejected:   st.Duplicates,
		UniqueStates:        st.Unique,
		PeakFrontier:        st.PeakFrontier,
		AllocatedBytes:      st.AllocBytesAfter - st.AllocBytesBefore,
		Mallocs:             st.MallocsAfter - st.MallocsBefore,
		NumGC:               st.NumGCAfter - st.NumGCBefore,
	}
}

func buildCargoClasses(board *Board) []cargoClass {
	type ck struct {
		k PieceKind
		s int
		o Orientation
	}
	order := make([]ck, 0)
	groups := make(map[ck][]int)
	for i, p := range board.Pieces {
		c := ck{p.Kind, p.Size, p.Orientation}
		if _, ok := groups[c]; !ok {
			order = append(order, c)
		}
		groups[c] = append(groups[c], i)
	}
	out := make([]cargoClass, 0, len(order))
	for _, c := range order {
		out = append(out, cargoClass{
			kind:        c.k,
			size:        c.s,
			orientation: c.o,
			indexes:     groups[c],
		})
	}
	return out
}

// CanonicalCargoKey builds a permutation-invariant key for interchangeable pieces.
func CanonicalCargoKey(board *Board) cargoKey {
	return encodeCargoKeyFromPositions(positionsOf(board), board.won, buildCargoClasses(board))
}

func positionsOf(board *Board) []int {
	pos := make([]int, len(board.Pieces))
	for i := range board.Pieces {
		pos[i] = board.Pieces[i].Position
	}
	return pos
}

func encodeCargoKeyFromPositions(positions []int, won bool, classes []cargoClass) cargoKey {
	var key cargoKey
	for i := range key.pos {
		key.pos[i] = 0xFF
	}
	key.won = won
	slot := 0
	buf := make([]int, 0, MaxPieces)
	for _, cl := range classes {
		buf = buf[:0]
		for _, idx := range cl.indexes {
			buf = append(buf, positions[idx])
		}
		sort.Ints(buf)
		for _, p := range buf {
			key.pos[slot] = uint8(p)
			slot++
		}
	}
	return key
}

// encodeCargoKeyInto fills key using reusable sort buffer.
func encodeCargoKeyInto(positions []int, won bool, classes []cargoClass, sortBuf []int, key *cargoKey) {
	for i := range key.pos {
		key.pos[i] = 0xFF
	}
	key.won = won
	slot := 0
	for _, cl := range classes {
		sortBuf = sortBuf[:0]
		for _, idx := range cl.indexes {
			sortBuf = append(sortBuf, positions[idx])
		}
		sort.Ints(sortBuf)
		for _, p := range sortBuf {
			key.pos[slot] = uint8(p)
			slot++
		}
	}
}

type cargoItem struct {
	pos    []int
	won    bool
	parent int
	move   Move
}

// solveCargoBFS finds a shortest gesture path for Cargo Flow using BFS over
// canonical (permutation-invariant) state keys.
func (board *Board) solveCargoBFS(budget SolveBudget, skipChecks bool) Solution {
	if !skipChecks {
		if err := board.Validate(); err != nil {
			return Solution{}
		}
	}
	if board.cargoIsSolved() {
		return Solution{Solvable: true}
	}

	classes := buildCargoClasses(board)
	nPieces := len(board.Pieces)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	stats := cargoBFSStats{
		AllocBytesBefore: ms.TotalAlloc,
		MallocsBefore:    ms.Mallocs,
		NumGCBefore:      ms.NumGC,
	}

	startPos := make([]int, nPieces)
	for i := range board.Pieces {
		startPos[i] = board.Pieces[i].Position
	}
	var startKey cargoKey
	sortBuf := make([]int, 0, MaxPieces)
	encodeCargoKeyInto(startPos, board.won, classes, sortBuf, &startKey)

	visited := make(map[cargoKey]int, 8192)
	nodes := make([]cargoItem, 0, 8192)
	nodes = append(nodes, cargoItem{
		pos:    append([]int(nil), startPos...),
		won:    board.won,
		parent: -1,
	})
	visited[startKey] = 0

	work := board.Copy()
	moveBuf := make([]Move, 0, 64)
	posScratch := make([]int, nPieces)
	var nextKey cargoKey

	deadline := time.Time{}
	hasTime := budget.TimeLimit > 0
	if hasTime {
		deadline = time.Now().Add(budget.TimeLimit)
	}
	maxVisited := budget.MaxVisited

	head := 0
	for head < len(nodes) {
		if hasTime && time.Now().After(deadline) {
			finishCargoStats(&stats)
			stats.Unique = len(visited)
			lastCargoStats = stats.toSolveStats()
			return Solution{TimedOut: true, BudgetExceeded: true, MemoSize: len(visited)}
		}
		if maxVisited > 0 && len(visited) >= maxVisited {
			finishCargoStats(&stats)
			stats.Unique = len(visited)
			lastCargoStats = stats.toSolveStats()
			return Solution{BudgetExceeded: true, MemoSize: len(visited)}
		}
		frontierSize := len(nodes) - head
		if frontierSize > stats.PeakFrontier {
			stats.PeakFrontier = frontierSize
		}

		curIdx := head
		cur := nodes[head]
		head++
		stats.Expanded++

		applyPositions(work, cur.pos, cur.won)
		moveBuf = work.Moves(moveBuf[:0])
		for _, m := range moveBuf {
			stats.Generated++
			work.DoMove(m)
			for i := range work.Pieces {
				posScratch[i] = work.Pieces[i].Position
			}
			newWon := work.won
			encodeCargoKeyInto(posScratch, newWon, classes, sortBuf, &nextKey)
			work.UndoMove(m)

			if _, ok := visited[nextKey]; ok {
				stats.Duplicates++
				continue
			}
			visited[nextKey] = len(nodes)
			newPos := make([]int, nPieces)
			copy(newPos, posScratch)
			nodes = append(nodes, cargoItem{
				pos:    newPos,
				won:    newWon,
				parent: curIdx,
				move:   m,
			})
			if newWon {
				finishCargoStats(&stats)
				stats.Unique = len(visited)
				sol := reconstructCargoSolution(nodes, len(nodes)-1)
				sol.MemoSize = len(visited)
				lastCargoStats = stats.toSolveStats()
				return sol
			}
		}
	}

	finishCargoStats(&stats)
	stats.Unique = len(visited)
	lastCargoStats = stats.toSolveStats()
	return Solution{MemoSize: len(visited)}
}

func finishCargoStats(stats *cargoBFSStats) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	stats.AllocBytesAfter = ms.TotalAlloc
	stats.MallocsAfter = ms.Mallocs
	stats.NumGCAfter = ms.NumGC
}

func applyPositions(board *Board, pos []int, won bool) {
	for i := range board.occupied {
		board.occupied[i] = false
	}
	for _, w := range board.Walls {
		board.occupied[w] = true
	}
	for i := range board.Pieces {
		board.Pieces[i].Position = pos[i]
		board.memoKey[i] = pos[i]
		p := board.Pieces[i]
		stride := p.Stride(board.Width)
		if p.Kind == PieceUnit {
			stride = 1
		}
		idx := p.Position
		for j := 0; j < p.Size; j++ {
			board.occupied[idx] = true
			idx += stride
		}
	}
	board.won = won
}

func reconstructCargoSolution(nodes []cargoItem, idx int) Solution {
	var moves []Move
	for idx > 0 {
		moves = append(moves, nodes[idx].move)
		idx = nodes[idx].parent
	}
	for i, j := 0, len(moves)-1; i < j; i, j = i+1, j-1 {
		moves[i], moves[j] = moves[j], moves[i]
	}
	steps := 0
	for _, m := range moves {
		steps += m.AbsSteps()
	}
	return Solution{
		Solvable: true,
		Moves:    moves,
		NumMoves: len(moves),
		NumSteps: steps,
		Depth:    len(moves),
	}
}
