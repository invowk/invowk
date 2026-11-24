---
name: virtual-lua
description: Virtual-lua runtime and golua integration guidance. Use when editing internal/runtime/lua*.go, the hidden exec-virtual-lua command, internal/app/commandadapters Lua interactive plumbing, invowk bridge APIs, golua library loading, Lua stdlib/require behavior, virtual.utilities.enabled for Lua, Lua audit discovery/checks, or virtual-lua tests, docs and examples.
---

# Virtual Lua Runtime

Use this with `.agents/skills/go/SKILL.md` and `.agents/skills/go-testing/SKILL.md`. Also load `.agents/skills/uroot/SKILL.md` when `invowk.cmd` or `invowk.capture` touches built-in utilities, and `.agents/skills/module-security/SKILL.md` when audit behavior changes.

## Runtime Invariants

- Create a fresh `golua` VM for each top-level `Execute`, `ExecuteCapture`, and
  `RunLuaScript` path. Do not reuse a VM across command executions.
- Wire stdin/stdout/stderr through `ExecutionContext`; interactive execution attaches streams but must not add a Lua REPL.
- `virtual.utilities.enabled` controls only Invowk-provided built-in utilities. Host binaries are governed only by runtime `allowed_binaries` plus `binary_lookup_mode`.
- `virtual-lua` is a safety harness for VM-controlled operations, not process isolation; the full statement is in `AGENTS.md` "Hard invariants".
- Preserve no-silent-fallback behavior: if an enabled built-in utility is registered and fails, do not retry as a host binary.

## Golua Library Loading

- Load `packagelib.LibLoader` before any named golua library loader such as `coroutine`, `string`, `table`, `math`, or `utf8`. Named loaders save themselves in `package.loaded`; without the package registry, execution can panic once Lua code is exercised.
- Serialize golua library bootstrap behind a package-level mutex if the setup path touches process-global golua state (for example package compliance declarations or library registries). Parallel `virtual-lua` tests with `-race` can otherwise expose data races that serial local runs miss.
- After loading package support, remove or replace unrestricted globals that are not part of Invowk's bridge contract: `package`, `require`, `dofile`, `loadfile`, `rawset`, `debug`, `os.execute`, `io.popen`, `package.loadlib`, and dynamic `golib` import.
- Do not load `debuglib`, `golib`, unrestricted `iolib`, or unrestricted `oslib` for `virtual-lua`. If file I/O or `require` is needed, implement narrow wrappers that call the shared virtual path resolver/validator first.
- Run Lua tests with `t.Parallel()` where possible. Parallel runtime tests are good at exposing VM setup and library-loader assumptions that single serial tests can miss.

## Bridge Contract

- Expose a read-only global `invowk` table with:
  - `invowk.path(nameOrAnchor)` resolving standard anchors and selected-platform `virtual.filesystem.paths` handles.
  - `invowk.env.NAME` plus controlled `os.getenv("NAME")` reading the effective command environment.
  - `invowk.state.bin_path` reflecting the last resolved host binary path.
  - `invowk.cmd.<name>(...)` for streaming execution.
  - `invowk.capture.<name>(...)` returning stdout, stderr, and exit code.
- Make `invowk`, `invowk.env`, `invowk.state`, `invowk.cmd`, and `invowk.capture` read-only from Lua. Use a mutable internal backing table for state updates, then expose a read-only proxy.
- Update `invowk.state.bin_path` only after host binary policy resolution succeeds. Built-in utilities do not count as host binary resolution.
- `invowk.cmd` and `invowk.capture` should support both named helper calls and direct calls with an explicit binary path/name.

## Paths And Require

- Route Lua file APIs through the shared virtual path resolver/validator in
  `internal/runtime/virtual_policy.go`.
- Route module-local `require` through the source-tree-only resolver
  `luaRequireBridge.resolveModule` in `internal/runtime/lua_io.go`, which
  confines candidates to the bridge's `scriptBasePath` using
  `normalizeExistingOrParent` and `pathWithin` from `virtual_policy.go`. Keep it
  distinct from general VM-controlled file I/O.
- `normalizeExistingOrParent` is the strict resolver for every containment
  decision (the validator under restricted access, the allowed roots, `require`):
  it walks the path through every symlink and Windows junction to the deepest
  existing ancestor (`walkVirtualPath`) and has no lexical fallback, so an
  unresolvable path is an error. Only `resolve`, `resolveBridgePath` and
  declared `filesystem.paths` go through `normalizeVirtualPathForAccess`, whose
  full-access fallback (a link's own path, or the cleaned path) must never
  reach a containment check. The validated path is the resolved path, and the
  operation runs on it, for every operation that follows its last component.
  The operations that act on the last component itself (`invowk.cmd.rm`
  operands, `invowk.cmd.mv` sources, `invowk.cmd.ln`'s link name, and the
  virtual-sh `-L`/`-h` stat) use the no-follow form instead:
  `validateNoFollow`/`resolveNoFollow` (`internal/runtime/virtual_path_nofollow.go`)
  resolve the parent with `normalizeVirtualPathForAccess` and join the final
  name unread; an operand ending in a separator, `.` or `..` is resolved in
  full and refused with `ENOTDIR` when it is not a directory. Both hooks are
  set on the u-root handler context (`ValidatePath` and
  `ValidateNoFollowPath`) by virtual-sh and virtual-lua alike.
- Standard anchors: `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@home`, and `@work`.
- In restricted mode, implicit allowed roots are config/data/cache/state/tmp/work plus script/module source roots, mapped `virtual.filesystem.paths`, and the system temp roots from `defaultTempRoots()` (`/tmp` on Unix, none on Windows). `@home` is metadata/resolution only unless explicitly mapped through selected-platform `virtual.filesystem.paths`.
- `virtual.filesystem.paths` keys are environment suffixes; keep them uppercase and safe for `INVOWK_PATH_<KEY>`.
- In full mode, VM-controlled file operations can access normalized host paths after resolver checks; path handles remain bridge names, not the permission boundary.
- Module-local `require` may load Lua source inside the inline script context or source invowkmod tree only. Block absolute/traversal escapes and native shared-library loading.

## Interactive Subprocess Sync

When interactive virtual-lua changes, keep these surfaces aligned in one patch:

- `LuaInteractiveCommandSpec` and `PrepareCommand` in `internal/runtime/lua_interactive.go`.
- `RunLuaScript` arguments and stdin/stdout/stderr handling.
- Hidden Cobra flags in `cmd/invowk/internal_exec_lua.go`.
- `internal/app/commandadapters/lua_interactive.go` argument transport.
- Focused runtime and commandadapter tests for both normal and interactive paths.

## Audit Surfaces

When Lua audit behavior changes, also load `module-security` and verify:

- `internal/audit/checks_lua.go` detects disabled APIs, bridge env reads,
  host-binary opt-outs, network-capable host binaries, and broad filesystem access.
- `internal/audit/scan_context_artifacts.go` discovers module-local `.lua` files.
- `internal/audit/scan_context.go` builds script references consistently.
- `internal/audit/checks_lua_test.go` covers each new diagnostic.

For docs/examples changes, check `website/docs/runtime-modes/virtual-lua.mdx`,
`website/src/components/Snippet/data/runtime-modes.ts`, and the current pt-BR docs.

## Tests

Add focused Go tests before relying on CLI fixtures:

- Basic `virtual-lua` execution and stdout/stderr capture.
- Lua-compatible interpreter acceptance and non-Lua interpreter rejection.
- Positional `arg` table behavior.
- `invowk.path`, `invowk.env`, controlled `os.getenv`, and read-only bridge tables.
- Resolver-backed path/env assertions must use the virtual resolver as the expected-value source (`resolver.anchors`, `resolver.paths`, and `resolveBridgePath`). Do not compare against separately normalized host paths; macOS can differ as `/var` vs `/private/var`, and Windows can differ as 8.3 short paths such as `RUNNER~1`.
- `invowk.cmd` streaming and `invowk.capture` stdout/stderr/code behavior.
- Utility enabled/disabled behavior independent from host binary allowlisting.
- Host binary deny-by-default, named allow, wildcard allow, absolute executable entries, strict lookup, host lookup, and `bin_path` metadata.
- Cross-platform host-binary CLI fixtures must invoke the executable name that the host can actually run. In txtar tests, prefer `tool.bat`/`tool.cmd` on Windows and `tool` on Unix when both files are present; a bare `tool` can resolve to the extensionless Unix fixture on Windows and return exit code 1.
- Stdlib restrictions, path-validated file I/O, module-local `require`, traversal denial, and resource-limit diagnostics.

Useful targeted gates:

`-race` needs cgo (a C compiler and `CGO_ENABLED=1`).

```bash
go test -race -count=1 ./internal/runtime
go test ./internal/runtime ./cmd/invowk ./internal/app/commandadapters
go test ./pkg/invowkfile -run 'RuntimeConfig|Implementation|SchemaSync|VirtualFilesystem'
go test ./internal/audit ./tests/cli/...
```
