package rush

import (
	"fmt"
	"time"
)

// CoreExpansionClass labels structural core-expansion transforms (not decorative fill).
type CoreExpansionClass string

const (
	CoreExpNone               CoreExpansionClass = "None"
	CoreExpDependencyPullLeft CoreExpansionClass = "DependencyPullLeft"
	CoreExpDependencyPullRight CoreExpansionClass = "DependencyPullRight"
	CoreExpBottomExtension    CoreExpansionClass = "BottomExtension"
	CoreExpTopSideExtension   CoreExpansionClass = "TopSideExtension"
	CoreExpSplitCoreVertical  CoreExpansionClass = "SplitCoreVertical"
	CoreExpSplitCoreHorizontal CoreExpansionClass = "SplitCoreHorizontal"
	CoreExpOuterLongBlockGate CoreExpansionClass = "OuterLongBlockGate"
	CoreExpSideChamberChain   CoreExpansionClass = "SideChamberChain"
	CoreExpCrossFieldDependency CoreExpansionClass = "CrossFieldDependency"
)

// CoreExpansionMeta provenance for a structural expansion result.
type CoreExpansionMeta struct {
	ExpansionClass     CoreExpansionClass `json:"expansionClass"`
	BaseOptimal        int                `json:"baseOptimal"`
	NativeOptimal      int                `json:"nativeOptimal"`
	OptimalDelta       int                `json:"optimalDelta"`
	PiecesRelocated    int                `json:"piecesRelocated"`
	CoreSpace          CoreSpaceMetrics   `json:"coreSpace"`
	RejectedReason     string             `json:"rejectedReason,omitempty"`
}

// CoreExpansionConfig bounds structural expansion search.
type CoreExpansionConfig struct {
	MaxProposalsPerBoard int
	MaxAcceptedPerBoard  int
	SoftDeltaMin         int
	SoftDeltaMax         int
	HardShortcutDelta    int
	FitThreshold         float64
	RequireNotFitIn6x6   bool // prefer/require CanMeaningfulStructureFitInAny6x6=false
}

func DefaultCoreExpansionConfig() CoreExpansionConfig {
	return CoreExpansionConfig{
		MaxProposalsPerBoard: 16,
		MaxAcceptedPerBoard:  2,
		SoftDeltaMin:         -1,
		SoftDeltaMax:         8,
		HardShortcutDelta:    -4,
		FitThreshold:         Default6x6FitThreshold,
		RequireNotFitIn6x6:   false, // soft during generation; pilot reports hard goals
	}
}

type coreExpansionProposal struct {
	Class      CoreExpansionClass
	Relocate   []struct{ FromIdx, NewPos int; Ori Orientation; Size int }
	AddPieces  []Piece
	AddWalls   []int
	Labels     []string
}

// GenerateCoreExpansionCandidates relocates dependency pieces into outer 7×8 space.
func GenerateCoreExpansionCandidates(base *Board, offX, offY, baseOptimal int, budget SolveBudget, cfg CoreExpansionConfig) ([]struct {
	Board *Board
	Sol   Solution
	Meta  CoreExpansionMeta
}, map[string]int) {
	rejected := map[string]int{}
	if base == nil {
		rejected["NilBase"]++
		return nil, rejected
	}
	if cfg.MaxProposalsPerBoard <= 0 {
		cfg = DefaultCoreExpansionConfig()
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	baseSol := base.SolveWithBudget(budget)
	if !baseSol.Solvable || baseSol.TimedOut || baseSol.BudgetExceeded {
		rejected["BaseUnsolved"]++
		return nil, rejected
	}
	if baseOptimal <= 0 {
		baseOptimal = baseSol.NumMoves
	}

	proposals := proposeCoreExpansions(base, baseSol, offX, offY, cfg)
	if len(proposals) > cfg.MaxProposalsPerBoard {
		proposals = proposals[:cfg.MaxProposalsPerBoard]
	}

	type acc struct {
		Board *Board
		Sol   Solution
		Meta  CoreExpansionMeta
	}
	out := []acc{}
	seen := map[string]bool{}

	for _, prop := range proposals {
		if len(out) >= cfg.MaxAcceptedPerBoard {
			break
		}
		nb, reason := applyCoreExpansion(base, prop)
		if nb == nil {
			rejected[reason]++
			continue
		}
		if nb.cargoTargetCanExit() {
			rejected["ImmediateVictory"]++
			continue
		}
		if err := nb.Validate(); err != nil {
			rejected["ValidateFailed"]++
			continue
		}
		fp := nativeVariantFingerprint(nb, AugmentationClass(prop.Class))
		if seen[fp] {
			rejected["DuplicateFingerprint"]++
			continue
		}
		seen[fp] = true

		sol := nb.SolveWithBudget(budget)
		if sol.TimedOut || sol.BudgetExceeded {
			rejected["DifficultyUnknown"]++
			continue
		}
		if !sol.Solvable {
			rejected["Unsolvable"]++
			continue
		}
		work := nb.Copy()
		if err := work.Replay(sol.Moves); err != nil {
			rejected["ReplayFailed"]++
			continue
		}
		delta := sol.NumMoves - baseOptimal
		if delta < cfg.HardShortcutDelta {
			rejected["ShortcutDegradation"]++
			continue
		}
		// Relocated pieces must remain relevant.
		if !relocatedPiecesRelevant(base, nb, prop, sol, budget) {
			rejected["RelocatedIrrelevant"]++
			continue
		}
		cs := ComputeCoreSpaceMetrics(nb, offX, offY, sol, budget)
		ApplyCoreSpaceThreshold(&cs, cfg.FitThreshold)
		if cfg.RequireNotFitIn6x6 && cs.CanMeaningfulStructureFitInAny6x6 {
			rejected["StillFitsIn6x6"]++
			continue
		}
		// Prefer improvement vs base containment when possible (soft).
		baseCS := ComputeCoreSpaceMetrics(base, offX, offY, baseSol, budget)
		if cs.Best6x6MeaningfulContainmentRatio > baseCS.Best6x6MeaningfulContainmentRatio+0.02 &&
			cs.CanMeaningfulStructureFitInAny6x6 {
			// No improvement in distribution — soft reject only if we already have acceptances.
			if len(out) > 0 {
				rejected["NoContainmentImprovement"]++
				continue
			}
		}

		meta := CoreExpansionMeta{
			ExpansionClass:  prop.Class,
			BaseOptimal:     baseOptimal,
			NativeOptimal:   sol.NumMoves,
			OptimalDelta:    delta,
			PiecesRelocated: len(prop.Relocate),
			CoreSpace:       cs,
		}
		out = append(out, acc{Board: nb, Sol: sol, Meta: meta})
	}

	result := make([]struct {
		Board *Board
		Sol   Solution
		Meta  CoreExpansionMeta
	}, len(out))
	for i := range out {
		result[i].Board = out[i].Board
		result[i].Sol = out[i].Sol
		result[i].Meta = out[i].Meta
	}
	return result, rejected
}

func proposeCoreExpansions(base *Board, sol Solution, offX, offY int, cfg CoreExpansionConfig) []coreExpansionProposal {
	w := base.Width
	h := base.Height
	moved := map[int]bool{}
	for _, mv := range sol.Moves {
		if mv.Piece > 0 {
			moved[mv.Piece] = true
		}
	}
	out := []coreExpansionProposal{}

	// DependencyPullLeft/Right: relocate a moved Normal piece into outer column.
	for idx := 1; idx < len(base.Pieces) && len(out) < cfg.MaxProposalsPerBoard; idx++ {
		if !moved[idx] {
			continue
		}
		p := base.Pieces[idx]
		if p.Kind == PieceTarget || p.Kind == PieceUnit {
			continue
		}
		r, c := p.Position/w, p.Position%w
		// Pull left into col 0
		if c > 0 {
			newPos := r*w + 0
			np := Piece{Position: newPos, Size: p.Size, Orientation: p.Orientation, Kind: p.Kind}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpDependencyPullLeft,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: p.Orientation, Size: p.Size}},
				})
			}
		}
		// Pull right: place so piece occupies col w-1
		if p.Orientation == Vertical {
			newPos := r*w + (w - 1)
			np := Piece{Position: newPos, Size: p.Size, Orientation: Vertical, Kind: p.Kind}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpDependencyPullRight,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: Vertical, Size: p.Size}},
				})
			}
		} else if p.Orientation == Horizontal && c+p.Size < w {
			newPos := r*w + (w - p.Size)
			np := Piece{Position: newPos, Size: p.Size, Orientation: Horizontal, Kind: p.Kind}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpDependencyPullRight,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: Horizontal, Size: p.Size}},
				})
			}
		}
	}

	// BottomExtension: move a horizontal moved piece into row h-2 or h-1.
	for idx := 1; idx < len(base.Pieces) && len(out) < cfg.MaxProposalsPerBoard; idx++ {
		if !moved[idx] {
			continue
		}
		p := base.Pieces[idx]
		if p.Kind == PieceTarget {
			continue
		}
		c := p.Position % w
		for _, row := range []int{h - 2, h - 1} {
			if p.Orientation == Vertical && row+p.Size > h {
				continue
			}
			newPos := row*w + c
			if p.Orientation == Horizontal {
				newPos = row*w + c
			}
			np := Piece{Position: newPos, Size: p.Size, Orientation: p.Orientation, Kind: p.Kind}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpBottomExtension,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: p.Orientation, Size: p.Size}},
				})
				break
			}
		}
	}

	// SplitCoreVertical: pull one moved piece left and another right.
	movedIdx := []int{}
	for i := 1; i < len(base.Pieces); i++ {
		if moved[i] && base.Pieces[i].Kind != PieceTarget {
			movedIdx = append(movedIdx, i)
		}
	}
	if len(movedIdx) >= 2 {
		a, b := movedIdx[0], movedIdx[1]
		pa, pb := base.Pieces[a], base.Pieces[b]
		ra, rb := pa.Position/w, pb.Position/w
		left := Piece{Position: ra * w, Size: pa.Size, Orientation: Vertical, Kind: PieceNormal}
		if pa.Orientation == Horizontal {
			left = Piece{Position: ra * w, Size: pa.Size, Orientation: Horizontal, Kind: pa.Kind}
		} else {
			left = Piece{Position: ra*w + 0, Size: pa.Size, Orientation: pa.Orientation, Kind: pa.Kind}
		}
		rightPos := rb*w + (w - 1)
		if pb.Orientation == Horizontal {
			rightPos = rb*w + (w - pb.Size)
		}
		right := Piece{Position: rightPos, Size: pb.Size, Orientation: pb.Orientation, Kind: pb.Kind}
		if pieceFitsBoard(left, w, h) && pieceFitsBoard(right, w, h) {
			out = append(out, coreExpansionProposal{
				Class: CoreExpSplitCoreVertical,
				Relocate: []struct {
					FromIdx, NewPos int
					Ori             Orientation
					Size            int
				}{
					{FromIdx: a, NewPos: left.Position, Ori: left.Orientation, Size: left.Size},
					{FromIdx: b, NewPos: right.Position, Ori: right.Orientation, Size: right.Size},
				},
			})
		}
	}

	// OuterLongBlockGate: relocate a size≥2 piece into bottom outer as horizontal gate.
	for idx := 1; idx < len(base.Pieces) && len(out) < cfg.MaxProposalsPerBoard; idx++ {
		p := base.Pieces[idx]
		if !moved[idx] || p.Size < 2 || p.Kind == PieceTarget {
			continue
		}
		row := h - 1
		for col := 0; col <= w-p.Size; col++ {
			newPos := row*w + col
			np := Piece{Position: newPos, Size: p.Size, Orientation: Horizontal, Kind: PieceNormal}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpOuterLongBlockGate,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: Horizontal, Size: p.Size}},
				})
				break
			}
		}
	}

	// CrossFieldDependency: pull one piece to bottom and one to side.
	if len(movedIdx) >= 2 {
		a, b := movedIdx[0], movedIdx[len(movedIdx)-1]
		pa, pb := base.Pieces[a], base.Pieces[b]
		bottom := Piece{Position: (h-1)*w + (pa.Position % w), Size: pa.Size, Orientation: pa.Orientation, Kind: pa.Kind}
		if pa.Orientation == Horizontal && (pa.Position%w)+pa.Size > w {
			bottom.Position = (h-1)*w + 0
		}
		side := Piece{Position: (pb.Position/w)*w + 0, Size: pb.Size, Orientation: pb.Orientation, Kind: pb.Kind}
		if pieceFitsBoard(bottom, w, h) && pieceFitsBoard(side, w, h) {
			out = append(out, coreExpansionProposal{
				Class: CoreExpCrossFieldDependency,
				Relocate: []struct {
					FromIdx, NewPos int
					Ori             Orientation
					Size            int
				}{
					{FromIdx: a, NewPos: bottom.Position, Ori: bottom.Orientation, Size: bottom.Size},
					{FromIdx: b, NewPos: side.Position, Ori: side.Orientation, Size: side.Size},
				},
			})
		}
	}

	// SplitCoreHorizontal: one piece to top outer row, one to bottom.
	if len(movedIdx) >= 2 {
		a, b := movedIdx[0], movedIdx[len(movedIdx)-1]
		pa, pb := base.Pieces[a], base.Pieces[b]
		topCol := pa.Position % w
		botCol := pb.Position % w
		if pa.Orientation == Horizontal && topCol+pa.Size > w {
			topCol = 0
		}
		if pb.Orientation == Horizontal && botCol+pb.Size > w {
			botCol = 0
		}
		top := Piece{Position: 0*w + topCol, Size: pa.Size, Orientation: pa.Orientation, Kind: pa.Kind}
		bot := Piece{Position: (h-1)*w + botCol, Size: pb.Size, Orientation: pb.Orientation, Kind: pb.Kind}
		if pieceFitsBoard(top, w, h) && pieceFitsBoard(bot, w, h) {
			out = append(out, coreExpansionProposal{
				Class: CoreExpSplitCoreHorizontal,
				Relocate: []struct {
					FromIdx, NewPos int
					Ori             Orientation
					Size            int
				}{
					{FromIdx: a, NewPos: top.Position, Ori: top.Orientation, Size: top.Size},
					{FromIdx: b, NewPos: bot.Position, Ori: bot.Orientation, Size: bot.Size},
				},
			})
		}
	}

	// TopSideExtension: relocate a moved piece into top row at outer column.
	for idx := 1; idx < len(base.Pieces) && len(out) < cfg.MaxProposalsPerBoard; idx++ {
		if !moved[idx] {
			continue
		}
		p := base.Pieces[idx]
		if p.Kind == PieceTarget {
			continue
		}
		for _, col := range []int{0, w - 1} {
			newPos := 0*w + col
			if p.Orientation == Horizontal {
				if col == w-1 {
					newPos = 0*w + (w - p.Size)
				} else {
					newPos = 0
				}
			}
			np := Piece{Position: newPos, Size: p.Size, Orientation: p.Orientation, Kind: p.Kind}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpTopSideExtension,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: p.Orientation, Size: p.Size}},
				})
				break
			}
		}
	}

	// SideChamberChain: relocate a long (≥2) dependency piece into left outer as vertical gate.
	for idx := 1; idx < len(base.Pieces) && len(out) < cfg.MaxProposalsPerBoard; idx++ {
		p := base.Pieces[idx]
		if !moved[idx] || p.Size < 2 || p.Kind == PieceTarget {
			continue
		}
		for row := 0; row <= h-p.Size; row++ {
			newPos := row*w + 0
			np := Piece{Position: newPos, Size: p.Size, Orientation: Vertical, Kind: PieceNormal}
			if pieceFitsBoard(np, w, h) {
				out = append(out, coreExpansionProposal{
					Class: CoreExpSideChamberChain,
					Relocate: []struct {
						FromIdx, NewPos int
						Ori             Orientation
						Size            int
					}{{FromIdx: idx, NewPos: newPos, Ori: Vertical, Size: p.Size}},
				})
				break
			}
		}
	}

	_ = offX
	_ = offY
	_ = time.Second
	return out
}

func applyCoreExpansion(base *Board, prop coreExpansionProposal) (*Board, string) {
	b := base.Copy()
	// Apply relocations: clear old by rebuilding piece list carefully.
	// Strategy: copy pieces, update relocated indices, rebuild board occupancy.
	pieces := append([]Piece{}, b.Pieces...)
	labels := append([]string{}, b.Labels...)
	for _, rel := range prop.Relocate {
		if rel.FromIdx <= 0 || rel.FromIdx >= len(pieces) {
			return nil, "BadRelocateIndex"
		}
		pieces[rel.FromIdx] = Piece{
			Position:    rel.NewPos,
			Size:        rel.Size,
			Orientation: rel.Ori,
			Kind:        pieces[rel.FromIdx].Kind,
		}
	}
	nb := NewEmptyBoard(b.Width, b.Height)
	nb.Rules = b.Rules
	nb.ExitCol = b.ExitCol
	for _, wall := range b.Walls {
		if !nb.AddWall(wall) {
			return nil, "WallRebuildFail"
		}
	}
	for _, wall := range prop.AddWalls {
		if !nb.AddWall(wall) {
			return nil, "AddWallFail"
		}
	}
	newLabels := make([]string, 0, len(pieces)+len(prop.AddPieces))
	for i, p := range pieces {
		if !nb.AddPiece(p) {
			return nil, fmt.Sprintf("RelocateOverlap:%d", i)
		}
		if i < len(labels) {
			newLabels = append(newLabels, labels[i])
		} else {
			newLabels = append(newLabels, fmt.Sprintf("P%02d", i))
		}
	}
	for i, p := range prop.AddPieces {
		if !nb.AddPiece(p) {
			return nil, "AddPieceOverlap"
		}
		lab := "N"
		if i < len(prop.Labels) {
			lab = prop.Labels[i]
		}
		newLabels = append(newLabels, lab)
	}
	nb.Labels = newLabels
	return nb, ""
}

func relocatedPiecesRelevant(base, native *Board, prop coreExpansionProposal, sol Solution, budget SolveBudget) bool {
	if len(prop.Relocate) == 0 {
		return true
	}
	moved := map[int]bool{}
	for _, mv := range sol.Moves {
		moved[mv.Piece] = true
	}
	for _, rel := range prop.Relocate {
		idx := rel.FromIdx
		if idx >= len(native.Pieces) {
			return false
		}
		if moved[idx] {
			continue
		}
		rest := native.Copy()
		rest.ImmobilePieces = make([]bool, len(rest.Pieces))
		rest.ImmobilePieces[idx] = true
		rsol := rest.SolveWithBudget(budget)
		if rsol.TimedOut || rsol.BudgetExceeded {
			return false
		}
		if !rsol.Solvable || rsol.NumMoves > sol.NumMoves {
			continue
		}
		return false
	}
	_ = base
	return true
}
