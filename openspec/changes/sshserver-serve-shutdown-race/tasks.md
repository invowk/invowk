# Tasks

All paths are under the worktree `/var/home/danilo/Workspace/github/invowk/invowk-sshrace`. New tests go in a new file, `internal/sshserver/server_lifecycle_race_test.go`, so `server_test.go` (659 lines) stays well under the 1000-line limit.

## 1. Negative control on the base

- [ ] 1.1 In a throwaway worktree of 36f85fd6 under this stage's scratch directory, add only the D6 seam (the `hooks *lifecycleHooks` field and the `beforeServe` call in `serve()` after it reads `srv` and `listener`) and the D7 regression test `TestServerStopWhileServeLoopDelayed` (waiting for `State() == serverbase.StateStopping`, so it compiles there). Run `go test -race -run TestServerStopWhileServeLoopDelayed -count=20 ./internal/sshserver/` and record in this task the command, the base SHA and the `WARNING: DATA RACE` frames (`charm.land/ssh@v0.4.3/server.go:275` against `:312`), or the failure count if not every run fails. Remove the throwaway worktree with `git worktree remove --force`. Done when the failure is recorded here.

## 2. Readiness at the first Accept (D1, D2, D6, D8)

- [ ] 2.1 Add the nil-safe serve gate type and the `lifecycleHooks` seam (`beforePublish func(net.Addr)`, `beforeServe func()`) to `internal/sshserver` (`server.go` fields, helpers in `server_lifecycle.go`). Verify with unit tests: a nil gate's open and wait are no-ops, open is idempotent, wait returns false when its context ends first.
- [ ] 2.2 Wrap the listener passed to `srv.Serve` so its first `Accept` calls `TransitionToRunning` and opens the gate; remove the early `TransitionToRunning` from `serve()`; open the gate in a `defer` at the top of `serve()`. Add the D8 comment naming the upstream defect by behaviour (no TODO marker, no URL). Verify with `TestServerStartWaitsForServeLoop`: with `beforeServe` holding, `Start` has not returned and `State()` is `Starting`; after release `Start` returns `nil` and `State()` is `Running`.
- [ ] 2.3 Cover every exit path of `serve()`: extend `TestServerServeFailureTransitionsToFailed` (literal construction, no gate) to still pass, and add a test that calls `serve()` on a literal `Server` with a gate and a nil `srv`, asserting the gate opens. Check: `go test -race -run 'TestServerStartWaitsForServeLoop|TestServerServe|TestServeGate' ./internal/sshserver/`.

## 3. Stop ordering (D3, D7)

- [ ] 3.1 In `doStop`, take the gate with `srv` and `listener`, keep closing the listener first, wait for the gate with `shutdownCtx` before `srv.Shutdown`, and on a timeout log, skip `Shutdown` and return an error wrapping `context.DeadlineExceeded`. Verify with `TestServerStopWhileServeLoopDelayed` (D7): `Stop` returns `nil`, `Start` returns an error matching `errStoppedDuringStartup`, the bound port refuses a TCP dial, `Wait()` returns at once, and `-race` is clean.
- [ ] 3.2 Add `TestServerStopTimesOutBeforeServeLoop`: `ShutdownTimeout` of 50 ms, `beforeServe` held until well after the timeout, then released; `Stop` returns an error matching `context.DeadlineExceeded` only after the serve goroutine exited, the port refuses a dial, and `-race` is clean. Check: `go test -race -run 'TestServerStop' -count=20 ./internal/sshserver/`.

## 4. Stop during Start (D4, D5)

- [ ] 4.1 Add the `stopBegun` flag set by `doStop` under `srvMu`; move the publication of `listener`, `addr`, `srv` and the gate and both `AddGoroutine` calls into one `srvMu` critical section in `Start` that checks `stopBegun`, closes the bound listener and returns `errStoppedDuringStartup` when set. Verify with `TestServerStopBeforeStartPublishes`: `beforePublish` captures the bound address and waits for `State() == serverbase.StateStopped`; `Stop` returns `nil`, `Start` returns `errStoppedDuringStartup`, the captured port refuses a dial, `Wait()` returns at once, and `-race` is clean.
- [ ] 4.2 Change `Start`'s closed-error-channel branch per D5 (`LastError()` if recorded, `nil` if already ready, else `errStoppedDuringStartup`). Verified by 3.1 and 4.1; also check that `TestStopWithoutStart`, `TestServerStartWithCancelledContext` and `TestServerServeFailureTransitionsToFailed` still pass. Check: `go test -race -run 'TestServerStop|TestStop|TestServerStart|TestServerServe' ./internal/sshserver/`.

## 5. Behaviour under repetition

- [ ] 5.1 Add `TestServerStartStopLoop`: 25 fresh servers each started and stopped back to back (every `Start` and `Stop` returns `nil`, each `Stop` well inside `ShutdownTimeout`, the port refuses a dial after `Stop`), and 25 more with `Stop` racing a `Start` launched in a goroutine (both return, `Wait()` returns, no goroutine left). Keep the whole test under a few seconds. Check: `go test -race -run TestServerStartStopLoop -count=3 ./internal/sshserver/`, then `go test -race -count=50 ./internal/sshserver/` for the whole package.

## 6. Docs and formal

- [ ] 6.1 Update `.agents/skills/server/SKILL.md`, Start rule 4, with the D9 readiness rule (library-registered listeners, first `Accept` of a wrapped listener, no library `Shutdown` before it). Check: `make check-agent-docs`.
- [ ] 6.2 Regenerate the formal plan and run `python3 scripts/formal_mutation.py symbols --plan <plan> <file:symbol,...>` for every symbol this change touched (`Start`, `Stop`, `doStop`, `serve` and the new helpers); expect all unplanned or new, with no ledger refresh needed, and record the output here. Run `python3 scripts/formal.py correspondence` and `make formal-reach`. Done when both pass and the symbols output is recorded.

## 7. Completion gates

- [ ] 7.1 Run the gates of `.agents/rules/checklist.md` on the final tree (the fix stage runs them from step 2 after the rebase): `make tidy`, `make license-check` (new Go file), `make lint`, `make test`, `make check-baseline` with every new goplint finding triaged, `make check-file-length`, `make sonar-local`, `make check-agent-docs`, and `make test-scripts` only if `scripts/` changed. Done when every gate passes on one HEAD and that HEAD is recorded.

## Workflow follow-up

- The maintainer session adds the upstream charm.land/ssh issue URL to the D8 comment in a later revise once the issue is filed.
- Open question (design.md): the startup-timeout and cancelled-context paths of `Start` leave the server serving; decide on a follow-up change.
- Archive the change after review and gates.
