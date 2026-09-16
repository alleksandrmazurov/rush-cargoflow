package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Cargo Flow JSON schema version for offline Unity <-> Rush interchange.
const CargoFlowJSONSchemaVersion = 1

// CoordinateSpaceUnity means JSON (x,y) match Unity LevelDefinition GridX/GridY:
// X rightward, Y toward the exit (increasing Y exits off the top of the board).
const CoordinateSpaceUnity = "unity"

// LevelJSON is the neutral development interchange for one Cargo Flow level.
type LevelJSON struct {
	SchemaVersion    int         `json:"schemaVersion"`
	LevelID          string      `json:"levelId"`
	Width            int         `json:"width"`
	Height           int         `json:"height"`
	CoordinateSpace  string      `json:"coordinateSpace"`
	Exit             ExitJSON    `json:"exit"`
	Pieces           []PieceJSON `json:"pieces"`
	CanonicalGestures *int             `json:"canonicalGestures,omitempty"`
	Source            string           `json:"source,omitempty"`
	Transplant        *TransplantJSON  `json:"transplant,omitempty"`
	Enrichment        *EnrichmentJSON  `json:"enrichment,omitempty"`
	BoardSpace        *BoardSpaceJSON  `json:"boardSpace,omitempty"`
}

// EnrichmentJSON records RUSH-010 / RUSH-010.1 movable-1x1 enrichment provenance.
type EnrichmentJSON struct {
	BaseCandidateId              string  `json:"baseCandidateId"`
	BaseFamilyId                 string  `json:"baseFamilyId"`
	BaseSourcePuzzleId           string  `json:"baseSourcePuzzleId"`
	Added1x1Count                int     `json:"added1x1Count"`
	Essential1x1Count            int     `json:"essential1x1Count"`
	OneByOneMovesInOptimal       int     `json:"oneByOneMovesInOptimal"`
	BaseOptimal                  int     `json:"baseOptimal"`
	EnrichedOptimal              int     `json:"enrichedOptimal"`
	OptimalDelta                 int     `json:"optimalDelta"`
	EnrichmentImpact             float64 `json:"enrichmentImpact"`
	PlacementCells               []int   `json:"placementCells"`
	OneByOneRole                      string `json:"oneByOneRole,omitempty"`
	OneByOneInitiallyInTargetCorridor bool   `json:"oneByOneInitiallyInTargetCorridor"`
	RoleEvidenceSummary               string `json:"roleEvidenceSummary,omitempty"`
	ReleasedCells                []int   `json:"releasedCells,omitempty"`
	SubsequentPieceClass         string  `json:"subsequentPieceClass,omitempty"`
	// RUSH-010.2 inventory diversity metadata (sequencer-ready).
	InventoryClass       string `json:"inventoryClass,omitempty"`
	InventorySignature   string `json:"inventorySignature,omitempty"`
	Relevant1x1Count     int    `json:"relevant1x1Count,omitempty"`
	Corridor1x1Count     int    `json:"corridor1x1Count,omitempty"`
	DistinctRowsUsed     int    `json:"distinctRowsUsed,omitempty"`
	DistinctColumnsUsed  int    `json:"distinctColumnsUsed,omitempty"`
	BoardRegionsUsed     int    `json:"boardRegionsUsed,omitempty"`
}

// TransplantJSON records Rush-database provenance for RUSH-008 candidates.
type TransplantJSON struct {
	SourceDataset          string `json:"sourceDataset"`
	SourcePuzzleId         string `json:"sourcePuzzleId"`
	SourceLine             int    `json:"sourceLine"`
	OriginalBoard          string `json:"originalBoard"`
	OriginalOptimalMoves   int    `json:"originalOptimalMoves"`
	OriginalClusterSize    int    `json:"originalClusterSize"`
	TransformRotation      string `json:"transformRotation"`
	EmbeddingVariant       string `json:"embeddingVariant"`
	OffsetX                int    `json:"offsetX"`
	OffsetY                int    `json:"offsetY"`
	CargoFlowOptimalGestures *int `json:"cargoFlowOptimalGestures,omitempty"`
}

type ExitJSON struct {
	Side   string `json:"side"`
	Column int    `json:"column"`
}

type PieceJSON struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Movable  bool   `json:"movable"`
}

// SolutionJSON is the exported solver result (Unity coordinate space in from/to).
type SolutionJSON struct {
	SchemaVersion    int            `json:"schemaVersion"`
	LevelID          string         `json:"levelId"`
	Solved           bool           `json:"solved"`
	Optimal          bool           `json:"optimal"`
	OptimalGestures  int            `json:"optimalGestures"`
	VisitedStates    int            `json:"visitedStates"`
	ElapsedMs        int64          `json:"elapsedMs"`
	TimedOut         bool           `json:"timedOut"`
	BudgetExceeded   bool           `json:"budgetExceeded"`
	ReplayPass       bool           `json:"replayPass"`
	ReplayVerified   bool           `json:"replayVerified"`
	Fingerprint      string         `json:"fingerprint"`
	Moves            []MoveJSON     `json:"moves"`
}

type MoveJSON struct {
	PieceID  string `json:"pieceId"`
	FromX    int    `json:"fromX"`
	FromY    int    `json:"fromY"`
	ToX      int    `json:"toX"`
	ToY      int    `json:"toY"`
	Axis     string `json:"axis"`
	Distance int    `json:"distance"`
	IsExit   bool   `json:"isExit"`
}

// SolveBudget bounds Cargo Flow searches for pilot validation.
type SolveBudget struct {
	TimeLimit   time.Duration
	MaxVisited  int
	MaxDepth    int
}

// DefaultCargoFlowSolveBudget is a finite safety bound for real-level pilots.
func DefaultCargoFlowSolveBudget() SolveBudget {
	return SolveBudget{
		TimeLimit:  8 * time.Second,
		MaxVisited: 1_500_000,
		MaxDepth:   64,
	}
}

// LoadCargoFlowLevelJSONFile reads and validates a level JSON file.
func LoadCargoFlowLevelJSONFile(path string) (*LevelJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCargoFlowLevelJSON(data)
}

// ParseCargoFlowLevelJSON parses level JSON bytes.
func ParseCargoFlowLevelJSON(data []byte) (*LevelJSON, error) {
	var level LevelJSON
	if err := json.Unmarshal(data, &level); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := level.Validate(); err != nil {
		return nil, err
	}
	return &level, nil
}

// Validate checks schema and structural constraints (Unity coordinate space).
func (level *LevelJSON) Validate() error {
	if level.SchemaVersion != CargoFlowJSONSchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d (want %d)", level.SchemaVersion, CargoFlowJSONSchemaVersion)
	}
	if level.Width != CargoFlowWidth {
		return fmt.Errorf("width must be %d, got %d", CargoFlowWidth, level.Width)
	}
	if level.Height != CargoFlowHeight {
		return fmt.Errorf("height must be %d, got %d", CargoFlowHeight, level.Height)
	}
	space := level.CoordinateSpace
	if space == "" {
		space = CoordinateSpaceUnity
	}
	if space != CoordinateSpaceUnity {
		return fmt.Errorf("unsupported coordinateSpace %q (want %q)", space, CoordinateSpaceUnity)
	}
	if !strings.EqualFold(level.Exit.Side, "top") {
		return fmt.Errorf("exit.side must be \"top\", got %q", level.Exit.Side)
	}
	if level.Exit.Column < 0 || level.Exit.Column >= level.Width {
		return fmt.Errorf("exit.column out of range: %d", level.Exit.Column)
	}

	if len(level.Pieces) == 0 {
		return fmt.Errorf("pieces required")
	}

	occ := make(map[string]string)
	targets := 0
	ids := make(map[string]bool)

	for i, p := range level.Pieces {
		if p.ID == "" {
			return fmt.Errorf("piece[%d]: id required", i)
		}
		if ids[p.ID] {
			return fmt.Errorf("duplicate piece id %q", p.ID)
		}
		ids[p.ID] = true
		if p.Width < 1 || p.Height < 1 {
			return fmt.Errorf("piece %q: invalid footprint %dx%d", p.ID, p.Width, p.Height)
		}

		typ, err := normalizePieceType(p.Type, p.Width, p.Height, p.Movable)
		if err != nil {
			return fmt.Errorf("piece %q: %w", p.ID, err)
		}
		level.Pieces[i].Type = typ
		level.Pieces[i].Movable = typ != "static1x1"

		if typ == "target" {
			targets++
			if p.Width != 1 || p.Height != 2 {
				return fmt.Errorf("piece %q: target must be 1x2", p.ID)
			}
		}

		for dy := 0; dy < p.Height; dy++ {
			for dx := 0; dx < p.Width; dx++ {
				x, y := p.X+dx, p.Y+dy
				if x < 0 || x >= level.Width || y < 0 || y >= level.Height {
					return fmt.Errorf("piece %q: cell (%d,%d) out of board", p.ID, x, y)
				}
				key := fmt.Sprintf("%d,%d", x, y)
				if other, ok := occ[key]; ok {
					return fmt.Errorf("overlap at (%d,%d) between %q and %q", x, y, other, p.ID)
				}
				occ[key] = p.ID
			}
		}
	}

	if targets == 0 {
		return fmt.Errorf("exactly one target required (found 0)")
	}
	if targets > 1 {
		return fmt.Errorf("exactly one target required (found %d)", targets)
	}
	return nil
}

func normalizePieceType(typ string, w, h int, movable bool) (string, error) {
	t := strings.ToLower(strings.TrimSpace(typ))
	switch t {
	case "target":
		return "target", nil
	case "static1x1", "static":
		if w != 1 || h != 1 {
			return "", fmt.Errorf("static must be 1x1")
		}
		return "static1x1", nil
	case "movable1x1":
		if w != 1 || h != 1 {
			return "", fmt.Errorf("movable1x1 must be 1x1")
		}
		return "movable1x1", nil
	case "movable1x2", "movable2x1":
		if w*h != 2 || (w != 1 && h != 1) {
			return "", fmt.Errorf("movable1x2 footprint must be 1x2 or 2x1")
		}
		return "movable1x2", nil
	case "movable1x3", "movable3x1":
		if w*h != 3 || (w != 1 && h != 1) {
			return "", fmt.Errorf("movable1x3 footprint must be 1x3 or 3x1")
		}
		return "movable1x3", nil
	case "movable":
		if !movable {
			if w == 1 && h == 1 {
				return "static1x1", nil
			}
			return "", fmt.Errorf("immovable non-1x1 not supported")
		}
		switch {
		case w == 1 && h == 1:
			return "movable1x1", nil
		case w*h == 2 && (w == 1 || h == 1):
			return "movable1x2", nil
		case w*h == 3 && (w == 1 || h == 1):
			return "movable1x3", nil
		default:
			return "", fmt.Errorf("unsupported movable footprint %dx%d", w, h)
		}
	default:
		return "", fmt.Errorf("unknown type %q", typ)
	}
}

// BoardFromLevelJSON builds a Cargo Flow Board.
// JSON uses Unity coordinates; Rush internal rows have exit at row 0 (top).
func BoardFromLevelJSON(level *LevelJSON) (*Board, error) {
	if err := level.Validate(); err != nil {
		return nil, err
	}
	w, h := level.Width, level.Height
	occupied := make([]bool, w*h)
	var walls []int
	var target *Piece
	var targetID string
	var others []Piece
	var otherIDs []string

	for _, p := range level.Pieces {
		rushX, rushY := UnityToRushOrigin(p.X, p.Y, p.Width, p.Height, h)
		cells := footprintCells(rushX, rushY, p.Width, p.Height, w)
		switch p.Type {
		case "static1x1":
			i := cells[0]
			if occupied[i] {
				return nil, fmt.Errorf("static %q overlap", p.ID)
			}
			occupied[i] = true
			walls = append(walls, i)
		case "target":
			piece := Piece{Position: cells[0], Size: 2, Orientation: Vertical, Kind: PieceTarget}
			target = &piece
			targetID = p.ID
			for _, c := range cells {
				if occupied[c] {
					return nil, fmt.Errorf("target overlap")
				}
				occupied[c] = true
			}
		case "movable1x1":
			piece := Piece{Position: cells[0], Size: 1, Orientation: Horizontal, Kind: PieceUnit}
			others = append(others, piece)
			otherIDs = append(otherIDs, p.ID)
			if occupied[cells[0]] {
				return nil, fmt.Errorf("piece %q overlap", p.ID)
			}
			occupied[cells[0]] = true
		default: // movable1x2 / movable1x3
			orient := Horizontal
			size := p.Width
			if p.Height > p.Width {
				orient = Vertical
				size = p.Height
			}
			piece := Piece{Position: cells[0], Size: size, Orientation: orient, Kind: PieceNormal}
			others = append(others, piece)
			otherIDs = append(otherIDs, p.ID)
			for _, c := range cells {
				if occupied[c] {
					return nil, fmt.Errorf("piece %q overlap", p.ID)
				}
				occupied[c] = true
			}
		}
	}

	if target == nil {
		return nil, fmt.Errorf("missing target")
	}

	// Stable ordering: Target first, then other pieces sorted by id (matches Labels).
	order := make([]int, len(others))
	for i := range others {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return otherIDs[order[i]] < otherIDs[order[j]] })

	pieces := []Piece{*target}
	labels := []string{targetID}
	for _, idx := range order {
		pieces = append(pieces, others[idx])
		labels = append(labels, otherIDs[idx])
	}

	sort.Ints(walls)
	board := &Board{
		Width:    w,
		Height:   h,
		Pieces:   pieces,
		Walls:    walls,
		Labels:   labels,
		Rules:    RulesCargoFlow,
		occupied: occupied,
		memoKey:  MakeMemoKey(pieces),
		ExitCol:  level.Exit.Column,
	}
	if err := board.Validate(); err != nil {
		return nil, err
	}
	return board, nil
}

func footprintCells(x, y, pw, ph, boardW int) []int {
	cells := make([]int, 0, pw*ph)
	for dy := 0; dy < ph; dy++ {
		for dx := 0; dx < pw; dx++ {
			cells = append(cells, (y+dy)*boardW+(x+dx))
		}
	}
	sort.Ints(cells)
	return cells
}

// UnityToRushOrigin converts Unity min-corner (GridX,GridY) to Rush top-left cell.
// Unity: Y increases toward exit. Rush: row 0 is exit side.
func UnityToRushOrigin(unityX, unityY, width, height, boardHeight int) (rushX, rushY int) {
	rushX = unityX
	rushY = boardHeight - unityY - height
	return rushX, rushY
}

// RushToUnityOrigin converts Rush top-left cell to Unity min-corner.
func RushToUnityOrigin(rushX, rushY, width, height, boardHeight int) (unityX, unityY int) {
	unityX = rushX
	unityY = boardHeight - rushY - height
	return unityX, unityY
}

// FingerprintUnity returns a stable textual dump in Unity coordinates.
func (level *LevelJSON) FingerprintUnity() string {
	grid := make([][]byte, level.Height)
	for y := 0; y < level.Height; y++ {
		row := make([]byte, level.Width)
		for x := 0; x < level.Width; x++ {
			row[x] = '.'
		}
		grid[y] = row
	}
	// Paint in deterministic id order.
	idxs := make([]int, len(level.Pieces))
	for i := range idxs {
		idxs[i] = i
	}
	sort.Slice(idxs, func(i, j int) bool { return level.Pieces[idxs[i]].ID < level.Pieces[idxs[j]].ID })
	for _, i := range idxs {
		p := level.Pieces[i]
		ch := byte('#')
		switch p.Type {
		case "target":
			ch = 'T'
		case "static1x1":
			ch = 'x'
		case "movable1x1":
			ch = 'o'
		default:
			ch = 'B'
		}
		for dy := 0; dy < p.Height; dy++ {
			for dx := 0; dx < p.Width; dx++ {
				grid[p.Y+dy][p.X+dx] = ch
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "unity %dx%d exit=%d\n", level.Width, level.Height, level.Exit.Column)
	// Print Unity Y from high (exit) to low for human readability (top=exit).
	for y := level.Height - 1; y >= 0; y-- {
		b.Write(grid[y])
		b.WriteByte('\n')
	}
	return b.String()
}

// BoardFingerprintRush dumps Rush-internal ASCII (row0=exit/top).
func BoardFingerprintRush(board *Board) string {
	return board.String() + "\n"
}

// ExportSolutionJSON builds a Unity-space solution document.
func ExportSolutionJSON(level *LevelJSON, board *Board, sol Solution, elapsed time.Duration, replayPass bool) SolutionJSON {
	out := SolutionJSON{
		SchemaVersion:   CargoFlowJSONSchemaVersion,
		LevelID:         level.LevelID,
		Solved:          sol.Solvable,
		Optimal:         sol.Solvable && !sol.TimedOut && !sol.BudgetExceeded,
		OptimalGestures: sol.NumMoves,
		VisitedStates:   sol.MemoSize,
		ElapsedMs:       elapsed.Milliseconds(),
		TimedOut:        sol.TimedOut,
		BudgetExceeded:  sol.BudgetExceeded,
		ReplayPass:      replayPass,
		ReplayVerified:  replayPass,
		Fingerprint:     level.FingerprintUnity(),
		Moves:           make([]MoveJSON, 0, len(sol.Moves)),
	}
	if !sol.Solvable {
		return out
	}

	work := board.Copy()
	for _, m := range sol.Moves {
		label := pieceDisplayLabel(work, m.Piece)
		p := work.Pieces[m.Piece]
		pw, ph := pieceWidth(p), pieceHeight(p)
		fromUX, fromUY := RushToUnityOrigin(p.Col(work.Width), p.Row(work.Width), pw, ph, work.Height)

		var toUX, toUY, dist int
		axis := "none"
		if m.Exit {
			axis = "vertical"
			toUX = fromUX
			toUY = work.Height
			dist = work.Height - fromUY
			work.DoMove(m)
		} else {
			work.DoMove(m)
			p2 := work.Pieces[m.Piece]
			toUX, toUY = RushToUnityOrigin(p2.Col(work.Width), p2.Row(work.Width), pieceWidth(p2), pieceHeight(p2), work.Height)
			if toUX != fromUX {
				axis = "horizontal"
				dist = toUX - fromUX
			} else {
				axis = "vertical"
				dist = toUY - fromUY
			}
			if dist < 0 {
				dist = -dist
			}
		}

		out.Moves = append(out.Moves, MoveJSON{
			PieceID:  label,
			FromX:    fromUX,
			FromY:    fromUY,
			ToX:      toUX,
			ToY:      toUY,
			Axis:     axis,
			Distance: dist,
			IsExit:   m.Exit,
		})
	}
	return out
}

func pieceWidth(p Piece) int {
	if p.Orientation == Horizontal {
		return p.Size
	}
	return 1
}

func pieceHeight(p Piece) int {
	if p.Orientation == Vertical {
		return p.Size
	}
	return 1
}

// WriteJSONFile writes pretty JSON.
func WriteJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
