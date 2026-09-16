# Cargo Flow 1×1 Enrichment (RUSH-010)

Adds **gameplay-relevant movable 1×1** pieces to curated RUSH-009 Rush-derived
puzzles. Does **not** change RUSH-009 diversity / clustering logic.

## Goal

Answer: can unique Cargo Flow dual-axis 1×1 mechanics be integrated into good
Rush-derived puzzles without destroying their quality?

## Rules

- Start from exact-solved RUSH-009 shortlist bases only
- Add 1 (max 2) movable 1×1 via deterministic empty-cell enumeration
- Exact CF solve must **move** at least one added 1×1 (`OneByOneUnused` reject)
- Strong necessity: with added 1×1 **immobile**, solve is unsolved or strictly longer
- Prefer 1–3 1×1 gestures in optimal; OptimalDelta roughly 0..+6
- Max one accepted enrichment per base family

## RUSH-010.1 Role diversity

```bash
go run ./cmd/cargoflow-enrich --role-diversity \
  --batch output/RUSH009_CuratedShortlist_001 \
  --output output/RUSH0101_OneByOneRoleDiversity_001 \
  --bases 20 --count 8
```

Prefer **off-corridor** placements first. Classify `OneByOneRole`:

| Role | Meaning |
|------|---------|
| DirectTargetBlocker | starts on Target exit corridor |
| IndirectBlocker | frees cells for dependency chain |
| SpaceMaker | frees destination for 1x2/1x3 |
| SideShuttle | ≥2 off-corridor 1x1 moves |
| GateKeeper | constrained side passage |

Quota: DirectTargetBlocker ≤ ~30%. Target ≥5 with `InitiallyInTargetCorridor=false`.

## Output

Unity-compatible JSON (+ optional `enrichment` metadata), solutions, manifest,
`EnrichmentReport.json`, `HumanReview.csv`.

## Calibration

Human RUSH-009 reviews live in
`data/calibration/rush009_human_review.json` (subjective expert notes).
