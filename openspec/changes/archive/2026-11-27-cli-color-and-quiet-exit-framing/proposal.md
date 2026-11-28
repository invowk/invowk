# Proposal: CLI Color Control, Non-TTY Cleaning, and Quiet Exit Framing

## Why

Invowk currently emits hardcoded ANSI escape sequences and Glamour `"dark"` markdown output even when stdout/stderr are redirected, piped, or run with `NO_COLOR=1` / `TERM=dumb`, and `fang.Execute` injects an `ERROR / Exit status N.` box on stderr whenever a user command or built-in exits non-zero. Clean non-TTY output and quiet exit forwarding are essential for CI logs, `systemd` units, and shell wrappers (such as `cst doctor` or `cst perf` returning `1` for normal warning states).

## What Changes

- Wrap Invowk's own CLI `stdout` and `stderr` writers (`cmd/invowk/app.go`) with `github.com/charmbracelet/colorprofile.NewWriter` while leaving child-process `ExecutionContext.IO` streams unwrapped so user commands keep raw `*os.File` TTY/FD behavior.
- Add a persistent `--ivk-color=auto|always|never` flag on the root command (precedence: `--ivk-color` > `NO_COLOR` / `CLICOLOR_FORCE` / `TERM=dumb` > TTY detection), orthogonal to the color theme (`ui.color_scheme`: `auto|dark|light`).
- Wire `ui.color_scheme` and color enablement into Glamour rendering (`cmd/invowk/service_error.go`, `cmd/invowk/issue_render.go`), selecting `"notty"` when color is disabled and `"auto"|"dark"|"light"` when enabled. Keep `invowk tui style` and `invowk tui format` explicitly opt-in to styling.
- **BREAKING**: Suppress `fang`'s `ERROR / Exit status N.` error frame unconditionally (both TTY and non-TTY) whenever the returned error is an `*ExitError` with `Err == nil` (user command exit codes, `invowk audit` findings, `invowk validate` validation errors, `invowk tui confirm` declines). Under `--ivk-verbose`, emit an optional single-line status note to stderr instead of a box frame. Update `tests/cli/testdata/audit_findings.txtar` accordingly.

## Migration notes

- Shell wrappers, CI scripts, or tests that previously matched `Exit status <N>` on `invowk` stderr when a user command or built-in (`audit`, `validate`, `tui confirm`) exited with status `N` will no longer see that banner; inspect the process exit code `$?` directly (or pass `--ivk-verbose` for diagnostic stderr tracing).
- Scripts or CI pipelines that previously stripped ANSI codes from `invowk` help, listings, or errors when `stdout`/`stderr` are not a TTY or when `NO_COLOR=1` is set will now receive plain text automatically; pass `--ivk-color=always` or `CLICOLOR_FORCE=1` to force ANSI styling in non-TTY pipes.

## Capabilities

### New Capabilities

- `cli-output-styling`: Defines color enablement precedence (`--ivk-color`, `NO_COLOR`, `CLICOLOR_FORCE`, `TERM=dumb`, TTY detection), orthogonal `ui.color_scheme` theme selection for Glamour and Lip Gloss, raw child-FD preservation, opt-in styling behavior for `tui style`/`tui format`, and quiet `*ExitError` exit-status forwarding without `fang` error banners.

### Modified Capabilities

(none)

## Impact

- CLI wiring: `cmd/invowk/root.go`, `cmd/invowk/app.go`, `cmd/invowk/styles.go`, `cmd/invowk/service_error.go`, `cmd/invowk/issue_render.go`, `cmd/invowk/cmd_discovery.go`, `cmd/invowk/cmd_dryrun.go`, `cmd/invowk/config.go`.
- Config & Dependencies: `internal/config/config_schema.cue` (documenting orthogonality of `ui.color_scheme`), `go.mod` (promoting `github.com/charmbracelet/colorprofile` from indirect to direct dependency).
- Tests & Docs: `cmd/invowk/*_test.go`, `tests/cli/testdata/audit_findings.txtar`, new non-TTY / `NO_COLOR` / `--ivk-color` / quiet-exit testscript cases, and `website/` CLI reference docs.
