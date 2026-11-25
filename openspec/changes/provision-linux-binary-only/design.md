# Design

## Context

See proposal.md (Why) for the problem. The current code path:

- `buildProvisionConfig` first calls `applyHostProvisionDefaults`, which sets `provision.Config.InvowkBinaryPath` to `os.Executable()` with no OS or architecture check (`internal/runtime/container_provision.go:232-241`), then overwrites it with `container.auto_provision.binary_path` when that is set (`internal/runtime/container_provision.go:201-203`). After this, nothing can tell a configured path from the default.
- `LayerProvisioner` uses `InvowkBinaryPath != ""` as the single "install a binary" switch: in the cache key (`internal/provision/layer_provisioner.go:194-201`), when copying into the build context (`internal/provision/layer_provisioner.go:310-317`), in the Dockerfile `COPY`/`chmod` and `ENV PATH` lines (`internal/provision/dockerfile.go:25-30`, `:43-45`) and in the env vars returned to the runtime (`internal/provision/dockerfile.go:69-71`).
- A cache hit returns before any build and without warnings (`internal/provision/layer_provisioner.go:110-118`).
- `ensureProvisionedImage` turns every `Provision` error into a warning plus the bare base image unless `strict` is set (`internal/runtime/container_provision.go:78-91`); provisioning warnings become `container_provisioning_warning` diagnostics (`internal/runtime/container_provision.go:92-98`).
- Persistent containers compute their provisioned image identity through `GetProvisionedImageTag`, which calls the same `calculateCacheKey` (`internal/runtime/container_persistent.go:253-273`, `internal/provision/layer_provisioner.go:144-153`).
- invowk never passes a platform to the engine: neither `BuildArgs` nor `RunArgs` adds `--platform` (`internal/container/engine_base.go:243-330`). The container therefore always runs the engine's default platform for the image, and "the platform of the run" carries no information beyond the image's own architecture.
- The engine adapters already expose a read-only query runner, `EngineQueryRunner.RunQuery`, implemented by `BaseCLIEngine` and forwarded by `SandboxAwareEngine` to the host engine (`internal/container/network_facts.go:38-45`, `internal/container/engine_base.go:446-448`, `internal/container/sandbox_engine.go:95-108`). `InspectImage` exists but has no caller (`internal/container/engine_base.go:787-789`).
- When the run uses a Containerfile, `ensureImage` builds the base image locally before provisioning (`internal/runtime/container_provision.go:104-171`), so the engine knows its architecture. An `image:` base may not be local yet: the provisioned-layer build pulls it.
- Formal-bound symbols nearby: `CopyDir` (`internal/provision/helpers.go`) and `ContainerRuntime.getContainerWorkDir` (`internal/runtime/container_provision.go`). This change edits neither.

## Goals / Non-Goals

**Goals:**

- One binary decision per provisioning request, shared by `Provision` and `GetProvisionedImageTag`, made before the cache key.
- Every branch testable on a Linux CI runner with small fixture files.

**Non-Goals:**

- Building, downloading or cross-compiling a Linux invowk for the user.
- Checking that the file is really invowk, its version, whether it is statically linked, or the amd64 microarchitecture level (release builds use `GOAMD64=v3`, which an ELF header does not record).
- Asking the daemon for its own architecture (`docker info`/`podman info`), honouring `DOCKER_DEFAULT_PLATFORM`, or adding a `--platform` option.
- Cleaning up provisioned images cached under the old keys.

## Decisions

### D1. Decide by the file format, with `debug/elf`

The candidate file is opened and parsed with `debug/elf.NewFile`; only the file header is used. It is usable when all hold:

- it parses as ELF;
- `OSABI` is `ELFOSABI_NONE` (Go's Linux builds) or `ELFOSABI_LINUX`;
- `Type` is `ET_EXEC` or `ET_DYN` (a PIE);
- `Class`, `Data` and `Machine` match the container architecture, from a small table keyed by Go/OCI architecture names: `amd64` (64-bit, little-endian, `EM_X86_64`), `arm64` (`EM_AARCH64`), `386` (32-bit, `EM_386`), `arm` (32-bit, `EM_ARM`), `ppc64le` (`EM_PPC64`, little-endian), `s390x` (`EM_S390`, big-endian), `riscv64` (`EM_RISCV`, 64-bit). An architecture name outside the table is treated as unknown (D2), not as a mismatch.

Failures carry a reason the messages repeat: "not an ELF file" (with "Mach-O" or "PE" named when the magic shows it), "ELF for another operating system", "not an executable", "built for <arch>, container is <arch>", or the read error. An ELF without section headers (UPX output) parses: with `e_shoff` and `e_shnum` both zero, `NewFile` reads no section table (`$GOROOT/src/debug/elf/file.go`, the `shoff`/`shnum` checks in `NewFile`). Go's linker writes `ELFOSABI_NONE` for Linux and for DragonFly, illumos and Solaris builds too (`$GOROOT/src/cmd/link/internal/ld/elf.go`, the `osabi` switch), so those few are indistinguishable from Linux by header; FreeBSD, NetBSD and OpenBSD builds are rejected.

*Alternatives:* check `runtime.GOOS` (rejected by the maintainer: it misses a wrong explicit `binary_path` and a Linux host whose architecture differs from the image's); run the binary inside the container (`invowk --version`) to test it (rejected: it runs an extra container); hand-parse the first 64 bytes (rejected: `debug/elf` is in the standard library and already validates the identity bytes).

### D2. Container architecture: image inspect, then the Linux host

A new read-only query in `internal/container` runs `<engine> image inspect --format '{{.Architecture}}' <base image>` through `EngineQueryRunner`. Docker and Podman both report Go/OCI names (`amd64`, `arm64`). Any error or empty output means "unknown". The arm variant (`v6`/`v7`) is ignored. The provisioner takes the query as an optional capability of its engine, by interface assertion, as the persistent-container code already does for `GetProvisionedImageTag`: engines without it (test mocks) give "unknown".

When it is unknown: on a Linux host the expected architecture is `runtime.GOARCH`; on any other host there is none, and D1 then accepts a Linux ELF for any machine in the table. The host OS and architecture, and the query, are fields of `LayerProvisioner` that internal tests set, like the existing `copyDir` hook.

Assumed, and recorded here as the maintainer asked: on a Linux host, an engine that does not yet have the image will pull it for the host's architecture. This is false when `DOCKER_DEFAULT_PLATFORM` is set, or when `DOCKER_HOST` points at a daemon on another architecture. On a non-Linux host nothing is assumed: a Mach-O or PE default binary is rejected by D1 alone, and an explicit Linux `binary_path` is trusted until the engine has the image.

*Alternatives:* ask the daemon for its own architecture (`docker info --format '{{.Architecture}}'`, `podman info --format '{{.Host.Arch}}'`); it would also cover Docker Desktop and remote daemons before the first pull, but the maintainer decision limits this change to the image inspect and the Linux-host fallback, so it is a candidate follow-up. Pull the image first to inspect it (rejected: "do not pull or run anything extra").

### D3. Keep the default apart from the configured path

`provision.Config` gains `DefaultInvowkBinaryPath`, the binary the runtime adapter supplies (`os.Executable()`); `InvowkBinaryPath` now means only the configured `binary_path`. `applyHostProvisionDefaults` fills the new field, and `buildProvisionConfig` sets `InvowkBinaryPath` only from config. The configured path wins when both are set. Both are validated as today.

*Alternative:* a boolean "explicit" flag next to one path. Rejected: two named fields keep each value's origin visible where it is set, and the zero value of the old field keeps meaning "nothing configured".

### D4. One resolution step in the provisioner

`LayerProvisioner` resolves the binary once per request, from the base image, before the cache key: the result is the path to install (or none) and an optional warning, or an error. `Provision` and `GetProvisionedImageTag` both call it; the cache key, the build-context copy, the Dockerfile and the env vars take the resolved path as an argument instead of reading `p.config.InvowkBinaryPath`. The shared `Config` is never mutated, so concurrent runs on one provisioner do not race.

- Configured path unusable → `*UnusableInvowkBinaryError{Path, Reason}`, wrapping the sentinel `ErrUnusableInvowkBinary`.
- Default path unusable, or empty → no binary; the warning only when a default path existed but was rejected.
- The warning is attached to the result on a cache hit as well as after a build.

### D5. The runtime does not soften an explicit failure

`ensureProvisionedImage` returns an error matching `ErrUnusableInvowkBinary` as is, before the non-strict fallback; the run fails through the existing "failed to prepare container image" path (`internal/runtime/container_exec.go:118-121`). `persistentSpecImage` already returns `GetProvisionedImageTag` errors. The skip warning reuses the existing `container_provisioning_warning` code, so the closed `InitDiagnosticCode` set and its validation do not change. Its text:

> invowk binary `<path>` was not installed in the container: `<reason>`. Nested invowk commands inside the container will not work. Set container.auto_provision.binary_path to a Linux `<arch>` build of invowk (`<arch>` is "the image's architecture" when unknown).

A missing explicit `binary_path` is an unusable explicit binary too: today it is hashed in the cache key, fails, and under non-strict mode degrades to the base image with a warning. Treating it the same as a wrong format keeps one rule for "the user asked for this file".

### D6. Cache keys

The binary enters the cache key only when it is installed, exactly as an empty path does today, so an image built without the binary has its own key and a later usable binary gets a new image. The skip reason is not part of the key: it is recomputed and warned about on every run. `provisionedLayerCacheVersion` is not bumped: an image that held an unusable binary had a key containing that binary's hash, which a skipping run no longer produces.

### D7. Tests on Linux

Fixture files are written by a test helper from `encoding/binary`: a 64-bit Mach-O magic, a minimal PE (`MZ` stub plus `PE\0\0` header), ELF headers for amd64 and arm64, an ELF with FreeBSD OS ABI, an `ET_REL` object, a header-only ELF without section headers, a truncated ELF, an empty file and a shell script. A test that needs two different usable binaries appends different bytes after the same header. Provisioner tests inject the host OS/architecture and the inspect result; `internal/container` tests check the inspect arguments and parsing with a fake `EngineQueryRunner`. Evidence for macOS and Windows hosts is therefore simulated on Linux.

## Risks / Trade-offs

- [Linux host, remote daemon or `DOCKER_DEFAULT_PLATFORM` on another architecture, image not yet local] → The first run installs a host-architecture binary that cannot execute (today's behaviour). Once the image is local, the inspect reports the real architecture: an explicit `binary_path` then fails, a default binary is skipped with the warning, and the cache key changes. A persistent container created on that first run keeps the old identity until it is recreated.
- [macOS or Windows host, explicit `binary_path` for the wrong Linux architecture, image not yet local] → Accepted on the first run, rejected with an error from the second. The docs tell users to pick the archive that matches the image.
- [Every skipped-binary run prints a warning] → Intended: it is the actionable message the maintainer asked for. Users silence it by setting `binary_path` or disabling auto-provision.
- [One extra `image inspect` per provisioned run, two for persistent containers] → Only when there is a candidate binary; it is a local read-only query.
- [Existing tests use fake binaries (`fake-binary-content`, a shell script stub) as an explicit path] → They now fail by design; they move to ELF fixtures (D7), and the non-strict fallback tests force failure through the mocked build error instead of a missing binary.

## Migration Plan

No data migration. The behaviour change for explicit paths is in proposal.md "Migration notes" and goes into the release notes. Rollback is a revert; cached images from either version stay usable because their keys differ.

## Maintainer decisions

2026-11-19, decisions 44 and 45 (group D, tracker N12), as stated in the group D brief:

1. Decide by the binary's format, not by `GOOS`: ELF with a machine that matches the target container architecture (D1).
2. An unusable default binary is not installed; the run continues with an actionable diagnostic naming `container.auto_provision.binary_path`. An unusable explicit `binary_path` fails with an error (D4, D5).
3. Image architecture comes from what the engine already reports; when unknown, compare with the host architecture only on a Linux host, stating the assumption here. Nothing extra is pulled or run (D2).
4. Docs: `options.mdx` and `config-schema.mdx` and their pt-BR copies say what happens on macOS and Windows hosts and how to set `binary_path`; example images are `debian:stable-slim` only.
5. Evidence is simulated on Linux unless CI shows more; no VM is started.
6. Run `formal_mutation.py symbols` on what is touched in `internal/provision` and `internal/container`.

## Open Questions

- Whether a follow-up should ask the daemon for its own architecture when the image is not local (D2 alternative), which would close the first-run gap on Docker Desktop and remote daemons.
