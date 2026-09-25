## ADDED Requirements

### Requirement: Trace-validated models
Invowk SHALL trace-validate these TLA+ models against real code: `Serverbase`, `AtomicWrite`, `Watch`, `HostCallbackToken`, and `LockIntegrity`. A model added later SHALL declare whether it is trace-validated or bound by characterisation tests.

Each trace-validated model SHALL have a `[[trace]]` entry in `formal/manifest.toml` that names the model and its harness package. The harness SHALL:
- be named `Test<Model>_TraceHarness`;
- record through `tlatrace.Recorder` and write through `tlatrace.WriteSuite`;
- skip unless `INVOWK_FORMAL_TRACE_DIR` is set.

Its trace spec SHALL be `formal/tla/<Model>Trace.tla`, SHALL `INSTANCE TraceBase`, and SHALL define only `Proj`, its step constraint, `TraceInit`, `TraceNext`, and `TraceSpec`, as the formal-infrastructure-generation capability requires.

#### Scenario: Suites run under make formal-traces
- **WHEN** `make formal-traces` runs
- **THEN** it SHALL record and check the `HostCallbackToken` and `LockIntegrity` suites together with the existing three
- **THEN** it SHALL fail when either suite has no accepted traces, has no targeted mutations, accepts a mutation, rejects a recorded trace, or records fields that differ from its `Proj`

#### Scenario: Ordinary test runs
- **WHEN** `make test` runs without `INVOWK_FORMAL_TRACE_DIR`
- **THEN** both new harnesses SHALL skip, while the F11 and F12 replay tests and the characterisation tests SHALL run

### Requirement: No skipped observable state in token and lock traces
`HostCallbackTokenTrace.tla` and `LockIntegrityTrace.tla` SHALL consume records with `TraceAdvanceOnChange(Proj, Proj')`:
- a model step that changes the projection SHALL match the next record;
- a step that leaves the projection unchanged SHALL keep the trace position.

Both harnesses SHALL observe the projection after every operation. Duplicate records are dropped by the recorder that the formal-infrastructure-generation capability defines. Each projection SHALL be flat: every observed variable is a top-level field, so the suite writer's key-set check covers every field.

#### Scenario: Two changes merged into one record
- **WHEN** a record differs from its predecessor in a way no single model step produces, for example:
  - two tokens expiring together;
  - a lock rewrite and a discovery result landing in one record
- **THEN** validation SHALL reject the trace

### Requirement: Host-callback token trace
`TestHostCallbackToken_TraceHarness` in `internal/sshserver` SHALL drive a started `Server` that uses a fake clock and a `ShutdownTimeout` of at most 100 ms. Its operations and the model actions they stand for SHALL be:

| Operation | Model action |
|---|---|
| `GetConnectionInfo` | `Start` |
| login with a real `golang.org/x/crypto/ssh` client over loopback | `Auth` |
| `RevokeToken` | `End` |
| closing the client | `EndSession` |
| setting the fake clock past one token's `ExpiresAt` and before every other token's | `Expire` |
| `Stop` | `Stop` |

The projection SHALL have these fields:
- for each of two executions `eN`:
  - `eN_exec`: the harness's execution phase;
  - `eN_usable`: whether the server would admit the execution's token now;
  - `eN_session`: whether an authenticated connection of that execution is still open;
- `server`: the server state.

The harness SHALL:
- read token admissibility under `tokenMu`, without side effects;
- read connection liveness from the client side of the transport, and cross-check it against the server's connection map;
- accept a `context.DeadlineExceeded` error from `Stop`, and record `server = "stopped"` once `State()` is Stopped.

#### Scenario: Real traces accepted
- **WHEN** the harness enumerates its bounded operation sequences
- **THEN** `HostCallbackTokenTrace.tla` SHALL accept every recorded trace
- **THEN** the harness SHALL assert directly that every login the model allows with an admissible token succeeds

#### Scenario: Targeted token mutations rejected
- **WHEN** the harness emits its mutations
- **THEN** validation SHALL reject at least:
  - a connection that stays open after its execution's token is revoked (F7);
  - a login that succeeds after revocation;
  - expiry closing a connection;
  - revoking one execution closing another execution's connection;
  - a token that becomes admissible again after revocation;
  - two tokens expiring in one record

#### Scenario: Finding F11, Stop keeps authenticated connections open
- **WHEN** a real client stays connected through `Stop`
- **THEN** `HostCallbackToken.tla` SHALL mirror the code with `StopClosesSessions = FALSE` in its base constants
- **THEN** `findingF11StopKeepsSessions` SHALL produce a counterexample of `NoSessionAfterStop`, and `noSessionAfterStopFixed` (`StopClosesSessions = TRUE`) SHALL pass, guarded by `mutantListenerLeftOpen`
- **THEN** the Go replay test `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` SHALL reproduce the finding in the real code
- **THEN** `formal/README.md` SHALL record F11 as open, with its production reachability
- **THEN** the product fix SHALL be left to a separate change

### Requirement: Lock-integrity trace
`TestLockIntegrity_TraceHarness` SHALL live in the external test package of `internal/app/modulesync`. It SHALL use the resolver's fake-fetcher seam, which `export_test.go` exports to tests. Its operations and the model actions they stand for SHALL be:

| Operation | Model action |
|---|---|
| `Resolver.Sync` | `Sync` |
| `Resolver.LoadDeclaredFromLock` followed by `moduleops.VendorModules` on its result (the update-false path of `resolveVendorDependencies`) | `Vendor` |
| `invowkmod.VerifyLockedVendoredModuleHash` on C's vendored copy, loaded with `invowkmod.Load`, against C's lock entry | `Discover` |
| `invowkmod.IsDeclaredLockedCommandSource` over C's lock, read from disk with `invowkmod.LoadLockFile`, and a sibling's vendored copy | `CallViaSibling` |
| re-pointing the fetcher at the `c2` commit and tree | `MoveTag` |
| replacing the cache or the vendored copy with the `Evil` tree | `TamperCache`, `TamperVendor` |
| removing the cache entry | `WipeCache` |

The projection SHALL be flat and computed from the filesystem and from call results:
- `lockVer`, `lockCommit`, and `lockHash` from the lock file on disk;
- `cache`, `vendored`, and `pVendored` from `invowkmod.ComputeModuleHash` of each directory;
- `loaded` and `called` from the discovery-check and admission results;
- `remoteCommit` and `syncs` from the fetcher and the harness.

Real hashes and commits SHALL map to the model's labels (`Good`, `Evil`, `Other`, `c1`, `c2`, `none`, `absent`). An unmapped hash SHALL fail the harness. The suite SHALL override `LegacyLock = "TRUE"`.

#### Scenario: Real traces accepted
- **WHEN** the harness enumerates its initial states and bounded operation sequences
- **THEN** the harness SHALL respect the model's bounds: at most two syncs, one discovery, and one sibling call made only while C has no vendored copy
- **THEN** `LockIntegrityTrace.tla` SHALL accept every recorded trace

#### Scenario: Targeted lock mutations rejected
- **WHEN** the harness emits its mutations
- **THEN** validation SHALL reject at least:
  - a sync after a moved tag that rewrites the lock to `c2` and `Evil` (F5);
  - a hashless lock whose `Evil` vendored copy is loaded (F4);
  - a sibling copy of `Other` admitted under C's `Good` hash (F6);
  - a tampered vendored copy that is loaded;
  - a failed sync that rewrites the lock;
  - a v1.0 lock that vendors

#### Scenario: Model mirrors observed code paths
- **WHEN** the real code differs from `LockIntegrity.tla` on a path the model covers
- **THEN** the model SHALL be changed to mirror the code, every existing command SHALL keep its declared verdict, and the header SHALL record the difference
- **THEN** this SHALL cover a commit mismatch that keeps the cache, and a failed v2 hash check that leaves the copied content in `invowk_modules/`

### Requirement: Finding F12, hashless caller entries admit sibling copies
Command-scope admission admits a sibling's vendored copy when the caller's lock entry has no content hash. `LockIntegrity.tla` SHALL characterise this defect with an `AdmitRequiresCallerHash` constant, set to `FALSE` in its base constants to mirror the code.

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `findingF12HashlessSiblingAdmission` (`LegacyLock = TRUE`) SHALL produce a counterexample of `SiblingCallTrusted`
- **THEN** `f12AdmitRequiresCallerHash` (`AdmitRequiresCallerHash = TRUE`, `LegacyLock = TRUE`) SHALL pass, and a seeded mutant with `mutant_of` pointing at it SHALL produce a counterexample

#### Scenario: Replay in the real code
- **WHEN** `make test` runs
- **THEN** `TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy` SHALL assert today's behaviour: `IsDeclaredLockedCommandSource` returns true for a sibling copy of different content under a hashless caller entry
- **THEN** `formal/README.md` SHALL record F12 as open, and the product fix SHALL be left to a separate change

### Requirement: Targeted mutation provenance
Each targeted mutation SHALL be a small edit of an accepted real trace. It SHALL carry a comment that names the property or finding it violates.

The harness SHALL:
- de-duplicate identical accepted traces, and identical mutations, before writing;
- report the number of unique traces;
- fail when a mutation equals an accepted trace.

A behaviour that the base constants accept because of an open finding SHALL NOT be declared as a mutation. This covers a connection open after `Stop` (F11) and a hashless-entry sibling admission (F12).

#### Scenario: Mutation that the code actually produces
- **WHEN** a declared mutation is identical to a recorded real trace
- **THEN** the harness SHALL fail before any trace is written

### Requirement: Suite constant overrides
A `[[trace]]` entry SHALL allow only the keys `name`, `package`, and `constants`, where `constants` is an inline table. The runner SHALL merge constants in this order:
1. the model's base constants;
2. the suite's overrides;
3. `TraceSet` and `TraceIndex`.

The runner SHALL fail closed on an unknown `[[trace]]` key, and on an override of a constant that the model's base constants do not define.

#### Scenario: Unknown override or key
- **WHEN** a `[[trace]]` entry overrides a constant absent from `[model.constants]`, or has a key other than `name`, `package`, or `constants`
- **THEN** `scripts/formal.py traces` SHALL exit non-zero, and `scripts/test_formal.py` SHALL cover both paths
