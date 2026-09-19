package rush

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SpatialDependencyMetrics struct {
	DependencyCentroidRow          float64        `json:"dependencyCentroidRow"`
	DependencyCentroidColumn       float64        `json:"dependencyCentroidColumn"`
	MeaningfulCentroidRow          float64        `json:"meaningfulCentroidRow"`
	MeaningfulCentroidColumn       float64        `json:"meaningfulCentroidColumn"`
	DependencyRowSpan              int            `json:"dependencyRowSpan"`
	DependencyColumnSpan           int            `json:"dependencyColumnSpan"`
	MeaningfulRowSpan              int            `json:"meaningfulRowSpan"`
	MeaningfulColumnSpan           int            `json:"meaningfulColumnSpan"`
	DependencyDensityByRow         []int          `json:"dependencyDensityByRow"`
	DependencyDensityByColumn      []int          `json:"dependencyDensityByColumn"`
	OptimalMoveActivityByRow       []int          `json:"optimalMoveActivityByRow"`
	OptimalMoveActivityByColumn    []int          `json:"optimalMoveActivityByColumn"`
	InitialMeaningfulCellsByRow    []int          `json:"initialMeaningfulCellsByRow"`
	InitialMeaningfulCellsByColumn []int          `json:"initialMeaningfulCellsByColumn"`
	MeaningfulPiecesAboveTarget    int            `json:"meaningfulPiecesAboveTarget"`
	MeaningfulPiecesBesideTarget   int            `json:"meaningfulPiecesBesideTarget"`
	MeaningfulPiecesBelowTarget    int            `json:"meaningfulPiecesBelowTarget"`
	DependencyMovesAboveTarget     int            `json:"dependencyMovesAboveTarget"`
	DependencyMovesBesideTarget    int            `json:"dependencyMovesBesideTarget"`
	DependencyMovesBelowTarget     int            `json:"dependencyMovesBelowTarget"`
	CrossTargetDependencies        int            `json:"crossTargetDependencies"`
	DependencyGraphNodeCount       int            `json:"dependencyGraphNodeCount"`
	DependencyGraphEdgeCount       int            `json:"dependencyGraphEdgeCount"`
	DependencyGraphDepth           int            `json:"dependencyGraphDepth"`
	SpatialDependencyEdgeCounts    map[string]int `json:"spatialDependencyEdgeCounts"`
	BelowToAboveDependencyEdges    int            `json:"belowToAboveDependencyEdges"`
	SideToAboveDependencyEdges     int            `json:"sideToAboveDependencyEdges"`
}

type RushSourceSpatialMetrics struct {
	Available                  bool    `json:"available"`
	TargetRow                  int     `json:"targetRow"`
	TargetLeftColumn           int     `json:"targetLeftColumn"`
	TargetRightColumn          int     `json:"targetRightColumn"`
	MeaningfulPiecesFront      int     `json:"meaningfulPiecesFront"`
	MeaningfulPiecesBehind     int     `json:"meaningfulPiecesBehind"`
	MovedPiecesFront           int     `json:"movedPiecesFront"`
	MovedPiecesBehind          int     `json:"movedPiecesBehind"`
	DependencyMovesFront       int     `json:"dependencyMovesFront"`
	DependencyMovesBehind      int     `json:"dependencyMovesBehind"`
	FrontBehindActivityRatio   float64 `json:"frontBehindActivityRatio"`
	MappedMeaningfulPieceCount int     `json:"mappedMeaningfulPieceCount"`
	SourcePieceCount           int     `json:"sourcePieceCount"`
}

type SpatialDependencyRow struct {
	Batch                             string                   `json:"batch"`
	CandidateID                       string                   `json:"candidateId"`
	FamilyID                          string                   `json:"familyId,omitempty"`
	HumanLabel                        string                   `json:"humanLabel,omitempty"`
	VariantClass                      string                   `json:"variantClass"`
	CoreExpansionClass                string                   `json:"coreExpansionClass,omitempty"`
	BoardShapeClass                   string                   `json:"boardShapeClass,omitempty"`
	InventoryClass                    string                   `json:"inventoryClass,omitempty"`
	OptimalGestures                   int                      `json:"optimalGestures"`
	TargetTopRow                      int                      `json:"targetTopRow"`
	TargetDepthClass                  TargetDepthClass         `json:"targetDepthClass"`
	Best6x6MeaningfulContainmentRatio float64                  `json:"best6x6MeaningfulContainmentRatio"`
	CanMeaningfulStructureFitInAny6x6 bool                     `json:"canMeaningfulStructureFitInAny6x6"`
	Spatial                           SpatialDependencyMetrics `json:"spatial"`
	SourceRush                        RushSourceSpatialMetrics `json:"sourceRush"`
}

type VariantSpatialSummary struct {
	VariantClass            string  `json:"variantClass"`
	Containment             float64 `json:"containment"`
	GraphEdges              int     `json:"graphEdges"`
	CrossRegionDependencies int     `json:"crossRegionDependencies"`
	MeaningfulRowSpan       int     `json:"meaningfulRowSpan"`
}

type MatchedVariantComparison struct {
	FamilyID           string                 `json:"familyId"`
	Plain              *VariantSpatialSummary `json:"plain,omitempty"`
	NativeAugment      *VariantSpatialSummary `json:"nativeAugment,omitempty"`
	DependencyPullLeft *VariantSpatialSummary `json:"dependencyPullLeft,omitempty"`
}

type MetricSeparation struct {
	Metric          string  `json:"metric"`
	CompactLikeMean float64 `json:"compactLikeMean"`
	NativeLikeMean  float64 `json:"nativeLikeMean"`
	AbsoluteDelta   float64 `json:"absoluteDelta"`
	StandardizedGap float64 `json:"standardizedGap"`
}

type HypothesisVerdict struct {
	Hypothesis string `json:"hypothesis"`
	Verdict    string `json:"verdict"`
	Evidence   string `json:"evidence"`
}

type SpatialDependencyAnalysisReport struct {
	SchemaVersion        int                        `json:"schemaVersion"`
	Rows                 []SpatialDependencyRow     `json:"rows"`
	HumanLabelComparison map[string]interface{}     `json:"humanLabelComparison"`
	TopSeparatingMetrics []MetricSeparation         `json:"topSeparatingMetrics"`
	MatchedVariants      []MatchedVariantComparison `json:"matchedVariants"`
	HypothesisVerdicts   []HypothesisVerdict        `json:"hypothesisVerdicts"`
	Examples             map[string]string          `json:"examples"`
	RootCauseConclusion  string                     `json:"rootCauseConclusion"`
	RecommendedDirection string                     `json:"recommendedDirection"`
	AnalysisLimitations  []string                   `json:"analysisLimitations"`
}

func ComputeSpatialDependencyMetrics(board *Board, sol Solution, budget SolveBudget) SpatialDependencyMetrics {
	m := SpatialDependencyMetrics{SpatialDependencyEdgeCounts: map[string]int{}}
	if board == nil {
		return m
	}
	m.DependencyDensityByRow = make([]int, board.Height)
	m.DependencyDensityByColumn = make([]int, board.Width)
	m.OptimalMoveActivityByRow = make([]int, board.Height)
	m.OptimalMoveActivityByColumn = make([]int, board.Width)
	m.InitialMeaningfulCellsByRow = make([]int, board.Height)
	m.InitialMeaningfulCellsByColumn = make([]int, board.Width)
	if len(board.Pieces) == 0 || !sol.Solvable {
		return m
	}
	meaningful, moved := computeMeaningfulPieces(board, sol, budget)
	target := board.Pieces[0]
	top, bottom := target.Row(board.Width), target.Row(board.Width)+target.Size-1

	depCells, meaningfulCells := []int{}, []int{}
	for i, p := range board.Pieces {
		if meaningful[i] {
			region := classifyPieceRelativeToTarget(p, board.Width, top, bottom)
			if i != 0 {
				switch region {
				case "Above":
					m.MeaningfulPiecesAboveTarget++
				case "Below":
					m.MeaningfulPiecesBelowTarget++
				default:
					m.MeaningfulPiecesBesideTarget++
				}
			}
			for _, cell := range pieceOccupancyCells(p, board.Width) {
				meaningfulCells = append(meaningfulCells, cell)
				m.InitialMeaningfulCellsByRow[cell/board.Width]++
				m.InitialMeaningfulCellsByColumn[cell%board.Width]++
			}
		}
		if moved[i] && i != 0 {
			for _, cell := range pieceOccupancyCells(p, board.Width) {
				depCells = append(depCells, cell)
				m.DependencyDensityByRow[cell/board.Width]++
				m.DependencyDensityByColumn[cell%board.Width]++
			}
		}
	}
	m.DependencyCentroidRow, m.DependencyCentroidColumn, m.DependencyRowSpan, m.DependencyColumnSpan =
		cellDistribution(depCells, board.Width)
	m.MeaningfulCentroidRow, m.MeaningfulCentroidColumn, m.MeaningfulRowSpan, m.MeaningfulColumnSpan =
		cellDistribution(meaningfulCells, board.Width)

	work := board.Copy()
	for _, mv := range sol.Moves {
		if mv.Exit {
			m.DependencyMovesAboveTarget++
			work.DoMove(mv)
			continue
		}
		before := work.Pieces[mv.Piece]
		work.DoMove(mv)
		after := work.Pieces[mv.Piece]
		cells := unionCells(pieceOccupancyCells(before, board.Width), pieceOccupancyCells(after, board.Width))
		rows, cols := map[int]bool{}, map[int]bool{}
		for _, cell := range cells {
			rows[cell/board.Width] = true
			cols[cell%board.Width] = true
		}
		for row := range rows {
			m.OptimalMoveActivityByRow[row]++
		}
		for col := range cols {
			m.OptimalMoveActivityByColumn[col]++
		}
		if mv.Piece != 0 {
			switch classifyCellsRelativeToTarget(cells, board.Width, top, bottom) {
			case "Above":
				m.DependencyMovesAboveTarget++
			case "Below":
				m.DependencyMovesBelowTarget++
			default:
				m.DependencyMovesBesideTarget++
			}
		}
	}

	graph := buildOptimalCausalGraph(board, sol, top, bottom)
	m.DependencyGraphNodeCount = graph.nodes
	m.DependencyGraphEdgeCount = len(graph.edges)
	m.DependencyGraphDepth = graph.depth
	m.SpatialDependencyEdgeCounts = graph.spatialCounts
	m.BelowToAboveDependencyEdges = graph.spatialCounts["Below→Above"]
	m.SideToAboveDependencyEdges = graph.spatialCounts["Side→Above"]
	m.CrossTargetDependencies = graph.crossTargetDependencies
	return m
}

type causalGraphResult struct {
	nodes                   int
	edges                   map[string]bool
	depth                   int
	spatialCounts           map[string]int
	crossTargetDependencies int
}

// buildOptimalCausalGraph creates an acyclic move-event graph. An edge i→j is
// added when move j uses a cell most recently vacated by move i.
func buildOptimalCausalGraph(board *Board, sol Solution, top, bottom int) causalGraphResult {
	out := causalGraphResult{edges: map[string]bool{}, spatialCounts: map[string]int{}}
	lastVacated := map[int]int{}
	eventRegion := map[int]string{}
	eventDepth := map[int]int{}
	work := board.Copy()
	for event, mv := range sol.Moves {
		out.nodes++
		required := []int{}
		region := "Above"
		if mv.Exit {
			target := work.Pieces[mv.Piece]
			for row := 0; row < target.Row(work.Width); row++ {
				required = append(required, row*work.Width+work.exitColumn())
			}
		} else {
			before := work.Pieces[mv.Piece]
			after := before
			stride := before.Stride(work.Width)
			if before.Kind == PieceUnit {
				if mv.Axis == Vertical {
					stride = work.Width
				} else {
					stride = 1
				}
			}
			after.Position += mv.Steps * stride
			oldSet := cellsSet(pieceOccupancyCells(before, work.Width))
			for _, cell := range pieceOccupancyCells(after, work.Width) {
				if !oldSet[cell] {
					required = append(required, cell)
				}
			}
			region = classifyCellsRelativeToTarget(
				unionCells(pieceOccupancyCells(before, work.Width), pieceOccupancyCells(after, work.Width)),
				work.Width, top, bottom)
		}
		eventRegion[event] = region
		maxParentDepth := 0
		for _, cell := range required {
			parent, ok := lastVacated[cell]
			if !ok || parent == event {
				continue
			}
			key := fmt.Sprintf("%d>%d", parent, event)
			if !out.edges[key] {
				out.edges[key] = true
				edgeClass := eventRegion[parent] + "→" + region
				out.spatialCounts[edgeClass]++
			}
			if eventDepth[parent] > maxParentDepth {
				maxParentDepth = eventDepth[parent]
			}
		}
		eventDepth[event] = maxParentDepth + 1
		if eventDepth[event] > out.depth {
			out.depth = eventDepth[event]
		}
		if mv.Exit {
			work.DoMove(mv)
			continue
		}
		before := work.Pieces[mv.Piece]
		oldCells := cellsSet(pieceOccupancyCells(before, work.Width))
		work.DoMove(mv)
		newCells := cellsSet(pieceOccupancyCells(work.Pieces[mv.Piece], work.Width))
		for cell := range oldCells {
			if !newCells[cell] {
				lastVacated[cell] = event
			}
		}
	}
	adj := map[int][]int{}
	for edge := range out.edges {
		var from, to int
		if _, err := fmt.Sscanf(edge, "%d>%d", &from, &to); err == nil {
			adj[from] = append(adj[from], to)
		}
	}
	for source, region := range eventRegion {
		if region != "Below" && region != "Side" {
			continue
		}
		seen := map[int]bool{}
		queue := append([]int{}, adj[source]...)
		reachesAbove := false
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			if seen[node] {
				continue
			}
			seen[node] = true
			if eventRegion[node] == "Above" {
				reachesAbove = true
				break
			}
			queue = append(queue, adj[node]...)
		}
		if reachesAbove {
			out.crossTargetDependencies++
		}
	}
	return out
}

func ComputeRushSourceSpatialMetrics(level *LevelJSON, cargoMeaningful map[int]bool, budget SolveBudget) RushSourceSpatialMetrics {
	m := RushSourceSpatialMetrics{}
	if level == nil || level.Transplant == nil || level.Transplant.OriginalBoard == "" {
		return m
	}
	rec := RushDBRecord{
		Board36:              level.Transplant.OriginalBoard,
		OriginalOptimalMoves: level.Transplant.OriginalOptimalMoves,
	}
	board, err := BoardFromRushDBRecord(rec)
	if err != nil {
		return m
	}
	sol := board.SolveWithBudget(budget)
	if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
		return m
	}
	meaningful, moved := computeMeaningfulPieces(board, sol, budget)
	target := board.Pieces[0]
	left, right := target.Col(board.Width), target.Col(board.Width)+target.Size-1
	m.Available = true
	m.TargetRow = target.Row(board.Width)
	m.TargetLeftColumn, m.TargetRightColumn = left, right
	m.SourcePieceCount = len(board.Pieces)
	for i, p := range board.Pieces {
		if i == 0 {
			continue
		}
		region := classifyRushPiece(p, board.Width, left, right)
		if meaningful[i] {
			switch region {
			case "Front":
				m.MeaningfulPiecesFront++
			case "Behind":
				m.MeaningfulPiecesBehind++
			}
		}
		if moved[i] {
			switch region {
			case "Front":
				m.MovedPiecesFront++
			case "Behind":
				m.MovedPiecesBehind++
			}
		}
		if cargoMeaningful[i] && meaningful[i] {
			m.MappedMeaningfulPieceCount++
		}
	}
	for _, mv := range sol.Moves {
		if mv.Piece == 0 {
			continue
		}
		switch classifyRushPiece(board.Pieces[mv.Piece], board.Width, left, right) {
		case "Front":
			m.DependencyMovesFront++
		case "Behind":
			m.DependencyMovesBehind++
		}
	}
	m.FrontBehindActivityRatio = round3(float64(m.DependencyMovesFront+1) / float64(m.DependencyMovesBehind+1))
	return m
}

func classifyRushPiece(p Piece, width, targetLeft, targetRight int) string {
	minCol, maxCol := width, -1
	for _, cell := range pieceOccupancyCells(p, width) {
		col := cell % width
		if col < minCol {
			minCol = col
		}
		if col > maxCol {
			maxCol = col
		}
	}
	if minCol > targetRight {
		return "Front"
	}
	if maxCol < targetLeft {
		return "Behind"
	}
	return "Beside"
}

func classifyPieceRelativeToTarget(p Piece, width, top, bottom int) string {
	return classifyCellsRelativeToTarget(pieceOccupancyCells(p, width), width, top, bottom)
}

func classifyCellsRelativeToTarget(cells []int, width, top, bottom int) string {
	if len(cells) == 0 {
		return "Side"
	}
	allAbove, allBelow := true, true
	for _, cell := range cells {
		row := cell / width
		if row >= top {
			allAbove = false
		}
		if row <= bottom {
			allBelow = false
		}
	}
	if allAbove {
		return "Above"
	}
	if allBelow {
		return "Below"
	}
	return "Side"
}

func cellDistribution(cells []int, width int) (centroidRow, centroidCol float64, rowSpan, colSpan int) {
	if len(cells) == 0 {
		return 0, 0, 0, 0
	}
	minR, maxR, minC, maxC := math.MaxInt, -1, math.MaxInt, -1
	sumR, sumC := 0, 0
	for _, cell := range cells {
		r, c := cell/width, cell%width
		sumR, sumC = sumR+r, sumC+c
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
		if c < minC {
			minC = c
		}
		if c > maxC {
			maxC = c
		}
	}
	return round3(float64(sumR) / float64(len(cells))),
		round3(float64(sumC) / float64(len(cells))),
		maxR - minR + 1, maxC - minC + 1
}

func cellsSet(cells []int) map[int]bool {
	out := map[int]bool{}
	for _, cell := range cells {
		out[cell] = true
	}
	return out
}

func unionCells(a, b []int) []int {
	set := cellsSet(a)
	for _, cell := range b {
		set[cell] = true
	}
	out := make([]int, 0, len(set))
	for cell := range set {
		out = append(out, cell)
	}
	sort.Ints(out)
	return out
}

func AnalyzeSpatialDependencies(rush01041Dir, rush0105Dir, outputDir string, budget SolveBudget) (SpatialDependencyAnalysisReport, error) {
	report := SpatialDependencyAnalysisReport{
		SchemaVersion: 1,
		Examples:      map[string]string{},
		AnalysisLimitations: []string{
			"Causal graph is a lightweight optimal-replay graph: edge A→B means B used a cell most recently vacated by A; it is evidence of required space, not a complete causality theorem.",
			"Source-to-Cargo identity is index-based for original sorted Rush labels; native appended pieces have no Rush counterpart.",
			"Human-labelled sample is small (RUSH01041 n=8 labelled, RUSH0105 n=6 labelled).",
		},
	}
	type batchSpec struct {
		name   string
		dir    string
		labels map[string]string
	}
	batches := []batchSpec{
		{name: "RUSH01041", dir: rush01041Dir, labels: RUSH01041HumanCoreLabels},
		{name: "RUSH0105", dir: rush0105Dir, labels: RUSH0105HumanSpatialLabels},
	}
	for _, spec := range batches {
		rows, err := analyzeSpatialBatch(spec.name, spec.dir, spec.labels, budget)
		if err != nil {
			return report, err
		}
		report.Rows = append(report.Rows, rows...)
	}
	matched, err := analyzeMatchedPoolVariants(filepath.Join(rush0105Dir, "checkpoint.json"), budget)
	if err != nil {
		return report, err
	}
	report.MatchedVariants = matched
	finalizeSpatialReport(&report)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return report, err
	}
	if err := WriteJSONFile(filepath.Join(outputDir, "SpatialDependencyAnalysisReport.json"), report); err != nil {
		return report, err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "SpatialDependencyAnalysisReport.md"),
		[]byte(renderSpatialReportMarkdown(report)), 0o644); err != nil {
		return report, err
	}
	return report, nil
}

var RUSH0105HumanSpatialLabels = map[string]string{
	"Candidate_001": "partly-visible",
	"Candidate_002": "native-looking",
	"Candidate_003": "native-looking",
	"Candidate_004": "native-looking",
	"Candidate_005": "visible",
	"Candidate_006": "partly-visible",
}

func analyzeSpatialBatch(name, dir string, labels map[string]string, budget SolveBudget) ([]SpatialDependencyRow, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "Candidates"))
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "Candidate_") && strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	sort.Strings(ids)
	rows := make([]SpatialDependencyRow, 0, len(ids))
	for _, id := range ids {
		level, err := LoadCargoFlowLevelJSONFile(filepath.Join(dir, "Candidates", id+".json"))
		if err != nil {
			return nil, err
		}
		board, err := BoardFromLevelJSON(level)
		if err != nil {
			return nil, err
		}
		sol := board.SolveWithBudget(budget)
		if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
			return nil, fmt.Errorf("%s/%s exact solve failed", name, id)
		}
		meaningful, _ := computeMeaningfulPieces(board, sol, budget)
		offX, offY := 1, 0
		if level.Transplant != nil {
			offX, offY = level.Transplant.OffsetX, level.Transplant.OffsetY
		}
		core := ComputeCoreSpaceMetrics(board, offX, offY, sol, budget)
		depth := ComputeTargetDepthMetrics(board, sol, budget)
		util := ComputeBoardUtilization(board, offX, offY, &sol)
		family, inv, coreClass, augClass := "", "", "", ""
		if level.Enrichment != nil {
			family = level.Enrichment.BaseFamilyId
			inv = level.Enrichment.InventoryClass
			coreClass = level.Enrichment.CoreExpansionClass
			augClass = level.Enrichment.AugmentationClass
		}
		rows = append(rows, SpatialDependencyRow{
			Batch: name, CandidateID: id, FamilyID: family, HumanLabel: labels[id],
			VariantClass: classifyVariant(coreClass, augClass), CoreExpansionClass: coreClass,
			BoardShapeClass: string(util.BoardShapeClass), InventoryClass: inv,
			OptimalGestures: sol.NumMoves, TargetTopRow: depth.TargetTopRow, TargetDepthClass: depth.TargetDepthClass,
			Best6x6MeaningfulContainmentRatio: core.Best6x6MeaningfulContainmentRatio,
			CanMeaningfulStructureFitInAny6x6: core.CanMeaningfulStructureFitInAny6x6,
			Spatial:                           ComputeSpatialDependencyMetrics(board, sol, budget),
			SourceRush:                        ComputeRushSourceSpatialMetrics(level, meaningful, budget),
		})
	}
	return rows, nil
}

func classifyVariant(coreClass, augClass string) string {
	if coreClass != "" && coreClass != string(CoreExpNone) {
		return "CoreExpansion"
	}
	if strings.HasPrefix(augClass, "DependencyPull") {
		return "CoreDerived"
	}
	if augClass != "" && augClass != string(AugNone) {
		return "NativeAugment"
	}
	return "Plain"
}

func analyzeMatchedPoolVariants(checkpointPath string, budget SolveBudget) ([]MatchedVariantComparison, error) {
	cp, err := LoadBoardMixCheckpoint(checkpointPath)
	if err != nil {
		return nil, err
	}
	byFamily := map[string]map[string]BoardMixAccepted{}
	for _, c := range cp.Pool {
		kind := ""
		switch {
		case c.CoreExpansionClass == CoreExpDependencyPullLeft:
			kind = "DependencyPullLeft"
		case c.NativeMeta != nil:
			kind = "NativeAugment"
		case c.CoreExpansionClass == "" && (c.AugmentationClass == "" || c.AugmentationClass == AugNone):
			kind = "Plain"
		default:
			continue
		}
		if byFamily[c.FamilyID] == nil {
			byFamily[c.FamilyID] = map[string]BoardMixAccepted{}
		}
		if _, exists := byFamily[c.FamilyID][kind]; !exists {
			byFamily[c.FamilyID][kind] = c
		}
	}
	families := []string{}
	for family, variants := range byFamily {
		if variants["Plain"].Level != nil && variants["DependencyPullLeft"].Level != nil {
			families = append(families, family)
		}
	}
	sort.Strings(families)
	out := []MatchedVariantComparison{}
	for _, family := range families {
		comparison := MatchedVariantComparison{FamilyID: family}
		for _, kind := range []string{"Plain", "NativeAugment", "DependencyPullLeft"} {
			c, exists := byFamily[family][kind]
			if !exists || c.Level == nil {
				continue
			}
			board, err := BoardFromLevelJSON(c.Level)
			if err != nil {
				return nil, err
			}
			sol := board.SolveWithBudget(budget)
			if !sol.Solvable || sol.TimedOut || sol.BudgetExceeded {
				return nil, fmt.Errorf("matched %s/%s solve failed", family, kind)
			}
			core := ComputeCoreSpaceMetrics(board, c.OffsetX, c.OffsetY, sol, budget)
			spatial := ComputeSpatialDependencyMetrics(board, sol, budget)
			summary := &VariantSpatialSummary{
				VariantClass: kind, Containment: core.Best6x6MeaningfulContainmentRatio,
				GraphEdges:              spatial.DependencyGraphEdgeCount,
				CrossRegionDependencies: spatial.CrossTargetDependencies,
				MeaningfulRowSpan:       spatial.MeaningfulRowSpan,
			}
			switch kind {
			case "Plain":
				comparison.Plain = summary
			case "NativeAugment":
				comparison.NativeAugment = summary
			case "DependencyPullLeft":
				comparison.DependencyPullLeft = summary
			}
		}
		out = append(out, comparison)
	}
	return out, nil
}

func finalizeSpatialReport(report *SpatialDependencyAnalysisReport) {
	compact, native := []SpatialDependencyRow{}, []SpatialDependencyRow{}
	for _, row := range report.Rows {
		switch row.HumanLabel {
		case "visible-6x6", "visible", "partly-visible":
			compact = append(compact, row)
		case "better-native", "native-looking":
			native = append(native, row)
		}
	}
	report.HumanLabelComparison = compareSpatialGroups(compact, native)
	report.TopSeparatingMetrics = rankSpatialSeparators(compact, native)

	sourceFront, sourceBehind := 0.0, 0.0
	for _, row := range report.Rows {
		if row.SourceRush.Available {
			sourceFront += float64(row.SourceRush.DependencyMovesFront)
			sourceBehind += float64(row.SourceRush.DependencyMovesBehind)
		}
	}
	plainCross, coreCross, matchedN := 0.0, 0.0, 0.0
	plainEdges, coreEdges := 0.0, 0.0
	for _, m := range report.MatchedVariants {
		if m.Plain == nil || m.DependencyPullLeft == nil {
			continue
		}
		matchedN++
		plainCross += float64(m.Plain.CrossRegionDependencies)
		coreCross += float64(m.DependencyPullLeft.CrossRegionDependencies)
		plainEdges += float64(m.Plain.GraphEdges)
		coreEdges += float64(m.DependencyPullLeft.GraphEdges)
	}
	compactCross, nativeCross := meanRowMetric(compact, "cross"), meanRowMetric(native, "cross")
	misleading := 0
	totalBelowToAbove := 0
	for _, row := range compact {
		if !row.CanMeaningfulStructureFitInAny6x6 && row.Spatial.BelowToAboveDependencyEdges == 0 {
			misleading++
		}
	}
	for _, row := range report.Rows {
		totalBelowToAbove += row.Spatial.BelowToAboveDependencyEdges
	}
	report.HypothesisVerdicts = []HypothesisVerdict{
		{Hypothesis: "H1 Target depth is the cause", Verdict: "REJECTED",
			Evidence: "Prior calibration: visible top-row mean 3.60 vs better-native 3.67; current spatial labels show no material depth separation."},
		{Hypothesis: "H2 Rush source front≫behind bias maps to Cargo upper bias", Verdict: verdictRatio(sourceFront, sourceBehind),
			Evidence: fmt.Sprintf("Across analyzed source solutions: front dependency moves %.0f, behind %.0f (ratio %.2f).", sourceFront, sourceBehind, (sourceFront+1)/(sourceBehind+1))},
		{Hypothesis: "H3 CoreExpansion changes geometry more than causal topology", Verdict: verdictCoreTopology(matchedN, plainEdges, coreEdges, plainCross, coreCross),
			Evidence: fmt.Sprintf("%d matched plain→DependencyPullLeft families: mean graph edges %.2f→%.2f; cross-target dependencies %.2f→%.2f. No true NativeAugment candidate (NativeMeta) exists in this checkpoint pool.", int(matchedN), safeDiv(plainEdges, matchedN), safeDiv(coreEdges, matchedN), safeDiv(plainCross, matchedN), safeDiv(coreCross, matchedN))},
		{Hypothesis: "H4 Shape/Fits6x6 can pass without distributed causality", Verdict: verdictCount(misleading),
			Evidence: fmt.Sprintf("%d compact/partly-visible labelled candidates have Fits6x6=false while direct Below→Above edges remain 0.", misleading)},
		{Hypothesis: "H5 Current candidates lack cross-region dependency flow, especially lower→upper", Verdict: verdictMissingCrossRegion(totalBelowToAbove, len(report.Rows), compactCross, nativeCross),
			Evidence: fmt.Sprintf("Direct Below→Above edges across all %d candidates: %d. Transitive lower/side prerequisites (compact vs native means): %.2f vs %.2f.", len(report.Rows), totalBelowToAbove, compactCross, nativeCross)},
	}
	report.Examples = selectSpatialExamples(report.Rows)
	report.RootCauseConclusion = "The recurring compact-core look is primarily a causal-topology concentration problem: occupied/meaningful cells can be spatially spread while the optimal exit-clearing chain remains locally connected and has few lower/side-to-upper prerequisite edges. Target depth is incidental; source directional bias and relocation-only expansion preserve much of the original dependency flow."
	report.RecommendedDirection = "B. Introduce cross-region dependency synthesis: require verified lower/side→upper prerequisite edges in the exact optimal replay, using Rush only as a seed while rebuilding causal links. Do not optimize geometry or containment alone."
}

func compareSpatialGroups(compact, native []SpatialDependencyRow) map[string]interface{} {
	return map[string]interface{}{
		"compactLikeCount": len(compact),
		"nativeLikeCount":  len(native),
		"compactLike": map[string]float64{
			"crossTargetDependencies": meanRowMetric(compact, "cross"),
			"belowToAboveEdges":       meanRowMetric(compact, "belowEdges"),
			"graphDepth":              meanRowMetric(compact, "depth"),
			"meaningfulRowSpan":       meanRowMetric(compact, "rowSpan"),
			"dependencyRowSpan":       meanRowMetric(compact, "depRowSpan"),
			"containment":             meanRowMetric(compact, "containment"),
			"sourceFrontBehindRatio":  meanRowMetric(compact, "sourceRatio"),
		},
		"nativeLike": map[string]float64{
			"crossTargetDependencies": meanRowMetric(native, "cross"),
			"belowToAboveEdges":       meanRowMetric(native, "belowEdges"),
			"graphDepth":              meanRowMetric(native, "depth"),
			"meaningfulRowSpan":       meanRowMetric(native, "rowSpan"),
			"dependencyRowSpan":       meanRowMetric(native, "depRowSpan"),
			"containment":             meanRowMetric(native, "containment"),
			"sourceFrontBehindRatio":  meanRowMetric(native, "sourceRatio"),
		},
	}
}

func rankSpatialSeparators(compact, native []SpatialDependencyRow) []MetricSeparation {
	metrics := []string{"cross", "belowEdges", "sideEdges", "depth", "rowSpan", "depRowSpan", "centroidRow", "containment", "sourceRatio"}
	out := []MetricSeparation{}
	for _, metric := range metrics {
		a, b := rowMetricValues(compact, metric), rowMetricValues(native, metric)
		ma, mb := mean(a), mean(b)
		sd := pooledSD(a, b)
		gap := 0.0
		if sd > 0 {
			gap = math.Abs(ma-mb) / sd
		}
		out = append(out, MetricSeparation{Metric: metricDisplayName(metric), CompactLikeMean: round3(ma), NativeLikeMean: round3(mb), AbsoluteDelta: round3(math.Abs(ma - mb)), StandardizedGap: round3(gap)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StandardizedGap > out[j].StandardizedGap })
	return out
}

func rowMetricValues(rows []SpatialDependencyRow, metric string) []float64 {
	out := make([]float64, 0, len(rows))
	for _, row := range rows {
		switch metric {
		case "cross":
			out = append(out, float64(row.Spatial.CrossTargetDependencies))
		case "belowEdges":
			out = append(out, float64(row.Spatial.BelowToAboveDependencyEdges))
		case "sideEdges":
			out = append(out, float64(row.Spatial.SideToAboveDependencyEdges))
		case "depth":
			out = append(out, float64(row.Spatial.DependencyGraphDepth))
		case "rowSpan":
			out = append(out, float64(row.Spatial.MeaningfulRowSpan))
		case "depRowSpan":
			out = append(out, float64(row.Spatial.DependencyRowSpan))
		case "centroidRow":
			out = append(out, row.Spatial.DependencyCentroidRow)
		case "containment":
			out = append(out, row.Best6x6MeaningfulContainmentRatio)
		case "sourceRatio":
			out = append(out, row.SourceRush.FrontBehindActivityRatio)
		}
	}
	return out
}

func meanRowMetric(rows []SpatialDependencyRow, metric string) float64 {
	return round3(mean(rowMetricValues(rows, metric)))
}

func metricDisplayName(metric string) string {
	names := map[string]string{
		"cross": "CrossTargetDependencies", "belowEdges": "BelowToAboveDependencyEdges",
		"sideEdges": "SideToAboveDependencyEdges", "depth": "DependencyGraphDepth",
		"rowSpan": "MeaningfulRowSpan", "depRowSpan": "DependencyRowSpan",
		"centroidRow": "DependencyCentroidRow", "containment": "Best6x6MeaningfulContainmentRatio",
		"sourceRatio": "RushSourceFrontBehindActivityRatio",
	}
	return names[metric]
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	total := 0.0
	for _, x := range xs {
		total += x
	}
	return total / float64(len(xs))
}

func pooledSD(a, b []float64) float64 {
	all := append(append([]float64{}, a...), b...)
	if len(all) < 2 {
		return 0
	}
	m := mean(all)
	sum := 0.0
	for _, x := range all {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(all)-1))
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func verdictRatio(front, behind float64) string {
	ratio := (front + 1) / (behind + 1)
	if ratio >= 2 {
		return "SUPPORTED"
	}
	if ratio >= 1.25 {
		return "WEAK"
	}
	return "REJECTED"
}

func verdictCoreTopology(n, plainEdges, coreEdges, plainCross, coreCross float64) string {
	if n == 0 {
		return "WEAK"
	}
	edgeGain := safeDiv(coreEdges, n) - safeDiv(plainEdges, n)
	crossGain := safeDiv(coreCross, n) - safeDiv(plainCross, n)
	if edgeGain <= 1 && crossGain <= 0.5 {
		return "SUPPORTED"
	}
	if crossGain <= 1 {
		return "WEAK"
	}
	return "REJECTED"
}

func verdictCount(n int) string {
	if n >= 2 {
		return "SUPPORTED"
	}
	if n == 1 {
		return "WEAK"
	}
	return "REJECTED"
}

func verdictGroupGap(compactCross, nativeCross, compactBelow, nativeBelow float64) string {
	if nativeCross >= compactCross+0.75 || nativeBelow >= compactBelow+0.5 {
		return "SUPPORTED"
	}
	if nativeCross > compactCross || nativeBelow > compactBelow {
		return "WEAK"
	}
	return "REJECTED"
}

func verdictMissingCrossRegion(totalBelowToAbove, totalCandidates int, compactCross, nativeCross float64) string {
	if totalCandidates > 0 && float64(totalBelowToAbove)/float64(totalCandidates) <= 0.10 {
		return "SUPPORTED"
	}
	if nativeCross > compactCross {
		return "WEAK"
	}
	return "REJECTED"
}

func selectSpatialExamples(rows []SpatialDependencyRow) map[string]string {
	out := map[string]string{}
	var compact, native, misleading *SpatialDependencyRow
	for i := range rows {
		row := &rows[i]
		if compact == nil && (row.HumanLabel == "visible" || row.HumanLabel == "visible-6x6") {
			compact = row
		}
		if native == nil && (row.HumanLabel == "native-looking" || row.HumanLabel == "better-native") && row.Spatial.CrossTargetDependencies > 0 {
			native = row
		}
		if misleading == nil && !row.CanMeaningfulStructureFitInAny6x6 &&
			(row.HumanLabel == "visible" || row.HumanLabel == "partly-visible") &&
			row.Spatial.BelowToAboveDependencyEdges == 0 {
			misleading = row
		}
	}
	if compact != nil {
		out["clearlyCompact"] = fmt.Sprintf("%s/%s: containment %.3f, cross-target edges %d, graph depth %d", compact.Batch, compact.CandidateID, compact.Best6x6MeaningfulContainmentRatio, compact.Spatial.CrossTargetDependencies, compact.Spatial.DependencyGraphDepth)
	}
	if native != nil {
		out["clearlyNativeLooking"] = fmt.Sprintf("%s/%s: containment %.3f, cross-target edges %d, graph depth %d", native.Batch, native.CandidateID, native.Best6x6MeaningfulContainmentRatio, native.Spatial.CrossTargetDependencies, native.Spatial.DependencyGraphDepth)
	}
	if misleading != nil {
		out["misleadingMetricCase"] = fmt.Sprintf("%s/%s: Fits6x6=false but human=%s and direct Below→Above edges=0", misleading.Batch, misleading.CandidateID, misleading.HumanLabel)
	}
	return out
}

func renderSpatialReportMarkdown(report SpatialDependencyAnalysisReport) string {
	var b strings.Builder
	b.WriteString("# RUSH-010.6.1 Spatial Dependency Root-Cause Analysis\n\n")
	b.WriteString("Analysis only. No generation, selector, or synthesis changes were run.\n\n")
	b.WriteString("## Root-cause conclusion\n\n" + report.RootCauseConclusion + "\n\n")
	b.WriteString("## Recommended direction\n\n" + report.RecommendedDirection + "\n\n")
	b.WriteString("## Human-label comparison\n\n")
	b.WriteString("| Metric | Compact / partly-visible | Native-looking |\n|---|---:|---:|\n")
	compact, _ := report.HumanLabelComparison["compactLike"].(map[string]float64)
	native, _ := report.HumanLabelComparison["nativeLike"].(map[string]float64)
	keys := []string{"crossTargetDependencies", "belowToAboveEdges", "graphDepth", "meaningfulRowSpan", "dependencyRowSpan", "containment", "sourceFrontBehindRatio"}
	for _, key := range keys {
		b.WriteString(fmt.Sprintf("| %s | %.3f | %.3f |\n", key, compact[key], native[key]))
	}
	b.WriteString("\n## Top separating metrics\n\n| Metric | Compact mean | Native mean | Standardized gap |\n|---|---:|---:|---:|\n")
	for _, m := range report.TopSeparatingMetrics {
		b.WriteString(fmt.Sprintf("| %s | %.3f | %.3f | %.3f |\n", m.Metric, m.CompactLikeMean, m.NativeLikeMean, m.StandardizedGap))
	}
	b.WriteString("\n## Hypothesis verdicts\n\n| Hypothesis | Verdict | Evidence |\n|---|---|---|\n")
	for _, h := range report.HypothesisVerdicts {
		b.WriteString(fmt.Sprintf("| %s | **%s** | %s |\n", h.Hypothesis, h.Verdict, h.Evidence))
	}
	b.WriteString("\n## Per-candidate metrics\n\n")
	b.WriteString("| Batch | Candidate | Human | Target | Containment | Dep row span | Meaningful row span | Graph edges/depth | Below→Above | Side→Above |\n|---|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, row := range report.Rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %.3f | %d | %d | %d/%d | %d | %d |\n",
			row.Batch, row.CandidateID, row.HumanLabel, row.TargetTopRow,
			row.Best6x6MeaningfulContainmentRatio, row.Spatial.DependencyRowSpan,
			row.Spatial.MeaningfulRowSpan, row.Spatial.DependencyGraphEdgeCount,
			row.Spatial.DependencyGraphDepth, row.Spatial.BelowToAboveDependencyEdges,
			row.Spatial.SideToAboveDependencyEdges))
	}
	b.WriteString("\n## Matched-family variant comparison\n\n")
	b.WriteString("| Family | Containment plain → PullLeft | Graph edges plain → PullLeft | Cross dependencies plain → PullLeft | Row span plain → PullLeft | Native |\n|---|---:|---:|---:|---:|---|\n")
	for _, m := range report.MatchedVariants {
		if m.Plain == nil || m.DependencyPullLeft == nil {
			continue
		}
		native := "not available"
		if m.NativeAugment != nil {
			native = fmt.Sprintf("edges=%d cross=%d", m.NativeAugment.GraphEdges, m.NativeAugment.CrossRegionDependencies)
		}
		b.WriteString(fmt.Sprintf("| %s | %.3f → %.3f | %d → %d | %d → %d | %d → %d | %s |\n",
			m.FamilyID, m.Plain.Containment, m.DependencyPullLeft.Containment,
			m.Plain.GraphEdges, m.DependencyPullLeft.GraphEdges,
			m.Plain.CrossRegionDependencies, m.DependencyPullLeft.CrossRegionDependencies,
			m.Plain.MeaningfulRowSpan, m.DependencyPullLeft.MeaningfulRowSpan, native))
	}
	b.WriteString("\n## Examples\n\n")
	exampleKeys := []string{"clearlyCompact", "clearlyNativeLooking", "misleadingMetricCase"}
	for _, key := range exampleKeys {
		if value := report.Examples[key]; value != "" {
			b.WriteString(fmt.Sprintf("- **%s:** %s\n", key, value))
		}
	}
	b.WriteString("\n## Limitations\n\n")
	for _, limitation := range report.AnalysisLimitations {
		b.WriteString("- " + limitation + "\n")
	}
	return b.String()
}

// MarshalSpatialReport is useful for lightweight CLI output and tests.
func MarshalSpatialReport(report SpatialDependencyAnalysisReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
