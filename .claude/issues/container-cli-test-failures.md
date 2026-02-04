# Container CLI Test Flakiness

**Status: OPEN**
**Severity: Medium**
**Affects: CI reliability, developer experience**

## Issue Summary

Container CLI integration tests (`tests/cli/testdata/container_*.txtar`) exhibit flaky behavior - they pass/fail inconsistently across runs. The failures are **unrelated** to the Flatpak sandbox implementation work and were discovered as pre-existing issues.

## Observed Behavior

### Symptoms

When running container tests, results vary between runs:

```bash
go test -v -run "TestCLI/container_" ./tests/cli/... -count=1
```

| Run 1 | Run 2 | Run 3 |
|-------|-------|-------|
| container_basic ✓ | container_basic ✗ | container_basic ✓ |
| container_args ✓ | container_args ✓ | container_args ✗ |
| container_provision ✗ | container_provision ✗ | container_provision ✗ |
| container_env ✓ | container_env ✗ | container_env ✓ |
| container_exitcode ✓ | container_exitcode ✗ | container_exitcode ✓ |
| container_callback ✓ | container_callback ✗ | container_callback ✓ |
| container_dockerfile ✓ | container_dockerfile ✓ | container_dockerfile ✓ |

**Note:** `container_provision` fails consistently, while others are intermittent.

### Error Messages

#### Permission Denied (most common)

```
grep: /workspace/invkfile.cue: Permission denied
cat: /workspace/custom.txt: Permission denied
sh: 0: cannot open /workspace/scripts/helper.sh: Permission denied
```

This occurs despite the `--userns=keep-id` fix being applied (commit `d75e4ee`).

#### Image Tagging Noise

The stderr output shows excessive tagging of cached images:

```
Successfully tagged localhost/invowk-provisioned:78bb71527060
Successfully tagged localhost/invowk-provisioned:d67cc6a9be95
Successfully tagged localhost/invowk-provisioned:c84ef8532de7
... (30+ lines)
```

This suggests parallel tests are sharing/competing for the same image cache.

## Environment Details

- **Host OS:** Fedora Silverblue (immutable, rpm-ostree based)
- **Container Engine:** Podman (rootless)
- **Podman Mode:** `rootless: true` with `runRoot: /run/user/1000/containers`
- **User:** uid=1000, gid=1000
- **Go Version:** 1.25+
- **Test Framework:** testscript

## Root Cause Hypotheses

### 1. Parallel Test Execution Conflicts (High Confidence)

Tests run in parallel (`t.Parallel()` via testscript) and compete for:
- Container image cache (same base image `debian:stable-slim`)
- Provisioned image tags (`invowk-provisioned:*`)
- Volume mount paths

**Evidence:** Tests pass consistently when run individually but fail when run in parallel.

### 2. User Namespace / Volume Mount Timing (Medium Confidence)

Even with `--userns=keep-id`, rootless Podman may have race conditions when:
- Multiple containers mount volumes simultaneously
- Container starts before volume permissions are properly propagated
- SELinux relabeling (`:Z` flag) conflicts between parallel mounts

### 3. Container Image Cache Corruption (Low Confidence)

Stale or corrupted container image layers may cause inconsistent behavior:
- Old provisioned images with different permission settings
- Partial image builds from interrupted tests

## Investigation Steps

### Verify Parallel Execution Issue

Run tests sequentially to confirm:

```bash
# Sequential execution (should be more reliable)
go test -v -run "TestCLI/container_" ./tests/cli/... -count=1 -p=1

# Or run individual tests
go test -v -run "TestCLI/container_basic$" ./tests/cli/... -count=1
go test -v -run "TestCLI/container_provision$" ./tests/cli/... -count=1
```

### Clean Container Cache

Remove stale provisioned images:

```bash
podman images --filter "reference=invowk-provisioned" --format "{{.ID}}" | xargs -r podman rmi -f
podman system prune -f
```

### Check SELinux Context

```bash
# Verify SELinux isn't blocking volume mounts
ausearch -m avc -ts recent | grep -i container
getenforce
```

### Test Volume Permissions Directly

```bash
# Create a test volume mount
mkdir -p /tmp/test-vol
echo "test" > /tmp/test-vol/test.txt
podman run --rm --userns=keep-id -v /tmp/test-vol:/workspace:Z debian:stable-slim cat /workspace/test.txt
```

## Potential Fixes

### Option A: Serialize Container Tests

Add test isolation by running container tests sequentially:

```go
// In tests/cli/cmd_test.go
func TestCLI(t *testing.T) {
    // Don't parallelize container tests
    if strings.HasPrefix(testName, "container_") {
        // Don't call t.Parallel()
    }
}
```

### Option B: Unique Container Names per Test

Ensure each test uses unique image tags and container names to avoid conflicts.

### Option C: Add Retry Logic

Implement test retry for flaky container tests (last resort).

### Option D: Investigate Permission Propagation Timing

Add a small delay or synchronization after volume mount setup.

## Related Files

- `tests/cli/cmd_test.go` - Test runner configuration
- `tests/cli/testdata/container_*.txtar` - Test definitions
- `internal/container/podman.go` - Podman engine with `--userns=keep-id`
- `internal/runtime/container_provision.go` - Container provisioning logic

## Workarounds

Until fixed, developers can:

1. **Run tests individually** when debugging container issues
2. **Clean container cache** before test runs: `podman system prune -f`
3. **Run tests multiple times** to verify behavior (flaky tests may pass on retry)

## Notes

- The `--userns=keep-id` fix (commit `d75e4ee`) improved reliability but didn't eliminate flakiness
- This issue is specific to local development; CI may have different behavior due to fresh container state
- The Flatpak sandbox tests (separate issue) are correctly skipped when in sandbox environments
