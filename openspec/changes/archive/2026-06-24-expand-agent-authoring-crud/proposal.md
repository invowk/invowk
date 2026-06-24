## Why

`invowk agent cmd create` can only create commands today, and it lets the model choose the durable command name. That makes the agent authoring surface asymmetrical, harder to automate safely, and incomplete for users who need to change or remove generated commands or author whole modules.

## What Changes

- **BREAKING** Require `invowk agent cmd create <name> [description...]`; the legacy `create [description...]` form is removed.
- **BREAKING** Remove `agent cmd create --replace`; users must use `agent cmd change <name> [description...]` to modify an existing command.
- Add `invowk agent cmd change <name> [description...]` for LLM-assisted replacement of exactly one existing command in an invowkfile.
- Add `invowk agent cmd remove <name>` for deterministic removal of exactly one existing command from an invowkfile.
- Extend `invowk agent cmd prompt` so it can print operation-specific command authoring prompts for create, change, and remove.
- Add `invowk agent mod create <module-id> [description...]` for LLM-assisted local module scaffold creation.
- Add `invowk agent mod change <module-id-or-path> [description...]` for LLM-assisted updates to an existing local module's `invowkmod.cue` and `invowkfile.cue`.
- Add `invowk agent mod remove <module-id-or-path>` for guarded local module directory removal, not dependency removal.
- Add `invowk agent mod prompt` for operation-specific module authoring prompts.
- Keep explicit public verbs instead of a single coalesced CRUD command; share implementation machinery internally where that reduces duplication.
- Update CLI tests, unit tests, README, website docs/snippets, diagrams, and review-doc sync maps for the clean-break command contract.

## Capabilities

### New Capabilities
- `agent-authoring-crud`: Defines the LLM-assisted command and module authoring contract for explicit create, change, remove, and prompt operations.

### Modified Capabilities
- None.

## Impact

- `cmd/invowk/agent.go` Cobra tree, help text, validation, result rendering, and LLM flag plumbing.
- `internal/agentcmd/` command prompt, generation, parsing, patching, removal, and validation workflow.
- New or extended internal package support for module authoring prompts, scaffold generation, patch planning, validation, and guarded deletion.
- Existing module scaffold logic in `pkg/invowkmod/` and module operation boundaries in `internal/app/moduleops/`.
- CLI txtar coverage in `tests/cli/testdata/` and targeted Go unit tests.
- README, website docs/snippets, architecture diagrams, and `.agents` review-doc references that mention LLM-assisted command authoring.
