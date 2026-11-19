# Proposal

## Why

Since #181 (c6b32b89), `triage --check` fails when a formal-bindings survivor's anchored code is gone, and `[triage] stale_survivors` can only shrink. So a change that rewrites a bound function must refresh that function's ledger and baseline entries in the same PR (design D7 of refresh-anchors-mutation-ledger). Today that refresh is done by hand, with two uncommitted scratch scripts and per-mutant digests copied into the baseline JSON. The next formal-verification changes (F13+F17, F18, F14, F15) rewrite bound functions with 3 to 15 ledger entries each, so the hand procedure would be repeated four times and is easy to get subtly wrong. The archived design left this as its open question; maintainer decision 42 answers it: make the refresh a committed subcommand.

## What Changes

- New `scripts/formal_mutation.py refresh` subcommand. It reads a report-only formal-bindings run (`make mutation-formal`) and refreshes the survivors of named symbols (`--symbol [FILE:]SYMBOL`, repeatable), of every symbol with an entry that fails the staleness or anchor check (`--stale`), or both:
  - removes the ledger entries of those symbols (and their baseline entries) whose ids the run reports escaped nowhere, and moves an id to another chosen symbol when it escapes only there;
  - adds every escaped id of those symbols that is not baselined, with its anchor (`removed`, `added`, `context`), its own evidence `digest`, and an untriaged ledger placeholder;
  - leaves every other entry byte-identical, except that a kept entry whose anchor no longer matches gets its anchor keys rewritten from the run, and every id of a chosen symbol leaves `[triage] stale_survivors`; ids shared with a symbol that was not chosen stay with their owner;
  - checks the rerun evidence of the symbols' entries and names the `make mutation-formal-rerun` command for the ids that still lack it;
  - prints a summary: removed, moved, added, untriaged, kept, re-anchored, shared, unattributed, and awaiting evidence.
- `triage --check` rejects the untriaged placeholder with its own message until a person writes the class and the reason, so an untriaged survivor cannot pass.
- `merge-reports` records the run's plan-inputs digest and `run_formal_profile` each group's exit status in `run-metadata.txt`; the refresh refuses a run whose inputs differ from the current tree's or that is incomplete (a group without reports or with a failing status).
- New `make mutation-formal-refresh` target, listed in `.agents/rules/commands.md`.
- Tests in `scripts/test_formal_mutation.py`; docs in `formal/README.md`, `.agents/runbooks/mutation-testing.md` and the formal-verification skill's survivor feedback loop, where the refresh becomes the documented same-change step.
- No new mutation profile: the refresh consumes the existing full report-only run. The tooling cannot narrow a run to one function, so each refresh still needs the ~35-minute full run (see design, Open questions).
- Out of scope: the full ~400-survivor retriage, and the change-flow template wording (`.agents/workflows/change-flow.js`, `.agents/runbooks/change-flow.md`), which a separate PR updates.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mutation-testing`: adds the formal-bindings partial-refresh requirement (scope and attribution, removals and moves, additions, byte-identical other entries and re-anchored kept entries, complete run of the current inputs, evidence check, summary) and the rejection of untriaged placeholders by the triage check.

## Impact

- Code: `scripts/formal_mutation.py` (new subcommand, a placeholder rule in `check_triage`, the digest line in `merge_reports`), `scripts/mutation.sh` (the per-group status line), `scripts/test_formal_mutation.py` and a script test for the status line, `Makefile`.
- Data touched only when a maintainer runs the refresh: `tools/mutation/baselines/formal-bindings-baseline.json`, `tools/mutation/triage/formal-bindings.toml`, and `formal/manifest.toml` `[triage] stale_survivors` (refreshed ids leave it). This change does not run it on the committed data.
- Docs: `.agents/rules/commands.md`, `.agents/runbooks/mutation-testing.md`, `.agents/skills/formal-verification/SKILL.md`, `formal/README.md`.
- No Go code, no user-facing behavior, no dependency change. Not breaking.
