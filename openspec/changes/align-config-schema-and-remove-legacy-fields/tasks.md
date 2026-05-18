## 1. Baseline And Tests

- [x] 1.1 Add config schema tests that verify `DefaultConfig()` is produced by evaluating an empty config through `#Config`.
- [x] 1.2 Add config behavioral tests for defaulted top-level fields, nested defaults, valid overrides, and invalid overrides.
- [x] 1.3 Add invowkfile schema tests that assert legacy field names are absent from `#Invowkfile` extracted fields.
- [x] 1.4 Add closed-struct rejection tests for unknown fields in config and invowkfile schemas.

## 2. Config Schema And Loading

- [x] 2.1 Convert `internal/config/config_schema.cue` from patch-style optional fields to a closed full-shape schema with CUE defaults.
- [x] 2.2 Model LLM no-backend, provider-backend, and API-backend config as closed CUE disjunction branches.
- [x] 2.3 Update config decoding so user config evaluates to a complete `Config` without relying on pointer patch DTO defaults.
- [x] 2.4 Preserve dynamic Go validation for filesystem paths, include collection uniqueness, URL parsing, durations, and value-object defense-in-depth checks.
- [x] 2.5 Replace the hand-written `DefaultConfig()` default struct with a CUE-derived implementation.

## 3. Invowkfile Schema Cleanup

- [x] 3.1 Remove explicit legacy unsupported-field declarations from `pkg/invowkfile/invowkfile_schema.cue`.
- [x] 3.2 Verify `#Invowkfile` and nested user-facing structs remain closed after legacy tombstone removal.
- [x] 3.3 Update tests that previously expected legacy field-specific tombstone behavior to expect ordinary closed-schema rejection.

## 4. Documentation And Generated Output

- [x] 4.1 Update config docs and generated config references to describe config as a default-bearing full schema.
- [x] 4.2 Remove docs or snippets that describe old invowkfile fields as special migration cases.
- [x] 4.3 Update generated config output and round-trip tests so all effective fields are emitted, including default-valued fields.

## 5. Verification

- [x] 5.1 Run `go test ./internal/config ./pkg/invowkfile ./pkg/invowkmod`.
- [x] 5.2 Run `make check-agent-docs` if docs, `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed.
- [x] 5.3 Run the relevant repo gate from `.agents/rules/commands.md` for schema/config changes.
- [x] 5.4 Run `openspec validate align-config-schema-and-remove-legacy-fields --strict`.
