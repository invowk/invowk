---
name: tmux-testing
description: tmux-based TUI testing for durable CI coverage of interactive `invowk tui` commands, keyboard flows, TTY behavior, ANSI/text verification, coverage exemptions, and TUI flake hardening. Use testscript instead for pipe-based `tui format`/`tui style` helpers.
---

# tmux-Based TUI Testing

Use this skill for durable, CI-friendly tests of interactive `invowk tui`
commands when text output, ANSI state, keyboard navigation, or completion
behavior needs to be verified. Use `tui-testing` only for ad hoc VHS visual
debugging or documentation demos.

**Pre-write guardrails**: Before writing `*_test.go`, follow
`.agents/skills/testing/SKILL.md` § "Pre-Write Checklist".

## Current Harness

Durable tmux coverage lives in `tests/cli/tui_tmux_test.go`. The command
coverage guardrail in `cmd/invowk/cmd_coverage_test.go` requires every
interactive TUI exemption to have a matching tmux E2E marker.

Run the focused checks:

```bash
go test -v -run 'TestTUI_' ./tests/cli/...
go test -v -run TestTUIExemptionTmuxCoverage ./cmd/invowk/...
```

Do not create `tests/tui/*.sh` or `tests/tui/run-all.sh`; that shell harness no
longer exists. Add new coverage to the Go tmux harness.

## Workflow

1. Build or locate the invowk test binary using the existing helpers in
   `tests/cli/tui_tmux_test.go`.
2. Start a unique per-test tmux session through the Go helper contract:
   `requireTmux`, serialized session setup, stale-session cleanup, deterministic
   `100x30` size, input-settle delays, and independent cleanup timeouts.
3. Send the command and keys with `tmux send-keys`.
4. Poll captured pane output until the expected prompt, selection, or exit
   marker appears.
5. Assert on plain text for behavior and ANSI capture only when style matters.
   Use `INVOWK_EXIT:$?` or a command-specific marker for completion/exit checks,
   and quote filesystem paths with the existing helpers.
6. Always clean up the session with `t.Cleanup`.

Do not copy ad hoc tmux shell commands into tests. If you need to experiment
manually, keep the experiment outside the committed test and port the result back
to `tests/cli/tui_tmux_test.go`.

When adding a new interactive TUI command, update the txtar exemption, update
the tmux marker map in `cmd/invowk/cmd_coverage_test.go`, add
`TestTUI_<Command>` with marker text, then run the two focused checks above.

## Command Notes

- `tui choose` single-select is the default.
- Multi-select uses `--limit N` or `--no-limit`; there is no `--multi` flag.
- Prefer representative keyboard flows over asserting every frame.
- Keep tests resilient to harmless ANSI/style changes unless the style is the
  behavior under test.

## When To Use Something Else

Use testscript for non-interactive commands and pipe-based TUI helpers such as
`tui format` or `tui style`.

Use unit tests under `internal/tui/` for Bubble Tea model update logic.

Use VHS through `tui-testing` only when a screenshot is needed for visual
inspection or a documentation demo.
