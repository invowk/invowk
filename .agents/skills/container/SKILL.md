---
name: container
description: Container engine abstraction, Docker/Podman patterns, path handling, Linux-only policy
disable-model-invocation: false
---

# Container Engine Skill

This skill covers the container runtime implementation in Invowk, including the engine abstraction layer, Docker/Podman support, and sandbox-aware execution.

Use this skill when working on:
- `internal/container/` - Container engine abstraction
- `internal/runtime/container.go` - Container runtime implementation
- `internal/provision/` - Container provisioning logic
- Container-related tests

---

## Linux-Only Container Support

**CRITICAL: The container runtime ONLY supports Linux containers.**

| Supported | NOT Supported |
|-----------|---------------|
| Debian-based images (`debian:stable-slim`) | Alpine-based images (`alpine:*`) |
| Standard Linux containers | Windows container images |

**Why no Alpine:** musl-based environments have many subtle gotchas; we prioritize reliability over image size.

**Why no Windows containers:** They're rarely used and would introduce too much extra complexity to Invowk's auto-provisioning logic.

**In tests, docs, and examples:** Always use `debian:stable-slim` as the reference image.

### Image Validation (Early Rejection)

`validateSupportedContainerImage()` (`container_provision.go`) enforces the image policy **before** provisioning to fail fast:

```go
validateSupportedContainerImage(image) error
  ├── isWindowsContainerImage(image) — pattern matching (mcr.microsoft.com/windows/*, etc.)
  └── isAlpineContainerImage(image)  — segment-aware matching (last path segment only)
```

**Segment-aware Alpine detection:** `isAlpineContainerImage()` strips tag/digest suffixes, then checks if the bare name equals `"alpine"` or has `/alpine` as the last path segment. This avoids false positives on images like `"go-alpine-builder:v1"` or `"myorg/alpine-tools"`. Matches: `alpine`, `alpine:3.20`, `docker.io/library/alpine:latest`.

---

## Engine Interface

The `Engine` interface (`engine.go`) defines the unified contract for all container operations:

```go
type Engine interface {
    // Core operations
    Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
    Run(ctx context.Context, opts RunOptions) (*RunResult, error)
    Remove(ctx context.Context, containerID string) error
    ImageExists(ctx context.Context, image string) (bool, error)
    RemoveImage(ctx context.Context, image string) error

    // Metadata
    Name() string
    Version(ctx context.Context) (string, error)
    Available() bool

    // Interactive mode support
    BuildRunArgs(opts RunOptions) []string
    BinaryPath() string
}
```

**Key Pattern:** The interface doesn't expose vendor-specific methods. Methods like `Exec()` and `InspectImage()` exist only on concrete types.

---

## BaseCLIEngine Embedding Pattern

Both Docker and Podman engines embed `BaseCLIEngine` (`engine_base.go`) for shared CLI command construction:

```go
type BaseCLIEngine struct {
    binaryPath         string
    execCommand        ExecCommandFunc       // For mocking in tests
    volumeFormatter    VolumeFormatFunc      // SELinux label injection
    runArgsTransformer RunArgsTransformer    // Podman --userns=keep-id
}
```

### Responsibilities

| Method | Purpose |
|--------|---------|
| `BuildArgs()`, `RunArgs()` | Construct CLI arguments |
| `RunCommand()`, `RunCommandCombined()` | Execute commands |
| `FormatVolumeMount()`, `ParseVolumeMount()` | Volume mount handling |
| `ResolveDockerfilePath()` | Path resolution with traversal protection |

### Functional Options

```go
// For testing - inject mock command executor
eng := NewDockerEngine(WithExecCommand(mockExec))

// For Podman - SELinux label injection
eng := NewPodmanEngine(WithVolumeFormatter(selinuxFormatter))

// For Podman - rootless mode
eng := NewPodmanEngine(WithRunArgsTransformer(usernsKeepID))
```

---

## Docker vs Podman Implementation

### Docker (`docker.go`)

Minimal implementation—mostly delegates to `BaseCLIEngine`:

```go
type DockerEngine struct {
    *BaseCLIEngine
}

func NewDockerEngine(opts ...Option) (*DockerEngine, error) {
    path, err := exec.LookPath("docker")
    if err != nil {
        return nil, err
    }
    return &DockerEngine{BaseCLIEngine: newBase(path, opts...)}, nil
}
```

### Podman (`podman.go`)

More complex due to Linux-specific features:

**Binary Discovery:**
```go
// Tries podman first, then podman-remote (for immutable distros like Silverblue)
path, err := exec.LookPath("podman")
if err != nil {
    path, err = exec.LookPath("podman-remote")
}
```

**Automatic Enhancements:**

1. **SELinux Volume Labels**: Automatically adds `:z` labels to volumes on SELinux systems
   ```go
   // Checks /sys/fs/selinux existence (more reliable than checking enforce status)
   func isSELinuxPresent() bool {
       _, err := os.Stat("/sys/fs/selinux")
       return err == nil
   }
   ```

2. **Rootless Compatibility**: Injects `--userns=keep-id` to preserve host UID/GID
   ```go
   // Only transforms 'run' commands, inserted before image name
   func makeUsernsKeepIDAdder() RunArgsTransformer { ... }
   ```

---

## Sysctl Override (ping_group_range Prevention)

Rootless Podman's `default_sysctls` configuration causes `crun` to write `net.ipv4.ping_group_range=0 0` in each new network namespace. When multiple containers start concurrently, these writes race and produce `EINVAL` (exit code 126).

### Prevention Layer

On **Linux with local Podman**, `NewPodmanEngine()` calls `sysctlOverrideOpts(binaryPath)` which:
1. Checks if the binary is `podman-remote` (via name + symlink resolution) — skips if remote
2. Creates a temp file via `createSysctlOverrideTempFile()` containing `[containers]\ndefault_sysctls = []\n`
3. Returns `WithCmdEnvOverride("CONTAINERS_CONF_OVERRIDE", tempPath)` + `WithSysctlOverridePath(tempPath)` + `WithSysctlOverrideActive(true)`
4. Every Podman subprocess opens the path independently and reads the override config
5. The temp file is cleaned up by `BaseCLIEngine.Close()` when the engine is released

On **non-Linux** platforms, `sysctlOverrideOpts()` (`podman_sysctl_other.go`) returns nil — Podman runs inside a VM where host-side env vars don't reach crun. Instead, `runWithRetry()` falls back to `containerRunMu` (in-process mutex) since flock can't reach the VM.

On **podman-remote** (Fedora Silverblue/toolbox), `sysctlOverrideOpts()` returns nil — the env var only affects the remote client, not the Podman service that calls crun. Detected via `isRemotePodman()` which checks binary name + follows symlinks. On Linux, `runWithRetry()` uses **flock** (`acquireRunLock()`) for cross-process serialization instead.

### Cross-Process Serialization (flock)

When the sysctl override is not active, `runWithRetry()` serializes container runs to prevent the ping_group_range race. On Linux, `acquireRunLock()` (`run_lock_linux.go`) acquires a blocking `flock(2)` on `$XDG_RUNTIME_DIR/invowk-podman.lock` (fallback: `os.TempDir()`). This provides **cross-process** serialization — all invowk processes on the same machine share the flock. On non-Linux, `acquireRunLock()` returns an error, causing fallback to `sync.Mutex` for intra-process protection only.

### Stderr Buffering

`runWithRetry()` buffers stderr per-attempt so that transient error messages from crun (written directly to the inherited stderr fd before Go can decide to retry) never leak to the user's terminal. On success, non-transient failure, or retry exhaustion, the final attempt's buffer is flushed to the caller's original writer. On transient failure with retries remaining, the buffer is discarded and retried. Interactive mode (`PrepareCommand`) is unaffected — it uses a PTY and bypasses `runWithRetry()`.

### SysctlOverrideChecker Interface

The `SysctlOverrideChecker` interface (`engine_base.go`) lets the runtime layer query whether the temp file override is active:

```go
type SysctlOverrideChecker interface {
    SysctlOverrideActive() bool
}
```

**Implemented by:** `PodmanEngine`, `SandboxAwareEngine` (forwards to wrapped engine)

**Used in:** `runWithRetry()` — when the checker returns false, acquires flock (Linux) or mutex (non-Linux); when not implemented (Docker), skips serialization entirely

### CmdCustomizer Interface

The `CmdCustomizer` interface (`engine_base.go`) propagates overrides through engines that create `exec.Cmd` outside `CreateCommand()`:

```go
type CmdCustomizer interface {
    CustomizeCmd(cmd *exec.Cmd)
}
```

**Implemented by:** `BaseCLIEngine`, `SandboxAwareEngine`

**Used in:**
- `SandboxAwareEngine.Build/Run/Remove/ImageExists/RemoveImage` — sandbox commands bypass `CreateCommand`
- `ContainerRuntime.PrepareCommand()` — interactive mode creates its own `exec.Cmd` for PTY attachment

### Key Files

| File | Purpose |
|------|---------|
| `podman_sysctl_linux.go` | `createSysctlOverrideTempFile()`, `isRemotePodman()`, `sysctlOverrideOpts()` (Linux temp file) |
| `podman_sysctl_other.go` | No-op `sysctlOverrideOpts()` (macOS/Windows stub) |
| `engine_base.go` | `CmdCustomizer`, `SysctlOverrideChecker`, `EngineCloser`, `WithCmdEnvOverride()`, `WithSysctlOverridePath()`, `WithSysctlOverrideActive()`, `Close()` |
| `podman.go` | `SysctlOverrideActive()`, `Close()` methods on `PodmanEngine` |
| `internal/runtime/container_exec.go` | `containerRunMu` (fallback mutex), `runWithRetry()` (flock + stderr buffering), `flushStderr()` |
| `internal/runtime/container_prepare.go` | `CmdCustomizer` type assertion in `PrepareCommand()` |

---

## Path Handling (Host vs Container)

**CRITICAL:** Container paths always use forward slashes (`/`), regardless of host platform.

### Two Path Domains

| Domain | Separator | Example |
|--------|-----------|---------|
| Host paths | Platform-native (`\` on Windows) | `C:\app\config.json` |
| Container paths | Always `/` | `/workspace/script.sh` |

### Conversion Pattern

```go
// Converting host path to container path
containerPath := "/workspace/" + filepath.ToSlash(relPath)

// WRONG: filepath.Join uses backslashes on Windows
containerPath := filepath.Join("/workspace", relPath)  // Broken on Windows!
```

### Path Security

`ResolveDockerfilePath()` includes path traversal detection to prevent `../..` escapes.

See `.agents/rules/windows.md` for comprehensive path handling guidance.

---

## SandboxAwareEngine Wrapper

The `SandboxAwareEngine` (`sandbox_engine.go`) is a decorator for Flatpak/Snap execution:

**Problem:** Container engines run on the host, not inside the sandbox. Paths don't match.

**Solution:** Execute commands via `flatpak-spawn --host` or `snap run --shell`.

```go
type SandboxAwareEngine struct {
    wrapped     Engine
    sandboxType platform.SandboxType
}

// Factory function wraps engine if sandbox detected
func NewEngine(preferredType EngineType) (Engine, error) {
    engine := createEngine(preferredType)
    return NewSandboxAwareEngine(engine), nil  // Auto-wraps if needed
}
```

---

## Engine Factory Functions

### Preference with Fallback

```go
// Tries preferred engine first, falls back to alternative
engine, err := container.NewEngine(container.Podman)  // or container.Docker
```

### Auto-Detection

```go
// Tries Podman first (better for rootless), then Docker
engine, err := container.AutoDetectEngine()
```

Both return wrapped `SandboxAwareEngine`.

---

## Exit Code Handling

Container engines absorb `exec.ExitError` into `result.ExitCode` and return `(result, nil)`. This means **the error return is always nil** for process exit failures — callers must check `result.ExitCode`:

```go
result := &RunResult{}
if err != nil {
    if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
        result.ExitCode = exitErr.ExitCode()  // Absorbed into result, err return is nil
    } else {
        result.ExitCode = 1
        result.Error = err  // Actual error (network, etc.)
    }
}
return result, nil  // Always nil error for exit code failures
```

**Important for retry logic:** Since `engine.Run()` returns `(result, nil)` for transient exit codes (125, 126), retry code must check both the error return AND `result.ExitCode`. See `runWithRetry()` in `container_exec.go`.

---

## Testing Patterns

### Unit Tests with Per-Test Mock Recorders

All container unit tests use per-test `MockCommandRecorder` instances for parallel safety:

```go
func TestDockerBuild(t *testing.T) {
    t.Parallel()

    t.Run("with no-cache", func(t *testing.T) {
        t.Parallel()
        recorder := NewMockCommandRecorder()
        eng := newTestDockerEngine(t, recorder)  // Injects via WithExecCommand()

        eng.Build(ctx, opts)

        // Verify expected arguments
        if !slices.Contains(recorder.LastArgs, "--no-cache") {
            t.Error("expected --no-cache flag")
        }
    })
}
```

**Never use package-level global mutation** (`execCommand = mockFn`) for mock injection. The `execCommand` var is test-scoped in `engine_mock_test.go` and only used by 3 mock infrastructure self-tests.

### Integration Tests

```go
func TestDockerBuild_Integration(t *testing.T) {
    t.Parallel()
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // Test with real container engine — transient errors handled by runWithRetry()
}
```

### testscript HOME Fix

Container tests using testscript need `HOME` set to a writable directory:

```go
Setup: func(env *testscript.Env) error {
    // Docker/Podman CLI requires valid HOME for config storage
    env.Setenv("HOME", env.WorkDir)
    return nil
},
```

### Container Test Timeout Strategy

Multi-layer timeout strategy prevents indefinite hangs:

1. **Per-test deadline** (3 minutes): `testscript.Params{Deadline: deadline}`
2. **Cleanup via `env.Defer()`**: Removes orphaned containers
3. **CI explicit timeout** (15 minutes): Safety net for catastrophic failures

---

## Transient Error Classification

The `IsTransientError()` function (`transient.go`) is a shared classifier for transient container engine errors that may succeed on retry. It is used by both production retry logic (`ensureImage()` in `internal/runtime/container_provision.go`) and the test smoke test (`tests/cli/cmd_test.go`).

**Classified as transient:**
- Exit code 125 (generic engine error — storage/cgroup issues)
- `ping_group_range` (rootless Podman user namespace race)
- `OCI runtime error` (generic OCI failures)
- Network errors: `Temporary failure resolving`, `Could not resolve host`, `connection timed out`, `connection refused`
- Storage errors: `error creating overlay mount`, `error mounting layer`

**Explicitly NOT transient:**
- `nil` errors
- `context.Canceled` / `context.DeadlineExceeded` (retrying cancelled operations is never useful)

### Build Retry in ensureImage()

Container image builds (`engine.Build()`) are retried up to 3 times with exponential backoff (2s, 4s) on transient errors. Non-transient errors fail immediately. The caller's context deadline naturally bounds total retry time.

### Run Retry in runWithRetry()

Container runs (`engine.Run()`) are retried up to 5 times with exponential backoff (1s, 2s, 4s, 8s) on transient errors. This is critical because `engine.Run()` absorbs `exec.ExitError` into `result.ExitCode` and always returns `(result, nil)` — so the retry logic must check **both** the error return AND `result.ExitCode` via `isTransientExitCode()` (exit codes 125 and 126). Run retries are more aggressive than build retries (5 vs 3 attempts) because Podman `ping_group_range` races are more frequent under heavy parallelism and runs are fast.

---

## File Organization

| File | Purpose |
|------|---------|
| `engine.go` | Interface, factories, engine types |
| `engine_base.go` | Shared CLI implementation, `CmdCustomizer` interface |
| `docker.go` | Docker concrete implementation |
| `podman.go` | Podman + SELinux/rootless logic |
| `podman_sysctl_linux.go` | Temp file-based sysctl override (Linux only) |
| `podman_sysctl_other.go` | No-op sysctl override stub (non-Linux) |
| `sandbox_engine.go` | Flatpak/Snap wrapper decorator |
| `transient.go` | Shared transient error classifier |
| `doc.go` | Package documentation |

**Runtime files** (in `internal/runtime/`):

| File | Purpose |
|------|---------|
| `container_exec.go` | Container execution, `runWithRetry()`, `isTransientExitCode()`, `flushStderr()` |
| `container_provision.go` | Image building, `ensureImage()` retry, retry constants, **image validation** (`validateSupportedContainerImage`, `isAlpineContainerImage`, `isWindowsContainerImage`) |
| `container_prepare.go` | `CmdCustomizer` type assertion in `PrepareCommand()` |
| `run_lock_linux.go` | flock-based cross-process lock (`acquireRunLock()`, `runLock`) |
| `run_lock_other.go` | No-op stub, forces fallback to `sync.Mutex` |
| `container_exec_test.go` | Unit tests for `runWithRetry()`: serialization decision, stderr buffering, exit codes, context cancellation |
| `container_test.go` | Unit tests for `isAlpineContainerImage()`, `isWindowsContainerImage()`, `validateSupportedContainerImage()` |

---

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Using `filepath.Join()` for container paths | Backslashes on Windows | Use string concat with `/` or `filepath.ToSlash()` |
| Forgetting `HOME` in testscript | "mkdir /no-home: permission denied" | Set `HOME` to `env.WorkDir` in Setup |
| Testing with Alpine images | Unexpected musl behavior | Always use `debian:stable-slim` |
| Missing SELinux labels | Permission denied in Podman | Use Podman's auto-labeling or explicit `:z` |
| Container tests hanging | CI timeout | Use per-test deadline + cleanup in `env.Defer()` |
| Flaky container builds in CI | Exit code 125, DNS failures | `IsTransientError()` + build retry in `ensureImage()` handles this; CI pre-pulls `debian:stable-slim` |
| Flaky container runs under parallelism | Exit code 125/126, ping_group_range | `runWithRetry()` in `container_exec.go` retries runs with exponential backoff; checks both `err` and `result.ExitCode` |
| Mock tests share recorder across parallel subtests | Race condition on recorder state | Use per-subtest `NewMockCommandRecorder()` + engine instances; never share a recorder with `Reset()` across parallel subtests |
