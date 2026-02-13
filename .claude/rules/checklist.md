# Agent Checklist

## Mandatory Verification

**CRITICAL: Work is NOT complete until the full test suite passes.**

Run `make test` (NOT `make test-short`) and verify all tests pass before considering any task finished. This is non-negotiable - partial test runs or skipped verifications are not acceptable.

## Pre-Completion Checklist

Before considering work complete:

1. **Full test suite passes**: `make test` - Run the FULL test suite, not short mode.
2. **Linting passes**: `make lint`.
3. **License headers**: `make license-check` (for new Go files).
4. **Dependencies tidy**: `make tidy`.
5. **Documentation updated**: Check sync map in `.claude/skills/docs/SKILL.md` for affected docs.
6. **Website builds**: `cd website && npm run build` (if website changed).
7. **Sample modules valid**: `go run . module validate modules/*.invowkmod --deep` (if module-related).
8. **CLI tests pass**: `make test-cli` (if CLI commands/output changed).
9. **Native runtime mirrors**: If CLI tests were added/modified, verify native mirrors exist (`native_*.txtar` for each feature test). Exempt: u-root, container, discovery/ambiguity, dogfooding tests.
10. **Architecture diagrams current**: If changes affect component relationships, execution flow, discovery logic, or runtime behavior, verify diagrams in `docs/architecture/` reflect the changes. Run `make render-diagrams` if D2 sources were updated.

## Why Full Test Suite?

- `make test-short` skips integration tests that catch real issues.
- Container runtime tests, cross-package integration, and end-to-end CLI tests only run in full mode.
- CI runs the full test suite - local verification must match CI expectations.
