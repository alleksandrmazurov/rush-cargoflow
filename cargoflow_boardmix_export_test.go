package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func syntheticExportCandidate(t *testing.T, id string) BoardMixAccepted {
	t.Helper()
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.ExitCol = CargoFlowExitCol
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 2*CargoFlowWidth + 1, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	level, err := LevelJSONFromBoard(board, id, "test")
	if err != nil {
		t.Fatal(err)
	}
	sol := board.SolveWithBudget(SolveBudget{TimeLimit: 0, MaxVisited: 50_000})
	c := BoardMixAccepted{
		CandidateID:     id,
		BaseCandidateID: "Base_001",
		FamilyID:        "F_TEST",
		InventoryClass:  InvNo1x1,
		BoardUtil: BoardUtilizationMetrics{
			BoardShapeClass:   ShapeShiftedCore,
			OuterZoneRelevant: true,
		},
		AugmentationClass: AugSideGate,
		OptimalGestures:   sol.NumMoves,
		ReplayVerified:    sol.Solvable,
		Level:             level,
		Board:             board,
		Solution:          sol,
		ASCIIPreview:      "x",
	}
	if sol.Solvable {
		c.SolutionDoc = ExportSolutionJSON(level, board, sol, 0, true)
	}
	return c
}

func TestKnownGoodBatchContractStillPASS(t *testing.T) {
	dir := "output/RUSH009_CuratedShortlist_001"
	if _, err := os.Stat(filepath.Join(dir, "BatchManifest.json")); err != nil {
		t.Skip("known-good batch not present")
	}
	data, err := os.ReadFile(filepath.Join(dir, "BatchManifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var cands []map[string]interface{}
	if err := json.Unmarshal(raw["candidates"], &cands); err != nil {
		t.Fatal(err)
	}
	if len(cands) == 0 {
		t.Fatal("empty")
	}
	lf, _ := cands[0]["levelFile"].(string)
	if lf == "" || lf[:10] != "Candidates" {
		t.Fatalf("known-good must use levelFile paths, got %q", lf)
	}
}

func TestNativeFinalCandidateExportsLevelJson(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	res := BoardMixResult{Accepted: []BoardMixAccepted{c}}
	if err := WriteBoardMixBatch(dir, res); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Candidates", "Candidate_001.json")); err != nil {
		t.Fatal(err)
	}
}

func TestManifestEntryLevelJsonExists(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}}); err != nil {
		t.Fatal(err)
	}
	rep, err := ValidateUnityBatchReport(dir)
	if err != nil || !rep.OK {
		t.Fatalf("%+v %v", rep, err)
	}
}

func TestCandidateIdMatchesFilenameOrManifestPASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_007")
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "BatchManifest.json"))
	var man BoardMixUnityManifest
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	if man.Candidates[0].CandidateID != "Candidate_007" {
		t.Fatal(man.Candidates[0].CandidateID)
	}
	if man.Candidates[0].LevelFile != "Candidates/Candidate_007.json" {
		t.Fatal(man.Candidates[0].LevelFile)
	}
}

func TestExportedNativeLevelParsesPASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, "Candidates", "Candidate_001.json"))
	if err != nil {
		t.Fatal(err)
	}
	if level.LevelID != "Candidate_001" {
		t.Fatal(level.LevelID)
	}
}

func TestExportedNativeLevelUses7x8PASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, "Candidates", "Candidate_001.json"))
	if err != nil {
		t.Fatal(err)
	}
	if level.Width != 7 || level.Height != 8 {
		t.Fatalf("%dx%d", level.Width, level.Height)
	}
}

func TestExportedNativeLevelReplayPASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	if !c.Solution.Solvable {
		t.Skip("synthetic unsolvable under tiny budget")
	}
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, "Candidates", "Candidate_001.json"))
	if err != nil {
		t.Fatal(err)
	}
	board, err := BoardFromLevelJSON(level)
	if err != nil {
		t.Fatal(err)
	}
	if err := board.Replay(c.Solution.Moves); err != nil {
		t.Fatal(err)
	}
}

func TestOptionalNativeMetadataDoesNotBreakContractPASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	c.NativeVariantFingerprint = "fp-test"
	c.AugmentationClass = AugLongPieceExt
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	data, _ := os.ReadFile(filepath.Join(dir, "BatchManifest.json"))
	var man BoardMixUnityManifest
	_ = json.Unmarshal(data, &man)
	if man.Candidates[0].LevelFile == "" {
		t.Fatal("levelFile required")
	}
	// optional fields present but importer ignores unknowns
	if man.Candidates[0].AugmentationClass == "" {
		t.Fatal("optional metadata should serialize")
	}
}

func TestMissingLevelJsonValidatorFailsPASS(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	_ = os.Remove(filepath.Join(dir, "Candidates", "Candidate_001.json"))
	rep, _ := ValidateUnityBatchReport(dir)
	if rep.OK {
		t.Fatal("expected failure")
	}
	found := false
	for _, e := range rep.Errors {
		if containsSubstr(e, "Missing level JSON") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors=%v", rep.Errors)
	}
}

func TestTwelveFinalCandidatesProduceTwelveLevelJsonPASS(t *testing.T) {
	dir := t.TempDir()
	accepted := make([]BoardMixAccepted, 12)
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		accepted[i] = syntheticExportCandidate(t, id)
		accepted[i].FamilyID = fmt.Sprintf("F%03d", i+1)
	}
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: accepted}); err != nil {
		t.Fatal(err)
	}
	rep, err := ValidateUnityBatchReport(dir)
	if err != nil || !rep.OK || rep.LevelJSON != 12 || rep.Accepted != 12 {
		t.Fatalf("%+v err=%v", rep, err)
	}
}

func TestFatManifestWithoutLevelFileFailsValidation(t *testing.T) {
	dir := t.TempDir()
	// Simulate RUSH-010.4.1 broken contract: candidates without levelFile.
	man := map[string]interface{}{
		"candidateCount": 1,
		"candidates": []map[string]interface{}{
			{"candidateId": "Candidate_001", "familyId": "F1"},
		},
	}
	_ = WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), man)
	_ = os.MkdirAll(filepath.Join(dir, "Candidates"), 0o755)
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteJSONFile(filepath.Join(dir, "Candidates", "Candidate_001.json"), c.Level)
	rep, _ := ValidateUnityBatchReport(dir)
	if rep.OK {
		t.Fatal("fat/broken manifest without levelFile must fail")
	}
}
