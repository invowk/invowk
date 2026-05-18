## Affected File Inventory

Recorded before implementation edits for tasks 1.1-1.3.

## Hand-Edited Code And Schemas

- `pkg/invowkfile/invowkfile_schema.cue`
- `pkg/invowkfile/dependency.go`
- `pkg/invowkfile/command.go`
- `pkg/invowkfile/implementation.go`
- `pkg/invowkfile/validation_primitives.go`
- `pkg/invowkfile/validation_filesystem.go`
- `pkg/invowkfile/validation_structure_deps.go`
- `pkg/invowkfile/containerfile_path.go`
- `pkg/invowkfile/parse.go`
- `pkg/invowkfile/runtime.go`
- `internal/app/deps/deps.go`
- `internal/app/deps/checks.go`
- `internal/app/deps/types.go`
- `internal/audit/scan_context_clone.go`
- `internal/config/config_schema.cue`
- `internal/config/config.go`
- `internal/config/types.go`

## Tests

- `pkg/invowkfile/sync_runtime_behavioral_test.go`
- `pkg/invowkfile/sync_behavioral_test.go`
- `pkg/invowkfile/sync_test.go`
- `pkg/invowkfile/validation_test.go`
- `pkg/invowkfile/containerfile_path_test.go`
- `pkg/invowkfile/runtime_validate_test.go`
- `pkg/invowkfile/runtime_deps_test.go`
- `pkg/invowkfile/invowkfile_deps*.go`
- `internal/app/deps/deps_test.go`
- `internal/app/deps/deps_alternative_scope_test.go`
- `internal/app/deps/deps_scope_identity_test.go`
- `internal/app/deps/command_scope_lock_test.go`
- `internal/app/deps/checks_test.go`
- `internal/config/config_schema_behavior_test.go`
- `internal/config/sync_test.go`

## Documentation And Snippets

- `README.md`
- `website/docs/dependencies/commands.mdx`
- `website/docs/core-concepts/commands-and-namespaces.mdx`
- `website/docs/runtime-modes/container.mdx`
- `website/docs/reference/invowkfile-schema.mdx`
- `website/docs/configuration/options.mdx`
- `website/docs/reference/config-schema.mdx`
- `website/src/components/Snippet/data/core-concepts.ts`
- `website/src/components/Snippet/data/dependencies.ts`
- `website/src/components/Snippet/data/runtime-modes.ts`
- `website/src/components/Snippet/data/config.ts`
- `website/src/components/Snippet/versions/v0.13.0.ts`
- Versioned snippet data containing stale source-prefix-with-space command dependency examples.

## Agent-Facing Guidance

- `.agents/rules/cue-patterns.md`
- `.agents/agents/cue-schema-agent.md`
- `.agents/skills/cue/SKILL.md`

## Regenerated Or Derived Surfaces

- `website/src/components/Snippet/versions/*.ts` are generated/versioned snippet bundles and may need synchronized edits where stale current behavior is embedded.
- `website/versioned_docs/version-0.13.0/**` mirrors current release docs and may need direct updates when it repeats current behavior.
