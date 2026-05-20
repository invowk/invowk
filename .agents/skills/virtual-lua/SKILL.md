---
name: virtual-lua
description: Virtual-lua runtime and golua integration guidance for Invowk. Use when editing internal/runtime/lua.go, virtual-lua bridge APIs, golua library loading, Lua stdlib/require behavior, virtual-lua tests, virtual.utilities.enabled behavior for Lua, Lua audit discovery/checks, or docs/examples for the virtual-lua runtime.
---

# Virtual Lua Runtime

Use this with `.agents/skills/go/SKILL.md` and `.agents/skills/go-testing/SKILL.md`. Also load `.agents/skills/uroot/SKILL.md` when `invowk.cmd` or `invowk.capture` touches built-in utilities, and `.agents/skills/module-security/SKILL.md` when audit behavior changes.

## Runtime Invariants

- Create a fresh `golua` VM per command execution and per capture execution path.
- Wire stdin/stdout/stderr through `ExecutionContext`; interactive execution attaches streams but must not add a Lua REPL.
- `virtual.utilities.enabled` controls only Invowk-provided built-in utilities. Host binaries are governed only by runtime `allowed_binaries` plus `binary_lookup_mode`.
- Keep the virtual family boundary honest: `virtual-lua` is a safety harness for VM-controlled operations, not process isolation. Explicitly allowed host binaries run as native host processes.
- Preserve no-silent-fallback behavior: if an enabled built-in utility is registered and fails, do not retry as a host binary.

## Golua Library Loading

- Load `packagelib.LibLoader` before any named golua library loader such as `coroutine`, `string`, `table`, `math`, or `utf8`. Named loaders save themselves in `package.loaded`; without the package registry, execution can panic once Lua code is exercised.
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

- Route Lua file APIs and module-local `require` through the shared virtual path resolver/validator in `internal/runtime/virtual_policy.go`.
- Standard anchors: `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@home`, and `@work`.
- In restricted mode, implicit allowed roots are config/data/cache/state/tmp/work plus script/module source roots. `@home` is metadata/resolution only unless explicitly mapped through selected-platform `virtual.filesystem.paths`.
- `virtual.filesystem.paths` keys are environment suffixes; keep them uppercase and safe for `INVOWK_PATH_<KEY>`.
- In full mode, VM-controlled file operations can access normalized host paths after resolver checks; path handles remain bridge names, not the permission boundary.
- Module-local `require` may load Lua source inside the inline script context or source invowkmod tree only. Block absolute/traversal escapes and native shared-library loading.

## Tests

Add focused Go tests before relying on CLI fixtures:

- Basic `virtual-lua` execution and stdout/stderr capture.
- Lua-compatible interpreter acceptance and non-Lua interpreter rejection.
- Positional `arg` table behavior.
- `invowk.path`, `invowk.env`, controlled `os.getenv`, and read-only bridge tables.
- `invowk.cmd` streaming and `invowk.capture` stdout/stderr/code behavior.
- Utility enabled/disabled behavior independent from host binary allowlisting.
- Host binary deny-by-default, named allow, wildcard allow, absolute executable entries, strict lookup, host lookup, and `bin_path` metadata.
- Stdlib restrictions, path-validated file I/O, module-local `require`, traversal denial, and resource-limit diagnostics.

Useful targeted gates:

```bash
go test ./internal/runtime ./cmd/invowk ./internal/app/commandadapters
go test ./pkg/invowkfile -run 'RuntimeConfig|Implementation|SchemaSync|VirtualFilesystem'
go test ./internal/audit ./tests/cli/...
```
