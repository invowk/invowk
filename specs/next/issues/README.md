# Improvement Issues

Individual issue specifications for Invowk codebase improvements. Each file is designed to be directly usable as a GitHub issue.

See [`../improvements-plan.md`](../improvements-plan.md) for the master plan with implementation order and dependencies.

## Quick Reference

| ID | Title | Category | Priority | Effort | Labels |
|----|-------|----------|----------|--------|--------|
| [001](./001-uroot-error-handling.md) | Fix Error Handling in uroot | Quick Wins | High | Low | `code-quality`, `good-first-issue` |
| [002](./002-uroot-file-helper.md) | Extract File Processing Helper | Quick Wins | Medium | Low | `code-quality`, `refactoring` |
| [003](./003-tuiserver-close-comments.md) | Add Close Error Comments | Quick Wins | Low | Low | `documentation`, `good-first-issue` |
| [004](./004-runtime-tests.md) | Add Runtime Package Tests | Testability | High | High | `testing`, `runtime` |
| [005](./005-container-engine-tests.md) | Add Container Engine Tests | Testability | Medium | Medium | `testing`, `container` |
| [006](./006-execution-context-decomposition.md) | Decompose ExecutionContext | Testability | High | Medium | `refactoring`, `architecture` |
| [007](./007-container-integration-tests.md) | Add Container Integration Tests | Testability | Medium | High | `testing`, `integration` |
| [008](./008-env-builder-interface.md) | Extract EnvBuilder Interface | Architecture | Medium | Medium | `architecture`, `refactoring` |
| [009](./009-provision-package.md) | Extract Provisioning Package | Architecture | Medium | Medium | `architecture`, `container` |
| [010](./010-validator-interface.md) | Create Validator Interface | Architecture | Low | Medium | `architecture`, `validation` |
| [011](./011-package-documentation.md) | Add Package Documentation | Documentation | Low | Low | `documentation`, `good-first-issue` |
| [012](./012-resolve-todos.md) | Resolve Existing TODOs | Documentation | Medium | Medium | `enhancement`, `technical-debt` |
| [013](./013-website-documentation.md) | Update Website Documentation | Documentation | Low | Medium | `documentation`, `website` |

## Categories

### Quick Wins (Low effort, immediate value)
- **001** - Error handling fixes in uroot package
- **002** - Code deduplication in uroot package
- **003** - Documentation consistency in tuiserver

### Testability (Critical for quality)
- **004** - Runtime package unit tests
- **005** - Container engine unit tests
- **006** - ExecutionContext refactoring
- **007** - Container integration tests

### Architecture (Long-term maintainability)
- **008** - EnvBuilder interface extraction
- **009** - Provisioning package extraction
- **010** - Validator interface for invkfile

### Documentation (Developer experience)
- **011** - Package-level documentation
- **012** - TODO resolution
- **013** - Website documentation updates

## Dependencies

```
001 ─────────────────────────────────────┐
                                         │
002 (depends on 001) ────────────────────┤
                                         ├──► 004
006 ─────────────────────────────────────┤
                                         │
008 (depends on 006) ────────────────────┘

009 ──► 012 (force rebuild part)

004 ──┐
      ├──► 007
005 ──┘
```

## Recommended Implementation Order

1. **Phase 1: Quick Wins** (1-2 days)
   - 001 - Fix error handling
   - 003 - Add comments
   - 011 - Package docs

2. **Phase 2: Core Testability** (1 week)
   - 006 - ExecutionContext decomposition
   - 004 - Runtime tests (start with env.go)

3. **Phase 3: Architecture** (1 week)
   - 008 - EnvBuilder interface
   - 002 - File processing helper
   - 009 - Provisioning package

4. **Phase 4: Extended Testing** (1 week)
   - 005 - Container engine tests
   - 007 - Container integration tests

5. **Phase 5: Polish** (3 days)
   - 012 - Resolve TODOs
   - 013 - Website docs
   - 010 - Validator interface (if time)

## Good First Issues

These issues are suitable for new contributors:
- **001** - Mechanical changes with clear patterns
- **003** - Simple documentation addition
- **011** - Package documentation (no code changes)

## Creating GitHub Issues

To create GitHub issues from these specs:

```bash
# Example using gh CLI
gh issue create \
  --title "Fix Error Handling in internal/uroot/ Commands" \
  --body-file specs/next/issues/001-uroot-error-handling.md \
  --label "code-quality" \
  --label "good-first-issue"
```

Or manually copy the markdown content into GitHub's issue editor.
