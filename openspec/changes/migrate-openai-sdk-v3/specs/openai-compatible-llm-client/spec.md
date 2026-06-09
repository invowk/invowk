## ADDED Requirements

### Requirement: Configurable OpenAI-compatible endpoint
Invowk SHALL provide an internal LLM client that can call OpenAI-compatible HTTP APIs using a configurable base URL, model name, API key, and timeout.

#### Scenario: Default local endpoint is used
- **WHEN** an LLM client is created without an explicit base URL or model
- **THEN** it MUST use the local OpenAI-compatible default base URL
- **AND** it MUST use the configured default code-focused model

#### Scenario: Empty API key does not use ambient credentials
- **WHEN** an LLM client is created without an API key
- **THEN** it MUST NOT depend on an ambient `OPENAI_API_KEY` value for local provider requests
- **AND** local OpenAI-compatible providers MUST remain usable without user-provided credentials
- **AND** any Authorization header used to suppress SDK environment fallback MUST be deterministic and MUST NOT contain the ambient `OPENAI_API_KEY` value

### Requirement: Chat completion behavior is preserved
Invowk SHALL send system and user prompts through the OpenAI-compatible chat completion flow and return the assistant's textual response.

#### Scenario: Completion request succeeds
- **WHEN** the configured provider returns a chat completion with assistant text
- **THEN** Invowk MUST send the request to the Chat Completions endpoint
- **AND** it MUST NOT use the Responses API for this SDK migration
- **AND** the request MUST preserve the configured model, separate system and user messages, and deterministic temperature setting
- **AND** Invowk MUST return the trimmed assistant text to the caller

#### Scenario: Completion response is empty
- **WHEN** the provider returns no choices or empty assistant content
- **THEN** Invowk MUST return the existing empty-response error classification

#### Scenario: Completion is content filtered
- **WHEN** the provider reports a content-filter finish reason
- **THEN** Invowk MUST return the existing content-filtered error classification

### Requirement: Structured output remains OpenAI-only
Invowk SHALL request JSON schema constrained output only for providers known to support OpenAI structured outputs.

#### Scenario: OpenAI host supports JSON schema response format
- **WHEN** the LLM client is configured for `api.openai.com` and a valid JSON schema format is requested
- **THEN** Invowk MUST send a schema-constrained completion request
- **AND** the request MUST use the Chat Completions response format field with `type` set to `json_schema`
- **AND** the request MUST include the configured schema name, description, schema body, and strict flag
- **AND** the returned assistant content MUST be processed through the same completion response rules

#### Scenario: Local compatible provider rejects structured output path
- **WHEN** the LLM client is configured for a non-OpenAI host and structured output is requested
- **THEN** Invowk MUST return `llm.ErrStructuredOutputUnsupported`
- **AND** it MUST NOT send a schema-constrained request to that provider
- **AND** it MUST NOT make any HTTP request for that structured-output attempt

### Requirement: Model verification uses provider model listing
Invowk SHALL verify configured model availability by querying the provider's OpenAI-compatible model listing endpoint.

#### Scenario: Exact model is available
- **WHEN** the provider model list contains the configured model ID
- **THEN** model verification MUST succeed

#### Scenario: Compatible prefix model is available
- **WHEN** the provider model list contains a prefix-compatible model ID for the configured model
- **THEN** model verification MUST succeed

#### Scenario: Model is not available
- **WHEN** the provider model list does not contain the configured model or a compatible prefix match
- **THEN** Invowk MUST return a model-not-found error that includes available model information and a code-model suggestion when possible

### Requirement: SDK migrations preserve Invowk LLM behavior
Invowk SHALL allow internal OpenAI SDK version migrations only when the public behavior of its LLM client remains stable.

#### Scenario: SDK module path changes
- **WHEN** maintainers migrate the internal OpenAI SDK dependency to a newer major module path
- **THEN** completion, structured-output gating, model verification, timeout configuration, base URL configuration, and error classification MUST remain covered by tests
- **AND** no real OpenAI API key, local Ollama server, or network service MUST be required for those tests
- **AND** tests MUST prove the migration still uses Chat Completions rather than the Responses API
- **AND** tests MUST use poison ambient OpenAI environment values where needed to prove explicit client configuration wins
