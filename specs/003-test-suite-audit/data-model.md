# Data Model: Test Suite Audit

**Feature**: 003-test-suite-audit | **Date**: 2026-01-29

## Overview

This document defines the types and structures for the new test helper infrastructure in the `testutil` package. No persistent data storage is involved; all entities are in-memory structures used during test execution.

---

## Entities

### 1. Clock (Time Abstraction)

**Purpose**: Enable deterministic testing of time-dependent code.

```go
// Clock abstracts time operations for testing.
type Clock interface {
    // Now returns the current time.
    Now() time.Time
    // After returns a channel that receives the current time after duration d.
    After(d time.Duration) <-chan time.Time
    // Since returns the time elapsed since t.
    Since(t time.Time) time.Duration
}
```

**Fields**: Interface only (no fields).

**Relationships**:
- Injected into types that perform time-based operations (e.g., SSH server token expiration)
- Two implementations: `RealClock` (production) and `FakeClock` (testing)

**Validation Rules**: None (behavioral contract only).

**State Transitions**: N/A (stateless interface).

---

### 2. RealClock

**Purpose**: Production implementation using actual system time.

```go
// RealClock implements Clock using the actual system time.
type RealClock struct{}
```

**Fields**: None (stateless).

**Relationships**: Implements `Clock` interface.

**Validation Rules**: None.

**State Transitions**: N/A.

---

### 3. FakeClock

**Purpose**: Test implementation allowing manual time control.

```go
// FakeClock implements Clock with manually controlled time for testing.
type FakeClock struct {
    current time.Time   // Current fake time
    mu      sync.Mutex  // Protects current from concurrent access
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `current` | `time.Time` | The current fake time, advanced by test code |
| `mu` | `sync.Mutex` | Protects concurrent access to `current` |

**Relationships**: Implements `Clock` interface.

**Validation Rules**:
- `current` must be initialized (zero time causes confusing test failures)

**State Transitions**:
```
Initialized(t0) → Advance(d) → Initialized(t0+d) → ...
```

**Methods**:
```go
// NewFakeClock creates a FakeClock initialized to the given time.
func NewFakeClock(initial time.Time) *FakeClock

// Advance moves the clock forward by the specified duration.
func (c *FakeClock) Advance(d time.Duration)

// Set sets the clock to a specific time.
func (c *FakeClock) Set(t time.Time)
```

---

### 4. CommandOption

**Purpose**: Functional option for configuring test commands.

```go
// CommandOption configures a test command.
type CommandOption func(*invkfile.Command)
```

**Relationships**:
- Used by `NewTestCommand()` for flexible command construction
- Each option modifies a specific field of `invkfile.Command`

**Pre-defined Options**:

| Option | Effect |
|--------|--------|
| `WithScript(s string)` | Sets the implementation script |
| `WithRuntime(r RuntimeType)` | Sets the runtime (native, virtual, container) |
| `WithRuntimes(rs ...RuntimeType)` | Sets multiple runtimes |
| `WithPlatform(p string)` | Adds a platform constraint |
| `WithEnv(key, value string)` | Adds an environment variable |
| `WithFlag(name string, opts ...FlagOption)` | Adds a flag definition |
| `WithArg(name string, opts ...ArgOption)` | Adds an argument definition |
| `WithDependency(d Dependency)` | Adds a dependency |
| `WithWorkDir(dir string)` | Sets the working directory |
| `WithDescription(desc string)` | Sets the command description |

---

### 5. FlagOption / ArgOption

**Purpose**: Configure flag and argument definitions within test commands.

```go
// FlagOption configures a test command flag.
type FlagOption func(*invkfile.Flag)

// ArgOption configures a test command argument.
type ArgOption func(*invkfile.Arg)
```

**Pre-defined Flag Options**:

| Option | Effect |
|--------|--------|
| `FlagRequired()` | Marks the flag as required |
| `FlagDefault(v string)` | Sets default value |
| `FlagEnvMapping(env string)` | Sets environment variable mapping |
| `FlagShorthand(s string)` | Sets single-character shorthand |

**Pre-defined Arg Options**:

| Option | Effect |
|--------|--------|
| `ArgRequired()` | Marks the argument as required |
| `ArgDefault(v string)` | Sets default value |
| `ArgVariadic()` | Marks as variadic (accepts multiple values) |

---

### 6. Test File Organization

**Purpose**: Document the split structure for large test files.

This is a conceptual entity representing file organization, not a Go type.

| Original File | Split Files | Line Budget |
|---------------|-------------|-------------|
| `invkfile_test.go` | 8 files | ~800 each |
| `cmd_test.go` | 5 files | ~500 each |
| `discovery_test.go` | 3 files | ~600 each |
| `runtime_test.go` | 3 files | ~500 each |

**Split Categories**:

```
┌─────────────────────────────────────────────────────────────┐
│                    invkfile_test.go                         │
├─────────────────────────────────────────────────────────────┤
│ parsing_test.go     │ Script parsing, resolution, cache     │
│ deps_test.go        │ Dependency generation & validation    │
│ flags_test.go       │ Flag validation & mapping             │
│ args_test.go        │ Positional arguments                  │
│ platforms_test.go   │ Platform filtering, capabilities      │
│ env_test.go         │ Environment variables, isolation      │
│ workdir_test.go     │ Working directory configuration       │
│ schema_test.go      │ Schema validation edge cases          │
└─────────────────────────────────────────────────────────────┘
```

---

## Relationships Diagram

```
                    ┌─────────────────┐
                    │     Clock       │◄─────────── Interface
                    │   (interface)   │
                    └────────┬────────┘
                             │ implements
              ┌──────────────┴──────────────┐
              │                             │
      ┌───────▼───────┐            ┌────────▼────────┐
      │   RealClock   │            │    FakeClock    │
      │  (production) │            │     (test)      │
      └───────────────┘            └─────────────────┘


      ┌────────────────────────────────────────────────────┐
      │                 NewTestCommand()                    │
      └────────────────────────────────────────────────────┘
                             │
                             │ accepts
                             ▼
      ┌────────────────────────────────────────────────────┐
      │                CommandOption...                     │
      │  ┌─────────────┐ ┌─────────────┐ ┌───────────────┐ │
      │  │ WithScript  │ │ WithRuntime │ │ WithFlag      │ │
      │  └─────────────┘ └─────────────┘ └───────────────┘ │
      │  ┌─────────────┐ ┌─────────────┐ ┌───────────────┐ │
      │  │ WithPlatform│ │ WithEnv     │ │ WithArg       │ │
      │  └─────────────┘ └─────────────┘ └───────────────┘ │
      └────────────────────────────────────────────────────┘
                             │
                             │ produces
                             ▼
      ┌────────────────────────────────────────────────────┐
      │              *invkfile.Command                      │
      └────────────────────────────────────────────────────┘
```

---

## Migration Notes

### Helper Consolidation

When migrating from duplicated helpers:

1. **`testCommand()` in `pkg/invkfile/invkfile_test.go`**:
   ```go
   // Before
   cmd := testCommand("hello", "echo hello")

   // After
   cmd := testutil.NewTestCommand("hello", testutil.WithScript("echo hello"))
   ```

2. **`testCmd()` in `cmd/invowk/cmd_test.go`**:
   ```go
   // Before
   cmd := testCmd("hello", "echo hello")

   // After
   cmd := testutil.NewTestCommand("hello", testutil.WithScript("echo hello"))
   ```

3. **`setHomeDirEnv()` everywhere**:
   ```go
   // Before
   cleanup := setHomeDirEnv(t, tmpDir)
   defer cleanup()

   // After
   cleanup := testutil.SetHomeDir(t, tmpDir)
   defer cleanup()
   ```

### Clock Migration

For `sshserver` token expiration tests:

```go
// Before (in server.go)
func (s *Server) isTokenExpired(token *Token) bool {
    return time.Since(token.CreatedAt) > s.config.TokenTTL
}

// After (with Clock injection)
func (s *Server) isTokenExpired(token *Token) bool {
    return s.clock.Since(token.CreatedAt) > s.config.TokenTTL
}

// In test
clock := testutil.NewFakeClock(time.Now())
srv := sshserver.NewWithClock(cfg, clock)
token, _ := srv.GenerateToken("test")
clock.Advance(cfg.TokenTTL + time.Millisecond)  // Deterministic!
_, ok := srv.ValidateToken(token.Value)
// ok should be false
```
