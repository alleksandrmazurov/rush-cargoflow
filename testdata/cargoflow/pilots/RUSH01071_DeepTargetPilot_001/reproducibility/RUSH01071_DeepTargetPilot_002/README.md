# RUSH-010.7.1 reproducibility rerun

Second run with seed `20260920` and otherwise unchanged production settings.

Result:

- 12/12 accepted;
- exact solve/replay/Unity batch validation passed;
- 8 valid causal expansions;
- 9 candidates at Target row 5+;
- 5 candidates at Target row 6;
- 12/12 exact start-state repeats against the control;
- 12/12 FamilyId overlap;
- 0 genuinely new states and 0 new families.

The boardmix pipeline currently does not use `BoardMixConfig.Seed`. This rerun
therefore verifies deterministic stability, not an independent seeded sample.
Duplicate candidate JSON/solution files are not stored a second time; exact
fingerprints and pairings are in `PilotComparisonReport.json`.
