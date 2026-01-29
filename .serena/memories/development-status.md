# Invowk Development Status

**Last Updated**: 2026-01-26

## Active Branch
`001-module-cmd-discovery` - Multi-source command discovery feature

## Feature: Module-Aware Command Discovery

### Summary
Extends `invowk cmd` to discover commands from:
1. Root `invkfile.cue`
2. First-level `*.invkmod` directories

### Key Capabilities
- Transparent namespace for unambiguous commands
- Canonical namespace for conflicts: `@<source>` or `--from <source>`
- Source grouping in command listings
- Graceful handling of invalid modules (warn + skip)

### Disambiguation Syntax
```bash
# For modules
invowk cmd @foo deploy
invowk cmd --from foo deploy

# For root invkfile
invowk cmd @invkfile deploy
invowk cmd --from invkfile deploy
```

### Implementation Location
- `internal/discovery/` - Discovery logic
- `cmd/invowk/cmd.go` - CLI integration

## Recent Commits
- `ecf14d4` feat(discovery): implement multi-source command discovery
- `46a4d26` docs: amend constitution to v1.2.0
- `17a599c` fix(runtime): prevent mvdan/sh from interpreting positional args
- `12a61b6` feat(discovery): enforce leaf-only args constraint
- `c7c0b03` chore(license): replace EPL-2.0 with MPL-2.0

## Spec Location
`specs/001-module-cmd-discovery/`
- `spec.md` - Feature specification
- `tasks.md` - Implementation tasks
- `plan.md` - Implementation plan
