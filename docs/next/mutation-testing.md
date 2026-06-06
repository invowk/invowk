# Mutation Testing

Invowk mutation testing is an advisory quality gate at rollout. It measures whether package-level Go tests can detect source-level changes in production packages without adding mutation work to the regular test matrix.

## Profiles

Run profiles through Make, not raw workflow YAML:

```bash
make mutation-dry-run MUTATION_MODULE=root
make mutation-pr MUTATION_BASE_REF=origin/main MUTATION_MODE=advisory
make mutation-full MUTATION_MODULE=all MUTATION_MODE=advisory
make mutation-baseline-update MUTATION_MODULE=root
make mutation-rerun MUTATION_MODULE=root MUTATION_MUTANT_ID=<id>
```

The root module scans a curated seed of production packages under `internal/` and `pkg/`. The `tools/goplint` profile runs from its own module root with separate reports and baselines, and its initial full manifest selects explicit analyzer source files instead of the whole analyzer support package.

## Reports and Baselines

Reports are generated under `artifacts/mutation/<profile>/<module>/` and are ignored by Git. Each run records target-selection evidence:

- `resolved-targets.txt`
- `excluded-packages.txt`
- `not-covered-packages.txt`
- `go-mutesting.log`
- `go-mutesting-summary.json`, when emitted
- `go-mutesting-agentic.json`, when escaped mutants are emitted

Committed baselines live under `tools/mutation/baselines/`. The first accepted-survivor pass was generated from completed advisory full scans, then immediately tightened with focused tests for high-value survivors. The current committed baselines accept 2 root-module survivor rows covering 2 stable mutant IDs and 0 `tools/goplint` survivor rows covering 0 stable mutant IDs. Treat baseline updates as explicit review events: kill high-value survivors with focused tests first, then run `make mutation-baseline-update MUTATION_MODULE=<module>` or regenerate from a reviewed full report to remove killed historical survivors and accept the remaining current set.

## Operating Model

The wrapper verifies the pinned `go-mutesting` version before running. The current tool version is `github.com/jonbaldie/go-mutesting/v2` `v2.7.0`, pinned in the root `go.mod` tool directive. Upgrade it only through the version-pinning workflow.

Default mutation profiles run package-level Go tests with `-short`, even when the target manifest names explicit source files. They do not run `-race`, container-engine tests, or CLI `testscript` suites. Add focused opt-in profiles separately if a survivor needs those heavier oracles.

Mutating local profiles reject tracked dirty work outside mutation baselines/reports and restore package source directories after the tool exits. `dry-run` does not mutate source files.
