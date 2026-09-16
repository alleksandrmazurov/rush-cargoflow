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

## RUSH-010.2 Inventory diversity

```bash
go run ./cmd/cargoflow-enrich --inventory-diversity \
  --batch output/RUSH009_CuratedShortlist_001 \
  --output output/RUSH0102_InventoryDiversity_001 \
  --bases 24 --count 12
```

Supports inventory classes (not difficulty labels):

| InventoryClass | Movable1x1Count |
|----------------|-----------------|
| No1x1 | 0 (clean RUSH-009) |
| One1x1 | 1 (must be essential) |
| Two1x1 | 2 (≥1 essential, both relevant) |
| Three1x1 | 3 (≥1 essential, ≥2 relevant) |

Metadata for later sequencing: `inventoryClass`, `inventorySignature`, roles,
corridor/spatial counts. Prefer ≤1 added 1×1 initially in Target corridor.
Pilot target: 3 per class, distinct `BaseFamilyId`, mixed difficulty bands.

## RUSH-010.3 Board space diversity + long-run CLI

Why levels look 6×6: Rush sources are 6×6; classic CW90 + FlushTop/ShiftDown1
embeds that block into 7×8, leaving outer rows/columns empty.

New embeddings: `FlushBottom`, `*MirrorH` (horizontal Rush mirror before CW90,
Target stays on exit col 3). Optional outer-zone structural 1×1 augment.

Metrics / `BoardShapeClass`: Compact6x6, ShiftedCore, Tall, Wide, Expanded, FullField.

**Do not run the full pilot inside Cursor.** Build and run from PowerShell:

```powershell
go build -o bin\cargoflow-boardmix.exe .\cmd\cargoflow-boardmix
.\bin\cargoflow-boardmix.exe `
  --config configs\RUSH0103_BoardDiversityPilot_001.json `
  --output output\RUSH0103_BoardDiversityPilot_001 `
  --resume
```

Checkpoint: `output/.../checkpoint.json` + `progress.json`. Ctrl+C saves and exits.
Reuse RUSH-009 solve cache for classic embeddings (`cw90-embed-v1`).

Smoke: `.\bin\cargoflow-boardmix.exe --smoke`

## RUSH-010.3.1 Diversity quota fix

Root cause of all-ShiftedCore / all-No1x1 pilot: acceptance was `first N under target`
with `inventoryQuotas={No1x1:12}` and no pool→select stage.

Fix: generate bounded **pool** (maxPoolSize/maxAttempts), optional inventory enrichment
(RUSH-010.2), then **SelectBoardMixShortlist** with shape/inventory fraction caps,
min distinct shapes, min OuterZoneRelevant, unique FamilyId.

Manual pilot:

```powershell
go build -o bin\cargoflow-boardmix.exe .\cmd\cargoflow-boardmix
.\bin\cargoflow-boardmix.exe `
  --config configs\RUSH01031_DiversityQuotaPilot_001.json `
  --output output\RUSH01031_DiversityQuotaPilot_001 `
  --resume
```

## Output

Unity-compatible JSON (+ optional `enrichment` / `boardSpace` metadata), solutions, manifest,
`EnrichmentReport.json` / `InventoryDiversityReport.json` / `BoardDiversityReport.json`, `HumanReview.csv`.

## Calibration

Human RUSH-009 reviews live in
`data/calibration/rush009_human_review.json` (subjective expert notes).
