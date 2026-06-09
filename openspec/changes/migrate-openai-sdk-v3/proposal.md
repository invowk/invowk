## Why

The root dependency audit confirmed that `github.com/openai/openai-go` is current on its v1 module path, but the actively moving SDK surface is now available on the `github.com/openai/openai-go/v3` module path. Invowk's LLM client uses OpenAI-compatible Chat Completions against both OpenAI and local Ollama-style endpoints, so this should be handled as a compatibility-preserving migration rather than a routine dependency bump.

## What Changes

- Migrate the internal OpenAI-compatible LLM client from `github.com/openai/openai-go` v1 to `github.com/openai/openai-go/v3`.
- Preserve current local endpoint behavior, including the Ollama default base URL, optional API key handling, model listing, chat completion requests, and error classification.
- Preserve OpenAI-only structured JSON schema output behavior and keep unsupported OpenAI-compatible providers on the existing fallback path.
- Add or update tests that exercise request construction, model verification, structured-output gating, and representative API/network error handling with the v3 SDK.

## Capabilities

### New Capabilities
- `openai-compatible-llm-client`: Defines Invowk's contract for the internal OpenAI-compatible LLM client across official OpenAI and local compatible endpoints.

### Modified Capabilities
- None.

## Impact

- `internal/auditllm/` SDK imports, request construction, response parsing, and error handling.
- Root Go module dependency graph.
- LLM/audit tests that cover local provider compatibility and OpenAI-only structured outputs.
- Potential documentation or agent guidance if SDK migration changes provider compatibility assumptions.
