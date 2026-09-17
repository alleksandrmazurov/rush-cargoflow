package rush

import (
	"fmt"
	"sort"
	"strings"
)

// AugmentationClass labels native 7×8 structural templates (sequencer-facing).
type AugmentationClass string

const (
	AugNone             AugmentationClass = "None"
	AugSideGate         AugmentationClass = "SideGate"
	AugOuterParking     AugmentationClass = "OuterParking"
	AugCrossDependency  AugmentationClass = "CrossDependency"
	AugLongPieceExt     AugmentationClass = "LongPieceExtension"
	AugOuterShuttle     AugmentationClass = "OuterShuttle"
	AugDoubleCorridor   AugmentationClass = "DoubleCorridor"
	AugSideChamber      AugmentationClass = "SideChamber"
	AugLegacyOuter1x1   AugmentationClass = "LegacyOuter1x1"
)

// NativeAugmentMeta is provenance for a native structural augmentation.
type NativeAugmentMeta struct {
	AugmentationClass        AugmentationClass `json:"augmentationClass"`
	Added1x1Count            int               `json:"added1x1Count"`
	Added1x2Count            int               `json:"added1x2Count"`
	Added1x3Count            int               `json:"added1x3Count"`
	AddedStaticCount         int               `json:"addedStaticCount"`
	BaseOptimal              int               `json:"baseOptimal"`
	NativeOptimal            int               `json:"nativeOptimal"`
	OptimalDelta             int               `json:"optimalDelta"`
	NativeVariantFingerprint string            `json:"nativeVariantFingerprint"`
	OuterZoneEssential       bool              `json:"outerZoneEssential"`
	RestrictedOuterOptimal   int               `json:"restrictedOuterOptimal,omitempty"`
	RelevanceNotes           []string          `json:"relevanceNotes,omitempty"`
}

// nativeProposal is one bounded template proposal before solve.
type nativeProposal struct {
	Class  AugmentationClass
	Pieces []Piece
	Walls  []int
	Labels []string
}

// NativeAugmentConfig bounds native generation search.
type NativeAugmentConfig struct {
	MaxProposalsPerEmbed int
	MaxAcceptedPerEmbed  int
	SoftDeltaMin         int
	SoftDeltaMax         int
	HardShortcutDelta    int // reject if NativeOptimal < BaseOptimal + this (negative)
	PreferTemplates      []AugmentationClass
}

func DefaultNativeAugmentConfig() NativeAugmentConfig {
	return NativeAugmentConfig{
		MaxProposalsPerEmbed: 12,
		MaxAcceptedPerEmbed:  3,
		SoftDeltaMin:         0,
		SoftDeltaMax:         8,
		HardShortcutDelta:    -4, // reject if ≥4 gestures easier
		PreferTemplates: []AugmentationClass{
			AugSideGate, AugOuterParking, AugLongPieceExt, AugCrossDependency,
			AugOuterShuttle, AugDoubleCorridor, AugSideChamber,
		},
	}
}

// GenerateNativeAugmentCandidates proposes bounded native 7×8 augmentations of a Rush-derived core.
func GenerateNativeAugmentCandidates(base *Board, offX, offY, baseOptimal int, budget SolveBudget, cfg NativeAugmentConfig) ([]struct {
	Board  *Board
	Sol    Solution
	Meta   NativeAugmentMeta
}, map[string]int) {
	rejected := map[string]int{}
	if base == nil {
		rejected["NilBase"]++
		return nil, rejected
	}
	if cfg.MaxProposalsPerEmbed <= 0 {
		cfg = DefaultNativeAugmentConfig()
	}
	proposals := proposeNativeTemplates(base, offX, offY, cfg)
	if len(proposals) > cfg.MaxProposalsPerEmbed {
		proposals = proposals[:cfg.MaxProposalsPerEmbed]
	}

	type acc struct {
		Board *Board
		Sol   Solution
		Meta  NativeAugmentMeta
	}
	out := []acc{}
	seenFP := map[string]bool{}

	for _, prop := range proposals {
		if len(out) >= cfg.MaxAcceptedPerEmbed {
			break
		}
		b, okReason := applyNativeProposal(base, prop)
		if b == nil {
			rejected[okReason]++
			continue
		}
		fp := nativeVariantFingerprint(b, prop.Class)
		if seenFP[fp] {
			rejected["DuplicateFingerprint"]++
			continue
		}
		seenFP[fp] = true

		if b.cargoTargetCanExit() {
			rejected["ImmediateVictory"]++
			continue
		}
		if err := b.Validate(); err != nil {
			rejected["ValidateFailed"]++
			continue
		}

		sol := b.SolveWithBudget(budget)
		if sol.TimedOut || sol.BudgetExceeded {
			rejected["DifficultyUnknown"]++
			continue
		}
		if !sol.Solvable {
			rejected["Unsolvable"]++
			continue
		}
		work := b.Copy()
		if err := work.Replay(sol.Moves); err != nil {
			rejected["ReplayFailed"]++
			continue
		}

		delta := sol.NumMoves - baseOptimal
		if delta < cfg.HardShortcutDelta {
			rejected["ShortcutDegradation"]++
			continue
		}

		addedStart := len(base.Pieces)
		if !addedMovablesRelevant(b, base, sol, addedStart, budget, rejected) {
			continue
		}
		if !addedStaticsRelevant(b, base, prop.Walls, sol, budget, rejected) {
			continue
		}

		util := ComputeBoardUtilization(b, offX, offY, &sol)
		essential, restOpt := outerZoneEssentiality(b, offX, offY, sol, budget)
		cls := util.BoardShapeClass
		if (cls == ShapeExpanded || cls == ShapeFullField) && !essential {
			cls = downgradeNonEssentialShape(util, offX, offY)
			util.BoardShapeClass = cls
			rejected["ExpandedWithoutEssentialOuter"]++
		}
		if (cls == ShapeExpanded || cls == ShapeFullField) && !util.OuterZoneRelevant {
			rejected["ExpandedOuterZoneIrrelevant"]++
			continue
		}
		if cls == ShapeExpanded || cls == ShapeFullField {
			if !essential {
				rejected["NativeExpandedNotEssential"]++
				continue
			}
			util.OuterZoneRelevant = true
			util.OuterZoneEvidence = "native-outer-essential"
			util.BoardShapeClass = cls
		}

		meta := countAddedPieces(base, b, prop)
		meta.AugmentationClass = prop.Class
		meta.BaseOptimal = baseOptimal
		meta.NativeOptimal = sol.NumMoves
		meta.OptimalDelta = delta
		meta.NativeVariantFingerprint = fp
		meta.OuterZoneEssential = essential
		meta.RestrictedOuterOptimal = restOpt
		if delta < cfg.SoftDeltaMin || delta > cfg.SoftDeltaMax {
			meta.RelevanceNotes = append(meta.RelevanceNotes,
				fmt.Sprintf("OptimalDelta=%d outside soft preference [%d..%d]", delta, cfg.SoftDeltaMin, cfg.SoftDeltaMax))
		}

		out = append(out, acc{Board: b, Sol: sol, Meta: meta})
		_ = util
	}

	result := make([]struct {
		Board *Board
		Sol   Solution
		Meta  NativeAugmentMeta
	}, len(out))
	for i := range out {
		result[i].Board = out[i].Board
		result[i].Sol = out[i].Sol
		result[i].Meta = out[i].Meta
	}
	return result, rejected
}

func downgradeNonEssentialShape(m BoardUtilizationMetrics, offX, offY int) BoardShapeClass {
	bboxW, bboxH := m.OccupiedBoundingBoxWidth, m.OccupiedBoundingBoxHeight
	if bboxH >= 7 && bboxW <= 6 {
		return ShapeTall
	}
	if bboxW >= 7 && bboxH <= 6 {
		return ShapeWide
	}
	if offY >= 1 || offX != 0 {
		return ShapeShiftedCore
	}
	return ShapeCompact6x6
}

func proposeNativeTemplates(base *Board, offX, offY int, cfg NativeAugmentConfig) []nativeProposal {
	w, h := base.Width, base.Height
	outerFree := []int{}
	for cell := 0; cell < w*h; cell++ {
		if base.occupied[cell] {
			continue
		}
		if !cellInOriginal6x6Core(cell, w, offX, offY) {
			outerFree = append(outerFree, cell)
		}
	}
	sort.Ints(outerFree)

	out := []nativeProposal{}
	templates := cfg.PreferTemplates
	if len(templates) == 0 {
		templates = DefaultNativeAugmentConfig().PreferTemplates
	}

	for _, tmpl := range templates {
		switch tmpl {
		case AugSideGate:
			out = append(out, proposeSideGate(base, outerFree, w, h)...)
		case AugOuterParking:
			out = append(out, proposeOuterParking(base, outerFree, w, h)...)
		case AugLongPieceExt:
			out = append(out, proposeLongPieceExt(base, outerFree, w, h)...)
		case AugCrossDependency:
			out = append(out, proposeCrossDependency(base, outerFree, w, h)...)
		case AugOuterShuttle:
			out = append(out, proposeOuterShuttle(outerFree)...)
		case AugDoubleCorridor:
			out = append(out, proposeDoubleCorridor(base, outerFree, w, h)...)
		case AugSideChamber:
			out = append(out, proposeSideChamber(base, outerFree, w, h)...)
		}
	}
	return out
}

func canPlaceOn(base *Board, p Piece) bool {
	if !pieceFitsBoard(p, base.Width, base.Height) {
		return false
	}
	return !base.isOccupied(p)
}

func proposeSideGate(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	for _, cell := range outer {
		c := cell % w
		if c != 0 && c != w-1 {
			continue
		}
		for _, size := range []int{2, 3} {
			p := Piece{Position: cell, Size: size, Orientation: Vertical, Kind: PieceNormal}
			if canPlaceOn(base, p) {
				out = append(out, nativeProposal{
					Class: AugSideGate, Pieces: []Piece{p},
					Labels: []string{fmt.Sprintf("N%d", size)},
				})
			}
			ph := Piece{Position: cell, Size: size, Orientation: Horizontal, Kind: PieceNormal}
			if c == 0 && canPlaceOn(base, ph) {
				out = append(out, nativeProposal{
					Class: AugSideGate, Pieces: []Piece{ph},
					Labels: []string{fmt.Sprintf("N%dH", size)},
				})
			}
		}
		if len(out) >= 4 {
			break
		}
	}
	_ = h
	return out
}

func proposeOuterParking(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	for _, cell := range outer {
		r := cell / w
		if r != 0 && r != h-1 && r != h-2 {
			continue
		}
		for _, size := range []int{2, 3} {
			p := Piece{Position: cell, Size: size, Orientation: Horizontal, Kind: PieceNormal}
			if canPlaceOn(base, p) {
				out = append(out, nativeProposal{
					Class: AugOuterParking, Pieces: []Piece{p},
					Labels: []string{fmt.Sprintf("P%d", size)},
				})
			}
		}
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func proposeLongPieceExt(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	for _, cell := range outer {
		p := Piece{Position: cell, Size: 3, Orientation: Vertical, Kind: PieceNormal}
		if canPlaceOn(base, p) {
			out = append(out, nativeProposal{Class: AugLongPieceExt, Pieces: []Piece{p}, Labels: []string{"L3V"}})
		}
		p2 := Piece{Position: cell, Size: 3, Orientation: Horizontal, Kind: PieceNormal}
		if canPlaceOn(base, p2) {
			out = append(out, nativeProposal{Class: AugLongPieceExt, Pieces: []Piece{p2}, Labels: []string{"L3H"}})
		}
		if len(out) >= 4 {
			break
		}
	}
	_ = h
	return out
}

func proposeCrossDependency(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	if len(outer) < 2 {
		return out
	}
	for i := 0; i < len(outer) && len(out) < 3; i++ {
		a := outer[i]
		for j := i + 1; j < len(outer) && j < i+8; j++ {
			bcell := outer[j]
			p1 := Piece{Position: a, Size: 2, Orientation: Horizontal, Kind: PieceNormal}
			p2 := Piece{Position: bcell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
			if !canPlaceOn(base, p1) {
				p1 = Piece{Position: a, Size: 2, Orientation: Vertical, Kind: PieceNormal}
			}
			if canPlaceOn(base, p1) && canPlaceOn(base, p2) && !pieceCellsOverlap(p1, p2, w) {
				// Ensure both fit together on a copy.
				tmp := base.Copy()
				if tmp.AddPiece(p1) && tmp.AddPiece(p2) {
					out = append(out, nativeProposal{
						Class:  AugCrossDependency,
						Pieces: []Piece{p1, p2},
						Labels: []string{"X2", "X1"},
					})
					break
				}
			}
		}
	}
	_ = h
	return out
}

func proposeOuterShuttle(outer []int) []nativeProposal {
	out := []nativeProposal{}
	for _, cell := range outer {
		p := Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
		out = append(out, nativeProposal{Class: AugOuterShuttle, Pieces: []Piece{p}, Labels: []string{"S1"}})
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func proposeDoubleCorridor(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	left, right := []int{}, []int{}
	for _, cell := range outer {
		c := cell % w
		if c == 0 {
			left = append(left, cell)
		}
		if c == w-1 {
			right = append(right, cell)
		}
	}
	if len(left) == 0 || len(right) == 0 {
		return out
	}
	p1 := Piece{Position: left[0], Size: 2, Orientation: Vertical, Kind: PieceNormal}
	p2 := Piece{Position: right[0], Size: 2, Orientation: Vertical, Kind: PieceNormal}
	if canPlaceOn(base, p1) && canPlaceOn(base, p2) {
		tmp := base.Copy()
		if tmp.AddPiece(p1) && tmp.AddPiece(p2) {
			out = append(out, nativeProposal{
				Class: AugDoubleCorridor, Pieces: []Piece{p1, p2},
				Labels: []string{"DL", "DR"},
			})
		}
	}
	_ = h
	return out
}

func proposeSideChamber(base *Board, outer []int, w, h int) []nativeProposal {
	out := []nativeProposal{}
	if len(outer) < 2 {
		return out
	}
	wallCell := outer[0]
	unitCell := outer[1]
	if wallCell == unitCell && len(outer) > 2 {
		unitCell = outer[2]
	}
	p := Piece{Position: unitCell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
	if canPlaceOn(base, p) && !base.occupied[wallCell] && wallCell != unitCell {
		out = append(out, nativeProposal{
			Class:  AugSideChamber,
			Pieces: []Piece{p},
			Walls:  []int{wallCell},
			Labels: []string{"C1"},
		})
	}
	for _, cell := range outer {
		if cell == wallCell {
			continue
		}
		lp := Piece{Position: cell, Size: 2, Orientation: Horizontal, Kind: PieceNormal}
		if canPlaceOn(base, lp) && !pieceCoversCell(lp, wallCell, w) && !base.occupied[wallCell] {
			out = append(out, nativeProposal{
				Class:  AugSideChamber,
				Pieces: []Piece{lp},
				Walls:  []int{wallCell},
				Labels: []string{"C2"},
			})
			break
		}
	}
	_ = h
	return out
}

func applyNativeProposal(base *Board, prop nativeProposal) (*Board, string) {
	b := base.Copy()
	for _, cell := range prop.Walls {
		if cell < 0 || cell >= b.Width*b.Height || b.occupied[cell] {
			return nil, "WallOverlap"
		}
		if !b.AddWall(cell) {
			return nil, "WallOverlap"
		}
	}
	for i, p := range prop.Pieces {
		if !pieceFitsBoard(p, b.Width, b.Height) {
			return nil, "OutOfBounds"
		}
		if !b.AddPiece(p) {
			return nil, "Overlap"
		}
		label := "N"
		if i < len(prop.Labels) {
			label = prop.Labels[i]
		}
		b.Labels = append(b.Labels, label)
	}
	return b, ""
}

func pieceFitsBoard(p Piece, w, h int) bool {
	if p.Position < 0 || p.Position >= w*h || p.Size < 1 {
		return false
	}
	r, c := p.Position/w, p.Position%w
	if p.Orientation == Horizontal {
		return c+p.Size <= w
	}
	return r+p.Size <= h
}

func pieceCellsOverlap(a, b Piece, w int) bool {
	set := map[int]bool{}
	idx := a.Position
	stride := a.Stride(w)
	for s := 0; s < a.Size; s++ {
		set[idx] = true
		idx += stride
	}
	idx = b.Position
	stride = b.Stride(w)
	for s := 0; s < b.Size; s++ {
		if set[idx] {
			return true
		}
		idx += stride
	}
	return false
}

func pieceCoversCell(p Piece, cell, w int) bool {
	idx := p.Position
	stride := p.Stride(w)
	for s := 0; s < p.Size; s++ {
		if idx == cell {
			return true
		}
		idx += stride
	}
	return false
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func countAddedPieces(base, native *Board, prop nativeProposal) NativeAugmentMeta {
	m := NativeAugmentMeta{AddedStaticCount: len(prop.Walls)}
	for i := len(base.Pieces); i < len(native.Pieces); i++ {
		p := native.Pieces[i]
		switch {
		case p.Kind == PieceUnit || p.Size == 1:
			m.Added1x1Count++
		case p.Size == 2:
			m.Added1x2Count++
		case p.Size >= 3:
			m.Added1x3Count++
		}
	}
	return m
}

func nativeVariantFingerprint(board *Board, class AugmentationClass) string {
	parts := []string{string(class)}
	for i, p := range board.Pieces {
		parts = append(parts, fmt.Sprintf("P%d:%d:%d:%d:%d", i, p.Position, p.Size, int(p.Orientation), int(p.Kind)))
	}
	walls := append([]int{}, board.Walls...)
	sort.Ints(walls)
	for _, w := range walls {
		parts = append(parts, fmt.Sprintf("W%d", w))
	}
	return strings.Join(parts, "|")
}

// addedMovablesRelevant ensures each added movable is non-decorative.
func addedMovablesRelevant(native, base *Board, sol Solution, addedStart int, budget SolveBudget, rejected map[string]int) bool {
	if addedStart >= len(native.Pieces) {
		return true
	}
	moved := map[int]bool{}
	for _, mv := range sol.Moves {
		if mv.Piece >= addedStart {
			moved[mv.Piece] = true
		}
	}
	for idx := addedStart; idx < len(native.Pieces); idx++ {
		if moved[idx] {
			continue
		}
		// B: immobilize → worse or unsolved
		rest := native.Copy()
		rest.ImmobilePieces = make([]bool, len(rest.Pieces))
		rest.ImmobilePieces[idx] = true
		rsol := rest.SolveWithBudget(budget)
		if rsol.TimedOut || rsol.BudgetExceeded {
			rejected["NecessityUnknown"]++
			return false
		}
		if !rsol.Solvable || rsol.NumMoves > sol.NumMoves {
			continue // relevant via necessity
		}
		rejected["DecorativeAddedMovable"]++
		return false
	}
	return true
}

func addedStaticsRelevant(native, base *Board, walls []int, sol Solution, budget SolveBudget, rejected map[string]int) bool {
	if len(walls) == 0 {
		return true
	}
	// Removing static walls must not leave an equal-or-better solvable board without structural effect.
	cleared := native.Copy()
	// Rebuild without the added walls: copy base pieces+native pieces but drop walls that were added.
	addedWallSet := map[int]bool{}
	for _, w := range walls {
		addedWallSet[w] = true
	}
	nb := base.Copy()
	// Re-add native pieces that aren't walls
	for i := len(base.Pieces); i < len(native.Pieces); i++ {
		p := native.Pieces[i]
		if !nb.AddPiece(p) {
			rejected["StaticRelevanceRebuildFail"]++
			return false
		}
		if i < len(native.Labels) {
			nb.Labels = append(nb.Labels, native.Labels[i])
		}
	}
	// nb has no added walls
	_ = cleared
	rsol := nb.SolveWithBudget(budget)
	if rsol.TimedOut || rsol.BudgetExceeded {
		rejected["StaticRelevanceUnknown"]++
		return false
	}
	// Static is relevant if without it the puzzle is easier or still solvable with shorter/equal path
	// while with wall it's harder — i.e. removing wall improves or enables shortcut.
	if rsol.Solvable && rsol.NumMoves < sol.NumMoves {
		return true // wall made it harder
	}
	if !rsol.Solvable && sol.Solvable {
		return true // wall somehow required for solvability path uniqueness — rare
	}
	// Corridor block heuristic: wall sits adjacent to a piece that moved in optimal.
	w := native.Width
	for _, cell := range walls {
		for _, mv := range sol.Moves {
			p := native.Pieces[mv.Piece]
			cells := pieceOccupancyCells(p, w)
			for _, c := range cells {
				if absInt(c-cell) == 1 || absInt(c-cell) == w {
					return true
				}
			}
		}
	}
	rejected["DecorativeStatic"]++
	return false
}

// outerZoneEssentiality compares normal vs outer-restricted solves.
func outerZoneEssentiality(board *Board, offX, offY int, sol Solution, budget SolveBudget) (bool, int) {
	if !sol.Solvable {
		return false, 0
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
	rsol := sealed.SolveWithBudget(budget)
	if rsol.TimedOut || rsol.BudgetExceeded {
		// Fall back: optimal moves touching outer.
		util := ComputeBoardUtilization(board, offX, offY, &sol)
		return util.OptimalMovesUsingOuterZone > 0, 0
	}
	if !rsol.Solvable {
		return true, -1
	}
	if rsol.NumMoves > sol.NumMoves {
		return true, rsol.NumMoves
	}
	util := ComputeBoardUtilization(board, offX, offY, &sol)
	if util.OptimalMovesUsingOuterZone > 0 {
		return true, rsol.NumMoves
	}
	return false, rsol.NumMoves
}

// RefineBoardShapeForNative applies Expanded/FullField outer-essentiality policy.
func RefineBoardShapeForNative(m BoardUtilizationMetrics, offX, offY int, essential bool) BoardUtilizationMetrics {
	cls := m.BoardShapeClass
	if (cls == ShapeExpanded || cls == ShapeFullField) && (!essential || !m.OuterZoneRelevant) {
		m.BoardShapeClass = downgradeNonEssentialShape(m, offX, offY)
		return m
	}
	// Prefer Expanded when both outer row and column axes used and essential.
	if essential && m.OuterZoneRelevant && m.OuterRowUsage > 0 && m.OuterColumnUsage > 0 {
		bboxW, bboxH := m.OccupiedBoundingBoxWidth, m.OccupiedBoundingBoxHeight
		fullish := bboxW >= CargoFlowWidth && bboxH >= CargoFlowHeight-1 && m.BoardUtilizationRatio >= 0.35
		if fullish {
			m.BoardShapeClass = ShapeFullField
		} else if bboxW >= 7 && bboxH >= 7 {
			m.BoardShapeClass = ShapeExpanded
		}
	}
	return m
}
