## Objective

Deliver a complete, implementation-accurate documentation surface for Invowk by auditing and fixing mismatches between user-facing behavior (CLI/schema/runtime) and docs/examples, then enforcing parity with tests and CI guardrails.

## Expected result

- No known docs↔API mismatches in README, website docs (EN/PT-BR), snippets, or user-facing help/error text.
- --config behaves as documented.
- Regressions are blocked by automated parity checks.

———

## Phase 0 — Baseline & Parity Matrix (Discovery Only)

### Goals

- Freeze current mismatch inventory and remediation targets before edits.

### Tasks

- Create parity matrix section in this plan file with columns:
    - ID, Severity, Observed, Expected, Source files, Fix files, Validation.
- Record confirmed mismatches:
    - invk_mods vs invk_modules
    - invalid README runtime examples (runtime: shape)
    - undocumented module_aliases
    - discovery order mismatch
    - missing --config in docs
    - exposed but non-functional --config
    - stale help/issue commands (cmd list, invowk fix, test.unit)
    - Alpine snippet contradiction

### Exit criteria

- Matrix approved as source of truth for execution phases.

———

## Phase 1 — Behavior Corrections in CLI/Core (Code First)

### Goals

- Ensure runtime behavior and user-facing command UX are internally consistent before doc updates.

### Tasks

1. Implement functional --config loading:
    - Add explicit-path loader in internal/config (LoadFromPath(path string) or equivalent).
    - Update cmd/invowk/root.go init flow to honor cfgFile when provided.
2. Update user-facing help/issue strings:
    - Replace test.unit style with space-style examples.
    - Replace invowk cmd list guidance with invowk cmd / invowk cmd --list.
    - Remove/replace invowk fix recommendation with existing supported commands.
3. Adjust discovery collision hint text if command syntax shown is stale.

### Tests

- Add/adjust tests for:
    - --config custom path behavior.
    - updated help/issue text (where assertions exist).

### Exit criteria

- Behavior and internal UX text match intended product semantics.
- All relevant Go tests pass.

———

## Phase 2 — Canonical Docs & Snippet Remediation (EN First)

### Goals

- Align all English docs/examples to the corrected behavior.

### Tasks

1. Replace invk_mods → invk_modules in README + website docs + snippets.
2. Rewrite invalid README CUE examples to current schema:
    - command uses implementations with runtimes[]; no legacy runtime: command field.
3. Document module_aliases in:
    - README config section
    - website/docs/configuration/options.mdx
    - website/docs/reference/config-schema.mdx
    - website/src/components/Snippet/snippets.ts
4. Fix discovery-order documentation to match implementation ordering.
5. Add --config to global CLI flags docs.
6. Replace unsupported Alpine snippet example with supported Linux/glibc image example.

### Validation

- Snippet ID references remain valid.
- No broken internal links introduced.

### Exit criteria

- EN docs are behavior-accurate and buildable.

———

## Phase 3 — PT-BR Synchronization

### Goals

- Ensure translated docs mirror EN structure and semantics for all touched pages.

### Tasks

- For each updated EN .mdx, update matching PT-BR path:
    - same snippet IDs
    - translated prose parity
    - aligned tables, flags, and command guidance.

### Validation

- PT-BR docs compile with shared snippets.
- No missing translation-path counterparts for touched EN pages.

### Exit criteria

- EN/PT-BR parity complete for all modified docs.

———

## Phase 4 — Guardrails & Automation

### Goals

- Prevent recurrence of known drift patterns.

### Tasks

1. Add parity script: scripts/docs/check-parity.sh:
    - fail if forbidden tokens appear (invk_mods, invowk cmd list, stale patterns).
    - fail if required items missing (module_aliases docs, --config global flag docs).
2. Integrate script into CI docs/check job.
3. Add checklist reference in .claude/rules/checklist.md for parity check execution.

### Exit criteria

- CI fails on reintroduced parity regressions.
- Contributor workflow includes parity check.

———

## Phase 5 — Final Verification & Closure

### Goals

- Confirm end-to-end correctness before merge.

### Tasks

- Run verification suite:
    - make test
    - make test-cli (if CLI/help output changed)
    - cd website && npm run build
    - scripts/docs/check-parity.sh
- Update matrix status (Open → Closed) with evidence:
    - changed file paths
    - test/build output summary
    - residual risks (if any).

### Exit criteria

- All checks green.
- Matrix fully closed or explicitly risk-accepted.

———

## Deliverables

- specs/next/docs-api-parity-remediation-plan.md (this phased plan + matrix).
- Code fixes for CLI/config/help behavior.
- Updated README/docs/snippets (EN/PT-BR).
- New parity check script + CI integration.
- Passing validation/test/build results.

## Assumptions and defaults

- --config remains public and must be implemented (not removed).
- Current implementation is source of truth unless explicitly changed in Phase 1.
- PT-BR updates are mandatory for every touched EN docs page.