# Design: CLI Color Control, Non-TTY Cleaning, and Quiet Exit Framing

## Context

Current behavior in `main`:
- `cmd/invowk/root.go:103-108` invokes `fang.Execute` with default error handling (`fang.WithVersion`, `fang.WithNotifySignal(os.Interrupt)`). `cmd/invowk/app.go:118-124` defaults `d.Stdout = os.Stdout` and `d.Stderr = os.Stderr` directly, while `RunE` handlers return `&ExitError{Code: result.ExitCode}` (`cmd/invowk/cmd.go:380`, `cmd/invowk/validate.go:156,210,299,323,417`, `cmd/invowk/audit.go:345,350`, `cmd/invowk/tui_confirm.go:90`). Because `fang.Execute` renders returned non-nil errors unless handled, `fang` prints an `ERROR / Exit status N.` frame on stderr even when `silenceOnExitError` sets `cmd.SilenceErrors = true` (`cmd/invowk/cmd.go:264-271`) and even when stderr is redirected. `tests/cli/testdata/audit_findings.txtar:12` currently asserts `stderr 'Exit status 1'`.
- `cmd/invowk/service_error.go:61` and `cmd/invowk/issue_render.go:39` call `renderIssueCatalogEntry(..., "dark")`, which delegates to `glamour.Render(..., stylePath)` (`cmd/invowk/issue_render.go:51`) unconditionally with `"dark"` regardless of TTY, `NO_COLOR`, or `ui.color_scheme`.
- `internal/config/config_schema.cue:14-15,135` defines `#ColorSchemeType: "auto" | "dark" | "light"` (`ui.color_scheme`, stored and displayed by `cmd/invowk/config.go:27,35,176`), but no renderer reads it.
- `cmd/invowk/styles.go:8-72` defines Lip Gloss styles rendered straight into strings (and `cmd/invowk/root.go:48-65` bakes `TitleStyle.Render`/`SubtitleStyle.Render` into `rootCmd.Long`) written to `app.stdout`/`app.stderr` (`cmd/invowk/app.go:43-44,188-189`) without color-profile downsampling.
- `cmd/invowk/watch_tty.go:26-33` checks `o.stdout.(*os.File)` / `stdoutIsTerminal(o.stdout)` for `WatchTerminalOpener`, and `cmd/invowk/cmd_watch.go` writes raw ANSI `clear_screen` sequences to `app.stdout`.
- `cmd/invowk/cmd.go:324-358` (`firstTopLevelToken`) scans root flags before `cmd` during pre-parse command registration and `__complete`, skipping unknown `-*` tokens without consuming a following value.
- `go.mod:92` already includes `github.com/charmbracelet/colorprofile v0.4.3` as an indirect dependency.

## Goals / Non-Goals

**Goals:**
- Emit clean, ANSI-free text from Invowk's own CLI output (`--help`, `invowk cmd` listings, diagnostics, dry-run, issue cards, errors) when redirected/non-TTY or when `NO_COLOR` / `TERM=dumb` / `--ivk-color=never` is active.
- Provide `--ivk-color=auto|always|never` and keep color enablement cleanly separated from color theme (`ui.color_scheme`).
- Suppress `fang`'s `ERROR / Exit status N.` banner unconditionally whenever forwarding a bare `*ExitError` (`Err == nil`), while preserving all `UsageError` and real Invowk failure banners.
- Preserve raw `*os.File` FDs for child commands and keep `invowk tui style` / `invowk tui format` explicitly opt-in to styling.

**Non-Goals:**
- Stripping ANSI codes emitted by a user command's own stdout/stderr (user commands manage their own TTY/color detection via unwrapped `ExecutionContext.IO`).
- Changing the exit-code numbers defined in `openspec/specs/cli-exit-codes/spec.md` (`0`, `N`, `2`, `1`, and documented built-in exceptions).

## Decisions

### D1. Dedicated `uiOut`/`uiErr` colorprofile writers (preserving raw `App.stdout`/`App.stderr` & Cobra help)
- Introduce dedicated `uiOut` and `uiErr` `io.Writer`s (backed by `github.com/charmbracelet/colorprofile.NewWriter`) on `App` (`cmd/invowk/app.go:33-45`) for Invowk's own formatted CLI output (`listCommands`, `renderDryRun`, `defaultDiagnosticRenderer`, `renderServiceError`, `printIssueCatalogEntry`, and `fang` error/help rendering).
- **Critical boundary**: Keep `App.stdout` and `App.stderr` unwrapped so:
  1. Child commands in `ExecutionContext.IO` keep raw `*os.File` TTY/pipe file descriptors.
  2. `defaultWatchTerminalOpener` (`cmd/invowk/watch_tty.go:26-33`) still sees `*os.File == os.Stdout` and `clear_screen` still emits terminal clear sequences.
  3. `invowk tui style` (`cmd/invowk/tui_style.go:66-100`) and `invowk tui format` (`cmd/invowk/tui_format.go:93`) write to raw stdout by default unless `--ivk-color=never` is passed (Cobra `SetOut` must not blindly wrap `tui style`/`tui format` in `colorprofile.NoTTY` when piped!).
- Override/wrap `fang`'s help output (since `fang.Execute` sets its own `HelpFunc` with `colorprofile.NewWriter(c.OutOrStdout(), os.Environ())`, ignoring `--ivk-color=always|never` and breaking TTY detection if `OutOrStdout()` is already wrapped) so `--help` (including pre-rendered `rootCmd.Long` at `cmd/invowk/root.go:48-65`) honors the resolved `--ivk-color` profile.
- Update `firstTopLevelToken` (`cmd/invowk/cmd.go:324-358`) to skip `--ivk-color <val>` and `--ivk-color=<val>` just like `--ivk-config`, so `invowk --ivk-color always cmd <target>` and `invowk __complete --ivk-color never cmd ...` still discover and register commands properly.

### D2. Orthogonal model: color enablement vs color theme (`ui.color_scheme`)
- Keep two separate axes instead of mixing `ui.color_scheme` into `--ivk-color` precedence:
  1. **Color enablement (`colorEnabled` / profile)**: `--ivk-color=always|never|auto` > `NO_COLOR` (when non-empty) / `CLICOLOR_FORCE` (when non-empty and not `"0"`) / `TERM=dumb` > writer TTY detection (via `colorprofile.Detect`). Note: `--ivk-color=never` overrides `CLICOLOR_FORCE`, and `--ivk-color=always` overrides `NO_COLOR`/non-TTY.
  2. **Glamour theme (`glamourStyle`)**: when color is disabled -> `"notty"` (strips ANSI escapes while keeping readable markdown formatting); when color is enabled -> map `ui.color_scheme`: `"auto"` -> `"auto"`, `"dark"` -> `"dark"`, `"light"` -> `"light"`.
- Update `renderServiceError` (`cmd/invowk/service_error.go:47-68`) and `printIssueCatalogEntry` (`cmd/invowk/issue_render.go:38-44`) to accept the resolved Glamour style instead of hardcoding `"dark"`.
- `invowk tui style` (`cmd/invowk/tui_style.go:14-64`) and `invowk tui format` (`cmd/invowk/tui_format.go:15-49`) are explicit styling primitives invoked by scripts inside pipelines (`echo "x" | invowk tui style ...`); they stay explicitly opt-in to ANSI output unless `--ivk-color=never` is passed.

### D3. Unconditional suppression of `fang` banner for bare `*ExitError`
- Install a custom `fang.WithErrorHandler` in `cmd/invowk/root.go:103-108`:
  - Check `var exitErr *ExitError; if errors.As(err, &exitErr) && exitErr.Err == nil`: suppress `fang`'s `ERROR / Exit status N.` frame unconditionally in **both** TTY and non-TTY modes. If `--ivk-verbose` is active, optionally write a single plain diagnostic line (`invowk: command exited with status N`) to Invowk's stderr.
  - Note: this cleanly fixes not only user commands (`cmd/invowk/cmd.go:380`), but also `invowk tui confirm` declining (`cmd/invowk/tui_confirm.go:90`), `invowk validate` failing (`cmd/invowk/validate.go:156`), and `invowk audit` findings (`cmd/invowk/audit.go:350`) where `fang` previously printed a spurious red `Exit status 1.` card below the tool's own output.
  - For all other errors (`*UsageError`, `*ServiceError`, `*ExitError` with `Err != nil`, Cobra parse/find errors), delegate to `fang`'s styled handler (routed through Invowk's colorprofile-wrapped stderr so `NO_COLOR`/non-TTY renders clean plain text without ANSI escapes) while preserving exact error text and `Try --help for usage` required by `openspec/specs/cli-exit-codes/spec.md:66`.
- Update `tests/cli/testdata/audit_findings.txtar:12` to remove the assertion on `stderr 'Exit status 1'`.

## Risks / Trade-offs

- **Risk**: Silent user commands (e.g., `grep -q`) that exit `1` without stderr output produce no banner even on an interactive TTY.
  -> **Mitigation**: Matches standard POSIX shell and task-runner (`make`, `mise`, `cargo`) UX; `--ivk-verbose` exposes the exit status line on stderr for debugging.
- **Risk**: Downstream scripts parsing `Exit status 1` on stderr break.
  -> **Mitigation**: Marked **BREAKING** in `proposal.md` with explicit migration notes; exit codes remain unchanged per `cli-exit-codes`.

## Maintainer decisions

- **2026-11-27**: Confirmed wrapping only Invowk's CLI output writers (never child `ExecutionContext.IO` FDs), keeping `--ivk-color` enablement orthogonal to `ui.color_scheme` theme selection, and suppressing `fang`'s `ERROR / Exit status N.` banner unconditionally (both TTY and non-TTY) for `*ExitError{Err: nil}` (rejecting a `--ivk-quiet` workaround flag).

## Open questions

- None blocking; confirm exact `fang.WithErrorHandler` signature in `github.com/charmbracelet/fang v1.0.0` during implementation so Cobra `--help` hint formatting matches existing `cli-exit-codes` tests.
