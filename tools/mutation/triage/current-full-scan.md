# Mutation Full-Scan Triage

This note records the first accepted-survivor baseline pass after the real advisory full scans and focused survivor-remediation batches.

## Source Reports

- Root: `artifacts/mutation/full/root/go-mutesting-agentic.json`, generated `2026-06-01T22:37:38Z`.
- `tools/goplint`: `artifacts/mutation/full/goplint/go-mutesting-agentic.json`, generated `2026-06-02T02:49:18Z`.

The source reports are ignored artifacts; the accepted survivor state is committed in:

- `tools/mutation/baselines/root-baseline.json`: 372 accepted escaped mutants.
- `tools/mutation/baselines/goplint-baseline.json`: 810 accepted escaped mutants.

## Root Profile

Summary from `artifacts/mutation/full/root/go-mutesting-summary.json`:

- Total: 9,389
- Killed: 5,224
- Escaped in source report: 1,636
- Accepted baseline after remediation: 372
- Not covered: 2,529
- MSI: 55.64%
- Covered-code MSI: 76.15%

Top accepted clusters after the current remediation batches:

- `pkg/invowkmod/invowkmod.go`: 13 accepted
- `internal/core/serverbase/base.go`: 10 accepted
- `internal/discovery/discovery_validate.go`: 10 accepted
- `internal/app/execute/orchestrator_validate.go`: 9 accepted
- `internal/discovery/discovery.go`: 9 accepted
- `pkg/invowkfile/dependency.go`: 9 accepted
- `pkg/invowkfile/validation_structure_command.go`: 9 accepted
- `pkg/invowkfile/virtual_filesystem.go`: 9 accepted
- `pkg/invowkmod/content_hash.go`: 9 accepted
- `pkg/invowkmod/resolver_validate.go`: 9 accepted
- `pkg/invowkmod/semver.go`: 9 accepted
- `internal/app/deps/filepaths.go`: 8 accepted
- `internal/discovery/discovery_commands.go`: 8 accepted
- `pkg/invowkfile/containerfile_path.go`: 8 accepted
- `pkg/invowkmod/operations_validate.go`: 8 accepted

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

Eleventh remediation batch:

- Add dependency-check contract coverage for `internal/app/deps/checks.go`, including container wrapper short-circuit/probe requirements, invalid custom-check result wrapping, regex syntax wrapping, host custom-check probe requirements, custom-check structured failures, container env early success, qualified command probe names, resolved command fallback alternatives, capability de-duplication, host capability messages, host env alternative trimming, and host env regex wrapping.
- Commit the `checks` coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-deps-checks/`, generated `2026-06-02T15:21:54Z`, with 261 total mutants, 226 killed, 11 not covered, 24 escaped, MSI 86.59%, and covered-code MSI 90.40%.
- The focused rerun proved 35 accepted `internal/app/deps/checks.go` survivor records killed and removed from the root baseline, dropping that file from 58 to 23 accepted mutants.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Twelfth remediation batch:

- Add value-type and dependency contract coverage for `pkg/invowkfile/dependency.go`, including binary/check/source error payloads, command dependency reference parsing and structured validation, dependency field-error preservation, custom-check script source/path resolution, custom-check direct/alternative projection, and optional expected-output validation.
- Commit the `dependency` coverage before running the focused mutation pass to avoid source-restore clobbering of uncommitted tests.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-dependency/`, generated `2026-06-02T15:31:26Z`, with 411 total mutants, 385 killed, 12 not covered, 14 escaped, MSI 93.67%, and covered-code MSI 96.49%.
- The focused rerun proved 48 accepted `pkg/invowkfile/dependency.go` survivor records killed and removed from the root baseline, dropping that file from 57 to 9 accepted mutants.
- The focused rerun also surfaced 5 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirteenth remediation batch:

- Extend dependency-check coverage for `internal/app/deps/checks.go`, including container wrapper success/failure payloads, custom-check script resolution errors, host/runtime custom-check analysis boundaries, validation message separators, bare container command probe names, distinct capability alternative sets, and host env wrapper payloads.
- Focused rerun: `artifacts/mutation/focused/root-deps-checks/`, generated `2026-06-02T23:50:42Z`, with 261 total mutants, 246 killed, 11 not covered, 4 escaped, MSI 94.25%, and covered-code MSI 98.40%.
- The focused rerun proved 20 additional accepted `internal/app/deps/checks.go` survivor records killed and removed from the root baseline, dropping that file from 23 to 3 accepted mutants.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Fourteenth remediation batch:

- Add filesystem-validation boundary coverage for `pkg/invowkfile/validation_filesystem.go`, including filename/control-character limits, containerfile/env-file path length boundaries, parent-segment rejection, filepath dependency alternative indexing, command dependency name length, portable absolute-path dialects, and Windows drive-letter byte boundaries.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-validation-filesystem/`, generated `2026-06-02T23:51:26Z`, with 151 total mutants, 150 killed, 0 not covered, 1 escaped, MSI 99.34%, and covered-code MSI 99.34%.
- The focused rerun proved 25 accepted `pkg/invowkfile/validation_filesystem.go` survivor records killed and removed from the root baseline, dropping that file from 26 to 1 accepted mutant.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkfile/validation_filesystem.go`.

Fifteenth remediation batch:

- Add CUE error-formatting coverage for `pkg/cueutil/error.go`, including redundant path-prefix trimming, multi-error validation blocks, pathless CUE errors, and numeric path segment boundaries.
- Focused rerun: `artifacts/mutation/focused/root-cueutil-error/`, generated `2026-06-02T23:56:55Z`, with 75 total mutants, 63 killed, 3 not covered, 9 escaped, MSI 84.00%, and covered-code MSI 87.50%.
- The focused rerun proved 16 accepted `pkg/cueutil/error.go` survivor records killed and removed from the root baseline, dropping that file from 22 to 6 accepted mutants.
- The focused rerun also surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Sixteenth remediation batch:

- Add semver resolver coverage for `pkg/invowkmod/semver.go`, including non-nil resolver construction, parsed constraint field preservation, integer-overflow parse errors, no-valid-version vs no-match resolve errors, and exact sorted/filtered version outputs.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-semver/`, generated `2026-06-03T00:05:35Z`, with 232 total mutants, 201 killed, 11 not covered, 20 escaped, MSI 86.64%, and covered-code MSI 90.95%.
- The focused rerun proved 5 accepted `pkg/invowkmod/semver.go` survivor records killed and removed from the root baseline, dropping that file from 21 to 16 accepted mutants.
- The focused rerun also surfaced 4 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Seventeenth remediation batch:

- Extend LLM resolution coverage for `internal/app/llmconfig/resolve.go`, including loader context/path propagation, configured provider/API model preservation, zero-timeout validation boundaries, unknown-mode validation, configured API-key env precedence, and invalid env-model errors.
- Focused rerun: `artifacts/mutation/focused/root-llmconfig-resolve/`, generated `2026-06-03T00:10:34Z`, with 206 total mutants, 183 killed, 11 not covered, 12 escaped, MSI 88.83%, and covered-code MSI 93.85%.
- The focused rerun proved 9 accepted `internal/app/llmconfig/resolve.go` survivor records killed and removed from the root baseline, dropping that file from 21 to 12 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `internal/app/llmconfig/resolve.go`.

Eighteenth remediation batch:

- Add implementation contract coverage for `pkg/invowkfile/implementation.go`, including optional field validation on otherwise-valid implementations, script source/file/interpreter validation, script read-error payloads and wrapping, non-container host-SSH behavior, empty dependency checks, exact parent traversal rejection, and inline script path lookup.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-implementation/`, generated `2026-06-03T00:19:04Z`, with 181 total mutants, 139 killed, 40 not covered, 2 escaped, MSI 76.80%, and covered-code MSI 98.58%.
- The focused rerun proved 19 accepted `pkg/invowkfile/implementation.go` survivor records killed and removed from the root baseline, dropping that file from 20 to 1 accepted mutant.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Nineteenth remediation batch:

- Add invowkmod edit contract coverage for `pkg/invowkmod/invowkmod_edit.go`, including read-error wrapping, non-missing read failures, empty-file append trimming, EOF requires-block removal, leading blank-line trimming, and first-duplicate removal ordering.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-edit/`, generated `2026-06-03T00:27:08Z`, with 168 total mutants, 160 killed, 0 not covered, 8 escaped, MSI 95.24%, and covered-code MSI 95.24%.
- The focused rerun proved 14 accepted `pkg/invowkmod/invowkmod_edit.go` survivor records killed and removed from the root baseline, dropping that file from 20 to 6 accepted mutants.
- The focused rerun also surfaced 2 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Twentieth remediation batch:

- Add invowkmod verify contract coverage for `pkg/invowkmod/verify.go`, including ambiguity validation aggregation, vendored hash evaluation payload preservation, empty-hash lock skipping, malformed-lock error wrapping, ignored vendor entry traversal, ambiguous-key error text, and mismatch error fields.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-verify/`, generated `2026-06-03T00:36:19Z`, with 168 total mutants, 102 killed, 62 not covered, 4 escaped, MSI 60.71%, and covered-code MSI 96.23%.
- The focused rerun proved 17 accepted `pkg/invowkmod/verify.go` survivor records killed and removed from the root baseline, dropping that file from 20 to 3 accepted mutants.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Twenty-first remediation batch:

- Add argument contract coverage for `pkg/invowkfile/argument.go`, including invalid argument type payload/error text, argument name value/reason payloads, delegated name and regex field errors, default-value type error wrapping, validation-pattern guard behavior, and runtime value validation for invalid argument types.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-argument/`, generated `2026-06-03T00:51:04Z`, with 84 total mutants, 81 killed, 1 not covered, 2 escaped, MSI 96.43%, and covered-code MSI 97.59%.
- The focused rerun proved 17 accepted `pkg/invowkfile/argument.go` survivor records killed and removed from the root baseline, dropping that file from 19 to 2 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkfile/argument.go`.

Twenty-second remediation batch:

- Add argument structure-validator coverage for `pkg/invowkfile/validation_structure_args.go`, including nil no-args output, argument name and description length diagnostics, unsafe-regex cause preservation, default-value compatibility diagnostics, duplicate argument names, required-after-optional ordering, and variadic-not-last state tracking.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-validation-structure-args/`, generated `2026-06-03T01:01:02Z`, with 120 total mutants, 79 killed, 32 not covered, 9 escaped, MSI 65.83%, and covered-code MSI 89.77%.
- The focused rerun proved 13 accepted `pkg/invowkfile/validation_structure_args.go` survivor records killed and removed from the root baseline, dropping that file from 19 to 6 accepted mutants.
- The focused rerun also surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Twenty-third remediation batch:

- Add command-scope coverage for `pkg/invowkmod/command_scope.go`, including joined target validation errors, local target validation, exact scope-decision payloads, same-module source fallback behavior, and complete discovery identity requirements for global and direct dependency visibility.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-command-scope/`, generated `2026-06-03T01:07:49Z`, with 109 total mutants, 79 killed, 30 not covered, 0 escaped, MSI 72.48%, and covered-code MSI 100.00%.
- The focused rerun proved all 19 accepted `pkg/invowkmod/command_scope.go` survivor records killed and removed from the root baseline, dropping that file from 19 to 0 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkmod/command_scope.go`.

Twenty-fourth remediation batch:

- Add command-discovery payload coverage for `internal/discovery/discovery_commands.go`, including invalid `SourceID` error payloads, ambiguity analysis boundaries, source-order stability, root/module/global command metadata, duplicate non-module suppression, parse-skip diagnostics, invalid command-name wrapping, and successful lookup diagnostic preservation.
- Focused rerun: `artifacts/mutation/focused/root-discovery-commands/`, generated `2026-06-03T01:19:48Z`, with 124 total mutants, 86 killed, 27 not covered, 11 escaped, MSI 69.35%, and covered-code MSI 88.66%.
- The focused rerun proved 10 accepted `internal/discovery/discovery_commands.go` survivor records killed and removed from the root baseline, dropping that file from 18 to 8 accepted mutants.
- The focused rerun also surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Twenty-fifth remediation batch:

- Add argument-input coverage for `internal/app/deps/input.go`, including required flags with valid values, empty optional typed flags, and exact `ArgumentValidationError` payload preservation for count and value failures.
- Focused rerun: `artifacts/mutation/focused/root-deps-input-current/`, generated `2026-06-04T09:48:00Z`, with 94 total mutants, 90 killed, 0 not covered, 4 escaped, MSI 95.74%, and covered-code MSI 95.74%.
- The focused rerun proved 14 accepted `internal/app/deps/input.go` survivor records killed and removed from the root baseline, dropping that file from 17 to 3 accepted mutants.
- The focused rerun also surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Twenty-sixth remediation batch:

- Add dependency type coverage for `internal/app/deps/types.go`, including command-dependency alternative validation ordering, dependency-message whitespace rejection, `DependencyFailure` payload and invalid-field handling, legacy failure aggregation, structured-failure cloning, and dependency-message normalization.
- Focused rerun: `artifacts/mutation/focused/root-deps-types-current/`, generated `2026-06-04T09:49:20Z`, with 108 total mutants, 61 killed, 47 not covered, 0 escaped, MSI 56.48%, and covered-code MSI 100.00%.
- The focused rerun proved all 15 accepted `internal/app/deps/types.go` survivor records killed and removed from the root baseline, dropping that file from 15 to 0 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `internal/app/deps/types.go`.

Twenty-seventh remediation batch:

- Add validation-option coverage for `pkg/invowkfile/validation_options.go`, including nil and empty-filepath defaults, file-derived workdir and default filesystem roots, custom filesystem preservation, and complete validation-context projection for workdir, platform, strict mode, filepath, and filesystem.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-validation-options-current/`, generated `2026-06-04T09:54:00Z`, with 40 total mutants, 39 killed, 1 not covered, 0 escaped, MSI 97.50%, and covered-code MSI 100.00%.
- The focused rerun proved all 17 accepted `pkg/invowkfile/validation_options.go` survivor records killed and removed from the root baseline, dropping that file from 17 to 0 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkfile/validation_options.go`.

Twenty-eighth remediation batch:

- Add flag value-type coverage for `pkg/invowkfile/flag.go`, including invalid type/name/shorthand payloads, invalid-flag field-error sentinels, formatted flag-name errors, valid default-value no-op behavior, wrapped default type errors, invalid-regex default handling, and regex mismatch defaults.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-flag-current/`, generated `2026-06-04T10:03:07Z`, with 89 total mutants, 85 killed, 2 not covered, 2 escaped, MSI 95.51%, and covered-code MSI 97.70%.
- The focused rerun proved 15 accepted `pkg/invowkfile/flag.go` survivor records killed and removed from the root baseline, dropping that file from 17 to 2 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/invowkfile/flag.go`.

Twenty-ninth remediation batch:

- Add runtime-preflight coverage for `pkg/invowkfile/runtime_preflight.go`, including parser fallback behavior, missing and unknown runtime names, native/virtual/container field splits, image-only versus containerfile-only container sources, nested runtime index paths, AST list filtering, and exact `runtimePreflightError` metadata.
- Focused rerun: `artifacts/mutation/focused/root-invowkfile-runtime-preflight-current/`, generated `2026-06-04T10:17:31Z`, with 97 total mutants, 80 killed, 13 not covered, 4 escaped, MSI 82.47%, and covered-code MSI 95.24%.
- The focused rerun proved 16 accepted `pkg/invowkfile/runtime_preflight.go` survivor records killed and removed from the root baseline, dropping that file from 17 to 1 accepted mutant.
- The focused rerun surfaced 3 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirtieth remediation batch:

- Add watcher contract coverage for `internal/watch/watcher.go`, including watch config field-error wrapping and labels, constructor debounce/default ignore behavior, backend add-error wrapping, helper pattern diagnostics, trailing-slash directory ignores, and `maybeAddDir` file/dir/ignored/missing-path behavior.
- Focused rerun: `artifacts/mutation/focused/root-watch-watcher-current/`, generated `2026-06-04T10:20:11Z`, with 265 total mutants, 143 killed, 75 not covered, 47 escaped, MSI 53.96%, and covered-code MSI 75.26%.
- The focused rerun proved 7 accepted `internal/watch/watcher.go` survivor records killed and removed from the root baseline, dropping that file from 18 to 11 accepted mutants.
- The focused rerun also surfaced 36 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-first remediation batch:

- Strengthen fake-backend watcher coverage with custom ignore capacity stress, plain-directory and trailing-slash directory ignore checks, and a deterministic manual debounce scheduler for skip-if-busy retry behavior.
- Focused rerun: `artifacts/mutation/focused/root-watch-watcher-current/`, generated `2026-06-04T10:27:53Z`, with 265 total mutants, 176 killed, 73 not covered, 16 escaped, MSI 66.42%, and covered-code MSI 91.67%.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-watch-watcher-current/single-reruns.tsv` proved 7 additional accepted `internal/watch/watcher.go` survivor records killed and removed from the root baseline, dropping that file from 11 to 4 accepted mutants.
- The focused rerun surfaced escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-second remediation batch:

- Add semver parser and constraint-boundary coverage for `pkg/invowkmod/semver.go`, including prerelease parsing without a patch segment, wrapped numeric overflow causes, and zero-major caret plus tilde cross-boundary exclusions.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-semver-current/`, generated `2026-06-04T10:29:21Z`, with 232 total mutants, 211 killed, 11 not covered, 10 escaped, MSI 90.95%, and covered-code MSI 95.48%.
- The focused rerun proved 7 accepted `pkg/invowkmod/semver.go` survivor records killed and removed from the root baseline, dropping that file from 16 to 9 accepted mutants.
- The focused rerun surfaced 1 escaped ID that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Thirty-third remediation batch:

- Add invowkmod value and path boundary coverage for `pkg/invowkmod/invowkmod.go`, including `A:` and `z:` drive-letter rejection through `SubdirectoryPath` and `isWindowsDrivePath`.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-invowkmod-current/`, generated `2026-06-04T10:36:03Z`, with 288 total mutants, 243 killed, 20 not covered, 25 escaped, MSI 84.38%, and covered-code MSI 90.67%.
- The focused rerun proved 3 accepted `pkg/invowkmod/invowkmod.go` survivor records killed and removed from the root baseline, dropping that file from 16 to 13 accepted mutants.
- The focused rerun surfaced 12 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-fourth remediation batch:

- Add operations validation coverage for `pkg/invowkmod/operations_validate.go`, including initialized validation result issue slices, exact `invowkmod.cue` path recording, symlink handling for entries named `invowk_modules`, file-not-directory `.invowkmod` suffix handling, and `Load` issue-message aggregation.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-operations-validate-current/`, generated `2026-06-04T10:41:46Z`, with 137 total mutants, 81 killed, 41 not covered, 15 escaped, MSI 59.12%, and covered-code MSI 84.38%.
- The focused rerun proved 7 accepted `pkg/invowkmod/operations_validate.go` survivor records killed and removed from the root baseline, dropping that file from 15 to 8 accepted mutants.
- The focused rerun surfaced 7 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-fifth remediation batch:

- Add portable container-name grammar coverage for `pkg/containerargs/container_name.go`, including max-length boundaries, exact invalid error payloads, ASCII start/body boundary bytes, `String()`, and invalid-name error formatting.
- Focused rerun: `artifacts/mutation/focused/root-containerargs-container-name-current/`, generated `2026-06-04T10:45:27Z`, with 60 total mutants, 59 killed, 0 not covered, 1 escaped, MSI 98.33%, and covered-code MSI 0.00% as emitted by the focused file run.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-containerargs-container-name-current/single-reruns.tsv` proved all 15 accepted `pkg/containerargs/container_name.go` survivor records killed and removed from the root baseline, dropping that file from 15 to 0 accepted mutants.
- The focused rerun surfaced 1 escaped branch-case artifact that was not in the accepted baseline for this file. It was not added during this shrink-only pass; reconcile it with the next full root mutation profile before any broader baseline refresh.

Thirty-sixth remediation batch:

- Add optional positive duration coverage for `pkg/types/duration.go`, including the one-nanosecond positive boundary, zero returned duration on errors, concrete invalid-duration values/reasons, `String()`, and invalid-duration error formatting.
- Focused rerun: `artifacts/mutation/focused/root-types-duration-current/`, generated `2026-06-04T10:50:46Z`, with 27 total mutants, 27 killed, 0 not covered, 0 escaped, MSI 100.00%, and covered-code MSI 0.00% as emitted by the focused file run.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-types-duration-current/single-reruns.tsv` proved all 9 accepted `pkg/types/duration.go` survivor records killed and removed from the root baseline, dropping that file from 9 to 0 accepted mutants.
- The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `pkg/types/duration.go`.

Thirty-seventh remediation batch:

- Add CUE parser error-boundary coverage for `pkg/cueutil/parse.go`, including default filename use for early size errors, schema compile failures, user compile failures, missing schema definitions, concrete and non-concrete validation formatting, and decode error formatting.
- Focused rerun: `artifacts/mutation/focused/root-cueutil-parse-current/`, generated `2026-06-04T10:54:56Z`, with 39 total mutants, 27 killed, 0 not covered, 12 escaped, MSI 69.23%, and covered-code MSI 0.00% as emitted by the focused file run.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-cueutil-parse-current/single-reruns.tsv` proved 4 accepted `pkg/cueutil/parse.go` survivor records killed and removed from the root baseline, dropping that file from 9 to 5 accepted mutants.
- Five validation/fallthrough guard survivors remained accepted because the mutated path still reaches equivalent formatted CUE/decode errors under the current public API. The focused rerun also surfaced escaped IDs that were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-eighth remediation batch:

- Add discovery-file coverage for `internal/discovery/discovery_files.go`, including skipped include entries continuing to later valid includes, vendored integrity errors propagating from local and configured include sources, `invowkfile.cue` discovery preserving non-zero source values, nil-metadata vendored parent short-circuiting, and global-marked discovered files without modules being ignored safely.
- Focused rerun: `artifacts/mutation/focused/root-discovery-files-current/`, generated `2026-06-04T11:03:41Z`, with 276 total mutants, 226 killed, 31 not covered, 19 escaped, MSI 81.88%, and covered-code MSI 92.24%.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-discovery-files-current/single-reruns.tsv` proved 9 accepted `internal/discovery/discovery_files.go` survivor records killed and removed from the root baseline, dropping that file from 14 to 5 accepted mutants.
- Five remaining accepted survivors are equivalent or equivalent-like under the current helper contracts: ambiguity keys are never length one, nil-child transitive diagnostics still fall through to no diagnostic, empty effective command namespaces return empty either way, and an empty global-ID map produces no diagnostics whether or not the early return runs. The focused rerun surfaced escaped IDs that were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Thirty-ninth remediation batch:

- Add lock-file parser error-contract coverage for `pkg/invowkmod/lockfile_parser.go`, including unknown-version payload fields, wrapped generated timestamp parse errors, wrapped CUE parse/validation/decode errors, non-concrete validation before decode, and v2 split-metadata `InvalidLockedModuleError` module-key preservation.
- Focused rerun: `artifacts/mutation/focused/root-invowkmod-lockfile-parser-current/`, generated `2026-06-04T11:10:40Z`, with 168 total mutants, 74 killed, 92 not covered, 2 escaped, MSI 44.05%, and covered-code MSI 97.37%.
- The focused rerun proved all 13 accepted `pkg/invowkmod/lockfile_parser.go` survivor records killed and removed from the root baseline, dropping that file from 13 to 0 accepted mutants.
- The focused rerun surfaced 2 escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Fortieth remediation batch:

- Extend LLM resolver mutation coverage for `internal/app/llmconfig/resolve.go`, including all known-mode validation, exact negative-timeout boundaries, changed API-key env precedence, invalid environment URL propagation through `Resolve`, invalid configured-provider validation, and duplicate-free configured API concurrency errors.
- Focused rerun: `artifacts/mutation/focused/root-llmconfig-resolve-current/`, generated `2026-06-04T11:19:56Z`, with 206 total mutants, 196 killed, 8 not covered, 2 escaped, MSI 95.15%, and covered-code MSI 98.99%.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-llmconfig-resolve-current/single-reruns.tsv` proved 10 accepted `internal/app/llmconfig/resolve.go` survivor records killed and removed from the root baseline, dropping that file from 12 to 2 accepted mutants.
- Two remaining accepted survivors are equivalent under the current public contract: clearing `Mode: ModeNone` from a returned `Resolved` literal preserves the zero-value result, and the branch-case artifact has no source line while all known mode labels and validations are covered. The focused rerun surfaced 0 escaped IDs that were not in the accepted baseline for this file, so the shrink-only pass did not need to defer any focused-only survivor reconciliation for `internal/app/llmconfig/resolve.go`.

Forty-first remediation batch:

- Reconcile the refreshed watcher focused report against the accepted baseline after the deterministic scheduler coverage landed.
- Focused rerun: `artifacts/mutation/focused/root-watch-watcher-current/`, generated `2026-06-04T10:27:53Z`, with 265 total mutants, 176 killed, 73 not covered, 16 escaped, MSI 66.42%, and covered-code MSI 91.67%.
- The focused rerun proved 2 additional accepted `internal/watch/watcher.go` survivor records killed and removed from the root baseline, dropping that file from 4 to 2 accepted mutants.
- The focused rerun still surfaced escaped IDs that were not in the accepted baseline for this file. Those were not added during this shrink-only pass; reconcile them with the next full root mutation profile before any broader baseline refresh.

Forty-second remediation batch:

- Reconcile `pkg/invowkfile/validation_primitives.go` against the current regex-safety boundary coverage.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-invowkfile-validation-primitives-current/single-reruns.tsv` proved all 12 accepted `pkg/invowkfile/validation_primitives.go` survivor records killed and removed from the root baseline, dropping that file to 0 accepted mutants.

Forty-third remediation batch:

- Reconcile `pkg/invowkfile/validation_structure_deps.go` against the current dependency-diagnostic coverage.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-invowkfile-validation-structure-deps-current/single-reruns.tsv` proved 8 unique stable IDs covering all 11 accepted `pkg/invowkfile/validation_structure_deps.go` survivor records killed and removed from the root baseline, dropping that file to 0 accepted mutants.

Forty-fourth remediation batch:

- Reconcile `pkg/invowkfile/validation_structure_flags.go` against the current flag-diagnostic severity coverage.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-invowkfile-validation-structure-flags-current/single-reruns.tsv` proved 2 unique stable IDs covering all 10 accepted `pkg/invowkfile/validation_structure_flags.go` survivor records killed and removed from the root baseline, dropping that file to 0 accepted mutants.

Forty-fifth remediation batch:

- Reconcile `pkg/invowkfile/runtime.go` against the current runtime, shebang, interpreter, and runtime-config boundary coverage.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-invowkfile-runtime-current/single-reruns.tsv` proved all 10 accepted `pkg/invowkfile/runtime.go` survivor records killed and removed from the root baseline, dropping that file to 0 accepted mutants.

Forty-sixth remediation batch:

- Reconcile `pkg/invowkfile/command.go` against the current command validation and implementation-selection coverage.
- Single-mutant reruns recorded in `artifacts/mutation/focused/root-invowkfile-command-current/single-reruns.tsv` proved 4 unique stable IDs covering 5 accepted `pkg/invowkfile/command.go` survivor records killed and removed from the root baseline, dropping that file from 10 to 5 accepted mutants.
- Five `pkg/invowkfile/command.go` survivor IDs still escaped targeted reruns and remain accepted in the baseline.

## Policy

The committed baseline accepts the current escaped set so blocking mode can distinguish new escapes from known historical survivors. Future survivor reduction should follow this loop:

1. Pick a high-value survivor cluster from the agentic report.
2. Add the smallest behavior test that kills the survivor.
3. Rerun the affected stable mutant ID.
4. Remove killed IDs from the accepted baseline, or run a reviewed baseline update after a broader profile completes.
5. Keep not-covered clusters visible until package-level tests or a documented high-assurance profile cover them.
