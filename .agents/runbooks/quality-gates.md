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

- Single list: `scripts/script-tests.sh` (`SCRIPT_TESTS`). Each run first checks that every script test file in the repository (tracked or untracked `test_*.{sh,bash,py,mjs,js,ps1}` and `*_test.{sh,bash,py,mjs,js}`, outside `tests/cli/` and `testdata/`) is listed there or, with a reason, in `SCRIPT_TESTS_ELSEWHERE` (only `test_install.ps1`, which runs on Windows CI); `--check-list` runs only that check. CI runs `make test-scripts` in the `script tests (worktree-safety gate)` job of `lint.yml` (every PR, no path filter; a ruleset-required check, so keep the name stable) and in the Linux test legs of `release.yml`, each first exporting `INVOWK_TRIAGE_BASE` from `scripts/ci-triage-base.sh` (the `triage --check` growth base; a depth-1 checkout has no `origin/main`); do not copy its test commands into workflows. `ci.yml` runs only the PowerShell install tests on Windows. Some tests also run, unguarded, in the lanes of the tool they cover: `lint.yml` (`test_golangci_lint.sh`, and `test_review_docs.py` through `make check-agent-docs`) and `formal-verification.yml` (`test_formal.py`, `test_formal_promotion_gate.py`).
- Worktree-safety gate: `scripts/check-script-tests-worktree-safety.sh` runs the list in a throwaway repository that is not linked to the real one: `git init`, an alternates file that reads the real object store, a copy of the refs (not the stash) and no remote, with HEAD checked out detached. Its config, refs, stash and worktree list are private, so a suite that runs `git config`, `git stash clear`, `git branch -D` or `git worktree remove` cannot reach the real repository or its sibling worktrees. The gate mirrors the checkout's staged, unstaged and untracked work and plants canaries: an unstaged edit to a tracked text file and an untracked file in every top-level tracked directory and at the root, one staged edit, one stash entry and one branch. Afterwards it compares the canary bytes, the staged and unstaged binary diffs, the untracked set (with content hashes), and the throwaway's config file, refs and stash list. It also compares the real checkout before and after: HEAD, the branch, both diffs, the untracked set and the safety-relevant local config keys (`core.hooksPath`, signing, `user.*`, includes). It does not compare the real stash, refs, worktree list or other config keys, because sibling worktrees change them concurrently.
- Canary placement: a canary edit never goes into a file a suite reads, parses or hashes, or the gate would change the run it guards. Candidates (tracked text files, Markdown first, Go last) matching `CANARY_EXCLUDE` (the suites' scripts and fixtures, `tools/`, `formal/manifest.toml` and the models, workflows, testdata, Go tests, MDX, dependency manifests, `.gitignore` files) are skipped, and so is any path that a file under `scripts/`, `tools/`, `formal/`, `.github/workflows/` or `.agents/skills/*/scripts/` names (for a Go file, also its package in a `tools/mutation/` list). Directories with no candidate left get only the untracked canary, with a warning. The rule comes from an atime trace of each list entry in the throwaway (tmpfs keeps atime). The one exception is `review_docs.py`, which hashes every non-ignored file but compares two hashes from the same run. `--list-canaries` prints the choice; the self-test checks the rule. Re-derive it when a suite starts reading new files.
- Not mirrored: ignored files (`bin/`, `.cache/`, `node_modules/`, downloaded formal jars). No list entry needs them: guarded and in-place runs give the same per-suite pass counts and skip nothing. A new test that needs an ignored file must create it or receive it explicitly, never through a symlink into the real checkout. Remote-tracking refs (`origin/main`) are copied with the other refs, but a depth-1 CI checkout has none to copy.
- Needs bash 4.4 or newer on `PATH` (macOS: `brew install bash`); the gate and the list fail with a clear error on bash 3.2.
- Exit status: 0 pass, 1 suite failed, 2 setup error, 3 a checkout changed or could no longer be read. The real checkout is compared even when the suite broke the throwaway repository, and from the exit trap after an interrupt. On 3 for the real checkout it keeps the before-state (diffs, guarded config and a tar of untracked files) and prints recovery commands; it never restores anything. The throwaway repository is always deleted; it is never registered as a worktree, so nothing needs pruning.
- Self-test: `scripts/test_check_script_tests_worktree_safety.sh`, in the list, injects destructive commands into the gate over a fixture repository (with a tag, a branch, a stash entry and a sibling worktree) and asserts exit 3. One of them is the old `git restore -- cmd internal pkg`.
- `make test-scripts-unguarded` (`--in-place`) runs the list in this checkout with only the before/after fingerprint: damage is detected and recoverable, not prevented, and changes to the shared stash, refs or worktree list go unnoticed. Use it only for quick iteration.
- The throwaway repository mirrors uncommitted work, so you need not commit before `make test-scripts`. Commit anyway before `test-scripts-unguarded` or before running a single script test directly.

## sonar-local

- API-only: `scripts/sonar-local.sh` reads the quality gate and unresolved issues from SonarCloud; it does not scan locally. CI analysis is SonarCloud Automatic Analysis (GitHub App).
- Requires `curl` and `jq`. Reports go to `.sonar/reports/` (`quality-gate.json`, `issues.json`).
- Environment: `SONAR_TOKEN` (optional; public projects work without it), `SONAR_HOST_URL` (default `https://sonarcloud.io`), `SONAR_PROJECT_KEY` (default `invowk_invowk`), `SONAR_BRANCH`, `SONAR_PAGE_SIZE` (default 500).
- False positives: the maintainer resolves the issue in the SonarCloud UI (False Positive or Accepted, with a comment naming the entry); an agent lists it for the maintainer instead (`.agents/rules/checklist.md` step 7). Record it in BOTH `sonar-project.properties` and `.sonarcloud.properties`, keeping multicriteria IDs gapless. Automatic Analysis honours only scope settings in `.sonarcloud.properties` (sources, tests, exclusions), not `sonar.issue.ignore.multicriteria`. Verified 2026-11-14: e13 had no effect after re-analysis, and the go:S107, go:S1313 and go:S4507 issues covered by earlier entries were resolved in the UI. The file entries are the reviewable record and apply to any local or CI scanner run.
- `make sonar-local` on an unpushed branch finds no analysis and reports 0 issues; use `SONAR_BRANCH=main` or the PR's SonarCloud check for a real signal.

## CI workflows (`.github/workflows/`)

Rulesets: "Safety" on `main` (pull request required with 0 approvals, signatures, linear history, the `goplint consumer gates`, `Models, golden vectors, and correspondence` and `script tests (worktree-safety gate)` checks; bypass: the `invowk-docs-bot` App) and "Release tags" (`refs/tags/v*` creation, update and deletion; bypass: the admin role). Details: `.agents/rules/git.md`, `.agents/runbooks/git-merge.md`.

| Workflow | Triggers | Purpose |
|----------|----------|---------|
| `ci.yml` | push/PR to main (path-filtered), release, dispatch | tests (Linux full; macOS/Windows short unit + CLI; PowerShell install-script tests on Windows), build verification, license check, all-module govulncheck, `pgo-sanity`, unsigned GoReleaser dry run (`Release Dry Run`) |
| `lint.yml` | push to main (path-filtered), every PR (no path filter), release, weekly schedule, dispatch | **required** (`goplint consumer gates` check): golangci-lint (including Darwin build-tag check via `make lint-darwin`), formatter/config checks, agent-docs integrity, goplint consumer gates, `script tests (worktree-safety gate)` (`make test-scripts`) |
| `release-signing.yml` | push to main (ci.yml's paths), release, dispatch; no pull request trigger | `Release Dry Run (signed)`: GoReleaser snapshot with Cosign keyless signing (`id-token: write`); `ci.yml`'s `release-dry-run` (`Release Dry Run`, required for gomod Dependabot PRs) runs the same snapshot tokenless with `--skip=sign` |
| `release.yml` | tag push `v*`, dispatch | test, build and publish a release on a tag push; a dispatch is a dry run only and never pushes a tag (procedure: publish-release skill) |
| `formal-verification.yml` | weekly schedule, dispatch, every PR (classified by `scripts/formal.py affected` against `[ci] paths`) | Alloy/TLA+ models, golden freshness, correspondence, trace validation, replay plan; required status check since 2026-11-14 (`make formal-promotion-gate` passed) |
| `mutation-testing.yml` | dispatch only | curated mutation profiles (`target_set`: `root` or `formal-bindings`, with optional `symbol`/`stale`/`jobs` scoping for `formal-bindings`); see `mutation-testing.md` |
| `benchmarks.yml` | weekly schedule, dispatch, PRs labelled `benchmarks: run` | benchmark runs for Bencher (bencher skill) |
| `benchmarks-upload.yml` | `workflow_run` of Benchmarks | uploads fork-PR benchmark results (labelled PRs only) |
| `pgo-benchstat.yml` | weekly schedule, dispatch | compare `pgo=off` vs `pgo=on` with `benchstat` |
| `test-website.yml` | PR (website/diagram/script changes), release, dispatch | validate version assets and build the website; this build, `deploy-website.yml`'s and `version-docs.yml`'s run through `scripts/ci-website-build.sh` (memory bounds for Rspack, a watchdog under the job timeout, memory samples and low-memory annotations) |
| `validate-diagrams.yml` | PR, push, release | D2 syntax checks |
| `deploy-website.yml` | push, dispatch | GitHub Pages deployment |
| `version-docs.yml` | release published, dispatch | docs versioning |
| `codeql.yml` | push, PR, schedule | advanced-setup CodeQL for actions, go, javascript-typescript, python (`go-version-file: go.mod`) |
| `claude.yml` | issue/PR comments and reviews | Claude Code GitHub integration |
| `dependabot-gate.yml` | `workflow_run` of CI, Lint, Formal Verification, CodeQL and Test Website Build | decides whether a Dependabot gomod or npm PR may merge (`scripts/dependabot-gate.py`); in `merge` mode it writes the verdict to the job summary and squash-merges qualifying pull requests via `invowk-dependabot-merge` (`decide-only` writes the summary only). Each required check must be a successful job, with a runner and no failed step, of the latest attempt of Dependabot's own run (a fork's runs are skipped); the summary also lists the `go.mod` lines admitted by the two relaxations, `indirect-side-effect` and `allow-0x-minor` (rule: `ci-workflows.md`, "Dependabot merge gate") |
