## ADDED Requirements

### Requirement: Bindings reach the functions they bind
For every bound correspondence row, each of the row's killer tests SHALL execute at least one statement inside the line range of the function the row names. A killer test is a binding test that is neither a trace harness nor a characterisation test.

Rows the plan reports as unbound (`no-symbol`, `no-binding`, `type-symbol`, `trace-only`, `characterisation-only`) SHALL be listed with their reason and abstraction cell, and SHALL NOT be gated.

#### Scenario: A binding never calls its function
- **WHEN** a killer's coverage profile has no block with a non-zero count inside the named function's line range
- **THEN** the gate SHALL fail and name the model, the row element, the symbol, the file, the killer, and the covered and total statement counts

#### Scenario: Unbound rows are exempt and reported
- **WHEN** a row has only trace-harness or characterisation bindings, or no binding, or names a type
- **THEN** the gate SHALL list the row with its plan reason and abstraction cell, and SHALL NOT fail because of it

#### Scenario: Calibration on the pre-change tree
- **WHEN** the gate runs on the tree before this change rebinds any row
- **THEN** it SHALL fail and name exactly eight (row, killer) pairs, the ones the next two scenarios list, and no others
- **THEN** the result SHALL be recorded in the `[reach]` calibration text of `formal/manifest.toml`

#### Scenario: Calibration pairs under the module-path golden
- **WHEN** the gate runs on the tree before this change rebinds any row
- **THEN** four of the eight pairs SHALL be `validateDestinationPath`, `LoadEnvFile`, `CustomCheckScript.ResolveWithFSAndModule`, and `Invowkfile.GetEffectiveWorkDir`, each under `TestModulePathContainment_GoldenVectors`

#### Scenario: Calibration pairs under the harness golden and the token model
- **WHEN** the gate runs on the tree before this change rebinds any row
- **THEN** three pairs SHALL be `newVirtualPathResolverForFilesystem`, `standardVirtualAnchorsForOS`, and `VirtualFilesystemConfig.EffectiveAccess`, each under `TestVirtualPathHarness_GoldenVectors`
- **THEN** the eighth SHALL be `Server.ValidateToken` under `TestHostCallbackToken_LifecycleMatchesModel`

#### Scenario: The gate passes after the rows are resolved
- **WHEN** this change has rebound or unbound each of the eight rows
- **THEN** the gate SHALL pass with every bound (row, killer) pair reached, and SHALL list every row it unbound, `Auth` and `unpackRejectsEscape`, as `no-binding`

### Requirement: Reach gate command
`scripts/formal_mutation.py reach` (`make formal-reach`) SHALL take the rows, functions, line ranges, killers, and packages from the plan that `scripts/formal_mutation.py plan` builds, keeping every fail-closed condition of the plan. It SHALL run each killer alone with `go test -count=1 -json -covermode=set -coverpkg=<the packages of the functions it kills> -coverprofile=…`. It SHALL pin `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` as the mutation profiles do.

#### Scenario: Plan failure stops the gate
- **WHEN** the formal-bindings plan fails closed, for example on a correspondence failure or a killer declared in two packages
- **THEN** the reach gate SHALL exit non-zero before running any killer

#### Scenario: Report is written
- **WHEN** the gate finishes
- **THEN** it SHALL write `reach.json` and a text summary under its output directory (default `artifacts/formal/reach/`), listing every (row, killer, function) triple with its covered and total statements, every failure, and every unbound row

### Requirement: Reach evidence fails closed
The reach gate SHALL treat missing or unusable coverage evidence as a failure that names the killer and its rows. It SHALL NOT treat such a row as reached.

#### Scenario: Missing or unusable evidence
- **WHEN** any of these holds:
  - a killer's profile is missing or does not start with a `mode:` line;
  - the function's file has no blocks in the profile;
  - the function's range contains no coverage blocks;
  - the killer is reported as failed, skipped, or not run;
  - `go test` exits non-zero, even if its JSON reports a pass
- **THEN** the gate SHALL fail and name the killer and its rows

### Requirement: Per-row killers in the formal-bindings plan
The formal-bindings plan SHALL record each row's own killers in `functions[].rows[].killers`, next to the per-function union, and its version SHALL be 2. The reach gate SHALL check each row against its own killers only.

#### Scenario: Reach is checked per row
- **WHEN** two rows name the same function, and only the first row's killer reaches it
- **THEN** the gate SHALL fail for the second row

#### Scenario: Old plan versions are rejected
- **WHEN** a plan file with version 1 is loaded
- **THEN** loading SHALL fail with an unsupported-version error

### Requirement: Reach budget
`formal/manifest.toml` SHALL carry a `[reach]` table with exactly the keys `budget_seconds`, `budget_source`, and `calibration`, and manifest validation SHALL reject any other key. The gate SHALL report its wall time against `budget_seconds` as a soft budget, the same way as model and trace budgets: an overrun warns and does not change the exit status.

#### Scenario: Unknown reach key
- **WHEN** `[reach]` contains a key other than `budget_seconds`, `budget_source`, or `calibration`
- **THEN** manifest validation SHALL fail

#### Scenario: Soft overrun only warns
- **WHEN** the gate's wall time exceeds `[reach] budget_seconds`
- **THEN** the timing report SHALL warn, and the gate's exit status SHALL depend only on reach

### Requirement: Reach step in the formal workflow
The formal-verification workflow SHALL run `make formal-reach` in a `Binding reach` step after the golden and property replay step. The step SHALL use the same `affected` condition as the other heavy steps and SHALL have a step `timeout-minutes`. `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` SHALL be in `[ci] paths`.

#### Scenario: Workflow wiring is checked
- **WHEN** `scripts/test_formal.py` checks the workflow
- **THEN** it SHALL require the `Binding reach` step, its `timeout-minutes`, and its `affected` condition

#### Scenario: A change to the gate runs the lane
- **WHEN** a pull request changes only `scripts/formal_mutation.py`
- **THEN** the `affected` classifier SHALL report `run=true`

### Requirement: Platform-only capabilities fail on their own platform
`internal/testutil/fstree` SHALL define, per capability, the GOOS values on which the capability is required:
- directory junctions: required on `windows`;
- symbolic links: required on `linux` and `darwin`, and on `windows` only when `GITHUB_ACTIONS=true`.

A test that needs a capability required on the running GOOS SHALL fail when the capability is unavailable, and SHALL NOT skip.

#### Scenario: Junction unavailable on Windows
- **WHEN** a binding test needs a junction on Windows and junction creation fails
- **THEN** the test SHALL fail and report the creation error

#### Scenario: Junction unavailable off Windows
- **WHEN** a binding test needs a junction on Linux or macOS
- **THEN** the test or instance SHALL be skipped or counted as skipped, and SHALL NOT fail

#### Scenario: Symlinks on a local Windows host
- **WHEN** symlink creation fails on Windows and `GITHUB_ACTIONS` is not `true`
- **THEN** the test SHALL skip, and the skip message SHALL name `windows` and the probe error

#### Scenario: Golden cache honours the policy
- **WHEN** a golden replay's materialisation cache meets an instance that needs a capability that is required on the running GOOS but unavailable
- **THEN** the replay SHALL fail instead of counting the instance as skipped

### Requirement: Capability probes keep their errors
The capability probe SHALL keep the error that made a capability unavailable. A required-but-unavailable capability SHALL fail with a message that names the capability, the GOOS, and that error. Off its required platforms, a skip message SHALL name the GOOS and that error. The policy SHALL be testable for every GOOS on one host.

#### Scenario: Policy unit test
- **WHEN** the `fstree` policy test runs with injected GOOS values, environment, and probe results
- **THEN** it SHALL show a failure for each required-but-unavailable case and a skip for every other unavailable case, each message carrying the probe error

### Requirement: Binding tests skip only through the capability policy
A `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, or `*_trace_test.go` file SHALL NOT call `Skip`, `Skipf`, or `SkipNow` directly. Its capability skips SHALL go through the `fstree` policy helpers. `scripts/formal.py correspondence` SHALL enforce this.

#### Scenario: Raw skip in a binding-test file
- **WHEN** a binding-test file contains a direct `.Skip(`, `.Skipf(`, or `.SkipNow(` call
- **THEN** the correspondence check SHALL fail and name the file and line

#### Scenario: Ordinary test files are unaffected
- **WHEN** a `_test.go` file without a binding-test suffix calls `t.Skip`
- **THEN** the correspondence check SHALL NOT report it

### Requirement: Correspondence rows name existing model elements
`scripts/formal.py correspondence` SHALL fail on a row whose element cell's leading identifier occurs, as a whole case-sensitive word, neither in the model source, with comments stripped, nor in a command `name` or `body` of that model in `formal/manifest.toml`. The failure SHALL name the model, the row, and the file. A pair listed in `[correspondence] unresolved_elements` SHALL be reported instead of failing.

#### Scenario: Element missing from the model
- **WHEN** a new row's element names nothing in its model source or manifest commands, and the pair is not listed
- **THEN** the correspondence check SHALL fail and name the model, the row, and the file

#### Scenario: Element named by a manifest command
- **WHEN** a row's leading identifier occurs only in a manifest command `body` of its model
- **THEN** the row SHALL resolve

#### Scenario: Case-only match does not resolve
- **WHEN** a row's leading identifier matches a model identifier only when case is ignored, for example `State` against `state`
- **THEN** the row SHALL NOT resolve

### Requirement: Unresolved model elements are listed and shrink
`formal/manifest.toml` SHALL list each `"<model>: <element>"` pair that did not resolve when the element check was introduced in `[correspondence] unresolved_elements`, and manifest validation SHALL reject other keys in that table. No new pair SHALL be added. A listed pair that resolves, or that names no row, SHALL fail the correspondence check as stale. The survivor-triage follow-up SHALL rename the listed elements and empty the list.

#### Scenario: Listed legacy element
- **WHEN** the `unpackRejectsEscape` row is checked and `"ModulePathContainment: unpackRejectsEscape"` is listed
- **THEN** the check SHALL report the row and SHALL NOT fail

#### Scenario: Stale list entry
- **WHEN** a listed pair's element now resolves, or no row has that model and element
- **THEN** the correspondence check SHALL fail and name the stale entry

### Requirement: Rebinding preserves row identity
When a row's binding is changed so that it reaches the named function, or when the row is unbound, only the row's binding and abstraction cells SHALL change. Its model element, Go symbol, and file cells SHALL stay, so `scripts/formal_mutation.py triage --check` keeps matching every ledger survivor without ledger edits. A characterisation test SHALL NOT be named as the reaching binding.

#### Scenario: Rebinding keeps the triage ledger valid
- **WHEN** the eight rows are rebound or unbound
- **THEN** `python3 scripts/formal_mutation.py triage --check` SHALL pass without ledger edits, once the `[triage]` list is seeded

#### Scenario: Characterisation test offered as a binding
- **WHEN** the only test that reaches a function is a characterisation test
- **THEN** it SHALL NOT be named in the row's binding cell as the reaching binding

### Requirement: Rebound assertions compare with the model
A rebound binding SHALL compare the real function's result with the model's current-code verdict, including the verdicts of open findings. Before a failure is treated as a defect, the comparison target SHALL be checked against the model source: the model must make that claim for that operation kind. A failure caused by a transcription error in the replay SHALL be fixed, not recorded.

#### Scenario: Open finding in the verdict
- **WHEN** the model's current-code verdict for an instance reproduces an open finding
- **THEN** the rebound assertion SHALL expect the finding's behaviour

#### Scenario: Transcription error
- **WHEN** a rebound replay fails because it compares with a decision that the model does not make for that operation kind
- **THEN** the replay SHALL be corrected, and no finding SHALL be recorded

### Requirement: A rebinding that fails on unmodified code is a finding
When a rebound binding fails on unmodified code and no open finding explains the failure, the work SHALL stop and the failure SHALL be reported. The discrepancy SHALL be recorded under the next free finding id (F18 as of this change) through the finding procedure, and the failing binding SHALL NOT be committed. The assertion SHALL NOT be weakened, and product code SHALL NOT change.

#### Scenario: A rebinding exposes a defect
- **WHEN** a new real-function replay fails on unmodified code, and the failure is not explained by an open finding
- **THEN** the work SHALL stop and report the failure, with no weaker assertion and no product change
- **THEN** the discrepancy SHALL be recorded as F18, with a manifest `finding` command next to a passing fix configuration, a characterisation test, and a `formal/README.md` Findings row

### Requirement: Rows whose function does not carry the model's claim are unbound
A row SHALL NOT be bound by a test that calls its function directly when any of these holds:
- the function has no production caller;
- the function alone does not make the claim the model element states;
- the model states nothing about the function's result that a replay could compare with.

Such a row SHALL have binding `-`, and its abstraction cell SHALL state the reason and the follow-up.

#### Scenario: Function with no production caller
- **WHEN** the `HostCallbackToken` `Auth` row names `Server.ValidateToken`, and no non-test code calls it
- **THEN** the row's binding SHALL be `-`, and its abstraction cell SHALL say that it is not gated because the function has no production caller, that the model's `Auth` is implemented by `Server.admitConn`, and that re-pointing the row is the survivor-triage follow-up
