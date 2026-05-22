## ADDED Requirements

### Requirement: Dormant ACP foundation

Invowk SHALL provide an internal ACP foundation for future stateful agent-session features without changing any current user-facing behavior.

#### Scenario: Existing LLM workflows do not use ACP

- **WHEN** a user runs `invowk audit --llm`, `invowk audit --llm-provider`, or `invowk agent cmd create`
- **THEN** Invowk SHALL continue using the existing direct LLM provider and completion paths
- **THEN** Invowk SHALL NOT route those workflows through the ACP foundation

#### Scenario: No user-facing ACP configuration is introduced

- **WHEN** this change is implemented
- **THEN** Invowk SHALL NOT add ACP CLI flags, config schema fields, CUE schema fields, generated reference entries, or website snippets
- **THEN** existing provider values such as `auto`, `claude`, `codex`, `gemini`, and `ollama` SHALL keep their current semantics

### Requirement: ACP client substrate

Invowk SHALL expose an internal client-side ACP substrate that can launch or connect to an explicitly supplied ACP-compatible agent process and drive a bounded agent turn.

#### Scenario: ACP session lifecycle succeeds with a fake agent

- **WHEN** a test fake ACP agent accepts initialization, creates a session, streams an update, and completes a prompt turn
- **THEN** the ACP foundation SHALL perform protocol initialization, create the session, send the prompt, collect the streamed update, and return the final stop reason to the caller

#### Scenario: ACP session cancellation is forwarded

- **WHEN** the caller cancels an in-flight ACP prompt context
- **THEN** the ACP foundation SHALL send an ACP cancellation notification for the active session when a session exists
- **THEN** the prompt call SHALL return a cancellation error without leaking the agent process

#### Scenario: Agent process lifecycle is owned by the foundation

- **WHEN** an ACP agent process exits, fails to initialize, or the caller closes the session
- **THEN** the ACP foundation SHALL close the connection, reap the process, and return an actionable error when startup or protocol negotiation failed

### Requirement: ACP policy callbacks

Invowk SHALL mediate ACP agent callbacks through explicit internal policy interfaces rather than allowing the ACP peer to access host resources implicitly.

#### Scenario: Permission request is delegated to policy

- **WHEN** an ACP agent requests permission for a tool call
- **THEN** the ACP foundation SHALL delegate the request to the configured Invowk policy
- **THEN** the selected permission outcome SHALL be sent back to the ACP agent

#### Scenario: Filesystem callback is delegated to policy

- **WHEN** an ACP agent requests a text-file read or write
- **THEN** the ACP foundation SHALL delegate the operation to the configured Invowk filesystem policy
- **THEN** the foundation SHALL NOT perform direct filesystem access unless the configured policy performs it

#### Scenario: Unsupported terminal callback is explicit

- **WHEN** an ACP agent requests terminal functionality and the configured policy does not support terminals
- **THEN** the ACP foundation SHALL return an unsupported-operation error to the ACP agent

### Requirement: Dependency isolation and deterministic tests

Invowk SHALL keep the ACP SDK isolated to the internal ACP foundation and verify the foundation without real external agent tools.

#### Scenario: ACP SDK imports are localized

- **WHEN** the ACP foundation is implemented
- **THEN** imports of `github.com/coder/acp-go-sdk` SHALL be limited to the internal ACP foundation package and its tests unless a later OpenSpec change explicitly expands the integration surface

#### Scenario: Tests avoid real agent CLIs

- **WHEN** the ACP foundation tests run
- **THEN** they SHALL use fake ACP peers or test binaries
- **THEN** they SHALL NOT require Claude Code, Codex, Gemini, OAuth state, API keys, network access, or installed external ACP adapters
