package rush

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

const CargoFlowGeneratorVersion = "cf-gen-poc-3"

// CargoFlowGenerationConfig controls offline Cargo Flow candidate generation.
type CargoFlowGenerationConfig struct {
	MasterSeed             int64
	TargetAccepted         int
	MaxAttempts            int
	MinOptimalGestures     int
	MaxVisitedStates       int
	SolveTimeLimit         time.Duration
	SimilarityThreshold    float64 // reject if similarity >= threshold
	MinCorridorBlockers    int     // forced occupied cells in exit column above Target
	ScrambleMoves          int     // random legal non-exit gestures after placement
	Profiles               []CargoFlowInventoryProfile
	GeneratorVersion       string
}

// CargoFlowInventoryProfile names a piece inventory for random placement.
type CargoFlowInventoryProfile struct {
	Name       string
	Count1x1   int
	Count1x2H  int
	Count1x2V  int
	Count1x3H  int
	Count1x3V  int
	StaticCount int
}

func DefaultCargoFlowProfiles() []CargoFlowInventoryProfile {
	// Tuned for exact-solver filter: dense enough to clear minOptimal≈10,
	// but light enough that typical boards finish inside the POC solve budget.
	return []CargoFlowInventoryProfile{
		{
			Name: "MediumDense",
			Count1x1: 5, Count1x2H: 2, Count1x2V: 2, Count1x3H: 1, Count1x3V: 1, StaticCount: 4,
		},
		{
			Name: "HardDense",
			Count1x1: 6, Count1x2H: 2, Count1x2V: 1, Count1x3H: 1, Count1x3V: 1, StaticCount: 5,
		},
		{
			Name: "HardMixed",
			Count1x1: 5, Count1x2H: 2, Count1x2V: 2, Count1x3H: 2, Count1x3V: 1, StaticCount: 4,
		},
	}
}

func DefaultCargoFlowGenerationConfig(seed int64) CargoFlowGenerationConfig {
	return CargoFlowGenerationConfig{
		MasterSeed:          seed,
		TargetAccepted:      10,
		MaxAttempts:         300,
		MinOptimalGestures:  10,
		MaxVisitedStates:    4_000_000,
		SolveTimeLimit:      10 * time.Second,
		SimilarityThreshold: 0.85,
		MinCorridorBlockers: 4,
		ScrambleMoves:       8,
		Profiles:            DefaultCargoFlowProfiles(),
		GeneratorVersion:    CargoFlowGeneratorVersion,
	}
}

// RejectionReason categorizes failed attempts.
type RejectionReason string

const (
	RejectInvalidLayout      RejectionReason = "InvalidLayout"
	RejectImmediateVictory   RejectionReason = "ImmediateVictory"
	RejectUnsolvable         RejectionReason = "Unsolvable"
	RejectTooEasy            RejectionReason = "TooEasy"
	RejectDuplicate          RejectionReason = "Duplicate"
	RejectTooSimilar         RejectionReason = "TooSimilar"
	RejectDifficultyUnknown  RejectionReason = "DifficultyUnknown"
	RejectReplayFailed       RejectionReason = "ReplayFailed"
	RejectOther              RejectionReason = "Other"
)

// CargoFlowAcceptedCandidate is one validated generated puzzle.
type CargoFlowAcceptedCandidate struct {
	CandidateID          string
	Attempt              int
	Profile              string
	AttemptSeed          int64
	Level                *LevelJSON
	OptimalGestures      int
	VisitedStates        int
	ElapsedMs            int64
	Fingerprint          string
	SimilarityToNearest  float64
	Solution             Solution
	SolutionDoc          SolutionJSON
}

// CargoFlowGenerationResult summarizes a pilot batch.
type CargoFlowGenerationResult struct {
	Config           CargoFlowGenerationConfig
	Accepted         []CargoFlowAcceptedCandidate
	Attempts         int
	Rejected         map[RejectionReason]int
	TotalElapsed     time.Duration
	SolveTimesMs     []int64
	ProfileAttempts  map[string]int
}

// CargoFlowGenerator creates Cargo Flow candidates via random valid placement
// followed by exact-solver filtering (difficulty = ExactOptimalGestures only).
type CargoFlowGenerator struct {
	cfg CargoFlowGenerationConfig
	rng *rand.Rand
}

func NewCargoFlowGenerator(cfg CargoFlowGenerationConfig) *CargoFlowGenerator {
	if cfg.GeneratorVersion == "" {
		cfg.GeneratorVersion = CargoFlowGeneratorVersion
	}
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = DefaultCargoFlowProfiles()
	}
	return &CargoFlowGenerator{
		cfg: cfg,
		rng: rand.New(rand.NewSource(cfg.MasterSeed)),
	}
}

// Generate runs a bounded pilot batch.
func (g *CargoFlowGenerator) Generate() CargoFlowGenerationResult {
	start := time.Now()
	res := CargoFlowGenerationResult{
		Config:          g.cfg,
		Rejected:        map[RejectionReason]int{},
		ProfileAttempts: map[string]int{},
		SolveTimesMs:    []int64{},
	}
	seenFP := map[string]bool{}
	acceptedFP := []string{}
	acceptedOcc := [][]bool{}

	for attempt := 1; attempt <= g.cfg.MaxAttempts && len(res.Accepted) < g.cfg.TargetAccepted; attempt++ {
		res.Attempts = attempt
		attemptSeed := g.rng.Int63()
		profile := g.cfg.Profiles[(attempt-1)%len(g.cfg.Profiles)]
		res.ProfileAttempts[profile.Name]++

		board, level, reason := g.tryPlace(profile, attemptSeed)
		if reason != "" {
			res.Rejected[reason]++
			continue
		}

		if board.cargoTargetCanExit() {
			res.Rejected[RejectImmediateVictory]++
			continue
		}

		attemptRng := rand.New(rand.NewSource(attemptSeed ^ int64(-7046029254386353131))) // golden ratio mix
		budget := SolveBudget{
			TimeLimit:  g.cfg.SolveTimeLimit,
			MaxVisited: g.cfg.MaxVisitedStates,
		}

		var sol Solution
		var elapsed time.Duration
		const maxDeepen = 6
		for deepen := 0; deepen <= maxDeepen; deepen++ {
			t0 := time.Now()
			sol = board.SolveWithBudget(budget)
			elapsed = time.Since(t0)
			res.SolveTimesMs = append(res.SolveTimesMs, elapsed.Milliseconds())

			if sol.TimedOut || sol.BudgetExceeded {
				res.Rejected[RejectDifficultyUnknown]++
				sol = Solution{} // mark handled
				break
			}
			if !sol.Solvable {
				res.Rejected[RejectUnsolvable]++
				sol = Solution{}
				break
			}
			if sol.NumMoves >= g.cfg.MinOptimalGestures || deepen == maxDeepen {
				break
			}
			// Too easy: push away from the short solution via mid-path scramble.
			if !g.deepenAwayFromSolution(board, attemptRng, sol) {
				res.Rejected[RejectTooEasy]++
				sol = Solution{}
				break
			}
			if board.cargoTargetCanExit() {
				res.Rejected[RejectImmediateVictory]++
				sol = Solution{}
				break
			}
			var err error
			level, err = LevelJSONFromBoard(board, "pending", profile.Name)
			if err != nil {
				res.Rejected[RejectInvalidLayout]++
				sol = Solution{}
				break
			}
		}
		if sol.NumMoves == 0 && !sol.Solvable {
			continue
		}
		if sol.TimedOut || sol.BudgetExceeded || !sol.Solvable {
			continue
		}
		if sol.NumMoves < g.cfg.MinOptimalGestures {
			res.Rejected[RejectTooEasy]++
			continue
		}

		b2, err := BoardFromLevelJSON(level)
		if err != nil {
			res.Rejected[RejectOther]++
			continue
		}
		if err := b2.Replay(sol.Moves); err != nil {
			res.Rejected[RejectReplayFailed]++
			continue
		}

		fp := layoutFingerprint(board)
		if seenFP[fp] {
			res.Rejected[RejectDuplicate]++
			continue
		}

		occ := occupancyMask(board)
		sim, _ := maxSimilarity(occ, acceptedOcc, acceptedFP, fp)
		if len(acceptedOcc) > 0 && sim >= g.cfg.SimilarityThreshold {
			res.Rejected[RejectTooSimilar]++
			continue
		}

		seenFP[fp] = true
		acceptedFP = append(acceptedFP, fp)
		acceptedOcc = append(acceptedOcc, occ)

		id := fmt.Sprintf("Candidate_%03d", len(res.Accepted)+1)
		level.LevelID = id
		level.Source = fmt.Sprintf("%s/%s/seed=%d", g.cfg.GeneratorVersion, profile.Name, attemptSeed)
		opt := sol.NumMoves
		level.CanonicalGestures = &opt

		doc := ExportSolutionJSON(level, board, sol, elapsed, true)
		res.Accepted = append(res.Accepted, CargoFlowAcceptedCandidate{
			CandidateID:         id,
			Attempt:             attempt,
			Profile:             profile.Name,
			AttemptSeed:         attemptSeed,
			Level:               level,
			OptimalGestures:     sol.NumMoves,
			VisitedStates:       sol.MemoSize,
			ElapsedMs:           elapsed.Milliseconds(),
			Fingerprint:         fp,
			SimilarityToNearest: sim,
			Solution:            sol,
			SolutionDoc:         doc,
		})
	}

	res.TotalElapsed = time.Since(start)
	return res
}

func (g *CargoFlowGenerator) tryPlace(profile CargoFlowInventoryProfile, attemptSeed int64) (*Board, *LevelJSON, RejectionReason) {
	rng := rand.New(rand.NewSource(attemptSeed))
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.ExitCol = CargoFlowExitCol

	// Target deep on the exit column so several nested blockers fit above it.
	minTgtRow := 4
	if minTgtRow > CargoFlowHeight-CargoFlowTargetSize {
		minTgtRow = CargoFlowHeight - CargoFlowTargetSize
	}
	tgtRows := []int{}
	for y := minTgtRow; y <= CargoFlowHeight-CargoFlowTargetSize; y++ {
		tgtRows = append(tgtRows, y)
	}
	rng.Shuffle(len(tgtRows), func(i, j int) { tgtRows[i], tgtRows[j] = tgtRows[j], tgtRows[i] })
	placed := false
	for _, y := range tgtRows {
		p := Piece{Position: y*CargoFlowWidth + CargoFlowExitCol, Size: CargoFlowTargetSize, Orientation: Vertical, Kind: PieceTarget}
		if board.AddPiece(p) {
			placed = true
			break
		}
	}
	if !placed {
		return nil, nil, RejectInvalidLayout
	}

	// Nested corridor: movable units on exit column, each flanked so they cannot
	// simply slide aside without clearing neighbors first.
	need := g.cfg.MinCorridorBlockers
	if need < 2 {
		need = 2
	}
	corridorUnits := g.placeNestedCorridor(board, rng, need)
	if corridorUnits < 1 || clearExitCorridor(board) {
		return nil, nil, RejectInvalidLayout
	}

	type spec struct {
		size int
		ori  Orientation
		kind PieceKind
	}
	// Corridor already consumed some 1x1 units; place the remainder of the profile.
	remain1x1 := profile.Count1x1
	if remain1x1 > corridorUnits {
		remain1x1 -= corridorUnits
	} else {
		remain1x1 = 0
	}
	var specs []spec
	for i := 0; i < remain1x1; i++ {
		specs = append(specs, spec{1, Horizontal, PieceUnit})
	}
	for i := 0; i < profile.Count1x2H; i++ {
		specs = append(specs, spec{2, Horizontal, PieceNormal})
	}
	for i := 0; i < profile.Count1x2V; i++ {
		specs = append(specs, spec{2, Vertical, PieceNormal})
	}
	for i := 0; i < profile.Count1x3H; i++ {
		specs = append(specs, spec{3, Horizontal, PieceNormal})
	}
	for i := 0; i < profile.Count1x3V; i++ {
		specs = append(specs, spec{3, Vertical, PieceNormal})
	}
	rng.Shuffle(len(specs), func(i, j int) { specs[i], specs[j] = specs[j], specs[i] })

	for _, s := range specs {
		if len(board.Pieces) >= MaxPieces {
			break
		}
		if !g.placeRandom(board, rng, s.size, s.ori, s.kind, 120) {
			return nil, nil, RejectInvalidLayout
		}
	}
	for i := 0; i < profile.StaticCount; i++ {
		if !g.placeRandomWall(board, rng, 120) {
			return nil, nil, RejectInvalidLayout
		}
	}

	if err := board.Validate(); err != nil {
		return nil, nil, RejectInvalidLayout
	}
	if clearExitCorridor(board) || board.cargoTargetCanExit() {
		return nil, nil, RejectImmediateVictory
	}

	g.assignLabels(board)
	// Light scramble only — heavy random walks collapse back to short optima.
	g.scramble(board, rng, g.cfg.ScrambleMoves)
	if board.cargoTargetCanExit() {
		return nil, nil, RejectImmediateVictory
	}
	if err := board.Validate(); err != nil {
		return nil, nil, RejectInvalidLayout
	}

	level, err := LevelJSONFromBoard(board, "pending", profile.Name)
	if err != nil {
		return nil, nil, RejectInvalidLayout
	}
	return board, level, ""
}

// placeNestedCorridor puts 1x1 units on the exit column and pins them with
// off-column static neighbors (walls do not consume MaxPieces slots).
func (g *CargoFlowGenerator) placeNestedCorridor(board *Board, rng *rand.Rand, need int) int {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	rows := []int{}
	for row := 0; row < t.Row(w); row++ {
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return 0
	}
	placed := 0
	for i := len(rows) - 1; i >= 0 && placed < need; i-- {
		if len(board.Pieces) >= MaxPieces {
			break
		}
		row := rows[i]
		cell := row*w + col
		p := Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
		if !board.AddPiece(p) {
			continue
		}
		placed++
		sides := []int{}
		if col > 0 {
			sides = append(sides, row*w+col-1)
		}
		if col+1 < w {
			sides = append(sides, row*w+col+1)
		}
		rng.Shuffle(len(sides), func(a, b int) { sides[a], sides[b] = sides[b], sides[a] })
		pinned := 0
		for _, s := range sides {
			if pinned >= 1 && rng.Float64() < 0.4 {
				break
			}
			if board.AddWall(s) {
				pinned++
			}
		}
	}
	return placed
}

func (g *CargoFlowGenerator) placeRandomWall(board *Board, rng *rand.Rand, maxAttempts int) bool {
	n := board.Width * board.Height
	col := board.exitColumn()
	tgtRow := board.Pieces[0].Row(board.Width)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		i := rng.Intn(n)
		row := i / board.Width
		// Keep exit column above Target free of static walls.
		if i%board.Width == col && row < tgtRow {
			continue
		}
		if board.AddWall(i) {
			return true
		}
	}
	return false
}

func (g *CargoFlowGenerator) assignLabels(board *Board) {
	labels := make([]string, len(board.Pieces))
	labels[0] = "target"
	// Zero-padded sequential IDs keep BoardFromLevelJSON's sort-by-ID order
	// identical to board.Pieces indices (required for index-based Replay).
	for i := 1; i < len(board.Pieces); i++ {
		labels[i] = fmt.Sprintf("P%02d", i)
	}
	board.Labels = labels
}

// deepenAwayFromSolution walks along a short exact solution almost to Exit,
// then random-walks. ExactOptimalGestures remains the only difficulty metric.
func (g *CargoFlowGenerator) deepenAwayFromSolution(board *Board, rng *rand.Rand, sol Solution) bool {
	if !sol.Solvable || len(sol.Moves) == 0 {
		g.scramble(board, rng, 60)
		return !board.cargoTargetCanExit()
	}
	work := board.Copy()
	for _, m := range sol.Moves {
		if m.Exit {
			break
		}
		work.DoMove(m)
	}
	// From a near-exit state, scramble into a farther reachable position.
	g.scramble(work, rng, 70+rng.Intn(40))
	if work.cargoTargetCanExit() {
		g.scramble(work, rng, 30)
		if work.cargoTargetCanExit() {
			return false
		}
	}
	board.Pieces = append([]Piece(nil), work.Pieces...)
	board.Walls = append([]int(nil), work.Walls...)
	board.Labels = append([]string(nil), work.Labels...)
	board.occupied = append([]bool(nil), work.occupied...)
	board.won = work.won
	return true
}

// scramble applies random legal non-exit gestures. Length is NOT used as difficulty.
func (g *CargoFlowGenerator) scramble(board *Board, rng *rand.Rand, n int) {
	if n <= 0 {
		return
	}
	buf := make([]Move, 0, 64)
	for i := 0; i < n; i++ {
		moves := board.Moves(buf)
		filtered := moves[:0]
		for _, m := range moves {
			if !m.Exit {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) == 0 {
			return
		}
		board.DoMove(filtered[rng.Intn(len(filtered))])
		buf = moves
	}
}

func clearExitCorridor(board *Board) bool {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	for row := 0; row < t.Row(w); row++ {
		if board.occupied[row*w+col] {
			return false
		}
	}
	return true
}

func (g *CargoFlowGenerator) placeRandom(board *Board, rng *rand.Rand, size int, ori Orientation, kind PieceKind, maxAttempts int) bool {
	if len(board.Pieces) >= MaxPieces {
		return false
	}
	w, h := board.Width, board.Height
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var x, y int
		if ori == Horizontal {
			x = rng.Intn(w - size + 1)
			y = rng.Intn(h)
		} else {
			x = rng.Intn(w)
			y = rng.Intn(h - size + 1)
		}
		p := Piece{Position: y*w + x, Size: size, Orientation: ori, Kind: kind}
		if board.AddPiece(p) {
			return true
		}
	}
	return false
}

// LevelJSONFromBoard exports a Cargo Flow board into Unity-coordinate JSON.
func LevelJSONFromBoard(board *Board, levelID, profile string) (*LevelJSON, error) {
	if board.Rules != RulesCargoFlow {
		return nil, fmt.Errorf("board is not Cargo Flow")
	}
	level := &LevelJSON{
		SchemaVersion:   CargoFlowJSONSchemaVersion,
		LevelID:         levelID,
		Width:           board.Width,
		Height:          board.Height,
		CoordinateSpace: CoordinateSpaceUnity,
		Exit:            ExitJSON{Side: "top", Column: board.exitColumn()},
		Source:          profile,
		Pieces:          make([]PieceJSON, 0, len(board.Pieces)+len(board.Walls)),
	}
	for i, p := range board.Pieces {
		pw, ph := pieceWidth(p), pieceHeight(p)
		ux, uy := RushToUnityOrigin(p.Col(board.Width), p.Row(board.Width), pw, ph, board.Height)
		id := pieceDisplayLabel(board, i)
		typ := "movable1x1"
		movable := true
		switch p.Kind {
		case PieceTarget:
			typ = "target"
		case PieceUnit:
			typ = "movable1x1"
		default:
			if pw*ph == 2 {
				typ = "movable1x2"
			} else {
				typ = "movable1x3"
			}
		}
		level.Pieces = append(level.Pieces, PieceJSON{
			ID: id, Type: typ, X: ux, Y: uy, Width: pw, Height: ph, Movable: movable,
		})
	}
	for i, wi := range board.Walls {
		ux := wi % board.Width
		uyRush := wi / board.Width
		ux2, uy := RushToUnityOrigin(ux, uyRush, 1, 1, board.Height)
		level.Pieces = append(level.Pieces, PieceJSON{
			ID: fmt.Sprintf("S%d", i+1), Type: "static1x1", X: ux2, Y: uy, Width: 1, Height: 1, Movable: false,
		})
	}
	if err := level.Validate(); err != nil {
		return nil, err
	}
	return level, nil
}

func layoutFingerprint(board *Board) string {
	key := CanonicalCargoKey(board)
	// Include static occupancy so different wall layouts differ.
	walls := append([]int(nil), board.Walls...)
	sort.Ints(walls)
	return fmt.Sprintf("%v|won=%v|walls=%v", key.pos, key.won, walls)
}

func occupancyMask(board *Board) []bool {
	m := make([]bool, board.Width*board.Height)
	copy(m, board.occupied)
	return m
}

func mirrorOccupancy(occ []bool, w, h int) []bool {
	out := make([]bool, len(occ))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out[y*w+(w-1-x)] = occ[y*w+x]
		}
	}
	return out
}

func occupancySimilarity(a, b []bool) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	inter, uni := 0, 0
	for i := range a {
		if a[i] || b[i] {
			uni++
		}
		if a[i] && b[i] {
			inter++
		}
	}
	if uni == 0 {
		return 1
	}
	return float64(inter) / float64(uni)
}

func maxSimilarity(occ []bool, accepted [][]bool, fps []string, fp string) (float64, int) {
	best := 0.0
	bestI := -1
	w, h := CargoFlowWidth, CargoFlowHeight
	mir := mirrorOccupancy(occ, w, h)
	for i, other := range accepted {
		s := occupancySimilarity(occ, other)
		sm := occupancySimilarity(mir, other)
		if sm > s {
			s = sm
		}
		if s > best {
			best = s
			bestI = i
		}
		_ = fps
	}
	return best, bestI
}

// ManifestJSON is written for a generation batch.
type ManifestJSON struct {
	GeneratorVersion string                         `json:"generatorVersion"`
	MasterSeed       int64                          `json:"masterSeed"`
	MinOptimal       int                            `json:"minOptimalGestures"`
	MaxAttempts      int                            `json:"maxAttempts"`
	TargetAccepted   int                            `json:"targetAccepted"`
	Attempts         int                            `json:"attempts"`
	AcceptedCount    int                            `json:"accepted"`
	Rejected         map[string]int                 `json:"rejected"`
	TotalElapsedMs   int64                          `json:"totalElapsedMs"`
	Profiles         []string                       `json:"profiles"`
	Candidates       []ManifestCandidateJSON        `json:"candidates"`
}

type ManifestCandidateJSON struct {
	CandidateID         string  `json:"candidateId"`
	Profile             string  `json:"profile"`
	AttemptSeed         int64   `json:"attemptSeed"`
	Attempt             int     `json:"attempt"`
	PieceCount          int     `json:"pieceCount"`
	OptimalGestures     int     `json:"optimalGestures"`
	VisitedStates       int     `json:"visitedStates"`
	ElapsedMs           int64   `json:"elapsedMs"`
	Fingerprint         string  `json:"fingerprint"`
	SimilarityToNearest float64 `json:"similarityToNearest"`
	LevelFile           string  `json:"levelFile"`
	SolutionFile        string  `json:"solutionFile"`
}

func (r CargoFlowGenerationResult) ToManifest() ManifestJSON {
	rej := map[string]int{}
	for k, v := range r.Rejected {
		rej[string(k)] = v
	}
	profiles := make([]string, 0, len(r.Config.Profiles))
	for _, p := range r.Config.Profiles {
		profiles = append(profiles, p.Name)
	}
	m := ManifestJSON{
		GeneratorVersion: r.Config.GeneratorVersion,
		MasterSeed:       r.Config.MasterSeed,
		MinOptimal:       r.Config.MinOptimalGestures,
		MaxAttempts:      r.Config.MaxAttempts,
		TargetAccepted:   r.Config.TargetAccepted,
		Attempts:         r.Attempts,
		AcceptedCount:    len(r.Accepted),
		Rejected:         rej,
		TotalElapsedMs:   r.TotalElapsed.Milliseconds(),
		Profiles:         profiles,
	}
	for _, c := range r.Accepted {
		m.Candidates = append(m.Candidates, ManifestCandidateJSON{
			CandidateID:         c.CandidateID,
			Profile:             c.Profile,
			AttemptSeed:         c.AttemptSeed,
			Attempt:             c.Attempt,
			PieceCount:          len(c.Level.Pieces),
			OptimalGestures:     c.OptimalGestures,
			VisitedStates:       c.VisitedStates,
			ElapsedMs:           c.ElapsedMs,
			Fingerprint:         c.Fingerprint,
			SimilarityToNearest: c.SimilarityToNearest,
			LevelFile:           c.CandidateID + ".json",
			SolutionFile:        c.CandidateID + ".solution.json",
		})
	}
	return m
}

// SolveTimeStats summarizes per-candidate exact-solve wall times.
type SolveTimeStats struct {
	Count   int
	Average float64
	Median  float64
	Max     int64
}

func (r CargoFlowGenerationResult) SolveTimeStats() (SolveTimeStats, bool) {
	if len(r.SolveTimesMs) == 0 {
		return SolveTimeStats{}, false
	}
	return SolveTimeStats{
		Count:   len(r.SolveTimesMs),
		Average: meanInt64(r.SolveTimesMs),
		Median:  medianInt64(r.SolveTimesMs),
		Max:     maxInt64(r.SolveTimesMs),
	}, true
}

func medianInt64(vals []int64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]int64(nil), vals...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	n := len(cp)
	if n%2 == 1 {
		return float64(cp[n/2])
	}
	return float64(cp[n/2-1]+cp[n/2]) / 2
}

func meanInt64(vals []int64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s int64
	for _, v := range vals {
		s += v
	}
	return float64(s) / float64(len(vals))
}

func maxInt64(vals []int64) int64 {
	var m int64
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
