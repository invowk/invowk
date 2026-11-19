# Design

## Context

For the motivation, see proposal.md, section Why. The current state that shapes the approach:

- **Ledger and baseline.** `tools/mutation/triage/formal-bindings.toml` holds 419 `[[survivor]]` entries. The fields are strict: `SURVIVOR_KEYS` (`scripts/formal_mutation.py:91`) are id, file, symbol, model, element, class, reason and follow_up. `tools/mutation/baselines/formal-bindings-baseline.json` stores `{id, file, mutator, line}` per mutant and one top-level `digest` (`7c0bfca9…`). `merge_baselines` (`scripts/formal_mutation.py:659-671`) writes that digest.
- **Evidence.** `check_triage` (`scripts/formal_mutation.py:995-1095`) requires two `escaped` reruns per id at the baseline's single digest (`:1061-1078`). `rerun` stamps the *current* plan-inputs digest (`:1384-1385`). Every one of the 860 rerun records is at `7c0bfca9…`.
- **What the check never does.** It never compares an entry with the current source. Its symbol lookup only confirms that a correspondence row names the symbol (`:1050-1054`).
- **Mutant ids.** A go-mutesting stable id is `md5(relFile \0 mutator \0 removed-lines \0 added-lines)`, each diff line followed by `\n`, with no line numbers (`internal/baseline/baseline.go:36-57`, v2.8.3). The baseline `line` drifts with any edit above a function; the id does not.
- **Baseline parsing.** go-mutesting reads the baseline with `json.Unmarshal` (`baseline.go:71`), so extra fields are ignored. The top-level `digest` already relies on this.
- **`[triage]` table.** It has exactly two keys (`scripts/formal.py:180`, `:318-331`). The growth check (`scripts/formal_mutation.py:800-864`) treats a base with only some of its lists as an error (decision 19 of harden-formal-instruments).
- **The function body.** `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go:154-176`) now reads `basedir.Dirs`. Its only per-OS split is `case goosWindows` versus `default` (`:163-174`).
  - `TestStandardVirtualAnchorsMatchLegacy` (`internal/runtime/virtual_anchors_legacy_test.go:54`) runs `goos = "windows"` on every host.
  - It is not the row's killer. The `anchors` row binds `TestVirtualPathHarness_RootConstruction` (`formal/alloy/VirtualPathHarness.als:31`).
- **CI checkouts.** CI runs `make test-scripts` from depth-1 checkouts (`.github/workflows/lint.yml:192`). Git history is not available to the check. Only the base manifest is fetched (`scripts/ci-triage-base.sh`).

### Measurement (spec stage, read-only)

A scratch script compared each ledger entry with the current tree. It reused `function_decls` and `find_function`, and took each entry's original line from the evidence commit `90e56ad` at its baseline `line`.

| Candidate rule | anchors entries flagged (of 22) | other entries flagged (of 397) |
| --- | --- | --- |
| symbol missing | 0 | 0 |
| baseline `line` outside the symbol's current range | 7 | 21 (line drift, too coarse) |
| `Changes:` text in the reason absent from the body (353 of 419 reasons carry it) | 11 | 0 |
| original line at the baseline line absent from the body | 19 | 0 |

The three anchors entries that the text rule does not flag are `virtualAnchorTmp:  tempDir,` and `virtualAnchorWork: workDir,` (field-clear) and `return anchors` (statement/return). Their removed and added lines are unchanged, so go-mutesting regenerates the same ids in the new body. The S4 run reports them killed. They are current mutants that are no longer survivors. A text rule correctly leaves them alone, and the refresh removes them because the fresh run kills them.

## Goals / Non-Goals

**Goals:**
- Detect, statically and per entry, a ledger entry whose mutant no longer exists in the current code.
- Refresh exactly the `standardVirtualAnchorsForOS` entries, without disturbing the other entries' evidence.

**Non-Goals:**
- Detecting a survivor that is now killed. Only a mutation run can tell, and the existing evidence rule covers it at the next full refresh.
- Re-deriving the other ~397 survivors' rerun evidence.
- Making the check fail on any edit of a bound file.

## Decisions

### D1. Anchor each entry with the diff lines that define its id
Each `[[survivor]]` gets `removed` and `added`: the exact removed and added lines of the mutant's go-mutesting diff, as TOML multi-line literal strings with their indentation. An insertion-only mutant (no removed lines) also gets `context`, the unchanged line just before its first added line.

A read-only check of the 3747 mutants in the primary checkout's existing `artifacts/mutation/**/go-mutesting-agentic.json` reports supports this design:
- the formula below reproduces every reported id;
- 70 mutants remove no line (`loop/range_break` 66, `branch/case` 4), which is why `context` exists;
- 8 have removed lines separated by context, and some of those remove a comment after the function that go/printer re-flowed. So the removed lines are matched in order but not as one consecutive block, and only the first must lie inside the function.

The check validates the fields in two ways:
- **Integrity.** `md5(file \0 baseline mutator \0 removed \0 added)` must equal the id. The anchor then cannot be typed wrong or point at the wrong mutant, and anyone can verify it without tools.
- **Staleness.**
  - The symbol must still name a function in `file`. The check uses the existing `function_decls` and `find_function`, with no `go list`.
  - The anchor lines (`removed`, or `context` when `removed` is empty), each stripped of surrounding whitespace, must appear in order in the current file. The first one must be inside the symbol's current body.
  - Failures name the id, the symbol, and which condition failed.

Alternatives considered:
- *Parse the `Changes:` text from `reason`.* 66 of 419 reasons have none: `branch/if`, `composite/field-clear` and some `statement/remove` mutants. It flags only 11 of the 22, and it keeps a machine contract inside prose.
- *Baseline `line` within the symbol's range.* Line drift flags 21 healthy entries, and the 22 stale ones mostly still fall inside the function.
- *Read the original line from the evidence commit with `git show`.* Squash merges drop those commits, and CI checks out at depth 1.
- *A per-symbol body hash.* It fails on every edit of the function, which the brief rules out.

Matching uses whitespace-stripped lines, so `gofmt` re-indentation does not flag an entry. A changed line flags it. A `removed` block that also appears elsewhere in the body still passes. That is accepted: the id integrity check pins which mutant the entry is, and the follow-up refresh re-derives everything anyway.

### D2. Backfill the anchors from the fresh run's reports
The report-only `make mutation-formal` run on 567e7b00 writes per-group `report.json` files. They list escaped and killed mutants with `mutator.mutatorName`, `mutator.originalFilePath` and `diff` (`scripts/formal_mutation.py:678-697`). A one-off script in the scratchpad does the backfill. It is not committed, but its rule is written down here:
1. It recomputes each reported mutant's id with D1's hash.
2. For every baselined id the run regenerates, it writes `removed` and `added` into the ledger entry.
3. A baselined id that the run does not regenerate cannot be anchored.
   - An `standardVirtualAnchorsForOS` id leaves the ledger and the baseline (D4).
   - An id listed in `[triage] orphaned_survivors` is exempt from anchoring. The 20 orphans are exactly the ledger survivors whose function the plan no longer targets (`validateDestinationPath` 12, `Server.ValidateToken` 8). A spec-stage plan build confirmed that every other ledger symbol is planned. Anchoring them would only list them twice.
   - Any other such id goes to `[triage] stale_survivors` (D5) and is listed in the result, not fixed.

The run regenerates the old `Tmp`/`Work`/`return anchors` ids as killed. Those stay out of the baseline because D4 removes every old anchors entry.

### D3. Per-mutant evidence digest
A baseline mutant may carry its own `digest`. The evidence rule uses it when present, and the baseline's top-level digest otherwise.
- The new anchors entries carry the plan-inputs digest of their own two `mutation-formal-rerun` runs on this branch.
- `merge_baselines` keeps writing only the top-level digest, so a full regeneration drops the per-mutant ones.
- The other ~397 survivors keep the old digest. Their evidence was already stale by the digest convention, as `tasks/next/formal-binding-gaps.md` says ("the rerun evidence of all 419 survivors is stale"). The survivor-triage follow-up owns that.

Alternatives considered:
- *Re-stamp the top-level digest.* It invalidates evidence for 397 entries: the full refresh, which is out of scope.
- *Exempt new entries from evidence.* It weakens the survivor rule.
- *Record evidence in the ledger.* That duplicates the rerun log.

### D4. Replace the anchors entries
- **Remove.** Delete all 22 old `standardVirtualAnchorsForOS` ids from the baseline and the ledger.
- **Add.** Add each fresh-run escaped mutant of that symbol: `{id, file, mutator, line}`, verbatim from the run, and its own `digest`. Keep the baseline sort order, file, line, mutator, id (`:669`).
- **Ledger entries.** Each new entry has class `binding-gap`, model `VirtualPathHarness`, element `anchors`, and follow_up `tasks/next/formal-binding-gaps.md#standardvirtualanchorsforos`. Its reason says the mutant is in the `windows` case. The formal-bindings mutation lane runs only on Linux, where the killer `TestVirtualPathHarness_RootConstruction` reaches only the host GOOS. `TestStandardVirtualAnchorsMatchLegacy` checks every anchor for `goos = "windows"` as a unit test and is deliberately not a killer.
- **Expected and actual counts.** The S4 run expects 5 survivors (the case no-op and four dropped assignments), so the entry count would go 419 → 402. The actual set and counts come from the fresh run and are reported. If the fresh run escapes a mutant outside the `windows` case, the apply stage stops and lists it in `maintainer_asks`, because decision 35 expects only Windows-case survivors.

### D5. `[triage] stale_survivors`, a shrink-only list seeded by this change
Entries outside the anchors set that D1 flags, or that D2 cannot anchor, are listed by id in `formal/manifest.toml` `[triage] stale_survivors`. A listed id is exempt from the D1 checks but printed in the report. A listed id that passes both checks fails as a stale list entry. The list joins the growth check as a fourth list.

The line-text measurement and the orphan exemption predict an empty list; task 1.3 confirms it. It is still added now, empty or not, so that:
- the follow-up has a mechanism;
- an entry that a later edit breaks cannot be silently deleted from the baseline to make the check green. Removing a ledger id still requires the baseline to change with it (the existing ledger-equals-baseline rule), and that is visible in review.

Seeding: today a base with only some lists fails (decision 19). That would fail this very PR, since origin/main has the first three lists. The rule therefore becomes:
- the first three lists keep their all-or-nothing seeding;
- `stale_survivors` seeds from a base that has all three but lacks it;
- a base with only some of the three still fails for each missing one, including `stale_survivors`.

Renaming any key still cannot reopen seeding, because a rename makes one of the three missing.

Alternatives considered:
- *A `stale = true` flag in the ledger.* Shrink-only would then need the base ledger, which is a new git read.
- *No list, block on any flagged entry.* The brief asks for a non-blocking mechanism.
- *Put the list in the ledger file.* `check_triage` rejects unknown ledger tables (`:1016-1018`), and the growth machinery already reads the manifest.

### D6. Calibration
Calibration proves the check fails on the old ledger and passes on the new one.
- It builds the old ledger, anchored, from a dry-run in a throwaway worktree at `90e56ad`. `mutation-formal-dry-run` lists candidate mutants and their diffs without running tests. If its reports carry no diffs, the calibration runs the old-tree report-only run in that worktree instead.
- It runs the old ledger's 22 anchored entries through the new check on the current tree. Expected result, from the spec-stage line-text measurement and to be confirmed by task 1.3: the check fails and names 19 ids. The three whose ids the new body regenerates and kills (`Tmp`/`Work` field-clear, `return anchors`) are current mutants, not stale ones. They are explained, not flagged. This differs from the brief's "the 22 stale entries": the check detects mutants that no longer exist, not survivors that are now killed.
- It records the new ledger passing.
- Unit tests in `scripts/test_formal_mutation.py` cover each failure condition with fixtures, so the calibration does not depend on the throwaway worktree.

## Risks / Trade-offs

- [The report-only run lists baselined escapes only in a filtered form] → Before backfilling, check that `report.json` contains every baselined id the run regenerates. Otherwise compute the ids from the `diff` of all reported mutants (escaped and killed), whose ids D1's hash reproduces.
- [The ledger grows by about 2 multi-line strings per entry, roughly +1600 lines] → This is accepted for a machine-checkable anchor. `make check-file-length` covers only Go files.
- [A duplicate `removed` block elsewhere in the body masks a real staleness] → Accepted (D1). The id integrity check prevents mis-anchoring.
- [The formal-verification skill says "Do not `mutation-formal-rerun` ledger ids of the removed body" (`.agents/skills/formal-verification/SKILL.md:217-218`)] → Task 4.2 reruns only the new ids from the current body, never the 22 removed ones. The skill's report-only run belongs in a throwaway `git worktree add --detach`, so task 1.1 runs it there.
- [The mutation wrapper reverts `cmd/`, `internal/` and `pkg/` and refuses a dirty tree (`.agents/rules/commands.md`)] → Do the run first, on a clean committed worktree, under the heavy-command lock, before any uncommitted edit.
- [Per-mutant digests make evidence harder to reason about] → They exist only between partial refreshes. The full regeneration removes them (spec scenario "Full regeneration drops per-mutant digests").

## Migration Plan

This changes repository tooling and data only. To roll back, revert the commit. The old ledger without anchors would then fail only the new check, which leaves with the revert.

## Maintainer decisions

- 2026-11-19, decision 35:
  - Refresh the 22 stale anchors entries from a fresh report-only run on origin/main 567e7b00.
  - Add a staleness check to `triage --check`.
  - Keep `TestStandardVirtualAnchorsMatchLegacy` as a unit test, not a formal-bindings killer.
  - Triage the Windows-case survivors as binding gaps whose reason names the Linux-only lane and the legacy test.
  - Do the other survivor-triage items in the follow-up, and only list them here.

## Open Questions

- Whether `mutation-formal-dry-run` reports carry diffs. It decides only which calibration route D6 takes, not the check.
