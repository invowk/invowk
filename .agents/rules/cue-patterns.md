---
paths:
  - "**/*.cue"
  - "pkg/invowkfile/**"
  - "pkg/invowkmod/**"
  - "internal/config/**"
  - "pkg/cueutil/**"
---

# CUE Patterns

## String Validation

### Use Regex Constraints for Negative Checks

CUE's `strings.HasPrefix()`, `strings.Contains()` and `strings.HasSuffix()` are validators: they work positively (`string & strings.HasPrefix("/")`), but they cannot be negated. `!strings.HasPrefix("/")` fails with `invalid operation !strings.HasPrefix("/") (! string)`. Express negative constraints with regex:

```cue
field?: string & =~"^[^/]"      // Does not start with /
field?: string & !~"\\s"       // Does not contain whitespace
field?: string & !~"\\.\\."    // Does not contain ..
```

### Common Regex Patterns

| Pattern | Meaning | Example Use |
|---------|---------|-------------|
| `=~"^[^/]"` | Does not start with `/` | Relative paths only |
| `!~"\\s"` | Does not contain whitespace | Compact identifiers |
| `=~"^[a-zA-Z]"` | Starts with letter | Identifiers |
| `=~"^\\s*\\S.*$"` | Non-empty after trim | Required text fields |
| `=~"^[A-Za-z_][A-Za-z0-9_]*$"` | POSIX identifier | Env var names |

`strings.MaxRunes(n)` and `strings.MinRunes(n)` are the usual length validators.

## Schema Organization

- Model runtime variants as closed structs in a discriminated union on `name`; see `#RuntimeConfig` in `pkg/invowkfile/invowkfile_schema.cue` (`#RuntimeConfigNative | #RuntimeConfigVirtualSh | #RuntimeConfigVirtualLua | #RuntimeConfigContainer`).
- **CUE v0.18+ `explicitopen` embedding rule:** Embedding a shared definition (`#Base`) in CUE v0.18+ closes the enclosing struct to `#Base`'s own fields unless the postfix spread operator (`#Base...`) is used. When composing variant or closed definitions from shared open base blocks (`#RuntimeConfigBase`, `#LLMCommonConfig`, `#ScriptSourceCommon`), embed them with `#Base...` and wrap the concrete variants in `close({...})`.
- Require at least one list element with `& [_, ...]`, e.g. `implementations: [...#Implementation] & [_, ...]`.
- Mutual exclusion inside disjunction arms uses an optional bottom field: the image arm of the container runtime declares `containerfile?: _|_`, and the containerfile arm declares `image?: _|_`.
- Field removal: for clean breaking changes, remove the obsolete field and let closed structs reject it. Add an `old_field?: _|_` tombstone only when a change explicitly calls for compatibility diagnostics.

## Validation Responsibilities

CUE owns static shape: format patterns, length limits and enum unions. Go owns what CUE cannot express: path traversal and cross-platform path security, file existence and cross-field logic (full table: `.agents/skills/cue/SKILL.md`). Mark every Go-only validation with a `[GO-ONLY]` comment in the schema, as the `containerfile` field does (`#NonWhitespaceString & strings.MaxRunes(4096)` in CUE; absolute paths and `..` segments rejected in Go).

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Negating `strings.HasPrefix()` / `strings.Contains()` | "invalid operation (! string)" | Use `=~"^[^x]"` / `!~"pattern"` regex |
| Forgetting to escape `.` in regex | Matches any char instead of literal | Use `\\.` |
| Missing `close()` on struct | Extra fields silently allowed | Always use `close({...})` |
| Bare `#Base` embedding inside `close({...})` or extended structs | `field not allowed` or `empty disjunction` in CUE v0.18+ | Embed shared base definitions with `#Base...` (`explicitopen`) |
