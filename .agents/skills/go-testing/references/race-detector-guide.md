# Race Detector Deep Dive

## How It Works

The Go race detector is built on ThreadSanitizer v2 (TSan), originally developed at
Google. It instruments memory accesses at compile time and maintains a happens-before
graph at runtime.

### Shadow Memory

Every 8-byte aligned memory word gets a "shadow word" that tracks:
- The goroutine ID that last accessed it
- Whether the access was a read or write
- A logical clock value (vector clock component)

This is why memory overhead is 5-10x — every application word needs shadow state.

### Happens-Before Tracking

The detector maintains a vector clock per goroutine. Synchronization operations
(channel send/receive, mutex lock/unlock, `sync.WaitGroup`, atomic operations)
create happens-before edges. A data race is two accesses to the same memory where:
1. At least one is a write
2. They are from different goroutines
3. There is no happens-before relationship between them

### What It Does NOT Catch

- **Unexecuted code paths**: only races on code that actually runs during the test
- **Goroutine leaks**: not a data race (use `goleak` or manual checks)
- **Deadlocks**: not a data race (use `-timeout` flag for detection)
- **Logic races**: correct synchronization but wrong algorithm (e.g., check-then-act)

## Platform-Specific Behavior

### Linux

Uses `futex` syscalls for internal synchronization. Lowest overhead of the three
platforms. `TSAN_OPTIONS` environment variable can tune behavior but is rarely needed.

### macOS

Uses Mach VM operations for shadow memory allocation. Performance is similar to
Linux on Apple Silicon (M1+). On Intel Macs, overhead is slightly higher.

The key macOS-specific race issue in invowk is the gotestsum `-v` requirement:
without verbose output, parallel subtest status is misreported. This is not a
race detector issue per se, but the `-race` flag exacerbates it by changing timing.

### Windows

Uses Windows synchronization primitives internally (`CRITICAL_SECTION`, `SRWLock`).
Overhead is typically higher than Linux/macOS for two reasons:
1. Windows API calls have higher baseline cost than Linux syscalls
2. `TerminateProcess` semantics interact poorly with TSan shutdown

#### The lipgloss Terminal Detection Race

The `lipgloss` library (used by the TUI) performs terminal capability detection
using `sync.Once`. On Windows, multiple tests initializing lipgloss concurrently
can trigger a race on the terminal state cache because the underlying Windows
console API calls (`GetConsoleMode`) are not atomic with respect to the `sync.Once`
initialization pattern across packages.

**Fix**: Pre-warm lipgloss in `TestMain` before any parallel tests run:

```go
//go:build windows

package tui

import (
    "os"
    "testing"

    "github.com/charmbracelet/lipgloss"
)

func TestMain(m *testing.M) {
    // Pre-initialize lipgloss to avoid race condition on Windows
    // where concurrent tests trigger parallel terminal detection.
    lipgloss.NewStyle()
    os.Exit(m.Run())
}
```

This pattern lives in `internal/tui/testmain_windows_test.go`.

## Common Race Patterns in Go Tests

### 1. `t.Fatal` in Goroutines

```go
// BUG: t.Fatal calls runtime.Goexit() — only exits the goroutine, not the test
go func() {
    if err != nil {
        t.Fatal(err) // race: accesses test state from non-test goroutine
    }
}()
```

Fix: use `t.Error` + channel, or communicate failures back to the test goroutine.

### 2. Shared Mock Without Synchronization

```go
// BUG: mock.calls accessed from multiple parallel subtests without sync
mock := &MockService{}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        mock.Do(tt.input) // race: concurrent writes to mock.calls
    })
}
```

Fix: create a new mock per subtest, or protect with `sync.Mutex`.

### 3. CUE Thread Safety

`cue.Value` and `*cue.Context` are NOT thread-safe. `Unify()` and `CompileString()`
mutate internal state. Never share across parallel subtests.

```go
// BUG: shared CUE context across parallel subtests
ctx := cuecontext.New()
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        v := ctx.CompileString(tt.input) // race: mutates ctx internal state
    })
}
```

Fix: serial subtests with `//nolint:tparallel`, or create a new CUE context per subtest.

### 4. Process-Global State

```go
// BUG: os.Stdin replacement races with any parallel test that reads stdin
oldStdin := os.Stdin
r, w, _ := os.Pipe()
os.Stdin = r // race: process-global
defer func() { os.Stdin = oldStdin }()
```

Fix: never use `t.Parallel()` when modifying `os.Stdin`, `os.Stdout`, `os.Stderr`, or calling `os.Chdir`.

### 5. Map Concurrent Access

```go
// BUG: range over map produces non-deterministic key order; concurrent map writes race
results := make(map[string]int)
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        results[tt.name] = compute(tt.input) // race: concurrent map write
    })
}
```

Fix: use `sync.Map`, or collect results via channels, or use per-subtest local variables.

## Reading Race Detector Output

A race report looks like:

```
==================
WARNING: DATA RACE
Write at 0x00c0001a4020 by goroutine 8:
  pkg.(*Foo).Bar()
      /path/to/foo.go:42 +0x128

Previous read at 0x00c0001a4020 by goroutine 7:
  pkg.(*Foo).Baz()
      /path/to/foo.go:57 +0x94

Goroutine 8 (running) created at:
  pkg.TestConcurrent()
      /path/to/foo_test.go:23 +0x1a4

Goroutine 7 (running) created at:
  pkg.TestConcurrent()
      /path/to/foo_test.go:20 +0x150
==================
```

**How to read it:**
1. **"Write at"**: the racing write — look at the file:line
2. **"Previous read/write"**: the conflicting access
3. **"Goroutine N created at"**: where each goroutine was spawned
4. The fix is to add synchronization between the two access points

## Race Detector Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GORACE="halt_on_error=1"` | 0 | Crash immediately on first race (useful for CI) |
| `GORACE="history_size=N"` | 1 | Per-goroutine history buffer (powers of 2: 0-7) |
| `GORACE="atexit_sleep_ms=N"` | 1000 | Wait before exit to flush reports |
| `GORACE="log_path=FILE"` | stderr | Write reports to file |

CI recommendation: `GORACE="halt_on_error=1"` to fail fast on races.
