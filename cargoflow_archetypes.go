package rush

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"
)

// PuzzleArchetype names a generation/validation strategy with real constraints.
type PuzzleArchetype string

const (
	ArchetypeCrossLock         PuzzleArchetype = "CrossLock"
	ArchetypeSideChain         PuzzleArchetype = "SideChain"
	ArchetypeLongBlockChain    PuzzleArchetype = "LongBlockChain"
	ArchetypeSmallBlockShuttle PuzzleArchetype = "SmallBlockShuttle"
	ArchetypeStaticGate        PuzzleArchetype = "StaticGate"
)

// AllPuzzleArchetypes returns the POC archetype set in cycle order.
func AllPuzzleArchetypes() []PuzzleArchetype {
	// Order biases early accepts toward structurally distinct families so the
	// functional anti-clone filter does not lock the batch onto one pattern.
	return []PuzzleArchetype{
		ArchetypeLongBlockChain,
		ArchetypeCrossLock,
		ArchetypeStaticGate,
		ArchetypeSideChain,
		ArchetypeSmallBlockShuttle,
	}
}

// ArchetypeProfile is the inventory bias for an archetype.
func ArchetypeProfile(a PuzzleArchetype) CargoFlowInventoryProfile {
	switch a {
	case ArchetypeCrossLock:
		return CargoFlowInventoryProfile{
			Name: string(a), Count1x1: 2, Count1x2H: 3, Count1x2V: 1, Count1x3H: 2, Count1x3V: 0, StaticCount: 3,
		}
	case ArchetypeSideChain:
		return CargoFlowInventoryProfile{
			Name: string(a), Count1x1: 4, Count1x2H: 2, Count1x2V: 2, Count1x3H: 1, Count1x3V: 1, StaticCount: 4,
		}
	case ArchetypeLongBlockChain:
		return CargoFlowInventoryProfile{
			Name: string(a), Count1x1: 1, Count1x2H: 3, Count1x2V: 2, Count1x3H: 2, Count1x3V: 2, StaticCount: 2,
		}
	case ArchetypeSmallBlockShuttle:
		return CargoFlowInventoryProfile{
			Name: string(a), Count1x1: 5, Count1x2H: 2, Count1x2V: 2, Count1x3H: 1, Count1x3V: 1, StaticCount: 4,
		}
	case ArchetypeStaticGate:
		return CargoFlowInventoryProfile{
			Name: string(a), Count1x1: 3, Count1x2H: 2, Count1x2V: 1, Count1x3H: 1, Count1x3V: 1, StaticCount: 8,
		}
	default:
		return DefaultCargoFlowProfiles()[0]
	}
}

// ValidateArchetype checks post-solve constraints. Returns "" if OK.
func ValidateArchetype(a PuzzleArchetype, sig PuzzleSignature, board *Board) RejectionReason {
	switch a {
	case ArchetypeCrossLock:
		hBlock := 0
		for _, c := range sig.DirectTargetBlockerClasses {
			if c == "1x2H" || c == "1x3H" {
				hBlock++
			}
		}
		// Generation places H locks; after deepen/scramble they may leave the corridor.
		// Accept if H pieces were moved in the optimal solution OR remain as blockers.
		hMoved := sig.HistogramCount("1x2H") + sig.HistogramCount("1x3H")
		if sig.InitialTargetBlockerCount < 2 {
			return RejectArchetypeMismatch
		}
		if hBlock < 1 && hMoved < 1 {
			return RejectArchetypeMismatch
		}
		if sig.MovedLongBlocks() < 1 && hMoved < 1 {
			return RejectArchetypeMismatch
		}
	case ArchetypeSideChain:
		if sig.DependencyDepth < 2 {
			return RejectArchetypeMismatch
		}
		if sig.DistinctNonTargetMovedPieces < 2 {
			return RejectArchetypeMismatch
		}
	case ArchetypeLongBlockChain:
		moved3 := sig.HistogramCount("1x3H") + sig.HistogramCount("1x3V")
		moved2 := sig.HistogramCount("1x2H") + sig.HistogramCount("1x2V")
		if moved3+moved2 < 1 {
			return RejectArchetypeMismatch
		}
		if sig.MovedLongBlocks() < 1 {
			return RejectArchetypeMismatch
		}
	case ArchetypeSmallBlockShuttle:
		if sig.HistogramCount("1x1") < 2 {
			return RejectArchetypeMismatch
		}
		// Must not collapse into pure long-block puzzle.
		if sig.MovedLongBlocks() > sig.HistogramCount("1x1")+1 {
			return RejectArchetypeMismatch
		}
	case ArchetypeStaticGate:
		if len(board.Walls) < 3 {
			return RejectArchetypeMismatch
		}
		if !staticAffectsMobility(board) {
			return RejectArchetypeMismatch
		}
	default:
		return RejectArchetypeMismatch
	}
	return ""
}

func staticAffectsMobility(board *Board) bool {
	// At least one wall shares an edge with a movable piece (constrains channels).
	w := board.Width
	wall := map[int]bool{}
	for _, i := range board.Walls {
		wall[i] = true
	}
	for i, p := range board.Pieces {
		if p.Kind == PieceTarget {
			continue
		}
		for c := range pieceCells(board, i) {
			for _, d := range []int{-1, 1, -w, w} {
				n := c + d
				if n < 0 || n >= w*board.Height {
					continue
				}
				// horizontal wrap guard
				if d == -1 || d == 1 {
					if n/w != c/w {
						continue
					}
				}
				if wall[n] {
					return true
				}
			}
		}
	}
	return false
}

func (g *CargoFlowGenerator) tryPlaceArchetype(arch PuzzleArchetype, attemptSeed int64) (*Board, *LevelJSON, RejectionReason) {
	rng := rand.New(rand.NewSource(attemptSeed))
	profile := ArchetypeProfile(arch)
	// Ensure enough density for deepen-to-floor (RUSH-005 lesson).
	if profile.Count1x1 < 4 {
		profile.Count1x1 = 4
	}
	if profile.StaticCount < 3 {
		profile.StaticCount = 3
	}

	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.ExitCol = CargoFlowExitCol

	if !g.placeTargetDeep(board, rng) {
		return nil, nil, RejectInvalidLayout
	}

	var ok bool
	switch arch {
	case ArchetypeCrossLock:
		ok = g.placeCrossLock(board, rng)
	case ArchetypeSideChain:
		ok = g.placeSideChain(board, rng)
	case ArchetypeLongBlockChain:
		ok = g.placeLongBlockChain(board, rng)
	case ArchetypeSmallBlockShuttle:
		ok = g.placeSmallBlockShuttle(board, rng)
	case ArchetypeStaticGate:
		ok = g.placeStaticGate(board, rng)
	default:
		return nil, nil, RejectOther
	}
	if !ok {
		// Fallback: RUSH-005 hardness path with archetype inventory bias.
		// Archetype still enforced post-solve by ValidateArchetype.
		return g.tryPlace(profile, attemptSeed)
	}

	// Same inventory fill style as RUSH-005 tryPlace (subtract already-placed classes).
	if !g.fillRemainingInventory(board, rng, profile) {
		return nil, nil, RejectInvalidLayout
	}

	if err := board.Validate(); err != nil {
		return nil, nil, RejectInvalidLayout
	}
	if clearExitCorridor(board) || board.cargoTargetCanExit() {
		return nil, nil, RejectImmediateVictory
	}

	g.assignLabels(board)
	g.scramble(board, rng, g.cfg.ScrambleMoves)
	if board.cargoTargetCanExit() {
		return nil, nil, RejectImmediateVictory
	}
	if err := board.Validate(); err != nil {
		return nil, nil, RejectInvalidLayout
	}

	level, err := LevelJSONFromBoard(board, "pending", string(arch))
	if err != nil {
		return nil, nil, RejectInvalidLayout
	}
	return board, level, ""
}

func (g *CargoFlowGenerator) fillRemainingInventory(board *Board, rng *rand.Rand, profile CargoFlowInventoryProfile) bool {
	have := map[string]int{}
	for _, p := range board.Pieces {
		have[pieceClass(p)]++
	}
	type spec struct {
		size int
		ori  Orientation
		kind PieceKind
	}
	var specs []spec
	addN := func(key string, n, size int, ori Orientation, kind PieceKind) {
		need := n - have[key]
		for i := 0; i < need; i++ {
			specs = append(specs, spec{size, ori, kind})
		}
	}
	addN("1x1", profile.Count1x1, 1, Horizontal, PieceUnit)
	addN("1x2H", profile.Count1x2H, 2, Horizontal, PieceNormal)
	addN("1x2V", profile.Count1x2V, 2, Vertical, PieceNormal)
	addN("1x3H", profile.Count1x3H, 3, Horizontal, PieceNormal)
	addN("1x3V", profile.Count1x3V, 3, Vertical, PieceNormal)
	rng.Shuffle(len(specs), func(i, j int) { specs[i], specs[j] = specs[j], specs[i] })
	for _, s := range specs {
		if len(board.Pieces) >= MaxPieces {
			break
		}
		_ = g.placeRandom(board, rng, s.size, s.ori, s.kind, 120)
	}
	for len(board.Walls) < profile.StaticCount {
		if !g.placeRandomWall(board, rng, 120) {
			break
		}
	}
	return true
}

func (g *CargoFlowGenerator) placeTargetDeep(board *Board, rng *rand.Rand) bool {
	minRow := 4
	if minRow > CargoFlowHeight-CargoFlowTargetSize {
		minRow = CargoFlowHeight - CargoFlowTargetSize
	}
	rows := []int{}
	for y := minRow; y <= CargoFlowHeight-CargoFlowTargetSize; y++ {
		rows = append(rows, y)
	}
	// Occasional higher target for variation (still ≥2 corridor rows when possible).
	if rng.Float64() < 0.35 {
		for y := 2; y < minRow; y++ {
			rows = append(rows, y)
		}
	}
	rng.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	for _, y := range rows {
		p := Piece{Position: y*CargoFlowWidth + CargoFlowExitCol, Size: CargoFlowTargetSize, Orientation: Vertical, Kind: PieceTarget}
		if board.AddPiece(p) {
			return true
		}
	}
	return false
}

func (g *CargoFlowGenerator) placeCrossLock(board *Board, rng *rand.Rand) bool {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	tgtRow := t.Row(w)
	// Nested corridor for deepen-to-floor hardness.
	if g.placeNestedCorridor(board, rng, 4) < 3 {
		return false
	}
	placedH := 0
	// Side-adjacent H locks that cover exit column where a unit can be "under" conceptually
	// — place H on rows if we can remove a unit temporarily... Instead place H beside
	// that must move for corridor units to escape (covers exit col by spanning).
	for row := 0; row < tgtRow && placedH < 2; row++ {
		cell := row*w + col
		if !board.occupied[cell] {
			continue
		}
		// Try place size-3 H that includes exit column — only works if we free cell.
		// Place vertical-adjacent H one column over that blocks escape: size 2 at col-1.
		for _, x := range []int{col - 2, col - 1, col} {
			if x < 0 {
				continue
			}
			for _, size := range []int{3, 2} {
				if x+size > w {
					continue
				}
				if !(x <= col && col < x+size) {
					continue
				}
				// Cannot overlap occupied corridor unit.
				overlap := false
				for dx := 0; dx < size; dx++ {
					if board.occupied[row*w+x+dx] {
						overlap = true
						break
					}
				}
				if overlap {
					continue
				}
				if board.AddPiece(Piece{Position: row*w + x, Size: size, Orientation: Horizontal, Kind: PieceNormal}) {
					placedH++
					goto next
				}
			}
		}
		// Place H on a free row above nested stack.
	next:
	}
	for row := 0; row < tgtRow && placedH < 2; row++ {
		if board.occupied[row*w+col] {
			continue
		}
		x := maxInt(0, col-1)
		size := 2
		if x+3 <= w {
			size = 3
			x = maxInt(0, col-2)
		}
		if board.AddPiece(Piece{Position: row*w + x, Size: size, Orientation: Horizontal, Kind: PieceNormal}) {
			placedH++
		}
	}
	return placedH >= 1 && !clearExitCorridor(board)
}

func (g *CargoFlowGenerator) placeSideChain(board *Board, rng *rand.Rand) bool {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	tgtRow := t.Row(w)
	if tgtRow < 3 {
		return false
	}
	// Corridor: one H lock + nested units (mix, not pure 1x1 shuttle).
	row := maxInt(0, tgtRow-3)
	x := maxInt(0, col-1)
	_ = board.AddPiece(Piece{Position: row*w + x, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	if g.placeNestedCorridor(board, rng, 3) < 2 {
		return false
	}
	side := col - 1
	if side < 0 || rng.Float64() < 0.5 {
		side = col + 1
	}
	if side < 0 || side >= w {
		return false
	}
	chain := 0
	for r := tgtRow - 1; r >= 0 && chain < 3; r-- {
		cell := r*w + side
		if board.occupied[cell] {
			continue
		}
		if board.AddPiece(Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}) {
			chain++
		}
	}
	return chain >= 2 && !clearExitCorridor(board)
}

func (g *CargoFlowGenerator) placeStaticGate(board *Board, rng *rand.Rand) bool {
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	tgtRow := t.Row(w)
	h := board.Height
	for row := 0; row < h; row++ {
		if row >= tgtRow && row <= tgtRow+1 {
			continue
		}
		if col-2 >= 0 {
			_ = board.AddWall(row*w + col - 2)
		}
		if col+2 < w {
			_ = board.AddWall(row*w + col + 2)
		}
	}
	for _, row := range []int{1, maxInt(1, tgtRow/2)} {
		if row > 0 && row < tgtRow {
			if col-1 >= 0 {
				_ = board.AddWall(row*w + col - 1)
			}
			if col+1 < w {
				_ = board.AddWall(row*w + col + 1)
			}
		}
	}
	// Prefer H corridor locks so solution is not pure 1x1 shuttle (human-ref gate).
	placedH := 0
	for row := 0; row < tgtRow && placedH < 2; row++ {
		x := maxInt(0, col-2)
		size := 3
		if x+size > w {
			size = 2
			x = maxInt(0, col-1)
		}
		if board.AddPiece(Piece{Position: row*w + x, Size: size, Orientation: Horizontal, Kind: PieceNormal}) {
			placedH++
		}
	}
	n := g.placeNestedCorridor(board, rng, 2)
	return placedH >= 1 && n >= 1 && len(board.Walls) >= 4 && !clearExitCorridor(board)
}

func (g *CargoFlowGenerator) placeLongBlockChain(board *Board, rng *rand.Rand) bool {
	if g.placeNestedCorridor(board, rng, 3) < 2 {
		return false
	}
	t := board.Pieces[0]
	w := board.Width
	col := board.exitColumn()
	tgtRow := t.Row(w)
	placed := 0
	for row := 0; row < tgtRow && placed < 3; row++ {
		size := 3
		if rng.Float64() < 0.4 {
			size = 2
		}
		x := maxInt(0, col-size+1)
		if x+size > w {
			continue
		}
		if board.occupied[row*w+col] {
			// Place vertical long beside corridor.
			vx := col - 1
			if vx < 0 {
				vx = col + 1
			}
			if vx >= 0 && vx < w && row+2 <= tgtRow {
				if board.AddPiece(Piece{Position: row*w + vx, Size: 2, Orientation: Vertical, Kind: PieceNormal}) {
					placed++
				}
			}
			continue
		}
		if board.AddPiece(Piece{Position: row*w + x, Size: size, Orientation: Horizontal, Kind: PieceNormal}) {
			placed++
		}
	}
	return placed >= 1 && !clearExitCorridor(board)
}

func (g *CargoFlowGenerator) placeSmallBlockShuttle(board *Board, rng *rand.Rand) bool {
	need := 4
	if g.cfg.MinCorridorBlockers > 0 {
		need = maxInt(3, g.cfg.MinCorridorBlockers)
	}
	n := g.placeNestedCorridor(board, rng, need)
	return n >= 3 && !clearExitCorridor(board)
}

func (g *CargoFlowGenerator) fillInventory(board *Board, rng *rand.Rand, profile CargoFlowInventoryProfile) bool {
	type spec struct {
		size int
		ori  Orientation
		kind PieceKind
	}
	have := map[string]int{}
	for _, p := range board.Pieces {
		have[pieceClass(p)]++
	}
	want := map[string]int{
		"1x1":  profile.Count1x1,
		"1x2H": profile.Count1x2H,
		"1x2V": profile.Count1x2V,
		"1x3H": profile.Count1x3H,
		"1x3V": profile.Count1x3V,
	}
	var specs []spec
	add := func(key string, size int, ori Orientation, kind PieceKind) {
		need := want[key] - have[key]
		for i := 0; i < need; i++ {
			specs = append(specs, spec{size, ori, kind})
		}
	}
	add("1x1", 1, Horizontal, PieceUnit)
	add("1x2H", 2, Horizontal, PieceNormal)
	add("1x2V", 2, Vertical, PieceNormal)
	add("1x3H", 3, Horizontal, PieceNormal)
	add("1x3V", 3, Vertical, PieceNormal)
	rng.Shuffle(len(specs), func(i, j int) { specs[i], specs[j] = specs[j], specs[i] })
	for _, s := range specs {
		if len(board.Pieces) >= MaxPieces {
			break
		}
		if !g.placeRandom(board, rng, s.size, s.ori, s.kind, 100) {
			// Soft fail: continue; inventory is approximate.
			continue
		}
	}
	// Walls up to profile.StaticCount (existing walls from archetype count).
	for len(board.Walls) < profile.StaticCount {
		if !g.placeRandomWall(board, rng, 80) {
			break
		}
	}
	return board.Validate() == nil
}

// LoadHumanRejectedPilotSignatures loads PuzzleSignatures for RUSH-005 refs.
func LoadHumanRejectedPilotSignatures(dir string, ids []string) ([]PuzzleSignature, error) {
	out := make([]PuzzleSignature, 0, len(ids))
	for _, id := range ids {
		level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, id+".json"))
		if err != nil {
			return nil, err
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return nil, err
		}
		sol := board.SolveWithBudget(SolveBudget{
			TimeLimit:  15 * time.Second,
			MaxVisited: 4_000_000,
		})
		if !sol.Solvable {
			return nil, fmt.Errorf("%s not solvable for reference signature", id)
		}
		out = append(out, BuildPuzzleSignature(board, sol))
	}
	return out, nil
}
