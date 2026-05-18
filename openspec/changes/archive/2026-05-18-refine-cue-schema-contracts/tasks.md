## 1. Contract Inventory

- [x] 1.1 Search code, schemas, docs, snippets, samples, and agent docs for old command dependency syntax, `#LLMNoBackendConfig`, broad `..` containerfile wording, and ambiguous CUE-vs-Go validation comments.
- [x] 1.2 Record the exact affected files before editing so implementation can verify that no stale references remain.
- [x] 1.3 Identify generated or versioned documentation surfaces that must be updated directly versus regenerated.

## 2. Source-Qualified Command Dependency References

- [x] 2.1 Add a dedicated command dependency reference value type that parses bare command refs and `@source command` refs without reusing raw `CommandName` as the full alternative type.
- [x] 2.2 Update the invowkfile CUE schema so `depends_on.cmds[].alternatives` accepts bare command refs and explicit `@source command` refs, including dotted source IDs.
- [x] 2.3 Remove support for treating `tools lint` or `com.company.tools lint` as source-qualified command dependency syntax.
- [x] 2.4 Update command dependency resolution so bare refs resolve only against the declaring command's own source.
- [x] 2.5 Update source-qualified resolution to match by source ID and command name before applying same-source, direct-dependency, and global-source scope policy.
- [x] 2.6 Update dependency diagnostics to preserve user-written refs such as `@tools lint` and distinguish missing source/command from forbidden out-of-scope sources.
- [x] 2.7 Add CUE/Go behavioral sync tests for accepted and rejected command dependency refs.
- [x] 2.8 Add dependency resolution tests for local bare refs, direct dependency source refs, dotted source IDs, same-source refs, unknown sources, missing commands, and forbidden sources.

## 3. Containerfile Path Policy

- [x] 3.1 Remove broad textual `..` rejection from the CUE `containerfile` field while keeping non-empty, length, and relative-looking shape constraints.
- [x] 3.2 Update Go containerfile path validation to reject any raw path segment exactly equal to `..` after normalizing backslashes to slashes, before path cleaning can hide the segment.
- [x] 3.3 Keep rejecting absolute Unix paths, Windows drive-qualified paths, UNC/rooted paths, NUL bytes, invalid filename components, and paths outside the invowkfile directory.
- [x] 3.4 Preserve acceptance for `Containerfile`, `docker/Containerfile`, `./Containerfile`, `docker/./Containerfile`, `Containerfile..backup`, and `docker/v1..2/Containerfile`.
- [x] 3.5 Add tests for slash and backslash parent segments, internal dot segments, filenames containing consecutive dots, and runtime build path resolution relative to `invowkfile.cue`.

## 4. Runtime Diagnostics And Validation Ownership

- [x] 4.1 Add targeted invowkfile diagnostics for container-only runtime fields on native and virtual runtimes.
- [x] 4.2 Add targeted diagnostics for virtual runtime interpreter overrides.
- [x] 4.3 Ensure missing container source and duplicate `image` plus `containerfile` errors remain actionable and include the runtime field path.
- [x] 4.4 Update schema comments for runtime source invariants, args, flags, command hierarchy, regex safety, filesystem paths, and other Go-only validations to state whether CUE or Invowk owns each rule.
- [x] 4.5 Add tests asserting actionable diagnostics for representative native, virtual, and container runtime failures.

## 5. LLM Config Schema Naming

- [x] 5.1 Rename `#LLMNoBackendConfig` to `#LLMDefaultsConfig` in `internal/config/config_schema.cue`.
- [x] 5.2 Remove all references to the old helper name instead of keeping a compatibility alias.
- [x] 5.3 Update config schema comments so top-level `llm.model`, `llm.timeout`, and `llm.concurrency` are documented as common backend defaults.
- [x] 5.4 Document and test that `llm.api.model` overrides top-level `llm.model` for API-backed execution.
- [x] 5.5 Update config schema sync and snippet/reference tests so stale `#LLMNoBackendConfig` references fail.

## 6. Documentation And Examples

- [x] 6.1 Update README command dependency examples to use local bare refs and explicit `@source command` refs.
- [x] 6.2 Update website current docs and snippet data for command dependencies, module source examples, CLI disambiguation consistency, containerfile path policy, LLM defaults naming, and validation ownership.
- [x] 6.3 Update versioned snippet data or generated reference snippets where they are part of the current documentation source.
- [x] 6.4 Update samples and schema reference snippets that mention affected fields or old dependency syntax.
- [x] 6.5 Update `.agents` rules, skills, commands, and AGENTS indexes only where they contain stale affected behavior.
- [x] 6.6 Search after docs edits for old source-prefix-with-space dependency examples, `#LLMNoBackendConfig`, and broad "cannot contain `..`" containerfile wording; remove or rewrite every current-behavior leftover.

## 7. Verification

- [x] 7.1 Run targeted Go tests for `pkg/invowkfile`, `internal/app/deps`, `internal/config`, `internal/runtime`, and `internal/container`.
- [x] 7.2 Run schema sync, behavioral sync, and CLI testscript coverage for affected invowkfile/config behavior.
- [x] 7.3 Run documentation integrity checks, including `make check-agent-docs` if any AGENTS, rules, skills, or agent command docs changed.
- [x] 7.4 Run repository baseline checks required by the touched surfaces, including lint and focused integration tests.
- [x] 7.5 Run `openspec validate refine-cue-schema-contracts --strict` and fix all reported issues.
- [x] 7.6 Perform a final clean-break search for removed syntax and helper names before marking implementation complete.
