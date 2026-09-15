package rush

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// TransplantRejection reasons for RUSH-008 filtering.
type TransplantRejection string

const (
	TRejectInvalidTransform      TransplantRejection = "InvalidTransform"
	TRejectImmediateVictory      TransplantRejection = "ImmediateVictory"
	TRejectUnsolvable            TransplantRejection = "Unsolvable"
	TRejectTooEasy               TransplantRejection = "TooEasy"
	TRejectOutsideRequestedRange TransplantRejection = "OutsideRequestedRange"
	TRejectDifficultyUnknown     TransplantRejection = "DifficultyUnknown"
	TRejectDuplicate             TransplantRejection = "Duplicate"
	TRejectSameSourceAlternative TransplantRejection = "SameSourceAlternative"
	TRejectReplayFailed          TransplantRejection = "ReplayFailed"
	TRejectOther                 TransplantRejection = "Other"
)

// TransplantConfig controls the RUSH-008 database transplant pilot.
type TransplantConfig struct {
	DatasetPath      string
	DatasetName      string
	MaxSourcePuzzles int
	MinOptimal       int
	MaxOptimal       int
	TargetAccepted   int
	BucketQuotas     map[string]int // "8-10","11-13","14-16","17-20"
	Embeddings       []EmbeddingVariant
	SolveTimeLimit   time.Duration
	MaxVisitedStates int
	OutputDir        string
}

func DefaultTransplantConfig(datasetPath string) TransplantConfig {
	return TransplantConfig{
		DatasetPath:      datasetPath,
		DatasetName:      "rush1000.txt",
		MaxSourcePuzzles: 1000,
		MinOptimal:       8,
		MaxOptimal:       20,
		TargetAccepted:   12,
		BucketQuotas: map[string]int{
			"8-10": 3, "11-13": 3, "14-16": 3, "17-20": 3,
		},
		Embeddings:       DefaultEmbeddingVariants(),
		SolveTimeLimit:   10 * time.Second,
		MaxVisitedStates: 4_000_000,
		OutputDir:        "output/RUSH008_DatabaseTransplant_Pilot",
	}
}

func optimalBucket(n int) string {
	switch {
	case n >= 8 && n <= 10:
		return "8-10"
	case n >= 11 && n <= 13:
		return "11-13"
	case n >= 14 && n <= 16:
		return "14-16"
	case n >= 17 && n <= 20:
		return "17-20"
	default:
		return ""
	}
}

// TransplantAccepted is one validated Cargo Flow candidate from the Rush DB.
type TransplantAccepted struct {
	CandidateID              string
	Source                   RushDBRecord
	Embedding                EmbeddingVariant
	Rotation                 string
	OffsetX, OffsetY         int
	Level                    *LevelJSON
	Board                    *Board
	OptimalGestures          int
	VisitedStates            int
	ElapsedMs                int64
	Fingerprint              string
	Solution                 Solution
	SolutionDoc              SolutionJSON
	ReplayVerified           bool
	ASCIIPreview             string
	OriginalOptimalMoves     int
	OriginalClusterSize      int
}

// TransplantBatchResult summarizes a transplant pilot run.
type TransplantBatchResult struct {
	Config              TransplantConfig
	SourceScanned       int
	TransformAttempts   int
	ValidTransforms     int
	ExactSolved         int
	Accepted            []TransplantAccepted
	Rejected            map[TransplantRejection]int
	TotalElapsed        time.Duration
	SolveTimesMs        []int64
	BucketCounts        map[string]int
}

// RunTransplantPilot scans the Rush DB, transforms, solves, and selects a diverse pilot.
// Phase 1: best qualifying candidate per SourcePuzzleId (scan up to MaxSourcePuzzles).
// Phase 2: fill bucket quotas deterministically (harder first within bucket).
func RunTransplantPilot(cfg TransplantConfig) (TransplantBatchResult, error) {
	start := time.Now()
	res := TransplantBatchResult{
		Config:       cfg,
		Rejected:     map[TransplantRejection]int{},
		SolveTimesMs: []int64{},
		BucketCounts: map[string]int{},
	}
	for k := range cfg.BucketQuotas {
		res.BucketCounts[k] = 0
	}

	records, err := LoadRushDBFile(cfg.DatasetPath, cfg.MaxSourcePuzzles)
	if err != nil {
		return res, err
	}
	res.SourceScanned = len(records)

	embeds := cfg.Embeddings
	if len(embeds) == 0 {
		embeds = DefaultEmbeddingVariants()
	}
	budget := SolveBudget{TimeLimit: cfg.SolveTimeLimit, MaxVisited: cfg.MaxVisitedStates}

	type pooled struct {
		rec RushDBRecord
		c   *transplantCand
	}
	pool := []pooled{}
	seenFPDuringScan := map[string]bool{}

	for _, rec := range records {
		var best *transplantCand
		alts := 0
		for _, emb := range embeds {
			res.TransformAttempts++
			tr, err := TransformRushRecordToCargoFlow(rec, emb)
			if err != nil {
				res.Rejected[TRejectInvalidTransform]++
				continue
			}
			res.ValidTransforms++
			if tr.Board.cargoTargetCanExit() {
				res.Rejected[TRejectImmediateVictory]++
				continue
			}

			t0 := time.Now()
			sol := tr.Board.SolveWithBudget(budget)
			elapsed := time.Since(t0)
			res.SolveTimesMs = append(res.SolveTimesMs, elapsed.Milliseconds())

			if sol.TimedOut || sol.BudgetExceeded {
				res.Rejected[TRejectDifficultyUnknown]++
				continue
			}
			if !sol.Solvable {
				res.Rejected[TRejectUnsolvable]++
				continue
			}
			res.ExactSolved++

			if sol.NumMoves < cfg.MinOptimal {
				res.Rejected[TRejectTooEasy]++
				continue
			}
			if sol.NumMoves > cfg.MaxOptimal {
				res.Rejected[TRejectOutsideRequestedRange]++
				continue
			}
			if optimalBucket(sol.NumMoves) == "" {
				res.Rejected[TRejectOutsideRequestedRange]++
				continue
			}

			fp := layoutFingerprint(tr.Board)
			if seenFPDuringScan[fp] {
				res.Rejected[TRejectDuplicate]++
				continue
			}

			b2 := tr.Board.Copy()
			if err := b2.Replay(sol.Moves); err != nil {
				res.Rejected[TRejectReplayFailed]++
				continue
			}

			c := &transplantCand{tr: tr, sol: sol, elapsed: elapsed, fp: fp}
			alts++
			if best == nil || betterTransplantCand(c, best) {
				best = c
			}
		}
		if best == nil {
			continue
		}
		if alts > 1 {
			res.Rejected[TRejectSameSourceAlternative] += alts - 1
		}
		seenFPDuringScan[best.fp] = true
		pool = append(pool, pooled{rec: rec, c: best})
	}

	// Prefer filling harder buckets first so early easy solves do not crowd out hard ones.
	bucketOrder := []string{"17-20", "14-16", "11-13", "8-10"}
	sort.SliceStable(pool, func(i, j int) bool {
		return betterTransplantCand(pool[i].c, pool[j].c)
	})

	acceptedFP := map[string]bool{}
	acceptedSource := map[string]bool{}

	pick := func(wantBucket string) bool {
		for _, p := range pool {
			if acceptedSource[p.rec.SourcePuzzleID] {
				continue
			}
			if acceptedFP[p.c.fp] {
				continue
			}
			bkt := optimalBucket(p.c.sol.NumMoves)
			if wantBucket != "" && bkt != wantBucket {
				continue
			}
			q := cfg.BucketQuotas[bkt]
			if q > 0 && res.BucketCounts[bkt] >= q {
				continue
			}
			if len(res.Accepted) >= cfg.TargetAccepted {
				return false
			}

			id := fmt.Sprintf("Candidate_%03d", len(res.Accepted)+1)
			opt := p.c.sol.NumMoves
			p.c.tr.Level.LevelID = id
			p.c.tr.Level.Source = fmt.Sprintf("RushDatabase/%s/%s/%s", cfg.DatasetName, p.rec.SourcePuzzleID, p.c.tr.Embedding)
			p.c.tr.Level.CanonicalGestures = &opt
			p.c.tr.Level.Transplant = &TransplantJSON{
				SourceDataset:            cfg.DatasetName,
				SourcePuzzleId:           p.rec.SourcePuzzleID,
				SourceLine:               p.rec.LineNumber,
				OriginalBoard:            p.rec.Board36,
				OriginalOptimalMoves:     p.rec.OriginalOptimalMoves,
				OriginalClusterSize:      p.rec.OriginalClusterSize,
				TransformRotation:        p.c.tr.Rotation,
				EmbeddingVariant:         string(p.c.tr.Embedding),
				OffsetX:                  p.c.tr.OffsetX,
				OffsetY:                  p.c.tr.OffsetY,
				CargoFlowOptimalGestures: &opt,
			}
			doc := ExportSolutionJSON(p.c.tr.Level, p.c.tr.Board, p.c.sol, p.c.elapsed, true)
			res.Accepted = append(res.Accepted, TransplantAccepted{
				CandidateID:          id,
				Source:               p.rec,
				Embedding:            p.c.tr.Embedding,
				Rotation:             p.c.tr.Rotation,
				OffsetX:              p.c.tr.OffsetX,
				OffsetY:              p.c.tr.OffsetY,
				Level:                p.c.tr.Level,
				Board:                p.c.tr.Board,
				OptimalGestures:      p.c.sol.NumMoves,
				VisitedStates:        p.c.sol.MemoSize,
				ElapsedMs:            p.c.elapsed.Milliseconds(),
				Fingerprint:          p.c.fp,
				Solution:             p.c.sol,
				SolutionDoc:          doc,
				ReplayVerified:       true,
				ASCIIPreview:         p.c.tr.ASCIIPreview,
				OriginalOptimalMoves: p.rec.OriginalOptimalMoves,
				OriginalClusterSize:  p.rec.OriginalClusterSize,
			})
			acceptedFP[p.c.fp] = true
			acceptedSource[p.rec.SourcePuzzleID] = true
			res.BucketCounts[bkt]++
			return true
		}
		return false
	}

	for _, bkt := range bucketOrder {
		q := cfg.BucketQuotas[bkt]
		for res.BucketCounts[bkt] < q && len(res.Accepted) < cfg.TargetAccepted {
			if !pick(bkt) {
				break
			}
		}
	}
	for len(res.Accepted) < cfg.TargetAccepted {
		if !pick("") {
			break
		}
	}

	res.TotalElapsed = time.Since(start)
	return res, nil
}

type transplantCand struct {
	tr      *TransplantResult
	sol     Solution
	elapsed time.Duration
	fp      string
}

func betterTransplantCand(a, b *transplantCand) bool {
	if a.sol.NumMoves != b.sol.NumMoves {
		return a.sol.NumMoves > b.sol.NumMoves
	}
	if a.tr.Embedding != b.tr.Embedding {
		return embeddingRank(a.tr.Embedding) < embeddingRank(b.tr.Embedding)
	}
	return a.sol.MemoSize < b.sol.MemoSize
}

func embeddingRank(v EmbeddingVariant) int {
	switch v {
	case EmbedFlushTop:
		return 0
	case EmbedShiftDown1:
		return 1
	case EmbedFlushBottom:
		return 2
	default:
		return 9
	}
}

// WriteTransplantBatch writes candidates, solutions, and BatchManifest.json.
func WriteTransplantBatch(dir string, result TransplantBatchResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, c := range result.Accepted {
		if err := WriteJSONFile(filepath.Join(dir, c.CandidateID+".json"), c.Level); err != nil {
			return err
		}
		if err := WriteJSONFile(filepath.Join(dir, c.CandidateID+".solution.json"), c.SolutionDoc); err != nil {
			return err
		}
		if c.ASCIIPreview != "" {
			_ = os.WriteFile(filepath.Join(dir, c.CandidateID+".ascii.txt"), []byte(c.ASCIIPreview), 0o644)
		}
	}
	manifest := result.ToManifest()
	return WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), manifest)
}

// TransplantManifestJSON is the RUSH-008 batch manifest (Unity-compatible extensions).
type TransplantManifestJSON struct {
	GeneratorVersion string                       `json:"generatorVersion"`
	Pipeline         string                       `json:"pipeline"`
	SourceDataset    string                       `json:"sourceDataset"`
	SourceScanned    int                          `json:"sourceScanned"`
	TransformAttempts int                         `json:"transformAttempts"`
	ValidTransforms  int                          `json:"validTransforms"`
	ExactSolved      int                          `json:"exactSolved"`
	AcceptedCount    int                          `json:"accepted"`
	Rejected         map[string]int               `json:"rejected"`
	BucketCounts     map[string]int               `json:"bucketCounts"`
	TotalElapsedMs   int64                        `json:"totalElapsedMs"`
	MinOptimal       int                          `json:"minOptimalGestures"`
	MaxOptimal       int                          `json:"maxOptimalGestures"`
	Candidates       []TransplantManifestCandJSON `json:"candidates"`
}

type TransplantManifestCandJSON struct {
	CandidateID            string  `json:"candidateId"`
	SourcePuzzleId         string  `json:"sourcePuzzleId"`
	SourceLine             int     `json:"sourceLine"`
	OriginalOptimalMoves   int     `json:"originalOptimalMoves"`
	OriginalClusterSize    int     `json:"originalClusterSize"`
	EmbeddingVariant       string  `json:"embeddingVariant"`
	TransformRotation      string  `json:"transformRotation"`
	PieceCount             int     `json:"pieceCount"`
	OptimalGestures        int     `json:"optimalGestures"`
	VisitedStates          int     `json:"visitedStates"`
	ElapsedMs              int64   `json:"elapsedMs"`
	Fingerprint            string  `json:"fingerprint"`
	ReplayVerified         bool    `json:"replayVerified"`
	LevelFile              string  `json:"levelFile"`
	SolutionFile           string  `json:"solutionFile"`
}

func (r TransplantBatchResult) ToManifest() TransplantManifestJSON {
	rej := map[string]int{}
	for k, v := range r.Rejected {
		rej[string(k)] = v
	}
	m := TransplantManifestJSON{
		GeneratorVersion:  "cf-transplant-poc-1",
		Pipeline:          "RushDatabaseTransplant",
		SourceDataset:     r.Config.DatasetName,
		SourceScanned:     r.SourceScanned,
		TransformAttempts: r.TransformAttempts,
		ValidTransforms:   r.ValidTransforms,
		ExactSolved:       r.ExactSolved,
		AcceptedCount:     len(r.Accepted),
		Rejected:          rej,
		BucketCounts:      r.BucketCounts,
		TotalElapsedMs:    r.TotalElapsed.Milliseconds(),
		MinOptimal:        r.Config.MinOptimal,
		MaxOptimal:        r.Config.MaxOptimal,
	}
	for _, c := range r.Accepted {
		m.Candidates = append(m.Candidates, TransplantManifestCandJSON{
			CandidateID:          c.CandidateID,
			SourcePuzzleId:       c.Source.SourcePuzzleID,
			SourceLine:           c.Source.LineNumber,
			OriginalOptimalMoves: c.OriginalOptimalMoves,
			OriginalClusterSize:  c.OriginalClusterSize,
			EmbeddingVariant:     string(c.Embedding),
			TransformRotation:    c.Rotation,
			PieceCount:           len(c.Level.Pieces),
			OptimalGestures:      c.OptimalGestures,
			VisitedStates:        c.VisitedStates,
			ElapsedMs:            c.ElapsedMs,
			Fingerprint:          c.Fingerprint,
			ReplayVerified:       c.ReplayVerified,
			LevelFile:            c.CandidateID + ".json",
			SolutionFile:         c.CandidateID + ".solution.json",
		})
	}
	return m
}

func (r TransplantBatchResult) SolveTimeStats() (SolveTimeStats, bool) {
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

// UniqueSourceIDs reports whether all accepted candidates have distinct sources.
func (r TransplantBatchResult) UniqueSourceIDs() bool {
	seen := map[string]bool{}
	for _, c := range r.Accepted {
		if seen[c.Source.SourcePuzzleID] {
			return false
		}
		seen[c.Source.SourcePuzzleID] = true
	}
	return true
}

func sortedRejectionKeys(m map[TransplantRejection]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	return keys
}
