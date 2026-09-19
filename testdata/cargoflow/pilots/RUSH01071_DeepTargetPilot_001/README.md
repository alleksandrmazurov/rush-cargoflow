# RUSH-010.7.1 Deep Target control batch

Tracked control snapshot of the 12 accepted levels from
`output/RUSH01071_DeepTargetPilot_001`.

Included:

- exact pilot config (`pilot-config.json`);
- `BatchManifest.json`;
- `BoardDiversityReport.json`;
- 12 candidate level JSON files;
- 12 exact solution/replay JSON files;
- SHA-256 checksums (`SHA256SUMS.txt`).

The resumable `checkpoint.json`, progress log, and non-final pool are intentionally
not copied: they remain generated output and are not needed to identify or replay
the accepted control batch.

Human review applies only to Candidate_001, Candidate_004, Candidate_006, and
Candidate_008. See `data/calibration/rush01071_human_review.json`.

The reproducibility rerun (`RUSH01071_DeepTargetPilot_002`, configured with seed
`20260920`) is recorded under `reproducibility/`. It produced 12/12 exact
start-state repeats and no new families. `BoardMixConfig.Seed` is currently not
consumed by the boardmix pipeline, so this demonstrates deterministic rerun
stability, not independent seeded sampling.
