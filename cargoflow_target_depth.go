package rush

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TargetDepthClass describes the target's initial distance from the top exit.
type TargetDepthClass string

const (
	TargetDepthUnknown TargetDepthClass = "Unknown"
	TargetDepthShallow TargetDepthClass = "Shallow" // top row 1–2
	TargetDepthMedium  TargetDepthClass = "Medium"  // top row 3
	TargetDepthDeep    TargetDepthClass = "Deep"    // top row 4–5
)

// TargetDepthMetrics measures vertical dependency use around the initial target.
// Rows use Rush internal coordinates: row 0 is adjacent to the top exit.
type TargetDepthMetrics struct {
	TargetTopRow                     int              `json:"targetTopRow"`
	TargetDepthClass                 TargetDepthClass `json:"targetDepthClass"`
	ExitCorridorLength               int              `json:"exitCorridorLength"`
	ExitCorridorBlockingPieceCount   int              `json:"exitCorridorBlockingPieceCount"`
	ExitCorridorDependencyPieceCount int              `json:"exitCorridorDependencyPieceCount"`
	ExitCorridorDependencyMoves      int              `json:"exitCorridorDependencyMoves"`
	RowsBetweenTargetAndExitUsed     int              `json:"rowsBetweenTargetAndExitUsed"`
	BelowTargetMeaningfulPieceCount  int              `json:"belowTargetMeaningfulPieceCount"`
	BelowTargetDependencyMoves       int              `json:"belowTargetDependencyMoves"`
	AboveTargetDependencyMoves       int              `json:"aboveTargetDependencyMoves"`
	AboveBelowDependencyBalance      float64          `json:"aboveBelowDependencyBalance"`
}

func ClassifyTargetDepth(topRow int) TargetDepthClass {
	switch {
	case topRow >= 1 && topRow <= 2:
		return TargetDepthShallow
	case topRow == 3:
		return TargetDepthMedium
	case topRow >= 4 && topRow <= 5:
		return TargetDepthDeep
	default:
		return TargetDepthUnknown
	}
}

// ComputeTargetDepthMetrics uses exact optimal-solution movement plus the same
// freeze-necessity test as CoreSpaceMetrics to identify meaningful pieces.
func ComputeTargetDepthMetrics(board *Board, sol Solution, budget SolveBudget) TargetDepthMetrics {
	m := TargetDepthMetrics{}
	if board == nil || len(board.Pieces) == 0 || !sol.Solvable {
		m.TargetDepthClass = TargetDepthUnknown
		return m
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	target := board.Pieces[0]
	top := target.Row(board.Width)
	bottom := top + target.Size - 1
	m.TargetTopRow = top
	m.TargetDepthClass = ClassifyTargetDepth(top)
	m.ExitCorridorLength = top

	meaningful, _ := computeMeaningfulPieces(board, sol, budget)
	corridorPieces := map[int]bool{}
	dependencyCorridorPieces := map[int]bool{}
	rowsUsed := map[int]bool{}
	for i, p := range board.Pieces {
		if i == 0 {
			continue
		}
		inCorridor := pieceIntersectsRegion(p, board.Width, func(r, c int) bool {
			return c == board.exitColumn() && r >= 0 && r < top
		})
		if inCorridor {
			corridorPieces[i] = true
			if meaningful[i] {
				dependencyCorridorPieces[i] = true
			}
		}
		if meaningful[i] {
			below := pieceIntersectsRegion(p, board.Width, func(r, _ int) bool { return r > bottom })
			if below {
				m.BelowTargetMeaningfulPieceCount++
			}
			for _, cell := range pieceOccupancyCells(p, board.Width) {
				r := cell / board.Width
				if r >= 0 && r < top {
					rowsUsed[r] = true
				}
			}
		}
	}
	m.ExitCorridorBlockingPieceCount = len(corridorPieces)

	work := board.Copy()
	for _, mv := range sol.Moves {
		if mv.Piece == 0 {
			work.DoMove(mv)
			continue
		}
		before := work.Pieces[mv.Piece]
		work.DoMove(mv)
		after := work.Pieces[mv.Piece]
		touchesCorridor := pieceIntersectsRegion(before, board.Width, func(r, c int) bool {
			return c == board.exitColumn() && r >= 0 && r < top
		}) || pieceIntersectsRegion(after, board.Width, func(r, c int) bool {
			return c == board.exitColumn() && r >= 0 && r < top
		})
		if touchesCorridor {
			m.ExitCorridorDependencyMoves++
			dependencyCorridorPieces[mv.Piece] = true
		}
		touchesBelow := pieceIntersectsRegion(before, board.Width, func(r, _ int) bool { return r > bottom }) ||
			pieceIntersectsRegion(after, board.Width, func(r, _ int) bool { return r > bottom })
		if touchesBelow {
			m.BelowTargetDependencyMoves++
		}
		touchesAbove := pieceIntersectsRegion(before, board.Width, func(r, _ int) bool { return r < top }) ||
			pieceIntersectsRegion(after, board.Width, func(r, _ int) bool { return r < top })
		if touchesAbove {
			m.AboveTargetDependencyMoves++
		}
	}
	m.ExitCorridorDependencyPieceCount = len(dependencyCorridorPieces)
	m.RowsBetweenTargetAndExitUsed = len(rowsUsed)
	total := m.AboveTargetDependencyMoves + m.BelowTargetDependencyMoves
	if total > 0 {
		m.AboveBelowDependencyBalance = round3(float64(m.BelowTargetDependencyMoves) / float64(total))
	}
	return m
}

func computeMeaningfulPieces(board *Board, sol Solution, budget SolveBudget) (map[int]bool, map[int]bool) {
	meaningful := map[int]bool{}
	moved := map[int]bool{}
	if board == nil {
		return meaningful, moved
	}
	if len(board.Pieces) > 0 {
		meaningful[0] = true
	}
	for _, mv := range sol.Moves {
		moved[mv.Piece] = true
		meaningful[mv.Piece] = true
	}
	for i := 1; i < len(board.Pieces); i++ {
		if moved[i] {
			continue
		}
		rest := board.Copy()
		rest.ImmobilePieces = make([]bool, len(rest.Pieces))
		rest.ImmobilePieces[i] = true
		rsol := rest.SolveWithBudget(budget)
		if rsol.TimedOut || rsol.BudgetExceeded {
			continue
		}
		if !rsol.Solvable || rsol.NumMoves > sol.NumMoves {
			meaningful[i] = true
		}
	}
	return meaningful, moved
}

func ApplyTargetDepthMetricsToBoardSpace(space *BoardSpaceJSON, m TargetDepthMetrics) {
	if space == nil {
		return
	}
	space.TargetTopRow = m.TargetTopRow
	space.TargetDepthClass = string(m.TargetDepthClass)
	space.ExitCorridorLength = m.ExitCorridorLength
	space.ExitCorridorBlockingPieceCount = m.ExitCorridorBlockingPieceCount
	space.ExitCorridorDependencyPieceCount = m.ExitCorridorDependencyPieceCount
	space.ExitCorridorDependencyMoves = m.ExitCorridorDependencyMoves
	space.RowsBetweenTargetAndExitUsed = m.RowsBetweenTargetAndExitUsed
	space.BelowTargetMeaningfulPieceCount = m.BelowTargetMeaningfulPieceCount
	space.BelowTargetDependencyMoves = m.BelowTargetDependencyMoves
	space.AboveBelowDependencyBalance = m.AboveBelowDependencyBalance
}

func pieceIntersectsRegion(p Piece, width int, pred func(row, col int) bool) bool {
	for _, cell := range pieceOccupancyCells(p, width) {
		if pred(cell/width, cell%width) {
			return true
		}
	}
	return false
}

type TargetDepthCalibrationRow struct {
	CandidateID     string `json:"candidateId"`
	HumanLabel      string `json:"humanLabel,omitempty"`
	OptimalGestures int    `json:"optimalGestures"`
	TargetDepthMetrics
	Best6x6MeaningfulContainmentRatio float64 `json:"best6x6MeaningfulContainmentRatio"`
}

// CalibrateTargetDepthOnBatch solves existing Candidate JSON files only.
func CalibrateTargetDepthOnBatch(batchDir string, labels map[string]string, budget SolveBudget) ([]TargetDepthCalibrationRow, map[string]interface{}, error) {
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	candDir := filepath.Join(batchDir, "Candidates")
	entries, err := os.ReadDir(candDir)
	if err != nil {
		return nil, nil, err
	}
	ids := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasPrefix(name, "Candidate_") && strings.HasSuffix(name, ".json") {
			ids = append(ids, strings.TrimSuffix(name, ".json"))
		}
	}
	sort.Strings(ids)
	rows := make([]TargetDepthCalibrationRow, 0, len(ids))
	for _, id := range ids {
		level, err := LoadCargoFlowLevelJSONFile(filepath.Join(candDir, id+".json"))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", id, err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", id, err)
		}
		sol := board.SolveWithBudget(budget)
		if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
			return nil, nil, fmt.Errorf("%s: exact solve failed", id)
		}
		depth := ComputeTargetDepthMetrics(board, sol, budget)
		offX, offY := 1, 0
		if level.Transplant != nil {
			offX, offY = level.Transplant.OffsetX, level.Transplant.OffsetY
		}
		core := ComputeCoreSpaceMetrics(board, offX, offY, sol, budget)
		rows = append(rows, TargetDepthCalibrationRow{
			CandidateID:                       id,
			HumanLabel:                        labels[id],
			OptimalGestures:                   sol.NumMoves,
			TargetDepthMetrics:                depth,
			Best6x6MeaningfulContainmentRatio: core.Best6x6MeaningfulContainmentRatio,
		})
	}
	return rows, summarizeTargetDepthCalibration(rows), nil
}

func summarizeTargetDepthCalibration(rows []TargetDepthCalibrationRow) map[string]interface{} {
	type sums struct {
		n                   int
		top, belowPieces    float64
		belowMoves, balance float64
		containment         float64
	}
	groups := map[string]*sums{}
	depthDist := map[string]int{}
	for _, row := range rows {
		depthDist[string(row.TargetDepthClass)]++
		keys := []string{"all"}
		if row.HumanLabel != "" {
			keys = append(keys, row.HumanLabel)
		}
		for _, key := range keys {
			s := groups[key]
			if s == nil {
				s = &sums{}
				groups[key] = s
			}
			s.n++
			s.top += float64(row.TargetTopRow)
			s.belowPieces += float64(row.BelowTargetMeaningfulPieceCount)
			s.belowMoves += float64(row.BelowTargetDependencyMoves)
			s.balance += row.AboveBelowDependencyBalance
			s.containment += row.Best6x6MeaningfulContainmentRatio
		}
	}
	outGroups := map[string]interface{}{}
	for key, s := range groups {
		if s.n == 0 {
			continue
		}
		outGroups[key] = map[string]interface{}{
			"count":                           s.n,
			"meanTargetTopRow":                round3(s.top / float64(s.n)),
			"meanBelowTargetMeaningfulPieces": round3(s.belowPieces / float64(s.n)),
			"meanBelowTargetDependencyMoves":  round3(s.belowMoves / float64(s.n)),
			"meanAboveBelowBalance":           round3(s.balance / float64(s.n)),
			"meanBest6x6Containment":          round3(s.containment / float64(s.n)),
		}
	}
	supported := false
	reason := "No human-labelled comparison available."
	bad, badOK := groups["visible-6x6"]
	good, goodOK := groups["better-native"]
	if badOK && goodOK && bad.n > 0 && good.n > 0 {
		badTop := bad.top / float64(bad.n)
		goodTop := good.top / float64(good.n)
		badBelow := bad.belowMoves / float64(bad.n)
		goodBelow := good.belowMoves / float64(good.n)
		// Hypothesis predicts visible failures are shallower and have materially less lower dependency use.
		supported = badTop+0.5 <= goodTop && badBelow+1.0 <= goodBelow
		reason = fmt.Sprintf("Human labels: visible mean topRow=%.3f vs better=%.3f; belowMoves=%.3f vs %.3f. Support requires both materially shallower target and fewer lower dependency moves.", badTop, goodTop, badBelow, goodBelow)
	}
	return map[string]interface{}{
		"targetDepthClassDistribution":   depthDist,
		"groups":                         outGroups,
		"targetDepthHypothesisSupported": supported,
		"hypothesisDecisionReason":       reason,
	}
}
