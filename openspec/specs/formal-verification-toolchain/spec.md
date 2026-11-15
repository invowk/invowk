# formal-verification-toolchain Specification

## Purpose
This capability specifies the toolchain Invowk uses to check Alloy and TLA+ models of its behaviour and to bind those models to the real Go code. It pins and checksum-verifies the formal tools, fails closed on missing or unexpected verdicts, guards against vacuous checks, and ties every model to code through golden vectors, counterexample replays, rapid tests, trace validation and correspondence tables. The aim is that a model is only claimed to verify a property once it is calibrated and bound to code, and that bounded or omitted behaviour is stated as an abstraction rather than claimed as verified.

## Requirements

### Requirement: Pinned and checksum-verified formal tools
Invowk SHALL acquire every formal-verification tool at an exact pinned version, SHALL verify each downloaded artefact against a recorded SHA-256 checksum before every use, and SHALL NOT use `latest`, floating tags, prerelease builds, or unverified downloads.

#### Scenario: Tool download is verified
- **WHEN** `scripts/formal.py` fetches a tool jar that is absent from `bin/formal/`
- **THEN** it SHALL compare the artefact's SHA-256 with the value recorded in the script
- **THEN** a mismatch SHALL delete the artefact and exit non-zero before any model is checked

#### Scenario: Cached tool is re-verified
- **WHEN** a tool jar already exists in `bin/formal/`
- **THEN** the runner SHALL re-verify its checksum before the run and print the tool name, pinned version, and checksum used

#### Scenario: Version references stay synchronized
- **WHEN** the Alloy, TLA+ tools, JDK, or `setup-java` version changes
- **THEN** `formal/manifest.toml`, the workflow, `.agents/rules/version-pinning.md`, the `ci-update` skill's sync-pair table, and the formal-verification skill SHALL change in the same commit

#### Scenario: No extra TLA+ modules are required
- **WHEN** trace validation runs
- **THEN** it SHALL use only modules shipped in the pinned `tla2tools.jar` plus generated trace modules, and SHALL NOT depend on TLA+ CommunityModules

### Requirement: Pinned formal tool set
The pinned set SHALL be Alloy Analyzer `6.2.0` (`org.alloytools.alloy.dist.jar`), TLA+ tools `1.7.4` (`tla2tools.jar`, the latest stable; the `1.8.0` prerelease is excluded on purpose), and Temurin JDK 25 at an exact patch `java-version` installed in CI through `actions/setup-java@v6` and verified with `java -version`. `pgregory.net/rapid` SHALL be pinned only through `go.mod`, which is its single source of truth.

#### Scenario: Pinned versions are used
- **WHEN** the formal lane acquires its tools
- **THEN** it SHALL use Alloy Analyzer `6.2.0`, TLA+ tools `1.7.4` (never the `1.8.0` prerelease) and Temurin JDK 25 at the exact patch `java-version` installed through `actions/setup-java@v6` and verified with `java -version`

#### Scenario: rapid is pinned by go.mod
- **WHEN** the `pgregory.net/rapid` version is needed
- **THEN** it SHALL come only from `go.mod`

### Requirement: Fail-closed verdict handling
Every model command SHALL have a declared expected verdict in `formal/manifest.toml`. The runner SHALL render each Alloy command, including its native `expect 0|1` bit, and each TLC configuration from the manifest (change `formal-infra-refinements`). It SHALL fail unless the observed verdict matches the manifest. A checker that exits zero without a parseable verdict SHALL be treated as a failure.

#### Scenario: Missing verdict fails the run
- **WHEN** a checker exits with status 0 but its output has no recognised verdict line
- **THEN** the runner SHALL report `NO-VERDICT` for that command and exit non-zero

#### Scenario: Unexpected verdict fails the run
- **WHEN** a command expected to pass produces a counterexample, or a command expected to produce a counterexample passes
- **THEN** the runner SHALL exit non-zero and keep the counterexample or trace under `artifacts/formal/`

#### Scenario: Command declared outside the manifest
- **WHEN** an Alloy model source declares a `run` or `check` command instead of leaving it to the manifest
- **THEN** the runner SHALL fail before invoking the solver, as specified by the `formal-infra-refinements` scenario "Command left in the model source"

### Requirement: Vacuity and coverage guards
The runner SHALL enforce the following guards for every model:
- **Named-violation mutants:** each safety property has at least one rejecting mutant, and the mutant counts only if the checker's output names that property. A deadlock or a different property does not count.
- **Witness invariants:** each interesting situation the model claims to cover has an invariant `~W`, and TLC must violate it.

#### Scenario: Unguarded property
- **WHEN** the inventory finds a safety property with no rejecting mutant, or a mutant whose violation does not name its property
- **THEN** the runner SHALL fail and name the unguarded property

#### Scenario: State-count drift
- **WHEN** a model's distinct-state count differs from the count recorded in the manifest
- **THEN** the runner SHALL print a warning with both counts, and the manifest update SHALL be part of the change that caused the drift

### Requirement: Antecedent and action coverage guards
The runner SHALL enforce the following guards for every model:
- **Per-check antecedents:** each Alloy `check` has a paired `run` showing its premise is satisfiable.
- **Action coverage:** TLC runs with coverage reporting, and every action with zero generated states fails unless the manifest declares it dead with a reason.

#### Scenario: Uncovered action
- **WHEN** TLC coverage reports an action with zero generated states that the manifest does not declare dead
- **THEN** the runner SHALL fail and name the action

#### Scenario: Unsatisfiable antecedent
- **WHEN** the antecedent `run` paired with an Alloy `check` finds no instance
- **THEN** the runner SHALL fail, because that check would pass vacuously

### Requirement: Fairness-free twins and deadlock checking guards
The runner SHALL enforce the following guards for every model:
- **Fairness-free twins:** each liveness configuration has a twin without fairness that is expected to fail.
- **Deadlock checking stays on:** terminating specifications use an explicit terminal stuttering action instead of disabling deadlock checking.

#### Scenario: Liveness configuration has a fairness-free twin
- **WHEN** a liveness configuration is checked
- **THEN** its twin without fairness SHALL be expected to fail

#### Scenario: Terminating specification keeps deadlock checking
- **WHEN** a specification terminates
- **THEN** it SHALL use an explicit terminal stuttering action and SHALL NOT disable deadlock checking

### Requirement: Liveness configurations are sound
Liveness configurations SHALL NOT use `SYMMETRY` or `VIEW`. Weak or strong fairness SHALL be attached only to system actions and never to environment actions such as cancellation, fatal errors, attacker steps, or callback errors. Event volume SHALL be bounded by an environment budget variable, not by a state constraint.

#### Scenario: Symmetry in a liveness configuration
- **WHEN** a configuration that checks a temporal property declares `SYMMETRY` or `VIEW`
- **THEN** the runner SHALL fail before invoking TLC

#### Scenario: Safety configuration uses VIEW
- **WHEN** a safety configuration uses `VIEW`
- **THEN** the manifest SHALL record a matching no-`VIEW` run at a smaller bound with the same verdict

### Requirement: Calibration before a model counts
A model SHALL NOT be listed as verifying a property until it has reproduced a known answer: a mutant from the mutation baseline, a reverted historical fix, or a seeded code defect that its bindings detect.

#### Scenario: Uncalibrated model
- **WHEN** a model's manifest entry has no calibration record
- **THEN** `formal/README.md` SHALL list the model as uncalibrated and SHALL NOT claim its properties as verified

### Requirement: Model-to-code bindings
Every model SHALL be bound to the real Go code by at least one of:
- **Golden vectors:** exhaustive small-scope Alloy instances, exported as JSON and replayed against the real function by an ordinary Go test.
- **Counterexample replay:** each recorded counterexample becomes a Go regression test.
- **rapid property or state-machine tests:** these reuse the model's property names.
- **Trace validation.**

#### Scenario: Golden vectors are fresh
- **WHEN** `make formal` runs
- **THEN** the runner SHALL re-enumerate every golden command and fail if the committed vectors differ from the result

#### Scenario: Golden vectors disagree with code
- **WHEN** the real function's result on an exported Alloy instance differs from the model's predicate value
- **THEN** the Go test SHALL fail and name the instance file and the model predicate

### Requirement: Required binding kinds
Relational models SHALL use golden vectors. Every recorded counterexample SHALL have a replay test.

#### Scenario: Relational model binding
- **WHEN** a relational model is bound to code
- **THEN** its bindings SHALL include golden vectors

#### Scenario: Counterexample replay
- **WHEN** a model command records a counterexample as an expected finding
- **THEN** a Go test SHALL reproduce the same outcome against the real code, or SHALL be listed as model-only evidence with the reason a deterministic reproduction needs a seam that does not yet exist

### Requirement: Model-to-code correspondence record
Every model SHALL carry a header table with these columns: model element, Go symbol, file, binding test, and abstraction. Every omitted or bounded behaviour SHALL be listed as an abstraction. The binding-test column MAY list several tests separated by commas (`mutation-test-formal-bindings`, "Correspondence binding cells may name several tests"); a multi-test cell satisfies this requirement exactly as a single-test cell does.

#### Scenario: Stale symbol
- **WHEN** the correspondence check runs
- **THEN** every Go symbol named in a table SHALL exist in the named file, and a missing symbol SHALL fail the check with the model and row

#### Scenario: Abstractions are explicit
- **WHEN** a model bounds or omits behaviour of the real code
- **THEN** the abstraction SHALL be listed in the table and SHALL NOT be claimed as verified elsewhere

### Requirement: Layout, targets, and scripts
The layout SHALL be:
- models under `formal/alloy/` and `formal/tla/`;
- the manifest at `formal/manifest.toml`;
- tools under `bin/formal/`;
- reports under `artifacts/formal/`.

No Go package SHALL be placed under `formal/`. Invowk SHALL expose `make formal`, `make formal-alloy`, `make formal-tla`, `make formal-traces`, `make formal-golden`, and `make formal-rapid-deep`.

#### Scenario: Ordinary builds need no formal tooling
- **WHEN** a contributor runs `make build`, `make test`, or `make lint` without Java or formal jars
- **THEN** those targets SHALL succeed, rapid tests SHALL run as ordinary Go tests, and trace harnesses SHALL skip

#### Scenario: Deep rapid mode is scoped
- **WHEN** `make formal-rapid-deep` runs
- **THEN** it SHALL set `RAPID_CHECKS` to at least 100 times rapid's default, and run only the packages that contain rapid tests

#### Scenario: Reports are not tracked
- **WHEN** the runner or rapid writes reports, traces, or failure files
- **THEN** they SHALL go under `artifacts/formal/` or `testdata/rapid/`, both ignored by `.gitignore`

### Requirement: Formal runner script and license headers
The runner SHALL be `scripts/formal.py`, using the Python standard library only, and its fail-closed behaviour SHALL be tested by `scripts/test_formal.py` under `make test-scripts`. Model, manifest, and script files SHALL carry an SPDX `MPL-2.0` header comment.

#### Scenario: Runner tests run under test-scripts
- **WHEN** `make test-scripts` runs
- **THEN** `scripts/test_formal.py` SHALL test the fail-closed behaviour of `scripts/formal.py`

#### Scenario: License header present
- **WHEN** a model, manifest, or script file is added or changed
- **THEN** it SHALL carry an SPDX `MPL-2.0` header comment

### Requirement: CI lane and promotion
A workflow SHALL run the full lane plus `make formal-rapid-deep` weekly and on dispatch. It SHALL also run the full lane on pull requests that touch `formal/` or a modelled package, because the full lane is fast enough (about 35 s) that selecting individual models adds risk without saving time.

The lane SHALL NOT be a required status check until it has four consecutive green weekly runs.

#### Scenario: Relevant pull request
- **WHEN** a pull request changes a path named in any model's correspondence table, or `formal/`
- **THEN** the workflow SHALL run the full lane and the bindings within its `timeout-minutes` budget

#### Scenario: Premature promotion
- **WHEN** a change proposes making the lane required
- **THEN** it SHALL cite four consecutive green weekly runs

### Requirement: Formal workflow configuration
The formal-verification workflow SHALL:
- declare `permissions: contents: read`, `concurrency`, and `timeout-minutes`;
- use job-level `env:` for shared variables;
- upload `artifacts/formal/` on failure.

#### Scenario: Failed lane keeps its reports
- **WHEN** the formal lane fails in CI
- **THEN** the workflow SHALL upload `artifacts/formal/`

### Requirement: Rejected-tool decision record
`formal/README.md` SHALL contain a decision record with one entry per evaluated and rejected tool: Quint, Gobra, Dafny, Lean 4, Goose/Perennial, and Gomela. Each entry SHALL give the reason for rejection and the evidence source. The record SHALL include a summary of the Lushtext measurements it relies on, so the record stands without the sibling repository.

#### Scenario: Record completeness
- **WHEN** a reviewer inspects `formal/README.md`
- **THEN** each rejected tool SHALL have a reason and an evidence source, and the Lushtext summary SHALL name the false-green and scaling results

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
- extending the model with a new fact or property (a closed model gap; see "Model gap extensions are complete");
- deferring the gap in the ledger with a follow-up;
- declaring the behaviour as an abstraction in the named row;
- recording the mutant as equivalent;
- recording a defect as a finding.

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

### Requirement: Model gap extensions are complete
A survivor resolved by extending the model SHALL add the new fact or property together with its rejecting mutant, an antecedent, a correspondence row, and regenerated golden vectors when the scope changes.

#### Scenario: Model extension carries its guards
- **WHEN** a model is extended with a new fact or property to close a model gap
- **THEN** the change SHALL add its rejecting mutant, an antecedent and a correspondence row, and SHALL regenerate golden vectors when the scope changes

### Requirement: Defect fixes are separate from finding records
A product fix for a defect SHALL NOT be part of the change that records it as a finding.

#### Scenario: Finding recorded without its fix
- **WHEN** a change records a defect as a finding
- **THEN** that change SHALL NOT contain the product fix for the defect

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

### Requirement: Receiver-qualified symbols name a declared method
A row whose Go symbol is `Recv.name` and whose file declares `name` as a function or method SHALL name a declaration with receiver `Recv`. `scripts/formal.py correspondence` SHALL fail on such a row when no declaration has that receiver, naming the row, the symbol, and the receivers declared. The formal-bindings plan and `triage --check` SHALL fail on it too, and SHALL NOT classify it as `type-symbol`.

#### Scenario: Receiver renamed without the table
- **WHEN** a row names `CallbackServer.RevokeToken` and the file declares only `Server.RevokeToken`
- **THEN** the correspondence check, the plan, and `triage --check` SHALL fail and name the row and the declared receiver `Server`

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
