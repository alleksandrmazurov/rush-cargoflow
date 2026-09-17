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

const BoardMixVersion = "boardmix-v1.4.2"

// BoardMixConfig drives RUSH-010.3 / 010.4 / 010.4.1 board-space diversity generation.
type BoardMixConfig struct {
	BatchDir                   string         `json:"batchDir"`
	OutputDir                  string         `json:"outputDir"`
	CachePath                  string         `json:"cachePath"`
	CheckpointPath             string         `json:"checkpointPath"`
	DatabasePath               string         `json:"databasePath"`
	TargetAccepted             int            `json:"targetAccepted"`
	BaseCount                  int            `json:"baseCount"`
	Seed                       int64          `json:"seed"`
	Workers                    int            `json:"workers"`
	CheckpointEvery            int            `json:"checkpointEvery"`
	Resume                     bool           `json:"resume"`
	SolveTimeLimitMs           int            `json:"solveTimeLimitMs"`
	MaxVisitedStates           int            `json:"maxVisitedStates"`
	TryOuterAugment            bool           `json:"tryOuterAugment"`
	TryNativeAugment           bool           `json:"tryNativeAugment"`
	TryInventoryEnrichment     bool           `json:"tryInventoryEnrichment"`
	MaxNativeAcceptedPerEmbed  int            `json:"maxNativeAcceptedPerEmbed"`
	MaxNativeProposalsPerEmbed int            `json:"maxNativeProposalsPerEmbed"`
	FamilyFirstExploration     bool           `json:"familyFirstExploration"`
	PerFamilyPoolCap           int            `json:"perFamilyPoolCap"`
	MinUniqueFamiliesInPool    int            `json:"minUniqueFamiliesInPool"`
	MinDistinctInventoryClasses int           `json:"minDistinctInventoryClasses"`
	Embeddings                 []string       `json:"embeddings"`
	ShapeQuotas                map[string]int `json:"shapeQuotas"`
	InventoryQuotas            map[string]int `json:"inventoryQuotas"`
	ProgressEvery              int            `json:"progressEvery"`
	MaxPoolSize                int            `json:"maxPoolSize"`
	MaxAttempts                int            `json:"maxAttempts"`
	MinDistinctBoardShapes     int            `json:"minDistinctBoardShapes"`
	MaxBoardShapeFraction      float64        `json:"maxBoardShapeFraction"`
	MinOuterZoneRelevant       int            `json:"minOuterZoneRelevant"`
	MaxInventoryClassFraction  float64        `json:"maxInventoryClassFraction"`
	UniqueFamily               bool           `json:"uniqueFamily"`
}

// DefaultBoardMixConfig returns sane long-run defaults (manual pilot).
func DefaultBoardMixConfig() BoardMixConfig {
	return BoardMixConfig{
		BatchDir:               "output/RUSH009_CuratedShortlist_001",
		OutputDir:              "output/RUSH01031_DiversityQuotaPilot_001",
		CachePath:              "data/cache/curator/solve_cache.jsonl",
		DatabasePath:           "data/external/rush/rush.txt",
		TargetAccepted:         12,
		BaseCount:              32,
		Seed:                   20260916,
		Workers:                2,
		CheckpointEvery:        1,
		Resume:                 true,
		SolveTimeLimitMs:       8000,
		MaxVisitedStates:       2_000_000,
		TryOuterAugment:            true,
		TryNativeAugment:           true,
		TryInventoryEnrichment:     true,
		MaxNativeAcceptedPerEmbed:  2,
		MaxNativeProposalsPerEmbed: 10,
		FamilyFirstExploration:     true,
		PerFamilyPoolCap:           4,
		MinUniqueFamiliesInPool:    12,
		MinDistinctInventoryClasses: 3,
		Embeddings: []string{
			string(EmbedFlushTop), string(EmbedShiftDown1), string(EmbedFlushBottom),
			string(EmbedFlushTopMirrorH), string(EmbedShiftDown1MirrorH),
		},
		ShapeQuotas: map[string]int{
			string(ShapeShiftedCore): 3,
			string(ShapeExpanded):    2,
			string(ShapeTall):        2,
			string(ShapeWide):        2,
			string(ShapeFullField):   2,
		},
		InventoryQuotas: map[string]int{
			string(InvNo1x1): 3, string(InvOne1x1): 3,
			string(InvTwo1x1): 3, // Three1x1 is soft/rare — not a hard pilot quota
		},
		ProgressEvery:             1,
		MaxPoolSize:               80,
		MaxAttempts:               200,
		MinDistinctBoardShapes:    3,
		MaxBoardShapeFraction:     0.40,
		MinOuterZoneRelevant:      8,
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
	if cfg.BaseCount <= 0 {
		cfg.BaseCount = 24
	}
	if cfg.MaxNativeAcceptedPerEmbed <= 0 {
		cfg.MaxNativeAcceptedPerEmbed = 2
	}
	if cfg.MaxNativeProposalsPerEmbed <= 0 {
		cfg.MaxNativeProposalsPerEmbed = 10
	}
	if cfg.PerFamilyPoolCap <= 0 {
		cfg.PerFamilyPoolCap = 4
	}
	if cfg.MinUniqueFamiliesInPool <= 0 {
		cfg.MinUniqueFamiliesInPool = 12
	}
	if cfg.MinDistinctInventoryClasses <= 0 {
		cfg.MinDistinctInventoryClasses = 3
	}
	if cfg.TryNativeAugment {
		cfg.FamilyFirstExploration = true
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
	Attempts                  int            `json:"attempts"`
	ExactSolves               int            `json:"exactSolves"`
	RestrictedSolves          int            `json:"restrictedSolves"`
	CacheHits                 int            `json:"cacheHits"`
	Accepted                  int            `json:"accepted"`
	BaseFamiliesTried         int            `json:"baseFamiliesTried"`
	EmbeddingsTried           int            `json:"embeddingsTried"`
	AugmentationsProposed     int            `json:"augmentationsProposed"`
	AcceptedByShape           map[string]int `json:"acceptedByShape,omitempty"`
	AcceptedByAugmentationClass map[string]int `json:"acceptedByAugmentationClass,omitempty"`
	ElapsedMs                 int64          `json:"elapsedMs"`
	LastProgressAt            time.Time      `json:"lastProgressAt"`
}

// BoardMixAccepted is one validated board-diversity candidate.
type BoardMixAccepted struct {
	CandidateID              string                  `json:"candidateId"`
	BaseCandidateID          string                  `json:"baseCandidateId"`
	FamilyID                 string                  `json:"familyId"` // BaseFamilyId
	SourcePuzzleID           string                  `json:"sourcePuzzleId"`
	Embedding                EmbeddingVariant        `json:"embedding"`
	OffsetX                  int                     `json:"offsetX"`
	OffsetY                  int                     `json:"offsetY"`
	InventoryClass           InventoryClass          `json:"inventoryClass"`
	BoardUtil                BoardUtilizationMetrics `json:"boardUtilization"`
	OptimalGestures          int                     `json:"optimalGestures"`
	SelectionBand            string                  `json:"selectionBand"`
	Augmented                bool                    `json:"augmented"`
	AugmentationClass        AugmentationClass       `json:"augmentationClass,omitempty"`
	NativeMeta               *NativeAugmentMeta      `json:"nativeAugment,omitempty"`
	NativeVariantFingerprint string                  `json:"nativeVariantFingerprint,omitempty"`
	ReplayVerified           bool                    `json:"replayVerified"`
	Level                    *LevelJSON              `json:"level,omitempty"`
	Board                    *Board                  `json:"-"`
	Solution                 Solution                `json:"-"`
	SolutionDoc              SolutionJSON            `json:"-"`
	ASCIIPreview             string                  `json:"-"`
}

// BoardMixResult is the final run summary.
type BoardMixResult struct {
	Config         BoardMixConfig
	Accepted       []BoardMixAccepted
	Pool           []BoardMixAccepted
	Rejected       map[string]int
	Stats          BoardMixStats
	ShapeDist      map[string]int
	InvDist        map[string]int
	SelectReport   BoardMixSelectReport
	FamilyCoverage FamilyCoverageReport
	TotalWall      time.Duration
	RootCauseNote  string
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
		RootCauseNote: "RUSH-010.4.1: family-first native coverage + pool→diversity-select.",
	}
	out.Stats.AcceptedByShape = map[string]int{}
	out.Stats.AcceptedByAugmentationClass = map[string]int{}
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

	type job = boardMixJob
	jobs := buildBoardMixJobs(bases, embeds, cfg)
	famTried := map[string]bool{}
	for _, base := range bases {
		famTried[base.FamilyID] = true
	}
	out.Stats.BaseFamiliesTried = len(famTried)
	out.Stats.EmbeddingsTried = len(embeds)
	funnelMap := initFamilyFunnel(bases)
	requestedFamilies := len(famTried)
	coveragePassPending := 0
	for _, j := range jobs {
		if j.pass == 1 {
			coveragePassPending++
		}
	}
	earlyStopFlag := false

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
				if stop || int(attempts) >= cfg.MaxAttempts {
					stop = true
					earlyStopFlag = true
					mu.Unlock()
					return
				}
				// Soft pool full: stop only if family coverage target met.
				if len(pool) >= cfg.MaxPoolSize {
					u := uniqueFamilyCount(pool)
					minU := cfg.MinUniqueFamiliesInPool
					if minU <= 0 {
						minU = 12
					}
					if u >= minU || u >= requestedFamilies || coveragePassPending <= 0 {
						stop = true
						earlyStopFlag = u < requestedFamilies
						mu.Unlock()
						return
					}
					// Pool full but coverage incomplete — skip deepen jobs; still allow pass-1.
					if j.pass > 1 {
						mu.Unlock()
						continue
					}
				}
				mu.Unlock()

				augTag := j.mode
				key := boardMixAttemptKey{BaseID: j.base.CandidateID, Embedding: string(j.embed), Augment: augTag}.String()
				mu.Lock()
				if completed[key] {
					mu.Unlock()
					continue
				}
				mu.Unlock()

				var cands []*BoardMixAccepted
				var reason string
				var cached bool
				augProposed := 0
				switch j.mode {
				case "native":
					cands, reason, cached = evaluateNativeBoardMixJob(j.base, j.embed, budget, cache, dbHash, cfg)
					augProposed = len(cands)
					mu.Lock()
					out.Stats.AugmentationsProposed += len(cands)
					mu.Unlock()
				default:
					cand, r, c := evaluateBoardMixJob(j.base, j.embed, j.mode == "outer1", budget, cache, dbHash)
					reason, cached = r, c
					if cand != nil {
						if j.mode == "outer1" {
							cand.AugmentationClass = AugLegacyOuter1x1
						} else {
							cand.AugmentationClass = AugNone
						}
						// Metadata invariant: FamilyID must match source base.
						cand.FamilyID = j.base.FamilyID
						cand.BaseCandidateID = j.base.CandidateID
						cands = []*BoardMixAccepted{cand}
					}
				}
				// Ensure native candidates keep source family.
				for _, c := range cands {
					if c != nil {
						c.FamilyID = j.base.FamilyID
						c.BaseCandidateID = j.base.CandidateID
					}
				}
				n := int(atomic.AddInt32(&attempts, 1))
				mu.Lock()
				if j.pass == 1 {
					coveragePassPending--
				}
				completed[key] = true
				cp.CompletedAttemptKeys = append(cp.CompletedAttemptKeys, key)
				out.Stats.Attempts++
				exact := !cached
				if cached {
					out.Stats.CacheHits++
				} else {
					out.Stats.ExactSolves++
				}
				if reason != "" && len(cands) == 0 {
					out.Rejected[reason]++
				}

				acceptedNow := 0
				filtered := []*BoardMixAccepted{}
				for _, c := range cands {
					if c == nil {
						continue
					}
					if !shouldAcceptFamilyVariant(pool, c.FamilyID, requestedFamilies, cfg.PerFamilyPoolCap) {
						out.Rejected["PerFamilyCap"]++
						noteFamilyAttempt(funnelMap, j.base.FamilyID, "PerFamilyCap", exact, 0, 0)
						continue
					}
					filtered = append(filtered, c)
				}
				cands = filtered

				doEnrich := cfg.TryInventoryEnrichment
				plainList := append([]*BoardMixAccepted{}, cands...)
				mu.Unlock()

				var enriched []BoardMixAccepted
				if doEnrich {
					for _, plain := range plainList {
						if plain == nil || plain.Board == nil {
							continue
						}
						mu.Lock()
						allowEnrich := shouldAcceptFamilyVariant(pool, plain.FamilyID, requestedFamilies, cfg.PerFamilyPoolCap)
						mu.Unlock()
						if !allowEnrich {
							continue
						}
						tmp := BoardMixResult{Rejected: map[string]int{}, Stats: BoardMixStats{}}
						got := enrichBoardMixInventory(*plain, budget, &tmp)
						mu.Lock()
						for k, v := range tmp.Rejected {
							out.Rejected["Enrich:"+k] += v
						}
						out.Stats.ExactSolves += tmp.Stats.ExactSolves
						mu.Unlock()
						enriched = append(enriched, got...)
					}
				}

				mu.Lock()
				for _, plain := range plainList {
					if plain == nil {
						continue
					}
					if !shouldAcceptFamilyVariant(pool, plain.FamilyID, requestedFamilies, cfg.PerFamilyPoolCap) {
						out.Rejected["PerFamilyCap"]++
						continue
					}
					pool = append(pool, *plain)
					acceptedNow++
				}
				for _, e := range enriched {
					e.FamilyID = j.base.FamilyID
					e.BaseCandidateID = j.base.CandidateID
					if !shouldAcceptFamilyVariant(pool, e.FamilyID, requestedFamilies, cfg.PerFamilyPoolCap) {
						out.Rejected["PerFamilyCap"]++
						continue
					}
					pool = append(pool, e)
					acceptedNow++
				}
				noteFamilyAttempt(funnelMap, j.base.FamilyID, reason, exact, augProposed, acceptedNow)
				cp.Pool = append([]BoardMixAccepted{}, pool...)
				cp.Rejected = copyIntMap(out.Rejected)
				cp.Stats = out.Stats
				cp.Stats.ElapsedMs = time.Since(start).Milliseconds()
				cp.Stats.LastProgressAt = time.Now()
				cp.UpdatedAt = time.Now()
				if cfg.CheckpointEvery > 0 && n%cfg.CheckpointEvery == 0 {
					_ = SaveBoardMixCheckpoint(cfg.CheckpointPath, cp)
					_ = writeBoardMixProgress(cfg.OutputDir, cp, len(jobs))
				}
				if cfg.ProgressEvery > 0 && n%cfg.ProgressEvery == 0 {
					fmt.Printf("[%d/%d] Pool: %d families=%d Solved: %d Cached: %d Rejected: %d Elapsed: %s\n",
						n, len(jobs), len(pool), uniqueFamilyCount(pool), out.Stats.ExactSolves, out.Stats.CacheHits,
						sumIntMap(out.Rejected), time.Since(start).Round(time.Second))
				}
				if int(attempts) >= cfg.MaxAttempts {
					stop = true
					earlyStopFlag = true
				}
				if len(pool) >= cfg.MaxPoolSize {
					u := uniqueFamilyCount(pool)
					minU := cfg.MinUniqueFamiliesInPool
					if minU <= 0 {
						minU = 12
					}
					if u >= minU || u >= requestedFamilies {
						stop = true
						earlyStopFlag = u < requestedFamilies
					}
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	funnelList := finalizeFamilyFunnel(funnelMap, pool)
	out.FamilyCoverage = buildFamilyCoverageReport(cfg, bases, pool, funnelList, earlyStopFlag)

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
	if out.Stats.AcceptedByShape == nil {
		out.Stats.AcceptedByShape = map[string]int{}
	}
	if out.Stats.AcceptedByAugmentationClass == nil {
		out.Stats.AcceptedByAugmentationClass = map[string]int{}
	}
	for _, a := range selected {
		out.Stats.AcceptedByShape[string(a.BoardUtil.BoardShapeClass)]++
		cls := string(a.AugmentationClass)
		if cls == "" {
			cls = string(AugNone)
		}
		out.Stats.AcceptedByAugmentationClass[cls]++
	}

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
			BaseCandidateID:   baseCand.BaseCandidateID,
			FamilyID:          baseCand.FamilyID,
			SourcePuzzleID:    baseCand.SourcePuzzleID,
			Embedding:         baseCand.Embedding,
			OffsetX:           baseCand.OffsetX,
			OffsetY:           baseCand.OffsetY,
			InventoryClass:    inv.InventoryClass,
			BoardUtil:         util,
			OptimalGestures:   best.EnrichedOptimal,
			SelectionBand:     baseCand.SelectionBand,
			Augmented:         baseCand.Augmented,
			AugmentationClass: baseCand.AugmentationClass,
			NativeMeta:        baseCand.NativeMeta,
			NativeVariantFingerprint: baseCand.NativeVariantFingerprint,
			ReplayVerified:    best.ReplayVerified,
			Level:             level,
			Board:             best.Board,
			Solution:          best.Solution,
			ASCIIPreview:      best.ASCIIPreview,
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
		BaseCandidateID:   base.CandidateID,
		FamilyID:          base.FamilyID,
		SourcePuzzleID:    firstNonEmpty(base.SourcePuzzleID, trMeta.SourcePuzzleId),
		Embedding:         embed,
		OffsetX:           offX,
		OffsetY:           offY,
		InventoryClass:    inv.InventoryClass,
		BoardUtil:         util,
		OptimalGestures:   sol.NumMoves,
		SelectionBand:     base.SelectionBand,
		Augmented:         aug,
		AugmentationClass: AugNone,
		ReplayVerified:    true,
		Level:             level,
		Board:             board,
		Solution:          sol,
		ASCIIPreview:      asciiCargoBoard(board),
	}, "", cached
}

// evaluateNativeBoardMixJob builds Rush core then applies bounded native structural templates.
func evaluateNativeBoardMixJob(base EnrichmentBase, embed EmbeddingVariant, budget SolveBudget, cache *SolveCache, dbHash string, cfg BoardMixConfig) ([]*BoardMixAccepted, string, bool) {
	plain, reason, cached := evaluateBoardMixJob(base, embed, false, budget, cache, dbHash)
	if plain == nil || plain.Board == nil {
		if reason == "" {
			reason = "NativeBaseFailed"
		}
		return nil, reason, cached
	}
	ncfg := DefaultNativeAugmentConfig()
	ncfg.MaxAcceptedPerEmbed = cfg.MaxNativeAcceptedPerEmbed
	ncfg.MaxProposalsPerEmbed = cfg.MaxNativeProposalsPerEmbed
	cands, rej := GenerateNativeAugmentCandidates(plain.Board, plain.OffsetX, plain.OffsetY, plain.OptimalGestures, budget, ncfg)
	if len(cands) == 0 {
		// Surface dominant reject reason.
		top, topN := "NativeAugmentFailed", 0
		for k, v := range rej {
			if v > topN {
				top, topN = k, v
			}
		}
		return nil, top, cached
	}
	out := []*BoardMixAccepted{}
	for _, c := range cands {
		util := ComputeBoardUtilization(c.Board, plain.OffsetX, plain.OffsetY, &c.Sol)
		essential, _ := outerZoneEssentiality(c.Board, plain.OffsetX, plain.OffsetY, c.Sol, budget)
		util = RefineBoardShapeForNative(util, plain.OffsetX, plain.OffsetY, essential)
		if (util.BoardShapeClass == ShapeExpanded || util.BoardShapeClass == ShapeFullField) && !util.OuterZoneRelevant {
			continue
		}
		inv := BuildCargoInventorySignature(c.Board)
		level, err := LevelJSONFromBoard(c.Board, "pending", "RushDatabaseNativeBoardMix")
		if err != nil {
			continue
		}
		level.Transplant = plain.Level.Transplant
		if level.Transplant != nil {
			cp := *level.Transplant
			cp.EmbeddingVariant = string(embed)
			cp.OffsetX = plain.OffsetX
			cp.OffsetY = plain.OffsetY
			cp.TransformRotation = TransformRotationCW90
			level.Transplant = &cp
		}
		level.BoardSpace = util.ToJSON(embed)
		meta := c.Meta
		out = append(out, &BoardMixAccepted{
			BaseCandidateID:          plain.BaseCandidateID,
			FamilyID:                 plain.FamilyID,
			SourcePuzzleID:           plain.SourcePuzzleID,
			Embedding:                embed,
			OffsetX:                  plain.OffsetX,
			OffsetY:                  plain.OffsetY,
			InventoryClass:           inv.InventoryClass,
			BoardUtil:                util,
			OptimalGestures:          c.Sol.NumMoves,
			SelectionBand:            plain.SelectionBand,
			Augmented:                true,
			AugmentationClass:        meta.AugmentationClass,
			NativeMeta:               &meta,
			NativeVariantFingerprint: meta.NativeVariantFingerprint,
			ReplayVerified:           true,
			Level:                    level,
			Board:                    c.Board,
			Solution:                 c.Sol,
			ASCIIPreview:             asciiCargoBoard(c.Board),
		})
	}
	if len(out) == 0 {
		return nil, "NativeAugmentFiltered", cached
	}
	return out, "", cached
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
		c.Level.Enrichment.BaseFamilyId = c.FamilyID
		c.Level.Enrichment.AugmentationClass = string(c.AugmentationClass)
		c.Level.Enrichment.NativeVariantFingerprint = c.NativeVariantFingerprint
		if c.NativeMeta != nil {
			c.Level.Enrichment.BaseOptimal = c.NativeMeta.BaseOptimal
			c.Level.Enrichment.EnrichedOptimal = c.NativeMeta.NativeOptimal
			c.Level.Enrichment.OptimalDelta = c.NativeMeta.OptimalDelta
			c.Level.Enrichment.Added1x1Count = c.NativeMeta.Added1x1Count
			c.Level.Enrichment.Added1x2Count = c.NativeMeta.Added1x2Count
			c.Level.Enrichment.Added1x3Count = c.NativeMeta.Added1x3Count
			c.Level.Enrichment.AddedStaticCount = c.NativeMeta.AddedStaticCount
			c.Level.Enrichment.OuterZoneEssential = c.NativeMeta.OuterZoneEssential
		}
		inv := BuildCargoInventorySignature(c.Board)
		c.Level.Enrichment.InventorySignature = inv.Signature
		if c.Level.BoardSpace != nil {
			c.Level.BoardSpace.BoardShapeClass = string(c.BoardUtil.BoardShapeClass)
		}
		c.SolutionDoc = ExportSolutionJSON(c.Level, c.Board, c.Solution, 0, c.ReplayVerified)
	}
}

// WriteBoardMixBatch is implemented in cargoflow_boardmix_export.go (Unity BatchManifest contract).

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
