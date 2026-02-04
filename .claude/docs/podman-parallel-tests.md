# Podman Parallel Test Flakiness

## Issue Summary

Container CLI tests may exhibit sporadic failures when run in parallel on systems using rootless Podman. The failures manifest as:

```
Error: preparing container <id> for attach: crun: write to `/proc/sys/net/ipv4/ping_group_range`
(are all the IDs mapped in the user namespace?): Invalid argument: OCI runtime error
```

This is a **known Podman/crun issue**, not a bug in invowk code.

## Solution (Implemented)

The test infrastructure now handles this automatically:

1. **Sequential container tests**: `TestContainerCLI` in `tests/cli/cmd_container_test.go` runs all `container_*.txtar` tests sequentially (no `t.Parallel()`)
2. **Parallel non-container tests**: `TestCLI` in `tests/cli/cmd_test.go` runs all other tests in parallel for speed
3. **Smoke test retry**: The container availability check includes retry logic with exponential backoff to handle transient OCI errors

### Test Execution

```bash
# Run all tests - container tests sequential, others parallel
make test

# Run only container tests (sequential)
go test -v -run "TestContainerCLI" ./tests/cli/...

# Run only non-container tests (parallel)
go test -v -run "TestCLI$" ./tests/cli/...

# Skip container tests (short mode)
go test -v -short ./tests/cli/...
```

## Root Cause

When multiple rootless Podman containers start simultaneously, they may race to configure user namespace settings. The `ping_group_range` sysctl is particularly prone to this issue because:

1. Each container attempts to map the setting into its user namespace
2. Concurrent writes to `/proc/sys/net/ipv4/ping_group_range` can fail
3. The crun runtime surfaces this as an "Invalid argument" error

## Affected Environments

- **Fedora Silverblue/Kinoite** (uses `podman-remote` by default)
- **Rootless Podman** on any Linux distribution
- **CI environments** running parallel container tests

## Manual Workarounds (Legacy)

The following workarounds are **no longer needed** since the fix is implemented, but are documented for reference:

### 1. Run Container Tests Sequentially

Tests pass reliably when run one at a time:

```bash
# Run a single container test
go test -v -run "TestContainerCLI/container_provision" ./tests/cli/...

# Run all container tests sequentially (bash loop)
for test in container_basic container_provision container_args container_env; do
    go test -v -run "TestContainerCLI/$test" ./tests/cli/...
done
```

### 2. Retry Failed Tests

The issue is transient - re-running often succeeds:

```bash
# Run with retries using go-test-retry or similar
go test -v -count=1 -run "TestContainerCLI" ./tests/cli/... || \
go test -v -count=1 -run "TestContainerCLI" ./tests/cli/...
```

### 3. Reduce Parallelism

Limit the number of parallel tests:

```bash
go test -v -parallel 1 -run "TestContainerCLI" ./tests/cli/...
```

## Verification

To verify whether a failure is this known issue vs. an actual bug:

1. Check the error message contains `ping_group_range` and `OCI runtime error`
2. Re-run the specific failing test - if it passes, it was this issue
3. Run the test sequentially - if it passes consistently, it was this issue

## Related Issues

- Podman issue tracker: Search for "ping_group_range" and "parallel"
- crun issue tracker: User namespace race conditions

## Not Affected

- **Docker** (uses different namespace handling)
- **Rootful Podman** (doesn't use user namespaces)
- **Sequential test execution** (no race condition)
- **Individual test runs** (always pass)
