# Cargo Flow Solver Performance (RUSH-004)

## Problem

Production Level 28 failed exact search on amd64 with the legacy IDA* +
piece-index `MemoKey`:

| | RUSH-003.1 (before) |
|--|--|
| Algorithm | IDA* over labeled piece indexes |
| Visited | 8,000,000 (budget) |
| Result | BUDGET_EXCEEDED |
| Optimal | UNKNOWN |

Level 28 layout has **ten interchangeable movable 1×1** pieces. Index-based
state keys treat permutations of identical blocks as distinct states, exploding
the search space (~10! labelings of the same occupancy).

## Profiler / baseline findings (pre-fix)

From Level 28 inventory + short IDA* runs:

- Piece classes: Target×1, Unit1x1×**10**, plus unique longs (1×2 H/V, 1×3 H/V)
- Initial branching factor: 4 legal gestures
- Hot path: `map[MemoKey]` with `MemoKey=[18]int`, repeated IDA* deepening
- Allocations dominated by map growth / search stack; on 386 previously OOM’d

Duplicate ratio after canonical BFS (measured): **~90%** of generated successors
were already-seen canonical states (`dup 12.4M / gen 13.8M` on L28).

## Optimizations (exact, no rule changes)

1. **Cargo Flow exact BFS** (gesture unit cost = 1) instead of IDA* for
   `RulesCargoFlow`. Original Rush Hour keeps IDA*.
2. **Permutation-invariant canonical keys**: within each
   `(Kind, Size, Orientation)` class, positions are sorted before packing into
   a compact `cargoKey` (`[MaxPieces]uint8` + `won`).
3. **Compact key**: fixed-size comparable struct (no strings, no crypto hash).
4. **Static vs dynamic**: walls / sizes / axes stay on `Board`; visited set stores
   only canonical positions + `won`.
5. **Path reconstruction**: parent index + `Move` on the labeled working board;
   replay unchanged.

No A* / IDA* redesign for Cargo Flow. Heuristics not required after canonical BFS.

## Correctness

- Successors still come from existing `Board.Moves` (same gesture graph).
- Canonical keys merge only interchangeable permutations; Target / distinct
  classes remain distinguished.
- BFS ⇒ first solution is minimal gesture count.
- Regression locks prior exact optima for L1/L10/L20/L30, Rush smoke (9), POC (4).

## Benchmark (windows/amd64)

Bounds for L28: `TimeLimit=60s`, `MaxVisited=16_000_000`.

| Level | Canonical | Optimal | Visited (unique) | Expanded | Elapsed | Replay |
|------:|----------:|--------:|-----------------:|---------:|--------:|:------:|
| 01 | 4 | **4** | 85 | 33 | ~0 ms | PASS |
| 10 | 8 | **6** | 5531 | 2850 | ~6 ms | PASS |
| 20 | 12 | **9** | 994909 | 420016 | ~2.1 s | PASS |
| **28** | 20 | **17** | **1328622** | **725161** | **~3.1 s** | **PASS** |
| 30 | 24 | **21** | 2204214 | 1693121 | ~6.4 s | PASS |

### Level 28 before → after

| Metric | Before (IDA* / index keys) | After (BFS / canonical) |
|--------|---------------------------:|------------------------:|
| Result | BUDGET_EXCEEDED @ 8M | **exact Solved** |
| Optimal gestures | UNKNOWN | **17** |
| Visited | 8,000,000 (cap) | **1,328,622** |
| Elapsed | ~17 s (cap hit) | **~3.1 s** |
| Generated successors | n/a | 13,766,826 |
| Duplicates rejected | n/a | 12,438,205 (~90%) |
| Peak frontier | n/a | 603,456 |
| Alloc bytes Δ | n/a (prev OOM on 386) | ~888 MB |
| GC Δ | n/a | 14 |

## Why Level 28 is hard

Measured: **10 identical 1×1** movables inside a cage of statics create a huge
labeled state space. Without canonicalization, index permutations dominate
visited growth. After merging permutations, unique occupancy states (~1.3M)
fit comfortably under the 16M / 60s bound; remaining cost is high branching
among those 1×1s (large generated/duplicate volume).
