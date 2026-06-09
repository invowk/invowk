## 1. SDK Migration

- [ ] 1.1 Re-check the latest `github.com/openai/openai-go/v3` version and confirm the selected version before editing dependencies.
- [ ] 1.2 Update the root Go module to depend on `github.com/openai/openai-go/v3` and remove the v1 module path if it is no longer needed.
- [ ] 1.3 Adapt `internal/auditllm` imports, client construction, request options, and API error handling for the v3 SDK.
- [ ] 1.4 Adapt chat completion request construction and response parsing while preserving current `llm.Completer` behavior.
- [ ] 1.5 Adapt JSON schema structured-output request construction while preserving the OpenAI-host-only gate.
- [ ] 1.6 Adapt model listing and model verification while preserving exact and prefix-compatible model matching.
- [ ] 1.7 Confirm the implementation continues to use Chat Completions and does not route `Complete` or `CompleteJSONSchema` through the Responses API.

## 2. Compatibility Tests

- [ ] 2.1 Add or update fake HTTP server tests for local OpenAI-compatible chat completion requests, asserting `/chat/completions`, configured model, role-separated system/user messages, and deterministic temperature.
- [ ] 2.2 Test that an empty API key does not rely on ambient `OPENAI_API_KEY` for local endpoint use by setting a poison environment value and asserting it is not sent.
- [ ] 2.3 Test OpenAI-host structured output request construction, including `response_format.type`, schema name, description, schema body, and strict flag.
- [ ] 2.4 Test non-OpenAI `llm.ErrStructuredOutputUnsupported` fallback and assert no HTTP request is made for that structured-output attempt.
- [ ] 2.5 Test model verification success, prefix-compatible success, and model-not-found suggestions.
- [ ] 2.6 Test API error, network error, context cancellation, empty response, and content-filter classifications with the v3 SDK adapter.
- [ ] 2.7 Add a guard assertion that fake-server tests fail on `/responses` so the migration cannot silently switch APIs.

## 3. Verification

- [ ] 3.1 Run `go test ./internal/auditllm ./internal/llm`.
- [ ] 3.2 Run root `go mod tidy` and confirm `go mod tidy -diff` is clean.
- [ ] 3.3 Run `govulncheck ./...` from the root module.
- [ ] 3.4 Run `make test` and `make lint`.
- [ ] 3.5 Confirm no user-facing LLM provider flags, config fields, or ACP routing behavior changed.

## 4. Review

- [ ] 4.1 Document in the implementation summary whether the migration preserved Chat Completions rather than moving to Responses API.
- [ ] 4.2 Record the selected v3 version and any v3 SDK behavior differences that required adapter changes.
- [ ] 4.3 Confirm local OpenAI-compatible provider assumptions remain represented in tests.
