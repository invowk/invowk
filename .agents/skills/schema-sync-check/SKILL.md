---
name: schema-sync-check
description: Validate CUE schema sync after editing schemas, Go structs with JSON tags, value-type Validate() methods, enum/disjunction constraints, config defaults, or schema-owned decode behavior. Runs targeted structural and behavioral sync tests (sync*_test.go, schematest) and reports mismatches such as a missing JSON tag or behavioral drift.
---

# Schema Sync Check

Validate structural CUE/Go field alignment and the behavioral contracts that
must agree across schemas, Go validation, defaults, and decoding. This skill is
the canonical home for the sync-test inventory and procedure; the `cue` skill
points here.

## What It Checks

### Schema Files → Go Structs

| Schema | Go Package | Sync Test |
|--------|-----------|-----------|
| `pkg/invowkfile/invowkfile_schema.cue` | `pkg/invowkfile/` | `pkg/invowkfile/sync_test.go`, `pkg/invowkfile/sync_runtime_test.go`, `pkg/invowkfile/sync_behavioral_test.go`, `pkg/invowkfile/sync_runtime_behavioral_test.go` |
| `pkg/invowkmod/invowkmod_schema.cue` | `pkg/invowkmod/` | `pkg/invowkmod/sync_test.go` |
| `internal/config/config_schema.cue` | `internal/config/types.go` | `internal/config/sync_test.go` |

### Sync Test Pattern

The structural sync tests (helpers in `internal/testutil/schematest`) verify:
1. Every CUE field name has a matching JSON tag in the Go struct
2. Every Go struct JSON tag has a matching CUE field name
3. CUE optional fields vs Go `omitempty`: a mismatch is only logged with `t.Logf` as a `Note:` and never fails the test, so check `-v` output

Skipped by design: Go fields tagged `json:"-"` or untagged, and CUE `_|_` tombstones.

Behavioral sync tests then cover parser/schema behavior such as runtime defaults,
validation constraints, and config decode compatibility: each `behavioralSyncCase`
feeds the same input to Go `Validate()` and the CUE constraint. A non-empty
`divergeNote` marks an intended disagreement, which is only logged. The structural
helper does not prove Go/CUE type compatibility by itself.

## Workflow

### Step 1: Select the Affected Owner

Map each changed schema, Go type, `Validate()` contract, or default to
`pkg/invowkfile`, `pkg/invowkmod`, or `internal/config`. Run the affected
package first; run all three when a shared helper or cross-schema behavior
changed.

### Step 2: Run Structural and Behavioral Tests

```bash
go test -v -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/
```

Then run the targeted packages to cover behavioral/default tests whose names do
not include `Sync`:

```bash
go test -v ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/
```

### Step 3: Interpret Results

**All tests pass**: Report which packages and structural/behavioral surfaces
were exercised. Do not claim type-level semantic equivalence from structural
JSON-tag tests alone. Mention any `Note:` (omitempty) or `Expected divergence:`
log lines that the change touched.

**Test failures**: Structural failures name the definition and field, e.g. under
the `TestSchemaSync/#Command` subtest in `pkg/invowkfile`:

```
[#Command] CUE field "new_field" not found in Go struct (missing JSON tag)
[#Command] Go JSON tag "old_field" not found in CUE schema (missing CUE field)
```

Behavioral failures:

```
BEHAVIORAL DRIFT: Go accept=true, CUE accept=false for input "..."
Go Validate() unexpected: got accept=..., want ... (err=...)
CUE Validate() unexpected: got accept=..., want ... (err=...)
```

`BEHAVIORAL DRIFT` means Go and CUE disagree on a case without a `divergeNote`;
the `unexpected` lines mean one side no longer matches the case's pinned
`goExpect`/`cueExpect`.

### Step 4: Fix Mismatches

For each mismatch, determine the correct action:

| Mismatch Type | Fix |
|---------------|-----|
| CUE field missing Go tag | Add `json:"field_name"` tag to Go struct |
| Go tag missing CUE field | Add field to CUE schema definition |
| Field renamed in CUE | Update Go JSON tag to match |
| Field renamed in Go | Update CUE field name to match |
| Field removed from CUE | Remove the Go struct field; if it is a Go-only field, tag it `json:"-"` |
| Field removed from Go | Remove the CUE field (or verify it is intentionally CUE-only; a removed field kept for diagnostics is a `_\|_` tombstone) |
| Behavioral drift | Align the Go `Validate()` and the CUE constraint, or, if the divergence is intended, set a `divergeNote` and add a `[GO-ONLY]` comment |
| Stale `divergeNote` | When Go and CUE agree again, update `goExpect`/`cueExpect` and remove the note |

### Step 5: Re-verify

Re-run the Step 2 commands, then run `make test` (sync changes may affect parsing behavior).

If the task is an audit or diagnosis rather than an authorized fix, stop after
reporting mismatches and the owning files.

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
3. For invowkfile types, add the type to the table-driven `TestSchemaSync` case
   list in `pkg/invowkfile/sync_test.go` first; otherwise add a targeted test
   function with the shared helper:
   ```go
   func TestNewTypeSchemaSync(t *testing.T) {
       schema, _ := getCUESchema(t)
       def := schematest.LookupDefinition(t, schema, "#NewType")
       cueFields := schematest.ExtractCUEFields(t, def)
       goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[NewType]())
       schematest.AssertFieldsSync(t, "NewType", cueFields, goFields)
   }
   ```

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| `omitempty` mismatch | CUE optional (`?`) vs Go `omitempty` | Ensure optional CUE fields use `omitempty` in Go. Reported only as a `t.Logf` `Note:`, so check `-v` output; it never fails the test |
| Helper limitation | `schematest.ExtractGoJSONTags` is direct-field oriented and treats `omitempty` as optional; tags such as `omitzero` need review | Inspect the helper output and add explicit behavioral coverage when needed |
| Nested struct sync | Inner struct not tested | Add separate sync test for nested type |
| Mapstructure tag | Viper config fields need both tags | Add `mapstructure:"field_name"` alongside `json` |
| Stale `divergeNote` | A behavioral case still marks a divergence that the change removed, or hides a disagreement that should now fail | Re-check every non-empty `divergeNote` the change touched; remove it once Go and CUE agree |
