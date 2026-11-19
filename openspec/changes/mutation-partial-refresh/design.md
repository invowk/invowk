# Design

## Context

See proposal.md, Why. The current state that shapes the approach:

- **Ledger and baseline.** `tools/mutation/triage/formal-bindings.toml` holds 403 `[[survivor]]` entries and some `[[closed]]` entries after them (first `[[closed]]` at line 6490). `tools/mutation/baselines/formal-bindings-baseline.json` is `json.dumps(data, indent=2) + "\n"` byte for byte (checked on c6b32b89), sorted by file, line, mutator, id (`scripts/formal_mutation.py:680`). `merge_baselines` writes only go-mutesting's four fields plus the top-level digest (`scripts/formal_mutation.py:668-692`).
- **Anchors and staleness.** `anchor_problem` (`scripts/formal_mutation.py:805-832`) returns the failed condition for one entry, reading only the current tree through `source_reader` (`:835-861`). `check_triage` calls it for every non-orphan entry (`:1190-1209`) and fails on an unknown class with a generic message (`:1214-1215`).
- **Evidence.** A mutant's evidence digest is its own baseline `digest`, else the top-level one; two `escaped` reruns at that digest are required (`scripts/formal_mutation.py:1237-1258`). `rerun-record` stamps `plan_digest` of the plan built at rerun time (`:1563-1567`); `plan_digest` hashes the plan's `inputs` read from `plan["root"]` (`:416-440`).
- **Run reports.** `make mutation-formal` (`Makefile:275-276`) runs `run_formal_profile` (`scripts/mutation.sh:944-1012`): plan, pre-flight, one go-mutesting call per group, exit restore, then `merge-reports`. `merge_reports` (`scripts/formal_mutation.py:695-743`) writes the merged `report.json` with `killed`, `escaped`, `skipped` and `errored` lists, and appends to `run-metadata.txt`; the plan stays at `<report-dir>/formal-plan.json`. Reported paths are absolute under the run's root, which `merge_reports` strips with `REPO_ROOT` (`:723-727`). `--baseline` only gates the exit status: `report.json` still lists baselined escapes (formal-verification skill, `.agents/skills/formal-verification/SKILL.md:228-232`). The archived run confirms it: of 419 baselined ids, 350 came back escaped, 30 killed and 39 not generated (scratchpad `analysis-new.json` of refresh-anchors-mutation-ledger).
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

### D2. Run validation and the recorded digest
`merge_reports` appends `inputs_digest=<plan_digest(plan)>` to `run-metadata.txt`, computed from the plan in the report dir after the exit restore, so it is the digest of the tree the run mutated. The refresh:
1. loads `<report-dir>/formal-plan.json` and the recorded `inputs_digest` (fails when absent: "rerun `make mutation-formal`");
2. builds the current plan (`build_plan`, which runs `go list`; the refresh is not the static triage check) and requires its `inputs` to equal the run plan's and its digest, read from the current tree, to equal the recorded one;
3. resolves reported paths by stripping the run plan's `root`, so a run in a throwaway worktree at the same inputs is accepted.

The new entries' per-mutant `digest` is that digest. It is exactly what `rerun-record` will stamp for a rerun on this tree, so the evidence check lines up without copying digests by hand.

Alternatives considered: reading the inputs from the run's root (the throwaway worktree may be gone); trusting the run blindly (a run on older inputs would add mutants of code that no longer exists).

### D3. Attribution and choice of symbols
A reported mutant belongs to the plan function whose `[start, end]` contains its `mutator.originalStartLine` in its file (the run plan's ranges, equal to the current tree's by D2). `--symbol` resolves against the current plan's functions: a bare name must match exactly one; `FILE:SYMBOL` disambiguates. `--stale` chooses the symbols of every ledger entry, orphans excluded, for which `anchor_problem` returns a problem, which includes ids in `[triage] stale_survivors`. A chosen symbol that still names a function in its file but is not planned (its row is unbound, as for an orphan) fails the whole command before any write. A symbol that no longer names a function in its file (deleted or renamed) is removal-only: all its entries are removed and nothing is added, and the summary says so; the renamed function's new name is refreshed with `--symbol`.

### D4. What the refresh writes
For each chosen symbol, with `E` = the run's escaped ids in it (deduplicated by id, first by line, as `merge_baselines` does at `:684-690`):
- **remove** every baseline/ledger entry of the symbol whose id is not in `E`, reporting why (not generated, or the run's status); remove the id from `[triage] stale_survivors` too. A ledger entry's symbol is the ledger's `symbol` field; a baseline entry without a ledger entry is not touched (the triage check already reports it).
- **keep** every entry whose id is in `E` and baselined: no byte changes.
- **add** every id in `E` that is not baselined: baseline `{id, file, mutator, line, digest}` (line = `originalStartLine`), inserted at its sort position; ledger entry `id, file, symbol, model, element, class, reason, follow_up, removed, added[, context]`.
  - `model` and `element` come from the plan function's first row (sorted); when the function has several rows, the reason placeholder lists the others so the person triaging picks the right one.
  - `class = "untriaged"`, `reason = "UNTRIAGED: <mutator> (<agentic description>) survives <killers>; write the class and the reason"`, `follow_up = ""`.
  - It is inserted after the last ledger entry of the same symbol, else of the same file, else before the first `[[closed]]` (else at the end).
  - An id in `E` that the baseline already lists under another symbol fails the command, naming both, instead of moving it.

Writing is text-level for the ledger and the manifest (split on `[[`-blocks as `apply.py` does, and delete single `"<id>",` lines from the `stale_survivors` array), and `json.dumps(indent=2) + "\n"` for the baseline. Before writing, the refresh checks that the baseline round-trips byte-identically through that serializer and that the new ledger text parses back (with `tomllib`) to the old entries plus the added ones minus the removed ones, with each new anchor equal to the run's diff lines; otherwise it fails without writing.

Alternatives considered: a TOML writer for the whole ledger (would reformat 6000 lines and break byte-identity); dropping removed ids from `stale_survivors` by hand (the triage check would then fail on a stale entry, a trap for every user).

### D5. The untriaged placeholder
`UNTRIAGED_CLASS = "untriaged"` and `UNTRIAGED_MARK = "UNTRIAGED:"`. `check_triage` fails, before the class check, for an entry whose class is the placeholder or whose reason starts with the mark: "survivor <id> (<symbol>): untriaged placeholder from `refresh`: write the class (one of …) and the reason". The class stays out of `TRIAGE_CLASSES`, so the generic unknown-class failure cannot be bypassed either. Checking both fields means setting only the class still fails.

Alternatives considered: an empty reason (already fails, but with a message that does not say what to do, and an empty class string would also need a new rule); a separate `untriaged = true` field (one more key to keep in `SURVIVOR_KEYS` forever).

### D6. Evidence: validate, never run
After writing, the refresh applies the evidence rule of `check_triage` (`:1237-1258`) to the chosen symbols' remaining entries, reusing the same code (factored into a helper both call). Ids with fewer than two escaped reruns at their digest are printed with `make mutation-formal-rerun MUTATION_MUTANT_ID=<id,id,...>` and "run it twice". Mutation runs take minutes to an hour and need the heavy lock and a clean tracked tree; driving them from a data-edit command would hide that cost and could not be unit-tested. Since the baseline and ledger are on the wrapper's allow-list (`scripts/mutation.sh:329-331`), the reruns can follow the refresh without a commit, but `formal/manifest.toml` is not: when the refresh changed it, the summary says to commit before rerunning.

Exit status: 0 when the refresh wrote (or found nothing to change); 1 on any refusal. Outstanding triage and evidence are reported, and `triage --check` is the gate. Running the refresh again after the reruns reports zero awaiting evidence (it is idempotent).

### D7. Summary
One line per removed, added and kept id (id, symbol, mutator, line, and for removals the reason), then `refresh: removed N, added N, untriaged N, kept N, awaiting evidence N`. "Untriaged" counts all entries of the chosen symbols that still carry the placeholder, not only the ones just added.

### D8. Make target
`make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=<sym>[,<sym>...]` or `MUTATION_FORMAL_STALE=1` (both allowed), passing `--report-dir "$(MUTATION_REPORT_DIR)/full/formal-bindings"`. With neither, it prints usage and exits 1, like `mutation-formal-rerun` (`Makefile:281-285`). It runs Python only, so it is not a heavy command.

### D9. Calibration
- Unit tests in `scripts/test_formal_mutation.py` with fixture roots and synthetic `report.json`/plan/metadata: removed id dropped; new escapee added and rejected by `triage --check` until triaged and evidenced, then accepted; unrelated entries byte-identical; killed id not added; inputs-digest mismatch refused; unplanned symbol refused; idempotence; `stale_survivors` shrink.
- One end-to-end run on a scratch copy: a detached worktree at the branch commit, one anchored line of `normalizeExistingOrParent` (`internal/runtime/virtual_policy.go:325`, 12 ledger entries) rewritten and committed there, a full report-only run (no narrowing exists, see Context), then `triage --check` fails ("mutated code gone"), `refresh --symbol normalizeExistingOrParent`, `triage --check` fails on the untriaged placeholders (and evidence), reruns twice, a written triage, `triage --check` passes. The worktree is removed afterwards; nothing of it is committed.

## Risks / Trade-offs

- [The baseline `line` of a new entry may not match what go-mutesting would write for the same mutant] → go-mutesting matches by id only (archived design, Context); the line is informational. The calibration compares the added entry with a `baseline-update`-style group baseline when one is at hand, otherwise records it as unverified.
- [A refactor that renames or deletes a bound function] → Its old entries fail "symbol missing", so `--stale` chooses the old name as removal-only (D3); the new name needs `--symbol` (and a correspondence-row update, which the plan then targets). The docs say so.
- [The full run costs ~35 minutes per refresh] → Accepted for now; see Open questions.
- [Text-level ledger edits can corrupt TOML] → The parse-back check of D4 fails closed before writing.
- [Kept entries of a rewritten function keep evidence at the old top-level digest] → Unchanged from today's rule; the full retriage owns it.

## Migration Plan

Tooling and docs only; no committed data is refreshed by this change. Rollback: revert the commit. Runs made before it lack `inputs_digest`; the refresh asks for a new run.

## Maintainer decisions

- 2026-11-19, decision 42 (advisor-reviewed): the partial refresh becomes a committed `formal_mutation.py` subcommand with a make target, tests, docs and a spec delta; it validates rerun evidence rather than driving reruns; the change-flow template wording is a separate PR.

## Open Questions

- A focused formal-bindings run (one plan function or file) would cut each refresh from ~35 minutes to the function's own mutants. It needs a plan filter whose digest still covers the full inputs list. It changes neither this subcommand nor its specs, so it can follow separately.
- The ledger header comment (`tools/mutation/triage/formal-bindings.toml:24-26`) still says anchor lines match "whitespace-trimmed", while the check is verbatim. Fixing it is outside this brief; it is listed for the maintainer.
