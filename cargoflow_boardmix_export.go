package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BoardMixUnityManifestEntry is the Unity Import Rush Batch contract entry.
// Matches RUSH-009 curator / RUSH-010.2 enrichment BatchManifest shape.
type BoardMixUnityManifestEntry struct {
	CandidateID              string `json:"candidateId"`
	BaseCandidateID          string `json:"baseCandidateId,omitempty"`
	BaseFamilyID             string `json:"baseFamilyId,omitempty"`
	FamilyID                 string `json:"familyId,omitempty"`
	SourcePuzzleID           string `json:"sourcePuzzleId,omitempty"`
	OptimalGestures          int    `json:"optimalGestures,omitempty"`
	InventoryClass           string `json:"inventoryClass,omitempty"`
	BoardShapeClass          string `json:"boardShapeClass,omitempty"`
	AugmentationClass        string `json:"augmentationClass,omitempty"`
	OuterZoneRelevant        bool   `json:"outerZoneRelevant,omitempty"`
	NativeVariantFingerprint string `json:"nativeVariantFingerprint,omitempty"`
	ReplayVerified           bool   `json:"replayVerified"`
	LevelFile                string `json:"levelFile"`
	SolutionFile             string `json:"solutionFile"`
}

// BoardMixUnityManifest is the top-level BatchManifest.json for Unity.
type BoardMixUnityManifest struct {
	GeneratorVersion string                       `json:"generatorVersion"`
	Pipeline         string                       `json:"pipeline"`
	Accepted         int                          `json:"accepted"`
	CandidateCount   int                          `json:"candidateCount"`
	Candidates       []BoardMixUnityManifestEntry `json:"candidates"`
}

// ToUnityBatchManifest builds the thin Unity-compatible manifest (with levelFile paths).
func ToUnityBatchManifest(accepted []BoardMixAccepted) BoardMixUnityManifest {
	entries := make([]BoardMixUnityManifestEntry, 0, len(accepted))
	for _, c := range accepted {
		id := c.CandidateID
		if id == "" && c.Level != nil {
			id = c.Level.LevelID
		}
		entries = append(entries, BoardMixUnityManifestEntry{
			CandidateID:              id,
			BaseCandidateID:          c.BaseCandidateID,
			BaseFamilyID:             c.FamilyID,
			FamilyID:                 c.FamilyID,
			SourcePuzzleID:           c.SourcePuzzleID,
			OptimalGestures:          c.OptimalGestures,
			InventoryClass:           string(c.InventoryClass),
			BoardShapeClass:          string(c.BoardUtil.BoardShapeClass),
			AugmentationClass:        string(c.AugmentationClass),
			OuterZoneRelevant:        c.BoardUtil.OuterZoneRelevant,
			NativeVariantFingerprint: c.NativeVariantFingerprint,
			ReplayVerified:           c.ReplayVerified,
			LevelFile:                "Candidates/" + id + ".json",
			SolutionFile:             "Solutions/" + id + ".solution.json",
		})
	}
	return BoardMixUnityManifest{
		GeneratorVersion: BoardMixVersion,
		Pipeline:         "CargoFlowNative7x8BoardMix",
		Accepted:         len(entries),
		CandidateCount:   len(entries),
		Candidates:       entries,
	}
}

func ensureAcceptedLevelMaterialized(c *BoardMixAccepted) error {
	if c.CandidateID == "" {
		return fmt.Errorf("accepted candidate missing CandidateID")
	}
	if c.Level != nil {
		c.Level.LevelID = c.CandidateID
		return nil
	}
	if c.Board == nil {
		return fmt.Errorf("%s: missing Level JSON and Board — cannot export", c.CandidateID)
	}
	level, err := LevelJSONFromBoard(c.Board, c.CandidateID, "RushDatabaseNativeBoardMix")
	if err != nil {
		return fmt.Errorf("%s: LevelJSONFromBoard: %w", c.CandidateID, err)
	}
	c.Level = level
	finalizeBoardMixCandidate(c, c.CandidateID)
	return nil
}

// solutionDocIsValid reports whether SolutionDoc meets the Unity solution contract.
func solutionDocIsValid(doc SolutionJSON, candidateID string) bool {
	if doc.SchemaVersion != CargoFlowJSONSchemaVersion {
		return false
	}
	if candidateID != "" && doc.LevelID != candidateID {
		return false
	}
	if !doc.Solved || !doc.Optimal {
		return false
	}
	if doc.OptimalGestures <= 0 {
		return false
	}
	if len(doc.Moves) == 0 {
		return false
	}
	if !doc.ReplayPass || !doc.ReplayVerified {
		return false
	}
	if doc.TimedOut || doc.BudgetExceeded {
		return false
	}
	return true
}

// ensureAcceptedSolutionMaterialized guarantees a full Unity SolutionJSON (never zero-value).
// Prefers an already-valid in-memory or on-disk solution; otherwise exact-solves the Level.
func ensureAcceptedSolutionMaterialized(c *BoardMixAccepted, budget SolveBudget, existingSolPath string) error {
	if err := ensureAcceptedLevelMaterialized(c); err != nil {
		return err
	}
	if solutionDocIsValid(c.SolutionDoc, c.CandidateID) {
		return nil
	}
	if existingSolPath != "" {
		if data, err := os.ReadFile(existingSolPath); err == nil {
			var doc SolutionJSON
			if json.Unmarshal(data, &doc) == nil && solutionDocIsValid(doc, c.CandidateID) {
				c.SolutionDoc = doc
				c.ReplayVerified = true
				if c.OptimalGestures <= 0 {
					c.OptimalGestures = doc.OptimalGestures
				}
				return nil
			}
		}
	}
	if c.Board == nil {
		b, err := BoardFromLevelJSON(c.Level)
		if err != nil {
			return fmt.Errorf("%s: BoardFromLevelJSON: %w", c.CandidateID, err)
		}
		c.Board = b
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	start := time.Now()
	sol := c.Board.SolveWithBudget(budget)
	elapsed := time.Since(start)
	if sol.TimedOut || sol.BudgetExceeded {
		return fmt.Errorf("%s: DifficultyUnknown (timedOut=%v budgetExceeded=%v)", c.CandidateID, sol.TimedOut, sol.BudgetExceeded)
	}
	if !sol.Solvable {
		return fmt.Errorf("%s: Unsolvable during solution materialize", c.CandidateID)
	}
	work := c.Board.Copy()
	if err := work.Replay(sol.Moves); err != nil {
		return fmt.Errorf("%s: ReplayFailed: %w", c.CandidateID, err)
	}
	c.Solution = sol
	c.OptimalGestures = sol.NumMoves
	c.ReplayVerified = true
	opt := sol.NumMoves
	c.Level.CanonicalGestures = &opt
	c.SolutionDoc = ExportSolutionJSON(c.Level, c.Board, sol, elapsed, true)
	if !solutionDocIsValid(c.SolutionDoc, c.CandidateID) {
		return fmt.Errorf("%s: ExportSolutionJSON produced invalid SolutionDoc (schemaVersion=%d)", c.CandidateID, c.SolutionDoc.SchemaVersion)
	}
	return nil
}

// WriteBoardMixBatch writes Unity-compatible Candidates/Solutions + thin BatchManifest.
func WriteBoardMixBatch(dir string, res BoardMixResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candDir := filepath.Join(dir, "Candidates")
	solDir := filepath.Join(dir, "Solutions")
	_ = os.MkdirAll(candDir, 0o755)
	_ = os.MkdirAll(solDir, 0o755)

	budget := DefaultCargoFlowSolveBudget()
	if res.Config.SolveTimeLimitMs > 0 {
		budget.TimeLimit = time.Duration(res.Config.SolveTimeLimitMs) * time.Millisecond
	}
	if res.Config.MaxVisitedStates > 0 {
		budget.MaxVisited = res.Config.MaxVisitedStates
	}

	for i := range res.Accepted {
		c := &res.Accepted[i]
		solPath := filepath.Join(solDir, c.CandidateID+".solution.json")
		if err := ensureAcceptedSolutionMaterialized(c, budget, solPath); err != nil {
			return err
		}
		if !solutionDocIsValid(c.SolutionDoc, c.CandidateID) {
			return fmt.Errorf("%s: refuse to export zero/invalid SolutionDoc", c.CandidateID)
		}
		levelPath := filepath.Join(candDir, c.CandidateID+".json")
		if err := WriteJSONFile(levelPath, c.Level); err != nil {
			return err
		}
		if err := WriteJSONFile(solPath, c.SolutionDoc); err != nil {
			return err
		}
		if c.ASCIIPreview != "" {
			_ = os.WriteFile(filepath.Join(candDir, c.CandidateID+".ascii.txt"), []byte(c.ASCIIPreview), 0o644)
		}
	}

	augDist := map[string]int{}
	for _, c := range res.Pool {
		k := string(c.AugmentationClass)
		if k == "" {
			k = string(AugNone)
		}
		augDist[k]++
	}
	report := map[string]interface{}{
		"generatorVersion":  BoardMixVersion,
		"pipeline":          "CargoFlowNative7x8BoardMix",
		"rootCauseRUSH0104": "6x6-core-centric embeds produced ShiftedCore/Tall/Wide but almost never Expanded/FullField; native structural augmentation adds interacting Cargo pieces in free 7x8 space.",
		"performance":       res.Stats,
		"poolDistribution": map[string]interface{}{
			"size":                res.SelectReport.PoolSize,
			"uniqueFamilies":      res.SelectReport.PoolUniqueFamilies,
			"boardShapeClass":     res.SelectReport.PoolShapeDist,
			"inventoryClass":      res.SelectReport.PoolInventoryDist,
			"shapeInventoryCross": res.SelectReport.PoolShapeInventoryCross,
			"difficultyBands":     res.SelectReport.PoolDifficultyBands,
			"outerZoneRelevant":   res.SelectReport.PoolOuterZoneRelevant,
			"crossAvailability":   res.SelectReport.PoolCrossAvailability,
			"augmentationClass":   augDist,
		},
		"finalDistribution": map[string]interface{}{
			"finalTarget":       res.SelectReport.FinalTarget,
			"finalAccepted":     res.SelectReport.FinalAccepted,
			"missingCount":      res.SelectReport.MissingCount,
			"boardShapeClass":   res.SelectReport.FinalShapeDist,
			"inventoryClass":    res.SelectReport.FinalInventoryDist,
			"outerZoneRelevant": res.SelectReport.FinalOuterZoneRelevant,
			"distinctShapes":    res.SelectReport.DistinctBoardShapes,
		},
		"familyCoverage":        res.FamilyCoverage,
		"unmetRequirements":     res.SelectReport.UnmetRequirements,
		"whyFinalShort":         res.SelectReport.WhyFinalShort,
		"quotaRelaxations":      res.SelectReport.QuotaRelaxations,
		"hardConstraints":       res.SelectReport.HardConstraints,
		"softConstraints":       res.SelectReport.SoftConstraints,
		"expandedFullFieldDiag": res.SelectReport.ExpandedFullFieldDiag,
		"selectReport":          res.SelectReport,
		"shapeDistribution":     res.ShapeDist,
		"inventoryDistribution": res.InvDist,
		"rejected":              res.Rejected,
		"accepted":              res.Accepted,
		"totalWallMs":           res.TotalWall.Milliseconds(),
		"diversityTargetUnmet":  res.SelectReport.DiversityTargetUnmet,
		"unityExportNote":       "BatchManifest.json uses levelFile/solutionFile paths (RUSH-009 contract). Solutions are always materialized (exact solve+replay) before write.",
	}
	if err := WriteJSONFile(filepath.Join(dir, "BoardDiversityReport.json"), report); err != nil {
		return err
	}
	manifest := ToUnityBatchManifest(res.Accepted)
	if err := WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), manifest); err != nil {
		return err
	}
	if err := ValidateUnityBatch(dir); err != nil {
		return fmt.Errorf("batch export validation failed: %w", err)
	}
	return nil
}

// BatchValidationReport is the result of ValidateUnityBatch.
type BatchValidationReport struct {
	OK           bool     `json:"ok"`
	Dir          string   `json:"dir"`
	Accepted     int      `json:"accepted"`
	LevelJSON    int      `json:"levelJsonCount"`
	SolutionJSON int      `json:"solutionJsonCount"`
	Errors       []string `json:"errors,omitempty"`
}

// ValidateUnityBatch checks BatchManifest ↔ Candidates/Solutions Unity contract.
func ValidateUnityBatch(dir string) error {
	rep, err := ValidateUnityBatchReport(dir)
	if err != nil {
		return err
	}
	if !rep.OK {
		return fmt.Errorf("%s", strings.Join(rep.Errors, "; "))
	}
	return nil
}

func validateSolutionFileDeep(dir, candidateID, solRel string, level *LevelJSON) []string {
	var errs []string
	if solRel == "" {
		return []string{candidateID + ": Missing solutionFile in manifest"}
	}
	solPath := filepath.Join(dir, filepath.FromSlash(solRel))
	data, err := os.ReadFile(solPath)
	if err != nil {
		return []string{candidateID + ": Missing solution JSON"}
	}
	var doc SolutionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return []string{candidateID + ": solution JSON parse failed: " + err.Error()}
	}
	if doc.SchemaVersion != CargoFlowJSONSchemaVersion {
		errs = append(errs, fmt.Sprintf("%s: Solution schemaVersion unsupported (%d want %d)", candidateID, doc.SchemaVersion, CargoFlowJSONSchemaVersion))
	}
	if doc.LevelID != candidateID {
		errs = append(errs, fmt.Sprintf("%s: solution levelId mismatch (%q)", candidateID, doc.LevelID))
	}
	if level != nil && level.LevelID != "" && doc.LevelID != level.LevelID {
		errs = append(errs, fmt.Sprintf("%s: solution.levelId != level.levelId", candidateID))
	}
	if !doc.Solved {
		errs = append(errs, candidateID+": solution.solved != true")
	}
	if !doc.Optimal {
		errs = append(errs, candidateID+": solution.optimal != true")
	}
	if doc.OptimalGestures <= 0 {
		errs = append(errs, candidateID+": solution.optimalGestures invalid")
	}
	if len(doc.Moves) == 0 {
		errs = append(errs, candidateID+": solution.moves empty")
	}
	if !doc.ReplayPass {
		errs = append(errs, candidateID+": solution.replayPass != true")
	}
	if !doc.ReplayVerified {
		errs = append(errs, candidateID+": solution.replayVerified != true")
	}
	if level != nil && level.CanonicalGestures != nil && doc.OptimalGestures != *level.CanonicalGestures {
		errs = append(errs, fmt.Sprintf("%s: solution.optimalGestures (%d) != level.canonicalGestures (%d)",
			candidateID, doc.OptimalGestures, *level.CanonicalGestures))
	}
	if len(errs) == 0 && !solutionDocIsValid(doc, candidateID) {
		errs = append(errs, candidateID+": solutionDocIsValid failed")
	}
	return errs
}

// ValidateUnityBatchReport returns a structured validation report with deep solution checks.
func ValidateUnityBatchReport(dir string) (BatchValidationReport, error) {
	rep := BatchValidationReport{Dir: dir}
	manPath := filepath.Join(dir, "BatchManifest.json")
	data, err := os.ReadFile(manPath)
	if err != nil {
		rep.Errors = append(rep.Errors, "BatchManifest.json missing: "+err.Error())
		return rep, nil
	}
	var man BoardMixUnityManifest
	if err := json.Unmarshal(data, &man); err != nil || len(man.Candidates) == 0 {
		var raw map[string]json.RawMessage
		if err2 := json.Unmarshal(data, &raw); err2 != nil {
			rep.Errors = append(rep.Errors, "BatchManifest.json parse failed: "+err.Error())
			return rep, nil
		}
		var cands []map[string]interface{}
		if err2 := json.Unmarshal(raw["candidates"], &cands); err2 != nil {
			rep.Errors = append(rep.Errors, "BatchManifest candidates parse failed")
			return rep, nil
		}
		for i, c := range cands {
			id, _ := c["candidateId"].(string)
			lf, _ := c["levelFile"].(string)
			sf, _ := c["solutionFile"].(string)
			if id == "" {
				id = fmt.Sprintf("index_%d", i)
			}
			if lf == "" {
				rep.Errors = append(rep.Errors, id+": Missing levelFile in manifest (Unity cannot resolve level JSON)")
				continue
			}
			levelPath := filepath.Join(dir, filepath.FromSlash(lf))
			level, lerr := LoadCargoFlowLevelJSONFile(levelPath)
			if lerr != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: Missing or invalid level JSON at %s", id, lf))
				continue
			}
			rep.LevelJSON++
			serrs := validateSolutionFileDeep(dir, id, sf, level)
			if len(serrs) == 0 {
				rep.SolutionJSON++
			} else {
				rep.Errors = append(rep.Errors, serrs...)
			}
		}
		rep.Accepted = len(cands)
		rep.OK = len(rep.Errors) == 0
		return rep, nil
	}

	rep.Accepted = len(man.Candidates)
	seen := map[string]bool{}
	for _, e := range man.Candidates {
		id := e.CandidateID
		if id == "" {
			rep.Errors = append(rep.Errors, "manifest entry missing candidateId")
			continue
		}
		if seen[id] {
			rep.Errors = append(rep.Errors, id+": duplicate candidateId")
		}
		seen[id] = true
		if e.LevelFile == "" {
			rep.Errors = append(rep.Errors, id+": Missing levelFile in manifest")
			continue
		}
		levelPath := filepath.Join(dir, filepath.FromSlash(e.LevelFile))
		level, err := LoadCargoFlowLevelJSONFile(levelPath)
		if err != nil {
			rep.Errors = append(rep.Errors, id+": Missing level JSON / parse failed: "+err.Error())
			continue
		}
		if level.LevelID != "" && level.LevelID != id {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: levelId mismatch (%s)", id, level.LevelID))
		}
		if level.Width != CargoFlowWidth || level.Height != CargoFlowHeight {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: expected 7x8, got %dx%d", id, level.Width, level.Height))
		}
		rep.LevelJSON++

		serrs := validateSolutionFileDeep(dir, id, e.SolutionFile, level)
		if len(serrs) == 0 {
			rep.SolutionJSON++
			if board, err := BoardFromLevelJSON(level); err == nil {
				sol := board.SolveWithBudget(SolveBudget{TimeLimit: 3 * time.Second, MaxVisited: 500_000})
				if sol.Solvable && !sol.TimedOut && !sol.BudgetExceeded {
					work := board.Copy()
					if err := work.Replay(sol.Moves); err != nil {
						rep.Errors = append(rep.Errors, id+": level exact-solve replay failed: "+err.Error())
					}
				}
			}
		} else {
			rep.Errors = append(rep.Errors, serrs...)
		}
	}
	rep.OK = len(rep.Errors) == 0
	return rep, nil
}

// ReexportBoardMixBatchFromDir rebuilds Unity Candidates + Solutions + thin BatchManifest
// from existing final Level JSON / checkpoint metadata — without content generation.
func ReexportBoardMixBatchFromDir(dir string) (BatchValidationReport, error) {
	accepted, err := loadAcceptedForReexport(dir)
	if err != nil {
		return BatchValidationReport{Dir: dir, Errors: []string{err.Error()}}, err
	}
	res := BoardMixResult{Accepted: accepted}
	if cp, err := LoadBoardMixCheckpoint(filepath.Join(dir, "checkpoint.json")); err == nil {
		res.Pool = cp.Pool
		res.SelectReport = cp.SelectReport
		res.Stats = cp.Stats
		res.Rejected = cp.Rejected
	}
	res.Config.SolveTimeLimitMs = 8000
	res.Config.MaxVisitedStates = 2_000_000
	if err := WriteBoardMixBatch(dir, res); err != nil {
		return BatchValidationReport{Dir: dir, Errors: []string{err.Error()}}, err
	}
	return ValidateUnityBatchReport(dir)
}

func loadAcceptedForReexport(dir string) ([]BoardMixAccepted, error) {
	candDir := filepath.Join(dir, "Candidates")
	entries, err := os.ReadDir(candDir)
	if err == nil {
		ids := []string{}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".json") {
				continue
			}
			id := strings.TrimSuffix(name, ".json")
			if strings.HasPrefix(id, "Candidate_") {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			metaByID := map[string]BoardMixAccepted{}
			if cp, err := LoadBoardMixCheckpoint(filepath.Join(dir, "checkpoint.json")); err == nil {
				for _, c := range cp.Accepted {
					metaByID[c.CandidateID] = c
				}
			}
			out := make([]BoardMixAccepted, 0, len(ids))
			for _, id := range ids {
				level, err := LoadCargoFlowLevelJSONFile(filepath.Join(candDir, id+".json"))
				if err != nil {
					return nil, fmt.Errorf("%s: load level: %w", id, err)
				}
				cc := metaByID[id]
				cc.CandidateID = id
				cc.Level = level
				if b, err := BoardFromLevelJSON(level); err == nil {
					cc.Board = b
				}
				cc.SolutionDoc = SolutionJSON{}
				cc.Solution = Solution{}
				out = append(out, cc)
			}
			return out, nil
		}
	}

	if cp, err := LoadBoardMixCheckpoint(filepath.Join(dir, "checkpoint.json")); err == nil && len(cp.Accepted) > 0 {
		out := make([]BoardMixAccepted, 0, len(cp.Accepted))
		for _, c := range cp.Accepted {
			cc := c
			cc.SolutionDoc = SolutionJSON{}
			cc.Solution = Solution{}
			p := filepath.Join(dir, "Candidates", cc.CandidateID+".json")
			if level, err := LoadCargoFlowLevelJSONFile(p); err == nil {
				cc.Level = level
				if b, err := BoardFromLevelJSON(level); err == nil {
					cc.Board = b
				}
			}
			if cc.Level == nil {
				return nil, fmt.Errorf("checkpoint accepted %s has no Level on disk — cannot reexport", cc.CandidateID)
			}
			out = append(out, cc)
		}
		return out, nil
	}

	reportPath := filepath.Join(dir, "BoardDiversityReport.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("no Candidates/ and no checkpoint/report: %w", err)
	}
	var raw struct {
		Accepted []BoardMixAccepted `json:"accepted"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw.Accepted) == 0 {
		return nil, fmt.Errorf("BoardDiversityReport.accepted empty")
	}
	for i := range raw.Accepted {
		c := &raw.Accepted[i]
		c.SolutionDoc = SolutionJSON{}
		c.Solution = Solution{}
		p := filepath.Join(dir, "Candidates", c.CandidateID+".json")
		if level, err := LoadCargoFlowLevelJSONFile(p); err == nil {
			c.Level = level
			if b, err := BoardFromLevelJSON(level); err == nil {
				c.Board = b
			}
		}
		if err := ensureAcceptedLevelMaterialized(c); err != nil {
			return nil, err
		}
	}
	return raw.Accepted, nil
}
