# Design: Watch Mode Program Resume (`--ivk-watch`, OSC 7577)

## Context

See `proposal.md` for motivation. Current codebase facts:

- `cmd/invowk/cmd_watch.go:56-179` (`runWatchMode`) resolves the watched command (`ResolveWatchPlan`), performs an initial execution via `session.InitialExecution(ctx)` (`cmd/invowk/cmd_watch.go:135`), creates a `watch.Watcher` (`internal/watch/watcher.go:56-87`), and runs `w.Run(ctx)` (`cmd/invowk/cmd_watch.go:178`) until `ctx` is cancelled or a fatal watcher error occurs.
- `cmd/invowk/cmd_watch.go:107-127` (`reexecute`) builds child `ExecuteRequest` values with `childFlags.watch = false` and calls `executeWatchRequest(execCtx, cmd, app, req)` (`cmd/invowk/cmd_watch.go:181-194`).
- `cmd/invowk/root.go:103-109` calls `fang.Execute(context.Background(), rootCmd, ..., fang.WithNotifySignal(os.Interrupt))`. Currently `fang` cancels `cmd.Context()` on `os.Interrupt` (`SIGINT`), while `SIGHUP` and `SIGTERM` are left with OS default signal disposition.
- `cmd/invowk/cmd_execute_helpers.go:100-154` (`buildCommandExecuteRequest`) extracts flags including `--ivk-env-var`, `--ivk-env-file`, `--ivk-workdir`, `--ivk-config`, `--ivk-no-cwd`, `--ivk-runtime`, `--ivk-from`, and dynamic command flags.
- `internal/app/commandsvc/types.go:83-90`, `internal/app/execute/orchestrator.go:71-73`, and `internal/runtime/env.go:70-111` construct command environments without a per-request option to strip `OSC7577_TOKEN` for non-interactive `native` children.

## Goals / Non-Goals

**Goals:**
- Support OSC 7577 Program Resume in `invowk cmd --ivk-watch` using `github.com/charmbracelet/x/ansi` (`v0.12.0`).
- Prevent watched child commands from seeing `OSC7577_TOKEN` or clobbering the watcher's resume record.
- Ensure `SIGINT` / error exits clear the record (`clear=1`) while `SIGHUP` / `SIGTERM` preserve it.
- Keep all terminal I/O, clock, PID, and signal behavior deterministically unit-testable.

**Non-Goals:**
- Rewriting `runWatchMode` as a Bubble Tea TUI program (unnecessary overhead; `x/ansi` emits the exact same protocol bytes directly).
- Supporting OSC 7577 on non-watch `invowk cmd` runs (covered separately in `declarative-command-program-resume`).

## Decisions

### D1. Direct `github.com/charmbracelet/x/ansi` emission instead of wrapping watch mode in Bubble Tea
`runWatchMode` (`cmd/invowk/cmd_watch.go:56`) streams plain/styled text directly to `app.stdout` and `app.stderr` via `internal/watch/watcher.go`. Wrapping watch mode in Bubble Tea solely to set `View.ProgramResume` would alter scrollback/pipe output semantics and duplicate `-i`. Using `ansi.SetProgramResume` and `ansi.ClearProgramResume` directly in a small watch-resume controller keeps output behavior identical.

### D2. Controlling-terminal / TTY write target
- On Unix, prefer writing OSC 7577 sequences to `/dev/tty` when openable (falling back to `app.stdout` only if it is a terminal per `term.IsTerminal`); on Windows, write to `CONOUT$` (or `app.stdout` if console). This ensures redirected stdout (`invowk cmd --ivk-watch ... > log.txt`) never leaks escape sequences into log files, while remaining compatible with PTY tests where `stdout` is a PTY.
- Inject writer, `os.Getenv`, `os.Getpid`, and `time.Now` via a small `watchResumeReporter` struct so unit tests can verify exact MAC-signed `ESC ] 7577 ; ...` and `clear=1` sequences without spawning a real terminal.

### D3. Canonical `argv` reconstruction and guardrails
- Check `--ivk-env-var` via `cmd.Flags().Changed("ivk-env-var")` / `stringArrayFlagValue(cmd, "ivk-env-var")`: if non-empty, **do not report** (avoids embedding secrets in terminal state or resuming without required env overrides).
- Build `argv` as `append([]string{exePath}, os.Args[1:]...)` where `exePath` comes from `os.Executable()` (if `os.Executable()` fails, skip reporting rather than guessing a relative path).
- Materialize `INVOWK_CONFIG` and `INVOWK_NO_CWD`: if `INVOWK_CONFIG` is set in the environment and `--ivk-config` / `-c` was not passed on CLI, append `--ivk-config=<val>` right after `"cmd"` (before any `--` or positional command); likewise append `--ivk-no-cwd` when `noCwdFromContext(ctx)` came from `INVOWK_NO_CWD`. Because the resumed command runs in a fresh shell environment, this keeps discovery scope identical.
- Pass `Dir` from `os.Getwd()` (or skip reporting if `os.Getwd()` fails) to `ansi.SetProgramResume(token, pid, now, ansi.ProgramResume{Args: argv, Dir: cwd})`. Because `ansi.SetProgramResume` returns `""` when `Args` or `Dir` violates OSC 7577 rules (empty `Args`, relative `Args[0]`, control characters, or >2048 bytes / >64 args per `x/ansi v0.12.0`), check for `""` and skip emission cleanly.
- Alternative considered: reconstructing `argv` manually from `ExecuteRequest` fields — rejected because dynamic command flags (`opts.FlagDefs`) and `@source` syntax are complex to re-serialize losslessly, whereas `os.Args[1:]` preserves the exact CLI invocation.

### D4. Signal lifecycle (`SIGINT` vs `SIGHUP`/`SIGTERM`)
- `cmd/invowk/root.go:107` passes `fang.WithNotifySignal(os.Interrupt)` so `SIGINT` cancels `cmd.Context()`.
- In `runWatchMode`:
  - Listen for terminal-shutdown signals (`SIGHUP`, `SIGTERM` via platform-split Unix/other helper) alongside `ctx.Done()`.
  - Track a `preserveResumeOnExit` boolean: set it `true` if `SIGHUP` or `SIGTERM` arrives (or `ctx` was cancelled by a shutdown signal rather than `os.Interrupt`), and cancel the watcher/child context cleanly.
  - Note: in `Execute()` (`cmd/invowk/root.go:103`), if `SIGHUP`/`SIGTERM` are NOT in `fang.WithNotifySignal`, catching them locally in `runWatchMode` ensures child processes shut down cleanly AND guarantees `clear=1` is skipped when `preserveResumeOnExit` is true.
  - On `SIGINT` (Ctrl+C) or normal error return from `runWatchMode`, emit `ansi.ClearProgramResume(token, pid, time.Now())`.

### D5. Stripping `OSC7577_TOKEN` from watched child executions
- Add `StripProgramResumeToken bool` to `ExecuteRequest` / `commandsvc.ExecuteRequest` / `execute.BuildExecutionContextOptions` / `runtime.EnvContext` (all validated/default `false`).
- In `runWatchMode`'s `reexecute` closure (`cmd/invowk/cmd_watch.go:107`), set `StripProgramResumeToken: true` on child requests so `DefaultEnvBuilder.Build` (`internal/runtime/env_builder.go:71`) strips `OSC7577_TOKEN` even for non-interactive `native` children (and interactive `-i` children already strip it per Change 1).
- Note: read `OSC7577_TOKEN` once at `runWatchMode` entry and do NOT mutate `invowk`'s own process env with `os.Unsetenv`, avoiding process-global side effects in parallel tests.

## Risks / Trade-offs

- **Risk**: `os.Args[1:]` in unit tests of `runWatchMode` contains `go test` flags instead of `invowk cmd ...` args.
  -> **Mitigation**: Accept `rawArgs []string` (defaulting to `os.Args[1:]`) in a testable helper/option on `watchResumeReporter` so unit tests inject deterministic CLI args.
- **Risk**: A child command running in `--ivk-watch -i` briefly uses an alternate screen; when it exits, the terminal returns to the main screen.
  -> **Mitigation**: Because `x/vt` strips `OSC7577_TOKEN` from the `-i` child and Bubble Tea `interactiveModel` leaves `View.ProgramResume == nil`, the `-i` viewer does not send `clear=1` on exit; optionally re-emit `SetProgramResume` after an interactive iteration for fresh timestamping.

## Maintainer decisions

- **2026-11-27**: Scope `--ivk-watch` OSC 7577 support as Change 2 (`watch-mode-program-resume`), layered on top of Change 1 (`program-resume-token-hygiene`).

## Open questions

- **Default-on vs opt-in for watch mode**: Design currently recommends reporting **by default** whenever `OSC7577_TOKEN` is present and guardrails pass (zero effect on unsupported terminals). Confirm whether a config/flag opt-out (e.g., `--ivk-no-resume` or `ui.program_resume: false`) is desired.
- **Env-to-flag materialization**: Confirm whether `INVOWK_CONFIG` and `INVOWK_NO_CWD` set via environment variables should be materialized into `--ivk-config` / `--ivk-no-cwd` in the resume `argv` (recommended: yes, because OSC 7577 does not carry environment variables across terminal restarts).
