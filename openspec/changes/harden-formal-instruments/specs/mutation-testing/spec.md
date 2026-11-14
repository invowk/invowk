## ADDED Requirements

### Requirement: Triage check lists unplanned rows and orphaned survivors
`formal/manifest.toml` SHALL hold a `[triage]` table with exactly `unbound_rows` (model, element, symbol) and `orphaned_survivors` (mutant ids). `triage --check` SHALL classify rows as the formal-bindings plan does, without `go list`. It SHALL fail on an unbound row or orphaned survivor that is not listed, and on a stale entry. A survivor is orphaned when the row it names (model, element, symbol) is unbound. The reach gate SHALL NOT read the table.

#### Scenario: New unbound row
- **WHEN** a correspondence row becomes unbound and is not in `[triage] unbound_rows`
- **THEN** `triage --check` SHALL fail and name the row

#### Scenario: New orphaned survivor
- **WHEN** a baselined survivor's row becomes unbound and its id is not in `[triage] orphaned_survivors`
- **THEN** `triage --check` SHALL fail and name the survivor id and its row

#### Scenario: Stale entry
- **WHEN** a listed row is bound or names no row, or a listed id is not in the ledger or its row is bound
- **THEN** `triage --check` SHALL fail and name the stale entry

#### Scenario: Unknown triage key
- **WHEN** `[triage]` contains a key other than `unbound_rows` or `orphaned_survivors`
- **THEN** manifest validation SHALL fail

### Requirement: The unplanned list only shrinks
The `[triage]` list SHALL be seeded once, after this change unbinds `Auth`, `unpackRejectsEscape`, and `anchors`: 54 unbound rows and 42 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12, `anchors` 22). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the list.

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the list
- **THEN** it SHALL pass, report 54 listed rows and 42 listed orphans, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail
