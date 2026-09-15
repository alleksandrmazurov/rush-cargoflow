package rush

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// SolveCacheKey identifies a transformed Cargo Flow solve.
type SolveCacheKey struct {
	SourceDatabaseHash string `json:"sourceDatabaseHash"`
	SourcePuzzleID     string `json:"sourcePuzzleId"`
	TransformVersion   string `json:"transformVersion"`
	EmbeddingVariant   string `json:"embeddingVariant"`
	CargoRulesVersion  string `json:"cargoRulesVersion"`
	SolverVersion      string `json:"solverVersion"`
}

func (k SolveCacheKey) String() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		k.SourceDatabaseHash, k.SourcePuzzleID, k.TransformVersion,
		k.EmbeddingVariant, k.CargoRulesVersion, k.SolverVersion)
}

// SolveCacheEntry stores one expensive solve result.
type SolveCacheEntry struct {
	Key              SolveCacheKey `json:"key"`
	Valid            bool          `json:"valid"`
	ImmediateVictory bool          `json:"immediateVictory"`
	Solved           bool          `json:"solved"`
	TimedOut         bool          `json:"timedOut"`
	BudgetExceeded   bool          `json:"budgetExceeded"`
	OptimalGestures  int           `json:"optimalGestures"`
	VisitedStates    int           `json:"visitedStates"`
	ElapsedMs        int64         `json:"elapsedMs"`
	Fingerprint      string        `json:"fingerprint"`
	ReplayVerified   bool          `json:"replayVerified"`
	OffsetX          int           `json:"offsetX"`
	OffsetY          int           `json:"offsetY"`
	Moves            []Move        `json:"moves,omitempty"`
	RejectReason     string        `json:"rejectReason,omitempty"`
}

// SolveCache is a thread-safe append-only JSONL cache with in-memory index.
type SolveCache struct {
	path string
	mu   sync.Mutex
	idx  map[string]SolveCacheEntry
}

func OpenSolveCache(path string) (*SolveCache, error) {
	c := &SolveCache{
		path: path,
		idx:  map[string]SolveCacheEntry{},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e SolveCacheEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		c.idx[e.Key.String()] = e
	}
	return c, sc.Err()
}

func (c *SolveCache) Get(key SolveCacheKey) (SolveCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.idx[key.String()]
	return e, ok
}

func (c *SolveCache) Put(e SolveCacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.idx[e.Key.String()] = e
	f, err := os.OpenFile(c.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func (c *SolveCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.idx)
}

// HashFileSHA256 streams SHA256 of path (safe for multi-hundred-MB files).
func HashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
