## 1. Runtime Names And Schema Shape

- [x] 1.1 Rename `#RuntimeType` in `pkg/invowkfile/invowkfile_schema.cue` from `virtual` to `virtual-sh` and add `virtual-lua`.
- [x] 1.2 Update `internal/config/config_schema.cue`, generated config output, and config validation so `default_runtime` accepts `virtual-sh` and `virtual-lua` and rejects `virtual`.
- [x] 1.3 Replace config `virtual_shell` with the family-level `virtual.utilities.enabled` shape, default it to true, and reject `virtual_shell` as an unknown legacy field.
- [x] 1.4 Update generated config output so it emits the full explicit `virtual: { utilities: { enabled: true } }` shape and never emits `virtual_shell`.
- [x] 1.5 Update runtime constants and validation in `pkg/types/runtime.go`, `pkg/invowkfile/runtime.go`, and `internal/runtime/runtime.go` so valid names are `native`, `virtual-sh`, `virtual-lua`, and `container`.
- [x] 1.6 Update config/schema sync tests so `virtual` is accepted only as the config namespace and rejected as a runtime selector.
- [x] 1.7 Rename shell runtime symbols and files from virtual-generic naming to shell-specific naming, including `VirtualRuntime` to `ShRuntime` where appropriate and `internal/runtime/virtual.go` to `internal/runtime/sh.go`.
- [x] 1.8 Update registry wiring in `internal/app/commandadapters/runtime_registry.go` to register `virtual-sh` and `virtual-lua`.
- [x] 1.9 Update CLI runtime parsing, list output, dry-run output, error templates, generated CUE, and tests so `virtual` is never emitted or accepted as a runtime name.
- [x] 1.10 Add explicit regression tests proving `virtual` fails in invowkfile runtimes, config `default_runtime`, and `--ivk-runtime virtual`.

## 2. Shared Virtual Safety Harness

- [x] 2.1 Implement a shared virtual path validator with OS-native normalization, symlink resolution, Windows drive/UNC handling, and allowed-root containment checks.
- [x] 2.2 Apply the path validator to `virtual-sh` shell redirections, file opens, and relevant mvdan/sh open handlers.
- [x] 2.3 Implement shared virtual utility configuration from `virtual.utilities.enabled` and apply it consistently to `virtual-sh` command resolution and `virtual-lua` `invowk.cmd`/`invowk.capture`.
- [x] 2.4 Apply the path validator to enabled u-root-backed utilities used by virtual runtimes.
- [x] 2.5 Implement host binary execution gating with `allowed_binaries` defaulting to deny-all and `["*"]` explicitly allowing all host binaries.
- [x] 2.6 Implement `binary_lookup_mode` with default `"host"` and `"strict"` hardcoded platform system paths.
- [x] 2.7 Expose resolved host binary metadata as `INVOWK_STATE_BIN_PATH` for `virtual-sh` and `invowk.state.bin_path` for `virtual-lua`.
- [x] 2.8 Ensure Invowk-reserved `INVOWK_` state variables cannot be overridden by user-provided env config.
- [x] 2.9 Add unit tests for utility enable/disable behavior, gating, wildcard behavior, strict lookup, host lookup, absolute executable entries, built-in exemption, and binary metadata exposure.

## 3. Anchors And Logical Paths

- [x] 3.1 Implement the anchor resolver for `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@home`, and `@work` on Linux, macOS, and Windows.
- [x] 3.2 Make `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work` implicit allowed roots, while keeping `@home` resolvable but not a blanket implicit home allow root.
- [x] 3.3 Add implementation-scoped `allowed_paths` to the invowkfile CUE schema and Go types, supporting common string values and platform-keyed `linux`/`macos`/`windows` values.
- [x] 3.4 Validate `allowed_paths` logical names as safe environment suffixes and reject missing platform mappings for selected platforms.
- [x] 3.5 Inject `INVOWK_ANCHOR_*`, `INVOWK_PATH_*`, and `INVOWK_STATE_*` variables for `virtual-sh`.
- [x] 3.6 Add tests for XDG, macOS Library, Windows AppData, temp, workdir, home metadata, platform-keyed mappings, traversal rejection, and shell/Lua path exposure.

## 4. Lua Runtime And Bridge

- [x] 4.1 Add `github.com/arnodel/golua` and implement `LuaRuntime` in `internal/runtime/lua.go`.
- [x] 4.2 Validate Lua-compatible shebang and `script.interpreter` intent for `virtual-lua`, rejecting non-Lua interpreters before execution.
- [x] 4.3 Create a fresh Lua VM per command execution, including command dependency execution paths.
- [x] 4.4 Wire Lua stdin/stdout/stderr to `ExecutionContext` streams and support interactive stream attachment without adding a Lua REPL.
- [x] 4.5 Populate Lua positional arguments through `arg` and chunk varargs.
- [x] 4.6 Implement the read-only `invowk` bridge with `path`, `env`, `state`, `cmd`, and `capture`.
- [x] 4.7 Restrict Lua stdlib behavior: path-validated file I/O, controlled `os.getenv`, no `os.execute`, no `io.popen`, no unrestricted `package.loadlib`, no `debug`, and no dynamic `golib` import.
- [x] 4.8 Implement module-contained `require` for Lua source files and block traversal or native shared-library loading.
- [x] 4.9 Add optional `cpu_limit` and `memory_limit` fields for `virtual-lua` and enforce them through golua runtime contexts.
- [x] 4.10 Add Lua runtime tests for execution, interpreter validation, args, streams, bridge path/env/state, command streaming, command capture, stdlib restrictions, require containment, and resource limits.

## 5. Audit And Security Review

- [x] 5.1 Extend `internal/audit/` discovery to include inline `virtual-lua` scripts, Lua `script.file` entries, module-local required Lua files, and self-contained `.lua` files in invowkmods.
- [x] 5.2 Add deterministic Lua risk checks for disabled APIs, broad host binary opt-out, sensitive environment reads, network-capable allowed binaries, and path-mapping risks.
- [x] 5.3 Update LLM audit prompts, module-security skill guidance, and related agent docs so Lua reviews understand bridge semantics and request subagents when useful.
- [x] 5.4 Add audit tests covering Lua discovery, deterministic findings, required Lua files, unreferenced module Lua files, and LLM instruction text.

## 6. Documentation, Samples, And I18n

- [x] 6.1 Update README runtime overview, generated config docs, examples, environment variable reference, interpreter docs, audit/security sections, and architecture references.
- [x] 6.2 Update website current docs, snippets, generated snippets, diagrams when affected, and runtime-mode pages for `virtual-sh`, `virtual-lua`, `virtual.utilities.enabled`, safety-harness boundaries, anchors, and binary gating.
- [x] 6.3 Update Portuguese i18n for current docs to match the new runtime names and safety model.
- [x] 6.4 Update samples in `samples/invowkmods/` and any generated/example invowkfiles to use `virtual-sh` or `virtual-lua`.
- [x] 6.5 Remove or rewrite references that describe the virtual runtime as wide-open host PATH fallback behavior.
- [x] 6.6 Run docs and agent-doc integrity checks, including `make check-agent-docs` when `.agents/` or agent guidance changes.

## 7. Test Suite And Verification

- [x] 7.1 Update all Go unit tests and fixtures that reference `virtual`.
- [x] 7.2 Update all CLI testscript fixtures in `tests/cli/testdata/` from `virtual` to `virtual-sh` where shell behavior is intended.
- [x] 7.3 Add CLI tests for `virtual-lua`, `virtual.utilities.enabled`, denied host binaries, named allowed binaries, wildcard allowed binaries, `binary_lookup_mode`, anchors, `allowed_paths`, and dry-run safety metadata.
- [x] 7.4 Update virtual/native mirror exemptions and mirror tests where runtime naming changes affect expectations.
- [x] 7.5 Run targeted Go tests for schema, runtime, command adapters, audit, config, and CLI packages.
- [x] 7.6 Run `make test-cli`, `make check-baseline`, `make lint`, `make test`, `openspec validate rename-virtual-and-add-lua --strict`, and `openspec validate --specs --strict`.
- [x] 7.7 Confirm no user-facing `virtual` runtime references remain except explicit legacy-rejection tests or historical versioned docs that are intentionally preserved by docs policy.
