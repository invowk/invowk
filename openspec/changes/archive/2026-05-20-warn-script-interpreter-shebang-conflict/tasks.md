## 1. Interpreter Analysis

- [x] 1.1 Add a reusable script interpreter analysis helper that accepts resolved script bytes, source metadata, and `script.interpreter`.
- [x] 1.2 Detect explicit interpreter overrides only when the explicit interpreter is concrete, the resolved bytes contain a valid shebang, and the parsed selections are non-equivalent.
- [x] 1.3 Treat omitted interpreter and `interpreter: "auto"` as shebang-driven selection and suppress override diagnostics.
- [x] 1.4 Compare parsed interpreter and argument selections so equivalent `/usr/bin/env` and direct forms do not warn, while argument differences do warn.
- [x] 1.5 Preserve existing execution precedence so explicit `script.interpreter` remains authoritative for native and container execution.

## 2. Diagnostic Surfacing

- [x] 2.1 Wire advisory diagnostics into implementation script resolution after `script.file` containment validation and file reads.
- [x] 2.2 Surface override diagnostics through `invowk validate` without converting valid scripts into validation failures.
- [x] 2.3 Update dry-run rendering to show interpreter provenance for explicit, shebang-detected, default-shell, and virtual-shell decisions.
- [x] 2.4 Include override warning details in dry-run output when explicit interpreter and shebang differ.
- [x] 2.5 Ensure diagnostics report authored `script.file` paths when available rather than host-specific resolved paths.

## 3. Custom Checks

- [x] 3.1 Apply interpreter override analysis to resolved host custom-check scripts before host check execution.
- [x] 3.2 Apply interpreter override analysis to resolved container custom-check scripts before container validation execution.
- [x] 3.3 Preserve host custom-check virtual-shell routing for shell-compatible interpreter intent.
- [x] 3.4 Preserve existing missing-interpreter and virtual non-shell rejection behavior.

## 4. Tests

- [x] 4.1 Add unit tests for interpreter override analysis, including auto, omitted, equivalent, differing interpreter, and differing argument cases.
- [x] 4.2 Add runtime tests proving explicit-over-shebang precedence for inline and module-contained `script.file` implementations.
- [x] 4.3 Add dry-run CLI tests for interpreter provenance and override warning output.
- [x] 4.4 Add dependency tests for host and container custom-check override diagnostics.
- [x] 4.5 Add regression tests ensuring invalid or unreadable `script.file` sources do not emit shebang override diagnostics.

## 5. Documentation and Verification

- [x] 5.1 Update README or website docs that explain `script.interpreter` precedence, `auto`, shebang detection, and override warnings.
- [x] 5.2 Update generated or maintained schema/reference docs if they describe dry-run or interpreter behavior.
- [x] 5.3 Run `openspec validate warn-script-interpreter-shebang-conflict --strict`.
- [x] 5.4 Run targeted Go and CLI tests covering interpreter resolution, dry-run output, custom checks, and runtime execution.
- [x] 5.5 Run repo guardrails required for touched areas, including `make check-agent-docs` if agent docs change.
