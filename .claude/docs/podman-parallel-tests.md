# Podman Parallel Test Flakiness

## Issue Summary

Container CLI tests may exhibit sporadic failures when run in parallel on systems using rootless Podman. The failures manifest as:

```
Error: preparing container <id> for attach: crun: write to `/proc/sys/net/ipv4/ping_group_range`
(are all the IDs mapped in the user namespace?): Invalid argument: OCI runtime error
```

This is a **known Podman/crun issue**, not a bug in invowk code.

## Root Cause

When multiple rootless Podman containers start simultaneously, they may race to configure user namespace settings. The `ping_group_range` sysctl is particularly prone to this issue because:

1. Each container attempts to map the setting into its user namespace
2. Concurrent writes to `/proc/sys/net/ipv4/ping_group_range` can fail
3. The crun runtime surfaces this as an "Invalid argument" error

## Affected Environments

- **Fedora Silverblue/Kinoite** (uses `podman-remote` by default)
- **Rootless Podman** on any Linux distribution
- **CI environments** running parallel container tests

## Workarounds

### 1. Run Container Tests Sequentially

Tests pass reliably when run one at a time:

```bash
# Run a single container test
go test -v -run "TestCLI/container_provision" ./tests/cli/...

# Run all container tests sequentially (bash loop)
for test in container_basic container_provision container_args container_env; do
    go test -v -run "TestCLI/$test" ./tests/cli/...
done
```

### 2. Retry Failed Tests

The issue is transient - re-running often succeeds:

```bash
# Run with retries using go-test-retry or similar
go test -v -count=1 -run "TestCLI/container" ./tests/cli/... || \
go test -v -count=1 -run "TestCLI/container" ./tests/cli/...
```

### 3. Reduce Parallelism

Limit the number of parallel tests:

```bash
go test -v -parallel 1 -run "TestCLI/container" ./tests/cli/...
```

## CI Recommendations

For CI pipelines on affected systems:

1. **Accept occasional flakiness** - The tests are valid; failures are environment-specific
2. **Configure retries** - Most CI systems support automatic retry on failure
3. **Run container tests in a dedicated job** - Isolate from other parallel test runs

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
