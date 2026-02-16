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
script: """
    echo "Line 1"
    echo "Line 2"
    """`,
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
    implementations: [...]       // Required: how to run the command
    flags?: [...]                // Optional: command flags
    args?: [...]                 // Optional: positional arguments
    env?: #EnvConfig             // Optional: environment config
    workdir?: string             // Optional: working directory
    depends_on?: #DependsOn      // Optional: dependencies
}`,
  },

  'core-concepts/multi-platform-impl': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        // Unix implementation
        {
            script: "make build"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows implementation
        {
            script: "msbuild /p:Configuration=Release"
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  'core-concepts/script-inline': {
    language: 'cue',
    code: `// Single line
script: "echo 'Hello!'"

// Multi-line
script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build ./...
    """`,
  },

  'core-concepts/script-external': {
    language: 'cue',
    code: `// Relative to invowkfile location
script: "./scripts/build.sh"

// Just the filename (recognized extensions)
script: "deploy.sh"`,
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
                script: """
                    echo "Building $APP_NAME..."
                    go build -o bin/$APP_NAME ./...
                    """
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
                script: "./scripts/deploy.sh"
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
            _nativeUnix & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _nativeUnix & {script: "make test"}
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
                // Module-prefixed commands for dependencies
                {alternatives: ["build"]},
                {alternatives: ["com.company.tools lint"]},
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
            script: "go build ./..."
            
            // 2. Target constraints (runtime + platform)
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    ]
}`,
  },

  'implementations/inline-single': {
    language: 'cue',
    code: `script: "echo 'Hello, World!'"`,
  },

  'implementations/inline-multi': {
    language: 'cue',
    code: `script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build -o bin/app ./...
    echo "Done!"
    """`,
  },

  'implementations/runtimes-list': {
    language: 'cue',
    code: `runtimes: [
    {name: "native"},      // System shell
    {name: "virtual"},     // Built-in POSIX shell
    {name: "container", image: "debian:stable-slim"}  // Container
]`,
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
            script: "rm -rf build/"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows implementation
        {
            script: "rmdir /s /q build"
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
            script: "go build ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        },
        // Reproducible container build
        {
            script: "go build -o /workspace/bin/app ./..."
            runtimes: [{name: "container", image: "golang:1.26"}]
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
            script: "echo "Deploying to \$PLATFORM with config at \$CONFIG_PATH""
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
            script: "echo "Deploying to \$PLATFORM with config at \$CONFIG_PATH""
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
            script: "npm run build"
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
            script: "make build"
            runtimes: [{name: "native"}, {name: "virtual"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        {
            script: "msbuild"
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
  build - Build the project [native*, virtual] (linux, macos)
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
            _unixNative & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _unixNative & {script: "make test"}
        ]
    },
    {
        name: "version"
        implementations: [
            _allPlatforms & {script: "cat VERSION"}
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
            script: "make build"
            runtimes: [
                {name: "native"},
                {name: "container", image: "debian:stable-slim"}
            ]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows native only
        {
            script: "msbuild /p:Configuration=Release"
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
        script: """
            # When run with --ivk-interactive, TUI components appear as overlays
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" api cli library)
            
            if invowk tui confirm "Create $TYPE project '$NAME'?"; then
                mkdir -p "$NAME"
                echo "Created $NAME"
            fi
            """
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
        script: """
            # TUI components work inside containers!
            ENV=$(invowk tui choose "Select environment" "dev" "staging" "prod")

            if invowk tui confirm "Deploy to \$ENV?"; then
                ./deploy.sh "\$ENV"
            fi
            """
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
        }]
    }]
}`,
  },

  'interactive/deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy with confirmations"
    implementations: [{
        script: """
            echo "Deploying to production..."

            # This sudo prompt works because we're in interactive mode
            sudo systemctl restart myapp

            echo "Deployment complete!"
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'interactive/db-migrate-example': {
    language: 'cue',
    code: `{
    name: "db migrate"
    description: "Run database migrations"
    implementations: [{
        script: """
            echo "=== Database Migration ==="

            # Interactive confirmation
            if invowk tui confirm "Apply migrations to production?"; then
                # Password prompt works in interactive mode
                psql -h prod-db -U admin -W -f migrations.sql
            fi
            """
        runtimes: [{name: "native"}]
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
    implementations: [...#Implementation] // Required - at least one
    env?:            #EnvConfig           // Optional
    workdir?:        string               // Optional
    depends_on?:     #DependsOn           // Optional
    flags?:          [...#Flag]           // Optional
    args?:           [...#Argument]       // Optional
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
    script:      string       // Required - inline script or file path
    runtimes:    [...#RuntimeConfig] & [_, ...]  // Required - runtime configurations
    platforms:   [...#PlatformConfig] & [_, ...]  // Required - at least one platform
    env?:        #EnvConfig   // Optional
    workdir?:    string       // Optional
    depends_on?: #DependsOn   // Optional
}`,
  },

  'reference/invowkfile/script-examples': {
    language: 'cue',
    code: `// Inline script
script: "echo 'Hello, World!'"

// Multi-line script
script: """
    echo "Building..."
    go build -o app .
    echo "Done!"
    """

// Script file reference
script: "./scripts/build.sh"
script: "deploy.py"`,
  },

  'reference/invowkfile/runtimes-examples': {
    language: 'cue',
    code: `// Native only
runtimes: [{name: "native"}]

// Multiple runtimes
runtimes: [
    {name: "native"},
    {name: "virtual"},
]

// Container with options
runtimes: [{
    name: "container"
    image: "golang:1.26"
    volumes: ["./:/app"]
}]`,
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
    env_inherit_allow?: [...string]
    env_inherit_deny?:  [...string]
}

// Native runtime: supports interpreter
#RuntimeConfigNative: close({
    #RuntimeConfigBase
    name:         "native"
    interpreter?: string  // "auto", "python3", "/usr/bin/env perl -w", etc.
})

// Virtual runtime: no additional fields
#RuntimeConfigVirtual: close({
    #RuntimeConfigBase
    name: "virtual"
    // NOTE: interpreter is NOT allowed here (CUE validation error)
})

// Container runtime: image/containerfile + extras
#RuntimeConfigContainer: close({
    #RuntimeConfigBase
    name:              "container"
    interpreter?:      string
    enable_host_ssh?:  bool
    containerfile?:    string  // mutually exclusive with image
    image?:            string  // mutually exclusive with containerfile
    volumes?:          [...string]
    ports?:            [...string]
})

// Discriminated union of all runtime types
#RuntimeConfig: #RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer`,
  },

  'reference/invowkfile/env-inherit-example': {
    language: 'cue',
    code: `runtimes: [{
    name:              "container"
    image:             "debian:stable-slim"
    env_inherit_mode:  "allow"
    env_inherit_allow: ["TERM", "LANG"]
    env_inherit_deny:  ["AWS_SECRET_ACCESS_KEY"]
}]`,
  },

  'reference/invowkfile/interpreter-examples': {
    language: 'cue',
    code: `// Auto-detect from shebang
interpreter: "auto"

// Specific interpreter
interpreter: "python3"
interpreter: "node"
interpreter: "/usr/bin/ruby"

// With arguments
interpreter: "python3 -u"
interpreter: "/usr/bin/env perl -w"`,
  },

  'reference/invowkfile/enable-host-ssh-example': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "debian:stable-slim"
    enable_host_ssh: true
}]`,
  },

  'reference/invowkfile/containerfile-image-examples': {
    language: 'cue',
    code: `// Use a pre-built image
image: "debian:stable-slim"
image: "golang:1.26"

// Build from a Containerfile
containerfile: "./Containerfile"
containerfile: "./docker/Dockerfile.build"`,
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
    alternatives: [...string]  // At least one - tool names
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
    code: `#CommandDependency: {
    alternatives: [...string]  // Command names
}`,
  },

  'reference/invowkfile/filepath-dependency-structure': {
    language: 'cue',
    code: `#FilepathDependency: {
    alternatives: [...string]  // File/directory paths
    readable?:    bool
    writable?:    bool
    executable?:  bool
}`,
  },

  'reference/invowkfile/capability-dependency-structure': {
    language: 'cue',
    code: `#CapabilityDependency: {
    alternatives: [...("local-area-network" | "internet" | "containers" | "tty")]
}`,
  },

  'reference/invowkfile/env-var-dependency-structure': {
    language: 'cue',
    code: `#EnvVarDependency: {
    alternatives: [...#EnvVarCheck]
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
    check_script:     string  // Script to run
    expected_code?:   int     // Expected exit code (default: 0)
    expected_output?: string  // Regex to match output
}

#CustomCheckAlternatives: {
    alternatives: [...#CustomCheck]
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
                script: """
                    if [ "$INVOWK_FLAG_RELEASE" = "true" ]; then
                        go build -ldflags="-s -w" -o app .
                    else
                        go build -o app .
                    fi
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            },
            {
                script: """
                    $flags = if ($env:INVOWK_FLAG_RELEASE -eq "true") { "-ldflags=-s -w" } else { "" }
                    go build $flags -o app.exe .
                    """
                runtimes: [{name: "native", interpreter: "pwsh"}]
                platforms: [{name: "windows"}]
            },
        ]
        
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
    },
]`,
  },
};
