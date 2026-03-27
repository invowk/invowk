---
name: macos-testing
description: >-
  Deep macOS-specific testing knowledge for Go. Covers APFS case-insensitivity
  (case-preserving), /tmp → /private/tmp symlink pitfalls, kqueue vs inotify
  behavior (more aggressive event coalescing), timer coalescing (100ms sleep
  may take 200ms), file descriptor limits (soft 256 vs Linux 1024), flock
  inheritance across fork, code signing edge cases, and ARM64 memory ordering
  on Apple Silicon. Use when debugging macOS-only test failures, watcher test
  flakiness, or understanding why timing tests flake on macos-15 CI runners.
disable-model-invocation: false
---

# macOS Testing Skill

macOS is deceptively similar to Linux but differs in subtle ways that cause
intermittent test failures. Most cross-platform Go code works on macOS without
changes, but filesystem semantics, timer behavior, and process lifecycle have
meaningful differences that surface as test flakiness rather than hard failures.

This skill is the primary reference for macOS-specific testing concerns. No
dedicated macOS rule exists in `.agents/rules/`; this skill fills that gap.

## Normative Precedence

1. `.agents/rules/testing.md` -- authoritative test policy.
2. `.agents/rules/go-patterns.md` -- context propagation, code style.
3. `.agents/skills/go-testing/SKILL.md` -- Go testing toolchain knowledge, decision frameworks.
4. This skill -- macOS OS primitives affecting test behavior.
5. `.agents/skills/testing/SKILL.md` -- invowk-specific test patterns, testscript, TUI/container.

If this skill conflicts with a rule, follow the rule.

**Cross-references:**
- `go-testing` -- primary testing entry point; routes macOS symptoms here.
- `windows-testing` -- Windows OS primitives (TerminateProcess, NTFS, timer resolution).
- `linux-testing` -- Linux OS primitives (inotify, cgroups, flock, container infra).

---

## Process Lifecycle

macOS uses a BSD-derived kernel (XNU) with a Mach microkernel layer underneath.
Process creation follows the POSIX `fork+exec` model, but modern macOS strongly
prefers `posix_spawn` for new process creation. Go's `exec.Command` uses
`fork+exec` internally, which is safe because Go's runtime synchronizes all
threads around the fork point.

Mach ports provide the low-level IPC mechanism. Each task (process) has a Mach
port namespace. Mach exception handling (EXC_BAD_ACCESS, EXC_BREAKPOINT, etc.)
is delivered via ports before POSIX signal conversion. Go's runtime handles this
transparently -- test code does not interact with Mach ports directly.

Signal handling on macOS follows full POSIX semantics. Go installs `SA_ONSTACK`
signal handlers and delivers signals to a dedicated signal-handling thread.
However, Mach exceptions can intercept certain faults before POSIX delivery,
which affects debugger-based test approaches. Delve works correctly for
user-space Go programs.

Deep dive: `references/process-signals.md`.

---

## APFS File System

APFS (Apple File System) is the default on all modern macOS. Its most impactful
testing characteristic is **case-insensitive, case-preserving** behavior.

### Case Insensitivity

On APFS (default configuration), `README.md` and `readme.md` refer to the
**same file**. Creating `Readme.md` when `README.md` already exists overwrites
it silently. This breaks tests that:
- Create multiple files with case-variant names in the same directory.
- Assert that two files with different-case names coexist.
- Use case-sensitive path comparisons against filesystem results.

```go
// WRONG on macOS: these are the SAME file
os.WriteFile(dir+"/Config.cue", data1, 0o644)
os.WriteFile(dir+"/config.cue", data2, 0o644)
// data1 is now LOST -- only data2 exists

// CORRECT: use distinct names
os.WriteFile(dir+"/app_config.cue", data1, 0o644)
os.WriteFile(dir+"/user_config.cue", data2, 0o644)
```

### The `/tmp` Symlink

`os.TempDir()` returns `/tmp` on macOS, but `/tmp` is a symlink to
`/private/tmp`. This means:

- `os.TempDir()` returns `/tmp`
- `filepath.EvalSymlinks("/tmp")` returns `/private/tmp`
- `t.TempDir()` returns the **resolved** path (under `/private/tmp/...`)
- Hardcoded `/tmp/...` paths will NOT match `t.TempDir()` output

Any test that compares absolute paths will fail if one side resolves the
symlink and the other does not. Always use `t.TempDir()` for test directories,
and never hardcode `/tmp` in test assertions.

```go
// WRONG: path comparison may fail
expected := "/tmp/test-dir/output.txt"
got := filepath.Join(t.TempDir(), "output.txt") // /private/tmp/...

// CORRECT: use t.TempDir() consistently
dir := t.TempDir()
expected := filepath.Join(dir, "output.txt")
```

### Unicode Normalization

APFS stores filenames in NFD (decomposed Unicode form). Go strings typically
use NFC (composed form). A filename containing accented characters may not
match a Go string comparison even though they look identical:

- APFS stores: `cafe\u0301` (NFD -- `e` + combining accent)
- Go literal: `caf\u00e9` (NFC -- precomposed e-acute)
- `bytes.Equal([]byte(nfd), []byte(nfc))` returns `false`

This rarely affects invowk tests (which use ASCII filenames), but is relevant
for any test creating files with non-ASCII names.

### Extended Attributes and .DS_Store

macOS stores metadata as extended attributes (`xattr`). The `com.apple.quarantine`
attribute is set on downloaded files and can cause Gatekeeper prompts.
`os.Stat()` does not include xattr size. `os.Remove()` removes xattrs too.

Finder creates `.DS_Store` files when viewing a directory. These can appear
unexpectedly in `os.ReadDir()` output. Tests listing directory contents should
filter or ignore `.DS_Store`.

Full deep dive: `references/filesystem-kqueue.md`.

---

## kqueue vs inotify

macOS uses `kqueue` for file system event notification (Go's `fsnotify` library
abstracts this). Key differences from Linux's `inotify`:

### Event Coalescing

kqueue coalesces events more aggressively than inotify. Multiple rapid writes
to the same file may produce a single `NOTE_WRITE` event instead of one per
write. This is by design for efficiency but means file watchers may miss
intermediate states.

**Impact on invowk**: The `time.Sleep` calls in `internal/watch/watcher_test.go`
(8 occurrences) are necessary because kqueue may coalesce rapid writes into
fewer events than expected. The sleep between writes ensures each write
generates a separate fsnotify event rather than being batched by the OS.

### File Descriptor-Based Watches

kqueue watches file descriptors, not paths. If a file is renamed, the watch
follows the file descriptor (not the name). If a file is deleted and recreated,
the old watch is dead -- the new file has a different descriptor. This differs
from inotify, which can report the old and new paths during rename.

### No Recursive Watching

kqueue does not support recursive directory watching. To watch a directory tree,
each subdirectory must be opened and added to the kqueue individually. `fsnotify`
handles this automatically on macOS but it means higher file descriptor usage.

### Resource Implications

Each watched file/directory consumes one file descriptor. Combined with the
lower default file descriptor limit on macOS (see "File Descriptor Limits"
below), deep directory trees can exhaust available descriptors.

Full comparison table and deep dive: `references/filesystem-kqueue.md`.

---

## flock Behavior

On macOS, `flock` locks are inherited across `fork()`. Child processes created
via `exec.Command` (which does fork+exec internally) briefly inherit the
parent's flock during the fork-to-exec window. On Linux, flocks are
per-open-file-description and are NOT inherited across fork.

This difference is **academic for invowk** because `run_lock_other.go`
(build tag `!linux`) returns `errFlockUnavailable` on macOS, falling back
to `sync.Mutex`. On macOS/Windows, Podman runs inside a Linux VM
(podman machine / WSL2), so a host-side flock cannot reach the VM's
filesystem. The in-process mutex is the best available protection.

If future code adds macOS flock support, be aware:
- Locks inherited during fork may cause unexpected serialization.
- `flock` advisory locks on macOS are per-process (not per-fd as on Linux).
- NFS-mounted volumes on macOS do not reliably support `flock`.

---

## Timer Coalescing

macOS aggressively coalesces timers to save power. This feature, introduced in
macOS 10.9 Mavericks, groups timer firings to reduce CPU wake-ups.

### How It Affects Tests

`time.Sleep(100 * time.Millisecond)` may actually sleep 100-200ms depending on
system load and power state. `time.After` and `time.Ticker` channels are
similarly affected. The coalescing window grows when the system is under light
load (ironic -- tests are more likely to flake on idle machines).

### Comparison with Other Platforms

| Platform | Timer Resolution | Behavior |
|----------|-----------------|----------|
| Linux | ~1ms (hrtimers) | Predictable, minimal coalescing |
| Windows | 15.6ms default | Consistent quantum, adjustable via `timeBeginPeriod` |
| macOS | 1ms nominal | Non-deterministic coalescing; same test may pass or fail |

### Why This Matters for invowk

Both `internal/watch/watcher_test.go` and TUI tmux tests use `time.Sleep` for
synchronization:

- **Watcher tests**: Sleeps between file writes (10ms, 50ms) ensure kqueue
  delivers separate events. Sleeps after write sequences (200ms, 300ms) wait
  for debounce windows to close. Timer coalescing can extend these sleeps,
  which is acceptable (tests wait longer) but can cause timeouts if the total
  accumulated delay exceeds the test deadline.

- **TUI tmux tests**: Sleeps between key-send and content-read operations allow
  the TUI model to process input. Coalesced timers may delay processing,
  causing assertion failures if content is checked too early.

### Mitigation Strategies

1. **Use generous timeouts**: 5s safety timeouts instead of 1s.
2. **Prefer event-based synchronization**: channels, mutexes, condition variables
   over sleeps wherever possible.
3. **Poll with deadline**: instead of `time.Sleep(200ms)`, use a poll loop:
   ```go
   deadline := time.Now().Add(2 * time.Second)
   for time.Now().Before(deadline) {
       if condition() {
           break
       }
       time.Sleep(50 * time.Millisecond)
   }
   ```
4. **Accept timing variance**: tests that measure timing should use wide
   tolerance (2x-3x expected duration on macOS vs 1.5x on Linux).

---

## File Descriptor Limits

macOS has a default soft limit of 256 open file descriptors per process, compared
to 1024 on Linux. Go's runtime raises the soft limit to the hard limit at
startup via `setrlimit`, but if the hard limit is also constrained, tests
opening many files or sockets simultaneously can hit `EMFILE` ("too many open
files").

### Practical Impact

- kqueue watches consume one fd per watched path (see "kqueue" section above).
- Parallel tests in the same process share the fd pool.
- Tests creating many temp files without closing them can exhaust fds.

### Checking Limits

```bash
ulimit -n      # soft limit (typically 256 pre-Go, raised by Go runtime)
ulimit -Hn     # hard limit (varies by system, typically 10240+)
```

CI runners (`macos-15`) typically have higher hard limits, but they are not
infinite. Tests that open many files should close them promptly.

### invowk Relevance

The watcher tests in `internal/watch/` open file descriptors for each watched
path. Deep directory hierarchies can consume significant fd budget. The current
test fixtures use shallow directory structures, keeping fd usage manageable.

---

## Code Signing and Gatekeeper

Binaries produced by `go build` are unsigned. On macOS, Gatekeeper may
quarantine unsigned binaries downloaded from the internet, blocking execution.

In CI (`macos-15` runner), this is handled by the runner environment -- binaries
built locally during the CI job are not quarantined. For local development,
unsigned binaries may be blocked on first execution.

### Workarounds

```bash
# Remove quarantine attribute
xattr -d com.apple.quarantine ./invowk

# Ad-hoc sign (no Apple Developer ID needed)
codesign -s - ./invowk
```

This rarely affects tests directly but can cause "cannot execute binary file"
errors when tests spawn the invowk binary on a developer machine where the
binary was downloaded rather than built locally.

---

## SIP (System Integrity Protection)

System Integrity Protection prevents modification of system directories
(`/usr/bin/`, `/System/`) and restricts `ptrace` on system processes.

### Impact on Testing

- Cannot modify `/usr/bin/` or `/usr/local/bin/` without disabling SIP.
- `ptrace`-based debugging is restricted for certain system processes, but
  Delve works correctly for user-space Go programs.
- Rarely relevant for invowk tests, which operate entirely in user space.

---

## CI Configuration

The invowk project runs macOS tests on `macos-15` (Apple Silicon M1) runners
in `-short` mode with no container engine available.

### CI Matrix Entry

```yaml
- os: macos
  runner: macos-15
  engine: ""           # No container engine
  test-mode: short     # Unit tests only
```

There is also a `macos-15-intel` entry for cross-compilation verification in
the build matrix, but tests run only on ARM64.

### Test Execution

```bash
# Short mode (unit + CLI tests, no container integration)
gotestsum \
  --format testdox \
  --junitfile test-results.xml \
  --rerun-fails \
  --rerun-fails-max-failures 3 \
  --rerun-fails-report rerun-report.txt \
  --packages ./... \
  -- -race -short -v -timeout 20m -coverprofile=coverage.out

# CLI integration tests (separate step)
gotestsum \
  --format testdox \
  --junitfile cli-test-results.xml \
  --rerun-fails \
  --rerun-fails-max-failures 3 \
  --packages ./tests/cli/... \
  -- -race -timeout 5m
```

### The gotestsum `-v` Requirement

Without the `-v` flag, gotestsum cannot reconcile parallel subtest statuses.
The specific failure mode: `testdox` format without `-v` does not receive
per-subtest PASS/FAIL lines from Go's test output. When all subtests of a
parent test pass in parallel, gotestsum may still report the parent as FAIL
because it never saw individual completion messages. This issue is most
frequently observed on macOS CI runners, likely due to scheduling differences
on Apple Silicon.

**The fix is always `-v`**: every gotestsum invocation in CI uses `-v` for
this reason. Locally, `-v` is optional for `go test` but required for
gotestsum `--rerun-fails`.

### No Container Engine

macOS CI runners do not have Docker or Podman installed. All container runtime
tests are gated by `testing.Short()` and auto-skip in short mode. The
`[!container-available]` testscript condition also handles this for CLI tests.

---

## ARM64 Specifics (Apple Silicon)

CI runs on Apple Silicon (M1) via the `macos-15` runner. Apple's ARM64
implementation uses a relaxed memory ordering model by default, unlike x86's
Total Store Order (TSO).

### Memory Ordering

- **Pure Go code is safe**: Go's memory model provides sequential consistency
  for properly synchronized programs. The compiler and runtime insert the
  necessary memory barriers on ARM64. `sync.Mutex`, channels, `sync/atomic`,
  and `sync.WaitGroup` all work identically on ARM64 and x86.
- **Rosetta 2 emulation**: x86 binaries running under Rosetta 2 get TSO
  emulation. Native ARM64 binaries see the relaxed model. Go cross-compiles
  natively for ARM64, so Rosetta is not involved.
- **CGo interop**: C code compiled for ARM64 sees the weak memory model. If
  invowk ever uses cgo (it currently does not), C-side synchronization must
  use explicit barriers or atomics.
- **Atomic operations**: `sync/atomic` operations are cheaper on ARM64 than
  on x86 because ARM64 has dedicated atomic instructions (LDXR/STXR) that
  do not require the bus lock used by x86 `LOCK` prefix instructions.

### Race Detector on ARM64

The race detector uses Mach VM operations for shadow memory allocation on
macOS (both Intel and ARM64). Performance on Apple Silicon is similar to
Linux ARM64. The race detector works identically on both architectures --
it detects the same races regardless of the underlying memory model because
it tracks happens-before relationships, not hardware memory ordering.

---

## Invowk-Specific Patterns

### Watcher Test Timing

`internal/watch/watcher_test.go` has 7 `time.Sleep` calls compensating for
kqueue event coalescing and debounce verification:

| Sleep Duration | Purpose |
|----------------|---------|
| 10ms | Between rapid file writes to prevent kqueue batching |
| 50ms | Short pause for event delivery before assertions |
| 100ms | Wait for debounce timer to expire |
| 200ms | Negative-condition check (verify no spurious callbacks) |
| 300ms | Longer negative-condition window for complex test scenarios |

These values are tuned to balance reliability (large enough to survive timer
coalescing) against test speed (not wastefully long).

### TUI tmux Tests

TUI tmux tests (`internal/tui/tui_tmux_test.go`) use `time.Sleep` between
key-send and content-read operations. These are similarly affected by timer
coalescing.

### Testscript `[darwin]` Condition

Testscript provides the `[darwin]` built-in condition for platform-specific
test blocks. Use it for macOS-specific assertions:

```
[darwin] stdout 'Library/Application Support/invowk'
[linux] stdout '.config/invowk'
```

Note: the condition is `[darwin]` (Go's GOOS value), not `[macos]`.

### flock Fallback

`internal/runtime/run_lock_other.go` (build tag `!linux`) returns
`errFlockUnavailable`, causing the container runtime to fall back to
`sync.Mutex` for run serialization. This is correct behavior -- see the
"flock Behavior" section above.

### No `*_darwin_test.go` Files

The project currently has no `*_darwin_test.go` build-tagged test files.
macOS-specific test behavior is handled through runtime `GOOS` checks and
testscript `[darwin]` conditions rather than build tags.

---

## Common macOS Test Failure Matrix

| Symptom | Probable Cause | Investigation | Fix |
|---------|---------------|---------------|-----|
| Path comparison fails in tests using `t.TempDir()` | `/tmp` -> `/private/tmp` symlink | Check if one side resolves symlinks | Use `t.TempDir()` consistently; never hardcode `/tmp` |
| Two files with case-variant names collide | APFS case-insensitive | Check APFS default config | Use distinct filenames; do not rely on case distinction |
| Watcher test misses file events | kqueue event coalescing | Increase inter-write sleep | Add `time.Sleep` between writes; use generous safety timeouts |
| `time.Sleep` takes longer than expected | Timer coalescing | Check actual vs expected sleep duration | Use 2-3x safety margin; prefer event-based sync |
| gotestsum reports FAIL but all subtests pass | Missing `-v` flag | Check gotestsum invocation | Always use `-v` with gotestsum `--rerun-fails` |
| "too many open files" in watcher/parallel tests | Low fd soft limit | `ulimit -n` check | Close fds promptly; reduce parallel file operations |
| Binary execution blocked by Gatekeeper | Missing code signature | Check xattr quarantine | `xattr -d com.apple.quarantine binary` or `codesign -s -` |
| Unicode filename comparison fails | APFS NFD vs Go NFC strings | Compare byte representations | Normalize to same form before comparison; avoid non-ASCII in test filenames |
| Container tests skip unexpectedly | No container engine on macOS CI | Verify `testing.Short()` gating | Expected behavior; container tests are Linux-only in CI |
| Race detector finds races only on macOS | Timing differences expose latent races | Run with `-race` locally | Fix the race; ARM64 scheduling differs from x86 |

---

## Related Skills

| Skill | When to Use |
|-------|-------------|
| `go-testing` | Primary testing entry point; Go toolchain, flags, coverage, parallelism |
| `windows-testing` | Windows OS primitives: TerminateProcess, NTFS, timer resolution, lipgloss race |
| `linux-testing` | Linux OS primitives: inotify, cgroups, flock, container infrastructure, OOM |
| `testing` | Invowk-specific: testscript patterns, TUI component testing, container runtime |
| `tmux-testing` | TUI end-to-end testing with tmux |
| `tui-testing` | VHS-based TUI visual testing |
