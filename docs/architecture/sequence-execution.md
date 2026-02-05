# Command Execution Sequence Diagram

This diagram shows the temporal flow from CLI invocation through discovery, runtime selection, and execution. Understanding this flow helps with debugging and extending Invowk.

## Main Execution Flow

```mermaid
sequenceDiagram
    autonumber
    participant User
    participant CLI as CLI (cmd.go)
    participant Config as Configuration
    participant Disc as Discovery Engine
    participant CUE as CUE Parser
    participant Registry as Runtime Registry
    participant Runtime as Selected Runtime
    participant Exec as Executor

    User->>CLI: invowk cmd <name> [args...]

    rect rgb(240, 248, 255)
        note right of CLI: Initialization Phase
        CLI->>Config: Load configuration
        Config->>CUE: Parse config.cue
        CUE-->>Config: Config struct
        Config-->>CLI: Configuration loaded
    end

    rect rgb(255, 248, 240)
        note right of CLI: Discovery Phase
        CLI->>Disc: DiscoverCommands(searchPaths)
        Disc->>CUE: Parse invkfile.cue files
        CUE-->>Disc: Invkfile structs
        Disc->>CUE: Parse *.invkmod directories
        CUE-->>Disc: Module structs
        Disc-->>CLI: CommandInfo[] (unified tree)
    end

    rect rgb(240, 255, 240)
        note right of CLI: Resolution Phase
        CLI->>CLI: Match command by name
        CLI->>CLI: Select implementation (platform match)
        CLI->>Registry: GetRuntime(implementation.runtime)
        Registry-->>CLI: Runtime instance
        CLI->>Runtime: Validate(context)
        Runtime-->>CLI: Validation result
    end

    rect rgb(255, 240, 255)
        note right of CLI: Execution Phase
        CLI->>Runtime: Execute(context)
        Runtime->>Exec: Run script/command
        Exec-->>Runtime: Exit code, output
        Runtime-->>CLI: Result
    end

    CLI-->>User: Output + Exit code
```

## Container Runtime Flow (Detailed)

When the container runtime is selected, additional steps occur:

```mermaid
sequenceDiagram
    autonumber
    participant CLI
    participant ContainerRT as Container Runtime
    participant Engine as Engine Abstraction
    participant Provision as Image Provisioner
    participant SSH as SSH Server
    participant TUI as TUI Server
    participant Container as Docker/Podman

    CLI->>ContainerRT: Execute(context)

    rect rgb(255, 248, 240)
        note right of ContainerRT: Provisioning Phase
        ContainerRT->>Engine: Detect available engine
        Engine-->>ContainerRT: docker/podman

        ContainerRT->>Provision: ProvisionImage(baseImage)
        Provision->>Engine: Check if base exists
        alt Base image missing
            Engine->>Container: Pull image
            Container-->>Engine: Image pulled
        end
        Provision->>Provision: Create ephemeral layer
        note right of Provision: Layer contains:<br/>- invowk binary<br/>- invkfiles<br/>- required modules
        Provision->>Engine: Build provisioned image
        Engine->>Container: docker build
        Container-->>Engine: Image built
        Provision-->>ContainerRT: Provisioned image tag
    end

    rect rgb(240, 255, 240)
        note right of ContainerRT: Server Setup (if needed)
        alt enable_host_ssh is true
            ContainerRT->>SSH: Start SSH server
            SSH-->>ContainerRT: Server address + token
        end
        ContainerRT->>TUI: Ensure TUI server running
        TUI-->>ContainerRT: TUI server address
    end

    rect rgb(240, 248, 255)
        note right of ContainerRT: Execution Phase
        ContainerRT->>Engine: Run container
        note right of Engine: Mounts, env vars,<br/>network config
        Engine->>Container: docker run
        Container-->>Engine: Exit code, output
        Engine-->>ContainerRT: Result
    end

    rect rgb(255, 240, 240)
        note right of ContainerRT: Cleanup Phase
        alt SSH server was started
            ContainerRT->>SSH: Stop server
        end
    end

    ContainerRT-->>CLI: Result
```

## Virtual Runtime Flow

The virtual runtime uses the embedded mvdan/sh interpreter:

```mermaid
sequenceDiagram
    autonumber
    participant CLI
    participant VirtualRT as Virtual Runtime
    participant Parser as mvdan/sh Parser
    participant Interp as mvdan/sh Interpreter
    participant Builtins as u-root Builtins
    participant TUI as TUI Server

    CLI->>VirtualRT: Execute(context)

    rect rgb(240, 248, 255)
        note right of VirtualRT: Parse Phase
        VirtualRT->>Parser: Parse script
        Parser-->>VirtualRT: AST
    end

    rect rgb(240, 255, 240)
        note right of VirtualRT: Interpretation Phase
        VirtualRT->>Interp: Run(AST)
        loop For each command
            Interp->>Interp: Evaluate command
            alt Is u-root builtin
                Interp->>Builtins: Execute builtin
                Builtins-->>Interp: Result
            else Is TUI request
                Interp->>TUI: Request TUI component
                TUI-->>Interp: User response
            else Is external command
                Interp->>Interp: Execute via host
            end
        end
        Interp-->>VirtualRT: Exit code
    end

    VirtualRT-->>CLI: Result
```

## Phase Descriptions

### 1. Initialization Phase

| Step | Component | Action |
|------|-----------|--------|
| 1 | CLI | Receive user command |
| 2-4 | Config + CUE | Load and parse `~/.config/invowk/config.cue` |

**Key decisions made:**
- Container engine preference (docker vs podman)
- Search paths for invkfiles/modules
- Default runtime if not specified

### 2. Discovery Phase

| Step | Component | Action |
|------|-----------|--------|
| 5 | Discovery | Start command discovery |
| 6-7 | CUE Parser | Parse all `invkfile.cue` files |
| 8-9 | CUE Parser | Parse all `*.invkmod` directories |
| 10 | Discovery | Build unified command tree |

**Precedence order (highest to lowest):**
1. Current directory `invkfile.cue`
2. Current directory `*.invkmod`
3. User directory `~/.invowk/cmds/`
4. Configured search paths

### 3. Resolution Phase

| Step | Component | Action |
|------|-----------|--------|
| 11 | CLI | Match command name to discovered commands |
| 12 | CLI | Select platform-specific implementation |
| 13-14 | Registry | Get appropriate runtime instance |
| 15-16 | Runtime | Validate execution context |

**Platform matching:**
- Check for `platforms: ["linux"]`, `["darwin"]`, `["windows"]`
- Fall back to default implementation if no platform match

### 4. Execution Phase

| Step | Component | Action |
|------|-----------|--------|
| 17 | Runtime | Begin execution |
| 18-19 | Executor | Run the actual script/command |
| 20 | Runtime | Return result |
| 21 | CLI | Output to user |

**Runtime-specific behavior:**
- **Native**: Spawns host shell process
- **Virtual**: Interprets via mvdan/sh
- **Container**: Provisions image, runs container

## Error Handling Points

```mermaid
flowchart TD
    subgraph "Error Categories"
        E1[Config Parse Error]
        E2[Discovery Error]
        E3[Command Not Found]
        E4[Runtime Unavailable]
        E5[Validation Error]
        E6[Execution Error]
    end

    E1 --> |"Invalid CUE syntax"| Fix1[Check config.cue syntax]
    E2 --> |"Module parse failure"| Fix2[Check invkfile.cue syntax]
    E3 --> |"No matching command"| Fix3[Verify command name, check discovery]
    E4 --> |"e.g., Docker not running"| Fix4[Start container engine]
    E5 --> |"Missing required fields"| Fix5[Check command implementation]
    E6 --> |"Script error"| Fix6[Check script content]
```

## Performance Considerations

| Phase | Typical Duration | Optimization |
|-------|------------------|--------------|
| Initialization | < 10ms | Config cached after first load |
| Discovery | 10-100ms | Depends on number of files/modules |
| Resolution | < 1ms | Simple lookup |
| Execution | Variable | Depends on command |

**Bottlenecks to watch:**
- Many modules in search paths → slower discovery
- Large invkfiles → slower parsing
- Container image pulls → can be minutes

## Related Diagrams

- [C4 Container Diagram](./c4-container.md) - Component relationships
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
- [Discovery Precedence Flowchart](./flowchart-discovery.md) - How commands are discovered
