# Agent Checklist

## Mandatory Verification

**CRITICAL: Work is NOT complete until the full test suite passes.**

Run `make test` (NOT `make test-short`) and verify all tests pass before considering any task finished. This is non-negotiable - partial test runs or skipped verifications are not acceptable.

## Pre-Completion Checklist

Before considering work complete:

1. **Full test suite passes**: `make test` - Run the FULL test suite, not short mode.
2. **Linting passes**: `make lint` - Fix ALL issues EVEN if pre-existing.
3. **License headers**: `make license-check` (for new Go files).
4. **Dependencies tidy**: `make tidy`.
5. **Documentation updated**: Check sync map in `.agents/skills/docs/SKILL.md` for affected docs.
6. **Website builds**: `cd website && npm run build` (if website changed).
7. **Sample modules valid**: `go run . validate modules/*.invowkmod` (if module-related).
8. **CLI tests pass**: `make test-cli` (if CLI commands/output changed).
9. **Native runtime mirrors**: If CLI tests were added/modified, verify native mirrors exist (`native_*.txtar` for each feature test). Exempt: u-root, container, discovery/ambiguity, dogfooding, built-in command tests (config/module/completion/tui/init/validate).
10. **Architecture diagrams current**: If changes affect component relationships, execution flow, discovery logic, or runtime behavior, verify diagrams in `docs/architecture/` reflect the changes. Run `make render-diagrams` if D2 sources were updated.
11. **Baseline check passes**: `make check-baseline` - Verify no new goplint findings introduced. Note: baseline scoped to production packages (`./cmd/... ./internal/... ./pkg/...`).
12. **New goplint findings triaged with the user**: Every newly surfaced goplint violation must be evaluated carefully to decide whether the right fix belongs in Invowk production code or in goplint itself, regardless of the original task scope. If both are plausible, stop and ask the user to choose the final direction before closing the work.
13. **File length check**: `make check-file-length` - All Go files (production + test) must be under 1000 lines.
14. **Agent docs integrity**: `make check-agent-docs` (if `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed).

## Why Full Test Suite?

- `make test-short` skips integration tests that catch real issues.
- Container runtime tests, cross-package integration, and end-to-end CLI tests only run in full mode.
- CI runs the full test suite - local verification must match CI expectations.
