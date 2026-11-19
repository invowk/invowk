## MODIFIED Requirements

### Requirement: Formal-bindings baseline and triage ledger
Accepted formal-bindings survivors SHALL be stored in a baseline separate from the root baseline. Each SHALL be classified in a checked triage ledger as `equivalent`, `abstraction`, `binding-gap`, `model-gap`, or `defect`, identified by model and element, and anchored to the code it mutates by the `removed` and `added` lines of its diff (plus, for a mutant that only inserts lines, the `context` line it inserts after). The triage check SHALL run in `make test-scripts` and in `mutation-formal-baseline-update`.

#### Scenario: Ledger matches baseline
- **WHEN** the triage check runs
- **THEN** it SHALL fail unless the set of mutant IDs in `tools/mutation/baselines/formal-bindings-baseline.json` equals the set of survivor IDs in `tools/mutation/triage/formal-bindings.toml`

#### Scenario: Gaps and defects carry a follow-up
- **WHEN** a ledger entry is classified `binding-gap` or `model-gap`
- **THEN** the check SHALL fail unless the entry names a follow-up change or task
- **WHEN** a ledger entry is classified `defect`
- **THEN** the check SHALL fail unless its follow-up names a finding id declared by a `finding` command in `formal/manifest.toml`

#### Scenario: Abstractions are declared in the named row
- **WHEN** a ledger entry is classified `abstraction`
- **THEN** the check SHALL fail unless, after whitespace is collapsed, its reason is a substring of the abstraction cell of the correspondence row identified by the entry's `model` and `element`

#### Scenario: Baseline update enforces the ledger
- **WHEN** maintainers run `mutation-formal-baseline-update`
- **THEN** the wrapper SHALL run the triage check after writing the baseline, and SHALL exit non-zero while any baseline ID lacks a valid ledger entry

#### Scenario: IDs are executor-independent
- **WHEN** the same mutation is produced under the formal-bindings executor and under the built-in executor
- **THEN** both SHALL report the same stable mutant ID

#### Scenario: Anchor text matches the id
- **WHEN** the triage check reads a ledger entry that records `removed` and `added`
- **THEN** it SHALL fail, naming the id and symbol, unless go-mutesting's stable-id hash of the entry's file, the baseline's mutator for that id, and those lines equals the entry's id

#### Scenario: Survivor whose mutated code is gone
- **WHEN** the entry's symbol no longer names a function in its file, or the entry's anchor lines (its `removed` lines, or its `context` line when it removes none), matched verbatim with their indentation, no longer appear in order in the current file starting at a line inside that function's body, with every later code line inside that body too (a later comment line may follow the function)
- **THEN** the triage check SHALL fail and name the entry's id, its symbol, and which of the two conditions failed

#### Scenario: Edits elsewhere in a bound file
- **WHEN** a bound file is edited so that lines move, or code outside an entry's `removed` lines changes, while every entry's anchor lines still appear in its symbol's body
- **THEN** the staleness check SHALL pass

#### Scenario: Unanchored entry
- **WHEN** a ledger entry lacks `removed` or `added`, or has an empty `removed` and no `context`
- **THEN** the triage check SHALL fail and name the id and symbol, unless the id is listed in `[triage] stale_survivors` or `[triage] orphaned_survivors`

#### Scenario: Orphaned survivors are not anchored
- **WHEN** a ledger entry's id is listed in `[triage] orphaned_survivors`
- **THEN** the triage check SHALL NOT require or check its anchor, because the plan no longer targets its function

#### Scenario: Staleness check is static
- **WHEN** the triage check runs, with or without `--base`
- **THEN** it SHALL read only the current tree's Go sources, the ledger, the baseline, and `formal/manifest.toml` to decide staleness: it SHALL NOT run a mutation, `go list`, or the Go toolchain, and SHALL NOT read git history: a depth-1 checkout SHALL give the same staleness verdict

### Requirement: Deterministic formal-bindings execution
The formal-bindings executor SHALL run with the wrapper's fixed rapid seed, disabled failure files, and bounded shrink time, and SHALL refuse to run without them. Focused reruns SHALL record their outcomes as evidence.

#### Scenario: Missing determinism settings are refused
- **WHEN** the executor is invoked without `RAPID_SEED`, without `RAPID_NOFAILFILE=1`, or without `RAPID_SHRINKTIME`
- **THEN** it SHALL exit with the errored status without running tests

#### Scenario: Reruns are recorded
- **WHEN** `mutation-formal-rerun` runs a mutant
- **THEN** it SHALL append the mutant ID, status, timestamp, and the SHA-256 digest of the plan inputs (the mutated target files and the files declaring their killer tests, sorted by path, each hashed as path, NUL, bytes) to `tools/mutation/triage/formal-bindings-reruns.jsonl`
- **THEN** a recorded commit, if any, SHALL be information only and SHALL NOT decide whether evidence counts

#### Scenario: Survivors are confirmed by evidence
- **WHEN** the triage check runs
- **THEN** it SHALL fail for any baselined ID without at least two recorded `escaped` reruns whose digest equals that mutant's evidence digest: the `digest` its baseline entry records when it has one, else the digest the baseline records

#### Scenario: Per-mutant digest after a partial refresh
- **WHEN** a change replaces some baseline entries without regenerating the whole baseline
- **THEN** each new entry SHALL record, as its own `digest`, the plan-inputs digest of its reruns, and the baseline's top-level digest and the other entries' evidence SHALL stay unchanged

#### Scenario: Full regeneration drops per-mutant digests
- **WHEN** `mutation-formal-baseline-update` writes the baseline
- **THEN** it SHALL write only the top-level digest, and no entry SHALL carry its own

#### Scenario: Evidence survives squash merges
- **WHEN** the branch that recorded the evidence is squash-merged and deleted, or the repository is freshly cloned
- **THEN** the triage check SHALL give the same verdict as on that branch, as long as the plan inputs are unchanged

### Requirement: Triage check lists binding gaps and orphaned survivors
`formal/manifest.toml` SHALL hold a `[triage]` table with exactly `binding_gaps` (model, element, symbol), `orphaned_survivors` (mutant ids) and `stale_survivors` (mutant ids). `triage --check` SHALL classify rows as the formal-bindings plan does, without `go list`, and fail on an unlisted `no-binding` row, orphaned survivor or stale survivor, and on a stale entry. A survivor is orphaned when the row it names is unbound. The reach gate SHALL NOT read the table.

#### Scenario: New binding gap
- **WHEN** a correspondence row has no binding, names a function, and is not in `[triage] binding_gaps`
- **THEN** `triage --check` SHALL fail and name the row

#### Scenario: New orphaned survivor
- **WHEN** a baselined survivor's row becomes unbound and its id is not in `[triage] orphaned_survivors`
- **THEN** `triage --check` SHALL fail and name the survivor id and its row

#### Scenario: Listed stale survivor
- **WHEN** a ledger entry fails the staleness or anchor check and its id is in `[triage] stale_survivors`
- **THEN** `triage --check` SHALL NOT fail because of it, and SHALL print it in the report with its symbol and the failed condition

#### Scenario: Stale entry
- **WHEN** a row listed in `binding_gaps` is bound, unbound by design, or names no row; or an id listed in `orphaned_survivors` is not in the ledger or its row is bound; or an id listed in `stale_survivors` is not in the ledger or passes the staleness and anchor checks
- **THEN** `triage --check` SHALL fail and name the stale entry

#### Scenario: Unknown triage key
- **WHEN** `[triage]` contains a key other than `binding_gaps`, `orphaned_survivors` or `stale_survivors`
- **THEN** manifest validation SHALL fail

#### Scenario: Missing stale list key
- **WHEN** `[triage]` has no `stale_survivors` key
- **THEN** manifest validation SHALL fail, so deleting the key cannot reopen its seeding in a later change

### Requirement: The triage lists cannot grow against the base
`triage --check` SHALL read the base commit's `formal/manifest.toml` and fail on any entry of `[correspondence] unresolved_elements`, `[triage] binding_gaps`, `[triage] orphaned_survivors`, or `[triage] stale_survivors` that the base does not list. The base SHALL be `--base`, else a non-empty `INVOWK_TRIAGE_BASE`, else the merge-base of `HEAD` and `origin/main`. A base that cannot be determined or read SHALL fail closed with a message naming the base, `INVOWK_TRIAGE_BASE`, and `--base`.

#### Scenario: Entry absent at the base
- **WHEN** the tree lists an entry in one of the four lists that the base manifest does not list
- **THEN** `triage --check` SHALL fail and name the list, the entry, and the base

#### Scenario: Shrink passes
- **WHEN** the tree lists a subset of the base's entries
- **THEN** the growth check SHALL pass

#### Scenario: Unreadable base
- **WHEN** the base cannot be determined or `git show <base>:formal/manifest.toml` fails
- **THEN** `triage --check` SHALL fail with a message that names the base and says to fetch it, set `INVOWK_TRIAGE_BASE`, or pass `--base`

#### Scenario: Empty variable
- **WHEN** `INVOWK_TRIAGE_BASE` is set to an empty string and `--base` is not given
- **THEN** `triage --check` SHALL use the merge-base, as if the variable were unset

### Requirement: The triage lists are seeded only from a base without them
The growth check SHALL treat a base manifest that has none of the first three lists (`unresolved_elements`, `binding_gaps`, `orphaned_survivors`) as the seeding change for all four: it SHALL pass and print one seeding notice per list. A base that has some but not all of those three SHALL fail for each missing list, so renaming a key cannot reopen seeding. `stale_survivors` SHALL be seeded from a base that has all three but lacks it, and SHALL fail against a base that lacks it while having only some of the three.

#### Scenario: Base that introduces no list
- **WHEN** the base manifest is readable and has none of the first three lists, as for the change that introduces them
- **THEN** the growth check SHALL pass and print a seeding notice for each list

#### Scenario: Base with only some lists
- **WHEN** the base manifest has one or two of the first three lists
- **THEN** the growth check SHALL fail and name each list the base lacks

#### Scenario: Base before the stale list
- **WHEN** the base manifest has the first three lists and no `stale_survivors`, as for the change that introduces it
- **THEN** the growth check SHALL pass for `stale_survivors` with a seeding notice, and SHALL check the first three lists for growth as usual

### Requirement: The unplanned lists only shrink
The `[triage]` lists `binding_gaps` and `orphaned_survivors` SHALL be seeded once by this change: 19 binding gaps and 20 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the lists. `stale_survivors` follows its own requirement, "The stale survivors list only shrinks".

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the lists
- **THEN** it SHALL pass, report 19 listed gaps, 20 listed orphans, and the derived by-design rows, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail

## ADDED Requirements

### Requirement: The stale survivors list only shrinks
`[triage] stale_survivors` SHALL list only baselined survivors that fail the staleness or anchor check and that the change that seeds the list does not refresh. Later changes SHALL NOT add entries. An id leaving the list SHALL be explained: it passes the check, or it left the ledger.

#### Scenario: Seeded from the refresh run
- **WHEN** the change that introduces the list lands
- **THEN** the list SHALL hold exactly the baselined ids, outside `standardVirtualAnchorsForOS` and outside `orphaned_survivors`, that the fresh report-only run could not anchor or that fail the staleness check, and SHALL be empty if there are none

#### Scenario: Stale survivor fixed since the base
- **WHEN** the base lists a stale survivor whose ledger entry now passes the staleness and anchor checks, or whose id left the ledger
- **THEN** `triage --check` SHALL pass and print a notice naming the id and the reason it left
