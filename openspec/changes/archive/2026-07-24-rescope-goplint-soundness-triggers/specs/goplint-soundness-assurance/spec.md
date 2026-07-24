## ADDED Requirements

### Requirement: Completion record identity separates semantic content from prose
The retained completion record SHALL bind the proof tree through two reviewed digests — a semantic-content digest covering every path class that any gate executes or reads, and a prose digest covering documentation-class paths — so that evidence validity follows the content that produced it.

#### Scenario: Aggregate evidence binds only to semantic content
- **WHEN** a completion record is generated or re-bound
- **THEN** the retained aggregate report MUST be attributable to the semantic-content digest alone
- **THEN** re-binding MUST NOT modify, truncate, or re-attribute any retained report, observation, population, or mutation attribution

#### Scenario: Re-binding is rejected when semantic content drifted
- **WHEN** re-binding is requested and the current semantic-content digest differs from the retained one
- **THEN** re-binding MUST fail closed and require full regeneration with fresh gate execution
- **THEN** the failure MUST name the drifted semantic paths

#### Scenario: Record format migration stays visible
- **WHEN** a verifier encounters a retained record in the previous single-digest format
- **THEN** it MUST reject the record with an explicit migration notice rather than silently accepting or reinterpreting it

### Requirement: Harness-tier assurance proves orchestration integrity
Changes confined to gate orchestration SHALL be proven by evidence-preservation obligations rather than semantic population re-execution: the module test suites, one shared repository audit, and normalized-report parity between the serial reference and parallel executors over a reviewed fixture.

#### Scenario: Parity check guards evidence handling
- **WHEN** the harness tier runs
- **THEN** the serial-reference and parallel executors MUST produce byte-identical normalized reports for the reviewed fixture manifest
- **THEN** any divergence, lost observation, or population mismatch MUST fail the tier

#### Scenario: Harness tier never substitutes for semantic assurance
- **WHEN** a diff also touches any analyzer-semantics path, or the event is a schedule, release, completion, or exhaustive dispatch
- **THEN** the harness tier MUST NOT be selected as the final assurance answer
- **THEN** the applicable semantic or completion profile MUST run

### Requirement: Task-ledger completion expectations have one reviewed source
The reviewed completion plan SHALL be the single source of truth for expected task-ledger names, paths, dependency order, and permitted pending identifiers; no code constant, schema constant, or fixture MAY duplicate those expected values.

#### Scenario: Closing a change edits one reviewed file
- **WHEN** a change's task-ledger expectation transitions, including archiving the change directory
- **THEN** only the reviewed completion plan MUST require editing
- **THEN** structural validation — dependency ordering, archived-predecessor policy, well-formed pending sets — MUST remain enforced by code and schema without embedding the expected values

#### Scenario: Ledger validation stays fail closed
- **WHEN** the live repository task ledgers differ from the reviewed plan's expectations
- **THEN** evidence generation and verification MUST fail with the exact ledger, expected state, and observed state
- **THEN** the mismatch MUST NOT be baselinable or exceptable
