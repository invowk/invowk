# Container and TUI Test Patterns

## Container Test Shapes

Keep the three coordination models distinct:

1. Unit tests use a per-test injected command recorder or mock. Never reset one
   shared recorder from parallel subtests.
2. Go integration tests that perform real Docker/Podman operations call
   `t.Parallel()`, skip in short mode, acquire
   `testutil.AcquireContainerSemaphore(t)`, and use
   `testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)`.
3. CLI container tests use `TestContainerCLI`, `containerSetup`, and
   `testutil.AcquireContainerSuiteLock(t)` for cross-process serialization.
   Do not add the suite lock to Go package tests.

Prefer the live CLI container harness in `tests/cli/container_harness.go`; it
probes engine availability, Linux-container run support, and build support.
Use `debian:stable-slim` for generic container fixtures. Container commands are
Linux-only.

Scripts run by `/bin/sh -c` do not imply `set -e`; include it when the test
requires fail-fast shell semantics.

When a real container test hangs or is killed, load `linux-testing` and its
container deep dive. Do not copy timeout values or CI matrix details from old
reports; inspect the helper constants and workflow files.

## TUI Testing Router

- Test Bubble Tea model state transitions and pure rendering helpers under
  `internal/tui/` without starting a terminal program.
- Use `tmux-testing` for durable interactive CI behavior, keyboard flows,
  prompts, and exit markers.
- Use testscript for pipe-based `tui format` and `tui style` behavior.
- Use `tui-testing` and VHS only for visual diagnosis or intentional demo
  assets.

Prefer representative state transitions over frame-by-frame snapshots. Assert
plain text unless ANSI styling is the behavior under test.

## Focused Verification

```bash
go test -v -run 'TestTUI_' ./tests/cli/...
go test -v -run TestTUIExemptionTmuxCoverage ./cmd/invowk/...
go test -v ./internal/tui/...
```

For container work, run the focused package or CLI suite first, then the full
repository gates required by `.agents/rules/checklist.md` on a Linux host with
a supported engine.
