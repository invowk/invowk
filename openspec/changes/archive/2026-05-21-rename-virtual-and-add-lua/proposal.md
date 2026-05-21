## Why

The current `virtual` runtime name is ambiguous now that Invowk is adding another Go-native interpreter. This change makes the virtual runtime family explicit, coherent, and safe by default: `virtual-sh` for embedded mvdan/sh and `virtual-lua` for embedded Lua.

## What Changes

- **BREAKING**: Remove the legacy `virtual` runtime name from every user-facing selector. `virtual` MUST NOT be accepted as an alias in `invowkfile.cue`, config `default_runtime`, CLI runtime overrides, generated output, docs, samples, snippets, or tests.
- **BREAKING**: Rename the embedded shell runtime to `virtual-sh` across schemas, Go runtime names, CLI output, dry-run output, docs, i18n, samples, and test fixtures.
- **BREAKING**: Replace config `virtual_shell` with a family-level `virtual` config block. `virtual` remains valid only as a config namespace, not as a runtime selector.
- Add `virtual-lua` as a first-class built-in runtime powered by `github.com/arnodel/golua`.
- Redefine `virtual-*` runtimes as a Go-native safety harness:
  - VM-controlled filesystem access is path-sanitized.
  - Host binary execution is denied by default.
  - u-root built-ins remain Go-native and sandboxed.
  - Host binaries, once explicitly allowed and launched, are outside the Go-level filesystem sandbox; use `container` for process-level isolation.
- Add virtual runtime controls:
  - `allowed_binaries`: list of host binaries allowed to run; empty or omitted means no host binaries; `["*"]` explicitly allows all host binaries.
  - `binary_lookup_mode`: `"host"` by default, or `"strict"` for hardcoded platform system paths.
  - `platforms[].virtual.filesystem.access`: `"restricted"` by default, or `"full"` for explicit broad host filesystem access by VM-controlled operations.
  - `platforms[].virtual.filesystem.paths`: platform-scoped logical path mappings exposed as script handles.
  - `virtual.utilities.enabled`: config-level toggle for Invowk-provided Go-native utility commands shared by `virtual-sh` and `virtual-lua`.
- Add cross-platform logical anchors and shell/Lua bridge exposure:
  - Lua: `invowk.path(...)`, `invowk.env`, `invowk.state`, `invowk.cmd`, and `invowk.capture`.
  - Shell: `INVOWK_ANCHOR_*`, `INVOWK_PATH_*`, and `INVOWK_STATE_*`.
- Extend deterministic and LLM-assisted audit behavior so Lua scripts and self-contained Lua files in invowkmods are included in module security review.
- Update README, website docs, versioned docs where applicable, Portuguese i18n, snippets, samples, generated examples, architecture diagrams if affected, and the full test suite.

## Capabilities

### New Capabilities
- `virtual-runtime-sandbox`: Shared path validation, host-binary gating, binary lookup modes, and safety-boundary behavior for all `virtual-*` runtimes.
- `cross-platform-path-anchors`: Standard anchors, platform-scoped virtual filesystem paths, platform resolution, and shell/Lua path exposure.
- `virtual-lua-interpreter`: Lua runtime execution, Lua bridge API, Lua stdlib restrictions, resource limits, arguments, interactive streams, and module-local require behavior.
- `virtual-lua-audit`: Deterministic and LLM-assisted auditing for Lua inline scripts, script files, and self-contained Lua module files.

### Modified Capabilities
- `script-scoped-interpreters`: Rename `virtual` to `virtual-sh`, add `virtual-lua` interpreter validation, and reject the legacy `virtual` runtime name everywhere user-authored runtime names are accepted.
- `script-interpreter-diagnostics`: Update dry-run and advisory diagnostics for explicit `virtual-sh` and `virtual-lua` interpreter normalization plus virtual sandbox metadata.

## Impact

- **Schemas and generated config**: `pkg/invowkfile/invowkfile_schema.cue`, `internal/config/config_schema.cue`, generated config output, schema sync tests, and all Go types using runtime names.
- **Runtime layer**: split/rename embedded shell runtime, add Lua runtime, share path validation, binary gating, anchor resolution, and bridge injection helpers.
- **Command execution**: registry wiring, runtime selection, CLI `--ivk-runtime`, dry-run output, interactive execution, dependency/custom-check behavior, and runtime diagnostics.
- **Security audit**: `internal/audit/`, Lua script/file discovery, deterministic risk checks, and LLM/subagent instructions.
- **Documentation and examples**: README, website docs, versioned docs policy for current docs, Portuguese i18n, snippets, diagrams if affected, samples, and all user-facing examples.
- **Tests**: Go unit tests, schema sync tests, runtime tests, audit tests, CLI testscript fixtures, virtual/native mirror fixtures where applicable, Windows/macOS/Linux path behavior, and docs integrity checks.
