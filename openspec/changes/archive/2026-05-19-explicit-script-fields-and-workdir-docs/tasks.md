## 1. Script Schema and Go Model

- [x] 1.1 Add an implementation-script value object or struct that represents exactly one of inline content or file reference.
- [x] 1.2 Update `pkg/invowkfile/invowkfile_schema.cue` so `#Implementation.script` is a closed disjunction of `content` and `file` variants.
- [x] 1.3 Validate `script.content` with the existing script-content constraints and 10 MiB content limit.
- [x] 1.4 Validate `script.file` as a non-empty path-like string with an appropriate path length limit.
- [x] 1.5 Reject old implementation `script: "..."` strings during CUE parsing or post-decode validation.
- [x] 1.6 Reject `script: {}`, `script: {content: "", ...}`, `script: {file: "", ...}`, and objects containing both `content` and `file`.
- [x] 1.7 Update Go JSON tags, parsing structs, validation delegation, and schema sync expectations for the new nested script shape.
- [x] 1.8 Update `pkg/invowkfile/generate.go` so generated CUE emits `script.content` or `script.file` and round-trips through the parser.
- [x] 1.9 Remove the old `Implementation.Script ScriptContent` model for implementation scripts and the old custom-check `check_script`/`CheckScript ScriptContent` model; retain `ScriptContent` only for reusable inline content validation if still appropriate.
- [x] 1.10 Remove any fallback decoder, automatic converter, or dual-shape parsing path that accepts or rewrites old implementation script strings.
- [x] 1.11 Update `#CustomCheck` so custom checks use `script: {content: ...}` or `script: {file: ...}` and reject `check_script`.
- [x] 1.12 Update custom-check Go structs, JSON tags, validation delegation, schema sync expectations, and generated-CUE support for the new `script` field.

## 2. Script Resolution and Runtime Integration

- [x] 2.1 Delete or rename `Implementation.IsScriptFile()` so implementation script mode is no longer inferred from string contents.
- [x] 2.2 Replace script-file path helpers so `script.file` is accepted only when the source invowkfile is inside an invowkmod and resolves relative to that module root.
- [x] 2.3 Reject non-module `script.file` values and reject absolute, rooted, drive-qualified, UNC, or traversal values whose resolved target is outside the source invowkmod.
- [x] 2.4 Preserve and extend module containment validation before reading implementation or custom-check script files.
- [x] 2.5 Preserve execution-time filesystem reads in `internal/runtime.ScriptResolver` rather than moving file I/O into schema parsing.
- [x] 2.6 Update native, virtual, container, persistent-container, and provisioning call sites that resolve selected scripts or script paths.
- [x] 2.7 Ensure file-read failures and resolved-content validation errors remain actionable and include the selected script file path where relevant.
- [x] 2.8 Remove `scriptFileExtensions` or any equivalent implementation-script extension table used for mode detection.
- [x] 2.9 Remove path-prefix and drive-letter mode detection for implementation scripts.
- [x] 2.10 Resolve custom-check `script.file` values to validated script content before invoking host custom-check probes.
- [x] 2.11 Resolve runtime-level container custom-check `script.file` values on the host/module side, then execute the resolved content inside the container dependency probe.

## 3. Parser, Validation, and Runtime Tests

- [x] 3.1 Add parser tests for accepted `script.content` values, including single-line and multi-line content.
- [x] 3.2 Add parser tests for accepted module-contained `script.file` values, including extensionless paths and dot-prefixed paths.
- [x] 3.3 Add parser or validation tests for rejected old strings, old `check_script`, empty objects, duplicated variants, empty content, empty file paths, non-module files, and outside-module files.
- [x] 3.4 Add tests proving `script.content: "scripts/build.sh"` is treated as inline content and does not trigger a file read.
- [x] 3.5 Add tests proving module-contained `script.file: "scripts/build"` is treated as a file even without path-prefix or extension heuristics.
- [x] 3.6 Update script path traversal tests so implementation and custom-check module `script.file` values cannot escape module boundaries.
- [x] 3.7 Update native, virtual, and container runtime tests that construct `Implementation` values directly.
- [x] 3.8 Update generated-CUE tests so inline and file-backed scripts emit the new shape and parse back equivalently.
- [x] 3.9 Update schema sync and behavioral sync tests for the nested script object.
- [x] 3.10 Add negative tests proving no parser path accepts old implementation script strings.
- [x] 3.11 Add code-level or behavioral tests proving old file-detection heuristics no longer control implementation script mode.
- [x] 3.12 Add host and container custom-check tests for inline content, file content, non-module file rejection, missing files, invalid resolved content, `expected_code`, and `expected_output`.

## 4. First-Party Commands, Samples, and Fixtures

- [x] 4.1 Migrate every implementation script in the repository root `invowkfile.cue` to `script.content`.
- [x] 4.2 Preserve the meaning of any root `invowkfile.cue` file-backed example by inlining the script content or moving the file-backed example to a module fixture, because non-module invowkfiles cannot use `script.file`.
- [x] 4.3 Migrate safe sample modules under `samples/invowkmods/` to the explicit script shape for implementations and custom checks.
- [x] 4.4 Migrate audit fixture modules under `samples/invowkmods/` without weakening the unsafe behaviors they are intended to demonstrate.
- [x] 4.5 Update module creation templates such as `pkg/invowkmod/operations_create.go`.
- [x] 4.6 Update CLI testscript archives under `tests/cli/testdata/` that embed invowkfile command definitions or custom checks.
- [x] 4.7 Update Go test fixtures and helper constructors that embed CUE snippets, direct `Implementation` values, or direct custom-check values.
- [x] 4.8 Migrate every dependency custom-check example and fixture from `check_script` to `script.content` or module-contained `script.file`.

## 5. README, Website, Generated Docs, and i18n

- [x] 5.1 Update README command and custom-check examples to use `script.content` and module-contained `script.file`.
- [x] 5.2 Add or update README clean-break guidance showing old-to-new examples for implementation inline scripts, implementation script files, custom-check inline scripts, and custom-check script files.
- [x] 5.3 Update current website docs and MDX snippets that show implementation scripts or custom checks.
- [x] 5.4 Update generated invowkfile schema/reference documentation to describe `script.content`, `script.file`, exact-one semantics, module-only file semantics, rejected old implementation strings, and rejected old `check_script` fields.
- [x] 5.5 Update runtime-mode, implementation, dependency, custom-check, filepath, interpreter, and workdir docs where examples include executable scripts.
- [x] 5.6 Update localized/i18n docs and versioned docs that are part of the current documentation parity workflow.
- [x] 5.7 Update architecture docs or diagrams that show `script: "..."` examples.
- [x] 5.8 Update agent-facing schema/testing guidance only where it contains stale implementation-script examples, stale custom-check examples, module-file semantics, or stale workdir wording.

## 6. Workdir Documentation Clarification

- [x] 6.1 Update schema comments for root-, command-, and implementation-level `workdir` to state the effective precedence: CLI override, implementation, command, root, default.
- [x] 6.2 Document that `workdir` is execution-scoped and applies across native, virtual, and container runtimes.
- [x] 6.3 Document that `workdir` remains reusable at root, command, and implementation levels and is not being moved into the new `script` object.
- [x] 6.4 Clarify relative-path resolution from the invowkfile directory or module root.
- [x] 6.5 Clarify container workdir behavior for relative values and leading-slash absolute/container-absolute values.
- [x] 6.6 Ensure all workdir examples use the new explicit script shape while preserving the current behavior being demonstrated.

## 7. Validation Ownership Documentation

- [x] 7.1 Update schema comments and generated references to identify CUE as the owner of closed script shape, exact-one variant selection, non-empty strings, and local length constraints.
- [x] 7.2 Update code comments or docs to identify Go/runtime validation as the owner of script-file path resolution, module containment, filesystem reads, and resolved script-content validation.
- [x] 7.3 Update workdir comments/docs to identify Go/runtime logic as the owner of effective precedence, path resolution, container mapping, and execution-time directory existence checks.
- [x] 7.4 Update user-facing diagnostics where common script-shape mistakes would otherwise produce confusing CUE disjunction errors.
- [x] 7.5 Confirm the change does not include or rename dependency requirement `version` fields.

## 8. Verification

- [x] 8.1 Run repository-wide searches for old implementation `script: "..."`, `script: """`, `script: #"""`, `check_script:`, and `CheckScript` examples/wiring and finish only after current docs, examples, fixtures, generated references, and code no longer use them.
- [x] 8.2 Run targeted package tests for `pkg/invowkfile`, `internal/runtime`, `internal/provision`, `internal/app/deps`, `internal/app/commandadapters`, and other touched Go packages.
- [x] 8.3 Run CLI tests covering migrated testscript fixtures.
- [x] 8.4 Run sample module validation against safe sample modules.
- [x] 8.5 Run docs/snippet/i18n/website checks required by touched documentation files.
- [x] 8.6 Run `make check-agent-docs` if `.agents/`, `.claude/`, or `.codex/skills/` guidance changes.
- [x] 8.7 Run `openspec validate explicit-script-fields-and-workdir-docs --strict` and fix all reported issues.
- [x] 8.8 Record any verification command that cannot be run locally, with the blocker and nearest completed substitute.
- [x] 8.9 Run repository-wide searches for removed executable-script wiring names such as `scriptFileExtensions`, `IsScriptFile`, `Script: ScriptContent`, `CheckScript`, and `check_script` and finish only after no old implementation-script or custom-check script mode detection remains.
- [x] 8.10 Run validation/tests for module-only `script.file` behavior, including implementation and custom-check rejection from non-module invowkfiles.
