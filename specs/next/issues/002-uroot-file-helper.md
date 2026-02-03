# Issue: Extract File Processing Helper in `internal/uroot/`

**Category**: Quick Wins
**Priority**: Medium
**Effort**: Low (< 1 day)
**Labels**: `code-quality`, `refactoring`
**Depends On**: #001 (uroot error handling)

## Summary

All 7 utility commands in `internal/uroot/` repeat identical file opening/processing boilerplate (~30 lines each). Extract a shared helper to reduce duplication and ensure consistent error handling.

## Problem

Each utility command duplicates this pattern:

```go
files := fs.Args()
if len(files) == 0 {
    // Handle stdin
    return processReader(ctx.Stdin, ...)
} else {
    for _, file := range files {
        path := file
        if !filepath.IsAbs(path) {
            path = filepath.Join(hc.Dir, path)
        }
        f, err := os.Open(path)
        if err != nil {
            return wrapError(c.name, err)
        }
        // Process file
        f.Close()
    }
}
return nil
```

**Files with this pattern** (lines approximate):
- `internal/uroot/head.go` - lines 56-87
- `internal/uroot/tail.go` - lines 60-93
- `internal/uroot/grep.go` - lines 94-139
- `internal/uroot/cut.go` - lines 89-113
- `internal/uroot/wc.go` - lines 89-138
- `internal/uroot/sort.go` - lines 80-103
- `internal/uroot/uniq.go` - lines 65-81

## Solution

Create a new helper file `internal/uroot/files.go`:

```go
// SPDX-License-Identifier: MPL-2.0

package uroot

import (
    "io"
    "os"
    "path/filepath"
)

// FileProcessor processes a single reader and returns any error.
type FileProcessor func(r io.Reader, filename string) error

// ProcessFilesOrStdin processes files from args or stdin if no files given.
// The processor is called for each file (or stdin) with the reader and filename.
// For stdin, filename is "-".
func ProcessFilesOrStdin(
    args []string,
    stdin io.Reader,
    workDir string,
    cmdName string,
    processor FileProcessor,
) error {
    if len(args) == 0 {
        return processor(stdin, "-")
    }

    for _, file := range args {
        path := file
        if !filepath.IsAbs(path) {
            path = filepath.Join(workDir, path)
        }

        f, err := os.Open(path)
        if err != nil {
            return wrapError(cmdName, err)
        }

        processErr := processor(f, file)
        _ = f.Close() // Read-only file; close error non-critical

        if processErr != nil {
            return processErr
        }
    }

    return nil
}
```

### Refactored Usage Example

```go
// Before (head.go)
files := fs.Args()
if len(files) == 0 {
    return c.processHead(ctx.Stdin, ctx.Stdout, lines)
} else {
    for _, file := range files { ... }
}

// After (head.go)
return ProcessFilesOrStdin(fs.Args(), ctx.Stdin, hc.Dir, c.name,
    func(r io.Reader, _ string) error {
        return c.processHead(r, ctx.Stdout, lines)
    })
```

## Implementation Steps

1. [ ] Create `internal/uroot/files.go` with `ProcessFilesOrStdin` helper
2. [ ] Create `internal/uroot/files_test.go` with unit tests for the helper
3. [ ] Refactor `head.go` - Extract `processHead()` method, use helper
4. [ ] Refactor `tail.go` - Extract `processTail()` method, use helper
5. [ ] Refactor `grep.go` - Extract `processGrep()` method (note: needs filename)
6. [ ] Refactor `cut.go` - Extract `processCut()` method, use helper
7. [ ] Refactor `wc.go` - Extract `processWc()` method (note: aggregates counts)
8. [ ] Refactor `sort.go` - Extract `processSort()` method, use helper
9. [ ] Refactor `uniq.go` - Extract `processUniq()` method, use helper

## Special Cases

### `grep` Command
Grep outputs filename prefixes when processing multiple files. The `filename` parameter in `FileProcessor` addresses this:

```go
return ProcessFilesOrStdin(files, stdin, workDir, "grep",
    func(r io.Reader, filename string) error {
        return c.processGrep(r, stdout, pattern, filename, showFilename)
    })
```

### `wc` Command
Word count aggregates totals across files. May need a modified approach:

```go
var totalLines, totalWords, totalBytes int
err := ProcessFilesOrStdin(files, stdin, workDir, "wc",
    func(r io.Reader, filename string) error {
        lines, words, bytes, err := c.countFile(r)
        if err != nil {
            return err
        }
        totalLines += lines
        totalWords += words
        totalBytes += bytes
        return c.printCounts(stdout, lines, words, bytes, filename)
    })
if err != nil {
    return err
}
if len(files) > 1 {
    return c.printCounts(stdout, totalLines, totalWords, totalBytes, "total")
}
```

## Acceptance Criteria

- [ ] New `files.go` helper created with doc comments
- [ ] Unit tests for `ProcessFilesOrStdin` cover:
  - [ ] Empty args (stdin mode)
  - [ ] Single file
  - [ ] Multiple files
  - [ ] File not found error
  - [ ] Processor error handling
- [ ] All 7 commands refactored to use helper
- [ ] No functionality changes (verified by existing tests)
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] Lines of code reduced by ~150 lines total

## Testing

```bash
# Run uroot tests to verify no regression
go test -v ./internal/uroot/...

# Verify line count reduction
wc -l internal/uroot/*.go
```

## Notes

- Wait for #001 (error handling fix) to be merged first to avoid conflicts
- The `filename` parameter in `FileProcessor` enables grep-style output formatting
- Consider adding a `ProcessFilesOrStdinWithTotal` variant if more commands need aggregation
