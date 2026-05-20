## Context

`pkg/invowkfile` currently models an implementation script as a single `ScriptContent` string. Runtime resolution then guesses whether that string is inline content or a file path using syntactic heuristics: path prefixes, Windows drive-letter shape, and known script extensions. The model is compact, but it makes ordinary cases ambiguous: `scripts/build.sh` means file mode because of the extension, while `scripts/build` means inline content unless the user writes `./scripts/build`.

Custom dependency checks currently use `check_script` as a plain validation script string. Leaving that field behind would keep a second executable-script shape in the same schema and would make docs explain an exception immediately after introducing explicit script sources. This update folds custom checks into the same clean-break design: custom checks use `script.content` for inline script text or `script.file` for a module-contained script file.

This change deliberately makes the invowkfile format less magical and more self-contained. Implementations and custom checks will declare either `script.content` for inline script text or `script.file` for a script-file reference. `script.file` is intentionally restricted to invowkfiles loaded from an invowkmod, and the resolved target file must stay inside that same invowkmod. Runtime filesystem reads still happen at the runtime/dependency-validation boundary, and module containment checks protect third-party module file reads.

The change is intentionally broad because user-facing examples are part of the contract. The root `invowkfile.cue`, README, Docusaurus docs, generated schema references, snippets, i18n/versioned docs, sample modules, and CLI fixtures all teach the current `script: "..."` or `check_script: "..."` shapes. Leaving those stale would keep the removed contract alive even if the parser rejects it.

## Goals / Non-Goals

**Goals:**

- Replace ambiguous implementation `script` strings with a closed `script` object using exactly one of `content` or `file`.
- Replace custom-check `check_script` strings with the same closed `script` object shape inside custom check entries and alternatives.
- Remove file-detection heuristics from the implementation script contract.
- Restrict `script.file` to module invowkfiles and require resolved script files to remain contained in the source invowkmod.
- Preserve current script-file file-read, content-validation, runtime, and module-containment protections while tightening non-module file references.
- Make extensionless script files expressible with `script.file` without requiring `./`.
- Keep `script.content` valid for inline script text regardless of file-like suffixes in the text.
- Migrate all repository-owned command definitions, custom-check definitions, tests, samples, README examples, website snippets/docs, generated references, and i18n/versioned documentation.
- Clarify `workdir` as execution-scoped configuration with existing precedence and runtime behavior; do not move it under a runtime-specific or implementation-only container.
- Clarify which invowkfile validations are CUE shape checks and which remain Go/runtime semantic checks.

**Non-Goals:**

- Do not rename or change `requires.version` or dependency version-constraint semantics.
- Do not change custom-check `expected_code`, `expected_output`, direct-vs-alternatives semantics, or host-vs-container validation placement beyond resolving the new script source shape.
- Do not move `workdir` into the implementation object only, into runtime config, or into the new `script` object.
- Do not change runtime selection, interpreter selection, dependency validation, environment inheritance, or container persistent-target behavior except where tests/fixtures must be updated for the new script shape.
- Do not keep any dual-shape parser, fallback decoder, automatic converter, retained implementation-script string path, or retained `check_script` alias.

## Decisions

1. Use `script.content` and `script.file` as the public shape.

   The schema should model `script` as a closed disjunction with two variants: a content variant requiring `content` and forbidding `file`, and a file variant requiring `file` and forbidding `content`. This keeps the existing domain noun `script` and makes intent explicit without introducing broader terminology such as `source`. `content` is preferred over `inline` because it names the value stored in the field, while `inline` names where the value appears.

   Alternative considered: `source.inline` and `source.file`. Rejected because `source` is broader than this feature and can be confused with module source, Git source, command source, or provenance.

   Custom checks should use the same field name, `script`, instead of making `check_script` an object. `script.content` and `script.file` read consistently across implementations and custom checks, while `check_script.content` repeats the noun and preserves the old field name in the new contract.

   Alternative considered: `check_script: {content: ...}` and `check_script: {file: ...}`. Rejected because this is a clean break and the more coherent shape is worth the one-time rename.

2. Treat the change as a clean break.

   The CUE schema and Go parser should reject old implementation `script` strings and old custom-check `check_script` strings, and the implementation should remove the old wiring that made implementation scripts and custom checks plain `ScriptContent` values. The finished code should have one public executable-script shape, not two accepted shapes and not a hidden converter between them.

   The implementation should delete or repurpose removed helpers such as extension lists, heuristic file detection, and string-based implementation script resolution so future code cannot accidentally keep depending on the old model.

3. Restrict `script.file` to module-contained files.

   A file-backed script source should only be accepted when the source `invowkfile.cue` was loaded from an invowkmod. The file path should resolve relative to that module root, and the resolved target must remain inside the same module before any file read occurs. Non-module invowkfiles, including the repository root `invowkfile.cue`, must use `script.content`.

   This rule applies equally to implementation scripts and custom-check scripts. It keeps file-backed scripts portable with the module that owns them and avoids teaching root/project-local file references as a general user-facing contract.

   Alternative considered: allowing non-module `script.file` values relative to the invowkfile directory. Rejected because it weakens the self-contained-module model and creates a different trust boundary for the same field depending on where it appears.

4. Keep file path resolution at the runtime/file-read boundary.

   The invowkfile model should expose enough information to tell whether a script is content or a file, but it should not perform filesystem I/O. `internal/runtime.ScriptResolver` should remain the execution-time boundary that reads implementation script files. Custom-check validation should resolve file-backed custom-check scripts before invoking host/container probes, so probes continue to receive script content.

   The Go model can be a new value type or struct such as `ImplementationScript` with `Content` and `File` fields, plus helper methods that replace `IsScriptFile`, `GetScriptFilePathWithModule`, and `ResolveScriptWithFSAndModule`. The exact type name is an implementation detail, but the public JSON/CUE field names are not. Any removed method names that remain must delegate to explicit `script.file` behavior only and must not preserve string-shape inference.

   For runtime-level custom checks inside container runtime configuration, the host/module file is read and validated first, then the resolved script content is executed inside the container using the same dependency probe path as current inline checks. The `script.file` path is not interpreted as a container path.

5. Keep `workdir` schema placement unchanged and improve documentation only.

   `workdir` currently applies across native, virtual, and container execution. Its effective precedence is CLI override, implementation, command, root, then default invowkfile or module directory. That model is coherent and reusable; the weak spot is documentation clarity, not field placement.

   Alternative considered: moving `workdir` into implementation or runtime-specific config. Rejected because it would make reusable command- and root-level execution defaults harder and would make a runtime-neutral feature look runtime-specific.

6. Make validation ownership explicit.

   CUE should own local structural shape: closed structs, required fields, exactly-one `script.content` or `script.file`, non-empty strings, and maximum lengths. Go/runtime validation should own context-sensitive behavior: whether the source invowkfile belongs to a module, script-file path resolution, module containment, filesystem reads, resolved script-content validation, command hierarchy checks, selected runtime behavior, and working-directory existence at execution time.

## Risks / Trade-offs

- Existing docs and fixtures may miss a `script: "..."` or `check_script: "..."` occurrence -> Mitigate with repository-wide searches in CUE, Go string fixtures, testscript archives, README, website docs, snippets, versioned docs, i18n, samples, and agent guidance before final verification.
- The CUE disjunction may produce less friendly errors when users omit or duplicate `content` and `file` -> Mitigate with targeted parser/preflight validation or focused error tests for common mistakes.
- Go struct churn may touch many tests that construct `Implementation` or `CustomCheck` values directly -> Mitigate with small helper constructors in tests only where they reduce repetitive noise without hiding the new public shape.
- Generated docs or snippets may update many versioned/i18n files -> Mitigate by treating docs parity as required scope and running the existing docs integrity checks for touched surfaces.
- `script.file` could be mistaken for a host path or a container path -> Mitigate with docs that state file-backed script sources are module-contained host/module reads whose resolved content is then executed by the selected runtime.
- Root `invowkfile.cue` file-backed examples may no longer be valid -> Mitigate by inlining those examples with `script.content` or moving file-backed examples into sample modules.
- Workdir docs could drift into implying runtime-specific placement -> Mitigate by documenting the same precedence in schema comments, README/website docs, and examples, and by linking it to the existing effective workdir tests rather than changing behavior.

## Migration Plan

1. Introduce the shared explicit script model and CUE schema variants for implementation scripts and custom-check scripts.
2. Remove old implementation-script string parsing, custom-check `check_script` parsing, heuristic file detection, and string-based helper wiring.
3. Update parser, validation, generation, runtime script resolution, dependency custom-check probes, and affected tests.
4. Enforce module-only `script.file` semantics for both implementation scripts and custom-check scripts.
5. Migrate root `invowkfile.cue`, sample modules, testscript fixtures, and generated command fixtures, inlining root file-backed examples where needed because root invowkfiles cannot use `script.file`.
6. Migrate README, website docs/snippets/reference pages, generated schema references, i18n/versioned docs, and agent guidance that shows implementation scripts or custom checks.
7. Run schema sync, targeted package tests, CLI tests that cover invowkfile parsing, docs parity/build checks required by touched files, and `openspec validate explicit-script-fields-and-workdir-docs --strict`.

Rollback during implementation is a source-control revert of the branch. The delivered change itself must remain a single-shape schema and implementation.

## Decision

Use separate Go wrapper types per surface: `ImplementationScript` for command implementations and `CustomCheckScript` for dependency custom checks. They share the same CUE shape and module-file containment policy, while keeping validation errors and generated output phrased in the vocabulary of each surface.
