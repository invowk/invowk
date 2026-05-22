## Why

Invowk's current LLM integration is efficient for one-shot completion workflows, but it does not provide a protocol-level foundation for future stateful agent sessions, streamed updates, permission mediation, or editor-like agent interoperability. ACP is emerging as the right boundary for those interactive agent workflows, so Invowk should establish a small, unused internal foundation now before adding user-facing agent features on top of it.

## What Changes

- Add an internal ACP foundation for future stateful agent-session integrations using `github.com/coder/acp-go-sdk`.
- Define a narrow internal boundary for launching ACP-compatible agent processes, negotiating protocol capabilities, creating sessions, sending prompts, handling streamed updates, and routing permission/file/terminal callbacks through Invowk-owned policy hooks.
- Keep the foundation dormant: no existing command, config key, provider selection, `invowk audit --llm`, or `invowk agent cmd create` flow will use ACP in this change.
- Preserve the current direct OpenAI-compatible API and CLI completion paths for existing one-shot LLM workflows.
- Add focused tests and documentation comments that make the dormant foundation explicit and prevent accidental user-visible behavior changes.

## Capabilities

### New Capabilities

- `agent-client-protocol-foundation`: Internal ACP substrate for future stateful agent-session features, including process lifecycle, protocol handshake, session creation, prompt dispatch, update collection, and policy callback boundaries.

### Modified Capabilities

- None.

## Impact

- Adds a new Go module dependency on `github.com/coder/acp-go-sdk`.
- Adds internal packages for ACP session/process orchestration and callback policy boundaries.
- Adds unit tests with fake ACP agents and no dependency on real Claude Code, Codex, Gemini, or networked model providers.
- Does not change current user-facing CLI behavior, config schema, CUE schema, docs snippets, generated references, or existing LLM provider semantics.
