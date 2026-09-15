package rush

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CuratorConfig bounds the RUSH-009 offline curator.
type CuratorConfig struct {
	DatabasePath       string
	DatasetTag         string
	OutputDir          string
	CachePath          string
	CalibrationPath    string
	Seed               int64
	Workers            int
	SourcePoolSize     int
	MaxEmbeddings      int
	MaxSolveAttempts   int
	ShortlistSize      int
	MinOptimal         int
	MaxOptimal         int
	FamilyThreshold    float64
	SolveTimeLimit     time.Duration
	MaxVisitedStates   int
	Resume             bool
	Embeddings         []EmbeddingVariant
	BandQuotas         map[string]int
}

func DefaultCuratorConfig(databasePath string) CuratorConfig {
	return CuratorConfig{
		DatabasePath:     databasePath,
		DatasetTag:       "rush",
		OutputDir:        "output/RUSH009_CuratedShortlist_001",
		CachePath:        "data/cache/curator/solve_cache.jsonl",
		CalibrationPath:  "data/calibration/rush008_human_review.json",
		Seed:             20260915,
		Workers:          2,
		SourcePoolSize:   1500,
		MaxEmbeddings:    2,
		MaxSolveAttempts: 3000,
		ShortlistSize:    24,
		MinOptimal:       6,
		MaxOptimal:       40,
		FamilyThreshold:  0.28,
		SolveTimeLimit:   10 * time.Second,
		MaxVisitedStates: 4_000_000,
		Resume:           true,
		Embeddings:       []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1},
		BandQuotas: map[string]int{
			"6-8": 5, "9-11": 5, "12-14": 5, "15-17": 5, "18+": 4,
		},
	}
}

// CuratorSolvedCandidate is one exact-solved transformed candidate in the pool.
type CuratorSolvedCandidate struct {
	Source        SourceFeatureVector
	Embedding     EmbeddingVariant
	Rotation      string
	OffsetX       int
	OffsetY       int
	Board         *Board
	Level         *LevelJSON
	Solution      Solution
	ElapsedMs     int64
	ReplayVerified bool
	Metrics       CandidateMetrics
	Signals       DifficultySignals
	ASCIIPreview  string
	CacheHit      bool
	FamilyID      string
}

// CuratorResult is the full curator run outcome.
type CuratorResult struct {
	Config                 CuratorConfig
	DatabaseHash           string
	SourceRecordsScanned   int
	CheapSourcesRetained   int
	TransformsAttempted    int
	ExactSolvesAttempted   int
	CacheHits              int
	ExactSolved            int
	TimeoutOrBudget        int
	PoolAfterFilter        int
	FamiliesFound          int
	Shortlisted            []CuratorSolvedCandidate
	Families               []PuzzleFamily
	Pool                   []CuratorSolvedCandidate
	TotalWallTime          time.Duration
	SolverWallTime         time.Duration
	SolveTimesMs           []int64
	Calibration            CalibrationReport
	RejectCounts           map[string]int
}

type CalibrationReport struct {
	Entries []CalibrationEntryResult `json:"entries"`
	Notes   []string                 `json:"notes"`
}

type CalibrationEntryResult struct {
	CandidateID          string  `json:"candidateId"`
	HumanFamily          string  `json:"humanFamily"`
	HumanMovesApprox     int     `json:"humanMovesApprox"`
	OptimalGestures      int     `json:"optimalGestures"`
	HumanEffortRatio     float64 `json:"humanEffortRatio"`
	NearestCandidateID   string  `json:"nearestCandidateId"`
	NearestHumanFamily   string  `json:"nearestHumanFamily"`
	NearestSimilarity    float64 `json:"nearestSimilarity"`
	DifficultyScore      float64 `json:"difficultyScore"`
}

type humanCalibrationFile struct {
	CorpusID string `json:"corpusId"`
	Note     string `json:"note"`
	BatchDir string `json:"batchDir"`
	Entries  []struct {
		CandidateID      string `json:"candidateId"`
		HumanMovesApprox int    `json:"humanMovesApprox"`
		HumanMovesRange  string `json:"humanMovesRange"`
		HumanDifficulty  string `json:"humanDifficulty"`
		SimilarityFamily string `json:"similarityFamily"`
	} `json:"entries"`
}

// RunCurator executes the bounded RUSH-009 pipeline.
func RunCurator(cfg CuratorConfig) (CuratorResult, error) {
	start := time.Now()
	res := CuratorResult{
		Config:       cfg,
		RejectCounts: map[string]int{},
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if len(cfg.Embeddings) == 0 {
		cfg.Embeddings = []EmbeddingVariant{EmbedFlushTop, EmbedShiftDown1}
	}
	if cfg.MaxEmbeddings > 0 && len(cfg.Embeddings) > cfg.MaxEmbeddings {
		cfg.Embeddings = cfg.Embeddings[:cfg.MaxEmbeddings]
	}

	dbHash, err := HashFileSHA256(cfg.DatabasePath)
	if err != nil {
		return res, fmt.Errorf("hash database: %w", err)
	}
	res.DatabaseHash = dbHash

	sampleCfg := DefaultStratifiedSampleConfig()
	sampleCfg.Seed = cfg.Seed
	sampleCfg.TargetPoolSize = cfg.SourcePoolSize
	sampleCfg.PerStratumCap = (cfg.SourcePoolSize + 4) / 5
	sampleCfg.DatasetTag = cfg.DatasetTag
	sources, scanned, err := StratifiedSampleSources(cfg.DatabasePath, sampleCfg)
	if err != nil {
		return res, err
	}
	res.SourceRecordsScanned = scanned
	res.CheapSourcesRetained = len(sources)

	cache, err := OpenSolveCache(cfg.CachePath)
	if err != nil {
		return res, err
	}

	budget := SolveBudget{TimeLimit: cfg.SolveTimeLimit, MaxVisited: cfg.MaxVisitedStates}
	type job struct {
		src SourceFeatureVector
		emb EmbeddingVariant
	}
	jobs := make([]curatorJob, 0, len(sources)*len(cfg.Embeddings))
	for _, src := range sources {
		for _, emb := range cfg.Embeddings {
			jobs = append(jobs, curatorJob{src: src, emb: emb})
			if cfg.MaxSolveAttempts > 0 && len(jobs) >= cfg.MaxSolveAttempts {
				break
			}
		}
		if cfg.MaxSolveAttempts > 0 && len(jobs) >= cfg.MaxSolveAttempts {
			break
		}
	}
	res.TransformsAttempted = len(jobs)

	outCh := make(chan curatorOutcome, cfg.Workers*2)
	jobCh := make(chan curatorJob, cfg.Workers*2)
	var wg sync.WaitGroup
	solverStart := time.Now()

	worker := func() {
		defer wg.Done()
		for j := range jobCh {
			key := SolveCacheKey{
				SourceDatabaseHash: dbHash,
				SourcePuzzleID:     j.src.SourcePuzzleID,
				TransformVersion:   TransformVersionCW90,
				EmbeddingVariant:   string(j.emb),
				CargoRulesVersion:  CargoRulesVersionCF,
				SolverVersion:      SolverVersionBFS,
			}
			if cfg.Resume {
				if e, ok := cache.Get(key); ok {
					outCh <- outcomeFromCache(j, e)
					continue
				}
			}
			rec := RushDBRecord{
				LineNumber:           j.src.LineNumber,
				SourcePuzzleID:       j.src.SourcePuzzleID,
				Board36:              j.src.Board36,
				OriginalOptimalMoves: j.src.OriginalOptimalMoves,
				OriginalClusterSize:  j.src.OriginalClusterSize,
			}
			tr, err := TransformRushRecordToCargoFlow(rec, j.emb)
			if err != nil {
				_ = cache.Put(SolveCacheEntry{Key: key, Valid: false, RejectReason: "InvalidTransform"})
				outCh <- curatorOutcome{rej: "InvalidTransform", hit: false}
				continue
			}
			if tr.Board.cargoTargetCanExit() {
				_ = cache.Put(SolveCacheEntry{Key: key, Valid: false, ImmediateVictory: true, RejectReason: "ImmediateVictory"})
				outCh <- curatorOutcome{rej: "ImmediateVictory"}
				continue
			}
			t0 := time.Now()
			sol := tr.Board.SolveWithBudget(budget)
			elapsed := time.Since(t0)
			entry := SolveCacheEntry{
				Key:             key,
				Valid:           true,
				OffsetX:         tr.OffsetX,
				OffsetY:         tr.OffsetY,
				ElapsedMs:       elapsed.Milliseconds(),
				TimedOut:        sol.TimedOut,
				BudgetExceeded:  sol.BudgetExceeded,
				Solved:          sol.Solvable,
				OptimalGestures: sol.NumMoves,
				VisitedStates:   sol.MemoSize,
				Fingerprint:     layoutFingerprint(tr.Board),
				Moves:           sol.Moves,
			}
			if sol.TimedOut || sol.BudgetExceeded {
				entry.RejectReason = "DifficultyUnknown"
				_ = cache.Put(entry)
				outCh <- curatorOutcome{rej: "DifficultyUnknown", ms: elapsed.Milliseconds()}
				continue
			}
			if !sol.Solvable {
				entry.RejectReason = "Unsolvable"
				_ = cache.Put(entry)
				outCh <- curatorOutcome{rej: "Unsolvable", ms: elapsed.Milliseconds()}
				continue
			}
			b2 := tr.Board.Copy()
			if err := b2.Replay(sol.Moves); err != nil {
				entry.RejectReason = "ReplayFailed"
				_ = cache.Put(entry)
				outCh <- curatorOutcome{rej: "ReplayFailed", ms: elapsed.Milliseconds()}
				continue
			}
			entry.ReplayVerified = true
			_ = cache.Put(entry)

			metrics := BuildCandidateMetrics(tr.Board, sol)
			signals := BuildDifficultySignals(metrics)
			cand := &CuratorSolvedCandidate{
				Source:         j.src,
				Embedding:      j.emb,
				Rotation:       tr.Rotation,
				OffsetX:        tr.OffsetX,
				OffsetY:        tr.OffsetY,
				Board:          tr.Board,
				Level:          tr.Level,
				Solution:       sol,
				ElapsedMs:      elapsed.Milliseconds(),
				ReplayVerified: true,
				Metrics:        metrics,
				Signals:        signals,
				ASCIIPreview:   tr.ASCIIPreview,
			}
			outCh <- curatorOutcome{cand: cand, ms: elapsed.Milliseconds()}
		}
	}

	wg.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go worker()
	}
	go func() {
		for _, j := range jobs {
			jobCh <- j
		}
		close(jobCh)
		wg.Wait()
		close(outCh)
	}()

	bestBySource := map[string]*CuratorSolvedCandidate{}
	for o := range outCh {
		res.ExactSolvesAttempted++
		if o.hit {
			res.CacheHits++
		}
		if o.ms > 0 {
			res.SolveTimesMs = append(res.SolveTimesMs, o.ms)
		}
		if o.rej != "" {
			res.RejectCounts[o.rej]++
			if o.rej == "DifficultyUnknown" {
				res.TimeoutOrBudget++
			}
			continue
		}
		if o.cand == nil {
			continue
		}
		res.ExactSolved++
		opt := o.cand.Metrics.OptimalGestures
		if opt < cfg.MinOptimal || opt > cfg.MaxOptimal {
			res.RejectCounts["OutsideRequestedRange"]++
			continue
		}
		prev := bestBySource[o.cand.Source.SourcePuzzleID]
		if prev == nil || betterCuratorCand(o.cand, prev) {
			bestBySource[o.cand.Source.SourcePuzzleID] = o.cand
			if prev != nil {
				res.RejectCounts["SameSourceAlternative"]++
			}
		} else {
			res.RejectCounts["SameSourceAlternative"]++
		}
	}
	res.SolverWallTime = time.Since(solverStart)

	pool := make([]CuratorSolvedCandidate, 0, len(bestBySource))
	for _, c := range bestBySource {
		pool = append(pool, *c)
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].Source.LineNumber != pool[j].Source.LineNumber {
			return pool[i].Source.LineNumber < pool[j].Source.LineNumber
		}
		return embeddingRank(pool[i].Embedding) < embeddingRank(pool[j].Embedding)
	})
	// Dedup fingerprints
	seenFP := map[string]bool{}
	dedup := pool[:0]
	for _, c := range pool {
		fp := c.Metrics.LayoutFingerprint
		if seenFP[fp] {
			res.RejectCounts["Duplicate"]++
			continue
		}
		seenFP[fp] = true
		dedup = append(dedup, c)
	}
	pool = dedup
	res.Pool = pool
	res.PoolAfterFilter = len(pool)

	sigs := make([]PuzzleSignature, len(pool))
	for i := range pool {
		sigs[i] = pool[i].Metrics.Signature
	}
	families := ClusterByFamilyDistance(sigs, cfg.FamilyThreshold)
	res.Families = families
	res.FamiliesFound = len(families)
	for _, f := range families {
		for _, mi := range f.MemberIndexes {
			pool[mi].FamilyID = f.FamilyID
		}
	}

	idxs := FarthestPointShortlist(pool, families, cfg.ShortlistSize, cfg.BandQuotas, cfg.Seed)
	short := make([]CuratorSolvedCandidate, 0, len(idxs))
	for i, idx := range idxs {
		c := pool[idx]
		id := fmt.Sprintf("Candidate_%03d", i+1)
		opt := c.Metrics.OptimalGestures
		c.Level.LevelID = id
		c.Level.Source = fmt.Sprintf("RushDatabaseCurator/%s/%s/%s", cfg.DatasetTag, c.Source.SourcePuzzleID, c.Embedding)
		c.Level.CanonicalGestures = &opt
		c.Level.Transplant = &TransplantJSON{
			SourceDataset:            cfg.DatasetTag,
			SourcePuzzleId:           c.Source.SourcePuzzleID,
			SourceLine:               c.Source.LineNumber,
			OriginalBoard:            c.Source.Board36,
			OriginalOptimalMoves:     c.Source.OriginalOptimalMoves,
			OriginalClusterSize:      c.Source.OriginalClusterSize,
			TransformRotation:        c.Rotation,
			EmbeddingVariant:         string(c.Embedding),
			OffsetX:                  c.OffsetX,
			OffsetY:                  c.OffsetY,
			CargoFlowOptimalGestures: &opt,
		}
		short = append(short, c)
	}
	res.Shortlisted = short

	cal, err := runCalibration(cfg, short)
	if err == nil {
		res.Calibration = cal
	} else {
		res.Calibration = CalibrationReport{Notes: []string{err.Error()}}
	}

	res.TotalWallTime = time.Since(start)
	return res, nil
}

func betterCuratorCand(a, b *CuratorSolvedCandidate) bool {
	if a.Metrics.OptimalGestures != b.Metrics.OptimalGestures {
		return a.Metrics.OptimalGestures > b.Metrics.OptimalGestures
	}
	if a.Embedding != b.Embedding {
		return embeddingRank(a.Embedding) < embeddingRank(b.Embedding)
	}
	return a.Metrics.VisitedStates < b.Metrics.VisitedStates
}

type curatorJob struct {
	src SourceFeatureVector
	emb EmbeddingVariant
}

type curatorOutcome struct {
	cand *CuratorSolvedCandidate
	rej  string
	ms   int64
	hit  bool
}

func outcomeFromCache(j curatorJob, e SolveCacheEntry) curatorOutcome {
	if !e.Valid || e.RejectReason != "" {
		rej := e.RejectReason
		if rej == "" {
			rej = "CachedReject"
		}
		return curatorOutcome{rej: rej, hit: true, ms: e.ElapsedMs}
	}
	if e.TimedOut || e.BudgetExceeded {
		return curatorOutcome{rej: "DifficultyUnknown", hit: true, ms: e.ElapsedMs}
	}
	if !e.Solved {
		return curatorOutcome{rej: "Unsolvable", hit: true, ms: e.ElapsedMs}
	}
	rec := RushDBRecord{
		LineNumber:           j.src.LineNumber,
		SourcePuzzleID:       j.src.SourcePuzzleID,
		Board36:              j.src.Board36,
		OriginalOptimalMoves: j.src.OriginalOptimalMoves,
		OriginalClusterSize:  j.src.OriginalClusterSize,
	}
	tr, err := TransformRushRecordToCargoFlow(rec, j.emb)
	if err != nil {
		return curatorOutcome{rej: "InvalidTransform", hit: true}
	}
	sol := Solution{
		Solvable: true,
		Moves:    e.Moves,
		NumMoves: e.OptimalGestures,
		MemoSize: e.VisitedStates,
	}
	metrics := BuildCandidateMetrics(tr.Board, sol)
	signals := BuildDifficultySignals(metrics)
	cand := &CuratorSolvedCandidate{
		Source:         j.src,
		Embedding:      j.emb,
		Rotation:       tr.Rotation,
		OffsetX:        tr.OffsetX,
		OffsetY:        tr.OffsetY,
		Board:          tr.Board,
		Level:          tr.Level,
		Solution:       sol,
		ElapsedMs:      e.ElapsedMs,
		ReplayVerified: e.ReplayVerified,
		Metrics:        metrics,
		Signals:        signals,
		ASCIIPreview:   tr.ASCIIPreview,
		CacheHit:       true,
	}
	return curatorOutcome{cand: cand, ms: e.ElapsedMs, hit: true}
}

// WriteCuratorShortlist writes Unity-compatible batch + reports.
func WriteCuratorShortlist(dir string, res CuratorResult) error {
	candDir := filepath.Join(dir, "Candidates")
	solDir := filepath.Join(dir, "Solutions")
	if err := os.MkdirAll(candDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(solDir, 0o755); err != nil {
		return err
	}
	for _, c := range res.Shortlisted {
		id := c.Level.LevelID
		if err := WriteJSONFile(filepath.Join(candDir, id+".json"), c.Level); err != nil {
			return err
		}
		doc := ExportSolutionJSON(c.Level, c.Board, c.Solution, time.Duration(c.ElapsedMs)*time.Millisecond, c.ReplayVerified)
		if err := WriteJSONFile(filepath.Join(solDir, id+".solution.json"), doc); err != nil {
			return err
		}
		if c.ASCIIPreview != "" {
			_ = os.WriteFile(filepath.Join(candDir, id+".ascii.txt"), []byte(c.ASCIIPreview), 0o644)
		}
	}
	if err := WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), res.ToBatchManifest()); err != nil {
		return err
	}
	if err := WriteJSONFile(filepath.Join(dir, "CuratorReport.json"), res.ToCuratorReport()); err != nil {
		return err
	}
	return writeHumanReviewCSV(filepath.Join(dir, "HumanReview.csv"), res.Shortlisted)
}

func writeHumanReviewCSV(path string, short []CuratorSolvedCandidate) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{
		"CandidateId", "SourcePuzzleId", "OptimalGestures", "EstimatedDifficulty",
		"FamilyId", "HumanMoves", "HumanTimeSeconds", "HumanRating", "TooSimilarTo", "Keep", "Notes",
	})
	for _, c := range short {
		_ = w.Write([]string{
			c.Level.LevelID,
			c.Source.SourcePuzzleID,
			fmt.Sprintf("%d", c.Metrics.OptimalGestures),
			fmt.Sprintf("%.1f", c.Signals.EstimatedHumanDifficultyScore),
			c.FamilyID,
			"", "", "", "", "", "",
		})
	}
	w.Flush()
	return w.Error()
}

func (r CuratorResult) ToBatchManifest() map[string]interface{} {
	cands := make([]map[string]interface{}, 0, len(r.Shortlisted))
	for _, c := range r.Shortlisted {
		cands = append(cands, map[string]interface{}{
			"candidateId":          c.Level.LevelID,
			"sourcePuzzleId":       c.Source.SourcePuzzleID,
			"originalOptimalMoves": c.Source.OriginalOptimalMoves,
			"optimalGestures":      c.Metrics.OptimalGestures,
			"familyId":             c.FamilyID,
			"embeddingVariant":     string(c.Embedding),
			"pieceCount":           c.Metrics.PieceCount,
			"visitedStates":        c.Metrics.VisitedStates,
			"fingerprint":          c.Metrics.LayoutFingerprint,
			"replayVerified":       c.ReplayVerified,
			"levelFile":            "Candidates/" + c.Level.LevelID + ".json",
			"solutionFile":         "Solutions/" + c.Level.LevelID + ".solution.json",
		})
	}
	return map[string]interface{}{
		"generatorVersion": CuratorVersion,
		"pipeline":         "RushDatabaseCurator",
		"sourceDataset":    r.Config.DatasetTag,
		"databaseHash":     r.DatabaseHash,
		"seed":             r.Config.Seed,
		"accepted":         len(r.Shortlisted),
		"familiesFound":    r.FamiliesFound,
		"candidates":       cands,
	}
}

func (r CuratorResult) ToCuratorReport() map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(r.Shortlisted))
	for i, c := range r.Shortlisted {
		nearestID, nearestSim := "", 0.0
		for j, o := range r.Shortlisted {
			if i == j {
				continue
			}
			sim := PuzzleFamilySimilarity(c.Metrics.Signature, o.Metrics.Signature)
			if sim > nearestSim {
				nearestSim = sim
				nearestID = o.Level.LevelID
			}
		}
		rows = append(rows, map[string]interface{}{
			"candidateId":            c.Level.LevelID,
			"sourcePuzzleId":         c.Source.SourcePuzzleID,
			"originalOptimalMoves":   c.Source.OriginalOptimalMoves,
			"cargoOptimalGestures":   c.Metrics.OptimalGestures,
			"familyId":               c.FamilyID,
			"nearestCandidate":       nearestID,
			"familySimilarity":       nearestSim,
			"pieceCount":             c.Metrics.PieceCount,
			"inventorySignature":     c.Metrics.InventorySignature,
			"dependencyDepth":        c.Metrics.DependencyDepth,
			"distinctMovedPieces":    c.Metrics.DistinctMovedPieces,
			"targetBlockers":         c.Metrics.InitialTargetBlockerCount,
			"visitedStates":          c.Metrics.VisitedStates,
			"estimatedHumanDifficulty": c.Signals.EstimatedHumanDifficultyScore,
			"sourceClusterSize":      c.Source.OriginalClusterSize,
			"replay":                 c.ReplayVerified,
			"fingerprint":            c.Metrics.LayoutFingerprint,
		})
	}
	famRows := make([]map[string]interface{}, 0, len(r.Families))
	for _, f := range r.Families {
		famRows = append(famRows, map[string]interface{}{
			"familyId":             f.FamilyID,
			"membersInSolvedPool":  len(f.MemberIndexes),
			"representativeIndex":  f.RepresentativeIdx,
			"averageOptimal":       f.AvgOptimal,
			"inventoryProfile":     f.InventoryProfile,
			"dominantSolutionSig":  truncate(f.DominantSequence, 120),
		})
	}
	st := SolveTimeStats{}
	if len(r.SolveTimesMs) > 0 {
		st = SolveTimeStats{
			Count:   len(r.SolveTimesMs),
			Average: meanInt64(r.SolveTimesMs),
			Median:  medianInt64(r.SolveTimesMs),
			Max:     maxInt64(r.SolveTimesMs),
		}
	}
	cacheSavedEst := float64(r.CacheHits) * st.Average
	return map[string]interface{}{
		"performance": map[string]interface{}{
			"sourceRecordsScanned":   r.SourceRecordsScanned,
			"cheapSourcesRetained":   r.CheapSourcesRetained,
			"transformsAttempted":    r.TransformsAttempted,
			"exactSolvesAttempted":   r.ExactSolvesAttempted,
			"cacheHits":              r.CacheHits,
			"exactSolved":            r.ExactSolved,
			"timeoutOrBudget":        r.TimeoutOrBudget,
			"poolAfterFilter":        r.PoolAfterFilter,
			"familiesFound":          r.FamiliesFound,
			"finalShortlisted":       len(r.Shortlisted),
			"totalWallMs":            r.TotalWallTime.Milliseconds(),
			"solverWallMs":           r.SolverWallTime.Milliseconds(),
			"solveTimeAvgMs":         st.Average,
			"solveTimeMedianMs":      st.Median,
			"solveTimeMaxMs":         st.Max,
			"cacheSavedTimeEstMs":    cacheSavedEst,
		},
		"rejectCounts":  r.RejectCounts,
		"shortlist":     rows,
		"families":      famRows,
		"calibration":   r.Calibration,
		"limitations": []string{
			"Rush source has no Cargo Flow movable 1x1 — shortlist is Rush-derived subset.",
			"EstimatedHumanDifficulty is experimental, not objective truth.",
			"12 human-reviewed examples are insufficient for a statistical model.",
			"Curator selects diversity; final human review is still required.",
			"DecisionAmbiguity deferred (too expensive for full pool).",
		},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func runCalibration(cfg CuratorConfig, short []CuratorSolvedCandidate) (CalibrationReport, error) {
	rep := CalibrationReport{}
	data, err := os.ReadFile(cfg.CalibrationPath)
	if err != nil {
		return rep, err
	}
	var file humanCalibrationFile
	if err := json.Unmarshal(data, &file); err != nil {
		return rep, err
	}
	// Load RUSH008 levels for signature comparison among human corpus.
	type loaded struct {
		id   string
		fam  string
		moves int
		sig  PuzzleSignature
		opt  int
		score float64
	}
	loadedList := []loaded{}
	for _, e := range file.Entries {
		path := filepath.Join(file.BatchDir, e.CandidateID+".json")
		level, err := LoadCargoFlowLevelJSONFile(path)
		if err != nil {
			rep.Notes = append(rep.Notes, fmt.Sprintf("missing %s: %v", e.CandidateID, err))
			continue
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			rep.Notes = append(rep.Notes, fmt.Sprintf("board %s: %v", e.CandidateID, err))
			continue
		}
		solPath := filepath.Join(file.BatchDir, e.CandidateID+".solution.json")
		solDoc, err := loadSolutionJSON(solPath)
		if err != nil {
			// re-solve lightly
			sol := board.SolveWithBudget(SolveBudget{TimeLimit: 5 * time.Second, MaxVisited: 2_000_000})
			if !sol.Solvable {
				continue
			}
			sig := BuildPuzzleSignature(board, sol)
			metrics := BuildCandidateMetrics(board, sol)
			signals := BuildDifficultySignals(metrics)
			loadedList = append(loadedList, loaded{
				id: e.CandidateID, fam: e.SimilarityFamily, moves: e.HumanMovesApprox,
				sig: sig, opt: sol.NumMoves, score: signals.EstimatedHumanDifficultyScore,
			})
			continue
		}
		sol := solutionFromDoc(board, solDoc)
		sig := BuildPuzzleSignature(board, sol)
		metrics := BuildCandidateMetrics(board, sol)
		signals := BuildDifficultySignals(metrics)
		loadedList = append(loadedList, loaded{
			id: e.CandidateID, fam: e.SimilarityFamily, moves: e.HumanMovesApprox,
			sig: sig, opt: sol.NumMoves, score: signals.EstimatedHumanDifficultyScore,
		})
		_ = short
	}
	for _, a := range loadedList {
		bestID, bestFam := "", ""
		bestSim := -1.0
		for _, b := range loadedList {
			if a.id == b.id {
				continue
			}
			sim := PuzzleFamilySimilarity(a.sig, b.sig)
			if sim > bestSim {
				bestSim = sim
				bestID = b.id
				bestFam = b.fam
			}
		}
		ratio := 0.0
		if a.opt > 0 {
			ratio = float64(a.moves) / float64(a.opt)
		}
		rep.Entries = append(rep.Entries, CalibrationEntryResult{
			CandidateID:        a.id,
			HumanFamily:        a.fam,
			HumanMovesApprox:   a.moves,
			OptimalGestures:    a.opt,
			HumanEffortRatio:   round3(ratio),
			NearestCandidateID: bestID,
			NearestHumanFamily: bestFam,
			NearestSimilarity:  bestSim,
			DifficultyScore:    a.score,
		})
	}
	return rep, nil
}

func loadSolutionJSON(path string) (SolutionJSON, error) {
	var doc SolutionJSON
	data, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	err = json.Unmarshal(data, &doc)
	return doc, err
}

func solutionFromDoc(board *Board, doc SolutionJSON) Solution {
	// Prefer replaying exported gestures by re-solving for authoritative moves if needed.
	// For calibration, re-solve is safer than reconstructing piece indices from JSON.
	_ = board
	_ = doc
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 8 * time.Second, MaxVisited: 4_000_000})
	return sol
}
