## ADDED Requirements

### Requirement: Pinned and checksum-verified formal tools
Invowk SHALL acquire every formal-verification tool at an exact pinned version, SHALL verify each downloaded artefact against a recorded SHA-256 checksum before every use, and SHALL NOT use `latest`, floating tags, prerelease builds, or unverified downloads. The pinned set SHALL be Alloy Analyzer `6.2.0` (`org.alloytools.alloy.dist.jar`), TLA+ tools `1.7.4` (`tla2tools.jar`, the latest stable; the `1.8.0` prerelease is excluded on purpose), and Temurin JDK 25 at an exact patch `java-version` installed in CI through `actions/setup-java@v6` and verified with `java -version`. `pgregory.net/rapid` SHALL be pinned only through `go.mod`, which is its single source of truth.

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
- **Per-check antecedents:** each Alloy `check` has a paired `run` showing its premise is satisfiable.
- **Action coverage:** TLC runs with coverage reporting, and every action with zero generated states fails unless the manifest declares it dead with a reason.
- **Fairness-free twins:** each liveness configuration has a twin without fairness that is expected to fail.
- **Deadlock checking stays on:** terminating specifications use an explicit terminal stuttering action instead of disabling deadlock checking.

#### Scenario: Unguarded property
- **WHEN** the inventory finds a safety property with no rejecting mutant, or a mutant whose violation does not name its property
- **THEN** the runner SHALL fail and name the unguarded property

#### Scenario: Uncovered action
- **WHEN** TLC coverage reports an action with zero generated states that the manifest does not declare dead
- **THEN** the runner SHALL fail and name the action

#### Scenario: Unsatisfiable antecedent
- **WHEN** the antecedent `run` paired with an Alloy `check` finds no instance
- **THEN** the runner SHALL fail, because that check would pass vacuously

#### Scenario: State-count drift
- **WHEN** a model's distinct-state count differs from the count recorded in the manifest
- **THEN** the runner SHALL print a warning with both counts, and the manifest update SHALL be part of the change that caused the drift

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

Relational models SHALL use golden vectors. Every recorded counterexample SHALL have a replay test.

#### Scenario: Golden vectors are fresh
- **WHEN** `make formal` runs
- **THEN** the runner SHALL re-enumerate every golden command and fail if the committed vectors differ from the result

#### Scenario: Golden vectors disagree with code
- **WHEN** the real function's result on an exported Alloy instance differs from the model's predicate value
- **THEN** the Go test SHALL fail and name the instance file and the model predicate

#### Scenario: Counterexample replay
- **WHEN** a model command records a counterexample as an expected finding
- **THEN** a Go test SHALL reproduce the same outcome against the real code, or SHALL be listed as model-only evidence with the reason a deterministic reproduction needs a seam that does not yet exist

### Requirement: Model-to-code correspondence record
Every model SHALL carry a header table with these columns: model element, Go symbol, file, binding test, and abstraction. Every omitted or bounded behaviour SHALL be listed as an abstraction.

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

No Go package SHALL be placed under `formal/`. Invowk SHALL expose `make formal`, `make formal-alloy`, `make formal-tla`, `make formal-traces`, `make formal-golden`, and `make formal-rapid-deep`. The runner SHALL be `scripts/formal.py`, using the Python standard library only, and its fail-closed behaviour SHALL be tested by `scripts/test_formal.py` under `make test-scripts`. Model, manifest, and script files SHALL carry an SPDX `MPL-2.0` header comment.

#### Scenario: Ordinary builds need no formal tooling
- **WHEN** a contributor runs `make build`, `make test`, or `make lint` without Java or formal jars
- **THEN** those targets SHALL succeed, rapid tests SHALL run as ordinary Go tests, and trace harnesses SHALL skip

#### Scenario: Deep rapid mode is scoped
- **WHEN** `make formal-rapid-deep` runs
- **THEN** it SHALL set `RAPID_CHECKS` to at least 100 times rapid's default, and run only the packages that contain rapid tests

#### Scenario: Reports are not tracked
- **WHEN** the runner or rapid writes reports, traces, or failure files
- **THEN** they SHALL go under `artifacts/formal/` or `testdata/rapid/`, both ignored by `.gitignore`

### Requirement: CI lane and promotion
A workflow SHALL run the full lane plus `make formal-rapid-deep` weekly and on dispatch. It SHALL also run the full lane on pull requests that touch `formal/` or a modelled package, because the full lane is fast enough (about 35 s) that selecting individual models adds risk without saving time. The workflow SHALL:
- declare `permissions: contents: read`, `concurrency`, and `timeout-minutes`;
- use job-level `env:` for shared variables;
- upload `artifacts/formal/` on failure.

The lane SHALL NOT be a required status check until it has four consecutive green weekly runs.

#### Scenario: Relevant pull request
- **WHEN** a pull request changes a path named in any model's correspondence table, or `formal/`
- **THEN** the workflow SHALL run the full lane and the bindings within its `timeout-minutes` budget

#### Scenario: Premature promotion
- **WHEN** a change proposes making the lane required
- **THEN** it SHALL cite four consecutive green weekly runs

### Requirement: Rejected-tool decision record
`formal/README.md` SHALL contain a decision record with one entry per evaluated and rejected tool: Quint, Gobra, Dafny, Lean 4, Goose/Perennial, and Gomela. Each entry SHALL give the reason for rejection and the evidence source. The record SHALL include a summary of the Lushtext measurements it relies on, so the record stands without the sibling repository.

#### Scenario: Record completeness
- **WHEN** a reviewer inspects `formal/README.md`
- **THEN** each rejected tool SHALL have a reason and an evidence source, and the Lushtext summary SHALL name the false-green and scaling results
