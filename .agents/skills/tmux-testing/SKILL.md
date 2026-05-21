---
name: tmux-testing
description: tmux-based TUI testing for autonomous text and ANSI verification
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
2. Start a per-test tmux session with deterministic size.
3. Send the command and keys with `tmux send-keys`.
4. Poll captured pane output until the expected prompt, selection, or exit
   marker appears.
5. Assert on plain text for behavior and ANSI capture only when style matters.
6. Always clean up the session with `t.Cleanup`.

The core tmux commands are:

```bash
tmux new-session -d -s invowk-test -x 80 -y 24
tmux send-keys -t invowk-test "invowk tui choose 'A' 'B'" Enter
tmux capture-pane -t invowk-test -p
tmux capture-pane -t invowk-test -p -e
tmux kill-session -t invowk-test
```

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
