# BoardMix_PlaytestPack_001

Sequential playtest pack of **existing** BoardMix levels for human sequential play.
No new generation. Native7x8 levels are not included.

Unity import folder:

`testdata/cargoflow/playtests/BoardMix_PlaytestPack_001`

## Sources

- `testdata/cargoflow/pilots/RUSH01071_DeepTargetPilot_001`
- `output/RUSH01071_SeededPilot_003` (content copied into this pack)

Excluded as sources of new content:

- `RUSH01071_DeepTargetPilot_002` — exact reproducibility rerun of pilot 001
- `Native7x8Experiment_001` — experiment stopped after human review

## Play order

| # | Pack ID | Source | FamilyId | Optimal | Inventory | Shape | TargetRow | Human review |
|---:|---|---|---|---:|---|---|---:|---|
| 1 | Level_01 | RUSH01071_SeededPilot_003/Candidate_006 | F130 | 7 | No1x1 | FullField | 4 | included in positively played recommended sample (pilot 003); no per-level score |
| 2 | Level_02 | RUSH01071_DeepTargetPilot_001/Candidate_005 | F065 | 7 | No1x1 | Wide | 5 | not individually reviewed |
| 3 | Level_03 | RUSH01071_SeededPilot_003/Candidate_009 | F143 | 8 | No1x1 | ShiftedCore | 6 | not individually reviewed |
| 4 | Level_04 | RUSH01071_DeepTargetPilot_001/Candidate_008 | F125 | 9 | No1x1 | ShiftedCore | 5 | positive spot-check (pilot 001) |
| 5 | Level_05 | RUSH01071_SeededPilot_003/Candidate_004 | F103 | 9 | Two1x1 | ShiftedCore | 5 | not individually reviewed |
| 6 | Level_06 | RUSH01071_DeepTargetPilot_001/Candidate_011 | F086 | 10 | Two1x1 | Tall | 6 | not individually reviewed |
| 7 | Level_07 | RUSH01071_DeepTargetPilot_001/Candidate_001 | F014 | 12 | One1x1 | ShiftedCore | 6 | positive spot-check (pilot 001) |
| 8 | Level_08 | RUSH01071_SeededPilot_003/Candidate_003 | F047 | 13 | No1x1 | Tall | 5 | not individually reviewed |
| 9 | Level_09 | RUSH01071_SeededPilot_003/Candidate_012 | F042 | 13 | One1x1 | ShiftedCore | 4 | not individually reviewed |
| 10 | Level_10 | RUSH01071_SeededPilot_003/Candidate_005 | F050 | 20 | One1x1 | ShiftedCore | 5 | included in positively played recommended sample (pilot 003); no per-level score |
| 11 | Level_11 | RUSH01071_DeepTargetPilot_001/Candidate_004 | F013 | 22 | No1x1 | ShiftedCore | 5 | positive spot-check (pilot 001) |
| 12 | Level_12 | RUSH01071_DeepTargetPilot_001/Candidate_006 | F036 | 28 | No1x1 | Tall | 2 | positive spot-check (pilot 001) |

## Selection rules applied

- Exact start-state duplicates removed (ID-independent).
- At most one level per FamilyId.
- Soft translation/mirror-normalized proximity checked; no additional soft matches beyond the exact duplicate.
- Prefer positive pilot-001 spot-check levels when competing on FamilyId.
- Prefer SeededPilot_003 recommended-sample members when FamilyId is free.
- Diversity of inventory, Target row, board shape, and augmentation used as soft balancing.
- No new hard CausalExpanded / deep-Target quotas for this pack.

## Exact duplicate handled

| Kept | Excluded | Reason |
|---|---|---|
| DeepTargetPilot_001/Candidate_004 (F013, human+) | SeededPilot_003/Candidate_010 | identical start state |

## Notable FamilyId exclusions (kept preferred source)

| Excluded | FamilyId | Prefer kept |
|---|---|---|
| SeededPilot_003/Candidate_001 | F014 | DeepTargetPilot_001/Candidate_001 (human+) |
| SeededPilot_003/Candidate_008 | F036 | DeepTargetPilot_001/Candidate_006 (human+) |
| DeepTargetPilot_001/Candidate_002 | F050 | SeededPilot_003/Candidate_005 (recommended sample) |
| DeepTargetPilot_001/Candidate_007 | F130 | SeededPilot_003/Candidate_006 (recommended sample + FullField) |
| SeededPilot_003/Candidate_002 | F086 | DeepTargetPilot_001/Candidate_011 (Two1x1 Tall) |
| SeededPilot_003/Candidate_007 | F065 | DeepTargetPilot_001/Candidate_005 (Wide) |
| SeededPilot_003/Candidate_011 | F015 | not selected (other diversity slots filled) |
| DeepTargetPilot_001/Candidate_012 | F015 | not selected |
| DeepTargetPilot_001/Candidate_003 | F035 | not selected (opt 40 spike reserved out of pack) |
| DeepTargetPilot_001/Candidate_009 | F063 | not selected (opt 3 too trivial for this pack) |
| DeepTargetPilot_001/Candidate_010 | F095 | not selected (opt 3) |

Full exclusion list: `Exclusions.json`.

## Difficulty notes

- Unique families: **12**
- Optimal gestures range: **7–28**
- Soft step-ups: 13→20 (Level_09→10), 22→28 (Level_11→12)
- Adjacent Two1x1 inventories at Level_05–06 (accepted to keep ascending optima and Tall shape)
- `optimalGestures` is a rough sequencing guide, not proven human difficulty

## Validation

```powershell
go run ./cmd/cargoflow-boardmix -validate-batch testdata/cargoflow/playtests/BoardMix_PlaytestPack_001
```

Expected: `ok=true`, 12 levels, 12 solutions, Unity BatchManifest contract.
