---
name: native-mirror
description: Generate, update, or audit native_*.txtar mirror tests from virtual_*.txtar tests with platform-split CUE patterns and runtime mirror exemption rules. Use when creating or reviewing testscript CLI tests that need native runtime mirrors.
---

# Native Mirror Generator

Generate `native_*.txtar` test files that mirror existing `virtual_*.txtar` tests using native shell implementations with platform-split CUE.

## Workflow

### Step 1: Check Exemptions

Before generating a mirror, verify the virtual test is NOT exempt:

> **Source of truth**: The machine-enforced exemption list is in
> `tests/cli/runtime_mirror_exemptions.json`. Do not duplicate it in this skill.
> `TestShRuntimeMirrorCoverage` enforces the JSON entries; `TestVirtualNativeCommandPathAlignment`
> enforces command-path alignment except for justified `command_path_exempt` entries.

Only `virtual_*.txtar` files are in this skill's mirror scope. Files such as
`container_*.txtar`, `config_*.txtar`, `module_*.txtar`, `completion.txtar`,
`tui_format.txtar`, `tui_style.txtar`, and `init_*.txtar` are outside the mirror
scanner rather than runtime-mirror exemptions.

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
    script: {content: "echo 'Hello'"}
    runtimes: [{name: "virtual-sh"}]
    platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
}]
```

**To (native, platform-split)**:
```cue
implementations: [
    {
        script: {content: "echo 'Hello'"}
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: {content: "Write-Output 'Hello'"}
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

The `stdout`/`stderr` assertions and `exec` command paths should match between
virtual and native tests by default. Intentional output or command-path divergence
must be recorded in `tests/cli/runtime_mirror_exemptions.json` with a justification,
using `command_path_exempt` for command target differences.

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
        script: {content: "echo 'Hello from invowk!'"}
        runtimes: [{name: "virtual-sh"}]
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
            script: {content: "echo 'Hello from invowk!'"}
            runtimes:  [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        {
            script: {content: "Write-Output 'Hello from invowk!'"}
            runtimes:  [{name: "native"}]
            platforms: [{name: "windows"}]
        },
    ]
}]
```

## Audit Mode

To check for missing native mirrors, run:

```bash
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...
```

Use the test output plus `tests/cli/runtime_mirror_exemptions.json` to report missing
mirrors, stale exemptions, or command-path divergences.
