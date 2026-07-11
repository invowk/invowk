# Typesystem Maintenance Workflow

Use this workflow whenever value types are added, removed, renamed, or semantically changed.

## 1. Detect Current Value Types

The extractor must inventory all Go source under `cmd/`, `internal/`, and
`pkg/` for each of its three sections: `Validate() error` methods,
primitive-wrapper declarations, and alias/re-export declarations. Do not add a
new production root to only one extraction pass.

```bash
tmp_catalog=$(mktemp)
.agents/skills/invowk-typesystem/scripts/extract_value_types.sh > "$tmp_catalog"
diff -u .agents/skills/invowk-typesystem/references/type-catalog.md "$tmp_catalog"
```

The diff must be empty before completion. If it is not, inspect the changes and
refresh the tracked catalog intentionally.

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
make check-types
make check-baseline
make test
make lint
make check-file-length
```

## 6. AGENTS Index Sync

If the skill folder is renamed/added/removed, update `AGENTS.md` Skills Index in the same patch.
