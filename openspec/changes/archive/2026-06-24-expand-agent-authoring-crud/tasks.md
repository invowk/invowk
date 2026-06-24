## 1. Contract Inventory

- [x] 1.1 Inventory current `invowk agent` Cobra commands, help text, flags, and validation paths in `cmd/invowk/agent.go`.
- [x] 1.2 Inventory current command authoring internals in `internal/agentcmd/`, including prompt rendering, response parsing, repair attempts, invowkfile patching, and diff output.
- [x] 1.3 Inventory module scaffold, validation, local module detection, symlink policy, and create/remove helpers in `pkg/invowkmod/` and `internal/app/moduleops/`.
- [x] 1.4 Inventory all LLM-assisted authoring docs, snippets, diagrams, review-doc references, and CLI txtar tests that mention `agent cmd prompt`, `agent cmd create`, or `--replace`.

## 2. Shared Authoring Internals

- [x] 2.1 Refactor description loading, LLM completer resolution inputs, structured-output fallback, bounded repair attempts, and response parsing into reusable helpers for create/change operations.
- [x] 2.2 Introduce operation-aware prompt document types that can render text and JSON for command create, command change, command remove, module create, module change, and module remove.
- [x] 2.3 Introduce shared write-mode validation for `--dry-run`, `--print`, and `--verify` so invalid mode combinations fail before provider resolution.
- [x] 2.4 Add shared file-diff/render helpers that support single-file command patches and multi-file module create/change plans.
- [x] 2.5 Preserve existing LLM provider/config behavior for authoring operations, including configured defaults and per-run LLM flags.

## 3. Command Authoring Implementation

- [x] 3.1 Change `invowk agent cmd create` to require `create <name> [description...]`, remove `--replace`, and require description text from arguments or `--from-file`.
- [x] 3.2 Validate that generated command CUE contains exactly one command whose `name` equals the requested `<name>`, including bounded repair feedback for mismatches.
- [x] 3.3 Make command create fail when the target invowkfile already contains the requested command and point users to `agent cmd change <name>`.
- [x] 3.4 Implement `invowk agent cmd change <name> [description...]` to require an existing target command, send existing command context to the LLM, and replace only that command.
- [x] 3.5 Implement `invowk agent cmd remove <name>` as a deterministic invowkfile edit with `--file` and `--dry-run`, no LLM provider resolution, and validation before write.
- [x] 3.6 Update command prompt rendering so `agent cmd prompt`, `agent cmd prompt create`, `agent cmd prompt change`, and `agent cmd prompt remove` produce accurate text and JSON output.
- [x] 3.7 Update command result rendering and errors for create, change, remove, dry-run, print, and verify modes.

## 4. Module Authoring Implementation

- [x] 4.1 Add `invowk agent mod` as a sibling of `agent cmd` with `create`, `change`, `remove`, and `prompt` subcommands.
- [x] 4.2 Implement `agent mod create <module-id> [description...]` using caller-owned module identity, existing module naming validation, and generated `invowkmod.cue` plus `invowkfile.cue` content.
- [x] 4.3 Make module create fail when the target local module directory already exists and point users to `agent mod change <module-id>`.
- [x] 4.4 Implement `agent mod change <module-id-or-path> [description...]` to resolve an existing local module and update only `invowkmod.cue` and `invowkfile.cue`.
- [x] 4.5 Validate module create/change output so `invowkmod.cue` keeps the requested module ID and the resulting module validates before any write.
- [x] 4.6 Implement module `--dry-run`, `--print`, and `--verify` modes with structured print output and module validation after writes.
- [x] 4.7 Implement `agent mod remove <module-id-or-path>` with `--dry-run`, required `--force` for deletion, exact local module validation, symlink rejection, and no dependency or lock-file edits.
- [x] 4.8 Implement module prompt rendering so `agent mod prompt`, `agent mod prompt create`, `agent mod prompt change`, and `agent mod prompt remove` produce accurate text and JSON output.

## 5. Unit Tests

- [x] 5.1 Update `cmd/invowk/agent_test.go` for the new command create arity, removed `--replace` flag, shared mode validation, and new command/module subcommands.
- [x] 5.2 Update `internal/agentcmd` tests for caller-owned command names, create duplicate failure, change missing failure, replace-only-target behavior, deterministic remove, and prompt output.
- [x] 5.3 Add module authoring unit tests for generated file bundle parsing, module identity mismatch repair, existing module failure, missing module failure, arbitrary file write rejection, and structured print output.
- [x] 5.4 Add module removal unit tests for dry-run, force-required deletion, exact directory deletion, symlink rejection, invalid module rejection, and no dependency/lock-file mutation.
- [x] 5.5 Add tests proving remove operations do not resolve or call an LLM provider.

## 6. CLI Integration Tests

- [x] 6.1 Rewrite `tests/cli/testdata/agent_cmd_create.txtar` and `agent_cmd_create_config.txtar` for `agent cmd create <name> [description...]` and no `--replace`.
- [x] 6.2 Add command change and remove txtar coverage for successful change, missing-command failure, identity mismatch repair/failure, deterministic remove, dry-run remove, and provider-not-called remove.
- [x] 6.3 Update `agent_cmd_prompt.txtar` for operation-specific text and JSON prompt output.
- [x] 6.4 Add `agent_mod_*` txtar coverage for module create, change, remove, prompt, dry-run, print, verify, existing module failure, missing module failure, and force-required removal.
- [x] 6.5 Add clean-break assertions that legacy description-only command create and `agent cmd create --replace` fail with useful errors.

## 7. Documentation and Diagrams

- [x] 7.1 Update README feature summary and LLM-assisted authoring section for `agent cmd` and `agent mod` create/change/remove/prompt behavior.
- [x] 7.2 Update website current docs for CLI reference, config-schema references, and advanced LLM-assisted authoring content.
- [x] 7.3 Update current Portuguese i18n docs that mention `agent cmd create` or LLM authoring defaults.
- [x] 7.4 Update website snippet data so generated CLI snippets use required create identities and the new change/remove/module examples.
- [x] 7.5 Update C4 architecture docs and D2 diagrams so Agent Authoring reads/patches invowkfiles and local modules, not only commands.
- [x] 7.6 Regenerate checked-in rendered diagrams if the repository expects rendered artifacts to stay in sync.
- [x] 7.7 Update `.agents/skills/review-docs/references/` authoring checklists and consolidated sync maps for the expanded agent authoring surface.

## 8. Verification

- [x] 8.1 Run `gofmt` on changed Go files.
- [x] 8.2 Run targeted Go tests for `./cmd/invowk`, `./internal/agentcmd`, new or changed module-authoring packages, `./internal/app/moduleops`, and `./pkg/invowkmod`.
- [x] 8.3 Run CLI testscript coverage with `go test ./tests/cli`.
- [x] 8.4 Run `openspec validate expand-agent-authoring-crud --strict`.
- [x] 8.5 Run `openspec validate --changes --strict` and `openspec validate --specs --strict`.
- [x] 8.6 Run documentation integrity checks, including `make check-agent-docs` and diagram render checks when diagrams changed.
- [x] 8.7 Run repository gates appropriate for the final change set, including `make test` and `make lint`.
- [x] 8.8 Confirm final docs and release-summary notes clearly call out the clean break: command create now requires `<name>` and `--replace` was replaced by `change`.
