# Module Supply Chain Security

> Brainstorming analysis — captures design rationale for invowk's module dependency security posture.

## The Question

**Should we remove the transitive modules/dependencies feature to reduce supply chain attack surface, or is there a better approach?**

Invowk is fundamentally a code execution engine. With supply chain attacks becoming increasingly common, the module dependency system — which fetches and executes third-party code — deserves careful scrutiny. The transitive dependency feature amplifies this risk: a module you trust can pull in modules you don't know about.

## Current State

### What the module dependency system does

- Modules declare `requires` in `invowkmod.cue` with a Git URL + semver constraint.
- Resolution clones repos, resolves versions, and recursively resolves transitive deps.
- Lock file pins to exact Git commit SHAs.
- Vendoring copies resolved modules into `invowk_modules/`.

### Security measures that exist

- **Path traversal prevention**: rejects `..`, absolute paths, null bytes.
- **Symlink rejection**: flagged as security issues during module validation.
- **Git commit SHA pinning** in lock file for reproducibility.
- **Discovery depth limit**: only scans one level deep (no nested `invowk_modules/` recursion).

### Security measures that DON'T exist

- No code signing or checksum verification of module content.
- No SBOM or provenance attestation.
- `CanCall()` visibility enforcement is **not wired into the execution path** — it's defined and tested but unenforced at runtime.
- No protection against Git tag mutation (someone replaces a tag pointing to a different commit).

## Analysis

### Why not remove transitive deps entirely

1. **Composability is the core value proposition** of a module system. Without transitive deps, every module author must inline everything or users must manually declare deeply nested dependency chains — which is worse for security because it's harder to audit.
2. **Flat deps with manual declaration** just shifts the supply chain risk without reducing it. Users would still pull untrusted code, just with more friction.

### Recommended improvements (ordered by impact)

1. **Enforce `CanCall()` at runtime** — this is the low-hanging fruit. The code exists in `pkg/invowkmod/command_scope.go`, it's tested, it just needs to be wired into `commandsvc` dispatch. This makes the "transitive deps can't call your commands" rule a real boundary, not just documentation.

2. **Add content-hash verification** — store a SHA-256 of the module tree in the lock file alongside the Git commit. Verify on every load. This catches tag mutation and tampering after clone.

3. **Add an `invowk audit` command** — similar to `npm audit` or `go mod verify`. Show the full dependency tree, flag modules that have changed since lock, and surface any known issues.

4. **Consider an allowlist/deny-list model** — instead of "trust everything in `requires`", let users declare which module namespaces are trusted. Untrusted modules could run in a restricted mode (e.g., virtual shell only, no native execution).

5. **Sandboxing for untrusted modules** — invowk already has the virtual shell runtime (mvdan/sh). Transitive deps could default to virtual-only execution unless explicitly promoted to native/container. **However, see the critical caveat below.**

### The nuclear option: explicit-only dependencies (no transitive resolution)

Modeled after **Go modules** — arguably the most security-conscious dependency model in mainstream use today. The core principle: every module in the dependency tree must appear explicitly in **your** `invowkmod.cue`. No implicit transitive resolution.

#### How it works today (transitive)

```
Module A (your project)
  requires: Module B

Module B
  requires: Module C    ← invowk auto-resolves this
  requires: Module D    ← and this

Module C
  requires: Module E    ← and this, recursively
```

You declare `B`. Invowk's `resolveAll()` in `pkg/invowkmod/resolver_deps.go` recursively clones and resolves `C`, `D`, and `E` automatically via `loadTransitiveDeps()`. Your lock file ends up with 4 modules, but you only explicitly chose 1.

#### How it would work with explicit-only

```
Module A (your project)
  requires: Module B
  requires: Module C    ← YOU must declare this
  requires: Module D    ← and this
  requires: Module E    ← and this
```

Every module in the dependency tree must appear in **your** `invowkmod.cue`. If `B` needs `C` but you didn't declare `C`, invowk fails with an actionable error:

```
error: module "B" requires "C", but "C" is not declared in your invowkmod.cue.
Add it to your requires list:
  requires: [{git_url: "https://github.com/foo/C.invowkmod", version: "^1.0.0"}]
```

#### Why this is "nuclear" for security

1. **Full visibility**: You can't accidentally pull in code you didn't explicitly approve. Every dependency is a conscious decision with a visible line in your config.
2. **Audit surface is flat**: `invowkmod.cue` is the complete list of everything that will run. No hidden transitive chains. Security tools can scan one file to know your full exposure.
3. **Tag mutation/compromise is scoped**: If module `E` gets compromised, only projects that explicitly declared `E` are affected. In the transitive model, any project using `B` is silently exposed through the chain `B → C → E`.
4. **No "leftpad" problem**: A deep transitive dependency being unpublished or compromised can't cascade through the graph.

#### Why it's painful

1. **Verbose `requires` lists**: If a module has a deep dependency tree, users must declare every node. This is the Go modules experience — `go.mod` files can get long.
2. **Version coordination burden**: When `B` upgrades and now requires `C@v2` instead of `C@v1`, users must manually update their `C` declaration too. In the transitive model, `resolveAll()` handles this.
3. **Module authors can't encapsulate**: A module can't hide its implementation detail dependencies. If `B` internally refactors from `C` to `F`, all consumers must update their `requires`.

#### The Go modules precedent

Go chose this model deliberately. From the Go modules design doc:

> *"A module can only use packages from its own modules or from modules explicitly required in its go.mod."*

Go mitigates the pain with `go mod tidy`, which automatically adds missing transitive requirements to your `go.mod`. The key insight is: **you still see every dependency**, even if a tool adds them for you. The tool provides convenience; the explicitness provides auditability.

#### How this aligns with invowk's existing 1-level visibility design

Invowk already has a partial version of this philosophy built in — but it's inconsistently applied:

**What already works in this direction:**

- **Discovery is already depth-1**: `discoverVendoredModulesWithDiagnostics` in `internal/discovery/discovery_files.go` only scans `invowk_modules/` one level deep. Nested `invowk_modules/` directories inside vendored modules emit a `SeverityWarning` diagnostic (`CodeVendoredNestedIgnored`). Discovery already treats the module graph as flat.

- **`CommandScope.CanCall()` enforces 1-level visibility**: The rule "commands from module A can only call commands from module A's direct dependencies, not transitive ones" is already coded in `pkg/invowkmod/command_scope.go`. Only direct deps from `requires` are added to the `DirectDeps` map. This means even if transitive modules are resolved, their commands are invisible to the root module at the visibility level.

- **Vendoring is already flat**: `VendorModules()` in `pkg/invowkmod/operations_vendor.go` copies resolved modules into a single `invowk_modules/` directory, not nested. The on-disk layout is already flat even though the resolution was recursive.

**What contradicts this direction:**

- **`resolveAll()` recursively resolves**: Despite discovery and visibility being depth-1, the resolver in `pkg/invowkmod/resolver_deps.go` recursively calls `resolveOne()` for every transitive `requires`. It clones repos, resolves semver, and stores everything in the lock file — even modules the root will never be able to call.

- **Lock file includes everything**: `invowkmod.lock.cue` contains entries for the entire transitive graph, even though only depth-1 modules are usable. This means `invowk module sync` fetches code that the root module can never interact with (assuming `CanCall()` is enforced).

- **The gap creates a confusing security posture**: Transitive modules are fetched, vendored, and present on disk — but (once `CanCall()` is enforced) their commands are invisible. An attacker who compromises a transitive module still gets their code cloned to the user's machine and present in `invowk_modules/`. The code exists; it just can't be *called*. But "code on disk that can't be called" is still code on disk — it's in the build environment, CI pipelines, and container layers.

#### What implementation would look like

Given the existing 1-level-deep design in discovery and visibility, the nuclear option would actually be a *simplification* — aligning resolution with the rest of the system:

1. **Stop recursive resolution in `resolveAll()`**: Change `resolveAll()` to resolve only the modules in the input `requirements` list — don't call `resolve()` recursively for `TransitiveDeps`. This is the core change: one `if` guard or removing the recursive call after the `resolveOne()` loop.

2. **Add a validation pass**: After resolving all declared deps, iterate each resolved module's `TransitiveDeps` (from `loadTransitiveDeps()`) and verify each one is also declared in the root `invowkmod.cue`. Fail with actionable errors if not. This is the "fail loudly" step that tells users exactly what to add.

3. **Add `invowk module tidy`**: Auto-scan all resolved modules' `requires`, and add any missing transitive requirements to the root `invowkmod.cue`. This is the convenience layer — like `go mod tidy`. Users run it once after adding a dependency, review the additions, and commit. The tool provides convenience; the explicitness provides auditability.

4. **Keep the lock file**: Even with explicit-only deps, the lock file still provides reproducibility via commit SHA pinning. The lock file would now be a 1:1 mirror of `requires` (no extra transitive entries), making it easier to audit.

5. **Simplify vendoring**: Since all deps are explicitly declared and resolution is flat, `VendorModules()` would work exactly as today — no changes needed (it's already flat).

#### The middle ground: transitive resolution with explicit approval

A less disruptive alternative that preserves convenience while adding a consent checkpoint:

- **Keep `resolveAll()` recursive** for discovery purposes.
- After resolution, **show the user the full transitive tree** and require explicit approval before writing the lock file (similar to `npm` showing what it's installing).
- Store an `approved_transitive` list in `invowkmod.cue` or a separate file, so subsequent `sync` operations don't re-prompt for already-approved modules.
- New transitive deps (from upstream upgrades) trigger a re-approval prompt.

This preserves the convenience of automatic resolution while ensuring informed consent — every transitive dependency was explicitly seen and approved by the user at least once.

#### Assessment

The nuclear option is **the right call if invowk modules will be shared publicly** (like npm packages or Go modules). Once there's a public ecosystem, supply chain attacks become a real threat and the Go model is proven.

If modules are primarily **private/internal** (teams sharing within an org), the current model with `CanCall()` enforcement + content-hash verification is probably sufficient — the trust boundary is the org itself.

The fact that discovery, visibility, and vendoring are already depth-1 makes the nuclear option surprisingly low-effort to implement — it's mostly about **removing** recursive resolution code rather than adding new systems. The hardest part is `invowk module tidy`, which is a convenience feature, not a security one.

### The 2nd nuclear option: no module-to-module dependencies at all

A fundamentally different approach: **remove the `requires` concept from modules entirely**. Modules become inert, self-contained packages of commands. The only place that references modules is the user's `config.cue` via `includes`.

#### Key design principle

The dependency concept is removed from the module system entirely — not moved, not flattened, *eliminated*. Module authors cannot declare dependencies on other modules. Only the user's configuration layer (`config.cue` `includes`) controls which modules are active, and that is always under the user's direct control.

#### How it works today

```
invowkmod.cue (module B):
  requires: [Module C, Module D]   ← modules can reference other modules

config.cue (user):
  includes: [{path: "B.invowkmod"}]   ← user also references modules
```

Two separate mechanisms reference modules: `invowkmod.cue:requires` (module-to-module) and `config.cue:includes` (user-to-module). The resolver recursively fetches transitive deps from `requires`. The `includes` path is flat but `requires` isn't.

#### How it would work

```
invowkmod.cue (module B):
  # No requires field exists in the schema at all.
  # Module B is self-contained.

config.cue (user):
  includes: [
      {path: "/home/user/.invowk/modules/B.invowkmod"},
      {path: "/home/user/.invowk/modules/C.invowkmod"},  ← user adds this explicitly
  ]
```

Modules have no `requires`. They define commands and scripts — nothing more. The user controls 100% of what is on disk and what is active via a single mechanism (`includes`).

The git-fetching convenience becomes a separate CLI command:
```bash
# Clones repo, resolves version, adds an includes entry to config.cue
invowk module add https://github.com/org/build-tools.invowkmod@^1.0.0
```

#### Research findings that inform the design

Two research findings are critical to understanding how this would work in practice:

**Finding 1: `depends_on.cmds` is a discoverability check only, never execution.**

When a command declares `depends_on.cmds: ["bar"]`, invowk does NOT execute `bar` before the main command. It only checks that `bar` is *discoverable* — i.e., that it exists somewhere in the aggregated command set. This is explicitly documented in `internal/app/deps/deps.go` (`ValidateDependencies`, line 15): *"depends_on.cmds is a discoverability check only... Neither phase executes the referenced commands."*

The validation happens at execution time (in `dispatchExecution()` at `internal/app/commandsvc/dispatch.go`, line 62), after the execution context is built but before any script runs. It calls `CheckCommandDependenciesExist()` which runs the full discovery pipeline and checks a flat `available` map.

This means `depends_on.cmds` is essentially "assert this command is installed/available on this system" — a precondition guard, not an execution dependency.

**Finding 2: All included modules' commands live in one flat namespace with full cross-visibility.**

Discovery in `internal/discovery/discovery_commands.go` (`DiscoverCommandSet`, line 185) aggregates commands from all sources — current directory, local modules, config includes, and user cmds dir — into a single flat `DiscoveredCommandSet.ByName` map. There is no namespace isolation between modules at the discovery level.

This means: if modules A and B are both in `config.cue` `includes`, a command in module A that declares `depends_on.cmds: ["bar"]` where `bar` is from module B will pass validation — because `bar` is in the aggregated command set.

`CommandScope.CanCall()` in `pkg/invowkmod/command_scope.go` — which could restrict cross-module visibility — is **never called in any production code path**. It exists only in tests. Today, all included modules can see all other included modules' commands, with no enforcement boundary.

#### How this design resolves cross-module `depends_on`

Under the 2nd nuclear option, the cross-module `depends_on.cmds` scenario works exactly as it does today, but with a cleaner mental model:

```cue
// config.cue — user controls everything
includes: [
    {path: "deploy-tools.invowkmod"},   // has command "deploy" with depends_on.cmds: ["build"]
    {path: "build-tools.invowkmod"},    // has command "build"
]
```

- `deploy-tools` defines a `deploy` command with `depends_on.cmds: ["build"]`.
- `build-tools` defines a `build` command.
- Both are in `config.cue` `includes`, so both are discovered.
- When user runs `invowk cmd deploy`, the `depends_on.cmds` check runs `CheckCommandDependenciesExist()`, finds `build` in the aggregated command set, and passes.
- The `deploy` script runs. If it needs to call `build`, it does so explicitly via `invowk cmd build` in its script — this is a subprocess call, not a `depends_on` mechanism.

The key insight: **`depends_on.cmds` was never about module-to-module relationships**. It's about "this command needs another command to be available on this system." Under the 2nd nuclear option, the user explicitly made both available via `config.cue`. No module needed to "know about" another module.

#### What happens to `CommandScope.CanCall()`

Since `CanCall()` is never called in production today, there are two options:

1. **Remove it entirely**: Under this design, all included modules' commands are in the same flat namespace with full cross-visibility. `CanCall()` scoping is unnecessary because there are no transitive deps to hide from — everything the user includes is intentionally visible.

2. **Keep it for future use**: If a future feature needs to restrict which modules can reference which (e.g., "module A should only see module B"), `CanCall()` could be wired in. But this would be an opt-in governance feature, not a security feature — the user already controls what's included.

The recommendation is to **remove it** (or at minimum, remove the `DirectDeps` and `GlobalModules` concepts). With no module-to-module `requires`, the distinction between "direct dep" and "global module" is meaningless. Everything is user-included.

#### What would be removed / simplified

| Component | Current state | After 2nd nuclear option |
|-----------|--------------|--------------------------|
| `invowkmod.cue` `requires` field | Declares git URL + semver deps | **Removed from schema** |
| `Resolver` / `resolveAll()` / `resolveOne()` | Recursive git clone + semver resolution | **Removed entirely** |
| `loadTransitiveDeps()` | Parses dep module's `requires` for recursive resolution | **Removed** |
| Lock file (`invowkmod.lock.cue`) | Pins entire transitive graph | **Removed or simplified** (git SHAs could move to a simpler manifest alongside config) |
| `VendorModules()` / `invowk_modules/` | Copies resolved modules from cache | **Simplified** — modules live where the user puts them (local path or `~/.invowk/modules/`) |
| `CommandScope` / `CanCall()` | Enforces 1-level visibility (never called in prod) | **Removed** — flat namespace, user controls visibility via includes |
| `config.cue` `includes` | References local module paths | **Unchanged** — becomes the sole module reference mechanism |
| `CheckCommandDependenciesExist()` | Checks aggregated command set | **Unchanged** — works exactly as today |
| Git fetching | Built into resolver | **Separate CLI command** (`invowk module add <url>`) |

#### Comparison with 1st nuclear option

| Aspect | 1st nuclear option (Go-style) | 2nd nuclear option (no module deps) |
|--------|-------------------------------|-------------------------------------|
| Where modules are referenced | `invowkmod.cue` `requires` | `config.cue` `includes` only |
| Who controls module list | Module authors + root user | **User only** |
| Dependency concept in modules | Exists but explicit-only | **Doesn't exist** |
| Resolver complexity | Simplified (no recursion, still needed) | **Removed entirely** |
| Lock file | Still needed (simplified) | Removed or minimal |
| `invowk module tidy` needed | Yes | No |
| Implementation effort | Medium (modify resolver) | **Low (delete resolver)** |
| Module composability | Preserved (with friction) | Not supported |
| Attack surface from modules | Module authors can influence dep list | **Zero** — user controls everything |
| Fits invowk's use case | Yes | **Better** |

#### Why this fits invowk better than the 1st nuclear option

Invowk is a **command runner**, not a package manager. The unit of reuse is a command definition with a script — not a library with an API surface. The need for deep module composition is fundamentally different from npm/Go where code calls code.

1. **Commands are the reuse unit, not code.** An invowk module provides commands with scripts. Those scripts use system tools (`git`, `docker`, `curl`, `make`) directly — not other invowk modules' internal commands.

2. **Scripts are self-contained by nature.** A shell script that needs functionality from another script typically `source`s it or calls it as a subprocess. It doesn't go through a module dependency system.

3. **`depends_on.cmds` already proves the model works.** It's a discoverability check against the flat command namespace. It doesn't care *which module* a command came from — only that it exists. This is the natural model for a config-driven includes system.

4. **Duplication in scripts is cheap.** A 20-line deploy script duplicated across two modules is nothing compared to the security/complexity cost of transitive dependencies.

5. **The alternative (forking/copying) is better for security.** When you copy a module's commands instead of depending on it, you pin to a specific version of the *code*, not just the *reference*. No upstream supply chain risk.

The case where this hurts is large enterprise module ecosystems with shared "base infrastructure" modules. But even then, the org could use a monorepo with a single large `invowkmod` containing all shared commands, rather than splitting across interdependent modules.

#### Assessment

The 2nd nuclear option is the strongest security choice for invowk. It eliminates the entire dependency resolution attack surface by removing the concept entirely — modules are self-contained, the user controls what's active, and the flat command namespace already supports cross-module `depends_on.cmds` checks without any module-to-module awareness.

The implementation is primarily **deletion**: remove the resolver, the lock file, `CommandScope`, and the `requires` schema field. The `config.cue` `includes` mechanism and `CheckCommandDependenciesExist()` work unchanged. The main new work is the `invowk module add <url>` convenience command.

## Virtual Shell Is NOT a Sandbox

**Critical finding: the virtual shell runtime does not provide any security boundary.**

The virtual shell's `execHandler` middleware chain in `internal/runtime/virtual.go` works as follows:

1. Try the command against u-root built-in registry (28 commands: `cat`, `cp`, `grep`, `rm`, `ls`, `mkdir`, etc.).
2. If not a built-in, **unconditionally call `next(ctx, args)`** — which is `interp.DefaultExecHandler`.
3. `DefaultExecHandler` resolves the command from the host `$PATH` via `LookPathDir` and runs it with `exec.Cmd.Start()`.

This means any virtual shell script can execute arbitrary host binaries (`git`, `curl`, `python`, `bash`, `wget`, etc.) as long as they exist in `$PATH`. The default env mode is `EnvInheritAll`, so the host's full `$PATH` is inherited.

**mvdan/sh v3.13.0 has no sandbox API.** The only restriction mechanism is the `ExecHandlers` middleware itself, which invowk currently uses additively (intercept known commands) rather than restrictively (block unknown commands).

### What would be needed to make virtual shell a real sandbox

1. **Restrictive `ExecHandler`** — return an error for any command not in an explicit allowlist, instead of calling `next`:

   ```go
   // Current (permissive):
   return next(ctx, args)

   // Sandboxed (restrictive):
   if !allowlist[args[0]] {
       return fmt.Errorf("command %q not allowed in sandboxed mode", args[0])
   }
   return next(ctx, args)
   ```

2. **Strict env inheritance** — switch to `EnvInheritNone` or a strict allowlist for sandboxed modules so `$PATH` and sensitive env vars aren't leaked.

3. **Allowlist design** — this is the hard problem. Too restrictive and modules are useless; too permissive and the sandbox is meaningless. Possible approaches:
   - Only u-root built-ins (28 commands) — very restrictive, most real scripts would break.
   - Curated "safe" list (u-root + `jq`, `sed`, `awk`, etc.) — subjective and maintenance burden.
   - Module-declared capabilities (`needs: [git, curl]`) requiring user approval — most flexible but complex.

### Implication for recommendation #5

Option 5 ("sandbox transitive deps via virtual-shell-only") **does not work today** and requires significant implementation effort. The virtual shell is a *portable shell interpreter*, not a security boundary. This shifts the recommendation priority: content-hash verification and `CanCall()` enforcement remain the best immediate wins, while true sandboxing is a longer-term research item.

## Recommendation

**Preferred direction: 2nd nuclear option (no module-to-module dependencies).**

This is the strongest security choice and the best fit for invowk's nature as a command runner. Modules become self-contained packages of commands; only `config.cue` `includes` references modules; the entire resolver/lock-file/`CommandScope` system is removed. Cross-module `depends_on.cmds` continues to work unchanged because it checks the flat aggregated command namespace, not module relationships. The implementation is primarily deletion, with the main new work being an `invowk module add <url>` convenience command.

**If the 2nd nuclear option is too aggressive (breaking change for existing users with `requires`):**
- **1st nuclear option** (Go-style explicit-only): keep `requires` but remove transitive resolution. Users must declare every dependency. Add `invowk module tidy` for convenience. Medium effort.
- **Middle ground**: keep transitive resolution but add an explicit approval checkpoint. Least disruptive, still provides informed consent.
- **Minimum viable improvement**: enforce `CanCall()` at runtime + add content-hash verification. Closes the two biggest gaps (unenforced visibility + no tamper detection) without changing the dependency model.

**Not viable without significant work:**
- Virtual-shell sandboxing requires a restrictive `ExecHandler`, env isolation, and careful allowlist design. This is a longer-term research direction, not a quick win.
