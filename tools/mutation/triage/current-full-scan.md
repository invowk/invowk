# Mutation Full-Scan Triage

This note records the first accepted-survivor baseline pass after the real advisory full scans and the first focused survivor-remediation batch.

## Source Reports

- Root: `artifacts/mutation/full/root/go-mutesting-agentic.json`, generated `2026-06-01T22:37:38Z`.
- `tools/goplint`: `artifacts/mutation/full/goplint/go-mutesting-agentic.json`, generated `2026-06-02T02:49:18Z`.

The source reports are ignored artifacts; the accepted survivor state is committed in:

- `tools/mutation/baselines/root-baseline.json`: 1,631 accepted escaped mutants.
- `tools/mutation/baselines/goplint-baseline.json`: 925 accepted escaped mutants.

## Root Profile

Summary from `artifacts/mutation/full/root/go-mutesting-summary.json`:

- Total: 9,389
- Killed: 5,224
- Escaped in source report: 1,636
- Accepted baseline after remediation: 1,631
- Not covered: 2,529
- MSI: 55.64%
- Covered-code MSI: 76.15%

Top accepted clusters after the first remediation batch:

- `pkg/invowkfile/generate.go`: 125 accepted
- `pkg/invowkfile/validation_primitives.go`: 83 accepted
- `pkg/invowkmod/invowkmod.go`: 80 accepted
- `internal/app/llmconfig/resolve.go`: 68 accepted
- `internal/app/deps/deps.go`: 64 accepted
- `internal/app/deps/checks.go`: 58 accepted
- `pkg/invowkfile/dependency.go`: 57 accepted
- `internal/discovery/discovery_files.go`: 53 accepted
- `pkg/invowkfile/validation_structure_flags.go`: 52 accepted
- `pkg/invowkfile/runtime.go`: 51 accepted

Top not-covered clusters:

- `pkg/containerargs/container_specs.go`: 281 not covered
- `internal/config/types.go`: 96 not covered
- `internal/watch/watcher.go`: 84 not covered
- `internal/app/llmconfig/resolve.go`: 82 not covered
- `pkg/invowkmod/verify.go`: 75 not covered
- `internal/discovery/discovery_files.go`: 72 not covered
- `internal/app/deps/types.go`: 71 not covered
- `pkg/invowkmod/lockfile_parser.go`: 69 not covered
- `pkg/invowkfile/runtime.go`: 67 not covered
- `pkg/invowkfile/validation_structure_deps.go`: 66 not covered

First remediation batch:

- Strengthen `internal/app/deps/checks.go` tests for non-zero expected custom-check exit codes.
- Strengthen invalid custom-check dependency assertions so validation-detail paths are the oracle, not merely any later dependency error.
- Add direct coverage for custom-check interpreter target string/validation behavior, host/runtime analysis runtime selection, and nil diagnostic reporter safety.
- Targeted reruns proved five `internal/app/deps/checks.go` survivors killed and removed from the root baseline: `d1e0c62621ee382f2c63abf340d5ab5c`, `3516b642d6028196cfe6a996ee67c7c4`, `303b1ee2dffb0848c8c6a731f895c8ca`, `ad103cd5f1e62744d9da82901aeb861a`, and `c473885f743fc4bfb583bc34c7ebdf1a`.

## Goplint Profile

Summary from `artifacts/mutation/full/goplint/go-mutesting-summary.json`:

- Total: 3,978
- Killed: 2,693
- Escaped in corrected source report: 928
- Accepted baseline after remediation: 925
- Not covered: 357
- MSI: 67.70%
- Covered-code MSI: 74.37%

Top accepted clusters after the first remediation batch:

- `goplint/analyzer_windows_pitfalls.go`: 144 accepted
- `goplint/analyzer_validate_delegation.go`: 124 accepted
- `goplint/analyzer_boundary_request_validation.go`: 110 accepted
- `goplint/analyzer_constructor_validates.go`: 104 accepted
- `goplint/analyzer_constructor_validates_cfa.go`: 93 accepted
- `goplint/analyzer_cross_platform_path.go`: 60 accepted
- `goplint/analyzer_structural.go`: 56 accepted
- `goplint/analyzer.go`: 38 accepted
- `goplint/analyzer_enum_sync.go`: 38 accepted
- `goplint/analyzer_cast_validation.go`: 33 accepted

Top not-covered clusters:

- `goplint/analyzer_validate_delegation.go`: 83 not covered
- `goplint/analyzer_windows_pitfalls.go`: 62 not covered
- `goplint/analyzer_boundary_request_validation.go`: 48 not covered
- `goplint/analyzer_constructor_validates_cfa.go`: 25 not covered
- `goplint/analyzer_structural.go`: 20 not covered
- `goplint/analyzer_constructor_validates.go`: 18 not covered
- `goplint/analyzer_enum_sync.go`: 18 not covered
- `goplint/analyzer_path_domain.go`: 15 not covered
- `goplint/analyzer_test_home_env.go`: 14 not covered
- `goplint/analyzer.go`: 12 not covered

First remediation batch:

- Add helper-level tests for constructor matching, exact constructor priority, interface-return constructors, and generic type-key matching.
- Add helper-level tests for discarded `Validate()` result indexing in assignments and value specs, including absent-call and out-of-range mapping cases.
- Add validate-usage analyzer fixture coverage proving non-`Validate` selectors on validatable types are ignored.
- Targeted reruns proved three corrected goplint survivors killed and removed from the goplint baseline: `1929608a0a10ab5a308df2fd9da9aac2`, `6ba6215e0ffe9d11859f6e4afe35b732`, and `f80395f432f965d70a921170d34bff76`.
- The `35c34fb880bcea25305f58ababa44806` untyped-receiver guard mutant still escaped after a focused rerun and remains accepted; it is equivalent with the current helper because `hasValidateMethod(nil)` returns false.

## Policy

The committed baseline accepts the current escaped set so blocking mode can distinguish new escapes from known historical survivors. Future survivor reduction should follow this loop:

1. Pick a high-value survivor cluster from the agentic report.
2. Add the smallest behavior test that kills the survivor.
3. Rerun the affected stable mutant ID.
4. Remove killed IDs from the accepted baseline, or run a reviewed baseline update after a broader profile completes.
5. Keep not-covered clusters visible until package-level tests or a documented high-assurance profile cover them.
