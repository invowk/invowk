<!--
SYNC IMPACT REPORT
==================
Version change: 1.0.0 → 1.1.0 (MINOR: New principle added)

Modified principles: None

Added sections:
- VI. Documentation Synchronization (NON-NEGOTIABLE) - new principle

Removed sections: None

Modified sections:
- Quality Gates: Added "Documentation Sync" row
- Development Workflow > During Implementation: Added documentation step
- Development Workflow > Before Committing: Added documentation verification step

Templates requiring updates:
- .specify/templates/plan-template.md ✅ (Constitution Check section already generic)
- .specify/templates/spec-template.md ✅ (No constitution-specific content)
- .specify/templates/tasks-template.md ✅ (Task types align with principles)

Follow-up TODOs: None
-->

# Invowk Constitution

## Core Principles

### I. Idiomatic Go & Schema-Driven Design

All code MUST follow established Go idioms and project-specific patterns:

- **Error Handling**: Use named returns with defer close pattern to aggregate resource cleanup errors. Never silently discard close errors. See `.claude/rules/error-handling.md`.
- **CUE Schemas**: All CUE struct definitions MUST be closed (`close({ ... })`) to reject unknown fields. Include validation constraints (regex patterns, range limits) beyond simple type declarations.
- **SPDX Headers**: Every `.go` file MUST have `// SPDX-License-Identifier: MPL-2.0` as its first line.
- **Declaration Order**: Follow `const` → `var` → `type` → `func` ordering (enforced by `decorder` linter). Place exported functions before unexported ones (enforced by `funcorder` linter).
- **Server Pattern**: Long-running components MUST implement the state machine pattern (Created → Starting → Running → Stopping → Stopped/Failed) with atomic state reads and mutex-protected transitions.

**Rationale**: Consistency across the codebase reduces cognitive load, prevents common bugs (resource leaks, race conditions), and enables automated quality enforcement.

### II. Comprehensive Testing Discipline

Every behavior change MUST have corresponding test coverage:

- **Unit Tests**: Table-driven tests for pure logic. Use `t.TempDir()` for temporary files. Reset global state via cleanup functions.
- **CLI Integration Tests**: Use `testscript` (`.txtar` format) in `tests/cli/testdata/` for CLI behavior verification. Tests run in isolated environments (`HOME=/no-home`). Use `--` separator for command flags.
- **Race Detection**: Run tests with `-race` flag. For race condition fixes, execute 10+ times with `-count=1` to bypass cache.
- **Module Validation**: After module-related changes, run `go run . module validate modules/*.invkmod --deep`.

**Mandatory commands before merge**:
```bash
make lint      # golangci-lint
make test      # All unit tests
make test-cli  # CLI integration tests (if CLI changed)
```

**Rationale**: Tests are the primary documentation of expected behavior and the safety net for refactoring. CLI tests ensure user-facing behavior remains stable.

### III. Consistent User Experience

All user-facing interfaces MUST maintain behavioral and visual consistency:

- **CLI Behavior**: Commands follow `invowk <noun> <verb>` pattern. Error messages use styled output with clear guidance. Exit codes: 0 = success, 1 = user error, 2+ = internal error.
- **Flag Conventions**: POSIX-style flags (`--flag=value`, `-f`). Use `--` to separate invowk flags from command flags.
- **Output Modes**: Support both human-readable (styled with Lip Gloss) and machine-readable (JSON) output where applicable.
- **Cross-Platform**: Scripts MUST use platform-specific implementations when native runtime with bash syntax is involved. Forward slashes in module paths for cross-platform compatibility.
- **Container Runtime**: ONLY Debian-based Linux containers are supported. Never use Alpine or Windows container images.

**Rationale**: Predictable CLI behavior reduces learning curve. Styled output improves readability while JSON enables scripting integration.

### IV. Single-Binary Performance

As a CLI tool, Invowk MUST prioritize startup time and resource efficiency:

- **Startup Latency**: Target <100ms for `invowk --help` on modern hardware. Lazy-load expensive resources (container engine detection, remote module resolution).
- **Memory Footprint**: Minimize allocations in hot paths. Stream large outputs rather than buffering.
- **Binary Size**: Avoid pulling in large dependencies for marginal features. Prefer standard library when adequate.
- **Concurrency Safety**: Use `atomic.Int32` for lock-free state reads. Protect state transitions with mutexes. Check context cancellation before setup work.

**Rationale**: CLI tools are invoked frequently; poor startup time compounds across workflows. Single-binary distribution means no external dependency management.

### V. Simplicity & Minimalism

Complexity MUST be justified and documented:

- **YAGNI**: Only implement what's needed now. Don't add features, abstractions, or "improvements" beyond what was requested.
- **No Over-Engineering**: Avoid premature abstractions. Three similar lines of code is better than a premature helper function.
- **No Backward-Compat Hacks**: If something is unused, delete it completely. No `_var` renames, re-exports, or `// removed` comments.
- **Minimal Comments**: Add comments for non-obvious behavior or business rules. Don't add docstrings/comments to unchanged code.
- **Focused Changes**: Bug fixes don't need surrounding code cleaned up. Simple features don't need extra configurability.

**Rationale**: Simplicity reduces maintenance burden and makes the codebase approachable for contributors.

### VI. Documentation Synchronization (NON-NEGOTIABLE)

**A task is NOT complete until all user-facing documentation is updated.**

Any change that affects user-facing behavior MUST have corresponding documentation updates:

- **CLI Changes**: New, modified, or removed commands/subcommands MUST be reflected in `README.md` and website docs (`website/docs/`).
- **CUE Schema Changes**: Updates to `invkfile.cue` or `invkmod.cue` schemas MUST be documented with examples showing the new syntax.
- **Configuration Changes**: New config options or behavioral defaults MUST be documented in the configuration section.
- **Behavior Changes**: Side-effects, error message changes, exit code changes, or runtime behavior modifications MUST be documented.
- **Flag/Argument Changes**: New flags, renamed flags, or removed flags MUST be updated in command help text AND documentation.

**Documentation locations to check**:
- `README.md` - Primary user documentation
- `website/docs/` - Website documentation (if applicable)
- `invkfile.cue` / sample modules - Example files that users copy
- CLI `--help` text - Embedded documentation

**Enforcement**:
- PRs with user-facing changes MUST include documentation updates in the same PR
- Documentation-only PRs are acceptable for clarifications but NOT for catching up on missed changes
- Reviewers MUST verify documentation completeness before approving

**Rationale**: Documentation that drifts from implementation creates user confusion, increases support burden, and damages project credibility. Users rely on documentation to learn the tool—stale docs teach incorrect usage.

## Quality Gates

Every PR MUST pass the following gates before merge:

| Gate | Command | Scope |
|------|---------|-------|
| Linting | `make lint` | All code |
| Unit Tests | `make test` | All code |
| CLI Tests | `make test-cli` | If CLI output/behavior changed |
| License Headers | `make license-check` | New `.go` files |
| Dependencies | `make tidy` | If dependencies changed |
| Module Validation | `go run . module validate modules/*.invkmod --deep` | If module logic changed |
| Website Build | `cd website && npm run build` | If website content changed |
| **Documentation Sync** | **Manual review** | **If ANY user-facing behavior changed** |

## Development Workflow

### Before Writing Code

1. Read relevant files before proposing changes
2. Use `EnterPlanMode` for non-trivial implementations
3. Verify understanding of existing patterns

### During Implementation

1. Follow declaration ordering (const → var → type → func)
2. Add SPDX headers to new Go files
3. Use named returns for resource cleanup
4. Write tests for behavior changes
5. Avoid introducing OWASP Top 10 vulnerabilities
6. **Update documentation alongside code changes** (Principle VI)

### Before Committing

1. Run the Agent Checklist (`.claude/rules/checklist.md`)
2. Use signed commits (`git commit -S` or `commit.gpgsign = true`)
3. Write Conventional Commit messages with descriptive bullet points
4. Ensure no unexplained complexity was added
5. **Verify all user-facing documentation is updated** (Principle VI)

## Governance

This constitution is the authoritative guide for technical decisions in Invowk:

1. **Supremacy**: Constitution principles take precedence over ad-hoc decisions. Amendments require explicit documentation and migration plans.

2. **Compliance Verification**: All PRs and code reviews MUST verify adherence to these principles. Violations require justification in the Complexity Tracking section of implementation plans.

3. **Amendment Process**:
   - MAJOR version: Backward-incompatible principle changes or removals
   - MINOR version: New principles or material expansions
   - PATCH version: Clarifications, typos, non-semantic refinements

4. **Reference**: The `.claude/rules/` directory contains detailed implementation guidance that operationalizes these principles. Rules files are authoritative for their specific domains.

5. **Conflict Resolution**: When principles appear to conflict, Simplicity (Principle V) is the tiebreaker—choose the simpler approach unless security or correctness requires otherwise.

**Version**: 1.1.0 | **Ratified**: 2026-01-21 | **Last Amended**: 2026-01-21
