# Mutation Full-Scan Triage

This note records the first accepted-survivor baseline pass after the real advisory full scans.

## Source Reports

- Root: `artifacts/mutation/full/root/go-mutesting-agentic.json`, generated `2026-06-01T22:37:38Z`.
- `tools/goplint`: `artifacts/mutation/full/goplint/go-mutesting-agentic.json`, generated `2026-06-02T01:00:44Z`.

The source reports are ignored artifacts; the accepted survivor state is committed in:

- `tools/mutation/baselines/root-baseline.json`: 1,636 escaped mutants.
- `tools/mutation/baselines/goplint-baseline.json`: 946 escaped mutants.

## Root Profile

Summary from `artifacts/mutation/full/root/go-mutesting-summary.json`:

- Total: 9,389
- Killed: 5,224
- Escaped: 1,636
- Not covered: 2,529
- MSI: 55.64%
- Covered-code MSI: 76.15%

Top escaped clusters:

- `pkg/invowkfile/generate.go`: 125 escaped
- `pkg/invowkfile/validation_primitives.go`: 83 escaped
- `pkg/invowkmod/invowkmod.go`: 80 escaped
- `internal/app/llmconfig/resolve.go`: 68 escaped
- `internal/app/deps/deps.go`: 64 escaped
- `internal/app/deps/checks.go`: 63 escaped
- `pkg/invowkfile/dependency.go`: 57 escaped
- `internal/discovery/discovery_files.go`: 53 escaped
- `pkg/invowkfile/validation_structure_flags.go`: 52 escaped
- `pkg/invowkfile/runtime.go`: 51 escaped

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

## Goplint Profile

Summary from `artifacts/mutation/full/goplint/go-mutesting-summary.json`:

- Total: 3,978
- Killed: 2,675
- Escaped: 946
- Not covered: 357
- MSI: 67.24%
- Covered-code MSI: 73.87%

Top escaped clusters:

- `./goplint/analyzer_windows_pitfalls.go`: 144 escaped
- `./goplint/analyzer_validate_delegation.go`: 124 escaped
- `./goplint/analyzer_boundary_request_validation.go`: 110 escaped
- `./goplint/analyzer_constructor_validates.go`: 106 escaped
- `./goplint/analyzer_constructor_validates_cfa.go`: 96 escaped
- `./goplint/analyzer_cross_platform_path.go`: 60 escaped
- `./goplint/analyzer_structural.go`: 56 escaped
- `./goplint/analyzer.go`: 42 escaped
- `./goplint/analyzer_enum_sync.go`: 38 escaped
- `./goplint/analyzer_cast_validation.go`: 33 escaped

Top not-covered clusters:

- `./goplint/analyzer_validate_delegation.go`: 83 not covered
- `./goplint/analyzer_windows_pitfalls.go`: 62 not covered
- `./goplint/analyzer_boundary_request_validation.go`: 48 not covered
- `./goplint/analyzer_constructor_validates_cfa.go`: 25 not covered
- `./goplint/analyzer_structural.go`: 20 not covered
- `./goplint/analyzer_constructor_validates.go`: 18 not covered
- `./goplint/analyzer_enum_sync.go`: 18 not covered
- `./goplint/analyzer_path_domain.go`: 15 not covered
- `./goplint/analyzer_test_home_env.go`: 14 not covered
- `./goplint/analyzer.go`: 12 not covered

First remediation batch:

- Add helper-level tests for constructor matching, exact constructor priority, interface-return constructors, and generic type-key matching.
- Add helper-level tests for discarded `Validate()` result indexing in assignments and value specs, including absent-call and out-of-range mapping cases.

## Policy

The committed baseline accepts the current escaped set so blocking mode can distinguish new escapes from known historical survivors. Future survivor reduction should follow this loop:

1. Pick a high-value survivor cluster from the agentic report.
2. Add the smallest behavior test that kills the survivor.
3. Rerun the affected stable mutant ID.
4. Remove killed IDs from the accepted baseline, or run a reviewed baseline update after a broader profile completes.
5. Keep not-covered clusters visible until package-level tests or a documented high-assurance profile cover them.
