# Agent Checklist

## Mandatory Verification

**CRITICAL: Work is NOT complete until the full test suite passes.**

Run `make test` (NOT `make test-short`) and verify all tests pass before considering any task finished. This is non-negotiable - partial test runs or skipped verifications are not acceptable.

## Pre-Completion Checklist

Before considering work complete, follow this sequence. If any step produces code changes, re-verify from step 1.

### Code Refinement

1. **Simplify changed code**: Run `/simplify` to review changed code for reuse, quality, and efficiency. Fix any issues found. This step may produce additional changes — all subsequent steps verify the final state.
2. **Dependencies tidy**: `make tidy`.
3. **License headers**: `make license-check` (for new Go files).

### Verification

4. **Linting passes**: `make lint` - Fix ALL issues EVEN if pre-existing. *(Pre-commit hook.)*
5. **Full test suite passes**: `make test` - Run the FULL test suite, not short mode.
6. **CLI tests pass**: `make test-cli` (if CLI commands/output changed).
7. **Baseline check passes**: `make check-baseline` - Verify no new goplint findings introduced. Note: baseline scoped to production packages (`./cmd/... ./internal/... ./pkg/...`). *(Pre-commit hook.)*
8. **New goplint findings triaged with the user**: Every newly surfaced goplint violation must be evaluated carefully to decide whether the right fix belongs in Invowk production code or in goplint itself, regardless of the original task scope. If both are plausible, stop and ask the user to choose the final direction before closing the work.
9. **File length check**: `make check-file-length` - All Go files (production + test) must be under 1000 lines.
10. **Sonar issues resolved**: Run `make sonar-local` and review all unresolved issues. Fix real bugs and vulnerabilities. For false positives, add suppressions in `sonar-project.properties` and `.sonarcloud.properties` (multicriteria IDs must be gapless). CI analysis is handled by SonarCloud automatic analysis (GitHub App). *(Pre-commit hook.)*

### Documentation & Housekeeping

11. **Documentation updated**: Check sync map in `.agents/skills/docs/SKILL.md` for affected docs.
12. **Website builds**: `cd website && npm run build` (if website changed).
13. **Sample modules valid**: `go run . validate modules/*.invowkmod` (if module-related).
14. **Native runtime mirrors**: If CLI tests were added/modified, verify native mirrors exist (`native_*.txtar` for each feature test). Exempt: u-root, container, discovery/ambiguity, dogfooding, built-in command tests (config/module/completion/tui/init/validate).
15. **Architecture diagrams current**: If changes affect component relationships, execution flow, discovery logic, or runtime behavior, verify diagrams in `docs/architecture/` reflect the changes. Run `make render-diagrams` if D2 sources were updated.
16. **Agent docs integrity**: `make check-agent-docs` (if `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed).

### Knowledge Capture

17. **Capture learnings**: Run `/learn` to review and update `.claude/CLAUDE.md`, memory notes, hooks, rules, and skills with key learnings from this session. This is always the last step — it captures insights from the entire workflow including any discoveries made during verification.

## Pre-Commit Hook Coverage

The following items are also enforced by pre-commit hooks (`.pre-commit-config.yaml`), providing a safety net at commit time:

| Hook | Checklist Step | Notes |
|------|---------------|-------|
| `golangci-lint` | 4 (Linting) | `golangci-lint run --config=.golangci.toml ./...` |
| `goplint-baseline` | 7 (Baseline) | `make check-baseline` |
| `goplint-behavior` | — | Semantic-spec, IFDS, Phase C/D gates (not explicitly in checklist) |
| `sonar-local` | 10 (Sonar) | API-only check; `SONAR_TOKEN` optional (public projects work without auth) |

Items NOT covered by any hook (manual discipline required): `make test`, `make tidy`, `make test-cli`, `make check-file-length`, `make check-agent-docs`, documentation/diagram/module validation, `/simplify`, and `/learn`.

## Why Full Test Suite?

- `make test-short` skips integration tests that catch real issues.
- Container runtime tests, cross-package integration, and end-to-end CLI tests only run in full mode.
- CI runs the full test suite - local verification must match CI expectations.
