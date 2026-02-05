# Invowk Architecture Documentation

This directory contains C4 model diagrams and supplementary architecture documentation for Invowk.

## Diagram Overview

| Diagram | Type | Purpose |
|---------|------|---------|
| [C4 Context (C1)](./c4-context.md) | C4 Model | System boundaries, users, external systems |
| [C4 Container (C2)](./c4-container.md) | C4 Model | Internal components and data stores |
| [Command Execution](./sequence-execution.md) | Sequence | Temporal flow from CLI to execution |
| [Runtime Selection](./flowchart-runtime-selection.md) | Flowchart | Decision tree for choosing runtime |
| [Discovery Precedence](./flowchart-discovery.md) | Flowchart | How commands are found and conflicts resolved |

## Reading Order

For newcomers to the codebase:

1. **Start with [C4 Context](./c4-context.md)** - Understand what Invowk is and what it interacts with
2. **Then [C4 Container](./c4-container.md)** - See the major internal components
3. **Read [Command Execution](./sequence-execution.md)** - Follow the main user journey
4. **Reference others as needed** - Runtime selection and discovery when debugging or extending

## Diagram Technology

All diagrams use [Mermaid](https://mermaid.js.org/) syntax, which renders natively on GitHub and in many documentation tools.

### Viewing Diagrams

- **GitHub**: Renders automatically in markdown preview
- **VS Code**: Use the "Markdown Preview Mermaid Support" extension
- **CLI**: Use `mmdc` (Mermaid CLI) to generate images
- **Docusaurus**: Mermaid plugin renders diagrams in the website

### Editing Diagrams

The [Mermaid Live Editor](https://mermaid.live/) is useful for testing changes before committing.

## C4 Model Background

The [C4 model](https://c4model.com/) provides a hierarchical way to describe software architecture:

| Level | Name | Description |
|-------|------|-------------|
| C1 | Context | System as black box with external actors |
| C2 | Container | Major deployable/runnable units |
| C3 | Component | Internal modules within containers |
| C4 | Code | Class/code-level details |

For Invowk (a single CLI binary), C1 and C2 are most valuable. C3 would show internal Go packages, which are better documented in code.

## Keeping Diagrams Updated

When making significant architectural changes:

1. **Check if diagrams need updates** - New components, changed relationships, removed features
2. **Update the relevant diagram(s)** - Keep changes focused
3. **Verify rendering** - Test in GitHub or Mermaid Live Editor
4. **Update tables and text** - Diagrams alone may not capture all context

## Future Diagrams

Additional diagrams that could be valuable:

- **State Diagram: Server Lifecycle** - SSH and TUI server state machines
- **Class Diagram: Runtime Interface Hierarchy** - Interface relationships
- **Flowchart: Container Provisioning** - Ephemeral layer creation
- **ERD: Module Dependency Structure** - Module field relationships
