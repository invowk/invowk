# Tasks

All paths are relative to the change worktree, `/var/home/danilo/Workspace/github/invowk/invowk-w2b`. Never run mutation in the primary checkout. Wrap every heavy command (`make test`, `make lint`, `make test-scripts`, `make check-baseline`, mutation targets, including `make mutation-formal-refresh`) and every `git commit` in `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" …`, and sign every commit. Do not edit `.agents/workflows/change-flow.js` or `.agents/runbooks/change-flow.md`.

## 1. Run digest and untriaged placeholder

- [x] 1.1 In `scripts/formal_mutation.py`, make `merge_reports` append `inputs_digest=<plan_digest>` of `<report-dir>/formal-plan.json` to `run-metadata.txt` (design D2), and in `scripts/mutation.sh` make `run_formal_profile` append `group_status[<n>]=<status>` after each group; leave the rerun path (`run_formal_rerun`, which calls `merge-reports` per id) working. Verify: a test in `RerunAndReportTests` asserts the digest line and its value for a fixture plan, and a script test (in the `scripts/script-tests.sh` list) asserts one `group_status` line per group, with a failing group's status.
- [x] 1.2 Add `UNTRIAGED_CLASS`/`UNTRIAGED_MARK` and the `check_triage` failure of design D5 (class placeholder or reason mark), before the generic class check. Verify: `TriageTests` cases for a placeholder entry (fails with the untriaged message), a valid class with the placeholder reason (still fails), and a triaged entry (no such failure).
- [x] 1.3 Factor the evidence rule of `check_triage` into a helper that returns per-id failures, with `check_triage` output unchanged. Verify: `python3 scripts/test_formal_mutation.py` passes with no expectation changed.

## 2. The `refresh` subcommand

- [x] 2.1 Add the diff-to-anchor function (from `anchors.py:diff_anchor`) and report loading: read the merged `report.json`, strip the run plan's `root`, recompute ids with `mutant_id`, attribute every mutant to its plan function by `originalStartLine`, then build each symbol's escaped and generated id sets (dedupe per symbol, escaped beats killed) and the unattributed list (design D3). Verify: a test whose fixture diffs cover a replace, a non-adjacent removal and an insertion-only mutant, and asserts the recomputed ids and `context`; a test with one id in two functions of one file, one with an id both killed and escaped in one function, and one with a mutant outside every plan function.
- [x] 2.2 Implement run validation (design D2): a profile other than `full`, missing `inputs_digest`, a different `inputs` list, a digest mismatch, and an incomplete run (a plan group without `report.json`, without `group_status`, or with a status other than 0 or 4) each refuse with `make mutation-formal` (and both digests for a mismatch, the groups for an incomplete run); a `build_plan` failure refuses with its message; a run whose root differs from the current tree but whose inputs match is accepted. Verify: one test per case.
- [x] 2.3 Implement symbol choice (design D3), resolving each name in D3's order: planned, removal-only (ledger entries and no function in the file), unplanned-but-present (refuse), unknown (refuse, so a typo removes nothing); ambiguous bare names refuse; `--stale` chooses the symbols of non-orphan entries with an `anchor_problem`, including listed stale survivors. Verify: tests for each outcome, including removal-only reached by both `--symbol FILE:SYMBOL` and `--stale` after the correspondence row was removed, and the refusal while the row still names the deleted function.
- [x] 2.4 Implement the write (design D4): the id-set precondition, removals, moves, shared ids left with their owner, kept entries untouched or re-anchored (anchor keys only), additions with anchors, own `digest`, model/element from the first row and the UNTRIAGED placeholder, placement rules, every chosen symbol's ids out of `[triage] stale_survivors`, and the pre-write round-trip, parse-back and anchor checks. Verify tests (the brief's calibrations, then D4's rules):
  - a removed id is dropped from the baseline, the ledger and `stale_survivors`;
  - a re-escaped entry whose `context` line changed, and a re-escaped entry listed in `stale_survivors`, are re-anchored (only `removed`/`added`/`context` change, class and reason kept), leave `stale_survivors`, and `check_triage` raises no staleness or anchor failure for them;
  - an id shared with an unchosen symbol stays with its owner and is reported as shared; an id killed and escaped in one symbol is kept; a move between two chosen symbols comes back untriaged; a move to an unchosen symbol refuses without writing; a baseline id without a ledger entry refuses;
  - a new entry is placed after the same symbol's entries, else the same file's, else before the first `[[closed]]`;
  - an unattributed mutant is counted and not added;
  - an injected parse-back mismatch fails without writing any file;
  - a new escapee is added, `triage --check` (via `check_triage`) fails on it as untriaged and for missing evidence, then passes after a written class, reason, follow-up and two escaped rerun records at its digest;
  - unrelated baseline, ledger and manifest bytes are identical (compare the files outside the changed blocks);
  - a killed (and a skipped) id is not added, and a baselined id now killed is removed;
  - a second refresh changes no byte.
- [x] 2.5 Implement the evidence report and summary (design D6, D7) and the `refresh` argparse entry, with the module docstring's Usage updated; exit 0 on a write or no-op, 1 on a refusal. Verify: a test captures the summary counts (removed, moved, added, untriaged, kept, re-anchored, shared, unattributed, awaiting evidence) and the printed `make mutation-formal-rerun MUTATION_MUTANT_ID=…` line, and `python3 scripts/test_formal_mutation.py` passes.

## 3. Make target and docs

- [x] 3.1 Add `mutation-formal-refresh` to the Makefile (`MUTATION_FORMAL_SYMBOL`, `MUTATION_FORMAL_STALE`, usage error when neither; design D8) and to the `help` list, and list it in the Mutation row of `.agents/rules/commands.md`. Verify: `make mutation-formal-refresh` with neither variable prints usage and exits 1, and `make check-agent-docs` passes.
- [x] 3.2 Rewrite the partial-refresh procedure in `.agents/skills/formal-verification/SKILL.md` (survivor feedback loop, currently `:221-243`) around the subcommand: commit, report-only run in a throwaway detached worktree, `make mutation-formal-refresh`, triage the placeholders, reruns twice, `triage --check`; mention removal-only for a deleted or renamed function. Update `formal/README.md` (triage ledger paragraph near `:135-152`) and `.agents/runbooks/mutation-testing.md` (`:55` and the commands it lists) to match. Verify: `make check-agent-docs` passes and `grep -n "mutation-formal-refresh" .agents/skills/formal-verification/SKILL.md formal/README.md .agents/runbooks/mutation-testing.md .agents/rules/commands.md` hits each file.

## 4. End-to-end calibration (scratch copy, nothing committed from it)

- [x] 4.1 Commit groups 1-3 (signed, under the lock). Create a throwaway `git worktree add --detach` of that commit outside the primary checkout. In it, rewrite one anchored line of `normalizeExistingOrParent` (`internal/runtime/virtual_policy.go:325`) without changing behavior, commit it there (signed), and record `triage --check` failing with "mutated code gone" for that symbol. Check whether the tooling can narrow a run to one function; it cannot today (design Context), so say so. Run `make mutation-formal` there in the background under the lock with `> <log> 2>&1; echo rc=$? >> <log>`, and wait with Monitor. Verify: the log ends with `rc=`, and `run-metadata.txt` has `inputs_digest=`.
- [x] 4.2 In the throwaway worktree, run `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=normalizeExistingOrParent` under the lock, record its summary, and record `triage --check` failing on the untriaged placeholders (and evidence) and on no staleness or anchor problem of that symbol. Run `make mutation-formal-rerun MUTATION_MUTANT_ID=<added ids>` twice under the lock, write a class and reason for each added entry, and record `triage --check` passing. Verify: the three `triage --check` outputs and the refresh summary are in the result; `git diff` of the baseline and ledger shows only the symbol's entries; then remove the throwaway worktree.

## 5. Completion gates (.agents/rules/checklist.md)

- [ ] 5.1 `/simplify` on the changed Python, then `make tidy` and `make lint`. Verify: no findings.
- [ ] 5.2 `make test-scripts` and `make test`, under the lock. Verify: both pass.
- [ ] 5.3 `make check-baseline`, `make check-file-length`, `make sonar-local`, `python3 scripts/formal.py correspondence`, `make check-agent-docs`, and `openspec validate mutation-partial-refresh --strict`. Verify: all pass, and any new finding is triaged per the checklist.
- [ ] 5.4 `/learn`. Verify: a durable lesson is recorded, or none is noted.

## Workflow follow-up

- Draft PR only.
- The change-flow template wording ("report-only" in `.agents/workflows/change-flow.js:484` and `.agents/runbooks/change-flow.md:49`) is updated by a separate PR right after this one.
- Maintainer ask: the ledger header comment says anchors match "whitespace-trimmed" (`tools/mutation/triage/formal-bindings.toml:24-26`); the check is verbatim.

## Calibration record (task 4, 2026-11-19)

Throwaway detached worktree at 6cbdd27e plus one signed local commit that rewrote `internal/runtime/virtual_policy.go:327` (`errors.New("path must not be empty")` -> `errors.New("path must " + "not be empty")`, same string). Nothing from it is committed here; the worktree was removed.

- No narrowing: the plan targets every bound function and neither `scripts/mutation.sh` nor the Makefile selects one, so the full run was used (781 mutants: 370 killed, 356 escaped, 55 skipped; `rc=0`, `group_status[0]=0`, `group_status[1]=0`, `inputs_digest=d3de6367...`).
- `triage --check` before: FAILED, "mutated code gone" for f4becbedb1da1e13b1b1d17d8aa10003 and 64d6a9fe921e5bcee1bd15a768e6b2d3 (normalizeExistingOrParent).
- `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=normalizeExistingOrParent`: `refresh: removed 2, moved 0, added 2, untriaged 2, kept 10, re-anchored 0, shared 0, unattributed 0, awaiting evidence 2` (added 284b951d196d87d6a1e8afe239fa9e12, c7e55be1a5b9c7a51d0bd5a7dc7f318c).
- `triage --check` after the refresh: FAILED only on the two untriaged placeholders and their missing evidence; no staleness or anchor failure.
- Two `make mutation-formal-rerun` runs recorded `escaped` at inputs d3de636778a2 for both ids (the refresh's own digest); after writing class, reason and follow-up, `triage --check` passed (403 survivors, 19 closed, 0 listed stale).
- `git diff` touched only the symbol's baseline and ledger entries (plus four rerun records); a second refresh reported `kept 12` and changed no byte.
- Baseline `line` of the added entries (327 for both) equals the `ESCAPED internal/runtime/virtual_policy.go:327` lines go-mutesting printed in the run log (its `originalStartLine`); no `baseline-update` group baseline was at hand, so the comparison with what go-mutesting would write into a baseline is unverified (design Risks; go-mutesting matches by id only).
