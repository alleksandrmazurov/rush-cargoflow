# Cargo Flow Solver POC (RUSH-002)

Proof-of-concept adaptation of the Rush Hour core so the solver can model and
solve **one** Cargo Flow puzzle under real gesture semantics.

Original Rush Hour mode remains the default and stays regression-tested.

## Modes

| Ruleset | Constant | Meaning |
|---------|----------|---------|
| Original Rush Hour | `RulesOriginalRush` (default) | Classic 6x6-style Rush Hour |
| Cargo Flow | `RulesCargoFlow` | 7x8 POC rules |

Selected via `Board.Rules`. `NewBoard` / existing APIs keep Original Rush.
Cargo Flow boards are created with `NewCargoFlowBoard`.

## Coordinate system

- Board size: **width = 7**, **height = 8**
- Index: `position = row*7 + col`
- **Row 0 = TOP** (exit side), row 7 = bottom
- **Col 0 = LEFT**, col 6 = right
- Exit: **1 cell**, above the board, column **`CargoFlowExitCol = 3`**
  (center of a 7-wide grid, 0-based)

Unity assets were **not** inspected in this POC (no Unity integration).
Column 3 is the documented POC assumption for a centered top exit.

## Gesture definition

One solver `Move` = **one player drag/gesture**.

If a block can slide 1, 2, or 3 cells along a free axis, the engine emits
**three successor transitions**, each with cost **1** — not three unit steps
forced as three gestures.

A gesture never turns a corner; 1x1 must use two gestures to change axis.

## Piece rules

| Piece | Size | Motion |
|-------|------|--------|
| Target (`T`) | vertical 1x2 | vertical only; may Exit |
| Long block (`A`–`Z`) | 1x2 / 1x3 (axis-aligned) | along long axis only |
| Movable 1x1 (`a`–`z`) | 1x1 | horizontal **or** vertical per gesture |
| Static (`x`) | 1x1 | never moves; blocks collision |

Static blockers are stored as `Board.Walls` (same as Rush walls).

## Target / Exit / Victory

Victory representation: **terminal Exit move** (`Move.Exit = true`).

- Target must be on exit column 3
- Cells strictly above Target in that column must be empty
- Applying Exit sets `board.won = true` without storing half-off-board geometry
- Undo clears `won`

Exit is itself one gesture (can cover multi-cell travel off the top).

## ASCII legend (`NewCargoFlowBoard`)

```
.  empty
x  static
T  Target (vertical 1x2)
A-Z long movable (not T)
a-z movable 1x1
```

## POC fixture

```
.......
BBBaHH.
...b...
...T...
...T...
.VVV...
...C...
...C.x.
```

Includes Target, two 1x1s, horizontal longs (`BBB`,`HH`,`VVV`), vertical long (`C`),
and static `x`.

### Optimal solution (4 gestures)

Verified by IDA* minimizing gesture count:

1. `H H+1` — free the right side of `a`
2. `a H+1` — clear exit column row 1
3. `b H-1` — clear exit column row 2
4. `T -> Exit`

Baseline metrics from `cmd/cargoflow-smoke`:

- Solved: true
- Optimal gestures: **4**
- Visited states: **154** (memo size; may vary slightly with search internals only if code changes)

## Commands

```bash
go test .
go run ./cmd/rush-smoke
go run ./cmd/cargoflow-smoke
```

## Known limitations

- No generator adaptation
- No Unity / JSON / campaign tooling
- Piece permutation symmetry not canonicalized
- Display `Board.String()` still uses A/B/C index letters (Labels used in solution formatting)
- Exit column fixed to 3 without Unity verification
- `cmd/enumerate` / `cmd/unsolver` remain legacy-broken
- Original Rush static impossibility analyzer is skipped for Cargo Flow
