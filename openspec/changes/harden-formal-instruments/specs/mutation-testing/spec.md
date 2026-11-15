## ADDED Requirements

### Requirement: Triage check lists binding gaps and orphaned survivors
`formal/manifest.toml` SHALL hold a `[triage]` table with exactly `binding_gaps` (model, element, symbol) and `orphaned_survivors` (mutant ids). `triage --check` SHALL classify rows as the formal-bindings plan does, without `go list`, and fail on an unlisted `no-binding` row or orphaned survivor and on a stale entry. A survivor is orphaned when the row it names is unbound. The reach gate SHALL NOT read the table.

#### Scenario: New binding gap
- **WHEN** a correspondence row has no binding, names a function, and is not in `[triage] binding_gaps`
- **THEN** `triage --check` SHALL fail and name the row

#### Scenario: New orphaned survivor
- **WHEN** a baselined survivor's row becomes unbound and its id is not in `[triage] orphaned_survivors`
- **THEN** `triage --check` SHALL fail and name the survivor id and its row

#### Scenario: Stale entry
- **WHEN** a listed row is bound, unbound by design, or names no row, or a listed id is not in the ledger or its row is bound
- **THEN** `triage --check` SHALL fail and name the stale entry

#### Scenario: Unknown triage key
- **WHEN** `[triage]` contains a key other than `binding_gaps` or `orphaned_survivors`
- **THEN** manifest validation SHALL fail

### Requirement: By-design unbound rows are derived
Rows unbound by design (`no-symbol`, `type-symbol`, `trace-only`, `characterisation-only`) SHALL NOT be listed in `[triage]`. `triage --check` SHALL derive them with the plan's row classification and print them as a report. A row whose symbol names no function SHALL be `type-symbol` whether or not it has a binding.

#### Scenario: New by-design row passes
- **WHEN** a new characterisation-only or type-symbol row is added
- **THEN** `triage --check` SHALL report it and SHALL NOT fail because of it

#### Scenario: Type symbol without a binding
- **WHEN** a row names a type, field, or interface, such as `Serverbase` `State`, and has binding `-`
- **THEN** it SHALL be classified as `type-symbol`, not as a binding gap

#### Scenario: Method symbol names a function
- **WHEN** a row's symbol is a `Type.Method` that the file declares as a method
- **THEN** it SHALL be classified as a function, not as `type-symbol`

### Requirement: The triage lists cannot grow against the base
`triage --check` SHALL read the base commit's `formal/manifest.toml` and fail on any entry of `[correspondence] unresolved_elements`, `[triage] binding_gaps`, or `[triage] orphaned_survivors` that the base does not list. The base SHALL be `--base`, else a non-empty `INVOWK_TRIAGE_BASE`, else the merge-base of `HEAD` and `origin/main`. A base that cannot be determined or read SHALL fail closed with a message naming the base, `INVOWK_TRIAGE_BASE`, and `--base`.

#### Scenario: Entry absent at the base
- **WHEN** the tree lists an entry in one of the three lists that the base manifest does not list
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
The growth check SHALL treat a base manifest that has none of the three lists as the seeding change: it SHALL pass and print one seeding notice per list. A base that has some but not all of the three lists SHALL fail for each missing list, so renaming a key cannot reopen seeding.

#### Scenario: Base that introduces no list
- **WHEN** the base manifest is readable and has none of the three lists, as for the change that introduces them
- **THEN** the growth check SHALL pass and print a seeding notice for each list

#### Scenario: Base with only some lists
- **WHEN** the base manifest has one or two of the three lists
- **THEN** the growth check SHALL fail and name each list the base lacks

### Requirement: Rows bound at the base stay bound or leave visibly
`triage --check` SHALL classify the base commit's correspondence tables, read with the model and Go sources at the base, as the plan does. It SHALL fail when a row bound at the base is unbound by design in the tree: the same (model, element, symbol) row, or every row with its model and element, is `no-symbol`, `type-symbol`, `trace-only`, or `characterisation-only`. Other departures SHALL be notices. A base with none of the three lists SHALL report demotions as notices only.

#### Scenario: Bound row demoted to characterisation
- **WHEN** a row bound at the base gets an abstraction cell beginning `characterisation:`, or only trace-harness or characterisation bindings
- **THEN** `triage --check` SHALL fail and name the row, its by-design reason, and the base

#### Scenario: Bound row deleted
- **WHEN** a row bound at the base is deleted from its table
- **THEN** `triage --check` SHALL pass and print a notice that the row was deleted

### Requirement: Unresolved rows cannot grow against the base
`triage --check` SHALL fail on a row in the tree whose `"<model>: <element>"` pair is listed in `[correspondence] unresolved_elements` and whose (model, element, symbol) row the base tables do not have, so no new row enters an unresolved pair. A base with none of the three lists SHALL report such rows as notices only.

#### Scenario: New row under a listed pair
- **WHEN** a row is added under a listed unresolved pair, such as a second `readOp` row naming `IsModule`
- **THEN** `triage --check` SHALL fail and name the row and the base

### Requirement: Entries that leave a triage list are explained
When the base lists an entry that the tree does not, `triage --check` SHALL print a notice naming the list, the entry, and why it left: for a binding gap, its row is bound, reclassified by design, or deleted; for an orphan, its row is bound or its survivor left the ledger; for an unresolved element, it resolves or its row is deleted.

#### Scenario: Gap bound since the base
- **WHEN** the base lists a binding gap whose row the tree binds
- **THEN** `triage --check` SHALL pass and print a notice that the entry left because the row is bound

### Requirement: The triage checks run in the formal lane
The formal-verification workflow SHALL run `python3 scripts/test_formal_mutation.py` and `python3 scripts/formal_mutation.py triage --check` under the `affected` condition, with `INVOWK_TRIAGE_BASE` set by `scripts/ci-triage-base.sh` in the same step. The triage ledger, the formal-bindings baseline, and the helper SHALL be in `[ci] paths`.

#### Scenario: Workflow wiring is checked
- **WHEN** `scripts/test_formal.py` checks the workflows
- **THEN** it SHALL require both commands, the `affected` condition, and the helper's output exported as `INVOWK_TRIAGE_BASE` before `triage --check` and before each `make test-scripts` in `lint.yml` and `release.yml`

### Requirement: The triage base is resolved in one place
`scripts/ci-triage-base.sh` SHALL print the base as a full SHA on stdout and notices on stderr. On `pull_request` it SHALL fetch the base branch tip at depth 1 and print it, exiting non-zero when the fetch fails. On other events it SHALL print the commit under test with a notice that the base equals HEAD.

#### Scenario: Pull-request base
- **WHEN** the helper runs on a `pull_request` event in a depth-1 checkout
- **THEN** it SHALL fetch the base branch tip and print its full SHA

#### Scenario: Base reaches the gated suite
- **WHEN** `INVOWK_TRIAGE_BASE` names a commit that only the real checkout's object store holds and `make test-scripts` runs
- **THEN** the suite inside the throwaway repository SHALL see the variable, and `git show <base>:formal/manifest.toml` SHALL resolve there

### Requirement: The unplanned lists only shrink
The `[triage]` lists SHALL be seeded once by this change: 19 binding gaps and 20 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the lists.

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the lists
- **THEN** it SHALL pass, report 19 listed gaps, 20 listed orphans, and the derived by-design rows, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail
