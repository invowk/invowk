# Issue: Add Integration Tests for Container Runtime

**Category**: Testability
**Priority**: Medium
**Effort**: High (3-4 days)
**Labels**: `testing`, `integration`, `container`
**Depends On**: #004 (runtime tests), #005 (container engine tests)

## Summary

No integration tests exist for container runtime execution. Add testscript-based tests that verify end-to-end container execution with Docker/Podman.

## Problem

The container runtime is a critical execution path but lacks integration test coverage for:
- Basic container execution
- Provisioning (binary/module mounting)
- SSH callback (host access from container)
- Image handling (Dockerfile, caching)

## Solution

Create testscript-based integration tests in `tests/cli/testdata/` that exercise container functionality.

### Test Scenarios

#### 1. Basic Container Execution (`container_basic.txtar`)

```txtar
# Test basic container command execution

[!exec:docker] [!exec:podman] skip 'no container runtime available'

# Create invkfile with container command
exec cat invkfile.cue
cmp stdout golden/invkfile.cue

# Run container command
exec invowk cmd run hello
stdout 'Hello from container'

-- invkfile.cue --
cmds: {
    hello: {
        desc: "Hello from container"
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: "echo 'Hello from container'"
        }]
    }
}

-- golden/invkfile.cue --
cmds: {
    hello: {
        desc: "Hello from container"
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: "echo 'Hello from container'"
        }]
    }
}
```

#### 2. Container with Arguments (`container_args.txtar`)

```txtar
# Test passing arguments to container command

[!exec:docker] [!exec:podman] skip 'no container runtime available'

exec invowk cmd run greet -- World
stdout 'Hello, World!'

-- invkfile.cue --
cmds: {
    greet: {
        desc: "Greet someone"
        args: [{
            name: "name"
            desc: "Name to greet"
        }]
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: "echo \"Hello, $1!\""
        }]
    }
}
```

#### 3. Container with Environment Variables (`container_env.txtar`)

```txtar
# Test environment variable passing to container

[!exec:docker] [!exec:podman] skip 'no container runtime available'

env FOO=bar
exec invowk cmd run show-env
stdout 'FOO=bar'
stdout 'CUSTOM=value'

-- invkfile.cue --
cmds: {
    "show-env": {
        desc: "Show environment"
        env: {
            CUSTOM: "value"
        }
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: """
                echo "FOO=$FOO"
                echo "CUSTOM=$CUSTOM"
                """
        }]
    }
}
```

#### 4. Container Provisioning (`container_provision.txtar`)

```txtar
# Test that invowk binary and modules are available in container

[!exec:docker] [!exec:podman] skip 'no container runtime available'

# Verify invowk binary is mounted
exec invowk cmd run check-binary
stdout 'invowk binary found'

# Verify module files are accessible
exec invowk cmd run check-files
stdout 'invkfile.cue found'

-- invkfile.cue --
cmds: {
    "check-binary": {
        desc: "Check invowk binary"
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: """
                if [ -x /workspace/.invowk/bin/invowk ]; then
                    echo "invowk binary found"
                else
                    echo "invowk binary NOT found"
                    exit 1
                fi
                """
        }]
    }
    "check-files": {
        desc: "Check workspace files"
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: """
                if [ -f /workspace/invkfile.cue ]; then
                    echo "invkfile.cue found"
                else
                    echo "invkfile.cue NOT found"
                    exit 1
                fi
                """
        }]
    }
}
```

#### 5. Container with Containerfile (`container_dockerfile.txtar`)

```txtar
# Test custom Containerfile support

[!exec:docker] [!exec:podman] skip 'no container runtime available'

exec invowk cmd run custom-image
stdout 'Custom tool version'

-- invkfile.cue --
cmds: {
    "custom-image": {
        desc: "Run with custom image"
        impls: [{
            container: {
                containerfile: "Containerfile"
            }
            script: "custom-tool --version"
        }]
    }
}

-- Containerfile --
FROM debian:stable-slim
RUN echo '#!/bin/sh\necho "Custom tool version 1.0"' > /usr/local/bin/custom-tool && \
    chmod +x /usr/local/bin/custom-tool
```

#### 6. SSH Callback (`container_callback.txtar`)

```txtar
# Test SSH callback for host access from container

[!exec:docker] [!exec:podman] skip 'no container runtime available'

# Create a file on host, verify container can see it via callback
exec touch host-file.txt
exec invowk cmd run callback-test
stdout 'Callback successful'

-- invkfile.cue --
cmds: {
    "callback-test": {
        desc: "Test SSH callback"
        impls: [{
            container: {
                image: "debian:stable-slim"
                host_access: true
            }
            script: """
                # Use invowk host command to access host filesystem
                if invowk host exec ls host-file.txt 2>/dev/null; then
                    echo "Callback successful"
                else
                    echo "Callback failed"
                    exit 1
                fi
                """
        }]
    }
}
```

#### 7. Exit Code Handling (`container_exitcode.txtar`)

```txtar
# Test exit code propagation from container

[!exec:docker] [!exec:podman] skip 'no container runtime available'

! exec invowk cmd run fail
stderr 'exit code: 42'

-- invkfile.cue --
cmds: {
    fail: {
        desc: "Command that fails"
        impls: [{
            container: {
                image: "debian:stable-slim"
            }
            script: "exit 42"
        }]
    }
}
```

## Implementation Steps

1. [ ] Create `tests/cli/testdata/container_basic.txtar`
2. [ ] Create `tests/cli/testdata/container_args.txtar`
3. [ ] Create `tests/cli/testdata/container_env.txtar`
4. [ ] Create `tests/cli/testdata/container_provision.txtar`
5. [ ] Create `tests/cli/testdata/container_dockerfile.txtar`
6. [ ] Create `tests/cli/testdata/container_callback.txtar`
7. [ ] Create `tests/cli/testdata/container_exitcode.txtar`
8. [ ] Update CI to skip container tests (already configured)
9. [ ] Add Makefile target for container tests
10. [ ] Document how to run container tests locally

## Makefile Target

```makefile
# Run container integration tests (requires Docker or Podman)
test-container:
	@echo "Running container integration tests..."
	go test -v ./tests/cli/... -run 'container_'

# Run all integration tests except container
test-cli:
	go test -v ./tests/cli/... -skip 'container_'
```

## Acceptance Criteria

- [ ] Basic container execution tested
- [ ] Argument passing tested
- [ ] Environment variable passing tested
- [ ] Provisioning (binary/module mounting) tested
- [ ] Custom Containerfile tested
- [ ] SSH callback tested (if feasible)
- [ ] Exit code propagation tested
- [ ] Tests properly skip when no container runtime available
- [ ] CI continues to work (tests skip in GitHub Actions)
- [ ] Local development documented

## Testing

```bash
# Run container tests locally (requires Docker or Podman)
make test-container

# Run with verbose output
go test -v ./tests/cli/... -run 'container_'

# Skip container tests in CI
go test -v ./tests/cli/... -skip 'container_'
```

## Notes

- Tests must gracefully skip when Docker/Podman is unavailable
- Use `[!exec:docker] [!exec:podman] skip` directive
- Always use `debian:stable-slim` as base image (per project rules)
- SSH callback tests may require special setup or be skipped in some environments
- Consider test isolation (each test should be independent)
