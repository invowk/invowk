---
name: learn
description: Learning workflow after substantive Invowk repo work (architecture changes, refactors, feature work, bug fixes, CI repairs) or explicit `/learn` requests. Use to keep `AGENTS.md`, rules, skills, runbooks, memorials, hooks and allowed memory notes up to date with durable lessons.
---

# Learning Workflow

Use this at the end of substantive work, not after trivial read-only answers.

1. Gather evidence: changed files, commands run, failures fixed, stale guidance found, and durable repo behavior learned.
2. Classify each learning:
   - `AGENTS.md`: repository-wide governance, indexes, or routing. `AGENTS.md` is the canonical name; it is a symlink to `.claude/CLAUDE.md`.
   - `.agents/rules/`: mandatory policy that should override skills.
   - `.agents/skills/`: procedural workflow for a specific domain.
   - `.agents/runbooks/`: on-demand procedures with the standing of a skill. List every file in `.agents/runbooks/README.md`.
   - `.agents/memorials/`: history and past decisions, never normative. List every file in `.agents/memorials/README.md`.
   - Hooks: canonical repo surfaces such as `.pre-commit-config.yaml`, `make install-hooks`, or tracked hook scripts. Do not edit generated `.git/hooks/*` files directly. Claude Code harness hooks (PreToolUse/PostToolUse) live in `.claude/settings.json`, a real tracked file edited in place.
   - Compatibility surfaces: `.claude/{rules,skills,agents,commands}` are symlinks to `.agents/...`, so edit the canonical file and never cite those `.claude/*` paths as canonical. `.claude/docs/` and `.claude/issues/` are real directories, not aliases.
   - Memory notes: only when current higher-priority instructions and the user request permit writing memory. Otherwise report the suggested memory note.
3. Update only high-signal durable guidance. Avoid recording one-off command output, temporary branch state, or facts that should be rediscovered from source.
4. Keep wording concise and source-backed. If `AGENTS.md`, `.claude/`, `.agents/`, or `scripts/check-agent-docs.sh` changed, run `make check-agent-docs` (checklist step 8).
5. Recheck `git diff` and confirm the learning update does not conflict with rule precedence or existing indexes.
