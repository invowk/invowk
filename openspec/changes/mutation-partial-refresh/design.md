# Design

## Context

See proposal.md, Why. The current state that shapes the approach:

- **Ledger and baseline.** `tools/mutation/triage/formal-bindings.toml` holds 403 `[[survivor]]` entries and some `[[closed]]` entries after them (first `[[closed]]` at line 6490). `tools/mutation/baselines/formal-bindings-baseline.json` is `json.dumps(data, indent=2) + "\n"` byte for byte (checked on c6b32b89), sorted by file, line, mutator, id (`scripts/formal_mutation.py:680`). `merge_baselines` writes only go-mutesting's four fields plus the top-level digest (`scripts/formal_mutation.py:668-692`).
- **Anchors and staleness.** `anchor_problem` (`scripts/formal_mutation.py:805-832`) returns the failed condition for one entry, reading only the current tree through `source_reader` (`:835-861`). `check_triage` calls it for every non-orphan entry (`:1190-1209`) and fails on an unknown class with a generic message (`:1214-1215`).
- **Evidence.** A mutant's evidence digest is its own baseline `digest`, else the top-level one; two `escaped` reruns at that digest are required (`scripts/formal_mutation.py:1237-1258`). `rerun-record` stamps `plan_digest` of the plan built at rerun time (`:1563-1567`); `plan_digest` hashes the plan's `inputs` read from `plan["root"]` (`:416-440`).
- **Run reports.** `make mutation-formal` (`Makefile:275-276`) runs `run_formal_profile` (`scripts/mutation.sh:944-1012`): plan, pre-flight, one go-mutesting call per group, exit restore, then `merge-reports`. `merge_reports` (`scripts/formal_mutation.py:695-743`) writes the merged `report.json` with `killed`, `escaped`, `skipped` and `errored` lists, and appends to `run-metadata.txt`; it skips a group without `report.json` silently, and nothing records each group's exit status (`run_formal_profile` keeps only the worst one, and merges after a failed group); the plan stays at `<report-dir>/formal-plan.json`. Reported paths are absolute under the run's root, which `merge_reports` strips with `REPO_ROOT` (`:723-727`). `--baseline` only gates the exit status: `report.json` still lists baselined escapes (formal-verification skill, `.agents/skills/formal-verification/SKILL.md:228-232`). The archived run confirms it: of 419 baselined ids, 350 came back escaped, 30 killed and 39 not generated (scratchpad `analysis-new.json` of refresh-anchors-mutation-ledger).
- **No narrowing.** The plan targets every bound function (`build_plan`, `scripts/formal_mutation.py:344-395`); neither `mutation.sh` nor the Makefile has a per-function or per-symbol selector. A refresh therefore needs the full run (~35 minutes).
- **Plan functions.** Each plan function has `file`, `symbol`, `start`, `end` and `rows` (model, element, killers) (`scripts/formal_mutation.py:331-339`), which is what a new ledger entry's `model`, `element` and reason need.
- **The manual procedure** this replaces: `.agents/skills/formal-verification/SKILL.md:221-243`, and the scratch scripts `anchors.py` (diff to anchor, id recomputation) and `apply.py` (text-level ledger splice, baseline rewrite) in `/var/home/danilo/Workspace/github/invowk/w2-partial-refresh/`.
- **Mutation wrapper allow-list.** A rerun or run accepts uncommitted changes only to the baseline, the ledger, the reruns file and `artifacts/mutation/*` (`scripts/mutation.sh:322-341`). `formal/manifest.toml` is not on it.

## Goals / Non-Goals

**Goals:**
- One command that turns a report-only run into the D7 partial refresh for chosen symbols, with output a reviewer can check entry by entry.
- An untriaged survivor can never pass `triage --check`.

**Non-Goals:**
- Running mutation from the refresh (it validates evidence; it does not produce it).
- A focused per-function run profile.
- Retriaging survivors outside the chosen symbols, or rewriting their evidence.
- The change-flow template wording.

## Decisions

### D1. Subcommand `refresh` in `scripts/formal_mutation.py`
`formal_mutation.py refresh [--report-dir DIR] (--symbol [FILE:]SYMBOL ... | --stale) [--baseline F] [--ledger F] [--reruns F] [--manifest F]`. `--symbol` and `--stale` combine (union). `--report-dir` defaults to `artifacts/mutation/full/formal-bindings` under the repository root. The paths default to the committed files, so tests pass fixtures.

The code lives beside `check_triage`, reusing `mutant_id`, `anchor_problem`, `source_reader`, `read_baseline`, `read_reruns`, `load_plan` and `plan_digest`. The anchor derivation from a diff (`anchors.py:diff_anchor`) moves in as a function shared with nothing else today; the archived design's measurement showed it reproduces every reported id. If the file grows past readability, the implementer may move refresh helpers to a sibling module imported by `formal_mutation.py`; the subcommand stays where the brief and docs name it.

Alternatives considered: a separate script (another entry point to document and test-list for no gain); keeping the scratch scripts (the hand job this change removes).

### D2. Run validation, completeness and the recorded digest
`merge_reports` appends `inputs_digest=<plan_digest(plan)>` to `run-metadata.txt`, computed from the plan in the report dir after the exit restore, so it is the digest of the tree the run mutated. `run_formal_profile` appends `group_status[<n>]=<status>` to `run-metadata.txt` after each group's go-mutesting call (today nothing records it, and `merge_reports` silently skips a group without `report.json`, `scripts/formal_mutation.py:711-714`). The refresh:
1. loads `<report-dir>/formal-plan.json` and `run-metadata.txt`, and requires `profile=full` (not `dry-run`, `baseline-update` or a rerun) and the recorded `inputs_digest` (fails when absent: "rerun `make mutation-formal`");
2. requires the run to be complete: for every group `0..group-count-1` of the run plan, a `group-<n>/report.json` and a `group_status[<n>]` of `0` or `QUALITY_GATE_EXIT_CODE` (`4`, what a report-only run returns when it reports escapes); a missing report, a missing status line (the loop stopped early, `scripts/mutation.sh:995`) or any other status fails, naming the groups and "rerun `make mutation-formal`". Otherwise a crashed group's functions would report no mutants and their survivors would be removed as "not generated";
3. builds the current plan (`build_plan`, which runs `go list`; the refresh is not the static triage check) and requires its `inputs` to equal the run plan's and its digest, read from the current tree, to equal the recorded one. `build_plan` failing (for example a correspondence row that still names a deleted function, `check_correspondence` at `:349-351`) fails the refresh with its message;
4. resolves reported paths by stripping the run plan's `root`, so a run in a throwaway worktree at the same inputs is accepted.

The new entries' per-mutant `digest` is that digest. It is exactly what `rerun-record` will stamp for a rerun on this tree, so the evidence check lines up without copying digests by hand.

Alternatives considered: reading the inputs from the run's root (the throwaway worktree may be gone); trusting the run blindly (a run on older inputs would add mutants of code that no longer exists, and an incomplete run would delete valid survivors).

### D3. Attribution and choice of symbols
**Attribution.** Every reported mutant (all statuses) gets its id recomputed with `mutant_id` from its diff, and belongs to the plan function whose `[start, end]` contains its `mutator.originalStartLine` in its file (the run plan's ranges, equal to the current tree's by D2). A mutant that falls in no plan function is *unattributed*: it is counted and listed in the summary, never added, and it still counts for "escaped anywhere" in D4. For each symbol `S`, `E(S)` is the set of ids with at least one *escaped* mutant attributed to `S`; `G(S)` is the set of ids with any mutant attributed to `S`. Deduplication is per symbol, after attribution. An id escaped in one mutant and killed in another counts as escaped, because go-mutesting's baseline matches by id only and suppresses the id wherever it escapes (as `merge_baselines` keeps one entry per id, `:684-690`).

**Choice.** A ledger entry's symbol is its `(file, symbol)`; baseline entries have no symbol and are attributed only through the ledger entry of the same id.
- `--symbol SYMBOL` / `--symbol FILE:SYMBOL`. A name is resolved in this order: (a) it names exactly one current plan function (a bare name matching several files fails as ambiguous, `FILE:` disambiguates): refresh it; (b) otherwise, the ledger has entries for it (in `FILE`, or in exactly one file for a bare name) and `find_function` on the current source of that file returns no function: it is *removal-only*; (c) otherwise, it still names a function in its file (the named `FILE`, or a file whose ledger entries name it) but the plan does not target it (its row is unbound, as for an orphan): refuse as unplanned; (d) otherwise: refuse as unknown, naming the symbol. So a typo, which has no ledger entries, never removes anything.
- `--stale` chooses the `(file, symbol)` of every ledger entry, orphans excluded, for which `anchor_problem` returns a problem, which includes ids in `[triage] stale_survivors`. Each such symbol is resolved by (a) to (c) above.
- `--symbol` and `--stale` combine (union). Any refusal fails the whole command before any write.

**Removal-only** (a deleted or renamed function): all of the symbol's entries are removed, nothing is added under it, and the summary says so. Its correspondence row must be removed or renamed first, because the current plan (D2 step 3) fails while a row names a function that no longer exists; the refusal message says so. The renamed function's new name is refreshed with `--symbol` in the same command; ids that move from the old name to the new one follow D4's move rule and come back untriaged.

### D4. What the refresh writes
For the chosen symbols, with `E(S)` and `G(S)` from D3 and `owner(id)` the `(file, symbol)` of the id's ledger entry:
- **Preconditions.** An id in `E(S)` of a chosen `S` that the baseline lists but the ledger does not (or the reverse) fails the command: the id sets already disagree, which `triage --check` reports, and the refresh does not guess an owner.
- **keep**: an entry of chosen `S` whose id is in `E(S)`. Its baseline entry is never changed. Its ledger entry stays byte-identical when it passes `anchor_problem`; when it does not (a changed `context` line of an insertion-only mutant, an anchor found outside the function, or an anchor mismatch), only its `removed`, `added` and `context` keys are rewritten from the run's diff of that id in `S` (`removed`/`added` are fixed by the id's hash, so in practice `context` changes or the stored anchor was wrong), and it is counted as *re-anchored*. Its class, reason and other keys stay as written.
- **move**: an entry of chosen `S` whose id is not in `E(S)` but is in `E(T)` of another chosen symbol `T`: removed under `S` and added under `T` as below (untriaged, own digest), and reported as moved. When the id escapes only in unchosen symbols, the command fails, naming `S` and those symbols, and says to choose them too.
- **remove**: an entry of chosen `S` whose id is escaped nowhere in the run: removed from the baseline and the ledger, reporting why (its mutants' statuses in `S`, or "not generated" when `G(S)` lacks it).
- **shared**: an id in `E(S)` of chosen `S` whose owner is an unchosen symbol is not touched and is reported as shared with its owner. There is no cross-symbol refusal: the same line mutated the same way in two functions of one file (e.g. `\t\treturn nil, err`) gives one id, and it stays with its owner.
- **add**: every id in `E(S)` of a chosen `S` that is not baselined (after removals and moves): baseline `{id, file, mutator, line, digest}` (line = `originalStartLine` of the first such mutant in `S` by line), inserted at its sort position; ledger entry `id, file, symbol, model, element, class, reason, follow_up, removed, added[, context]`. An id not baselined and escaped in several chosen symbols is added once, under the first by `(file, start line)`; the summary names the others.
  - `model` and `element` come from the plan function's first row (sorted); when the function has several rows, the reason placeholder lists the others so the person triaging picks the right one.
  - `class = "untriaged"`, `reason = "UNTRIAGED: <mutator> (<agentic description>) survives <killers>; write the class and the reason"`, `follow_up = ""`.
  - It is inserted after the last ledger entry of the same symbol, else of the same file, else before the first `[[closed]]` (else at the end).
- **`[triage] stale_survivors`**: every id of a chosen symbol leaves the list, whether removed, moved, kept or re-anchored. After the refresh every remaining entry of a chosen symbol passes `anchor_problem` (verified before writing, below), so a listed id would fail "passes the staleness and anchor checks; remove it" (`:1205-1207`). Ids of other symbols stay listed.

Writing is text-level for the ledger and the manifest (split on `[[`-blocks as `apply.py` does, rewrite single anchor keys inside a kept block, and delete single `"<id>",` lines from the `stale_survivors` array), and `json.dumps(indent=2) + "\n"` for the baseline. Before writing, the refresh checks that the baseline round-trips byte-identically through that serializer; that the new ledger text parses back (with `tomllib`) to the old entries plus the added ones minus the removed ones, with only the anchor keys of re-anchored entries changed and each new or rewritten anchor equal to the run's diff lines; and that every remaining entry of a chosen symbol passes `anchor_problem` against the current tree. Otherwise it fails without writing.

Alternatives considered: a TOML writer for the whole ledger (would reformat 6000 lines and break byte-identity); keeping re-escaped entries byte-identical even when their anchor fails (a refresh could finish and `triage --check` still fail on exactly the symbols `--stale` picks); refusing every id shared with an unchosen symbol (common Go lines would block most refreshes); dropping ids from `stale_survivors` by hand (a trap for every user).

### D5. The untriaged placeholder
`UNTRIAGED_CLASS = "untriaged"` and `UNTRIAGED_MARK = "UNTRIAGED:"`. `check_triage` fails, before the class check, for an entry whose class is the placeholder or whose reason starts with the mark: "survivor <id> (<symbol>): untriaged placeholder from `refresh`: write the class (one of …) and the reason". The class stays out of `TRIAGE_CLASSES`, so the generic unknown-class failure cannot be bypassed either. Checking both fields means setting only the class still fails.

Alternatives considered: an empty reason (already fails, but with a message that does not say what to do, and an empty class string would also need a new rule); a separate `untriaged = true` field (one more key to keep in `SURVIVOR_KEYS` forever).

### D6. Evidence: validate, never run
After writing, the refresh applies the evidence rule of `check_triage` (`:1237-1258`) to the chosen symbols' remaining entries, reusing the same code (factored into a helper both call). Ids with fewer than two escaped reruns at their digest are printed with `make mutation-formal-rerun MUTATION_MUTANT_ID=<id,id,...>` and "run it twice". Mutation runs take minutes to an hour and need the heavy lock and a clean tracked tree; driving them from a data-edit command would hide that cost and could not be unit-tested. Since the baseline and ledger are on the wrapper's allow-list (`scripts/mutation.sh:329-331`), the reruns can follow the refresh without a commit, but `formal/manifest.toml` is not: when the refresh changed it, the summary says to commit before rerunning.

Exit status: 0 when the refresh wrote (or found nothing to change); 1 on any refusal. Outstanding triage and evidence are reported, and `triage --check` is the gate. Running the refresh again with the same run reports every entry of the chosen symbols as kept, none re-anchored, and changes no byte (it is idempotent); after the reruns it reports zero awaiting evidence.

### D7. Summary
One line per removed, moved, added, kept, re-anchored, shared and unattributed id (id, symbol, mutator, line, and for removals the reason, for shared ids the owner), then `refresh: removed N, moved N, added N, untriaged N, kept N, re-anchored N, shared N, unattributed N, awaiting evidence N`. Removal-only symbols get a line of their own. "Untriaged" counts all entries of the chosen symbols that still carry the placeholder, not only the ones just added. "Kept" includes the re-anchored entries.

### D8. Make target
`make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=<sym>[,<sym>...]` or `MUTATION_FORMAL_STALE=1` (both allowed), passing `--report-dir "$(MUTATION_REPORT_DIR)/full/formal-bindings"`. With neither, it prints usage and exits 1, like `mutation-formal-rerun` (`Makefile:281-285`). It runs Python and `go list` and mutates nothing, but `.agents/rules/agent-workflows.md` lists "mutation targets" among the heavy commands, so workflow agents run it under the heavy lock like the others (tasks 4.2); it needs no clean tree and no exit restore.

### D9. Calibration
- Unit tests in `scripts/test_formal_mutation.py` with fixture roots and synthetic `report.json`/plan/metadata: removed id dropped; new escapee added and rejected by `triage --check` until triaged and evidenced, then accepted; unrelated entries byte-identical; killed id not added; inputs-digest mismatch refused; incomplete run (a group without `report.json`, a missing or failing `group_status`) refused; unplanned and unknown symbols refused; removal-only for a deleted function with ledger entries, and its row-update precondition; a re-escaped entry with a failing anchor (changed `context`, and one listed in `stale_survivors`) re-anchored, left out of `stale_survivors`, and passing `triage --check`'s anchor checks afterwards; an id shared with an unchosen symbol left with its owner; an id both killed and escaped in one symbol kept; a move between two chosen symbols, and the refusal when the target is unchosen; placement after the same symbol, the same file, and before the first `[[closed]]`; an unattributed mutant counted and not added; the pre-write parse-back failing without writing; idempotence; `stale_survivors` shrink.
- One end-to-end run on a scratch copy: a detached worktree at the branch commit, one anchored line of `normalizeExistingOrParent` (`internal/runtime/virtual_policy.go:325`, 12 ledger entries) rewritten and committed there, a full report-only run (no narrowing exists, see Context), then `triage --check` fails ("mutated code gone"), `refresh --symbol normalizeExistingOrParent`, `triage --check` fails on the untriaged placeholders (and evidence) and on nothing else for that symbol (re-anchored entries pass), reruns twice, a written triage, `triage --check` passes. The worktree is removed afterwards; nothing of it is committed.

## Risks / Trade-offs

- [The baseline `line` of a new entry may not match what go-mutesting would write for the same mutant] → go-mutesting matches by id only (archived design, Context); the line is informational. The calibration compares the added entry with a `baseline-update`-style group baseline when one is at hand, otherwise records it as unverified.
- [A refactor that renames or deletes a bound function] → Its row is updated first (the plan fails otherwise); its old entries then fail "symbol missing", so `--stale` or `--symbol FILE:OLD` makes it removal-only (D3); the new name is refreshed with `--symbol`, and ids that moved come back untriaged (D4). The docs say so.
- [Moved and re-attributed ids lose their written triage] → Accepted: a moved id is a mutant of different code, so it is triaged again; re-anchored ids keep theirs.
- [The full run costs ~35 minutes per refresh] → Accepted for now; see Open questions.
- [Text-level ledger edits can corrupt TOML] → The parse-back check of D4 fails closed before writing.
- [Kept entries of a rewritten function keep evidence at the old top-level digest] → Unchanged from today's rule; the full retriage owns it.

## Migration Plan

Tooling and docs only; no committed data is refreshed by this change. Rollback: revert the commit. Runs made before it lack `inputs_digest` and `group_status[<n>]`; the refresh asks for a new run.

## Maintainer decisions

- 2026-11-19, decision 42 (advisor-reviewed): the partial refresh becomes a committed `formal_mutation.py` subcommand with a make target, tests, docs and a spec delta; it validates rerun evidence rather than driving reruns; the change-flow template wording is a separate PR.

## Open Questions

- A focused formal-bindings run (one plan function or file) would cut each refresh from ~35 minutes to the function's own mutants. It needs a plan filter whose digest still covers the full inputs list. It changes neither this subcommand nor its specs, so it can follow separately.
- The ledger header comment (`tools/mutation/triage/formal-bindings.toml:24-26`) still says anchor lines match "whitespace-trimmed", while the check is verbatim. Fixing it is outside this brief; it is listed for the maintainer.
