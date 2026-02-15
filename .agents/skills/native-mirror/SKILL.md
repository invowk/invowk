---
name: native-mirror
description: Generate native_*.txtar mirror tests from virtual_*.txtar tests with platform-split CUE patterns and exemption rules. Use when creating or updating testscript CLI tests that need native runtime mirrors.
user-invocable: true
disable-model-invocation: false
---

# Native Mirror Generator

Generate `native_*.txtar` test files that mirror existing `virtual_*.txtar` tests using native shell implementations with platform-split CUE.

## When to Use

Invoke this skill (`/native-mirror`) when:
- A new `virtual_*.txtar` test has been created and needs a native mirror
- An existing virtual test has been modified and its native mirror needs updating
- You want to verify which virtual tests are missing native mirrors

## Workflow

### Step 1: Check Exemptions

Before generating a mirror, verify the virtual test is NOT exempt:

**Exempt categories** (do NOT create native mirrors):
- `virtual_uroot_*.txtar` — u-root commands are virtual shell built-ins
- `virtual_shell.txtar` — Tests virtual-shell-specific features
- `container_*.txtar` — Linux-only container runtime
- `virtual_edge_cases.txtar` — CUE schema validation, not runtime behavior
- `virtual_args_subcommand_conflict.txtar` — CUE schema validation
- `virtual_ambiguity.txtar` — Command resolution logic
- `virtual_disambiguation.txtar` — Command resolution logic
- `virtual_multi_source.txtar` — Command resolution logic
- `dogfooding_invowkfile.txtar` — Already exercises native runtime
- `config_*.txtar` — Built-in CLI commands (Cobra handlers, not user-defined runtimes)
- `module_*.txtar` — Built-in CLI commands (Cobra handlers, not user-defined runtimes)
- `completion.txtar` — Built-in CLI command (shell completion generation)
- `tui_format.txtar`, `tui_style.txtar` — Built-in CLI commands (non-interactive TUI utilities)
- `init_*.txtar` — Built-in CLI command (project initialization)

If the test is exempt, report it and stop.

### Step 2: Read the Virtual Test

Read the source `virtual_*.txtar` file and extract:
1. The test description comments
2. All `exec` commands and `stdout`/`stderr` assertions
3. The embedded `invowkfile.cue` and any other files
4. Environment variable usage

### Step 3: Transform the CUE

Replace each virtual implementation with a platform-split native pair:

**From (virtual)**:
```cue
implementations: [{
    script: "echo 'Hello'"
    runtimes: [{name: "virtual"}]
    platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
}]
```

**To (native, platform-split)**:
```cue
implementations: [
    {
        script: "echo 'Hello'"
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: "Write-Output 'Hello'"
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

### Step 4: Translate Shell Syntax

Apply these translations for the Windows (PowerShell) implementation:

| Bash/Zsh | PowerShell |
|----------|------------|
| `echo "text"` | `Write-Output "text"` |
| `echo -n "text"` | `Write-Host -NoNewline "text"` |
| `$VAR` | `$env:VAR` |
| `"Value: $VAR"` | `"Value: $($env:VAR)"` |
| `if [ "$V" = "x" ]; then ... fi` | `if ($env:V -eq 'x') { ... }` |
| `if [ -n "$V" ]; then ... fi` | `if ($env:V) { ... }` |
| `if [ -z "$V" ]; then ... fi` | `if (-not $env:V) { ... }` |
| `export VAR=val` | `$env:VAR = 'val'` |
| `set -e` | `$ErrorActionPreference = 'Stop'` |
| `for x in a b c; do ... done` | `foreach ($x in @('a','b','c')) { ... }` |
| `$INVOWK_FLAG_NAME` | `$env:INVOWK_FLAG_NAME` |
| `$INVOWK_ARG_NAME` | `$env:INVOWK_ARG_NAME` |

### Step 5: Preserve Assertions

The `stdout` and `stderr` assertions must be **identical** between virtual and native tests. Only the script syntax differs, not the output.

### Step 6: Write the Mirror

Create `native_<feature>.txtar` with:
- Updated description noting it's a native mirror
- Same `exec` commands and assertions
- Platform-split CUE implementations
- Same embedded support files

### Step 7: Verify

```bash
make test-cli
```

## Example

Given `virtual_simple.txtar`:
```txtar
# Test: Basic hello command
cd $WORK
exec invowk cmd hello
stdout 'Hello from invowk!'
! stderr .

-- invowkfile.cue --
cmds: [{
    name: "hello"
    description: "Say hello"
    implementations: [{
        script: "echo 'Hello from invowk!'"
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}]
```

Generate `native_simple.txtar`:
```txtar
# Test: Basic hello command (native runtime mirror)
# Native shell mirror of virtual_simple.txtar
cd $WORK
exec invowk cmd hello
stdout 'Hello from invowk!'
! stderr .

-- invowkfile.cue --
cmds: [{
    name: "hello"
    description: "Say hello"
    implementations: [
        {
            script: "echo 'Hello from invowk!'"
            runtimes:  [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        {
            script: "Write-Output 'Hello from invowk!'"
            runtimes:  [{name: "native"}]
            platforms: [{name: "windows"}]
        },
    ]
}]
```

## Audit Mode

To check for missing native mirrors, run:

```bash
# List virtual tests
ls tests/cli/testdata/virtual_*.txtar

# List native tests
ls tests/cli/testdata/native_*.txtar

# Compare (accounting for exemptions)
```

Report which virtual tests are missing their native mirrors, excluding exempt files.
