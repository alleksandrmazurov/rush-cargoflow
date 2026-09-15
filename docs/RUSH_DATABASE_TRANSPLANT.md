# Rush Database Transplant (RUSH-008)

Offline pipeline that converts **Michael Fogleman Rush Hour database** puzzles
into Cargo Flow 7×8 candidates. Cargo Flow exact solver remains the only
authoritative difficulty metric after transform.

Custom generators (RUSH-005 / RUSH-007) are **not** used here; they remain as a
frozen experimental branch.

## Source dataset

- Preview file: [`testdata/rushdb/rush1000.txt`](../testdata/rushdb/rush1000.txt)
- Upstream: https://www.michaelfogleman.com/rush/ (`rush1000.txt`)
- License: MIT (Michael Fogleman) — see [`LICENSE.md`](../LICENSE.md) and
  [`testdata/rushdb/ATTRIBUTION.md`](../testdata/rushdb/ATTRIBUTION.md)

### Record format

Each line:

```text
<originalOptimalMoves> <36-char board> <clusterSize>
```

Board is a 6×6 row-major string:

| Char | Meaning |
|------|---------|
| `o` / `.` | empty |
| `x` | wall (static) |
| `A` | primary (red car), horizontal, exits **right** |
| `B`–`Z` | other cars/trucks (length 2 or 3) |

## Coordinate transform

Classic Rush: primary moves **right** toward the exit.

Cargo Flow: Target moves **up** toward exit column 3.

**Clockwise 90°** maps:

- cell `(r,c)` → `(n-1-c, r)`
- right edge → top edge
- horizontal pieces → vertical (and vice versa)
- primary length-2 horizontal → Target vertical 1×2 on exit column

`OriginalOptimalMoves` is metadata only. After embed into 7×8 free space, the
puzzle may become much easier or harder — always re-solve with RUSH-004 BFS.

## Embedding (6×6 → 7×8)

Offset X is forced so Target lands on column 3:

`offsetX = 3 - primaryColAfterRotation`

Valid only when `offsetX ∈ {0,1}` (one free column). Otherwise
`InvalidTransform`.

Offset Y variants:

| Variant | offsetY |
|---------|--------:|
| FlushTop | 0 |
| ShiftDown1 | 1 |
| FlushBottom | 2 |

No artificial fillers in free cells for this POC.

## Validation pipeline

1. Parse record  
2. Transform + embed (≤3 variants)  
3. Reject ImmediateVictory / invalid layout  
4. Exact Cargo Flow solve (10s / 4M visited)  
5. Keep OptimalGestures in **8..20**  
6. Max one Accepted per `SourcePuzzleId`  
7. Layout fingerprint dedupe  
8. Replay PASS  

## CLI

```bash
go run ./cmd/cargoflow-transplant \
  --dataset testdata/rushdb/rush1000.txt \
  --output output/RUSH008_DatabaseTransplant_Pilot \
  --max-source 1000 \
  --min-optimal 8 \
  --max-optimal 20 \
  --count 12
```

## Output

Compatible with RUSH-006 Unity importer schema (`coordinateSpace: unity`),
plus optional `transplant` metadata on each level JSON.

## Known limitations

- Extra 7×8 space often reduces optimal length vs original Rush.
- Only puzzles whose primary row maps to CF exit column alignment embed.
- Cluster **size** is not a cluster **id**; uniqueness is by source line id.
- No 1×1 movables added in this POC.
