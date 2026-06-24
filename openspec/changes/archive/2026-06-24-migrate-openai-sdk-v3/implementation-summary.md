## Implementation Summary

Selected SDK version: `github.com/openai/openai-go/v3 v3.39.0`.

The migration preserves the existing Chat Completions adapter behavior. The
`internal/auditllm` client still sends chat requests to `/chat/completions` and
does not route `Complete` or `CompleteJSONSchema` through the Responses API.
The existing caller-owned LLM ports remain unchanged: `llm.Completer`,
`llm.StructuredCompleter`, and model verification continue to be implemented by
the audit LLM adapter.

The v3 SDK change required updating the module path and imports to the
versioned `github.com/openai/openai-go/v3` package family. The adapter kept the
same custom base URL, API key, chat completion, model listing, response format,
and SDK error-classification behavior, with tests pinning the request shape and
error handling expectations.

Local OpenAI-compatible provider assumptions remain represented in tests:

- Custom base URLs are exercised through fake HTTP servers.
- Local chat completion requests assert `/chat/completions`, configured model,
  role-separated system/user messages, deterministic temperature, and no
  `/responses` fallback.
- Empty local API keys are verified against a poison ambient `OPENAI_API_KEY`
  so local endpoints do not accidentally inherit host credentials.
- Structured output remains gated to OpenAI hosts; non-OpenAI hosts return
  `llm.ErrStructuredOutputUnsupported` without issuing an HTTP request.
- Model verification covers exact matches, prefix-compatible matches, and
  not-found suggestions.
- API error, network error, context cancellation, empty response, and
  content-filter classifications are covered by adapter tests.

Verification completed:

- `go test ./internal/auditllm ./internal/llm`
- `go mod tidy`
- `go mod tidy -diff`
- `govulncheck ./...`
- `make lint`
- `make check-baseline`
- `make check-file-length`
- `make test`
- Empty diff check for user-facing LLM provider flags, config fields, and ACP
  routing surfaces.
