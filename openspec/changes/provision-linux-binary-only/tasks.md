# Tasks

## 1. Binary format check (`internal/provision`)

- [ ] 1.1 Add the architecture table and the header check of design D1 in a new file (for example `internal/provision/binary_format.go`, with the MPL-2.0 SPDX header): open the file, `debug/elf.NewFile`, accept only `ELFOSABI_NONE`/`ELFOSABI_LINUX`, `ET_EXEC`/`ET_DYN`, and class, byte order and machine from the table; an expected architecture of "unknown" skips only the machine comparison. Return a typed reason (Mach-O and PE named from their magic). Verify with `go build ./internal/provision/`.
- [ ] 1.2 Add a fixture helper and table tests for every scenario of the "Only a Linux binary for the container architecture is installed" requirement: Mach-O, PE, ELF amd64 against amd64 and arm64, FreeBSD OS ABI, `ET_REL`, header-only ELF without section headers, truncated ELF, empty file, shell script, missing file, and an unknown expected architecture. Verify with `go test -run 'Binary' ./internal/provision/`.

## 2. Image architecture query (`internal/container`)

- [ ] 2.1 Add a read-only query that runs `image inspect --format '{{.Architecture}}' <image>` through `EngineQueryRunner` and returns the trimmed architecture, or "unknown" on an error or empty output (design D2). Do not touch `ImageExists`, the engines' build and run arguments, or any pull. Verify with `go build ./internal/container/`.
- [ ] 2.2 Unit-test it with a fake `EngineQueryRunner`: exact arguments for Docker and Podman, `amd64`/`arm64` output with surrounding whitespace, empty output, and a query error. Verify with `go test -run 'ImageArchitecture' ./internal/container/`.

## 3. Binary resolution in the provisioner

- [ ] 3.1 Add `DefaultInvowkBinaryPath` to `provision.Config` (validated like `InvowkBinaryPath`; update the field comments and `Validate`'s doc comment) and keep `InvowkBinaryPath` for the configured path only (design D3). Verify with `go test -run 'Config' ./internal/provision/`.
- [ ] 3.2 Add the per-request resolution of design D4 to `LayerProvisioner`: the configured path wins over the default; expected architecture from the engine query (by interface assertion), else `runtime.GOARCH` on a Linux host, else unknown; host OS/architecture and the query as internal test hooks. Configured and unusable → `*UnusableInvowkBinaryError` wrapping `ErrUnusableInvowkBinary`; default and unusable → no binary plus the D5 warning text. Verify with `go build ./internal/provision/`.
- [ ] 3.3 Make `calculateCacheKey`, `prepareBuildContext`, `generateDockerfile` and `buildEnvVars` take the resolved path instead of reading `p.config.InvowkBinaryPath`; call the resolution from `Provision` and `GetProvisionedImageTag`; attach the warning on the cache-hit return too (design D4, D6). Do not edit `CopyDir`. Verify with `go test ./internal/provision/`.
- [ ] 3.4 Move existing provisioner tests that use fake binaries (`fake-binary-content`, `binary-v1`/`binary-v2`, the shell stub in `provision_integration_test.go`) to the ELF fixtures of 1.2 for the host architecture, and add tests for: default skipped (no `COPY invowk`, no `ENV PATH`, no `PATH` env var, warning present on build and on cache hit), configured unusable (error, no build call), keys differ with and without the binary, and `GetProvisionedImageTag` equal to the built tag when the default is skipped. Verify with `go test ./internal/provision/` and, with an engine available, `go test -run 'Integration' ./internal/provision/`.
- [ ] 3.5 Run `python3 scripts/formal_mutation.py plan --out <scratch dir>` and then `python3 scripts/formal_mutation.py symbols --plan <scratch dir>/formal-plan.json` on every function changed in `internal/provision` and `internal/container`; confirm none is bound (only `CopyDir` and `ContainerRuntime.getContainerWorkDir` are, and neither changes) and record the output in the PR body.

## 4. Runtime wiring (`internal/runtime`)

- [ ] 4.1 Make `applyHostProvisionDefaults` fill `DefaultInvowkBinaryPath` from `os.Executable()` and `buildProvisionConfig` set `InvowkBinaryPath` only from `container.auto_provision.binary_path` (design D3). Do not edit `getContainerWorkDir`. Verify with `go test -run 'ProvisionConfig' ./internal/runtime/`.
- [ ] 4.2 In `ensureProvisionedImage`, return an error matching `provision.ErrUnusableInvowkBinary` before the non-strict fallback, whatever `strict` says (design D5). Verify with `go test -run 'EnsureProvisionedImage' ./internal/runtime/`.
- [ ] 4.3 Update the runtime tests: `TestBuildProvisionConfig*` for the two fields; the strict and non-strict tests at `internal/runtime/container_provision_test.go` force failure through the mocked build error instead of a missing binary; add an unusable explicit binary under `strict: false` (error, no fallback), a skipped default (run continues with one `container_provisioning_warning` naming `container.auto_provision.binary_path`), and a persistent container with an unusable explicit binary (`persistentSpecImage` fails). Verify with `go test -run 'Provision|Persistent' ./internal/runtime/`.
- [ ] 4.4 Confirm that `internal/app/commandadapters/interactive_exposure_integration_test.go` still passes with its freshly built host binary set as `InvowkBinaryPath`. Verify with `go test -run 'InteractiveExposure' ./internal/app/commandadapters/` where an engine is available, and say so if it skips.

## 5. Documentation

- [ ] 5.1 Rewrite `container.auto_provision.binary_path` in `website/docs/configuration/options.mdx` and `binary_path` in `website/docs/reference/config-schema.mdx`: the default is the running binary when it is a Linux executable for the image's architecture; on macOS and Windows hosts (and for other architectures) it is skipped with a warning and nested `invowk` calls do not work; an explicit path must be such a binary or the command fails, whatever `strict` says; how to set it to a Linux release build. Any example uses `debian:stable-slim`. Verify by reading the rendered pages after 5.4.
- [ ] 5.2 Update the "Auto-provisioning" section of `website/docs/runtime-modes/container.mdx` (it says the layer always includes the binary), the README config sample comment for `binary_path`, and the `binary_path` comment in `internal/config/config_schema.cue` (comment only). Verify with `go test -run Sync ./internal/config/`.
- [ ] 5.3 Mirror 5.1 and 5.2 in the pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/` (`configuration/options.mdx`, `reference/config-schema.mdx`, `runtime-modes/container.mdx`). Re-check `docs/architecture/c4-component-container.md` and `website/docs/architecture/c4-component-container.mdx` for a binary claim that no longer holds, and list the new image-architecture query there next to the network facts query. Verify the pages match by section.
- [ ] 5.4 Build the website. Verify with `cd website && npm run build`.

## 6. Completion gates (`.agents/rules/checklist.md`)

Run every locked command in the worktree under `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock"`, backgrounded with an `rc=` line and a Monitor wait (`.agents/rules/agent-workflows.md`).

- [ ] 6.1 `make tidy` and `make license-check` (new Go files).
- [ ] 6.2 `make lint`, fixing every finding.
- [ ] 6.3 `make test` (full suite, including the CLI and container tests).
- [ ] 6.4 `make check-baseline`, triaging every new goplint finding (no baseline or ignore for protocol inconclusives).
- [ ] 6.5 `make check-file-length` (`internal/provision/provisioner_test.go` is already at 863 lines: put new tests in new files).
- [ ] 6.6 `make sonar-local`: no new code-fixable issues (go:S8209, go:S8193, go:S1192 outside tests, go:S107, go:S8239).
- [ ] 6.7 `python3 scripts/formal.py correspondence` (code next to bound symbols changed).
- [ ] 6.8 `make check-agent-docs` if `.agents/` or `AGENTS.md` changed (for example the container skill's provisioning notes).
- [ ] 6.9 `/learn`.

## Workflow follow-up

- State in the PR and the result that macOS and Windows behaviour is simulated on Linux with fixture headers; no macOS or Windows host run.
- Tracker N12 and the row-6 release Warning wording (N5) are updated by the maintainer session, not by this change.
- Archive the change after the PR merges.
