# Data Model: Go Codebase Quality Audit

**Feature Branch**: `006-go-codebase-audit`
**Date**: 2026-01-30

## Overview

This document defines the entity models for the extracted abstractions in the codebase audit. Since this is a refactoring effort, the data models represent Go types rather than database entities.

---

## 1. Server State Machine Entities

### State

Represents the lifecycle state of a long-running server component.

| Field | Type | Description |
|-------|------|-------------|
| value | int32 | Atomic integer representation (0-5) |

**State Values**:

| Name | Value | Description |
|------|-------|-------------|
| Created | 0 | Server instance created, `Start()` not called |
| Starting | 1 | `Start()` called, server initializing |
| Running | 2 | Server accepting connections/requests |
| Stopping | 3 | `Stop()` called, graceful shutdown in progress |
| Stopped | 4 | Terminal: server has stopped |
| Failed | 5 | Terminal: server failed to start or fatal error |

**State Transitions**:
```
Created → Starting → Running → Stopping → Stopped
                ↓                          ↑
              Failed ─────────────────────
```

### Base (Server Base)

Common fields and lifecycle infrastructure for all server types.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| state | atomic.Int32 | - | Lock-free state reads |
| stateMu | sync.Mutex | - | Protects state transitions |
| ctx | context.Context | - | Server context for cancellation |
| cancel | context.CancelFunc | - | Cancellation function |
| wg | sync.WaitGroup | - | Tracks background goroutines |
| startedCh | chan struct{} | - | Closed when server is ready |
| errCh | chan error | buffered(1) | Async error notifications |
| lastErr | error | - | Stores error for Failed state |

**Validation Rules**:
- State transitions must be protected by `stateMu`
- `Start()` can only be called from `Created` state
- `Stop()` is idempotent from any state
- Server is single-use; once stopped/failed, create new instance

---

## 2. Container Engine Entities

### BaseCLIEngine

Common fields and methods for CLI-based container engines (Docker, Podman).

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| binaryName | string | not empty | CLI binary name ("docker" or "podman") |
| execCommand | func | not nil | Command execution function (injectable for testing) |

### BuildArgs

Arguments for container image build operations.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| ContextPath | string | valid path | Build context directory |
| Dockerfile | string | optional | Path to Dockerfile (relative or absolute) |
| ImageTag | string | not empty | Tag for built image |
| NoCache | bool | - | Disable layer caching |
| BuildArgs | map[string]string | - | Build-time variables |

**Validation Rules**:
- If Dockerfile is relative, resolve against ContextPath
- ImageTag must be valid Docker image reference

### RunArgs

Arguments for container run operations.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| Image | string | not empty | Container image to run |
| Command | []string | - | Command and arguments |
| Volumes | []VolumeMount | - | Volume mount specifications |
| Ports | []PortMapping | - | Port mapping specifications |
| Env | map[string]string | - | Environment variables |
| Workdir | string | optional | Working directory in container |
| Remove | bool | default true | Remove container after exit |
| Interactive | bool | - | Allocate TTY |

### VolumeMount

Volume mount specification.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| HostPath | string | valid path | Path on host |
| ContainerPath | string | absolute | Path in container |
| ReadOnly | bool | - | Mount as read-only |
| SELinux | string | optional | SELinux label (`:z` or `:Z`) |

### PortMapping

Port mapping specification.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| HostPort | uint16 | 1-65535 | Port on host |
| ContainerPort | uint16 | 1-65535 | Port in container |
| Protocol | string | "tcp" or "udp" | Protocol (default: tcp) |

---

## 3. CUE Parsing Entities

### ParseOptions

Configuration for CUE parsing operations.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| MaxFileSize | int64 | > 0, default 5MB | Maximum allowed file size |
| SchemaPath | string | CUE path format | Path to schema definition (e.g., "#Invkfile") |
| Filename | string | - | Filename for error messages |
| Concrete | bool | default true | Require all values to be concrete |

### ParseResult[T]

Generic result from CUE parsing.

| Field | Type | Description |
|-------|------|-------------|
| Value | *T | Decoded Go struct |
| Unified | cue.Value | Unified CUE value (for advanced use) |

---

## 4. Error Context Entities

### ActionableError

Error with context for user-facing error messages.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| Operation | string | not empty | What was being attempted |
| Resource | string | optional | File, path, or entity involved |
| Suggestions | []string | optional | How to fix the issue |
| Cause | error | optional | Wrapped underlying error |

**Rendering Rules**:
- Default format: `"failed to {Operation}: {Resource}"`
- With suggestions: append bulleted list
- With --verbose: include full error chain from Cause

### ErrorContext

Helper for building errors with context.

| Method | Description |
|--------|-------------|
| `WithOperation(op string)` | Set the operation being performed |
| `WithResource(res string)` | Set the resource involved |
| `WithSuggestion(sug string)` | Add a suggestion |
| `Wrap(err error)` | Wrap an underlying error |
| `Build()` | Create ActionableError |

---

## 5. Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                        Server Components                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐                                              │
│   │ serverbase   │                                              │
│   │   .State     │◄────── State transitions, String()           │
│   │   .Base      │◄────── Common fields, lifecycle helpers      │
│   └──────┬───────┘                                              │
│          │ embeds                                                │
│    ┌─────┴─────┐                                                │
│    │           │                                                │
│ ┌──▼──┐    ┌──▼──┐                                             │
│ │ SSH │    │ TUI │                                              │
│ │Server│   │Server│  ◄── Server-specific: tokens, HTTP, etc.   │
│ └──────┘   └──────┘                                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     Container Engine Components                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐                                              │
│   │BaseCLIEngine │                                              │
│   │ .BuildArgs() │◄────── Argument construction                 │
│   │ .RunArgs()   │◄────── Volume, port, env handling           │
│   └──────┬───────┘                                              │
│          │ embeds                                                │
│    ┌─────┴─────┐                                                │
│    │           │                                                │
│ ┌──▼──┐    ┌──▼──┐                                             │
│ │Docker│   │Podman│                                             │
│ │Engine│   │Engine│  ◄── Engine-specific: version fmt, SELinux │
│ └──────┘   └──────┘                                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      CUE Parsing Flow                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌────────────┐    ┌──────────────┐    ┌────────────────┐     │
│   │ Schema     │───▶│ ParseAndDecode│───▶│ Typed Result   │     │
│   │ (embedded) │    │ [T any]       │    │ (*Invkfile,    │     │
│   └────────────┘    └───────┬───────┘    │  *Invkmod,     │     │
│                             │            │  *Config)      │     │
│   ┌────────────┐            │            └────────────────┘     │
│   │ User Data  │────────────┘                                   │
│   │ (.cue file)│                                                │
│   └────────────┘                                                │
│                                                                  │
│   Steps: 1. Compile schema → 2. Unify → 3. Validate → 4. Decode │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 6. CUE Schema Changes

### New Constraints

| Entity | Field | Constraint | Reason |
|--------|-------|------------|--------|
| #Command | description | `=~"^\\s*\\S.*$"` | Non-empty with content |
| #RuntimeConfigContainer | image | `strings.MaxRunes(512)` | Prevent abuse |
| #RuntimeConfigNative | interpreter | `strings.MaxRunes(1024)` | Consistent with default_shell |
| #RuntimeConfigContainer | interpreter | `strings.MaxRunes(1024)` | Consistent with default_shell |
| #Flag | default_value | `strings.MaxRunes(4096)` | Reasonable flag value limit |
| #Argument | default_value | `strings.MaxRunes(4096)` | Reasonable arg value limit |

### Schema Sync Test Updates

Each constraint addition requires corresponding sync test verification:
- Add test cases for boundary values (exactly at limit, one over)
- Verify Go struct tags match CUE field names
- Ensure error messages include CUE path for debugging
