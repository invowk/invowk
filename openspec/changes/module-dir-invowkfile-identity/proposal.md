## Why

A module's commands should behave the same whichever directory invowk runs from. When invowk runs from its parent directory, discovery loads a module as a module: its checks run, and its commands carry the module's identity. When the current directory is the module directory itself, discovery parses the module's `invowkfile.cue` as a plain invowkfile, without the module's checks or identity. This change hardens discovery: an invowkfile inside a module directory is always loaded as part of that module.

## What Changes

- When the current directory is a `*.invowkmod` directory, discovery loads it as a module, through the same checks as a module found from its parent directory, and never parses its invowkfile as a plain invowkfile. A module that fails to load contributes nothing, and the skip is reported as a diagnostic.
- "Module directory" means a directory whose name ends in `.invowkmod`, the rule the parent-directory scan already uses. A directory that only contains an `invowkmod.cue` (for example a project root that declares `requires`) is unchanged.
- A current directory whose own entry is a symbolic link to a module directory is treated as the parent-directory scan treats that link: it is skipped with a diagnostic. A link in an ancestor directory does not count.
- **BREAKING:** from inside a module directory, the module's commands are published under the module's namespace (`<module> <cmd>`, `@<module>`, `--ivk-from <module>`), as from any other directory. Bare names keep working when they are unique across sources, as before. `@invowkfile` and `--ivk-from invowkfile` no longer select them.
- **BREAKING:** from inside a module directory, workspace `invowk validate` and `invowk cmd` report the module's load problems (for example `module_load_skipped`) as they do from the parent directory, and the module's `depends_on.cmds` are checked against the module's command scope.
- **BREAKING:** an extensionless `invowkfile` in a module directory is no longer read; a module's commands come from its `invowkfile.cue`.
- Inside a module directory, features that need module identity now work: `script.file` sources, and the module's declared and locked vendored dependencies. `invowk module list` lists the module.
- The same module directory reached through more than one discovery source (the current directory, the current-directory module scan, configured `includes`, the user commands directory, provisioned entries) is loaded once instead of failing with a module name collision. It keeps the highest-precedence source, is global when any of its sources is the user commands directory, and publishes under a configured include alias.
- `invowk agent cmd create|change --verify` run inside a module directory verifies the written command under the module's namespace.
- `--ivk-no-cwd` also skips a current directory that is itself a module directory.

## Capabilities

### New Capabilities

- `module-directory-identity`: an invowkfile inside a module directory is loaded as part of that module from every directory, and one module directory reached through several discovery sources is one module.

### Modified Capabilities

- `cwd-discovery-scope`: the requirement "Switch disables current-directory command sources" also covers a current directory that is itself a module directory.

## Impact

- Code: `internal/discovery` (`discovery_files.go`, `discovery.go`, new helper file), `cmd/invowk/agent.go` (verify source selection). No consumer of module identity changes: they all read the identity discovery attaches.
- Tests: `internal/discovery` unit tests (Linux, macOS, Windows), `tests/cli` testscript pairs that run each module behaviour from the module's parent and from inside the module directory, and an inside leg in `virtual_module_workdir_roots.txtar`.
- Docs: README discovery sections, `website/docs` (discovery architecture, commands and namespaces, modules overview) in English and pt-BR.
- Formal: no model change expected. The ScopeConstruction facts keep holding (`RootSrc` still has no module identity; command sources stay unique after deduplication), and the new `invowkmod.Load` call stays gated by `IsModule`, as `ModulePathContainment` assumes.

## Migration notes

- Inside a module directory, run the module's commands as `invowk cmd <module> <cmd>` or `invowk cmd @<module> <cmd>` when a bare name is ambiguous. Replace `@invowkfile` / `--ivk-from invowkfile` there with the module's namespace.
- A module that `invowk validate` rejects from its parent directory is now rejected from inside it too. Fix the reported problem, for example a symbolic link or a nested module inside the module, or a `module` field that does not match the directory name.
- A module directory that holds only an extensionless `invowkfile`: rename it to `invowkfile.cue`.
- `depends_on.cmds` entries of a module's commands must name commands the module may call (its own, global modules, or declared and locked `requires`), from every directory.
