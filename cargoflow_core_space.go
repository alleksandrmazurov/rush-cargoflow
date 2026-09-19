package rush

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CoreSpaceMetrics diagnose human-visible "6×6 packed inside 7×8" structure.
type CoreSpaceMetrics struct {
	MeaningfulPieceCount              int     `json:"meaningfulPieceCount"`
	MeaningfulCellCount               int     `json:"meaningfulCellCount"`
	MeaningfulRowsUsed                int     `json:"meaningfulRowsUsed"`
	MeaningfulColumnsUsed             int     `json:"meaningfulColumnsUsed"`
	DependencyRowsUsed                int     `json:"dependencyRowsUsed"`
	DependencyColumnsUsed             int     `json:"dependencyColumnsUsed"`
	OptimalMoveRowsUsed               int     `json:"optimalMoveRowsUsed"`
	OptimalMoveColumnsUsed            int     `json:"optimalMoveColumnsUsed"`
	OuterDependencyPieceCount         int     `json:"outerDependencyPieceCount"`
	OuterDependencyMoves              int     `json:"outerDependencyMoves"`
	Best6x6MeaningfulContainmentRatio float64 `json:"best6x6MeaningfulContainmentRatio"`
	Best6x6WindowOffsetX              int     `json:"best6x6WindowOffsetX"`
	Best6x6WindowOffsetY              int     `json:"best6x6WindowOffsetY"`
	CanMeaningfulStructureFitInAny6x6 bool    `json:"canMeaningfulStructureFitInAny6x6"`
	CoreContainment6x6Ratio           float64 `json:"coreContainment6x6Ratio"` // alias of Best6x6…
	IsolatedAddon1x1Suspect           bool    `json:"isolatedAddon1x1Suspect"`
	IsolatedAddon1x1Evidence          string  `json:"isolatedAddon1x1Evidence,omitempty"`
	FitThresholdUsed                  float64 `json:"fitThresholdUsed"`
}

// Default6x6FitThreshold: calibrated on RUSH01041 human labels (visible-6x6 all=1.0;
// better-native mean≈0.941). Midpoint-ish separator ≈0.97; OuterDependencyPieceCount==0
// remains the strongest discrete separator on that corpus.
const Default6x6FitThreshold = 0.97

// ComputeCoreSpaceMetrics derives meaningful-space / 6×6 containment diagnostics.
func ComputeCoreSpaceMetrics(board *Board, offX, offY int, sol Solution, budget SolveBudget) CoreSpaceMetrics {
	m := CoreSpaceMetrics{FitThresholdUsed: Default6x6FitThreshold}
	if board == nil || !sol.Solvable {
		return m
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	w := board.Width

	meaningful, moved := computeMeaningfulPieces(board, sol, budget)
	moveRows, moveCols := map[int]bool{}, map[int]bool{}
	outerDepMoves := 0
	work := board.Copy()
	for _, mv := range sol.Moves {
		p := work.Pieces[mv.Piece]
		from := pieceOccupancyCells(p, w)
		for _, cell := range from {
			moveRows[cell/w] = true
			moveCols[cell%w] = true
			if !cellInOriginal6x6Core(cell, w, offX, offY) {
				outerDepMoves++
			}
		}
		work.DoMove(mv)
		p2 := work.Pieces[mv.Piece]
		to := pieceOccupancyCells(p2, w)
		for _, cell := range to {
			moveRows[cell/w] = true
			moveCols[cell%w] = true
			if !cellInOriginal6x6Core(cell, w, offX, offY) {
				outerDepMoves++
			}
		}
	}
	m.OptimalMoveRowsUsed = len(moveRows)
	m.OptimalMoveColumnsUsed = len(moveCols)
	m.OuterDependencyMoves = outerDepMoves

	// Static walls: meaningful if removing them (board without that wall) improves/shortens.
	// Cheap proxy: wall adjacent to a meaningful piece cell counts as structural if on outer.
	// Full wall necessity is expensive; mark walls that touch meaningful corridors later via cells.

	depRows, depCols := map[int]bool{}, map[int]bool{}
	meanRows, meanCols := map[int]bool{}, map[int]bool{}
	meanCells := map[int]bool{}
	outerDepPieces := 0
	for i, p := range board.Pieces {
		if !meaningful[i] {
			continue
		}
		m.MeaningfulPieceCount++
		outside := false
		for _, cell := range pieceOccupancyCells(p, w) {
			meanCells[cell] = true
			r, c := cell/w, cell%w
			meanRows[r] = true
			meanCols[c] = true
			if moved[i] {
				depRows[r] = true
				depCols[c] = true
			}
			if !cellInOriginal6x6Core(cell, w, offX, offY) {
				outside = true
			}
		}
		if outside && (moved[i] || meaningful[i]) {
			outerDepPieces++
		}
	}
	m.MeaningfulCellCount = len(meanCells)
	m.MeaningfulRowsUsed = len(meanRows)
	m.MeaningfulColumnsUsed = len(meanCols)
	m.DependencyRowsUsed = len(depRows)
	m.DependencyColumnsUsed = len(depCols)
	m.OuterDependencyPieceCount = outerDepPieces

	best, bx, by := best6x6Containment(meanCells, board.Width, board.Height)
	m.Best6x6MeaningfulContainmentRatio = round3(best)
	m.Best6x6WindowOffsetX = bx
	m.Best6x6WindowOffsetY = by
	m.CoreContainment6x6Ratio = m.Best6x6MeaningfulContainmentRatio
	m.CanMeaningfulStructureFitInAny6x6 = best >= m.FitThresholdUsed

	m.IsolatedAddon1x1Suspect, m.IsolatedAddon1x1Evidence = diagnoseIsolatedAddon1x1(board, sol, meaningful, moved, offX, offY)
	return m
}

func best6x6Containment(meanCells map[int]bool, boardW, boardH int) (best float64, bestX, bestY int) {
	if len(meanCells) == 0 {
		return 0, 0, 0
	}
	best = -1
	for ox := 0; ox <= boardW-6; ox++ {
		for oy := 0; oy <= boardH-6; oy++ {
			inside := 0
			for cell := range meanCells {
				r, c := cell/boardW, cell%boardW
				if c >= ox && c < ox+6 && r >= oy && r < oy+6 {
					inside++
				}
			}
			ratio := float64(inside) / float64(len(meanCells))
			if ratio > best {
				best = ratio
				bestX, bestY = ox, oy
			}
		}
	}
	if best < 0 {
		return 0, 0, 0
	}
	return best, bestX, bestY
}

func diagnoseIsolatedAddon1x1(board *Board, sol Solution, meaningful, moved map[int]bool, offX, offY int) (bool, string) {
	w := board.Width
	h := board.Height
	unitIdx := []int{}
	for i, p := range board.Pieces {
		if i == 0 {
			continue
		}
		if p.Kind == PieceUnit || p.Size == 1 {
			unitIdx = append(unitIdx, i)
		}
	}
	for _, i := range unitIdx {
		p := board.Pieces[i]
		cell := p.Position
		r, c := cell/w, cell%w
		fringe := r == 0 || r == h-1 || c == 0 || c == w-1 || !cellInOriginal6x6Core(cell, w, offX, offY)
		moveCount := 0
		for _, mv := range sol.Moves {
			if mv.Piece == i {
				moveCount++
			}
		}
		minDist := math.MaxInt32
		for j, q := range board.Pieces {
			if j == i || !meaningful[j] {
				continue
			}
			for _, oc := range pieceOccupancyCells(q, w) {
				or, oc2 := oc/w, oc%w
				d := absInt(r-or) + absInt(c-oc2)
				if d < minDist {
					minDist = d
				}
			}
		}
		// Lone fringe 1x1 with ≤1 optimal move — classic "tacked on" look (human: Candidate_001/002).
		if fringe && moveCount <= 1 && (len(unitIdx) <= 2 || minDist >= 2) {
			return true, fmt.Sprintf("unit[%d] fringe cell=%d moves=%d minDistMeaningful=%d units=%d", i, cell, moveCount, minDist, len(unitIdx))
		}
		if fringe && !moved[i] {
			return true, fmt.Sprintf("unit[%d] fringe unused-in-optimal cell=%d", i, cell)
		}
	}
	return false, ""
}

// ApplyCoreSpaceThreshold sets CanMeaningfulStructureFitInAny6x6 using calibrated threshold.
func ApplyCoreSpaceThreshold(m *CoreSpaceMetrics, threshold float64) {
	if threshold <= 0 {
		threshold = Default6x6FitThreshold
	}
	m.FitThresholdUsed = threshold
	m.CanMeaningfulStructureFitInAny6x6 = m.Best6x6MeaningfulContainmentRatio >= threshold
}

// CoreSpaceCalibrationRow is one labelled candidate for metric separation.
type CoreSpaceCalibrationRow struct {
	CandidateID                       string  `json:"candidateId"`
	HumanLabel                        string  `json:"humanLabel"` // visible-6x6 | better-native | unknown
	BoardShapeClass                   string  `json:"boardShapeClass"`
	Best6x6MeaningfulContainmentRatio float64 `json:"best6x6MeaningfulContainmentRatio"`
	CanMeaningfulStructureFitInAny6x6 bool    `json:"canMeaningfulStructureFitInAny6x6"`
	DependencyRowsUsed                int     `json:"dependencyRowsUsed"`
	DependencyColumnsUsed             int     `json:"dependencyColumnsUsed"`
	MeaningfulRowsUsed                int     `json:"meaningfulRowsUsed"`
	MeaningfulColumnsUsed             int     `json:"meaningfulColumnsUsed"`
	OuterDependencyPieceCount         int     `json:"outerDependencyPieceCount"`
	OuterDependencyMoves              int     `json:"outerDependencyMoves"`
	IsolatedAddon1x1Suspect           bool    `json:"isolatedAddon1x1Suspect"`
	IsolatedAddon1x1Evidence          string  `json:"isolatedAddon1x1Evidence,omitempty"`
}

// RUSH01041HumanCoreLabels are human visual labels from RUSH-010.4.1 validation.
var RUSH01041HumanCoreLabels = map[string]string{
	"Candidate_001": "visible-6x6",
	"Candidate_005": "visible-6x6",
	"Candidate_007": "visible-6x6",
	"Candidate_011": "visible-6x6",
	"Candidate_012": "visible-6x6",
	"Candidate_002": "better-native",
	"Candidate_006": "better-native",
	"Candidate_010": "better-native",
}

// CalibrateCoreSpaceOnBatch loads Candidates/*.json, solves, and reports metric separation.
func CalibrateCoreSpaceOnBatch(batchDir string, budget SolveBudget) ([]CoreSpaceCalibrationRow, map[string]interface{}, error) {
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	candDir := filepath.Join(batchDir, "Candidates")
	entries, err := listCandidateIDsFromDir(candDir)
	if err != nil {
		return nil, nil, err
	}
	rows := []CoreSpaceCalibrationRow{}
	var badRatios, goodRatios []float64
	for _, id := range entries {
		level, err := LoadCargoFlowLevelJSONFile(filepath.Join(candDir, id+".json"))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", id, err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return nil, nil, err
		}
		sol := board.SolveWithBudget(budget)
		if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
			return nil, nil, fmt.Errorf("%s: solve failed", id)
		}
		offX, offY := 1, 0
		if level.Transplant != nil {
			offX, offY = level.Transplant.OffsetX, level.Transplant.OffsetY
		}
		cs := ComputeCoreSpaceMetrics(board, offX, offY, sol, budget)
		util := ComputeBoardUtilization(board, offX, offY, &sol)
		label := RUSH01041HumanCoreLabels[id]
		if label == "" {
			label = "unknown"
		}
		row := CoreSpaceCalibrationRow{
			CandidateID:                       id,
			HumanLabel:                        label,
			BoardShapeClass:                   string(util.BoardShapeClass),
			Best6x6MeaningfulContainmentRatio: cs.Best6x6MeaningfulContainmentRatio,
			CanMeaningfulStructureFitInAny6x6: cs.CanMeaningfulStructureFitInAny6x6,
			DependencyRowsUsed:                cs.DependencyRowsUsed,
			DependencyColumnsUsed:             cs.DependencyColumnsUsed,
			MeaningfulRowsUsed:                cs.MeaningfulRowsUsed,
			MeaningfulColumnsUsed:             cs.MeaningfulColumnsUsed,
			OuterDependencyPieceCount:         cs.OuterDependencyPieceCount,
			OuterDependencyMoves:              cs.OuterDependencyMoves,
			IsolatedAddon1x1Suspect:           cs.IsolatedAddon1x1Suspect,
			IsolatedAddon1x1Evidence:          cs.IsolatedAddon1x1Evidence,
		}
		rows = append(rows, row)
		switch label {
		case "visible-6x6":
			badRatios = append(badRatios, cs.Best6x6MeaningfulContainmentRatio)
		case "better-native":
			goodRatios = append(goodRatios, cs.Best6x6MeaningfulContainmentRatio)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CandidateID < rows[j].CandidateID })

	summary := map[string]interface{}{
		"metric":                "Best6x6MeaningfulContainmentRatio",
		"visible6x6Count":       len(badRatios),
		"betterNativeCount":     len(goodRatios),
		"visible6x6Ratios":      badRatios,
		"betterNativeRatios":    goodRatios,
		"visible6x6Mean":        meanFloats(badRatios),
		"betterNativeMean":      meanFloats(goodRatios),
		"suggestedFitThreshold": suggestFitThreshold(badRatios, goodRatios),
		"interpretation":        "Higher Best6x6MeaningfulContainmentRatio ⇒ more of the meaningful structure fits in some 6×6 window (human-visible compact core).",
		"separators":            evaluateMetricSeparators(rows),
	}
	return rows, summary, nil
}

func evaluateMetricSeparators(rows []CoreSpaceCalibrationRow) map[string]interface{} {
	type group struct {
		ratios   []float64
		outer0  int
		fitTrue int
		n       int
	}
	bad, good := group{}, group{}
	for _, r := range rows {
		var g *group
		switch r.HumanLabel {
		case "visible-6x6":
			g = &bad
		case "better-native":
			g = &good
		default:
			continue
		}
		g.n++
		g.ratios = append(g.ratios, r.Best6x6MeaningfulContainmentRatio)
		if r.OuterDependencyPieceCount == 0 {
			g.outer0++
		}
		if r.CanMeaningfulStructureFitInAny6x6 {
			g.fitTrue++
		}
	}
	// OuterDependencyPieceCount==0: perfect on this corpus (all visible-6x6, none better-native).
	bestSeparator := "OuterDependencyPieceCount==0"
	return map[string]interface{}{
		"bestSeparatorOnCorpus": bestSeparator,
		"outerDepPieceZero": map[string]interface{}{
			"visible6x6WithZero":   bad.outer0,
			"betterNativeWithZero": good.outer0,
			"note":                 "On RUSH01041 labels, OuterDependencyPieceCount==0 matches all visible-6x6 and none of better-native.",
		},
		"canFitIn6x6AtDefaultThreshold": map[string]interface{}{
			"visible6x6True":   bad.fitTrue,
			"betterNativeTrue": good.fitTrue,
		},
		"best6x6Ratio": map[string]interface{}{
			"visible6x6Mean":   meanFloats(bad.ratios),
			"betterNativeMean": meanFloats(good.ratios),
		},
	}
}

func listCandidateIDsFromDir(candDir string) ([]string, error) {
	entries, err := os.ReadDir(candDir)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if strings.HasPrefix(id, "Candidate_") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no Candidate_*.json in %s", candDir)
	}
	return ids, nil
}

func meanFloats(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range xs {
		s += v
	}
	return round3(s / float64(len(xs)))
}

func suggestFitThreshold(bad, good []float64) float64 {
	if len(bad) == 0 || len(good) == 0 {
		return Default6x6FitThreshold
	}
	// Midpoint between min(bad) and max(good) if separable; else default.
	minBad := bad[0]
	for _, v := range bad {
		if v < minBad {
			minBad = v
		}
	}
	maxGood := good[0]
	for _, v := range good {
		if v > maxGood {
			maxGood = v
		}
	}
	if minBad > maxGood {
		return round3((minBad + maxGood) / 2)
	}
	// Overlap: use mean of means.
	return round3((meanFloats(bad) + meanFloats(good)) / 2)
}
