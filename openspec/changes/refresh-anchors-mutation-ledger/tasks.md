# Tasks

All paths are relative to the change worktree, `/var/home/danilo/Workspace/github/invowk/invowk-w2a`. Wrap every heavy command, and every `git commit`, in `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" …`.

## 1. Fresh report-only run (before any uncommitted edit)

- [ ] 1.1 Confirm that the worktree is clean and that `HEAD` contains origin/main 567e7b00. Then run `make mutation-formal` (report only, baseline untouched) in the background, under the lock, with `> <log> 2>&1; echo rc=$? >> <log>`, and wait for it with Monitor. Record the log path and the report directory. Verify: the log ends with an `rc=` line, and `git status --porcelain` is still empty afterwards.
- [ ] 1.2 From the log's `ESCAPED` lines and the run's `report.json` files, list every mutant generated in `standardVirtualAnchorsForOS` with its status. Verify: report the killed and escaped counts (expected 9 and 5), and that every escaped mutant is in the `case goosWindows` block (`internal/runtime/virtual_policy.go:164-168`). If one is outside it, stop and add it to `maintainer_asks` (design D4).
- [ ] 1.3 With a scratchpad script, recompute each reported mutant's id with go-mutesting's `MutantID` rule (design D1). Map each of the 419 baselined ids to "regenerated (escaped or killed) with its diff" or "not regenerated". Verify: the report lists the 22 anchors ids with their mapping, and the non-anchors ids that are not regenerated or would fail D1. Record that set; it seeds 4.3.

## 2. Triage check: anchors, staleness, per-mutant digest

- [ ] 2.1 In `scripts/formal_mutation.py`:
  - add `removed` and `added` to the survivor fields;
  - add the D1 integrity check (the hash equals the id, using the baseline's mutator);
  - add the staleness check: the symbol must exist, and the `removed` lines, stripped, must appear consecutively in the stripped body;
  - fail on an unanchored entry, unless the id is listed in `stale_survivors`.
  Each failure names the id, the symbol and the condition. Verify: new cases in `scripts/test_formal_mutation.py` cover a matching entry, a bad hash, a missing symbol, removed lines that are gone, an edit elsewhere in the body that passes, and an unanchored entry. Run `python3 scripts/test_formal_mutation.py`.
- [ ] 2.2 Make the evidence rule use a mutant's own baseline `digest` when it has one (design D3), and keep `merge_baselines` writing only the top-level digest. Verify: tests for evidence at the per-mutant digest, a per-mutant digest without evidence (fails), and the merge output having no per-mutant digest. Run `python3 scripts/test_formal_mutation.py`.
- [ ] 2.3 Prove the staleness decision reads no git history and runs no Go toolchain under `--base` (spec "Staleness check is static"). Verify: a test runs `check_triage` with a fixture root and a fake base reader and asserts that no `go` or `git log` call happens. Run `python3 scripts/test_formal_mutation.py`.

## 3. `[triage] stale_survivors`

- [ ] 3.1 In `scripts/formal.py`, extend `TRIAGE_KEYS` and `parse_triage` with `stale_survivors` (a list of unique, non-empty ids). In `scripts/formal_mutation.py`:
  - extend `GROWTH_LISTS`, `shrink_lists` and `check_growth` with the D5 seeding rule;
  - add the listed or stale-entry handling and the leave-notice reason in `check_triage`/`departure_reasons`.
  Verify: tests in `scripts/test_formal.py` and `scripts/test_formal_mutation.py` cover an unknown key, a base with all three lists but no `stale_survivors` (seeds, with a notice), a base with only some lists (fails, naming `stale_survivors` too), growth (fails), a listed id that passes the checks (fails as stale), and an id leaving (notice).
- [ ] 3.2 Add `stale_survivors = []` (or the set from 1.3) to `formal/manifest.toml` `[triage]`, with a header comment stating its shrink-only rule. Verify: `python3 scripts/formal.py correspondence` passes.

## 4. Refresh the ledger and baseline

- [ ] 4.1 Remove the 22 old `standardVirtualAnchorsForOS` ids from `tools/mutation/baselines/formal-bindings-baseline.json` and `tools/mutation/triage/formal-bindings.toml`. Add the fresh escaped mutants (`{id, file, mutator, line}` verbatim from the run, in sorted order) and their ledger entries, using the D4 class, model, element, follow_up and reason wording. Verify: the old and new entry counts are reported (expected 419 → 402), and no anchors id from the old set remains.
- [ ] 4.2 Commit 4.1 first, signed and under the lock: the mutation wrapper refuses tracked work outside its allow-list. Then run `make mutation-formal-rerun MUTATION_MUTANT_ID=<id>` twice per new id, under the lock, and set each new baseline entry's `digest` to the digest those reruns recorded. Verify: `formal-bindings-reruns.jsonl` has two `escaped` records per new id at that digest.
- [ ] 4.3 Backfill `removed` and `added` into every ledger entry the fresh run regenerated (design D2). Put the ids from 1.3 that cannot be anchored or that fail D1, if any, into `[triage] stale_survivors`, and list them in the result without fixing them. Verify: `python3 scripts/formal_mutation.py triage --check --base origin/main` passes and reports the stale_survivors count.
- [ ] 4.4 Calibration (design D6): build an anchored copy of the old ledger's 22 entries, from a dry run (or a report-only run) in a throwaway worktree at `90e56ad`, outside the primary checkout. Run `triage --check --ledger <old copy>` against the current tree. Verify: it fails and names each stale id with its symbol (expected 19). The other 3 are explained as regenerated and killed. The committed ledger passes. Record both outputs and remove the throwaway worktree.

## 5. Docs and history

- [ ] 5.1 Update `tasks/next/formal-binding-gaps.md`:
  - the "Retriaged by require-absolute-base-dirs" bullet and "### standardVirtualAnchorsForOS" (the refresh done, the new survivors, the legacy unit test);
  - the 419/22 counts;
  - a Future work bullet that empties `[triage] stale_survivors`.
  Update the ledger header comment (the new fields and checks), the `[triage]` comment in `formal/manifest.toml`, `formal/README.md`, `.agents/skills/formal-verification/SKILL.md` and `.agents/runbooks/mutation-testing.md` where they describe ledger fields or `[triage]`. Verify: `grep -rn "419\|22 survivors" tasks/next/formal-binding-gaps.md formal/README.md tools/mutation/triage/formal-bindings.toml` shows only intended history, and `make check-agent-docs` passes.

## 6. Completion gates (.agents/rules/checklist.md)

- [ ] 6.1 `/simplify` on the changed Python, `make tidy`, then `make lint`. Verify: no lint findings.
- [ ] 6.2 `make test-scripts` (scripts, `tools/mutation/` and `formal/` changed) and `make test`, under the lock. Verify: both pass.
- [ ] 6.3 `make check-baseline`, `make check-file-length`, `make sonar-local`, `python3 scripts/formal.py correspondence` and `make check-agent-docs`. Verify: all pass, and any new finding is triaged per the checklist.
- [ ] 6.4 `/learn`. Verify: any durable lesson is recorded, or none is noted.

## Workflow follow-up

- Draft PR only. Nothing merges before v0.17.0 is tagged at 567e7b00.
- Out of scope for the follow-up survivor-triage change:
  - the ~397 other survivors and the full baseline refresh;
  - the `Auth` row;
  - a `normalizeZIPPath` row;
  - per-OS anchors in `VirtualPathHarness.als`;
  - emptying `[triage] stale_survivors`.
