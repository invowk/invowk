## 1. Schema and Domain Types

- [x] 1.1 Add `persistent` container-runtime configuration types to `pkg/invowkfile.RuntimeConfig`, including `create_if_missing` and optional `name`
- [x] 1.2 Update `pkg/invowkfile/invowkfile_schema.cue` so `persistent` is accepted only for `name: "container"` runtimes
- [x] 1.3 Add validation tests for accepted container persistent config, rejected native/virtual persistent config, default `create_if_missing: false`, and invalid explicit names
- [x] 1.4 Add or update schema sync coverage so Go JSON tags and CUE fields remain aligned
- [x] 1.5 Add a strict portable container-name validation helper and deterministic derived-name helper with table tests for root and module command namespaces

## 2. CLI and Service Propagation

- [x] 2.1 Add `--ivk-container-name` to `invowk cmd` persistent flags and discovered leaf command execution
- [x] 2.2 Thread the optional container-name override through `ExecuteRequest`, command service `Request`, execution-context build options, and `runtime.ExecutionContext`
- [x] 2.3 Add CLI adapter and service tests proving the flag wins over runtime config and is available to runtime execution
- [x] 2.4 Extend dry-run plan data and rendering to show disposable versus persistent execution, effective target name, name source, and creation policy
- [x] 2.5 Add dry-run tests that prove no persistent lifecycle operations run during `--ivk-dry-run`

## 3. Container Engine Lifecycle

- [x] 3.1 Add container inspect, create, start, and promoted exec methods to the `container.Engine` interface with typed options/results and sentinel errors for missing containers and name conflicts
- [x] 3.2 Implement shared CLI argument builders and execution logic in `BaseCLIEngine` for inspect, create, start, and exec
- [x] 3.3 Implement Docker and Podman inspect parsing with fixture tests covering running, stopped, missing, labeled, and unlabeled containers
- [x] 3.4 Update `SandboxAwareEngine` to forward inspect, create, start, and exec through host-spawn wrapping
- [x] 3.5 Update mock engines and existing tests to satisfy the expanded engine contract
- [x] 3.6 Add unit tests for lifecycle command arguments, sandbox wrapping, exec environment/workdir handling, and error mapping

## 4. Runtime Lifecycle Implementation

- [x] 4.1 Add a persistent target resolver that applies CLI/config/derived name precedence and classifies name source
- [x] 4.2 Add managed-container labels and creation-time fingerprint generation for image, volumes, ports, extra hosts, idle command, and provisioning identity
- [x] 4.3 Split container preparation into creation-time data and exec-time data so dynamic env, temp scripts, stdio, and host callback tokens are exec-only
- [x] 4.4 Implement managed missing-target creation with inspect-before-create, create, start, and exec
- [x] 4.5 Implement reuse of matching managed containers, including start-before-exec for stopped managed targets
- [x] 4.6 Implement external CLI target behavior for running, stopped, and missing containers without adding labels or starting stopped external containers
- [x] 4.7 Implement drift detection for managed containers and return actionable errors without mutation or cleanup
- [x] 4.8 Add per-engine/per-container lifecycle locking and name-conflict re-inspection for concurrent first use
- [x] 4.9 Ensure `Execute` and `ExecuteCapture` both use the persistent target selection rules
- [x] 4.10 Ensure `--ivk-force-rebuild` affects only missing-target creation and does not rebuild or replace existing persistent containers
- [x] 4.11 Ensure persistent creation does not clean up provisioned images that managed persistent containers depend on

## 5. Automated Tests

- [x] 5.1 Add runtime unit tests for disposable default behavior, managed creation, managed reuse, managed stopped start, unmanaged config conflict, external override success, external stopped failure, missing target failure, and drift failure
- [x] 5.2 Add tests proving per-run env, `INVOWK_SSH_*`, flag env, env-file values, temp scripts, workdir, and stdio are passed only to exec for persistent targets
- [x] 5.3 Add concurrency tests for two invocations targeting the same missing managed container
- [x] 5.4 Add testscript coverage for invowkfile schema behavior, CLI override plumbing, dry-run output, invalid names, and user-facing error messages
- [x] 5.5 Add live Docker/Podman integration coverage gated behind the existing container-test infrastructure for create-if-missing, state preservation, external pre-created target, managed restart, and cleanup by Invowk labels
- [x] 5.6 Add dependency-validation coverage proving capture execution uses the same persistent target as normal command execution

## 6. Documentation

- [x] 6.1 Update container runtime docs to explain disposable default behavior, persistent opt-in behavior, managed versus external targets, and statefulness
- [x] 6.2 Update invowkfile reference docs for `persistent.create_if_missing`, `persistent.name`, derived names, validation rules, and examples using `debian:stable-slim`
- [x] 6.3 Update CLI reference docs for `--ivk-container-name`, precedence, external target expectations, and missing/stopped target behavior
- [x] 6.4 Update architecture documentation or diagrams if container runtime lifecycle diagrams currently imply only disposable `run --rm`
- [x] 6.5 Update snippets or samples that need to demonstrate persistent containers without changing default examples away from disposable execution

## 7. Verification

- [x] 7.1 Run targeted Go tests for `pkg/invowkfile`, `internal/container`, `internal/runtime`, `internal/app/commandsvc`, and `cmd/invowk`
- [x] 7.2 Run relevant CLI testscript suites, including container-specific tests when a supported engine is available
- [x] 7.3 Run schema sync checks after CUE and Go struct changes
- [x] 7.4 Run documentation validation for changed docs and snippets
- [x] 7.5 Run the repository pre-completion checks required by `.agents/rules/checklist.md`
