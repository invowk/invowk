# Error Handling

## Defer Close Pattern with Named Returns

When functions open resources that need closing (files, connections, readers, writers), use **named return values** to aggregate close errors with the primary operation's error. This ensures close errors are never silently ignored.

### Required Pattern

```go
// CORRECT: Use named return to capture close error
func processFile(path string) (err error) {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := f.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    // ... work with f ...
    return nil
}

// CORRECT: Multiple resources
func copyFile(src, dst string) (err error) {
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := srcFile.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    dstFile, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer func() {
        if closeErr := dstFile.Close(); closeErr != nil && err == nil {
            err = closeErr
        }
    }()

    _, err = io.Copy(dstFile, srcFile)
    return err
}
```

### Anti-Patterns to Avoid

```go
// WRONG: Error silently ignored
defer f.Close()

// WRONG: Error explicitly discarded without aggregation
defer func() { _ = f.Close() }()

// WRONG: Only logging (acceptable in rare cases with justification)
defer func() {
    if err := f.Close(); err != nil {
        log.Printf("close error: %v", err)
    }
}()
```

### When This Pattern Applies

Use this pattern for any `io.Closer` or similar resource:
- `*os.File`
- `*zip.ReadCloser`, `*zip.Writer`
- `net.Conn`
- `*http.Response.Body`
- `*sql.Rows`
- Custom types implementing `Close() error`

### Exceptions

1. **Test code**: Use test helpers (e.g., `testutil.MustClose(t, f)`) instead of named returns.
2. **Terminal operations in SSH sessions**: Where `sess.Exit()` errors cannot be meaningfully handled, use `_ =` with a comment explaining why.
3. **Best-effort cleanup after primary error**: When the function already has an error, logging the close error may be appropriate rather than overwriting the primary error.

### Rationale

- Close operations can fail (disk full, network issues, flush failures)
- Silent failures lead to data corruption or resource leaks
- Named returns make the error handling explicit and testable
- This pattern is idiomatic Go and recommended by the Go team
