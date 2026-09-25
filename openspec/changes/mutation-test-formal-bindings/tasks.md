## 0. Preconditions

- [ ] 0.1 Confirm that `formal-infra-refinements`, `trace-validate-token-and-lock`, `model-concurrent-lock-writes`, and `model-module-path-containment` have landed on the base branch; rebase onto it

## 1. Correspondence extensions (scripts/formal.py)

- [ ] 1.1 Parse comma-separated binding cells in `correspondence_rows` and `check_correspondence`, and validate each test name. Add `scripts/test_formal.py` cases for a valid list and for a list with one missing test
- [ ] 1.2 Add the completeness guard over `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, and `*_trace_test.go`. `Test*_TraceHarness` functions must be in a `[[trace]]` package; every other test must be tabled. Add fail-closed fixture tests for an untabled test and for a harness outside a suite
- [ ] 1.3 Recognise characterisation tests: an abstraction note beginning `characterisation:`, or a `Test…_Characterisation` / `Test…_FindingF<n>` name. Add fixture tests
- [ ] 1.4 Record in `adopt-formal-verification`'s correspondence-record requirement (or confirm before archiving) that multi-test cells supersede the singular "binding test" wording
- [ ] 1.5 Run the guard on the post-sibling tree and table every test it reports. Today this includes `TestScopeConstruction_MatchesIntent` and `TestTidyToFixedPoint_GoldenVectors`; the sibling changes add more. If `formal-infra-refinements`' fingerprint still hashes the `.als` file, run `python3 scripts/formal.py fetch` and `make formal-golden`. Then run `make formal`

## 2. Plan generator (scripts/formal_mutation.py plan)

- [ ] 2.1 Create `scripts/formal_mutation.py` (SPDX header, standard library only), importing `correspondence_rows`, `load_manifest`, and `check_correspondence` from `formal.py`. Implement `plan --out DIR`: per named function, record its file, its line range (`^func …name(` to the next `^}`), its rows, its killer tests, and their packages (`go list`). Write `formal-plan.json`, `resolved-targets.txt`, and `unbound-rows.txt`
- [ ] 2.2 Exclude `Test*_TraceHarness` and characterisation tests from killer sets. Report rows left without a killer as `trace-only` or `characterisation-only`, and the other unbound rows as `no-binding` or `type-symbol`
- [ ] 2.3 Emit the union `--match` regex, and fail closed on a collision, a test found in zero or several packages, an empty plan, or a correspondence failure
- [ ] 2.4 Add `scripts/test_formal_mutation.py` with fixtures for every fail-closed path, the trace-only and characterisation-only rows, line-range extraction, and a golden plan. Wire it into `make test-scripts`
- [ ] 2.5 On the post-sibling tree, re-measure the file, leaf, and candidate counts (union `--match` dry-run) and the clean binding time per package. Replace the HEAD 4831f23d figures in design.md

## 3. Executor and pre-flight

- [ ] 3.1 Write `scripts/mutation-formal-exec.sh`:
  - require `MUTATION_FORMAL_PLAN`, `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` (exit 3 otherwise);
  - find the function containing the first changed line;
  - write a temp overlay;
  - run `go test -count=1 -overlay … -timeout <file timeout>s -run '^(…)$' <pkgs>`;
  - map pass→1, test failure→0, build/setup failure→2, timeout→0 logged as `timeout-kill`, other→3;
  - clean up on every exit path
- [ ] 3.2 Implement the pre-flight in `scripts/mutation.sh`: for each target file, run the exec script with `MUTATE_CHANGED=MUTATE_ORIGINAL` and `go test -json`. Require exit 1 and at least one killer per function reported as passed and not skipped; otherwise fail the run. Write the clean times and timeouts (`max(10 s, 5 × clean)`, `MUTATION_FORMAL_EXEC_TIMEOUT` override) into the plan
- [ ] 3.3 Add an SPDX header and bash strict mode, and add the script to the Makefile `lint-scripts` shellcheck list
- [ ] 3.4 Cover the exec script and the pre-flight in `scripts/test_mutation.sh` with a stub `GO_CMD`: every exit-code mapping, each missing variable, an unknown file or function, a pre-flight failure, a skip, and a timeout, and check that no tracked file is modified

## 4. Wrapper and Make wiring

- [ ] 4.1 Add `--target-set root|formal-bindings` / `MUTATION_TARGET_SET` to `scripts/mutation.sh`. Route `resolve_targets`, `baseline_path`, and `profile_report_dir` (`artifacts/mutation/<profile>/formal-bindings/`), and reject `pr`. Update the usage block for `MUTATION_TARGET_SET` and `MUTATION_FORMAL_EXEC_TIMEOUT`
- [ ] 4.2 Build the formal-bindings arguments: `--match`, `--exec`, `--exec-timeout` (the maximum per-file timeout), the baseline, and the logger flags. Do not pass `--coverage`, `--per-test`, `--test-flags`, or `--timeout-coefficient`. Export `MUTATION_FORMAL_PLAN` into the `export_rapid_determinism_env` subshell
- [ ] 4.3 Make the rerun profile for the target set append `{id, status, timestamp, commit}` to `tools/mutation/triage/formal-bindings-reruns.jsonl`
- [ ] 4.4 Extend `dirty_path_is_allowed` (the baseline, the ledger, and the rerun evidence) and `write_run_metadata` (`target_set`, timeout-kill count per file)
- [ ] 4.5 Add the Make targets `mutation-formal-dry-run`, `mutation-formal`, `mutation-formal-baseline-update`, and `mutation-formal-rerun`, their help lines, and the help env-var lines for the two new variables
- [ ] 4.6 Extend `scripts/test_mutation.sh` for target-set argument construction, report paths, `pr` rejection, rerun evidence appends, and the allowed dirty paths

## 5. Baseline and triage ledger

- [ ] 5.1 Create `tools/mutation/baselines/formal-bindings-baseline.json` (`{"version":1,"mutants":[]}`), `tools/mutation/triage/formal-bindings.toml` (a header documenting the classes, the fields, and the `[[closed]]` table), and an empty `formal-bindings-reruns.jsonl`
- [ ] 5.2 Implement `formal_mutation.py triage --check`. It must fail when:
  - the baseline and ledger ID sets differ;
  - a gap has no follow-up, or a defect's follow-up names no existing `finding` command;
  - an `abstraction` reason, whitespace-collapsed, is not a substring of the named model and element's abstraction cell;
  - an ID has fewer than two recorded `escaped` reruns;
  - a `[[closed]]` ID is missing from its model's `calibration` text.

  Unit-test each failure
- [ ] 5.3 Run the triage check from `make test-scripts` and after `mutation-formal-baseline-update`; not from `make formal`

## 6. Workflow

- [ ] 6.1 Add a `target_set` choice (`root`, `formal-bindings`) to `.github/workflows/mutation-testing.yml`, route it to the matching Make target, upload `artifacts/mutation/full/<target-set>/`, and set `timeout-minutes: 90` for the formal-bindings run. Keep `workflow_dispatch` only, advisory mode, and minimal permissions

## 7. First run and triage

- [ ] 7.1 Run `make mutation-formal-dry-run`, then `make mutation-formal` on a clean tree. Record in design.md the wall time; the killed, escaped, skipped, and errored counts; the timeout-kill count per file; and the escapes per file. If the run exceeds the 90-minute budget, raise the job budget and open the sharding follow-up (no `-short`)
- [ ] 7.2 Rerun every escaped mutant twice with `make mutation-formal-rerun`, and classify each per design D7
- [ ] 7.3 Close only the gaps whose fix is an assertion or generator change inside an existing binding test: confirm the kill, append `mutation <id>: …` to the model's `calibration`, and add the ID to `[[closed]]`. Defer every other gap with a follow-up
- [ ] 7.4 For each `defect`: allocate the next free finding id after `model-module-path-containment`'s ids; add the `finding` command next to a passing fix configuration, a characterisation test (marked per 1.3), and a `formal/README.md` Findings row. Do not fix the product code
- [ ] 7.5 Run `make mutation-formal-baseline-update`, fill in the ledger, and make `formal_mutation.py triage --check` pass

## 8. Documentation

- [ ] 8.1 Update `.agents/rules/commands.md`, Mutation Testing section: the target set, the Make targets, the exec script and pre-flight, runtime, advisory status, and rerun evidence
- [ ] 8.2 Update `.agents/skills/formal-verification/SKILL.md` (multi-test cells, the completeness guard, characterisation marking, and the survivor feedback loop including defects) and `formal/README.md` (replace the note that modulesync and container add no killers with a "Mutation testing" section)
- [ ] 8.3 Run `make check-agent-docs`

## 9. Verification

- [ ] 9.1 Run `python3 scripts/test_formal.py`, `python3 scripts/test_formal_mutation.py`, `bash scripts/test_mutation.sh`, and `make test-scripts`
- [ ] 9.2 Run `make formal` (correspondence, golden freshness), `make lint`, and `make lint-scripts`
- [ ] 9.3 Run `make test` (the full suite) and `make license-check`
- [ ] 9.4 Run `openspec validate mutation-test-formal-bindings --strict`
