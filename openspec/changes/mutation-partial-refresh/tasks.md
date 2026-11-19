# Tasks

All paths are relative to the change worktree, `/var/home/danilo/Workspace/github/invowk/invowk-w2b`. Never run mutation in the primary checkout. Wrap every heavy command (`make test`, `make lint`, `make test-scripts`, `make check-baseline`, mutation targets) and every `git commit` in `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" …`, and sign every commit. Do not edit `.agents/workflows/change-flow.js` or `.agents/runbooks/change-flow.md`.

## 1. Run digest and untriaged placeholder

- [ ] 1.1 In `scripts/formal_mutation.py`, make `merge_reports` append `inputs_digest=<plan_digest>` of `<report-dir>/formal-plan.json` to `run-metadata.txt` (design D2), and leave the rerun path (`run_formal_rerun`, which calls `merge-reports` per id) working. Verify: a test in `RerunAndReportTests` asserts the line and its value for a fixture plan.
- [ ] 1.2 Add `UNTRIAGED_CLASS`/`UNTRIAGED_MARK` and the `check_triage` failure of design D5 (class placeholder or reason mark), before the generic class check. Verify: `TriageTests` cases for a placeholder entry (fails with the untriaged message), a valid class with the placeholder reason (still fails), and a triaged entry (no such failure).
- [ ] 1.3 Factor the evidence rule of `check_triage` into a helper that returns per-id failures, with `check_triage` output unchanged. Verify: `python3 scripts/test_formal_mutation.py` passes with no expectation changed.

## 2. The `refresh` subcommand

- [ ] 2.1 Add the diff-to-anchor function (from `anchors.py:diff_anchor`) and report loading: read the merged `report.json`, strip the run plan's `root`, recompute ids with `mutant_id`, keep each id's first mutant, attribute each to its plan function by `originalStartLine` (design D3). Verify: a test whose fixture diffs cover a replace, a non-adjacent removal and an insertion-only mutant, and asserts the recomputed ids and `context`.
- [ ] 2.2 Implement run validation (design D2): missing `inputs_digest`, a different `inputs` list, and a digest mismatch each refuse with both digests and `make mutation-formal`; a run whose root differs from the current tree but whose inputs match is accepted. Verify: one test per case.
- [ ] 2.3 Implement symbol choice (design D3): `--symbol [FILE:]SYMBOL` (unknown, ambiguous, unplanned-but-present refuse; deleted is removal-only) and `--stale` (symbols of non-orphan entries with an `anchor_problem`, including listed stale survivors). Verify: tests for each outcome.
- [ ] 2.4 Implement the write (design D4): removals (also from `[triage] stale_survivors`), kept entries untouched, additions with anchors, own `digest`, model/element from the first row and the UNTRIAGED placeholder, placement rules, the cross-symbol id refusal, and the pre-write round-trip and parse-back checks. Verify tests (the brief's calibrations):
  - a removed id is dropped from the baseline, the ledger and `stale_survivors`;
  - a new escapee is added, `triage --check` (via `check_triage`) fails on it as untriaged and for missing evidence, then passes after a written class, reason, follow-up and two escaped rerun records at its digest;
  - unrelated baseline, ledger and manifest bytes are identical (compare the files outside the changed blocks);
  - a killed (and a skipped) id is not added, and a baselined id now killed is removed;
  - a second refresh changes no byte.
- [ ] 2.5 Implement the evidence report and summary (design D6, D7) and the `refresh` argparse entry, with the module docstring's Usage updated; exit 0 on a write or no-op, 1 on a refusal. Verify: a test captures the summary counts and the printed `make mutation-formal-rerun MUTATION_MUTANT_ID=…` line, and `python3 scripts/test_formal_mutation.py` passes.

## 3. Make target and docs

- [ ] 3.1 Add `mutation-formal-refresh` to the Makefile (`MUTATION_FORMAL_SYMBOL`, `MUTATION_FORMAL_STALE`, usage error when neither; design D8) and to the `help` list, and list it in the Mutation row of `.agents/rules/commands.md`. Verify: `make mutation-formal-refresh` with neither variable prints usage and exits 1, and `make check-agent-docs` passes.
- [ ] 3.2 Rewrite the partial-refresh procedure in `.agents/skills/formal-verification/SKILL.md` (survivor feedback loop, currently `:221-243`) around the subcommand: commit, report-only run in a throwaway detached worktree, `make mutation-formal-refresh`, triage the placeholders, reruns twice, `triage --check`; mention removal-only for a deleted or renamed function. Update `formal/README.md` (triage ledger paragraph near `:135-152`) and `.agents/runbooks/mutation-testing.md` (`:55` and the commands it lists) to match. Verify: `make check-agent-docs` passes and `grep -n "mutation-formal-refresh" .agents/skills/formal-verification/SKILL.md formal/README.md .agents/runbooks/mutation-testing.md .agents/rules/commands.md` hits each file.

## 4. End-to-end calibration (scratch copy, nothing committed from it)

- [ ] 4.1 Commit groups 1-3 (signed, under the lock). Create a throwaway `git worktree add --detach` of that commit outside the primary checkout. In it, rewrite one anchored line of `normalizeExistingOrParent` (`internal/runtime/virtual_policy.go:325`) without changing behavior, commit it there (signed), and record `triage --check` failing with "mutated code gone" for that symbol. Check whether the tooling can narrow a run to one function; it cannot today (design Context), so say so. Run `make mutation-formal` there in the background under the lock with `> <log> 2>&1; echo rc=$? >> <log>`, and wait with Monitor. Verify: the log ends with `rc=`, and `run-metadata.txt` has `inputs_digest=`.
- [ ] 4.2 In the throwaway worktree, run `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=normalizeExistingOrParent`, record its summary, and record `triage --check` failing on the untriaged placeholders (and evidence). Run `make mutation-formal-rerun MUTATION_MUTANT_ID=<added ids>` twice under the lock, write a class and reason for each added entry, and record `triage --check` passing. Verify: the three `triage --check` outputs and the refresh summary are in the result; `git diff` of the baseline and ledger shows only the symbol's entries; then remove the throwaway worktree.

## 5. Completion gates (.agents/rules/checklist.md)

- [ ] 5.1 `/simplify` on the changed Python, then `make tidy` and `make lint`. Verify: no findings.
- [ ] 5.2 `make test-scripts` and `make test`, under the lock. Verify: both pass.
- [ ] 5.3 `make check-baseline`, `make check-file-length`, `make sonar-local`, `python3 scripts/formal.py correspondence`, `make check-agent-docs`, and `openspec validate mutation-partial-refresh --strict`. Verify: all pass, and any new finding is triaged per the checklist.
- [ ] 5.4 `/learn`. Verify: a durable lesson is recorded, or none is noted.

## Workflow follow-up

- Draft PR only.
- The change-flow template wording ("report-only" in `.agents/workflows/change-flow.js:484` and `.agents/runbooks/change-flow.md:49`) is updated by a separate PR right after this one.
- Maintainer ask: the ledger header comment says anchors match "whitespace-trimmed" (`tools/mutation/triage/formal-bindings.toml:24-26`); the check is verbatim.
