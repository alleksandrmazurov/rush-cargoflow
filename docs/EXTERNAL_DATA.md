# External data provenance

## Michael Fogleman Rush Hour full database

| Field | Value |
|-------|-------|
| Source name | Michael Fogleman Rush Hour puzzle database |
| Official page | https://www.michaelfogleman.com/rush/ |
| Official download URL (working) | https://www.michaelfogleman.com/static/rush/rush.txt.gz |
| Alternate mentioned on page | `rush.txt.bz2` (same site; path may vary) |
| Preview corpus (in-repo) | `testdata/rushdb/rush1000.txt` |
| Retrieval date | 2026-09-15 |
| Local filename (compressed) | `data/external/rush/rush.txt.gz` |
| Local filename (uncompressed) | `data/external/rush/rush.txt` |
| Compressed size | 34,122,602 bytes (~33 MiB) |
| Uncompressed size | 115,243,649 bytes (~110 MiB) |
| Record count | 2,577,412 interesting 6×6 puzzles |
| SHA256 (gzip) | `844e00940a9dc8f8a68a332f447d1cccbbc239cf42e04ea49cbda4bb2e5fdf44` |
| SHA256 (txt) | `fca9f04db491415ac257416cd25b304670f2859f5f1bb1f4948c7f30ba14626f` |

### Licensing / redistribution

- The **Go/C++ software** in upstream `fogleman/rush` is MIT-licensed (see repository `LICENSE.md`).
- The author states on the project page that the code is open source with a permissive license and that **the resulting database is available for download**.
- A **separate, explicit license file dedicated solely to the puzzle database file** was **not** found at retrieval time.
- Therefore: **do not invent a database license**. Treat redistribution status as **author-published download; separate DB license not explicitly attached**. Keep the full database **out of git** until redistribution policy is confirmed for your use case.

### Local layout (not committed)

```text
data/external/rush/rush.txt.gz
data/external/rush/rush.txt
data/external/rush/PROVENANCE.json
data/cache/curator/          # solve cache (local)
```

Fetch:

```bash
go run ./cmd/rush-db-fetch --out data/external/rush
```

### Format reminder

Each line: `<originalOptimalMoves> <36-char board> <clusterSize>`

Board chars: `o` empty, `x` wall, `A` primary, `B`–`Z` other pieces.
