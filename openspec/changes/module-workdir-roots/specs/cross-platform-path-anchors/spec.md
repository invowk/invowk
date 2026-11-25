# Spec Delta

## MODIFIED Requirements

### Requirement: Anchor allow roots are explicit
Invowk SHALL automatically allow restricted virtual runtime access to `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work`, except that `@work` SHALL NOT be an allow root when the effective work directory is not one: a work directory that a module's own invowkfile declares and that resolves physically outside the module directory (virtual-runtime-sandbox "The work directory is an allowed root unless a module declares it outside the module"). `@work` SHALL still resolve to that directory. Invowk SHALL resolve `@home` but MUST NOT grant blanket recursive home-directory access in restricted mode unless the selected platform explicitly maps it through `virtual.filesystem.paths`.

#### Scenario: App-scoped anchor access is allowed
- **WHEN** a `virtual-lua` script opens `invowk.path("@cache/build.json")`
- **THEN** Invowk SHALL allow the access if the resolved path remains under the `@cache` root

#### Scenario: Home anchor does not grant implicit home access
- **WHEN** a virtual script tries to read `@home/.ssh/id_rsa` in restricted mode without an explicit selected-platform `virtual.filesystem.paths` mapping
- **THEN** Invowk SHALL block the access

#### Scenario: Home anchor can be explicitly mapped
- **WHEN** an implementation platform declares a `virtual.filesystem.paths` entry that resolves to a path under `@home`
- **THEN** virtual scripts SHALL be able to access only that mapped path subtree

#### Scenario: Work anchor of a trusted work directory is allowed
- **WHEN** the effective work directory is the `--ivk-workdir` override, a workdir of an invowkfile that is not in a module, a module's default work directory, or a module-declared workdir inside the module, and a restricted virtual script opens `invowk.path("@work/out.json")`
- **THEN** Invowk SHALL allow the access if the resolved path remains under `@work`

#### Scenario: Work anchor of a module workdir outside the module is not a root
- **WHEN** a module declares a workdir that resolves physically outside the module directory, and a restricted virtual script opens `invowk.path("@work/out.json")` or reads `$INVOWK_ANCHOR_WORK/data.txt`
- **THEN** `invowk.path` and `INVOWK_ANCHOR_WORK` SHALL report the work directory, and Invowk SHALL refuse the access unless another allowed root, such as a `virtual.filesystem.paths` entry for `@work`, encloses the resolved path
