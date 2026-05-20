## Why

Invowk currently uses a single implementation `script` string for both inline script bodies and script-file references, then relies on path-prefix and file-extension heuristics to decide which mode the user intended. Custom dependency checks still use a separate `check_script` string shape, which would leave one executable script surface inconsistent after the implementation-script cleanup. This change makes script intent explicit across both command implementations and custom checks while tightening the surrounding documentation for `workdir`, module-contained script files, and validation ownership.

## What Changes

- **BREAKING**: Replace implementation-level `script: string` with an explicit closed object using exactly one of `script.content` or `script.file`.
- **BREAKING**: Replace custom-check `check_script: string` with `script: {content: ...}` or `script: {file: ...}` inside custom check dependency entries and alternatives.
- **BREAKING CLEAN BREAK**: Remove the old implementation-script string shape and custom-check `check_script` shape completely; do not keep fallback parsing, dual-shape decoding, automatic conversion, aliases, or old script helpers after the change.
- Remove script file-detection heuristics from the user-facing contract; script-file references SHALL be selected by `script.file`, and inline scripts SHALL be selected by `script.content`.
- Restrict `script.file` to invowkfiles loaded from an invowkmod and require the resolved target file to stay contained in that same invowkmod. Non-module invowkfiles SHALL use `script.content`.
- Update all repository examples and fixtures that define command implementations or custom checks, including every command in the root `invowkfile.cue`, sample invowk modules, CLI test fixtures, docs snippets, generated schema references, README examples, website docs, and localized/i18n documentation.
- Clarify `workdir` documentation without relocating the field: `workdir` remains root-, command-, and implementation-scoped execution configuration that applies across native, virtual, and container runtimes with documented precedence.
- Clarify the CUE-vs-Go validation boundary for invowkfile script shape, script-file resolution, module containment, contextual command rules, and other semantic checks that intentionally remain outside pure CUE validation.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cue-schema-coherence`: Replace ambiguous implementation script strings and custom-check `check_script` strings with explicit `script.content` and `script.file` variants, document module-contained file semantics and `workdir` semantics, and make the validation ownership model visible across schemas, examples, docs, and tests.

## Impact

- `pkg/invowkfile/invowkfile_schema.cue`
- `pkg/invowkfile` structs, parsing, validation, schema sync tests, script resolution tests, custom-check dependency tests, and implementation tests, including removal of old implementation-script string wiring, old `check_script` wiring, and heuristic helpers
- `internal/runtime` script resolution call sites and tests
- `internal/provision` and container execution paths that consume resolved scripts or script paths
- `internal/app/deps` and `internal/app/commandadapters` custom-check host/container probes that consume resolved custom-check script content
- Root `invowkfile.cue` command definitions
- CLI testscript fixtures under `tests/cli/testdata/`
- Safe and audit sample modules under `samples/invowkmods/`
- README examples, website docs, generated schema/reference docs, MDX snippets, and i18n/localized documentation that show or describe command scripts, custom checks, script files, `workdir`, or validation behavior
- Agent-facing schema guidance only where it documents stale script or workdir behavior
