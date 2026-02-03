# Issue: Add Package Documentation

**Category**: Documentation
**Priority**: Low
**Effort**: Low (< 1 day)
**Labels**: `documentation`, `good-first-issue`

## Summary

Several internal packages lack `doc.go` files with package-level documentation. Add documentation to improve developer experience and code navigation.

## Problem

**Packages missing `doc.go`**:
- `internal/container/` - No package documentation
- `internal/runtime/` - No package documentation
- `internal/discovery/` - Has inline docs but no dedicated `doc.go`

**Existing good examples**:
- `internal/tui/doc.go` - Documents TUI components
- `pkg/invkfile/doc.go` - Documents invkfile parsing

## Solution

Create `doc.go` files with comprehensive package documentation.

### `internal/container/doc.go`

```go
// SPDX-License-Identifier: MPL-2.0

// Package container provides an abstraction layer for container engines.
//
// This package supports both Docker and Podman through a unified Engine
// interface, allowing the rest of the codebase to work with containers
// without coupling to a specific implementation.
//
// # Engine Interface
//
// The Engine interface defines operations for:
//   - Building container images
//   - Running containers
//   - Managing images and containers
//   - Checking engine availability
//
// # Supported Engines
//
// Two engine implementations are provided:
//   - DockerEngine: Wraps the Docker CLI
//   - PodmanEngine: Wraps the Podman CLI
//
// # Engine Selection
//
// Use NewEngine to create an engine based on configuration and availability:
//
//	engine, err := container.NewEngine(container.Config{
//	    PreferredEngine: container.EngineDocker,
//	})
//
// The function will fall back to the other engine if the preferred one
// is not available.
//
// # Limitations
//
// This package only supports Linux containers. Windows containers and
// Alpine-based images are not supported due to compatibility issues
// with invowk's provisioning system.
package container
```

### `internal/runtime/doc.go`

```go
// SPDX-License-Identifier: MPL-2.0

// Package runtime provides command execution runtimes for invowk.
//
// # Runtime Types
//
// Three runtime implementations are available:
//
//   - NativeRuntime: Executes commands using the host's shell (bash, sh, zsh).
//     This is the default runtime for most use cases.
//
//   - VirtualRuntime: Executes commands using mvdan/sh, a pure-Go shell
//     implementation. Useful for environments without a shell or for
//     consistent cross-platform behavior.
//
//   - ContainerRuntime: Executes commands inside Docker or Podman containers.
//     Provides isolation and reproducible environments.
//
// # Runtime Interface
//
// All runtimes implement the Runtime interface:
//
//	type Runtime interface {
//	    Name() string
//	    Execute(ctx *ExecutionContext) *Result
//	    Available() bool
//	    Validate(ctx *ExecutionContext) error
//	}
//
// # Extended Interfaces
//
// Runtimes may optionally implement additional interfaces:
//
//   - CapturingRuntime: For capturing command output instead of streaming
//   - InteractiveRuntime: For PTY-based interactive execution
//
// Use type assertions to check for these capabilities:
//
//	if cr, ok := runtime.(CapturingRuntime); ok {
//	    result := cr.ExecuteCapture(ctx)
//	    fmt.Println(result.Output)
//	}
//
// # Environment Building
//
// The package handles environment variable inheritance and precedence.
// See buildRuntimeEnv() for the 10-level precedence hierarchy.
//
// # Provisioning
//
// Container runtime uses provisioning to prepare images with the invowk
// binary and necessary modules. See the provision sub-package for details.
package runtime
```

### `internal/discovery/doc.go`

```go
// SPDX-License-Identifier: MPL-2.0

// Package discovery handles invkfile and invkmod discovery and command aggregation.
//
// This package intentionally combines two related concerns:
//   - File discovery: locating invkfile.cue and invkmod directories
//   - Command aggregation: building the unified command tree from discovered files
//
// These concerns are tightly coupled because command aggregation depends directly
// on discovery results and ordering. Splitting them would create unnecessary
// indirection without meaningful abstraction benefit.
//
// # Discovery Process
//
// The discovery process:
//  1. Searches for invkfile.cue in the current directory and parent directories
//  2. Searches for *.invkmod directories in configured search paths
//  3. Resolves module dependencies
//  4. Builds the aggregated command tree
//
// # Search Paths
//
// Default search paths include:
//   - Current directory
//   - ~/.invowk/modules/
//   - /usr/share/invowk/modules/ (Linux)
//
// Additional paths can be configured via config.cue or environment variables.
//
// # Module Resolution
//
// Modules are identified by their directory name (*.invkmod). Dependencies
// between modules are resolved using semantic versioning when available.
//
// # Command Aggregation
//
// Commands from multiple sources are aggregated with the following precedence:
//  1. Local invkfile.cue (highest)
//  2. First-level module dependencies
//  3. Default commands (lowest)
//
// Only first-level dependencies are visible; transitive dependencies are not
// directly accessible to prevent unexpected behavior.
package discovery
```

## Implementation Steps

1. [ ] Create `internal/container/doc.go`
2. [ ] Create `internal/runtime/doc.go`
3. [ ] Create or update `internal/discovery/doc.go`
4. [ ] Verify documentation renders correctly with `go doc`
5. [ ] Run `make lint` to verify no issues

## Acceptance Criteria

- [ ] `internal/container/doc.go` exists with comprehensive docs
- [ ] `internal/runtime/doc.go` exists with comprehensive docs
- [ ] `internal/discovery/doc.go` exists with comprehensive docs
- [ ] Documentation explains package purpose
- [ ] Key types and patterns documented
- [ ] Limitations noted where applicable
- [ ] `go doc ./internal/container` shows package docs
- [ ] `make lint` passes

## Testing

```bash
# View package documentation
go doc ./internal/container
go doc ./internal/runtime
go doc ./internal/discovery

# Verify lint passes
make lint
```

## Notes

- This is a good first issue for new contributors
- Follow existing patterns from `internal/tui/doc.go` and `pkg/invkfile/doc.go`
- Include code examples where helpful
- Document any non-obvious design decisions
