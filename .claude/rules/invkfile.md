# Invkfile Examples

## Built-in Examples (`invkfile.cue` at project root)

- Always update the example file when invkfile definitions or features are added, modified, or removed.
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
- Module metadata does not belong in invkfile examples; it lives in `invkmod.cue` for modules.

## Command Structure Validation

### Leaf-Only Args Constraint

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
2. During `invowk module validate --deep`
