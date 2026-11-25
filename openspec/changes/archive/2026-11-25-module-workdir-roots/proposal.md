# Proposal

## Why

Under `restricted` virtual filesystem access, the virtual runtimes (`virtual-sh`, `virtual-lua`) add the effective working directory, and the `@work` anchor that points at it, to their allowed roots. When that working directory comes from a module's own `invowkfile.cue` and lies outside the module, the module silently widens its own read and write roots to a directory its author chose, with nothing in `virtual.filesystem.paths` to show it. The formal model `VirtualPathHarness` records this as finding F15, and a Go replay reproduces it. This change hardens the virtual-runtime read roots so that only a working directory the user chose, or one inside the module, becomes a root.

The maintainer chose the narrow fix (decision 45 of the fixes program): no working directory is dropped from the roots unless it is declared by a module and resolves physically outside that module. The strict alternative, where no working directory is ever a root, would reverse two shipped requirements and break monorepo workdirs in plain invowkfiles.

## What Changes

- Under `restricted` access, a working directory declared by a module's own CUE (its root, command or implementation `workdir`) that resolves physically outside the module root is no longer an allowed root, and neither is the `@work` anchor. The command still starts in that directory, and `@work`, `INVOWK_ANCHOR_WORK` and `invowk.path("@work")` still report it.
- "Outside the module" is decided physically: links and Windows junctions on both paths are resolved, macOS `/private` aliases compare equal, and a path that cannot be resolved, or that is on another volume, counts as outside.
- These working directories stay allowed roots, as today: the CLI override `--ivk-workdir`, every working directory of a plain (non-module) invowkfile, a module's default working directory (the module root), and a module-declared working directory inside the module.
- A directory listed in `virtual.filesystem.paths` stays an allowed root whatever the working directory is (for example `WORK: "@work"`). The docs name it as the way to keep a module's outside working directory readable. `access: "full"` is unaffected. Native and container runtimes are unaffected.
- The interactive virtual subprocesses (`invowk internal exec-virtual-sh` and `exec-virtual-lua`, which stay hidden) receive what they need to make the same decision.
- The `VirtualPathHarness` model gains the working-directory source and containment. Its fixed predicate changes, F15 becomes a pre-fix regression mutant, its replay is inverted and renamed, and the golden vectors, construction replay and rapid property gain the working-directory dimension.
- Docs: the virtual runtime pages, the workdir page, their pt-BR copies, the README if it lists the roots, and `formal/README.md`.
- Release note: this is a security fix for the virtual runtimes. A module that declares a `workdir` outside its own directory and reads or writes there from a virtual runtime under `restricted` access must declare that directory in `virtual.filesystem.paths`. This change breaks only modules of that kind: plain invowkfiles and `--ivk-workdir` behave as before.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `virtual-runtime-sandbox`: "Path sanitization for virtual runtimes" changes. Relative workdir access stays allowed for trusted working directories, and a module-declared working directory outside the module is not an allowed root.
- `cross-platform-path-anchors`: "Anchor allow roots are explicit" changes. `@work` is an automatic allow root except when it points at a module-declared working directory outside the module.
- `module-path-containment-model`: the harness model and its bindings change. The changed requirements are the allowed-location predicate set (a new `allowedTrustedWorkdir`), the no-follow abstraction (its "model and golden unchanged" clause is scoped to the no-follow form), the harness model scope, harness workdir admission, findings recording (F15 fixed), the construction replay, the anchors row's masked cases, and pre-fix regression mutants (F15 joins F14, F18 and F19). New requirements cover the trusted-working-directory completeness check and the working-directory dimension of the golden, rapid and construction replays.

## Impact

- Code: `internal/runtime/virtual_policy.go` (`newVirtualPathResolverForFilesystem`, `newVirtualPathResolver`, and a new working-directory root decision), `internal/runtime/sh.go`, `lua.go`, `lua_interactive.go`, `virtual_interactive.go` (interactive options), `internal/app/commandadapters/sh_interactive.go`, `lua_interactive.go` (the specs that pass the hidden flag), `cmd/invowk/internal_exec_sh.go`, `internal_exec_lua.go` (a hidden flag), and `pkg/invowkfile/invowkfile.go` (a typed effective-workdir result with its source; the body of `GetEffectiveWorkDir` is unchanged).
- Formal: `formal/alloy/VirtualPathHarness.als`, `formal/manifest.toml`, the harness golden file, `internal/runtime/virtual_path_harness_{formal,golden,rapid}_test.go`, `tools/mutation/triage/formal-bindings.toml` (a partial refresh of the touched entries), and `formal/README.md`.
- Tests: unit tests for each working-directory source, `tests/cli/testdata/virtual_module_workdir_roots.txtar` with a runtime-mirror exemption, and Windows and macOS legs that run through the existing CI matrix.
- Users: only modules that declare an outside working directory and do file I/O there from a virtual runtime under `restricted` access. There are no schema or CLI changes and no new dependencies.
