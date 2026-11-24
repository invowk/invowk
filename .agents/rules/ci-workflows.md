---
paths:
  - ".github/**"
  - "scripts/**"
  - "Makefile"
  - "tools/**"
  - ".pre-commit-config.yaml"
---

# CI Workflows and Gate Tooling

- Step-level `env:` applies to that step only. Promote a variable used by several steps to job-level `env:` (or repeat it); otherwise a `set -u` report step can fail after the real build or test step succeeded.
- Write generated reports under ignored artifact directories unless they are intentional release or docs assets. Before committing a workflow or tool change that moves reports, check with `git ls-files` that root-level outputs (`report.json`, `go-mutesting-summary.json`, `go-mutesting-agentic.json`, `go-mutesting-gitlab.json`, `go-mutesting-report.html`) are not tracked.
- `lint.yml` is required: it carries the ruleset-required `goplint consumer gates` check, and its `pull_request` trigger deliberately has no path filter. Keep it that way. Its `script tests (worktree-safety gate)` job runs `make test-scripts` on every PR and is meant to become a required check: keep the job name stable, and do not add `make test-scripts` steps to `ci.yml`. Every CI `make test-scripts` step first exports `INVOWK_TRIAGE_BASE` from `scripts/ci-triage-base.sh` in the same step (a depth-1 checkout has no `origin/main`).
- `release.yml`'s `test` job mirrors `ci.yml`'s: change the matrix, the test, engine-setup and matrix-gated steps (their `if`, `INVOWK_TEST_*` env at any level, and `go test` packages and flags) in both. `scripts/check-ci-release-parity.py` (run by `make test-scripts`) fails on drift; record an intentional difference in its exception tables with the reason.
- `formal-verification.yml` carries the ruleset-required `Models, golden vectors, and correspondence` check (required since 2026-11-14, pinned to the GitHub Actions app). It must report on every PR, including docs-only ones (it skips its heavy steps via `scripts/formal.py affected`); never add a path filter that stops it reporting.
- `mutation-testing.yml` is manual dispatch only; never make it a PR or scheduled gate.
- CodeQL stays on the advanced setup (`codeql.yml`, with `go-version-file: go.mod` for Go). Never enable GitHub's default CodeQL setup.
- Pins: `.agents/rules/version-pinning.md`. Workflow inventory and pre-commit hooks: `.agents/runbooks/quality-gates.md`.

## Dependabot merge gate

- `dependabot-gate.yml` runs on `workflow_run` of the workflows whose checks it requires, so it always executes main's copy of itself and of `scripts/dependabot-gate.py`. Never give it a `pull_request_target` trigger, and never check out or run pull request code in it.
- The script decides `merge` or `hold` from read-only API calls and fails closed. `merge` needs all of: a pull request by `dependabot[bot]` on `main` from this repository, open, not a draft and without conflict; one verified Dependabot commit that is the head the triggering run tested; gomod or npm files only (GitHub Actions updates never merge); semver-patch or semver-minor updates only, and no minor bump to a 0.x version; nothing on the sync-pair deny list; no `hold` or `do-not-merge` label; every check on the head commit green, except `Track Benchmarks`; and every required check of the ecosystem present and successful.
- `GATE_MODE` is a constant in the workflow file, now `decide-only`: the job writes the verdict and its reasons to the job summary and stops. The move to `merge` is a reviewed PR that changes that line; never read the mode from a repository variable.
- Only in `merge` mode and after a `merge` verdict does the job mint the `invowk-dependabot-merge` App token (`vars.DEPENDABOT_MERGE_APP_ID`, `secrets.DEPENDABOT_MERGE_APP_PRIVATE_KEY`) and squash-merge with `--match-head-commit`. The App has no ruleset bypass, so the `main` ruleset still applies to its merges. Keep `GITHUB_TOKEN` read-only in this workflow, and add no comments, labels or approvals.
- Keep the tables in step. A job renamed in `ci.yml`, `lint.yml`, `formal-verification.yml`, `codeql.yml` or `test-website.yml` needs the same change in `REQUIRED_*`. A new sync pair goes into `DENY_GOMOD` or `DENY_NPM` and into the `exclude-patterns` of `.github/dependabot.yml`. `scripts/test_dependabot_gate.py` (run by `make test-scripts`) fails on drift.
