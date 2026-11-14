---
name: testing
description: >-
  Invowk test workflow: choose the test surface and apply repo conventions for
  Go `*_test.go`, testscript `.txtar` CLI tests, virtual/native runtime
  mirrors, real container integration tests, TUI components, race and flake
  prevention, and `internal/testutil` helpers. Use when creating or modifying
  `*_test.go` or `.txtar` tests or selecting the correct test surface. Go
  toolchain details: `go-testing`; read-only suite audits: `review-tests`.
---

# Invowk Testing

Apply repository test policy from `.agents/rules/testing.md` first. Use this
skill to select the correct Invowk test surface and load only the reference that
owns the implementation details.

For Alloy/TLA+ models, golden vectors under `testdata/formal/`, and rapid
property tests that bind them to code, use `formal-verification`.

## Workflow

1. Identify the behavior boundary: pure Go package, CLI/testscript, runtime
   mirror, real container engine, TUI model, interactive TUI, or visual demo.
2. Read [references/pre-write-and-go-patterns.md](references/pre-write-and-go-patterns.md)
   before editing Go tests.
3. Load the surface reference and related skill from the router below.
4. Reproduce or run the narrowest relevant test with cache bypass.
5. Implement behavioral coverage, including meaningful error and boundary
   paths. Avoid tests that merely restate struct fields or mocks.
6. Run the focused guardrails, then the full gates required by
   `.agents/rules/checklist.md`.

## Surface Router

| Surface | Read / Use |
|---|---|
| Go test execution, flags, race reports, context, parallelism | `go-testing` |
| General Go test patterns and repository helpers | [pre-write-and-go-patterns.md](references/pre-write-and-go-patterns.md) |
| CLI `.txtar`, environment isolation, cross-platform fixtures | [testscript-and-mirrors.md](references/testscript-and-mirrors.md) |
| Virtual/native mirror creation or audit | `native-mirror` |
| CUE command fixtures | `invowk-schema` |
| Real container tests and TUI surface selection | [container-and-tui.md](references/container-and-tui.md) |
| Linux container hangs, OOM, flock, inotify | `linux-testing` |
| Windows paths, processes, PowerShell, ConPTY | `windows-testing` |
| macOS resolver paths, APFS, kqueue, timer behavior | `macos-testing` |
| Durable interactive TUI E2E tests | `tmux-testing` |
| VHS visual investigation and demos | `tui-testing` |
| Read-only suite quality or coverage audit | `review-tests` |

## Non-Negotiable Guardrails

- `t.Parallel()` policy is in `.agents/rules/testing.md` § "Test Parallelism";
  verify shared-state safety first with the `go-testing` "Resource Safety
  Matrix" (environment, working directory, stdin, home overrides, CUE state,
  shared mocks).
- Use `t.Context()` by default; bound subprocess, network and container work
  (rule "Test Context Usage"; `go-testing` "Context Patterns Decision Tree").
- Prefer event-based synchronization, fake clocks, or polling deadlines over
  sleeps. Preserve evidence-backed sleeps only when timing creates the event
  distinction under test.
- Use existing `internal/testutil` helpers. Inspect their live API instead of
  relying on copied inventories.
- For home-dependent tests use `testutil.SetHomeDir(t, dir)`, which sets the
  platform home variable (`USERPROFILE` on Windows, `HOME` elsewhere); register
  its returned restore func (`t.Cleanup(testutil.SetHomeDir(t, dir))`) and keep
  these tests serial.
- Resolver-backed expected paths: `go-testing` "Cross-Platform Path Contract
  Tests".
- Mirror exemptions live only in `tests/cli/runtime_mirror_exemptions.json`
  (rule "Platform Coverage and Runtime Mirrors").
- Use only `debian:stable-slim` for generic container fixtures, and keep
  container execution Linux-only.
- Keep VHS out of durable CI; use the Go tmux harness for interactive TUI
  coverage.

## Verification Router

```bash
# Focused Go package
go test -count=1 -run '<TestName>' ./path/to/package/...

# CLI and mirror guardrails
go test -v -run 'TestBuiltinCommandTxtarCoverage|TestTUIExemptionTmuxCoverage' ./cmd/invowk/...
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...
make test-cli

# Cross-platform compile/vet coverage
make check-windows-build

# Repository completion gates
make lint
make test

# goplint consumer gates (analyzer changes live in github.com/invowk/goplint)
make check-goplint-consumer-routed
```

Add the surface-specific gates from `.agents/rules/checklist.md`; do not treat a
focused pass as completion evidence for the repository.
