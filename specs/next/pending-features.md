# Pending Features - Partially Implemented

This document tracks features that were identified during codebase analysis as partially implemented or pending. These require new feature design work, not cleanup.

## 1. Custom Config File Path

**Location**: `cmd/invowk/root.go`
- Line 29: `cfgFile string` declared
- Line 81: `--config` flag binds to `cfgFile`
- Lines 122-123: `cfgFile` explicitly discarded with TODO comment

**Current State**: The `--config` CLI flag exists and is parsed, but the value is never used. Configuration always loads from the default location (`~/.invowk/config.cue`).

**Expected Behavior**: Users should be able to specify a custom config file path via `invowk --config /path/to/custom.cue`.

**Implementation Notes**:
- Modify `initRootConfig()` in `root.go` to use `cfgFile` if provided
- Update `internal/config/config.go` to accept custom paths
- Add tests for custom config loading
- Document the feature in CLI help and website

---

## 2. Force Rebuild Container Images

**Location**: `internal/runtime/container.go:683`
```go
// TODO: Add an option to force rebuild
```

**Current State**: Container images are cached. The `ImageExists()` check at line 650-654 reuses existing images without any bypass option.

**Expected Behavior**: Users should be able to force a rebuild of provisioned container images, bypassing the cache.

**Implementation Notes**:
- Add `--force-rebuild` flag to relevant commands
- Wire through `ContainerProvisionConfig` in `provision.go`
- Modify `ensureProvisionedImage()` to skip cache when flag is set
- Add tests for force rebuild behavior

---

## 3. U-Root Utils Integration

**Location**: `internal/runtime/virtual.go`
- Line 28: Constructor accepts `enableUroot` parameter
- Lines 339-354: `urootMv` and `urootCp` methods are stubbed (return `nil, nil`)

**Config Reference**: `internal/config/config_schema.cue:62` - `enable_uroot_utils` field exists

**Current State**: The configuration field and plumbing exist, but the actual u-root integration is stubbed out. The `urootMv` and `urootCp` commands return no output.

**Expected Behavior**: When `enable_uroot_utils: true`, the virtual shell should provide enhanced file operations via u-root utilities.

**Implementation Notes**:
- Research u-root integration requirements
- Implement actual u-root binary operations in `urootMv()` and `urootCp()`
- Add integration tests with u-root enabled
- Document when users might want this feature (embedded systems, minimal environments)

---

## Discovered During Audit (2026-01-29)

The following items were discovered during the codebase cleanup audit but are out of scope for that effort:

| Feature | Location | Priority |
| ------- | -------- | -------- |
| Custom config file path | root.go | Medium |
| Force rebuild containers | container.go | Low |
| U-root utils integration | virtual.go | Low |
