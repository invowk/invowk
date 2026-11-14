## Why

`HostCallbackToken` and `LockIntegrity` are checked by TLC but bound to the code only by rapid tests and Go replays. The rapid token test calls `admitConn` on recording connections, and the lock replays each cover one finding. Nothing checks that a whole run of the real code is a behaviour of the model. The other three TLA+ models (`Serverbase`, `AtomicWrite`, `Watch`) have that check. The fixes for F4, F6, and F7 changed exactly the code these two models describe. Reading that code while designing this change also exposed two new defects, F11 and F12.

## What Changes

- **Token harness.** A new `TestHostCallbackToken_TraceHarness` in `internal/sshserver` drives a started `Server` that uses a fake clock and logs in with a real `golang.org/x/crypto/ssh` client over loopback. Its operations are generate, login, revoke, session end, expiry, and stop. It records a flat projection: token usability, open authenticated connections, execution phase, and server state.
- **Lock harness.** A new `TestLockIntegrity_TraceHarness` in the external test package of `internal/app/modulesync` runs four real operations:
  - `Resolver.Sync`;
  - vendoring, through `LoadDeclaredFromLock` and then `VendorModules`;
  - the discovery-time check of a vendored copy;
  - command-scope admission through a sibling's copy.

  Its fixtures are local directory trees, served by commit through the resolver's fetcher seam (exported via `export_test.go`). The harness re-points the fetcher from `c1` to `c2` and tampers with the cache and vendored copies. Real content hashes map to the model's `Good`, `Evil`, and `Other`.
- **Trace specs.** New `formal/tla/HostCallbackTokenTrace.tla` and `formal/tla/LockIntegrityTrace.tla` instantiate `formal-infra-refinements`' `TraceBase` and advance on every projection change. A trace cannot skip an observable state.
- **Targeted mutations.** Each harness emits targeted mutations, recorded and written through `tlatrace.Recorder` and `tlatrace.WriteSuite`. Every safety property and every fixed finding has one, and validation must reject each.
- **Manifest.** Two new `[[trace]]` entries run under `make formal-traces`. A trace entry may now override constants: `LockIntegrity` records hashless-lock scenarios, and the base constants exclude them.
- **Finding F11.** `Server.Stop` does not close open SSH sessions, because `ssh.Server.Shutdown` waits for connections and closes none. The change records it as follows:
  - a `finding = "F11"` command next to a passing fix configuration;
  - a `StopClosesSessions` constant in `HostCallbackToken.tla`;
  - a Go replay test;
  - a `formal/README.md` row.
- **Finding F12.** Command-scope admission admits a sibling's vendored copy when the caller's lock entry has no content hash (`IsDeclaredLockedCommandSource`), and command-scope lock loading still parses v1.0 locks. The change records it as follows:
  - an `AdmitRequiresCallerHash` constant in `LockIntegrity.tla`;
  - a finding command, a fix configuration, and a seeded mutant;
  - a Go replay test;
  - a README row.
- **Model alignment.** `LockIntegrity.tla` changes in two places so that it mirrors the code. A sync that fails on a commit mismatch keeps the cache. A v2 vendor that fails its hash check leaves the copied content behind, which is a calibration note because discovery still rejects that content. No property changes.
- **Docs.** The correspondence tables, `formal/README.md`, the formal-verification skill, and the Makefile help text list the new suites. The `adopt-formal-verification` sentence saying a sync trace "would add little" is reversed.
- The finding ids F11 and F12 were allocated by the cross-change plan. F8–F10 belong to `model-concurrent-lock-writes`. Product fixes for F11 and F12 are out of scope.

## Capabilities

### New Capabilities

- `protocol-trace-validation`: which TLA+ models are trace-validated against real code, what the `HostCallbackToken` and `LockIntegrity` traces project, how targeted mutations are chosen, how trace suites override constants, and the F11 and F12 finding records.

### Modified Capabilities

(none; `protocol-model-verification` has not been archived into `openspec/specs/` yet. Its trace sentence is updated in place by task 5.3.)

## Impact

- **Requires `formal-infra-refinements` to be merged first:**
  - `TraceBase.tla`, with the prefixed operators `TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, and `NotFullyConsumed`;
  - `tlatrace.Recorder` and `WriteSuite`;
  - the runner's `Proj`-field comparison and its unknown-manifest-key checks.
- **Test-only Go files:**
  - `internal/sshserver/token_trace_test.go`;
  - `internal/app/modulesync/lock_trace_test.go`, `lock_trace_ops_test.go`, and `export_test.go`;
  - replay tests in `internal/sshserver/token_formal_test.go` and `pkg/invowkmod/lock_integrity_formal_test.go`;
  - two characterisation tests, in `modulesync` and `moduleops`.
- No production code changes.
- **Formal files and docs:**
  - `formal/tla/HostCallbackToken.tla`, `LockIntegrity.tla`, and the two trace specs;
  - `formal/manifest.toml`;
  - `scripts/formal.py` and `scripts/test_formal.py`, for trace-suite constants;
  - `formal/README.md`, `.agents/skills/formal-verification/SKILL.md`, and the Makefile help text.
- `make formal-traces` takes longer, because the runner starts one TLC process per trace. Each harness de-duplicates its traces and caps their count. The unique-trace count and the wall time are recorded for `promote-formal-ci-gate`'s budgets.
