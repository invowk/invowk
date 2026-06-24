## Why

`go-mutesting v2.7.1` is available and replaces confusing per-mutant terminal labels: killed mutants now print `KILLED` and surviving mutants now print `ESCAPED` instead of the older `PASS`/`FAIL` inversion. Invowk's scripts do not appear to parse those old labels directly, but mutation triage documentation and agent guidance currently explain the old semantics and should be updated with the tool bump.

## What Changes

- Update the pinned `go-mutesting` Go tool dependency from `v2.7.0` to `v2.7.1`.
- Update version-pinning guidance and any wrapper/version checks that reference the mutation tool version.
- Update mutation triage documentation and agent-facing guidance so terminal label semantics use `KILLED` and `ESCAPED`.
- Verify mutation wrapper tests and a lightweight dry-run/focused command path without requiring a full mutation scan.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `mutation-testing`: Updates the pinned mutation-testing toolchain and report-label semantics contract.

## Impact

- Root `go.mod` / `go.sum` tool dependency entries.
- `.agents/rules/version-pinning.md` and mutation-related documentation.
- `tools/mutation/triage/current-full-scan.md` or successor triage notes that mention v2.7.0 label interpretation.
- `scripts/mutation.sh`, `scripts/test_mutation.sh`, Make targets, and mutation CI workflows if any version checks or output expectations need adjustment.
