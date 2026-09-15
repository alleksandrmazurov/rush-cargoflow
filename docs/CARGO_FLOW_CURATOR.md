# Cargo Flow Curator (RUSH-009)

Offline automatic curator that reduces the **full** Michael Fogleman Rush Hour
database to a small, diverse, exact-solved Cargo Flow shortlist for human review.

This is **not** a generator. RUSH-005/007 remain frozen.

## Pipeline

1. **Acquire** full `rush.txt` (see [`EXTERNAL_DATA.md`](EXTERNAL_DATA.md))
2. **Streaming cheap scan** — `SourceFeatureVector` per record (no CF solve)
3. **Stratified sampling** — ~1500 sources across original-moves strata + inventory diversity
4. **Transform** — reuse RUSH-008 CW90 + ≤2 embeddings
5. **Exact CF solve** — bounded BFS; authoritative difficulty
6. **Persistent cache** — JSONL keyed by DB hash + puzzle + transform/solver versions
7. **Metrics / difficulty signals** — explainable features (not ML)
8. **Family clustering** — `PuzzleFamilyDistance` + greedy clusters
9. **Farthest-point shortlist** — diversity-first with CF optimal bands
10. **Output** — Unity-compatible JSON + `HumanReview.csv`

## CLI

```bash
go run ./cmd/rush-db-fetch --out data/external/rush

go run ./cmd/cargoflow-curate \
  --database data/external/rush/rush.txt \
  --shortlist 24 \
  --seed 20260915 \
  --workers 2 \
  --source-pool 1500 \
  --max-solves 3000 \
  --output output/RUSH009_CuratedShortlist_001
```

`--resume` (default true) reuses `data/cache/curator/solve_cache.jsonl`.

## Family similarity

`PuzzleFamilyDistance` = 1 − weighted blend of:

| Signal | Weight |
|--------|-------:|
| Solution class sequence | 0.32 |
| Inventory | 0.18 |
| Moved-class histogram | 0.14 |
| Target blockers | 0.14 |
| Dependency depth | 0.10 |
| Target move pattern | 0.08 |
| Optimal gestures | 0.04 |

Piece IDs and raw coordinates are not used. Shuttle/1×1 bias from RUSH-007 is
intentionally reduced (Rush DB has no movable 1×1).

## EstimatedHumanDifficultyScore (experimental)

```text
100 * (0.30*norm(Optimal) + 0.25*norm(log Visited)
     + 0.20*norm(DependencyDepth) + 0.15*norm(Branching)
     + 0.10*norm(DistinctMoved))
```

Not objective truth — diagnostic rank only.

## Human review loop

1. Import shortlist with existing Unity RUSH-006 tool (no Unity code changes)
2. Fill `HumanReview.csv`
3. Optionally extend `data/calibration/rush008_human_review.json`

## Limitations

- No Cargo Flow movable 1×1 (RUSH-010 enrichment later)
- Estimated human difficulty is experimental
- 12 calibration samples ≠ statistical model
- Final human review still required
- `DecisionAmbiguity` deferred (too expensive for full pool)

## Bounds (POC)

| Limit | Default |
|-------|--------:|
| Source pool | 1500 |
| Embeddings / source | 2 |
| Max solve attempts | 3000 |
| Shortlist target | 24 |
| Workers | 2 |
| Solve budget | 10s / 4M visited |
