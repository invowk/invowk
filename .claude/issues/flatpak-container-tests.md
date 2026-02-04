# Flatpak Sandbox Container Test Failures

**Status: RESOLVED**

## Issue Summary

Container CLI integration tests fail when invowk is run inside a Flatpak sandbox (e.g., when using a Flatpak-packaged IDE or terminal). The tests pass when running invowk directly on the host.

## Root Cause

When running inside a Flatpak sandbox:

1. **Path translation mismatch**: The Flatpak sandbox has its own filesystem namespace. Paths like `/tmp` inside the sandbox are mapped to a different location on the host.

2. **Container engine runs on host**: Podman runs on the host system (accessed via `flatpak-spawn --host` or similar), but receives paths from within the sandbox.

3. **Volume mount confusion**: When invowk (inside Flatpak) tells Podman (on host) to mount `/tmp/testscript123:/workspace`, Podman mounts the host's `/tmp/testscript123`, not the Flatpak sandbox's `/tmp/testscript123`.

## Implemented Solution

We implemented the **flatpak-spawn --host command wrapping** approach using a decorator pattern.

### How It Works

1. **Sandbox Detection** (`pkg/platform/sandbox.go`):
   - `DetectSandbox()` identifies Flatpak (via `/.flatpak-info`) or Snap (via `SNAP_NAME` env var)
   - Results are cached via `sync.Once` for performance

2. **Engine Wrapper** (`internal/container/sandbox_engine.go`):
   - `SandboxAwareEngine` decorates any `Engine` implementation
   - When in a sandbox, commands are wrapped with `flatpak-spawn --host`
   - This executes the **entire command on the host** where paths resolve correctly

3. **Automatic Integration**:
   - `AutoDetectEngine()` and `NewEngine()` automatically wrap engines with sandbox awareness
   - No configuration required - detection is automatic

### Example Flow (Flatpak)

```
invowk cmd test
  → PodmanEngine.Run()
  → SandboxAwareEngine intercepts
  → exec.Command("flatpak-spawn", "--host", "podman", "run", "-v", "/tmp/test:/workspace", ...)
  → flatpak-spawn executes podman on HOST
  → HOST's /tmp/test is mounted
  → ✓ Works
```

## Files Changed

### New Files
- `pkg/platform/sandbox.go` - Sandbox detection utilities
- `pkg/platform/sandbox_test.go` - Tests for sandbox detection
- `internal/container/sandbox_engine.go` - Sandbox-aware engine wrapper
- `internal/container/sandbox_engine_test.go` - Tests for engine wrapper

### Modified Files
- `pkg/platform/doc.go` - Updated package documentation
- `internal/container/engine.go` - Wrapped engines with sandbox awareness
- `tests/cli/cmd_test.go` - Added `in-sandbox` testscript condition
- `tests/cli/testdata/container_*.txtar` - Added skip conditions for sandbox environments

## User Requirements

For the sandbox wrapping to work, users must grant filesystem permissions for directories they want to access:

```bash
# Flatpak
flatpak override --user --filesystem=/tmp com.example.App

# Or via flathub/manifest permissions
```

If permissions aren't granted, container commands will fail with permission errors (expected behavior).

## Test Behavior

- Tests automatically detect sandbox environment
- Container tests skip with a clear message when in sandbox
- The skip message guides users to either run tests on host or grant permissions

## Limitations

| Scenario | Handling |
|----------|----------|
| User hasn't granted filesystem permissions | Container fails with permission error (expected) |
| Interactive TTY mode | Should work - flatpak-spawn forwards I/O |
| Snap sandbox | Basic support via `snap run --shell` (may need refinement) |
| Nested sandboxes | Not supported (rare edge case) |
| flatpak-spawn not available | Falls back to direct execution |

## Related Information

- Flatpak sandbox documentation: https://docs.flatpak.org/en/latest/sandbox-permissions.html
- `flatpak-spawn` for host access: https://docs.flatpak.org/en/latest/flatpak-command-reference.html#flatpak-spawn
