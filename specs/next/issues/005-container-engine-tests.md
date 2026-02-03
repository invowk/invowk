# Issue: Add Unit Tests for Container Engine Package

**Category**: Testability
**Priority**: Medium
**Effort**: Medium (2-3 days)
**Labels**: `testing`, `container`

## Summary

The `internal/container/` package has 3 implementation files without corresponding test files. This package provides the abstraction layer for Docker and Podman, and testing is essential for reliable cross-engine behavior.

## Problem

**Files without tests**:

| File | Lines | Description |
|------|-------|-------------|
| `docker.go` | ~150 | Docker CLI wrapper |
| `podman.go` | ~150 | Podman CLI wrapper |
| `engine_base.go` | ~100 | Base CLI engine functionality |

**Note**: `engine_mock_test.go` exists but is used by other packages' tests, not for testing the container package itself.

## Solution

Create comprehensive unit tests that verify command construction without actually invoking Docker/Podman.

### Test Strategy

1. **Mock exec.Command** - Intercept command execution to verify arguments
2. **Test command construction** - Verify correct flags and arguments
3. **Test error handling** - Verify graceful handling of failures

### Example Test Pattern

```go
// internal/container/docker_test.go

func TestDockerEngine_BuildRunArgs(t *testing.T) {
    engine := &DockerEngine{
        execPath: "/usr/bin/docker",
    }

    tests := []struct {
        name     string
        opts     RunOptions
        wantArgs []string
    }{
        {
            name: "basic run",
            opts: RunOptions{
                Image:   "debian:stable-slim",
                Command: []string{"echo", "hello"},
            },
            wantArgs: []string{
                "run", "--rm",
                "debian:stable-slim",
                "echo", "hello",
            },
        },
        {
            name: "with volume mount",
            opts: RunOptions{
                Image:   "debian:stable-slim",
                Volumes: []VolumeMount{{Source: "/host", Target: "/container"}},
                Command: []string{"ls"},
            },
            wantArgs: []string{
                "run", "--rm",
                "-v", "/host:/container",
                "debian:stable-slim",
                "ls",
            },
        },
        {
            name: "with environment variables",
            opts: RunOptions{
                Image: "debian:stable-slim",
                Env:   map[string]string{"FOO": "bar", "BAZ": "qux"},
            },
            wantArgs: []string{
                "run", "--rm",
                "-e", "BAZ=qux", "-e", "FOO=bar", // Sorted for determinism
                "debian:stable-slim",
            },
        },
        {
            name: "interactive with TTY",
            opts: RunOptions{
                Image:       "debian:stable-slim",
                Interactive: true,
                TTY:         true,
            },
            wantArgs: []string{
                "run", "--rm", "-i", "-t",
                "debian:stable-slim",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := engine.BuildRunArgs(tt.opts)
            // Compare arguments (may need to handle ordering for env vars)
            if !reflect.DeepEqual(got, tt.wantArgs) {
                t.Errorf("BuildRunArgs() = %v, want %v", got, tt.wantArgs)
            }
        })
    }
}
```

### Docker-Specific Tests (`docker_test.go`)

**Test cases**:
- [ ] `BuildRunArgs()` with various options
- [ ] `BuildBuildArgs()` for Dockerfile builds
- [ ] `BuildPullArgs()` for image pulling
- [ ] `BuildRemoveArgs()` for container/image removal
- [ ] Volume mount formatting (`-v source:target:opts`)
- [ ] Environment variable formatting (`-e KEY=value`)
- [ ] Port mapping formatting (`-p host:container`)
- [ ] Network configuration (`--network`)

### Podman-Specific Tests (`podman_test.go`)

**Test cases**:
- [ ] Same cases as Docker (API compatibility)
- [ ] Rootless mode detection and handling
- [ ] Cgroup v2 configuration differences
- [ ] User namespace mapping (`--userns`)
- [ ] Security options (`--security-opt`)

### Engine Detection Tests (`engine_test.go`)

```go
func TestNewEngine(t *testing.T) {
    tests := []struct {
        name       string
        preference ContainerEngine
        dockerPath string
        podmanPath string
        wantType   string
        wantErr    bool
    }{
        {
            name:       "prefer docker when available",
            preference: ContainerEngineDocker,
            dockerPath: "/usr/bin/docker",
            wantType:   "*DockerEngine",
        },
        {
            name:       "fallback to podman",
            preference: ContainerEngineDocker,
            dockerPath: "",
            podmanPath: "/usr/bin/podman",
            wantType:   "*PodmanEngine",
        },
        {
            name:       "error when none available",
            preference: ContainerEngineDocker,
            dockerPath: "",
            podmanPath: "",
            wantErr:    true,
        },
    }
    // Use exec.LookPath mocking or environment manipulation
}
```

**Test cases for engine.go**:
- [ ] `NewEngine()` returns correct type based on preference
- [ ] `NewEngine()` falls back when preferred not available
- [ ] `NewEngine()` errors when no engine available
- [ ] `Available()` correctly detects engine presence
- [ ] Engine path detection via PATH

### Base Engine Tests (`engine_base_test.go`)

**Test cases**:
- [ ] Command execution with mock
- [ ] Output capture
- [ ] Exit code extraction
- [ ] Timeout handling
- [ ] Context cancellation

## Implementation Steps

1. [ ] Create `engine_test.go` with detection logic tests
2. [ ] Create `docker_test.go` with command construction tests
3. [ ] Create `podman_test.go` with Podman-specific tests
4. [ ] Create `engine_base_test.go` with execution tests
5. [ ] Add mock exec helper for command interception
6. [ ] Verify cross-platform behavior (especially Windows)

## Acceptance Criteria

- [ ] Engine detection tested
- [ ] Docker command construction tested for all operations
- [ ] Podman command construction tested for all operations
- [ ] Error handling tested
- [ ] Test coverage for container package > 60%
- [ ] All tests pass without requiring Docker/Podman installed
- [ ] `make test` passes

## Testing

```bash
# Run container package tests
go test -v ./internal/container/...

# Check coverage
go test -cover ./internal/container/...
```

## Notes

- Tests should NOT require Docker/Podman to be installed
- Use command argument inspection rather than actual execution
- Consider extracting command execution to an interface for easier mocking
- Remember: Only Linux containers are supported (no Windows containers)
