---
name: container
description: >-
  Container engine abstraction, Docker/Podman patterns, persistent container
  lifecycle, provisioning, containerfile validation, path handling, transient
  retry behavior, and Linux-only policy. Use when working on
  internal/container/, internal/containerplan/, internal/provision/,
  internal/runtime/container*.go, pkg/invowkfile container runtime fields, or
  container-related tests/docs.
---

# Container Engine

Apply this skill to container runtime implementation, schema-facing container
configuration, provisioning, and container tests.

## Read First

- `.agents/rules/version-pinning.md` owns canonical image and pinning policy.
- `.agents/rules/windows.md` owns host-versus-container path semantics.
- `.agents/rules/testing.md` owns cross-platform and container test policy.
- Use `.agents/skills/go/SKILL.md` for Go edits and
  `.agents/skills/linux-testing/SKILL.md` for Linux/container CI failures.

Read the bundled references according to the change:

- Read [references/engine-lifecycle.md](references/engine-lifecycle.md) for
  `Engine`, Docker/Podman construction, sandbox wrapping, persistent targets,
  sysctl mitigation, and lifecycle serialization.
- Read [references/retries-and-testing.md](references/retries-and-testing.md)
  for exit-code absorption, transient classification, retry behavior, stderr
  buffering, and container test setup.

## Non-Negotiable Policy

The container runtime supports Linux containers only.

| Supported | Rejected |
| --- | --- |
| Debian-based images, with `debian:stable-slim` as the reference | Alpine images |
| Standard Linux containers | Windows container images |
| Approved language-specific slim images for language examples | Unsupported general-purpose substitutes such as `ubuntu:*` |

`ValidateSupportedRuntimeImage()` in `internal/container/image_policy.go`
enforces Alpine and Windows rejection before provisioning. Keep matching
segment-aware: reject an image whose last repository segment is `alpine`, while
avoiding false positives such as `myorg/alpine-tools`.

## Ownership Boundaries

| Package | Owns |
| --- | --- |
| `internal/container/` | Engine contracts, Docker/Podman CLI adapters, sandbox decorator, image policy, serialization primitives |
| `internal/containerplan/` | Pure persistent-target planning and `create_if_missing` policy |
| `internal/runtime/container*.go` | Runtime orchestration, provisioning, execution, retries, interactive preparation |
| `internal/provision/` | Ephemeral provisioning layer and module/file attachment |
| `pkg/invowkfile/` | User-facing runtime fields and containerfile path validation |

Keep pure target selection in `containerplan`, engine state mutation in
`container`, and command execution orchestration in `runtime`.

## Core Contracts

- The main `Engine` interface exposes portable build/run/inspect/create/start/
  exec/remove/image operations. Keep interactive PTY preparation on the smaller
  `CommandPreparer` contract.
- Docker and Podman embed `BaseCLIEngine`; inject command execution, volume
  formatting, and run-argument transformation through functional options.
- Constructors represent a missing executable with an empty binary path;
  factory selection and `Available()` report availability.
- `NewEngine()` and `AutoDetectEngine()` return sandbox-aware engines when
  Flatpak or Snap requires host execution.
- Persistent create/start/exec/remove paths use `LifecycleCoordinator` when the
  selected engine implements it.
- Prepared interactive commands return cleanup functions. Hold serialization
  leases until the PTY command has exited.

## Host And Container Paths

Container paths always use `/`, independent of the host OS.

```go
containerPath := "/workspace/" + filepath.ToSlash(relPath)
```

Do not use `filepath.Join("/workspace", relPath)` to construct a container
path; it emits backslashes on Windows hosts.

Containerfile validation is layered:

1. `pkg/invowkfile.ContainerfilePath.Validate()` and
   `ValidateContainerfilePath()` enforce the user-facing relative-path contract.
2. Behavioral sync tests keep Go validation aligned with the CUE schema.
3. `internal/container.ResolveDockerfilePath()` prevents traversal when mapping
   the build context and Containerfile into engine arguments.

## Exit And Retry Invariants

- Engine process exit failures are normally represented in
  `RunResult.ExitCode`; callers cannot rely only on the returned `error`.
- Retry and dependency-validation paths must inspect both the returned error and
  absorbed engine exit codes 125/126.
- Never retry `context.Canceled` or `context.DeadlineExceeded`.
- Discard stderr only for an attempt that is actually retried. Flush the final
  attempt on success, non-transient failure, or retry exhaustion.
- Interactive PTY execution bypasses buffered run retries, but Podman
  serialization still applies where required.

## Change Workflow

1. Classify the change by ownership boundary and read the matching reference.
2. Preserve Docker/Podman parity unless a behavior is explicitly engine-specific.
3. Validate host and container path domains separately.
4. For a new engine operation, update the narrowest applicable interface,
   concrete engines, sandbox forwarding, mocks, and lifecycle/retry callers.
5. For persistent behavior, test pure planning separately from engine mutation.
6. For retry changes, test absorbed exit codes, returned errors, cancellation,
   final stderr, and retry exhaustion.
7. Follow `.agents/rules/checklist.md` for completion gates.

## Common Pitfalls

| Pitfall | Required correction |
| --- | --- |
| Alpine or Windows image in test/docs | Use `debian:stable-slim` or an approved language-specific slim image |
| `filepath.Join` for an in-container path | Concatenate `/` paths and normalize host-relative fragments with `filepath.ToSlash` |
| Checking only `err` after `Engine.Run` | Also inspect `RunResult.Error` and `RunResult.ExitCode` |
| Persistent lifecycle bypasses coordination | Acquire through `LifecycleCoordinator` and release on every exit path |
| Shared mutable mock recorder in parallel tests | Create a recorder and engine per test/subtest |
| Real-engine container operation without bounded context/cleanup | Use the container semaphore, `ContainerTestContext`, and deferred cleanup; do not add them to `Validate()`-only, type-assertion, mocked-engine, or pre-engine error-path tests |
| Podman prepared-command lease released before `Wait` | Compose cleanup with the command lifecycle and release after process exit |

## Focused Verification

Choose the narrow checks that match the edit, then run the repository completion
gates:

```bash
go test ./internal/container ./internal/containerplan ./internal/provision
go test ./internal/runtime -run 'Container|Persistent|Retry'
go test ./internal/app/commandadapters -run 'Container|Dependency'
go test ./pkg/invowkfile -run 'Container|Runtime|SchemaSync'
```
