# go-toolchain-upgrades Specification (Delta)

## ADDED Requirements

### Requirement: Go modules and toolchain-pinned images move in lockstep
Invowk SHALL upgrade the Go toolchain by updating every Go module's `go` directive and every container image that pins the Go toolchain within the same change, so that no intermediate state mixes toolchain versions.

#### Scenario: Both Go modules are bumped together
- **WHEN** the Go toolchain version is upgraded
- **THEN** the root `go.mod` and `tools/goplint/go.mod` MUST declare the same new `go` version
- **AND** CI MUST continue to resolve the toolchain via `go-version-file:` rather than hardcoded workflow versions

#### Scenario: GOTOOLCHAIN=local images cannot lag the modules
- **WHEN** a repository container image sets `GOTOOLCHAIN=local` (such as `build/bencher/Dockerfile`)
- **THEN** its Go base image MUST be updated to the new toolchain major/minor in the same change as the `go.mod` bump
- **AND** the change MUST NOT be considered complete while any toolchain-pinned image would fail to build the modules

### Requirement: Toolchain-coupled analysis tools are upgraded with the toolchain
Invowk SHALL upgrade tools that parse or type-check Go source (golangci-lint, `golang.org/x/tools`, go-mutesting) to releases built for the new Go version as part of the toolchain upgrade, using exact version pins.

#### Scenario: Analysis tools understand the new Go version
- **WHEN** the Go toolchain is upgraded to a new minor version
- **THEN** `golang.org/x/tools` MUST be upgraded in both Go modules to a release synchronized with that Go version
- **AND** golangci-lint and go-mutesting MUST be upgraded to exact versions released for that Go version
- **AND** every enforcing pin location (tool directives, wrapper scripts, wrapper test fixtures, version-pinning documentation) MUST record the same new versions

### Requirement: Toolchain-bound artifacts are regenerated, not hand-edited
Invowk SHALL regenerate every artifact whose validity is bound to the running toolchain — timing censuses, benchmark threshold manifests, PGO profiles, and retained clean-tree evidence — on the new toolchain before the upgrade is complete, and SHALL NOT carry forward measured values from the previous toolchain.

#### Scenario: Exact-match timing census is regenerated
- **WHEN** the recorded timing census toolchain no longer equals `runtime.Version()`
- **THEN** the census MUST be regenerated with its dedicated refresh command on the new toolchain
- **AND** editing only the recorded toolchain string without re-measuring MUST NOT be accepted

#### Scenario: Benchmark thresholds are re-derived on the new toolchain
- **WHEN** benchmark threshold manifests prefix-check a Go toolchain that the upgrade replaces
- **THEN** the manifests MUST be updated to the new toolchain prefix
- **AND** threshold values MUST be derived from measurements taken on the new toolchain

#### Scenario: Clean-tree evidence is fully regenerated
- **WHEN** a toolchain upgrade changes recorded tool version banners in retained evidence
- **THEN** the evidence MUST be regenerated with the full generation command rather than re-bound as prose-only drift

#### Scenario: PGO profile reflects the new compiler
- **WHEN** the Go toolchain upgrade changes compiler or runtime allocation behavior
- **THEN** the committed PGO profile MUST be regenerated on the new toolchain before the upgrade is complete

### Requirement: New Go language constructs gain analyzer fixture coverage
Invowk SHALL add goplint fixture coverage for any new Go language construct, introduced by the upgraded toolchain, that changes the shape of declarations goplint analyzes (methods, constructors, struct literals, type parameters), so that analyzer behavior on the new construct is pinned by test.

#### Scenario: Generic methods are exercised by fixtures
- **WHEN** the upgraded toolchain permits method declarations with type-parameter lists
- **THEN** goplint testdata MUST include a value type declaring a generic method
- **AND** the analyzer's classification of that fixture MUST be asserted by a test
- **AND** if the analyzer cannot soundly classify the construct, the pinned outcome MUST be fail-closed (inconclusive or reported), never silently safe

### Requirement: Integrity-sensitive JSON records are decoded strictly
Invowk SHALL decode integrity-sensitive JSON records — goplint clean-tree evidence, execution plans, timing censuses, baselines, and related gate artifacts — with strict JSON semantics that reject duplicate object keys and invalid UTF-8, once the toolchain provides them.

#### Scenario: Duplicate keys in an evidence record are rejected
- **WHEN** a goplint gate reads a JSON record containing a duplicate object key
- **THEN** the read MUST fail with an actionable error
- **AND** the record MUST NOT be accepted with last-value-wins semantics

#### Scenario: Strict decoding is verified against self-produced records
- **WHEN** a decode path is converted to strict JSON semantics
- **THEN** a round-trip test MUST verify that records written by the corresponding producer decode successfully
- **AND** field-name matching differences from the previous decoder (case sensitivity) MUST be covered by that test

#### Scenario: Lenient decode paths remain deliberate
- **WHEN** a decode path consumes externally produced or inherently loose input (such as LLM responses)
- **THEN** it MAY retain lenient decoding
- **AND** the choice MUST be per-call-site and reviewable, not a global toggle

### Requirement: Server lifecycle tests assert zero goroutine leaks
Invowk SHALL assert, in serverbase, sshserver, and tuiserver lifecycle tests, that stopping a server leaks no goroutines, using the toolchain's goroutine-leak detection.

#### Scenario: Stopped server leaves no leaked goroutines
- **WHEN** a lifecycle test drives a server to a terminal stopped state
- **THEN** the test MUST assert that no goroutines attributable to that server remain blocked on unreachable synchronization primitives

#### Scenario: Surfaced leaks are fixed, not suppressed
- **WHEN** a leak assertion fails for a pre-existing goroutine leak
- **THEN** the leak MUST be fixed or explicitly triaged
- **AND** the assertion MUST NOT be weakened by exclusion lists or baselines

### Requirement: Documentation and examples track the supported toolchain
Invowk SHALL update all prose prerequisites, example version strings, and paired example fixtures that state the minimum or example Go version when the toolchain is upgraded, preserving localization and snippet parity.

#### Scenario: Prerequisite prose is synchronized
- **WHEN** the minimum Go version changes
- **THEN** README, website installation docs, localized (i18n) counterparts, agent governance documents, and rules that state the Go prerequisite MUST all state the new version
- **AND** existing documentation parity gates MUST pass

#### Scenario: Version-bearing examples stay valid
- **WHEN** examples embed the Go version in image tags, grep patterns, or numeric threshold comparisons
- **THEN** those examples MUST be updated so they remain correct for the new toolchain
- **AND** example fixtures paired with schema documentation comments MUST be updated together
