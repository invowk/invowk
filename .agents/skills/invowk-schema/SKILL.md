---
name: invowk-schema
description: Schema guidelines for invowkfile.cue, invowkmod.cue, pkg/invowkfile/invowkfile_schema.cue, pkg/invowkmod/invowkmod_schema.cue, root invowkfile.cue examples, samples/invowkmods/**/*.cue, command structures, script sources/interpreters, dependency declarations, cross-platform runtime examples, capability checks, and environment variables.
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
as `@source command`. Validation checks both discoverability and scope: module
commands can reference the same module, globally installed modules, or direct
dependencies whose identity/source agree with the lock data. It is a static
declaration check, not a runtime subprocess interceptor.

### Command Structure Validation

#### Leaf-Only Args Constraint

**Commands with positional arguments (`args`) cannot have subcommands.** This is enforced during command discovery and module validation.

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

**The problem:** The native runtime on Windows uses PowerShell (or `cmd`), which cannot parse bash syntax. When a command declares `runtimes: [{name: "native"}, {name: "virtual-sh"}]`, the first runtime (native) becomes the default. This causes bash scripts to fail on Windows with PowerShell parser errors.

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
implementations: [
    {
        script: {content: """
            echo "Hello from bash"
            if [ -f /etc/os-release ]; then
                cat /etc/os-release
            fi
            """}
        runtimes: [{name: "native"}, {name: "virtual-sh"}]
        // Missing platforms restriction - will fail on Windows!
    },
]
```

**When to apply this pattern:**
- Any command with bash/POSIX shell scripts
- That declares both native and virtual-sh runtimes
- And should work on Windows

**Exceptions (no split needed):**
- Commands with `runtimes: [{name: "virtual-sh"}]` only (already cross-platform)
- Commands with `runtimes: [{name: "native"}]` only and `platforms: [{name: "linux"}, {name: "macos"}]` (Linux/macOS only)
- Commands with PowerShell scripts intended for Windows

**Native-only platform-split pattern (for testing):**

When writing native runtime mirror tests (`native_*.txtar`), use native-only implementations without a virtual fallback:

```cue
implementations: [
    {
        script: {content: """
            echo "Hello from native shell"
            echo "VAR=$MY_VAR"
            """}
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: {content: """
            Write-Output "Hello from native shell"
            Write-Output "VAR=$($env:MY_VAR)"
            """}
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

**When to use native-only vs native+virtual-sh:**
- **Native-only** (`runtimes: [{name: "native"}]`): For test files that specifically validate native shell behavior on each platform. Used in `native_*.txtar` mirror tests.
- **Native+virtual-sh** (`runtimes: [{name: "native"}, {name: "virtual-sh"}]`): For production commands that prefer native shell but also allow the embedded shell. Requires the standard platform-split pattern (see above) to avoid PowerShell parsing bash syntax on Windows.

## Invowkmod Modules

### Samples (`samples/invowkmods/` directory)

The `samples/invowkmods/` directory contains sample modules that serve as reference implementations, validation tests, and audit fixtures.

- A module is a `.invowkmod` directory containing required `invowkmod.cue` metadata. `invowkfile.cue` is optional for library-only modules and contains command definitions plus invowkfile-scoped root settings, but no module metadata.
- The invowkmod schema lives in `pkg/invowkmod/invowkmod_schema.cue` and the invowkfile schema in `pkg/invowkfile/invowkfile_schema.cue`.
- Always update sample modules when the invowkmod schema, validation rules, or module behavior changes.
- Modules should demonstrate module-specific features (script file references, cross-platform paths, requirements).
- After module-related changes, run validation against safe samples: `go run . validate samples/invowkmods/io.invowk.sample.invowkmod`.
- Intentionally unsafe audit fixtures under `samples/invowkmods/com.example.audit.*.invowkmod` should be verified with `invowk audit`, not normal validation.

### Current Sample Modules

Read `samples/invowkmods/README.md` and refresh the live inventory with:

```bash
find samples/invowkmods -maxdepth 1 -type d -name '*.invowkmod' | sort
```

### Module Validation Checklist

When modifying module-related code, verify:
1. Safe modules in `samples/invowkmods/` pass validation: `go run . validate samples/invowkmods/io.invowk.sample.invowkmod`.
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
