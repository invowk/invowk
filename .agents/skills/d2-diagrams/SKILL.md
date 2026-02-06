---
name: d2-diagrams
description: Agent-optimized D2 diagram generation with TALA layout. Use for C4 architecture, sequence, flowcharts, and any diagram requiring superior auto-layout. D2 is the DEFAULT choice for new diagrams. Triggers include "d2 diagram", "architecture diagram", "generate diagram", "visualize", "model", or any diagramming request. Optimized for validation, error recovery, and deterministic output.
disable-model-invocation: false
---

# D2 Diagramming (Agent-Optimized)

D2 is a modern diagram scripting language designed for software architecture visualization. It produces high-quality diagrams with superior auto-layout, especially when using the TALA layout engine.

**D2 is the default choice for new diagrams.**

## Why D2 for AI Agents

- **Error messages** - Line:col with context for easy debugging
- **Formatter** - `d2 fmt` canonicalizes for determinism
- **Validation** - `d2 validate` catches errors without rendering
- **Layout control** - TALA provides `near`, grids, and explicit positioning
- **Determinism** - Seed configuration ensures reproducible output
- **C4 support** - First-class with layers and suspend pattern

## Core Syntax Structure

All D2 diagrams are text files with `.d2` extension:

```d2
# Comments start with #

# Shapes (nodes)
server: Web Server
database: PostgreSQL {
  shape: cylinder
}

# Connections
server -> database: queries

# Containers (grouping)
backend: Backend {
  api: REST API
  worker: Background Worker
  api -> worker: enqueues
}

# Styling
server.style: {
  fill: "#4A90D9"
  stroke: "#2E5A8C"
}
```

**Key principles:**
- Shapes are created by naming them: `mynode: Label`
- Connections use arrows: `a -> b`, `a <- b`, `a <-> b`
- Containers use braces: `group: { ... }`
- Styling uses nested `.style` blocks
- Labels support Markdown: `node: |md # Heading |`

## Diagram Type Selection Guide

**Choose D2 when:**
1. **C4 Architecture** - First-class support with layers and suspend
2. **Complex layouts** - TALA provides superior auto-arrangement
3. **Deterministic output needed** - Seed configuration ensures reproducibility
4. **Validation pipeline** - `d2 validate` catches errors before rendering
5. **Grid layouts** - TALA's grid feature for aligned components
6. **Multi-view diagrams** - Layers/scenarios for different perspectives

## Quick Start Examples

### C4 Container Diagram

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}

title: |md
  ## E-Commerce Platform
  Container Diagram
|

# External actors
customer: Customer {
  shape: person
  style.fill: "#08427B"
}

# System boundary
ecommerce: E-Commerce System {
  style.stroke: "#1168BD"
  style.stroke-dash: 3

  web: Web Application {
    style.fill: "#438DD5"
  }
  api: API Gateway {
    style.fill: "#438DD5"
  }
  db: Database {
    shape: cylinder
    style.fill: "#438DD5"
  }

  web -> api: REST/JSON
  api -> db: SQL
}

# External systems
payment: Payment Gateway {
  style.fill: "#999999"
}

customer -> ecommerce.web: HTTPS
ecommerce.api -> payment: Process payments
```

### Sequence Diagram

```d2
shape: sequence_diagram

alice: Alice
bob: Bob
server: Server

alice -> bob: Hello
bob -> server: Check status
server -> bob: OK
bob -> alice: Hi back
```

### Flowchart

```d2
direction: down

start: Start {
  shape: oval
}
input: Get user input
validate: Validate? {
  shape: diamond
}
process: Process data
error: Show error
end: End {
  shape: oval
}

start -> input -> validate
validate -> process: Yes
validate -> error: No
error -> input
process -> end
```

## Agent Workflow

The recommended validation pipeline for agents:

```bash
# 1. Format (canonicalize for determinism)
d2 fmt diagram.d2

# 2. Validate (fast, no rendering)
d2 validate diagram.d2

# 3. Render (only after validation passes)
d2 --layout=tala diagram.d2 diagram.svg
```

**Error format (parseable):**
```
diagram.d2:5:1: error: connection must reference valid shape: "undefined_node"
diagram.d2:12:15: error: invalid keyword: "styel" (did you mean "style"?)
```

See [references/agent-workflow.md](references/agent-workflow.md) for the complete self-correction loop pattern.

## Determinism Configuration

**Use `--tala-seeds` CLI flag for reproducible layouts:**

```bash
# TALA seeds is a CLI flag, NOT a d2-config option
# Seed 100 provides optimal compactness for flowcharts while maintaining C4 quality
d2 --layout=tala --tala-seeds=100 diagram.d2 diagram.svg
```

**d2-config block should only contain layout-engine:**

```d2
vars: {
  d2-config: {
    layout-engine: tala
  }
}
```

> **⚠️ Common Mistake:** Putting `tala-seeds` inside `d2-config` causes `"tala-seeds" is not a valid config` errors. The `--tala-seeds` flag is TALA-specific and must be passed via CLI, not in the D2 file.

Without seeds, TALA may produce slightly different layouts on each render. For CI/CD and version control, deterministic output is essential. Pass seeds via the render script or Makefile.

## Detailed References

For in-depth guidance on specific diagram types:

- **[references/agent-workflow.md](references/agent-workflow.md)** - Validation pipeline, error parsing, self-correction loop, CI/CD integration
- **[references/c4-diagrams.md](references/c4-diagrams.md)** - C4 model with layers, suspend pattern, multi-view architecture
- **[references/sequence-diagrams.md](references/sequence-diagrams.md)** - Participants, messages, lifelines, groups, notes
- **[references/flowcharts.md](references/flowcharts.md)** - Shapes, connections, containers, grid layouts
- **[references/layout-engines.md](references/layout-engines.md)** - Dagre vs ELK vs TALA comparison and selection guide
- **[references/docusaurus-github.md](references/docusaurus-github.md)** - Integration with Docusaurus and GitHub workflows

## Best Practices for Agent Generation

1. **Always include config block** - Set layout engine at the top
2. **Use `d2 fmt` before committing** - Ensures canonical formatting
3. **Validate before rendering** - `d2 validate` is faster than full render
4. **Parse errors programmatically** - Format is `file:line:col: error: message`
5. **Use Markdown labels for descriptions** - `|md ... |` for rich text
6. **Prefer containers over flat diagrams** - Better visual organization
7. **Set explicit direction** - `direction: down` or `direction: right`
8. **Use shapes semantically** - `cylinder` for databases, `person` for actors
9. **Quote edge labels with special characters** - Labels containing `\n`, `[`, `]` need quoting

## Edge Label Quoting

**Always quote edge labels that contain special characters:**

```d2
# ✅ CORRECT - Quote labels with newlines and brackets
a -> b: "Sends data\n[HTTP]"
user -> api: "Authenticates\n[OAuth 2.0]"

# ❌ WRONG - Unquoted labels with brackets cause parse errors
a -> b: Sends data\n[HTTP]
# Error: unexpected text after unquoted string
```

**Characters that require quoting in edge labels:**
- `\n` (newline escape)
- `[` and `]` (D2 interprets as glob/attribute syntax)
- `{` and `}` (D2 interprets as block syntax)
- `:` (D2 interprets as key-value separator)

## Layout Engine Quick Reference

| Engine | Cost | Best For |
|--------|------|----------|
| `dagre` | Free | Quick drafts, simple graphs |
| `elk` | Free | Complex graphs, orthogonal routing |
| `tala` | $5-20/mo | Production, grids, `near` positioning |

```d2
vars: {
  d2-config: {
    layout-engine: tala  # or: dagre, elk
  }
}
```

## Common Shapes

```d2
# Basic shapes
rect: Rectangle
oval: Oval { shape: oval }
diamond: Decision { shape: diamond }
cylinder: Database { shape: cylinder }
person: User { shape: person }
cloud: Cloud Service { shape: cloud }
queue: Message Queue { shape: queue }
package: Package { shape: package }
hexagon: Hexagon { shape: hexagon }
parallelogram: I/O { shape: parallelogram }
```

## Connection Types

```d2
# Arrow styles
a -> b: directed
a <- b: reverse
a <-> b: bidirectional
a -- b: undirected

# Line styles
a -> b: {
  style.stroke-dash: 3  # dashed
}

a -> b: {
  style.stroke-width: 3  # thick
}
```

## When to Update Diagrams

**CRITICAL: Diagrams must stay synchronized with code.** A code change is not complete until affected diagrams are evaluated and updated.

### Update Triggers

| Code Change | Diagrams to Evaluate |
|-------------|---------------------|
| Package addition/removal | C4 Container diagram |
| Interface/contract changes | Sequence diagrams showing that component |
| Discovery algorithm changes | Discovery flowchart, execution sequences |
| Runtime selection changes | Runtime selection flowchart |
| Execution flow changes | All execution sequence diagrams |
| New external system integration | C4 Context diagram |

### Update Workflow

1. **Evaluate**: Review diagrams for affected components
2. **Update source**: Edit `.d2` files in `docs/diagrams/`
3. **Validate**: `d2 validate <file>.d2`
4. **Render**: `make render-diagrams`
5. **Commit**: Both `.d2` source and `.svg` rendered files

### Verification Checklist

Before marking work complete, verify:
- [ ] All affected diagrams reviewed
- [ ] `.d2` sources updated if needed
- [ ] `make render-diagrams` run successfully
- [ ] SVG files committed with sources

## Common Pitfalls

| Pitfall | Error | Fix |
|---------|-------|-----|
| `tala-seeds` in d2-config | `"tala-seeds" is not a valid config` | Use `--tala-seeds=100` CLI flag instead |
| Unquoted edge labels with `[...]` | `unexpected text after unquoted string` | Quote the label: `"text\n[protocol]"` |
| Unquoted edge labels with `\n` | Parser confusion | Quote the label: `"line1\nline2"` |
| Missing layout-engine config | Inconsistent layouts | Always set `layout-engine: tala` in d2-config |
| Using TALA-specific config with ELK | Config validation errors | Keep d2-config layout-agnostic |
| `Participant."Label": { }` for sequence grouping | All arrows route to first participant | Use flat structure with comments or proper `.span:` syntax |
