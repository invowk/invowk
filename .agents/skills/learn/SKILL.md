---
name: learn
description: Learning workflow for when agents finish any architecture changes, refactoring, feature changes, or bug fix work. Use when you finish any other activity to ensure CLAUDE.md, hooks, rules, and skills are up-to-date with your latest learnings and don't have stale data or gaps.
disable-model-invocation: false
---

After you finish your work, carefully review CLAUDE.md, hooks, rules, and skills to double-check they're up-to-date with your latest learnings and don't have stale data or gaps.

When you have completed the /learn review, mark it done by running:
```
touch /tmp/claude-learn-$PPID
```
This clears the Stop hook reminder that triggers when code changes were made without a /learn pass.