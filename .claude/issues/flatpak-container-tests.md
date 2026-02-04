# Flatpak Sandbox Container Test Failures

## Issue Summary

Container CLI integration tests fail when invowk is run inside a Flatpak sandbox (e.g., when using a Flatpak-packaged IDE or terminal). The tests pass when running invowk directly on the host.

## Symptoms

```
grep: /workspace/invkfile.cue: Permission denied
cat: /workspace/custom.txt: Permission denied
sh: 0: cannot open /workspace/scripts/helper.sh: Permission denied
```

Despite `--userns=keep-id` correctly preserving the user's UID (verified via `id` command showing `uid=1000(danilo)`), files in `/workspace` remain inaccessible.

## Root Cause Analysis

### The Problem

When running inside a Flatpak sandbox:

1. **Path translation mismatch**: The Flatpak sandbox has its own filesystem namespace. Paths like `/tmp` inside the sandbox are mapped to a different location on the host (e.g., `~/.var/app/<app-id>/...`).

2. **Container engine runs on host**: Podman runs on the host system (accessed via `flatpak-spawn --host` or similar), but receives paths from within the sandbox.

3. **Volume mount confusion**: When invowk (inside Flatpak) tells Podman (on host) to mount `/tmp/testscript123:/workspace`, Podman mounts the host's `/tmp/testscript123`, not the Flatpak sandbox's `/tmp/testscript123`.

### Evidence

```bash
# Inside Flatpak sandbox
$ stat /tmp
File: /tmp
Uid: (65534/  nobody)   Gid: (65534/  nobody)  # Flatpak virtual /tmp

# Direct host access works
$ flatpak-spawn --host podman run --rm --userns=keep-id \
    -v "/var/home/danilo/real-path:/workspace:z" \
    debian:stable-slim ls -la /workspace/
# SUCCESS - files are readable

# But invowk inside Flatpak fails
$ ./bin/invowk cmd test-container
# FAILURE - Permission denied on /workspace files
```

### Why It Worked Before

Before the `--userns=keep-id` fix, the `containerAvailable` smoke test would fail (due to rootless Podman permission issues), causing all container tests to be **SKIPPED**. The Flatpak issue was hidden because the tests never ran.

After the fix, the smoke test passes, so container tests now **RUN** and expose this separate issue.

## Affected Tests

- `tests/cli/testdata/container_provision.txtar`
- `tests/cli/testdata/container_exitcode.txtar`
- `tests/cli/testdata/container_args.txtar`
- `tests/cli/testdata/container_env.txtar`
- `tests/cli/testdata/container_callback.txtar`
- `tests/cli/testdata/container_basic.txtar`
- `tests/cli/testdata/container_dockerfile.txtar`

## Potential Solutions

### Option 1: Detect Flatpak and Skip Tests

Add a condition to skip container tests when running inside a Flatpak sandbox:

```go
// In tests/cli/cmd_test.go
func isInsideFlatpak() bool {
    // Check for Flatpak-specific environment variables or paths
    _, err := os.Stat("/.flatpak-info")
    return err == nil
}

// In containerAvailable check
if isInsideFlatpak() {
    return false // Skip container tests in Flatpak
}
```

### Option 2: Use Host Paths via flatpak-spawn

Translate paths before passing to the container engine:

```go
// Resolve Flatpak sandbox paths to host paths
func resolveHostPath(sandboxPath string) (string, error) {
    // Use flatpak-spawn or XDG portals to get real host path
    // This is complex and may not be reliable
}
```

### Option 3: Run Tests Outside Flatpak

Document that container tests should be run outside the Flatpak sandbox:

```bash
# Run tests on host, not inside Flatpak IDE
flatpak-spawn --host make test
```

### Option 4: Use Toolbox/Distrobox

For development on immutable distros (Silverblue, Kinoite), use toolbox or distrobox which provide a more transparent container environment:

```bash
toolbox run make test
```

## Recommended Approach

1. **Short-term**: Add Flatpak detection to skip container tests with a clear message
2. **Documentation**: Add a note in CONTRIBUTING.md about running tests outside Flatpak
3. **Long-term**: Investigate if XDG Document Portal or similar can provide reliable path translation

## Related Information

- Flatpak sandbox documentation: https://docs.flatpak.org/en/latest/sandbox-permissions.html
- `flatpak-spawn` for host access: https://docs.flatpak.org/en/latest/flatpak-command-reference.html#flatpak-spawn
- Podman in Flatpak: https://github.com/containers/podman/issues/related-to-flatpak

## Test Environment Details

- Host OS: Fedora Silverblue/Kinoite (immutable)
- Flatpak app: IDE or terminal running invowk
- Container engine: Podman (rootless, on host)
- Issue discovered: 2026-02-04 while implementing `--userns=keep-id` fix
