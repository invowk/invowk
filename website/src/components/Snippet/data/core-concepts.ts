import type { Snippet } from '../snippets';

export const coreConceptsSnippets = {
  // =============================================================================
  // CORE CONCEPTS
  // =============================================================================

  'core-concepts/cue-basic-syntax': {
    language: 'cue',
    code: `// This is a comment

// Lists use square brackets
cmds: [
    {
        name: "hello"
        description: "A greeting command"
    }
]

// Multi-line strings use triple quotes
script: {content: """
    echo "Line 1"
    echo "Line 2"
    """}`,
  },

  'core-concepts/schema-overview': {
    language: 'cue',
    code: `// Root level
default_shell?: string  // Optional: override default shell
workdir?: string        // Optional: default working directory
env?: #EnvConfig        // Optional: global environment config
depends_on?: #DependsOn // Optional: global dependencies

// Required: at least one command
cmds: [...#Command] & [_, ...]  // at least one required`,
  },

  'core-concepts/command-structure': {
    language: 'cue',
    code: `{
    name: string                 // Required: command name
    description?: string         // Optional: help text
    category?: string            // Optional: group in listing
    implementations: [...]       // Required: how to run the command
    flags?: [...]                // Optional: command flags
    args?: [...]                 // Optional: positional arguments
    env?: #EnvConfig             // Optional: environment config
    workdir?: string             // Optional: working directory
    depends_on?: #DependsOn      // Optional: dependencies
    watch?: #WatchConfig         // Optional: file-watching config
}`,
  },

  'core-concepts/multi-platform-impl': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        // Unix implementation
        {
            script: {content: "make build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows implementation
        {
            script: {content: "msbuild /p:Configuration=Release"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  'core-concepts/script-inline': {
    language: 'cue',
    code: `// Single line
script: {content: "echo 'Hello!'"}

// Multi-line
script: {content: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build ./...
    """}`,
  },

  'core-concepts/script-external': {
    language: 'cue',
    code: `// Root/project invowkfile: invoke a project-local script from inline content
script: {content: "./scripts/build.sh"}

// Module invowkfile: reference a file contained in the invowkmod
script: {file: "scripts/deploy.sh"}`,
  },

  'core-concepts/full-example': {
    language: 'cue',
    code: `// Global environment
env: {
    vars: {
        APP_NAME: "myapp"
        LOG_LEVEL: "info"
    }
}

// Global dependencies
depends_on: {
    tools: [{alternatives: ["sh", "bash"]}]
}

cmds: [
    {
        name: "build"
        description: "Build the application"
        implementations: [
            {
                script: {content: """
                    echo "Building $APP_NAME..."
                    go build -o bin/$APP_NAME ./...
                    """}
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
        depends_on: {
            tools: [{alternatives: ["go"]}]
            filepaths: [{alternatives: ["go.mod"]}]
        }
    },
    {
        name: "deploy"
        description: "Deploy to production"
        implementations: [
            {
                script: {content: "./scripts/deploy.sh"}
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
        depends_on: {
            tools: [{alternatives: ["docker", "podman"]}]
            cmds: [{alternatives: ["build"]}]
        }
        flags: [
            {name: "env", description: "Target environment", required: true},
            {name: "dry-run", description: "Simulate deployment", type: "bool", default_value: "false"}
        ]
    }
]`,
  },

  'core-concepts/cue-template': {
    language: 'cue',
    code: `// Define a template
_nativeUnix: {
    runtimes: [{name: "native"}]
    platforms: [{name: "linux"}, {name: "macos"}]
}

cmds: [
    {
        name: "build"
        implementations: [
            _nativeUnix & {script: {content: "make build"}}
        ]
    },
    {
        name: "test"
        implementations: [
            _nativeUnix & {script: {content: "make test"}}
        ]
    }
]`,
  },

  // =============================================================================
  // COMMANDS AND NAMESPACES
  // =============================================================================

  'commands-namespaces/subcommand-names': {
    language: 'cue',
    code: `cmds: [
    {name: "test"},
    {name: "test unit"},
    {name: "test integration"},
    {name: "db migrate"},
    {name: "db seed"},
]`,
  },

  'commands-namespaces/module-prefix': {
    language: 'cue',
    code: `// invowkmod.cue
module: "com.company.frontend"
version: "1.0.0"

// invowkfile.cue
cmds: [
    {name: "build"},
    {name: "test unit"},
]`,
  },

  'commands-namespaces/valid-module-ids': {
    language: 'cue',
    code: `module: "frontend"
module: "backend"
module: "my.project"
module: "com.company.tools"
module: "io.github.username.cli"`,
  },

  'commands-namespaces/invalid-module-ids': {
    language: 'cue',
    code: `module: "my-project"   // Hyphens not allowed
module: "my_project"   // Underscores not allowed
module: ".project"     // Can't start with dot
module: "project."     // Can't end with dot
module: "my..project"  // No consecutive dots
module: "123project"   // Must start with letter`,
  },

  'commands-namespaces/discovery-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*] (linux, macos, windows)

From tools.invowkmod:
  hello - A greeting [native*] (linux, macos)`,
  },

  'commands-namespaces/command-dependency': {
    language: 'cue',
    code: `cmds: [
    {
        name: "build"
        implementations: [...]
    },
    {
        name: "test"
        implementations: [...]
        depends_on: {
            cmds: [
                // Reference by command name in the same invowkfile
                {alternatives: ["build"]}
            ]
        }
    },
    {
        name: "release"
        implementations: [...]
        depends_on: {
            cmds: [
                // Source-qualified commands for module dependencies
                {alternatives: ["build"]},
                {alternatives: ["@com.company.tools lint"]},
            ]
        }
    }
]`,
  },

  // =============================================================================
  // IMPLEMENTATIONS
  // =============================================================================

  'implementations/basic-structure': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            // 1. The script to run
            script: {content: "go build ./..."}
            
            // 2. Target constraints (runtime + platform)
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    ]
}`,
  },

  'implementations/inline-single': {
    language: 'cue',
    code: `script: {content: "echo 'Hello, World!'"}`,
  },

  'implementations/inline-multi': {
    language: 'cue',
    code: `script: {content: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build -o bin/app ./...
    echo "Done!"
    """}`,
  },

  'implementations/runtimes-list': {
    language: 'cue',
    code: `runtimes: [
    {name: "native"},      // System shell
    {name: "virtual-sh"},     // Built-in POSIX shell
    {name: "container", image: "debian:stable-slim"}  // Container
]
platforms: [{name: "linux"}, {name: "macos"}]`,
  },

  'implementations/platforms-list': {
    language: 'cue',
    code: `runtimes: [{name: "native"}]
platforms: [
    {name: "linux"},
    {name: "macos"},
    {name: "windows"}
]`,
  },

  'implementations/platform-specific': {
    language: 'cue',
    code: `{
    name: "clean"
    implementations: [
        // Unix implementation
        {
            script: {content: "rm -rf build/"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows implementation
        {
            script: {content: "rmdir /s /q build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  'implementations/runtime-specific': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        // Fast native build
        {
            script: {content: "go build ./..."}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        },
        // Reproducible container build
        {
            script: {content: "echo building in container"}
            runtimes: [{name: "container", image: "debian:stable-slim"}]
            platforms: [{name: "linux"}]
        }
    ]
}`,
  },

  'implementations/platform-env': {
    language: 'cue',
    code: `{
    name: "deploy"
    implementations: [
        // Linux implementation with platform-specific env
        {
            script: {content: "echo \\"Deploying to $PLATFORM with config at $CONFIG_PATH\\""
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
            env: {
                vars: {
                    PLATFORM: "Linux"
                    CONFIG_PATH: "/etc/app/config.yaml"
                }
            }
        },
        // macOS implementation with platform-specific env
        {
            script: {content: "echo \\"Deploying to $PLATFORM with config at $CONFIG_PATH\\""
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
            env: {
                vars: {
                    PLATFORM: "macOS"
                    CONFIG_PATH: "/usr/local/etc/app/config.yaml"
                }
            }
        }
    ]
}`,
  },

  'implementations/impl-level-settings': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: {content: "npm run build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]

            // Implementation-specific env
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
            
            // Implementation-specific workdir
            workdir: "./frontend"
            
            // Implementation-specific dependencies
            depends_on: {
                tools: [{alternatives: ["node", "npm"]}]
                filepaths: [{alternatives: ["package.json"]}]
            }
        }
    ]
}`,
  },

  'implementations/selection-example': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: {content: "make build"}
            runtimes: [{name: "native"}, {name: "virtual-sh"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        {
            script: {content: "msbuild"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  'implementations/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*, virtual-sh] (linux, macos)
  clean - Clean artifacts [native*] (linux, macos, windows)
  docker-build - Container build [container*] (linux, macos, windows)`,
  },

  'implementations/cue-templates': {
    language: 'cue',
    code: `// Define reusable templates
_unixNative: {
    runtimes: [{name: "native"}]
    platforms: [{name: "linux"}, {name: "macos"}]
}

_allPlatforms: {
    runtimes: [{name: "native"}]
    platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
}

cmds: [
    {
        name: "build"
        implementations: [
            _unixNative & {script: {content: "make build"}}
        ]
    },
    {
        name: "test"
        implementations: [
            _unixNative & {script: {content: "make test"}}
        ]
    },
    {
        name: "version"
        implementations: [
            _allPlatforms & {script: {content: "cat VERSION"}}
        ]
    }
]`,
  },

  'implementations/combined-platform-runtime': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        // Linux/macOS with multiple runtime options
        {
            script: {content: "make build"}
            runtimes: [
                {name: "native"},
                {name: "container", image: "debian:stable-slim"}
            ]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows native only
        {
            script: {content: "msbuild /p:Configuration=Release"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  // =============================================================================
  // INTERACTIVE MODE
  // =============================================================================

  'interactive/basic-usage': {
    language: 'bash',
    code: `# Run a command in interactive mode
invowk cmd build --ivk-interactive`,
  },

  'interactive/config-enable': {
    language: 'cue',
    code: `// In config file: ~/.config/invowk/config.cue
ui: {
    interactive: true  // Enable by default
}`,
  },

  'interactive/use-cases': {
    language: 'bash',
    code: `# Commands with password prompts
invowk cmd deploy --ivk-interactive

# Commands with sudo
invowk cmd system-update --ivk-interactive

# SSH sessions
invowk cmd remote-shell --ivk-interactive

# Any command with interactive input
invowk cmd database-cli --ivk-interactive`,
  },

  'interactive/embedded-tui': {
    language: 'cue',
    code: `{
    name: "interactive-setup"
    description: "Setup with embedded TUI prompts"
    implementations: [{
        script: {content: """
            # When run with --ivk-interactive, TUI components appear as overlays
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" api cli library)
            
            if invowk tui confirm "Create $TYPE project '$NAME'?"; then
                mkdir -p "$NAME"
                echo "Created $NAME"
            fi
            """}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'interactive/key-bindings-executing': {
    language: 'text',
    code: `During command execution:
  All keys      → Forwarded to running command
  Ctrl+\        → Emergency quit (force exit)`,
  },

  'interactive/key-bindings-completed': {
    language: 'text',
    code: `After command completion:
  ↑/k           → Scroll up one line
  ↓/j           → Scroll down one line
  PgUp/b        → Scroll up half page
  PgDown/f/Space→ Scroll down half page
  Home/g        → Go to top
  End/G         → Go to bottom
  q/Esc/Enter   → Exit and return to terminal`,
  },

  'interactive/container-tui-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    implementations: [{
        script: {content: """
            # TUI components work inside containers!
            ENV=$(invowk tui choose "Select environment" "dev" "staging" "prod")

            if invowk tui confirm "Deploy to \$ENV?"; then
                ./deploy.sh "\$ENV"
            fi
            """}
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
        }]
        platforms: [{name: "linux"}]
    }]
}`,
  },

  'interactive/deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy with confirmations"
    implementations: [{
        script: {content: """
            echo "Deploying to production..."

            # This sudo prompt works because we're in interactive mode
            sudo systemctl restart myapp

            echo "Deployment complete!"
            """}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'interactive/db-migrate-example': {
    language: 'cue',
    code: `{
    name: "db migrate"
    description: "Run database migrations"
    implementations: [{
        script: {content: """
            echo "=== Database Migration ==="

            # Interactive confirmation
            if invowk tui confirm "Apply migrations to production?"; then
                # Password prompt works in interactive mode
                psql -h prod-db -U admin -W -f migrations.sql
            fi
            """}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  // =============================================================================
  // REFERENCE - INVKFILE SCHEMA
  // =============================================================================

  'reference/invowkfile/root-structure': {
    language: 'cue',
    code: `#Invowkfile: {
    default_shell?: string    // Optional - override default shell
    workdir?:       string    // Optional - default working directory
    env?:           #EnvConfig      // Optional - global environment
    depends_on?:    #DependsOn      // Optional - global dependencies
    cmds:           [...#Command]   // Required - at least one command
}`,
  },

  'reference/invowkfile/default-shell-example': {
    language: 'cue',
    code: `default_shell: "/bin/bash"
default_shell: "pwsh"`,
  },

  'reference/invowkfile/workdir-example': {
    language: 'cue',
    code: `workdir: "./src"
workdir: "/opt/app"`,
  },

  'reference/invowkfile/command-structure': {
    language: 'cue',
    code: `#Command: {
    name:            string               // Required
    description?:    string               // Optional
    category?:       string               // Optional - groups in listing
    implementations: [...#Implementation] // Required - at least one
    env?:            #EnvConfig           // Optional
    workdir?:        string               // Optional
    depends_on?:     #DependsOn           // Optional
    flags?:          [...#Flag]           // Optional
    args?:           [...#Argument]       // Optional
    watch?:          #WatchConfig         // Optional - file-watching
}`,
  },

  'reference/invowkfile/command-name-examples': {
    language: 'cue',
    code: `name: "build"
name: "test unit"     // Spaces allowed for subcommand-like behavior
name: "deploy-prod"`,
  },

  'reference/invowkfile/command-description-example': {
    language: 'cue',
    code: `description: "Build the application for production"`,
  },

  'reference/invowkfile/implementation-structure': {
    language: 'cue',
    code: `#Implementation: {
    script:      #ImplementationScript       // Required - content or file
    runtimes:    [...#RuntimeConfig] & [_, ...]  // Required - runtime configurations
    platforms:   [...#PlatformConfig] & [_, ...]  // Required - at least one platform
    env?:        #EnvConfig   // Optional
    workdir?:    string       // Optional
    depends_on?: #DependsOn   // Optional
    timeout?:    #DurationString  // Optional - max execution time
}`,
  },

  'reference/invowkfile/script-examples': {
    language: 'cue',
    code: `// Inline script
script: {content: "echo 'Hello, World!'"}

// Multi-line script
script: {content: """
    echo "Building..."
    go build -o app .
    echo "Done!"
    """}

// Explicit interpreter on inline script
script: {content: "print('hello')", interpreter: "python3"}

// Module-contained script file reference
script: {file: "scripts/build.sh"}
script: {file: "scripts/deploy.py", interpreter: "python3"}`,
  },

  'reference/invowkfile/runtimes-examples': {
    language: 'cue',
    code: `// Native only
runtimes: [{name: "native"}]
platforms: [{name: "linux"}, {name: "macos"}]

// Multiple runtimes
runtimes: [
    {name: "native"},
    {name: "virtual-sh"},
]
platforms: [{name: "linux"}, {name: "macos"}]

// Container with options
runtimes: [{
    name: "container"
    image: "debian:stable-slim"
    volumes: ["./:/app"]
}]
platforms: [{name: "linux"}]`,
  },

  'reference/invowkfile/platforms-example': {
    language: 'cue',
    code: `// Linux and macOS only
platforms: [
    {name: "linux"},
    {name: "macos"},
]`,
  },

  'reference/invowkfile/runtime-config-structure': {
    language: 'cue',
    code: `// Shared base fields (all runtimes)
#RuntimeConfigBase: {
    name:               #RuntimeType
    env_inherit_mode?:  "none" | "allow" | "all"
    env_inherit_allow?: [...string]  // Requires env_inherit_mode: "allow"
    env_inherit_deny?:  [...string]
}

// Native runtime: no additional fields
#RuntimeConfigNative: close({
    #RuntimeConfigBase
    name: "native"
})

// Virtual-sh runtime
#RuntimeConfigVirtualSh: close({
    #RuntimeConfigBase
    name: "virtual-sh"
    allowed_binaries?: [...string]
    binary_lookup_mode?: "host" | "strict"
})

// Virtual-lua runtime
#RuntimeConfigVirtualLua: close({
    #RuntimeConfigBase
    name: "virtual-lua"
    allowed_binaries?: [...string]
    binary_lookup_mode?: "host" | "strict"
    cpu_limit?: uint
    memory_limit?: string
})

// Container runtime: exactly one source + extras
#RuntimeConfigContainerBase: {
    #RuntimeConfigBase
    name:              "container"
    enable_host_ssh?:  bool
    volumes?:          [...string]
    ports?:            [...string]
    depends_on?:       #DependsOn  // validated inside the container environment
}

#RuntimeConfigContainerWithImage: close({
    #RuntimeConfigContainerBase
    image:          string
    containerfile?: _|_
})

#RuntimeConfigContainerWithContainerfile: close({
    #RuntimeConfigContainerBase
    containerfile: string
    image?:        _|_
})

#RuntimeConfigContainer: #RuntimeConfigContainerWithImage | #RuntimeConfigContainerWithContainerfile

// Discriminated union of all runtime types
#RuntimeConfig: #RuntimeConfigNative | #RuntimeConfigVirtualSh | #RuntimeConfigVirtualLua | #RuntimeConfigContainer`,
  },

  'reference/invowkfile/env-inherit-example': {
    language: 'cue',
    code: `runtimes: [{
    name:              "container"
    image:             "debian:stable-slim"
    env_inherit_mode:  "allow"
    env_inherit_allow: ["TERM", "LANG"]
    env_inherit_deny:  ["AWS_SECRET_ACCESS_KEY"]
}]
platforms: [{name: "linux"}]`,
  },

  'reference/invowkfile/interpreter-examples': {
    language: 'cue',
    code: `// Auto-detect from shebang
script: {content: """
    #!/usr/bin/env python3
    print("hello")
    """, interpreter: "auto"}

// Specific interpreter
script: {content: "print('hello')", interpreter: "python3"}
script: {content: "console.log('hello')", interpreter: "node"}
script: {content: "puts 'hello'", interpreter: "/usr/bin/ruby"}

// With arguments
script: {content: "print('hello')", interpreter: "python3 -u"}
script: {content: "print qq(hello\\n)", interpreter: "/usr/bin/env perl -w"}`,
  },

  'reference/invowkfile/enable-host-ssh-example': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "debian:stable-slim"
    enable_host_ssh: true
}]
platforms: [{name: "linux"}]`,
  },

  'reference/invowkfile/containerfile-image-examples': {
    language: 'cue',
    code: `// Use a pre-built image
image: "debian:stable-slim"

// Build from a Containerfile
containerfile: "./Containerfile"
containerfile: "./docker/Dockerfile.build"`,
  },

  'reference/invowkfile/persistent-container-example': {
    language: 'cue',
    code: `persistent: {
    create_if_missing: true
    name: "myproject-build"  // Optional portable Docker/Podman name
}`,
  },

  'reference/invowkfile/volumes-example': {
    language: 'cue',
    code: `volumes: [
    "./src:/app/src",
    "/tmp:/tmp:ro",
    "\${HOME}/.cache:/cache",
]`,
  },

  'reference/invowkfile/ports-example': {
    language: 'cue',
    code: `ports: [
    "8080:80",
    "3000:3000",
]`,
  },

  'reference/invowkfile/platform-config-structure': {
    language: 'cue',
    code: `#PlatformConfig: {
    name: "linux" | "macos" | "windows"
    virtual?: {
        filesystem?: {
            access?: "restricted" | "full"
            paths?: [string]: string  // Uppercase logical names, e.g. CACHE
        }
    }
}`,
  },

  'reference/invowkfile/env-config-structure': {
    language: 'cue',
    code: `#EnvConfig: {
    files?: [...string]         // Dotenv files to load
    vars?:  [string]: string    // Environment variables
}`,
  },

  'reference/invowkfile/env-files-example': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",
        ".env.local",
        ".env.\${ENVIRONMENT}?",  // '?' means optional
    ]
}`,
  },

  'reference/invowkfile/env-vars-example': {
    language: 'cue',
    code: `env: {
    vars: {
        NODE_ENV: "production"
        DEBUG: "false"
    }
}`,
  },

  'reference/invowkfile/depends-on-structure': {
    language: 'cue',
    code: `#DependsOn: {
    tools?:         [...#ToolDependency]
    cmds?:          [...#CommandDependency]
    filepaths?:     [...#FilepathDependency]
    capabilities?:  [...#CapabilityDependency]
    custom_checks?: [...#CustomCheckDependency]
    env_vars?:      [...#EnvVarDependency]
}`,
  },

  'reference/invowkfile/tool-dependency-structure': {
    language: 'cue',
    code: `#ToolDependency: {
    alternatives: [...string] & [_, ...]  // At least one - tool names
}`,
  },

  'reference/invowkfile/tool-dependency-example': {
    language: 'cue',
    code: `depends_on: {
    tools: [
        {alternatives: ["go"]},
        {alternatives: ["podman", "docker"]},  // Either works
    ]
}`,
  },

  'reference/invowkfile/command-dependency-structure': {
    language: 'cue',
    code: `#CommandDependencyRef: #BareCommandDependencyRef | #SourceQualifiedCommandDependencyRef
#BareCommandDependencyRef: string
#SourceQualifiedCommandDependencyRef: string  // @source command

#CommandDependency: {
    alternatives: [...#CommandDependencyRef] & [_, ...]
}`,
  },

  'reference/invowkfile/filepath-dependency-structure': {
    language: 'cue',
    code: `#FilepathDependency: {
    alternatives: [...string] & [_, ...]  // File/directory paths
    readable?:    bool
    writable?:    bool
    executable?:  bool
}`,
  },

  'reference/invowkfile/capability-dependency-structure': {
    language: 'cue',
    code: `#CapabilityDependency: {
    alternatives: [...("local-area-network" | "internet" | "containers" | "tty")] & [_, ...]
}`,
  },

  'reference/invowkfile/env-var-dependency-structure': {
    language: 'cue',
    code: `#EnvVarDependency: {
    alternatives: [...#EnvVarCheck] & [_, ...]
}

#EnvVarCheck: {
    name:        string    // Environment variable name
    validation?: string    // Regex pattern
}`,
  },

  'reference/invowkfile/custom-check-dependency-structure': {
    language: 'cue',
    code: `#CustomCheckDependency: #CustomCheck | #CustomCheckAlternatives

#CustomCheck: {
    name:             string  // Check identifier
    script:           #CustomCheckScript
    expected_code?:   int     // Expected exit code (default: 0)
    expected_output?: string  // Regex to match output
}

#CustomCheckScript: close({
    content:      string
    file?:        _|_
    interpreter?: string
}) | close({
    file:         string
    content?:     _|_
    interpreter?: string
})

#CustomCheckAlternatives: {
    alternatives: [...#CustomCheck] & [_, ...]
}`,
  },

  'reference/invowkfile/flag-structure': {
    language: 'cue',
    code: `#Flag: {
    name:          string    // POSIX-compliant name
    description:   string    // Help text
    default_value?: string   // Default value
    type?:         "string" | "bool" | "int" | "float"
    required?:     bool
    short?:        string    // Single character alias
    validation?:   string    // Regex pattern
}`,
  },

  'reference/invowkfile/flag-example': {
    language: 'cue',
    code: `flags: [
    {
        name: "output"
        short: "o"
        description: "Output file path"
        default_value: "./build"
    },
    {
        name: "verbose"
        short: "v"
        description: "Enable verbose output"
        type: "bool"
    },
]`,
  },

  'reference/invowkfile/argument-structure': {
    language: 'cue',
    code: `#Argument: {
    name:          string    // POSIX-compliant name
    description:   string    // Help text
    required?:     bool      // Must be provided
    default_value?: string   // Default if not provided
    type?:         "string" | "int" | "float"
    validation?:   string    // Regex pattern
    variadic?:     bool      // Accepts multiple values (last arg only)
}`,
  },

  'reference/invowkfile/argument-example': {
    language: 'cue',
    code: `args: [
    {
        name: "target"
        description: "Build target"
        required: true
    },
    {
        name: "files"
        description: "Files to process"
        variadic: true
    },
]`,
  },

  'reference/invowkfile/complete-example': {
    language: 'cue',
    code: `env: {
    files: [".env"]
    vars: {
        APP_NAME: "myapp"
    }
}

cmds: [
    {
        name: "build"
        description: "Build the application"
        
        flags: [
            {
                name: "release"
                short: "r"
                description: "Build for release"
                type: "bool"
            },
        ]
        
        implementations: [
            {
                script: {content: """
                    if [ "$INVOWK_FLAG_RELEASE" = "true" ]; then
                        go build -ldflags="-s -w" -o app .
                    else
                        go build -o app .
                    fi
                    """}
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            },
            {
                script: {
                    content: """
                    $flags = if ($env:INVOWK_FLAG_RELEASE -eq "true") { "-ldflags=-s -w" } else { "" }
                    go build $flags -o app.exe .
                    """
                    interpreter: "pwsh"
                }
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            },
        ]
        
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
    },
]`,
  },

  // =============================================================================
  // NEW SCHEMA FIELDS (category, timeout, watch, duration)
  // =============================================================================

  'reference/invowkfile/category-example': {
    language: 'cue',
    code: `cmds: [
    {
        name: "build"
        category: "Development"
        implementations: [...]
    },
    {
        name: "test unit"
        category: "Development"
        implementations: [...]
    },
    {
        name: "deploy"
        category: "Operations"
        implementations: [...]
    },
]`,
  },

  'reference/invowkfile/timeout-example': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: {content: "make build"}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
        timeout: "5m"
    }]
}`,
  },

  'reference/invowkfile/duration-string-type': {
    language: 'cue',
    code: `// #DurationString — shared by timeout and debounce
// Valid examples: "500ms", "30s", "5m", "1h30m", "2.5s"
#DurationString: string & =~"^([0-9]+(\\\\.[0-9]+)?(ns|us|\u00b5s|ms|s|m|h))+$"`,
  },

  'reference/invowkfile/watch-config-structure': {
    language: 'cue',
    code: `#WatchConfig: {
    patterns:      [...string] & [_, ...] // Required - glob patterns to watch
    debounce?:     #DurationString        // Optional - default "500ms"
    clear_screen?: bool                   // Optional - default false
    ignore?:       [...string]            // Optional - merged with built-in defaults
}`,
  },

  'reference/invowkfile/watch-config-example': {
    language: 'cue',
    code: `{
    name: "dev"
    description: "Run development server with auto-reload"
    watch: {
        patterns: ["src/**/*.go", "*.go"]
        debounce: "1s"
        clear_screen: true
        ignore: ["vendor/**"]
    }
    implementations: [{
        script: {content: "go run ./cmd/server"}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },
} satisfies Record<string, Snippet>;
