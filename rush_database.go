package rush

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	RushDBWidth  = 6
	RushDBHeight = 6
	RushDBCells  = RushDBWidth * RushDBHeight
)

// RushDBRecord is one line from the Fogleman Rush Hour database.
// Columns: original optimal moves, 36-char board, cluster size.
type RushDBRecord struct {
	LineNumber          int
	SourcePuzzleID      string // e.g. "rush1000:L0001"
	Board36             string
	OriginalOptimalMoves int
	OriginalClusterSize int
}

// ParseRushDBLine parses a single database line (default tag rush1000 for POC tests).
func ParseRushDBLine(line string, lineNumber int) (RushDBRecord, error) {
	return ParseRushDBLineNamed(line, lineNumber, "rush1000")
}

// ParseRushDBLineNamed parses a database line with an explicit dataset tag for SourcePuzzleID.
func ParseRushDBLineNamed(line string, lineNumber int, datasetTag string) (RushDBRecord, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return RushDBRecord{}, fmt.Errorf("empty line")
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return RushDBRecord{}, fmt.Errorf("malformed record: want moves board clusterSize, got %q", line)
	}
	moves, err := strconv.Atoi(fields[0])
	if err != nil {
		return RushDBRecord{}, fmt.Errorf("moves: %w", err)
	}
	board := fields[1]
	if len(board) != RushDBCells {
		return RushDBRecord{}, fmt.Errorf("board length %d want %d", len(board), RushDBCells)
	}
	for _, ch := range board {
		switch {
		case ch == 'o' || ch == '.' || ch == 'x':
		case ch >= 'A' && ch <= 'Z':
		default:
			return RushDBRecord{}, fmt.Errorf("invalid board char %q", string(ch))
		}
	}
	if !strings.ContainsRune(board, 'A') {
		return RushDBRecord{}, fmt.Errorf("missing primary piece A")
	}
	cluster, err := strconv.Atoi(fields[2])
	if err != nil {
		return RushDBRecord{}, fmt.Errorf("clusterSize: %w", err)
	}
	if datasetTag == "" {
		datasetTag = "rush"
	}
	return RushDBRecord{
		LineNumber:           lineNumber,
		SourcePuzzleID:       fmt.Sprintf("%s:L%07d", datasetTag, lineNumber),
		Board36:              board,
		OriginalOptimalMoves: moves,
		OriginalClusterSize:  cluster,
	}, nil
}

// LoadRushDBFile loads up to maxRecords records (0 = all).
func LoadRushDBFile(path string, maxRecords int) ([]RushDBRecord, error) {
	tag := datasetTagFromPath(path)
	out := []RushDBRecord{}
	_, err := StreamRushDBFile(path, tag, func(rec RushDBRecord) error {
		out = append(out, rec)
		if maxRecords > 0 && len(out) >= maxRecords {
			return ErrStreamStop
		}
		return nil
	})
	if err != nil && err != ErrStreamStop {
		return nil, err
	}
	return out, nil
}

// BoardFromRushDBRecord builds an original Rush 6x6 Board (RulesOriginalRush).
func BoardFromRushDBRecord(rec RushDBRecord) (*Board, error) {
	return NewBoardFromString(rec.Board36)
}

// RushDBBoardRows splits the 36-char string into 6 rows (accepts o or .).
func RushDBBoardRows(board36 string) ([]string, error) {
	if len(board36) != RushDBCells {
		return nil, fmt.Errorf("board length")
	}
	rows := make([]string, RushDBHeight)
	for i := 0; i < RushDBHeight; i++ {
		rows[i] = board36[i*RushDBWidth : (i+1)*RushDBWidth]
	}
	return rows, nil
}
