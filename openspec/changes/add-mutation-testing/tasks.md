## 1. Tool Pinning and Baseline Decisions

- [x] 1.1 Select the initial `go-mutesting` version, verify current upstream release state, and record the selected exact version in the implementation notes.
- [x] 1.2 Add the mutation-testing tool as a pinned Go tool dependency or exact-version CI install, avoiding `@latest` and floating tags.
- [x] 1.3 Add a tool-version verification path that fails before mutation execution when the resolved binary does not match the expected version.
- [x] 1.4 Update `.agents/rules/version-pinning.md` with the mutation-testing tool version and maintenance rules.

## 2. Mutation Configuration and Target Manifests

- [x] 2.1 Create root-module mutation configuration for eligible `cmd/`, `internal/`, and `pkg/` production Go packages.
- [x] 2.2 Create `tools/goplint` mutation configuration that runs from the nested module root and writes separate reports.
- [x] 2.3 Add target manifest logic that excludes tests, testdata, docs, website, samples, OpenSpec files, generated artifacts, and unsupported helper-only surfaces.
- [x] 2.4 Make packages without local Go test ownership explicit through exclusion rationale or not-covered reporting.
- [x] 2.5 Define stable report and baseline file locations for root-module and `tools/goplint` profiles.

## 3. Local Wrapper Commands

- [x] 3.1 Add a repository mutation wrapper script with `pr`, `full`, `baseline-update`, `dry-run`, and `rerun` profiles.
- [x] 3.2 Implement dirty-worktree protection or isolated temporary-worktree execution for local profiles that mutate source files.
- [x] 3.3 Implement changed-line PR mode with configurable base ref and successful no-mutation handling.
- [x] 3.4 Implement baseline-aware blocking and advisory modes for escaped mutants.
- [x] 3.5 Implement single-mutant rerun support using stable mutant IDs and selected module profile.
- [x] 3.6 Ensure default mutation profiles do not pass `-race`, run container tests, or run CLI `testscript` suites unless explicitly configured.

## 4. Make Targets and Documentation

- [x] 4.1 Add Make targets for mutation dry-run, PR/advisory run, full run, baseline update, and single-mutant rerun.
- [x] 4.2 Document the new targets, prerequisites, reports, baselines, and advisory/blocking behavior in `.agents/rules/commands.md`.
- [x] 4.3 Add developer-facing mutation-testing guidance if an existing maintainer documentation surface covers quality gates or test workflow.
- [x] 4.4 Update `AGENTS.md` or agent indexes only if new rules, skills, or command surfaces are added.

## 5. CI Integration

- [x] 5.1 Add a GitHub Actions mutation-testing workflow or job set with minimal permissions.
- [x] 5.2 Configure pull-request mutation runs to fetch enough history for changed-line filtering.
- [x] 5.3 Configure PR mutation mode to emit GitHub annotations and upload root-module and `tools/goplint` report artifacts.
- [x] 5.4 Configure scheduled or manual full mutation scans for broad root-module and `tools/goplint` profiles.
- [x] 5.5 Keep initial CI behavior advisory unless the baseline and timing data are stable enough to enable blocking mode.

## 6. Baseline and Report Artifacts

- [x] 6.1 Run mutation dry-runs for root-module and `tools/goplint` profiles and tune exclusions, worker counts, and timeouts.
- [x] 6.2 Generate initial accepted-survivor baselines for the selected profiles.
- [x] 6.3 Verify summary JSON and escaped-mutant reports are produced with distinguishable profile names.
- [x] 6.4 Confirm baseline updates remove killed historical survivors when tests improve.

## 7. Tests and Validation

- [x] 7.1 Add script or unit tests for target resolution, report path calculation, command construction, and dirty-worktree protection without running a full mutation scan.
- [x] 7.2 Run focused tests for the new wrapper and Make target plumbing.
- [x] 7.3 Run `make check-agent-docs` if `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed.
- [x] 7.4 Run `make lint` or focused lint checks for changed scripts, workflows, and Go tool dependency changes.
- [x] 7.5 Run `make test-short` or a narrower documented test set appropriate to the implementation scope.
- [x] 7.6 Run `openspec validate add-mutation-testing --strict`.
- [x] 7.7 Run `openspec validate --changes --strict` after all artifacts and implementation edits are complete.
