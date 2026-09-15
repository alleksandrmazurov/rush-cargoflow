# Cargo Flow JSON Contract (RUSH-003)

Offline development bridge between Unity Cargo Flow levels and the Rush solver.

## Schema version

`schemaVersion: 1`

## Level JSON

```json
{
  "schemaVersion": 1,
  "levelId": "level_01",
  "width": 7,
  "height": 8,
  "coordinateSpace": "unity",
  "exit": { "side": "top", "column": 3 },
  "canonicalGestures": 4,
  "source": "unity-asset:Level_01.asset",
  "pieces": [
    {
      "id": "target",
      "type": "target",
      "x": 3,
      "y": 2,
      "width": 1,
      "height": 2,
      "movable": true
    }
  ]
}
```

### Piece types

| type | meaning | footprint |
|------|---------|-----------|
| `target` | Target cargo | 1×2 vertical |
| `movable1x1` | free 1×1 | 1×1 |
| `movable1x2` | length-2 block | 1×2 or 2×1 (from width/height) |
| `movable1x3` | length-3 block | 1×3 or 3×1 |
| `static1x1` | immovable | 1×1 |

Orientation is implied by `width`/`height`.

## Coordinate system (Unity = JSON source of truth)

From `GridRules` / `GridView`:

- `x` increases to the right
- `y` increases **toward the exit** (world +Z)
- Target leaves the board when `y >= height`
- `ExitColumn` on `LevelDefinition` is the gate column

Verified production value:

| | value |
|--|--|
| Unity Exit Column | **3** |
| Rush Exit Column | **3** |
| Mapping | **identity** (columns match) |

### Unity → Rush internal conversion

Rush Cargo Flow keeps row **0 = exit side** (solver top):

```
rushX = unityX
rushY = height - unityY - pieceHeight
```

Inverse:

```
unityX = rushX
unityY = height - rushY - pieceHeight
```

Vertical motion sign flips across the bridge:

- Unity `+y` (toward exit) ↔ Rush negative row steps / Exit gesture

## Solution JSON

```json
{
  "schemaVersion": 1,
  "levelId": "level_01",
  "solved": true,
  "optimal": true,
  "optimalGestures": 4,
  "visitedStates": 33,
  "elapsedMs": 1,
  "timedOut": false,
  "budgetExceeded": false,
  "replayPass": true,
  "fingerprint": "...",
  "moves": [
    {
      "pieceId": "M6",
      "fromX": 1,
      "fromY": 1,
      "toX": 0,
      "toY": 1,
      "axis": "horizontal",
      "distance": 1,
      "isExit": false
    }
  ]
}
```

`optimal: true` only when solved without timeout/budget interruption (true shortest gesture path).

## CLI

```bash
# Unity .asset → JSON (dev helper; Editor exporter is preferred)
go run ./cmd/unity-asset-to-json path/to/Level_01.asset out.json 4

# Solve
go run ./cmd/cargoflow-solve --input out.json --output solution.json
go run ./cmd/cargoflow-solve --input out.json --time-limit 8s --max-visited 1500000
```

## Unity Editor exporter

Menu:

- `CargoFlow → Diagnostics → Level Tools → Export Level for Rush Solver`
- `CargoFlow → Diagnostics → Level Tools → Export Production Pilot (1,10,20,28,30)`

Read-only: writes JSON only; does not modify LevelData / catalog / medals.

## Fingerprint

`fingerprint` / `FingerprintUnity()` dumps occupancy in Unity space with exit at the **top** of the text dump (high Y first) for human comparison.
