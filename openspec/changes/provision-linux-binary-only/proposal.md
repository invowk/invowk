## Why

Container auto-provision copies the invowk binary into every provisioned image and puts it on `PATH`, but it never checks that the binary can run there. By default it takes the running executable (`os.Executable()`). On a macOS host that is a Mach-O file, on a Windows host a PE file, and on a Linux host whose architecture differs from the image's it is an ELF file for the wrong machine. In all three cases the container run itself works, but every nested `invowk` call inside the container (in-container `invowk cmd` dependencies, `invowk tui *` under `-i`, Docker Desktop row 6) fails with an exec-format error. A wrong explicit `container.auto_provision.binary_path` fails the same way. Tracker N12; found by reading code, never run on those hosts.

## What Changes

- Before it installs a binary, auto-provision reads the candidate file's header and accepts only a Linux ELF executable whose machine matches the container architecture. The decision is made from the file format, not from the host OS, so it also covers an explicit `binary_path` and cross-architecture Linux hosts.
- The container architecture comes from what the engine already reports: the base image's recorded architecture, through a read-only image inspect. When the engine cannot say (for example, the image is not local yet), the host architecture is assumed only on a Linux host. On other hosts the architecture is not checked. Nothing is pulled or run to find out.
- **Default binary not usable** (no `binary_path` set): the binary is not installed. Provisioning still adds modules, the command runs, and a warning names `container.auto_provision.binary_path` and the architecture it needs. `strict` does not change this, because the run still gets a provisioned image.
- **BREAKING (minor): explicit `binary_path` not usable** (missing, unreadable, not a Linux ELF executable, or for another architecture): the command fails with an error that names the path and the reason, whatever `strict` says. Until now, a missing explicit path in the default non-strict mode only warned and ran the bare base image.
- When the host architecture was assumed and an explicit `binary_path` fails only because its architecture differs, the error says the architecture was assumed and tells the user to pull or build the image first, so a correct cross-architecture binary is not rejected for good.
- The warning appears on every run that prepares a provisioned image without the binary, including runs that reuse a cached provisioned image. A run that reuses an existing persistent container prepares no image and shows no warning; the run that created the container showed it. A provisioned image built without the binary is cached separately from one built with it.
- The docs stop saying only "uses the running invowk binary": they explain what happens on macOS, Windows and cross-architecture hosts, and how to set `binary_path` to a Linux build.

## Migration notes

If `container.auto_provision.binary_path` points at a file that does not exist, is not a Linux executable, or was built for another CPU architecture than the container image, container commands now stop with an error instead of warning and running without invowk resources. Point `binary_path` at a Linux build of invowk for the image's architecture (for example the `linux_arm64` release archive for an arm64 image), or remove the setting. Runs that do not set `binary_path` keep working: on macOS and Windows hosts they now print a warning and run without the in-container binary, where before they installed a binary that could not execute. A managed persistent container created by an earlier version on a macOS or Windows host gets a new provisioned image identity, so its first run after the upgrade fails with "was created by invowk with a different runtime configuration; remove or rename it": remove the container and run again. Upgrading invowk already changes that identity, because the binary's hash is part of it; adding or removing `binary_path` later does the same. On a Linux host whose engine does not have the image yet, invowk assumes the host architecture; if the image is for another platform and `binary_path` names a build for it, the first run fails with an error saying so: pull the image (or build its Containerfile) and run again.

## Capabilities

### New Capabilities

- `container-auto-provision`: what auto-provision installs into a container image, and when it refuses or skips the invowk binary.

### Modified Capabilities

(none)

## Impact

- **Code:** `internal/provision/` (binary check, binary resolution shared by `Provision` and `GetProvisionedImageTag`, cache key, Dockerfile and env vars), `internal/container/` (read-only image architecture query through the existing query runner), `internal/runtime/container_provision.go` (keep the running-executable default apart from the configured path; an unusable explicit binary bypasses the non-strict fallback). Formal-bound symbols in these areas (`CopyDir`, `ContainerRuntime.getContainerWorkDir`) are not changed.
- **Behaviour:** Linux hosts whose image architecture matches see no change. macOS and Windows hosts get a warning and no in-container binary unless `binary_path` is set. Misconfigured explicit `binary_path` values now fail the run.
- **Docs:** `website/docs/configuration/options.mdx`, `website/docs/reference/config-schema.mdx`, the auto-provisioning section of `website/docs/runtime-modes/container.mdx`, their pt-BR copies, the README config sample and the `binary_path` comment in `internal/config/config_schema.cue` (comment only, no schema change).
- **Evidence:** simulated on Linux with fixture headers (Mach-O, PE, ELF of another machine, non-Linux ELF, truncated). No macOS or Windows host run.
