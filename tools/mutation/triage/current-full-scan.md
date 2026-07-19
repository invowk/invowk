# Mutation Full-Scan Triage

This note records the first accepted-survivor baseline pass after the real advisory full scans and focused survivor-remediation batches.

## Terminal Label Versioning

Current `go-mutesting v2.7.1` terminal output uses `KILLED` for mutants caught by
tests and `ESCAPED` for mutants that survived. This triage note also preserves
historical evidence from `go-mutesting v2.7.0`, where terminal `PASS` meant a
killed mutant and terminal `FAIL` meant an escaped mutant. Treat every
`PASS`/`FAIL` interpretation in this file as historical v2.7.0 evidence, and
use machine-readable report fields or stable mutant IDs as the durable source of
truth for current automation.

## Source Reports

- Root: `artifacts/mutation/full/root/go-mutesting-agentic.json`, generated `2026-06-01T22:37:38Z`.
- `tools/goplint`: `artifacts/mutation/full/goplint/go-mutesting-agentic.json`, generated `2026-06-02T02:49:18Z`.

The source reports are ignored artifacts; the accepted survivor state is committed in:

- `tools/mutation/baselines/root-baseline.json`: 3 accepted escaped mutant rows covering 3 stable mutant IDs.
- `tools/mutation/baselines/goplint-baseline.json`: 0 accepted escaped mutant rows covering 0 stable mutant IDs.

## Root Profile

Summary from `artifacts/mutation/full/root/go-mutesting-summary.json`:

- Total: 9,389
- Killed: 5,224
- Escaped in source report: 1,636
- Accepted baseline after remediation: 3 rows covering 3 stable mutant IDs
- Not covered: 2,529
- MSI: 55.64%
- Covered-code MSI: 76.15%

Top accepted clusters after the current remediation batches:

- `pkg/fspath/fspath.go`: 1 accepted rows
- `pkg/platform/sandbox.go`: 1 accepted rows

Root exact-rerun status correction:

- The later root exact-rerun pruning notes that concluded the root baseline was empty are superseded by the corrected audit below. They interpreted `go-mutesting v2.7.0` terminal statuses backward: `PASS` means the mutant was killed, while `FAIL` means the mutant escaped.
- Historical v2.7.0 corrected exact reruns used `--run-mutant-id`, `--output-statuses=e`, and `--test-flags='-short -count=1'`; the emitted `FAIL` rows in those historical logs were escaped survivors. Current v2.7.1 runs emit `ESCAPED` for that status.
- Exact-reran the prior committed root baseline from `HEAD` in six isolated root-module mirrors: 372 committed rows covering 362 stable mutant IDs.
- The corrected proof emitted 186 current escaped rows covering 165 stable mutant IDs, then resolver-validation, semver, content-hash, operations-validation, discovery-collision, dependency-contract, and filepath-dependency remediation batches reduced the committed baseline to 159 rows covering 138 stable mutant IDs. The corrected proof proved 210 prior committed rows and 197 prior committed IDs killed or obsolete under corrected semantics.
- The proof also emitted 24 current same-ID escaped rows that were not exact file/line/mutator matches for the prior committed rows. Because `go-mutesting` suppresses by stable mutant ID, the repaired baseline stores current escaped row metadata for the remaining still-escaping accepted IDs.

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
- Accepted baseline after remediation: 0 rows covering 0 stable mutant IDs
- Not covered: 357
- MSI: 67.70%
- Covered-code MSI: 74.37%

Top accepted clusters after the current remediation batches:

- No accepted `tools/goplint` survivor clusters remain.

Top not-covered clusters:

- `goplint/analyzer_validate_delegation.go`: 83 not covered
- `goplint/analyzer_windows_pitfalls.go`: 62 not covered
- `goplint/analyzer_boundary_request_validation.go`: 48 not covered
- former constructor-validates CFA implementation (since consolidated and deleted): 25 not covered
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

Goplint exact-rerun status correction:

- The goplint exact-rerun pruning notes from the analyzer batch through the windows-pitfalls batch are superseded by the corrected audit below, including same-ID non-baseline site notes and line-shifted killed-site notes in those batches. They interpreted `go-mutesting v2.7.0` terminal statuses backward: `PASS` means the mutant was killed, while `FAIL` means the mutant escaped.
- Historical v2.7.0 corrected exact reruns used `--run-mutant-id`, `--output-statuses=e`, and `--test-flags='-short -count=1'`; the emitted `FAIL` rows in those historical logs were escaped survivors. Current v2.7.1 runs emit `ESCAPED` for that status.

Goplint analyzer exact-rerun batch:

- Fresh exact reruns proved 38 accepted `goplint/analyzer.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root. A no-`-count=1` probe produced misleading test-cache `PASS` output and was discarded before pruning.
- After this batch, the goplint baseline accepted 772 survivor records, and `goplint/analyzer.go` had no accepted records remaining.

Goplint constructor-usage exact-rerun batch:

- Fresh exact reruns proved all 25 accepted `goplint/analyzer_constructor_usage.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 747 survivor records, and `goplint/analyzer_constructor_usage.go` had no accepted records remaining.

Goplint path-domain exact-rerun batch:

- Fresh exact reruns proved all 22 accepted `goplint/analyzer_path_domain.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- The range-break rerun for the accepted line-180 record also surfaced same-ID non-baseline escaped sites at lines 43, 101, 156, and 165. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.
- After this batch, the goplint baseline accepted 725 survivor records, and `goplint/analyzer_path_domain.go` had no accepted records remaining.

Goplint pathmatrix-divergent exact-rerun batch:

- Fresh exact reruns proved all 21 accepted `goplint/analyzer_pathmatrix_divergent.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 704 survivor records, and `goplint/analyzer_pathmatrix_divergent.go` had no accepted records remaining.

Goplint validate-delegation-modes exact-rerun batch:

- Fresh exact reruns proved all 20 accepted `goplint/analyzer_validate_delegation_modes.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- The range-break rerun for the accepted line-285 record also surfaced same-ID non-baseline escaped sites at lines 48, 123, 161, and 201. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.
- After this batch, the goplint baseline accepted 684 survivor records, and `goplint/analyzer_validate_delegation_modes.go` had no accepted records remaining.

Goplint test-home-env exact-rerun batch:

- Fresh exact reruns proved all 12 accepted `goplint/analyzer_test_home_env.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 672 survivor records, and `goplint/analyzer_test_home_env.go` had no accepted records remaining.

Goplint nonzero exact-rerun batch:

- Fresh exact reruns proved all 9 accepted `goplint/analyzer_nonzero.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 663 survivor records, and `goplint/analyzer_nonzero.go` had no accepted records remaining.

Goplint remaining-small-clusters exact-rerun batch:

- Fresh exact reruns proved all 8 accepted `goplint/analyzer_redundant_conversion.go` survivor records killed and removed from the goplint baseline.
- Fresh exact reruns proved all 5 accepted `goplint/analyzer_run.go` survivor records killed and removed from the goplint baseline.
- Fresh exact reruns proved all 3 accepted `goplint/analyzer_validate_usage.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root. The `goplint/analyzer_run.go` reruns reported line-shifted killed sites after local source changes, but each removed accepted ID had a historical v2.7.0 exact-rerun `FAIL`.
- After this batch, the goplint baseline accepted 647 survivor records; only the nine larger goplint clusters remained accepted.

Goplint cast-validation exact-rerun batch:

- Fresh exact reruns proved all 33 accepted `goplint/analyzer_cast_validation.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root. Several reruns reported line-shifted killed sites after local source changes, but each removed accepted ID had a historical v2.7.0 exact-rerun `FAIL`.
- After this batch, the goplint baseline accepted 614 survivor records; eight larger goplint clusters remained accepted.

Goplint enum-sync exact-rerun batch:

- Fresh exact reruns proved all 38 accepted `goplint/analyzer_enum_sync.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 576 survivor records; seven larger goplint clusters remained accepted.

Goplint structural exact-rerun batch:

- Fresh exact reruns proved all 56 accepted `goplint/analyzer_structural.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root. Several reruns reported line-shifted killed sites after local source changes, but each removed accepted ID had a historical v2.7.0 exact-rerun `FAIL`.
- After this batch, the goplint baseline accepted 520 survivor records; six larger goplint clusters remained accepted.

Goplint cross-platform-path exact-rerun batch:

- Fresh exact reruns proved all 60 accepted `goplint/analyzer_cross_platform_path.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 460 survivor records; five larger goplint clusters remained accepted.

Goplint boundary-request exact-rerun batch:

- Fresh exact reruns proved all 66 accepted `goplint/analyzer_boundary_request_validation.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- After this batch, the goplint baseline accepted 394 survivor records; four larger goplint clusters remained accepted.

Goplint constructor-validates exact-rerun batch:

- Fresh exact reruns proved all 86 accepted `goplint/analyzer_constructor_validates.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- The range-break rerun for the accepted line-578 record also surfaced same-ID non-baseline escaped sites at lines 132 and 634. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.
- After this batch, the goplint baseline accepted 308 survivor records; three larger goplint clusters remained accepted.

Goplint constructor-validates-CFA exact-rerun batch:

- Fresh exact reruns proved all 93 accepted survivor records from the former constructor-validates CFA implementation killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- Range-break reruns also surfaced same-ID non-baseline escaped sites at lines 321 for the accepted line-330 record, and lines 207, 226, and 248 for the accepted line-351 record. Those were not added during this shrink-only pass; reconcile them with the next full goplint mutation profile before any broader baseline refresh.
- The committed goplint baseline now accepts 215 survivor records; two larger goplint clusters remain accepted.

Goplint validate-delegation exact-rerun batch:

- Fresh exact reruns proved all 105 accepted `goplint/analyzer_validate_delegation.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- The committed goplint baseline now accepts 110 survivor records; only the `goplint/analyzer_windows_pitfalls.go` cluster remains accepted.

Goplint windows-pitfalls exact-rerun batch:

- Fresh exact reruns proved all 110 accepted `goplint/analyzer_windows_pitfalls.go` survivor records killed and removed from the goplint baseline.
- The proof reruns used `--run-mutant-id` with `--test-flags='-short -count=1'` from isolated `tools/goplint` module mirrors.
- The same-ID non-baseline site note from this batch was based on the inverted status interpretation and is superseded by the corrected audit below.
- The zero-baseline conclusion from this batch is superseded by the corrected audit below.

Corrected goplint accepted-survivor audit:

- Exact-reran the prior committed goplint baseline from `HEAD` in six isolated `tools/goplint` mirrors: 810 committed rows covering 808 stable mutant IDs.
- The corrected proof emitted 812 current escaped rows covering 807 stable mutant IDs. One prior accepted ID no longer escaped; 56 prior committed rows were killed or obsolete under corrected semantics.
- The proof also emitted 58 current same-ID escaped rows that were not exact file/line/mutator matches for the prior committed rows. Because `go-mutesting` suppresses by stable mutant ID, the repaired baseline stores current escaped row metadata for all 807 still-escaping accepted IDs.
- The committed goplint baseline now accepts 812 current survivor rows covering 807 stable mutant IDs.

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

Forty-seventh remediation batch:

- Reconcile the last `pkg/invowkfile/command.go` accepted records against fresh single-mutant reruns in the current checkout.
- Fresh reruns proved the five remaining accepted `pkg/invowkfile/command.go` survivor records killed: `118093d1fac6facea665beb567e075c4`, `f8afbd682d2895e15c6249b4834abb01`, `80d0011a8fef9419f805a73ec7639568`, `e08111efc8b34b162bb28986bfc34c04`, and `2849f316acdaaea5dbb76db10504ff35`.
- The `f8afbd682d2895e15c6249b4834abb01` single-mutant rerun matched two same-ID sites in `pkg/invowkfile/command.go`; both were killed by current tests.
- The root baseline dropped from 372 to 367 survivor records, and `pkg/invowkfile/command.go` has no accepted records remaining.

Forty-eighth remediation batch:

- Reconcile the last `internal/watch/watcher.go` accepted records against fresh single-mutant reruns in the current checkout.
- Fresh reruns proved both remaining accepted `internal/watch/watcher.go` survivor records killed: `367e6a2bb66771e02dcebb43b0798951` and `84ac84318770d53e44aff93ebe40534e`.
- Adjacent current-artifact candidates in `pkg/cueutil`, `internal/discovery`, and `internal/app/llmconfig` still escaped at this checkpoint and remained accepted.
- The root baseline dropped from 367 to 365 survivor records, and `internal/watch/watcher.go` has no accepted records remaining.

Forty-ninth remediation batch:

- Add parser contract coverage for `pkg/cueutil/parse.go`, including user CUE syntax errors taking precedence over schema-path lookup and concrete validation rejecting schema-required fields outside the decoded Go struct.
- Fresh single-mutant reruns proved four accepted `pkg/cueutil/parse.go` survivor records killed: `266fbc22731d8f637dc8f3157f67bb30`, `46b3e17739e1334d26d3a7dc44c38a48`, `7830b0f6ed0908d635137e26274a80d0`, and `e9d96bf0afe1b2371b06c438c13275b6`.
- The `e9d96bf0afe1b2371b06c438c13275b6` rerun also matched a non-baseline non-concrete validation site at line 85 that still escaped; the accepted baseline record at line 81 was killed and removed.
- The root baseline now accepts 361 survivor records, and `pkg/cueutil/parse.go` has one accepted record remaining.

Fiftieth remediation batch:

- Add duration contract coverage for `pkg/invowkfile/duration.go`, including the exact maximum-rune valid boundary, `InvalidDurationStringError` value/reason payloads, and `parseDuration` wrapping of `ErrInvalidDurationString`.
- Fresh single-mutant reruns proved all seven accepted `pkg/invowkfile/duration.go` survivor records killed: `dcc4180329efa53eda698117a82fa11c`, `29044e2173368c1369cbc1f4953d5b9b`, `284a290511b569e9e445d110c9145c86`, `70c57c9a526f02fff00b2b7cdb302dc9`, `a5616880bb91c58935e7623da7f2c56b`, `fd89202048ed18fd339afaedee8e5c9a`, and `0d16cf9ce88fea080bb185393aed459d`.
- The root baseline now accepts 354 survivor records, and `pkg/invowkfile/duration.go` has no accepted records remaining.

Fifty-first remediation batch:

- Add server lifecycle coverage for `internal/core/serverbase/base.go`, including failure cancelling the lifecycle context, failure and stop-before-start closing `Err()`, default one-error async buffering, closed error-channel send suppression, and `WaitForReady` context error wrapping.
- Fresh single-mutant reruns proved eight accepted `internal/core/serverbase/base.go` survivor records killed: `4668d336aa44942a817e3038feab070b`, `2f52bcdf6623fd2146fd548c7726a4d4`, `8b57f44340d019690a2b4abe0e111e25`, `42beadc4af14ddd38f93860c898ddd2e`, `994ad9e875320d51eac28ead1de0df31`, `e5f750a5998bc2f843c7f0fc6ffe5805`, `85b0308e22a757b8fd28250cb448d5f5`, and `ef61dc3c69dc6b09f511172d07c4d9ce`.
- The root baseline now accepts 346 survivor records, and `internal/core/serverbase/base.go` has two accepted records remaining: the explicit zero-value `StateCreated` store and the `LastError` lock-hold mutation that is not deterministically observable in the non-race mutation profile.

Fifty-second remediation batch:

- Add discovery validation payload coverage for `internal/discovery/discovery_validate.go`, including exact `CommandInfo`, `DiscoveredFile`, and `LookupResult` field-error counts plus sentinel membership for nested validation failures.
- Fresh single-mutant reruns proved all ten accepted `internal/discovery/discovery_validate.go` survivor records killed: `b69c6540e8e46a586564a9c7091fcd53`, `5c448186597816e467dae92f3025a8eb`, `b8c43fb41cf8331165fda5e0ed2db48d`, `e6e7ccffe3085b951a4f549d0b68b227`, `f81444d536b6eb4a7ffc63be11218f67`, `99ce4d5a225d97f48b725c53a2f25006`, `77d9d1f432eac876c021c47c4f63130f`, `f5dc27cf69dd60acda90fd1f0f3db39c`, `96653ecb63a2d7b74b1f559a825d55b7`, and `7d23fb855e9ad63685687a0cc1258e74`.
- The root baseline now accepts 336 survivor records, and `internal/discovery/discovery_validate.go` has no accepted records remaining.

Fifty-third remediation batch:

- Add execution-context options validation payload coverage for `internal/app/execute/orchestrator_validate.go`, including invalid workdir, container name, dotenv path, env inherit mode, allow/deny env names, command name, source ID, platform, and explicit platform mismatch field errors.
- Fresh single-mutant reruns proved all nine accepted `internal/app/execute/orchestrator_validate.go` survivor records killed: `a800dcdbb8626383b1ebf7a4a60e19d7` (covering duplicate accepted records at lines 78 and 83), `8dc9b8bb071c158d5649c7d2be3fc0b2`, `5c3f62f1967773b923a327b914c79df8`, `bc66aea2f4704ea160574163ce7b516d`, `4c40d1786e2d1ff5782f805252359881`, `6eb7be263765009f20ec9f406c7a2532`, and `9643050e0ff3011bdaf42a029ac14d52` (covering duplicate accepted records at lines 77 and 82).
- The root baseline now accepts 327 survivor records, and `internal/app/execute/orchestrator_validate.go` has no accepted records remaining.

Fifty-fourth remediation batch:

- Reconcile `internal/discovery/discovery.go` against current discovery mutation-contract coverage.
- Fresh single-mutant reruns proved all nine accepted `internal/discovery/discovery.go` survivor records killed: `40b768df087ab427fa7b0d974cc28213`, `a84f3ad012f883b2a25dd2cbf0cf186f`, `0a7edec1ac46a0236c811e987ed3ba48`, `d3e09d06a2d71fd53746d1759deb63f5`, `6e096a861724dddc754851e36ea4e4a0`, `37a0917ba5968e571dc9b1d2b101c695`, `d53ca632a4baa4e9b5f022089e9a3661`, `ac8bdade87211c1b4da278d981c5093b`, and `cc7616bf46bc58f39879fbd4c73bd3ea`.
- The root baseline now accepts 318 survivor records, and `internal/discovery/discovery.go` has no accepted records remaining.

Fifty-fifth remediation batch:

- Reconcile `internal/discovery/discovery_commands.go` against current command aggregation and discovery-command mutation coverage.
- Fresh single-mutant reruns proved all eight accepted `internal/discovery/discovery_commands.go` survivor records killed: `86531b3722115bda9a11982aaf61ef4f`, `950358334c48d0851aee9b4dbccabbbf`, `5e6674e17fef51cc4fa120a2b02bf596`, `eb2d70cd1a70757c433759529e290420`, `388b7b083ee2916fc2b502d5e3be24d1`, `2799b64dd28eae3c04de345aa4b018c4`, `255eb50650713787bb9adbaf7eda105e`, and `171d14e79216dadc07594f5dd4deea41`.
- The root baseline now accepts 310 survivor records, and `internal/discovery/discovery_commands.go` has no accepted records remaining.

Fifty-sixth remediation batch:

- Add missing exact error-string and unwrap coverage for `InvalidCustomCheckScriptError`, then reconcile `pkg/invowkfile/dependency.go` against current dependency mutation coverage.
- Fresh single-mutant reruns proved eight accepted `pkg/invowkfile/dependency.go` survivor records killed: `5246cf74b6f9c4bb0fadd3db616a791e`, `76c9215d2f6205fac737094f79db5e14`, `25752f62b95c6bae3e61a08b47b9693d`, `bfc475e48d22374af0880625d6c03079`, `c7e1e9f11225adb1e0124a00ec4fcffb`, `de43060570b20d71f10a9a56926e5f55`, `0bf884cc767d5e37bd6f81860b038514`, and `962b42a637c4694c85c9863bdc95d3f8`.
- The remaining `pkg/invowkfile/dependency.go` accepted record, `755193adc2bf3c56a5e17ace34a74422`, still escaped as a no-diff `InvalidCustomCheckScriptError.Error()` return mutation and remains accepted. The root baseline now accepts 302 survivor records.

Fifty-seventh remediation batch:

- Reconcile `pkg/invowkfile/validation_structure_command.go` against current structure-validation diagnostic coverage.
- Fresh single-mutant reruns proved five unique stable IDs covering all nine accepted `pkg/invowkfile/validation_structure_command.go` survivor records killed: `208bf87f0a93eec5e85e2f33bb8eb39f` (covering duplicate accepted records at lines 41, 105, 120, 137, and 150), `043a7a04a1cd37e04ad3e5161fa6406e`, `5bdc85a32ebc268d6149d72be96327df`, `668cb62d3c4180fca08f5a33c89dccd2`, and `be9abefb2b214c67166ba6615ba1b69f`.
- The root baseline now accepts 293 survivor records, and `pkg/invowkfile/validation_structure_command.go` has no accepted records remaining.

Fifty-eighth remediation batch:

- Add direct `VirtualFilesystemConfig.HasFilesystemConfig()` and `PlatformVirtualConfig.HasConfig()` contract coverage for empty, access-only, paths-only, combined, nil, and empty nested configs, then reconcile `pkg/invowkfile/virtual_filesystem.go`.
- Fresh single-mutant reruns proved the accepted `VirtualFilesystemPathName` payload survivor `b9671a7fa11ee4b3829be8b0d3e51e3e` killed and removed from the root baseline.
- The eight remaining line-164 `pkg/invowkfile/virtual_filesystem.go` survivor records still escaped targeted reruns without concrete diffs despite direct boolean-contract coverage and remain accepted. The root baseline now accepts 292 survivor records.

Fifty-ninth remediation batch:

- Reconcile `pkg/invowkmod/content_hash.go` against current content-hash validation and filesystem-close coverage.
- Fresh single-mutant reruns proved all nine accepted `pkg/invowkmod/content_hash.go` survivor records killed: `b47604333c95a3af604b8d8d6f299971`, `edb156e489a60a3ca420fcbd94979b53`, `723f2dff8e48e3e4c596d2a10b0a7804`, `ca8fe76f39c86892a518bd367c46871a`, `dfb473d0840a14fffd0da4193acbc61c`, `bd8323411c9125b7b1eb323acafaf57c`, `6301ca8145720964fbb3340fc779ab9e`, `01019bb1d5981fbef7ea87a4c7f13a6d`, and `f9b124b9f677200c19971f07b0426f76`.
- This historical all-killed conclusion predates the corrected exact-rerun status audit. The current accepted state is recorded in the Root Profile summary and the later content-hash remediation batches.

Sixtieth remediation batch:

- Reconcile `pkg/invowkmod/resolver_validate.go` against current module-reference validation coverage.
- Fresh single-mutant reruns proved all nine accepted `pkg/invowkmod/resolver_validate.go` survivor records killed: `36caa7ea7a14fdf77803611bee45c79e`, `02497ba281ba0e1f367f4c322d58f1eb`, `e238ff6f626e6c526aaf89a84ed089a4`, `b0b78208c95792b81e85030eee1ac4f7`, `291390200dee7c2fe3525eb0b622c146`, `d30f4c01300474041d87f0f2a0bef9aa`, `937b4a210b38fbe7c4a6929349c005ea`, `ffd32a4174bc8fd5beda58843222a1b7`, and `e48fcdbf167f126f34a99c62144bad22`.
- The root baseline now accepts 274 survivor records, and `pkg/invowkmod/resolver_validate.go` has no accepted records remaining.

Sixty-first remediation batch:

- Reconcile `pkg/invowkmod/semver.go` against current semantic-version boundary coverage.
- Fresh single-mutant reruns proved all nine accepted `pkg/invowkmod/semver.go` survivor records killed: `f65fd5a40d510a9162961a3a9c3e9fff`, `3a97ffec512918954bc03c140870585c`, `e5099874de4b107ef5bc0bfa12b51aff`, `c0b5a6979a73e78eae95fa00c8d2aea5`, `1fa4d198fdecd1122af1a1a9c52c415b`, `fded7fda3f63509df7e9b5c82687cc19`, `be17c90d51426e1dd3598cd3e3225a14`, `b5dfde5e034703e5f4109d7b6a8a6b47`, and `340e2f9fcd00b7ca48031536caf342d9`.
- The root baseline now accepts 265 survivor records, and `pkg/invowkmod/semver.go` has no accepted records remaining.

Sixty-second remediation batch:

- Reconcile `pkg/invowkmod/operations_validate.go` against current operation-request validation coverage.
- Fresh single-mutant reruns proved all eight accepted `pkg/invowkmod/operations_validate.go` survivor records killed: `620748d1013c82b08b1d8d55508d415f`, `be11e8f377b644cf818e8d52fe291891`, `77e0b8ec4361d8413e9677161fedc55f`, `7c7bf95d3565d218bda33cb9b8596b22`, `caeb88fa544084fcbb5955f1d0be46c8`, `60bf193e2e15d459d107467bba62fcc3`, `4a60b03ed81dd05eb142e62693e7a8fb`, and `b69e8c821ab0fe5623a5fd24fc9d8e2c`.
- The root baseline now accepts 257 survivor records, and `pkg/invowkmod/operations_validate.go` has no accepted records remaining.

Sixty-third remediation batch:

- Add direct host filepath dependency coverage proving `CheckHostFilepathDependenciesWithProbe()` preserves failed filepath validation as both legacy `MissingFilepaths` output and structured filepath failures.
- Fresh single-mutant reruns proved all eight accepted `internal/app/deps/filepaths.go` survivor records killed: `6b2285144ac55c868471f2375fcaac07`, `a3863647bd2462895996068bad36996e`, `3ad6d74ea10d6bbc85ef5864440a7be8`, `43933fb78f25c70922ae3bb191735007`, `a8a304b06b74cc45e8d4af1ce22f70eb`, `6fbbdd4157a6d54965047b053a8c53e9`, `8603fb28830aa2026a834e29fc10ac62`, and `398f1def3758b0c0b5e9c511b10d3cdc`.
- The root baseline now accepts 249 survivor records, and `internal/app/deps/filepaths.go` has no accepted records remaining.

Sixty-fourth remediation batch:

- Simplify redundant `pkg/invowkmod/invowkmod.go` validation branches where an earlier or later check already enforced the same contract: empty `ModuleID` rejection remains covered by the module-ID regex, `SubdirectoryPath` traversal remains covered by normalized segment scanning, and `Module.ValidateScriptPath()` traversal remains covered by the final `filepath.Rel` containment check.
- Fresh targeted reruns proved one accepted `pkg/invowkmod/invowkmod.go` survivor killed after the cleanup: `959df52a4e8be3e79ab45612304d5d04`.
- Fresh targeted reruns also showed nine accepted records now have no matching mutation site and were removed as obsolete source-site records: `b71ad701762291dd5df8b0f3aa760ee1`, `352669e8366e477b7be0a75547fb396f`, `25b615afdce64cea4b9431e1683dbbd9`, `f597e78dae8e6ecc718fd223a467bbca`, `efd76f3837456d12af78222d855f6de2`, `6b34d34a90344f5ead71516a657bc998`, `9ca19de240a8655d1283c07120e5bba2`, `a3151e2278df128ccb20de521c5b767e`, and `87ecfc10959d97e6d72f5a50832bbb23`.
- The semver guard records `9ba9b26a1344ed06afa0e79296cbe719` and `3769fd5c1400c9e5c715ddca3c25fff7` still escape, but the direct `Version.Validate()` calls are required by the value-type delegation gate. The Unix-style absolute `ResolveScriptPath()` branch record `4ab9c5e50904f654f821e2dc5cc0c0cf` also still escapes on the Linux mutation profile while preserving Windows path semantics. The root baseline now accepts 239 survivor records, and `pkg/invowkmod/invowkmod.go` has three accepted records remaining.

Sixty-fifth remediation batch:

- Add config decode/load coverage for concrete CUE validation and nested auto-provision include errors.
- Fresh explicit single-mutant reruns proved two accepted `internal/config/config.go` survivor records killed: `0a8d25141599bfd91e5878c6bd8c683f` and `db43080e3fa7012cdab92f84420c5cef`.
- The root baseline now accepts 237 survivor records, and `internal/config/config.go` has five accepted records remaining.

Sixty-sixth remediation batch:

- Add `ContainerfilePath` validation payload and maximum-length boundary coverage.
- Fresh explicit single-mutant reruns proved all eight accepted `pkg/invowkfile/containerfile_path.go` survivor records killed: `2fd62261ee0d1e636493a595876a7868`, `36b9362419a9da56ba705457c1025a59`, `3b9a42fff1c5fbc666c8b9a1f7c4bb94`, `5a96cb16906e7eeac6f0572af28a7f1d`, `7e423ca019a294d9484f3f1e9fc916d0`, `99895ff1483833bdb6c4a17fc8140017`, `99e055abe07d0ee360073db76dffc755`, and `b7814e3a56d000c3f38ea3451381d929`.
- The root baseline now accepts 229 survivor records, and `pkg/invowkfile/containerfile_path.go` has no accepted records remaining.

Sixty-seventh remediation batch:

- Add interpreter-spec validation coverage for whitespace payloads, safe env forms, and unsafe env diagnostics.
- Fresh explicit single-mutant reruns proved four accepted `pkg/invowkfile/interpreter_spec.go` survivor records killed: `1a39f9a3ec855f1e7c61e75d7c897dde`, `6a01726ad46de70d55e753832d751ad5`, `87d16be347fd9e5f103e950877d0a3ca`, and `ceee9fb5ae9c4ed077c3babc79fcd8ee`.
- The env-path constant cleanup removed matching source sites for three accepted records that no longer generate current mutants: `1b88faec58ce502ba82f2a47b2a2cc0f`, `4d46475fcac6b8b02fb4a0e69ffbe4c3`, and `8142330b81e123267af7e94f064daace`.
- The root baseline now accepts 222 survivor records, and `pkg/invowkfile/interpreter_spec.go` has no accepted records remaining.

Sixty-eighth remediation batch:

- Add typed payload assertions for invalid `VirtualFilesystemPathName` values, then reconcile the remaining direct virtual-filesystem boolean-contract survivors against current coverage.
- Fresh explicit single-mutant reruns proved the previously removed `VirtualFilesystemPathName` payload survivor `b9671a7fa11ee4b3829be8b0d3e51e3e` killed in the current checkout.
- Fresh explicit single-mutant reruns also proved all eight remaining line-164 `pkg/invowkfile/virtual_filesystem.go` survivor records killed: `11ec949a65e8ea6bc1ebbb1995dbf229`, `32f7c7e6cd9f16055ac8f208866aebc1`, `9cba9867562808fdb2ce2771ac3bc485`, `9f2c9e0c85add401222a09f2f54e3478`, `a56e15ad7af39361289c529b2067e6ae`, `a6e47da7085b6e0d7efc203bae625e1a`, `d6803a779dac34fd1af08f7aa4e3314b`, and `da37129a9b4b718acd0288cf1a469240`.
- The root baseline now accepts 214 survivor records, and `pkg/invowkfile/virtual_filesystem.go` has no accepted records remaining.

Sixty-ninth remediation batch:

- Add helper-level validation-input coverage for integer sign boundaries, empty and sign-only integers, embedded and leading invalid runes, and float64-range parsing.
- Fresh explicit single-mutant reruns proved five accepted `pkg/invowkfile/validation_input.go` survivor records killed: `9752bad5704d1fd810b2dfc6f00cc597`, `8d5872606d23440e1ee1247f5fccbf7e`, `82cfc9d86869f9b6104a66374d9d7d0a`, `82cd4cdd75671b96eacf22e4ff0e9950`, and `222b40162a583794f6d7c3126981c0e5`.
- The `strconv.ParseFloat` bit-size records `a90a66d0d6d3ce50fa1c4b2a6841eb13` and `848d2ea3a491d5968ab1a351c719817f` still escaped explicit reruns and remain accepted.
- The root baseline now accepts 209 survivor records, and `pkg/invowkfile/validation_input.go` has two accepted records remaining.

Seventieth remediation batch:

- Add parse-contract coverage for `ParseInvowkmodBytes()` requirement path validation, including indexed `requires[1].path` errors, `ErrInvalidSubdirectoryPath` wrapping, typed payload preservation, and traversal reason text.
- Fresh explicit single-mutant reruns proved all seven accepted `pkg/invowkmod/parse.go` survivor records killed: `96a27bffb3d11dcc289e4eda319c6d62`, `3906b40413a9eb7f79d0026ee10cb4c3`, `4dc4ef2b510385283e4b7b65a0d50374`, `4d6ea6b85c6578e9c709bc4e76c9b112`, `9e9b4fbbe383a043456b7b61ee3cac1a`, `522cf3bcde3cdc4245017e36158254b1`, and `1a98f2bff48909984a5b50e0f1b4b622`.
- The root baseline now accepts 202 survivor records, and `pkg/invowkmod/parse.go` has no accepted records remaining.

Seventy-first remediation batch:

- Add declared-lock policy coverage for exact one-match returns, key and locked-payload preservation, nil lock rejection, empty module-ID rejection, and one-match ambiguity suppression.
- Fresh explicit single-mutant reruns proved all six accepted `pkg/invowkmod/vendored_policy.go` survivor records killed: `00161fa1e8a864a931249fb872d1a7e1`, `da7e3a19094db4cea391c161cb2825bd`, `3a2efbb3c2be294204b4326d80c5c3e2`, `d5d4b6ee7eee15b18430ca35b76f8030`, `87a3acafb8b8cae741bc1ae7b56a86ce`, and `2559f66a15e68ad81cee4b8ffba8c612`.
- The root baseline now accepts 196 survivor records, and `pkg/invowkmod/vendored_policy.go` has no accepted records remaining.

Seventy-second remediation batch:

- Make `FormatError()` explicitly detect non-CUE errors before calling CUE's `errors.Errors()`, preserving ordinary Go error wrapping while keeping CUE error rendering on the CUE path.
- Add CUE formatting guard coverage for pathful messages that do not start with their JSON path and for pathless messages with leading colon text, preventing weakened path-prefix checks from trimming meaningful message content.
- Fresh explicit single-mutant reruns proved three accepted `pkg/cueutil/error.go` survivor records killed: `a303f648fc0cb00c47c918db066ee22f`, `f6c1106b6e76e354a56600f529e43870`, and `52d35a5e0e316d431cceda65f46a6d22`.
- Fresh explicit single-mutant reruns also showed the old non-CUE fallback record `07857aea65fb3174d7b0acd67b4b8c97` now has no matching mutation site after the explicit CUE-error gate and was removed as an obsolete source-site record.
- The `formatPath()` loop-break record `0770024e240efd1b9173e2344365e36f` and empty-path length record `d7ad86d624243fdfcd0ba4ec9e383d8c` still escaped exact reruns and remain accepted as equivalent-looking formatting guards. The root baseline now accepts 192 survivor records, and `pkg/cueutil/error.go` has two accepted records remaining.

Seventy-third remediation batch:

- Add command-tree validation coverage for returned conflict payloads, parent commands without positional args, and nested leaf commands with positional args.
- Fresh explicit single-mutant reruns proved five accepted `pkg/invowkfile/command_tree.go` survivor records killed: `e1ec2d62aa2e7f80bd9c85ca3ebc28ae`, `59152a3e0d35dd2887da55da044476a2`, `9346c5bf4f406742365cbc091fb128ab`, `3a384dee2f7f6d750b66b37199c8d29b`, and `b7775d6e9a751d50a22526b6d716f27e`.
- The loop-start record `dc425ae28f9d698e749f92a3c86319e9` still escaped exact reruns and remains accepted as equivalent-looking for valid non-empty command names. The root baseline now accepts 187 survivor records, and `pkg/invowkfile/command_tree.go` has one accepted record remaining.

Seventy-fourth remediation batch:

- Add issue value-type validation coverage for invalid payload preservation on `Id`, `MarkdownMsg`, and `HttpLink` errors, plus non-empty clone-contract coverage for issue documentation and external link accessors.
- Fresh explicit single-mutant reruns proved all five accepted `internal/issue/issue.go` survivor records killed: `d0a2627166b3e2cad768761b5715c968`, `c1abc0fa02e154fb296ee3b52fc41b0c`, `5f98b616a26c1ec948b9520fb4cb8ea6`, `90b1e04397aa845e8dd014be494d5224`, and `3777e4757d3267b31caf54fa12d535b5`.
- The root baseline now accepts 182 survivor records, and `internal/issue/issue.go` has no accepted records remaining.

Seventy-fifth remediation batch:

- Add atomic write failure coverage for missing target directories preserving `os.ErrNotExist` wrapping and for rename failures when the target path is an existing directory, including preservation of the existing directory and failed-temp cleanup.
- Fresh explicit single-mutant reruns proved two accepted `pkg/fspath/atomic.go` survivor records killed: `85552d0508d7e81ccd967bd5cc46a610` and `b7dc395caa5af1cbf865983315c33a72`.
- The chmod, write, and close error-guard records `5639b6974fd232084b1deb951cc981f0`, `2d46e98ac61114c7ddcbbfc373f57cca`, and `558840d98feb2a5b0fb11a8edcd2b6ea` still escaped exact reruns and remain accepted because those `os.CreateTemp`-owned file-descriptor failures are not practically triggerable without fault injection. The root baseline now accepts 180 survivor records, and `pkg/fspath/atomic.go` has three accepted records remaining.

Seventy-sixth remediation batch:

- Add sandbox validation coverage for invalid value payload preservation and subprocess-based process-environment detection so `detectOnce`, `DetectSandbox()`, and `IsInSandbox()` are exercised after package initialization sees `SNAP_NAME`.
- Fresh explicit single-mutant reruns proved four accepted `pkg/platform/sandbox.go` survivor records killed: `a5708f3a74bbd6df2e3de4383146d2fa`, `933ccc61bad3a2170ea690871362513f`, `add1d9cbe5144c12b6952bd563ff8cb1`, and `7458c942cc5824f8c609b245d7f1fd40`.
- The no-sandbox fallback record `e78e96816010c22638d477a3277ba5fe` still escaped exact reruns and remains accepted because `SandboxNone` is the empty string. The root baseline now accepts 176 survivor records, and `pkg/platform/sandbox.go` has one accepted record remaining.

Seventy-seventh remediation batch:

- Add typed payload assertions for invalid `ModuleShortName` and `ModuleDirectoryName` validation errors.
- Fresh explicit single-mutant reruns proved two accepted `pkg/invowkmod/module_short_name.go` survivor records killed: `b8cc633e69ccb0d77ce56210890ac659` and `f28505d8f48515d78bc0bca9c81e7c0d`.
- The empty-string guard records `9ccb58585bf8f0f9895e888613c2f463` and `d3c9db5250d5a59363abeadfc6320901` still escaped exact reruns and remain accepted because the same regex branch rejects empty values. The root baseline now accepts 174 survivor records, and `pkg/invowkmod/module_short_name.go` has two accepted records remaining.

Seventy-eighth remediation batch:

- Add fallback source-ID coverage for legacy locked module paths plus scp-style and scheme-only Git URLs.
- Fresh explicit single-mutant reruns proved four accepted `pkg/invowkmod/dependency_types.go` survivor records killed: `89169834da15df59b308f1322a3efa7b` at line 220, `0693c5e278ad5ebf49ca7f6aa2dfd222`, `58339ada38dc5af4dd472442f47459a1`, and `0fc7a6a933df20ada40600d1128afd36`.
- The duplicate-ID scheme-strip branch record `89169834da15df59b308f1322a3efa7b` at line 217 still escaped exact reruns and remains accepted because the basename result is unchanged for ordinary scheme URLs. The root baseline now accepts 170 survivor records, and `pkg/invowkmod/dependency_types.go` has one accepted record remaining.

Seventy-ninth remediation batch:

- Add module scaffold coverage for the default generated description, the exact empty-name constructor guard, and each generated-field validation branch.
- Fresh explicit single-mutant reruns proved five accepted `pkg/invowkmod/operations_create.go` survivor records killed: `30b0744dc4659b711fb5dbb5f014af40`, `868c486f9d6129f850361ee3d35d9344`, `7ed860edb439aea8054d494a97586e59`, `f98d129a73688635a3a91946224a7373`, and `64aedb966a70cd8e5653bb9108fb44b6`.
- The constructor-level scaffold validation guard `1003e60196debaaf9c11ab8706fd075c` still escaped exact reruns and remains accepted because validated options always generate a valid scaffold. The root baseline now accepts 165 survivor records, and `pkg/invowkmod/operations_create.go` has one accepted record remaining.

Eightieth remediation batch:

- Add exact module-name parsing and validation error assertions plus `InvalidModuleIDError.Value` preservation for canonical directory suffix rejection.
- Fresh explicit single-mutant reruns proved five accepted `pkg/invowkmod/operations.go` survivor records killed: `a874510cc70290a5112993a1d5ffec5f`, `a010e9ded9a233f4405696964a58a470`, `1d4e8f093bdea29396680dac382e5e4f`, `00757e9135a239d48b48165ffb5a74cc`, and `bab2d20887112cad0d2e5c179d1249a3`.
- The canonical directory generated-name validation guard `1e84ecc8be35e5827a540d55c65fe839` still escaped exact reruns and remains accepted because validated module IDs generate non-empty scaffold directory names. The root baseline now accepts 160 survivor records, and `pkg/invowkmod/operations.go` has one accepted record remaining.

Eighty-first remediation batch:

- Teach the shared invowkmod string-value parser to read the first quoted CUE string even when trailing CUE syntax follows on the same line, then add compact path-field add/remove tests.
- Fresh explicit single-mutant reruns proved two accepted `pkg/invowkmod/invowkmod_edit.go` survivor records killed: `cd4a24f77a60b4d3422629042f6457cb` and `4f558b9db1ed4e301379086b63b27124`.
- The edit-list capacity record `79fd3cd7a8c99993f92b050bd2846fbf`, no-match sentinel record `a74ad8cdf03740a2102e3b4d2f8f48a0`, and `end+2` boundary records `98e422a76bc12101c83dd225cca43039` and `122333272091ac61c8d0056198b53bc7` still escaped exact reruns and remain accepted. The root baseline now accepts 158 survivor records, and `pkg/invowkmod/invowkmod_edit.go` has four accepted records remaining.

Eighty-second remediation batch:

- Add parser contract coverage for invalid module-path validation during module-attached parsing and for `ParseModule()` preserving a wrapped module-load error boundary.
- Fresh explicit single-mutant reruns proved two accepted `pkg/invowkfile/parse.go` survivor records killed: `7d9da9e9d4090311b1092bc79e328f40` and `5c01f76af32e5a4a3aa12d1c089333f6`.
- The three `ParseLoadedModuleInvowkfile()` unavailable-guard records at line 143 still escaped exact reruns and remain accepted because `Module.InvowkfilePath()` returns an empty path for library-only modules. The root baseline now accepts 156 survivor records, and `pkg/invowkfile/parse.go` has three accepted records remaining.

Eighty-third remediation batch:

- Add command-dependency contract coverage for stored `StructuredFailures` on mixed missing/forbidden command errors and for root-scope single missing-command diagnostics avoiding a synthetic empty source.
- Fresh explicit single-mutant reruns proved two accepted `internal/app/deps/deps.go` survivor records killed: `b8cf70437c3402a4aa2b5e2641537214` and `f829dceacfd1b13292ce458f88287c3c`.
- The `sourceCommandCandidates()` priority record at line 423 and three command identity helper records at lines 453, 463, and 477 still escaped exact reruns and remain accepted. The identity-helper records are equivalent under the current return-value contract, and the priority record only changes fallback map-iteration ordering. The root baseline now accepts 154 survivor records, and `internal/app/deps/deps.go` has four accepted records remaining.

Eighty-fourth remediation batch:

- Recheck the accepted `internal/app/deps/host_probe.go` records after the accumulated custom-check result coverage landed in the current checkout.
- Fresh explicit single-mutant reruns proved three accepted `internal/app/deps/host_probe.go` survivor records killed: `ae57729d5f5964e01a0d995ada09049d`, `d5adb8f569c05c614d1b473256fed9e3`, and `d4b9ed550ad35980671f42ec8d03bf96`.
- The two output-validation guard records at line 84 still escaped exact reruns and remain accepted because `CustomCheckOutput.Validate()` is intentionally nil for free-form process text. The root baseline now accepts 151 survivor records, and `internal/app/deps/host_probe.go` has two accepted records remaining.

Eighty-fifth remediation batch:

- Add tool-dependency boundary coverage for container nil/empty dependency short-circuiting and direct structured payload preservation for both runtime and host tool failures.
- Fresh explicit single-mutant reruns proved all five accepted `internal/app/deps/tools.go` survivor records killed: `43ba356c0789951f6665e84a9ca4b0fb`, `8bba0221585e4ee7b5909657ae3bb514`, `82912f81e5085fcc0370c8459d9abe9c`, `69933e3952c733fde6f3313af4c00218`, and `5ecaffda3f5c64557e58adde04f5b36b`.
- The root baseline now accepts 146 survivor records, and `internal/app/deps/tools.go` has no accepted records remaining.

Eighty-sixth remediation batch:

- Add provider-result validation coverage for invalid config payloads and invalid source paths, plus explicit `LoadWithSource()` load-error propagation for missing config files.
- Fresh explicit single-mutant reruns proved all five accepted `internal/config/provider.go` survivor records killed: `a0b8c294f199ef687e3868101e7a6d03`, `143b758484efe48d68ebf1bb1917304f`, `741162aaf9d1e91d6060b9b94b441730`, `964acb236643127282151cfd1452b7fc`, and `4e48a5ce85bf4a41db8ddd15af3c51bc`.
- The root baseline now accepts 141 survivor records, and `internal/config/provider.go` has no accepted records remaining.

Eighty-seventh remediation batch:

- Add runtime-config validation coverage proving contextual command/implementation wrapping preserves the first field-level runtime error sentinel.
- Fresh explicit single-mutant reruns proved one accepted `pkg/invowkfile/invowkfile_validation_struct.go` survivor record killed: `c130dc56691eefe5bfcedbe13a10128c`.
- The malformed aggregate guard records `e146a2accf949f6fbd4fd702c8aa9fd7`, `86633116f4f584499a4a8d43f83d02a5`, `92955ef5cb71ad1702d33b62d7d5fecf`, and `1b782ecc0452a016a64e4e4c45c97f56` still escaped exact reruns and remain accepted because current `RuntimeConfig.Validate()` paths do not return an `InvalidRuntimeConfigError` with no field errors. The root baseline now accepts 140 survivor records, and `pkg/invowkfile/invowkfile_validation_struct.go` has four accepted records remaining.

Eighty-eighth remediation batch:

- Add command-scope validation coverage for invalid module IDs when an explicit source ID is set, and add transitive-policy coverage for skipped incomplete vendored modules plus resolved-module requiring URL preservation.
- Fresh explicit single-mutant reruns proved six accepted `pkg/invowkmod` survivor records killed: `b8b06fd78d5623524d7fa6d0d607ab37`, `cdeff4498b9c4f87e57561229b3e958b`, `4d096a6aa7b8ce0b33c69c715c243466`, `21049436185c8015f85ec0d9d8b0f38b`, `cd2bb8db3e9497d310f5461237718f6c`, and `87a3948d90fea5e8c7ee8a46654a3345`.
- The default-source fallback records `6e96678c6d3cef42811502b0225f310e` and `522b85dbbfb2629949ea78a86c9b34e6` still escaped exact reruns and remain accepted because every current valid `ModuleID` is also a valid default `ModuleSourceID`. The root baseline now accepts 134 survivor records, `pkg/invowkmod/command_scope_validate.go` has two accepted records remaining, and `pkg/invowkmod/transitive_policy.go` has no accepted records remaining.

Eighty-ninth remediation batch:

- Add typed payload assertions for malformed glob patterns, invalid Git URL/commit values, and invalid semantic version/constraint/operator values. Also expand argument-structure diagnostics to cover empty names, invalid POSIX names, and invalid argument types.
- Fresh explicit single-mutant reruns proved eight accepted survivor records killed: `528dad5062fa7a079dba906eb71e6c32`, `736ba18b3dc28d117a1b194fb146b8b5`, `f7119161d87f3f74095e45c6a2062e69`, `1b104b9a64cbb42235873b9412034265`, `caf67e4c0c2fcecb1b14c55db70224c6`, `c73560d7d3807df9feb09cf7d9a8882b`, `89e6b44bfecbf66ef494112f45cc802a`, and `0398ed4a1c2052be44fa7582a82508f0`.
- The empty-glob `Value` field-clear record `bdd8f4b2624b5efb6a68bdeb43b76de5`, empty-Git-URL guard record `11441bdbc574304e3269e81e6eea1c2a`, argument severity/default no-args records, and sampled config defensive validation guards still escaped exact reruns and remain accepted as equivalent-looking under current zero-value and redundant-validation contracts. The root baseline now accepts 126 survivor records, `pkg/invowkfile/watch.go` has one accepted record remaining, `pkg/invowkmod/git_types.go` has one accepted record remaining, and `pkg/invowkmod/semver_types.go` has no accepted records remaining.

Ninetieth remediation batch:

- Add shared value-type payload assertions proving invalid `pkg/types` errors preserve the offending value for description text, exit codes, filesystem paths, listen ports, runtime modes, and shell paths.
- Fresh explicit single-mutant reruns proved six accepted `pkg/types` survivor records killed: `d152039399be0f16d5decaec9d793307`, `a657e9de1544c4bff33eb2d83551c19e`, `7f099c9babc5e0ee4da6e5dc55b82940`, `ef176ef81601e42ecadbd8e76fb15f19`, `b2ee608bd439c483b0bd491ee617c6e4`, and `3bc19562b5eaf9bca08034d95c68884e`.
- The root baseline now accepts 120 survivor records, and the sampled `pkg/types` value-error payload records have no accepted records remaining.

Ninety-first remediation batch:

- Add `pkg/invowkfile` value-error payload assertions for dotenv file paths, env aggregate field errors, port mappings, volume mounts, and workdir values.
- Fresh explicit single-mutant reruns proved seven accepted `pkg/invowkfile` survivor records killed: `4a84e8745703bc6cf1d0f709456f6156`, `a869088b7c549cea3380fb5e16c3232e`, `201f08ac1c2e7bd98666dac70da48d57`, `70629a72a03c58ec5f705efd15ced267`, `5b431d40ea40b90cd717b0c9f6de353a`, `6ae7ec8a86909acf9a23bf96c92e0944`, and `515c74d2183ae6a9253e11477dddb829`.
- The empty argument-name and flag-name `Value` field-clear records `d17a6322394f83d1ba4767b483d3be30` and `9450c4dac3484afc7f9d91a5f718ec28` still escaped exact reruns and remain accepted because the invalid empty value is already the zero value. The root baseline now accepts 113 survivor records.

Ninety-second remediation batch:

- Add `internal/discovery` payload assertions for invalid diagnostic severity/code values and command-tree validation coverage for conflict entry file paths. Add `pkg/cueutil` contracts for invalid CUE path value preservation and `WithConcrete()` option assignment.
- Fresh explicit single-mutant reruns proved six accepted survivor records killed: `1d3a99c8389d491f6a117f41a120dfd6`, `7d43050edf8eb8893c0e7c18efce2892`, `d7c97f6697aef263924f79c0d807d3c1`, `cc4c69cc826f3cb2acdb7694885a409a`, `fb53db15a616f139cd298b51da88c3b4`, and `307cf7499a9909d68a36386b3dfdd9a1`.
- The sampled `pkg/invowkfile/validation_structure_args.go`, `internal/config`, and `internal/discovery/discovery_files.go` records still escaped exact reruns and remain accepted as equivalent-looking zero-value or redundant-validation contracts. The root baseline now accepts 107 survivor records.

Ninety-third remediation batch:

- Add focused assertions for `pkg/invowkfile` module metadata field errors, no-command structure validator identity, and module script-base path precedence. Add `internal/core/serverbase` invalid-state payload assertions and `internal/provisionenv` manifest validation coverage for invalid entries and all-entry scanning.
- Fresh explicit single-mutant reruns proved eight accepted survivor records killed: `f72048960e53baf495258ab0b40430c5`, `7b369bb331132237a31026f4bdefb060`, `2aac354d60db6c22300790593f0dc88f`, `755193adc2bf3c56a5e17ace34a74422`, `4251ed812e2705cdc4a40a55e59df2dc`, `aaaf5c07d80f6f749c90aea4d4eb4bb3`, `23a0f44c2b68ed124ddd952db1d49e2e`, and `097477fe2b2991385b7555e5224f76aa`.
- The sampled zero-value and platform-equivalent records for structure/runtime severity, module empty-slice copying, `ModeNone`, serverbase initial state, and provision-env free-form value validation still escaped exact reruns and remain accepted. The root baseline now accepts 99 survivor records.

Ninety-fourth remediation batch:

- Add concrete structure-validator constructor coverage in `pkg/invowkfile`, parent-directory validation coverage for `pkg/invowkmod` create options, and `internal/app/deps` contracts proving `ExecutionContext.GoContext()` preserves explicit contexts and falls back from nil contexts.
- Fresh explicit single-mutant reruns proved four accepted survivor records killed: `1f858bb8141ef016a58ed3a15a81cfcc`, `001ebecc01483c4178d624336bf08fa7`, `27637618c504eba5d173c30198336842`, and `24073617015f4e8f75b94861e96bbe68`.
- The sampled CUE non-concrete validation guard, Linux-profile absolute-path fallback, script-interpreter non-concrete guard, env-var trim/regex overlap, container absolute path guards, path-cleaned `/../` check, empty glob and Git URL zero values, generated scaffold validation, and ignored-entry verify guards still escaped exact reruns and remain accepted as equivalent-looking under current contracts. The root baseline now accepts 95 survivor records.

Ninety-fifth remediation batch:

- Add a `pkg/invowkmod` module-ref source ID assertion for `ssh://` URLs with scp-like repository suffixes, proving scheme stripping is observable before the fallback colon split.
- Fresh explicit single-mutant reruns proved the duplicated `pkg/invowkmod/dependency_types.go` survivor record killed at both matching sites: `89169834da15df59b308f1322a3efa7b`.
- A full terse exact-ID sweep over the current root baseline found no other historical v2.7.0 PASS-only survivor IDs; mixed duplicate-site records such as `9cd961ddec23a699b5278fa5ec83ee2e` remain accepted because at least one matching site still escaped. The root baseline now accepts 94 survivor records.

Ninety-sixth remediation batch:

- A fresh explicit single-mutant rerun against the current working tree proved the accepted `internal/app/deps/deps.go` branch survivor `9cd961ddec23a699b5278fa5ec83ee2e` killed across all matching sites, with 3 killed and 0 escaped mutants.
- Sampled exact reruns for remaining `internal/app/deps`, `internal/config`, `pkg/cueutil`, `pkg/fspath`, `pkg/invowkfile`, and `pkg/invowkmod` records still escaped and remain accepted as equivalent-looking zero-value, capacity, redundant-validation, or hard-to-induce OS-error contracts. The root baseline now accepts 93 survivor records.

Ninety-seventh remediation batch:

- Add a module metadata accessor contract proving absent `requires` stays nil through `ModuleMetadata.Requires()`, while existing non-empty requirements remain defensively copied by adjacent coverage.
- Fresh explicit single-mutant reruns proved both accepted `pkg/invowkfile/module.go` survivor records killed: `f50009f71ede6f698ddb00c19a1fdf7a` and `80eb186f8fb296da2a1f0b13adf2abba`.
- Sampled exact reruns for remaining `internal/app/deps`, `internal/app/execute`, `internal/app/llmconfig`, `internal/config`, `internal/core/serverbase`, `internal/discovery`, `internal/provisionenv`, `pkg/cueutil`, `pkg/fspath`, `pkg/invowkfile`, `pkg/invowkmod`, and `pkg/platform` records still escaped and remain accepted as equivalent-looking zero-value, capacity, redundant-validation, platform-profile, or hard-to-induce OS-error contracts. The root baseline now accepts 91 survivor records, and `pkg/invowkfile/module.go` has no accepted records remaining.

Ninety-eighth remediation batch:

- Fresh exact reruns proved all six accepted `pkg/invowkfile/validation_structure_args.go` survivor records killed by existing focused validation coverage: `cc083158405e530d703e30784dd54a89`, `7a653c1029a70424052d2ca863cae892`, and four rows for `ac9829e1d8ddaef9293d8ba8a7fd2e2f`.
- The root baseline now accepts 85 survivor records, and `pkg/invowkfile/validation_structure_args.go` has no accepted records remaining.

Ninety-ninth remediation batch:

- This batch's zero-baseline conclusion is superseded by the corrected root accepted-survivor audit near the Root Profile summary.
- The corrected audit shows the committed root baseline now accepts 181 current survivor rows covering 160 stable mutant IDs.

One hundredth remediation batch:

- Fresh exact reruns proved five accepted `pkg/invowkmod/resolver_validate.go` survivor records killed by focused validation-payload tests: `02497ba281ba0e1f367f4c322d58f1eb`, `e238ff6f626e6c526aaf89a84ed089a4`, `291390200dee7c2fe3525eb0b622c146`, `d30f4c01300474041d87f0f2a0bef9aa`, and `ffd32a4174bc8fd5beda58843222a1b7`.
- The `ModuleNamespace` guard records `937b4a210b38fbe7c4a6929349c005ea` and `b0b78208c95792b81e85030eee1ac4f7` still escaped exact reruns and remain accepted because `ModuleNamespace.Validate()` only rejects the empty value while the guarded branches only validate non-empty namespaces. The root baseline now accepts 181 survivor records, and `pkg/invowkmod/resolver_validate.go` has four accepted records remaining.

One hundred and first remediation batch:

- Fresh exact reruns proved four accepted `pkg/invowkmod/semver.go` sort-comparator survivor records killed by equal-precedence version ordering coverage: `fded7fda3f63509df7e9b5c82687cc19`, `b5dfde5e034703e5f4109d7b6a8a6b47`, `1fa4d198fdecd1122af1a1a9c52c415b`, and `340e2f9fcd00b7ca48031536caf342d9`.
- The `Version.Compare()` `<=` records at lines 96, 103, 110, and 130 still escaped exact reruns and remain accepted because each mutation is inside a branch that has already proved the values unequal. The `ParseConstraint()` operator-validation guard `be17c90d51426e1dd3598cd3e3225a14` also remains accepted because the constraint regex only yields known operators. The root baseline now accepts 177 survivor records, and `pkg/invowkmod/semver.go` has five accepted records remaining.

One hundred and second remediation batch:

- Added content-hash mutation coverage asserting invalid hash errors preserve `InvalidContentHashError.Value`, `hashFileContent()` reports `lstat` failures without writing hash bytes, and `computeModuleHash()` propagates unreadable-file permission errors with relative file context.
- Fresh exact reruns proved three accepted `pkg/invowkmod/content_hash.go` survivor records killed: `b47604333c95a3af604b8d8d6f299971`, `dfb473d0840a14fffd0da4193acbc61c`, and `bd8323411c9125b7b1eb323acafaf57c`.
- The five deferred `f.Close()` close-error condition records at line 167 and the `f.Stat()` error guard `ca8fe76f39c86892a518bd367c46871a` still escaped exact reruns and remain accepted because deterministic package tests cannot induce those OS-level errors through `hashFileContent()`'s concrete `*os.File` path without adding a production test seam. The root baseline now accepts 174 survivor records, and `pkg/invowkmod/content_hash.go` has six accepted records remaining.

One hundred and third remediation batch:

- Added operations-validation permission coverage proving non-ENOENT `invowkmod.cue` stat failures are reported as `cannot access invowkmod.cue`, not collapsed into the missing-file diagnostic.
- Fresh exact reruns proved two accepted `pkg/invowkmod/operations_validate.go` survivor records killed: `caeb88fa544084fcbb5955f1d0be46c8` and `60bf193e2e15d459d107467bba62fcc3`.
- The remaining operations-validation records still escaped exact reruns: the `scanModuleTree()` returned-error guard is unreachable with the current callback, `FilesystemPath.Validate()` cannot fail after `filepath.Abs()` produces a non-empty path, the `true && os.IsNotExist(err)` mutation is equivalent, and the `Load()` metadata guard remains impossible after successful validation. The root baseline now accepts 172 survivor records, and `pkg/invowkmod/operations_validate.go` has six accepted records remaining.

One hundred and fourth remediation batch:

- Added `LoadAll()` collision coverage proving duplicate configured include module IDs surface as a `ModuleCollisionError` and abort the load without returning partial files.
- Fresh exact reruns proved the accepted `internal/discovery/discovery.go` module-collision error-guard survivor `0a7edec1ac46a0236c811e987ed3ba48` killed.
- The remaining eight `internal/discovery/discovery.go` records still escaped exact reruns: the default `os.Getwd()` and commands-dir error guards are difficult to induce without production seams, while the library-only/path disjunction guards in `LoadFirst()` and `loadAll()` are redundant with parser and discovery behavior. The root baseline now accepts 171 survivor rows covering 150 stable mutant IDs, and `internal/discovery/discovery.go` has eight accepted records remaining.

One hundred and fifth remediation batch:

- Added dependency contract coverage for whitespace-only binary names and command refs, exact-maximum command source IDs, direct custom-check dependencies missing scripts, and alternatives that mix direct script or expected-output fields.
- Fresh exact reruns proved six accepted `pkg/invowkfile/dependency.go` survivor records killed: `5246cf74b6f9c4bb0fadd3db616a791e`, `76c9215d2f6205fac737094f79db5e14`, `25752f62b95c6bae3e61a08b47b9693d`, `962b42a637c4694c85c9863bdc95d3f8`, `0bf884cc767d5e37bd6f81860b038514`, and `de43060570b20d71f10a9a56926e5f55`.
- The remaining `CommandDependencyRef.Parse()` and qualified-source error guards `bfc475e48d22374af0880625d6c03079` and `c7e1e9f11225adb1e0124a00ec4fcffb` still escaped exact reruns and remain accepted because later structured validation returns the same public error. The root baseline now accepts 165 survivor rows covering 144 stable mutant IDs, and `pkg/invowkfile/dependency.go` has two accepted records remaining.

One hundred and sixth remediation batch:

- Added filepath dependency coverage proving nil and empty container filepath dependencies skip the runtime probe, container filepath failures preserve structured failure payloads, slash-absolute host paths are not joined to the invowkfile directory, and single-alternative host errors keep the raw probe detail.
- Fresh exact reruns proved six accepted `internal/app/deps/filepaths.go` survivor records killed: `3ad6d74ea10d6bbc85ef5864440a7be8`, `43933fb78f25c70922ae3bb191735007`, `6b2285144ac55c868471f2375fcaac07`, `6fbbdd4157a6d54965047b053a8c53e9`, `8603fb28830aa2026a834e29fc10ac62`, and `a8a304b06b74cc45e8d4af1ce22f70eb`.
- The shared absolute-path branch ID `a3863647bd2462895996068bad36996e` still escaped exact reruns at both branch sites and remains accepted because the remaining behavior depends on host-native absolute path rules not observable on this Linux run. The root baseline now accepts 159 survivor rows covering 138 stable mutant IDs, and `internal/app/deps/filepaths.go` has two accepted records remaining.

One hundred and seventh remediation batch:

- Fresh exact reruns proved all twelve accepted `internal/config` survivor records killed by existing focused config coverage: `8c4a4614f5a6cc96fa48fb30208faccb`, `131284452621f9b3f6280a5763c4f0c3`, `64a35ca918593a5a26da1edadc602e3d`, `2545ece145717e0749df23a049900c60`, `0a8d25141599bfd91e5878c6bd8c683f`, `8f0a8255120bd01e6b97668e86376af9`, `71095fba576a612e1ee817c1c3b43ee7`, `d35751a473bb7a0b900926918e524bc5`, `35d6080ab9ac9e7c32bc1be7bb0efbf5`, `d07a0d7c32e673139cab6675010fbc32`, `2b9ba24205d3c89212277d678f9d8d04`, and `a66655ac3fcd3a722129afda6e02988e`.
- The root baseline now accepts 147 survivor rows covering 126 stable mutant IDs, and `internal/config/config.go` plus `internal/config/types.go` have no accepted records remaining.

One hundred and eighth remediation batch:

- Fresh exact reruns proved all sixteen accepted `pkg/invowkfile/validation_structure_command.go` survivor records killed by existing focused validation-structure coverage: `043a7a04a1cd37e04ad3e5161fa6406e`, `208bf87f0a93eec5e85e2f33bb8eb39f`, `5bdc85a32ebc268d6149d72be96327df`, `668cb62d3c4180fca08f5a33c89dccd2`, and `be9abefb2b214c67166ba6615ba1b69f`.
- The duplicate-site IDs `208bf87f0a93eec5e85e2f33bb8eb39f` and `668cb62d3c4180fca08f5a33c89dccd2` were removed only after the exact reruns showed every matching site failed. The root baseline now accepts 131 survivor rows covering 121 stable mutant IDs, and `pkg/invowkfile/validation_structure_command.go` has no accepted records remaining.

One hundred and ninth remediation batch:

- Fresh exact reruns proved all twelve accepted `pkg/invowkfile/validation_structure_args.go` survivor records killed by existing focused argument-structure coverage: `7a653c1029a70424052d2ca863cae892`, `ac9829e1d8ddaef9293d8ba8a7fd2e2f`, and `cc083158405e530d703e30784dd54a89`.
- The duplicate-site ID `ac9829e1d8ddaef9293d8ba8a7fd2e2f` was removed only after all ten matching sites failed under the exact rerun. The root baseline now accepts 119 survivor rows covering 118 stable mutant IDs, and `pkg/invowkfile/validation_structure_args.go` has no accepted records remaining.

One hundred and tenth remediation batch:

- Fresh exact reruns proved all eight accepted `internal/discovery/discovery_commands.go` survivor records killed by existing discovery command-set coverage: `171d14e79216dadc07594f5dd4deea41`, `255eb50650713787bb9adbaf7eda105e`, `2799b64dd28eae3c04de345aa4b018c4`, `388b7b083ee2916fc2b502d5e3be24d1`, `5e6674e17fef51cc4fa120a2b02bf596`, `86531b3722115bda9a11982aaf61ef4f`, `950358334c48d0851aee9b4dbccabbbf`, and `eb2d70cd1a70757c433759529e290420`.
- The root baseline now accepts 111 survivor rows covering 110 stable mutant IDs, and `internal/discovery/discovery_commands.go` has no accepted records remaining.

One hundred and eleventh remediation batch:

- Fresh exact reruns proved all eight accepted `internal/discovery/discovery.go` survivor records killed by existing discovery lifecycle and library-only coverage: `37a0917ba5968e571dc9b1d2b101c695`, `40b768df087ab427fa7b0d974cc28213`, `6e096a861724dddc754851e36ea4e4a0`, `a84f3ad012f883b2a25dd2cbf0cf186f`, `ac8bdade87211c1b4da278d981c5093b`, `cc7616bf46bc58f39879fbd4c73bd3ea`, `d3e09d06a2d71fd53746d1759deb63f5`, and `d53ca632a4baa4e9b5f022089e9a3661`.
- The root baseline now accepts 103 survivor rows covering 102 stable mutant IDs, and `internal/discovery/discovery.go` has no accepted records remaining.

One hundred and twelfth remediation batch:

- Fresh exact reruns proved all six accepted `pkg/invowkmod/operations_validate.go` survivor records killed by existing module-validation coverage: `4a60b03ed81dd05eb142e62693e7a8fb`, `620748d1013c82b08b1d8d55508d415f`, `77e0b8ec4361d8413e9677161fedc55f`, `7c7bf95d3565d218bda33cb9b8596b22`, `b69e8c821ab0fe5623a5fd24fc9d8e2c`, and `be11e8f377b644cf818e8d52fe291891`.
- The root baseline now accepts 97 survivor rows covering 96 stable mutant IDs, and `pkg/invowkmod/operations_validate.go` has no accepted records remaining.

One hundred and thirteenth remediation batch:

- Fresh exact reruns proved all six accepted `pkg/invowkmod/content_hash.go` survivor records killed by existing content-hash coverage: `01019bb1d5981fbef7ea87a4c7f13a6d`, `6301ca8145720964fbb3340fc779ab9e`, `723f2dff8e48e3e4c596d2a10b0a7804`, `ca8fe76f39c86892a518bd367c46871a`, `edb156e489a60a3ca420fcbd94979b53`, and `f9b124b9f677200c19971f07b0426f76`.
- The root baseline now accepts 91 survivor rows covering 90 stable mutant IDs, and `pkg/invowkmod/content_hash.go` has no accepted records remaining.

One hundred and fourteenth remediation batch:

- Fresh exact reruns proved all five accepted `pkg/invowkmod/semver.go` survivor records killed by existing semantic-version comparison and constraint coverage: `3a97ffec512918954bc03c140870585c`, `be17c90d51426e1dd3598cd3e3225a14`, `c0b5a6979a73e78eae95fa00c8d2aea5`, `e5099874de4b107ef5bc0bfa12b51aff`, and `f65fd5a40d510a9162961a3a9c3e9fff`.
- The root baseline now accepts 86 survivor rows covering 85 stable mutant IDs, and `pkg/invowkmod/semver.go` has no accepted records remaining.

One hundred and fifteenth remediation batch:

- Fresh exact reruns proved all five accepted `internal/discovery/discovery_files.go` survivor records killed by existing discovery file, vendored module, and shadowing coverage: `0adc23b771d073888d94a545eb1388c5`, `c948a9644826d365610499bc3031a733`, `da4c79145af11f130f3d2472da1bb980`, `eb5a0f4d7e42c6bfc75c83bc2ae3f1bd`, and `eed598cb6e26f2355fe86b2ac0bef3fd`.
- The root baseline now accepts 81 survivor rows covering 80 stable mutant IDs, and `internal/discovery/discovery_files.go` has no accepted records remaining.

One hundred and sixteenth remediation batch:

- Fresh exact reruns proved all accepted records killed for `pkg/invowkmod/resolver_validate.go`, `pkg/invowkmod/invowkmod_edit.go`, and `pkg/invowkfile/invowkfile_validation_struct.go`: `36caa7ea7a14fdf77803611bee45c79e`, `937b4a210b38fbe7c4a6929349c005ea`, `b0b78208c95792b81e85030eee1ac4f7`, `e48fcdbf167f126f34a99c62144bad22`, `122333272091ac61c8d0056198b53bc7`, `79fd3cd7a8c99993f92b050bd2846fbf`, `98e422a76bc12101c83dd225cca43039`, `a74ad8cdf03740a2102e3b4d2f8f48a0`, `1b782ecc0452a016a64e4e4c45c97f56`, `86633116f4f584499a4a8d43f83d02a5`, `92955ef5cb71ad1702d33b62d7d5fecf`, and `e146a2accf949f6fbd4fd702c8aa9fd7`.
- Fresh exact reruns also proved three accepted `internal/app/deps/deps.go` records killed: `16593089179b8ab5c8ef9a5401fb6444`, `54e39bc0a9529881e6b9d6cdbfc93496`, and `6c020c99b351e658be18df8789790f56`.
- The shared `internal/app/deps/deps.go` branch ID `9cd961ddec23a699b5278fa5ec83ee2e` remains accepted because the exact rerun failed at line 423 but still passed at sibling branch sites for the same stable ID. The root baseline now accepts 66 survivor rows covering 65 stable mutant IDs, and `pkg/invowkmod/resolver_validate.go`, `pkg/invowkmod/invowkmod_edit.go`, and `pkg/invowkfile/invowkfile_validation_struct.go` have no accepted records remaining.

One hundred and seventeenth remediation batch:

- Fresh exact reruns proved all accepted records killed for `pkg/invowkmod/verify.go`, `pkg/invowkmod/lockfile.go`, `pkg/invowkmod/invowkmod.go`, `pkg/invowkfile/parse.go`, and `pkg/fspath/atomic.go`: `6b5e6fd61314dbc1640786322e1e61f5`, `73722f5b0a9acc58b30909ceb5f09869`, `f19f89fbf474913128ceb118b2983463`, `50fae5bf607ffbf501f0c5c2e8c33f8a`, `6019b20a6b739a4a8f60d7a1b0649c17`, `7bbdbda4ba5d4ccd0e42319c999089a1`, `3769fd5c1400c9e5c715ddca3c25fff7`, `4ab9c5e50904f654f821e2dc5cc0c0cf`, `9ba9b26a1344ed06afa0e79296cbe719`, `504f07aa3cbe5b1a8ebb38ac5de5c7a4`, `77c5917c08a06f38ae13c8565be34117`, `fbb3f437c293a22523a80f060ba7df5c`, `2d46e98ac61114c7ddcbbfc373f57cca`, `558840d98feb2a5b0fb11a8edcd2b6ea`, and `5639b6974fd232084b1deb951cc981f0`.
- The root baseline now accepts 51 survivor rows covering 50 stable mutant IDs, and all five swept files have no accepted records remaining.

One hundred and eighteenth remediation batch:

- Ran a terse exact sweep over every remaining root baseline stable ID after the focused cluster pruning. Every remaining ID failed all exact rerun sites except `9cd961ddec23a699b5278fa5ec83ee2e` and `e9d96bf0afe1b2371b06c438c13275b6`.
- The shared `internal/app/deps/deps.go` branch ID `9cd961ddec23a699b5278fa5ec83ee2e` remains accepted because its exact rerun still reports mixed duplicate-site output: the baseline row at line 423 fails, while sibling branch sites at lines 428 and 439 pass.
- The `pkg/cueutil/parse.go` branch ID `e9d96bf0afe1b2371b06c438c13275b6` remains accepted because its exact rerun still reports a passing site. The root baseline now accepts 2 survivor rows covering 2 stable mutant IDs.

One hundred and nineteenth remediation batch:

- Re-ran the compact `tools/goplint` clusters with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'` from the `tools/goplint` module root. Under `go-mutesting` v2.7.0 terminal output, `PASS` lines are killed mutants and `FAIL` lines are escaped mutants.
- Fresh exact reruns proved 163 accepted `tools/goplint` survivor records killed with no escaped rows in the swept set: `goplint/analyzer.go` (38), `goplint/analyzer_constructor_usage.go` (25), `goplint/analyzer_path_domain.go` (22), `goplint/analyzer_pathmatrix_divergent.go` (21), `goplint/analyzer_validate_delegation_modes.go` (20), `goplint/analyzer_test_home_env.go` (12), `goplint/analyzer_nonzero.go` (9), `goplint/analyzer_redundant_conversion.go` (8), `goplint/analyzer_run.go` (5), and `goplint/analyzer_validate_usage.go` (3).
- The duplicate-site reruns for `goplint/analyzer_path_domain.go` line 180 and `goplint/analyzer_validate_delegation_modes.go` line 285 each reported five killed sites and zero escaped sites, so those stable IDs were pruned with the rest of the compact batch.
- A same-turn root survivor spot check confirmed the then-retained `internal/app/deps/deps.go` and `pkg/cueutil/parse.go` rows were still escaped or mixed. The broader root integrity audit in the next batch supersedes the two-row root count. The goplint baseline now accepts 649 survivor rows covering 644 stable mutant IDs.

One hundred and twentieth remediation batch:

- Audited the prior committed root baseline from `HEAD` against the current checkout with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'`. The audit covered 362 unique prior root stable IDs.
- The live audit classified 138 root stable IDs as still escaped, 211 as killed, and 13 as obsolete/no longer emitted by the current mutation target set. The repaired root baseline stores one accepted row per still-escaped stable ID because baseline suppression is keyed by stable mutant ID.
- Restored the still-escaped root IDs to `tools/mutation/baselines/root-baseline.json`, replacing the over-pruned two-row baseline. The repaired root baseline now accepts 138 survivor rows covering 138 stable mutant IDs.

One hundred and twenty-first remediation batch:

- Re-ran the medium `tools/goplint` clusters with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'` from the `tools/goplint` module root.
- Fresh exact reruns proved 127 accepted `tools/goplint` survivor rows killed with no escaped rows in the swept set: `goplint/analyzer_cast_validation.go` (33), `goplint/analyzer_enum_sync.go` (37), and `goplint/analyzer_structural.go` (57).
- The structural branch ID `9ee2cd9382d66d873957e40615b1e18c` appeared in two accepted rows, and both exact reruns reported killed sites only. The batch removed 127 accepted rows covering 126 stable IDs. The goplint baseline now accepts 522 survivor rows covering 518 stable mutant IDs.

One hundred and twenty-second remediation batch:

- Re-ran every remaining accepted `tools/goplint` stable ID with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'` from the `tools/goplint` module root. The proof covered 522 accepted rows covering 518 stable mutant IDs and emitted only killed rows, with no escaped or error rows.
- Fresh exact reruns proved the remaining accepted `tools/goplint` survivor rows killed: `goplint/analyzer_boundary_request_validation.go` (66), `goplint/analyzer_constructor_validates.go` (86), the former constructor-validates CFA implementation (94), `goplint/analyzer_cross_platform_path.go` (60), `goplint/analyzer_validate_delegation.go` (105), and `goplint/analyzer_windows_pitfalls.go` (111).
- The duplicate-site IDs `315fd0aa5bb486e5355539dcef93cc87`, `40c21a286186dfcc53666e46830f9d3f`, `7bdd7a9242372102f5fa04aede64be83`, and `a323a56d53585afece2ed1bc8fcf6cda` each reported killed sites only. The batch removed 522 accepted rows covering 518 stable IDs. The goplint baseline now accepts 0 survivor rows covering 0 stable mutant IDs.

One hundred and twenty-third remediation batch:

- Added focused config-loading coverage for the case where root `includes` and nested `container.auto_provision.includes` both fail validation. The test asserts that `loadWithOptions` preserves the first include collection error as the normalized top-level config error instead of leaking later include collection failures.
- Fresh exact reruns of the `internal/config` survivor cluster with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `0a8d25141599bfd91e5878c6bd8c683f` killed. The other eleven `internal/config` survivor rows still escaped and remain accepted.
- The root baseline now accepts 137 survivor rows covering 137 stable mutant IDs, and `internal/config/config.go` has five accepted records remaining.

One hundred and twenty-fourth remediation batch:

- Added resolver-validation payload coverage proving `InvalidModuleRefError.Error()` retains formatted field-error context and `RemoveResult.Validate()` preserves both invalid lock-key and removed-entry validation failures.
- Fresh exact reruns of the `pkg/invowkmod/resolver_validate.go` survivors with explicit stable IDs, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `e48fcdbf167f126f34a99c62144bad22` and `36caa7ea7a14fdf77803611bee45c79e` killed.
- The `ModuleNamespace` guard records `937b4a210b38fbe7c4a6929349c005ea` and `b0b78208c95792b81e85030eee1ac4f7` still escaped focused reruns and remain accepted because `ModuleNamespace.Validate()` only rejects the empty value while these branches only validate non-empty namespaces. The root baseline now accepts 135 survivor rows covering 135 stable mutant IDs, and `pkg/invowkmod/resolver_validate.go` has two accepted records remaining.

One hundred and twenty-fifth remediation batch:

- Added manual-debounce watcher coverage proving a scheduled timer fire after context cancellation skips `OnChange` while still allowing `Run` cleanup to finish and close the watcher backend.
- A fresh exact rerun of `internal/watch/watcher.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `84ac84318770d53e44aff93ebe40534e` killed.
- The sibling empty-pending guard record `367e6a2bb66771e02dcebb43b0798951` still escaped exact rerun and remains accepted. Same-turn exact sweeps over the remaining root baseline clusters produced no additional killed-only IDs. The root baseline now accepts 134 survivor rows covering 134 stable mutant IDs, and `internal/watch/watcher.go` has one accepted record remaining.

One hundred and twenty-sixth remediation batch:

- Added compact CUE requirement-edit coverage proving `AddRequirement` detects a duplicate entry even when one `requires` entry closes and the next opens on the same physical line (`}, {`).
- A fresh exact rerun of `pkg/invowkmod/invowkmod_edit.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `122333272091ac61c8d0056198b53bc7` killed.
- The sibling remove-side compact-boundary record `98e422a76bc12101c83dd225cca43039` still escaped exact rerun and remains accepted. The root baseline now accepts 133 survivor rows covering 133 stable mutant IDs, and `pkg/invowkmod/invowkmod_edit.go` has three accepted records remaining.

One hundred and twenty-seventh remediation batch:

- Added serial discovery-initialization coverage proving `New` records a `working_dir_unavailable` diagnostic and leaves `baseDir` empty when `os.Getwd()` fails because the process working directory was removed.
- A fresh exact rerun of `internal/discovery/discovery.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `a84f3ad012f883b2a25dd2cbf0cf186f` killed.
- The root baseline now accepts 132 survivor rows covering 132 stable mutant IDs, and `internal/discovery/discovery.go` has seven accepted records remaining.

One hundred and twenty-eighth remediation batch:

- Added serial discovery-initialization coverage proving `New` records a `commands_dir_unavailable` diagnostic and leaves `commandsDir` empty when the user home directory cannot be resolved.
- A fresh exact rerun of `internal/discovery/discovery.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `40b768df087ab427fa7b0d974cc28213` killed.
- The root baseline now accepts 131 survivor rows covering 131 stable mutant IDs, and `internal/discovery/discovery.go` has six accepted records remaining.

One hundred and twenty-ninth remediation batch:

- Fixed compact CUE requirement removal so removing a `requires` entry that starts on a shared `}, {` line preserves the previous entry's closing brace/comma instead of corrupting it.
- Added focused edit coverage proving `RemoveRequirement` skips the previous compact-adjacent entry when removing the second entry.
- A fresh exact rerun of `pkg/invowkmod/invowkmod_edit.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `98e422a76bc12101c83dd225cca43039` killed.
- The root baseline now accepts 130 survivor rows covering 130 stable mutant IDs, and `pkg/invowkmod/invowkmod_edit.go` has two accepted records remaining.

One hundred and thirtieth remediation batch:

- Added manual debounce coverage proving a stale timer fire after a previous fire drained all pending paths does not invoke `OnChange` with an empty changed-path list.
- A fresh exact rerun of `internal/watch/watcher.go` with explicit stable ID, `--output-statuses=ke`, and `--test-flags='-short -count=1'` proved `367e6a2bb66771e02dcebb43b0798951` killed.
- The root baseline now accepts 129 survivor rows covering 129 stable mutant IDs, and `internal/watch/watcher.go` has no accepted records remaining.

One hundred and thirty-first remediation batch:

- Removed equivalent guard/comparison structure that produced obsolete root survivors without changing behavior: `ParseLoadedModuleInvowkfile()` now relies on `Module.InvowkfilePath()` for library-only emptiness, `Version.Compare()` uses `cmp.Compare()` for already-unequal version components, and `Load()` checks the post-validation metadata invariant directly.
- Fresh exact reruns with stable IDs `504f07aa3cbe5b1a8ebb38ac5de5c7a4`, `77c5917c08a06f38ae13c8565be34117`, `fbb3f437c293a22523a80f060ba7df5c`, `f65fd5a40d510a9162961a3a9c3e9fff`, `3a97ffec512918954bc03c140870585c`, `e5099874de4b107ef5bc0bfa12b51aff`, `c0b5a6979a73e78eae95fa00c8d2aea5`, `620748d1013c82b08b1d8d55508d415f`, and `b69e8c821ab0fe5623a5fd24fc9d8e2c`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 120 survivor rows covering 120 stable mutant IDs, `pkg/invowkfile/parse.go` has no accepted records remaining, `pkg/invowkmod/semver.go` has one accepted record remaining, and `pkg/invowkmod/operations_validate.go` has four accepted records remaining.

One hundred and thirty-second remediation batch:

- Removed equivalent ambiguity-analysis and source-ordering mutation surfaces without changing discovery behavior: `Analyze()` now detects multi-source command names through a `SourceID` set helper and sorts sources through a root-first sort key.
- Fresh exact reruns with stable IDs `255eb50650713787bb9adbaf7eda105e`, `eb2d70cd1a70757c433759529e290420`, `950358334c48d0851aee9b4dbccabbbf`, `2799b64dd28eae3c04de345aa4b018c4`, and `388b7b083ee2916fc2b502d5e3be24d1`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 115 survivor rows covering 115 stable mutant IDs, and `internal/discovery/discovery_commands.go` has three accepted records remaining.

One hundred and thirty-third remediation batch:

- Removed stale non-module duplicate bookkeeping from `DiscoverCommandSet()`. Current discovery emits only one non-module file and `FlattenCommands()` has already collapsed duplicate command names before command-set construction, so the guard could not change observable command payloads.
- Fresh exact reruns with stable IDs `171d14e79216dadc07594f5dd4deea41`, `5e6674e17fef51cc4fa120a2b02bf596`, and `86531b3722115bda9a11982aaf61ef4f`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 112 survivor rows covering 112 stable mutant IDs, and `internal/discovery/discovery_commands.go` has no accepted records remaining.

One hundred and thirty-fourth remediation batch:

- Replaced the hand-written `Close()` error guard in `hashFileContent()` with `errors.Join(err, f.Close())`, preserving normal success/error outcomes while retaining both errors if content hashing and close both fail.
- Fresh exact reruns with stable IDs `01019bb1d5981fbef7ea87a4c7f13a6d`, `6301ca8145720964fbb3340fc779ab9e`, `723f2dff8e48e3e4c596d2a10b0a7804`, `edb156e489a60a3ca420fcbd94979b53`, and `f9b124b9f677200c19971f07b0426f76`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 107 survivor rows covering 107 stable mutant IDs, and `pkg/invowkmod/content_hash.go` has one accepted record remaining.

One hundred and thirty-fifth remediation batch:

- Removed duplicate library-only checks in discovery loading paths. `DiscoveredFile.Path` already records `Module.InvowkfilePath()`, which is empty for library-only modules, so the path is the single command-bearing invowkfile signal.
- Fresh exact reruns with stable IDs `37a0917ba5968e571dc9b1d2b101c695`, `6e096a861724dddc754851e36ea4e4a0`, `ac8bdade87211c1b4da278d981c5093b`, `cc7616bf46bc58f39879fbd4c73bd3ea`, `d3e09d06a2d71fd53746d1759deb63f5`, and `d53ca632a4baa4e9b5f022089e9a3661`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 101 survivor rows covering 101 stable mutant IDs, and `internal/discovery/discovery.go` has no accepted records remaining.

One hundred and thirty-sixth remediation batch:

- Simplified config validation/default construction without changing schema-backed defaults: `VirtualConfig.Validate()` now delegates directly to `Utilities.Validate()`, and `DefaultConfig()` assigns the embedded CUE source fields explicitly before decode.
- Fresh exact reruns with stable IDs `71095fba576a612e1ee817c1c3b43ee7`, `d35751a473bb7a0b900926918e524bc5`, `2b9ba24205d3c89212277d678f9d8d04`, and `a66655ac3fcd3a722129afda6e02988e`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 97 survivor rows covering 97 stable mutant IDs, and `internal/config/types.go` has two accepted records remaining.

One hundred and thirty-seventh remediation batch:

- Removed equivalent generated-path validation guards from config discovery paths and derived include short names directly after `ModuleIncludePath.Validate()` had already checked the module directory basename.
- Fresh exact reruns with stable IDs `8c4a4614f5a6cc96fa48fb30208faccb`, `131284452621f9b3f6280a5763c4f0c3`, `64a35ca918593a5a26da1edadc602e3d`, `2545ece145717e0749df23a049900c60`, and `8f0a8255120bd01e6b97668e86376af9`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 92 survivor rows covering 92 stable mutant IDs, and `internal/config/config.go` has no accepted records remaining.

One hundred and thirty-eighth remediation batch:

- Removed equivalent discovery-file guards without changing behavior: ambiguous vendored lock diagnostics now use the helper's nil/non-nil result contract, vendored transitive diagnostics rely on `CheckMissingVendoredTransitiveDeps()` to ignore nil children, vendored command namespaces return the effective source ID directly after the declared-lock check, and shadow detection no longer has a no-op empty-global early return.
- Fresh exact reruns with stable IDs `0adc23b771d073888d94a545eb1388c5`, `c948a9644826d365610499bc3031a733`, `da4c79145af11f130f3d2472da1bb980`, `eb5a0f4d7e42c6bfc75c83bc2ae3f1bd`, and `eed598cb6e26f2355fe86b2ac0bef3fd`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 87 survivor rows covering 87 stable mutant IDs, and `internal/discovery/discovery_files.go` has no accepted records remaining.

One hundred and thirty-ninth remediation batch:

- Removed explicit `Severity: SeverityError` assignments from command-structure validation diagnostics whose severity already defaults to the `SeverityError` zero value.
- Fresh exact reruns with stable IDs `043a7a04a1cd37e04ad3e5161fa6406e`, `208bf87f0a93eec5e85e2f33bb8eb39f`, `5bdc85a32ebc268d6149d72be96327df`, `668cb62d3c4180fca08f5a33c89dccd2`, and `be9abefb2b214c67166ba6615ba1b69f`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 82 survivor rows covering 82 stable mutant IDs, and `pkg/invowkfile/validation_structure_command.go` has no accepted records remaining.

One hundred and fortieth remediation batch:

- Removed equivalent module-validation guard surfaces: derived absolute module paths no longer revalidate non-emptiness after successful `filepath.Abs()`, missing-file checks use `os.IsNotExist(err)` directly, and module-tree walk errors are skipped through the callback's nil-error branch while preserving the existing hard-error return if `WalkDir` itself returns an error.
- Fresh exact reruns with stable IDs `4a60b03ed81dd05eb142e62693e7a8fb`, `77e0b8ec4361d8413e9677161fedc55f`, `7c7bf95d3565d218bda33cb9b8596b22`, and `be11e8f377b644cf818e8d52fe291891`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 78 survivor rows covering 78 stable mutant IDs, and `pkg/invowkmod/operations_validate.go` has no accepted records remaining.

One hundred and forty-first remediation batch:

- Simplified command dependency source matching without changing resolution behavior: matching now compares discovered source IDs directly, source-prefixed command names trim through `strings.TrimPrefix()` only when a source exists, and candidate collection uses one ordered unique-filtering path.
- Fresh exact reruns with stable IDs `16593089179b8ab5c8ef9a5401fb6444`, `54e39bc0a9529881e6b9d6cdbfc93496`, and `6c020c99b351e658be18df8789790f56`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification. The stable ID `9cd961ddec23a699b5278fa5ec83ee2e` reran as a current killed branch mutant only, with no escaped site remaining.
- The root baseline now accepts 74 survivor rows covering 74 stable mutant IDs, and `internal/app/deps/deps.go` has no accepted records remaining.

One hundred and forty-second remediation batch:

- Removed the unreachable empty-field guard from `validateRuntimeConfig()` after confirming `RuntimeConfig.Validate()` only returns `InvalidRuntimeConfigError` after collecting at least one field error.
- Fresh exact reruns with stable IDs `1b782ecc0452a016a64e4e4c45c97f56`, `86633116f4f584499a4a8d43f83d02a5`, `92955ef5cb71ad1702d33b62d7d5fecf`, and `e146a2accf949f6fbd4fd702c8aa9fd7`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 70 survivor rows covering 70 stable mutant IDs, and `pkg/invowkfile/invowkfile_validation_struct.go` has no accepted records remaining.

One hundred and forty-third remediation batch:

- Simplified dependency alternative failure guards in `internal/app/deps/checks.go`: validated custom checks, host capabilities, and container env/capability/command dependencies now branch on whether any alternative passed, while invalid container dependency objects are reported instead of silently skipped. Host env checks keep the existing invalid-regex reporting path and use the returned last error directly.
- Fresh exact reruns with stable IDs `1d7bf9e85b2c162331259b10e745ea57`, `463c0d925bf80dbf2f3ae517ef855aa9`, and `a79e46c84aae45e47d2f866ba2caebfc`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The boolean stable IDs initially resurfaced at same-ID capability guard sites after the custom-check guard was removed; the final cleanup removed every `!found && lastErr != nil` shape in `internal/app/deps/checks.go`.
- The root baseline now accepts 67 survivor rows covering 67 stable mutant IDs, and `internal/app/deps/checks.go` has no accepted records remaining.

One hundred and forty-fourth remediation batch:

- Simplified `internal/app/deps/input.go` equivalent flag and argument-validation guards: missing string-map flag values now rely on the zero-value lookup result, optional empty values are skipped by value alone, and argument value validation ranges over the shorter provided/defined slice instead of breaking after the definition list.
- Fresh exact reruns with stable IDs `5f701e1282f6c419ca97d847072ea644`, `968295f8a7df39a2b96100ee428a6eeb`, and `d041bc23805c6c6e2c65f2db91fd8576`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 64 survivor rows covering 64 stable mutant IDs, and `internal/app/deps/input.go` has no accepted records remaining.

One hundred and forty-fifth remediation batch:

- Added an unexported operation seam for `pkg/fspath/AtomicWriteFile()` and package-level fake temp-file tests for the chmod, write, and close failure branches, preserving the exported atomic-write API.
- Fresh exact reruns with stable IDs `2d46e98ac61114c7ddcbbfc373f57cca`, `558840d98feb2a5b0fb11a8edcd2b6ea`, and `5639b6974fd232084b1deb951cc981f0`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` killed the write and close guard mutants and reported `totalMutantsCount: 0` for the old chmod ID after the seam refactor.
- The root baseline now accepts 61 survivor rows covering 61 stable mutant IDs, and `pkg/fspath/atomic.go` has no accepted records remaining.

One hundred and forty-sixth remediation batch:

- Removed equivalent zero-value error-severity assignments from `pkg/invowkfile/validation_structure_args.go` and dropped the redundant empty-args guard; `SeverityError` is the `ValidationSeverity` zero value, and an empty args slice already returns nil after the loop.
- Fresh exact reruns with stable IDs `7a653c1029a70424052d2ca863cae892`, `ac9829e1d8ddaef9293d8ba8a7fd2e2f`, and `cc083158405e530d703e30784dd54a89`, `--output-statuses=ke`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the simplification.
- The root baseline now accepts 58 survivor rows covering 58 stable mutant IDs, and `pkg/invowkfile/validation_structure_args.go` has no accepted records remaining.

One hundred and forty-seventh remediation batch:

- Added focused vendored-module verification coverage for the directory/suffix scanner predicate, symlinked `.invowkmod` entries, and lock-entry identity fallback from namespace.
- Fresh exact reruns with stable IDs `6b5e6fd61314dbc1640786322e1e61f5`, `73722f5b0a9acc58b30909ceb5f09869`, and `f19f89fbf474913128ceb118b2983463`, `--coverage`, `--output-statuses=ken`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for each old ID after the scanner predicate refactor.
- A coverage-focused file sweep of `pkg/invowkmod/verify.go` with those accepted IDs removed reported 168 total mutants, 108 killed, 60 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 55 survivor rows covering 55 stable mutant IDs, and `pkg/invowkmod/verify.go` has no accepted records remaining.

One hundred and forty-eighth remediation batch:

- Removed equivalent zero-value branches from lock-file namespace/version validation and missing-lock inspection, preserving the same errors and absent-lock snapshot while eliminating indistinguishable mutation surfaces.
- Added lock-file parser and save-boundary coverage for unterminated quoted fallback values and `%w` wrapping of parent-directory creation failures.
- Fresh exact reruns with stable IDs `50fae5bf607ffbf501f0c5c2e8c33f8a`, `6019b20a6b739a4a8f60d7a1b0649c17`, and `7bbdbda4ba5d4ccd0e42319c999089a1`, `--coverage`, `--output-statuses=ken`, and `--test-flags='-short -count=1'` reported two obsolete IDs and one killed current site.
- A coverage-focused file sweep of `pkg/invowkmod/lockfile.go` with those accepted IDs removed reported 220 total mutants, 185 killed, 35 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 52 survivor rows covering 52 stable mutant IDs, and `pkg/invowkmod/lockfile.go` has no accepted records remaining.

One hundred and forty-ninth remediation batch:

- Removed equivalent declaration-level semver validation guards from `Invowkmod.Validate()` and `ModuleRequirement.Validate()`, preserved `ResolveScriptPath()` absolute pass-through while making relative fallthrough observable, replaced `ValidateScriptPath()` containment with normalized slash traversal checks, split symlink containment checks, and added parent-symlink plus `ContainsPath()` `filepath.Abs` failure coverage.
- Fresh exact reruns with stable IDs `3769fd5c1400c9e5c715ddca3c25fff7`, `4ab9c5e50904f654f821e2dc5cc0c0cf`, and `9ba9b26a1344ed06afa0e79296cbe719`, `--coverage`, `--output-statuses=ken`, and `--test-flags='-short -count=1'` reported two obsolete IDs and one killed current branch site.
- A coverage-focused file sweep of `pkg/invowkmod/invowkmod.go` with those accepted IDs removed reported 255 total mutants, 239 killed, 16 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 49 survivor rows covering 49 stable mutant IDs, and `pkg/invowkmod/invowkmod.go` has no accepted records remaining.

One hundred and fiftieth remediation batch:

- Removed redundant custom-check output validation from `CustomCheckResult.Validate()` because custom-check output is intentionally free-form process text, then added focused constructor and validator coverage for successful results and invalid exit-code rejection.
- Fresh exact reruns with stable IDs `2f9b4ef6367eb0d0c49dbaed0dbdfc08` and `65775e1bc21f1eb7eded282190f4ecc5`, `--coverage`, `--output-statuses=ken`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `internal/app/deps/host_probe.go` with those accepted IDs removed reported 11 total mutants, 11 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 47 survivor rows covering 47 stable mutant IDs, and `internal/app/deps/host_probe.go` has no accepted records remaining.

One hundred and fifty-first remediation batch:

- Removed redundant env-inherit validation from the private override applier after `BuildExecutionContextOptions.Validate()` had already validated those fields, projected variadic argument tails with `min(index, len(args))` to eliminate an equivalent boundary branch, and added public `BuildExecutionContext()` coverage for env-inherit overrides and empty variadic rest projection.
- Fresh exact reruns with stable IDs `19bd6854d61e76b5c35dd0c83dd19ef4` and `25a07748343b7e4c481f6c316ef08c86`, `--coverage`, `--output-statuses=ken`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `internal/app/execute/orchestrator.go` with those accepted IDs removed reported 144 total mutants, 134 killed, 10 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 45 survivor rows covering 45 stable mutant IDs, and `internal/app/execute/orchestrator.go` has no accepted records remaining.

One hundred and fifty-second remediation batch:

- Simplified `Resolve()` no-LLM/default selection by collapsing the empty configured-default switch arm into the `UseConfiguredDefault` branch and returning a zero `Resolved` for no-LLM requests, then strengthened no-LLM coverage to assert the minimal zero selection.
- Fresh exact reruns with stable IDs `5b2c16b38296b36b8beb25707db8900f` and `c7620c5fdcb1dbdb1d75d95a6ff0e343`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplifications.
- A coverage-focused file sweep of `internal/app/llmconfig/resolve.go` with those accepted IDs removed reported 201 total mutants, 193 killed, 8 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 43 survivor rows covering 43 stable mutant IDs, and `internal/app/llmconfig/resolve.go` has no accepted records remaining.

One hundred and fifty-third remediation batch:

- Removed the redundant `LLMBaseURL.Validate()` empty-scheme guard because the later HTTP/HTTPS scheme whitelist already rejects empty schemes while the host guard still rejects relative URLs.
- A fresh exact rerun with stable ID `35d6080ab9ac9e7c32bc1be7bb0efbf5`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` after the simplification.
- The companion exact rerun for `d07a0d7c32e673139cab6675010fbc32` still escaped: `VirtualConfig.Validate()` delegates to `VirtualUtilitiesConfig.Validate()`, which intentionally has no failing state while it contains only bool fields.
- A coverage-focused file sweep of `internal/config/types.go` with the obsolete URL ID removed reported 307 total mutants, 256 killed, 30 not covered, 21 escaped, and 92.42% covered-code MSI. The sweep included the one still-accepted config survivor plus 20 focused-only escaped rows that were not added during this shrink-only pass.
- The root baseline now accepts 42 survivor rows covering 42 stable mutant IDs, and `internal/config/types.go` has one accepted record remaining.

One hundred and fifty-fourth remediation batch:

- Simplified `serverbase.NewBase()` to rely on the atomic zero value for `StateCreated`, converted short `stateMu` critical sections from identical defer-unlock patterns to explicit unlocks, and added a liveness assertion proving `LastError()` releases the state lock before subsequent terminal-state transitions.
- Fresh exact reruns with stable IDs `beff1ced22204cb15e19e0f5a56f5295` and `bf90884fa793206366dc236b0a466af5`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplifications.
- A coverage-focused file sweep of `internal/core/serverbase/base.go` with those accepted IDs removed reported 70 total mutants, 63 killed, 7 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 40 survivor rows covering 40 stable mutant IDs, and `internal/core/serverbase/base.go` has no accepted records remaining.

One hundred and fifty-fifth remediation batch:

- Simplified `pkg/cueutil/error.go` path formatting by removing an equivalent empty-path guard and redundant inner-loop break, then tightened `formatPath()` coverage for digit `9` and punctuation below `0` path segments.
- Fresh exact reruns with stable IDs `0770024e240efd1b9173e2344365e36f` and `d7ad86d624243fdfcd0ba4ec9e383d8c`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `pkg/cueutil/error.go` with those accepted IDs removed reported 67 total mutants, 67 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 38 survivor rows covering 38 stable mutant IDs, and `pkg/cueutil/error.go` has no accepted records remaining.

One hundred and fifty-sixth remediation batch:

- Collapsed `ParseAndDecode()` validation to a single `unified.Validate(...)` call with optional `cue.Concrete(true)`, preserving validation-before-decode semantics while removing equivalent concrete/non-concrete branch structure. Added validation-only non-concrete coverage and wrapper assertions for internal CUE errors.
- Fresh exact reruns with stable IDs `37120c0b469acdb430997992983b3eb2` and `e9d96bf0afe1b2371b06c438c13275b6`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `pkg/cueutil/parse.go` with those accepted IDs removed reported 34 total mutants, 34 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 36 survivor rows covering 36 stable mutant IDs, and `pkg/cueutil/parse.go` has no accepted records remaining.

One hundred and fifty-seventh remediation batch:

- Removed equivalent empty-name `Value` fields from `ArgumentName.Validate()` and `FlagName.Validate()` because the offending value is the zero value on those branches, then simplified argument and flag default-value regex mismatch guards to rely on `RegexPattern.Validate()` plus `matchesValidation()` for empty and invalid patterns. Strengthened focused mutation tests for argument-name, flag-type, and flag-shorthand formatted diagnostics exposed by the file sweeps.
- Fresh exact reruns with stable IDs `4d65b62655a7f20c8453812d3dcde400`, `d17a6322394f83d1ba4767b483d3be30`, `9450c4dac3484afc7f9d91a5f718ec28`, and `d003fd5f99f2309827932373c4088433`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for all four old IDs after the simplifications.
- Coverage-focused file sweeps with those accepted IDs removed reported 79 total mutants, 79 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI for `pkg/invowkfile/argument.go`; and 84 total mutants, 84 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI for `pkg/invowkfile/flag.go`.
- The root baseline now accepts 32 survivor rows covering 32 stable mutant IDs, and `pkg/invowkfile/argument.go` plus `pkg/invowkfile/flag.go` have no accepted records remaining.

One hundred and fifty-eighth remediation batch:

- Removed redundant post-construction `CommandDependencyRefParts.Validate()` checks from command dependency parsing, moved custom-check script absoluteness to the shared `fspath.IsAbs()` helper, removed the redundant all-nil `MergeDependsOnAll()` branch, and omitted the unobservable zero `Value` field for empty command dependency source IDs. Added focused mutation coverage for custom-check script containment/read failures, interpreter resolution, and source-ID unwrap behavior.
- Fresh exact reruns with stable IDs `bfc475e48d22374af0880625d6c03079` and `c7e1e9f11225adb1e0124a00ec4fcffb`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported one obsolete old ID and one killed current source-ID guard after the simplifications.
- A coverage-focused file sweep of `pkg/invowkfile/dependency.go` with those accepted IDs removed reported 394 total mutants, 394 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 30 survivor rows covering 30 stable mutant IDs, and `pkg/invowkfile/dependency.go` has no accepted records remaining.

One hundred and fifty-ninth remediation batch:

- Replaced the raw `strconv.ParseFloat(..., 64)` bit-size literal with a named derived `parseFloat64BitSize` constant to remove equivalent 63/65 bit-size mutation surfaces, then made value-type validation branches explicit and simplified redundant float-empty and regex-empty handling. Added focused helper coverage for digit `9`, string values, unknown flag types, empty/invalid regex patterns, and regex mismatch diagnostics.
- Fresh exact reruns with stable IDs `848d2ea3a491d5968ab1a351c719817f` and `a90a66d0d6d3ce50fa1c4b2a6841eb13`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `pkg/invowkfile/validation_input.go` with those accepted IDs removed reported 53 total mutants, 53 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 28 survivor rows covering 28 stable mutant IDs, and `pkg/invowkfile/validation_input.go` has no accepted records remaining.

One hundred and sixtieth remediation batch:

- Removed redundant fallback validation of a valid `ModuleID` as a `ModuleSourceID` in `CommandScope.Validate()`. `ModuleID.Validate()` is stricter than `ModuleSourceID.Validate()` for the omitted-source case, while explicit source IDs still validate independently.
- Fresh exact reruns with stable IDs `522b85dbbfb2629949ea78a86c9b34e6` and `6e96678c6d3cef42811502b0225f310e`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported `totalMutantsCount: 0` for both old IDs after the simplification.
- A coverage-focused file sweep of `pkg/invowkmod/command_scope_validate.go` with those accepted IDs removed reported 17 total mutants, 17 killed, 0 not covered, 0 escaped, and 100.00% covered-code MSI.
- The root baseline now accepts 26 survivor rows covering 26 stable mutant IDs, and `pkg/invowkmod/command_scope_validate.go` has no accepted records remaining.

One hundred and sixty-first remediation batch:

- Removed equivalent capacity-only preallocation in `AddRequirement()` and `removeRequiresEntryLines()`, replaced the negative remove-index sentinel with pointer/presence tracking, and rewrote compact adjacent-entry preservation to use `strings.Cut()` plus exact whitespace tests.
- Fresh exact reruns with stable IDs `79fd3cd7a8c99993f92b050bd2846fbf` and `a74ad8cdf03740a2102e3b4d2f8f48a0`, `--run-mutant-id`, and `--test-flags='-short -count=1'` selected no current mutants after the simplification; the final report stats showed `totalMutantsCount: 0`.
- A focused file sweep of `pkg/invowkmod/invowkmod_edit.go` with those accepted IDs removed reported 171 total mutants, 171 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 24 survivor rows covering 24 stable mutant IDs, and `pkg/invowkmod/invowkmod_edit.go` has no accepted records remaining.

One hundred and sixty-second remediation batch:

- Removed equivalent empty-string guards from `ModuleShortName.Validate()` and `ModuleDirectoryName.Validate()` because both regexes already require an initial ASCII letter, then added direct error-string assertions for both invalid-name error types.
- Fresh exact reruns with stable IDs `9ccb58585bf8f0f9895e888613c2f463` and `d3c9db5250d5a59363abeadfc6320901`, `--run-mutant-id`, and `--test-flags='-short -count=1'` selected no current mutants after the simplification; the final report stats showed `totalMutantsCount: 0`.
- A focused file sweep of `pkg/invowkmod/module_short_name.go` with those accepted IDs removed reported 14 total mutants, 14 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 22 survivor rows covering 22 stable mutant IDs, and `pkg/invowkmod/module_short_name.go` has no accepted records remaining.

One hundred and sixty-third remediation batch:

- Removed redundant optional `ModuleNamespace` validation from `ResolvedModule.Validate()` and `AmbiguousMatch.Validate()` because `ModuleNamespace.Validate()` only rejects empty values and these optional fields already skip empty values. Added focused mutation coverage for rejectable `CommandSourceID` and `ContentHash` fields plus formatted resolved/ambiguous/remove-result error strings.
- Fresh exact reruns with stable IDs `937b4a210b38fbe7c4a6929349c005ea` and `b0b78208c95792b81e85030eee1ac4f7`, `--run-mutant-id`, and `--test-flags='-short -count=1'` selected no current mutants after the simplification; the final report stats showed `totalMutantsCount: 0`.
- A focused file sweep of `pkg/invowkmod/resolver_validate.go` with those accepted IDs removed reported 108 total mutants, 108 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 20 survivor rows covering 20 stable mutant IDs, and `pkg/invowkmod/resolver_validate.go` has no accepted records remaining.

One hundred and sixty-fourth remediation batch:

- Collapsed host filepath absolute-path handling onto the shared `fspath.IsAbs()` helper so Unix-style slash paths and host-native absolute paths share one cross-platform guard. Added focused filepath dependency assertions for container command names, host wrapper dependency errors, and nil host probes.
- A fresh exact rerun with stable ID `a3863647bd2462895996068bad36996e`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 1 total mutant, 1 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- A focused file sweep of `internal/app/deps/filepaths.go` with that accepted ID removed reported 58 total mutants, 58 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 19 survivor rows covering 19 stable mutant IDs, and `internal/app/deps/filepaths.go` has no accepted records remaining.

One hundred and sixty-fifth remediation batch:

- Removed the currently no-op root `VirtualConfig` delegation from `Config.Validate()` because `VirtualUtilitiesConfig` currently contains only bool fields and cannot fail validation; the CUE schema continues to enforce virtual config shape. Added compact config mutation contracts for include collection errors, LLM value payloads, LLM backend presence, nil unwrap sentinels, and aggregate error strings.
- A fresh exact rerun with stable ID `d07a0d7c32e673139cab6675010fbc32`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op delegation removal.
- A focused file sweep of `internal/config/types.go` with that accepted ID removed reported 304 total mutants, 299 killed, 0 not covered, 5 escaped, and 98.36% mutation score. The remaining focused-only escapes are the no-failing-state `VirtualConfig.Validate()` delegation and hard-coded-valid `DefaultConfig()` panic/error-reporting branches; they were not added during this shrink-only pass.
- The root baseline now accepts 18 survivor rows covering 18 stable mutant IDs, and `internal/config/types.go` has no accepted records remaining.

One hundred and sixty-sixth remediation batch:

- Removed the redundant nil-command guard in `internal/discovery.ValidateCommandTree()` because the delegated `pkg/invowkfile.ValidateCommandTree()` already skips nil `Command` entries. Added coverage proving nil `CommandInfo` entries do not stop validation of later real command conflicts.
- A fresh exact rerun with stable ID `59b11aee6a1f2842eefdfa9c08d52784`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the duplicate guard removal.
- A focused file sweep of `internal/discovery/validation.go` with that accepted ID removed reported 9 total mutants, 9 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 17 survivor rows covering 17 stable mutant IDs, and `internal/discovery/validation.go` has no accepted records remaining.

One hundred and sixty-seventh remediation batch:

- Removed no-op provisionenv validation paths: manifest marshal output uses `Value`, whose validation is intentionally nil, and manifest entry command namespaces are optional while non-empty `ModuleNamespace` has no failing state. Simplified legacy path-list parsing to rely on `MountTargetPath.Validate()` for blank segment rejection, then added focused coverage for provision env names, blank manifests, and blank path-list segments.
- A fresh exact rerun with stable ID `7a4b9619fccd4f81e86ac892d08eaca8`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op marshal-value validation removal.
- A focused file sweep of `internal/provisionenv/provisionenv.go` with that accepted ID removed reported 38 total mutants, 38 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 16 survivor rows covering 16 stable mutant IDs, and `internal/provisionenv/provisionenv.go` has no accepted records remaining.

One hundred and sixty-eighth remediation batch:

- Added focused command-tree coverage proving the tree-invariant check does not treat the empty root prefix as a synthetic parent command; invalid empty command names remain a field-validation concern for `CommandTreeEntry.Validate()`.
- A fresh exact rerun with stable ID `dc425ae28f9d698e749f92a3c86319e9`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 1 total mutant, 1 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 15 survivor rows covering 15 stable mutant IDs, and `pkg/invowkfile/command_tree.go` has no accepted records remaining.

One hundred and sixty-ninth remediation batch:

- Removed the redundant whitespace-only guard from `EnvVarName.Validate()` because the POSIX environment-name regex already rejects empty and whitespace-only values while preserving the same typed error payload.
- A fresh exact rerun with stable ID `0e97ba5b7502b28935da206b73cd8c66`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op guard removal.
- The root baseline now accepts 14 survivor rows covering 14 stable mutant IDs, and `pkg/invowkfile/env.go` has no accepted records remaining.

One hundred and seventieth remediation batch:

- Collapsed `Implementation.GetScriptFilePathWithModule()` absolute-script detection onto the shared `fspath.IsAbs()` helper, preserving Unix-style absolute paths on every platform and host-native absolute paths without a duplicate branch.
- A fresh exact rerun with stable ID `fc31794f23e14b695a9c603bce7545c8`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the duplicate absolute-path branch removal.
- The root baseline now accepts 13 survivor rows covering 13 stable mutant IDs, and `pkg/invowkfile/implementation.go` has no accepted records remaining.

One hundred and seventy-first remediation batch:

- Collapsed `Invowkfile.GetEffectiveWorkDir()` absolute-workdir detection onto the shared `fspath.IsAbs()` helper, preserving Unix-style absolute workdirs before native separator conversion while still accepting host-native absolute forms.
- A fresh exact rerun with stable ID `056133593d1af33fa7107e7d85442f8c`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the duplicate workdir absolute-path branch removal.
- The root baseline now accepts 12 survivor rows covering 12 stable mutant IDs, and `pkg/invowkfile/invowkfile.go` has no accepted records remaining.

One hundred and seventy-second remediation batch:

- Removed the explicit zero-valued `SeverityError` field from `runtimePreflightError()` because `SeverityError` is the default `ValidationSeverity` value; the existing helper test still asserts the produced diagnostic is an error.
- A fresh exact rerun with stable ID `5cb2477bc4828edd72aac7a634ad4d55`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the equivalent field assignment removal.
- The root baseline now accepts 11 survivor rows covering 11 stable mutant IDs, and `pkg/invowkfile/runtime_preflight.go` has no accepted records remaining.

One hundred and seventy-third remediation batch:

- Removed the redundant concrete-interpreter diagnostic guard from `AnalyzeScriptInterpreter()` because empty and `auto` interpreter specs resolve to the shebang itself, making any found effective interpreter equivalent to the shebang before diagnostics are considered.
- A fresh exact rerun with stable ID `f335fd4a9372bd4205bf984e59d484d8`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the redundant guard removal.
- The root baseline now accepts 10 survivor rows covering 10 stable mutant IDs, and `pkg/invowkfile/script_interpreter_diagnostics.go` has no accepted records remaining.

One hundred and seventy-fourth remediation batch:

- Removed the explicit zero-valued `SeverityError` field from the no-commands `StructureValidator` diagnostic because `SeverityError` is the default `ValidationSeverity` value.
- A fresh exact rerun with stable ID `481da2745399c6406408df76001187fe`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the equivalent field assignment removal.
- The root baseline now accepts 9 survivor rows covering 9 stable mutant IDs, and `pkg/invowkfile/validation_structure.go` has no accepted records remaining.

One hundred and seventy-fifth remediation batch:

- Reused the raw parent-directory segment scanner for env-file paths so authored middle `..` segments such as `config/../.env` cannot be hidden by path cleaning. Added direct env-file validation coverage for that middle traversal form.
- A fresh exact rerun with stable ID `009e94e222c7e9a514493a0a4e159e7e`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the traversal guard rewrite.
- The root baseline now accepts 8 survivor rows covering 8 stable mutant IDs, and `pkg/invowkfile/validation_filesystem.go` has no accepted records remaining.

One hundred and seventy-sixth remediation batch:

- Strengthened `GlobPattern.Validate()` tests to assert invalid-pattern payload values, then removed the explicit zero-valued `Value` assignment from the empty-pattern branch because an empty `GlobPattern` is already the field default.
- A fresh exact rerun with stable ID `bdd8f4b2624b5efb6a68bdeb43b76de5`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the equivalent field assignment removal.
- The root baseline now accepts 7 survivor rows covering 7 stable mutant IDs, and `pkg/invowkfile/watch.go` has no accepted records remaining.

One hundred and seventy-seventh remediation batch:

- Added a private filesystem-ops seam around content hashing so `hashFileContent()` can exercise the post-open `f.Stat()` failure path without changing the public API. Added a focused fake-file test proving fstat failures are wrapped with file context and do not stream bytes.
- A fresh exact rerun with stable ID `ca8fe76f39c86892a518bd367c46871a`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 1 total mutant, 1 killed, 0 not covered, 0 escaped, and 100.00% mutation score.
- The root baseline now accepts 6 survivor rows covering 6 stable mutant IDs, and `pkg/invowkmod/content_hash.go` has no accepted records remaining.

One hundred and seventy-eighth remediation batch:

- Removed the redundant explicit-empty guard from `GitURL.Validate()` because the allowed-prefix check already rejects empty values while preserving the invalid URL payload.
- A fresh exact rerun with stable ID `11441bdbc574304e3269e81e6eea1c2a`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op guard removal.
- The root baseline now accepts 5 survivor rows covering 5 stable mutant IDs, and `pkg/invowkmod/git_types.go` has no accepted records remaining.

One hundred and seventy-ninth remediation batch:

- Removed the redundant `ModuleScaffoldDirectoryName.Validate()` call from `CanonicalModuleDirectoryName()` because a valid non-suffixed `ModuleID` plus the fixed `.invowkmod` suffix cannot produce the only invalid scaffold name: empty.
- A fresh exact rerun with stable ID `1e84ecc8be35e5827a540d55c65fe839`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op validation removal.
- The root baseline now accepts 4 survivor rows covering 4 stable mutant IDs, and `pkg/invowkmod/operations.go` has no accepted records remaining.

One hundred and eightieth remediation batch:

- Removed the redundant `ModuleScaffold.Validate()` call from `NewModuleScaffold()` because validated create options plus static non-empty generated templates leave no failing state in the constructed scaffold.
- A fresh exact rerun with stable ID `1003e60196debaaf9c11ab8706fd075c`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op validation removal.
- The root baseline now accepts 3 survivor rows covering 3 stable mutant IDs, and `pkg/invowkmod/operations_create.go` has no accepted records remaining.

One hundred and eighty-first remediation batch:

- Removed the redundant `ConstraintOp.Validate()` call from `SemverResolver.ParseConstraint()` because the constraint regex only captures known operators, and the empty operator is normalized to `=`.
- A fresh exact rerun with stable ID `be17c90d51426e1dd3598cd3e3225a14`, `--run-mutant-id`, and `--test-flags='-short -count=1'` reported 0 total mutants after the no-op validation removal.
- The root baseline now accepts 2 survivor rows covering 2 stable mutant IDs, and `pkg/invowkmod/semver.go` has no accepted records remaining.

One hundred and eighty-second classification batch:

- Re-ran the remaining `pkg/platform/sandbox.go` stable ID `e78e96816010c22638d477a3277ba5fe`; it still escapes because the mutation replaces `return SandboxNone` with `return ""`, and `SandboxNone` is intentionally the empty-string zero value.
- Re-ran the remaining `pkg/fspath/fspath.go` stable ID `ed81a31addcdbefb529de74d4c9a8377`; it still escapes on the Linux mutation profile because the fallback `filepath.IsAbs(s)` only distinguishes host-native absolute forms after the cross-platform leading-slash guard.
- The root baseline remains at 2 survivor rows covering 2 stable mutant IDs.

## Policy

The committed baseline accepts the current escaped set so blocking mode can distinguish new escapes from known historical survivors. Future survivor reduction should follow this loop:

1. Pick a high-value survivor cluster from the agentic report.
2. Add the smallest behavior test that kills the survivor.
3. Rerun the affected stable mutant ID.
4. Remove killed IDs from the accepted baseline, or run a reviewed baseline update after a broader profile completes.
5. Keep not-covered clusters visible until package-level tests or a documented high-assurance profile cover them.
