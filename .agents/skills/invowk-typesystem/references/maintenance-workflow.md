# Typesystem Maintenance Workflow

Use this workflow whenever value types are added, removed, renamed, or semantically changed.

## 1. Detect Current Value Types

```bash
.agents/skills/invowk-typesystem/scripts/extract_value_types.sh > /tmp/type-catalog.md
```

## 2. Refresh the Skill Catalog

```bash
.agents/skills/invowk-typesystem/scripts/extract_value_types.sh > .agents/skills/invowk-typesystem/references/type-catalog.md
```

## 3. Review Classification Drift

Confirm newly introduced entries are classified correctly:
- `primitive-wrapper`: scalar wrappers with domain semantics.
- `composite-validator`: struct/multi-field validators.
- `alias/re-export`: compatibility aliases or boundary simplifications.

## 4. Validate Error-Shape Consistency

For each new/changed type, check:
- Sentinel exists and uses `ErrInvalid<Type>` naming.
- Typed error wraps sentinel via `Unwrap()`.
- `Validate() error` is present and behavior is deterministic.

## 5. Verification Commands

Minimum for skill-doc changes:

```bash
make check-agent-docs
```

When type-system code changed:

```bash
make check-baseline
go test ./...
```

## 6. AGENTS Index Sync

If the skill folder is renamed/added/removed, update `AGENTS.md` Skills Index in the same patch.

