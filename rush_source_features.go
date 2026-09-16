package rush

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	CuratorVersion              = "curator-poc-1"
	TransformVersionCW90        = "cw90-embed-v1"
	TransformVersionCW90MirrorH = "cw90-embed-mirrorh-v1"
	CargoRulesVersionCF         = "cf-7x8-top3-v1"
	SolverVersionBFS            = "cargo-bfs-budget-v1"
)

// SourceFeatureVector is cheap pre-transform metadata (no Cargo Flow solve).
type SourceFeatureVector struct {
	SourcePuzzleID          string  `json:"sourcePuzzleId"`
	LineNumber              int     `json:"lineNumber"`
	Board36                 string  `json:"board36"`
	OriginalOptimalMoves    int     `json:"originalOptimalMoves"`
	OriginalClusterSize     int     `json:"originalClusterSize"`
	LogClusterSize          float64 `json:"logClusterSize"`
	PieceCount              int     `json:"pieceCount"`
	Length2Count            int     `json:"length2Count"`
	Length3Count            int     `json:"length3Count"`
	HorizontalCount         int     `json:"horizontalCount"`
	VerticalCount           int     `json:"verticalCount"`
	OrientationBalance      float64 `json:"orientationBalance"` // |H-V|/(H+V)
	StaticCount             int     `json:"staticCount"`
	DirectTargetBlockerCount int    `json:"directTargetBlockerCount"`
	TargetRow               int     `json:"targetRow"`
	InventorySignature      string  `json:"inventorySignature"`
	MovesStratum            string  `json:"movesStratum"`
}

// ComputeSourceFeatures derives cheap features from a Rush DB record.
func ComputeSourceFeatures(rec RushDBRecord) (SourceFeatureVector, error) {
	rows, err := RushDBBoardRows(rec.Board36)
	if err != nil {
		return SourceFeatureVector{}, err
	}
	labelCells := map[string][][2]int{}
	statics := 0
	for r := 0; r < RushDBHeight; r++ {
		for c := 0; c < RushDBWidth; c++ {
			ch := rows[r][c]
			switch ch {
			case 'o', '.':
			case 'x':
				statics++
			default:
				lab := string(ch)
				labelCells[lab] = append(labelCells[lab], [2]int{r, c})
			}
		}
	}
	if _, ok := labelCells["A"]; !ok {
		return SourceFeatureVector{}, fmt.Errorf("missing primary A")
	}

	l2, l3, hCnt, vCnt := 0, 0, 0, 0
	invParts := []string{}
	targetRow := -1
	labels := make([]string, 0, len(labelCells))
	for lab := range labelCells {
		labels = append(labels, lab)
	}
	sort.Strings(labels)
	for _, lab := range labels {
		cells := labelCells[lab]
		sort.Slice(cells, func(i, j int) bool {
			if cells[i][0] != cells[j][0] {
				return cells[i][0] < cells[j][0]
			}
			return cells[i][1] < cells[j][1]
		})
		sz := len(cells)
		horiz := cells[0][0] == cells[len(cells)-1][0]
		ori := "V"
		if horiz {
			ori = "H"
			hCnt++
		} else {
			vCnt++
		}
		switch sz {
		case 2:
			l2++
		case 3:
			l3++
		}
		if lab == "A" {
			targetRow = cells[0][0]
			invParts = append(invParts, fmt.Sprintf("A:%d%s", sz, ori))
		} else {
			invParts = append(invParts, fmt.Sprintf("%d%s", sz, ori))
		}
	}
	sort.Strings(invParts[1:]) // keep A first-ish; full sort ok for signature
	sort.Strings(invParts)
	if statics > 0 {
		invParts = append(invParts, fmt.Sprintf("x:%d", statics))
	}

	blockers := 0
	if targetRow >= 0 {
		// Classic Rush: primary exits right on its row — count occupied cells to the right of A.
		aCells := labelCells["A"]
		maxC := -1
		for _, rc := range aCells {
			if rc[1] > maxC {
				maxC = rc[1]
			}
		}
		for c := maxC + 1; c < RushDBWidth; c++ {
			ch := rows[targetRow][c]
			if ch != 'o' && ch != '.' {
				blockers++
			}
		}
	}

	totalOri := hCnt + vCnt
	bal := 0.0
	if totalOri > 0 {
		bal = math.Abs(float64(hCnt-vCnt)) / float64(totalOri)
	}
	logCl := 0.0
	if rec.OriginalClusterSize > 0 {
		logCl = math.Log(float64(rec.OriginalClusterSize))
	}

	return SourceFeatureVector{
		SourcePuzzleID:           rec.SourcePuzzleID,
		LineNumber:               rec.LineNumber,
		Board36:                  rec.Board36,
		OriginalOptimalMoves:     rec.OriginalOptimalMoves,
		OriginalClusterSize:      rec.OriginalClusterSize,
		LogClusterSize:           logCl,
		PieceCount:               len(labelCells),
		Length2Count:             l2,
		Length3Count:             l3,
		HorizontalCount:          hCnt,
		VerticalCount:            vCnt,
		OrientationBalance:       bal,
		StaticCount:              statics,
		DirectTargetBlockerCount: blockers,
		TargetRow:                targetRow,
		InventorySignature:       strings.Join(invParts, ","),
		MovesStratum:             MovesStratum(rec.OriginalOptimalMoves),
	}, nil
}

// MovesStratum buckets original Rush optimal moves for stratified sampling.
func MovesStratum(moves int) string {
	switch {
	case moves <= 8:
		return "low"
	case moves <= 14:
		return "low-mid"
	case moves <= 22:
		return "mid"
	case moves <= 35:
		return "high"
	default:
		return "very-high"
	}
}

// StreamRushDBFile calls fn for each record. Supports plain text or .gz.
// Stops early if fn returns a non-nil error (io.EOF ends cleanly).
func StreamRushDBFile(path string, datasetTag string, fn func(RushDBRecord) error) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, err
		}
		defer gz.Close()
		r = gz
	}

	sc := bufio.NewScanner(r)
	// Long lines unlikely; bump buffer for safety.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	if datasetTag == "" {
		datasetTag = datasetTagFromPath(path)
	}
	lineNo := 0
	for sc.Scan() {
		lineNo++
		rec, err := ParseRushDBLineNamed(sc.Text(), lineNo, datasetTag)
		if err != nil {
			return lineNo, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if err := fn(rec); err != nil {
			if err == ErrStreamStop {
				return lineNo, ErrStreamStop
			}
			return lineNo, err
		}
	}
	if err := sc.Err(); err != nil {
		return lineNo, err
	}
	return lineNo, nil
}

// ErrStreamStop ends StreamRushDBFile early without treating it as failure.
var ErrStreamStop = errStreamStop{}

type errStreamStop struct{}

func (errStreamStop) Error() string { return "stream stop" }

func datasetTagFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSuffix(base, ".txt")
	if base == "" {
		return "rush"
	}
	return base
}
