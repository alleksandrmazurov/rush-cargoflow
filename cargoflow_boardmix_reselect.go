package rush

import (
	"fmt"
	"path/filepath"
	"time"
)

// ReselectReport summarizes a no-generation reselect from an existing checkpoint pool.
type ReselectReport struct {
	BatchDir              string               `json:"batchDir"`
	PoolSizeBefore        int                  `json:"poolSizeBefore"`
	PoolSizeAfter         int                  `json:"poolSizeAfter"`
	PoolPreserved         bool                 `json:"poolPreserved"`
	GenerationPerformed   bool                 `json:"generationPerformed"`
	SelectedCount         int                  `json:"selectedCount"`
	ExactSolvesSelected   int                  `json:"exactSolvesSelected"`
	SelectReport          BoardMixSelectReport `json:"selectReport"`
	Validation            BatchValidationReport `json:"validation"`
	OK                    bool                 `json:"ok"`
	Errors                []string             `json:"errors,omitempty"`
}

// ReselectBoardMixFromExisting reloads checkpoint.Pool, runs core-aware selection,
// materializes+solves ONLY the selected finalists, writes Unity batch, validates.
// Does not generate new candidates or mutate pool contents.
func ReselectBoardMixFromExisting(batchDir string, cfg BoardMixConfig) (ReselectReport, error) {
	rep := ReselectReport{
		BatchDir:            batchDir,
		GenerationPerformed: false,
	}
	cpPath := filepath.Join(batchDir, "checkpoint.json")
	cp, err := LoadBoardMixCheckpoint(cpPath)
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep, err
	}
	pool := append([]BoardMixAccepted{}, cp.Pool...)
	rep.PoolSizeBefore = len(pool)
	if len(pool) == 0 {
		err := fmt.Errorf("checkpoint pool empty: %s", cpPath)
		rep.Errors = append(rep.Errors, err.Error())
		return rep, err
	}

	cfg = normalizeBoardMixConfig(cfg)
	if cfg.OutputDir == "" {
		cfg.OutputDir = batchDir
	}
	if cfg.CheckpointPath == "" {
		cfg.CheckpointPath = cpPath
	}

	selected, selRep := SelectBoardMixShortlist(pool, cfg)
	rep.SelectReport = selRep
	rep.SelectedCount = len(selected)

	budget := DefaultCargoFlowSolveBudget()
	if cfg.SolveTimeLimitMs > 0 {
		budget.TimeLimit = time.Duration(cfg.SolveTimeLimitMs) * time.Millisecond
	}
	if cfg.MaxVisitedStates > 0 {
		budget.MaxVisited = cfg.MaxVisitedStates
	}

	exact := 0
	for i := range selected {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		selected[i].CandidateID = id
		selected[i].SolutionDoc = SolutionJSON{}
		selected[i].Solution = Solution{}
		selected[i].Board = nil // force rehydrate from Level
		if err := ensureAcceptedSolutionMaterialized(&selected[i], budget, ""); err != nil {
			rep.Errors = append(rep.Errors, err.Error())
			return rep, err
		}
		exact++
		finalizeBoardMixCandidate(&selected[i], id)
		if selected[i].Board == nil {
			err := fmt.Errorf("%s: board still nil after materialize", id)
			rep.Errors = append(rep.Errors, err.Error())
			return rep, err
		}
		if !selected[i].ReplayVerified {
			err := fmt.Errorf("%s: replay not verified", id)
			rep.Errors = append(rep.Errors, err.Error())
			return rep, err
		}
	}
	rep.ExactSolvesSelected = exact

	// Preserve original pool; update accepted + select report only.
	cp.Pool = pool
	cp.Accepted = append([]BoardMixAccepted{}, selected...)
	cp.AcceptedIDs = nil
	for _, a := range selected {
		cp.AcceptedIDs = append(cp.AcceptedIDs, a.CandidateID)
	}
	cp.SelectReport = selRep
	cp.Stats.Accepted = len(selected)
	cp.UpdatedAt = time.Now()
	cp.Version = BoardMixVersion
	if err := SaveBoardMixCheckpoint(cpPath, cp); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep, err
	}
	rep.PoolSizeAfter = len(cp.Pool)
	rep.PoolPreserved = rep.PoolSizeAfter == rep.PoolSizeBefore

	res := BoardMixResult{
		Config:       cfg,
		Accepted:     selected,
		Pool:         pool,
		Rejected:     cp.Rejected,
		Stats:        cp.Stats,
		ShapeDist:    selRep.FinalShapeDist,
		InvDist:      selRep.FinalInventoryDist,
		SelectReport: selRep,
	}
	if err := WriteBoardMixBatch(batchDir, res); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		return rep, err
	}
	val, verr := ValidateUnityBatchReport(batchDir)
	rep.Validation = val
	if verr != nil {
		rep.Errors = append(rep.Errors, verr.Error())
		return rep, verr
	}
	if !val.OK {
		err := fmt.Errorf("ValidateUnityBatch failed: %v", val.Errors)
		rep.Errors = append(rep.Errors, err.Error())
		return rep, err
	}
	rep.OK = true
	return rep, nil
}

func normalizeBoardMixConfig(cfg BoardMixConfig) BoardMixConfig {
	if cfg.TargetAccepted <= 0 {
		cfg.TargetAccepted = 12
	}
	if cfg.SolveTimeLimitMs <= 0 {
		cfg.SolveTimeLimitMs = 8000
	}
	if cfg.MaxVisitedStates <= 0 {
		cfg.MaxVisitedStates = 2_000_000
	}
	if cfg.CoreSpaceFitThreshold <= 0 {
		cfg.CoreSpaceFitThreshold = Default6x6FitThreshold
	}
	return cfg
}
