## ADDED Requirements

### Requirement: Correspondence binding cells may name several tests
A correspondence row's binding cell SHALL accept a comma-separated list of Go test names, and the correspondence check SHALL validate each name. This supersedes the singular "binding test" wording of the model-to-code correspondence record.

#### Scenario: Every listed binding exists
- **WHEN** a binding cell lists `TestA, TestB`
- **THEN** the correspondence check SHALL fail if either test is not declared in the repository, and SHALL name the model and row

#### Scenario: Multi-test cell satisfies the correspondence record
- **WHEN** a row's binding cell names several tests
- **THEN** the row SHALL satisfy the correspondence-record requirement's binding-test column exactly as a single-test cell does

### Requirement: Binding-test completeness
Every Go test function declared in a `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, or `*_trace_test.go` file SHALL be accounted for:
- A `Test*_TraceHarness` function SHALL belong to a package declared in a `[[trace]]` suite.
- Every other test SHALL appear in the binding cell of some correspondence row.

#### Scenario: Untabled binding test
- **WHEN** a scanned file declares a non-harness test that no correspondence row names
- **THEN** the correspondence check SHALL fail and name the test and its file

#### Scenario: Trace harness outside a suite
- **WHEN** a `Test*_TraceHarness` function in a `*_trace_test.go` file lives in a package that no `[[trace]]` suite declares
- **THEN** the correspondence check SHALL fail

### Requirement: Characterisation tests are marked
A test that asserts today's defective behaviour, such as a finding replay, SHALL be identifiable as a characterisation test. It is identified in one of two ways:
- it is tabled on its finding's row, with an abstraction note that begins `characterisation:`;
- it is named `Test…_Characterisation` or `Test…_FindingF<n>`.

Characterisation tests SHALL NOT be used as mutation killers.

#### Scenario: Characterisation test is excluded from killers
- **WHEN** the mutation plan is generated and a row names a characterisation test
- **THEN** that test SHALL be excluded from the row's killer set

### Requirement: Mutation survivors feed back into bindings, models, or findings
An escaped formal-bindings mutant SHALL be resolved in one of these ways:
- strengthening the binding (a closed binding gap);
- extending the model with a new fact or property, together with its rejecting mutant, an antecedent, a correspondence row, and regenerated golden vectors when the scope changes (a closed model gap);
- deferring the gap in the ledger with a follow-up;
- declaring the behaviour as an abstraction in the named row;
- recording the mutant as equivalent;
- recording a defect as a finding.

A product fix for a defect SHALL NOT be part of the change that records it.

#### Scenario: Gap closed
- **WHEN** a binding or model gap is closed
- **THEN** a focused rerun SHALL report the mutant killed
- **THEN** the mutant SHALL leave the baseline and be listed as closed in the ledger
- **THEN** the named model's `calibration` text SHALL contain the mutant's stable ID, and the triage check SHALL verify it

#### Scenario: Model gap closed
- **WHEN** a survivor is resolved as a model gap
- **THEN** the new property SHALL have a rejecting mutant command in `formal/manifest.toml`, the correspondence table SHALL name the code and binding, and `make formal` SHALL pass before the survivor leaves the baseline

#### Scenario: Defect recorded as a finding
- **WHEN** a strengthened or new binding fails on unmodified code
- **THEN** the defect SHALL be recorded under the next free finding id with a `finding` command next to a passing fix configuration, a characterisation test, and a row in the `formal/README.md` Findings table
- **THEN** the mutant SHALL stay in the ledger as `defect`, with the finding id as its follow-up

#### Scenario: Behaviour delegated to an unnamed helper
- **WHEN** triage finds that a survivor's behaviour is decided by a helper function that the correspondence table does not name
- **THEN** the resolution SHALL either add a row naming the helper, with the same rigour as a model gap, or declare the helper's behaviour as an abstraction on the calling row
