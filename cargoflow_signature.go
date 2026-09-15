package rush

import (
	"fmt"
	"sort"
	"strings"
)

// PuzzleSignature is a functional (solution-structure) fingerprint of an
// exact-solved Cargo Flow candidate. Piece IDs are never used — only classes.
type PuzzleSignature struct {
	InventorySignature                  string            `json:"inventorySignature"`
	InitialTargetBlockerCount           int               `json:"initialTargetBlockerCount"`
	DistinctMovedPieces                 int               `json:"distinctMovedPieces"`
	DistinctNonTargetMovedPieces        int               `json:"distinctNonTargetMovedPieces"`
	TargetMoveCount                     int               `json:"targetMoveCount"`
	NonTargetMovesBeforeFirstTargetMove int               `json:"nonTargetMovesBeforeFirstTargetMove"`
	MovedPieceClassHistogram            map[string]int    `json:"movedPieceClassHistogram"`
	OptimalSolutionClassSequence        []string          `json:"optimalSolutionClassSequence"`
	DirectTargetBlockerClasses          []string          `json:"directTargetBlockerClasses"`
	OptimalGestures                     int               `json:"optimalGestures"`
	DependencyDepth                     int               `json:"dependencyDepth"`
	DependencyEdges                     int               `json:"dependencyEdges"`
	TargetBlockerTypes                  []CorridorBlocker `json:"targetBlockerTypes"`
	TargetStartRow                      int               `json:"targetStartRow"`
	SequenceDigest                      string            `json:"sequenceDigest"`
}

// CorridorBlocker describes one direct occupant of the Target exit corridor.
type CorridorBlocker struct {
	Class             string `json:"class"`
	Orientation       string `json:"orientation"`
	EscapeLeft        bool   `json:"escapeLeft"`
	EscapeRight       bool   `json:"escapeRight"`
	EscapeUp          bool   `json:"escapeUp"`
	EscapeDown        bool   `json:"escapeDown"`
	InitiallyMovable  bool   `json:"initiallyMovable"`
}

// BuildPuzzleSignature derives a functional signature from board + exact solution.
func BuildPuzzleSignature(board *Board, sol Solution) PuzzleSignature {
	sig := PuzzleSignature{
		InventorySignature:           inventorySignature(board),
		OptimalGestures:              sol.NumMoves,
		MovedPieceClassHistogram:     map[string]int{},
		OptimalSolutionClassSequence: make([]string, 0, len(sol.Moves)),
		TargetStartRow:               board.Pieces[0].Row(board.Width),
	}

	blockers := analyzeCorridorBlockers(board)
	sig.TargetBlockerTypes = blockers
	sig.InitialTargetBlockerCount = len(blockers)
	classes := make([]string, 0, len(blockers))
	for _, b := range blockers {
		classes = append(classes, b.Class)
	}
	sort.Strings(classes)
	sig.DirectTargetBlockerClasses = classes

	moved := map[int]bool{}
	firstTarget := -1
	work := board.Copy()
	freedByMove := make([]map[int]bool, 0, len(sol.Moves))

	for mi, m := range sol.Moves {
		cls := pieceClass(work.Pieces[m.Piece])
		token := classMoveToken(work, m)
		sig.OptimalSolutionClassSequence = append(sig.OptimalSolutionClassSequence, token)

		beforeCells := pieceCells(work, m.Piece)
		pathCells := movePathCells(work, m)

		if m.Piece == 0 || work.Pieces[m.Piece].Kind == PieceTarget {
			sig.TargetMoveCount++
			if firstTarget < 0 {
				firstTarget = mi
			}
		} else {
			sig.MovedPieceClassHistogram[cls]++
		}
		moved[m.Piece] = true

		work.DoMove(m)
		afterCells := map[int]bool{}
		if !m.Exit {
			for c := range pieceCells(work, m.Piece) {
				afterCells[c] = true
			}
		}
		freed := map[int]bool{}
		for c := range beforeCells {
			if !afterCells[c] {
				freed[c] = true
			}
		}
		// Path cells that were cleared for transit also matter for later movers.
		for c := range pathCells {
			if !afterCells[c] {
				freed[c] = true
			}
		}
		freedByMove = append(freedByMove, freed)
		_ = pathCells
	}

	if firstTarget < 0 {
		sig.NonTargetMovesBeforeFirstTargetMove = len(sol.Moves)
	} else {
		sig.NonTargetMovesBeforeFirstTargetMove = firstTarget
	}
	sig.DistinctMovedPieces = len(moved)
	nonT := 0
	for idx := range moved {
		if idx != 0 && board.Pieces[idx].Kind != PieceTarget {
			nonT++
		}
	}
	sig.DistinctNonTargetMovedPieces = nonT

	depth, edges := approximateDependency(sol.Moves, freedByMove, board)
	sig.DependencyDepth = depth
	sig.DependencyEdges = edges
	sig.SequenceDigest = strings.Join(sig.OptimalSolutionClassSequence, "|")
	return sig
}

func inventorySignature(board *Board) string {
	counts := map[string]int{}
	for _, p := range board.Pieces {
		counts[pieceClass(p)]++
	}
	counts["Static"] = len(board.Walls)
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, counts[k]))
	}
	return strings.Join(parts, ",")
}

func pieceClass(p Piece) string {
	switch p.Kind {
	case PieceTarget:
		return "Target"
	case PieceUnit:
		return "1x1"
	default:
		if p.Orientation == Horizontal {
			return fmt.Sprintf("1x%dH", p.Size)
		}
		return fmt.Sprintf("1x%dV", p.Size)
	}
}

func classMoveToken(board *Board, m Move) string {
	cls := pieceClass(board.Pieces[m.Piece])
	if m.Exit {
		return cls + "-exit"
	}
	dir := moveDir(board, m)
	bucket := distBucket(m.AbsSteps())
	return fmt.Sprintf("%s-%s-%s", cls, dir, bucket)
}

func moveDir(board *Board, m Move) string {
	axis := board.Pieces[m.Piece].Orientation
	if board.Pieces[m.Piece].Kind == PieceUnit {
		axis = m.Axis
	}
	if axis == Horizontal {
		if m.Steps < 0 {
			return "left"
		}
		return "right"
	}
	if m.Steps < 0 {
		return "up"
	}
	return "down"
}

func distBucket(d int) string {
	switch {
	case d <= 1:
		return "1"
	case d == 2:
		return "2"
	default:
		return "3+"
	}
}

func pieceCells(board *Board, index int) map[int]bool {
	out := map[int]bool{}
	p := board.Pieces[index]
	idx := p.Position
	stride := p.Stride(board.Width)
	if p.Kind == PieceUnit {
		stride = 1
		// Units occupy one cell; Size is 1.
	}
	for i := 0; i < p.Size; i++ {
		out[idx] = true
		idx += stride
	}
	if p.Kind == PieceUnit {
		out[p.Position] = true
	}
	return out
}

func movePathCells(board *Board, m Move) map[int]bool {
	out := map[int]bool{}
	if m.Exit {
		p := board.Pieces[m.Piece]
		w := board.Width
		col := p.Col(w)
		for row := 0; row < p.Row(w); row++ {
			out[row*w+col] = true
		}
		return out
	}
	p := board.Pieces[m.Piece]
	axis := p.Orientation
	if p.Kind == PieceUnit {
		axis = m.Axis
	}
	stride := 1
	if axis == Vertical {
		stride = board.Width
	}
	step := 1
	if m.Steps < 0 {
		step = -1
	}
	abs := m.AbsSteps()
	pieceStride := p.Stride(board.Width)
	size := p.Size
	if p.Kind == PieceUnit {
		pieceStride = 1
		size = 1
	}
	for s := 1; s <= abs; s++ {
		for i := 0; i < size; i++ {
			out[p.Position+i*pieceStride+s*step*stride] = true
		}
	}
	return out
}

func analyzeCorridorBlockers(board *Board) []CorridorBlocker {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	tgtRow := t.Row(w)
	out := []CorridorBlocker{}
	for row := 0; row < tgtRow; row++ {
		cell := row*w + col
		if !board.occupied[cell] {
			continue
		}
		// Find piece covering cell (ignore walls — walls on exit col are invalid for CF
		// but may appear off-path; skip walls).
		isWall := false
		for _, wi := range board.Walls {
			if wi == cell {
				isWall = true
				break
			}
		}
		if isWall {
			out = append(out, CorridorBlocker{
				Class: "Static", Orientation: "none",
				InitiallyMovable: false,
			})
			continue
		}
		for i, p := range board.Pieces {
			cells := pieceCells(board, i)
			if !cells[cell] {
				continue
			}
			cls := pieceClass(p)
			ori := "H"
			if p.Orientation == Vertical {
				ori = "V"
			}
			if p.Kind == PieceUnit {
				ori = "both"
			}
			cb := CorridorBlocker{
				Class:            cls,
				Orientation:      ori,
				InitiallyMovable: p.Kind != PieceTarget,
			}
			cb.EscapeLeft, cb.EscapeRight, cb.EscapeUp, cb.EscapeDown = escapeDirs(board, i)
			out = append(out, cb)
			break
		}
	}
	return out
}

func escapeDirs(board *Board, index int) (left, right, up, down bool) {
	moves := board.Moves(nil)
	for _, m := range moves {
		if m.Piece != index || m.Exit {
			continue
		}
		switch moveDir(board, m) {
		case "left":
			left = true
		case "right":
			right = true
		case "up":
			up = true
		case "down":
			down = true
		}
	}
	return
}

func approximateDependency(moves []Move, freed []map[int]bool, board *Board) (depth, edges int) {
	n := len(moves)
	if n == 0 {
		return 0, 0
	}
	// Replay to know path cells needed by each move at decision time.
	work := board.Copy()
	need := make([]map[int]bool, n)
	for i, m := range moves {
		need[i] = movePathCells(work, m)
		// Also destination cells of the piece after move.
		before := pieceCells(work, m.Piece)
		work.DoMove(m)
		after := map[int]bool{}
		if !m.Exit {
			for c := range pieceCells(work, m.Piece) {
				after[c] = true
			}
		}
		for c := range after {
			if !before[c] {
				need[i][c] = true
			}
		}
	}

	adj := make([][]int, n)
	for j := 0; j < n; j++ {
		for i := 0; i < j; i++ {
			if moves[i].Piece == moves[j].Piece {
				continue
			}
			hit := false
			for c := range need[j] {
				if freed[i][c] {
					hit = true
					break
				}
			}
			if hit {
				adj[i] = append(adj[i], j)
				edges++
			}
		}
	}

	// Longest path in DAG (edges only forward in time).
	memo := make([]int, n)
	for i := range memo {
		memo[i] = -1
	}
	var dfs func(int) int
	dfs = func(u int) int {
		if memo[u] >= 0 {
			return memo[u]
		}
		best := 1
		for _, v := range adj[u] {
			if d := dfs(v) + 1; d > best {
				best = d
			}
		}
		memo[u] = best
		return best
	}
	for i := 0; i < n; i++ {
		if d := dfs(i); d > depth {
			depth = d
		}
	}
	if depth == 0 && n > 0 {
		depth = 1
	}
	return depth, edges
}

// FunctionalSimilarity returns 0..1 how alike two solution structures are.
// Deterministic weighted blend of sequence / histogram / dependency / blockers /
// inventory / target-move pattern / corridor-clearing shuttle shape.
// Layout geometry is intentionally excluded.
func FunctionalSimilarity(a, b PuzzleSignature) float64 {
	seq := sequenceSimilarity(a.OptimalSolutionClassSequence, b.OptimalSolutionClassSequence)
	hist := histogramSimilarity(a.MovedPieceClassHistogram, b.MovedPieceClassHistogram)
	dep := ratioSim(a.DependencyDepth, b.DependencyDepth)
	block := blockerSimilarity(a, b)
	inv := inventoryStringSimilarity(a.InventorySignature, b.InventorySignature)
	tgt := targetPatternSimilarity(a, b)
	shuttle := corridorShuttleSimilarity(a, b)

	const (
		wSeq     = 0.22
		wHist    = 0.10
		wDep     = 0.10
		wBlk     = 0.14
		wInv     = 0.08
		wTgt     = 0.08
		wShuttle = 0.28
	)
	s := wSeq*seq + wHist*hist + wDep*dep + wBlk*block + wInv*inv + wTgt*tgt + wShuttle*shuttle
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return round3(s)
}

// corridorShuttleSimilarity captures the RUSH-005 failure mode: stacked 1x1
// exit-corridor clearing with similar shuttle choreography.
func corridorShuttleSimilarity(a, b PuzzleSignature) float64 {
	a1 := oneByOneBlockerFraction(a)
	b1 := oneByOneBlockerFraction(b)
	aH := oneByOneMoveFraction(a)
	bH := oneByOneMoveFraction(b)
	blockCnt := ratioSim(a.InitialTargetBlockerCount, b.InitialTargetBlockerCount)
	// Both "clear central 1x1 corridor" puzzles score high together.
	bothShuttle := 0.0
	if a1 >= 0.6 && b1 >= 0.6 && a.InitialTargetBlockerCount >= 3 && b.InitialTargetBlockerCount >= 3 {
		bothShuttle = 1.0
	} else if a1 >= 0.5 && b1 >= 0.5 {
		bothShuttle = 0.6
	}
	return 0.25*ratioSimFloat(a1, b1) + 0.25*ratioSimFloat(aH, bH) + 0.20*blockCnt + 0.30*bothShuttle
}

func oneByOneBlockerFraction(s PuzzleSignature) float64 {
	if s.InitialTargetBlockerCount == 0 {
		return 0
	}
	n := 0
	for _, c := range s.DirectTargetBlockerClasses {
		if c == "1x1" {
			n++
		}
	}
	return float64(n) / float64(s.InitialTargetBlockerCount)
}

func oneByOneMoveFraction(s PuzzleSignature) float64 {
	total := 0
	for _, v := range s.MovedPieceClassHistogram {
		total += v
	}
	if total == 0 {
		return 0
	}
	return float64(s.HistogramCount("1x1")) / float64(total)
}

func ratioSimFloat(a, b float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if m < 1e-9 {
		return 1
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	return 1 - d/m
}

func sequenceSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	// Class+dir only (drop distance bucket) for coarser human-like match.
	ca := stripDist(a)
	cb := stripDist(b)
	lcs := lcsLen(ca, cb)
	return float64(lcs) / float64(max(len(ca), len(cb)))
}

func stripDist(seq []string) []string {
	out := make([]string, len(seq))
	for i, t := range seq {
		parts := strings.Split(t, "-")
		if len(parts) >= 2 {
			if parts[len(parts)-1] == "exit" {
				out[i] = parts[0] + "-exit"
			} else if len(parts) >= 2 {
				out[i] = parts[0] + "-" + parts[1]
			} else {
				out[i] = t
			}
		} else {
			out[i] = t
		}
	}
	return out
}

func lcsLen(a, b []string) int {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp[n][m]
}

func histogramSimilarity(a, b map[string]int) float64 {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	if len(keys) == 0 {
		return 1
	}
	inter, uni := 0, 0
	for k := range keys {
		av, bv := a[k], b[k]
		if av < bv {
			inter += av
			uni += bv
		} else {
			inter += bv
			uni += av
		}
	}
	if uni == 0 {
		return 1
	}
	return float64(inter) / float64(uni)
}

func ratioSim(a, b int) float64 {
	if a == 0 && b == 0 {
		return 1
	}
	m := maxInt(a, b)
	if m == 0 {
		return 1
	}
	d := a - b
	if d < 0 {
		d = -d
	}
	return 1 - float64(d)/float64(m)
}

func blockerSimilarity(a, b PuzzleSignature) float64 {
	cnt := ratioSim(a.InitialTargetBlockerCount, b.InitialTargetBlockerCount)
	cls := multisetJaccard(a.DirectTargetBlockerClasses, b.DirectTargetBlockerClasses)
	return 0.45*cnt + 0.55*cls
}

func multisetJaccard(a, b []string) float64 {
	ca := map[string]int{}
	cb := map[string]int{}
	for _, x := range a {
		ca[x]++
	}
	for _, x := range b {
		cb[x]++
	}
	keys := map[string]bool{}
	for k := range ca {
		keys[k] = true
	}
	for k := range cb {
		keys[k] = true
	}
	if len(keys) == 0 {
		return 1
	}
	inter, uni := 0, 0
	for k := range keys {
		av, bv := ca[k], cb[k]
		if av < bv {
			inter += av
			uni += bv
		} else {
			inter += bv
			uni += av
		}
	}
	if uni == 0 {
		return 1
	}
	return float64(inter) / float64(uni)
}

func inventoryStringSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	pa := parseInv(a)
	pb := parseInv(b)
	return histogramSimilarity(pa, pb)
}

func parseInv(s string) map[string]int {
	out := map[string]int{}
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		kv := strings.Split(part, ":")
		if len(kv) != 2 {
			continue
		}
		var n int
		fmt.Sscanf(kv[1], "%d", &n)
		out[kv[0]] = n
	}
	return out
}

func targetPatternSimilarity(a, b PuzzleSignature) float64 {
	tMoves := ratioSim(a.TargetMoveCount, b.TargetMoveCount)
	pre := ratioSim(a.NonTargetMovesBeforeFirstTargetMove, b.NonTargetMovesBeforeFirstTargetMove)
	row := ratioSim(a.TargetStartRow, b.TargetStartRow)
	return 0.4*tMoves + 0.4*pre + 0.2*row
}

// HistogramCount returns moved count for a class key (e.g. "1x1", "1x3H").
func (s PuzzleSignature) HistogramCount(class string) int {
	if s.MovedPieceClassHistogram == nil {
		return 0
	}
	return s.MovedPieceClassHistogram[class]
}

// MovedLongBlocks counts moved 1x2/1x3 pieces (any orientation) in the histogram.
func (s PuzzleSignature) MovedLongBlocks() int {
	n := 0
	for k, v := range s.MovedPieceClassHistogram {
		if strings.HasPrefix(k, "1x2") || strings.HasPrefix(k, "1x3") {
			n += v
		}
	}
	return n
}
