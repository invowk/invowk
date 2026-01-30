# Research: Test Suite Audit and Improvements

**Feature**: 003-test-suite-audit | **Date**: 2026-01-29

## Research Tasks Completed

### 1. Large Test File Analysis

**Decision**: Split test files exceeding 800 lines using logical concern boundaries.

**Rationale**: The project constitution (`.claude/rules/testing.md`) explicitly states test files MUST NOT exceed 800 lines. Large monolithic files are difficult to navigate for both humans and AI agents. Each split file should cover a single logical area.

**Alternatives Considered**:
- **Keep large files**: Rejected - violates constitution, creates maintenance burden
- **Split by alphabetical order**: Rejected - would not create logical groupings
- **Split by test function count**: Rejected - count doesn't reflect complexity or concern

**Findings - Current State**:

| File | Lines | Test Count | Split Strategy |
|------|-------|------------|----------------|
| `pkg/invkfile/invkfile_test.go` | 6,597 | 149 | 8 files by concern |
| `cmd/invowk/cmd_test.go` | 2,567 | 87 | 5 files by concern |
| `internal/discovery/discovery_test.go` | 1,842 | 39 | 3 files by concern |
| `pkg/invkmod/operations_test.go` | 1,683 | N/A | Monitor (near threshold) |
| `internal/runtime/runtime_test.go` | 1,605 | 33 | 3 files by concern |
| `internal/runtime/container_integration_test.go` | 847 | N/A | At threshold; acceptable |

**Concern Categories Identified in invkfile_test.go**:
1. Parsing: Script parsing, resolution, caching (~800 lines)
2. Dependencies: Dependency parsing, generation, validation (~700 lines)
3. Flags: Flag validation, mapping, boolean handling (~600 lines)
4. Args: Positional argument handling, variadic args (~500 lines)
5. Platforms: Platform filtering, host SSH, capabilities (~800 lines)
6. Environment: Environment variables, isolation, precedence (~800 lines)
7. Workdir: Working directory configuration (~400 lines)
8. Schema: Schema validation edge cases (~500 lines)

---

### 2. Duplicated Helper Analysis

**Decision**: Consolidate identical helpers into `testutil` package using options pattern.

**Rationale**: Three identical implementations of `testCommand()`/`testCmd()` exist across packages. Same for `setHomeDirEnv()`. Consolidation reduces maintenance burden and ensures consistent behavior.

**Alternatives Considered**:
- **Leave duplicates**: Rejected - violates DRY, creates sync issues
- **Simple function consolidation**: Rejected - doesn't allow flexibility
- **Options/Builder pattern**: Selected - allows customization while centralizing logic

**Findings - Duplicate Locations**:

| Helper | Locations | Usages |
|--------|-----------|--------|
| `testCommand()` | `pkg/invkfile/invkfile_test.go:15` | ~50 usages |
| `testCmd()` | `cmd/invowk/cmd_test.go:34` | ~30 usages |
| `setHomeDirEnv()` | `cmd/invowk/cmd_test.go:23`, `internal/config/config_test.go:16`, `internal/discovery/discovery_test.go:19` | ~30 usages total |

**Proposed Consolidation**:

```go
// testutil.NewTestCommand - replaces testCommand() and testCmd()
func NewTestCommand(name string, opts ...CommandOption) *invkfile.Command

// testutil.SetHomeDir - replaces setHomeDirEnv()
func SetHomeDir(t testing.TB, dir string) func()
```

---

### 3. Flaky Test Patterns

**Decision**: Implement Clock interface for deterministic time mocking.

**Rationale**: `time.Sleep()` in tests creates flaky behavior dependent on system load. Clock injection is the standard Go pattern for testing time-dependent code.

**Alternatives Considered**:
- **Increase sleep duration**: Rejected - masks the problem, slows tests
- **Use `time.After()` with select**: Rejected - still non-deterministic
- **Clock interface injection**: Selected - deterministic, standard pattern

**Findings - Affected Code**:

1. **`internal/sshserver/server_test.go:257`** - `TestExpiredToken`:
   ```go
   time.Sleep(10 * time.Millisecond)  // FLAKY
   ```

   **Fix approach**:
   - Add `Clock` interface to sshserver package
   - Inject `FakeClock` in tests
   - Advance clock deterministically

2. **Potential future issues** in timeout/rate-limiting code (none found currently)

**Clock Interface Design**:
```go
// Clock abstracts time operations for testing
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    Since(t time.Time) time.Duration
}

// RealClock uses actual time
type RealClock struct{}

// FakeClock allows manual time control
type FakeClock struct {
    current time.Time
    mu      sync.Mutex
}
```

---

### 4. TUI Component Testing Patterns

**Decision**: Test Bubble Tea model state transitions using message simulation.

**Rationale**: Terminal I/O cannot be mocked, but model state and text processing logic can be unit tested. Bubble Tea's `Update()` function accepts messages and returns new model state - this is fully testable.

**Alternatives Considered**:
- **Skip TUI testing**: Rejected - 4,250 lines of untested code is a significant risk
- **Full terminal emulation**: Rejected - complex, fragile, overkill
- **Model-only testing**: Selected - tests business logic without terminal dependencies

**Findings - Existing Pattern** (from `embeddable_test.go`):

```go
func TestCalculateModalSize(t *testing.T) {
    tests := []struct {
        name           string
        contentWidth   int
        contentHeight  int
        terminalWidth  int
        terminalHeight int
        expectedWidth  int
        expectedHeight int
    }{
        // table-driven tests...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test pure function
        })
    }
}
```

**Components to Test and Strategies**:

| Component | Lines | Testable Logic |
|-----------|-------|----------------|
| `choose.go` | 564 | Selection state, navigation, multi-select limit |
| `confirm.go` | 240 | Yes/No toggle, keyboard navigation |
| `input.go` | 274 | Text editing, validation, placeholder |
| `filter.go` | 566 | Search filtering, highlight matching |
| `table.go` | 425 | Row selection, sorting, column sizing |
| `format.go` | 270 | Text truncation, padding, alignment |
| `pager.go` | 260 | Page navigation, scroll position |
| `spin.go` | 402 | Animation frame cycling |
| `file.go` | 305 | Path navigation, selection |

---

### 5. Container Runtime Testing Patterns

**Decision**: Use mock `exec.Command` for deterministic unit tests; keep integration tests separate.

**Rationale**: Integration tests require actual Docker/Podman and skip in CI. Mock-based unit tests verify argument construction and error handling without containers.

**Alternatives Considered**:
- **Integration tests only**: Rejected - slow, require containers, can't run everywhere
- **Interface abstraction for engine**: Rejected - over-engineering for current needs
- **exec.Command mocking**: Selected - standard Go pattern, minimal changes

**Findings - Current Coverage**:

| File | Coverage Type |
|------|--------------|
| `engine_test.go` | Unit tests for types + gated integration tests |
| `container_integration_test.go` | Full integration (skipped when no engine) |

**Existing Gating Pattern**:
```go
if testing.Short() {
    t.Skip("skipping integration test in short mode")
}
```

**Mock exec.Command Pattern** (standard Go):
```go
var execCommand = exec.Command

func TestDockerBuild(t *testing.T) {
    // Save and restore
    oldExecCommand := execCommand
    defer func() { execCommand = oldExecCommand }()

    // Mock
    var capturedArgs []string
    execCommand = func(name string, args ...string) *exec.Cmd {
        capturedArgs = append([]string{name}, args...)
        return exec.Command("echo", "mock")
    }

    // Test
    engine.Build(ctx, opts)

    // Assert args
    if !contains(capturedArgs, "--no-cache") {
        t.Error("expected --no-cache flag")
    }
}
```

---

### 6. Test File Organization Best Practices

**Decision**: Follow naming convention `<package>_<concern>_test.go`.

**Rationale**: Clear naming allows developers to find relevant tests quickly. Concern-based organization mirrors how features are developed and modified.

**Research Sources**:
- Go testing best practices (stdlib patterns)
- Project constitution (`.claude/rules/testing.md`)
- Existing patterns in codebase

**Naming Convention**:
```
pkg/invkfile/
├── invkfile_test.go           # DELETE (replaced by split files)
├── invkfile_parsing_test.go   # Script parsing
├── invkfile_deps_test.go      # Dependencies
├── invkfile_flags_test.go     # Flags
├── invkfile_args_test.go      # Arguments
├── invkfile_platforms_test.go # Platforms
├── invkfile_env_test.go       # Environment
├── invkfile_workdir_test.go   # Workdir
└── invkfile_schema_test.go    # Schema
```

---

## Summary of Research Decisions

| Area | Decision | Pattern |
|------|----------|---------|
| Large files | Split by logical concern | `<pkg>_<concern>_test.go` |
| Duplicated helpers | Consolidate to testutil | Options/Builder pattern |
| Time-dependent tests | Clock injection | `Clock` interface + `FakeClock` |
| TUI testing | Model state testing | Bubble Tea `Update()` simulation |
| Container testing | exec.Command mocking | Package-level variable override |

All research questions have been resolved. Ready for Phase 1 design work.
