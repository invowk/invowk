## 1. Schema And Model Clean Break

- [x] 1.1 Remove `interpreter` from native and container runtime CUE schema variants in `pkg/invowkfile/invowkfile_schema.cue`.
- [x] 1.2 Add a reusable script-level interpreter constraint to the CUE schema and wire it into both `#ImplementationScript` and `#CustomCheckScript`.
- [x] 1.3 Add `Interpreter InterpreterSpec` to `ImplementationScript` and `CustomCheckScript` with JSON field name `interpreter`.
- [x] 1.4 Remove `Interpreter` from Go `RuntimeConfig` and delete runtime-level validation paths that only exist for that field.
- [x] 1.5 Replace `RuntimeConfig` interpreter helper methods with script-level helper functions or methods.
- [x] 1.6 Ensure `ImplementationScript.Validate()` and `CustomCheckScript.Validate()` validate interpreter only when present and still require exactly one source.
- [x] 1.7 Preserve `script.file` module-only and module-contained checks for implementation scripts and custom-check scripts when interpreter is present.
- [x] 1.8 Update CUE generation so implementation scripts and custom-check scripts emit `script.interpreter`, and runtime generation cannot emit `interpreter`.
- [x] 1.9 Remove or rewrite all Go tests and helpers that construct `RuntimeConfig{Interpreter: ...}`.
- [x] 1.10 Search the codebase for runtime-level interpreter references and delete stale design, validation, generation, and test leftovers rather than keeping compatibility shims.

## 2. Runtime Execution

- [x] 2.1 Rewire native runtime interpreter resolution to read `ctx.SelectedImpl.Script.Interpreter` after resolving inline or file script content.
- [x] 2.2 Rewire container runtime interpreter resolution to read `ctx.SelectedImpl.Script.Interpreter` after resolving inline or file script content.
- [x] 2.3 Update virtual runtime validation so omitted, auto, and shell-compatible script interpreters use mvdan/sh.
- [x] 2.4 Update virtual runtime validation so non-shell script interpreters or non-shell shebangs fail before execution.
- [x] 2.5 Ensure virtual runtime does not require host `/bin/sh`, `bash`, PowerShell, or `cmd.exe` for shell-compatible script execution.
- [x] 2.6 Ensure shebang detection for implementation `script.file` uses file bytes after module containment validation.
- [x] 2.7 Keep positional argument behavior unchanged for native, virtual, and container scripts after the interpreter move.
- [x] 2.8 Keep interpreter allowlist and unsafe interpreter diagnostics intact for script-level explicit interpreters.

## 3. Custom Check Execution

- [x] 3.1 Preserve custom-check `script.interpreter` when resolving `script.file` into executable script content.
- [x] 3.2 Replace host custom-check fixed shell path execution with a portable execution path based on the embedded virtual shell for shell-compatible checks.
- [x] 3.3 Add host custom-check execution for allowlisted non-shell explicit interpreters and non-shell shebangs.
- [x] 3.4 Ensure host custom-check diagnostics identify the check name when an explicit interpreter is missing or fails.
- [x] 3.5 Rewire container custom-check execution so selected container runtime checks honor `script.interpreter` and shebang detection inside the container.
- [x] 3.6 Keep custom-check expected exit-code and expected-output validation unchanged after interpreter execution.
- [x] 3.7 Ensure custom-check alternatives preserve per-check interpreter settings and early-return semantics.
- [x] 3.8 Remove stale comments claiming custom checks run through native shell paths.

## 4. Parser, Schema, And Unit Tests

- [x] 4.1 Add CUE parser tests accepting `script.interpreter` on implementation `script.content`.
- [x] 4.2 Add CUE parser tests accepting `script.interpreter` on implementation module `script.file`.
- [x] 4.3 Add CUE parser tests accepting `script.interpreter` on custom-check `script.content` and module `script.file`.
- [x] 4.4 Add parser rejection tests for native, virtual, and container runtime blocks containing `interpreter`.
- [x] 4.5 Add parser rejection tests for script objects with only `interpreter` and no `content` or `file`.
- [x] 4.6 Add parser or validation tests for empty, whitespace-only, unsafe, and over-length script-level interpreters.
- [x] 4.7 Update schema sync tests so script structs and CUE script definitions agree on `interpreter`.
- [x] 4.8 Update schema sync tests so runtime configs and CUE runtime definitions agree that `interpreter` is absent.
- [x] 4.9 Update generation tests for implementation and custom-check script-level interpreter output.
- [x] 4.10 Add a regression test that generated runtime configs never include `interpreter`.

## 5. Runtime And Dependency Tests

- [x] 5.1 Update native runtime interpreter tests to use `script.interpreter` for inline scripts.
- [x] 5.2 Update native runtime interpreter tests to cover file-backed scripts with shebang and explicit `script.interpreter`.
- [x] 5.3 Update container runtime interpreter tests to use `script.interpreter` for inline and file-backed scripts.
- [x] 5.4 Update virtual runtime tests to accept omitted, auto, and shell-compatible script interpreters.
- [x] 5.5 Update virtual runtime tests to reject explicit non-shell interpreters and non-shell shebangs.
- [x] 5.6 Add host custom-check tests for omitted interpreter using embedded virtual shell.
- [x] 5.7 Add host custom-check tests for shell-compatible explicit interpreter and shebang using embedded virtual shell.
- [x] 5.8 Add host custom-check tests for explicit non-shell interpreter success and missing-interpreter failure.
- [x] 5.9 Add container custom-check tests for omitted interpreter, shebang interpreter, and explicit non-shell interpreter.
- [x] 5.10 Update CLI testscript fixtures for native and virtual custom checks so they pass on Windows without fixed shell paths.
- [x] 5.11 Add or update CLI testscript fixtures that reject runtime-level interpreter fields.
- [x] 5.12 Update benchmark fixture generation in `scripts/bench-bmf.mjs` and add coverage or smoke verification for generated invowkfile shapes.

## 6. Documentation, Samples, And Authoring Surfaces

- [x] 6.1 Update `README.md` interpreter sections, custom-check sections, examples, and security notes to use only `script.interpreter`.
- [x] 6.2 Update current website docs for advanced interpreters, native runtime, virtual runtime, container runtime, custom checks, invowkfile schema reference, and implementation concepts.
- [x] 6.3 Update current Portuguese i18n docs for the same interpreter, runtime, custom-check, and schema surfaces.
- [x] 6.4 Update website snippet definitions so all interpreter snippets place `interpreter` inside `script`.
- [x] 6.5 Confirm versioned docs and i18n mirrors do not require edits for this unreleased clean break under the docs workflow; current docs and snippets carry the new contract.
- [x] 6.6 Update samples and invowkmod fixtures so no examples use runtime-level interpreter fields.
- [x] 6.7 Update LLM-assisted command authoring prompts, repair guidance, schemas, examples, and docs so generated commands use `script.interpreter`.
- [x] 6.8 Update dependency/custom-check docs to describe shell-compatible host checks using the embedded virtual shell by default.
- [x] 6.9 Add before/after documentation examples showing old runtime-level interpreter as invalid and new script-level interpreter as valid.
- [x] 6.10 Run a targeted stale-reference search for `interpreter:` near `runtimes` and fix every current-doc, snippet, sample, test, and generated-fixture hit.

## 7. Verification

- [x] 7.1 Run `openspec validate move-interpreter-to-script --strict`.
- [x] 7.2 Run targeted `go test` for `pkg/invowkfile`, `internal/runtime`, `internal/app/deps`, and `internal/app/commandadapters`.
- [x] 7.3 Run CLI testscript suites covering native, virtual, container dependency checks, and parser rejection fixtures.
- [x] 7.4 Run schema sync checks for invowkfile CUE and Go struct parity.
- [x] 7.5 Run docs validation/build/typecheck commands required by the docs workflow after website or snippet updates.
- [x] 7.6 Run `make check-agent-docs` if agent, rule, skill, or authoring docs are changed.
- [x] 7.7 Run repository guardrails such as `make check-baseline`, `make lint`, `make check-file-length`, and `git diff --check`.
- [x] 7.8 Confirm final searches show no compatibility shim, runtime-level interpreter field, generated old-shape example, or stale docs reference remains.
