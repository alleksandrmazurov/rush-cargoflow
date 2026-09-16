package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const BoardMixVersion = "boardmix-v1.1"

// BoardMixConfig drives RUSH-010.3 / 010.3.1 board-space diversity generation.
type BoardMixConfig struct {
	BatchDir                  string         `json:"batchDir"`
	OutputDir                 string         `json:"outputDir"`
	CachePath                 string         `json:"cachePath"`
	CheckpointPath            string         `json:"checkpointPath"`
	DatabasePath              string         `json:"databasePath"`
	TargetAccepted            int            `json:"targetAccepted"`
	BaseCount                 int            `json:"baseCount"`
	Seed                      int64          `json:"seed"`
	Workers                   int            `json:"workers"`
	CheckpointEvery           int            `json:"checkpointEvery"`
	Resume                    bool           `json:"resume"`
	SolveTimeLimitMs          int            `json:"solveTimeLimitMs"`
	MaxVisitedStates          int            `json:"maxVisitedStates"`
	TryOuterAugment           bool           `json:"tryOuterAugment"`
	TryInventoryEnrichment    bool           `json:"tryInventoryEnrichment"`
	Embeddings                []string       `json:"embeddings"`
	ShapeQuotas               map[string]int `json:"shapeQuotas"`
	InventoryQuotas           map[string]int `json:"inventoryQuotas"`
	ProgressEvery             int            `json:"progressEvery"`
	MaxPoolSize               int            `json:"maxPoolSize"`
	MaxAttempts               int            `json:"maxAttempts"`
	MinDistinctBoardShapes    int            `json:"minDistinctBoardShapes"`
	MaxBoardShapeFraction     float64        `json:"maxBoardShapeFraction"`
	MinOuterZoneRelevant      int            `json:"minOuterZoneRelevant"`
	MaxInventoryClassFraction float64        `json:"maxInventoryClassFraction"`
	UniqueFamily              bool           `json:"uniqueFamily"`
}

// DefaultBoardMixConfig returns sane long-run defaults (manual pilot).
func DefaultBoardMixConfig() BoardMixConfig {
	return BoardMixConfig{
		BatchDir:               "output/RUSH009_CuratedShortlist_001",
		OutputDir:              "output/RUSH01031_DiversityQuotaPilot_001",
		CachePath:              "data/cache/curator/solve_cache.jsonl",
		DatabasePath:           "data/external/rush/rush.txt",
		TargetAccepted:         12,
		BaseCount:              24,
		Seed:                   20260916,
		Workers:                2,
		CheckpointEvery:        1,
		Resume:                 true,
		SolveTimeLimitMs:       8000,
		MaxVisitedStates:       2_000_000,
		TryOuterAugment:        true,
		TryInventoryEnrichment: true,
		Embeddings: []string{
			string(EmbedFlushTop), string(EmbedShiftDown1), string(EmbedFlushBottom),
			string(EmbedFlushTopMirrorH), string(EmbedShiftDown1MirrorH),
		},
		ShapeQuotas: map[string]int{
			string(ShapeCompact6x6):  3,
			string(ShapeShiftedCore): 3,
			string(ShapeExpanded):    3,
			string(ShapeTall):        1,
			string(ShapeWide):        1,
			string(ShapeFullField):   1,
		},
		InventoryQuotas: map[string]int{
			string(InvNo1x1): 3, string(InvOne1x1): 3,
			string(InvTwo1x1): 3, string(InvThree1x1): 3,
		},
		ProgressEvery:             1,
		MaxPoolSize:               80,
		MaxAttempts:               200,
		MinDistinctBoardShapes:    3,
		MaxBoardShapeFraction:     0.40,
		MinOuterZoneRelevant:      4,
		MaxInventoryClassFraction: 0.40,
		UniqueFamily:              true,
	}
}

// LoadBoardMixConfigJSON loads config from path and merges defaults.
func LoadBoardMixConfigJSON(path string) (BoardMixConfig, error) {
	cfg := DefaultBoardMixConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	// Strip UTF-8 BOM from editors / PowerShell Set-Content.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.CheckpointPath == "" {
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.CheckpointEvery <= 0 {
		cfg.CheckpointEvery = 1
	}
	if cfg.MaxPoolSize <= 0 {
		cfg.MaxPoolSize = 80
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 200
	}
	if cfg.MinDistinctBoardShapes <= 0 {
		cfg.MinDistinctBoardShapes = 3
	}
	if cfg.MaxBoardShapeFraction <= 0 {
		cfg.MaxBoardShapeFraction = 0.40
	}
	if cfg.MaxInventoryClassFraction <= 0 {
		cfg.MaxInventoryClassFraction = 0.40
	}
	if cfg.MinOuterZoneRelevant <= 0 {
		cfg.MinOuterZoneRelevant = 4
	}
	return cfg, nil
}

// BoardMixCheckpoint is resumable progress for long boardmix runs.
type BoardMixCheckpoint struct {
	Version              string             `json:"version"`
	ConfigPath           string             `json:"configPath,omitempty"`
	CompletedAttemptKeys []string           `json:"completedAttemptKeys"`
	AcceptedIDs          []string           `json:"acceptedIds"`
	Accepted             []BoardMixAccepted `json:"accepted"`
	Pool                 []BoardMixAccepted `json:"pool"`
	Rejected             map[string]int     `json:"rejected"`
	Stats                BoardMixStats      `json:"stats"`
	SelectReport         BoardMixSelectReport `json:"selectReport,omitempty"`
	UpdatedAt            time.Time          `json:"updatedAt"`
}

type BoardMixStats struct {
	Attempts       int   `json:"attempts"`
	ExactSolves    int   `json:"exactSolves"`
	CacheHits      int   `json:"cacheHits"`
	Accepted       int   `json:"accepted"`
	ElapsedMs      int64 `json:"elapsedMs"`
	LastProgressAt time.Time `json:"lastProgressAt"`
}

// BoardMixAccepted is one validated board-diversity candidate.
type BoardMixAccepted struct {
	CandidateID      string                  `json:"candidateId"`
	BaseCandidateID  string                  `json:"baseCandidateId"`
	FamilyID         string                  `json:"familyId"`
	SourcePuzzleID   string                  `json:"sourcePuzzleId"`
	Embedding        EmbeddingVariant        `json:"embedding"`
	OffsetX          int                     `json:"offsetX"`
	OffsetY          int                     `json:"offsetY"`
	InventoryClass   InventoryClass          `json:"inventoryClass"`
	BoardUtil        BoardUtilizationMetrics `json:"boardUtilization"`
	OptimalGestures  int                     `json:"optimalGestures"`
	SelectionBand    string                  `json:"selectionBand"`
	Augmented        bool                    `json:"augmented"`
	ReplayVerified   bool                    `json:"replayVerified"`
	Level            *LevelJSON              `json:"level,omitempty"`
	Board            *Board                  `json:"-"`
	Solution         Solution                `json:"-"`
	SolutionDoc      SolutionJSON            `json:"-"`
	ASCIIPreview     string                  `json:"-"`
}

// BoardMixResult is the final run summary.
type BoardMixResult struct {
	Config       BoardMixConfig
	Accepted     []BoardMixAccepted
	Pool         []BoardMixAccepted
	Rejected     map[string]int
	Stats        BoardMixStats
	ShapeDist    map[string]int
	InvDist      map[string]int
	SelectReport BoardMixSelectReport
	TotalWall    time.Duration
	RootCauseNote string
}

type boardMixAttemptKey struct {
	BaseID    string
	Embedding string
	Augment   string
}

func (k boardMixAttemptKey) String() string {
	return k.BaseID + "|" + k.Embedding + "|" + k.Augment
}

// TryAddOuterStructuralPiece places one gameplay-relevant piece in the outer zone.
// Returns nil board if no valid placement found quickly.
func TryAddOuterStructuralPiece(board *Board, offX, offY int, budget SolveBudget) (*Board, Solution, bool) {
	baseSol := board.SolveWithBudget(budget)
	if !baseSol.Solvable || baseSol.TimedOut || baseSol.BudgetExceeded {
		return nil, Solution{}, false
	}
	w, h := board.Width, board.Height
	candidates := []int{}
	for cell := 0; cell < w*h; cell++ {
		if board.occupied[cell] {
			continue
		}
		if cellInOriginal6x6Core(cell, w, offX, offY) {
			continue
		}
		candidates = append(candidates, cell)
	}
	sort.Ints(candidates)
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}
	for _, cell := range candidates {
		// Prefer 1x1 unit in outer zone.
		b := board.Copy()
		p := Piece{Position: cell, Size: 1, Orientation: Horizontal, Kind: PieceUnit}
		if !b.AddPiece(p) {
			continue
		}
		b.Labels = append(b.Labels, fmt.Sprintf("U%02d", len(b.Pieces)))
		if b.cargoTargetCanExit() {
			continue
		}
		sol := b.SolveWithBudget(budget)
		if sol.TimedOut || sol.BudgetExceeded || !sol.Solvable {
			continue
		}
		// Must move the added unit or worsen without it (essential-ish).
		unitIdx := len(b.Pieces) - 1
		moved := false
		work := b.Copy()
		for _, m := range sol.Moves {
			if m.Piece == unitIdx {
				moved = true
			}
			work.DoMove(m)
		}
		if !moved && sol.NumMoves <= baseSol.NumMoves {
			continue
		}
		util := ComputeBoardUtilization(b, offX, offY, &sol)
		if !util.OuterZoneRelevant && util.PiecesOutsideOriginal6x6Core == 0 {
			continue
		}
		return b, sol, true
	}
	return nil, Solution{}, false
}

// RunBoardMixPilot generates board-shape diversity candidates with checkpoint/resume.
func RunBoardMixPilot(cfg BoardMixConfig, cancel <-chan struct{}) (BoardMixResult, error) {
	start := time.Now()
	out := BoardMixResult{
		Config:        cfg,
		Rejected:      map[string]int{},
		ShapeDist:     map[string]int{},
		InvDist:       map[string]int{},
		RootCauseNote: "RUSH-010.3.1: generate pool then diversity-select; do not stop at first N valids.",
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return out, err
	}
	if cfg.CheckpointPath == "" {
		cfg.CheckpointPath = filepath.Join(cfg.OutputDir, "checkpoint.json")
	}
	if cfg.MaxPoolSize <= 0 {
		cfg.MaxPoolSize = 80
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 200
	}

	bases, err := SelectEnrichmentBases(EnrichmentConfig{
		BatchDir:          cfg.BatchDir,
		CuratorReportPath: filepath.Join(cfg.BatchDir, "CuratorReport.json"),
		BaseCount:         cfg.BaseCount,
		SolveTimeLimit:    time.Duration(cfg.SolveTimeLimitMs) * time.Millisecond,
		MaxVisitedStates:  cfg.MaxVisitedStates,
	})
	if err != nil {
		return out, err
	}

	embeds := parseEmbedList(cfg.Embeddings)
	if len(embeds) == 0 {
		embeds = BoardDiversityEmbeddingVariants()
	}

	var cache *SolveCache
	dbHash := ""
	if cfg.CachePath != "" {
		cache, err = OpenSolveCache(cfg.CachePath)
		if err != nil {
			return out, err
		}
		if cfg.DatabasePath != "" {
			if h, err := HashFileSHA256(cfg.DatabasePath); err == nil {
				dbHash = h
			}
		}
	}

	cp := BoardMixCheckpoint{
		Version:              BoardMixVersion,
		CompletedAttemptKeys: []string{},
		Rejected:             map[string]int{},
		Pool:                 []BoardMixAccepted{},
	}
	completed := map[string]bool{}
	pool := []BoardMixAccepted{}
	if cfg.Resume {
		if loaded, err := LoadBoardMixCheckpoint(cfg.CheckpointPath); err == nil {
			cp = loaded
			for _, k := range cp.CompletedAttemptKeys {
				completed[k] = true
			}
			out.Rejected = cp.Rejected
			if out.Rejected == nil {
				out.Rejected = map[string]int{}
			}
			pool = append(pool, cp.Pool...)
			out.Stats = cp.Stats
			// Rehydrate boards from Level for enrichment/selection replay.
			for i := range pool {
				if pool[i].Board == nil && pool[i].Level != nil {
					if b, err := BoardFromLevelJSON(pool[i].Level); err == nil {
						pool[i].Board = b
					}
				}
			}
		}
	}

	budget := SolveBudget{
		TimeLimit:  time.Duration(cfg.SolveTimeLimitMs) * time.Millisecond,
		MaxVisited: cfg.MaxVisitedStates,
	}
	if budget.TimeLimit <= 0 {
		budget.TimeLimit = 8 * time.Second
	}
	if budget.MaxVisited <= 0 {
		budget.MaxVisited = 2_000_000
	}

	type job struct {
		base    EnrichmentBase
		embed   EmbeddingVariant
		augment bool
	}
	jobs := []job{}
	for _, base := range bases {
		for _, emb := range embeds {
			jobs = append(jobs, job{base: base, embed: emb, augment: false})
			if cfg.TryOuterAugment {
				jobs = append(jobs, job{base: base, embed: emb, augment: true})
			}
		}
	}

	var mu sync.Mutex
	var attempts int32
	stop := false
	workers := cfg.Workers
	if workers <= 0 {
		workers = 2
	}
	ch := make(chan job, len(jobs))
	for _, j := range jobs {
		ch <- j
	}
	close(ch)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if cancel != nil {
					select {
					case <-cancel:
						return
					default:
					}
				}
				mu.Lock()
				if stop || len(pool) >= cfg.MaxPoolSize || int(attempts) >= cfg.MaxAttempts {
					stop = true
					mu.Unlock()
					return
				}
				mu.Unlock()

				augTag := "plain"
				if j.augment {
					augTag = "outer1"
				}
				key := boardMixAttemptKey{BaseID: j.base.CandidateID, Embedding: string(j.embed), Augment: augTag}.String()
				mu.Lock()
				if completed[key] {
					mu.Unlock()
					continue
				}
				mu.Unlock()

				cand, reason, cached := evaluateBoardMixJob(j.base, j.embed, j.augment, budget, cache, dbHash)
				n := int(atomic.AddInt32(&attempts, 1))
				mu.Lock()
				completed[key] = true
				cp.CompletedAttemptKeys = append(cp.CompletedAttemptKeys, key)
				out.Stats.Attempts++
				if cached {
					out.Stats.CacheHits++
				} else {
					out.Stats.ExactSolves++
				}
				if reason != "" {
					out.Rejected[reason]++
					cand = nil
				}
				cp.Rejected = copyIntMap(out.Rejected)
				cp.Stats = out.Stats
				cp.Stats.ElapsedMs = time.Since(start).Milliseconds()
				cp.Stats.LastProgressAt = time.Now()
				cp.UpdatedAt = time.Now()
				doEnrich := cfg.TryInventoryEnrichment && cand != nil && cand.Board != nil
				plain := cand
				mu.Unlock()

				var enriched []BoardMixAccepted
				if doEnrich && plain != nil {
					tmp := BoardMixResult{Rejected: map[string]int{}, Stats: BoardMixStats{}}
					enriched = enrichBoardMixInventory(*plain, budget, &tmp)
					mu.Lock()
					for k, v := range tmp.Rejected {
						out.Rejected[k] += v
					}
					out.Stats.ExactSolves += tmp.Stats.ExactSolves
					mu.Unlock()
				}

				mu.Lock()
				if plain != nil {
					pool = append(pool, *plain)
					pool = append(pool, enriched...)
				}
				cp.Pool = append([]BoardMixAccepted{}, pool...)
				cp.Rejected = copyIntMap(out.Rejected)
				if cfg.CheckpointEvery > 0 && n%cfg.CheckpointEvery == 0 {
					_ = SaveBoardMixCheckpoint(cfg.CheckpointPath, cp)
					_ = writeBoardMixProgress(cfg.OutputDir, cp, len(jobs))
				}
				if cfg.ProgressEvery > 0 && n%cfg.ProgressEvery == 0 {
					fmt.Printf("[%d/%d] Pool: %d  Solved: %d  Cached: %d  Rejected: %d  Elapsed: %s\n",
						n, len(jobs), len(pool), out.Stats.ExactSolves, out.Stats.CacheHits,
						sumIntMap(out.Rejected), time.Since(start).Round(time.Second))
				}
				if len(pool) >= cfg.MaxPoolSize || n >= cfg.MaxAttempts {
					stop = true
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Diversity-aware final selection (does NOT take first N valids).
	selected, selRep := SelectBoardMixShortlist(pool, cfg)
	out.Pool = pool
	out.Accepted = selected
	out.SelectReport = selRep
	out.ShapeDist = selRep.FinalShapeDist
	out.InvDist = selRep.FinalInventoryDist
	out.Stats.Accepted = len(selected)
	out.Stats.ElapsedMs = time.Since(start).Milliseconds()
	out.TotalWall = time.Since(start)

	cp.Pool = append([]BoardMixAccepted{}, pool...)
	cp.Accepted = append([]BoardMixAccepted{}, selected...)
	cp.AcceptedIDs = nil
	for _, a := range selected {
		cp.AcceptedIDs = append(cp.AcceptedIDs, a.CandidateID)
	}
	cp.SelectReport = selRep
	cp.Stats = out.Stats
	cp.Rejected = copyIntMap(out.Rejected)
	cp.UpdatedAt = time.Now()
	_ = SaveBoardMixCheckpoint(cfg.CheckpointPath, cp)
	_ = writeBoardMixProgress(cfg.OutputDir, cp, len(jobs))
	return out, nil
}

// enrichBoardMixInventory reuses RUSH-010.2 placement/relevance tests for 1/2/3 movable 1x1.
func enrichBoardMixInventory(baseCand BoardMixAccepted, budget SolveBudget, res *BoardMixResult) []BoardMixAccepted {
	if baseCand.Board == nil {
		return nil
	}
	eb := EnrichmentBase{
		CandidateID:     baseCand.BaseCandidateID,
		FamilyID:        baseCand.FamilyID,
		SourcePuzzleID:  baseCand.SourcePuzzleID,
		BaseOptimal:     baseCand.OptimalGestures,
		VisitedStates:   baseCand.Solution.MemoSize,
		Board:           baseCand.Board.Copy(),
		Level:           baseCand.Level,
		Solution:        baseCand.Solution,
		SelectionBand:   baseCand.SelectionBand,
		DependencyDepth: baseCand.BoardUtil.OccupiedRows, // placeholder; enrichment uses metrics rebuild
	}
	ecfg := DefaultInventoryDiversityConfig(".")
	ecfg.MaxPlacements1 = 8
	ecfg.MaxPlacements2 = 4
	ecfg.MaxPlacements3 = 3
	ecfg.PreferOffCorridor = true
	ecfg.MaxCorridor1x1 = 1
	ecfg.SolveTimeLimit = budget.TimeLimit
	ecfg.MaxVisitedStates = budget.MaxVisited
	batch := &EnrichmentBatchResult{Rejected: map[string]int{}}
	out := []BoardMixAccepted{}
	for _, n := range []int{1, 2, 3} {
		beforeExact := batch.ExactSolves
		beforeNec := batch.NecessitySolves
		best, rej := enrichOneBaseCount(eb, n, ecfg, budget, batch)
		res.Stats.ExactSolves += (batch.ExactSolves - beforeExact) + (batch.NecessitySolves - beforeNec)
		for k, v := range rej {
			res.Rejected["Enrich:"+k] += v
		}
		if best == nil {
			continue
		}
		util := ComputeBoardUtilization(best.Board, baseCand.OffsetX, baseCand.OffsetY, &best.Solution)
		inv := BuildCargoInventorySignature(best.Board)
		level := best.Level
		if level != nil {
			level.BoardSpace = util.ToJSON(baseCand.Embedding)
			if level.Transplant == nil && baseCand.Level != nil {
				level.Transplant = baseCand.Level.Transplant
			}
		}
		out = append(out, BoardMixAccepted{
			BaseCandidateID: baseCand.BaseCandidateID,
			FamilyID:        baseCand.FamilyID,
			SourcePuzzleID:  baseCand.SourcePuzzleID,
			Embedding:       baseCand.Embedding,
			OffsetX:         baseCand.OffsetX,
			OffsetY:         baseCand.OffsetY,
			InventoryClass:  inv.InventoryClass,
			BoardUtil:       util,
			OptimalGestures: best.EnrichedOptimal,
			SelectionBand:   baseCand.SelectionBand,
			Augmented:       baseCand.Augmented,
			ReplayVerified:  best.ReplayVerified,
			Level:           level,
			Board:           best.Board,
			Solution:        best.Solution,
			ASCIIPreview:    best.ASCIIPreview,
		})
	}
	// Merge enrich reject counters from batch
	for k, v := range batch.Rejected {
		res.Rejected["Enrich:"+k] += v
	}
	return out
}

func evaluateBoardMixJob(base EnrichmentBase, embed EmbeddingVariant, augment bool, budget SolveBudget, cache *SolveCache, dbHash string) (*BoardMixAccepted, string, bool) {
	// Rebuild Rush record from transplant metadata when possible.
	if base.Level == nil || base.Level.Transplant == nil {
		return nil, "MissingTransplant", false
	}
	trMeta := base.Level.Transplant
	rec := RushDBRecord{
		Board36:              trMeta.OriginalBoard,
		OriginalOptimalMoves: trMeta.OriginalOptimalMoves,
		OriginalClusterSize:  trMeta.OriginalClusterSize,
		SourcePuzzleID:       trMeta.SourcePuzzleId,
		LineNumber:           trMeta.SourceLine,
	}
	if rec.Board36 == "" {
		return nil, "MissingSourceBoard", false
	}

	cacheKey := SolveCacheKey{
		SourceDatabaseHash: dbHash,
		SourcePuzzleID:     firstNonEmpty(trMeta.SourcePuzzleId, base.SourcePuzzleID),
		TransformVersion:   TransformVersionForEmbedding(embed),
		EmbeddingVariant:   string(embed),
		CargoRulesVersion:  CargoRulesVersionCF,
		SolverVersion:      SolverVersionBFS,
	}
	cached := false
	var board *Board
	var sol Solution
	var offX, offY int

	if cache != nil && dbHash != "" {
		if e, ok := cache.Get(cacheKey); ok && e.Valid && e.Solved && !augment {
			cached = true
			tr, err := TransformRushRecordToCargoFlow(rec, embed)
			if err != nil {
				return nil, "TransformFailed", cached
			}
			board = tr.Board
			offX, offY = tr.OffsetX, tr.OffsetY
			sol = Solution{Solvable: true, NumMoves: e.OptimalGestures, MemoSize: e.VisitedStates, Moves: e.Moves}
			if len(sol.Moves) == 0 {
				sol = board.SolveWithBudget(budget)
				cached = false
			}
		}
	}

	if board == nil {
		tr, err := TransformRushRecordToCargoFlow(rec, embed)
		if err != nil {
			return nil, "TransformFailed", false
		}
		board = tr.Board
		offX, offY = tr.OffsetX, tr.OffsetY
		if board.cargoTargetCanExit() {
			return nil, "ImmediateVictory", false
		}
		sol = board.SolveWithBudget(budget)
		if cache != nil && dbHash != "" && !augment {
			_ = cache.Put(SolveCacheEntry{
				Key: cacheKey, Valid: true, Solved: sol.Solvable,
				TimedOut: sol.TimedOut, BudgetExceeded: sol.BudgetExceeded,
				OptimalGestures: sol.NumMoves, VisitedStates: sol.MemoSize,
				OffsetX: offX, OffsetY: offY, Moves: sol.Moves,
				ReplayVerified: false,
			})
		}
	}

	if sol.TimedOut || sol.BudgetExceeded {
		return nil, "DifficultyUnknown", cached
	}
	if !sol.Solvable {
		return nil, "Unsolvable", cached
	}
	b2 := board.Copy()
	if err := b2.Replay(sol.Moves); err != nil {
		return nil, "ReplayFailed", cached
	}

	aug := false
	if augment {
		nb, nsol, ok := TryAddOuterStructuralPiece(board, offX, offY, budget)
		if !ok {
			return nil, "AugmentFailed", cached
		}
		board, sol, aug = nb, nsol, true
		b3 := board.Copy()
		if err := b3.Replay(sol.Moves); err != nil {
			return nil, "ReplayFailed", cached
		}
	}

	util := ComputeBoardUtilization(board, offX, offY, &sol)
	inv := BuildCargoInventorySignature(board)
	level, err := LevelJSONFromBoard(board, "pending", "RushDatabaseBoardMix")
	if err != nil {
		return nil, "ExportFailed", cached
	}
	level.Transplant = base.Level.Transplant
	if level.Transplant != nil {
		cp := *level.Transplant
		cp.EmbeddingVariant = string(embed)
		cp.OffsetX = offX
		cp.OffsetY = offY
		cp.TransformRotation = TransformRotationCW90
		level.Transplant = &cp
	}
	level.BoardSpace = util.ToJSON(embed)

	return &BoardMixAccepted{
		BaseCandidateID: base.CandidateID,
		FamilyID:        base.FamilyID,
		SourcePuzzleID:  firstNonEmpty(base.SourcePuzzleID, trMeta.SourcePuzzleId),
		Embedding:       embed,
		OffsetX:         offX,
		OffsetY:         offY,
		InventoryClass:  inv.InventoryClass,
		BoardUtil:       util,
		OptimalGestures: sol.NumMoves,
		SelectionBand:   base.SelectionBand,
		Augmented:       aug,
		ReplayVerified:  true,
		Level:           level,
		Board:           board,
		Solution:        sol,
		ASCIIPreview:    asciiCargoBoard(board),
	}, "", cached
}

func finalizeBoardMixCandidate(c *BoardMixAccepted, id string) {
	c.CandidateID = id
	if c.Level != nil {
		c.Level.LevelID = id
		opt := c.OptimalGestures
		c.Level.CanonicalGestures = &opt
		if c.Level.Enrichment == nil {
			c.Level.Enrichment = &EnrichmentJSON{
				BaseCandidateId:    c.BaseCandidateID,
				BaseFamilyId:       c.FamilyID,
				BaseSourcePuzzleId: c.SourcePuzzleID,
				InventoryClass:     string(c.InventoryClass),
			}
		}
		c.Level.Enrichment.InventoryClass = string(c.InventoryClass)
		if c.Level.BoardSpace != nil {
			c.Level.BoardSpace.BoardShapeClass = string(c.BoardUtil.BoardShapeClass)
		}
		c.SolutionDoc = ExportSolutionJSON(c.Level, c.Board, c.Solution, 0, c.ReplayVerified)
	}
}

// WriteBoardMixBatch writes Unity-compatible outputs + reports.
func WriteBoardMixBatch(dir string, res BoardMixResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candDir := filepath.Join(dir, "Candidates")
	solDir := filepath.Join(dir, "Solutions")
	_ = os.MkdirAll(candDir, 0o755)
	_ = os.MkdirAll(solDir, 0o755)
	for _, c := range res.Accepted {
		if c.Level == nil {
			continue
		}
		if err := WriteJSONFile(filepath.Join(candDir, c.CandidateID+".json"), c.Level); err != nil {
			return err
		}
		if err := WriteJSONFile(filepath.Join(solDir, c.CandidateID+".solution.json"), c.SolutionDoc); err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(candDir, c.CandidateID+".ascii.txt"), []byte(c.ASCIIPreview), 0o644)
	}
	report := map[string]interface{}{
		"generatorVersion": BoardMixVersion,
		"pipeline":         "CargoFlowBoardSpaceDiversity",
		"rootCauseRUSH01031": "Previous pilot accepted first N valids (underTarget) with inventoryQuotas={No1x1:12} and no pool→select stage; generation produced only clean No1x1 transforms.",
		"performance":      res.Stats,
		"poolDistribution": map[string]interface{}{
			"size":               res.SelectReport.PoolSize,
			"boardShapeClass":    res.SelectReport.PoolShapeDist,
			"inventoryClass":     res.SelectReport.PoolInventoryDist,
			"outerZoneRelevant":  res.SelectReport.PoolOuterZoneRelevant,
		},
		"finalDistribution": map[string]interface{}{
			"boardShapeClass":   res.SelectReport.FinalShapeDist,
			"inventoryClass":    res.SelectReport.FinalInventoryDist,
			"outerZoneRelevant": res.SelectReport.FinalOuterZoneRelevant,
			"distinctShapes":    res.SelectReport.DistinctBoardShapes,
		},
		"selectReport":          res.SelectReport,
		"shapeDistribution":     res.ShapeDist,
		"inventoryDistribution": res.InvDist,
		"rejected":              res.Rejected,
		"accepted":              res.Accepted,
		"totalWallMs":           res.TotalWall.Milliseconds(),
		"diversityTargetUnmet":  res.SelectReport.DiversityTargetUnmet,
		"whyLooks6x6":           "Rush Hour sources are 6x6; classic CW90+FlushTop/ShiftDown1 embeddings place that block inside 7x8, leaving outer rows/columns empty unless shifted/augmented.",
		"limitations": []string{
			"BoardShapeClass is not a difficulty label.",
			"Compact6x6 remains valid; full 7x8 is not mandatory.",
			"InventoryClass is independent of BoardShapeClass.",
		},
	}
	if err := WriteJSONFile(filepath.Join(dir, "BoardDiversityReport.json"), report); err != nil {
		return err
	}
	return WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), map[string]interface{}{
		"generatorVersion": BoardMixVersion,
		"candidateCount":   len(res.Accepted),
		"candidates":       res.Accepted,
	})
}

func SaveBoardMixCheckpoint(path string, cp BoardMixCheckpoint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cp.Version = BoardMixVersion
	return WriteJSONFile(path, cp)
}

func LoadBoardMixCheckpoint(path string) (BoardMixCheckpoint, error) {
	var cp BoardMixCheckpoint
	data, err := os.ReadFile(path)
	if err != nil {
		return cp, err
	}
	err = json.Unmarshal(data, &cp)
	return cp, err
}

func writeBoardMixProgress(dir string, cp BoardMixCheckpoint, totalJobs int) error {
	return WriteJSONFile(filepath.Join(dir, "progress.json"), map[string]interface{}{
		"attempts":     cp.Stats.Attempts,
		"totalJobs":    totalJobs,
		"accepted":     cp.Stats.Accepted,
		"exactSolves":  cp.Stats.ExactSolves,
		"cacheHits":    cp.Stats.CacheHits,
		"rejected":     cp.Rejected,
		"elapsedMs":    cp.Stats.ElapsedMs,
		"updatedAt":    cp.UpdatedAt,
	})
}

func parseEmbedList(names []string) []EmbeddingVariant {
	out := []EmbeddingVariant{}
	for _, n := range names {
		out = append(out, EmbeddingVariant(n))
	}
	return out
}

func copyQuotaMap(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func decQuota(m map[string]int, k string) {
	if m[k] > 0 {
		m[k]--
	}
}

func sumQuota(m map[string]int) int {
	s := 0
	for _, v := range m {
		s += v
	}
	return s
}

func sumIntMap(m map[string]int) int {
	s := 0
	for _, v := range m {
		s += v
	}
	return s
}

func copyIntMap(m map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func quotasSatisfied(shape, inv map[string]int, target, have int) bool {
	return have >= target && sumQuota(shape) == 0 && sumQuota(inv) == 0
}

func familyTaken(accepted []BoardMixAccepted, fam string) bool {
	for _, a := range accepted {
		if a.FamilyID == fam {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
