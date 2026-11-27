# Mutation Testing

Procedures for the manual mutation-testing profiles. Targets are listed in `.agents/rules/commands.md`, which also owns the enforced data-loss rule: the wrapper refuses to start with uncommitted tracked work outside its allow-list, and its restore acts only on a checkout it verified clean when the run started. It does not re-check, so do not edit `cmd/`, `internal/` or `pkg/` in that checkout during a run. Background and profile history: `docs/next/mutation-testing.md`.

## Scope

- Mutation testing is a manual, advisory quality signal. It is not part of `make test`, the CI test matrix or PR status checks; `mutation-testing.yml` is manual dispatch only.
- The wrapper (`scripts/mutation.sh`) verifies the pinned `go-mutesting` binary, resolves curated targets for the root module and writes reports under `artifacts/mutation/<profile>/<module>/`.
- The root full profile is a baselineable high-signal seed, not a blanket package scan. Broaden it only after local advisory timing and survivor data are stable.
- Goplint's own mutation profiles are governed in `github.com/invowk/goplint`.
- Terminal labels: `KILLED` means tests caught the mutant, `ESCAPED` means it survived. Automation reads the summary/agentic JSON or stable mutant IDs, never terminal labels.

## Invocations

```bash
make mutation-dry-run MUTATION_MODULE=root                          # count candidates, no mutation
make mutation-pr MUTATION_BASE_REF=origin/main MUTATION_MODE=advisory
make mutation-full MUTATION_MODULE=root MUTATION_MODE=advisory
make mutation-baseline-update MUTATION_MODULE=root                  # intentional baseline rewrite
make mutation-rerun MUTATION_MODULE=root MUTATION_MUTANT_ID=<id>    # id from go-mutesting-agentic.json
```

Profiles:
- `dry-run` counts candidates and does not mutate source files.
- `pr` mutates changed eligible Go lines relative to `MUTATION_BASE_REF` and exits successfully when none exist. It is a local/manual command, not a PR gate.
- `full` runs the curated root-module target manifest (`tools/mutation/`).
- `baseline-update` rewrites `tools/mutation/baselines/<module>-baseline.json`.
- `rerun` executes only one stable escaped-mutant ID.

Defaults: `MUTATION_MODULE=root`; `MUTATION_MODE=advisory` (use `blocking` only for explicit experiments after the baseline and runtime signal are stable); `MUTATION_REPORT_DIR=artifacts/mutation`; `MUTATION_WORKERS=0` locally (the workflow sets a bounded count).

Execution model: package-level Go tests with `-short`, even when a manifest selects explicit files; no `-race`, no CLI testscript suites, no container-engine profiles unless a future opt-in profile documents the cost. Local mutating profiles reject tracked dirty work outside mutation baselines/reports and restore mutated package sources on exit. `ensure_clean_tracked_worktree_for_mutation` arms the restore for the resolved checkout path; `restore_tracked_mutation_paths` and `remove_new_untracked_paths` call `require_armed_restore` first. Script tests of these helpers run in temporary repositories (`make_fixture_repo` in `scripts/test_mutation.sh`), never in the checkout.

## Baselines and reports

- Root baseline: `tools/mutation/baselines/root-baseline.json`. It holds accepted survivors from reviewed full-scan reports; update it only as an intentional review step after killing worthwhile survivors.
- Blocking mode fails only on new escaped mutants outside the selected baseline.
- Reports: `go-mutesting.log` (full output), `go-mutesting-summary.json` (metrics), `go-mutesting-agentic.json` (escaped mutants with stable IDs and context), and `resolved-targets.txt`, `excluded-packages.txt`, `not-covered-packages.txt` (target-selection evidence).

## Formal-bindings target set

`--target-set formal-bindings` (`MUTATION_TARGET_SET`) mutates only the Go functions named by bound rows of the formal models' correspondence tables, and credits a kill only to those functions' binding tests. Available on `dry-run`, `full`, `baseline-update` and `rerun`; `pr` rejects it. Manual and advisory.

```bash
make mutation-formal-dry-run                         # candidate counts per file (supports MUTATION_FORMAL_SYMBOL / MUTATION_FORMAL_STALE)
make mutation-formal                                 # pre-flight, then planned mutants (supports MUTATION_FORMAL_SYMBOL, MUTATION_FORMAL_STALE, MUTATION_FORMAL_JOBS)
make mutation-formal-rerun MUTATION_MUTANT_ID=<id>   # rerun one or more comma-separated mutants (parallelized via MUTATION_FORMAL_JOBS); appends rerun evidence
make mutation-formal-baseline-update                 # rewrite the baseline, then triage --check
make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=<sym>  # same-change refresh of <sym>'s survivors from the last full or matching scoped run
```

- **Once at the end.** Run formal-bindings mutation testing **once** on the final tree after code, review fixes, lint/tidy, SonarCloud fixes, and cross-platform test fixes settle, scoped with `MUTATION_FORMAL_SYMBOL` or `MUTATION_FORMAL_STALE=1` unless regenerating the full baseline.
- **Plan and scope.** `scripts/formal_mutation.py plan` builds `formal-plan.json` from the tables and `formal/manifest.toml` at run time: each function's file, line range, rows, killer tests and their packages (`go list`). Trace harnesses and characterisation tests are never killers; rows without a killer go to `unbound-rows.txt` (`no-symbol`, `no-binding`, `type-symbol`, `trace-only`, `characterisation-only`). The plan fails closed on a correspondence failure, a killer test declared in zero or several packages, an empty plan, or a same-file `--match` collision. When `--symbol` or `--stale` is passed (`formal_mutation.py scope`), `scope.json` restricts pre-flight and mutation to only the files declaring the chosen planned symbols (keeping every planned function in each selected file so `--match` union and cross-symbol move attribution remain intact; removal-only symbols produce zero shards and finish after writing empty reports).
- **Match groups and parallel sharding.** go-mutesting's `--match` sees only bare function names, so the plan splits files into the fewest invocations whose union regex selects exactly the named functions, and `--per-file` (`show --per-file shards`) emits one shard per `(group, file)` for scoped runs and parallel full runs (`MUTATION_FORMAL_JOBS`, defaulting to `clamp(1..6, nproc/4)` outside nested `INVOWK_MUTATION_TESTING=1`). When running with `jobs > 1`, the wrapper creates detached worker worktrees (`git worktree add --detach` cleaned up on exit/trap) so concurrent `go-mutesting` processes do not collide on tracked files, writes `.worker-root` so `report.json` `originalFilePath` entries normalize back to the main checkout's `plan_root`, and merges group/shard outputs into the profile directory with `per-file.tsv` (`killed`, `escaped`, `skipped`, `errored`, `timeout-kill` per file).
- **Pre-flight.** go-mutesting ignores `--noop` with `--exec`, so the wrapper first runs every selected file's killer tests on clean code with `go test -json` (parallelized across files up to `jobs`). Any failure, skip, absent test or timeout fails the run before a mutant executes. Each file's exec timeout is `max(10 s, 5 x clean wall time)`, recorded in the plan; override with `MUTATION_FORMAL_EXEC_TIMEOUT`.
- **Executor.** `scripts/mutation-formal-exec.sh` finds the planned function containing the first changed line and runs `go test -count=1 -vet=off -overlay=<mutant> -timeout <file timeout>s -run '^(<its killers>)$' <their packages>` in `MUTATION_FORMAL_WORKDIR` (for parallel worker worktrees): no `-short` (golden vectors replay in full), no `-race`, no trace validation or TLC, and the tracked file is never rewritten. Pass is escaped; a test failure or test-binary timeout is killed (the timeout is logged as `timeout-kill`); a build or setup failure is skipped; anything else is errored. It refuses to run (errored) without `MUTATION_FORMAL_PLAN`, `RAPID_SEED`, `RAPID_NOFAILFILE=1` and `RAPID_SHRINKTIME`.
- **Baseline and ledger.** Accepted survivors live in `tools/mutation/baselines/formal-bindings-baseline.json`, stamped with the digest of its plan inputs (the target files and the files declaring their killer tests), and are classified in `tools/mutation/triage/formal-bindings.toml` as `equivalent`, `abstraction`, `binding-gap`, `model-gap` or `defect`. Each `mutation-formal-rerun` appends `{id, status, timestamp, digest}` (plus `commit`, information only) to `tools/mutation/triage/formal-bindings-reruns.jsonl`; a baselined id needs two `escaped` records at its evidence digest: its own baseline `digest` after a partial refresh, else the baseline's. The baseline lists each id once (same-diff mutants share an id); `triage --check` fails on a repeated id. Each ledger survivor is anchored by `removed`/`added` (and `context` for an insertion-only mutant); the triage check fails on an anchor that does not hash to the id or no longer appears in its symbol's body, unless the id is in the shrink-only `[triage] stale_survivors` (or is an orphan). Digests, not commit ancestry, identify evidence, so it survives squash merges; editing a target or killer-test file makes the evidence stale until the survivors are rerun. `python3 scripts/formal_mutation.py triage --check` runs in `make test-scripts` and after `mutation-formal-baseline-update`, not in `make formal`. A change that rewrites a bound function refreshes that function's entries in the same change: a report-only **scoped** `make mutation-formal MUTATION_FORMAL_SYMBOL=<[file:]sym>[,...]` (or `MUTATION_FORMAL_STALE=1`) in a throwaway worktree, then `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=<[file:]sym>[,...]` (or `MUTATION_FORMAL_STALE=1`), which refuses an incomplete run, one of other inputs, or a scoped run that omitted any chosen symbol/file (`run-metadata.txt` records `inputs_digest`, `scope_mode`, `shard_count`, and each group/shard status), adds new escapees as `untriaged` placeholders that `triage --check` rejects, and prints the `mutation-formal-rerun` ids still owing two escaped reruns. The survivor feedback loop is in `.agents/skills/formal-verification/SKILL.md`.
- **Budget.** The manual workflow job allows 90 min for full target sets. Do not add `-short`; use `MUTATION_FORMAL_SYMBOL`/`MUTATION_FORMAL_STALE=1` and `MUTATION_FORMAL_JOBS` instead. Past run timings: `formal/README.md`.
- **Reruns.** `MUTATION_MUTANT_ID` may list several ids separated by commas. Each id is scoped to its file through the committed baseline or the last full report (the stable id hashes the file), so only that file is pre-flighted and mutated, and multiple ids are parallelized across worker worktrees up to `jobs`.
