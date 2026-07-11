---
name: d2-diagrams
description: Agent-optimized D2 diagram generation and maintenance for Invowk architecture documentation. Use when creating or editing `.d2` files, C4 diagrams, sequence diagrams, flowcharts, diagram renders, or when a user explicitly requests a repository diagram. D2 is the default for new Invowk diagrams.
---

# D2 Diagrams

Use D2 for new repository diagrams. Keep this file focused on the Invowk
workflow; load only the reference for the requested diagram type.

## Reference Router

| Need | Read |
|---|---|
| Validation, repair loop, CI integration | [references/agent-workflow.md](references/agent-workflow.md) |
| C4 context/container/component views | [references/c4-diagrams.md](references/c4-diagrams.md) |
| Sequence diagrams | [references/sequence-diagrams.md](references/sequence-diagrams.md) |
| Flowcharts and readability | [references/flowcharts.md](references/flowcharts.md) |
| Dagre, ELK, or TALA selection | [references/layout-engines.md](references/layout-engines.md) |
| Docusaurus/GitHub rendering | [references/docusaurus-github.md](references/docusaurus-github.md) |

## Repository Workflow

1. Identify the existing diagram and every source/render consumer. For new
   diagrams, place the `.d2` source with the repository's current diagram
   sources and follow the adjacent naming convention.
2. Choose the smallest diagram type that communicates the relationship:
   flowchart for branching flow, sequence for interactions over time, and C4
   for system/package boundaries.
3. Edit the D2 source. Set an explicit direction for ordinary graphs and use
   semantic shapes.
4. Format and validate the edited source:

   ```bash
   d2 fmt path/to/diagram.d2
   d2 fmt path/to/diagram.d2 --check
   d2 validate path/to/diagram.d2
   ```

5. Render and run repository guardrails:

   ```bash
   make render-diagrams
   make check-diagram-renders
   make check-diagram-readability
   ```

6. Review the source and SVG diffs together. Commit both when the render
   changes.

The repository render script owns layout-engine probing and deterministic
render flags. Prefer it over reconstructing a direct D2 render command.

## Required Readability Invariants

For flowcharts:

- define an explicit oval Start node and a visible edge from it;
- set `direction: down` or `direction: right`;
- label every outgoing decision edge;
- keep one clear primary entry path; and
- run `make check-diagram-readability`.

For person shapes, set consistent explicit dimensions because label width can
distort the shape. Existing diagrams use `width: 70` and `height: 100`.

Quote edge labels containing D2 syntax characters such as `\n`, brackets,
braces, or colons. Avoid literal `$` in labels unless intentionally using D2
substitution syntax.

## Layout and Determinism

Repository diagrams normally declare TALA in the D2 config:

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}
```

`tala-seeds` is a CLI flag, not a `d2-config` key. Do not hard-code assumptions
about its availability; `scripts/render-diagrams.sh` probes the installed D2
binary and applies the supported deterministic arguments. Use
`./scripts/render-diagrams.sh --allow-elk` only for the documented local preview
fallback, not as the committed production-render contract.

## Synchronization Triggers

Evaluate and update diagrams when code changes alter package boundaries,
component relationships, runtime/discovery selection, execution sequences, or
external integrations. A code change is not complete until affected diagrams
have been checked even when no diagram edit is ultimately necessary.

Before completion, confirm:

- edited `.d2` files format and validate;
- repository render and readability gates pass;
- generated SVGs match their source hashes;
- source and render diffs communicate the same architecture; and
- documentation links still target the generated assets.
