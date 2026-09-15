# RUSH-001 Baseline

Fork of [fogleman/rush](https://github.com/fogleman/rush) prepared as an offline
Rush Hour solver/generator baseline for a future Cargo Flow adapter.

**This baseline does not change Rush Hour rules or add Cargo Flow mechanics.**

## Original project structure

| Path | Role |
|------|------|
| `*.go` (root) | Core Go library: board model, solver, generator, enumerator, graph, render |
| `cmd/` | Assorted CLI utilities (solve, generate, enumerate, render, …) |
| `cpp/` | Separate C++ universe enumerator (legacy research tool) |
| `server/` | Python Flask API serving random puzzles from SQLite |
| `web/` | Browser player (JS/HTML/CSS) |

### Core library files

| File | Role |
|------|------|
| `model.go` | Board/piece/move model, parsing, legal moves, copy/validate |
| `solver.go` | IDA*-style depth-limited search with memoization |
| `generator.go` | Puzzle generation via annealing + unsolve |
| `graph.go` | State-graph / DOT export helpers |
| `enumerator.go` | Exhaustive layout enumeration helpers |
| `config.go` | Size limits (`MaxPieces`, board size bounds) |
| `static.go` | Static impossibility / blocked-square analysis |
| `unsolver.go` | Reverse search to harden puzzles |
| `anneal.go` | Simulated annealing for generation |
| `memo.go` | Visited-state memo table |
| `render.go` | PNG rendering via `fogleman/gg` (optional for solver use) |
| `util.go` | Small helpers |

## CORE vs LEGACY / OPTIONAL

### CORE (needed for future Cargo Flow adaptation)

- Root Go package `github.com/fogleman/rush`
- Solver + board model + move generation
- Generator / unsolver / static analyzer (generation pipeline)
- `cmd/solve`, `cmd/example`, `cmd/rush-smoke` (validation)

### LEGACY / OPTIONAL (ignore for now)

- `cpp/` — full-universe C++ research code
- `server/` — Flask + SQLite web API
- `web/` — online player UI
- `cmd/enumerate` — **does not build** (local `Enumerator` conflicts with library type via dot-import)
- `cmd/unsolver` — **does not build** (API mismatch: `Unsolve` returns 2 values)
- PNG rendering / macOS font paths in `render.go` — not required for solve/generate
- Other `cmd/*` utilities — useful demos, not required for the library baseline

## Module / Go version

- `go.mod` was **missing**; created as:

  ```text
  module github.com/fogleman/rush
  ```

  Module path keeps existing `github.com/fogleman/rush` imports (no mass rewrite).
  Git remote of this fork: `github.com/alleksandrmazurov/rush-cargoflow`.

- Required Go: **1.26+** (pulled by `golang.org/x/image` via `fogleman/gg`)
- Verified with: Go **1.27.1** (`windows/386` in the baseline environment)

## Build commands

```bash
go mod tidy
go build .
go build ./cmd/solve ./cmd/example ./cmd/rush-smoke
```

Core-only tests:

```bash
go test .
```

Full tree (expected to fail on legacy cmds):

```bash
go test ./...
```

Known failing packages (legacy, not core):

- `./cmd/enumerate`
- `./cmd/unsolver`

## Smoke command

```bash
go run ./cmd/rush-smoke
```

Expected output metrics:

```text
solved: true
moves: 9
steps: 21
visited states: 66
```

Puzzle (same as `cmd/forty` level 1):

```text
..B.CC
..B...
AAB...
DDD..E
.....E
.....E
```

## Known puzzle baseline results

| Puzzle | Moves | Steps | Memo (visited) | Notes |
|--------|------:|------:|---------------:|-------|
| simple (`..B` / `AAB`) | 2 | 7 | 4 | Minimal blocked exit |
| forty level 1 | 9 | 21 | 66 | Primary smoke baseline |
| `cmd/example` hard | 49 | 93 | 12266 | Documented in `cmd/example` |

Exit semantics (unchanged Rush Hour): primary piece `A` is horizontal; target is the rightmost cells of its row.

## Hygiene notes

- Do not commit binaries (`*.exe`), test binaries, PNG dumps, IDE settings, or secrets.
- See root `.gitignore`.

## Explicit non-goals (next stage)

Not present / not started in this baseline:

- 7×8 board assumptions
- Cargo Flow naming
- movable 1×1 pieces
- vertical target exit
- Unity integration
- JSON exporter/importer
- similarity logic
