---
name: invowk-schema
description: Schema and authoring guidelines for invowkfile.cue and invowkmod.cue (pkg/invowkfile/invowkfile_schema.cue, pkg/invowkmod/invowkmod_schema.cue), root invowkfile.cue examples, samples/invowkmods/**/*.cue, command structures and leaf-only args ("has both args and subcommands"), script sources/interpreters, depends_on dependency declarations, cross-platform runtime examples and the native/virtual-sh platform split, capability checks, and environment variables. Use when writing, reviewing or validating commands, examples, schema-facing fields or sample modules.
---

# Invowk Schema Guidelines

## Invowkfile Examples

### Built-in Examples (`invowkfile.cue` at project root)

- Always update the example file when invowkfile definitions or features are added, modified, or removed.
- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g., native "hello-world") to complex (e.g., container "hello-world" with `enable_host_ssh`).
- Examples should cover different features, such as:
  - Native vs. container execution.
  - Volume mounts for container execution.
  - Environment variables.
  - Host SSH access enabled vs. disabled.
  - Capabilities checks (with and without alternatives).
  - Tools checks (with and without alternatives).
  - Custom checks (with and without alternatives).
  - Runtime-level `depends_on` (container only — validated inside the container environment, not on host).
- Module metadata does not belong in invowkfile examples; it lives in `invowkmod.cue` for modules.

### Runtime-Level `depends_on` Pattern (Container Only)

`depends_on` can be placed inside a **container** runtime block to validate against the **container's own environment** (not the host). This is distinct from root/command/implementation-level `depends_on`, which always validates against the **host system**. Runtime-level `depends_on` is only available for the container runtime — native, virtual-sh, and virtual-lua runtimes do not support it (CUE schema rejects it at parse time).

```cue
runtimes: [{
    name: "container"
    image: "debian:stable-slim"
    // Validated INSIDE the container
    depends_on: {
        tools: [{alternatives: ["python3"]}]
    }
}]
// Validated on the HOST
depends_on: {
    tools: [{alternatives: ["docker"]}]
}
```

### Script Source And Interpreter Contract

- Put `interpreter` on `script`, not on runtime config. The schema attaches it to
  the shared script source contract.
- Virtual-sh accepts shell-compatible interpreters only; non-shell interpreters
  belong on native/container implementations or virtual-lua where appropriate.
- Keep inline `script.content`, file-backed script sources, and interpreter
  validation in sync across schema, root examples, docs, and txtar fixtures.

### Command Dependency Scope

`depends_on.cmds` supports bare local command refs and source-qualified refs such
as `@source command`. Validation checks both discoverability and scope; a direct
dependency counts only when its identity/source agree with the lock data. Scope
semantics (same module, global modules, first-level `requires`; a static check,
not a runtime interceptor): `AGENTS.md` "Hard invariants".

### Command Structure Validation

#### Leaf-Only Args Constraint

**Commands with positional arguments (`args`) cannot have subcommands.** This is enforced during command discovery and module validation (`ArgsSubcommandConflictError` in `pkg/invowkfile/command_tree.go`, checked from `internal/discovery/validation.go`; `validateCommandTree` in `cmd/invowk/cmd.go`).

Why: CLI parsers interpret positional arguments after a command name as potential subcommand names. If `deploy` has both `args: [{name: "env"}]` and a subcommand `deploy staging`, running `invowk cmd deploy prod` is ambiguous—is `prod` an argument or a subcommand name?

**Valid:**
```cue
// Leaf command with args (no subcommands)
cmds: [
    {name: "deploy", args: [{name: "env"}], ...}
]

// Parent command with subcommands (no args on parent)
cmds: [
    {name: "deploy"},           // No args here
    {name: "deploy staging"},   // Subcommand
    {name: "deploy prod"},      // Subcommand
]
```

**Invalid:**
```cue
cmds: [
    {name: "deploy", args: [{name: "env"}], ...},  // Has args
    {name: "deploy staging", ...}                   // Is a subcommand of deploy
]
// Error: command 'deploy' has both args and subcommands
```

This validation runs:
1. During `invowk cmd` execution (command discovery)
2. During `invowk validate <module-path>`

### Cross-Platform Runtime Selection

**Bash scripts with native+virtual-sh runtimes must use platform-specific implementations.**

**The problem:** The native runtime on Windows uses PowerShell (or `cmd`), which
cannot parse bash syntax. Runtime selection uses this precedence: an explicit
CLI override, then a compatible configured `default_runtime`, then the command
default (the first runtime in the first implementation matching the selected
platform). Therefore, a Windows-matching implementation that lists `native`
before `virtual-sh` can select native by default and feed Bash syntax to
PowerShell.

**The solution:** Split implementations by platform:
- **Linux/macOS**: Use `runtimes: [{name: "native"}, {name: "virtual-sh"}]` with `platforms: [{name: "linux"}, {name: "macos"}]`
- **Windows**: Use `runtimes: [{name: "virtual-sh"}]` with `platforms: [{name: "windows"}]` (virtual-sh uses `mvdan/sh`, a cross-platform POSIX shell)

**Valid (platform-specific implementations):**
```cue
implementations: [
    {
        script: {content: """
            echo "Hello from bash"
            if [ -f /etc/os-release ]; then
                cat /etc/os-release
            fi
            """}
        runtimes:  [{name: "native"}, {name: "virtual-sh"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: {content: """
            echo "Hello from bash"
            if [ -f /etc/os-release ]; then
                cat /etc/os-release
            fi
            """}
        runtimes:  [{name: "virtual-sh"}]
        platforms: [{name: "windows"}]
    },
]
```

**Invalid (bash script without platform restriction):**
```cue
implementations: [{
    script:   {content: "..."}  // same bash script as above
    runtimes: [{name: "native"}, {name: "virtual-sh"}]
    // Missing platforms restriction - will fail on Windows!
}]
```

**When to apply this pattern:**
- Any command with bash/POSIX shell scripts
- That declares both native and virtual-sh runtimes
- And should work on Windows

**Exceptions (no split needed):**
- Commands with `runtimes: [{name: "virtual-sh"}]` only (already cross-platform)
- Commands with `runtimes: [{name: "native"}]` only and `platforms: [{name: "linux"}, {name: "macos"}]` (Linux/macOS only)
- Commands with PowerShell scripts intended for Windows

**Native-only vs native+virtual-sh:**
- **Native-only** (`runtimes: [{name: "native"}]`, split into a `linux`/`macos` implementation and a PowerShell `windows` implementation): for `native_*.txtar` mirror tests that validate native shell behavior on each platform. Rules: `.agents/rules/testing.md` "Native tests"; generation example: `.agents/skills/native-mirror/SKILL.md`.
- **Native+virtual-sh** (`runtimes: [{name: "native"}, {name: "virtual-sh"}]`): for production commands that prefer native shell but also allow the embedded shell. Requires the platform split above to avoid PowerShell parsing bash syntax on Windows.

## Invowkmod Modules

### Samples (`samples/invowkmods/` directory)

The `samples/invowkmods/` directory contains sample modules that serve as reference implementations, validation tests, and audit fixtures.

- A module is a `.invowkmod` directory containing required `invowkmod.cue` metadata. `invowkfile.cue` is optional for library-only modules and contains command definitions plus invowkfile-scoped root settings, but no module metadata.
- Always update sample modules when the invowkmod schema, validation rules, or module behavior changes.
- Modules should demonstrate module-specific features (script file references, cross-platform paths, requirements).
- All samples, including the `com.example.audit.*.invowkmod` fixtures, pass `invowk validate` (the module-related loop in `.agents/rules/checklist.md`). The audit fixtures are intentionally malicious: do not run or sanitise their commands, and exercise their purpose with `invowk audit` (commands in `samples/invowkmods/README.md`).

### Current Sample Modules

Read `samples/invowkmods/README.md` and refresh the live inventory with:

```bash
find samples/invowkmods -maxdepth 1 -type d -name '*.invowkmod' | sort
```

### Module Validation Checklist

When modifying module-related code, verify:
1. All modules in `samples/invowkmods/` pass validation (run the module-related loop in `.agents/rules/checklist.md`).
2. Module naming conventions and module ID matching are enforced.
3. `invowkmod.cue` is required and parsed; optional `invowkfile.cue` contains commands and invowkfile-scoped root settings, not module metadata.
4. Script path resolution works correctly (forward slashes, relative paths).
5. Nested module detection works correctly.
6. The `pkg/invowkmod/` tests pass: `go test -v ./pkg/invowkmod/...`.

When modifying invowkfile schema or root examples, also run:

```bash
go run . validate ./invowkfile.cue
go test -v ./pkg/invowkfile/...
```

When behavior changes reach CLI fixtures, run the relevant `tests/cli` txtar
selection or `make test-cli`.

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Stale sample modules | Validation fails after schema changes | Update safe samples in `samples/invowkmods/` after module-related changes |
| Missing platform restrictions | Bash scripts fail on Windows | Add platform-specific implementations for native+virtual-sh runtimes |
| Args with subcommands | Discovery validation error | Commands with positional args cannot have subcommands |

For path handling in implementations, see `.agents/rules/windows.md`.
