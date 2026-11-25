---
name: tmux-testing
description: >-
  tmux-based durable CI tests for interactive `invowk tui` commands
  (`tests/cli/tui_tmux_test.go`, `TestTUI_*`, `TestTUIExemptionTmuxCoverage`):
  keyboard flows, TTY behavior, ANSI/text verification, coverage exemptions,
  and TUI flake hardening. Use testscript instead for pipe-based
  `tui format`/`tui style` helpers.
---

# tmux-Based TUI Testing

Use this skill for durable, CI-friendly tests of interactive `invowk tui`
commands when text output, ANSI state, keyboard navigation, or completion
behavior needs to be verified. Use `tui-testing` only for ad hoc VHS visual
debugging or documentation demos.

**Pre-write guardrails**: Before writing `*_test.go`, follow
`.agents/skills/testing/references/pre-write-and-go-patterns.md` § "Pre-Write Checks".

## Current Harness

Durable tmux coverage lives in `tests/cli/tui_tmux_test.go`.
`TestTUIExemptionTmuxCoverage` (`cmd/invowk/cmd_coverage_test.go`) parses that
file's AST and enforces, for every `tui <cmd>` key in
`builtinTxtarCoverageExemptions`:

- the exemption reason contains `e2e via tmux` (case-insensitive);
- some `s.sendKeys(...)` call's first argument invokes `tui <cmd>` in a
  recognised form: `binaryPath+" tui <cmd> ..."` (exactly one string literal; a
  chained `+ shellQuote(p)` is not matched) or
  `fmt.Sprintf("%s tui <cmd> ...", binaryPath, ...)` (use this for dynamic
  arguments), passed directly or through a variable assigned in the same
  function. Comments and other string literals do not count;
- the reverse: tmux coverage of a `tui` command that is not exempted fails.

Only `tui `-prefixed exemptions are checked. A new interactive command outside
the `tui` tree is not covered by this guard and needs its own coverage decision.

Run the focused checks:

```bash
go test -v -run 'TestTUI_' ./tests/cli/...
go test -v -run TestTUIExemptionTmuxCoverage ./cmd/invowk/...
```

`requireTmux` skips under `-short` and when tmux is missing, and
`newTmuxSession` skips when it cannot create a session: check the `-v` output
for `SKIP` before treating a focused run as coverage.

Do not create `tests/tui/*.sh` or `tests/tui/run-all.sh`; that shell harness no
longer exists. Add new coverage to the Go tmux harness.

## Workflow

1. Use the invowk binary that `tests/cli` `TestMain` builds once into
   `bin/invowk` (global `binaryPath`); do not build another one.
2. Start a unique per-test tmux session through the Go helper contract:
   `requireTmux`, then `newTmuxSession(t, suffix)`, which serializes sessions
   with `tmuxSessionMu`, names them by suffix and PID, kills a stale session of
   the same name, uses a deterministic `100x30` size, and registers
   `t.Cleanup(s.kill)`. Respect `tuiInputSettleDelay` before sending input and
   `tmuxWaitTimeout` for polls; `kill` uses its own `tmuxCleanupTimeout` on
   `context.Background()` because `t.Context()` is already cancelled during
   cleanup. Read the constants in the file instead of copying their values.
3. Send the command and keys with `s.sendKeys` (tmux `send-keys`).
4. Poll captured pane output (`s.waitFor`) until the expected prompt,
   selection, or exit marker appears. Make readiness text distinctive (a title
   or data header such as `tblhdr_name`, never a generic word like `name`), and
   absent from the command typed into the pane: the shell prompt and the echoed
   command can otherwise satisfy the wait before the UI draws.
5. Assert on plain text for behavior and ANSI capture only when style matters.
   Use `INVOWK_EXIT:$?` or a command-specific marker for completion/exit checks,
   and quote filesystem paths with the existing helpers (`shellQuote`).
6. Always clean up the session with `t.Cleanup` (done by `newTmuxSession`).

Do not copy ad hoc tmux shell commands into tests. If you need to experiment
manually, keep the experiment outside the committed test and port the result back
to `tests/cli/tui_tmux_test.go`.

When adding a new interactive `tui` command:

1. Add it to `builtinTxtarCoverageExemptions` with a reason that includes
   `E2E via tmux`.
2. Add `TestTUI_<Command>` that calls `s.sendKeys` in one of the recognised
   forms and waits for an exit marker (`INVOWK_EXIT:$?` or a command-specific
   marker).
3. Run the two focused checks above.

## Command Notes

- `tui choose` single-select is the default.
- Multi-select uses `--limit N` or `--no-limit`; there is no `--multi` flag.
- Prefer representative keyboard flows over asserting every frame.
- Keep tests resilient to harmless ANSI/style changes unless the style is the
  behavior under test.

## The `-i` Viewer

`tests/cli/interactive_viewer_tmux_test.go` drives `invowk -i cmd …` itself (the PTY viewer, not a `tui` component): controlling terminal, signals, screen emulation, history and query answering. The viewer only redraws on a Bubble Tea message, so wait for a pane line (`mustWaitLine`) rather than sleeping, and assert on whole pane rows (header row 0, command screen from row 2, footer last). Model-level viewer tests run without a terminal through `tui.InteractiveOptions.ProgramOptions` (`WithInput(nil)`, `WithOutput`, `WithWindowSize`, `WithFilter`); a filter sees the view only when a message arrives, so poke the program with a no-op message while waiting.

## When To Use Something Else

Testscript for `tui format`/`tui style` and other non-interactive commands,
unit tests under `internal/tui/` for Bubble Tea model logic, and VHS via
`tui-testing` for screenshots or demos. Full router:
`.agents/skills/testing/references/container-and-tui.md` § "TUI Testing Router".
