package rush

import (
	"math"
)

// CandidateMetrics are post-solve Cargo Flow features for curation.
type CandidateMetrics struct {
	OptimalGestures                     int               `json:"optimalGestures"`
	VisitedStates                       int               `json:"visitedStates"`
	PieceCount                          int               `json:"pieceCount"`
	Length2Count                        int               `json:"length2Count"`
	Length3Count                        int               `json:"length3Count"`
	StaticCount                         int               `json:"staticCount"`
	HorizontalCount                     int               `json:"horizontalCount"`
	VerticalCount                       int               `json:"verticalCount"`
	InitialTargetBlockerCount           int               `json:"initialTargetBlockerCount"`
	DistinctMovedPieces                 int               `json:"distinctMovedPieces"`
	DistinctNonTargetMovedPieces        int               `json:"distinctNonTargetMovedPieces"`
	TargetMoveCount                     int               `json:"targetMoveCount"`
	NonTargetMovesBeforeFirstTargetMove int               `json:"nonTargetMovesBeforeFirstTargetMove"`
	DependencyDepth                     int               `json:"dependencyDepth"`
	MovedPieceClassHistogram            map[string]int    `json:"movedPieceClassHistogram"`
	OptimalSolutionClassSequence        []string          `json:"optimalSolutionClassSequence"`
	InventorySignature                  string            `json:"inventorySignature"`
	LayoutFingerprint                   string            `json:"layoutFingerprint"`
	BranchingAtStart                    int               `json:"branchingAtStart"`
	AverageLegalMovesAlongOptimalPath   float64           `json:"averageLegalMovesAlongOptimalPath"`
	SolutionPieceReuseRatio             float64           `json:"solutionPieceReuseRatio"`
	DecisionAmbiguity                   float64           `json:"decisionAmbiguity"` // experimental; 0 if skipped
	Signature                           PuzzleSignature   `json:"-"`
}

// DifficultySignals are diagnostic human-difficulty proxies (not true difficulty).
type DifficultySignals struct {
	OptimalGestures                 int     `json:"optimalGestures"`
	SearchSpaceLog                  float64 `json:"searchSpaceLog"`
	DependencyDepth                 int     `json:"dependencyDepth"`
	DistinctMovedPieces             int     `json:"distinctMovedPieces"`
	BranchingAtStart                int     `json:"branchingAtStart"`
	AverageLegalMovesAlongOptimalPath float64 `json:"averageLegalMovesAlongOptimalPath"`
	TargetBlockerCount              int     `json:"targetBlockerCount"`
	SolutionPieceReuseRatio         float64 `json:"solutionPieceReuseRatio"`
	EstimatedHumanDifficultyScore   float64 `json:"estimatedHumanDifficultyScore"`
}

// BuildCandidateMetrics computes metrics from board + exact solution.
// DecisionAmbiguity is left 0 (deferred: would require per-state shortest-path queries).
func BuildCandidateMetrics(board *Board, sol Solution) CandidateMetrics {
	sig := BuildPuzzleSignature(board, sol)
	l2, l3, hCnt, vCnt := 0, 0, 0, 0
	for _, p := range board.Pieces {
		if p.Size == 2 {
			l2++
		} else if p.Size == 3 {
			l3++
		}
		if p.Orientation == Horizontal {
			hCnt++
		} else {
			vCnt++
		}
	}

	branch0 := len(board.Moves(nil))
	avgLegal, reuse := pathStats(board, sol)

	m := CandidateMetrics{
		OptimalGestures:                     sol.NumMoves,
		VisitedStates:                       sol.MemoSize,
		PieceCount:                          len(board.Pieces),
		Length2Count:                        l2,
		Length3Count:                        l3,
		StaticCount:                         len(board.Walls),
		HorizontalCount:                     hCnt,
		VerticalCount:                       vCnt,
		InitialTargetBlockerCount:           sig.InitialTargetBlockerCount,
		DistinctMovedPieces:                 sig.DistinctMovedPieces,
		DistinctNonTargetMovedPieces:        sig.DistinctNonTargetMovedPieces,
		TargetMoveCount:                     sig.TargetMoveCount,
		NonTargetMovesBeforeFirstTargetMove: sig.NonTargetMovesBeforeFirstTargetMove,
		DependencyDepth:                     sig.DependencyDepth,
		MovedPieceClassHistogram:            sig.MovedPieceClassHistogram,
		OptimalSolutionClassSequence:        sig.OptimalSolutionClassSequence,
		InventorySignature:                  sig.InventorySignature,
		LayoutFingerprint:                   layoutFingerprint(board),
		BranchingAtStart:                    branch0,
		AverageLegalMovesAlongOptimalPath:   avgLegal,
		SolutionPieceReuseRatio:             reuse,
		DecisionAmbiguity:                   0, // deferred — too expensive for full pool
		Signature:                           sig,
	}
	return m
}

func pathStats(board *Board, sol Solution) (avgLegal float64, reuseRatio float64) {
	if len(sol.Moves) == 0 {
		return float64(len(board.Moves(nil))), 0
	}
	work := board.Copy()
	sum := 0
	movedCounts := map[int]int{}
	for _, m := range sol.Moves {
		sum += len(work.Moves(nil))
		movedCounts[m.Piece]++
		work.DoMove(m)
	}
	avgLegal = float64(sum) / float64(len(sol.Moves))
	reuse := 0
	for _, c := range movedCounts {
		if c > 1 {
			reuse += c - 1
		}
	}
	reuseRatio = float64(reuse) / float64(len(sol.Moves))
	return avgLegal, reuseRatio
}

// BuildDifficultySignals derives experimental human-difficulty proxies.
func BuildDifficultySignals(m CandidateMetrics) DifficultySignals {
	searchLog := 0.0
	if m.VisitedStates > 0 {
		searchLog = math.Log(float64(m.VisitedStates))
	}
	// Transparent weighted score (normalized rough ranges). Not objective truth.
	// OptimalGestures ~0..25, SearchLog ~0..14, DepDepth ~0..20, Branch ~0..30, Distinct ~0..15
	normOpt := clamp01(float64(m.OptimalGestures) / 20.0)
	normSearch := clamp01(searchLog / 12.0)
	normDep := clamp01(float64(m.DependencyDepth) / 12.0)
	normBranch := clamp01(float64(m.BranchingAtStart) / 20.0)
	normDistinct := clamp01(float64(m.DistinctMovedPieces) / 10.0)
	score := 100 * (0.30*normOpt + 0.25*normSearch + 0.20*normDep + 0.15*normBranch + 0.10*normDistinct)

	return DifficultySignals{
		OptimalGestures:                   m.OptimalGestures,
		SearchSpaceLog:                    searchLog,
		DependencyDepth:                   m.DependencyDepth,
		DistinctMovedPieces:               m.DistinctMovedPieces,
		BranchingAtStart:                  m.BranchingAtStart,
		AverageLegalMovesAlongOptimalPath: m.AverageLegalMovesAlongOptimalPath,
		TargetBlockerCount:                m.InitialTargetBlockerCount,
		SolutionPieceReuseRatio:           m.SolutionPieceReuseRatio,
		EstimatedHumanDifficultyScore:     round3(score),
	}
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// PuzzleFamilyDistance returns 0..1 distance (0 = identical family structure).
// Tuned for Rush-derived Cargo Flow (no 1x1 shuttle bias). Simple explainable weights.
func PuzzleFamilyDistance(a, b PuzzleSignature) float64 {
	seq := sequenceSimilarity(a.OptimalSolutionClassSequence, b.OptimalSolutionClassSequence)
	hist := histogramSimilarity(a.MovedPieceClassHistogram, b.MovedPieceClassHistogram)
	dep := ratioSim(a.DependencyDepth, b.DependencyDepth)
	block := blockerSimilarity(a, b)
	inv := inventoryStringSimilarity(a.InventorySignature, b.InventorySignature)
	tgt := targetPatternSimilarity(a, b)
	opt := ratioSim(a.OptimalGestures, b.OptimalGestures)

	const (
		wSeq  = 0.32
		wInv  = 0.18
		wHist = 0.14
		wBlk  = 0.14
		wDep  = 0.10
		wTgt  = 0.08
		wOpt  = 0.04
	)
	sim := wSeq*seq + wInv*inv + wHist*hist + wBlk*block + wDep*dep + wTgt*tgt + wOpt*opt
	d := 1 - sim
	if d < 0 {
		return 0
	}
	if d > 1 {
		return 1
	}
	return round3(d)
}

// PuzzleFamilySimilarity is 1 - PuzzleFamilyDistance.
func PuzzleFamilySimilarity(a, b PuzzleSignature) float64 {
	return round3(1 - PuzzleFamilyDistance(a, b))
}
