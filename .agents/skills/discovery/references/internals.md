# Discovery Internals

Navigation tables for `internal/discovery/`. The package doc comment
(`internal/discovery/doc.go`) is authoritative for scope and precedence; these
tables drift, so confirm a row with `rg -n '^func|^type' internal/discovery/*.go`
before relying on it. The one-line invariants that prevent mistakes stay in
`SKILL.md` § "Design Invariants".

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Combined file discovery + command aggregation | Tightly coupled; splitting would require large intermediate data structures (documented in `doc.go`, per `.agents/rules/package-design.md`) |
| One non-module source, full-name first-wins | Only the current-directory invowkfile (`SourceCurrentDir`) is a non-module source. `DiscoveredCommandSet.ByName` keeps the first command per full name in precedence order; a simple name shared by the invowkfile and a module is flagged ambiguous, not shadowed |
| Module commands all included | Every discovered module (local, includes, provisioned, provisioned-global, user dir) is first-class and visible; conflicts surface through ambiguity flagging |
| Local module over global module with the same ID | Precedence lets the local one win (the safe direction: it does not gain global trust); `detectModuleShadowing` emits a `module_shadows_global` warning to surface typosquatting or accidental collisions |
| SimpleName-based collision detection | Enables transparent namespace while flagging attention for conflicts |
| Lazy parsing (LoadAll vs DiscoverAll) | Parsing deferred until needed; discovery is I/O only |
| Module aliases in config | Keeps discovery focused on filesystem; config handles naming |
| Graceful error handling | Invalid modules skipped with warning diagnostics; one bad module doesn't block others (vendored hash mismatches are the exception: hard errors) |
| Vendored modules use `SourceModule` | No new Source enum; vendored modules are modules, distinguished by `ParentModule != nil` |
| Single-level vendored scanning | No recursive nesting; vendored module's own `invowk_modules/` is ignored with diagnostic |
| `ParentModule` field for ownership | Minimal, backward-compatible; enables collision annotation (`(vendored in <parent>)`) and ownership tracking |
| Vendored children inherit `IsGlobalModule` | Scope enforcement treats globally installed vendored commands like their parent's commands |

## File Organization

| File | Purpose |
|------|---------|
| `discovery.go` | Core API and options: `New`, `WithBaseDir`, `WithCommandsDir`, `WithVendoredIntegrityVerification`, `WithProvisionedModuleEntries`, `WithProvisionedGlobalModuleEntries`; `LoadAll`, `LoadFirst`, `CheckModuleCollisions`, `GetEffectiveCommandNamespace`, `loadAllWithDiagnostics`, `ModuleCollisionError` |
| `discovery_files.go` | File/module discovery: `DiscoverAll`, `DiscoverModules` (`ModuleListResult`), `discoverAllWithDiagnostics`, `discoverInDir`, `discoverModulesInDirWithDiagnostics`, `discoverVendoredModulesWithDiagnostics`, `appendModulesWithVendored`, `loadIncludesWithDiagnostics`, `loadProvisionedModulesWithDiagnostics`, `detectModuleShadowing`, `Source` enum, `DiscoveredFile` (with `ParentModule`) |
| `discovery_commands.go` | Command aggregation: `DiscoverCommandSet`, `DiscoverAndValidateCommandSet`, `GetCommand`, `DiscoveredCommandSet` (`Add`, `Analyze`), `CommandInfo`, `SourceID`, `buildModuleCommandName` |
| `discovery_validate.go` | `Validate()` for `CommandInfo`, `DiscoveredFile`, and `LookupResult`, with their `Invalid*Error` types |
| `diagnostic.go` | `Diagnostic` type, `Severity` constants, diagnostic codes, `CommandSetResult`, `LookupResult` |
| `validation.go` | `ValidateCommandTree()` (leaf-only args rule; delegates to `invowkfile.ValidateCommandTree`) and the `ArgsSubcommandConflictError` alias |
| `doc.go` | Package documentation (scope, file map, precedence) |
| `discovery_vendored_test.go` | Vendored discovery tests: parent tracking, ordering, local/include/user-dir module sources, collision annotation, `IsGlobalModule` inheritance. Nested blocking, reserved-name skip, scan failures, and ambiguous-lock skips are in `discovery_vendored_diagnostics_test.go` |
| `discovery_modules_test.go` | Module sources, including provisioned and provisioned-global entries and global marking |
