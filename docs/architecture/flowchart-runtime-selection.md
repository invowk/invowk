# Runtime Selection Flowchart

This diagram documents the decision tree for selecting which runtime executes a command. Understanding this helps users choose the right runtime for their use case.

## Decision Flowchart

```mermaid
flowchart TD
    Start([Command Requested]) --> HasImpl{Has platform-specific<br/>implementation?}
    HasImpl -->|Yes| SelectPlatform[Select matching platform<br/>implementation]
    HasImpl -->|No| UseDefault[Use default implementation]

    SelectPlatform --> RuntimeType{What runtime<br/>type specified?}
    UseDefault --> RuntimeType

    RuntimeType -->|native| CheckNative{Host shell<br/>available?}
    RuntimeType -->|virtual| Virtual[Use mvdan/sh<br/>interpreter]
    RuntimeType -->|container| CheckContainer{Container engine<br/>available?}
    RuntimeType -->|not specified| InferRuntime{Infer from<br/>implementation}

    InferRuntime -->|has container config| CheckContainer
    InferRuntime -->|has script only| CheckNative

    CheckNative -->|Yes| Native[Execute via<br/>bash/sh/PowerShell]
    CheckNative -->|No| FallbackVirtual{Fallback to<br/>virtual?}
    FallbackVirtual -->|Yes| Virtual
    FallbackVirtual -->|No| Error1([Error: No shell found])

    CheckContainer -->|Yes| Provision[Provision ephemeral<br/>image layer]
    CheckContainer -->|No| Error2([Error: No container<br/>engine available])

    Provision --> ImageSource{Image source?}
    ImageSource -->|image specified| PullCheck{Image exists<br/>locally?}
    ImageSource -->|containerfile specified| BuildImage[Build from<br/>Containerfile]

    PullCheck -->|Yes| UseImage[Use existing image]
    PullCheck -->|No| PullImage[Pull image from registry]
    PullImage --> UseImage
    BuildImage --> UseImage

    UseImage --> AddLayer[Add ephemeral layer:<br/>invowk binary + modules]
    AddLayer --> SSHNeeded{enable_host_ssh<br/>= true?}

    SSHNeeded -->|Yes| StartSSH[Start SSH server<br/>for callbacks]
    SSHNeeded -->|No| RunContainer

    StartSSH --> RunContainer[Run container with<br/>mounted volumes]

    Virtual --> Execute([Execute command])
    Native --> Execute
    RunContainer --> Execute
```

## Platform Selection Details

```mermaid
flowchart TD
    subgraph "Platform Matching"
        Impl[Implementation List] --> Check1{Current OS in<br/>platforms list?}
        Check1 -->|Yes| UseIt[Use this implementation]
        Check1 -->|No| Check2{platforms list<br/>empty?}
        Check2 -->|Yes| UseIt
        Check2 -->|No| NextImpl[Try next implementation]
        NextImpl --> Impl
    end
```

### Platform Values

| Platform Value | Matches On |
|---------------|------------|
| `"linux"` | Linux systems |
| `"darwin"` | macOS systems |
| `"windows"` | Windows systems |
| (empty list) | All platforms (default) |

### Example Command Definition

```cue
cmds: {
    build: {
        default: {
            runtime: "native"
            script: "make build"
        }
        implementations: [
            {
                platforms: ["windows"]
                runtime: "native"
                script: "nmake build"
            },
            {
                platforms: ["linux"]
                runtime: "container"
                container: {
                    image: "golang:1.21"
                }
                script: "make build"
            }
        ]
    }
}
```

## Runtime Comparison

| Aspect | Native | Virtual | Container |
|--------|--------|---------|-----------|
| **Speed** | Fastest | Fast | Slower (overhead) |
| **Isolation** | None | Process | Full |
| **Portability** | Platform-dependent | High | Highest |
| **Shell features** | Full host shell | POSIX subset | Full (in container) |
| **Dependencies** | Host shell | None | Docker/Podman |
| **Best for** | Simple scripts | Cross-platform | Complex environments |

## Runtime Availability Checks

### Native Runtime

```mermaid
flowchart LR
    Start[Check Native] --> Unix{Unix-like OS?}
    Unix -->|Yes| FindShell[Find bash or sh]
    Unix -->|No| FindPS[Find PowerShell]
    FindShell --> HasShell{Found?}
    FindPS --> HasPS{Found?}
    HasShell -->|Yes| Available([Available])
    HasShell -->|No| Unavailable([Unavailable])
    HasPS -->|Yes| Available
    HasPS -->|No| Unavailable
```

### Virtual Runtime

```mermaid
flowchart LR
    Start[Check Virtual] --> Always([Always Available])
```

The virtual runtime is always available because it's embedded in the Invowk binary.

### Container Runtime

```mermaid
flowchart LR
    Start[Check Container] --> Pref{Preferred engine<br/>in config?}
    Pref -->|docker| TryDocker[Try Docker]
    Pref -->|podman| TryPodman[Try Podman]
    Pref -->|not set| TryDocker

    TryDocker --> DockerOK{Docker available?}
    DockerOK -->|Yes| UseDocker([Use Docker])
    DockerOK -->|No| TryPodman

    TryPodman --> PodmanOK{Podman available?}
    PodmanOK -->|Yes| UsePodman([Use Podman])
    PodmanOK -->|No| Unavailable([Unavailable])
```

## Container Provisioning Details

When the container runtime is selected, Invowk creates an ephemeral image layer:

```mermaid
flowchart TD
    subgraph "Ephemeral Layer Contents"
        Binary["/usr/local/bin/invowk<br/>(Invowk binary)"]
        Invkfiles["/workspace/invkfiles/<br/>(Command definitions)"]
        Modules["/workspace/modules/<br/>(Required modules)"]
        Scripts["/workspace/scripts/<br/>(Script files)"]
    end

    Base[Base Image] --> Layer[Ephemeral Layer]
    Layer --> Binary
    Layer --> Invkfiles
    Layer --> Modules
    Layer --> Scripts
```

### Why Ephemeral Layers?

1. **No image pollution**: Base images stay clean
2. **Fast iteration**: No full rebuild needed
3. **Portable**: Commands work with any compatible base image
4. **Secure**: Invowk binary is injected, not installed in image

## SSH Server for Callbacks

When `enable_host_ssh: true` is set, Invowk starts a temporary SSH server:

```mermaid
flowchart LR
    subgraph "Container"
        Cmd[Command] --> SSH_Client[SSH Client]
    end

    subgraph "Host"
        SSH_Server[Invowk SSH Server]
        HostCmd[Host Commands]
    end

    SSH_Client -->|Token auth| SSH_Server
    SSH_Server --> HostCmd
```

**Use cases:**
- Accessing host secrets
- Running host-only commands
- File synchronization

## Decision Guidelines

| Use Case | Recommended Runtime | Why |
|----------|-------------------|-----|
| Quick scripts | `native` | Fastest, no overhead |
| Cross-platform commands | `virtual` | Works everywhere |
| CI/CD pipelines | `container` | Reproducible |
| Commands needing specific tools | `container` | Isolated dependencies |
| Interactive TUI | `native` or `virtual` | Better terminal support |

## Related Diagrams

- [Command Execution Sequence](./sequence-execution.md) - Full execution flow
- [C4 Container Diagram](./c4-container.md) - Runtime component relationships
- [Discovery Precedence Flowchart](./flowchart-discovery.md) - How commands are found
