# Cargo Flow Generator POC (RUSH-005)

Offline candidate generation for Cargo Flow using the exact gesture solver as the
sole quantitative difficulty oracle.

## Strategy

**Nested corridor placement + exact-solver filter + solution-guided deepen**
(hybrid).

Why this approach for the POC:

- Original Rush uses annealing + unsolve toward *harder* Rush Hour positions;
  those assumptions (horizontal primary, right exit, no 1×1, reverse-search
  Unsolver) do not transfer cleanly to Cargo Flow.
- Pure random placement tended to produce either trivial optima (2–5) or
  permanently blocked exit columns (static walls on the Target column).
- The POC therefore:
  1. Places Target deep on exit column 3;
  2. Stacks **movable** 1×1 units on that column with off-column static pins;
  3. Fills the remaining inventory at random (never static on the exit column
     above Target — Target is vertically column-locked);
  4. Light scramble (not a difficulty metric);
  5. Exact BFS filter; if `Optimal < MinOptimal`, deepen by walking the short
     solution almost to Exit then scrambling into a farther reachable state and
     re-solving.
- Difficulty is **never** inferred from scramble/deepen length — only from
  `ExactOptimalGestures`.

Original `Generator` / `anneal` / `Unsolver` remain untouched for Rush Hour.

## Profiles

| Profile | 1x1 | 1x2H | 1x2V | 1x3H | 1x3V | Static |
|---------|----:|-----:|-----:|-----:|-----:|-------:|
| MediumDense | 5 | 2 | 2 | 1 | 1 | 4 |
| HardDense | 6 | 2 | 1 | 1 | 1 | 5 |
| HardMixed | 5 | 2 | 2 | 2 | 1 | 4 |

Corridor construction may consume some of the 1×1 budget and adds off-column
static pins. Piece count respects `MaxPieces = 18`.

## Filters

1. Valid layout / single Target / no overlaps  
2. Reject **ImmediateVictory** (`Target` can Exit immediately)  
3. Exact solve with budget (`TimeLimit`, `MaxVisited`)  
4. Reject unsolved / budget → `DifficultyUnknown`  
5. If `Optimal < MinOptimal`, deepen (bounded) and re-solve  
6. `OptimalGestures >= MinOptimal` (pilot default **10**) else `TooEasy`  
7. Replay PASS (stable `P01`… IDs so JSON load order matches indices)  
8. Layout fingerprint duplicate reject  
9. Occupancy Jaccard (+ horizontal mirror) ≥ 0.85 → `TooSimilar`

## CLI

```bash
go run ./cmd/cargoflow-generate \
  --count 10 \
  --min-optimal 10 \
  --seed 12345 \
  --max-attempts 300 \
  --output output/pilot
```

## Output

```
output/pilot/
  Candidate_001.json
  Candidate_001.solution.json
  ...
  BatchManifest.json
```

JSON schema matches RUSH-003 (`coordinateSpace: unity`). Solution docs include
`replayVerified: true`.

## Determinism

Same `GeneratorVersion` + config + `MasterSeed` ⇒ same attempt seeds and
accepted fingerprints (see `TestSameSeed_SameBatch`).

## Known limitations

- Acceptance rate depends on deepen; HardDense can still hit `DifficultyUnknown`.
- Similarity is occupancy Jaccard only (not full P7B).
- No Unity import / campaign promotion.
- Not a claim that ≥10 gestures equals “human hard”.
- `MaxPieces = 18` caps inventory (inherited from Rush).
