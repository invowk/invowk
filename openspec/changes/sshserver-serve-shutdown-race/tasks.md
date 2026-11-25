# Tasks

All paths are under the worktree `/var/home/danilo/Workspace/github/invowk/invowk-sshrace`. New tests go in a new file, `internal/sshserver/server_lifecycle_race_test.go`, so `server_test.go` (659 lines) stays well under the 1000-line limit.

Dial assertions ("the port refuses a dial") dial once per test, after `Stop` and any overlapping `Start` have returned, with a bounded `net.DialTimeout` (a refused loopback connect takes about 2 s on Windows), and expect a refusal (design.md, Risks). "Wait() returns" means `Wait()` returns within a bounded time (a few seconds) in a goroutine the test waits on.

## 1. Negative control on the base

- [ ] 1.1 Write the tests of tasks 2.2 and 3.1 (`TestServerStartWaitsForServeLoop`, `TestServerStopWhileServeLoopDelayed`) first. In a throwaway worktree of 36f85fd6 under this stage's scratch directory, add only the D6 seam (the `hooks *lifecycleHooks` field and the `beforeServe` call in `serve()` after it reads `srv` and `listener`; `serveEntered` and `beforePublish` may be declared but need not be called) and a declaration of `errStoppedDuringStartup` (no behaviour change), plus those two tests. Run `go test -race -run 'TestServerStartWaitsForServeLoop|TestServerStopWhileServeLoopDelayed' -count=5 ./internal/sshserver/` and record in this task the command, the base SHA and the failure messages. Expected on every run: `Start` has returned `nil` while the serve goroutine is held (D7). Done when every run of both tests fails that way and it is recorded here; if any run passes, stop and report it.
- [ ] 1.2 Supplement, not a done condition: in the same throwaway worktree, add the test of task 5.1 (`TestServerStartStopLoop`) and run `go test -race -run TestServerStartStopLoop -count=20 ./internal/sshserver/`. Record the command, the number of `WARNING: DATA RACE` reports (with their frames, for example `charm.land/ssh@v0.4.3/server.go:275` against `:312`) and the number of failing runs, zero included. Then remove the throwaway worktree with `git worktree remove --force`.

## 2. Readiness at the first Accept (D1, D2, D6, D8)

- [ ] 2.1 Add the nil-safe serve gate type and the `lifecycleHooks` seam (`beforePublish func(net.Addr)`, `serveEntered func()`, `beforeServe func()`) to `internal/sshserver` (`server.go` fields, helpers in `server_lifecycle.go`). Verify with unit tests: a nil gate's open and wait are no-ops, open is idempotent, wait returns false when its context ends first.
- [ ] 2.2 Change `serve()` to `serve(gate)`, launched as `go s.serve(gate)` with the gate `Start` created; `serve()` never reads the gate from the `Server`. Wrap the listener passed to `srv.Serve` so its first `Accept` calls `TransitionToRunning` and opens the gate; remove the early `TransitionToRunning` from `serve()`; open the gate in a `defer` at the top of `serve()`. Add the D8 comment naming the upstream defect by behaviour (no TODO marker, no URL). Verify with `TestServerStartWaitsForServeLoop`: with `beforeServe` holding, `Start` has not returned and `State()` is `Starting`; after release `Start` returns `nil` and `State()` is `Running`.
- [ ] 2.3 Cover every exit path of `serve()`: update `TestServerServeFailureTransitionsToFailed` (literal construction) to call `serve(nil)` and still pass, and add a test that calls `serve(gate)` on a literal `Server` with a nil `srv`, asserting the gate opens. The upstream `Serve` failing before `Accept` (only in `ensureHostSigner`, which cannot fail once `Start` configures a host key) uses the same deferred open and gets no separate test. Check: `go test -race -run 'TestServerStartWaitsForServeLoop|TestServerServe|TestServeGate' ./internal/sshserver/`.

## 3. Stop ordering (D3, D7)

- [ ] 3.1 In `doStop`, read the gate field with `srv` and `listener`, keep closing the listener first, wait for the gate with `shutdownCtx` before `srv.Shutdown`, and on a timeout log, skip `Shutdown` and return an error wrapping `context.DeadlineExceeded`. Verify with `TestServerStopWhileServeLoopDelayed` (D7, no sleep): once `beforeServe` is reached `Start` has not returned; then `Stop` returns `nil`, `Start` returns an error matching `errStoppedDuringStartup`, the bound port refuses a dial, `Wait()` returns, and `-race` is clean.
- [ ] 3.2 Add `TestServerStopTimesOutBeforeServeLoop`: `ShutdownTimeout` of 50 ms, `beforeServe` held until well after the timeout, then released; `Stop` returns an error matching `context.DeadlineExceeded` only after the serve goroutine exited, the port refuses a dial once `Start` has also returned, and `-race` is clean.
- [ ] 3.3 Add `TestServerStopBeforeServeReadsResources` (gate ownership, D2/D3): `ShutdownTimeout` of 5 s; `serveEntered` waits until `doStop` has taken the resources (`State()` is `Stopping` and `s.srv` is nil under `srvMu`) and then returns. Assert that `Stop` returns `nil` in under 1 s, `Start` returns an error matching `errStoppedDuringStartup`, the port refuses a dial, and `Wait()` returns. This fails (a `Stop` of about 5 s) if `serve()` takes its gate from a field `doStop` cleared. Check: `go test -race -run 'TestServerStop' -count=20 ./internal/sshserver/`.

## 4. Stop during Start (D4, D5, D11)

- [ ] 4.1 Add the `stopBegun` flag set by `doStop` under `srvMu`; move the publication of `listener`, `addr`, `srv` and the gate and both `AddGoroutine` calls into one `srvMu` critical section in `Start` that checks `stopBegun`, closes the bound listener and returns `errStoppedDuringStartup` when set. Verify with `TestServerStopBeforeStartPublishes`: `beforePublish` captures the bound address and waits for `State() == serverbase.StateStopped`; `Stop` returns `nil`, `Start` returns `errStoppedDuringStartup`, the captured port refuses a dial made after `Start` has returned, `Wait()` returns, and `-race` is clean.
- [ ] 4.2 Change `Start`'s closed-error-channel branch per D5 (`LastError()` if recorded, `nil` if already ready, else `errStoppedDuringStartup`). Verified by 3.1, 3.3 and 4.1; also check that `TestStopWithoutStart`, `TestServerStartWithCancelledContext` and `TestServerServeFailureTransitionsToFailed` still pass.
- [ ] 4.3 Serialise `Stop` with `stopMu` (D11). Verify with `TestServerConcurrentStopsDuringStart`: `beforeServe` holds; two `Stop` calls run in goroutines; while the hook holds neither has returned; after release both return `nil` and the serve goroutine has exited, `Start` returns an error matching `errStoppedDuringStartup`, `Wait()` returns, and `-race` is clean. Check: `go test -race -run 'TestServerStop|TestStop|TestServerStart|TestServerServe|TestServerConcurrent' ./internal/sshserver/`.

## 5. Behaviour under repetition

- [ ] 5.1 Add `TestServerStartStopLoop` with a `ShutdownTimeout` of 5 s: 25 fresh servers each started and stopped back to back (every `Start` and `Stop` returns `nil`, each `Stop` in under 1 s), and 25 more with `Stop` racing a `Start` launched in a goroutine (`Start` returns `nil` or an error matching `errStoppedDuringStartup`, each `Stop` returns `nil` in under 1 s, `Wait()` returns). Dial only after the first and the last server of each half. Keep the whole test under a few seconds. Check: `go test -race -run TestServerStartStopLoop -count=3 ./internal/sshserver/`, then `go test -race -count=50 ./internal/sshserver/` for the whole package.

## 6. Docs and formal

- [ ] 6.1 Update `.agents/skills/server/SKILL.md`, Start rule 4, with the D9 readiness rule (library-registered listeners, first `Accept` of a wrapped listener, no library `Shutdown` before it). Check: `make check-agent-docs`.
- [ ] 6.2 Regenerate the formal plan and run `python3 scripts/formal_mutation.py symbols --plan <plan> <file:symbol,...>` for every symbol this change touched (`Start`, `Stop`, `doStop`, `serve` and the new helpers); expect all unplanned or new, with no ledger refresh needed, and record the output here. Run `python3 scripts/formal.py correspondence`, `make formal-reach` and `make formal` (Java is available; under the heavy lock). Confirm that `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` (the F11 characterisation row for `Server.Stop`, `formal/tla/HostCallbackToken.tla:33`) still passes. Done when all pass and the symbols output is recorded.

## 7. Completion gates

- [ ] 7.1 Run the gates of `.agents/rules/checklist.md` on the final tree (the fix stage runs them from step 2 after the rebase): `make tidy`, `make license-check` (new Go file), `make lint`, `make test`, `make check-baseline` with every new goplint finding triaged, `make check-file-length`, `make sonar-local`, `make check-agent-docs`, `make formal` (code named by a correspondence row changed), and `make test-scripts` only if `scripts/` changed. Done when every gate passes on one HEAD and that HEAD is recorded.

## Workflow follow-up

- The maintainer session adds the upstream charm.land/ssh issue URL to the D8 comment in a later revise once the issue is filed.
- Open questions (design.md): the startup-timeout and cancelled-context paths of `Start` leave the server serving; the public `Wait()` during `Start`; and whether brief requirement 7 needs an actual `DATA RACE` report. Decide on follow-ups.
- Archive the change after review and gates.
