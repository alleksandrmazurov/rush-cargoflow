# Pilot results (RUSH-003)

| Level | Pieces(mov+target)/walls | Canonical | Optimal | Visited | Elapsed | Replay | Status |
|-------|--------------------------|-----------|---------|---------|---------|--------|--------|
| 01 | 9 / 0 | 4 | 4 | 33 | ~1ms | PASS | OK |
| 10 | 9 / 5 | 8 | 6 | 2850 | ~5ms | PASS | OK |
| 15 | 10 / 4 | 9 | 8 | 1875 | ~3ms | PASS | OK |
| 20 | 11 / 3 | 12 | 9 | 884811 | ~2.4s | PASS | OK |
| 25 | 13 / 2 | 11 | 9 | 842395 | ~2.2s | PASS | OK |
| 28 | 15 / 15 | 20 | — | 1500000 | ~2.7s | SKIP | BUDGET_EXCEEDED (1.5M) |
| 30 | 16 / 8 | 24 | 21 | 3166197 | ~17s | PASS | OK (needs ~4M visited) |

Unity ExitColumn = 3; Rush ExitCol = 3 (identity).
