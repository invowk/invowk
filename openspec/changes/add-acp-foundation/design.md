## Context

Invowk currently has LLM-aware workflows that are intentionally batch-shaped:

- `invowk audit --llm` asks a model to analyze bounded audit context and turns responses into findings.
- `invowk agent cmd create` asks a model for one validated CUE command object and may patch `invowkfile.cue`.
- `internal/auditllm` supports OpenAI-compatible HTTP APIs, local Ollama-style servers, and CLI fallback through Claude Code, Codex CLI, and Gemini CLI.

Those paths are good fits for one-shot completion, model preflight checks, structured output, validation feedback, and audit concurrency. ACP solves a different problem: long-lived agent sessions with protocol negotiation, streaming updates, permission prompts, file mediation, terminal mediation, and editor-like agent interoperability.

This change adds only the ACP foundation. It does not route any existing command through ACP and does not expose ACP configuration to users yet.

## Goals / Non-Goals

**Goals:**

- Add a small internal ACP client-side foundation using `github.com/coder/acp-go-sdk`.
- Isolate ACP-specific types and lifecycle handling behind an Invowk-owned package boundary.
- Support fake-agent tests for initialize, session creation, prompt dispatch, streamed updates, cancellation, and policy callbacks.
- Provide explicit policy interfaces for permission, filesystem, and terminal callbacks so future features can choose their safety model deliberately.
- Keep existing LLM workflows behaviorally unchanged.

**Non-Goals:**

- Do not replace `internal/auditllm` or the direct OpenAI-compatible API path.
- Do not move `invowk audit --llm` or `invowk agent cmd create` to ACP.
- Do not auto-detect ACP adapters for Claude Code, Codex, Gemini, or any other tool.
- Do not add user-facing CLI flags, config schema fields, CUE schema fields, generated docs, or website snippets.
- Do not implement an Invowk ACP agent server.

## Decisions

### Decision: Add an internal ACP client package, not a user-facing provider

Introduce an internal package such as `internal/acpclient` that owns ACP process/session orchestration. Future user-facing features can depend on this package, but no current command will call it.

Alternatives considered:

- Add `acp` as a new `llm.provider` value. Rejected because it would imply current one-shot workflows should use ACP immediately and would require user-facing configuration semantics before there is a feature consuming sessions.
- Replace the CLI fallback completer with ACP. Rejected because current CLI fallback is a batch completion adapter, while ACP is a stateful session protocol.

### Decision: Use `github.com/coder/acp-go-sdk`

Use `coder/acp-go-sdk` as the ACP dependency. It provides tagged releases, generated protocol types, client and agent APIs, stdio examples, and examples for Claude Code and Gemini-style ACP integration.

Alternatives considered:

- Use `github.com/ironpark/go-acp`. Rejected for this foundation because it has no semver tags surfaced through the Go module/version checks used during research, lower adoption, and a less stable dependency story despite useful ergonomics.
- Hand-roll ACP JSON-RPC. Rejected because ACP is still evolving and generated protocol types plus conformance-shaped tests reduce drift risk.

### Decision: Keep ACP SDK types at the adapter edge

The ACP package should translate SDK notifications, prompt responses, permissions, and file requests into Invowk-owned internal types or interfaces at its boundary. Other application packages should not need to import `github.com/coder/acp-go-sdk` unless they are deliberately joining the ACP adapter layer.

Alternatives considered:

- Let future features use `acp-go-sdk` directly. Rejected because it would spread protocol churn and make it harder to enforce Invowk policy boundaries.

### Decision: Policy callbacks are explicit and restrictive

ACP agents can request filesystem writes, terminal operations, and permissions. The foundation should require a caller-supplied policy object for these callbacks. The default test/noop policy should deny writes, decline permissions, and report unsupported terminal operations unless a future feature provides a stronger policy.

Alternatives considered:

- Auto-approve permissions for ease of testing. Rejected because it would create dangerous precedent for future agent features.
- Let the ACP agent read/write the host filesystem directly. Rejected because Invowk should mediate these operations with the same care it applies elsewhere.

### Decision: Tests use fake ACP peers only

Add tests that start an in-process or test-binary ACP peer over stdio/pipes. Tests must not require real Claude Code, Codex, Gemini, network access, OAuth state, or API keys.

Alternatives considered:

- Smoke-test real external ACP adapters. Rejected for CI determinism and because adapter compatibility belongs in future integration tests once there is a user-facing feature.

## Risks / Trade-offs

- [Risk] A dormant package can rot before the first feature uses it. -> Mitigation: include fake-peer lifecycle tests and keep the package small enough that follow-up features can reshape it.
- [Risk] ACP protocol churn can break the foundation. -> Mitigation: pin the SDK version and keep SDK types isolated in one internal package.
- [Risk] Users may expect ACP support immediately once the dependency lands. -> Mitigation: avoid CLI/config/docs claims until a user-facing feature is proposed.
- [Risk] The foundation could duplicate current LLM completion abstractions. -> Mitigation: keep one-shot completion untouched and name the ACP layer around sessions, not completions.
- [Risk] Permission and filesystem callbacks can become a security boundary by accident. -> Mitigation: default restrictive behavior and require future features to define policy explicitly.

## Migration Plan

This change has no runtime migration. It adds an internal package and dependency only.

Rollback is a normal code revert: remove the package, tests, and module dependency. Since no command uses the foundation, rollback should not affect user workflows.

## Open Questions

- Should the eventual user-facing feature be an interactive `invowk agent session` command, a TUI integration, or a backend for richer `agent cmd` authoring?
- Should future ACP-backed features support only explicit adapter commands, or should Invowk eventually auto-detect known ACP adapters?
- What is the first real permission policy: read-only analysis, write-with-confirmation, or feature-specific allowlists?
