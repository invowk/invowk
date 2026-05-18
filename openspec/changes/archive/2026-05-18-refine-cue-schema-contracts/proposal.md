## Why

Invowk's CUE schemas are broadly coherent, but several edges blur the line between schema-shape validation, Go-only semantic validation, command source identity, and documentation examples. This change makes that contract explicit, adopts source-qualified command dependency references, and removes stale or confusing schema/doc patterns as a clean break.

## What Changes

- **BREAKING** Replace command dependency alternatives with a reference grammar that supports bare local command names and explicit source-qualified references such as `@tools lint` and `@com.company.tools lint`.
- **BREAKING** Stop interpreting dependency alternatives such as `tools lint` as module/source-qualified references; source qualification requires an `@source` prefix.
- **BREAKING** Tighten containerfile path policy to reject any path segment equal to `..` after slash/backslash normalization, while allowing ordinary filenames or segments that merely contain `..` such as `Containerfile..backup`.
- **BREAKING** Remove stale schema/docs wording and snippets that imply old command dependency syntax, broad textual `..` rejection, or provider-only LLM common fields.
- Clarify CUE-vs-Go validation ownership in schema comments, validators, docs, and tests.
- Improve diagnostics for the enumerated runtime-variant-only fields and container runtime `image`/`containerfile` invariants so users see targeted errors instead of generic CUE disjunction failures.
- Rename the internal/documented LLM CUE helper concept from `#LLMNoBackendConfig` to `#LLMDefaultsConfig`, with no compatibility alias.
- Update README, website docs, snippets, versioned snippet data, samples, schema references, and agent-facing documentation so no stale syntax or validation descriptions remain.

## Capabilities

### New Capabilities
- `source-qualified-command-dependencies`: Defines command dependency reference syntax, parsing, scope enforcement, diagnostics, and documentation for explicit `@source command` references.

### Modified Capabilities
- `cue-schema-coherence`: Updates schema/Go validation boundaries, runtime diagnostics, LLM schema naming, containerfile path policy, tests, and documentation coherence requirements.

## Impact

- Affected schemas: `pkg/invowkfile/invowkfile_schema.cue`, `internal/config/config_schema.cue`, generated/current website schema snippets, and any schema reference snippets.
- Affected Go packages: `pkg/invowkfile`, `internal/app/deps`, `internal/config`, `internal/runtime`, and related test helpers/sync tests.
- Affected docs: README, website current docs and snippets, versioned snippets where applicable, samples, docs that mention command dependency syntax, container runtime `containerfile`, LLM config, or CUE validation ownership, plus agent docs if they repeat old behavior.
- Compatibility: this is an intentional clean break. Old source-prefix-with-space command dependency syntax and stale schema helper names should be removed rather than aliased.
