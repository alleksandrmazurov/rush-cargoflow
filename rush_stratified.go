package rush

import (
	"sort"
)

// StratifiedSampleConfig controls cheap source selection from the full DB.
type StratifiedSampleConfig struct {
	Seed           int64
	TargetPoolSize int
	PerStratumCap  int // 0 = even split of TargetPoolSize
	DatasetTag     string
	MaxScanRecords int // 0 = all
}

func DefaultStratifiedSampleConfig() StratifiedSampleConfig {
	return StratifiedSampleConfig{
		Seed:           20260915,
		TargetPoolSize: 1500,
		PerStratumCap:  300,
		DatasetTag:     "rush",
	}
}

var defaultStrataOrder = []string{"low", "low-mid", "mid", "high", "very-high"}

// StratifiedSampleSources streams the database and returns a diverse source pool.
// Deterministic for fixed seed + database contents + config.
func StratifiedSampleSources(path string, cfg StratifiedSampleConfig) ([]SourceFeatureVector, int, error) {
	if cfg.TargetPoolSize <= 0 {
		cfg.TargetPoolSize = 1500
	}
	if cfg.PerStratumCap <= 0 {
		cfg.PerStratumCap = (cfg.TargetPoolSize + len(defaultStrataOrder) - 1) / len(defaultStrataOrder)
	}

	// Oversample per stratum, then diversify-pick.
	oversample := cfg.PerStratumCap * 5
	if oversample < 100 {
		oversample = 100
	}
	buckets := map[string][]SourceFeatureVector{}
	for _, s := range defaultStrataOrder {
		buckets[s] = nil
	}

	scanned := 0
	_, err := StreamRushDBFile(path, cfg.DatasetTag, func(rec RushDBRecord) error {
		scanned++
		if cfg.MaxScanRecords > 0 && scanned > cfg.MaxScanRecords {
			return ErrStreamStop
		}
		feat, err := ComputeSourceFeatures(rec)
		if err != nil {
			return nil
		}
		list := buckets[feat.MovesStratum]
		list = append(list, feat)
		if len(list) > oversample*3 {
			// Periodically compact to oversample best keys to bound memory.
			list = keepBestKeys(list, oversample, cfg.Seed)
		}
		buckets[feat.MovesStratum] = list
		return nil
	})
	if err != nil && err != ErrStreamStop {
		return nil, scanned, err
	}

	out := make([]SourceFeatureVector, 0, cfg.TargetPoolSize)
	for _, s := range defaultStrataOrder {
		list := keepBestKeys(buckets[s], oversample, cfg.Seed)
		picked := diversifyPick(list, cfg.PerStratumCap, cfg.Seed)
		out = append(out, picked...)
	}
	if len(out) > cfg.TargetPoolSize {
		out = trimRoundRobin(out, cfg.TargetPoolSize, cfg.Seed)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MovesStratum != out[j].MovesStratum {
			return stratumRank(out[i].MovesStratum) < stratumRank(out[j].MovesStratum)
		}
		ki, kj := sampleKey(cfg.Seed, out[i]), sampleKey(cfg.Seed, out[j])
		if ki != kj {
			return ki < kj
		}
		return out[i].LineNumber < out[j].LineNumber
	})
	return out, scanned, nil
}

func stratumRank(s string) int {
	for i, x := range defaultStrataOrder {
		if x == s {
			return i
		}
	}
	return 99
}

func sampleKey(seed int64, f SourceFeatureVector) uint64 {
	// Mix seed + identity + inventory for deterministic pseudo-random order.
	h := uint64(seed) ^ 0x9e3779b97f4a7c15
	h ^= hashString(f.SourcePuzzleID) * 0x100000001b3
	h ^= hashString(f.InventorySignature) * 0xc2b2ae3d27d4eb4f
	h ^= uint64(f.OriginalOptimalMoves) << 17
	h ^= uint64(f.StaticCount) << 7
	h ^= uint64(f.PieceCount) << 3
	return h
}

func hashString(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func keepBestKeys(items []SourceFeatureVector, n int, seed int64) []SourceFeatureVector {
	if len(items) <= n {
		return items
	}
	type scored struct {
		f   SourceFeatureVector
		key uint64
	}
	arr := make([]scored, len(items))
	for i, f := range items {
		arr[i] = scored{f: f, key: sampleKey(seed, f)}
	}
	sort.SliceStable(arr, func(i, j int) bool {
		if arr[i].key != arr[j].key {
			return arr[i].key < arr[j].key
		}
		return arr[i].f.LineNumber < arr[j].f.LineNumber
	})
	out := make([]SourceFeatureVector, n)
	for i := 0; i < n; i++ {
		out[i] = arr[i].f
	}
	return out
}

// diversifyPick selects up to n items maximizing inventory diversity, then fills.
func diversifyPick(items []SourceFeatureVector, n int, seed int64) []SourceFeatureVector {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	type scored struct {
		f   SourceFeatureVector
		key uint64
	}
	arr := make([]scored, len(items))
	for i, f := range items {
		arr[i] = scored{f: f, key: sampleKey(seed, f)}
	}
	sort.SliceStable(arr, func(i, j int) bool {
		if arr[i].key != arr[j].key {
			return arr[i].key < arr[j].key
		}
		return arr[i].f.LineNumber < arr[j].f.LineNumber
	})

	seenInv := map[string]int{}
	out := make([]SourceFeatureVector, 0, n)
	for _, s := range arr {
		if len(out) >= n {
			break
		}
		if seenInv[s.f.InventorySignature] >= 2 {
			continue
		}
		seenInv[s.f.InventorySignature]++
		out = append(out, s.f)
	}
	if len(out) < n {
		have := map[string]bool{}
		for _, f := range out {
			have[f.SourcePuzzleID] = true
		}
		for _, s := range arr {
			if len(out) >= n {
				break
			}
			if have[s.f.SourcePuzzleID] {
				continue
			}
			out = append(out, s.f)
			have[s.f.SourcePuzzleID] = true
		}
	}
	return out
}

func trimRoundRobin(items []SourceFeatureVector, n int, seed int64) []SourceFeatureVector {
	by := map[string][]SourceFeatureVector{}
	for _, f := range items {
		by[f.MovesStratum] = append(by[f.MovesStratum], f)
	}
	for s := range by {
		sort.SliceStable(by[s], func(i, j int) bool {
			return sampleKey(seed, by[s][i]) < sampleKey(seed, by[s][j])
		})
	}
	out := make([]SourceFeatureVector, 0, n)
	for len(out) < n {
		added := false
		for _, s := range defaultStrataOrder {
			list := by[s]
			if len(list) == 0 {
				continue
			}
			out = append(out, list[0])
			by[s] = list[1:]
			added = true
			if len(out) >= n {
				break
			}
		}
		if !added {
			break
		}
	}
	return out
}
