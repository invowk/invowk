## Why

Interpreter selection is currently modeled on runtime configuration even though it describes how a specific script should be read and executed. Moving `interpreter` onto `script` makes implementations and custom dependency checks use the same source-plus-interpreter contract, removes runtime-level ambiguity, and gives custom checks first-class shebang and explicit-interpreter support.

## What Changes

- **BREAKING** Remove `interpreter` from runtime configuration shapes; `runtimes: [{name: "native", interpreter: "python3"}]` and container equivalents become invalid closed-schema fields.
- **BREAKING** Remove all runtime-level interpreter implementation, generated output, tests, documentation, and helper APIs that only exist to support the old location.
- Add optional `script.interpreter` to both implementation scripts and custom-check scripts, alongside `script.content` or `script.file`.
- Keep `script.content` and `script.file` mutually exclusive while allowing `script.interpreter` with either source variant.
- Resolve shebangs from the final script bytes for both inline and file-backed scripts when `script.interpreter` is omitted or set to `"auto"`.
- Make explicit `script.interpreter` take precedence over shebang detection for both implementations and custom checks.
- Keep `script.file` module-only and module-contained for implementations and custom checks; interpreter metadata does not loosen file source containment.
- Execute host custom checks with a portable default shell path based on the existing virtual shell machinery instead of assuming `/bin/sh`, Git Bash, PowerShell, or `cmd.exe` exists in a fixed location.
- Preserve runtime environment selection separately from interpreter selection: native runs host interpreters, container runs container interpreters, and virtual uses the embedded mvdan/sh shell for shell-compatible scripts only.
- Update README, website docs, current i18n docs, snippets, generated references, samples, tests, benchmark fixtures, and LLM authoring/docs surfaces so no runtime-level interpreter examples remain.
- Do not add compatibility shims, legacy aliases, dual-write/dual-read logic, fallback parsing, migration warnings, or silently ignored old fields.

## Capabilities

### New Capabilities
- `script-scoped-interpreters`: Defines script-level interpreter semantics for implementations and custom checks, including schema shape, clean-break rejection of runtime-level interpreters, virtual-shell limits, module-contained file handling, docs, and tests.

### Modified Capabilities
- `cue-schema-coherence`: The invowkfile schema and Go model must remain coherent while `interpreter` moves from runtime configs onto script source structs.

## Impact

- CUE schema: `pkg/invowkfile/invowkfile_schema.cue`.
- Go model and validation: `pkg/invowkfile` runtime, implementation, dependency, interpreter, generation, parse, sync, and validation code/tests.
- Runtime execution: `internal/runtime` native, virtual, container execution and interpreter validation paths.
- Dependency execution: `internal/app/deps` and `internal/app/commandadapters` host/container custom-check probes.
- Fixtures and benchmarks: `tests/cli/testdata`, samples, `scripts/bench-bmf.mjs`, and any generated invowkfile snippets.
- Documentation: `README.md`, website current docs, Portuguese current i18n docs, snippets under `website/src/components/Snippet`, reference docs, command-authoring/LLM docs, and documentation review surfaces.
- Verification: OpenSpec validation, CUE/schema sync tests, targeted Go tests, CLI testscript coverage for native/virtual/container behavior, docs checks, agent docs checks when applicable, and repo guardrails.
