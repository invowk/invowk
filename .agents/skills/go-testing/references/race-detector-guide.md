# Race Detector Deep Dive

Treat a race report as strong evidence of unsynchronized executed accesses and
inspect both stacks. Avoid absolute "no false positives" claims: `unsafe`, cgo,
runtime instrumentation, and stale external-library assumptions can require
additional confirmation even though ordinary Go reports are highly reliable.

## How It Works

The Go race detector (ThreadSanitizer-based) instruments memory accesses at
compile time and tracks happens-before edges at runtime. Synchronization
operations (channel send/receive, mutex lock/unlock, `sync.WaitGroup`, atomic
operations) create those edges. A data race is two accesses to the same memory
where:
1. At least one is a write
2. They are from different goroutines
3. There is no happens-before relationship between them

Memory and CPU overhead are workload- and platform-dependent; measure the
failing job rather than relying on one fixed ratio.

### What It Does NOT Catch

- **Unexecuted code paths**: only races on code that actually runs during the test
- **Goroutine leaks**: not a data race (use `goleak` or manual checks)
- **Deadlocks**: not a data race (use `-timeout` flag for detection)
- **Logic races**: correct synchronization but wrong algorithm (e.g., check-then-act)

## Platform-Specific Behavior

- Windows overhead is typically higher than Linux/macOS, and `TerminateProcess`
  semantics interact poorly with race-detector shutdown.
- gotestsum false-FAILs on parallel subtests (most visible on macOS, where
  `-race` timing makes them more frequent) are a missing-`-v` problem, not a race:
  see `SKILL.md` "gotestsum Integration".
- Windows TUI race reports: do not assume lipgloss terminal detection and do not
  add package-wide `TestMain` pre-warming as a generic fix (it was a no-op).
  Inspect both access stacks and distinguish a real race from timeout or memory
  pressure under `-race`. Canonical guidance: `.agents/skills/windows-testing/SKILL.md`.

## Common Race Patterns in Go Tests

### 1. `t.Fatal` in Goroutines

Not a data race but a `FailNow` misuse (`runtime.Goexit` exits only the worker
goroutine). See the `testinggoroutine` entry in `go-vet-analyzers.md`; report
failures to the test goroutine with `t.Error` plus a channel.

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
| `GORACE="halt_on_error=1"` | 0 | Crash immediately on first race |
| `GORACE="history_size=N"` | 1 | Per-goroutine history buffer (powers of 2: 0-7) |
| `GORACE="atexit_sleep_ms=N"` | 1000 | Wait before exit to flush reports |
| `GORACE="log_path=FILE"` | stderr | Write reports to file |

`halt_on_error=1` is optional for local debugging. CI does not set `GORACE`;
halting early would also interact with gotestsum `--rerun-fails`.
