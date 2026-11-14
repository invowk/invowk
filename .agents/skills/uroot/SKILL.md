---
name: uroot
description: u-root utility implementation patterns for internal/uroot/, virtual runtime built-in registry changes (BuildDefaultRegistry, Registry.Run), virtual filesystem path policy, streaming I/O, the [uroot] error prefix, combined short flags, symlinks, and tests/cli/testdata/virtual_uroot*.txtar coverage. Use when implementing or reviewing built-in utilities for virtual-sh or virtual-lua.
---

# u-root Utils Integration

This skill covers the shared u-root utility commands in Invowk's virtual runtime
family: `internal/uroot/`, new built-in utilities used by virtual-sh or
virtual-lua, and changes to u-root command behavior.

## Terminology

- **Project name**: "u-root" (lowercase with hyphen) — use in prose, documentation, and comments (matches upstream `github.com/u-root/u-root`).
- **Go identifiers**: "uroot" (lowercase, no hyphen) — use for package names, types, and variables (Go identifiers cannot contain hyphens).
- **Config option**: `virtual.utilities.enabled` — shared virtual runtime utility toggle.

Identifiers: package `internal/uroot/`; types `Command`, `HandlerContext`, `Registry`; functions `BuildDefaultRegistry()`, `tryUrootBuiltin()` for virtual-sh, `runVirtualUtility()` for virtual-lua; config `virtual.utilities.enabled: true`.

---

## Streaming I/O Requirement

**CRITICAL: New and changed u-root utility implementations should use streaming I/O for file operations and must not worsen unbounded heap growth.**

Do not buffer entire file contents into memory unless the command's semantics
require a full input set. Memory use must stay predictable and independent of
input size, with no OOM on large files.

### Required Pattern

Illustrative (the real `cp` is an upstream wrapper):

```go
// CORRECT: Resolve virtual paths before opening and stream copy with close handling.
func copyFile(hc *uroot.HandlerContext, src, dst string) (err error) {
    srcPath, err := hc.ResolvePath(src)
    if err != nil {
        return err
    }
    dstPath, err := hc.ResolvePath(dst)
    if err != nil {
        return err
    }

    srcFile, err := os.Open(srcPath)
    if err != nil {
        return err
    }
    defer func() { _ = srcFile.Close() }() // Read-only source; close error non-critical

    dstFile, err := os.Create(dstPath)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := dstFile.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    _, err = io.Copy(dstFile, srcFile)  // Streams in chunks
    return err
}
```

### Anti-Patterns to Avoid

```go
// WRONG: Loads entire file into memory
data, err := os.ReadFile(src)
if err != nil {
    return err
}
err = os.WriteFile(dst, data, 0644)

// WRONG: Buffering entire content
content, _ := io.ReadAll(reader)
writer.Write(content)
```

### Applies To

The full inventory (12 upstream wrappers, 16 custom commands) is in `internal/uroot/doc.go`.

- **Custom commands, enforce the rule here**: `cut`, `grep` (line-by-line), `head`, `tail`, `sort` (see exception below), `tee` (streaming to stdout + files), `tr`, `uniq`, `wc` (streaming counters), and generated output such as `seq`.
- **Upstream wrappers** (`base64`, `cat`, `cp`, `find`, `gzip`, `ls`, `mkdir`, `mv`, `rm`, `shasum`, `tar`, `touch`): u-root owns their I/O. Invowk-side handling around a wrapper (such as the path-argument validation in `Registry.Run()`), or replacing a wrapper with a custom command, must follow the streaming rule.

### Output Generation (Not Just File I/O)

Streaming applies to **generated output** too, not only file content. Commands like `seq` that produce potentially unbounded output must stream directly to stdout rather than accumulating results in memory:

```go
// CORRECT: Stream output directly
count := 0
for n := first; n <= last; n += increment {
    select {
    case <-ctx.Done():
        return wrapError(c.name, ctx.Err())
    default:
    }
    if count > 0 { fmt.Fprint(hc.Stdout, *separator) }
    fmt.Fprint(hc.Stdout, formatted)
    count++
}

// WRONG: Accumulate then join (OOMs on large ranges)
var parts []string
for n := first; n <= last; n += increment {
    parts = append(parts, formatted)  // Unbounded growth
}
fmt.Fprint(hc.Stdout, strings.Join(parts, sep))  // Doubles memory
```

**Context cancellation**: Generation loops must check `ctx.Done()` periodically. Without this, Ctrl+C has no effect until the loop completes naturally, which may be never for large ranges.

### Exception: Sorting Large Files

`sort` currently collects lines in memory to implement sorting. Treat that as
semantics/debt, not as a reusable implementation pattern. If changing `sort`,
consider temp-file support for large inputs. The key constraint remains: do not
add avoidable unbounded heap growth.

## Path Policy

- The runtime injects path validation for both virtual-sh and virtual-lua.
- `Registry.Run()` pre-validates path arguments for upstream wrappers before dispatch.
- Custom commands that touch the filesystem should resolve user paths with
  `HandlerContext.ResolvePath` before `os.Open`, `os.Create`, `os.Link`, or similar calls.
- The security boundary is normalized allowed-root validation, not symlink
  following by itself: following a symlink can expose its target path, and
  allowed-root validation decides whether that target may be accessed.

---

## Symlink Handling

**Default behavior for `cp`: Follow symlinks (copy target content, not the link).** This matches standard POSIX `cp`.

- `cp source dest` where `source` is a symlink → copies the target file content
- `cp -r dir/ dest/` where `dir/` contains symlinks → copies target contents, not links
- Symlink preservation requires the explicit `-P` flag

### Symlink Creation (`ln -s`)

**Preserve relative target strings for symbolic links.** `os.Symlink(target, linkName)` stores `target` as a literal string. For `-s`, pass the user-provided target as-is:

- `ln -s ../lib/foo link` → symlink contains `"../lib/foo"` (relative, portable)
- `ln -s /usr/lib/foo link` → symlink contains `"/usr/lib/foo"` (absolute)

Only resolve the **link name** to absolute (the OS needs the creation path). For **hard links** (no `-s`), resolve the target too because `os.Link` needs the real filesystem path.

---

## POSIX Combined Short Flags

Combined short flags (`ln -sf`) are split centrally in `Registry.Run()` for custom commands; upstream wrappers mark themselves with `NativePreprocessor` via `baseWrapper` and split internally. Read [references/flag-preprocessing.md](references/flag-preprocessing.md) before changing `Registry.Run()` preprocessing, `NativePreprocessor`/`baseWrapper`, or how the runtimes dispatch to the registry; it also lists the key files.

- Register new custom commands in `BuildDefaultRegistry()` and do NOT embed `baseWrapper` in them; they then get combined-flag support automatically.
- New upstream wrappers embed `baseWrapper` and skip central preprocessing.
- Never call `unixflag.ArgsToGoArgs()` in an individual command, and never remove `baseWrapper` from an upstream wrapper (see Common Pitfalls).

### Edge Cases

- **Value-taking flags in combined groups**: `ArgsToGoArgs` has no flag-value awareness. `-df:` splits to `-d -f -:`. Users must write `-d : -f` for value flags. This matches upstream behavior.
- **`--` end-of-flags is not preserved**: `ArgsToGoArgs` converts `--` to `-`.
  That is not a valid POSIX end-of-options delimiter and must not be described
  as benign. If a custom command needs operands beginning with `-`, preserve
  and handle `--` before calling `ArgsToGoArgs`, and add an end-to-end test.

---

## Cross-Platform Path Handling

**CRITICAL: Virtual runtime utilities must produce POSIX-consistent output on all platforms.**

Since u-root utilities are exposed through the virtual runtime family
(`virtual-sh` and `virtual-lua`) as POSIX-like command utilities, users expect
forward-slash paths. Implementations must output forward slashes regardless of
the host OS.

### Text-Manipulation Utilities (dirname, basename)

Use `path` (NOT `path/filepath`) for pure text operations that don't touch the filesystem:

```go
import "path"

// CORRECT: path.Dir always uses forward slashes
fmt.Fprintln(hc.Stdout, path.Dir(p))    // dirname
fmt.Fprintln(hc.Stdout, path.Base(p))   // basename

// WRONG: filepath.Dir returns backslashes on Windows
fmt.Fprintln(hc.Stdout, filepath.Dir(p))  // "foo\bar" on Windows!
```

**Why `path` is correct here:** `dirname` and `basename` are defined by POSIX as text manipulation — they parse path strings without filesystem I/O. The `path` package implements exactly this: slash-separated path processing.

### Filesystem-Resolving Utilities (realpath)

Use `path/filepath` for OS interaction, then convert output to slashes:

```go
import "path/filepath"

// Use filepath for actual filesystem resolution
resolved, err := filepath.EvalSymlinks(path)
resolved, err = filepath.Abs(resolved)

// Convert to forward slashes before outputting
fmt.Fprintln(hc.Stdout, filepath.ToSlash(resolved))
```

### Test Normalization

When testing filesystem-resolving utilities, resolve expected paths first to handle OS-level indirection:

```go
// macOS: /var → /private/var
// Windows: RUNNER~1 → runneradmin (8.3 short names)
tmpDir, err := filepath.EvalSymlinks(t.TempDir())

// Match the implementation's ToSlash output
want := filepath.ToSlash(filepath.Join(tmpDir, "file.txt"))
```

---

## Unsupported Flag Handling

Do not ignore the error returned by `FlagSet.Parse` and then continue as if only
the unknown flag had been skipped. Go's `flag` parser stops at the first unknown
flag, so later supported options and operands may remain unparsed or disappear
from the command's expected positional layout.

Choose and test one explicit contract per custom command:

- Return a prefixed usage/unsupported-flag error immediately (the default and
  safest behavior).
- Pre-filter a documented compatibility flag before `FlagSet.Parse`, preserving
  all following arguments exactly, when compatibility requires accepting it.
- Use a command-specific parser when POSIX `--`, interspersed options, or
  value-taking combined flags are required.

Upstream wrappers keep their upstream flag behavior. Do not claim a uniform
silent-ignore guarantee across the registry.

---

## Error Reporting Format

**All errors from u-root utilities MUST be prefixed with `[uroot]`**, so users can tell u-root implementation errors from host utility errors in mixed script output.

### Required Format

```
[uroot] <command>: <error message>
```

### Examples

```
[uroot] cp: cannot stat 'missing': No such file or directory
[uroot] mv: cannot move 'src' to 'dst': Permission denied
[uroot] cat: /path/to/file: Is a directory
[uroot] mkdir: cannot create directory 'existing': File exists
```

`Registry.Run()` itself returns `[uroot] <name>: command not found` for unknown commands.

### Implementation Pattern

Wrap errors with `wrapError(name, err)` from `internal/uroot/wrapper.go`, which produces `[uroot] <cmd>: <err>` (and returns nil for a nil error); do not hand-write the prefix.

```go
// CORRECT: wrapError adds the [uroot] <name>: prefix
if err != nil {
    return wrapError(c.name, err)
}

// WRONG: Raw error without prefix
if err != nil {
    return fmt.Errorf("cp: %w", err)  // Missing [uroot] prefix!
}
```

---

## Common Pitfalls

- **Buffering file contents** - Always use `io.Copy()` or similar streaming patterns. Never use `os.ReadFile()` or `io.ReadAll()` for arbitrary user files.
- **Missing error prefix** - All u-root errors must include the `[uroot]` prefix for source identification.
- **Naked defer Close()** - Never use `defer f.Close()`. For read-only files, use `defer func() { _ = f.Close() }()` with comment. For write operations, use named returns to capture close errors.
- **Combined flags silent failure** - Go's `flag.NewFlagSet` does NOT support POSIX-style combined short flags (`-sf`). With `flag.ContinueOnError` + `io.Discard`, combined flags silently fail (both flags stay `false`). This is handled centrally by `Registry.Run()` — do NOT add `unixflag.ArgsToGoArgs()` calls to individual commands.
- **Discarded parse error** - Ignoring `FlagSet.Parse` errors does not skip just
  the unsupported flag; parsing stops. Fail fast or explicitly pre-filter the
  compatibility flag while preserving later operands.
- **Lost `--` delimiter** - `unixflag.ArgsToGoArgs()` rewrites `--` to `-`.
  Commands that promise POSIX end-of-options semantics must handle the delimiter
  before central preprocessing and test dash-prefixed operands.
- **Double-splitting upstream wrappers** - Never remove the `baseWrapper` embedding from upstream wrappers. It provides the `NativePreprocessor` marker that prevents `Registry.Run()` from double-splitting already-preprocessed args, which would corrupt long flags (`--recursive` → `-r -e -c -u ...`).

## Verification Workflow

1. Classify the change as a custom command or upstream wrapper.
2. Register new commands in `BuildDefaultRegistry()` and update `internal/uroot/doc.go`.
3. Add or update unit tests in `internal/uroot/`.
4. Add or update `tests/cli/testdata/virtual_uroot*.txtar` when CLI-visible
   behavior changes.
5. Run focused gates (txtar files run as `TestCLI/<file>` subtests; `-run Uroot` matches nothing):

```bash
go test -v ./internal/uroot ./internal/runtime
go test -v ./tests/cli -run 'TestCLI/virtual_uroot'
```
