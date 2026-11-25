# Spec Delta

## MODIFIED Requirements

### Requirement: Trace-validated models
Invowk SHALL trace-validate these TLA+ models against real code: `Serverbase`, `AtomicWrite`, `Watch`, `HostCallbackToken`, and `LockIntegrity`. A model added later SHALL declare whether it is trace-validated or bound by characterisation tests.

#### Scenario: Suites run under make formal-traces
- **WHEN** `make formal-traces` runs
- **THEN** it SHALL record and check the `HostCallbackToken` and `LockIntegrity` suites together with the existing three
- **THEN** it SHALL fail when either suite has no accepted traces, has no targeted mutations, accepts a mutation, rejects a recorded trace, or records fields that differ from its `Proj`

#### Scenario: Ordinary test runs
- **WHEN** `make test` runs without `INVOWK_FORMAL_TRACE_DIR`
- **THEN** both new harnesses SHALL skip, while the F11 regression test, the F12 replay test and the characterisation tests SHALL run

### Requirement: Host-callback token observation
`TestHostCallbackToken_TraceHarness` SHALL observe token admissibility, connection liveness and shutdown as the scenario below details.

#### Scenario: Token harness observation rules
- **WHEN** the harness observes the server
- **THEN** it SHALL:
- read token admissibility under `tokenMu`, without side effects;
- read connection liveness from the client side of the transport, and cross-check it against the server's connection map;
- require `Stop` to return `nil`, wait for every client to see its connection closed after `Stop`, and record `server = "stopped"` once `State()` is Stopped.

#### Scenario: Targeted token mutations rejected
- **WHEN** the harness emits its mutations
- **THEN** validation SHALL reject at least:
  - a connection that stays open after its execution's token is revoked (F7);
  - a login that succeeds after revocation;
  - expiry closing a connection;
  - revoking one execution closing another execution's connection;
  - a token that becomes admissible again after revocation;
  - two tokens expiring in one record;
  - a connection that stays open after `Stop` (the pre-fix behaviour of F11)

#### Scenario: Finding F11, Stop keeps authenticated connections open
- **WHEN** `make formal` checks the pre-fix configuration of finding F11, in which `Stop` keeps authenticated connections open
- **THEN** `mutantPreFixF11StopKeepsSessions` (`StopClosesSessions = FALSE`, `mutant_of = "noSessionAfterStopFixed"`, no `finding` key) SHALL produce a counterexample of `NoSessionAfterStop`
- **THEN** `formal/README.md` SHALL record F11 as fixed, naming that mutant as the one that keeps the pre-fix behaviour

#### Scenario: Fixed finding F11, Stop closes every connection
- **WHEN** a real client stays connected through `Stop`
- **THEN** `HostCallbackToken.tla` SHALL mirror the code with `StopClosesSessions = TRUE` in its base constants
- **THEN** `noSessionAfterStopFixed` SHALL pass, and `mutantListenerLeftOpen` SHALL still produce a counterexample of `NoSessionAfterStop`
- **THEN** the Go regression test `TestHostCallbackToken_StopClosesAuthenticatedConnection` SHALL show in the real code that `Stop` returns `nil` and the client's connection is closed

### Requirement: Open findings are not mutations
A behaviour that the base constants accept because of an open finding SHALL NOT be declared as a mutation. This covers a hashless-entry sibling admission (F12). Once a finding is fixed and the base constants no longer accept its behaviour, that behaviour MAY be declared as a targeted mutation, as a connection open after `Stop` is after the fix of F11.

#### Scenario: Behaviour accepted because of an open finding
- **WHEN** a behaviour is accepted by the base constants because of an open finding, such as a hashless-entry sibling admission (F12)
- **THEN** it SHALL NOT be declared as a mutation

#### Scenario: Behaviour of a fixed finding
- **WHEN** the base constants reject a behaviour because its finding was fixed, such as a connection open after `Stop` (F11)
- **THEN** a targeted mutation of an accepted trace MAY declare that behaviour, and validation SHALL reject it
