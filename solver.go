package rush

import "time"

type Solution struct {
	Solvable       bool
	Moves          []Move
	NumMoves       int
	NumSteps       int
	Depth          int
	MemoSize       int
	MemoHits       uint64
	TimedOut       bool
	BudgetExceeded bool
}

type Solver struct {
	board     *Board
	target    int
	memo      *Memo
	sa        *StaticAnalyzer
	path      []Move
	moves     [][]Move
	budget    SolveBudget
	deadline  time.Time
	hasBudget bool
	aborted   bool
	abortKind string // "timeout" | "visited"
}

func NewSolverWithStaticAnalyzer(board *Board, sa *StaticAnalyzer) *Solver {
	solver := Solver{}
	solver.board = board
	solver.target = board.Target()
	solver.memo = NewMemo()
	solver.sa = sa
	return &solver
}

func NewSolver(board *Board) *Solver {
	return NewSolverWithStaticAnalyzer(board, theStaticAnalyzer)
}

func (solver *Solver) isSolved() bool {
	if solver.board.Rules == RulesCargoFlow {
		return solver.board.cargoIsSolved()
	}
	return solver.board.Pieces[0].Position == solver.target
}

func (solver *Solver) budgetExceeded() bool {
	if !solver.hasBudget {
		return false
	}
	if solver.budget.TimeLimit > 0 && time.Now().After(solver.deadline) {
		solver.aborted = true
		solver.abortKind = "timeout"
		return true
	}
	if solver.budget.MaxVisited > 0 && solver.memo.Size() >= solver.budget.MaxVisited {
		solver.aborted = true
		solver.abortKind = "visited"
		return true
	}
	return false
}

func (solver *Solver) search(depth, maxDepth, previousPiece int) bool {
	if solver.budgetExceeded() {
		return false
	}
	height := maxDepth - depth
	if height == 0 {
		return solver.isSolved()
	}

	board := solver.board
	if !solver.memo.Add(board.MemoKey(), height) {
		return false
	}

	// Original Rush Hour admissible pruning only.
	if board.Rules == RulesOriginalRush {
		primary := board.Pieces[0]
		i0 := primary.Position + primary.Size
		i1 := solver.target + primary.Size - 1
		minMoves := 0
		for i := i0; i <= i1; i++ {
			if board.occupied[i] {
				minMoves++
			}
		}
		if minMoves >= height {
			return false
		}
	}

	buf := &solver.moves[depth]
	*buf = board.Moves(*buf)
	for _, move := range *buf {
		// Original Rush: consecutive same-piece moves are dominated by multi-cell slides.
		// Cargo Flow 1x1 may legally gesture twice in a row on different axes.
		if board.Rules == RulesOriginalRush && move.Piece == previousPiece {
			continue
		}
		board.DoMove(move)
		solved := solver.search(depth+1, maxDepth, move.Piece)
		board.UndoMove(move)
		if solved {
			solver.memo.Set(board.MemoKey(), height-1)
			solver.path[depth] = move
			return true
		}
		if solver.aborted {
			return false
		}
	}
	return false
}

func (solver *Solver) solve(skipChecks bool) Solution {
	board := solver.board
	memo := solver.memo

	if !skipChecks {
		if err := board.Validate(); err != nil {
			return Solution{}
		}
		if board.Rules == RulesOriginalRush && solver.sa != nil && solver.sa.Impossible(board) {
			return Solution{}
		}
	}

	if solver.isSolved() {
		return Solution{Solvable: true}
	}

	previousMemoSize := 0
	noChange := 0
	cutoff := board.Width - board.Pieces[0].Size
	if board.Rules == RulesCargoFlow {
		cutoff = board.Height
	}
	maxDepth := 0
	if solver.hasBudget && solver.budget.MaxDepth > 0 {
		maxDepth = solver.budget.MaxDepth
	}

	for i := 1; ; i++ {
		if maxDepth > 0 && i > maxDepth {
			return Solution{
				Depth:          i - 1,
				MemoSize:       memo.Size(),
				MemoHits:       memo.Hits(),
				BudgetExceeded: true,
			}
		}
		if solver.budgetExceeded() {
			return Solution{
				Depth:          i - 1,
				MemoSize:       memo.Size(),
				MemoHits:       memo.Hits(),
				TimedOut:       solver.abortKind == "timeout",
				BudgetExceeded: solver.abortKind == "visited" || solver.abortKind == "timeout",
			}
		}
		solver.path = make([]Move, i)
		solver.moves = make([][]Move, i)
		if solver.search(0, i, -1) {
			moves := solver.path
			steps := 0
			for _, move := range moves {
				steps += move.AbsSteps()
			}
			return Solution{
				Solvable: true,
				Moves:    moves,
				NumMoves: len(moves),
				NumSteps: steps,
				Depth:    i,
				MemoSize: memo.Size(),
				MemoHits: memo.Hits(),
			}
		}
		if solver.aborted {
			return Solution{
				Depth:          i,
				MemoSize:       memo.Size(),
				MemoHits:       memo.Hits(),
				TimedOut:       solver.abortKind == "timeout",
				BudgetExceeded: true,
			}
		}
		memoSize := memo.Size()
		if memoSize == previousMemoSize {
			noChange++
		} else {
			noChange = 0
		}
		if !skipChecks && noChange > cutoff {
			return Solution{
				Depth:    i,
				MemoSize: memo.Size(),
				MemoHits: memo.Hits(),
			}
		}
		previousMemoSize = memoSize
	}
}

func (solver *Solver) Solve() Solution {
	return solver.solve(false)
}

func (solver *Solver) UnsafeSolve() Solution {
	return solver.solve(true)
}

// SolveWithBudget runs a bounded search. A successful return without TimedOut/
// BudgetExceeded is a true shortest-path (optimal gesture count).
func (board *Board) SolveWithBudget(budget SolveBudget) Solution {
	solver := NewSolver(board)
	solver.hasBudget = true
	solver.budget = budget
	if budget.TimeLimit > 0 {
		solver.deadline = time.Now().Add(budget.TimeLimit)
	}
	return solver.Solve()
}
