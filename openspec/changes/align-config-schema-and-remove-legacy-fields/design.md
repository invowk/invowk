## Context

Invowk currently has three user-facing CUE schema surfaces: `invowkfile.cue`, `invowkmod.cue`, and `config.cue`. `invowkfile.cue` and `invowkmod.cue` are complete document schemas: the schema describes the document shape directly, structs are closed, and required fields are visible in CUE.

`config.cue` is different today. The schema accepts a partial patch, Go decodes that patch into pointer DTOs, then applies it over `DefaultConfig()`. That preserves "unset vs explicitly false" in Go, but it means the CUE schema does not show the effective full shape or default values. This change makes CUE the visible full-shape contract for config, while preserving Go validation for checks CUE cannot do reliably.

The invowkfile schema also contains explicit tombstones for old field names such as `commands`, `module`, `version`, `description`, and `requires`. Those fields should no longer be modeled as special cases. Closed structs are sufficient: unknown fields are invalid because they are unknown.

## Goals / Non-Goals

**Goals:**

- Make `internal/config/config_schema.cue` describe the complete effective config shape, including defaults and constraints.
- Use CUE defaults for config values that `DefaultConfig()` currently supplies.
- Derive `DefaultConfig()` from evaluating the CUE config schema, so CUE is the single source of truth for defaults.
- Generate config output with all effective fields present, including default-valued fields.
- Use CUE disjunctions for true config variants, especially mutually exclusive shapes such as LLM provider-backed config versus API-backed config.
- Keep all user-facing CUE structs closed.
- Remove explicit old-field tombstones from invowkfile schema definitions.
- Keep Go validation for dynamic checks only: filesystem normalization, path absoluteness, duplicate collection checks, runtime probes, and other checks that need platform or environment state.
- Verify that decoded empty config, generated default config, and `DefaultConfig()` stay equivalent.

**Non-Goals:**

- Do not revisit module path traversal prefilters in this change.
- Do not add compatibility aliases for old invowkfile field names.
- Do not introduce new config fields or change default values unless required to faithfully express existing defaults in CUE.
- Do not remove defense-in-depth Go validation; only stop treating it as the primary source of default/config-shape truth.

## Decisions

1. `#Config` becomes the effective full config schema.

   Instead of optional top-level patch fields, `#Config` should define regular fields with defaults, for example:

   ```cue
   container_engine: *"podman" | "docker"
   includes:         *([]) | [...#IncludeEntry]
   default_runtime:  *"native" | "virtual" | "container"
   virtual_shell:    #VirtualShellConfig
   ui:               #UIConfig
   container:        #ContainerConfig
   llm:              #LLMConfig
   ```

   Nested structs should follow the same pattern. For example, `ui.color_scheme` defaults to `"auto"`, `ui.verbose` defaults to `false`, and `virtual_shell.enable_uroot_utils` defaults to `true`.

   Alternative considered: keep `#Config` patch-shaped and add a separate `#EffectiveConfig`. Rejected because it preserves two user-facing mental models and keeps `#Config` misleading.

2. `DefaultConfig()` is derived from CUE evaluation.

   `DefaultConfig()` must evaluate an empty user config against `#Config` and return the decoded effective config. Go must not carry an independent hand-written copy of the same default values.

   Alternative considered: keep `DefaultConfig()` hand-written with a mandatory parity test. Rejected because it still preserves two default definitions and makes CUE less authoritative than the schema contract says it is.

3. Config variants use CUE disjunctions where the shape differs.

   LLM config should be modeled as a closed disjunction rather than as one open-ish struct plus Go-only mutual exclusion. Conceptually:

   ```cue
   #LLMConfig: #LLMConfigNone | #LLMProviderConfig | #LLMAPIBackendConfig
   ```

   This makes valid shapes visible in the schema:

   - no LLM backend configured
   - local/provider backend configured
   - OpenAI-compatible API backend configured

   Go may still validate URL parsing and value-object invariants after decode, but provider/API shape exclusivity should be CUE-owned if the final implementation can express it cleanly.

   Alternative considered: leave provider/API mutual exclusion as Go-only. Rejected because this is exactly the kind of closed variant shape CUE disjunctions handle well.

4. Config loading decodes an effective config, not a patch DTO.

   Once `#Config` carries defaults, config decoding can validate with `cue.Concrete(true)` and decode into the normal `Config` shape rather than a pointer-based patch DTO. If a small transitional helper remains, it must not be the public schema model and must not reintroduce default ownership in Go.

   Alternative considered: keep patch DTOs indefinitely. Rejected because the schema would still not represent the full effective config shape.

5. Legacy invowkfile fields are removed from the schema rather than tombstoned.

   Remove explicit unsupported fields such as:

   ```cue
   commands?: _|_
   module?: _|_
   version?: _|_
   description?: _|_
   requires?: _|_
   ```

   Because `#Invowkfile` is closed, these names remain invalid as ordinary unknown fields. Documentation and tests should stop treating them as named migration paths.

   Alternative considered: keep tombstones for tailored error messages. Rejected because the desired contract is "only current fields are modeled."

6. Generated config output is full and explicit.

   `GenerateCUE(DefaultConfig())` and default config file creation must emit the complete effective config shape, including fields whose value is the CUE default. Generated output should optimize for inspectability and schema clarity, not brevity.

   Alternative considered: omit fields that equal CUE defaults and rely on schema defaulting during load. Rejected because generated config should be a concrete example of the full config shape.

## Risks / Trade-offs

- Default drift risk -> Derive `DefaultConfig()` from CUE evaluation, and keep a small regression test proving empty config decode and `DefaultConfig()` use the same path.
- Error message regression for old fields -> Accept this as part of the breaking change; closed-schema unknown-field errors are the intended behavior.
- Config generation churn -> Update `GenerateCUE(DefaultConfig())` expectations to include all effective fields and ensure generated config round-trips.
- LLM union complexity -> Keep the union small and explicit: none, provider, API. If a field is truly shared, factor it into a closed base struct that every branch embeds.
- Optional-vs-default confusion -> Prefer regular fields with CUE defaults for the effective config. Use optional fields only for values that are genuinely absent from the effective config, not merely omitted by the user.

## Migration Plan

1. Add or update tests that define the desired behavior before changing parser code.
2. Convert config schema fields from patch-style optional fields to default-bearing effective fields.
3. Adjust config decode so config files evaluate to a complete `Config`.
4. Remove invowkfile legacy tombstone fields.
5. Update generated config output to include all effective fields.
6. Update sync/behavioral tests, generated config tests, docs, and samples.
6. Run targeted schema packages, then repo gates required by agent-doc rules if docs change.

Rollback is straightforward: revert the schema/parser changes and restore patch DTO decoding. Because this is a proposal for a breaking cleanup, rollback should only be used if implementation exposes unacceptable compatibility fallout.

## Open Questions

None.
