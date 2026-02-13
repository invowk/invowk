---
name: schema-sync-check
description: Validate CUE schema sync after editing schemas or Go structs with JSON tags. Runs targeted sync tests and reports mismatches.
user-invocable: true
disable-model-invocation: false
---

# Schema Sync Check

Validate that CUE schema definitions and Go struct JSON tags are in sync.

## When to Use

Invoke this skill (`/schema-sync-check`) after:
- Editing any CUE schema file (`*_schema.cue`)
- Adding or modifying JSON tags on Go structs in `pkg/invowkfile/`, `pkg/invowkmod/`, or `internal/config/`
- Renaming CUE fields or Go struct fields
- Adding new CUE definitions with corresponding Go types

## What It Checks

### Schema Files → Go Structs

| Schema | Go Package | Sync Test |
|--------|-----------|-----------|
| `pkg/invowkfile/invowkfile_schema.cue` | `pkg/invowkfile/` | `pkg/invowkfile/sync_test.go` |
| `pkg/invowkmod/invowkmod_schema.cue` | `pkg/invowkmod/` | `pkg/invowkmod/sync_test.go` |
| `internal/config/config_schema.cue` | `internal/config/types.go` | `internal/config/sync_test.go` |

### Sync Test Pattern

Each sync test verifies:
1. Every CUE field name has a matching JSON tag in the Go struct
2. Every Go struct JSON tag has a matching CUE field name
3. Field types are compatible (string↔string, int↔int, etc.)

## Workflow

### Step 1: Run Sync Tests

```bash
go test -v -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/
```

### Step 2: Interpret Results

**All tests pass**: Schema and Go structs are in sync. No action needed.

**Test failures**: The output will show which fields are mismatched:

```
--- FAIL: TestCommandSchemaSync
    sync_test.go:142: CUE field "new_field" has no matching Go JSON tag
    sync_test.go:148: Go JSON tag "old_field" has no matching CUE field
```

### Step 3: Fix Mismatches

For each mismatch, determine the correct action:

| Mismatch Type | Fix |
|---------------|-----|
| CUE field missing Go tag | Add `json:"field_name"` tag to Go struct |
| Go tag missing CUE field | Add field to CUE schema definition |
| Field renamed in CUE | Update Go JSON tag to match |
| Field renamed in Go | Update CUE field name to match |
| Field removed from CUE | Remove Go struct field (or add exclusion if field is Go-only) |
| Field removed from Go | Remove CUE field (or verify it's intentionally CUE-only) |

### Step 4: Re-verify

After fixes, run the sync tests again to confirm:

```bash
go test -v -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/
```

### Step 5: Run Full Test Suite

Sync changes may affect parsing behavior:

```bash
make test
```

## Quick Reference

### Adding a New Field

1. Add to CUE schema with constraints:
   ```cue
   new_field: string & =~"^[a-z]+$" & strings.MaxRunes(128)
   ```

2. Add to Go struct with JSON tag:
   ```go
   NewField string `json:"new_field"`
   ```

3. Run sync check to verify alignment

### Adding a New Type

1. Define CUE definition: `#NewType: close({ ... })`
2. Create Go struct with JSON tags
3. Add sync test function:
   ```go
   func TestNewTypeSchemaSync(t *testing.T) {
       schema, _ := getCUESchema(t)
       cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#NewType"))
       goFields := extractGoJSONTags(t, reflect.TypeFor[NewType]())
       assertFieldsSync(t, "NewType", cueFields, goFields)
   }
   ```

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| `omitempty` mismatch | CUE optional (`?`) vs Go `omitempty` | Ensure optional CUE fields use `omitempty` in Go |
| Nested struct sync | Inner struct not tested | Add separate sync test for nested type |
| Mapstructure tag | Viper config fields need both tags | Add `mapstructure:"field_name"` alongside `json` |
| Stale exclusions | Removed fields still in exclusion list | Clean up sync test exclusions after refactoring |
