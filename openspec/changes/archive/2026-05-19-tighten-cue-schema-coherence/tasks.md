## 1. Environment Inheritance Contract

- [x] 1.1 Add Go validation that rejects `env_inherit_allow` unless the runtime environment inheritance mode is explicitly `allow`.
- [x] 1.2 Update CUE schema comments and generated schema references so `env_inherit_allow` is documented as requiring `env_inherit_mode: "allow"`.
- [x] 1.3 Add parser and value-validation tests for allowlists with omitted, `none`, `all`, and `allow` modes.

## 2. Mandatory Flag and Argument Descriptions

- [x] 2.1 Update `Flag.Validate()` to require a non-whitespace description.
- [x] 2.2 Update `Argument.Validate()` to require a non-whitespace description.
- [x] 2.3 Update unit tests, fixtures, and helper values that construct flags or arguments without descriptions.
- [x] 2.4 Add explicit tests that missing and whitespace-only descriptions are rejected by direct Go value validation and parsed CUE validation.

## 3. Container Runtime Source Contract

- [x] 3.1 Refactor the container runtime CUE schema into closed source variants that require exactly one of `image` or `containerfile`.
- [x] 3.2 Keep Go `RuntimeConfig.Validate()` enforcing the same source invariant for direct Go construction and defense-in-depth.
- [x] 3.3 Preserve or adapt runtime preflight diagnostics so missing and duplicated container sources produce actionable error messages.
- [x] 3.4 Update runtime schema sync and behavioral tests to handle the container source variants.
- [x] 3.5 Add or update tests for accepted image source, accepted containerfile source, missing source, duplicated source, and diagnostic text.

## 4. Module Requirement Path Contract

- [x] 4.1 Relax `#ModuleRequirement.path` CUE validation so ordinary path segments containing consecutive dots are accepted.
- [x] 4.2 Update `requires.path` schema comments to identify CUE as a portable prefilter and Go as the owner of traversal and absolute-path validation.
- [x] 4.3 Add behavioral sync tests proving safe consecutive-dot paths are accepted by CUE and Go.
- [x] 4.4 Add or update Go validation tests for parent-directory traversal, backslash-normalized traversal, Unix absolute paths, Windows drive-qualified paths, UNC paths, and rooted Windows paths.

## 5. Documentation and Verification

- [x] 5.1 Update README, website docs, snippets, samples, and generated schema references that describe affected fields.
- [x] 5.2 Update agent-facing guidance only if it contains stale behavior for these schema contracts.
- [x] 5.3 Run targeted package tests for `pkg/invowkfile` and `pkg/invowkmod`.
- [x] 5.4 Run schema/documentation integrity checks required by touched files, including `make check-agent-docs` if agent docs change.
- [x] 5.5 Run `openspec validate tighten-cue-schema-coherence --strict` and fix any proposal/spec/task issues.
