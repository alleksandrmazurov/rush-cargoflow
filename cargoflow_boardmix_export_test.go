package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func syntheticExportCandidate(t *testing.T, id string) BoardMixAccepted {
	t.Helper()
	// Prefer a known-good transplanted shortlist level when available.
	path := "output/RUSH009_CuratedShortlist_001/Candidates/Candidate_003.json"
	if level, err := LoadCargoFlowLevelJSONFile(path); err == nil {
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			t.Fatal(err)
		}
		sol := board.SolveWithBudget(DefaultCargoFlowSolveBudget())
		if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
			t.Skip("shortlist fixture not solvable under budget")
		}
		work := board.Copy()
		if err := work.Replay(sol.Moves); err != nil {
			t.Fatal(err)
		}
		level.LevelID = id
		opt := sol.NumMoves
		level.CanonicalGestures = &opt
		doc := ExportSolutionJSON(level, board, sol, 0, true)
		return BoardMixAccepted{
			CandidateID:       id,
			BaseCandidateID:   "Base_001",
			FamilyID:          "F_TEST",
			InventoryClass:    InvNo1x1,
			BoardUtil:         BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: true},
			AugmentationClass: AugSideGate,
			OptimalGestures:   sol.NumMoves,
			ReplayVerified:    true,
			Level:             level,
			Board:             board,
			Solution:          sol,
			SolutionDoc:       doc,
			ASCIIPreview:      "x",
		}
	}
	board := NewEmptyBoard(CargoFlowWidth, CargoFlowHeight)
	board.Rules = RulesCargoFlow
	board.ExitCol = CargoFlowExitCol
	board.AddPiece(Piece{Position: 1*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Vertical, Kind: PieceTarget})
	board.AddPiece(Piece{Position: 0*CargoFlowWidth + CargoFlowExitCol, Size: 2, Orientation: Horizontal, Kind: PieceNormal})
	sol := board.SolveWithBudget(DefaultCargoFlowSolveBudget())
	if !sol.Solvable {
		t.Skip("synthetic board unsolvable")
	}
	level, err := LevelJSONFromBoard(board, id, "test")
	if err != nil {
		t.Fatal(err)
	}
	opt := sol.NumMoves
	level.CanonicalGestures = &opt
	doc := ExportSolutionJSON(level, board, sol, 0, true)
	return BoardMixAccepted{
		CandidateID: id, FamilyID: "F_TEST", InventoryClass: InvNo1x1,
		BoardUtil: BoardUtilizationMetrics{BoardShapeClass: ShapeShiftedCore, OuterZoneRelevant: true},
		OptimalGestures: sol.NumMoves, ReplayVerified: true,
		Level: level, Board: board, Solution: sol, SolutionDoc: doc, ASCIIPreview: "x",
	}
}

func TestCheckpointDoesNotPersistSolutionDocRegression(t *testing.T) {
	c := BoardMixAccepted{
		CandidateID: "Candidate_001",
		SolutionDoc: SolutionJSON{SchemaVersion: 1, LevelID: "Candidate_001", Solved: true},
		Solution:    Solution{Solvable: true, NumMoves: 5},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	cp := BoardMixCheckpoint{Version: BoardMixVersion, Accepted: []BoardMixAccepted{c}}
	if err := SaveBoardMixCheckpoint(path, cp); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBoardMixCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accepted[0].SolutionDoc.SchemaVersion != 0 {
		t.Fatal("SolutionDoc must not persist in checkpoint (json:\"-\")")
	}
	if loaded.Accepted[0].Solution.Solvable {
		t.Fatal("Solution must not persist in checkpoint (json:\"-\")")
	}
}

func TestReexportRebuildsMissingSolutionDoc(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	// Simulate checkpoint reload: Level present, SolutionDoc zero.
	c.SolutionDoc = SolutionJSON{}
	c.Solution = Solution{}
	res := BoardMixResult{Accepted: []BoardMixAccepted{c}, Config: BoardMixConfig{SolveTimeLimitMs: 8000, MaxVisitedStates: 1_500_000}}
	if err := WriteBoardMixBatch(dir, res); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc SolutionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !solutionDocIsValid(doc, "Candidate_001") {
		t.Fatalf("rebuilt solution invalid: %+v", doc)
	}
}

func TestZeroSolutionDocNeverExported(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	c.SolutionDoc = SolutionJSON{} // force rebuild path
	c.Solution = Solution{}
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}, Config: BoardMixConfig{SolveTimeLimitMs: 8000, MaxVisitedStates: 1_500_000}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"))
	var doc SolutionJSON
	_ = json.Unmarshal(data, &doc)
	if doc.SchemaVersion == 0 {
		t.Fatal("zero SolutionDoc was exported")
	}
}

func TestSolutionSchemaVersionOne(t *testing.T) {
	c := syntheticExportCandidate(t, "Candidate_001")
	if c.SolutionDoc.SchemaVersion != CargoFlowJSONSchemaVersion {
		t.Fatal(c.SolutionDoc.SchemaVersion)
	}
}

func TestSolutionLevelIdMatchesCandidate(t *testing.T) {
	c := syntheticExportCandidate(t, "Candidate_007")
	if c.SolutionDoc.LevelID != "Candidate_007" {
		t.Fatal(c.SolutionDoc.LevelID)
	}
}

func TestSolutionSolvedOptimal(t *testing.T) {
	c := syntheticExportCandidate(t, "Candidate_001")
	if !c.SolutionDoc.Solved || !c.SolutionDoc.Optimal || c.SolutionDoc.OptimalGestures <= 0 {
		t.Fatalf("%+v", c.SolutionDoc)
	}
}

func TestSolutionReplayVerified(t *testing.T) {
	c := syntheticExportCandidate(t, "Candidate_001")
	if !c.SolutionDoc.ReplayPass || !c.SolutionDoc.ReplayVerified {
		t.Fatal(c.SolutionDoc)
	}
}

func TestSolutionMovesPresent(t *testing.T) {
	c := syntheticExportCandidate(t, "Candidate_001")
	if len(c.SolutionDoc.Moves) == 0 {
		t.Fatal("moves empty")
	}
}

func TestValidatorRejectsSchemaVersionZero(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	bad := SolutionJSON{SchemaVersion: 0, LevelID: "Candidate_001"}
	_ = WriteJSONFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"), bad)
	rep, _ := ValidateUnityBatchReport(dir)
	if rep.OK {
		t.Fatal("expected reject schemaVersion 0")
	}
}

func TestValidatorRejectsEmptySolution(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	_ = WriteJSONFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"), SolutionJSON{
		SchemaVersion: 1, LevelID: "Candidate_001", Solved: true, Optimal: true,
		OptimalGestures: 3, ReplayPass: true, ReplayVerified: true, Moves: nil,
	})
	rep, _ := ValidateUnityBatchReport(dir)
	if rep.OK {
		t.Fatal("empty moves must fail")
	}
}

func TestValidatorRejectsLevelIdMismatch(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	_ = WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}})
	doc := c.SolutionDoc
	doc.LevelID = "Candidate_999"
	_ = WriteJSONFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"), doc)
	rep, _ := ValidateUnityBatchReport(dir)
	if rep.OK {
		t.Fatal("levelId mismatch must fail")
	}
}

func TestValidatorAcceptsKnownGoodRUSH008Solution(t *testing.T) {
	path := "output/RUSH009_CuratedShortlist_001/Solutions/Candidate_001.solution.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("known-good solution missing")
	}
	var doc SolutionJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !solutionDocIsValid(doc, "Candidate_001") {
		t.Fatalf("known-good invalid: %+v", doc)
	}
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
	lf, _ := cands[0]["levelFile"].(string)
	if lf == "" || lf[:10] != "Candidates" {
		t.Fatalf("known-good must use levelFile paths, got %q", lf)
	}
}

func TestNativeFinalCandidateExportsLevelJson(t *testing.T) {
	dir := t.TempDir()
	c := syntheticExportCandidate(t, "Candidate_001")
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: []BoardMixAccepted{c}}); err != nil {
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
	c := syntheticExportCandidate(t, "Candidate_001")
	work := c.Board.Copy()
	if err := work.Replay(c.Solution.Moves); err != nil {
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
}

func TestTwelveFinalCandidatesProduceTwelveLevelJsonPASS(t *testing.T) {
	dir := t.TempDir()
	base := syntheticExportCandidate(t, "Candidate_001")
	accepted := make([]BoardMixAccepted, 12)
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("Candidate_%03d", i+1)
		cc := base
		cc.CandidateID = id
		cc.FamilyID = fmt.Sprintf("F%03d", i+1)
		if cc.Level != nil {
			cp := *cc.Level
			cp.LevelID = id
			cc.Level = &cp
		}
		doc := cc.SolutionDoc
		doc.LevelID = id
		cc.SolutionDoc = doc
		accepted[i] = cc
	}
	if err := WriteBoardMixBatch(dir, BoardMixResult{Accepted: accepted}); err != nil {
		t.Fatal(err)
	}
	rep, err := ValidateUnityBatchReport(dir)
	if err != nil || !rep.OK || rep.LevelJSON != 12 || rep.SolutionJSON != 12 {
		t.Fatalf("%+v err=%v", rep, err)
	}
}

func TestTwelveExistingLevelsReexportWithoutGeneration(t *testing.T) {
	src := "output/RUSH01041_NativeFamilyCoveragePilot_001/Candidates"
	if _, err := os.Stat(filepath.Join(src, "Candidate_001.json")); err != nil {
		t.Skip("RUSH01041 candidates not present")
	}
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "Candidates"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "Solutions"), 0o755)
	// Copy only 2 levels for lightweight fixture (not full 12-minute generation).
	for _, id := range []string{"Candidate_001", "Candidate_002"} {
		in, err := os.ReadFile(filepath.Join(src, id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dir, "Candidates", id+".json"), in, 0o644)
		// Plant broken zero solution like prior reexport.
		_ = WriteJSONFile(filepath.Join(dir, "Solutions", id+".solution.json"), SolutionJSON{})
	}
	// Minimal thin manifest so loadAccepted finds candidates via Candidates/.
	man := ToUnityBatchManifest([]BoardMixAccepted{
		{CandidateID: "Candidate_001"},
		{CandidateID: "Candidate_002"},
	})
	_ = WriteJSONFile(filepath.Join(dir, "BatchManifest.json"), man)

	start := time.Now()
	rep, err := ReexportBoardMixBatchFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK || rep.SolutionJSON != 2 {
		t.Fatalf("%+v", rep)
	}
	if time.Since(start) > 60*time.Second {
		t.Fatalf("reexport fixture too slow: %s", time.Since(start))
	}
	data, _ := os.ReadFile(filepath.Join(dir, "Solutions", "Candidate_001.solution.json"))
	var doc SolutionJSON
	_ = json.Unmarshal(data, &doc)
	if doc.SchemaVersion != 1 || !doc.Solved || len(doc.Moves) == 0 {
		t.Fatalf("solution not rebuilt: %+v", doc)
	}
}

func TestFatManifestWithoutLevelFileFailsValidation(t *testing.T) {
	dir := t.TempDir()
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
