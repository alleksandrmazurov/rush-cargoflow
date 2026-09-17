package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	GeneratorVersion string                      `json:"generatorVersion"`
	Pipeline         string                      `json:"pipeline"`
	Accepted         int                         `json:"accepted"`
	CandidateCount   int                         `json:"candidateCount"`
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

// WriteBoardMixBatch writes Unity-compatible Candidates/Solutions + thin BatchManifest.
func WriteBoardMixBatch(dir string, res BoardMixResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	candDir := filepath.Join(dir, "Candidates")
	solDir := filepath.Join(dir, "Solutions")
	_ = os.MkdirAll(candDir, 0o755)
	_ = os.MkdirAll(solDir, 0o755)

	for i := range res.Accepted {
		c := &res.Accepted[i]
		if err := ensureAcceptedLevelMaterialized(c); err != nil {
			return err
		}
		levelPath := filepath.Join(candDir, c.CandidateID+".json")
		if err := WriteJSONFile(levelPath, c.Level); err != nil {
			return err
		}
		solPath := filepath.Join(solDir, c.CandidateID+".solution.json")
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
		"unityExportNote":       "BatchManifest.json uses levelFile/solutionFile paths (RUSH-009 contract). Full accepted payloads remain in BoardDiversityReport.json / checkpoint.json.",
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
	OK            bool     `json:"ok"`
	Dir           string   `json:"dir"`
	Accepted      int      `json:"accepted"`
	LevelJSON     int      `json:"levelJsonCount"`
	SolutionJSON  int      `json:"solutionJsonCount"`
	Errors        []string `json:"errors,omitempty"`
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

// ValidateUnityBatchReport returns a structured validation report.
func ValidateUnityBatchReport(dir string) (BatchValidationReport, error) {
	rep := BatchValidationReport{Dir: dir}
	manPath := filepath.Join(dir, "BatchManifest.json")
	data, err := os.ReadFile(manPath)
	if err != nil {
		rep.Errors = append(rep.Errors, "BatchManifest.json missing: "+err.Error())
		return rep, nil
	}
	var man BoardMixUnityManifest
	if err := json.Unmarshal(data, &man); err != nil {
		// Tolerate legacy fat manifests during diagnosis, but still require levelFile.
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
			if lf == "" {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: Missing levelFile in manifest (Unity cannot resolve level JSON)", firstNonEmpty(id, fmt.Sprintf("index_%d", i))))
				continue
			}
			levelPath := filepath.Join(dir, filepath.FromSlash(lf))
			if _, err := os.Stat(levelPath); err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s: Missing level JSON at %s", id, lf))
			}
			if sf != "" {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(sf))); err != nil {
					rep.Errors = append(rep.Errors, fmt.Sprintf("%s: Missing solution JSON at %s", id, sf))
				}
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
		if _, err := os.Stat(levelPath); err != nil {
			rep.Errors = append(rep.Errors, id+": Missing level JSON")
			continue
		}
		level, err := LoadCargoFlowLevelJSONFile(levelPath)
		if err != nil {
			rep.Errors = append(rep.Errors, id+": level JSON parse failed: "+err.Error())
			continue
		}
		if level.LevelID != "" && level.LevelID != id {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: levelId mismatch (%s)", id, level.LevelID))
		}
		if level.Width != CargoFlowWidth || level.Height != CargoFlowHeight {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: expected 7x8, got %dx%d", id, level.Width, level.Height))
		}
		rep.LevelJSON++
		if e.SolutionFile == "" {
			rep.Errors = append(rep.Errors, id+": Missing solutionFile in manifest")
		} else {
			solPath := filepath.Join(dir, filepath.FromSlash(e.SolutionFile))
			if _, err := os.Stat(solPath); err != nil {
				rep.Errors = append(rep.Errors, id+": Missing solution JSON")
			} else {
				rep.SolutionJSON++
			}
		}
	}
	rep.OK = len(rep.Errors) == 0
	return rep, nil
}

// ReexportBoardMixBatchFromDir rebuilds Unity Candidates + thin BatchManifest from checkpoint/report
// without re-running exact generation.
func ReexportBoardMixBatchFromDir(dir string) (BatchValidationReport, error) {
	accepted, err := loadAcceptedForReexport(dir)
	if err != nil {
		return BatchValidationReport{Dir: dir, Errors: []string{err.Error()}}, err
	}
	res := BoardMixResult{Accepted: accepted}
	// Preserve existing reports if present.
	if data, err := os.ReadFile(filepath.Join(dir, "BoardDiversityReport.json")); err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			_ = raw
		}
	}
	if cp, err := LoadBoardMixCheckpoint(filepath.Join(dir, "checkpoint.json")); err == nil {
		res.Pool = cp.Pool
		res.SelectReport = cp.SelectReport
		res.Stats = cp.Stats
		res.Rejected = cp.Rejected
	}
	if err := WriteBoardMixBatch(dir, res); err != nil {
		return BatchValidationReport{Dir: dir, Errors: []string{err.Error()}}, err
	}
	return ValidateUnityBatchReport(dir)
}

func loadAcceptedForReexport(dir string) ([]BoardMixAccepted, error) {
	// Prefer checkpoint Accepted (has Level payloads from last successful select).
	if cp, err := LoadBoardMixCheckpoint(filepath.Join(dir, "checkpoint.json")); err == nil && len(cp.Accepted) > 0 {
		out := make([]BoardMixAccepted, 0, len(cp.Accepted))
		for _, c := range cp.Accepted {
			cc := c
			if cc.Level == nil && cc.Board == nil {
				// Try load existing file if already exported once.
				p := filepath.Join(dir, "Candidates", cc.CandidateID+".json")
				if level, err := LoadCargoFlowLevelJSONFile(p); err == nil {
					cc.Level = level
					if b, err := BoardFromLevelJSON(level); err == nil {
						cc.Board = b
					}
				}
			}
			if cc.Level == nil && cc.Board != nil {
				_ = ensureAcceptedLevelMaterialized(&cc)
			}
			if cc.Level == nil {
				return nil, fmt.Errorf("checkpoint accepted %s has no Level — cannot reexport", cc.CandidateID)
			}
			out = append(out, cc)
		}
		return out, nil
	}

	// Fall back: BoardDiversityReport.accepted
	reportPath := filepath.Join(dir, "BoardDiversityReport.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("no checkpoint Accepted and no BoardDiversityReport: %w", err)
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
		if c.Level == nil {
			p := filepath.Join(dir, "Candidates", c.CandidateID+".json")
			if level, err := LoadCargoFlowLevelJSONFile(p); err == nil {
				c.Level = level
			}
		}
		if err := ensureAcceptedLevelMaterialized(c); err != nil {
			return nil, err
		}
	}
	return raw.Accepted, nil
}
