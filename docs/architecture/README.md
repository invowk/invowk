# Invowk Architecture Documentation

This directory contains C4 model diagrams and supplementary architecture documentation for Invowk.

## Diagram Overview

| Diagram | Type | Purpose |
|---------|------|---------|
| [C4 Context (C1)](./c4-context.md) | C4 Model | System boundaries, users, external systems |
| [C4 Container (C2)](./c4-container.md) | C4 Model | Internal components and data stores |
| [C4 Component: Runtime (C3)](./c4-component-runtime.md) | C4 Model | Internal structure of the runtime package |
| [C4 Component: Container (C3)](./c4-component-container.md) | C4 Model | Internal structure of the container package |
| [Command Execution](./sequence-execution.md) | Sequence | Temporal flow from CLI to execution |
| [Runtime Selection](./flowchart-runtime-selection.md) | Flowchart | Decision tree for choosing runtime |
| [Discovery Precedence](./flowchart-discovery.md) | Flowchart | How commands are found and conflicts resolved |

## Reading Order

For newcomers to the codebase:

1. **Start with [C4 Context](./c4-context.md)** - Understand what Invowk is and what it interacts with
2. **Then [C4 Container](./c4-container.md)** - See the major internal components
3. **Optionally explore [Runtime Components](./c4-component-runtime.md) or [Container Components](./c4-component-container.md)** - Deep-dive into packages with non-trivial internal structure
4. **Read [Command Execution](./sequence-execution.md)** - Follow the main user journey
5. **Reference others as needed** - Runtime selection and discovery when debugging or extending

## Diagram Technology

All diagrams use [D2](https://d2lang.com/) as the source format with TALA layout engine for production-quality auto-arrangement. Diagrams are rendered to SVG and committed to the repository, ensuring consistent display across all platforms.

### Diagram Sources and Renders

```
docs/diagrams/
├── c4/                     # C4 model diagram sources (.d2)
├── sequences/              # Sequence diagram sources (.d2)
├── flowcharts/             # Flowchart sources (.d2)
└── rendered/               # Pre-rendered SVG files (committed to git)
    ├── c4/
    ├── sequences/
    └── flowcharts/
```

### Viewing Diagrams

- **GitHub**: SVG images render automatically in markdown preview
- **Docusaurus**: SVG files served from static directory
- **Local**: Open SVG files directly in any browser

### Editing Diagrams

1. **Edit the `.d2` source file** in `docs/diagrams/`
2. **Validate syntax**: `d2 validate <file>.d2`
3. **Preview locally** (optional): `d2 --layout=elk <file>.d2 preview.svg`
4. **Render all diagrams**: `make render-diagrams`
5. **Commit both** `.d2` source and `.svg` rendered files

**Note**: The `make render-diagrams` command uses TALA if available (for production quality), otherwise falls back to ELK layout engine.

### Why D2 Over Mermaid?

| Feature | D2 | Mermaid |
|---------|-----|---------|
| Error messages | Line:col with context | Often cryptic |
| Validation | `d2 validate` (no render) | Must render to detect errors |
| Layout quality | TALA: superior auto-arrangement | Limited hints |
| Determinism | Seed configuration | Non-deterministic |
| C4 support | First-class with containers | Basic extension |

## C4 Model Background

The [C4 model](https://c4model.com/) provides a hierarchical way to describe software architecture:

| Level | Name | Description |
|-------|------|-------------|
| C1 | Context | System as black box with external actors |
| C2 | Container | Major deployable/runnable units |
| C3 | Component | Internal modules within containers |
| C4 | Code | Class/code-level details |

For Invowk (a single CLI binary), C1 and C2 are most valuable. C3 (Component) diagrams are provided selectively for packages whose internal structure is genuinely complex — specifically `internal/runtime/` (interface segregation, registry pattern, 3 implementations) and `internal/container/` (composition via embedding, decorator pattern, functional options). Other packages have flat enough structure that code documentation suffices.

## Keeping Diagrams Updated

When making significant architectural changes:

1. **Check if diagrams need updates** - New components, changed relationships, removed features
2. **Edit the D2 source file** - Use `d2 validate` to check syntax
3. **Run `make render-diagrams`** - Re-render all diagrams to SVG
4. **Verify rendering** - Check SVG output visually
5. **Commit both source and rendered files** - Keep them in sync
6. **Update tables and text** - Diagrams alone may not capture all context

## For External Contributors

If you don't have D2 or TALA installed:

1. Edit `.d2` source files
2. Run `d2 validate` locally (D2 is free to install)
3. Submit PR with only `.d2` changes
4. Maintainer will run `make render-diagrams` and commit SVGs before merge

## Future Diagrams

Additional diagrams that could be valuable:

- **State Diagram: Server Lifecycle** - SSH and TUI server state machines
- **ERD: Module Dependency Structure** - Module field relationships
