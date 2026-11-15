# Quality Gates

Reference for what enforces each gate. The normative completion steps are in `.agents/rules/checklist.md`; CI-editing rules are in `.agents/rules/ci-workflows.md`; goplint gate internals are in `docs/goplint/README.md`.

## goplint baseline

- `.goplint/baseline.toml` uses baseline format v2 (`entries = [{id, message}]`); regenerate it only with `make update-baseline`.
- `check-baseline` and the other repository-audit modes scan production packages only: `./cmd/... ./internal/... ./pkg/...` (`GOPLINT_PACKAGES` in the `Makefile`).
- Protocol inconclusives are always visible and blocking; they can never be baselined, excepted or inline-ignored (`.agents/rules/checklist.md`).

## Pre-commit hooks (`.pre-commit-config.yaml`, installed with `make install-hooks`)

| Hook | Runs | Trigger | Notes |
|------|------|---------|-------|
| `golangci-lint` | `make lint` | every commit (`always_run`) | lint, formatter and config gates |
| `goplint-behavior` | `make check-goplint-consumer-routed` | every commit (`always_run`) | documentation-only diffs skip analysis; everything else, and any unmatched path, runs one shared audit with baseline, exception and full-scan verdicts |
| `windows-cross-build` | `make check-windows-build` | changes under `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum`, `scripts/check-windows-build.sh`, `Makefile`, `.pre-commit-config.yaml` | Windows cross-compile |
| `pgo-staleness` | inline script | non-test hot-path files staged without `default.pgo` | advisory only (exit 0); see `pgo-and-benchmarks.md` |
| `sonar-local` | `make sonar-local` | every commit (`always_run`) | blocks on quality-gate failure |

Not covered by any hook (manual discipline): `make test`, `make tidy`, `make check-file-length`, `make check-agent-docs`, `make test-scripts`, documentation, diagram and sample-module validation, the code-review/simplify pass, and `/learn`.

## Script tests (`make test-scripts`)

- Single list: `scripts/script-tests.sh` (`SCRIPT_TESTS`). Each run first checks that every `scripts/test_*` file is listed there or, with a reason, in `SCRIPT_TESTS_ELSEWHERE` (only `test_install.ps1`, which runs on Windows CI); `--check-list` runs only that check. CI (`ci.yml` and `release.yml`, Linux legs) runs `make test-scripts`; do not copy individual test commands into workflows.
- Worktree-safety gate: `scripts/check-script-tests-worktree-safety.sh` runs the list in a throwaway `git worktree add --detach` checkout at HEAD. It mirrors the checkout's staged, unstaged and untracked work and plants canaries: an unstaged edit under each of `cmd/`, `internal/` and `pkg/`, one staged edit, and one untracked file. Afterwards it compares the canary bytes, the staged and unstaged binary diffs and the untracked set (with content hashes). It also compares the real checkout before and after, including HEAD and the stash list.
- Exit status: 0 pass, 1 suite failed, 2 setup error, 3 a checkout changed. On 3 for the real checkout it keeps the before-state (diffs plus a tar of untracked files) and prints recovery commands; it never restores anything. The throwaway checkout is always removed with `git worktree remove --force`, never `git worktree prune`.
- Self-test: `scripts/test_check_script_tests_worktree_safety.sh`, in the list, injects destructive commands into the gate over a fixture repository and asserts exit 3. One of them is the old `git restore -- cmd internal pkg`.
- `make test-scripts-unguarded` runs the list in place, without the gate. Use it only for quick iteration.

## sonar-local

- API-only: `scripts/sonar-local.sh` reads the quality gate and unresolved issues from SonarCloud; it does not scan locally. CI analysis is SonarCloud Automatic Analysis (GitHub App).
- Requires `curl` and `jq`. Reports go to `.sonar/reports/` (`quality-gate.json`, `issues.json`).
- Environment: `SONAR_TOKEN` (optional; public projects work without it), `SONAR_HOST_URL` (default `https://sonarcloud.io`), `SONAR_PROJECT_KEY` (default `invowk_invowk`), `SONAR_BRANCH`, `SONAR_PAGE_SIZE` (default 500).
- False positives: resolve the issue in the SonarCloud UI (False Positive or Accepted, with a comment naming the entry), and record it in BOTH `sonar-project.properties` and `.sonarcloud.properties`, keeping multicriteria IDs gapless. Automatic Analysis honours only scope settings in `.sonarcloud.properties` (sources, tests, exclusions), not `sonar.issue.ignore.multicriteria`. Verified 2026-11-14: e13 had no effect after re-analysis, and the go:S107, go:S1313 and go:S4507 issues covered by earlier entries were resolved in the UI. The file entries are the reviewable record and apply to any local or CI scanner run.
- `make sonar-local` on an unpushed branch finds no analysis and reports 0 issues; use `SONAR_BRANCH=main` or the PR's SonarCloud check for a real signal.

## CI workflows (`.github/workflows/`)

| Workflow | Triggers | Purpose |
|----------|----------|---------|
| `ci.yml` | push/PR to main (path-filtered), release, dispatch | tests (Linux full; macOS/Windows short unit + CLI), build verification, license check, all-module govulncheck, `pgo-sanity` |
| `lint.yml` | push to main (path-filtered), every PR (no path filter), release, weekly schedule, dispatch | **required** (`goplint consumer gates` check): golangci-lint, formatter/config checks, agent-docs integrity, goplint consumer gates |
| `release.yml` | tag push `v*`, dispatch | validate, test, build and publish a release (procedure: publish-release skill) |
| `formal-verification.yml` | weekly schedule, dispatch, every PR (classified by `scripts/formal.py affected` against `[ci] paths`) | Alloy/TLA+ models, golden freshness, correspondence, trace validation, replay plan; required status check since 2026-11-14 (`make formal-promotion-gate` passed) |
| `mutation-testing.yml` | dispatch only | curated mutation profiles (`target_set`: `root` or `formal-bindings`); see `mutation-testing.md` |
| `benchmarks.yml` | weekly schedule, dispatch, PRs labelled `benchmarks: run` | benchmark runs for Bencher (bencher skill) |
| `benchmarks-upload.yml` | `workflow_run` of Benchmarks | uploads fork-PR benchmark results (labelled PRs only) |
| `pgo-benchstat.yml` | weekly schedule, dispatch | compare `pgo=off` vs `pgo=on` with `benchstat` |
| `test-website.yml` | PR (website/diagram/script changes), release, dispatch | validate version assets and build the website |
| `validate-diagrams.yml` | PR, push, release | D2 syntax checks |
| `deploy-website.yml` | push, dispatch | GitHub Pages deployment |
| `version-docs.yml` | release published, dispatch | docs versioning |
| `codeql.yml` | push, PR, schedule | advanced-setup CodeQL for actions, go, javascript-typescript, python (`go-version-file: go.mod`) |
| `claude.yml` | issue/PR comments and reviews | Claude Code GitHub integration |
