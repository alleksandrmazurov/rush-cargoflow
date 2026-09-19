package rush

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PilotDistribution struct {
	Accepted               int                   `json:"accepted"`
	OptimalGestures        map[int]int           `json:"optimalGestures"`
	TargetTopRows          map[int]int           `json:"targetTopRows"`
	BoardShapes            map[string]int        `json:"boardShapes"`
	InventoryClasses       map[string]int        `json:"inventoryClasses"`
	CausalTemplates        map[string]int        `json:"causalTemplates"`
	CausalExpanded         int                   `json:"causalExpanded"`
	ReplayVerified         int                   `json:"replayVerified"`
	ValidCausalProofs      int                   `json:"validCausalProofs"`
	TargetRow5Plus         int                   `json:"targetRow5Plus"`
	TargetRow6             int                   `json:"targetRow6"`
	DiversityTargetUnmet   bool                  `json:"diversityTargetUnmet"`
	UnityBatchValidation   BatchValidationReport `json:"unityBatchValidation"`
	AcceptanceCriteriaPass bool                  `json:"acceptanceCriteriaPass"`
}

type ExactStateRepeat struct {
	ControlCandidateID string `json:"controlCandidateId"`
	SecondCandidateID  string `json:"secondCandidateId"`
	Fingerprint        string `json:"fingerprint"`
}

type PilotComparisonReport struct {
	SchemaVersion               int                `json:"schemaVersion"`
	ControlDir                  string             `json:"controlDir"`
	SecondDir                   string             `json:"secondDir"`
	Control                     PilotDistribution  `json:"control"`
	Second                      PilotDistribution  `json:"second"`
	ExactStateRepeatCount       int                `json:"exactStateRepeatCount"`
	ExactStateRepeats           []ExactStateRepeat `json:"exactStateRepeats,omitempty"`
	NewStateCount               int                `json:"newStateCount"`
	ControlFamilyCount          int                `json:"controlFamilyCount"`
	SecondFamilyCount           int                `json:"secondFamilyCount"`
	FamilyIntersectionCount     int                `json:"familyIntersectionCount"`
	FamilyIntersection          []string           `json:"familyIntersection"`
	NewFamilies                 []string           `json:"newFamilies"`
	RemovedFamilies             []string           `json:"removedFamilies"`
	SeedAffectsBoardMix         bool               `json:"seedAffectsBoardMix"`
	SeedIndependenceConclusion  string             `json:"seedIndependenceConclusion"`
	RecommendedManualCandidates []string           `json:"recommendedManualCandidates,omitempty"`
	RecommendationNote          string             `json:"recommendationNote"`
}

type pilotBatchData struct {
	manifest             BoardMixUnityManifest
	accepted             []BoardMixAccepted
	selectReport         BoardMixSelectReport
	diversityTargetUnmet bool
	levels               map[string]*LevelJSON
	fingerprints         map[string]string
	validation           BatchValidationReport
}

// ExactStartStateFingerprint is independent of piece IDs while retaining
// dimensions, Cargo rules, exit, walls, piece movement classes, positions, and
// immobility.
func ExactStartStateFingerprint(board *Board) string {
	if board == nil {
		return ""
	}
	walls := append([]int{}, board.Walls...)
	sort.Ints(walls)
	pieces := make([]string, 0, len(board.Pieces))
	for i, piece := range board.Pieces {
		orientation := piece.Orientation
		if piece.Kind == PieceUnit {
			orientation = Horizontal // units are dual-axis; stored orientation is not a rule
		}
		immobile := false
		if i < len(board.ImmobilePieces) {
			immobile = board.ImmobilePieces[i]
		}
		pieces = append(pieces, fmt.Sprintf("%d:%d:%d:%d:%t",
			piece.Kind, piece.Size, orientation, piece.Position, immobile))
	}
	sort.Strings(pieces)
	payload := fmt.Sprintf("w=%d|h=%d|rules=%d|exit=%d|won=%t|walls=%v|pieces=%s",
		board.Width, board.Height, board.Rules, board.ExitCol, board.won,
		walls, strings.Join(pieces, ","))
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func CompareBoardMixPilots(controlDir, secondDir, outputDir string) (PilotComparisonReport, error) {
	report := PilotComparisonReport{
		SchemaVersion: 1, ControlDir: controlDir, SecondDir: secondDir,
		SeedAffectsBoardMix: false,
	}
	control, err := loadPilotBatchData(controlDir)
	if err != nil {
		return report, err
	}
	second, err := loadPilotBatchData(secondDir)
	if err != nil {
		return report, err
	}
	report.Control = summarizePilot(control)
	report.Second = summarizePilot(second)

	controlByFingerprint := map[string][]string{}
	for candidateID, fingerprint := range control.fingerprints {
		controlByFingerprint[fingerprint] = append(controlByFingerprint[fingerprint], candidateID)
	}
	newCandidateIDs := []string{}
	for secondID, fingerprint := range second.fingerprints {
		matches := controlByFingerprint[fingerprint]
		if len(matches) == 0 {
			newCandidateIDs = append(newCandidateIDs, secondID)
			continue
		}
		sort.Strings(matches)
		report.ExactStateRepeats = append(report.ExactStateRepeats, ExactStateRepeat{
			ControlCandidateID: matches[0], SecondCandidateID: secondID, Fingerprint: fingerprint,
		})
	}
	sort.Slice(report.ExactStateRepeats, func(i, j int) bool {
		return report.ExactStateRepeats[i].SecondCandidateID < report.ExactStateRepeats[j].SecondCandidateID
	})
	report.ExactStateRepeatCount = len(report.ExactStateRepeats)
	report.NewStateCount = len(newCandidateIDs)

	controlFamilies := acceptedFamilySet(control.accepted)
	secondFamilies := acceptedFamilySet(second.accepted)
	report.ControlFamilyCount = len(controlFamilies)
	report.SecondFamilyCount = len(secondFamilies)
	for family := range secondFamilies {
		if controlFamilies[family] {
			report.FamilyIntersection = append(report.FamilyIntersection, family)
		} else {
			report.NewFamilies = append(report.NewFamilies, family)
		}
	}
	for family := range controlFamilies {
		if !secondFamilies[family] {
			report.RemovedFamilies = append(report.RemovedFamilies, family)
		}
	}
	sort.Strings(report.FamilyIntersection)
	sort.Strings(report.NewFamilies)
	sort.Strings(report.RemovedFamilies)
	report.FamilyIntersectionCount = len(report.FamilyIntersection)

	report.RecommendedManualCandidates = recommendNewManualCandidates(second.accepted, newCandidateIDs, 4)
	if len(report.RecommendedManualCandidates) == 0 {
		report.RecommendationNote = "No genuinely new start states were produced; there are no new levels to recommend for manual play."
	} else if len(report.RecommendedManualCandidates) < 4 {
		report.RecommendationNote = fmt.Sprintf("Only %d genuinely new states exist; all are recommended.", len(report.RecommendedManualCandidates))
	} else {
		report.RecommendationNote = "Four genuinely new candidates selected for causal, target-row, shape, inventory, and optimal-gesture diversity."
	}
	if report.NewStateCount == 0 {
		report.SeedIndependenceConclusion = "Changing seed did not change any accepted start state. Either the available accepted alternatives collapsed to the same final set, or the compared run predates seeded Boardmix selection."
	} else {
		report.SeedAffectsBoardMix = true
		report.SeedIndependenceConclusion = "Accepted states changed. In Boardmix v1.7.2+, Config.Seed participates in deterministic job ordering, per-family retention, and final selection tie-breaks among admissible alternatives."
	}

	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return report, err
		}
		if err := WriteJSONFile(filepath.Join(outputDir, "PilotComparisonReport.json"), report); err != nil {
			return report, err
		}
		if err := os.WriteFile(filepath.Join(outputDir, "PilotComparisonReport.md"),
			[]byte(renderPilotComparisonMarkdown(report)), 0o644); err != nil {
			return report, err
		}
	}
	return report, nil
}

func loadPilotBatchData(dir string) (pilotBatchData, error) {
	data := pilotBatchData{
		levels: map[string]*LevelJSON{}, fingerprints: map[string]string{},
	}
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "BatchManifest.json"))
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(manifestBytes, &data.manifest); err != nil {
		return data, err
	}
	reportBytes, err := os.ReadFile(filepath.Join(dir, "BoardDiversityReport.json"))
	if err != nil {
		return data, err
	}
	var report struct {
		Accepted             []BoardMixAccepted   `json:"accepted"`
		SelectReport         BoardMixSelectReport `json:"selectReport"`
		DiversityTargetUnmet bool                 `json:"diversityTargetUnmet"`
	}
	if err := json.Unmarshal(reportBytes, &report); err != nil {
		return data, err
	}
	data.accepted = report.Accepted
	data.selectReport = report.SelectReport
	data.diversityTargetUnmet = report.DiversityTargetUnmet
	for _, entry := range data.manifest.Candidates {
		level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, filepath.FromSlash(entry.LevelFile)))
		if err != nil {
			return data, fmt.Errorf("%s: %w", entry.CandidateID, err)
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return data, fmt.Errorf("%s: %w", entry.CandidateID, err)
		}
		data.levels[entry.CandidateID] = level
		data.fingerprints[entry.CandidateID] = ExactStartStateFingerprint(board)
	}
	validation, err := ValidateUnityBatchReport(dir)
	if err != nil {
		return data, err
	}
	data.validation = validation
	return data, nil
}

func summarizePilot(data pilotBatchData) PilotDistribution {
	out := PilotDistribution{
		Accepted:        len(data.accepted),
		OptimalGestures: map[int]int{}, TargetTopRows: map[int]int{},
		BoardShapes: map[string]int{}, InventoryClasses: map[string]int{},
		CausalTemplates: map[string]int{}, DiversityTargetUnmet: data.diversityTargetUnmet,
		UnityBatchValidation: data.validation,
	}
	for _, candidate := range data.accepted {
		out.OptimalGestures[candidate.OptimalGestures]++
		row := candidateTargetTopRow(candidate)
		out.TargetTopRows[row]++
		if row >= 5 {
			out.TargetRow5Plus++
		}
		if row == 6 {
			out.TargetRow6++
		}
		out.BoardShapes[string(candidate.BoardUtil.BoardShapeClass)]++
		out.InventoryClasses[string(candidate.InventoryClass)]++
		if candidate.ReplayVerified {
			out.ReplayVerified++
		}
		if IsCausalExpanded(candidate) {
			out.CausalExpanded++
			out.CausalTemplates[string(candidate.CausalTemplate)]++
			if candidate.CausalProof.Valid && candidate.CausalProof.ReplayVerified &&
				candidate.CausalProof.CrossRegionDependencyEdgeCount > 0 {
				out.ValidCausalProofs++
			}
		}
	}
	out.AcceptanceCriteriaPass =
		out.Accepted == 12 &&
			out.CausalExpanded >= 8 &&
			out.ValidCausalProofs == out.CausalExpanded &&
			out.ReplayVerified == out.Accepted &&
			out.TargetRow5Plus >= 5 &&
			out.TargetRow6 >= 2 &&
			!out.DiversityTargetUnmet &&
			out.UnityBatchValidation.OK
	return out
}

func acceptedFamilySet(accepted []BoardMixAccepted) map[string]bool {
	out := map[string]bool{}
	for _, candidate := range accepted {
		out[candidate.FamilyID] = true
	}
	return out
}

func recommendNewManualCandidates(accepted []BoardMixAccepted, newIDs []string, count int) []string {
	newSet := map[string]bool{}
	for _, id := range newIDs {
		newSet[id] = true
	}
	remaining := []BoardMixAccepted{}
	for _, candidate := range accepted {
		if newSet[candidate.CandidateID] {
			remaining = append(remaining, candidate)
		}
	}
	selected := []string{}
	usedRows, usedShapes, usedInventory, usedTemplates := map[int]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for len(selected) < count && len(remaining) > 0 {
		bestIndex, bestScore := -1, -1
		for i, candidate := range remaining {
			score := candidate.OptimalGestures
			if IsCausalExpanded(candidate) {
				score += 100
			}
			row := candidateTargetTopRow(candidate)
			if !usedRows[row] {
				score += 40
			}
			shape := string(candidate.BoardUtil.BoardShapeClass)
			if !usedShapes[shape] {
				score += 20
			}
			inventory := string(candidate.InventoryClass)
			if !usedInventory[inventory] {
				score += 15
			}
			template := string(candidate.CausalTemplate)
			if template != "" && !usedTemplates[template] {
				score += 25
			}
			if score > bestScore || (score == bestScore &&
				(bestIndex < 0 || candidate.CandidateID < remaining[bestIndex].CandidateID)) {
				bestIndex, bestScore = i, score
			}
		}
		candidate := remaining[bestIndex]
		selected = append(selected, candidate.CandidateID)
		usedRows[candidateTargetTopRow(candidate)] = true
		usedShapes[string(candidate.BoardUtil.BoardShapeClass)] = true
		usedInventory[string(candidate.InventoryClass)] = true
		usedTemplates[string(candidate.CausalTemplate)] = true
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	return selected
}

func renderPilotComparisonMarkdown(report PilotComparisonReport) string {
	var b strings.Builder
	b.WriteString("# RUSH-010.7.1 pilot reproducibility\n\n")
	b.WriteString(fmt.Sprintf("- Exact repeated start states: %d/12\n", report.ExactStateRepeatCount))
	b.WriteString(fmt.Sprintf("- Genuinely new start states: %d\n", report.NewStateCount))
	b.WriteString(fmt.Sprintf("- Family intersection: %d (new families: %d)\n", report.FamilyIntersectionCount, len(report.NewFamilies)))
	b.WriteString(fmt.Sprintf("- Control criteria pass: %t\n", report.Control.AcceptanceCriteriaPass))
	b.WriteString(fmt.Sprintf("- Second criteria pass: %t\n", report.Second.AcceptanceCriteriaPass))
	b.WriteString("- Seed conclusion: " + report.SeedIndependenceConclusion + "\n\n")
	b.WriteString("## Distributions\n\n")
	b.WriteString(fmt.Sprintf("- Control optimal: %s\n", formatIntDistribution(report.Control.OptimalGestures)))
	b.WriteString(fmt.Sprintf("- Second optimal: %s\n", formatIntDistribution(report.Second.OptimalGestures)))
	b.WriteString(fmt.Sprintf("- Control Target rows: %s\n", formatIntDistribution(report.Control.TargetTopRows)))
	b.WriteString(fmt.Sprintf("- Second Target rows: %s\n", formatIntDistribution(report.Second.TargetTopRows)))
	b.WriteString(fmt.Sprintf("- Control shapes: %v\n", report.Control.BoardShapes))
	b.WriteString(fmt.Sprintf("- Second shapes: %v\n", report.Second.BoardShapes))
	b.WriteString(fmt.Sprintf("- Control inventory: %v\n", report.Control.InventoryClasses))
	b.WriteString(fmt.Sprintf("- Second inventory: %v\n", report.Second.InventoryClasses))
	b.WriteString("\n## Manual recommendations\n\n")
	b.WriteString(report.RecommendationNote + "\n")
	for _, candidateID := range report.RecommendedManualCandidates {
		b.WriteString("- " + candidateID + "\n")
	}
	return b.String()
}
