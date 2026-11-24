# Runbooks

On-demand procedures. Runbooks carry the authority of a skill: a rule wins on conflict. They are not auto-loaded; open the one you need. Every `.md` file here must be listed below (`make check-agent-docs` verifies it).

- [`change-flow.md`]: the per-change Workflow template, its modes, stage order, formal lane and probe facts.
- [`git-merge.md`]: squash-merge recipe (`gh pr merge --squash --match-head-commit`), branch cleanup and the release-tag ruleset.
- [`mutation-testing.md`]: mutation profiles, baselines, reports and the formal-bindings target set.
- [`pgo-and-benchmarks.md`]: `default.pgo` regeneration, staleness automation and BMF benchmark data.
- [`quality-gates.md`]: goplint baseline format and scope, pre-commit hooks, sonar-local and the CI workflow inventory.
- [`repo-map.md`]: package layout and code-area to skills routing.
