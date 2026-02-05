# Discovery Precedence Flowchart

This diagram shows how commands are discovered and how conflicts are resolved when the same command name appears in multiple locations.

## Discovery Flow

```mermaid
flowchart TD
    Start([Discovery Start]) --> PWD

    subgraph "Priority 1: Current Directory"
        PWD[Check ./invkfile.cue] --> HasPWD{Found?}
        HasPWD -->|Yes| ParsePWD[Parse invkfile.cue<br/>Priority: HIGHEST]
        HasPWD -->|No| Skip1[Continue]
    end

    ParsePWD --> Modules
    Skip1 --> Modules

    subgraph "Priority 2: Local Modules"
        Modules[Scan for ./*.invkmod] --> HasMods{Found modules?}
        HasMods -->|Yes| ParseMods[Parse each module<br/>Check dependencies]
        HasMods -->|No| Skip2[Continue]
    end

    ParseMods --> ResolveDeps
    subgraph "Dependency Resolution"
        ResolveDeps[Resolve module dependencies] --> DepLoop{More deps?}
        DepLoop -->|Yes| FetchDep[Fetch from Git/<br/>cache]
        FetchDep --> ParseDep[Parse dep module]
        ParseDep --> DepLoop
        DepLoop -->|No| DepsResolved[All deps resolved]
    end

    DepsResolved --> UserDir
    Skip2 --> UserDir

    subgraph "Priority 3: User Directory"
        UserDir[Check ~/.invowk/cmds/] --> HasUser{Found?}
        HasUser -->|Yes| ParseUser[Parse user commands]
        HasUser -->|No| Skip3[Continue]
    end

    ParseUser --> SearchPaths
    Skip3 --> SearchPaths

    subgraph "Priority 4: Search Paths"
        SearchPaths[Check configured<br/>search_paths] --> HasPaths{Found?}
        HasPaths -->|Yes| ParsePaths[Parse from search paths<br/>Priority: LOWEST]
        HasPaths -->|No| Done
    end

    ParsePaths --> Done([Build unified<br/>command tree])
```

## Conflict Resolution

When the same command name exists in multiple locations:

```mermaid
flowchart TD
    subgraph "Command 'build' found in multiple locations"
        L1["./invkfile.cue<br/>Priority: 1 (highest)"]
        L2["./mytools.invkmod<br/>Priority: 2"]
        L3["~/.invowk/cmds/<br/>Priority: 3"]
        L4["search_paths<br/>Priority: 4 (lowest)"]
    end

    Conflict{Same command name<br/>in multiple locations}
    Conflict --> Winner["Highest priority wins"]
    Winner --> L1

    style L1 fill:#90EE90
    style L2 fill:#FFE4B5
    style L3 fill:#FFE4B5
    style L4 fill:#FFE4B5
```

### Resolution Rules

| Priority | Source | Example Path |
|----------|--------|--------------|
| 1 (highest) | Current directory invkfile | `./invkfile.cue` |
| 2 | Local modules | `./mytools.invkmod/` |
| 3 | User directory | `~/.invowk/cmds/invkfile.cue` |
| 4 (lowest) | Search paths | `/opt/invowk-modules/` |

**Key principle**: Closer to the working directory = higher priority.

## Module Discovery Details

```mermaid
flowchart TD
    subgraph "Module Structure"
        ModDir["*.invkmod/"] --> InvkmodCue["invkmod.cue<br/>(metadata)"]
        ModDir --> InvkfileCue["invkfile.cue<br/>(commands)"]
        ModDir --> Scripts["scripts/<br/>(optional)"]
        ModDir --> Files["other files<br/>(optional)"]
    end

    subgraph "Module Metadata (invkmod.cue)"
        Meta["module: 'com.example.mymod'<br/>version: '1.0.0'<br/>requires: [...]"]
    end

    InvkmodCue --> Meta
```

### Required Module Fields

```cue
// invkmod.cue
module: "com.example.mymodule"  // RDNS naming convention
version: "1.0.0"                // Semantic version

// Optional
description: "My useful module"
requires: [
    {
        git: "https://github.com/org/repo.git"
        version: "v1.0.0"
    }
]
```

## Dependency Resolution

```mermaid
flowchart TD
    subgraph "Module A requires Module B"
        ModA[Module A] -->|requires| ModB[Module B]
        ModB -->|requires| ModC[Module C]
    end

    subgraph "Visibility Rules"
        CmdsA["A's commands"]
        CmdsB["B's commands"]
        CmdsC["C's commands"]

        CmdsA -->|can call| CmdsB
        CmdsA -.->|CANNOT call| CmdsC
        CmdsB -->|can call| CmdsC
    end

    Note["Only first-level<br/>dependencies are<br/>directly visible"]
```

### Transitive Dependency Visibility

| From | Can Access | Cannot Access |
|------|------------|---------------|
| Module A | A's commands, B's commands | C's commands (transitive) |
| Module B | B's commands, C's commands | - |
| Root invkfile | Own commands, direct deps | Transitive deps |

**Why this restriction?**
- Prevents implicit coupling to transitive dependencies
- Makes dependencies explicit in each module
- Enables dependency upgrades without breaking consumers

## Search Path Configuration

Search paths are configured in `~/.config/invowk/config.cue`:

```cue
search_paths: [
    "/opt/company-invowk-modules",
    "/home/shared/invowk",
]
```

### Path Resolution Order

```mermaid
flowchart LR
    subgraph "search_paths order matters"
        P1["/opt/company-modules"] --> P2["/home/shared"]
        P2 --> P3["...more paths"]
    end

    P1 -->|"Higher priority"| Win[First match wins]
    P3 -->|"Lower priority"| Win
```

## Discovery Caching

```mermaid
flowchart TD
    Request[Discovery Request] --> CacheCheck{Cache valid?}

    CacheCheck -->|Yes| UseCache[Return cached tree]
    CacheCheck -->|No| FullScan[Perform full scan]

    FullScan --> UpdateCache[Update cache]
    UpdateCache --> Return[Return command tree]

    subgraph "Cache Invalidation"
        FileChange[File modification]
        NewModule[New module added]
        ConfigChange[Config change]
    end

    FileChange --> Invalidate[Invalidate cache]
    NewModule --> Invalidate
    ConfigChange --> Invalidate
```

## Common Discovery Issues

### Problem: Command Not Found

```mermaid
flowchart TD
    NotFound[Command not found] --> Check1{invkfile.cue<br/>exists?}
    Check1 -->|No| Fix1[Create invkfile.cue]
    Check1 -->|Yes| Check2{Command defined<br/>in cmds?}
    Check2 -->|No| Fix2[Add command to cmds]
    Check2 -->|Yes| Check3{Syntax errors<br/>in CUE?}
    Check3 -->|Yes| Fix3[Fix CUE syntax]
    Check3 -->|No| Check4{Platform<br/>matches?}
    Check4 -->|No| Fix4[Add platform or<br/>use default impl]
    Check4 -->|Yes| Debug[Run with --verbose]
```

### Problem: Wrong Command Version

```mermaid
flowchart TD
    Wrong[Wrong command version] --> CheckPriority{Check discovery<br/>sources}
    CheckPriority --> ListSources[List all sources<br/>with same command]
    ListSources --> Identify[Identify which<br/>is being used]
    Identify --> Fix{Resolution}
    Fix -->|"Want local"| Ensure1[Ensure ./invkfile.cue<br/>has the command]
    Fix -->|"Want module"| Ensure2[Remove from<br/>higher-priority sources]
    Fix -->|"Want user"| Ensure3[Remove from<br/>local sources]
```

## Debug Commands

```bash
# List all discovered commands with sources
invowk cmd --list --verbose

# Show discovery order and conflicts
invowk internal discovery --debug

# Validate module structure
invowk module validate ./mymodule.invkmod
```

## Related Diagrams

- [Command Execution Sequence](./sequence-execution.md) - What happens after discovery
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
- [C4 Container Diagram](./c4-container.md) - Discovery component context
