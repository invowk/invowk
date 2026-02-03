# Issue: Resolve Existing TODOs

**Category**: Documentation
**Priority**: Medium
**Effort**: Medium (2-3 days)
**Labels**: `enhancement`, `technical-debt`
**Related**: specs/next/pending-features.md

## Summary

Two TODOs in the codebase represent incomplete features. Implement them or document them as intentionally deferred.

## Outstanding TODOs

### 1. Custom Config File Path

**Location**: `cmd/invowk/root.go:107`

```go
// Line 29: cfgFile string declared
// Line 81: --config flag binds to cfgFile
// Lines 122-123: cfgFile explicitly discarded with TODO comment
_ = cfgFile // TODO: Support custom config file path via cfgFile flag
```

**Current State**: The `--config` CLI flag exists and is parsed, but the value is never used. Configuration always loads from the default location (`~/.invowk/config.cue`).

**Expected Behavior**: Users should be able to specify a custom config file path:
```bash
invowk --config /path/to/custom.cue cmd run hello
```

**Implementation Steps**:
1. [ ] Modify `initRootConfig()` in `root.go` to check `cfgFile`
2. [ ] Update `internal/config/config.go` to accept custom paths
3. [ ] Add `LoadFromPath(path string)` function to config package
4. [ ] Validate custom config file exists before loading
5. [ ] Add tests for custom config loading
6. [ ] Update CLI help text to document the feature
7. [ ] Add example to website documentation

**Files to Modify**:
- `cmd/invowk/root.go`
- `internal/config/config.go`
- `internal/config/config_test.go` (new tests)

### 2. Force Rebuild Container Images

**Location**: `internal/runtime/container_provision.go:87`

```go
// TODO: Add an option to force rebuild
```

**Current State**: Container images are cached. The `ImageExists()` check reuses existing images without any bypass option.

**Expected Behavior**: Users should be able to force rebuild:
```bash
invowk cmd run --force-rebuild my-container-cmd
```

**Implementation Steps**:
1. [ ] Add `--force-rebuild` flag to `cmd run` command
2. [ ] Pass flag through ExecutionContext or ProvisionConfig
3. [ ] Modify `ensureProvisionedImage()` to skip cache when flag is set
4. [ ] Add tests for force rebuild behavior
5. [ ] Document in CLI help and website

**Files to Modify**:
- `cmd/invowk/cmd_run.go`
- `internal/runtime/container.go`
- `internal/runtime/container_provision.go`

**Note**: If #009 (Extract Provisioning Package) is done first, this becomes part of that work.

## Alternative: Document as Intentionally Deferred

If implementing these features is not a priority, update the TODO comments to explain why:

```go
// NOTE: Custom config path support is deferred. See specs/next/pending-features.md
// for implementation plan when this becomes a priority.
_ = cfgFile
```

## Acceptance Criteria

**If implementing**:
- [ ] `--config` flag works correctly
- [ ] `--force-rebuild` flag works correctly
- [ ] TODOs removed from code
- [ ] Features documented in CLI help
- [ ] Features documented on website
- [ ] Tests added for new functionality

**If deferring**:
- [ ] TODOs updated with clear explanation
- [ ] `pending-features.md` is authoritative source
- [ ] No misleading comments in code

## Testing

```bash
# Test custom config
invowk --config ./test-config.cue cmd list
invowk --config /nonexistent.cue cmd list  # Should error gracefully

# Test force rebuild
invowk cmd run --force-rebuild my-container-cmd
# Verify image is rebuilt (check docker images timestamps)
```

## Related Documentation

See `specs/next/pending-features.md` for detailed implementation notes on both features.

## Notes

- These are user-facing features that have been requested
- The --config flag already exists but does nothing (confusing UX)
- Force rebuild is useful for debugging container issues
- Consider whether #009 (Provisioning Package) should include force rebuild
