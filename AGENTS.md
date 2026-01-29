# AGENTS.md

For coding agent guidelines, see [.claude/CLAUDE.md](.claude/CLAUDE.md).

## Directory Structure

The [.claude/](.claude/) directory contains all agent configuration:

| Path | Purpose |
|------|---------|
| [.claude/CLAUDE.md](.claude/CLAUDE.md) | Main context file (project overview, architecture, key dependencies) |
| [.claude/rules/](.claude/rules/) | Project rules and conventions (18 files covering Go style, testing, CUE schemas, git, etc.) |
| [.claude/skills/](.claude/skills/) | Skill-based prompts invokable via `/skill-name` (speckit workflow commands) |
| [.claude/agents/](.claude/agents/) | Agent definitions and configurations |
| `.claude/settings.local.json` | Local permission settings (not committed) |

## Rules

The [.claude/rules/](.claude/rules/) directory contains project-specific rules:

- [general-rules.md](.claude/rules/general-rules.md) - Instruction priority, code quality, documentation
- [go-style.md](.claude/rules/go-style.md) - Go naming, imports, error handling, interfaces
- [commands.md](.claude/rules/commands.md) - Build, test, and release commands
- [testing.md](.claude/rules/testing.md) - Test patterns, CLI integration tests, VHS demos
- [cue.md](.claude/rules/cue.md) - CUE schema conventions
- [git.md](.claude/rules/git.md) - Commit signing, squash merge, message format
- [licensing.md](.claude/rules/licensing.md) - SPDX headers for MPL-2.0
- [linting.md](.claude/rules/linting.md) - golangci-lint configuration
- [error-handling.md](.claude/rules/error-handling.md) - Defer close pattern with named returns
- [servers.md](.claude/rules/servers.md) - Server lifecycle state machine
- [invkfile.md](.claude/rules/invkfile.md) - Invkfile example conventions
- [invkmod.md](.claude/rules/invkmod.md) - Module sample conventions
- [bash-scripting.md](.claude/rules/bash-scripting.md) - Strict mode and arithmetic gotchas
- [mvdan-sh.md](.claude/rules/mvdan-sh.md) - Virtual shell positional arguments
- [toctou-race-conditions.md](.claude/rules/toctou-race-conditions.md) - Context cancellation race patterns
- [plans-and-fixes.md](.claude/rules/plans-and-fixes.md) - Comprehensive fix handling
- [docs-website.md](.claude/rules/docs-website.md) - Documentation sync and i18n
- [checklist.md](.claude/rules/checklist.md) - Pre-completion verification steps

## Skills

The [.claude/skills/](.claude/skills/) directory contains invokable skills:

- [speckit.specify](.claude/skills/speckit.specify.md) - Create feature specifications
- [speckit.clarify](.claude/skills/speckit.clarify.md) - Clarify spec requirements
- [speckit.plan](.claude/skills/speckit.plan.md) - Generate implementation plans
- [speckit.tasks](.claude/skills/speckit.tasks.md) - Generate task lists
- [speckit.implement](.claude/skills/speckit.implement.md) - Execute implementation
- [speckit.analyze](.claude/skills/speckit.analyze.md) - Cross-artifact analysis
- [speckit.checklist](.claude/skills/speckit.checklist.md) - Generate custom checklists
- [speckit.constitution](.claude/skills/speckit.constitution.md) - Project constitution management
- [speckit.taskstoissues](.claude/skills/speckit.taskstoissues.md) - Convert tasks to GitHub issues

## Agents

The [.claude/agents/](.claude/agents/) directory is reserved for agent definitions.
