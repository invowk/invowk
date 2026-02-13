# Improvement Issues

Individual issue specifications for Invowk codebase improvements. Each file is designed to be directly usable as a GitHub issue.

See [`../improvements-plan.md`](../improvements-plan.md) for the master plan with implementation order and dependencies.

## Quick Reference

> Items marked **COMPLETED** were addressed by the spec-008 stateless composition refactoring (2026-02-06).

| ID | Title | Category | Priority | Effort | Status | Labels |
|----|-------|----------|----------|--------|--------|--------|
| [001](./001-uroot-error-handling.md) | Fix Error Handling in uroot | Quick Wins | High | Low | Open | `code-quality`, `good-first-issue` |
| [002](./002-uroot-file-helper.md) | Extract File Processing Helper | Quick Wins | Medium | Low | Open | `code-quality`, `refactoring` |
| [003](./003-tuiserver-close-comments.md) | Add Close Error Comments | Quick Wins | Low | Low | Open | `documentation`, `good-first-issue` |
| [004](./004-runtime-tests.md) | Add Runtime Package Tests | Testability | High | High | Open | `testing`, `runtime` |
| [005](./005-container-engine-tests.md) | Add Container Engine Tests | Testability | Medium | Medium | Open | `testing`, `container` |
| [006](./006-execution-context-decomposition.md) | Decompose ExecutionContext | Testability | High | Medium | **COMPLETED** | `refactoring`, `architecture` |
| [007](./007-container-integration-tests.md) | Add Container Integration Tests | Testability | Medium | High | Open | `testing`, `integration` |
| [008](./008-env-builder-interface.md) | Extract EnvBuilder Interface | Architecture | Medium | Medium | Open | `architecture`, `refactoring` |
| [009](./009-provision-package.md) | Extract Provisioning Package | Architecture | Medium | Medium | **COMPLETED** | `architecture`, `container` |
| [010](./010-validator-interface.md) | Create Validator Interface | Architecture | Low | Medium | Open | `architecture`, `validation` |
| [011](./011-package-documentation.md) | Add Package Documentation | Documentation | Low | Low | Open | `documentation`, `good-first-issue` |
| [012](./012-resolve-todos.md) | Resolve Existing TODOs | Documentation | Medium | Medium | **PARTIALLY DONE** | `enhancement`, `technical-debt` |
| [013](./013-website-documentation.md) | Update Website Documentation | Documentation | Low | Medium | Open | `documentation`, `website` |

## Categories

### Quick Wins (Low effort, immediate value)
- **001** - Error handling fixes in uroot package
- **002** - Code deduplication in uroot package
- **003** - Documentation consistency in tuiserver

### Testability (Critical for quality)
- **004** - Runtime package unit tests
- **005** - Container engine unit tests
- ~~**006** - ExecutionContext refactoring~~ **COMPLETED** (spec-008)
- **007** - Container integration tests

### Architecture (Long-term maintainability)
- **008** - EnvBuilder interface extraction
- ~~**009** - Provisioning package extraction~~ **COMPLETED** (spec-008)
- **010** - Validator interface for invowkfile

### Documentation (Developer experience)
- **011** - Package-level documentation
- **012** - TODO resolution (partially done: config path + force rebuild resolved)
- **013** - Website documentation updates

## Dependencies

> Items marked [DONE] were completed by spec-008 and no longer block downstream work.

```
001 ─────────────────────────────────────┐
                                         │
002 (depends on 001) ────────────────────┤
                                         ├──► 004
[DONE] 006 ──────────────────────────────┤
                                         │
008 (depends on 006 — now unblocked) ────┘

[DONE] 009 ──► [DONE] 012 (force rebuild part)

004 ──┐
      ├──► 007
005 ──┘
```

## Recommended Implementation Order

> Updated 2026-02-06: Items completed by spec-008 are struck through.

1. **Phase 1: Quick Wins** (1-2 days)
   - 001 - Fix error handling
   - 003 - Add comments
   - 011 - Package docs

2. **Phase 2: Core Testability** (1 week)
   - ~~006 - ExecutionContext decomposition~~ **COMPLETED**
   - 004 - Runtime tests (start with env.go)

3. **Phase 3: Architecture** (1 week)
   - 008 - EnvBuilder interface (now unblocked since 006 is done)
   - 002 - File processing helper
   - ~~009 - Provisioning package~~ **COMPLETED**

4. **Phase 4: Extended Testing** (1 week)
   - 005 - Container engine tests
   - 007 - Container integration tests

5. **Phase 5: Polish** (3 days)
   - ~~012 - Resolve TODOs~~ **PARTIALLY COMPLETED** (config path + force rebuild done)
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
