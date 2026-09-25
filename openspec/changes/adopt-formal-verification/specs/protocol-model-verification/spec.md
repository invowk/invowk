## ADDED Requirements

### Requirement: Lock-to-content integrity model
Invowk SHALL maintain a TLA+ model of lock-to-content integrity across `module sync`, `module vendor`, and discovery. The model SHALL include these attacker actions:
- edit or downgrade the lock file, including rewriting it as version 1.0 without hashes;
- edit a vendored module directory;
- move an upstream tag;
- wipe the module cache.

It SHALL include the code's verification points:
- `cacheModule` with and without an existing cache directory;
- `loadExistingLockHashes`;
- the `ContentHash` checks in `VerifyLockedVendoredModuleHash` and the vendor path;
- discovery's per-parent-lock hash check.

#### Scenario: Loaded content matches a trusted hash
- **WHEN** discovery loads a vendored module in any behaviour
- **THEN** the model SHALL check that exactly one declared lock entry of the caller matches its identity with a non-empty hash equal to its content, or that discovery fails

#### Scenario: Downgrade and fresh-cache paths are characterised
- **WHEN** the attacker downgrades the lock file to version 1.0, or wipes the cache before a sync whose lock holds a known hash
- **THEN** TLC SHALL produce the expected verdicts recorded for findings F4 and F5, and each counterexample SHALL have a Go replay test

#### Scenario: Cross-lock binding
- **WHEN** a vendored copy is hash-checked against its parent's lock, and a scope admits it through the caller's lock entry
- **THEN** the model SHALL check whether both refer to the same content, and the result SHALL be recorded for finding F6

### Requirement: Serverbase lifecycle model
Invowk SHALL maintain a TLA+ model of `internal/core/serverbase`. The model SHALL decompose every `Load`, `Store`, and compare-and-swap exactly as the code performs it, and SHALL mark which steps run under `stateMu`. It SHALL include every exported transition and `SendError`/`CloseErrChannel`, including:
- Created→Stopped without cancellation;
- Failed from any non-terminal state;
- `TransitionToStopped` from any non-terminal state.

TLC SHALL check the model with at least two concurrent callers drawn from all transitions.

#### Scenario: Error channel closed at most once
- **WHEN** TLC explores all interleavings
- **THEN** the error channel SHALL be closed at most once, and no send SHALL occur after the close

#### Scenario: Started channel closed by the winning CAS
- **WHEN** the started channel is closed
- **THEN** it SHALL be closed exactly once, only by the caller whose Starting→Running compare-and-swap succeeded

#### Scenario: Terminal-state overwrite is characterised
- **WHEN** a `TransitionToFailed` caller that observed a non-terminal state interleaves with a lock-free Created→Stopped compare-and-swap
- **THEN** the "terminal states are absorbing" invariant SHALL have the declared verdict `counterexample` for finding F2

#### Scenario: Stop-during-start cancellation
- **WHEN** all callers have returned, a stopper won, and a server context was created
- **THEN** the safety invariant "that context has been cancelled" SHALL have the declared verdict `counterexample` for finding F1
- **THEN** a `FixedOrdering` configuration, which stores the context before the compare-and-swap, SHALL pass the same invariant

#### Scenario: Caller mitigation is recorded
- **WHEN** the correspondence record describes F1 and F2
- **THEN** it SHALL state whether production callers (`HostAccess.Ensure`, the interactive TUI path) can reach each interleaving

### Requirement: SSH host-callback token lifecycle model
Invowk SHALL maintain a TLA+ model of the SSH host-callback token lifecycle: token generation, authentication, revocation, TTL expiry, the cleanup goroutine, server stop, and the success, error, and cancellation paths of container execution.

#### Scenario: No authentication after execution ends
- **WHEN** the execution that owns a token has returned on any path
- **THEN** no later authentication with that token SHALL succeed

#### Scenario: Tokens do not outlive the server
- **WHEN** the server has stopped
- **THEN** every generated token SHALL be revoked or expired, or the counterexample SHALL be recorded for finding F7

#### Scenario: In-flight sessions are characterised
- **WHEN** a token is revoked while a session it authenticated is still open
- **THEN** the model SHALL record whether that session keeps running, and the correspondence record SHALL classify the result

### Requirement: Watch loop model
Invowk SHALL maintain a TLA+ model of `Watcher.Run` covering the following:
- events;
- debounce schedule, reset, and stop, with the `Reset`/`Stop` return values of `time.AfterFunc`;
- the skip-if-busy re-arm;
- wait-group accounting;
- callback errors;
- closed event and error channels;
- context cancellation;
- fatal backend errors;
- the `Ready` channel.

#### Scenario: No overlapping callbacks
- **WHEN** a timer fires while a callback is in flight
- **THEN** no second callback SHALL start before the first returns

#### Scenario: Nothing runs after Run returns
- **WHEN** `Run` returns for any reason
- **THEN** no callback SHALL be running or start afterwards, and the wait group SHALL be zero

#### Scenario: Fatal errors end the loop
- **WHEN** the backend reports an error classified as fatal
- **THEN** `Run` SHALL return an error wrapping it, and an in-flight callback MAY complete before the return

#### Scenario: No lost burst
- **WHEN** an event arrives, and no cancellation, fatal error, callback error, or channel closure occurs
- **THEN** `Arrived(e) ~> (Delivered(e) \/ Terminated)` SHALL hold under weak fairness of the timer, callback-return, and loop-step actions only
- **THEN** the fairness-free twin SHALL fail

#### Scenario: Fake timer conforms to the real timer
- **WHEN** the fake scheduler used by the watch bindings is compared with `time.AfterFunc`
- **THEN** a conformance test SHALL show the same `Reset` and `Stop` return values after firing, before firing, and after stopping

### Requirement: Atomic-write durability characterisation
Invowk SHALL maintain a TLA+ model of `fspath.AtomicWriteFile` over an abstract filesystem that separates visible state from durable state. The model SHALL use these named axioms:
- writes are visible but not durable;
- `fsync(file)` makes content durable;
- rename is atomic in the visible namespace;
- directory entries become durable only after `fsync(dir)`;
- power loss replaces visible state with a nondeterministic durable choice;
- rename atomicity across a crash is an explicit filesystem assumption.

The model SHALL be checked in four configurations, parameterised by `SyncFile` and `SyncDir`.

#### Scenario: Readers never see partial content
- **WHEN** a reader reads the lock file at any step, excluding power loss
- **THEN** it SHALL observe the complete previous content or the complete new content

#### Scenario: Durability configurations
- **WHEN** TLC checks D-Atomic (old or new after power loss) and D-Commit (new after a returned write, then power loss) under each `SyncFile`/`SyncDir` configuration
- **THEN** each configuration SHALL have a declared verdict, the current-code configuration SHALL record finding F3, and the full-sync configuration SHALL pass

#### Scenario: Temp files on returned errors
- **WHEN** an atomic write returns an error
- **THEN** the temp file SHALL be removed, or the remove failure SHALL be joined into the returned error
- **THEN** temp files left by a process crash SHALL be recorded as an expected abstraction, not as a violation

### Requirement: Container retry binding
Invowk SHALL bind `retryWithBackoff` with a rapid test that exhaustively enumerates outcome sequences up to a small attempt bound, including `maxAttempts <= 0`. No TLA+ model is required for this function.

#### Scenario: Retry contract
- **WHEN** the rapid test runs with a fake sleeper
- **THEN** it SHALL check all of the following:
  - `op` runs at most `maxAttempts` times;
  - `maxAttempts <= 0` returns nil without running `op`;
  - a nil error wins over `retry`;
  - a non-retryable error is returned unchanged without sleeping;
  - a cancellation between attempts returns an error wrapping `ctx.Err()` or the sleeper's error;
  - exhaustion returns the last error.

### Requirement: Protocol bindings to code
Each TLA+ model SHALL be bound to the code as follows:
- **rapid state-machine tests** whose actions correspond one-to-one with the model's visible actions, and which reuse its invariant names;
- **replay tests** for every recorded counterexample;
- **trace validation** for the watch loop, serverbase sequential runs, and the atomic write. Sync and tidy are bound by the integrity tests of `verify-locked-module-integrity` and Go replays instead, because `LockIntegrity` abstracts sync into one adversarial action and a trace of it would add little.

Trace harnesses SHALL use the existing seams. The atomic write SHALL use `atomicWriteOps` inside `pkg/fspath`. The watch loop SHALL use `newWithBackend` with an assignable `schedule`, over a temp directory, a fake backend, and a fake timer conformant with `time.AfterFunc`. Trace specs SHALL forbid skipping an observable state: every projection change (or, for concurrent models, every point where all callers are settled) SHALL match the next record.

#### Scenario: No production instrumentation
- **WHEN** the bindings are built
- **THEN** they SHALL live in `_test.go` files that are always compiled, SHALL skip at runtime unless `INVOWK_FORMAL_TRACE_DIR` is set, and SHALL NOT change production code

#### Scenario: Trace acceptance is existential
- **WHEN** a harness emits a trace
- **THEN** it SHALL write a generated `<Model>Traces.tla` constant module, and TLC SHALL check `INVARIANT NotFullyConsumed` with deadlock checking off, because a trace that cannot continue is how a trace is rejected
- **THEN** a violation of that invariant SHALL mean ACCEPTED, and a pass SHALL mean REJECTED

#### Scenario: Targeted trace mutations
- **WHEN** the runner applies semantically targeted trace mutations, each with a declared verdict (for example a callback start while a callback runs, an op before its sleep, or a rename before close)
- **THEN** TLC SHALL reject each mutation declared as invalid, and random drop, duplicate, or reorder mutations SHALL be report-only

#### Scenario: Stress tests assert only passing invariants
- **WHEN** a concurrent stress test runs under `-race`
- **THEN** it SHALL assert only invariants the model verifies as passing, and SHALL bound its iterations under `testing.Short()`
- **THEN** invariants with a recorded counterexample SHALL be listed as model-only evidence

#### Scenario: Cross-platform execution
- **WHEN** protocol bindings run on Linux, macOS, and Windows CI
- **THEN** they SHALL use fake backends, schedulers, sleepers, fetchers, and filesystem operations, so results do not depend on host timer resolution or filesystem semantics
