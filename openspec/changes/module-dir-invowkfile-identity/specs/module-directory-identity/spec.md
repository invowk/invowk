# Spec Delta

## Purpose

Makes a module's identity a property of where its files live: an invowkfile inside a module directory is always loaded as part of that module, whichever directory invowk runs from, and one module directory reached through several discovery sources is one module.

## ADDED Requirements

### Requirement: A module directory as the current directory is loaded as that module

When the current directory is a module directory, command discovery SHALL load it as that module, with the checks, identity, diagnostics and vendored-dependency handling that apply when the module is found from its parent directory. Discovery SHALL NOT parse any invowkfile in that directory as a plain invowkfile.

#### Scenario: commands carry the module namespace from inside
- **WHEN** `tools.invowkmod` declares `module: "tools"` and command `hello`, and `invowk cmd` runs with `tools.invowkmod` as the current directory
- **THEN** the listing shows `hello` under `tools.invowkmod`, as it does when `invowk cmd` runs in the parent directory, and not under `invowkfile`

#### Scenario: module list includes the current module
- **WHEN** `invowk module list` runs with `tools.invowkmod` as the current directory
- **THEN** `tools` is listed

#### Scenario: a library-only module contributes no commands
- **WHEN** the current directory is `lib.invowkmod`, which has an `invowkmod.cue` and no `invowkfile.cue`
- **THEN** discovery lists no command from it and reports no `invowkfile_parse_skipped` diagnostic for it

#### Scenario: an extensionless invowkfile in a module directory is not read
- **WHEN** the current directory is `tools.invowkmod`, which has an `invowkmod.cue` and an extensionless `invowkfile` but no `invowkfile.cue`
- **THEN** no command from the extensionless file is listed, as when invowk runs in the parent directory

### Requirement: Module directories are recognised by their name

A directory SHALL be a module directory when its name ends in `.invowkmod`, the rule the scan of a parent directory applies. A directory whose name does not end in `.invowkmod` SHALL NOT become a module directory because it contains an `invowkmod.cue`. The name SHALL be judged on the current directory as given and on its physical location.

#### Scenario: a project root with invowkmod.cue stays a plain invowkfile
- **WHEN** the current directory `project` contains `invowkfile.cue` with command `build` and an `invowkmod.cue` that declares `requires`
- **THEN** `build` is listed under `invowkfile` and `invowk cmd build` runs, as before this change

#### Scenario: a link without the suffix to a module directory
- **WHEN** the current directory is reached through a link named `tools` whose target is `tools.invowkmod`
- **THEN** discovery does not parse the invowkfile in it as a plain invowkfile, and reports the skip as a diagnostic

### Requirement: A linked module directory is judged by its own entry

When the current directory's own entry is a symbolic link, a junction or another entry the parent-directory scan does not load as a module, discovery SHALL treat it as that scan does and load nothing from it as a module, reporting the skip as a diagnostic. A link in an ancestor directory SHALL NOT affect the decision.

#### Scenario: symlinked module directory as the current directory
- **WHEN** `link.invowkmod` is a symbolic link to a valid module directory and `invowk cmd` runs with `link.invowkmod` as the current directory
- **THEN** no command from it is listed and a `module_symlink_skipped` diagnostic names it

#### Scenario: an ancestor link does not count
- **WHEN** the current directory is `tools.invowkmod`, reached through a path whose ancestor directory is a symbolic link (for example `/var` to `/private/var` on macOS)
- **THEN** `tools.invowkmod` is loaded as module `tools`

#### Scenario: a junction module directory on Windows
- **WHEN** on Windows `j.invowkmod` is a junction to a valid module directory
- **THEN** `invowk cmd` gives the same result with `j.invowkmod` as the current directory as it gives in its parent directory

### Requirement: A module directory that fails to load contributes nothing

When the current directory is a module directory and the module fails to load, discovery SHALL load nothing from that directory, neither its invowkfile nor any directory below it, and SHALL report a `module_load_skipped` diagnostic. A reserved module name SHALL be reported with `reserved_module_name_skipped`.

#### Scenario: a symbolic link inside the module
- **WHEN** `tools.invowkmod` contains a symbolic link and `invowk validate` runs with `tools.invowkmod` as the current directory
- **THEN** the exit code is non-zero and a `module_load_skipped` diagnostic names the link, as from the parent directory

#### Scenario: a nested module is not loaded
- **WHEN** `tools.invowkmod` contains `inner.invowkmod` outside `invowk_modules/`, and `invowk cmd` runs with `tools.invowkmod` as the current directory
- **THEN** neither `tools` nor `inner` commands are listed, and a `module_load_skipped` diagnostic names `tools.invowkmod`

#### Scenario: module id does not match the directory
- **WHEN** `tools.invowkmod/invowkmod.cue` declares `module: "other"` and `invowk cmd` runs with `tools.invowkmod` as the current directory
- **THEN** no command from it is listed and a `module_load_skipped` diagnostic is reported, as from the parent directory

#### Scenario: reserved module name
- **WHEN** the current directory is `invowkfile.invowkmod`
- **THEN** nothing from it is loaded and a `reserved_module_name_skipped` diagnostic is reported

### Requirement: Module commands keep bare names from inside the module

From inside a module directory, the module's commands SHALL be published under the module's command namespace and SHALL keep their bare names whenever those names are unique across sources, under the same rule as every other source. The `invowkfile` source SHALL NOT include them.

#### Scenario: unique bare name runs from inside
- **WHEN** `tools.invowkmod` defines `hello`, no other source defines `hello`, and `invowk cmd hello` runs with `tools.invowkmod` as the current directory
- **THEN** `hello` from `tools` runs

#### Scenario: namespaced forms run from inside
- **WHEN** `invowk cmd tools hello` or `invowk cmd @tools hello` runs with `tools.invowkmod` as the current directory
- **THEN** `hello` from `tools` runs

#### Scenario: ambiguous bare name needs the namespace
- **WHEN** an included module `other` also defines `hello` and `invowk cmd hello` runs with `tools.invowkmod` as the current directory
- **THEN** the command is reported as ambiguous between `tools` and `other`

#### Scenario: the invowkfile source does not name module commands
- **WHEN** `invowk cmd @invowkfile hello` runs with `tools.invowkmod` as the current directory and no other invowkfile is discovered
- **THEN** the exit code is non-zero and the source `invowkfile` is reported as not found

### Requirement: Module behaviour does not depend on the current directory

A command of a module SHALL get the same command scope, virtual-runtime filesystem roots, env-file and script-file containment and vendored-dependency policy when invowk runs inside the module directory as when it runs in the module's parent directory.

#### Scenario: command scope applies from inside
- **WHEN** a command of `tools` has `depends_on.cmds` naming a command of a module that `tools` does not require, and it runs with `tools.invowkmod` as the current directory
- **THEN** the dependency check fails as it does from the parent directory

#### Scenario: module-declared workdir outside the module
- **WHEN** a virtual-sh command of `tools` declares a `workdir` outside the module and reads a file there under restricted filesystem access, run with `tools.invowkmod` as the current directory
- **THEN** the read is denied, as from the parent directory

#### Scenario: script files run from inside
- **WHEN** a command of `tools` uses `script.file` with a file inside the module, and it runs with `tools.invowkmod` as the current directory
- **THEN** the script runs, as from the parent directory

#### Scenario: undeclared vendored module is skipped from inside
- **WHEN** `tools.invowkmod/invowk_modules/vend.invowkmod` is not declared and locked by `tools`, and `invowk validate` runs with `tools.invowkmod` as the current directory
- **THEN** a `vendored_undeclared_skipped` diagnostic is reported and the vendored module's commands are not available, as from the parent directory

### Requirement: One module directory reached through several sources is one module

When two discovery sources yield the same physical module directory, discovery SHALL load it once, at the highest-precedence source, and SHALL NOT report a module name collision for it. The module SHALL be global if any of those sources is the user commands directory, and SHALL publish under a configured include alias when one names it.

#### Scenario: current module also listed in includes
- **WHEN** the selected config `includes` names `/abs/tools.invowkmod` and `invowk cmd` runs with `/abs/tools.invowkmod` as the current directory
- **THEN** the exit code is 0 and `tools` commands are listed once

#### Scenario: sibling module also listed in includes
- **WHEN** the selected config `includes` names `/abs/tools.invowkmod` and `invowk cmd` runs in `/abs`
- **THEN** the exit code is 0 and `tools` commands are listed once

#### Scenario: include alias applies from inside
- **WHEN** `includes` names `/abs/tools.invowkmod` with `alias: "t"` and `invowk cmd t hello` runs with `/abs/tools.invowkmod` as the current directory
- **THEN** `hello` from the module runs

#### Scenario: current module inside the user commands directory
- **WHEN** the current directory is `~/.invowk/cmds/g.invowkmod`
- **THEN** `g` is loaded once, as a global module, and no shadowing warning is reported for it

#### Scenario: two directories with the same module id still collide
- **WHEN** the current directory is `tools.invowkmod` and `~/.invowk/cmds` holds a different directory `tools.invowkmod` with the same module id
- **THEN** discovery fails with a module name collision, as it does from the parent directory

### Requirement: Authoring verification follows the module identity

`invowk agent cmd create --verify` and `invowk agent cmd change --verify` run in a module directory SHALL verify the written command under the command namespace the module publishes it in.

#### Scenario: verify inside a module directory
- **WHEN** `invowk agent cmd create lint --verify 'run lint'` writes `lint` into the module's `invowkfile.cue` with `tools.invowkmod` as the current directory
- **THEN** the dry-run verification resolves `lint` from `tools` and succeeds
