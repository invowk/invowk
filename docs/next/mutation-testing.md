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

The root module scans curated production packages under `cmd/`, `internal/`, and `pkg/`. The `tools/goplint` module runs from its own module root with separate reports and baselines.

## Reports and Baselines

Reports are generated under `artifacts/mutation/<profile>/<module>/` and are ignored by Git. Each run records target-selection evidence:

- `resolved-targets.txt`
- `excluded-packages.txt`
- `not-covered-packages.txt`
- `go-mutesting.log`
- `go-mutesting-summary.json`, when emitted
- `go-mutesting-agentic.json`, when escaped mutants are emitted

Committed baselines live under `tools/mutation/baselines/`. The rollout baselines intentionally start empty, so the first scheduled and pull-request runs collect advisory data before maintainers decide whether blocking mode is stable enough.

## Operating Model

The wrapper verifies the pinned `go-mutesting` version before running. The current tool version is `github.com/jonbaldie/go-mutesting/v2` `v2.7.0`, pinned in the root `go.mod` tool directive. Upgrade it only through the version-pinning workflow.

Default mutation profiles run package-level Go tests with `-short`. They do not run `-race`, container-engine tests, or CLI `testscript` suites. Add focused opt-in profiles separately if a survivor needs those heavier oracles.

Mutating local profiles reject tracked dirty work outside mutation baselines/reports and restore package source directories after the tool exits. `dry-run` does not mutate source files.
