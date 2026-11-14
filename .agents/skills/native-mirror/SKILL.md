---
name: native-mirror
description: >-
  Generate, update, or audit `native_*.txtar` mirrors of `virtual_*.txtar`
  testscript tests: platform-split CUE, mirror exemption rules
  (`runtime_mirror_exemptions.json`), and
  `TestShRuntimeMirrorCoverage`/`TestVirtualNativeCommandPathAlignment`
  failures (missing, orphan or superseded mirrors, command-path mismatch). Use
  when creating or reviewing testscript CLI tests that need native runtime
  mirrors.
---

# Native Mirror Generator

Generate `native_*.txtar` test files that mirror existing `virtual_*.txtar` tests using native shell implementations with platform-split CUE.

## Workflow

Choose the mode before acting:

- **Generate or update**: write the requested `native_*.txtar` mirror, then
  verify it.
- **Audit or review**: remain read-only and report missing pairs, stale
  exemptions, or divergence. Do not create mirrors unless the user also asks
  for implementation.

### Step 1: Check Exemptions

Before generating a mirror, verify the virtual test is NOT exempt:

> **Source of truth**: The machine-enforced exemption list is in
> `tests/cli/runtime_mirror_exemptions.json`. Do not duplicate it in this skill.
> `TestShRuntimeMirrorCoverage` enforces the JSON entries; `TestVirtualNativeCommandPathAlignment`
> enforces command-path alignment except for justified `command_path_exempt` entries.

The scanner considers only files whose names start with `virtual_` or `native_`
(`tests/cli/runtime_mirror_test.go`). Everything else (`container_*`,
`config_*`, `module_*`, `init_*`, `tui_*`, `completion.txtar`, `dogfooding_*`,
`audit_*`, `agent_*`, `validate.txtar`, and so on) is outside its scope rather
than exempt. `lua_virtual.txtar` ends in
`_virtual` but is not scanned. A `native_<x>.txtar` without `virtual_<x>.txtar`
fails as an orphan, so name native-only tests without the `native_` prefix.

If the test is exempt, report it and stop.

### Step 2: Read the Virtual Test

Read the source `virtual_*.txtar` file and extract:
1. The test description comments
2. All `exec` commands and `stdout`/`stderr` assertions
3. The embedded `invowkfile.cue` and any other files
4. Environment variable usage

### Step 3: Transform the CUE

Replace each virtual implementation with a platform-split native pair: one
`linux`/`macos` implementation and one `windows` implementation, each declaring
only `runtimes: [{name: "native"}]` (a virtual fallback defeats the test). See
the worked example below.

### Step 4: Translate Shell Syntax

Apply these translations for the Windows (PowerShell) implementation. Also
follow rule "Native tests": the Unix script must be valid in both bash and zsh,
PowerShell must stay 5.1-compatible (no ternary, `??`, or `&&`/`||` chains), and
anchored assertions need `\r?$`.

| Bash/Zsh | PowerShell |
|----------|------------|
| `echo "text"` | `Write-Output "text"` |
| `echo -n "text"` | `Write-Host -NoNewline "text"` (**unverified**, see note) |
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

Note on `echo -n`: `Write-Host` bypasses the PowerShell pipeline
(`.agents/skills/windows-testing/references/powershell-testing.md`), and no
current mirror uses it. Treat the row as an open question: confirm it with
`go test -run TestCLI/native_<name> ./tests/cli/` on Windows before relying on it.

### Step 5: Preserve Assertions

- `exec` command paths must match the virtual test unless `command_path_exempt`
  in `tests/cli/runtime_mirror_exemptions.json` records the difference with a
  justification; `TestVirtualNativeCommandPathAlignment` checks this.
- stdout/stderr assertions should match by default. Runtime-identity output
  (runtime name, dry-run rendering, runtime-specific error wording, as in
  `native_dryrun.txtar` and `native_timeout.txtar`) may differ. The JSON has no
  field for that and nothing checks it, so justify each difference with a
  comment in the native file.
- Within the native test, assertions are identical across platforms (rule
  "Native tests"); only the script syntax differs.

### Step 6: Write the Mirror

Create `native_<feature>.txtar` with:
- Updated description noting it's a native mirror
- Same `exec` commands and assertions, except the Step 5 runtime-identity
  differences
- Platform-split CUE implementations
- Same embedded support files

### Step 7: Verify

```bash
go test -v -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/...
make test-cli
```

Run the focused guardrails first so a pairing or command-path failure is clear
before the full CLI suite. Then run the repository completion gates required by
`.agents/rules/checklist.md` when files changed.

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

Map the test output to fixes (with `tests/cli/runtime_mirror_exemptions.json`):

| Failure | Meaning |
|---|---|
| `missing native runtime mirror` | Non-exempt `virtual_<x>` has no `native_<x>` |
| `stale exact mirror exemption` | `exact` entry names a virtual file that no longer exists |
| `superseded exact mirror exemption` | `exact` entry whose native mirror now exists; remove the entry |
| `stale prefix mirror exemption` | `prefix` entry matches no virtual file |
| `orphan native mirror` | `native_<x>` without `virtual_<x>` |
| `command-path mismatch` | Pair invokes different command paths and has no `command_path_exempt` entry |
| `stale command-path exemption` | `command_path_exempt` entry whose native mirror does not exist |
