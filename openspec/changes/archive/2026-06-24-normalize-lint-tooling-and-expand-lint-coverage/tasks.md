## 1. Version Pinning and Wrapper

- [x] 1.1 Re-check the current upstream golangci-lint latest release and record the exact selected version in the implementation notes or updated version-pinning documentation.
- [x] 1.2 Add golangci-lint as an exact root `go.mod` Go tool dependency using the selected version.
- [x] 1.3 Update `go.sum` and run the required Go module tidy verification after adding the tool dependency.
- [x] 1.4 Add a repository wrapper script that resolves golangci-lint with `go tool -n`, verifies the resolved module version, and fails before linting on mismatch.
- [x] 1.5 Add wrapper subcommands or flags for root run, `tools/goplint` run, root formatter check, `tools/goplint` formatter check, root config verification, `tools/goplint` config verification, and effective-linter inspection.
- [x] 1.6 Add focused script tests or shell-level validation for wrapper argument handling, module-root selection, version mismatch failure, and missing-tool failure.

## 2. Makefile, CI, and Pre-commit Parity

- [x] 2.1 Update `Makefile` lint targets so root and `tools/goplint` golangci-lint runs use the normalized wrapper.
- [x] 2.2 Add Make targets for root formatter check, `tools/goplint` formatter check, combined formatter check, root config verification, `tools/goplint` config verification, combined config verification, and effective-linter inspection.
- [x] 2.3 Ensure `make lint` runs the complete required lint gate for both modules and includes formatter checks or invokes a documented companion target required by the gate.
- [x] 2.4 Update GitHub Actions lint workflow to run the normalized Make targets for the root module and `tools/goplint` module.
- [x] 2.5 Remove or rewrite workflow comments, job names, and assumptions that previously implied root-only linting matched full local linting.
- [x] 2.6 Replace or update pre-commit golangci-lint hooks so staged root-module and `tools/goplint` changes use the normalized repository lint path.
- [x] 2.7 Ensure pre-commit has a supported path that runs the complete root plus `tools/goplint` lint and formatter gate.

## 3. Deterministic Golangci-lint Configs

- [x] 3.1 Capture the current effective root enabled-linter list before changing `.golangci.toml`.
- [x] 3.2 Convert root `.golangci.toml` to an explicit blocking linter set without dropping previously effective standard linters.
- [x] 3.3 Verify `tools/goplint/.golangci.toml` remains explicit and document any intentional root-versus-tools linter differences in config comments.
- [x] 3.4 Add config verification commands for both modules to the regular lint validation path.
- [x] 3.5 Remove or revise comments that misdescribe linter scope, linter responsibility, CI coverage, or formatter enforcement.
- [x] 3.6 Scope root `errcheck` exclusions so test-only and fixture-only rationales are path-scoped or removed from global exclusions.
- [x] 3.7 Configure `nolintlint` or a companion validation step so nolint directives are specific, non-stale, and rationale-backed.

## 4. High-signal Linter Expansion and Cleanup

- [x] 4.1 Enable exported-documentation linting for the root module and `tools/goplint`.
- [x] 4.2 Fix or locally justify all exported-documentation findings.
- [x] 4.3 Enable integer range modernization linting for the root module and `tools/goplint`.
- [x] 4.4 Convert affected loops to modern integer range forms or locally justify any older forms kept for clarity or semantics.
- [x] 4.5 Enable missing-`t.Parallel()` enforcement for eligible root-module and `tools/goplint` tests.
- [x] 4.6 Add `t.Parallel()` to eligible tests and subtests while preserving platform, filesystem, environment, process-global, and integration-test safety.
- [x] 4.7 Add local rationale-backed exclusions only for tests that cannot safely run in parallel.
- [x] 4.8 Enable actionable context propagation linting for production Go code.
- [x] 4.9 Fix real context propagation findings and locally justify intentional fresh-context or fallback-context cases.
- [x] 4.10 Enable unchecked type assertion reporting for production Go code.
- [x] 4.11 Convert unchecked type assertions to checked assertions or add local invariant-specific suppressions where framework or analyzer contracts make the assertion safe.
- [x] 4.12 Re-run the expanded lint set for both modules and repeat cleanup until no unresolved findings remain.

## 5. Goplint Gate and Exception Governance

- [x] 5.1 Audit current `tools/goplint` baseline behavior and update stale wording that implies nonzero accepted baseline findings when the current baseline is empty.
- [x] 5.2 Audit `tools/goplint/exceptions.toml` for malformed, stale, overdue, unsupported, broad, or weakly explained exceptions.
- [x] 5.3 Add review dates or an equivalent review mechanism for long-lived or broad goplint exceptions.
- [x] 5.4 Add or update Make targets that run goplint baseline checks and exception governance checks.
- [x] 5.5 Wire goplint exception governance into local and CI quality gates.
- [x] 5.6 Update any goplint docs or agent guidance that confuses baseline-suppressed categories with always-visible hard blockers.

## 6. Documentation and Agent Guidance

- [x] 6.1 Update `.agents/rules/version-pinning.md` so golangci-lint is documented under the correct normalized version source.
- [x] 6.2 Update `.agents/rules/commands.md` and Make help so contributors can find root lint, `tools/goplint` lint, formatter checks, config verification, and goplint exception checks.
- [x] 6.3 Update `.agents/rules/testing.md` so it accurately distinguishes missing-`t.Parallel()` enforcement from `tparallel` placement/subtest enforcement.
- [x] 6.4 Update `AGENTS.md` indexes or command summaries if any `.agents/rules/`, `.agents/skills/`, or command documentation changes require sync.
- [x] 6.5 Update any related README, tool README, or developer documentation that describes lint, format, version pinning, goplint baseline behavior, or exception auditing.

## 7. Final Verification

- [x] 7.1 Run golangci-lint config verification for the root module and `tools/goplint`.
- [x] 7.2 Run effective-linter inspection for the root module and `tools/goplint` and confirm the intended linter sets are enabled.
- [x] 7.3 Run formatter checks for the root module and `tools/goplint`.
- [x] 7.4 Run the full normalized lint gate for the root module and `tools/goplint`.
- [x] 7.5 Run goplint baseline and exception governance checks.
- [x] 7.6 Run targeted Go tests for every package whose source or tests changed during lint cleanup.
- [x] 7.7 Run `make check-agent-docs` if `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed.
- [x] 7.8 Run `openspec validate normalize-lint-tooling-and-expand-lint-coverage --strict`.
- [x] 7.9 Run `git diff --check`.
- [x] 7.10 Record any validation command that could not be run and the concrete reason.

All planned validation commands ran successfully; none were skipped.
