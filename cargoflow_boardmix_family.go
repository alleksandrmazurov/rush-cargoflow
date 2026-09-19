package rush

import (
	"fmt"
	"sort"
)

// FamilyFunnelEntry tracks per-base-family generation funnel for RUSH-010.4.1.
type FamilyFunnelEntry struct {
	BaseFamilyID            string         `json:"baseFamilyId"`
	SourceCandidateID       string         `json:"sourceCandidateId"`
	EmbeddingsAttempted     int            `json:"embeddingsAttempted"`
	AugmentationsProposed   int            `json:"augmentationsProposed"`
	Attempts                int            `json:"attempts"`
	CheapRejected           int            `json:"cheapRejected"`
	ExactSolved             int            `json:"exactSolved"`
	DifficultyUnknown       int            `json:"difficultyUnknown"`
	RelevanceRejected       int            `json:"relevanceRejected"`
	ShortcutRejected        int            `json:"shortcutRejected"`
	DuplicateRejected       int            `json:"duplicateRejected"`
	PerFamilyCapRejected    int            `json:"perFamilyCapRejected"`
	AcceptedIntoPool        int            `json:"acceptedIntoPool"`
	FinalPoolCandidateCount int            `json:"finalPoolCandidateCount"`
	RejectReasonCounts      map[string]int `json:"rejectReasonCounts,omitempty"`
	DominantRejectReason    string         `json:"dominantRejectReason,omitempty"`
}

// FamilyCoverageReport summarizes requested vs realized family coverage.
type FamilyCoverageReport struct {
	RequestedBaseCandidates        int                       `json:"requestedBaseCandidates"`
	DistinctRequestedBaseFamilyIds int                       `json:"distinctRequestedBaseFamilyIds"`
	RequestedFamilyIds             []string                  `json:"requestedFamilyIds"`
	PoolUniqueFamilyIds            int                       `json:"poolUniqueFamilyIds"`
	PoolFamilyIds                  []string                  `json:"poolFamilyIds"`
	MinUniqueFamiliesTarget        int                       `json:"minUniqueFamiliesTarget"`
	FamilyFirstExploration         bool                      `json:"familyFirstExploration"`
	PerFamilyPoolCap               int                       `json:"perFamilyPoolCap"`
	EarlyStopBeforeAllFamilies     bool                      `json:"earlyStopBeforeAllFamilies"`
	RootCauseSummary               string                    `json:"rootCauseSummary"`
	Funnel                         []FamilyFunnelEntry       `json:"funnel"`
	FamilyShapeCross               map[string]map[string]int `json:"familyShapeCross"`
	FamilyAugmentationCross        map[string]map[string]int `json:"familyAugmentationCross"`
	PoolAugmentationClass          map[string]int            `json:"poolAugmentationClass"`
	ExpandedZeroExplanation        string                    `json:"expandedZeroExplanation,omitempty"`
}

type boardMixJob struct {
	base  EnrichmentBase
	embed EmbeddingVariant
	mode  string // plain | outer1 | native
	pass  int    // 1 = coverage, 2 = deepen
}

func boardMixJobKey(baseID string, embed EmbeddingVariant, mode string) string {
	return baseID + "|" + string(embed) + "|" + mode
}

// buildBoardMixJobs schedules family-first coverage then deepen passes.
func buildBoardMixJobs(bases []EnrichmentBase, embeds []EmbeddingVariant, cfg BoardMixConfig) []boardMixJob {
	if len(embeds) == 0 {
		embeds = BoardDiversityEmbeddingVariants()
	}
	modes := boardMixModes(cfg)
	familyFirst := cfg.FamilyFirstExploration

	if !familyFirst {
		jobs := []boardMixJob{}
		for _, base := range bases {
			for _, emb := range embeds {
				for _, mode := range modes {
					jobs = append(jobs, boardMixJob{base: base, embed: emb, mode: mode, pass: 2})
				}
			}
		}
		return jobs
	}

	primary := embeds[0]
	pass1 := map[string]bool{}
	jobs := []boardMixJob{}

	// Pass 1: each family gets one plain (+ native/corexpand if enabled) on primary embed.
	for _, base := range bases {
		kPlain := boardMixJobKey(base.CandidateID, primary, "plain")
		jobs = append(jobs, boardMixJob{base: base, embed: primary, mode: "plain", pass: 1})
		pass1[kPlain] = true
		if cfg.TryCausalSynthesis {
			kCausal := boardMixJobKey(base.CandidateID, primary, "causal")
			jobs = append(jobs, boardMixJob{base: base, embed: primary, mode: "causal", pass: 1})
			pass1[kCausal] = true
		} else if cfg.TryCoreExpansion {
			kCore := boardMixJobKey(base.CandidateID, primary, "corexpand")
			jobs = append(jobs, boardMixJob{base: base, embed: primary, mode: "corexpand", pass: 1})
			pass1[kCore] = true
		} else if cfg.TryNativeAugment {
			kNat := boardMixJobKey(base.CandidateID, primary, "native")
			jobs = append(jobs, boardMixJob{base: base, embed: primary, mode: "native", pass: 1})
			pass1[kNat] = true
		} else if cfg.TryOuterAugment {
			kOut := boardMixJobKey(base.CandidateID, primary, "outer1")
			jobs = append(jobs, boardMixJob{base: base, embed: primary, mode: "outer1", pass: 1})
			pass1[kOut] = true
		}
	}

	// Pass 2: remaining family×embed×mode combinations.
	for _, base := range bases {
		for _, emb := range embeds {
			for _, mode := range modes {
				k := boardMixJobKey(base.CandidateID, emb, mode)
				if pass1[k] {
					continue
				}
				jobs = append(jobs, boardMixJob{base: base, embed: emb, mode: mode, pass: 2})
			}
		}
	}
	return jobs
}

func boardMixModes(cfg BoardMixConfig) []string {
	modes := []string{"plain"}
	if cfg.TryOuterAugment {
		modes = append(modes, "outer1")
	}
	if cfg.TryNativeAugment {
		modes = append(modes, "native")
	}
	if cfg.TryCoreExpansion {
		modes = append(modes, "corexpand")
	}
	if cfg.TryCausalSynthesis {
		modes = append(modes, "causal")
	}
	return modes
}

func initFamilyFunnel(bases []EnrichmentBase) map[string]*FamilyFunnelEntry {
	out := map[string]*FamilyFunnelEntry{}
	for _, b := range bases {
		out[b.FamilyID] = &FamilyFunnelEntry{
			BaseFamilyID:       b.FamilyID,
			SourceCandidateID:  b.CandidateID,
			RejectReasonCounts: map[string]int{},
		}
	}
	return out
}

func noteFamilyAttempt(funnel map[string]*FamilyFunnelEntry, fam, reason string, exact bool, augProposed, accepted int) {
	e := funnel[fam]
	if e == nil {
		e = &FamilyFunnelEntry{BaseFamilyID: fam, RejectReasonCounts: map[string]int{}}
		funnel[fam] = e
	}
	e.Attempts++
	e.EmbeddingsAttempted++
	e.AugmentationsProposed += augProposed
	if exact {
		e.ExactSolved++
	}
	if accepted > 0 {
		e.AcceptedIntoPool += accepted
	}
	if reason == "" {
		return
	}
	e.RejectReasonCounts[reason]++
	switch reason {
	case "DifficultyUnknown":
		e.DifficultyUnknown++
	case "DuplicateFingerprint", "DuplicateRejected":
		e.DuplicateRejected++
	case "ShortcutDegradation":
		e.ShortcutRejected++
	case "DecorativeAddedMovable", "DecorativeStatic", "NativeAugmentFiltered", "RelevanceRejected",
		"ExpandedOuterZoneIrrelevant", "NativeExpandedNotEssential", "ExpandedWithoutEssentialOuter",
		"AugmentFailed":
		e.RelevanceRejected++
	case "PerFamilyCap":
		e.PerFamilyCapRejected++
	default:
		e.CheapRejected++
	}
}

func finalizeFamilyFunnel(funnel map[string]*FamilyFunnelEntry, pool []BoardMixAccepted) []FamilyFunnelEntry {
	poolCount := map[string]int{}
	for _, c := range pool {
		poolCount[c.FamilyID]++
	}
	out := make([]FamilyFunnelEntry, 0, len(funnel))
	for _, e := range funnel {
		cp := *e
		cp.FinalPoolCandidateCount = poolCount[e.BaseFamilyID]
		dom, domN := "", 0
		for k, v := range e.RejectReasonCounts {
			if v > domN {
				dom, domN = k, v
			}
		}
		cp.DominantRejectReason = dom
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FinalPoolCandidateCount != out[j].FinalPoolCandidateCount {
			return out[i].FinalPoolCandidateCount > out[j].FinalPoolCandidateCount
		}
		return out[i].BaseFamilyID < out[j].BaseFamilyID
	})
	return out
}

func buildFamilyCoverageReport(cfg BoardMixConfig, bases []EnrichmentBase, pool []BoardMixAccepted, funnel []FamilyFunnelEntry, earlyStop bool) FamilyCoverageReport {
	reqFam := map[string]bool{}
	reqIDs := []string{}
	for _, b := range bases {
		if !reqFam[b.FamilyID] {
			reqFam[b.FamilyID] = true
			reqIDs = append(reqIDs, b.FamilyID)
		}
	}
	sort.Strings(reqIDs)
	poolFam := map[string]bool{}
	poolIDs := []string{}
	shapeCross := map[string]map[string]int{}
	augCross := map[string]map[string]int{}
	augDist := map[string]int{}
	for _, c := range pool {
		if !poolFam[c.FamilyID] {
			poolFam[c.FamilyID] = true
			poolIDs = append(poolIDs, c.FamilyID)
		}
		shape := string(c.BoardUtil.BoardShapeClass)
		if shapeCross[c.FamilyID] == nil {
			shapeCross[c.FamilyID] = map[string]int{}
		}
		shapeCross[c.FamilyID][shape]++
		aug := string(c.AugmentationClass)
		if aug == "" {
			aug = string(AugNone)
		}
		if augCross[c.FamilyID] == nil {
			augCross[c.FamilyID] = map[string]int{}
		}
		augCross[c.FamilyID][aug]++
		augDist[aug]++
	}
	sort.Strings(poolIDs)

	minUnique := cfg.MinUniqueFamiliesInPool
	if minUnique <= 0 {
		minUnique = 12
	}
	root := ""
	if len(reqFam) < cfg.BaseCount && cfg.BaseCount > 0 {
		root = fmt.Sprintf("SOURCE: DistinctRequestedBaseFamilyIds=%d < RequestedBaseCount=%d. ", len(reqFam), cfg.BaseCount)
	}
	if len(poolFam) < len(reqFam) {
		if earlyStop {
			root += fmt.Sprintf("SCHEDULING: early stop with pool=%d after only %d/%d families entered (MaxPoolSize filled by early families before coverage pass finished).",
				len(pool), len(poolFam), len(reqFam))
		} else {
			unaccepted := 0
			for _, e := range funnel {
				if e.FinalPoolCandidateCount == 0 {
					unaccepted++
				}
			}
			root += fmt.Sprintf("VALIDATION/GENERATION: %d/%d requested families produced 0 pool candidates after job walk.",
				unaccepted, len(reqFam))
		}
	} else if root == "" {
		root = "All requested families appear in pool."
	}

	fullN, expN := 0, 0
	for _, c := range pool {
		switch c.BoardUtil.BoardShapeClass {
		case ShapeFullField:
			fullN++
		case ShapeExpanded:
			expN++
		}
	}
	expExplain := ""
	if fullN > 0 && expN == 0 {
		expExplain = "Classifier orders FullField before Expanded: bbox≈7×8 + util≥0.35 + OuterZoneRelevant → FullField. " +
			"Native both-axis layouts often jump Tall/Wide → FullField, skipping Expanded. Acceptable natural distribution."
	}

	return FamilyCoverageReport{
		RequestedBaseCandidates:        len(bases),
		DistinctRequestedBaseFamilyIds: len(reqFam),
		RequestedFamilyIds:             reqIDs,
		PoolUniqueFamilyIds:            len(poolFam),
		PoolFamilyIds:                  poolIDs,
		MinUniqueFamiliesTarget:        minUnique,
		FamilyFirstExploration:         cfg.FamilyFirstExploration,
		PerFamilyPoolCap:               cfg.PerFamilyPoolCap,
		EarlyStopBeforeAllFamilies:     earlyStop && len(poolFam) < len(reqFam),
		RootCauseSummary:               root,
		Funnel:                         funnel,
		FamilyShapeCross:               shapeCross,
		FamilyAugmentationCross:        augCross,
		PoolAugmentationClass:          augDist,
		ExpandedZeroExplanation:        expExplain,
	}
}

func countPoolFamilies(pool []BoardMixAccepted) map[string]int {
	m := map[string]int{}
	for _, c := range pool {
		m[c.FamilyID]++
	}
	return m
}

func uniqueFamilyCount(pool []BoardMixAccepted) int {
	return len(countPoolFamilies(pool))
}

// shouldAcceptFamilyVariant enforces per-family pool cap while uncovered families remain.
func shouldAcceptFamilyVariant(pool []BoardMixAccepted, fam string, requestedFamilies, cap int) bool {
	if cap <= 0 {
		return true
	}
	counts := countPoolFamilies(pool)
	if counts[fam] < cap {
		return true
	}
	if len(counts) >= requestedFamilies {
		return true
	}
	return false
}
