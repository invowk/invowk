## Context

`move-interpreter-to-script` defines interpreter selection as script-scoped metadata. The current contract is intentionally simple: resolve `script.content` or module-contained `script.file` to final bytes, then use `script.interpreter` when explicit; otherwise use `"auto"` shebang detection and finally the runtime default shell behavior.

That model is correct, but a checked-in script file can still communicate a different intent through its shebang. For example, `script: {file: "scripts/build", interpreter: "python3"}` can point to a file that starts with `#!/bin/sh`. Execution should still use `python3`, but users should not have to infer that from a later syntax error.

## Goals / Non-Goals

**Goals:**
- Preserve explicit `script.interpreter` precedence over shebang detection.
- Emit advisory diagnostics when an explicit interpreter overrides a different resolved shebang.
- Make dry-run output disclose interpreter source and shebang override state.
- Cover inline scripts, module-contained `script.file`, implementation scripts, custom checks, native/container execution planning, and virtual-runtime shell compatibility.
- Keep diagnostics stable across platforms by reporting authored `script.file` paths when available.

**Non-Goals:**
- No schema-breaking change to `script.interpreter`, `script.content`, or `script.file`.
- No file-extension interpreter inference.
- No rejection of valid explicit interpreter plus shebang configurations.
- No attempt to execute or resolve host/container interpreter paths during static validation.

## Decisions

### Decision 1: explicit interpreter remains authoritative

`script.interpreter` continues to override shebang detection whenever it is set to a concrete interpreter value. This includes inline content and resolved module-contained file content. The only interpreter values that read the shebang for selection are an omitted field and `"auto"`.

Alternative considered: reject explicit interpreter plus mismatched shebang as invalid. Rejected because explicit configuration is a legitimate override mechanism for non-portable or stale shebangs.

### Decision 2: introduce advisory interpreter diagnostics

Add a reusable interpreter analysis step that consumes:
- the script source metadata (`content` or `file`)
- the resolved script bytes
- the parsed `script.interpreter`
- the parsed shebang, if present
- the selected runtime context when available

The analysis produces advisory diagnostics rather than validation errors. A shebang override diagnostic is emitted when:
- `script.interpreter` is a concrete explicit value, not empty and not `"auto"`
- the resolved script bytes contain a valid shebang
- the explicit interpreter selection differs from the parsed shebang selection

The diagnostic should include the script surface, authored file path when the source is `script.file`, explicit interpreter, shebang interpreter, and the fact that the explicit interpreter wins.

Alternative considered: only show the information in dry-run. Rejected because `invowk validate` is the natural command for catching surprising authoring issues before execution.

### Decision 3: interpreter equivalence should avoid noisy warnings

Diagnostics should compare parsed interpreter selections, not raw strings. `/usr/bin/env python3` and `python3` should be treated as equivalent when they parse to the same interpreter and arguments. Different arguments are meaningful because explicit interpreter arguments also override shebang arguments.

For shell-compatible interpreters under the virtual runtime, the diagnostic should account for virtual semantics: explicit shell-compatible names and shell shebangs both map to mvdan/sh intent. A virtual implementation should warn only when the explicit shell-compatible interpreter overrides a non-equivalent shell shebang in a way users should see, and it should continue to reject non-shell interpreter selections through the existing virtual-runtime error path.

Alternative considered: compare only interpreter base names. Rejected because it hides differences such as `python3` versus `python3 -u`.

### Decision 4: dry-run displays interpreter provenance

Dry-run output should show the effective interpreter decision next to the resolved script information:
- `explicit` when a concrete `script.interpreter` is used
- `shebang` when omitted or `"auto"` selects a shebang
- `default shell` when no interpreter is selected
- `virtual shell` when the virtual runtime accepts shell-compatible interpreter intent and runs mvdan/sh

When an explicit interpreter overrides a shebang, dry-run should include the warning inline with the script details so users can see the cause before execution.

### Decision 5: diagnostics flow through custom checks too

Custom checks use the same script object model as implementations, so they need the same interpreter analysis after file-backed checks are resolved. Host checks should report advisory warnings before choosing virtual versus native execution. Container checks should preserve warnings while building the validation script that runs inside the selected container runtime.

## Risks / Trade-offs

- [Risk] Advisory warnings can become noisy for intentional overrides. -> Mitigation: do not warn for omitted/`"auto"` interpreter and treat parsed-equivalent interpreter selections as non-conflicting.
- [Risk] Static validation may not have complete runtime context for container-only details. -> Mitigation: keep diagnostics about declared script metadata and parsed bytes only; do not check interpreter availability.
- [Risk] Dry-run output can become cluttered. -> Mitigation: keep the normal case compact and only add override detail when a shebang is present or the user asks for verbose output if existing dry-run patterns require that.
- [Risk] Virtual runtime shell compatibility can be confusing. -> Mitigation: phrase virtual diagnostics in terms of mvdan/sh execution rather than implying host `/bin/sh` or `bash` is used.

## Migration Plan

This is additive and non-breaking. Existing configurations continue to execute with the same interpreter precedence. Users with intentional overrides may see new advisory warnings and can either leave them in place or change the file shebang to match the explicit interpreter.

## Open Questions

- Should warnings be shown by default in `invowk validate`, or should they require a verbose flag if the current command has no warning channel?
- Should dry-run always show interpreter provenance, or only show override details unless verbose output is requested?
