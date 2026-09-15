# Cargo Flow Generator — Functional Diversity (RUSH-007)

Extends the RUSH-005 offline generator with **PuzzleSignature**,
**FunctionalSimilarity**, and **puzzle archetypes** so batches are not
geometric clones of one “clear 1×1 corridor” idea.

RUSH-005 `output/pilot/` is kept as the human-reviewed regression corpus
(complexity OK, diversity poor). Do not delete it.

## Why RUSH-005 felt identical

All five human-played candidates share:

- stacked **1×1** direct exit-corridor blockers;
- shuttle choreography (slide units aside, then Target-exit);
- similar inventory / dependency depth.

Layout Jaccard often sat at 0.5–0.7 (below the 0.85 layout reject), so
geometric filtering alone did not catch the shared **solution pattern**.

## PuzzleSignature

Built from the exact solution (class tokens, never piece IDs):

- InventorySignature
- InitialTargetBlockerCount / DirectTargetBlockerClasses / TargetBlockerTypes
- DistinctMovedPieces / DistinctNonTargetMovedPieces
- TargetMoveCount / NonTargetMovesBeforeFirstTargetMove
- MovedPieceClassHistogram
- OptimalSolutionClassSequence (`1x2H-left-1`, `Target-exit`, …)
- DependencyDepth / DependencyEdges (approx. freed-cell → later-move edges)
- OptimalGestures / TargetStartRow

## FunctionalSimilarity

Deterministic weighted blend in `[0,1]` (layout excluded):

| Term | Weight |
|------|-------:|
| corridor shuttle pattern | 0.28 |
| solution class sequence (LCS) | 0.22 |
| blocker signature | 0.14 |
| moved-class histogram | 0.10 |
| dependency depth | 0.10 |
| inventory | 0.08 |
| target move pattern | 0.08 |

Reject if **layout ≥ 0.85** OR **functional ≥ 0.72**.

Human-rejected gate: only when signature is shuttle-like
(≥85% 1×1 corridor blockers, ≥4 blockers, ≥65% 1×1 moves) AND
functional ≥ 0.70 vs RUSH-005 001–005.

## Archetypes

| Archetype | Generation bias | Post-solve validation |
|-----------|-----------------|------------------------|
| LongBlockChain | 1×2/1×3 inventory + long corridor pieces | ≥1 long-block move |
| CrossLock | nested corridor + H bars on exit column | ≥2 blockers + H involvement |
| StaticGate | static channels + H locks | walls + adjacency |
| SideChain | side column chain + H lock | DependencyDepth ≥ 2 |
| SmallBlockShuttle | dense 1×1 nested corridor | ≥2 moved 1×1 |

Max **2** accepted per archetype. Custom placement falls back to the RUSH-005
`tryPlace` hardness path when structural placement fails; archetype is still
enforced by `ValidateArchetype`.

## CLI

```bash
# RUSH-005 style
go run ./cmd/cargoflow-generate --count 10 --seed 12345 --output output/pilot

# RUSH-007 diverse
go run ./cmd/cargoflow-generate \
  --diverse --archetypes all \
  --human-rejected-ref output/pilot \
  --count 10 --min-optimal 10 --seed 202707 --max-attempts 300 \
  --output output/RUSH007_DiversePilot_001
```

## Known limitations

- Exact floor (≥10) + nested-corridor hardness still biases many solutions
  toward related clearing patterns; functional filter then rejects heavily
  (`TooSimilarFunctional`), so pilots may finish with 3–6 accepts / 3–4
  archetypes inside 300 attempts.
- DependencyDepth is approximate.
- SmallBlockShuttle is usually blocked by the human-rejected shuttle gate.
- Unity is not modified; import remains a separate RUSH-006 step.
