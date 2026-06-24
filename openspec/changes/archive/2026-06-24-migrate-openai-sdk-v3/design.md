## Context

Invowk's `internal/auditllm` package uses the official OpenAI Go SDK v1 module path to talk to OpenAI-compatible APIs. The client is intentionally compatible with local servers such as Ollama by defaulting to `http://localhost:11434/v1`, suppressing ambient `OPENAI_API_KEY` lookup when no API key is configured, and using Chat Completions plus `GET /v1/models`.

OpenAI's current documentation presents the Go SDK as an official SDK surface and emphasizes the Responses API for new OpenAI API examples. Invowk, however, is not only an official OpenAI API client: it is also a local OpenAI-compatible LLM client. The SDK major-path migration must therefore preserve compatibility with local Chat Completions providers unless a separate change explicitly migrates the product behavior.

Exploration on 2026-06-09 checked `github.com/openai/openai-go/v3@latest`,
which resolved to `v3.39.0`. The v3 SDK still exposes the compatibility
surface Invowk currently needs: `openai.NewClient`, `option.WithBaseURL`,
`option.WithAPIKey`, `client.Chat.Completions.New`, `client.Models.List`,
`openai.Error`, `openai.ChatModel`, `openai.ResponseFormatJSONSchemaParam`,
and `param.NewOpt`. This supports a narrow adapter migration rather than a
product behavior migration.

## Goals / Non-Goals

**Goals:**
- Move from `github.com/openai/openai-go` to `github.com/openai/openai-go/v3`.
- Preserve `llm.Completer`, `llm.StructuredCompleter`, and `ModelVerifier` behavior.
- Preserve local OpenAI-compatible endpoint support, including Ollama defaults and optional API key behavior.
- Preserve OpenAI-only structured JSON schema output gating.
- Cover the migration with deterministic tests that do not require real API keys or network services.

**Non-Goals:**
- Migrate Invowk's LLM workflows from Chat Completions to the Responses API.
- Add new model/provider configuration fields.
- Change `invowk audit --llm`, `--llm-provider`, or `agent cmd create` user-facing behavior.
- Add ACP-based LLM routing.

## Decisions

### Migrate the SDK module path while preserving API behavior

Replace v1 SDK imports with `github.com/openai/openai-go/v3` and adapt type names, request parameters, and error classification as needed. The external Invowk interfaces should remain unchanged: callers still receive strings or typed errors from `Complete`, `CompleteJSONSchema`, and `VerifyModel`.

Alternative considered: leave v1 in place because it is current on its module path. That avoids immediate churn but leaves Invowk behind the active major module stream and makes later migration larger.

### Keep Chat Completions for this change

Use the v3 SDK's Chat Completions support or equivalent compatibility path for this migration. The current local provider contract depends on `/v1/chat/completions`, and changing to Responses would require revalidating local server support, structured-output semantics, and response parsing.

Implementation MUST NOT route `Complete` or `CompleteJSONSchema` through the
Responses API. Fake-server tests should fail if the SDK adapter posts to
`/responses` or any path other than the existing OpenAI-compatible Chat
Completions endpoint.

Alternative considered: migrate to Responses API at the same time. That is attractive for official OpenAI usage but too broad for an SDK module-path refresh because Invowk supports local OpenAI-compatible backends.

### Test through a fake HTTP endpoint

Add or update tests with an in-process HTTP server that records `/v1/chat/completions` and `/v1/models` requests and returns representative OpenAI-compatible responses. This avoids external credentials and proves base URL, headers, request payloads, response parsing, and error classification.

Alternative considered: rely on compile-only coverage. That would miss subtle behavior regressions around API key fallback, local base URLs, and structured-output gating.

The current `internal/auditllm` tests already cover many completion and model
verification paths, but direct `CompleteJSONSchema` request-body coverage is
missing before this change. Add explicit structured-output tests during the
migration instead of relying on compile-time interface assertions.

### Preserve OpenAI-only structured-output gating

Keep schema-constrained output enabled only for `api.openai.com`. Other compatible providers should still return `llm.ErrStructuredOutputUnsupported` and let higher-level prompt-only fallback logic handle them.

Alternative considered: attempt structured outputs for all compatible providers. That would regress local providers that accept Chat Completions but not OpenAI's JSON schema response format.

The OpenAI-host path should assert the Chat Completions `response_format`
payload contains `type: json_schema` plus the configured schema name,
description, schema body, and strict flag. The non-OpenAI path should assert no
HTTP request is made for structured output.

### Preserve ambient credential suppression

The current client intentionally avoids accidentally picking up ambient
`OPENAI_API_KEY` when no API key is configured for local endpoints. Preserve
that behavior under v3. The test should set a poison `OPENAI_API_KEY` value and
prove that local requests do not use it. If the implementation continues using a
deterministic suppression credential such as `Bearer ollama`, that value should
remain explicit and tested.

## Risks / Trade-offs

- v3 SDK type names and error wrappers may differ -> Mitigate with compile fixes plus tests for API and network error classification.
- Official examples favor Responses API -> Mitigate by documenting that this migration preserves Invowk's existing OpenAI-compatible Chat Completions contract.
- Fake HTTP tests can overfit to current request shape -> Mitigate by asserting behavior-critical fields rather than every generated SDK detail.
- Local providers may reject changed headers -> Mitigate by preserving empty-key suppression behavior and testing no user-provided key cases.
- v3's default client options still read `OPENAI_API_KEY` and `OPENAI_BASE_URL`
  from the environment -> Mitigate by passing explicit base URL and explicit
  empty-key suppression options and testing with poison ambient environment
  values.

## Migration Plan

1. Update root Go module dependencies to use `github.com/openai/openai-go/v3`.
2. Adapt `internal/auditllm` imports, client construction, Chat Completions calls, model listing, structured-output parameters, and error classification.
3. Add or update fake-server tests for completion, structured output gating, model verification, API errors, network errors, and no-key local endpoint behavior.
4. Run targeted `internal/auditllm` tests, `make test`, `make lint`, and `govulncheck` after tidying.

Rollback is to restore the v1 SDK dependency and the previous request/error adapter code if v3 compatibility proves unsuitable before merge.

## Open Questions

None. If implementation discovers the v3 SDK cannot preserve Chat Completions compatibility for local providers, pause the change and split a separate Responses API design before changing user-visible behavior.
