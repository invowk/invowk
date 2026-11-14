## ADDED Requirements

### Requirement: Bindings reach the functions they bind
For every bound correspondence row, each of the row's killer tests SHALL execute at least one statement inside the line range of the function the row names. A killer test is a binding test after trace harnesses and characterisation tests are excluded.

`scripts/formal_mutation.py reach` (`make formal-reach`) SHALL check this as follows:
- it derives the rows, functions, line ranges, killers, and packages from the same plan `scripts/formal_mutation.py plan` builds, and keeps every fail-closed condition of the plan;
- it runs each killer alone with `go test -count=1 -json -covermode=set -coverpkg=<the packages of the functions it kills> -coverprofile=…`;
- it pins `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` as the mutation profiles do.

The plan SHALL record each row's own killers, so that a killer of one row never satisfies another row of the same function.

Rows the plan reports as unbound (`no-symbol`, `no-binding`, `type-symbol`, `trace-only`, `characterisation-only`) SHALL be listed in the report and SHALL NOT be gated.

#### Scenario: A binding never calls its function
- **WHEN** a killer's coverage profile has no block with a non-zero count inside the named function's line range
- **THEN** the gate SHALL fail and name the model, the row element, the symbol, the file, the killer, and the covered and total statement counts

#### Scenario: Reach is checked per row
- **WHEN** two rows name the same function, and only the first row's killer reaches it
- **THEN** the gate SHALL fail for the second row

#### Scenario: Missing or unusable evidence fails closed
- **WHEN** any of these holds:
  - a killer's profile is missing or does not start with a `mode:` line;
  - the function's file has no blocks in the profile;
  - the function's range contains no coverage blocks;
  - the killer is reported as failed, skipped, or not run;
  - `go test` exits non-zero
- **THEN** the gate SHALL fail, name the killer and its rows, and SHALL NOT treat the row as reached

#### Scenario: Unbound rows are exempt and reported
- **WHEN** a row has only trace-harness or characterisation bindings, or no binding, or names a type
- **THEN** the gate SHALL list the row with its plan reason and SHALL NOT fail because of it

#### Scenario: Calibration on the pre-change tree
- **WHEN** the gate runs on the tree before the rebinding of this change
- **THEN** it SHALL fail and name exactly these eight (row, killer) pairs, and the result SHALL be recorded in the `[reach]` calibration text of `formal/manifest.toml`:
  - `validateDestinationPath`, `LoadEnvFile`, `CustomCheckScript.ResolveWithFSAndModule`, and `Invowkfile.GetEffectiveWorkDir`, each under `TestModulePathContainment_GoldenVectors`;
  - `newVirtualPathResolverForFilesystem`, `standardVirtualAnchorsForOS`, and `VirtualFilesystemConfig.EffectiveAccess`, each under `TestVirtualPathHarness_GoldenVectors`;
  - `Server.ValidateToken` under `TestHostCallbackToken_LifecycleMatchesModel`

#### Scenario: The gate passes after rebinding
- **WHEN** the rebinding of this change is complete
- **THEN** the gate SHALL pass with every bound (row, killer) pair reached

### Requirement: The reach gate runs in the formal CI lane within a budget
The formal-verification workflow SHALL run `make formal-reach` in its own step, under the same `affected` condition as the other heavy steps, with a step `timeout-minutes`. The job timeout SHALL exceed the sum of the step timeouts plus 3 minutes. `formal/manifest.toml` SHALL carry a `[reach]` table with `budget_seconds`, `budget_source`, and `calibration`. The runner SHALL reject unknown keys in it. The gate's timing SHALL be reported against `budget_seconds` as a soft budget, the same way as model and trace budgets.

#### Scenario: Workflow and timeouts stay consistent
- **WHEN** `scripts/test_formal.py` checks the workflow
- **THEN** it SHALL require the `Binding reach` step, its timeout, its `affected` condition, and a job timeout above the summed step timeouts plus 3

#### Scenario: Unknown reach key
- **WHEN** `[reach]` contains a key other than `budget_seconds`, `budget_source`, or `calibration`
- **THEN** manifest validation SHALL fail

### Requirement: Platform-only capabilities fail on their own platform
`internal/testutil/fstree` SHALL define, per capability, the GOOS values on which the capability is required:
- directory junctions: required on `windows`;
- symbolic links: required on `linux` and `darwin`, and on `windows` when `GITHUB_ACTIONS=true`.

The capability probe SHALL keep the error that made a capability unavailable. A test that needs a capability required on the running GOOS SHALL fail with that error, and SHALL NOT skip. Off its required platforms, a missing capability MAY skip, and the skip message SHALL include the GOOS and the probe error. This narrows the "skip instances lacking a capability" behaviour of the path-containment bindings: it applies only off the required platforms.

#### Scenario: Junction unavailable on Windows
- **WHEN** a binding test needs a junction on Windows and junction creation fails
- **THEN** the test SHALL fail and report the creation error

#### Scenario: Junction unavailable off Windows
- **WHEN** a binding test needs a junction on Linux or macOS
- **THEN** the test or instance SHALL be skipped or counted as skipped, and SHALL NOT fail

#### Scenario: Golden cache honours the policy
- **WHEN** a golden replay's materialisation cache meets an instance that needs a capability required on the running GOOS but unavailable
- **THEN** the replay SHALL fail instead of counting the instance as skipped

### Requirement: Binding tests skip only through the capability policy
A `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, or `*_trace_test.go` file SHALL NOT call `Skip`, `Skipf`, or `SkipNow` directly. Its capability skips SHALL go through the `fstree` policy helpers. `scripts/formal.py correspondence` SHALL enforce this.

#### Scenario: Raw skip in a binding-test file
- **WHEN** a binding-test file contains a direct `.Skip(`, `.Skipf(`, or `.SkipNow(` call
- **THEN** the correspondence check SHALL fail and name the file and line

#### Scenario: Ordinary test files are unaffected
- **WHEN** a `_test.go` file without a binding-test suffix calls `t.Skip`
- **THEN** the correspondence check SHALL NOT report it

### Requirement: Rebinding preserves row identity and the finding procedure
When a row is rebound so that its binding reaches the named function:
- only the row's binding and abstraction cells SHALL change; its model element, Go symbol, and file cells SHALL stay, so `scripts/formal_mutation.py triage --check` keeps matching every ledger survivor;
- a characterisation test SHALL NOT be named as the reaching binding, because that would make the row exempt instead of bound;
- the new assertion SHALL compare the real function with the model's current-code verdict, including the verdicts of open findings;
- before a failure is treated as a defect, the replay's comparison target SHALL be checked against the model: the model must make that claim for that operation kind, so that a transcription error in the replay is fixed rather than recorded;
- a strengthened binding that fails on unmodified code SHALL be recorded as a finding under the next free id (F18 as of this change), through a manifest `finding` command next to a passing fix configuration, a characterisation test, and a `formal/README.md` Findings row, and the failing binding SHALL NOT be committed;
- product code SHALL NOT change.

#### Scenario: Rebinding keeps the triage ledger valid
- **WHEN** the eight rows are rebound
- **THEN** `python3 scripts/formal_mutation.py triage --check` SHALL pass without ledger edits

#### Scenario: A rebinding exposes a defect
- **WHEN** a new real-function replay fails on unmodified code, and the failure is not explained by an open finding
- **THEN** the work SHALL stop and the failure SHALL be reported, with no weaker assertion and no product change
- **THEN** the discrepancy SHALL be recorded as F18 through the finding procedure before the change continues

### Requirement: The token Auth row is bound by the lifecycle state machine
`TestHostCallbackToken_LifecycleMatchesModel` SHALL call `Server.ValidateToken` for every generated token after every step, and SHALL require the result to be admissible exactly when the Auth step's expectation holds: the model state is `valid` and the fake clock has not passed the token's TTL. The `HostCallbackToken` `Auth` row SHALL keep naming `Server.ValidateToken`.

#### Scenario: Expired token
- **WHEN** the state machine advances the clock past a token's TTL
- **THEN** `ValidateToken` SHALL report the token inadmissible, and later revocation SHALL still close the connections the token authenticated
