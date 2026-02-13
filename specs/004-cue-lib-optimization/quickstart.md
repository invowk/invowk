# Quickstart: CUE Library Usage Optimization

**Date**: 2026-01-30 | **Status**: Ready for Implementation

## Overview

This guide explains how to implement the CUE library usage optimization feature. After this work is complete, the codebase will have:

1. **Comprehensive CUE rules** documented in `.claude/rules/cue.md`
2. **Schema sync tests** that catch Go/CUE misalignment at CI time
3. **File size guards** preventing OOM on large files
4. **Consolidated validation** with clear CUE vs Go responsibilities

---

## Development Setup

No new dependencies required. This is a refactoring and documentation effort using existing packages:

```bash
# Verify build works
make build

# Run existing tests
make test

# Run linter
make lint
```

---

## Implementation Order

### Step 1: Expand `.claude/rules/cue.md`

The rules file should document:

1. **Schema compilation pattern** (the three-step flow)
2. **Validation responsibility matrix** (what goes in CUE vs Go)
3. **Decode usage rules** (always use `Decode()`, justify manual extraction)
4. **Field naming convention** (snake_case CUE → PascalCase Go)
5. **Error formatting** (path-prefixed messages)
6. **Version pinning** (upgrade review process)
7. **Common pitfalls** (unclosed structs, redundant validation)

### Step 2: Add File Size Guards

Add early size check to all parse functions:

**Files to modify**:
- `pkg/invowkfile/parse.go`
- `pkg/invowkmod/invowkmod.go`
- `internal/config/config.go`

**Pattern**:
```go
const DefaultMaxCUEFileSize = 5 * 1024 * 1024

func ParseBytes(data []byte, path string) (*Invowkfile, error) {
    if len(data) > DefaultMaxCUEFileSize {
        return nil, fmt.Errorf("%s: file size exceeds maximum (%d bytes)", path, DefaultMaxCUEFileSize)
    }
    // ... existing parsing ...
}
```

### Step 3: Add Schema Sync Tests

Create sync tests for each schema:

**Files to create**:
- `pkg/invowkfile/sync_test.go`
- `pkg/invowkmod/sync_test.go`
- `internal/config/sync_test.go`

**Test structure**:
```go
func TestFieldSync(t *testing.T) {
    cueFields := extractCUEFieldNames(t, schema, "#TypeName")
    goFields := extractGoJSONTags(t, reflect.TypeOf(StructType{}))

    // Every CUE field must have matching Go tag
    // Every Go tag must have matching CUE field
}
```

### Step 4: Audit and Remove Redundant Validation

Review `pkg/invowkfile/invowkfile_validation.go` for checks already in CUE:

| Check | CUE Location | Go Action |
|-------|--------------|-----------|
| Interpreter non-empty | `invowkfile_schema.cue` line ~150 | Remove from Go |
| Tool name format | `invowkfile_schema.cue` line ~95 | Keep in CUE |
| ReDoS safety | N/A | Keep in Go (CUE can't do this) |

### Step 5: Improve Error Formatting

Ensure all CUE errors include full paths:

```go
func formatCUEError(err error) error {
    cueErr, ok := errors.AsType[errors.Error](err)
    if !ok {
        return err
    }

    var lines []string
    for _, e := range errors.Errors(cueErr) {
        path := e.Path().String()
        msg := strings.TrimPrefix(e.Error(), path+":")
        lines = append(lines, fmt.Sprintf("%s: %s", path, strings.TrimSpace(msg)))
    }
    return fmt.Errorf("validation failed:\n  %s", strings.Join(lines, "\n  "))
}
```

---

## Testing

### Run Schema Sync Tests

```bash
# Run all tests including new sync tests
make test

# Run specific sync test
go test -v -run TestFieldSync ./pkg/invowkfile/...
```

### Verify Error Messages

```bash
# Create a test file with invalid data
echo 'cmds: [{name: ""}]' > /tmp/test.cue

# Run validation
go run . cmd --invowkfile=/tmp/test.cue

# Should see: cmds[0].name: value is too short
```

### Check File Size Limit

```bash
# Create oversized file
dd if=/dev/zero of=/tmp/large.cue bs=1M count=6

# Run validation
go run . cmd --invowkfile=/tmp/large.cue

# Should see: file size exceeds maximum
```

---

## Checklist

Before marking complete:

- [ ] `.claude/rules/cue.md` expanded with all patterns
- [ ] File size guards added to all parse functions
- [ ] Schema sync tests passing for all three schemas
- [ ] Redundant Go validation removed (with justification comments)
- [ ] Error messages include full JSON paths
- [ ] All tests pass: `make test`
- [ ] Linting passes: `make lint`
- [ ] Documentation sync map checked (no user-facing changes)

---

## Common Issues

### Q: Sync test fails with "field X not in Go struct"

**A**: The CUE schema has a field without a matching Go struct field. Either:
1. Add the missing field to the Go struct with correct JSON tag
2. Verify the CUE field is intentionally not decoded (add to exclusion list)

### Q: Sync test fails with "tag X not in CUE schema"

**A**: The Go struct has a JSON tag without a matching CUE field. Either:
1. The field is runtime-only (non-CUE) - add `json:"-"` tag
2. The CUE schema is missing the field - add it

### Q: CUE error path is empty

**A**: Ensure you're using the `errors` package from `cuelang.org/go/cue/errors`, not the standard library `errors`. CUE errors have path information only on CUE-specific error types.

---

## References

- [Spec](./spec.md) - Feature requirements
- [Research](./research.md) - Research findings
- [Data Model](./data-model.md) - Validation architecture
- [CUE Rules](.claude/rules/cue.md) - Existing rules (to be expanded)
