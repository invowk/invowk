# Container Retries And Testing Reference

Use this reference when changing container exit handling, transient retries,
dependency probes, integration tests, or testscript container setup.

## Contents

- [Exit-Code Contract](#exit-code-contract)
- [Transient Classification](#transient-classification)
- [Retry Policies](#retry-policies)
- [Stderr Buffering](#stderr-buffering)
- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [testscript Setup](#testscript-setup)
- [Focused Commands](#focused-commands)

## Exit-Code Contract

Engine CLI adapters absorb `exec.ExitError` into `RunResult.ExitCode` and return
the result without treating an ordinary child-process exit as an infrastructure
error. Callers must inspect all applicable channels:

1. the returned `error` for invocation/infrastructure failures;
2. `RunResult.Error` when the adapter records a non-exit failure; and
3. `RunResult.ExitCode` for child and engine exit status.

This is especially important for exit codes 125 and 126, which may represent
transient engine/runtime failures after the Go error has been absorbed.

## Transient Classification

`internal/container.IsTransientError()` centralizes retryable error text and
engine failures. Current families include:

- engine exit code 125;
- `ping_group_range` and generic OCI runtime failures;
- temporary DNS, timeout, refused-connection failures; and
- transient overlay/layer mount failures.

`context.Canceled` and `context.DeadlineExceeded` are never transient.

Keep textual classification narrow and case behavior tested. Domain-specific
messages such as "tool not found" must not mask an exhausted engine exit 125 or
126.

## Retry Policies

Image build/provisioning retries up to three attempts with 2s then 4s backoff.
Container runs retry up to five attempts with 1s, 2s, 4s, then 8s backoff.
The caller's context bounds the overall operation.

`runWithRetry()` checks both the returned error and absorbed exit code. The
container dependency adapter uses its transient-exit guard before interpreting
probe output; otherwise an engine failure can be misreported as a missing tool,
capability, or environment variable.

## Stderr Buffering

Non-interactive retries buffer stderr separately for each attempt:

- discard an attempt's buffer only when the failure is classified transient and
  another retry will occur;
- flush the final attempt on success, non-transient failure, cancellation, or
  retry exhaustion; and
- do not share buffers across attempts or concurrent executions.

Interactive execution uses a PTY and does not use buffered run retries. It may
still require Podman serialization from the engine lifecycle reference.

## Unit Tests

Use a new `MockCommandRecorder` and engine for every parallel test/subtest.
Inject through engine options. Never share a recorder that is reset by parallel
subtests.

Cover at least:

- argument construction and engine-specific transforms;
- absorbed child exit status;
- returned infrastructure errors;
- transient exit 125/126 retry and exhaustion;
- cancellation during command/backoff;
- final-attempt stderr behavior; and
- cleanup/lease release on every return path.

## Integration Tests

Real-engine tests must:

```go
if testing.Short() {
    t.Skip("skipping integration test in short mode")
}
testutil.AcquireContainerSemaphore(t)
ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
```

Use `debian:stable-slim`, not Alpine. Keep engine availability/health checks
separate from behavior assertions so unavailable infrastructure is reported
clearly.

## testscript Setup

Container txtar suites use `containerSetup` in
`tests/cli/cmd_container_test.go`. It composes common setup, assigns a dedicated
temporary `HOME`, writes test-scoped engine configuration, probes engine health,
and registers orphan cleanup with `env.Defer()`.

Preserve all timeout layers:

1. testscript deadline;
2. per-operation container context;
3. deferred orphan cleanup; and
4. outer CI timeout as catastrophic-failure protection.

## Focused Commands

```bash
go test ./internal/container -run 'Transient|Run|Podman|Docker'
go test ./internal/runtime -run 'Container.*Retry|RunWithRetry|Persistent'
go test ./internal/app/commandadapters -run 'Container|Dependency'
go test ./tests/cli -run 'Container'
```
