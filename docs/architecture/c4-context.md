# C4 System Context Diagram (C1)

This diagram shows Invowk from the highest level - the system boundaries, users, and external systems it interacts with.

## Diagram

![C4 System Context Diagram](../diagrams/rendered/c4/context.svg)

## Key Actors

| Actor | Description |
|-------|-------------|
| **Developer** | Primary user who creates `invkfile.cue` files and runs commands. Defines command implementations across different runtimes. |
| **Team Member** | Consumes shared modules (`.invkmod` directories) and runs commands defined by others. Benefits from portable, reproducible commands. |

## External Systems

| System | Purpose | Protocol |
|--------|---------|----------|
| **Docker Engine** | Container runtime for isolated command execution. Provides reproducible environments. | Docker CLI/API |
| **Podman Engine** | Alternative rootless container runtime. Preferred for security-conscious environments. | Podman CLI |
| **Git Repositories** | Source for remote module dependencies. Enables sharing and versioning of command modules. | Git protocol (HTTPS/SSH) |
| **Host Shell** | Native command execution. Uses `bash`/`sh` on Unix, `PowerShell` on Windows. | Process exec/spawn |
| **Filesystem** | Storage for configuration, invkfiles, modules, and scripts. | File I/O |

## Key Boundaries

### What Invowk Owns
- Command discovery and resolution
- Runtime selection and execution
- Module dependency management
- Configuration parsing and validation

### What Invowk Delegates
- Actual command execution (to shells or containers)
- Container image management (to Docker/Podman)
- Version control operations (to Git)
- Filesystem operations (to OS)

## Design Decisions Visible at This Level

1. **Multi-Runtime Support**: Invowk doesn't mandate a single execution environment. Users choose native shell for speed, virtual shell for portability, or containers for isolation.

2. **Container Engine Abstraction**: Both Docker and Podman are supported with automatic fallback, reducing vendor lock-in.

3. **Git-Based Module Distribution**: Remote modules use Git for versioning and distribution, leveraging existing infrastructure rather than creating a custom registry.

4. **Filesystem-Centric Configuration**: All configuration uses local files (CUE format), enabling version control and team sharing.

## Related Diagrams

- [C4 Container Diagram (C2)](./c4-container.md) - Zoom into Invowk's internal structure
- [Command Execution Sequence](./sequence-execution.md) - Temporal flow of command execution
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
