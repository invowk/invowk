Review all documentation surfaces for accuracy against the current codebase. Use the `/review-docs` skill (`.agents/skills/review-docs/SKILL.md`).

## Workflow

1. **Load the skill**: Read `.agents/skills/review-docs/SKILL.md`. Each subagent reads only its assigned reference files (see the orchestration table in the skill).
2. **Validate the contract and run programmatic checks first** (see `references/verification-commands.md`): run `scripts/review_docs.py validate`, then parity, container policy, diagram readability/validation/renders, agent-doc integrity, version assets, typecheck, and website build.
3. **Prepare canonical context**: record the nine gate results in external `checks.json`, then run `scripts/review_docs.py prepare` to validate exact page ownership, compute the i18n lag set, capture inventories, and hash the workspace.
4. **Run 11 surface subagents** — one per surface (S1–S11). Use available concurrency, queue remaining surfaces in numeric order, and never assume a fixed runtime limit.
5. **Validate every result**: each subagent returns canonical JSON with PASS/FAIL/SKIP/BLOCKED for every assigned item. Validate it against `context.json`; incomplete evidence is BLOCKED and makes the audit INCOMPLETE.
6. **Merge mechanically**: verify the snapshot and run `scripts/review_docs.py merge` to reject invalid input, deduplicate, sort, assign RD IDs, and generate Markdown plus canonical JSON. Do not hand-merge reports.

## Scope Rules

- Review ONLY `website/docs/` (next version). Do NOT review or change `website/versioned_docs/`.
- Review `README.md` for accuracy against code.
- Do NOT flag intentional simplifications (check `references/intentional-simplifications.md`).
- Do NOT treat broad directory scope as semantic coverage; every current MDX page must have one exact owner in `references/doc-ownership.json`.
- Do NOT flag style preferences, wording nuance, or possible omissions unless they map to an explicit checklist item with exact source-of-truth evidence.
- Delegate diagram-specific review details to the `/d2-diagrams` skill.
- Use the `/docs` skill only if edits are needed after review (this command is review-only).
- Use subagents and teammates coordinated by you.
