# Retries and Flake Tracking in Integration Tests

> Brainstorming analysis — captures design rationale for invowk's test resilience strategy.

## The Question

**Are retries and backoff in integration/container tests good practice, or do they mask flaky tests?**

This is a genuine tension in software engineering. Retries can legitimately handle environmental non-determinism (container engine races, network blips, filesystem contention), but they can also suppress real bugs by making failing tests "eventually pass." The answer depends on *what* is retried, *why*, and *where* the retry boundary sits.

## What Invowk Does Today

Invowk uses a **5-layer defense system** for container test reliability. Each layer addresses a different failure mode, and they compose vertically:

### Layer 1: Sysctl Prevention (`CONTAINERS_CONF_OVERRIDE`)

**Location:** `internal/runtime/container_exec.go`, `internal/container/podman_engine.go`

The root cause of the most common Podman failure — the `ping_group_range` race condition where crun and the kernel disagree on unprivileged ping permissions — is eliminated *before* any container operation. A `containers.conf` override pins the sysctl to a known-good value.

- This is **prevention**, not retry. It removes the failure mode entirely.
- Only applies to Podman (Docker doesn't have this issue).
- Detected and skipped on `podman-remote` (where the conf doesn't reach the service).

### Layer 2: Cross-Process Serialization (flock)

**Location:** `internal/runtime/run_lock_linux.go`, `internal/runtime/run_lock_other.go`

When `podman-remote` is detected (where Layer 1 can't help), a file lock (`flock` on Linux, `sync.Mutex` fallback elsewhere) serializes container operations. This prevents concurrent Podman invocations from racing over shared storage.

- Addresses resource contention, not test logic.
- Orthogonal to retry — ensures operations don't overlap.

### Layer 3: Production-Code Retry (`runWithRetry`, `ensureImage`)

**Location:** `internal/runtime/container_exec.go`, `internal/runtime/container_provision.go`

The runtime itself retries transient container failures:

- **`ensureImage()`** retries container image builds (3 attempts, 2s base backoff).
- **`runWithRetry()`** retries container runs (5 attempts, 1s base backoff).

Crucially, these retries use **classification**: `container.IsTransientError()` checks for known-transient patterns (OCI runtime errors, network failures, storage layer issues). Context cancellation is explicitly non-transient and breaks the loop immediately.

The **litmus test** passes here: these retries exist because they're *production features*. A user running `invowk cmd build` in a container should not fail because Podman had a momentary storage hiccup. The retry would exist even if invowk had zero tests.

### Layer 4: Test-Level Semaphore (`ContainerSemaphore`)

**Location:** `internal/testutil/container_semaphore.go`

A process-wide semaphore limits concurrent container operations to `min(GOMAXPROCS, 2)`. This prevents Podman resource exhaustion hangs on CI, where many parallel test goroutines would otherwise overwhelm the container engine.

- Addresses resource limits, not flakiness.
- Override via `INVOWK_TEST_CONTAINER_PARALLEL` env var.

### Layer 5: CI Environment Setup

**Location:** `.github/workflows/ci.yml`

- **Pre-pull** `debian:stable-slim` to remove image-pull latency from the critical path.
- **Engine masking**: `mv /usr/bin/docker /usr/bin/docker.disabled` ensures `AutoDetectEngine()` picks the correct engine on Ubuntu runners (which have both installed).
- **`INVOWK_TEST_CONTAINER_PARALLEL: "2"`** limits concurrency to match CI runner resources.

### Summary

```
Layer 1: Prevent (sysctl override — eliminate root cause)
Layer 2: Serialize (flock — prevent resource contention)
Layer 3: Retry (classified — production feature, not test artifact)
Layer 4: Throttle (semaphore — prevent resource exhaustion)
Layer 5: Stabilize (CI env — reduce environmental variance)
```

The layers are ordered by specificity: prevent > serialize > retry > throttle > stabilize. Retry (Layer 3) is only reached after prevention and serialization have already eliminated the most common failure modes.

## Industry Perspective

### Arguments FOR Retries in Integration Tests

1. **Environmental non-determinism is real.** Container engines, network stacks, and filesystem drivers have genuine transient failure modes that are outside the test's control. Retrying handles these without adding complexity to production code.

2. **Production resilience validation.** If production code retries (as invowk's does), tests that exercise those code paths validate the retry behavior. Not retrying in tests means you never test your retry logic.

3. **The math is favorable.** A test with a 1% transient failure rate, run in a 100-test suite, gives a ~63% chance of at least one failure per run. With a single retry, the per-test failure rate drops to 0.01%, making suite-level failure extremely rare.

4. **CI cost vs. developer time.** Re-running a 15-minute CI pipeline because of a transient Podman error wastes more compute (and developer attention) than a 5-second retry within the test.

### Arguments AGAINST Retries in Integration Tests

1. **Masking real bugs.** A retry that makes a flaky test "eventually pass" hides the underlying issue. The test signals "pass" but the bug persists, eroding confidence in the test suite.

2. **Cost amplification.** Google's internal analysis found that flaky tests cost ~170% more in compute than reliable tests (re-runs, investigation time, quarantine management). Retries are part of that cost.

3. **Signal erosion.** If tests routinely need retries to pass, developers stop trusting failures. "It's probably flaky" becomes the default response, and real regressions get dismissed.

4. **Kubernetes dropped `flakeAttempts`.** The Kubernetes project experimented with `flakeAttempts` in their test framework and ultimately removed it, arguing that it masked real issues and discouraged fixing flaky tests.

## What Major Projects Do

| Project | Approach | Details |
|---------|----------|---------|
| **Kubernetes** | Dropped `flakeAttempts`; uses external flake tracking | Removed retry from test framework. Uses `kind` for deterministic environments. Flake dashboard tracks per-test failure rates. |
| **Podman** | Retry in CI, not in tests | Podman's own CI retries jobs (via `ginkgo --flake-attempts`), not individual test assertions. Tests are expected to be deterministic. |
| **Docker/Moby** | Uses `gotestsum --rerun-fails` | Reruns failing tests at the CI level after the full suite completes. Clear separation: test code has no retry, CI handles transient infra failures. |
| **Google** | Extensive flake tracking + quarantine | Internal "Test Flakiness Dashboard" tracks per-test failure rates. Tests exceeding thresholds are auto-quarantined. Retries happen at the infrastructure level, not in test code. |
| **GitLab** | `retry: 2` in CI, flaky test annotation | CI-level retry for transient infra. Tests marked `quarantine: true` run in separate pipeline. |
| **Go ecosystem** | `gotestsum --rerun-fails` is de facto | HashiCorp, Docker, and others use gotestsum for CI-level retry. `go test` itself has no retry mechanism. |
| **pytest** | `pytest-rerunfailures` plugin | Widely used in Python ecosystem. Reruns failed tests with configurable count and delay. |

### Key Takeaway

The industry consensus is:
1. **Test code should not retry.** Tests should be deterministic and fail clearly.
2. **Production code can (and should) retry** when handling known-transient external failures.
3. **CI-level retry** (gotestsum, ginkgo flake-attempts, CI job retry) is acceptable for environmental non-determinism.
4. **Flake tracking** is essential regardless — you need to know which tests are unreliable.

## The Litmus Test

> "Would the same retry exist in production code?"

This is the clearest decision boundary:

- **YES:** The retry is a feature. `runWithRetry()` exists because users shouldn't see transient Podman errors. It would exist even without tests. **Keep it.**
- **NO:** The retry exists only to make tests pass. It masks a real issue. **Remove it and fix the root cause instead.**

Invowk's retries pass this test:
- `runWithRetry()` → production feature (handles transient container engine errors for users)
- `ensureImage()` retries → production feature (handles transient build failures)
- `ContainerSemaphore` → resource management, not retry
- Sysctl override → prevention, not retry

## Verdict for Invowk

Invowk's approach is **well-structured and defensible**:

1. The retry code (`runWithRetry`, `ensureImage`) is **production code**, not test infrastructure. It passes the litmus test.
2. The other layers (sysctl, flock, semaphore) are **prevention and resource management**, not retry.
3. Test code itself has **zero retries**. Tests call production APIs and either pass or fail.
4. The layered defense follows the correct priority: **prevent > serialize > retry > throttle**.

The gap is not in the retry strategy — it's in **CI-level resilience and observability**:
- No CI-level test retry for transient infrastructure failures (outside production code's domain)
- No flake tracking or visibility into which tests need reruns
- No JUnit-style test reporting for CI

## Improvement Opportunities

### 1. CI-Level Retry via `gotestsum`

Replace raw `go test` in CI with `gotestsum --rerun-fails`:
- Re-runs only failing tests after the full suite completes.
- `--rerun-fails-max-failures 5` skips reruns if too many tests fail (real regression, not flakiness).
- Zero changes to test code — transparent wrapper around `go test`.
- Produces JUnit XML for test result visualization in GitHub Actions.

### 2. Flake Tracking via Rerun Reports

`gotestsum --rerun-fails-report rerun-report.txt` logs which tests needed reruns:
- **Short-term:** Uploaded as CI artifacts (30-day retention). Manual inspection.
- **Medium-term:** Scheduled workflow aggregates reports, creates GitHub issues for repeatedly-flaky tests.
- **Long-term:** If needed, integrate with Datadog CI Visibility or BuildPulse for dashboards.

### 3. JUnit PR Annotations

`mikepenz/action-junit-report` renders test results as GitHub Check annotations:
- Failed tests appear inline on the PR "Files changed" tab.
- Passes are hidden (noise reduction).
- Gives immediate visibility without clicking into CI logs.

### 4. Quarantine (Future)

If flake tracking reveals persistently-flaky tests:
- Tag them with a build constraint or test skip condition.
- Run them in a separate CI job that doesn't block merges.
- Fix and un-quarantine as root causes are addressed.

## Sources

- [Kubernetes: Deprecating flakeAttempts](https://github.com/kubernetes/kubernetes/issues/105435) — Discussion on removing retry from the test framework.
- [Google Testing Blog: Flaky Tests at Google](https://testing.googleblog.com/2016/05/flaky-tests-at-google-and-how-we.html) — Internal analysis of flaky test costs and mitigation strategies.
- [Podman CI: ginkgo flake-attempts](https://github.com/containers/podman/blob/main/.cirrus.yml) — Podman's CI configuration with retry at the job level.
- [gotestsum: GitHub repository](https://github.com/gotestyourself/gotestsum) — De facto Go test runner with `--rerun-fails` support.
- [Datadog: CI Visibility and Flaky Test Detection](https://docs.datadoghq.com/continuous_integration/guides/flaky_test_management/) — Enterprise flake tracking and quarantine.
- [Azure DevOps: Flaky Test Management](https://learn.microsoft.com/en-us/azure/devops/pipelines/test/flaky-test-management) — Automatic flaky test detection and quarantine.
- [pytest-rerunfailures](https://github.com/pytest-dev/pytest-rerunfailures) — Python ecosystem equivalent of gotestsum's rerun-fails.
- [mikepenz/action-junit-report](https://github.com/mikepenz/action-junit-report) — GitHub Action for JUnit XML test reporting.
- [GitLab: Flaky Tests](https://docs.gitlab.com/ee/development/testing_guide/flaky_tests.html) — GitLab's approach to flake tracking and quarantine.
