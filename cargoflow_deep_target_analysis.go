package rush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type SourceColumnSummary struct {
	FamilyCount         int         `json:"familyCount"`
	ColumnDistribution  map[int]int `json:"sourceTargetLeftColumnDistribution"`
	DeepCapable5Count   int         `json:"deepCapable5Count"`
	BottomCapable6Count int         `json:"bottomCapable6Count"`
}

type SourceEmbeddingFunnelRow struct {
	FamilyID                    string           `json:"familyId"`
	BaseCandidateID             string           `json:"baseCandidateId"`
	SourcePuzzleID              string           `json:"sourcePuzzleId"`
	SourceTargetLeftColumn      int              `json:"sourceTargetLeftColumn"`
	Embedding                   EmbeddingVariant `json:"embedding"`
	ExpectedTargetTopRow        int              `json:"expectedTargetTopRow"`
	ActualTargetTopRow          int              `json:"actualTargetTopRow,omitempty"`
	TransformResult             string           `json:"transformResult"`
	PlainAttempted              bool             `json:"plainAttempted"`
	PlainExactResult            string           `json:"plainExactResult"`
	PlainOptimal                int              `json:"plainOptimal,omitempty"`
	NativeAttempted             bool             `json:"nativeAttempted"`
	NativePoolCount             int              `json:"nativePoolCount"`
	CoreAttempted               bool             `json:"coreAttempted"`
	CorePoolCount               int              `json:"corePoolCount"`
	CausalAttempted             bool             `json:"causalAttempted"`
	CausalPoolCount             int              `json:"causalPoolCount"`
	FinalPoolInclusion          bool             `json:"finalPoolInclusion"`
	ObservedPoolModes           []string         `json:"observedPoolModes,omitempty"`
	RejectionReason             string           `json:"rejectionReason,omitempty"`
	FamilyRejectionReasonCounts map[string]int   `json:"familyRejectionReasonCounts,omitempty"`
}

type DeepTargetHypothesisVerdict struct {
	Hypothesis string `json:"hypothesis"`
	Verdict    string `json:"verdict"`
	Evidence   string `json:"evidence"`
}

type DeepTargetRoutingAnalysisReport struct {
	SchemaVersion                    int                           `json:"schemaVersion"`
	Shortlist                        SourceColumnSummary           `json:"rush009CuratedShortlist"`
	AttemptedFamilies                SourceColumnSummary           `json:"rush0107AttemptedFamilies"`
	PoolFamilies                     SourceColumnSummary           `json:"rush0107PoolFamilies"`
	CausalFamilies                   SourceColumnSummary           `json:"rush0107CausalFamilies"`
	FullRushRecordColumnDistribution map[int]int                   `json:"fullRushRecordColumnDistribution,omitempty"`
	PoolTargetTopRowDistribution     map[int]int                   `json:"poolTargetTopRowDistribution"`
	CausalTargetTopRowDistribution   map[int]int                   `json:"causalTargetTopRowDistribution"`
	FinalTargetTopRowDistribution    map[int]int                   `json:"finalTargetTopRowDistribution"`
	DeepTargetPoolCount              int                           `json:"deepTargetPoolCount"`
	DeepTargetPoolUniqueFamilies     int                           `json:"deepTargetPoolUniqueFamilies"`
	BottomTargetPoolCount            int                           `json:"bottomTargetPoolCount"`
	BottomTargetPoolUniqueFamilies   int                           `json:"bottomTargetPoolUniqueFamilies"`
	SourceColumnEmbeddingFunnel      []SourceEmbeddingFunnelRow    `json:"sourceColumnEmbeddingFunnel"`
	RejectionReasonsByTargetRow      map[int]map[string]int        `json:"rejectionReasonsByTargetRow"`
	HypothesisVerdicts               []DeepTargetHypothesisVerdict `json:"hypothesisVerdicts"`
	DeepSourcesSufficient            bool                          `json:"deepSourcesSufficient"`
	RootCause                        string                        `json:"rootCause"`
	RecommendedAction                string                        `json:"recommendedAction"`
	Limitations                      []string                      `json:"limitations"`
}

func SourceTargetLeftColumn(originalBoard string) (int, error) {
	first := strings.IndexByte(originalBoard, 'A')
	if first < 0 {
		return 0, fmt.Errorf("target A not found")
	}
	second := strings.IndexByte(originalBoard[first+1:], 'A')
	if second < 0 {
		return 0, fmt.Errorf("target A must have size 2")
	}
	second += first + 1
	if first/RushDBWidth != second/RushDBWidth || second != first+1 {
		return 0, fmt.Errorf("target A is not horizontal contiguous")
	}
	return first % RushDBWidth, nil
}

func ExpectedTargetTopRow(sourceTargetLeftColumn int, embedding EmbeddingVariant) (int, error) {
	offset, err := embeddingOffsetY(embedding)
	if err != nil {
		return 0, err
	}
	if embeddingMirrorH(embedding) {
		sourceTargetLeftColumn = RushDBWidth - 2 - sourceTargetLeftColumn
	}
	return RushDBWidth - 2 - sourceTargetLeftColumn + offset, nil
}

func AnalyzeDeepTargetRouting(shortlistDir, checkpointPath, databasePath, outputDir string, budget SolveBudget) (DeepTargetRoutingAnalysisReport, error) {
	report := DeepTargetRoutingAnalysisReport{
		SchemaVersion:                  1,
		PoolTargetTopRowDistribution:   map[int]int{},
		CausalTargetTopRowDistribution: map[int]int{},
		FinalTargetTopRowDistribution:  map[int]int{},
		RejectionReasonsByTargetRow:    map[int]map[string]int{},
		Limitations: []string{
			"Historical checkpoint stores rejection counts by family, not by individual embedding/mode; exact per-attempt rejection reasons can only be reported where a pool hit or exact plain recheck identifies the outcome.",
			"Native/core/causal stages are not rerun for unattempted deep embeddings during this analysis.",
			"Historical BoardSpace.TargetTopRow is stale on some inventory-enriched pool entries; distributions are recomputed from level Target geometry and cross-checked against source-column transform math.",
		},
	}
	if budget.TimeLimit <= 0 {
		budget = DefaultCargoFlowSolveBudget()
	}
	bases, err := SelectEnrichmentBases(EnrichmentConfig{
		BatchDir: shortlistDir, CuratorReportPath: filepath.Join(shortlistDir, "CuratorReport.json"),
		BaseCount: 10_000, SolveTimeLimit: budget.TimeLimit, MaxVisitedStates: budget.MaxVisited,
	})
	if err != nil {
		return report, err
	}
	checkpoint, err := LoadBoardMixCheckpoint(checkpointPath)
	if err != nil {
		return report, err
	}
	baseByID := map[string]EnrichmentBase{}
	colByFamily := map[string]int{}
	for _, base := range bases {
		baseByID[base.CandidateID] = base
		if base.Level == nil || base.Level.Transplant == nil {
			continue
		}
		col, err := SourceTargetLeftColumn(base.Level.Transplant.OriginalBoard)
		if err != nil {
			return report, fmt.Errorf("%s: %w", base.CandidateID, err)
		}
		colByFamily[base.FamilyID] = col
	}
	attempted := map[string]bool{}
	attemptedFamilies := map[string]bool{}
	for _, key := range checkpoint.CompletedAttemptKeys {
		attempted[key] = true
		parts := strings.Split(key, "|")
		if len(parts) == 3 {
			if base, ok := baseByID[parts[0]]; ok {
				attemptedFamilies[base.FamilyID] = true
			}
		}
	}
	poolFamilies, causalFamilies := map[string]bool{}, map[string]bool{}
	poolIndex := map[string][]BoardMixAccepted{}
	for _, candidate := range checkpoint.Pool {
		poolFamilies[candidate.FamilyID] = true
		if IsCausalExpanded(candidate) {
			causalFamilies[candidate.FamilyID] = true
		}
		mode := boardMixCandidateMode(candidate)
		key := boardMixJobKey(candidate.BaseCandidateID, candidate.Embedding, mode)
		poolIndex[key] = append(poolIndex[key], candidate)
		row := candidateTargetTopRow(candidate)
		report.PoolTargetTopRowDistribution[row]++
		if IsCausalExpanded(candidate) {
			report.CausalTargetTopRowDistribution[row]++
		}
	}
	deepPoolFamilies, bottomPoolFamilies := map[string]bool{}, map[string]bool{}
	for _, candidate := range checkpoint.Pool {
		row := candidateTargetTopRow(candidate)
		if row >= 5 {
			report.DeepTargetPoolCount++
			deepPoolFamilies[candidate.FamilyID] = true
		}
		if row == 6 {
			report.BottomTargetPoolCount++
			bottomPoolFamilies[candidate.FamilyID] = true
		}
	}
	report.DeepTargetPoolUniqueFamilies = len(deepPoolFamilies)
	report.BottomTargetPoolUniqueFamilies = len(bottomPoolFamilies)
	for _, candidate := range checkpoint.Accepted {
		report.FinalTargetTopRowDistribution[candidateTargetTopRow(candidate)]++
	}
	allFamilies := map[string]bool{}
	for family := range colByFamily {
		allFamilies[family] = true
	}
	report.Shortlist = summarizeSourceColumns(allFamilies, colByFamily)
	report.AttemptedFamilies = summarizeSourceColumns(attemptedFamilies, colByFamily)
	report.PoolFamilies = summarizeSourceColumns(poolFamilies, colByFamily)
	report.CausalFamilies = summarizeSourceColumns(causalFamilies, colByFamily)

	if databasePath != "" {
		fullDist := map[int]int{}
		_, streamErr := StreamRushDBFile(databasePath, datasetTagFromPath(databasePath), func(rec RushDBRecord) error {
			if col, colErr := SourceTargetLeftColumn(rec.Board36); colErr == nil {
				fullDist[col]++
			}
			return nil
		})
		if streamErr != nil {
			return report, streamErr
		}
		report.FullRushRecordColumnDistribution = fullDist
	}

	familyRejects := map[string]map[string]int{}
	for family := range colByFamily {
		familyRejects[family] = map[string]int{}
	}
	if data, readErr := os.ReadFile(filepath.Join(filepath.Dir(checkpointPath), "BoardDiversityReport.json")); readErr == nil {
		var historical struct {
			FamilyCoverage FamilyCoverageReport `json:"familyCoverage"`
		}
		if unmarshalErr := json.Unmarshal(data, &historical); unmarshalErr == nil {
			for _, entry := range historical.FamilyCoverage.Funnel {
				familyRejects[entry.BaseFamilyID] = copyIntMap(entry.RejectReasonCounts)
			}
		}
	}
	analyzedEmbeddings := []EmbeddingVariant{
		EmbedFlushTop, EmbedShiftDown1, EmbedFlushBottom,
		EmbedFlushTopMirrorH, EmbedShiftDown1MirrorH,
	}
	for _, base := range bases {
		col := colByFamily[base.FamilyID]
		if col > 1 {
			continue
		}
		for _, embedding := range analyzedEmbeddings {
			expected, _ := ExpectedTargetTopRow(col, embedding)
			row := SourceEmbeddingFunnelRow{
				FamilyID: base.FamilyID, BaseCandidateID: base.CandidateID,
				SourcePuzzleID: base.SourcePuzzleID, SourceTargetLeftColumn: col,
				Embedding: embedding, ExpectedTargetTopRow: expected,
				TransformResult: "NotChecked", PlainExactResult: "NotChecked",
				FamilyRejectionReasonCounts: familyRejects[base.FamilyID],
			}
			row.PlainAttempted = attempted[boardMixJobKey(base.CandidateID, embedding, "plain")]
			row.NativeAttempted = attempted[boardMixJobKey(base.CandidateID, embedding, "native")]
			row.CoreAttempted = attempted[boardMixJobKey(base.CandidateID, embedding, "corexpand")]
			row.CausalAttempted = attempted[boardMixJobKey(base.CandidateID, embedding, "causal")]
			plain, reason, _ := evaluateBoardMixJob(base, embedding, false, budget, nil, "")
			if plain == nil {
				row.TransformResult = transformResultForReason(reason)
				row.PlainExactResult = reason
				row.RejectionReason = reason
			} else {
				row.TransformResult = "Success"
				row.PlainExactResult = "Solved"
				row.PlainOptimal = plain.OptimalGestures
				row.ActualTargetTopRow = plain.Board.Pieces[0].Row(plain.Board.Width)
				if row.ActualTargetTopRow != row.ExpectedTargetTopRow {
					row.RejectionReason = "ExpectedActualTargetRowMismatch"
				}
			}
			for _, mode := range []string{"plain", "native", "corexpand", "causal"} {
				key := boardMixJobKey(base.CandidateID, embedding, mode)
				hits := poolIndex[key]
				if len(hits) > 0 {
					row.ObservedPoolModes = append(row.ObservedPoolModes, mode)
					row.FinalPoolInclusion = true
				}
				switch mode {
				case "native":
					row.NativePoolCount = len(hits)
				case "corexpand":
					row.CorePoolCount = len(hits)
				case "causal":
					row.CausalPoolCount = len(hits)
				}
			}
			if !row.FinalPoolInclusion && row.RejectionReason == "" {
				if !row.PlainAttempted && !row.NativeAttempted && !row.CoreAttempted && !row.CausalAttempted {
					row.RejectionReason = "NotScheduled"
				} else {
					row.RejectionReason = "AttemptedNoPoolCandidate"
				}
			}
			if report.RejectionReasonsByTargetRow[expected] == nil {
				report.RejectionReasonsByTargetRow[expected] = map[string]int{}
			}
			if row.RejectionReason != "" {
				report.RejectionReasonsByTargetRow[expected][row.RejectionReason]++
			}
			report.SourceColumnEmbeddingFunnel = append(report.SourceColumnEmbeddingFunnel, row)
		}
	}
	sort.Slice(report.SourceColumnEmbeddingFunnel, func(i, j int) bool {
		a, b := report.SourceColumnEmbeddingFunnel[i], report.SourceColumnEmbeddingFunnel[j]
		if a.SourceTargetLeftColumn != b.SourceTargetLeftColumn {
			return a.SourceTargetLeftColumn < b.SourceTargetLeftColumn
		}
		if a.FamilyID != b.FamilyID {
			return a.FamilyID < b.FamilyID
		}
		return a.ExpectedTargetTopRow < b.ExpectedTargetTopRow
	})
	finalizeDeepTargetAnalysis(&report)
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return report, err
		}
		if err := WriteJSONFile(filepath.Join(outputDir, "DeepTargetRoutingAnalysis.json"), report); err != nil {
			return report, err
		}
		if err := os.WriteFile(filepath.Join(outputDir, "DeepTargetRoutingAnalysis.md"),
			[]byte(renderDeepTargetAnalysisMarkdown(report)), 0o644); err != nil {
			return report, err
		}
	}
	return report, nil
}

func summarizeSourceColumns(families map[string]bool, colByFamily map[string]int) SourceColumnSummary {
	summary := SourceColumnSummary{ColumnDistribution: map[int]int{}}
	for family := range families {
		col, ok := colByFamily[family]
		if !ok {
			continue
		}
		summary.FamilyCount++
		summary.ColumnDistribution[col]++
		if col <= 1 {
			summary.DeepCapable5Count++
		}
		if col == 0 {
			summary.BottomCapable6Count++
		}
	}
	return summary
}

func boardMixCandidateMode(candidate BoardMixAccepted) string {
	switch {
	case candidate.CausalProof != nil:
		return "causal"
	case candidate.CoreExpansionMeta != nil || candidate.CoreExpansionClass != "":
		return "corexpand"
	case candidate.NativeMeta != nil:
		return "native"
	default:
		return "plain"
	}
}

func candidateTargetTopRow(candidate BoardMixAccepted) int {
	if candidate.TargetTopRow > 0 {
		return candidate.TargetTopRow
	}
	if candidate.Level != nil {
		if board, err := BoardFromLevelJSON(candidate.Level); err == nil && len(board.Pieces) > 0 {
			return board.Pieces[0].Row(board.Width)
		}
		if candidate.Level.BoardSpace != nil {
			return candidate.Level.BoardSpace.TargetTopRow
		}
	}
	if candidate.Board != nil && len(candidate.Board.Pieces) > 0 {
		return candidate.Board.Pieces[0].Row(candidate.Board.Width)
	}
	return 0
}

func transformResultForReason(reason string) string {
	switch reason {
	case "TransformFailed", "MissingTransplant", "MissingSourceBoard":
		return "Failed"
	default:
		return "Success"
	}
}

func finalizeDeepTargetAnalysis(report *DeepTargetRoutingAnalysisReport) {
	deep := report.Shortlist.DeepCapable5Count
	bottom := report.Shortlist.BottomCapable6Count
	report.DeepSourcesSufficient = deep >= 5 && bottom >= 2
	deepRows, solvedDeepRows, attemptedDeepRows := 0, 0, 0
	for _, row := range report.SourceColumnEmbeddingFunnel {
		if row.ExpectedTargetTopRow < 5 {
			continue
		}
		deepRows++
		if row.PlainExactResult == "Solved" {
			solvedDeepRows++
		}
		if row.PlainAttempted || row.NativeAttempted || row.CoreAttempted || row.CausalAttempted {
			attemptedDeepRows++
		}
	}
	h1 := "REJECTED"
	if !report.DeepSourcesSufficient {
		h1 = "SUPPORTED"
	} else if deep < 8 || bottom < 3 {
		h1 = "WEAK"
	}
	h2 := "REJECTED"
	if deepRows > 0 && attemptedDeepRows == 0 {
		h2 = "SUPPORTED"
	} else if attemptedDeepRows*2 < deepRows {
		h2 = "WEAK"
	}
	h3 := "REJECTED"
	if deepRows > 0 && solvedDeepRows*2 < deepRows {
		h3 = "SUPPORTED"
	} else if solvedDeepRows < deepRows {
		h3 = "WEAK"
	}
	h4 := "WEAK"
	if attemptedDeepRows > 0 && solvedDeepRows > 0 && report.DeepTargetPoolCount == 0 {
		h4 = "SUPPORTED"
	}
	h5 := "WEAK"
	shortDeepRate := safeDiv(float64(deep), float64(report.Shortlist.FamilyCount))
	fullDeep := report.FullRushRecordColumnDistribution[0] + report.FullRushRecordColumnDistribution[1]
	fullTotal := 0
	for _, count := range report.FullRushRecordColumnDistribution {
		fullTotal += count
	}
	fullDeepRate := safeDiv(float64(fullDeep), float64(fullTotal))
	if fullTotal > 0 {
		if shortDeepRate+0.10 < fullDeepRate {
			h5 = "SUPPORTED"
		} else if shortDeepRate >= fullDeepRate-0.05 {
			h5 = "REJECTED"
		}
	}
	report.HypothesisVerdicts = []DeepTargetHypothesisVerdict{
		{Hypothesis: "H1 RUSH009 has almost no col0/1 families", Verdict: h1,
			Evidence: fmt.Sprintf("RUSH009: col0/1=%d/%d families; col0=%d. Sufficiency threshold is >=5 deep-capable and >=2 bottom-capable.", deep, report.Shortlist.FamilyCount, bottom)},
		{Hypothesis: "H2 scheduling/pool cap skips deep embeddings", Verdict: h2,
			Evidence: fmt.Sprintf("Deep-capable source×embedding rows >=5: %d; any historical attempt: %d.", deepRows, attemptedDeepRows)},
		{Hypothesis: "H3 deep transforms are unsolvable/trivial", Verdict: h3,
			Evidence: fmt.Sprintf("Exact plain recheck solved %d/%d natural row5/6 transforms.", solvedDeepRows, deepRows)},
		{Hypothesis: "H4 base transforms work but later stages reject", Verdict: h4,
			Evidence: fmt.Sprintf("Solved deep plain transforms=%d; historically attempted deep rows=%d; deep pool candidates=%d.", solvedDeepRows, attemptedDeepRows, report.DeepTargetPoolCount)},
		{Hypothesis: "H5 curator biases source target toward col2/3", Verdict: h5,
			Evidence: fmt.Sprintf("Deep-capable share: RUSH009 %.3f vs full Rush records %.3f.", shortDeepRate, fullDeepRate)},
	}
	if report.DeepSourcesSufficient && solvedDeepRows >= 5 {
		report.RootCause = "Natural deep-capable Rush seeds exist and their row5/6 transforms solve, but RUSH-010.7 family-first pass attempted only the first configured embedding before MaxPoolSize early-stop. Deep embeddings never entered the pool."
		report.RecommendedAction = "Implement source-aware embedding routing: make a deep embedding the primary pass-1 route for col0/1 sources, then select actual TargetTopRow quotas while preserving causal proof requirements."
	} else if !report.DeepSourcesSufficient {
		report.RootCause = "RUSH009 shortlist does not contain enough unique col0/1 source families for the requested row5/6 final mix."
		report.RecommendedAction = "Stratify the RUSH009 curator by SourceTargetLeftColumn before changing Target placement."
	} else {
		report.RootCause = "Deep-capable sources exist, but too few natural row5/6 transforms pass exact Cargo solve."
		report.RecommendedAction = "Expand source search in the full Rush database using SourceTargetLeftColumn stratification; do not relocate Target manually."
	}
}

func renderDeepTargetAnalysisMarkdown(report DeepTargetRoutingAnalysisReport) string {
	var b strings.Builder
	b.WriteString("# RUSH-010.7.1 Deep Target Routing Analysis\n\n")
	b.WriteString("## Root cause\n\n" + report.RootCause + "\n\n")
	b.WriteString("## Source-column summaries\n\n")
	b.WriteString("| Set | Families | col0 | col1 | col2 | col3 | DeepCapable5 | BottomCapable6 |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, row := range []struct {
		name string
		data SourceColumnSummary
	}{
		{"RUSH009 shortlist", report.Shortlist},
		{"RUSH0107 attempted", report.AttemptedFamilies},
		{"RUSH0107 pool", report.PoolFamilies},
		{"RUSH0107 causal", report.CausalFamilies},
	} {
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %d |\n",
			row.name, row.data.FamilyCount, row.data.ColumnDistribution[0],
			row.data.ColumnDistribution[1], row.data.ColumnDistribution[2],
			row.data.ColumnDistribution[3], row.data.DeepCapable5Count,
			row.data.BottomCapable6Count))
	}
	b.WriteString("\n## Target rows\n\n")
	b.WriteString("- Pool: " + formatIntDistribution(report.PoolTargetTopRowDistribution) + "\n")
	b.WriteString("- Causal: " + formatIntDistribution(report.CausalTargetTopRowDistribution) + "\n")
	b.WriteString("- Final: " + formatIntDistribution(report.FinalTargetTopRowDistribution) + "\n\n")
	b.WriteString("## Hypotheses\n\n")
	for _, h := range report.HypothesisVerdicts {
		b.WriteString(fmt.Sprintf("- **%s** — %s: %s\n", h.Verdict, h.Hypothesis, h.Evidence))
	}
	b.WriteString("\n## Deep-capable source × embedding funnel\n\n")
	b.WriteString("| Family | Source col | Embedding | Expected row | Attempted P/N/C/X | Plain exact | Optimal | Pool modes | Rejection |\n")
	b.WriteString("|---|---:|---|---:|---|---|---:|---|---|\n")
	for _, row := range report.SourceColumnEmbeddingFunnel {
		b.WriteString(fmt.Sprintf("| %s | %d | %s | %d | %t/%t/%t/%t | %s | %d | %s | %s |\n",
			row.FamilyID, row.SourceTargetLeftColumn, row.Embedding,
			row.ExpectedTargetTopRow, row.PlainAttempted, row.NativeAttempted,
			row.CoreAttempted, row.CausalAttempted, row.PlainExactResult,
			row.PlainOptimal, strings.Join(row.ObservedPoolModes, ","),
			row.RejectionReason))
	}
	b.WriteString("\n## Recommended action\n\n" + report.RecommendedAction + "\n")
	return b.String()
}

func formatIntDistribution(dist map[int]int) string {
	keys := make([]int, 0, len(dist))
	for key := range dist {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, strconv.Itoa(key)+"="+strconv.Itoa(dist[key]))
	}
	return strings.Join(parts, ", ")
}
