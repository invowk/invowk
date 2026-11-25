## Why

A module's identity should be a property of where its files live, not of the directory invowk runs from. This change hardens discovery: the invowkfile of a module directory is always loaded as part of that module, with the module's checks, identity and diagnostics, wherever invowk runs. It also makes one module directory reached through several discovery sources one module.

## What Changes

- When the current directory is a `*.invowkmod` directory, discovery loads it as a module, through the same entry classifier and checks that the scan of its parent directory uses. A file in a module directory is never parsed as a plain invowkfile. A module that does not load contributes nothing, and the skip is reported as a diagnostic.
- "Module directory" means a directory whose name, as its parent directory stores it, ends in `.invowkmod`: the rule the parent-directory scan already uses. A directory that only contains an `invowkmod.cue` (for example a project root that declares `requires`) is unchanged.
- The name is judged on the entry the parent directory stores, so the letter case or short form used to enter the directory does not change the result. A current directory whose name ends in `.invowkmod` in any letter case is never read as a plain invowkfile directory.
- A current directory whose own entry is a symbolic link or a junction to a module directory is treated as the parent-directory scan treats that link: nothing is loaded from it, and the skip is reported. A link in an ancestor directory does not count.
- **BREAKING:** in a module directory, the module's commands are published under the module's namespace (`<module> <cmd>`, `@<module>`, `--ivk-from <module>`), as from any other directory. Bare names keep working when they are unique across sources. `@invowkfile` and `--ivk-from invowkfile` do not select them.
- **BREAKING:** workspace `invowk validate` and `invowk cmd` report a module's load problems (`module_load_skipped`, `module_symlink_skipped`, `reserved_module_name_skipped`) and apply the module's command scope to its `depends_on.cmds` in every directory, the module directory included.
- **BREAKING:** an extensionless `invowkfile` in a module directory is not read; a module's commands come from its `invowkfile.cue`.
- **BREAKING:** a directory whose name ends in `.invowkmod` but that is not a loadable module (no `invowkmod.cue`, a name that is not a valid module name, a name whose suffix differs in letter case) loads nothing when invowk runs in it, and the reason is reported.
- In a module directory, features that need module identity work: `script.file` sources, and the module's declared and locked vendored dependencies. `invowk module list` lists the module.
- Precedence: the current-directory module takes the current directory's place, ahead of configured `includes`, provisioned entries and the user commands directory.
- The same module directory reached through more than one discovery source (the current directory, the current-directory module scan, configured `includes`, the user commands directory, provisioned entries) is loaded once instead of failing with a module name collision. It keeps the highest-precedence source, is global when any of its sources is the user commands directory, and publishes under a configured include alias.
- `invowk agent cmd create|change --verify` run in a module directory verifies the written command under the module's namespace.
- `--ivk-no-cwd` also skips a current directory that is itself a module directory.

## Capabilities

### New Capabilities

- `module-directory-identity`: the invowkfile of a module directory is loaded as part of that module from every directory, and one module directory reached through several discovery sources is one module.

### Modified Capabilities

- `cwd-discovery-scope`: the requirement "Switch disables current-directory command sources" also covers a current directory that is itself a module directory.
- `agent-authoring-crud`: the requirement "Command authoring modes remain safe" verifies a command written in a module directory under the module's namespace.

## Impact

- Code: `internal/discovery` (`discovery_files.go`, `discovery.go`, a new helper file), `cmd/invowk/agent.go` (verify source selection). No consumer of module identity changes: they all read the identity discovery attaches.
- Tests: `internal/discovery` unit tests (Linux, macOS, Windows), `tests/cli` testscript pairs that run each module behaviour from the module's parent and from the module directory, and a module-directory leg in `virtual_module_workdir_roots.txtar`.
- Docs: README discovery sections, `website/docs` (discovery architecture, commands and namespaces, modules overview) in English and pt-BR.
- Formal: no model change expected. The ScopeConstruction facts keep holding (`RootSrc` still has no module identity; command sources stay unique after deduplication), and the new `invowkmod.Load` call stays gated by `IsModule`, as `ModulePathContainment` assumes.

## Migration notes

- In a module directory, run the module's commands as `invowk cmd <module> <cmd>`, or `invowk cmd @<module> <cmd>` when a bare name is ambiguous. Use the module's namespace where `@invowkfile` / `--ivk-from invowkfile` was used.
- A module is checked the same way in every directory. Fix what `invowk validate` reports for it.
- A module directory that holds only an extensionless `invowkfile`: rename it to `invowkfile.cue`.
- A plain project directory whose name ends in `.invowkmod` (in any letter case): rename it, or make it a valid module with an `invowkmod.cue` whose `module` matches the name.
- `depends_on.cmds` entries of a module's commands name commands the module may call: its own, global modules, or declared and locked `requires`.
