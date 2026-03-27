# PowerShell Testing Reference for Go Testscript Tests

Complete PowerShell reference for writing and debugging testscript (`.txtar`)
tests that exercise the native runtime on Windows. This reference supports
the parent skill (`SKILL.md`) and complements `.agents/rules/windows.md`
(authoritative for CI patterns and common pitfalls).

## PowerShell Versions

Two major versions are relevant for testing:

| Version | Binary | Ships With | Notes |
|---------|--------|-----------|-------|
| 5.1 | `powershell.exe` | Windows 10/11, Server 2016+ | Windows built-in; always available on CI |
| 7+ | `pwsh.exe` (`pwsh`) | Separate install | Cross-platform; may or may not be on CI runners |

**Invowk policy**: Scripts MUST be compatible with both PowerShell 5.1 and
7+. This means avoiding 7-only features.

### Features to avoid (PowerShell 7+ only)

- Ternary operator: `$x ? "yes" : "no"`
- Null-coalescing: `$x ?? "default"`
- Null-conditional: `$x?.Property`
- Pipeline chain operators: `cmd1 && cmd2`, `cmd1 || cmd2`
- `ForEach-Object -Parallel`
- `$PSStyle` automatic variable
- `Clean` block in functions

## Environment Variable Access

PowerShell accesses environment variables through the `$env:` drive, not
bare `$VAR` syntax.

```powershell
# CORRECT: Environment variable
$env:HOME
$env:PATH
$env:MY_CUSTOM_VAR

# WRONG: This creates/reads a PowerShell variable, NOT an environment variable
$HOME        # This actually works for HOME (built-in automatic variable) but
             # is unreliable for custom vars
$MY_CUSTOM_VAR  # Always empty unless explicitly set as a PS variable
```

### Setting environment variables

```powershell
# Set for current session (equivalent of export VAR=val)
$env:MY_VAR = "value"

# Unset
$env:MY_VAR = $null
# or
Remove-Item Env:\MY_VAR

# PATH modification (append)
$env:PATH += ";C:\new\path"
```

There is no `export` concept in PowerShell. All `$env:` assignments are
visible to child processes (they become part of the environment block passed
to `CreateProcess`).

## Error Handling

### $ErrorActionPreference

The PowerShell equivalent of `set -e`. Controls what happens when a cmdlet
(not a native command) produces a non-terminating error.

```powershell
# Stop on any error (like set -e for cmdlets)
$ErrorActionPreference = 'Stop'

# Other values:
# 'Continue'       - Default. Print error, continue execution.
# 'SilentlyContinue' - Suppress error, continue.
# 'Inquire'        - Ask user (not useful in automation).
# 'Break'          - Enter debugger (not useful in automation).
```

**Limitation**: `$ErrorActionPreference` does NOT affect native command
exit codes. A native command that returns exit code 1 will NOT stop
execution. Use `$LASTEXITCODE` for that.

### $LASTEXITCODE

Holds the exit code of the last native (non-PowerShell) command:

```powershell
invowk cmd hello
if ($LASTEXITCODE -ne 0) {
    Write-Error "invowk failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}
```

### try/catch/finally

```powershell
try {
    $result = Get-Content "nonexistent.txt" -ErrorAction Stop
}
catch {
    Write-Error "Failed: $_"
    exit 1
}
finally {
    # Always runs, even after catch
    Write-Output "cleanup complete"
}
```

## Output Pipeline

PowerShell outputs **objects**, not strings. This is the most fundamental
difference from Unix shells.

```powershell
# Get-ChildItem returns FileInfo/DirectoryInfo objects
Get-ChildItem | ForEach-Object { $_.Name }

# Write-Output writes to the success pipeline (stdout equivalent)
Write-Output "Hello"

# Write-Error writes to the error pipeline (stderr equivalent)
Write-Error "Something went wrong"

# Write-Host writes directly to console (bypasses pipeline)
# Avoid in scripts -- output cannot be captured or redirected
Write-Host "This goes to console only"
```

### Explicit string output

For testscript tests, always use `Write-Output` for stdout assertions:

```powershell
# PREFERRED: Explicit, reliable across PS versions
Write-Output "Hello from PowerShell"

# OK but less explicit
echo "Hello from PowerShell"  # 'echo' is an alias for Write-Output

# AVOID: Bypasses pipeline, not capturable
Write-Host "Hello from PowerShell"
```

### Out-String for formatted output

When a command produces object output and you need the string
representation:

```powershell
Get-Process | Out-String
```

## Comparison Operators

PowerShell uses named operators, not symbols:

| Operation | PowerShell | Bash Equivalent |
|-----------|-----------|-----------------|
| Equal | `-eq` | `=` or `==` |
| Not equal | `-ne` | `!=` |
| Less than | `-lt` | `-lt` |
| Greater than | `-gt` | `-gt` |
| Less or equal | `-le` | `-le` |
| Greater or equal | `-ge` | `-ge` |
| Wildcard match | `-like` | (no direct equivalent) |
| Regex match | `-match` | `=~` |
| Contains (collection) | `-contains` | (no direct equivalent) |
| In (collection) | `-in` | (no direct equivalent) |
| Not variants | `-notlike`, `-notmatch`, `-notcontains`, `-notin` | |

Case-insensitive by default. For case-sensitive variants, prefix with `c`:
`-ceq`, `-cne`, `-clike`, `-cmatch`, etc.

```powershell
# String comparison
if ("hello" -eq "HELLO") { ... }     # TRUE (case-insensitive)
if ("hello" -ceq "HELLO") { ... }    # FALSE (case-sensitive)

# Wildcard
if ("hello.txt" -like "*.txt") { ... }

# Regex
if ("error: file not found" -match "error:\s+(.+)") {
    $Matches[1]  # "file not found"
}
```

## String Operations

```powershell
# Replace (regex)
"hello world" -replace "world", "PowerShell"

# Replace (literal, method)
"hello world".Replace("world", "PowerShell")

# Split
"a,b,c" -split ","        # Returns array: @("a", "b", "c")
"a,b,c".Split(",")        # Same result, method syntax

# Trim
"  hello  ".Trim()         # "hello"
"  hello  ".TrimStart()    # "hello  "
"  hello  ".TrimEnd()      # "  hello"

# Join
@("a", "b", "c") -join ","   # "a,b,c"

# Substring
"hello".Substring(0, 3)    # "hel"

# String interpolation
$name = "invowk"
"Hello from $name"         # "Hello from invowk"
"Path: $($env:PATH)"      # Subexpression for complex expressions
```

## Bash to PowerShell Translation Table

| Bash | PowerShell | Notes |
|------|-----------|-------|
| `echo "text"` | `Write-Output "text"` | PS `echo` is alias but `Write-Output` is preferred |
| `$VAR` | `$env:VAR` | Environment variable; `$VAR` is a PS variable |
| `export VAR=val` | `$env:VAR = "val"` | No `export` concept in PS |
| `set -e` | `$ErrorActionPreference = 'Stop'` | Per-script; does not affect native commands |
| `if [ "$X" = "Y" ]` | `if ($env:X -eq "Y")` | Named operators, not symbols |
| `if [ -z "$X" ]` | `if ([string]::IsNullOrEmpty($env:X))` | Empty/null string test |
| `if [ -n "$X" ]` | `if (-not [string]::IsNullOrEmpty($env:X))` | Non-empty string test |
| `[ -f "$FILE" ]` | `Test-Path $FILE -PathType Leaf` | File existence |
| `[ -d "$DIR" ]` | `Test-Path $DIR -PathType Container` | Directory existence |
| `[ -e "$PATH" ]` | `Test-Path $PATH` | Any path existence |
| `cat file` | `Get-Content file` | `cat` is alias in PS but less portable |
| `grep pattern file` | `Select-String -Pattern "pattern" file` | Returns match objects |
| `wc -l file` | `(Get-Content file).Count` | Line count |
| `head -n 5 file` | `Get-Content file -First 5` | First N lines |
| `tail -n 5 file` | `Get-Content file -Last 5` | Last N lines |
| `mkdir -p dir` | `New-Item -ItemType Directory -Force dir` | `-Force` creates parents |
| `rm -rf dir` | `Remove-Item -Recurse -Force dir` | Recursive delete |
| `cp src dst` | `Copy-Item src dst` | |
| `mv src dst` | `Move-Item src dst` | |
| `pwd` | `Get-Location` | `pwd` is alias |
| `cd dir` | `Set-Location dir` | `cd` is alias |
| `exit 0` | `exit 0` | Same syntax |
| `true` / `false` | `$true` / `$false` | Boolean literals |
| `command -v prog` | `Get-Command prog -ErrorAction SilentlyContinue` | Check command exists |
| `prog > file` | `prog > file` | Same redirection syntax (mostly) |
| `prog 2>&1` | `prog 2>&1` | Same stderr redirect syntax |
| `prog \| other` | `prog \| other` | Pipeline syntax is the same |
| `$(command)` | `$(command)` | Subexpression (same syntax in both) |

## Key Gotchas

### CRLF from Write-Output

`Write-Output` (and most PowerShell output) produces `\r\n` line endings.
In testscript regex assertions:

```txtar
# WRONG: $ matches before \n, not before \r
stdout '^Hello$'

# CORRECT: Account for optional \r
stdout '^Hello\r?$'

# ALSO CORRECT: Substring match (no anchoring)
stdout 'Hello'
```

### $null vs empty string

PowerShell distinguishes between `$null` and empty string `""`:

```powershell
$null -eq ""         # FALSE
$null -eq $null      # TRUE
"" -eq ""            # TRUE
[string]::IsNullOrEmpty($null)   # TRUE
[string]::IsNullOrEmpty("")      # TRUE
```

In environment variables, an unset variable returns `$null` from `$env:VAR`,
while an explicitly empty variable returns `""`.

### Automatic variable unwrapping

PowerShell automatically unwraps single-element arrays:

```powershell
$arr = @("one")
$arr.GetType().Name  # Returns "String", not "Object[]"!

# Force array type
[array]$arr = @("one")
$arr.GetType().Name  # Returns "Object[]"
```

This can cause surprising behavior when functions return 0 or 1 results
vs multiple results.

### Pipeline vs expression mode

PowerShell has two parsing modes:
- **Expression mode**: Started by operators, variables, literals.
  `2 + 2` is parsed as arithmetic.
- **Command mode**: Started by command names.
  `Write-Output 2 + 2` outputs three objects: `2`, `+`, `2`.

```powershell
# Expression mode
$result = 2 + 2          # $result = 4

# Command mode (probably not what you want)
Write-Output 2 + 2       # Outputs: 2, +, 2 (three separate objects)

# Force expression in command mode
Write-Output (2 + 2)     # Outputs: 4
```

### Encoding

PowerShell 5.1 defaults to UTF-16LE for output redirection. PowerShell 7+
defaults to UTF-8 (no BOM). This can affect file content comparisons:

```powershell
# PowerShell 5.1: Creates UTF-16LE file
"hello" > file.txt

# Cross-version safe: Explicit encoding
"hello" | Out-File -FilePath file.txt -Encoding utf8
```

## Invowk-Specific Patterns

### Testscript exec with invowk

In testscript files, the native runtime is exercised through `exec invowk`:

```txtar
exec invowk cmd hello
stdout 'Hello from native shell'
```

On Windows, invowk internally selects PowerShell as the native shell. The
testscript runner itself runs in Go, not in PowerShell -- only the command
script is executed via PowerShell.

### Assertion patterns for PowerShell output

```txtar
# Substring match (safe, no CRLF issues)
stdout 'expected output'

# Anchored regex (needs \r? for Windows)
stdout '^exact line\r?$'

# Negation
! stdout 'unexpected output'

# stderr from PowerShell Write-Error
stderr 'error message'
```

### Platform-split CUE for native tests

Native tests need separate implementations for Unix and Windows:

```cue
implementations: [
    {
        script: """
            echo "Hello from bash"
            echo "HOME=$HOME"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    },
    {
        script: """
            Write-Output "Hello from bash"
            Write-Output "HOME=$($env:HOME)"
            """
        runtimes:  [{name: "native"}]
        platforms: [{name: "windows"}]
    },
]
```

The assertions must be platform-agnostic (identical output from both
implementations) or use platform conditionals:

```txtar
stdout 'Hello from bash'

# Platform-conditional assertion (if output differs)
[!windows] stdout 'unix-specific'
[windows] stdout 'windows-specific'
```

### Environment variable patterns

```cue
# Unix implementation
script: """
    echo "VAR=$MY_VAR"
    """

# Windows implementation
script: """
    Write-Output "VAR=$($env:MY_VAR)"
    """
```

Note the `$(...)` subexpression syntax in PowerShell for embedding `$env:`
lookups inside double-quoted strings. Without it, `"VAR=$env:MY_VAR"` works
for simple cases but can break with adjacent text:
`"$env:MY_VARsuffix"` tries to read `$env:MY_VARsuffix`.

### Error-path testing

```cue
# Windows implementation that should fail
script: """
    Write-Error "something went wrong"
    exit 1
    """
```

```txtar
! exec invowk cmd failing-command
stderr 'something went wrong'
```

Note: PowerShell's `Write-Error` produces structured error output that may
include additional formatting. Use substring matching for stderr assertions
rather than exact line matching.
