# Proposal

## Why

`require-absolute-base-dirs` (S4, #172) rewrote `standardVirtualAnchorsForOS`, so the 22 formal-bindings survivors baselined for it mutate code that no longer exists (`getenv`, `firstNonEmpty`, the darwin case). `triage --check` still passes. It compares the baseline's digest with the rerun records and never looks at the current tree, so a green check said nothing about a rewritten function body. This change replaces the stale entries from a fresh run and makes the check fail when an entry no longer describes a mutant of the current code (maintainer decision 35).

## What Changes

- Replace the 22 stale `standardVirtualAnchorsForOS` entries in `tools/mutation/triage/formal-bindings.toml` and `tools/mutation/baselines/formal-bindings-baseline.json` with the survivors of a fresh report-only `make mutation-formal` run on origin/main 567e7b00. The S4 run expects 5 survivors, all in the `windows` case; the fresh run decides the final set.
- Triage the new survivors as `binding-gap`. The reason names the Linux-only mutation lane and `TestStandardVirtualAnchorsMatchLegacy`. That test stays a unit test and does not become a killer of the formal-bindings profile.
- Give each `[[survivor]]` ledger entry the mutant's own source text: the `removed` and `added` diff lines that go-mutesting hashes into the stable id. The fresh run backfills them for every baselined survivor it regenerates.
- Add a per-entry staleness check to `triage --check`:
  - The recorded text must hash, with the baseline's file and mutator, to the entry's id.
  - The `removed` lines must still appear, in order, in the current body of the entry's symbol, and the symbol must exist.
  - The check fails on each stale entry and names its id and symbol. It needs no mutation run and no `go list`, and it also runs under `--base`.
- Let a baseline mutant record its own rerun-evidence `digest`. Replacing 22 entries then does not re-stamp the baseline's single digest, which would invalidate the evidence of the ~397 other survivors.
- Add a shrink-only `[triage] stale_survivors` list to `formal/manifest.toml`. It holds survivors that the check flags, or that the fresh run cannot anchor, outside the anchors refresh, so the check can land without fixing them here. A base that already has the first three `[triage]` lists seeds it.
- Update the counts and the "Retriaged by require-absolute-base-dirs" text in `tasks/next/formal-binding-gaps.md`, and the ledger and manifest header comments.

Out of scope, recorded in the Future work section of `tasks/next/formal-binding-gaps.md`:
- triage of the other ~397 survivors, and the full baseline refresh at a new digest;
- re-pointing the `Auth` row;
- a `normalizeZIPPath` row;
- per-OS anchors in `VirtualPathHarness.als`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mutation-testing`:
  - "Formal-bindings baseline and triage ledger" gains the per-entry staleness check and the `removed`/`added` anchor fields.
  - "Deterministic formal-bindings execution" accepts a per-mutant evidence digest.
  - "Triage check lists binding gaps and orphaned survivors", "The triage lists cannot grow against the base" and "The triage lists are seeded only from a base without them" gain the `stale_survivors` list and its seeding rule.

## Impact

- `scripts/formal_mutation.py`: the triage check, `SURVIVOR_KEYS`, evidence-digest lookup and growth lists. `scripts/formal.py` gets manifest validation of the new `[triage]` key.
- `scripts/test_formal_mutation.py` and `scripts/test_formal.py`: unit tests.
- Data: `tools/mutation/triage/formal-bindings.toml`, `tools/mutation/baselines/formal-bindings-baseline.json`, `tools/mutation/triage/formal-bindings-reruns.jsonl` and `formal/manifest.toml`.
- Docs: `tasks/next/formal-binding-gaps.md`, `formal/README.md`, `.agents/runbooks/mutation-testing.md` and `.agents/skills/formal-verification/SKILL.md`, where they describe the ledger fields or `[triage]`.
- No Go production code changes. No user-facing behavior, CLI or schema changes. Not breaking.
- go-mutesting reads the baseline with `json.Unmarshal` (`internal/baseline/baseline.go:71` in v2.8.3), which ignores the extra per-mutant `digest` field.
