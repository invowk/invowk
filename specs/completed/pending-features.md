# Pending Features - COMPLETED

> **Status**: All items completed as of 2026-02-06.
> **Completed by**: spec-008 stateless composition refactoring and spec-005 u-root utils implementation.

This document tracked features that were identified during codebase analysis as partially implemented or pending. All items have been implemented.

## 1. Custom Config File Path -- COMPLETED

**Completed in**: spec-008 (stateless composition refactoring)

**Implementation**:
- `cmd/invowk/root.go:65`: `--config` flag binds to `rootFlags.configPath`
- `cmd/invowk/app.go:162-163`: `contextWithConfigPath()` attaches the config path to request context
- `cmd/invowk/app.go:218`: `loadConfigWithFallback()` passes the path through `config.LoadOptions{ConfigFilePath: configPath}`
- `internal/config/provider.go`: `Provider.Load()` respects `LoadOptions.ConfigFilePath`

The `--config` flag now fully propagates through the injected `ConfigProvider` interface, replacing the previous pattern where the flag value was parsed but discarded.

---

## 2. Force Rebuild Container Images -- COMPLETED

**Completed in**: spec-008 (stateless composition refactoring)

**Implementation**:
- `cmd/invowk/app.go:62`: `ExecuteRequest.ForceRebuild` field carries the flag value
- CLI `--force-rebuild` flag propagates through `CommandService.Execute()` into the container provisioning path
- `internal/provision/`: Provisioning package respects the `ForceRebuild` option to bypass image cache

---

## 3. U-Root Utils Integration -- COMPLETED

**Completed in**: spec-005 (u-root utils implementation)

**Implementation**:
- `internal/uroot/`: Complete implementations of `mv` and `cp` commands with full test coverage
- `internal/uroot/mv.go`, `internal/uroot/cp.go`: Production implementations (no longer stubs)
- Virtual shell registers u-root builtins when `enable_uroot_utils: true` in config

---

## Original Discovery Context (2026-01-29)

The following items were discovered during the codebase cleanup audit:

| Feature | Location | Priority | Status |
| ------- | -------- | -------- | ------ |
| Custom config file path | root.go, app.go | Medium | COMPLETED |
| Force rebuild containers | app.go, provision/ | Low | COMPLETED |
| U-root utils integration | internal/uroot/ | Low | COMPLETED |
