## 1. Dependency and Package Foundation

- [x] 1.1 Add pinned `github.com/coder/acp-go-sdk` dependency to `go.mod` and `go.sum`.
- [x] 1.2 Create an internal ACP foundation package, such as `internal/acpclient`, with package documentation that states it is dormant and not wired to current CLI flows.
- [x] 1.3 Define Invowk-owned request, response, event, lifecycle, and policy types so callers outside the ACP foundation do not need to import ACP SDK types.

## 2. ACP Session Lifecycle

- [x] 2.1 Implement explicit ACP process/session startup from caller-supplied argv and working directory.
- [x] 2.2 Implement ACP initialization and capability negotiation through the SDK client-side connection.
- [x] 2.3 Implement session creation, prompt dispatch, streamed update collection, final stop-reason return, and graceful close.
- [x] 2.4 Implement prompt cancellation forwarding and process cleanup for canceled or failed sessions.
- [x] 2.5 Normalize ACP startup, negotiation, prompt, cancellation, and process-exit failures into actionable internal errors.

## 3. Policy Callback Boundary

- [x] 3.1 Define explicit permission, filesystem, and terminal policy interfaces for ACP callbacks.
- [x] 3.2 Route ACP permission requests through the configured permission policy and return selected outcomes to the agent.
- [x] 3.3 Route ACP text-file read/write requests through the configured filesystem policy without direct filesystem access in the foundation.
- [x] 3.4 Return explicit unsupported-operation errors for terminal callbacks when no terminal policy is configured.
- [x] 3.5 Provide a restrictive noop policy for tests and future callers that need deny-by-default behavior.

## 4. Dormancy and Isolation Guarantees

- [x] 4.1 Verify no existing command path, including `invowk audit --llm` and `invowk agent cmd create`, imports or calls the ACP foundation.
- [x] 4.2 Verify no CLI flags, config schema fields, CUE schema fields, generated references, website snippets, or README user-facing ACP claims are added by this change.
- [x] 4.3 Add an import-boundary check or focused test that fails if `github.com/coder/acp-go-sdk` is imported outside the ACP foundation package and its tests.

## 5. Tests and Verification

- [x] 5.1 Add fake ACP peer tests for initialize, session creation, prompt dispatch, streamed update collection, and final stop reason.
- [x] 5.2 Add fake ACP peer tests for cancellation forwarding and process cleanup.
- [x] 5.3 Add policy callback tests for permission delegation, filesystem delegation, and unsupported terminal behavior.
- [x] 5.4 Run targeted ACP foundation tests.
- [x] 5.5 Run `go test ./internal/acpclient ./internal/auditllm ./internal/agentcmd ./cmd/invowk`.
- [x] 5.6 Run `make check-baseline`.
- [x] 5.7 Run `openspec validate add-acp-foundation --strict`.
