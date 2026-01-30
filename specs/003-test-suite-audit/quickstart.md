# Quickstart: Using New Test Helpers

**Feature**: 003-test-suite-audit | **Date**: 2026-01-29

This guide shows how to use the new consolidated test helpers after the audit is complete.

---

## 1. Creating Test Commands

### Before (duplicated helpers)

```go
// In pkg/invkfile/invkfile_test.go
func testCommand(name, script string) Command {
    return Command{
        Name: name,
        Implementations: []Implementation{
            {Script: script, Runtimes: []RuntimeConfig{{Name: RuntimeNative}}},
        },
    }
}

cmd := testCommand("hello", "echo hello")
```

```go
// In cmd/invowk/cmd_test.go (slightly different!)
func testCmd(name, script string) *invkfile.Command {
    return &invkfile.Command{
        Name: name,
        Implementations: []invkfile.Implementation{
            {Script: script, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}},
        },
    }
}

cmd := testCmd("hello", "echo hello")
```

### After (consolidated helper)

```go
import "invowk-cli/internal/testutil/invkfiletest"

// Simple case - same behavior as before
cmd := invkfiletest.NewTestCommand("hello", invkfiletest.WithScript("echo hello"))

// With additional configuration
cmd := invkfiletest.NewTestCommand("greet",
    invkfiletest.WithScript("echo Hello, $NAME"),
    invkfiletest.WithRuntime(invkfile.RuntimeVirtual),
    invkfiletest.WithEnv("NAME", "World"),
)

// With flags
cmd := invkfiletest.NewTestCommand("deploy",
    invkfiletest.WithScript("deploy --env=$ENV"),
    invkfiletest.WithFlag("env",
        invkfiletest.FlagRequired(),
        invkfiletest.FlagShorthand("e"),
    ),
)

// With positional arguments
cmd := invkfiletest.NewTestCommand("copy",
    invkfiletest.WithScript("cp $1 $2"),
    invkfiletest.WithArg("source", invkfiletest.ArgRequired()),
    invkfiletest.WithArg("dest", invkfiletest.ArgRequired()),
)
```

---

## 2. Setting Home Directory

### Before (duplicated in 3 files)

```go
// In cmd/invowk/cmd_test.go, internal/config/config_test.go, internal/discovery/discovery_test.go
func setHomeDirEnv(t *testing.T, dir string) func() {
    t.Helper()
    switch runtime.GOOS {
    case "windows":
        return testutil.MustSetenv(t, "USERPROFILE", dir)
    default:
        return testutil.MustSetenv(t, "HOME", dir)
    }
}

cleanup := setHomeDirEnv(t, tmpDir)
defer cleanup()
```

### After (consolidated)

```go
import "invowk-cli/internal/testutil"

// Option 1: With defer
cleanup := testutil.SetHomeDir(t, tmpDir)
defer cleanup()

// Option 2: With t.Cleanup (recommended)
t.Cleanup(testutil.SetHomeDir(t, tmpDir))
```

---

## 3. Testing Time-Dependent Code

### Before (flaky)

```go
func TestExpiredToken(t *testing.T) {
    cfg := DefaultConfig()
    cfg.TokenTTL = 1 * time.Millisecond
    srv := New(cfg)

    token, _ := srv.GenerateToken("test-command")

    // FLAKY: May pass or fail based on system load
    time.Sleep(10 * time.Millisecond)

    _, ok := srv.ValidateToken(token.Value)
    if ok {
        t.Error("Expired token should not be valid")
    }
}
```

### After (deterministic)

```go
import "invowk-cli/internal/testutil"

func TestExpiredToken(t *testing.T) {
    cfg := DefaultConfig()
    cfg.TokenTTL = 1 * time.Minute  // Use realistic duration

    clock := testutil.NewFakeClock(time.Time{})  // Zero time uses fixed reference
    srv := NewWithClock(cfg, clock)  // Inject clock

    token, _ := srv.GenerateToken("test-command")

    // DETERMINISTIC: Advance clock past TTL
    clock.Advance(cfg.TokenTTL + time.Second)

    _, ok := srv.ValidateToken(token.Value)
    if ok {
        t.Error("Expired token should not be valid")
    }
}
```

**Note**: The code under test must accept a `Clock` interface:

```go
// In production code
type Server struct {
    clock Clock  // Add clock field
    // ...
}

func New(cfg Config) *Server {
    return NewWithClock(cfg, testutil.RealClock{})
}

func NewWithClock(cfg Config, clock Clock) *Server {
    return &Server{clock: clock, /* ... */}
}

func (s *Server) isTokenExpired(token *Token) bool {
    return s.clock.Since(token.CreatedAt) > s.config.TokenTTL
}
```

---

## 4. Testing TUI Components

### Pattern: Test model state, not terminal I/O

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "invowk-cli/internal/tui"
)

func TestChooseModel_Navigation(t *testing.T) {
    // Create model with options
    options := []tui.Option[string]{
        {Title: "Option A", Value: "a"},
        {Title: "Option B", Value: "b"},
        {Title: "Option C", Value: "c"},
    }
    model := tui.NewChooseModel(options)

    // Simulate key press
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})

    // Verify state changed
    if model.Selected() != 1 {
        t.Errorf("expected selected=1 after down, got %d", model.Selected())
    }
}

func TestChooseModel_Selection(t *testing.T) {
    options := []tui.Option[string]{
        {Title: "Option A", Value: "a"},
        {Title: "Option B", Value: "b"},
    }
    model := tui.NewChooseModel(options)

    // Navigate and select
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Verify selection
    value, ok := model.Value()
    if !ok {
        t.Fatal("expected selection to complete")
    }
    if value != "b" {
        t.Errorf("expected 'b', got %q", value)
    }
}

func TestConfirmModel_Toggle(t *testing.T) {
    model := tui.NewConfirmModel("Delete file?", false)

    // Initial state
    if model.Confirmed() {
        t.Error("should start unconfirmed")
    }

    // Toggle with tab
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})

    if !model.Confirmed() {
        t.Error("should be confirmed after tab")
    }
}
```

---

## 5. Testing Container Runtime (with mocks)

### Pattern: Mock exec.Command for argument verification

```go
import (
    "os/exec"
    "invowk-cli/internal/container"
)

func TestDockerBuild_Arguments(t *testing.T) {
    // Capture executed command
    var executed string
    var capturedArgs []string

    // Override exec.Command
    oldExecCommand := container.ExecCommand
    container.ExecCommand = func(name string, args ...string) *exec.Cmd {
        executed = name
        capturedArgs = args
        // Return a command that succeeds without doing anything
        return exec.Command("true")
    }
    defer func() { container.ExecCommand = oldExecCommand }()

    // Run the function under test
    engine := container.NewDockerEngine()
    _ = engine.Build(context.Background(), container.BuildOptions{
        Dockerfile: "Dockerfile",
        Tag:        "test:latest",
        NoCache:    true,
    })

    // Verify arguments
    if executed != "docker" {
        t.Errorf("expected 'docker', got %q", executed)
    }
    if !slices.Contains(capturedArgs, "--no-cache") {
        t.Error("expected --no-cache flag")
    }
    if !slices.Contains(capturedArgs, "-t") {
        t.Error("expected -t flag")
    }
}
```

---

## Migration Checklist

When updating existing tests to use new helpers:

- [ ] Replace `testCommand()` / `testCmd()` with `testutil.NewTestCommand()`
- [ ] Replace `setHomeDirEnv()` with `testutil.SetHomeDir()`
- [ ] Replace `time.Sleep()` in assertions with `FakeClock.Advance()`
- [ ] Verify all tests still pass: `make test`
- [ ] Verify no duplicate helper definitions remain: `grep -r "func testCommand\|func testCmd\|func setHomeDirEnv" --include="*_test.go"`
