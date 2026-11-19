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
