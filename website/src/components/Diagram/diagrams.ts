/**
 * Centralized Mermaid diagrams for architecture documentation.
 *
 * These diagrams are shared across all translations to:
 * 1. Avoid duplication of diagram code in translation files
 * 2. Ensure consistency when diagrams are updated
 * 3. Keep docs/architecture/*.md as the authoritative source for GitHub rendering
 *
 * NAMING CONVENTION:
 * - architecture/c4-* - C4 Model diagrams
 * - architecture/execution-* - Command execution sequence diagrams
 * - architecture/runtime-* - Runtime selection flowcharts
 * - architecture/discovery-* - Discovery precedence flowcharts
 *
 * SOURCE OF TRUTH:
 * The diagrams in this file should match those in docs/architecture/*.md
 * which is the authoritative source that renders on GitHub.
 */

export interface Diagram {
  /** The Mermaid diagram code */
  code: string;
}

/**
 * All available diagrams organized by documentation section.
 */
export const diagrams = {
  // =============================================================================
  // C4 CONTEXT DIAGRAM (C1)
  // Source: docs/architecture/c4-context.md
  // =============================================================================

  'architecture/c4-context': {
    code: `C4Context
    title System Context Diagram - Invowk

    Person(developer, "Developer", "Uses invowk to run commands defined in invkfiles")
    Person(team_member, "Team Member", "Runs shared commands from invkmod modules")

    System(invowk, "Invowk CLI", "Dynamically extensible command runner supporting native shell, virtual shell, and containerized execution")

    System_Ext(docker, "Docker Engine", "Container runtime for isolated command execution")
    System_Ext(podman, "Podman Engine", "Alternative container runtime (rootless)")
    System_Ext(git, "Git Repositories", "Remote module repositories (GitHub, GitLab)")
    System_Ext(host_shell, "Host Shell", "bash/sh on Unix, PowerShell on Windows")
    SystemDb_Ext(filesystem, "Filesystem", "invkfile.cue, *.invkmod directories, scripts")

    Rel(developer, invowk, "Defines and runs commands", "CLI")
    Rel(team_member, invowk, "Runs shared commands", "CLI")

    Rel(invowk, docker, "Executes container commands", "Docker API/CLI")
    Rel(invowk, podman, "Executes container commands", "Podman CLI")
    Rel(invowk, git, "Fetches remote modules", "Git protocol")
    Rel(invowk, host_shell, "Executes native commands", "exec/spawn")
    Rel(invowk, filesystem, "Reads configs, invkfiles, modules", "File I/O")`,
  },

  // =============================================================================
  // C4 CONTAINER DIAGRAM (C2)
  // Source: docs/architecture/c4-container.md
  // =============================================================================

  'architecture/c4-container': {
    code: `C4Container
    title Container Diagram - Invowk

    Person(user, "User", "Developer or team member")

    System_Ext(docker, "Docker/Podman", "Container engine")
    System_Ext(git, "Git Repositories", "Remote modules")
    System_Ext(host_shell, "Host Shell", "bash/sh/PowerShell")

    Container_Boundary(invowk_cli, "Invowk CLI") {
        Container(cmd, "CLI Commands", "Go/Cobra", "Root, cmd, init, config, module, tui, completion subcommands")
        Container(discovery, "Discovery Engine", "Go", "Locates invkfiles and modules with precedence ordering")
        Container(config, "Configuration Manager", "Go/Viper+CUE", "Loads and validates user configuration")

        Container(runtime_native, "Native Runtime", "Go", "Executes via host shell (bash/sh/PowerShell)")
        Container(runtime_virtual, "Virtual Runtime", "Go/mvdan-sh", "Embedded shell interpreter with u-root builtins")
        Container(runtime_container, "Container Runtime", "Go", "Executes inside Docker/Podman containers")

        Container(container_engine, "Container Engine Abstraction", "Go", "Unified Docker/Podman interface with auto-fallback")
        Container(provision, "Image Provisioner", "Go", "Creates ephemeral layers with invowk binary and modules")

        Container(ssh_server, "SSH Server", "Go/Wish", "Enables container-to-host callbacks via SSH tunneling")
        Container(tui_server, "TUI Server", "Go/Bubble Tea", "HTTP server for TUI requests from child processes")

        Container(cue_parser, "CUE Parser", "Go/cuelang", "3-step parsing: compile schema, unify data, decode")
        Container(module_resolver, "Module Resolver", "Go", "Git-based dependency resolution with caching and lock files")
    }

    ContainerDb(config_file, "Config File", "CUE", "~/.config/invowk/config.cue")
    ContainerDb(invkfiles, "Invkfiles", "CUE", "invkfile.cue command definitions")
    ContainerDb(modules, "Modules", "Directories", "*.invkmod directories with invkmod.cue + invkfile.cue")
    ContainerDb(cache, "Module Cache", "Filesystem", "~/.cache/invowk/modules/")

    Rel(user, cmd, "Runs commands", "CLI")

    Rel(cmd, discovery, "Discovers commands")
    Rel(cmd, config, "Loads configuration")

    Rel(discovery, cue_parser, "Parses invkfiles/invkmods")
    Rel(discovery, invkfiles, "Reads", "File I/O")
    Rel(discovery, modules, "Reads", "File I/O")

    Rel(config, cue_parser, "Parses config")
    Rel(config, config_file, "Reads", "File I/O")

    Rel(cmd, runtime_native, "Executes native commands")
    Rel(cmd, runtime_virtual, "Executes virtual commands")
    Rel(cmd, runtime_container, "Executes container commands")

    Rel(runtime_native, host_shell, "Spawns shell", "exec")
    Rel(runtime_virtual, tui_server, "Requests TUI", "HTTP")

    Rel(runtime_container, container_engine, "Runs containers")
    Rel(runtime_container, provision, "Provisions images")
    Rel(runtime_container, ssh_server, "Starts for callbacks")
    Rel(runtime_container, tui_server, "Requests TUI", "HTTP")

    Rel(container_engine, docker, "Executes", "CLI/API")
    Rel(provision, container_engine, "Builds images")

    Rel(module_resolver, git, "Fetches", "Git protocol")
    Rel(module_resolver, cache, "Caches modules", "File I/O")
    Rel(discovery, module_resolver, "Resolves dependencies")`,
  },

  // =============================================================================
  // EXECUTION FLOW DIAGRAMS
  // Source: docs/architecture/sequence-execution.md
  // =============================================================================

  'architecture/execution-main': {
    code: `sequenceDiagram
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

    CLI-->>User: Output + Exit code`,
  },

  'architecture/execution-container': {
    code: `sequenceDiagram
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

    ContainerRT-->>CLI: Result`,
  },

  'architecture/execution-virtual': {
    code: `sequenceDiagram
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

    VirtualRT-->>CLI: Result`,
  },

  'architecture/execution-errors': {
    code: `flowchart TD
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
    E6 --> |"Script error"| Fix6[Check script content]`,
  },

  // =============================================================================
  // RUNTIME SELECTION DIAGRAMS
  // Source: docs/architecture/flowchart-runtime-selection.md
  // =============================================================================

  'architecture/runtime-decision': {
    code: `flowchart TD
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
    RunContainer --> Execute`,
  },

  'architecture/runtime-platform': {
    code: `flowchart TD
    subgraph "Platform Matching"
        Impl[Implementation List] --> Check1{Current OS in<br/>platforms list?}
        Check1 -->|Yes| UseIt[Use this implementation]
        Check1 -->|No| Check2{platforms list<br/>empty?}
        Check2 -->|Yes| UseIt
        Check2 -->|No| NextImpl[Try next implementation]
        NextImpl --> Impl
    end`,
  },

  'architecture/runtime-native-check': {
    code: `flowchart LR
    Start[Check Native] --> Unix{Unix-like OS?}
    Unix -->|Yes| FindShell[Find bash or sh]
    Unix -->|No| FindPS[Find PowerShell]
    FindShell --> HasShell{Found?}
    FindPS --> HasPS{Found?}
    HasShell -->|Yes| Available([Available])
    HasShell -->|No| Unavailable([Unavailable])
    HasPS -->|Yes| Available
    HasPS -->|No| Unavailable`,
  },

  'architecture/runtime-virtual-check': {
    code: `flowchart LR
    Start[Check Virtual] --> Always([Always Available])`,
  },

  'architecture/runtime-container-check': {
    code: `flowchart LR
    Start[Check Container] --> Pref{Preferred engine<br/>in config?}
    Pref -->|docker| TryDocker[Try Docker]
    Pref -->|podman| TryPodman[Try Podman]
    Pref -->|not set| TryDocker

    TryDocker --> DockerOK{Docker available?}
    DockerOK -->|Yes| UseDocker([Use Docker])
    DockerOK -->|No| TryPodman

    TryPodman --> PodmanOK{Podman available?}
    PodmanOK -->|Yes| UsePodman([Use Podman])
    PodmanOK -->|No| Unavailable([Unavailable])`,
  },

  'architecture/runtime-provision': {
    code: `flowchart TD
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
    Layer --> Scripts`,
  },

  'architecture/runtime-ssh': {
    code: `flowchart LR
    subgraph "Container"
        Cmd[Command] --> SSH_Client[SSH Client]
    end

    subgraph "Host"
        SSH_Server[Invowk SSH Server]
        HostCmd[Host Commands]
    end

    SSH_Client -->|Token auth| SSH_Server
    SSH_Server --> HostCmd`,
  },

  // =============================================================================
  // DISCOVERY DIAGRAMS
  // Source: docs/architecture/flowchart-discovery.md
  // =============================================================================

  'architecture/discovery-flow': {
    code: `flowchart TD
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

    ParsePaths --> Done([Build unified<br/>command tree])`,
  },

  'architecture/discovery-conflict': {
    code: `flowchart TD
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
    style L4 fill:#FFE4B5`,
  },

  'architecture/discovery-module-structure': {
    code: `flowchart TD
    subgraph "Module Structure"
        ModDir["*.invkmod/"] --> InvkmodCue["invkmod.cue<br/>(metadata)"]
        ModDir --> InvkfileCue["invkfile.cue<br/>(commands)"]
        ModDir --> Scripts["scripts/<br/>(optional)"]
        ModDir --> Files["other files<br/>(optional)"]
    end

    subgraph "Module Metadata (invkmod.cue)"
        Meta["module: 'com.example.mymod'<br/>version: '1.0.0'<br/>requires: [...]"]
    end

    InvkmodCue --> Meta`,
  },

  'architecture/discovery-deps': {
    code: `flowchart TD
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

    Note["Only first-level<br/>dependencies are<br/>directly visible"]`,
  },

  'architecture/discovery-search-paths': {
    code: `flowchart LR
    subgraph "search_paths order matters"
        P1["/opt/company-modules"] --> P2["/home/shared"]
        P2 --> P3["...more paths"]
    end

    P1 -->|"Higher priority"| Win[First match wins]
    P3 -->|"Lower priority"| Win`,
  },

  'architecture/discovery-cache': {
    code: `flowchart TD
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
    ConfigChange --> Invalidate`,
  },

  'architecture/discovery-not-found': {
    code: `flowchart TD
    NotFound[Command not found] --> Check1{invkfile.cue<br/>exists?}
    Check1 -->|No| Fix1[Create invkfile.cue]
    Check1 -->|Yes| Check2{Command defined<br/>in cmds?}
    Check2 -->|No| Fix2[Add command to cmds]
    Check2 -->|Yes| Check3{Syntax errors<br/>in CUE?}
    Check3 -->|Yes| Fix3[Fix CUE syntax]
    Check3 -->|No| Check4{Platform<br/>matches?}
    Check4 -->|No| Fix4[Add platform or<br/>use default impl]
    Check4 -->|Yes| Debug[Run with --verbose]`,
  },

  'architecture/discovery-wrong-version': {
    code: `flowchart TD
    Wrong[Wrong command version] --> CheckPriority{Check discovery<br/>sources}
    CheckPriority --> ListSources[List all sources<br/>with same command]
    ListSources --> Identify[Identify which<br/>is being used]
    Identify --> Fix{Resolution}
    Fix -->|"Want local"| Ensure1[Ensure ./invkfile.cue<br/>has the command]
    Fix -->|"Want module"| Ensure2[Remove from<br/>higher-priority sources]
    Fix -->|"Want user"| Ensure3[Remove from<br/>local sources]`,
  },
} as const;

export type DiagramId = keyof typeof diagrams;
