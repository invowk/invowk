## Context

Trace validation binds three TLA+ models to real code: `Serverbase`, `AtomicWrite`, and `Watch`. This change is written against the machinery that `formal-infra-refinements` lands first:

- `formal/tla/TraceBase.tla`, instantiated by every `*Trace.tla`, with the prefixed operators `TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, and `NotFullyConsumed`;
- `tlatrace.Recorder`, which appends only changed records, and `tlatrace.WriteSuite(t, "<Model>", traces)`, which fails on empty sets and mismatched key sets and writes the sorted `fields`;
- the runner's comparison of `Proj` field names against the recorded `fields`, and its rejection of unknown manifest keys.

`scripts/formal.py traces` still starts one TLC process per trace. It accepts a trace when TLC violates `NotFullyConsumed`, and it requires targeted mutations that TLC rejects.

The other two TLA+ models are bound more weakly:

- **`HostCallbackToken`** is bound by `TestHostCallbackToken_LifecycleMatchesModel`, a rapid test that calls `admitConn` on `recordingConn` stand-ins without a running server. A real SSH client appears only in `TestRevokeTokenClosesAuthenticatedConnection`.
- **`LockIntegrity`** is bound by per-finding replays in `pkg/invowkmod/lock_integrity_formal_test.go`, `internal/app/deps/deps_scope_integrity_test.go`, and the `modulesync` integrity tests. `adopt-formal-verification` concluded that a sync trace "would add little". That judgement predates the F4 and F6 fixes. It also missed that `Sync`, `Vendor`, and `Discover` make claims about file-system side effects, and only a whole-run trace checks those claims.

Code facts the design depends on:

- **Token start and end.** Production starts an execution with `Server.GetConnectionInfo`, which requires a running server, and ends it with `RevokeToken` in `internal/runtime/container_exec.go`. `HostAccess.Stop` is deferred after command execution (`internal/app/commandadapters/host_access.go:86-95`).
- **Expiry is lazy.** `admitConn`, `ValidateToken`, and a 5-minute sweep drop expired tokens. Connections survive expiry.
- **Stop closes no connections.** `Server.Stop` calls `ssh.Server.Shutdown`, which closes the listeners and then waits for connections until its context ends (`charm.land/ssh@v0.4.3/server.go:268`). `doStop` closes no `tokenConn`, and it returns the `context.DeadlineExceeded` from `Shutdown` because that is not a closed-connection error (`server_lifecycle.go:149-156`). The model's `Stop` clears every session. This is **F11**.
- **Sync.** `Resolver.Sync` fails a moved tag with `LockedCommitMismatchError` before it touches the cache. It keeps a tampered cache on failure, and removes a fresh copy whose hash mismatches. It rewrites the lock only on success, always as v2.
- **Vendor.** The CLI vendors through `resolveVendorDependencies`. When a lock exists and the update flag is off, it calls `Resolver.LoadDeclaredFromLock`, whose `loadV2LockFile` → `RequireV2` fails a v1.0 lock before anything is copied (`resolver.go:369-407`). `VendorModules` then removes the destination, copies from the cache, and only afterwards checks the hash (`vendor.go:107-138`). A v2 hash mismatch therefore leaves the copied content behind.
- **Discovery.** Discovery of a vendored copy whose lock entry mismatches or has no hash is a hard error for the whole run (`internal/discovery/discovery_files.go:620`).
- **Admission.** `IsDeclaredLockedCommandSource` admits when the caller's entry has no hash (`pkg/invowkmod/vendored_policy.go:129`). `LoadCommandScopeLock` loads the lock with `LoadLockFile` and no `RequireV2` (`internal/app/commandadapters/dependency_host.go:194`), so a v1.0 lock reaches that branch in production. `check-caller-hash-on-admission` deferred hashless entries to `reject-hashless-lock-entries`, and that change never touched admission. This is **F12**.
- **Packages.** `moduleops` imports `modulesync`. The fake fetcher and `newResolverWithFetcher` are unexported.

Finding ids F11 and F12 were allocated by the cross-change plan. F8–F10 belong to `model-concurrent-lock-writes`. If a finding does not reproduce, it is not recorded and its id stays unused.

## Goals / Non-Goals

**Goals:**

- Record real-code traces for both models and validate them with the shared `TraceBase` machinery: existential acceptance, no skipped observable state.
- Take projections from ground truth (the transport, the file system, and the lock on disk), not from the bookkeeping of the code under test.
- Give each safety property and each fixed finding a targeted mutation that must be rejected.
- Record F11 and F12 in full: a model command next to a passing fix configuration, a seeded mutant, calibration, a Go replay test, and a README row.
- Where a trace shows that the model and the code differ, make the model mirror the code without changing any property.

**Non-Goals:**

- Product fixes for F11 or F12. They come later, after maintainer approval.
- Tracing the container runtime's end paths (`prepareContainerExecution`). The harness calls `RevokeToken` directly.
- Driving `discovery.DiscoverAll` over a full fixture tree (see the D3 trade-off).
- Tracing `tidy`, and batching several traces into one TLC process.

## Decisions

### D1. Harness locations and seams

- **`TestHostCallbackToken_TraceHarness`** lives in `internal/sshserver/token_trace_test.go`, in package `sshserver`. It needs `tokenMu`, `tokens`, and `conns` for a side-effect-free peek, `NewWithClock` with `testutil.FakeClock`, and `golang.org/x/crypto/ssh`.
- **`TestLockIntegrity_TraceHarness`** lives in the external test package `modulesync_test`. The harness and enumeration go in `lock_trace_test.go`; the operation drivers, label maps, and projection go in `lock_trace_ops_test.go`. The split is planned up front so both stay under the 1000-line limit.
  - The package imports only `moduleops` and `invowkmod`, besides `modulesync`. `moduleops` imports `modulesync`, so an external test package avoids a cycle.
  - `export_test.go`, in package `modulesync` and with an SPDX header, exports `NewResolverWithFetcher` and a trace fetcher. The trace fetcher is a copy of `fakeModuleFetcher` that serves one tree per commit.
- *Alternative rejected:* putting the lock harness in `moduleops`. It would need the real `GitFetcher`, which accepts only remote URL prefixes.
- *Alternative rejected:* using real git repositories. The fetcher seam already controls which commit a tag names and which tree that commit serves. Real git adds time per trace and nothing the projection can see.

### D2. HostCallbackToken projection

The projection is flat, so `WriteSuite`'s key-set check and the runner's `Proj`-field comparison cover every field:

```tla
E1 == CHOOSE e \in Execs : TRUE
E2 == CHOOSE e \in Execs \ {E1} : TRUE
Proj == [e1_exec |-> exec[E1], e1_usable |-> token[E1] = "valid", e1_session |-> session[E1],
         e2_exec |-> exec[E2], e2_usable |-> token[E2] = "valid", e2_session |-> session[E2],
         server |-> server]
TraceInit == Init /\ TraceStart(Proj)
TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')
```

`Execs` holds model values, which a generated module cannot name. The model is symmetric in `Execs`, so the fixed `CHOOSE` bijection loses nothing. The nested alternative, and the `tlatrace.Rec` helper it would need, were dropped because the key-set checks cannot see nested keys.

- **`eN_exec`** is the harness's phase: `GetConnectionInfo` sets `running`, and `RevokeToken` sets `ended`. It is the only field that is not ground truth. It stands for the execution, which lives outside `internal/sshserver`.
- **`eN_usable`** is read under `tokenMu`: the token is present and `!clock.Now().After(ExpiresAt)`. The harness does not call `ValidateToken`, because that call drops expired entries. The field is a boolean rather than the four-valued `token` because the code cannot distinguish "revoked" from "expired" after the lazy drop.
- **`eN_session`** is TRUE when some client that authenticated with the token has not seen its connection close (`gossh.Client.Wait` has not returned). The harness also asserts that `len(s.conns[token])` agrees. `EndSession` closes every client of the execution, which keeps the field a function of the model state.
- **`server`** is `"running"` while `State()` is Running. After `Stop`, the harness accepts `context.DeadlineExceeded` from `Stop` and records `"stopped"` once `State()` is Stopped.
- The ghost variables `authEver` and `authAfterEnd` are not projected. A login the model forbids shows up anyway as a `session` step that no action allows. The harness asserts directly that logins the model allows succeed, because a wrongly refused login is only a stutter and existential acceptance cannot detect it.

**Expiry.** The two tokens are issued at different fake-clock times. An `Expire` sets the clock to one second past exactly one token's `ExpiresAt`. Expiring both in one step would be a merged record, which the spec rejects.

**Closes and cost.** `ShutdownTimeout` is at most 100 ms.
- After `RevokeToken` or a client close, the harness waits up to 5 s for `Wait` to return and `conns` to settle. On timeout it records the still-open state, and TLC rejects that trace.
- After `Stop`, it waits a fixed 200 ms settle plus the `conns` cross-check, not the 5 s bound. Under F11 the connection never closes, so the long bound would only add wall time.
- If F11 did not reproduce, `Stop` would use the 5 s bound like the other closes.
- Each enumerated trace runs its own server in a parallel subtest.

**F11 in the model and manifest:**

1. `Stop` becomes `session' = IF StopClosesSessions /\ Mutant /= "listener_left_open" THEN [e \in Execs |-> FALSE] ELSE session`. The base constants set `StopClosesSessions = "FALSE"`, which mirrors the code. The `Auth` guard on `listener_left_open` stays.
2. Add `noSessionAfterStopFixed`: `constants = { StopClosesSessions = "TRUE" }`, `expect = "pass"`, property `NoSessionAfterStop`.
3. Rename `noSessionAfterStop` to `findingF11StopKeepsSessions`: `finding = "F11"`, base constants, `expect = "counterexample"`, `mutant_of = "noSessionAfterStopFixed"`.
4. Re-point `mutantListenerLeftOpen`: `constants = { StopClosesSessions = "TRUE", Mutant = "\"listener_left_open\"" }`, `mutant_of = "noSessionAfterStopFixed"`. It is no longer trivially guarding a property that fails in the base.
5. Add the Go replay `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` to `internal/sshserver/token_formal_test.go`. It uses `ShutdownTimeout = 100ms` and a real client. It asserts that after `Stop` returns the client is still connected and `Stop`'s error wraps `context.DeadlineExceeded`. The later fix inverts it. It gets a row in the correspondence table.
6. Correct the Stop correspondence row's abstraction ("stopping closes the listener and ends open sessions", which is false today) to "closes the listener; open connections stay open (F11)", with the replay test as its binding.

The trace suite validates under one constant set, the base. An open connection after `Stop` is therefore accepted behaviour and is **not** a targeted mutation. `NoSessionAfterStop` stays guarded by the fix configuration and its mutant.

**Reachability, recorded with F11.** Production calls `Stop` after command execution, when `End` has already revoked the tokens and closed their connections. F11 therefore matters only for executions still running at `Stop`, for example when cancellation orders the two differently.

### D3. LockIntegrity projection

The projection is flat and carries every model variable: `remoteCommit`, `lockVer`, `lockCommit`, `lockHash`, `cache`, `vendored`, `pVendored`, `loaded`, `called`, `syncs`. `TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')`.

- **Labels.** The three fixture trees are valid `.invowkmod` directories. `Good` is the tree served at `c1` (`defaultFakeCommit`), `Evil` the tree at `c2`, and `Other` the sibling's pinned version. `ComputeModuleHash` is computed for each once, and every observed hash maps back to a label. An unmapped hash fails the test.
- **Lock fields.** `invowkmod.InspectLockFile` on disk (it parses v1.0) supplies `lockVer`, `lockCommit`, and `lockHash`. `lockHash = "none"` when the entry has no `content_hash`.
- **`cache`, `vendored`, `pVendored`.** Each is `absent` if the directory is missing, otherwise the label of its hash. `cache` is the resolver's cache path for the module, `vendored` is `invowk_modules/<canonical>` under C, and `pVendored` is the sibling's vendored copy.
- **`Vendor`.** `LoadDeclaredFromLock(ctx, requirements)` followed by `moduleops.VendorModules` on its result. This is the update-false path of `resolveVendorDependencies`, the production path when a lock exists.
- **`loaded`.** C's vendored copy is loaded with `invowkmod.Load` and passed to `VerifyLockedVendoredModuleHash` with C's lock entry, as `discoverVendoredModulesWithDiagnostics` does. An error wrapping `ErrContentHashMismatch` or `ErrLockEntryWithoutContentHash` maps to `"rejected"`. Success maps to the loaded copy's label.
- **`called`.** `IsDeclaredLockedCommandSource` runs with C's requirements, C's lock from `LoadLockFile` (as `LoadCommandScopeLock` loads it), and the sibling's copy. `true` maps to `pVendored`'s label and `false` to `"rejected"`.
- **Initial states.** A setup sync runs before the first record and is not counted. Then come the optional setup steps: wiping the cache, vendoring, choosing the sibling content, and a legacy downgrade. The downgrade strips `content_hash` and sets version `"1.0"`, as `TestSyncRefetchesWhenLockedEntryHasNoHash` does. Together they cover every `Init` state under `LegacyLock = TRUE`.
- **Bounds.** At most two syncs. `Discover` runs at most once. `CallViaSibling` runs at most once, and only while C has no vendored copy. Content is always a function of the commit. Changed content at an unchanged commit has no model counterpart and stays covered by `TestSyncFreshCacheRejectsChangedContent`.
- *Trade-off, and why `DiscoverAll` is out of scope:* checking with the verification function is cheap. It does not check how discovery finds the lock entry or applies its declaration gate, but `internal/discovery` tests keep that coverage. A full invowkfile tree per trace would multiply fixture cost.

### D4. LockIntegrity model changes

**Alignment with the code (no property changes):**

1. **Sync failure keeps the cache on a commit mismatch:** `cache' = IF ~commitOk \/ useCache THEN cache ELSE "absent"`.
2. **Vendor, split by lock version:**
   - **v2 hash mismatch** (`lockHash /= "none" /\ cache /= lockHash`): the copy happens and then the check fails, so `vendored' = cache`.
   - **v1/hashless** (`lockHash = "none"` under `RejectUnhashed`): `RequireV2` fails first and nothing changes. There is no step, as today.

   The leftover content is a calibration note, not a finding, because discovery still rejects it.

These two changes have permanent characterisation tests besides the trace harness:
- `TestSyncCommitMismatchKeepsCache` in `internal/app/modulesync/resolver_integrity_test.go`;
- `TestVendorHashMismatchLeavesCopiedContent` in `internal/app/moduleops/vendor_test.go`.

Both are named in the correspondence table.

**F12:**

3. New constant `AdmitRequiresCallerHash`, `"FALSE"` in the base constants (mirroring the code). `CallViaSibling` becomes:

   ```tla
   called' = IF CheckCallerHash /\ (lockHash /= "none" \/ AdmitRequiresCallerHash)
                /\ pVendored /= (IF Mutant = "compare_with_sibling_lock" \/
                                    (lockHash = "none" /\ Mutant = "hashless_uses_sibling_lock")
                                 THEN pVendored ELSE lockHash)
             THEN "rejected" ELSE pVendored
   ```

   Manifest commands:
   - `findingF12HashlessSiblingAdmission`: `finding = "F12"`, `LegacyLock = TRUE`, `expect = "counterexample"` on `SiblingCallTrusted`, `mutant_of = "f12AdmitRequiresCallerHash"`.
   - `f12AdmitRequiresCallerHash`: `AdmitRequiresCallerHash = TRUE`, `LegacyLock = TRUE`, `expect = "pass"`.
   - `mutantHashlessUsesSiblingLock`: the fix configuration plus `Mutant = "hashless_uses_sibling_lock"`, which falls back to the sibling's own lock when the caller has no hash. `expect = "counterexample"`, `mutant_of = "f12AdmitRequiresCallerHash"`.

   The Go replay `TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy` in `pkg/invowkmod/lock_integrity_formal_test.go` asserts today's behaviour. It builds a caller entry without a hash and a sibling copy of different content, and expects `IsDeclaredLockedCommandSource` to return true. The later fix inverts it.

After these changes, every existing command must keep its declared verdict, the existing mutants must still produce counterexamples, and the calibration notes record each change.

### D5. Targeted mutations

Each mutation is a one-step edit of an accepted real trace, taken from three families:

1. **Violation of a safety property:**
   - `NoAuthAfterExecution` (a login after revocation);
   - `NoSessionAfterExecution`;
   - `OwnLoadTrusted` (a tampered vendored copy that is loaded);
   - `SiblingCallTrusted` under a v2 caller hash.
2. **Pre-fix behaviour of a fixed finding:**
   - F7: revocation leaves the connection open;
   - F5: a sync after a moved tag rewrites the lock to `c2` and `Evil`;
   - F4: a hashless lock whose `Evil` vendored copy is loaded, and a v1.0 lock that vendors;
   - F6: `Other` admitted under C's `Good` hash.
3. **A merged or impossible step:**
   - two tokens expiring together;
   - revoking e1 closing e2's connection;
   - expiry closing a connection;
   - a revoked token becoming usable again;
   - a failed sync rewriting the lock;
   - a tamper and a discovery landing in one record.

Behaviours that the base constants accept because of an open finding are not mutations: a connection open after `Stop` (F11), and hashless sibling admission (F12). Each mutation carries a comment that names its property or finding. The harness fails if a mutation equals an accepted trace. Random drop, duplicate, and reorder mutations stay report-only.

### D6. Enumeration budget

- **HostCallbackToken.** Every sequence of `Start(e1)` followed by up to three operations from `{Auth, End, EndSession, Expire, Stop}` on e1. Curated two-execution traces are added on top: overlap, revoking one execution while the other's session is open, staggered expiry, and `Stop` with both executions active.
- **LockIntegrity.** Every sequence of up to two operations from the default initial state. Every single operation from each other initial state. Curated finding scenarios of four to six steps: the F4 refetch, the F5 moved tag, the F6 sibling, F12, and a tampered cache followed by sync, vendor, and discover.

Operations that the model disables are still performed whenever the code allows the call, and the projection must show no change.

Different sequences often produce identical traces, because disabled operations only stutter. The harness therefore de-duplicates identical accepted traces, and identical mutations, before `WriteSuite`, and logs the unique counts. Each suite is capped at about 250 unique traces.

### D7. Manifest and runner

```toml
[[trace]]
name = "HostCallbackToken"
package = "./internal/sshserver/"

[[trace]]
name = "LockIntegrity"
package = "./internal/app/modulesync/"
constants = { LegacyLock = "TRUE" }
```

- `TraceSuite` gains `constants`, and `constants` joins the allowed `[[trace]]` keys in `formal-infra-refinements`' unknown-key check. `promote-formal-ci-gate` will add `budget_seconds` to the same dataclass and key set.
- `check_trace` merges `model.constants | suite.constants | {TraceSet, TraceIndex}`. An override of a key missing from the base constants fails the run.
- `LegacyLock = TRUE` only widens `Init`.
- *Alternative rejected:* flipping `LegacyLock` in the base constants. That would change the verdicts of existing commands.

The correspondence tables name the new harnesses, the F11 and F12 replay tests, and the two characterisation tests as bindings. `scripts/formal.py correspondence`, and the completeness guard of `mutation-test-formal-bindings`, fail if any of them is renamed.

## Risks / Trade-offs

- **[Risk] Real-client traces flake on slow CI.** → Closes are awaited with a bound, and `formal-traces` is not part of `make test`. The harness uses loopback only, `ShutdownTimeout` is at most 100 ms, and `Stop` uses a fixed settle.
- **[Risk] Extra TLC starts slow `make formal-traces` (today 39–67 s in CI).** → De-duplicate and cap the traces (D6). Task 5.4 records unique counts and wall time for `promote-formal-ci-gate`'s budgets. If the cost is too high, propose a follow-up that checks several traces per TLC process.
- **[Risk] `eN_exec` is harness bookkeeping, so a harness bug could fabricate acceptance.** → It changes only on the harness's own calls. Every other field is ground truth.
- **[Risk] A 200 ms Stop settle could hide a slow close if F11 is later fixed.** → The fix change inverts the replay test and switches `Stop` back to the 5 s bound. The design records that dependency.
- **[Trade-off] The boolean `usable` hides the revoked/expired distinction.** The code does not keep it either, and the rapid binding still checks the four-valued state.
- **[Trade-off] Model changes (D4) in a change about bindings.** They change no property, and without them the real traces are rejected.

## Migration Plan

This change adds only test files, formal files, and runner edits. It changes no production code and no user-facing behaviour. It can be implemented only after `formal-infra-refinements` is merged. Rollback removes the two `[[trace]]` entries. The F11 and F12 records stay, because they describe the code.

## Notes on review edits

- **Review item 3 asks for permanent tests of the D4 checks, or for the trace harness to be named as their binding.** Both are done: permanent characterisation tests, plus the harness.
- **The prefixed `TraceBase` names.** This design uses the prefixed names from the cross-change decision (`TraceStart`, `TraceAdvanceOnChange`, …). The current `formal-infra-refinements` design text still shows unprefixed names (`Start`, `AdvanceOnChange`). Whichever names that change lands are the ones to use.

## Open Questions

1. The `adopt-formal-verification` protocol-binding sentence is edited in place by task 5.3, because that capability has not been archived. If it is archived first, this change needs a MODIFIED delta instead.
