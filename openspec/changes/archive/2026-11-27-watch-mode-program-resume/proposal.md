# Proposal: Watch Mode Program Resume (`--ivk-watch`, OSC 7577)

## Why

`invowk cmd --ivk-watch` (`cmd/invowk/cmd_watch.go`) runs a long-lived workspace watcher that re-executes a command whenever matching files change. When an OSC 7577-capable terminal (such as Rex) restarts after an update, crash, or reboot, active watch panes are lost and users must re-invoke them manually. Because watched commands are inherently repeatable and watch sessions are long-lived workspace panels, `--ivk-watch` is the natural first producer of OSC 7577 Program Resume reports in Invowk.

## What Changes

- Prerequisite: `program-resume-token-hygiene` (provides `github.com/charmbracelet/x/ansi` `v0.12.0` and `OSC7577_TOKEN` isolation helpers).
- Emit signed OSC 7577 Program Resume reports (`ansi.SetProgramResume`) from `invowk cmd --ivk-watch` when `OSC7577_TOKEN` is set in Invowk's environment, the session is attached to an interactive terminal, no `--ivk-env-var` flags were provided, and the reconstructed `argv` and `cwd` satisfy OSC 7577 validity and size rules.
- Reconstruct the resume command starting with the Invowk executable path (`os.Executable()`), preserving `--ivk-watch`, root/command flags, command name/source disambiguation, and positional arguments, plus the resolved absolute working directory (`os.Getwd()`), and materializing `INVOWK_CONFIG` / `INVOWK_NO_CWD` env posture when set without CLI flags.
- Strip `OSC7577_TOKEN` from all child executions launched by watch mode (both initial execution and change-triggered re-executions, including `native` non-interactive children) so child processes cannot overwrite the watcher's resume record.
- Distinguish voluntary termination from terminal shutdown: emit `ansi.ClearProgramResume` (`clear=1`) on `SIGINT` (Ctrl+C) or watcher error exit, and preserve the record (do NOT emit `clear=1`) when terminated by `SIGHUP` or `SIGTERM` (terminal restart/close).
- Handle `--ivk-watch -i` cleanly: the watcher owns the resume record while interactive child sessions strip `OSC7577_TOKEN`.
- Non-breaking change: terminals without `OSC7577_TOKEN`, non-TTY runs, and non-watch `invowk cmd` executions are unaffected.

## Capabilities

### New Capabilities

- `watch-mode-program-resume`: Defines OSC 7577 Program Resume reporting, child token isolation, secret/limit guardrails, and signal-lifecycle clearing behavior for `invowk cmd --ivk-watch`.

### Modified Capabilities

(none)

## Impact

- Code: `cmd/invowk/cmd_watch.go`, `cmd/invowk/root.go` (signal distinction for `SIGHUP`/`SIGTERM` vs `SIGINT`), a watch resume reporter helper (under `cmd/invowk/` or `internal/tui/`), and watch-child token-stripping option across `cmd/invowk` / `internal/app/commandsvc` / `internal/app/execute` / `internal/runtime`.
- Tests & Docs: unit tests for `argv`/guardrails/`SIGINT`-vs-`SIGHUP` behavior, PTY/CLI tests for watch resume emission and child token stripping, and watch-mode documentation in `website/`.
