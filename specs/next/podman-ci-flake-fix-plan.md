# Podman CI Flake Fix Plan (PR #43)

## Context

- PR: `https://github.com/invowk/invowk/pull/43`
- Failing check: `Test (ubuntu-24.04 / podman)`
- Failing run: `https://github.com/invowk/invowk/actions/runs/21975252649/job/63485482941`
- Failure signature:
  - `FAIL: testdata/container_exitcode.txtar:25: test timed out while running command`
  - `[context deadline exceeded]`
  - Timeout happens on `! exec invowk cmd fail-command` after `About to run a failing command`
  - Go stack in log shows the process blocked in `os/exec` pipe-copy path while waiting on container subprocess output.

Observed behavior across nearby runs:
- Same matrix job on earlier commits in the same PR passed.
- Failure is intermittent and Podman-specific (`ubuntu-24.04 / podman` only in this run).
- CI is expected to use local Podman (not `podman-remote`).

## Goals

1. Eliminate intermittent Podman container test hangs in CI.
2. Preserve overall CI throughput where possible.
3. Keep Docker path parallel and fast.
4. Address pre-existing test-harness weaknesses that increase flake surface area.
5. Leave a maintainable, explicit strategy for future container-test scaling.

## Non-Goals

- No broad runtime redesign.
- No migration to self-hosted runners in this change.
- No reduction of container test coverage.

## Key Risks To Control

1. Rootless Podman contention under many concurrent container runs/builds in one job.
2. Hidden hangs that are not classified as transient exit errors (`125`/`126`) and therefore bypass current retry logic.
3. Test cleanup logic that currently targets container names not guaranteed to be used by test executions.
4. Regressing Docker performance while hardening Podman.

## Proposed Strategy

Use a layered approach:

1. **Deterministic scheduling in test harness**
   - Keep Docker container tests parallel.
   - Run Podman container test scripts sequentially inside a single test process.
   - Rationale: remove high-contention path where rootless Podman + parallel testscript runs can deadlock/hang.

2. **CI-level containment**
   - For Podman matrix jobs only, run the container test suite in a dedicated serialized invocation.
   - Keep unit and non-container package tests parallel where safe.
   - Rationale: keeps throughput but ensures the problematic path is bounded.

3. **Pre-existing harness fix**
   - Rework or remove misleading cleanup path (`invowk-test-*`) that does not reliably map to created containers.
   - Replace with deterministic cleanup controls (bounded engine cleanup and unique identifiers that are actually used).

4. **Guardrails and observability**
   - Add explicit logging around selected engine and execution mode during container tests.
   - Add a stress command for local/CI validation (`count > 1` loop for Podman container suite).

## Detailed Change Plan

### Phase 1: Test Harness Hardening (`tests/cli`)

Files:
- `tests/cli/cmd_container_test.go`
- `tests/cli/cmd_test.go` (if engine detection helper reuse is needed)

Changes:
1. Determine container engine once for `TestContainerCLI` and expose boolean `isPodman`.
2. Branch execution policy:
   - `podman`: no `t.Parallel()` for per-file container subtests.
   - `docker`: keep existing per-file `t.Parallel()` behavior.
3. Keep per-script deadline, but ensure deadline value and behavior are logged for debugging.
4. Cleanup adjustments:
   - Either remove `cleanupTestContainers` if it is ineffective/noisy, or
   - Make it deterministic by tracking actual created resources (names/tags) and cleaning those only.
5. Maintain `ContinueOnError: true` unless debugging indicates it masks actionable diagnostics.

Acceptance criteria:
- `go test -run TestContainerCLI -v ./tests/cli` passes consistently on local Docker.
- Podman path behavior is explicitly serialized by code path.

### Phase 2: CI Workflow Hardening (`.github/workflows/ci.yml`)

File:
- `.github/workflows/ci.yml`

Changes:
1. Split Podman job test execution into:
   - non-container package tests (current broad run, parallel allowed),
   - container CLI tests (`./tests/cli`, `-run '^TestContainerCLI$'`) with serialized test parallelism for Podman.
2. Keep Docker jobs unchanged for speed.
3. Add explicit step output showing engine and selected container-test mode.
4. Preserve image pre-pull (`debian:stable-slim`) and engine masking logic.

Acceptance criteria:
- Podman jobs no longer run container CLI tests in high-concurrency mode.
- Docker jobs retain current throughput.

### Phase 3: Runtime Retry/Timeout Clarification (`internal/runtime`, optional in same PR)

Files (only if needed after Phase 1/2 validation):
- `internal/runtime/container_exec.go`
- `internal/container/transient.go`

Changes (optional):
1. Improve classification/documentation for hang-like conditions where process does not return transient exit code.
2. Add defensive, bounded retry behavior only when safe and clearly transient.
3. Keep behavior conservative to avoid masking genuine user script failures.

Acceptance criteria:
- No behavior regression in container exit-code semantics.
- Existing runtime tests pass unchanged or with targeted updates.

## Validation Plan

### Local

1. Baseline sanity:
   - `go test ./...`
2. Container suite:
   - `go test -run '^TestContainerCLI$' -count=1 -v ./tests/cli`
3. Stress check (repeat):
   - `go test -run '^TestContainerCLI$' -count=10 -v ./tests/cli`
4. Focus flake target:
   - `go test -run 'TestContainerCLI/container_exitcode/container_exitcode' -count=20 -v ./tests/cli`

### CI

1. Verify Podman matrix jobs on both:
   - `ubuntu-24.04 / podman`
   - `ubuntu-latest / podman`
2. Confirm Docker matrix remains parallel and green.
3. Watch for new long-tail timeouts or coverage artifact regressions.

## Rollout Plan

1. Land harness + workflow changes together (single PR update).
2. Re-run full PR checks.
3. If stable for 3 consecutive CI runs, keep as default.
4. If instability persists:
   - Temporarily reduce Podman job scope to serialized container suite only,
   - Open follow-up for deeper Podman runtime isolation (state-dir isolation or shard strategy).

## Rollback Plan

If regressions appear:
1. Revert workflow split first (fast rollback path).
2. Keep harmless logging changes.
3. Re-assess with captured failing logs and targeted reproducer command.

## Open Questions

1. Should Podman container tests be sharded across multiple jobs (serial inside each shard) once stability is proven?
2. Should we add a nightly stress workflow (`count=20`) for early flake detection?
3. Do we want strict cleanup assertions for orphaned images/containers in CI post-steps?

## Deliverables Checklist

- [ ] Engine-aware container test scheduling in `tests/cli/cmd_container_test.go`
- [ ] Deterministic cleanup strategy (or removal of ineffective cleanup path)
- [ ] CI Podman serialized container-test step in `.github/workflows/ci.yml`
- [ ] Validation evidence captured from local + CI reruns
- [ ] Follow-up issue (if any optional runtime-layer changes are deferred)
