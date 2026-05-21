---
name: learn
description: Learning workflow for after substantive Invowk repo work, especially architecture changes, refactors, feature work, bug fixes, CI repairs, or explicit `/learn` requests. Use to keep `AGENTS.md`, rules, skills, hooks, and allowed memory notes up to date with durable lessons.
---

# Learning Workflow

Use this at the end of substantive work, not after trivial read-only answers.

1. Gather evidence: changed files, commands run, failures fixed, stale guidance found, and durable repo behavior learned.
2. Classify each learning:
   - `AGENTS.md` / `.claude/CLAUDE.md`: repository-wide governance, indexes, or routing.
   - `.agents/rules/`: mandatory policy that should override skills.
   - `.agents/skills/`: procedural workflow for a specific domain.
   - Hooks: canonical repo surfaces such as `.pre-commit-config.yaml`, `make install-hooks`, or tracked hook scripts. Do not edit generated `.git/hooks/*` files directly.
   - Memory notes: only when current higher-priority instructions and the user request permit writing memory. Otherwise report the suggested memory note.
3. Update only high-signal durable guidance. Avoid recording one-off command output, temporary branch state, or facts that should be rediscovered from source.
4. Keep wording concise and source-backed. If `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed, run `make check-agent-docs`.
5. Recheck `git diff` and confirm the learning update does not conflict with rule precedence or existing indexes.
