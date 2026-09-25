## 0. Prerequisite

- [ ] 0.1 Confirm that `formal-infra-refinements` is merged, then rebase. It provides `TraceBase.tla` with the prefixed operators, `tlatrace.Recorder`/`WriteSuite`, the `Proj`-field check, and the unknown-manifest-key checks. Use the operator names it actually lands

## 1. Findings and characterisation (permanent tests)

- [ ] 1.1 Add `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` (F11) to `internal/sshserver/token_formal_test.go`. Use `ShutdownTimeout = 100ms` and a real `x/crypto/ssh` client. It asserts today's behaviour: after `Stop` returns, the client is still connected, and `Stop`'s error wraps `context.DeadlineExceeded`. The later fix inverts it. If it does not reproduce, drop F11 (the id stays unused) and use the 5 s close bound for `Stop`
- [ ] 1.2 Add the characterisation tests `TestSyncCommitMismatchKeepsCache` (`modulesync`) and `TestVendorHashMismatchLeavesCopiedContent` (`moduleops`). The second goes through `LoadDeclaredFromLock` and then `VendorModules`
- [ ] 1.3 Add `TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy` (F12) to `pkg/invowkmod/lock_integrity_formal_test.go`. It asserts today's behaviour: a caller entry without a hash admits a sibling copy of different content
- [ ] 1.4 Name every test from 1.1–1.3 in its model's correspondence table

## 2. Shared machinery

- [ ] 2.1 Add `constants` to `TraceSuite` in `scripts/formal.py` and to the allowed `[[trace]]` keys (`name`, `package`, `constants`). Merge the constants as `model | suite | TraceSet/TraceIndex`, and fail closed on an override key missing from the model's base constants
- [ ] 2.2 Add `scripts/test_formal.py` cases for: the override applied, an unknown override rejected, and an unknown `[[trace]]` key rejected. Then run `make test-scripts`

## 3. HostCallbackToken model and suite

- [ ] 3.1 In `HostCallbackToken.tla`, add `StopClosesSessions` and rewrite `Stop` as `session' = IF StopClosesSessions /\ Mutant /= "listener_left_open" THEN [e \in Execs |-> FALSE] ELSE session`. Set the base to `StopClosesSessions = "FALSE"`
- [ ] 3.2 Update the manifest:
  - add `noSessionAfterStopFixed` (`StopClosesSessions = TRUE`, pass);
  - rename `noSessionAfterStop` to `findingF11StopKeepsSessions` (`finding = "F11"`, counterexample, `mutant_of = "noSessionAfterStopFixed"`);
  - re-point `mutantListenerLeftOpen` to `{StopClosesSessions = TRUE, Mutant = listener_left_open}` with `mutant_of = "noSessionAfterStopFixed"`;
  - correct the Stop correspondence row's abstraction;
  - update the calibration.

  Then run `make formal-tla`
- [ ] 3.3 Write `formal/tla/HostCallbackTokenTrace.tla` on `INSTANCE TraceBase`. It defines `E1`/`E2` with `CHOOSE`, the flat `Proj` from D2, `TraceInit == Init /\ TraceStart(Proj)`, `TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')`, and `TraceSpec`, and nothing else
- [ ] 3.4 Write `TestHostCallbackToken_TraceHarness` in `internal/sshserver/token_trace_test.go`:
  - a started server per trace, in parallel subtests, with a fake clock and `ShutdownTimeout <= 100ms`;
  - real clients;
  - a side-effect-free `usable` peek and the `conns` cross-check;
  - a 5 s wait for revoke and session-end closes;
  - a 200 ms settle after `Stop`, accepting `context.DeadlineExceeded`;
  - staggered expiry.

  Record through `tlatrace.Recorder` and write with `tlatrace.WriteSuite(t, "HostCallbackToken", ...)`
- [ ] 3.5 Enumerate the D6 sequences and assert that allowed logins succeed. Add the D5 mutations with comments; do not add an open-after-`Stop` mutation. De-duplicate traces and mutations, and fail when a mutation equals an accepted trace
- [ ] 3.6 Add a `[[trace]]` entry for `HostCallbackToken`. Run `scripts/formal.py traces HostCallbackToken`: every recorded trace must be accepted and every mutation rejected

## 4. LockIntegrity model and suite

- [ ] 4.1 Edit `LockIntegrity.tla` as D4 describes:
  - a sync failure keeps the cache on a commit mismatch;
  - a v2 vendor hash mismatch leaves the copy (`vendored' = cache`), while a hashless or v1 lock takes no vendor step;
  - add `AdmitRequiresCallerHash` (base `FALSE`) and the `hashless_uses_sibling_lock` mutant in `CallViaSibling`.

  Update the header, the correspondence table, and the calibration (which records the vendor leftover as a note, not a finding)
- [ ] 4.2 Add `findingF12HashlessSiblingAdmission`, `f12AdmitRequiresCallerHash`, and `mutantHashlessUsesSiblingLock` to the manifest. Run `make formal-tla` and confirm that every existing command keeps its verdict
- [ ] 4.3 Add `internal/app/modulesync/export_test.go` (SPDX header). It exports `NewResolverWithFetcher` and a trace fetcher that serves the `c1` or `c2` tree by commit
- [ ] 4.4 Write `formal/tla/LockIntegrityTrace.tla` on `INSTANCE TraceBase`. It defines the flat full-variable `Proj`, `TraceInit`, `TraceNext == Next /\ TraceAdvanceOnChange(Proj, Proj')`, and `TraceSpec`, and nothing else
- [ ] 4.5 Write the harness in package `modulesync_test`:
  - `lock_trace_test.go`: harness, enumeration, mutations;
  - `lock_trace_ops_test.go`: fixture trees, which are valid `.invowkmod` directories loaded with `invowkmod.Load`; label maps; initial-state setup, including the legacy downgrade; operation drivers (`Sync`, `LoadDeclaredFromLock` + `VendorModules`, `VerifyLockedVendoredModuleHash`, `IsDeclaredLockedCommandSource` over `LoadLockFile`, and the attacker actions); and the projection.

  Record through `tlatrace.Recorder` and write with `WriteSuite(t, "LockIntegrity", ...)`
- [ ] 4.6 Enumerate the D6 sequences and the curated F4/F5/F6/F12 scenarios. Add the D5 mutations; do not add an F12 admission mutation. De-duplicate traces and mutations, and fail when a mutation equals an accepted trace
- [ ] 4.7 Add a `[[trace]]` entry for `LockIntegrity` with `constants = { LegacyLock = "TRUE" }`. Run `scripts/formal.py traces LockIntegrity`

## 5. Calibration and documentation

- [ ] 5.1 Calibrate by seeding these defects in the real code one at a time, and confirm that `make formal-traces` or the replay tests fail:
  - `RevokeToken` not closing connections;
  - `admitConn` ignoring expiry;
  - `Sync` rewriting the lock on a commit mismatch;
  - admission ignoring C's hash;
  - `VendorModules` skipping its hash check.

  Record the results in each model's `calibration`
- [ ] 5.2 Pass `scripts/formal.py correspondence` with the new bindings
- [ ] 5.3 Update `formal/README.md`:
  - Models table and Trace validation paragraph;
  - Findings rows for F11 (Open) and F12 (Open);
  - the F11 row's reachability note: production calls `Stop` after executions end, so F11 matters only for sessions of executions still running at `Stop`, for example under a different cancellation ordering.

  Also update `.agents/skills/formal-verification/SKILL.md`, and the Makefile help text for `formal-traces` (currently "phase 3; currently a no-op"). Replace the "a trace of sync would add little" sentence in `openspec/changes/adopt-formal-verification/specs/protocol-model-verification/spec.md`
- [ ] 5.4 Record the unique accepted and rejected trace counts per suite, and the `make formal-traces` wall time before and after, in the calibration notes for `promote-formal-ci-gate`'s budgets

## 6. Verification

- [ ] 6.1 `make formal`, `make formal-traces`, `python3 scripts/test_formal.py`
- [ ] 6.2 `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make license-check`, `make check-agent-docs`
- [ ] 6.3 `openspec validate trace-validate-token-and-lock --strict`
