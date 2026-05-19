## Why

Several CUE schema contracts are broadly coherent but still allow confusing gaps between schema validation, Go value validation, and post-decode semantic validation. Tightening these edges will make invalid configurations fail earlier or more consistently, while preserving Go as the source of truth for cross-platform filesystem and effective-runtime semantics.

## What Changes

- **BREAKING**: Reject `env_inherit_allow` unless the same runtime configuration explicitly sets `env_inherit_mode: "allow"`.
- **BREAKING**: Make flag and argument descriptions mandatory everywhere, including value-level `Flag.Validate()` and `Argument.Validate()` paths.
- Encode the container runtime source contract more clearly so a container runtime must declare exactly one of `image` or `containerfile`, while retaining actionable diagnostics and Go defense-in-depth.
- Relax module requirement `path` CUE validation so ordinary path segments containing consecutive dots are allowed, while Go continues to reject traversal, absolute paths, drive-qualified paths, and other cross-platform unsafe forms.
- Update tests and documentation so user-facing examples, schema comments, Go validators, and behavioral sync coverage describe the same validation ownership model.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cue-schema-coherence`: Tighten schema and Go validation parity for environment inheritance, flag and argument descriptions, container runtime source selection, and module requirement subdirectory paths.

## Impact

- `pkg/invowkfile/invowkfile_schema.cue`
- `pkg/invowkfile` runtime, flag, argument, structure validation, parser, and schema sync tests
- `pkg/invowkmod/invowkmod_schema.cue`
- `pkg/invowkmod` subdirectory path validation and parser tests
- Documentation and generated examples that describe runtime environment inheritance, command flags and arguments, container runtime source selection, or module requirement paths
