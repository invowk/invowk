# Plan: Fix Reference Docs (Task #4)

## Findings After Source-of-Truth Verification

After reading all source files (`cmd/invowk/cmd.go`, `internal/config/types.go`,
`pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkfile/command.go`,
`pkg/invowkfile/implementation.go`, `pkg/invowkfile/validation_structure_flags.go`)
and all three doc files, I found that **most issues described in the task have already
been fixed** by prior commits (notably `8abd909`).

### Status of Each Finding

| Finding | Status | Details |
|---------|--------|---------|
| M3a: Add `-f` short flag for `--ivk-from` | Already fixed | cli.mdx line 44 already shows `-f` |
| M3b: Add `--ivk-force-rebuild` flag | Already fixed | cli.mdx line 45 already present |
| M3c: Fix reserved flags list (remove `list`/`l`) | No issue found | cli.mdx has no mention of `list`/`l` as reserved |
| M4: Fix discovery priority order | Already fixed | config-schema.mdx lines 69-73 show correct order |
| M5: Fix u-root count (15→28) | Already fixed | config-schema.mdx line 139 already shows 28 |
| L4: Add `env`, `workdir`, `depends_on` to Command/Implementation | Already fixed | invowkfile-schema.mdx lines 100-119 (Command) and 180-199 (Implementation) already document these |
| L5: Add `strict` field to AutoProvisionConfig | Already fixed | config-schema.mdx lines 225-231 already document `strict` |

### One Unforeseen Issue Found

**invowkfile-schema.mdx lines 128-134**: The reserved flags warning is incomplete.
It lists `env-file`, `env-var`, `env-inherit-*`, `workdir`, `help`, `runtime`, `from`,
`force-rebuild` but is missing `version`.

The `reservedFlagNames` map in `validation_structure_flags.go` (lines 25-40) includes
`version` as a reserved flag, but the docs don't mention it.

Note: `verbose` (v), `config` (c), and `interactive` (i) are also in the reserved map
as `ivk-verbose`, `ivk-config`, `ivk-interactive` — but these are already covered by
the prefix reservation (`ivk-*` prefix is reserved), so listing them explicitly is
optional. `version` has no prefix so it should be listed.

## Proposed Edit

**File**: `website/docs/reference/invowkfile-schema.mdx` (lines 128-134)

Add `version` to the reserved flags warning:

```
:::warning Reserved Flags
The following flags are reserved by invowk and cannot be used in user-defined commands:
- `env-file` (short `e`), `env-var` (short `E`)
- `env-inherit-mode`, `env-inherit-allow`, `env-inherit-deny`
- `workdir` (short `w`), `help` (short `h`), `runtime` (short `r`)
- `from` (short `f`), `force-rebuild`, `version`
- The `ivk-`, `invowk-`, and `i-` prefixes are also reserved for system flags
:::
```

This adds `version` and also mentions the prefix reservation inline (currently only
in cli.mdx, not in the invowkfile-schema.mdx warning).
