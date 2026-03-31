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

### The nuclear option (maximum security)

If maximum security is the priority: **remove transitive resolution but keep single-level requires**, and require every dependency to be explicitly declared in the consuming module's `invowkmod.cue`. This is the Go modules approach — no implicit transitive deps, everything is explicit. It's more verbose but much easier to audit.

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

The biggest immediate win is **enforcing `CanCall()` + adding content-hash verification**. These two changes close the real security gaps (unenforced visibility + no tamper detection) without removing a useful feature.

The sandboxing idea (virtual-shell-only for transitive deps) is architecturally interesting but **not viable without a restrictive `ExecHandler` and env isolation** — both of which require careful allowlist design. This is a longer-term research direction, not a quick win.
