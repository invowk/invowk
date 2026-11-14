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
- `lint.yml` is required: it carries the ruleset-required `goplint consumer gates` check, and its `pull_request` trigger deliberately has no path filter. Keep it that way.
- `formal-verification.yml` stays optional until `make formal-promotion-gate` reports PASS (four consecutive first-attempt green weekly runs). Making it required is a manual maintainer step.
- `mutation-testing.yml` is manual dispatch only; never make it a PR or scheduled gate.
- CodeQL stays on the advanced setup (`codeql.yml`, with `go-version-file: go.mod` for Go). Never enable GitHub's default CodeQL setup.
- Pins: `.agents/rules/version-pinning.md`. Workflow inventory and pre-commit hooks: `.agents/runbooks/quality-gates.md`.
