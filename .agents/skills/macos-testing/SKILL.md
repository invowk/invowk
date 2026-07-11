---
name: macos-testing
description: >-
  Invowk macOS and Darwin testing guidance for APFS case and normalization
  behavior, `/tmp` and `/var` path aliases, kqueue/fsnotify events, timer and
  descriptor pressure, process cleanup, code signing, and Apple Silicon.
  Use when debugging macOS-only failures, virtual path assertion drift,
  watcher or TUI timing flakes, `[darwin]` testscript branches, or failures in
  the current macOS CI matrix.
---

# macOS Testing

Use this skill as a failure router. Verify current behavior in the checkout and
load only the reference for the affected primitive.

## Precedence and References

1. Follow `.agents/rules/testing.md` for mandatory test policy.
2. Use `.agents/skills/go-testing/SKILL.md` for Go flags, contexts,
   parallelism, race reports, and gotestsum behavior.
3. Use `.agents/skills/testing/SKILL.md` for Invowk-specific test patterns.
4. Apply this skill for Darwin filesystem, event, process, and CI behavior.

Load references conditionally:

- Read [references/filesystem-kqueue.md](references/filesystem-kqueue.md) for
  APFS, path aliases, kqueue, flock, timers, descriptor limits, and signing.
- Read [references/process-signals.md](references/process-signals.md) for XNU
  process creation, signal delivery, cancellation, and debugging.

## First Checks

1. Capture the failing test, runner architecture, path strings, event sequence,
   timeout layer, and race-detector status.
2. Read the current workflow rather than copying runner names or resource
   limits from guidance:

   ```bash
   rg -n 'macos|runner|gotestsum|timeout' .github/workflows/ci.yml .github/workflows/release.yml
   ```

3. Reproduce the smallest package or txtar case with `-count=1` and `-race`
   when concurrency is relevant.
4. Prefer filesystem identity and event evidence over assumptions based only on
   the displayed path or a single missed event.

## Filesystem Contract

- Default macOS APFS volumes are case-insensitive and case-preserving, but
  case-sensitive variants exist. Write portable tests that do not require two
  case-variant names to coexist.
- `/tmp` and `/var` may resolve through `/private`. Build expected test paths
  from the actual helper result. For identity checks, use `os.Stat` and
  `os.SameFile` rather than comparing unresolved and resolved strings.
- For resolver-backed APIs, derive expected strings through the same resolver
  contract as production. Do not replace an API string contract with ad hoc
  `EvalSymlinks` or host normalization.
- APFS preserves the normalization form supplied for a filename while current
  volumes are normally normalization-insensitive. Directory enumeration can
  therefore return bytes that differ from a canonically equivalent lookup
  string. Go does not normalize string literals automatically.

## Watcher and Timing Contract

- kqueue watches file descriptors and can coalesce events. A delete/recreate
  sequence produces a new object and may require watch registration to be
  refreshed.
- kqueue and `fsnotify` do not make directory watching recursive. Register each
  directory explicitly and handle new subdirectories.
- Do not assert exact write-event counts. Assert the resulting state or poll for
  a bounded condition through the existing debounce/watch helpers.
- Prefer channels, polling helpers, and explicit readiness over `time.Sleep`.
  When an external TUI or filesystem boundary requires a delay, derive it from
  current tests and keep a bounded outer timeout.
- Inspect `ulimit -n` and live descriptor ownership before attributing a failure
  to a copied macOS soft-limit value.

## Process and Architecture Contract

- Use `Cmd.Wait`/`Cmd.Run` and the repository's cleanup helpers for subprocess
  ownership. Consult `process-signals.md` before changing cancellation or
  process-tree behavior.
- Treat a race seen only on Apple Silicon as a real synchronization problem
  unless source evidence proves otherwise. Do not encode x86 ordering
  assumptions into tests.
- For downloaded binaries blocked from execution, inspect quarantine and code
  signing evidence. Do not weaken signing or system protections as a default
  test fix.
- Container integration coverage is owned by Linux CI. A macOS container skip
  is expected only when it matches the current workflow and test contract; it
  must not silently reduce Linux coverage.

## Invowk-Specific Patterns

- `internal/watch/` is the source of truth for current event/debounce timing.
- `tests/cli/tui_tmux_test.go` owns durable interactive TUI coverage; use
  `tmux-testing` for those flows and `tui-testing` only for visual diagnosis.
- Use testscript's `[darwin]` condition, not `[macos]`, for Darwin-specific
  expectations.
- `internal/container/run_lock_other.go` intentionally falls back from Linux
  flock to in-process serialization. Do not claim cross-binary locking on
  macOS without implementing it.
- Discover current Darwin-specific coverage rather than maintaining a copied
  inventory:

  ```bash
  rg --files -g '*_darwin_test.go' -g '*.txtar' internal pkg tests
  rg -n '\[darwin\]|runtime\.GOOS.*darwin' internal pkg tests
  ```

## Failure Matrix

| Symptom | Evidence to collect | Likely fix route |
|---|---|---|
| Absolute path mismatch | raw and resolved paths, resolver source | use the production resolver or identity assertion |
| Case-variant fixture collision | volume case behavior and created names | use portable distinct names |
| Canonically equivalent name differs by bytes | code points returned by enumeration | normalize only when the API contract permits it |
| Watcher misses or merges events | operation sequence and final state | state-based bounded assertion; refresh watches when needed |
| TUI/watcher timing flake | readiness signal and timeout layer | event/poll synchronization, not copied sleeps |
| `too many open files` | `ulimit -n` and live descriptor count | close leaks or bound concurrency |
| gotestsum parent status is confusing | current invocation and verbose output | follow `go-testing` rerun guidance |
| Binary execution is denied | quarantine/signature diagnostics | apply the narrow documented signing/quarantine remedy |
| Race appears only on ARM64 | both race stacks and shared state | fix synchronization; do not skip by architecture |

## Verification

Run the smallest affected surface first:

```bash
go test -count=1 -race ./internal/watch/...
go test -count=1 -race ./internal/runtime/... -run 'Virtual|Path|Watch'
go test -count=1 -race ./tests/cli/... -run 'TUI|Darwin'
```

Then run the repository gates required by `.agents/rules/checklist.md`, including
`make test`. Use current CI to obtain real macOS proof when the local host is not
Darwin; do not claim a Linux-only reproduction proves macOS behavior.

## Related Skills

| Skill | Use for |
|---|---|
| `go-testing` | Go test execution, race, context, benchmark, and coverage behavior |
| `testing` | Invowk testscript and component patterns |
| `windows-testing` | Windows process, path, and filesystem behavior |
| `linux-testing` | Linux process, inotify, container, and OOM behavior |
| `tmux-testing` | Durable interactive TUI tests |
| `tui-testing` | VHS visual debugging and documentation demos |
