---
name: tui-testing
description: VHS-based visual debugging workflow for Invowk TUI demos, screenshots, demo GIFs, VHS tapes, and visual TUI regressions. Use tmux-testing for durable CI TUI tests.
---

# VHS-Based TUI Debugging

Use this skill only when a TUI issue needs visual inspection through screenshots
or when maintaining documentation/demo recordings. Durable CI coverage for
interactive TUI commands belongs in `tmux-testing` and `tests/cli/tui_tmux_test.go`.

**Pre-write guardrails**: Before writing any `*_test.go`, follow
`.agents/skills/testing/SKILL.md` § "Pre-Write Checklist".

## Choose The Right Surface

| Need | Use |
|---|---|
| CI-friendly interactive TUI behavior | `tmux-testing` |
| Non-interactive stdout/stderr behavior | testscript `.txtar` |
| Bubble Tea model transitions | unit tests under `internal/tui/` |
| Screenshot-level visual inspection | this VHS workflow |
| Website or README demo GIFs | `vhs/demos/` and `make vhs-demos` |

VHS is not the durable test harness in this repo. Do not add `vhs/tui-tests/`
or CI test suites around VHS tapes.

## Current Repo Policy

Demo tapes live in `vhs/demos/`; see `vhs/README.md`.

```bash
make build
make vhs-validate
make vhs-demos
```

Use temporary scratch tapes for debugging unless the output is an intentional
demo artifact.

## Minimal Debug Tape

```tape
Output vhs/output/invowk-tui-debug.gif
Set Shell "bash"
Set Width 1000
Set Height 700
Set TypingSpeed 20ms

Type "./bin/invowk tui choose 'Option A' 'Option B' 'Option C'"
Enter
Sleep 300ms
Screenshot vhs/output/invowk-tui-initial.png
Down
Sleep 150ms
Screenshot vhs/output/invowk-tui-after-down.png
Enter
Sleep 150ms
```

For scratch tapes outside `vhs/demos/`, run `make build`, then
`vhs validate <scratch>.tape` and `vhs <scratch>.tape` directly.

For `tui choose`, single-select is the default. Multi-select uses `--limit N` or
`--no-limit`; there is no `--multi` flag.

## Screenshot Rules

- Capture only the states needed to diagnose the visual issue.
- Prefer stable dimensions and short sleeps.
- Use suffix numbers in screenshot names if order matters, such as
  `initial01.png` and `afterdown02.png`.
- Keep generated screenshots out of commits unless they are intentional demo
  assets.

## Verification

After a visual investigation leads to a code fix, add or update the durable test
surface as appropriate:

```bash
go test -v -run 'TestTUI_' ./tests/cli/...
go test -v -run TestTUIExemptionTmuxCoverage ./cmd/invowk/...
go test -v ./internal/tui/...
```
