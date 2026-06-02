# Mutation Full-Scan Triage

This note records the first accepted-survivor baseline pass after the real advisory full scans and focused survivor-remediation batches.

## Source Reports

- Root: `artifacts/mutation/full/root/go-mutesting-agentic.json`, generated `2026-06-01T22:37:38Z`.
- `tools/goplint`: `artifacts/mutation/full/goplint/go-mutesting-agentic.json`, generated `2026-06-02T02:49:18Z`.

The source reports are ignored artifacts; the accepted survivor state is committed in:

- `tools/mutation/baselines/root-baseline.json`: 1,388 accepted escaped mutants.
- `tools/mutation/baselines/goplint-baseline.json`: 810 accepted escaped mutants.

## Root Profile

Summary from `artifacts/mutation/full/root/go-mutesting-summary.json`:

- Total: 9,389
- Killed: 5,224
- Escaped in source report: 1,636
- Accepted baseline after remediation: 1,388
- Not covered: 2,529
- MSI: 55.64%
- Covered-code MSI: 76.15%

Top accepted clusters after the current remediation batches:

- `pkg/invowkfile/generate.go`: 61 accepted
- `internal/app/deps/checks.go`: 58 accepted
- `pkg/invowkfile/dependency.go`: 57 accepted
- `internal/discovery/discovery_files.go`: 53 accepted
- `pkg/invowkfile/validation_structure_flags.go`: 52 accepted
- `pkg/invowkmod/invowkmod_edit.go`: 51 accepted
- `pkg/invowkfile/runtime.go`: 51 accepted
- `pkg/invowkfile/validation_structure_command.go`: 45 accepted
- `internal/config/types.go`: 41 accepted
- `pkg/invowkfile/validation_structure_deps.go`: 39 accepted

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
- Accepted baseline after remediation: 810
- Not covered: 357
- MSI: 67.70%
- Covered-code MSI: 74.37%

Top accepted clusters after the current remediation batches:

- `goplint/analyzer_windows_pitfalls.go`: 110 accepted
- `goplint/analyzer_validate_delegation.go`: 105 accepted
- `goplint/analyzer_constructor_validates_cfa.go`: 93 accepted
- `goplint/analyzer_constructor_validates.go`: 86 accepted
- `goplint/analyzer_boundary_request_validation.go`: 66 accepted
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

Second remediation batch:

- Add fixture coverage for `goplint/analyzer_windows_pitfalls.go` command-wait-delay duplicate suppression, including duplicate direct `exec.CommandContext(...).Run()`, repeated runner calls, and repeated prepared-command values.
- Add fixture coverage proving non-execution `*exec.Cmd` method calls are ignored.
- Add fixture coverage for volume-mount `String()`/`WriteString()` path exposure positives and nearby negative controls for unrelated mount types, custom writer methods, and non-standard `WriteString` signatures.
- Focused rerun: `artifacts/mutation/focused/goplint-windows-pitfalls/`, generated `2026-06-02T13:31:53Z`, with 625 total mutants, 409 killed, 56 not covered, 160 escaped, MSI 65.44%, and covered-code MSI 71.88%.
- The focused rerun proved 34 accepted `goplint/analyzer_windows_pitfalls.go` survivors killed and removed from the goplint baseline, dropping that file from 144 to 110 accepted mutants.
- The focused rerun also surfaced 50 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.

Third remediation batch:

- Add helper-level coverage for `goplint/analyzer_validate_delegation.go` alias-binding guard inputs, alias clearing after non-field rebinding, `var` value-spec alias chaining, and range-value alias tracking.
- Add helper-level coverage for delegated helper-function arguments, including direct receiver-field arguments, alias arguments, manually constructed values, and empty receiver names.
- Add helper-level coverage for delegated helper parameter discovery across direct `Validate()` calls, indexed parameters, range value variables, and range index variables.
- Focused rerun: `artifacts/mutation/focused/goplint-validate-delegation/`, generated `2026-06-02T13:47:32Z`, with 561 total mutants, 365 killed, 83 not covered, 113 escaped, MSI 65.06%, and covered-code MSI 76.36%.
- The focused rerun proved 19 accepted `goplint/analyzer_validate_delegation.go` survivors killed and removed from the goplint baseline, dropping that file from 124 to 105 accepted mutants.
- The focused rerun also surfaced 8 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.

Fourth remediation batch:

- Add helper-level coverage for `goplint/analyzer_boundary_request_validation.go` request/option parameter collection, error-return detection, assigned error-name parsing, error condition parsing, block termination detection, use detection, safe delegation, defaulting, and zero-literal recognition.
- Commit the helper coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/goplint-boundary-request/`, generated `2026-06-02T13:59:29Z`, with 335 total mutants, 243 killed, 23 not covered, 69 escaped, MSI 72.54%, and covered-code MSI 77.88%.
- The focused rerun proved 44 accepted `goplint/analyzer_boundary_request_validation.go` survivors killed and removed from the goplint baseline, dropping that file from 110 to 66 accepted mutants.
- The focused rerun also surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.

Fifth remediation batch:

- Add helper-level coverage for `goplint/analyzer_constructor_validates.go` directive alias disambiguation when multiple packages expose same-named types, pointer/named return-type resolution, and transitive validation through bare helper chains and same-type method helpers.
- Commit the helper coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/goplint-constructor-validates/`, generated `2026-06-02T14:15:54Z`, with 309 total mutants, 198 killed, 20 not covered, 91 escaped, MSI 64.08%, and covered-code MSI 68.51%.
- The focused rerun proved 18 accepted `goplint/analyzer_constructor_validates.go` survivors killed and removed from the goplint baseline, dropping that file from 104 to 86 accepted mutants.
- The focused rerun also surfaced 5 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.

Sixth remediation batch:

- Add generator contract coverage for `pkg/invowkfile/generate.go`, including root/command/implementation round trips, omission of empty optional blocks, runtime field variants, runtime-level dependency variants, and virtual filesystem access without named paths.
- Commit the generator coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-generate/`, generated `2026-06-02T14:31:17Z`, with 491 total mutants, 398 killed, 32 not covered, 61 escaped, MSI 81.06%, and covered-code MSI 86.71%.
- The focused rerun proved 64 accepted `pkg/invowkfile/generate.go` survivors killed and removed from the root baseline, dropping that file from 125 to 61 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkfile/generate.go`.

Seventh remediation batch:

- Add helper-level coverage for `pkg/invowkfile/validation_primitives.go` regex validation boundaries, invalid-regex cause wrapping, required-description delegation, overlapping alternation edge cases, escaped nesting depth, character-class quantifier handling, and brace quantifier counting.
- Commit the primitive-helper coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-validation-primitives/`, generated `2026-06-02T14:37:31Z`, with 205 total mutants, 142 killed, 27 not covered, 36 escaped, MSI 69.27%, and covered-code MSI 79.78%.
- The focused rerun proved 50 accepted `pkg/invowkfile/validation_primitives.go` survivors killed and removed from the root baseline, dropping that file from 83 to 33 accepted mutants.
- The focused rerun also surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Eighth remediation batch:

- Add contract coverage for `pkg/invowkmod/invowkmod.go`, including aggregate `Invowkmod` field errors, `ModuleRequirement` joined errors, subdirectory path edge reasons, validation issue formatting/addition, module path helpers, symlink escape rejection, and Windows drive-prefix classification.
- Commit the `invowkmod` coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod/`, generated `2026-06-02T14:48:48Z`, with 288 total mutants, 220 killed, 23 not covered, 45 escaped, MSI 76.39%, and covered-code MSI 83.02%.
- The focused rerun proved 48 accepted `pkg/invowkmod/invowkmod.go` survivors killed and removed from the root baseline, dropping that file from 80 to 32 accepted mutants.
- The focused rerun also surfaced 13 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Ninth remediation batch:

- Add resolver contract coverage for `internal/app/llmconfig/resolve.go`, including provider/load validation ordering, mode labels, validation joins, configured API/provider precedence, environment overrides, flag overrides, concurrency normalization, and option accessors.
- Commit the `llmconfig` coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-llmconfig-resolve/`, generated `2026-06-02T15:00:31Z`, with 206 total mutants, 170 killed, 14 not covered, 22 escaped, MSI 82.52%, and covered-code MSI 88.54%.
- The focused rerun proved 47 accepted `internal/app/llmconfig/resolve.go` survivors killed and removed from the root baseline, dropping that file from 68 to 21 accepted mutants.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Tenth remediation batch:

- Add dependency-resolution contract coverage for `internal/app/deps/deps.go`, including host and runtime short-circuit ordering, command dependency wrapper behavior, command discovery context propagation, command-scope lock fallback, direct-requirement matching, accessible/forbidden source decisions, source candidate de-duplication, command-info source/simple-name helpers, and missing-command message variants.
- Commit the `deps` coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-deps-deps/`, generated `2026-06-02T15:14:02Z`, with 298 total mutants, 250 killed, 13 not covered, 35 escaped, MSI 83.89%, and covered-code MSI 87.72%.
- The focused rerun proved 34 accepted `internal/app/deps/deps.go` survivors killed and removed from the root baseline, dropping that file from 64 to 30 accepted mutants.
- The focused rerun also surfaced 5 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

## Policy

The committed baseline accepts the current escaped set so blocking mode can distinguish new escapes from known historical survivors. Future survivor reduction should follow this loop:

1. Pick a high-value survivor cluster from the agentic report.
2. Add the smallest behavior test that kills the survivor.
3. Rerun the affected stable mutant ID.
4. Remove killed IDs from the accepted baseline, or run a reviewed baseline update after a broader profile completes.
5. Keep not-covered clusters visible until package-level tests or a documented high-assurance profile cover them.
