# Git and Commits

## Commit Signing

**CRITICAL: All commits MUST be signed (GPG or SSH, whichever is already configured on the system).** The `main` branch is protected and will reject unsigned commits. Always use `git commit -S` or configure Git to sign commits by default:

```bash
git config --global commit.gpgsign true
```

## Commit Message Format

All commits should include a detailed description of what changed. Use a short Conventional Commit-style subject line, and a body with bullet points describing the key modifications.

- Subject: `type(scope): summary` (keep concise, <= 72 chars).
- Body: 3-6 bullets describing what was changed (and why if helpful).
- Call out user-facing behavior/schema changes and migrations.
- Avoid vague messages like "misc" or "wip".

Example:

```
refactor(invkfile): rename commands to cmds

- Rename invkfile root field `commands` -> `cmds`
- Update dependency key `depends_on.commands` -> `depends_on.cmds`
- Adjust docs/examples/tests to match the new schema
```
