## ADDED Requirements

### Requirement: Deterministic property-based tests under mutation profiles
Mutation profiles SHALL run property-based tests deterministically so that each mutant's kill or escape status is reproducible against the stable-ID baseline. The mutation wrapper SHALL set a fixed `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and a bounded `RAPID_SHRINKTIME` for every mutated test run.

#### Scenario: Mutant status is reproducible
- **WHEN** the same mutant is run twice under the same profile
- **THEN** the rapid tests SHALL use the same seed both times, and the mutant's kill or escape status SHALL be identical

#### Scenario: No failure files are written
- **WHEN** a mutant makes a rapid test fail
- **THEN** no file SHALL be written under any package's `testdata/rapid/` directory

#### Scenario: Shrinking is bounded
- **WHEN** a rapid test fails under a mutation profile
- **THEN** shrinking SHALL stop within the configured `RAPID_SHRINKTIME`

#### Scenario: Packages outside the curated manifest
- **WHEN** rapid tests are added to packages absent from `tools/mutation/root-packages.txt`
- **THEN** the formal-verification documentation SHALL state that those tests add no killers to the full profile until the manifest is changed deliberately
