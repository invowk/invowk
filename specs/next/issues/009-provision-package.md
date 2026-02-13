# Issue: Extract Provisioning to Separate Package

**Category**: Architecture
**Priority**: Medium
**Effort**: Medium (2-3 days)
**Labels**: `architecture`, `refactoring`, `container`

## Summary

Extract container provisioning logic (~300 lines across 3 files) from `internal/runtime/` into a dedicated `internal/provision/` package. This improves separation of concerns and enables the "force rebuild" feature.

## Problem

Container provisioning logic is currently embedded in the runtime package:
- `internal/runtime/provision.go` - Core provisioning logic (~100 lines)
- `internal/runtime/provision_layer.go` - Layer utilities (~100 lines)
- `internal/runtime/container_provision.go` - Container-specific provisioning (~100 lines)

**Issues**:
1. Runtime package mixes execution and preparation concerns
2. Provisioning logic is hard to test without full runtime
3. No way to force rebuild (existing TODO at `container_provision.go:87`)
4. Platform-specific logic (Docker vs Podman) not clearly separated

## Solution

Create a dedicated `internal/provision/` package:

### Package Structure

```
internal/provision/
├── doc.go                     # Package documentation
├── provisioner.go             # Provisioner interface
├── provisioner_test.go        # Interface tests
├── layer_provisioner.go       # LayerProvisioner implementation
├── layer_provisioner_test.go  # Implementation tests
├── layer.go                   # Layer building utilities
├── layer_test.go              # Layer tests
├── hash.go                    # Content hashing for caching
├── hash_test.go               # Hash tests
└── config.go                  # ProvisionConfig type
```

### Interface Definition

```go
// internal/provision/provisioner.go

// SPDX-License-Identifier: MPL-2.0

// Package provision handles resource provisioning for container execution.
//
// This package provides the Provisioner interface and implementations for
// preparing container images with the necessary resources (invowk binary,
// modules, invowkfiles) for command execution.
package provision

import (
    "context"
)

// Provisioner prepares resources for container execution.
type Provisioner interface {
    // Provision prepares the container image with necessary resources.
    // Returns the image ID to use for execution.
    Provision(ctx context.Context, opts *ProvisionOptions) (imageID string, err error)

    // Cleanup removes provisioned resources.
    Cleanup(ctx context.Context, imageID string) error
}

// ProvisionOptions configures what resources to provision.
type ProvisionOptions struct {
    // BaseImage is the starting image (e.g., "debian:stable-slim")
    BaseImage string

    // Containerfile is the path to a custom Containerfile (optional)
    Containerfile string

    // BinaryPath is the path to the invowk binary to mount
    BinaryPath string

    // Modules are module directories to mount
    Modules []ModuleMount

    // Invowkfiles are invowkfile paths to mount
    Invowkfiles []InvowkfileMount

    // ForceRebuild bypasses cache and forces a fresh build
    ForceRebuild bool

    // WorkDir is the working directory for the build
    WorkDir string

    // Engine is the container engine to use
    Engine Engine
}

// ModuleMount specifies a module directory to mount.
type ModuleMount struct {
    // SourcePath is the host path to the module directory
    SourcePath string

    // ModuleID is the module identifier (for container path)
    ModuleID string
}

// InvowkfileMount specifies an invowkfile to mount.
type InvowkfileMount struct {
    // SourcePath is the host path to the invowkfile
    SourcePath string

    // TargetPath is the container path (relative to /workspace)
    TargetPath string
}

// Engine abstracts container engine operations needed for provisioning.
type Engine interface {
    // Build builds an image from a Containerfile
    Build(ctx context.Context, opts BuildOptions) (imageID string, err error)

    // ImageExists checks if an image exists
    ImageExists(ctx context.Context, imageID string) (bool, error)

    // RemoveImage removes an image
    RemoveImage(ctx context.Context, imageID string) error
}
```

### Layer Provisioner Implementation

```go
// internal/provision/layer_provisioner.go

// LayerProvisioner creates ephemeral container image layers.
type LayerProvisioner struct {
    cacheDir string
}

// NewLayerProvisioner creates a provisioner that uses layer-based caching.
func NewLayerProvisioner(cacheDir string) *LayerProvisioner {
    return &LayerProvisioner{cacheDir: cacheDir}
}

// Provision creates a provisioned image with mounted resources.
func (p *LayerProvisioner) Provision(ctx context.Context, opts *ProvisionOptions) (string, error) {
    // Calculate content hash for caching
    hash, err := p.calculateHash(opts)
    if err != nil {
        return "", fmt.Errorf("calculating hash: %w", err)
    }

    imageID := fmt.Sprintf("invowk-provision:%s", hash)

    // Check cache (unless force rebuild)
    if !opts.ForceRebuild {
        exists, err := opts.Engine.ImageExists(ctx, imageID)
        if err != nil {
            return "", fmt.Errorf("checking image: %w", err)
        }
        if exists {
            return imageID, nil
        }
    }

    // Build new image
    containerfile, err := p.generateContainerfile(opts)
    if err != nil {
        return "", fmt.Errorf("generating containerfile: %w", err)
    }

    buildOpts := BuildOptions{
        Containerfile: containerfile,
        Tag:           imageID,
        Context:       opts.WorkDir,
    }

    if _, err := opts.Engine.Build(ctx, buildOpts); err != nil {
        return "", fmt.Errorf("building image: %w", err)
    }

    return imageID, nil
}

// Cleanup removes a provisioned image.
func (p *LayerProvisioner) Cleanup(ctx context.Context, imageID string) error {
    // Only cleanup invowk-provision images
    if !strings.HasPrefix(imageID, "invowk-provision:") {
        return nil
    }
    return opts.Engine.RemoveImage(ctx, imageID)
}

func (p *LayerProvisioner) calculateHash(opts *ProvisionOptions) (string, error) {
    // Hash based on:
    // - Base image
    // - Binary content
    // - Module contents
    // - Invowkfile contents
    // ...
}

func (p *LayerProvisioner) generateContainerfile(opts *ProvisionOptions) (string, error) {
    // Generate Containerfile that:
    // - Starts FROM base image
    // - COPYs invowk binary
    // - COPYs modules
    // - COPYs invowkfiles
    // - Sets up workspace structure
    // ...
}
```

### Config Type

```go
// internal/provision/config.go

// Config holds provisioning configuration.
type Config struct {
    // CacheDir is the directory for caching provisioned images
    CacheDir string

    // DefaultImage is the default base image when none specified
    DefaultImage string

    // ForceRebuild globally forces rebuild (overrides per-request)
    ForceRebuild bool
}

// DefaultConfig returns the default provisioning configuration.
func DefaultConfig() *Config {
    return &Config{
        CacheDir:     filepath.Join(os.TempDir(), "invowk-provision"),
        DefaultImage: "debian:stable-slim",
        ForceRebuild: false,
    }
}
```

## Files to Modify

### New Files

| File | Description |
|------|-------------|
| `internal/provision/doc.go` | Package documentation |
| `internal/provision/provisioner.go` | Interface definitions |
| `internal/provision/layer_provisioner.go` | LayerProvisioner implementation |
| `internal/provision/layer.go` | Layer building utilities |
| `internal/provision/hash.go` | Content hashing |
| `internal/provision/config.go` | Configuration types |
| `internal/provision/*_test.go` | Tests for each file |

### Files to Refactor

| File | Changes |
|------|---------|
| `internal/runtime/provision.go` | Move to `provision/layer_provisioner.go` |
| `internal/runtime/provision_layer.go` | Move to `provision/layer.go` |
| `internal/runtime/container_provision.go` | Move to `provision/` |
| `internal/runtime/container.go` | Use `provision.Provisioner` interface |

### Files to Update

| File | Changes |
|------|---------|
| `cmd/invowk/root.go` | Add `--force-rebuild` flag |
| `cmd/invowk/cmd_run.go` | Pass force-rebuild option |
| `internal/config/config.go` | Add provision config section |

## Implementation Steps

1. [ ] Create `internal/provision/` directory structure
2. [ ] Create `doc.go` with package documentation
3. [ ] Define `Provisioner` interface in `provisioner.go`
4. [ ] Define `Config` type in `config.go`
5. [ ] Move and refactor `provision.go` → `layer_provisioner.go`
6. [ ] Move and refactor `provision_layer.go` → `layer.go`
7. [ ] Move and refactor `container_provision.go` → remaining logic
8. [ ] Implement `ForceRebuild` option (resolves TODO)
9. [ ] Add `--force-rebuild` CLI flag
10. [ ] Update `ContainerRuntime` to use `provision.Provisioner`
11. [ ] Add comprehensive tests
12. [ ] Remove old files from `internal/runtime/`

## Acceptance Criteria

- [ ] New `internal/provision/` package created
- [ ] `Provisioner` interface defined and documented
- [ ] `LayerProvisioner` implements interface
- [ ] `ForceRebuild` option implemented (TODO resolved)
- [ ] `--force-rebuild` CLI flag added
- [ ] Existing functionality preserved
- [ ] Tests for provisioning logic (>60% coverage)
- [ ] Runtime package uses interface (can inject mock)
- [ ] Old files removed from runtime package
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make test-cli` passes

## Testing

```bash
# Run provision package tests
go test -v ./internal/provision/...

# Verify CLI flag
invowk cmd run --help | grep force-rebuild

# Test force rebuild behavior
invowk cmd run --force-rebuild my-container-cmd
```

## Notes

- This resolves the TODO at `container_provision.go:87`
- The interface allows mocking for runtime tests
- Consider supporting multiple provisioner implementations in the future
- Ensure Windows path handling is correct (see `.claude/rules/windows.md`)
