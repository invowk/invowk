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

### Requirement: The triage lists cannot grow against the base
`triage --check --base <sha>` SHALL read the base commit's `formal/manifest.toml` and fail on any entry of `[correspondence] unresolved_elements`, `[triage] binding_gaps`, or `[triage] orphaned_survivors` that the base does not list. Without `--base`, the base SHALL be the merge-base of `HEAD` and `origin/main`. A base that cannot be read SHALL fail closed with a message naming the base and `--base`.

#### Scenario: Entry absent at the base
- **WHEN** the tree lists an entry in one of the three lists that the base manifest does not list
- **THEN** `triage --check` SHALL fail and name the list, the entry, and the base

#### Scenario: Shrink passes
- **WHEN** the tree lists a subset of the base's entries
- **THEN** the growth check SHALL pass

#### Scenario: Base that introduces no list
- **WHEN** the base manifest is readable but has no such list, as for the change that introduces it
- **THEN** the growth check SHALL pass for that list and print a seeding notice

#### Scenario: Unreadable base
- **WHEN** the base cannot be determined or `git show <base>:formal/manifest.toml` fails
- **THEN** `triage --check` SHALL fail with a message that names the base and says to pass `--base` or fetch the base

### Requirement: The triage checks run in the formal lane
The formal-verification workflow SHALL run `python3 scripts/test_formal_mutation.py` and `python3 scripts/formal_mutation.py triage --check --base <base>` under the `affected` condition. On a pull request the base SHALL be the PR base SHA; on other events it SHALL be the run's own commit. The triage ledger and the formal-bindings baseline SHALL be in `[ci] paths`.

#### Scenario: Workflow wiring is checked
- **WHEN** `scripts/test_formal.py` checks the workflow
- **THEN** it SHALL require both commands, the `affected` condition, and the PR base SHA passed as `--base`

### Requirement: The unplanned lists only shrink
The `[triage]` lists SHALL be seeded once by this change: 19 binding gaps and 20 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the lists.

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the lists
- **THEN** it SHALL pass, report 19 listed gaps, 20 listed orphans, and the derived by-design rows, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail
