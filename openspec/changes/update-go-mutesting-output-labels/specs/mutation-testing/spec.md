## ADDED Requirements

### Requirement: Mutation terminal labels are current and unambiguous
Invowk SHALL document and interpret mutation-testing terminal output labels according to the pinned mutation-testing tool version.

#### Scenario: Current tool output uses explicit labels
- **WHEN** Invowk uses a mutation-testing tool version whose terminal output labels killed mutants as `KILLED` and surviving mutants as `ESCAPED`
- **THEN** current documentation and agent guidance MUST use `KILLED` for killed mutants
- **AND** current documentation and agent guidance MUST use `ESCAPED` for surviving mutants

#### Scenario: Historical output labels are version-scoped
- **WHEN** historical triage notes refer to older mutation-testing output where `PASS` meant killed and `FAIL` meant escaped
- **THEN** those notes MUST clearly identify that the label interpretation applies to the older tool version that produced the evidence
- **AND** those notes MUST NOT present `PASS` and `FAIL` as the current mutation-testing terminal labels

#### Scenario: Automation relies on machine-readable reports
- **WHEN** mutation-testing gates decide whether a run passes or fails
- **THEN** they MUST use machine-readable report fields or stable mutant IDs when available
- **AND** they MUST NOT introduce new parsing of human terminal status labels unless no machine-readable alternative exists
