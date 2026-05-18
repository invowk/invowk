## ADDED Requirements

### Requirement: Git requirements identify sources, not module names
Invowk SHALL treat `requires[*].git_url` as a Git source location and SHALL NOT require the repository name to end in `.invowkmod`.

#### Scenario: Ordinary repository root is accepted as source
- **WHEN** a requirement uses `git_url: "https://example.com/acme/tools.git"` and the fetched repository root contains `invowkmod.cue` and `invowkfile.cue`
- **THEN** module sync SHALL treat the repository root as the module source root

#### Scenario: Repository suffix does not define module identity
- **WHEN** a fetched repository is named `tools.git` and its `invowkmod.cue` declares `module: "com.acme.devtools"`
- **THEN** Invowk SHALL use `com.acme.devtools` as the module identity

#### Scenario: Git URL with invowkmod suffix remains valid
- **WHEN** a requirement uses a repository URL whose basename ends in `.invowkmod.git`
- **THEN** Invowk SHALL accept the URL when the source contains valid module metadata

### Requirement: Source module selection is explicit and deterministic
Invowk SHALL select exactly one source module directory from a fetched repository using the declared `path` value and SHALL fail when selection is ambiguous.

#### Scenario: Root source requires module files
- **WHEN** a requirement omits `path`
- **THEN** module sync SHALL require `invowkmod.cue` and `invowkfile.cue` at the fetched repository root

#### Scenario: Omitted path does not scan child modules
- **WHEN** a requirement omits `path`, the repository root lacks `invowkmod.cue`, and the repository contains one or more child directories ending in `.invowkmod`
- **THEN** module sync SHALL fail with an actionable error telling the user to set `path`

#### Scenario: Explicit subpath selects module directory
- **WHEN** a requirement declares `path: "modules/com.acme.devtools.invowkmod"` and that directory contains `invowkmod.cue` and `invowkfile.cue`
- **THEN** module sync SHALL select that directory as the module source root

#### Scenario: Explicit subpath must be an invowkmod directory
- **WHEN** a requirement declares `path: "modules/tools"` for a Git dependency
- **THEN** module sync SHALL reject the requirement because the selected subpath basename does not end in `.invowkmod`

#### Scenario: Explicit subpath cannot traverse
- **WHEN** a requirement declares an absolute, parent-traversing, drive-qualified, or null-byte-containing `path`
- **THEN** invowkmod parsing or module sync validation SHALL reject the path before reading outside the fetched repository

### Requirement: Subpath module directory names match metadata
Invowk SHALL require explicit subpath module directories to match their declared module identity.

#### Scenario: Subpath basename matches module
- **WHEN** a requirement selects `path: "modules/com.acme.devtools.invowkmod"` and the selected `invowkmod.cue` declares `module: "com.acme.devtools"`
- **THEN** module sync SHALL accept the subpath identity

#### Scenario: Subpath basename mismatches module
- **WHEN** a requirement selects `path: "modules/tools.invowkmod"` and the selected `invowkmod.cue` declares `module: "com.acme.devtools"`
- **THEN** module sync SHALL fail with an error explaining the directory/module mismatch

#### Scenario: Repository root name is not checked against module
- **WHEN** a requirement omits `path`, the repository root is named `tools`, and root `invowkmod.cue` declares `module: "com.acme.devtools"`
- **THEN** module sync SHALL NOT reject the source because the repository root basename differs from the module identity

### Requirement: Canonical local module directories use module metadata
Invowk SHALL materialize cached and vendored dependencies as directories named `<module-id>.invowkmod`, where `<module-id>` comes from parsed `invowkmod.cue`.

#### Scenario: Ordinary repository root is cached canonically
- **WHEN** `https://example.com/acme/tools.git` resolves to `module: "com.acme.devtools"`
- **THEN** the cached module directory SHALL end in `com.acme.devtools.invowkmod`

#### Scenario: Ordinary repository root is vendored canonically
- **WHEN** `invowk module vendor` vendors a resolved dependency whose metadata declares `module: "com.acme.devtools"`
- **THEN** the destination SHALL be `invowk_modules/com.acme.devtools.invowkmod`

#### Scenario: Canonical directory contains module files
- **WHEN** Invowk copies a resolved module into the cache or vendor directory
- **THEN** the canonical directory SHALL contain the selected source module's `invowkmod.cue`, `invowkfile.cue`, scripts, and supporting files without adding an extra nested wrapper directory

#### Scenario: Installed module directory is invowkmod-compatible
- **WHEN** a dependency is installed locally through sync or vendor
- **THEN** the resulting local directory name SHALL satisfy Invowk module directory naming rules

### Requirement: Module metadata drives default command namespace
Invowk SHALL derive the default dependency command namespace from the parsed module identity and resolved version, unless an alias is declared.

#### Scenario: Default namespace uses module identity
- **WHEN** a dependency resolves to `module: "com.acme.devtools"` at version `1.2.3` without an alias
- **THEN** its default command namespace SHALL be `com.acme.devtools@1.2.3`

#### Scenario: Alias overrides command namespace
- **WHEN** a dependency resolves to `module: "com.acme.devtools"` and the requirement declares `alias: "tools"`
- **THEN** its command namespace SHALL be `tools`

#### Scenario: Alias does not rename module identity
- **WHEN** a dependency declares `alias: "tools"` and metadata declares `module: "com.acme.devtools"`
- **THEN** cache, vendor, lock `module_id`, and collision detection SHALL continue to use `com.acme.devtools`

### Requirement: Lock keys preserve source identity
Invowk SHALL continue to identify dependency declarations by Git URL plus normalized optional subpath while storing resolved module identity separately.

#### Scenario: Root lock key omits subpath
- **WHEN** a requirement omits `path`
- **THEN** the lock entry key SHALL be based on the Git URL only

#### Scenario: Subpath lock key includes normalized path
- **WHEN** a requirement declares `path` using platform-specific separators
- **THEN** the lock entry key SHALL include the normalized slash-separated path

#### Scenario: Lock entry stores module identity
- **WHEN** a dependency resolves successfully
- **THEN** the lock entry SHALL store the parsed `module_id`, resolved namespace, command source ID, resolved version, Git commit, and content hash

#### Scenario: Lock-only loading uses canonical cache path
- **WHEN** vendoring loads a dependency from an existing lock entry with `module_id`
- **THEN** Invowk SHALL locate or repopulate the canonical cache directory named `<module_id>.invowkmod`

### Requirement: Canonical module collisions fail explicitly
Invowk SHALL detect when distinct dependency declarations resolve to the same canonical module directory and fail rather than overwrite.

#### Scenario: Different sources same module identity collide
- **WHEN** two different source identities resolve to `module: "com.acme.devtools"` with different source keys or content hashes
- **THEN** module sync or vendor SHALL fail with a collision error naming both requirements and `com.acme.devtools.invowkmod`

#### Scenario: Duplicate same source is deduplicated
- **WHEN** two declarations normalize to the same source identity and resolve to the same content
- **THEN** module sync SHALL resolve the dependency once

#### Scenario: Alias does not avoid module collision
- **WHEN** two different source identities resolve to the same module identity but declare different aliases
- **THEN** module sync or vendor SHALL still fail with a canonical module collision

### Requirement: Documentation describes source and module identity separately
Invowk documentation SHALL explain the difference between Git source names, module metadata, and local module directory names.

#### Scenario: Dependency docs show ordinary root repository
- **WHEN** users read module dependency documentation
- **THEN** the docs SHALL show a valid example where an ordinary repository such as `tools.git` contains root `invowkmod.cue` and vendors as `<module-id>.invowkmod`

#### Scenario: Dependency docs show subpath module
- **WHEN** users read module dependency documentation
- **THEN** the docs SHALL show a valid monorepo example using `path` that points to a `.invowkmod` directory

#### Scenario: Docs no longer require invowkmod Git suffix
- **WHEN** users read schema comments, website docs, README content, or samples for `requires[*].git_url`
- **THEN** those surfaces SHALL NOT state that Git repository names must end in `.invowkmod`
