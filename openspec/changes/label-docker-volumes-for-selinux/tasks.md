## 1. Engine

- [x] 1.1 Add the SELinux volume formatter to `NewDockerEngine`, backed by a cached `docker info` probe
- [x] 1.2 Add `NewDockerEngineWithSELinuxCheck` for tests
- [x] 1.3 Parse security options as JSON and match the exact `name=selinux` field

## 2. Test harness

- [x] 2.1 Forward engine connection variables into the CLI container testscript environment, with a unit test

## 3. Tests

- [x] 3.1 Table tests for security-option parsing, including substrings, null, and daemon errors
- [x] 3.2 Docker volume labeling tests mirroring the Podman table
- [x] 3.3 Probe tests: exact `docker info` arguments, failing daemon, missing binary
- [x] 3.4 Real-daemon check on a Fedora Silverblue SELinux host: the four previously failing `internal/runtime` container tests and all 23 `TestContainerCLI` tests pass

## 4. Documentation

- [x] 4.1 Update `docs/architecture/c4-component-container.md`, the website page, and the pt-BR translation
- [x] 4.2 Update `docs/diagrams/c4/component-container.d2` and re-render with TALA
- [x] 4.3 Build the website

## 5. Verification

- [x] 5.1 `make lint`
- [x] 5.2 `make test`, `make check-baseline`, `make check-file-length`, `make license-check`. All 8442 unit and integration tests pass, including the container tests. `make test-cli` inside `make test` needs `-race`, which this machine cannot build without gcc; the CLI suite passed without `-race`. One new goplint finding received an exception following the raw-exec-output precedent
