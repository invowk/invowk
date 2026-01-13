/**
 * Centralized code snippets for documentation.
 *
 * These snippets are shared across all translations to:
 * 1. Avoid duplication of code blocks in translation files
 * 2. Ensure consistency when code examples are updated
 * 3. Reduce translation maintenance burden
 *
 * GUIDELINES FOR ADDING SNIPPETS:
 * - Use descriptive, hierarchical IDs: "category/subcategory/name"
 * - Keep code examples minimal and focused
 * - Comments in CUE code should be in English (they're part of the code)
 * - For translatable comments, consider using separate snippets
 *
 * NAMING CONVENTION:
 * - getting-started/* - Snippets for getting-started section
 * - core-concepts/* - Snippets for core-concepts section
 * - runtime-modes/* - Snippets for runtime-modes section
 * - dependencies/* - Snippets for dependencies section
 * - environment/* - Snippets for environment section
 * - flags-args/* - Snippets for flags-and-arguments section
 * - advanced/* - Snippets for advanced section
 * - packs/* - Snippets for packs section
 * - tui/* - Snippets for TUI section
 * - config/* - Snippets for configuration section
 * - cli/* - Snippets for CLI commands and output
 */

export interface Snippet {
  /** The programming language for syntax highlighting */
  language: string;
  /** The code content */
  code: string;
}

/**
 * All available snippets organized by documentation section.
 */
export const snippets = {
  // =============================================================================
  // GETTING STARTED
  // =============================================================================

  'getting-started/invkfile-basic-structure': {
    language: 'cue',
    code: `group: "myproject"           // Required: namespace for your commands
version: "1.0"               // Optional: version of this invkfile
description: "My commands"   // Optional: what this file is about

cmds: [                  // Required: list of commands
    // ... your commands here
]`,
  },

  'getting-started/go-project-full': {
    language: 'cue',
    code: `group: "goproject"
version: "1.0"
description: "Commands for my Go project"

cmds: [
    // Simple build command
    {
        name: "build"
        description: "Build the project"
        implementations: [
            {
                script: """
                    echo "Building..."
                    go build -o bin/app ./...
                    echo "Done! Binary at bin/app"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Test command with subcommand-style naming
    {
        name: "test unit"
        description: "Run unit tests"
        implementations: [
            {
                script: "go test -v ./..."
                target: {
                    runtimes: [{name: "native"}, {name: "virtual"}]
                }
            }
        ]
    },

    // Test with coverage
    {
        name: "test coverage"
        description: "Run tests with coverage"
        implementations: [
            {
                script: """
                    go test -coverprofile=coverage.out ./...
                    go tool cover -html=coverage.out -o coverage.html
                    echo "Coverage report: coverage.html"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Clean command
    {
        name: "clean"
        description: "Remove build artifacts"
        implementations: [
            {
                script: "rm -rf bin/ coverage.out coverage.html"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]`,
  },

  'getting-started/runtimes-multiple': {
    language: 'cue',
    code: `runtimes: [{name: "native"}, {name: "virtual"}]`,
  },

  'getting-started/platforms-linux-macos': {
    language: 'cue',
    code: `platforms: [{name: "linux"}, {name: "macos"}]`,
  },

  'getting-started/build-with-deps': {
    language: 'cue',
    code: `{
    name: "build"
    description: "Build the project"
    implementations: [
        {
            script: """
                echo "Building..."
                go build -o bin/app ./...
                echo "Done!"
                """
            target: {
                runtimes: [{name: "native"}]
            }
        }
    ]
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"], readable: true}
        ]
    }
}`,
  },

  'getting-started/env-vars-levels': {
    language: 'cue',
    code: `group: "goproject"

// Root-level env applies to ALL commands
env: {
    vars: {
        GO111MODULE: "on"
    }
}

cmds: [
    {
        name: "build"
        // Command-level env applies to this command
        env: {
            vars: {
                CGO_ENABLED: "0"
            }
        }
        implementations: [
            {
                script: "go build -o bin/app ./..."
                // Implementation-level env is most specific
                env: {
                    vars: {
                        GOOS: "linux"
                        GOARCH: "amd64"
                    }
                }
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]`,
  },

  // =============================================================================
  // CLI COMMANDS
  // =============================================================================

  'cli/list-commands': {
    language: 'bash',
    code: `invowk cmd --list`,
  },

  'cli/run-command': {
    language: 'bash',
    code: `invowk cmd goproject build`,
  },

  'cli/run-subcommands': {
    language: 'bash',
    code: `invowk cmd goproject test unit
invowk cmd goproject test coverage`,
  },

  'cli/runtime-override': {
    language: 'bash',
    code: `# Use the default (native)
invowk cmd goproject test unit

# Explicitly use virtual runtime
invowk cmd goproject test unit --runtime virtual`,
  },

  'cli/cue-validate': {
    language: 'bash',
    code: `cue vet invkfile.cue path/to/invkfile_schema.cue -d '#Invkfile'`,
  },

  // =============================================================================
  // CLI OUTPUT EXAMPLES
  // =============================================================================

  'cli/output-list-commands': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  goproject build - Build the project [native*]
  goproject test unit - Run unit tests [native*, virtual]
  goproject test coverage - Run tests with coverage [native*]
  goproject clean - Remove build artifacts [native*] (linux, macos)`,
  },

  'cli/output-deps-not-satisfied': {
    language: 'text',
    code: `✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.`,
  },

  // =============================================================================
  // CORE CONCEPTS
  // =============================================================================

  'core-concepts/cue-basic-syntax': {
    language: 'cue',
    code: `// This is a comment
group: "myproject"
version: "1.0"

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
group: string           // Required: namespace prefix
version?: string        // Optional: invkfile version (e.g., "1.0")
description?: string    // Optional: what this file is about
default_shell?: string  // Optional: override default shell
workdir?: string        // Optional: default working directory
env?: #EnvConfig        // Optional: global environment config
depends_on?: #DependsOn // Optional: global dependencies

// Required: at least one command
cmds: [...#Command]`,
  },

  'core-concepts/group-namespace': {
    language: 'cue',
    code: `group: "myproject"

cmds: [
    {name: "build"},
    {name: "test"},
]`,
  },

  'core-concepts/rdns-naming': {
    language: 'cue',
    code: `group: "com.company.devtools"
group: "io.github.username.project"`,
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
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows implementation
        {
            script: "msbuild /p:Configuration=Release"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
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
    code: `// Relative to invkfile location
script: "./scripts/build.sh"

// Just the filename (recognized extensions)
script: "deploy.sh"`,
  },

  'core-concepts/full-example': {
    language: 'cue',
    code: `group: "myapp"
version: "1.0"
description: "Build and deployment commands for MyApp"

// Global environment
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
                target: {
                    runtimes: [{name: "native"}]
                }
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
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
        depends_on: {
            tools: [{alternatives: ["docker", "podman"]}]
            cmds: [{alternatives: ["myapp build"]}]
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
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
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
  // RUNTIME MODES
  // =============================================================================

  'runtime-modes/native-basic': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            target: {
                runtimes: [{name: "native"}]
            }
        }
    ]
}`,
  },

  'runtime-modes/virtual-basic': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            echo "Building..."
            go build -o bin/app ./...
            echo "Done!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}`,
  },

  'runtime-modes/virtual-run': {
    language: 'bash',
    code: `invowk cmd myproject build --runtime virtual`,
  },

  'runtime-modes/virtual-cross-platform': {
    language: 'cue',
    code: `{
    name: "setup"
    implementations: [{
        script: """
            # This works the same everywhere!
            if [ -d "node_modules" ]; then
                echo "Dependencies already installed"
            else
                echo "Installing dependencies..."
                npm install
            fi
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}`,
  },

  'runtime-modes/virtual-uroot-config': {
    language: 'cue',
    code: `// In your config file
virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'runtime-modes/virtual-variables': {
    language: 'cue',
    code: `script: """
    NAME="World"
    echo "Hello, $NAME!"
    
    # Parameter expansion
    echo "\${NAME:-default}"
    echo "\${#NAME}"  # Length
    """`,
  },

  'runtime-modes/virtual-conditionals': {
    language: 'cue',
    code: `script: """
    if [ "$ENV" = "production" ]; then
        echo "Production mode"
    elif [ "$ENV" = "staging" ]; then
        echo "Staging mode"
    else
        echo "Development mode"
    fi
    """`,
  },

  'runtime-modes/virtual-loops': {
    language: 'cue',
    code: `script: """
    # For loop
    for file in *.go; do
        echo "Processing $file"
    done
    
    # While loop
    count=0
    while [ $count -lt 5 ]; do
        echo "Count: $count"
        count=$((count + 1))
    done
    """`,
  },

  'runtime-modes/virtual-functions': {
    language: 'cue',
    code: `script: """
    greet() {
        echo "Hello, $1!"
    }
    
    greet "World"
    greet "Invowk"
    """`,
  },

  'runtime-modes/virtual-subshells': {
    language: 'cue',
    code: `script: """
    # Command substitution
    current_date=$(date +%Y-%m-%d)
    echo "Today is $current_date"
    
    # Subshell
    (cd /tmp && echo "In temp: $(pwd)")
    echo "Still in: $(pwd)"
    """`,
  },

  'runtime-modes/virtual-external-commands': {
    language: 'cue',
    code: `script: """
    # Calls the real 'go' binary
    go version
    
    # Calls the real 'git' binary
    git status
    """`,
  },

  'runtime-modes/virtual-env-vars': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [{
        script: """
            echo "Building in $BUILD_MODE mode"
            go build -ldflags="-s -w" ./...
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}`,
  },

  'runtime-modes/virtual-args': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            # Using environment variable
            echo "Hello, $INVOWK_ARG_NAME!"
            
            # Or positional parameter
            echo "Hello, $1!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}`,
  },

  'runtime-modes/virtual-no-interpreter': {
    language: 'cue',
    code: `// This will NOT work with virtual runtime!
{
    name: "bad-example"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            print("This won't work!")
            """
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERROR: Not supported
            }]
        }
    }]
}`,
  },

  'runtime-modes/virtual-bash-limitations': {
    language: 'cue',
    code: `// These won't work in virtual runtime:
script: """
    # Bash arrays (use $@ instead)
    declare -a arr=(1 2 3)  # Not supported
    
    # Bash-specific parameter expansion
    \${var^^}  # Uppercase - not supported
    \${var,,}  # Lowercase - not supported
    
    # Process substitution
    diff <(cmd1) <(cmd2)  # Not supported
    """`,
  },

  'runtime-modes/virtual-deps': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [
            // These will be checked in the virtual shell environment
            {alternatives: ["go"]},
            {alternatives: ["git"]}
        ]
    }
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}`,
  },

  'runtime-modes/virtual-config': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue (Linux)
// ~/Library/Application Support/invowk/config.cue (macOS)
// %APPDATA%\\invowk\\config.cue (Windows)

virtual_shell: {
    // Enable additional utilities from u-root
    enable_uroot_utils: true
}`,
  },

  'runtime-modes/container-basic': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "go build -o /workspace/bin/app ./..."
            target: {
                runtimes: [{
                    name: "container"
                    image: "golang:1.21"
                }]
            }
        }
    ]
}`,
  },

  'runtime-modes/container-volumes': {
    language: 'cue',
    code: `target: {
    runtimes: [{
        name: "container"
        image: "golang:1.21"
        volumes: [
            "./src:/workspace/src",
            "./data:/data:ro"
        ]
    }]
}`,
  },

  'runtime-modes/container-env': {
    language: 'cue',
    code: `target: {
    runtimes: [{
        name: "container"
        image: "node:18"
        env: {
            NODE_ENV: "production"
            DEBUG: "app:*"
        }
    }]
}`,
  },

  'runtime-modes/container-workdir': {
    language: 'cue',
    code: `target: {
    runtimes: [{
        name: "container"
        image: "python:3.11"
        workdir: "/app"
    }]
}`,
  },

  'runtime-modes/container-ssh': {
    language: 'cue',
    code: `target: {
    runtimes: [{
        name: "container"
        image: "debian:bookworm-slim"
        enable_host_ssh: true
    }]
}`,
  },

  // =============================================================================
  // DEPENDENCIES
  // =============================================================================

  'dependencies/tools-basic': {
    language: 'cue',
    code: `depends_on: {
    tools: [
        {alternatives: ["go"]}
    ]
}`,
  },

  'dependencies/tools-alternatives': {
    language: 'cue',
    code: `depends_on: {
    tools: [
        {alternatives: ["docker", "podman"]},
        {alternatives: ["node", "nodejs"]}
    ]
}`,
  },

  'dependencies/filepaths-basic': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["go.mod"]}
    ]
}`,
  },

  'dependencies/filepaths-options': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["config.yaml", "config.json"], readable: true},
        {alternatives: ["./output"], writable: true},
        {alternatives: [".env"], must_exist: true}
    ]
}`,
  },

  'dependencies/commands-basic': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        {alternatives: ["myapp build"]}
    ]
}`,
  },

  'dependencies/commands-alternatives': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        // Either command being discoverable satisfies this dependency
        {alternatives: ["myproject build debug", "myproject build release"]},
    ]
}`,
  },

  'dependencies/commands-multiple': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        {alternatives: ["myproject build"]},
        {alternatives: ["myproject test unit", "myproject test integration"]},
    ]
}`,
  },

  'dependencies/commands-cross-invkfile': {
    language: 'cue',
    code: `depends_on: {
    cmds: [{alternatives: ["shared generate-types"]}]
}`,
  },

  'dependencies/commands-workflow': {
    language: 'bash',
    code: `invowk cmd myproject build && invowk cmd myproject deploy`,
  },

  'dependencies/capabilities-basic': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["network"]},
        {alternatives: ["root", "sudo"]}
    ]
}`,
  },

  'dependencies/env-vars-basic': {
    language: 'cue',
    code: `depends_on: {
    env_vars: [
        {alternatives: ["API_KEY"]},
        {alternatives: ["DATABASE_URL", "DB_URL"]}
    ]
}`,
  },

  'dependencies/custom-checks': {
    language: 'cue',
    code: `depends_on: {
    custom: [
        {
            alternatives: [{
                name: "docker-running"
                description: "Docker daemon must be running"
                check: "docker info > /dev/null 2>&1"
            }]
        }
    ]
}`,
  },

  // =============================================================================
  // ENVIRONMENT
  // =============================================================================

  'environment/env-files': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",           // Required - fails if missing
        ".env.local?",    // Optional - suffix with ?
        ".env.\${ENV}?",   // Interpolation - uses ENV variable
    ]
}`,
  },

  'environment/env-vars': {
    language: 'cue',
    code: `env: {
    vars: {
        API_URL: "https://api.example.com"
        DEBUG: "true"
        VERSION: "1.0.0"
    }
}`,
  },

  'environment/env-combined': {
    language: 'cue',
    code: `env: {
    files: [".env"]
    vars: {
        // These override values from .env
        OVERRIDE_VALUE: "from-invkfile"
    }
}`,
  },

  // Overview page snippets
  'environment/overview-quick-example': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        // Load from .env files
        files: [".env", ".env.local?"]  // ? means optional
        
        // Set variables directly
        vars: {
            NODE_ENV: "production"
            BUILD_DATE: "$(date +%Y-%m-%d)"
        }
    }
    implementations: [{
        script: """
            echo "Building for $NODE_ENV"
            echo "Date: $BUILD_DATE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/scope-root': {
    language: 'cue',
    code: `group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
    }
}

cmds: [...]  // All commands get PROJECT_NAME`,
  },

  'environment/scope-command': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [...]
}`,
  },

  'environment/scope-implementation': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
        }
    ]
}`,
  },

  'environment/scope-platform': {
    language: 'cue',
    code: `implementations: [{
    script: "echo $CONFIG_PATH"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {
                name: "linux"
                env: {CONFIG_PATH: "/etc/myapp/config"}
            },
            {
                name: "macos"
                env: {CONFIG_PATH: "/usr/local/etc/myapp/config"}
            }
        ]
    }
}]`,
  },

  'environment/cli-overrides': {
    language: 'bash',
    code: `# Set a single variable
invowk cmd myproject build --env-var NODE_ENV=development

# Set multiple variables
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true

# Load from a file
invowk cmd myproject build --env-file .env.local

# Combine
invowk cmd myproject build --env-file .env.local -E OVERRIDE=value`,
  },

  'environment/container-env': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            BUILD_ENV: "container"
        }
    }
    implementations: [{
        script: "echo $BUILD_ENV"  // Available inside container
        target: {
            runtimes: [{name: "container", image: "debian:bookworm-slim"}]
        }
    }]
}`,
  },

  // Env Files page snippets
  'environment/env-files-basic': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        files: [".env"]
    }
    implementations: [...]
}`,
  },

  'environment/dotenv-example': {
    language: 'bash',
    code: `# .env
API_KEY=secret123
DATABASE_URL=postgres://localhost/mydb
DEBUG=false`,
  },

  'environment/dotenv-format': {
    language: 'bash',
    code: `# Comments start with #
KEY=value

# Quoted values (spaces preserved)
MESSAGE="Hello World"
PATH_WITH_SPACES='/path/to/my file'

# Empty value
EMPTY_VAR=

# No value (same as empty)
NO_VALUE

# Multiline (use quotes)
MULTILINE="line1
line2
line3"`,
  },

  'environment/env-files-optional': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",           // Required - error if missing
        ".env.local?",    // Optional - ignored if missing
        ".env.secrets?",  // Optional
    ]
}`,
  },

  'environment/env-files-order': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",           // Base config
        ".env.\${ENV}?",   // Environment-specific overrides
        ".env.local?",    // Local overrides (highest priority)
    ]
}`,
  },

  'environment/env-files-order-example': {
    language: 'bash',
    code: `# .env
API_URL=http://localhost:3000
DEBUG=true

# .env.production
API_URL=https://api.example.com
DEBUG=false

# .env.local (developer override)
DEBUG=true`,
  },

  'environment/env-files-path-resolution': {
    language: 'text',
    code: `project/
├── invkfile.cue
├── .env                  # files: [".env"]
├── config/
│   └── .env.prod         # files: ["config/.env.prod"]
└── src/`,
  },

  'environment/env-files-interpolation': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",
        ".env.\${NODE_ENV}?",    // Uses NODE_ENV value
        ".env.\${USER}?",        // Uses current user
    ]
}`,
  },

  'environment/env-files-interpolation-result': {
    language: 'bash',
    code: `# If NODE_ENV=production, loads:
# - .env
# - .env.production (if exists)
# - .env.john (if exists and USER=john)`,
  },

  'environment/env-files-scope-root': {
    language: 'cue',
    code: `group: "myproject"

env: {
    files: [".env"]  // Loaded for all commands
}

cmds: [...]`,
  },

  'environment/env-files-scope-command': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        files: [".env.build"]  // Only for this command
    }
    implementations: [...]
}`,
  },

  'environment/env-files-scope-implementation': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                files: [".env.node"]  // Only for this implementation
            }
        }
    ]
}`,
  },

  'environment/env-files-cli-override': {
    language: 'bash',
    code: `# Load extra file
invowk cmd myproject build --env-file .env.custom

# Short form
invowk cmd myproject build -e .env.custom

# Multiple files
invowk cmd myproject build -e .env.custom -e .env.secrets`,
  },

  'environment/env-files-dev-prod': {
    language: 'cue',
    code: `{
    name: "start"
    env: {
        files: [
            ".env",                    // Base config
            ".env.\${NODE_ENV:-dev}?",  // Environment-specific
            ".env.local?",             // Local overrides
        ]
    }
    implementations: [{
        script: "node server.js"
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-files-secrets': {
    language: 'cue',
    code: `{
    name: "deploy"
    env: {
        files: [
            ".env",                    // Non-sensitive config
            ".env.secrets?",           // Sensitive - not in git
        ]
    }
    implementations: [{
        script: """
            echo "Deploying with API_KEY..."
            ./deploy.sh
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/gitignore-env': {
    language: 'text',
    code: `.env.secrets
.env.local`,
  },

  'environment/env-files-multi-env-structure': {
    language: 'text',
    code: `project/
├── invkfile.cue
├── .env                  # Shared defaults
├── .env.development      # Dev settings
├── .env.staging          # Staging settings
└── .env.production       # Production settings`,
  },

  'environment/env-files-multi-env': {
    language: 'cue',
    code: `{
    name: "deploy"
    env: {
        files: [
            ".env",
            ".env.\${DEPLOY_ENV}",  // DEPLOY_ENV must be set
        ]
    }
    depends_on: {
        env_vars: [
            {alternatives: [{name: "DEPLOY_ENV", validation: "^(development|staging|production)$"}]}
        ]
    }
    implementations: [...]
}`,
  },

  // Env Vars page snippets
  'environment/env-vars-basic': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            NODE_ENV: "production"
            API_URL: "https://api.example.com"
            DEBUG: "false"
        }
    }
    implementations: [{
        script: """
            echo "Building for $NODE_ENV"
            echo "API: $API_URL"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-vars-syntax': {
    language: 'cue',
    code: `vars: {
    // Simple values
    NAME: "value"
    
    // Numbers (still strings in shell)
    PORT: "3000"
    TIMEOUT: "30"
    
    // Boolean-like (strings "true"/"false")
    DEBUG: "true"
    VERBOSE: "false"
    
    // Paths
    CONFIG_PATH: "/etc/myapp/config.yaml"
    OUTPUT_DIR: "./dist"
    
    // URLs
    API_URL: "https://api.example.com"
}`,
  },

  'environment/env-vars-references': {
    language: 'cue',
    code: `vars: {
    // Use system variable
    HOME_CONFIG: "\${HOME}/.config/myapp"
    
    // With default value
    LOG_LEVEL: "\${LOG_LEVEL:-info}"
    
    // Combine variables
    FULL_PATH: "\${HOME}/projects/\${PROJECT_NAME}"
}`,
  },

  'environment/env-vars-scope-root': {
    language: 'cue',
    code: `group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
        VERSION: "1.0.0"
    }
}

cmds: [
    {
        name: "build"
        // Gets PROJECT_NAME and VERSION
        implementations: [...]
    },
    {
        name: "deploy"
        // Also gets PROJECT_NAME and VERSION
        implementations: [...]
    }
]`,
  },

  'environment/env-vars-scope-command': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [...]
}`,
  },

  'environment/env-vars-scope-implementation': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
        },
        {
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    CGO_ENABLED: "0"
                }
            }
        }
    ]
}`,
  },

  'environment/env-vars-scope-platform': {
    language: 'cue',
    code: `implementations: [{
    script: "echo $PLATFORM_CONFIG"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {
                name: "linux"
                env: {
                    PLATFORM_CONFIG: "/etc/myapp"
                    PLATFORM_NAME: "Linux"
                }
            },
            {
                name: "macos"
                env: {
                    PLATFORM_CONFIG: "/usr/local/etc/myapp"
                    PLATFORM_NAME: "macOS"
                }
            },
            {
                name: "windows"
                env: {
                    PLATFORM_CONFIG: "%APPDATA%\\\\myapp"
                    PLATFORM_NAME: "Windows"
                }
            }
        ]
    }
}]`,
  },

  'environment/env-vars-combined-files': {
    language: 'cue',
    code: `env: {
    files: [".env"]  // Loaded first
    vars: {
        // These override .env values
        OVERRIDE: "from-invkfile"
    }
}`,
  },

  'environment/env-vars-cli-override': {
    language: 'bash',
    code: `# Single variable
invowk cmd myproject build --env-var NODE_ENV=development

# Short form
invowk cmd myproject build -E NODE_ENV=development

# Multiple variables
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true -E PORT=8080`,
  },

  'environment/env-vars-build-config': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            NODE_ENV: "production"
            BUILD_TARGET: "es2020"
            SOURCEMAP: "false"
        }
    }
    implementations: [{
        script: "npm run build"
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-vars-api-config': {
    language: 'cue',
    code: `{
    name: "start"
    env: {
        vars: {
            API_HOST: "0.0.0.0"
            API_PORT: "3000"
            API_PREFIX: "/api/v1"
            CORS_ORIGIN: "*"
        }
    }
    implementations: [{
        script: "go run ./cmd/server"
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-vars-dynamic': {
    language: 'cue',
    code: `{
    name: "release"
    env: {
        vars: {
            // Git-based version
            GIT_SHA: "$(git rev-parse --short HEAD)"
            GIT_BRANCH: "$(git branch --show-current)"
            
            // Timestamp
            BUILD_TIME: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            
            // Combine values
            BUILD_ID: "\${GIT_BRANCH}-\${GIT_SHA}"
        }
    }
    implementations: [{
        script: """
            echo "Building $BUILD_ID at $BUILD_TIME"
            go build -ldflags="-X main.version=$BUILD_ID" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-vars-database': {
    language: 'cue',
    code: `{
    name: "db migrate"
    env: {
        vars: {
            DB_HOST: "\${DB_HOST:-localhost}"
            DB_PORT: "\${DB_PORT:-5432}"
            DB_NAME: "\${DB_NAME:-myapp}"
            DB_USER: "\${DB_USER:-postgres}"
            // Construct URL from parts
            DATABASE_URL: "postgres://\${DB_USER}@\${DB_HOST}:\${DB_PORT}/\${DB_NAME}"
        }
    }
    implementations: [{
        script: "migrate -database $DATABASE_URL up"
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'environment/env-vars-container': {
    language: 'cue',
    code: `{
    name: "build"
    env: {
        vars: {
            GOOS: "linux"
            GOARCH: "amd64"
            CGO_ENABLED: "0"
        }
    }
    implementations: [{
        script: "go build -o /workspace/bin/app ./..."
        target: {
            runtimes: [{name: "container", image: "golang:1.21"}]
        }
    }]
}`,
  },

  // Precedence page snippets
  'environment/precedence-hierarchy': {
    language: 'text',
    code: `CLI (highest priority)
├── --env-var KEY=value
└── --env-file .env.local
    │
Implementation Level
├── env.vars
└── env.files
    │
Command Level
├── env.vars
└── env.files
    │
Root Level
├── env.vars
└── env.files
    │
System Environment (lowest from invkfile)
│
Platform-specific env`,
  },

  'environment/precedence-invkfile': {
    language: 'cue',
    code: `group: "myproject"

// Root level
env: {
    files: [".env"]
    vars: {
        API_URL: "http://root.example.com"
        LOG_LEVEL: "info"
    }
}

cmds: [
    {
        name: "build"
        // Command level
        env: {
            files: [".env.build"]
            vars: {
                API_URL: "http://command.example.com"
                BUILD_MODE: "development"
            }
        }
        implementations: [{
            script: "echo $API_URL $LOG_LEVEL $BUILD_MODE $NODE_ENV"
            target: {runtimes: [{name: "native"}]}
            // Implementation level
            env: {
                vars: {
                    BUILD_MODE: "production"
                    NODE_ENV: "production"
                }
            }
        }]
    }
]`,
  },

  'environment/precedence-env-files': {
    language: 'bash',
    code: `# .env
API_URL=http://envfile.example.com
DATABASE_URL=postgres://localhost/db

# .env.build
BUILD_MODE=release
CACHE_DIR=./cache`,
  },

  'environment/precedence-result': {
    language: 'bash',
    code: `API_URL=http://command.example.com    # From command vars
LOG_LEVEL=info                         # From root vars
BUILD_MODE=production                  # From implementation vars
NODE_ENV=production                    # From implementation vars
DATABASE_URL=postgres://localhost/db   # From .env file
CACHE_DIR=./cache                      # From .env.build file`,
  },

  'environment/precedence-cli-override': {
    language: 'bash',
    code: `invowk cmd myproject build --env-var API_URL=http://cli.example.com`,
  },

  'environment/precedence-vars-over-files': {
    language: 'cue',
    code: `env: {
    files: [".env"]  // API_URL=from-file
    vars: {
        API_URL: "from-vars"  // This wins
    }
}`,
  },

  'environment/precedence-multiple-files': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",           // API_URL=base
        ".env.local",     // API_URL=local (wins)
    ]
}`,
  },

  'environment/precedence-platform': {
    language: 'cue',
    code: `implementations: [{
    script: "echo $CONFIG_PATH"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {name: "linux", env: {CONFIG_PATH: "/etc/app"}}
            {name: "macos", env: {CONFIG_PATH: "/usr/local/etc/app"}}
        ]
    }
    env: {
        vars: {
            OTHER_VAR: "value"
            // CONFIG_PATH not set here
        }
    }
}]`,
  },

  'environment/precedence-appropriate-levels': {
    language: 'cue',
    code: `// Root: shared across all commands
env: {
    vars: {
        PROJECT_NAME: "myapp"
        VERSION: "1.0.0"
    }
}

// Command: specific to this command
{
    name: "build"
    env: {
        vars: {
            BUILD_TARGET: "production"
        }
    }
}

// Implementation: specific to this runtime
implementations: [{
    target: {runtimes: [{name: "container", image: "node:20"}]}
    env: {
        vars: {
            NODE_OPTIONS: "--max-old-space-size=4096"
        }
    }
}]`,
  },

  'environment/precedence-override-pattern': {
    language: 'cue',
    code: `env: {
    files: [".env"]              // Defaults
    vars: {
        OVERRIDE_THIS: "value"   // Specific override
    }
}`,
  },

  'environment/precedence-local-dev': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",          // Committed defaults
        ".env.local?",   // Not committed, personal overrides
    ]
}`,
  },

  'environment/precedence-cli-temp': {
    language: 'bash',
    code: `# Quick test with different config
invowk cmd myproject build -E DEBUG=true -E LOG_LEVEL=debug`,
  },

  'environment/precedence-debug': {
    language: 'cue',
    code: `{
    name: "debug-env"
    implementations: [{
        script: """
            echo "API_URL=$API_URL"
            echo "LOG_LEVEL=$LOG_LEVEL"
            echo "BUILD_MODE=$BUILD_MODE"
            env | sort
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  // =============================================================================
  // FLAGS AND ARGUMENTS
  // =============================================================================

  'flags-args/flags-basic': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {name: "env", description: "Target environment", required: true},
        {name: "dry-run", description: "Simulate deployment", type: "bool", default_value: "false"},
        {name: "replicas", description: "Number of replicas", type: "int", default_value: "3"}
    ]
    implementations: [
        {
            script: """
                echo "Deploying to $INVOWK_FLAG_ENV with $INVOWK_FLAG_REPLICAS replicas"
                if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                    echo "(dry run - no changes made)"
                fi
                """
            target: {runtimes: [{name: "native"}]}
        }
    ]
}`,
  },

  'flags-args/args-basic': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "name", description: "Name to greet", required: true},
        {name: "title", description: "Optional title", required: false, default_value: ""}
    ]
    implementations: [
        {
            script: """
                if [ -n "$INVOWK_ARG_TITLE" ]; then
                    echo "Hello, $INVOWK_ARG_TITLE $INVOWK_ARG_NAME!"
                else
                    echo "Hello, $INVOWK_ARG_NAME!"
                fi
                """
            target: {runtimes: [{name: "native"}]}
        }
    ]
}`,
  },

  // Overview page snippets
  'flags-args/overview-example-command': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to an environment"
    
    // Flags - named options
    flags: [
        {name: "dry-run", type: "bool", default_value: "false"},
        {name: "replicas", type: "int", default_value: "1"},
    ]
    
    // Arguments - positional values
    args: [
        {name: "environment", required: true},
        {name: "services", variadic: true},
    ]
    
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENVIRONMENT"
            echo "Replicas: $INVOWK_FLAG_REPLICAS"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Services: $INVOWK_ARG_SERVICES"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/overview-usage': {
    language: 'bash',
    code: `invowk cmd myproject deploy production api web --dry-run --replicas=3`,
  },

  'flags-args/overview-flags-example': {
    language: 'cue',
    code: `flags: [
    {name: "verbose", type: "bool", short: "v"},
    {name: "output", type: "string", short: "o", default_value: "./dist"},
    {name: "count", type: "int", default_value: "1"},
]`,
  },

  'flags-args/overview-flags-usage': {
    language: 'bash',
    code: `# Long form
invowk cmd myproject build --verbose --output=./build --count=5

# Short form
invowk cmd myproject build -v -o=./build`,
  },

  'flags-args/overview-args-example': {
    language: 'cue',
    code: `args: [
    {name: "source", required: true},
    {name: "destination", default_value: "./output"},
    {name: "files", variadic: true},
]`,
  },

  'flags-args/overview-args-usage': {
    language: 'bash',
    code: `invowk cmd myproject copy ./src ./dest file1.txt file2.txt`,
  },

  'flags-args/overview-shell-positional': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name"},
        {name: "last-name"},
    ]
    implementations: [{
        script: """
            # Using environment variables
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            
            # Or using positional parameters
            echo "Hello, $1 $2!"
            
            # All arguments
            echo "All: $@"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/overview-mixing': {
    language: 'bash',
    code: `# All equivalent
invowk cmd myproject deploy production --dry-run api web
invowk cmd myproject deploy --dry-run production api web
invowk cmd myproject deploy production api web --dry-run`,
  },

  'flags-args/overview-help': {
    language: 'bash',
    code: `invowk cmd myproject deploy --help`,
  },

  'flags-args/overview-help-output': {
    language: 'text',
    code: `Usage:
  invowk cmd myproject deploy <environment> [services]... [flags]

Arguments:
  environment          (required) - Target environment
  services             (optional) (variadic) - Services to deploy

Flags:
      --dry-run          Perform a dry run (default: false)
  -n, --replicas int     Number of replicas (default: 1)
  -h, --help             help for deploy`,
  },

  // Flags page snippets
  'flags-args/flags-defining': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {
            name: "environment"
            description: "Target environment"
            required: true
        },
        {
            name: "dry-run"
            description: "Simulate without changes"
            type: "bool"
            default_value: "false"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        }
    ]
    implementations: [...]
}`,
  },

  'flags-args/flags-type-string': {
    language: 'cue',
    code: `{name: "message", description: "Custom message", type: "string"}
// or simply
{name: "message", description: "Custom message"}`,
  },

  'flags-args/flags-type-string-usage': {
    language: 'bash',
    code: `invowk cmd myproject run --message="Hello World"`,
  },

  'flags-args/flags-type-bool': {
    language: 'cue',
    code: `{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"}`,
  },

  'flags-args/flags-type-bool-usage': {
    language: 'bash',
    code: `# Enable
invowk cmd myproject run --verbose
invowk cmd myproject run --verbose=true

# Disable (explicit)
invowk cmd myproject run --verbose=false`,
  },

  'flags-args/flags-type-int': {
    language: 'cue',
    code: `{name: "count", description: "Number of iterations", type: "int", default_value: "5"}`,
  },

  'flags-args/flags-type-int-usage': {
    language: 'bash',
    code: `invowk cmd myproject run --count=10
invowk cmd myproject run --count=-1  # Negative allowed`,
  },

  'flags-args/flags-type-float': {
    language: 'cue',
    code: `{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.95"}`,
  },

  'flags-args/flags-type-float-usage': {
    language: 'bash',
    code: `invowk cmd myproject run --threshold=0.8
invowk cmd myproject run --threshold=1.5e-3  # Scientific notation`,
  },

  'flags-args/flags-required': {
    language: 'cue',
    code: `{
    name: "target"
    description: "Deployment target"
    required: true  // Must be provided
}`,
  },

  'flags-args/flags-required-usage': {
    language: 'bash',
    code: `# Error: missing required flag
invowk cmd myproject deploy
# Error: flag 'target' is required

# Success
invowk cmd myproject deploy --target=production`,
  },

  'flags-args/flags-optional': {
    language: 'cue',
    code: `{
    name: "timeout"
    description: "Request timeout in seconds"
    type: "int"
    default_value: "30"  // Used if not provided
}`,
  },

  'flags-args/flags-optional-usage': {
    language: 'bash',
    code: `# Uses default (30)
invowk cmd myproject request

# Override
invowk cmd myproject request --timeout=60`,
  },

  'flags-args/flags-short-aliases': {
    language: 'cue',
    code: `flags: [
    {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output file", short: "o"},
    {name: "force", description: "Force overwrite", type: "bool", short: "f"},
]`,
  },

  'flags-args/flags-short-usage': {
    language: 'bash',
    code: `# Long form
invowk cmd myproject build --verbose --output=./dist --force

# Short form
invowk cmd myproject build -v -o=./dist -f

# Mixed
invowk cmd myproject build -v --output=./dist -f`,
  },

  'flags-args/flags-validation': {
    language: 'cue',
    code: `flags: [
    {
        name: "env"
        description: "Environment name"
        validation: "^(dev|staging|prod)$"
        default_value: "dev"
    },
    {
        name: "version"
        description: "Semantic version"
        validation: "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+$"
    }
]`,
  },

  'flags-args/flags-validation-usage': {
    language: 'bash',
    code: `# Valid
invowk cmd myproject deploy --env=prod --version=1.2.3

# Invalid - fails before execution
invowk cmd myproject deploy --env=production
# Error: flag 'env' value 'production' does not match required pattern '^(dev|staging|prod)$'`,
  },

  'flags-args/flags-accessing': {
    language: 'cue',
    code: `{
    name: "deploy"
    flags: [
        {name: "env", description: "Environment", required: true},
        {name: "dry-run", description: "Dry run", type: "bool", default_value: "false"},
        {name: "replica-count", description: "Replicas", type: "int", default_value: "1"},
    ]
    implementations: [{
        script: """
            echo "Environment: $INVOWK_FLAG_ENV"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Replicas: $INVOWK_FLAG_REPLICA_COUNT"
            
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                echo "Would deploy $INVOWK_FLAG_REPLICA_COUNT replicas to $INVOWK_FLAG_ENV"
            else
                ./deploy.sh "$INVOWK_FLAG_ENV" "$INVOWK_FLAG_REPLICA_COUNT"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/flags-build-example': {
    language: 'cue',
    code: `{
    name: "build"
    description: "Build the application"
    flags: [
        {name: "mode", description: "Build mode", validation: "^(debug|release)$", default_value: "debug"},
        {name: "output", description: "Output directory", short: "o", default_value: "./build"},
        {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
        {name: "parallel", description: "Parallel jobs", type: "int", short: "j", default_value: "4"},
    ]
    implementations: [{
        script: """
            mkdir -p "$INVOWK_FLAG_OUTPUT"
            
            VERBOSE=""
            if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
                VERBOSE="-v"
            fi
            
            go build $VERBOSE -o "$INVOWK_FLAG_OUTPUT/app" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/flags-deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to cloud"
    flags: [
        {
            name: "env"
            description: "Target environment"
            short: "e"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "version"
            description: "Version to deploy"
            short: "v"
            validation: "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+$"
        },
        {
            name: "dry-run"
            description: "Simulate deployment"
            type: "bool"
            short: "n"
            default_value: "false"
        },
        {
            name: "timeout"
            description: "Deployment timeout (seconds)"
            type: "int"
            default_value: "300"
        }
    ]
    implementations: [{
        script: """
            echo "Deploying version \${INVOWK_FLAG_VERSION:-latest} to $INVOWK_FLAG_ENV"
            
            ARGS="--timeout=$INVOWK_FLAG_TIMEOUT"
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                ARGS="$ARGS --dry-run"
            fi
            
            ./scripts/deploy.sh "$INVOWK_FLAG_ENV" $ARGS
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/flags-syntax': {
    language: 'bash',
    code: `# Equals sign
--output=./dist

# Space separator
--output ./dist

# Short with equals
-o=./dist

# Short with value
-o ./dist

# Boolean toggle (enables)
--verbose
-v

# Boolean explicit
--verbose=true
--verbose=false`,
  },

  // Positional arguments page snippets
  'flags-args/args-defining': {
    language: 'cue',
    code: `{
    name: "copy"
    args: [
        {
            name: "source"
            description: "Source file or directory"
            required: true
        },
        {
            name: "destination"
            description: "Destination path"
            required: true
        }
    ]
    implementations: [...]
}`,
  },

  'flags-args/args-defining-usage': {
    language: 'bash',
    code: `invowk cmd myproject copy ./src ./dest`,
  },

  'flags-args/args-type-string': {
    language: 'cue',
    code: `{name: "filename", description: "File to process", type: "string"}`,
  },

  'flags-args/args-type-int': {
    language: 'cue',
    code: `{name: "count", description: "Number of items", type: "int", default_value: "10"}`,
  },

  'flags-args/args-type-int-usage': {
    language: 'bash',
    code: `invowk cmd myproject generate 5`,
  },

  'flags-args/args-type-float': {
    language: 'cue',
    code: `{name: "ratio", description: "Scaling ratio", type: "float", default_value: "1.0"}`,
  },

  'flags-args/args-type-float-usage': {
    language: 'bash',
    code: `invowk cmd myproject scale 0.5`,
  },

  'flags-args/args-required': {
    language: 'cue',
    code: `args: [
    {name: "input", description: "Input file", required: true},
    {name: "output", description: "Output file", required: true},
]`,
  },

  'flags-args/args-required-usage': {
    language: 'bash',
    code: `# Error: missing required argument
invowk cmd myproject convert input.txt
# Error: argument 'output' is required

# Success
invowk cmd myproject convert input.txt output.txt`,
  },

  'flags-args/args-optional': {
    language: 'cue',
    code: `args: [
    {name: "input", description: "Input file", required: true},
    {name: "format", description: "Output format", default_value: "json"},
]`,
  },

  'flags-args/args-optional-usage': {
    language: 'bash',
    code: `# Uses default format (json)
invowk cmd myproject parse input.txt

# Override format
invowk cmd myproject parse input.txt yaml`,
  },

  'flags-args/args-ordering': {
    language: 'cue',
    code: `// Good
args: [
    {name: "input", required: true},      // Required first
    {name: "output", required: true},     // Required second
    {name: "format", default_value: "json"}, // Optional last
]

// Bad - will cause validation error
args: [
    {name: "format", default_value: "json"}, // Optional can't come first
    {name: "input", required: true},
]`,
  },

  'flags-args/args-variadic': {
    language: 'cue',
    code: `{
    name: "process"
    args: [
        {name: "output", description: "Output file", required: true},
        {name: "inputs", description: "Input files", variadic: true},
    ]
    implementations: [{
        script: """
            echo "Output: $INVOWK_ARG_OUTPUT"
            echo "Inputs: $INVOWK_ARG_INPUTS"
            echo "Count: $INVOWK_ARG_INPUTS_COUNT"
            
            for i in $(seq 1 $INVOWK_ARG_INPUTS_COUNT); do
                eval "file=\\$INVOWK_ARG_INPUTS_$i"
                echo "Processing: $file"
            done
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-variadic-usage': {
    language: 'bash',
    code: `invowk cmd myproject process out.txt a.txt b.txt c.txt
# Output: out.txt
# Inputs: a.txt b.txt c.txt
# Count: 3
# Processing: a.txt
# Processing: b.txt
# Processing: c.txt`,
  },

  'flags-args/args-validation': {
    language: 'cue',
    code: `args: [
    {
        name: "environment"
        description: "Target environment"
        required: true
        validation: "^(dev|staging|prod)$"
    },
    {
        name: "version"
        description: "Version number"
        validation: "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+$"
    }
]`,
  },

  'flags-args/args-validation-usage': {
    language: 'bash',
    code: `# Valid
invowk cmd myproject deploy prod 1.2.3

# Invalid
invowk cmd myproject deploy production
# Error: argument 'environment' value 'production' does not match pattern '^(dev|staging|prod)$'`,
  },

  'flags-args/args-accessing': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name", required: true},
        {name: "last-name", default_value: "User"},
    ]
    implementations: [{
        script: """
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-positional-params': {
    language: 'cue',
    code: `{
    name: "copy"
    args: [
        {name: "source", required: true},
        {name: "dest", required: true},
    ]
    implementations: [{
        script: """
            # Using environment variables
            cp "$INVOWK_ARG_SOURCE" "$INVOWK_ARG_DEST"
            
            # Or positional parameters
            cp "$1" "$2"
            
            # All arguments
            echo "Args: $@"
            echo "Count: $#"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-convert-example': {
    language: 'cue',
    code: `{
    name: "convert"
    description: "Convert file format"
    args: [
        {
            name: "input"
            description: "Input file"
            required: true
        },
        {
            name: "output"
            description: "Output file"
            required: true
        },
        {
            name: "format"
            description: "Output format"
            default_value: "json"
            validation: "^(json|yaml|toml|xml)$"
        }
    ]
    implementations: [{
        script: """
            echo "Converting $1 to $2 as $3"
            ./converter --input="$1" --output="$2" --format="$3"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-compress-example': {
    language: 'cue',
    code: `{
    name: "compress"
    description: "Compress files into archive"
    args: [
        {
            name: "archive"
            description: "Output archive name"
            required: true
            validation: "\\\\.(zip|tar\\\\.gz|tgz)$"
        },
        {
            name: "files"
            description: "Files to compress"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            if [ -z "$INVOWK_ARG_FILES" ]; then
                echo "No files specified!"
                exit 1
            fi
            
            # Use the space-separated list
            tar -czvf "$INVOWK_ARG_ARCHIVE" $INVOWK_ARG_FILES
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-deploy-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy services"
    args: [
        {
            name: "env"
            description: "Target environment"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        },
        {
            name: "services"
            description: "Services to deploy"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
            
            if [ -n "$INVOWK_ARG_SERVICES" ]; then
                for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
                    eval "service=\\$INVOWK_ARG_SERVICES_$i"
                    echo "Deploying $service..."
                    kubectl scale deployment/$service --replicas=$INVOWK_ARG_REPLICAS
                done
            else
                echo "Deploying all services..."
                kubectl scale deployment --all --replicas=$INVOWK_ARG_REPLICAS
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'flags-args/args-mixing-flags': {
    language: 'bash',
    code: `# All equivalent
invowk cmd myproject deploy prod 3 --dry-run
invowk cmd myproject deploy --dry-run prod 3
invowk cmd myproject deploy prod --dry-run 3`,
  },

  // =============================================================================
  // ADVANCED
  // =============================================================================

  'advanced/interpreter-python': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [
        {
            interpreter: "python3"
            script: """
                import json
                import sys

                data = json.load(open('data.json'))
                print(f"Found {len(data)} records")
                """
            target: {runtimes: [{name: "native"}]}
        }
    ]
}`,
  },

  'advanced/interpreter-node': {
    language: 'cue',
    code: `{
    name: "process"
    implementations: [
        {
            interpreter: "node"
            script: """
                const fs = require('fs');
                const data = JSON.parse(fs.readFileSync('data.json'));
                console.log(\`Processing \${data.length} items\`);
                """
            target: {runtimes: [{name: "native"}]}
        }
    ]
}`,
  },

  'advanced/workdir': {
    language: 'cue',
    code: `// Global workdir for all commands
workdir: "./src"

cmds: [
    {
        name: "build"
        // Command-specific workdir
        workdir: "./src/app"
        implementations: [...]
    }
]`,
  },

  'advanced/platform-specific': {
    language: 'cue',
    code: `{
    name: "open-browser"
    implementations: [
        {
            script: "xdg-open $URL"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: "open $URL"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: "start $URL"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}`,
  },

  // Interpreters page snippets
  'advanced/interpreter-shebang': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import json
            
            data = {"status": "ok", "python": sys.version}
            print(json.dumps(data, indent=2))
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'advanced/interpreter-explicit': {
    language: 'cue',
    code: `{
    name: "script"
    implementations: [{
        script: """
            import sys
            print(f"Python {sys.version_info.major}.{sys.version_info.minor}")
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3"  // Explicit
            }]
        }
    }]
}`,
  },

  'advanced/interpreter-args': {
    language: 'cue',
    code: `{
    name: "unbuffered"
    implementations: [{
        script: """
            import time
            for i in range(5):
                print(f"Count: {i}")
                time.sleep(1)
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3 -u"  // Unbuffered output
            }]
        }
    }]
}`,
  },

  'advanced/interpreter-more-args': {
    language: 'cue',
    code: `// Perl with warnings
interpreter: "perl -w"

// Ruby with debug mode
interpreter: "ruby -d"

// Node with specific options
interpreter: "node --max-old-space-size=4096"`,
  },

  'advanced/interpreter-container-shebang': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import os
            print(f"Running in container at {os.getcwd()}")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11-slim"
            }]
        }
    }]
}`,
  },

  'advanced/interpreter-container-explicit': {
    language: 'cue',
    code: `{
    name: "script"
    implementations: [{
        script: """
            console.log('Hello from Node in container!')
            console.log('Node version:', process.version)
            """
        target: {
            runtimes: [{
                name: "container"
                image: "node:20-slim"
                interpreter: "node"
            }]
        }
    }]
}`,
  },

  'advanced/interpreter-args-access': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import os
            
            # Via command line args
            name = sys.argv[1] if len(sys.argv) > 1 else "World"
            print(f"Hello, {name}!")
            
            # Or via environment variable
            name = os.environ.get("INVOWK_ARG_NAME", "World")
            print(f"Hello again, {name}!")
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'advanced/interpreter-virtual-error': {
    language: 'cue',
    code: `// This will NOT work!
{
    name: "bad"
    implementations: [{
        script: "print('hello')"
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERROR!
            }]
        }
    }]
}`,
  },

  // Workdir page snippets
  'advanced/workdir-command': {
    language: 'cue',
    code: `{
    name: "build frontend"
    workdir: "./frontend"  // Run in frontend subdirectory
    implementations: [{
        script: "npm run build"
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'advanced/workdir-implementation': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            workdir: "./web"  // This implementation runs in ./web
        },
        {
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
            workdir: "./api"  // This implementation runs in ./api
        }
    ]
}`,
  },

  'advanced/workdir-root': {
    language: 'cue',
    code: `group: "myproject"
workdir: "./src"  // All commands default to ./src

cmds: [
    {
        name: "build"
        // Inherits workdir: ./src
        implementations: [...]
    },
    {
        name: "test"
        workdir: "./tests"  // Override to ./tests
        implementations: [...]
    }
]`,
  },

  'advanced/workdir-relative': {
    language: 'cue',
    code: `workdir: "./frontend"
workdir: "../shared"
workdir: "src/app"`,
  },

  'advanced/workdir-absolute': {
    language: 'cue',
    code: `workdir: "/opt/myapp"
workdir: "/home/user/projects/myapp"`,
  },

  'advanced/workdir-env-vars': {
    language: 'cue',
    code: `workdir: "\${HOME}/projects/myapp"
workdir: "\${PROJECT_ROOT}/src"`,
  },

  'advanced/workdir-precedence': {
    language: 'cue',
    code: `group: "myproject"
workdir: "./root"  // Default: ./root

cmds: [
    {
        name: "build"
        workdir: "./command"  // Override: ./command
        implementations: [
            {
                script: "make"
                workdir: "./implementation"  // Final: ./implementation
                target: {runtimes: [{name: "native"}]}
            }
        ]
    }
]`,
  },

  'advanced/workdir-monorepo': {
    language: 'cue',
    code: `group: "monorepo"

cmds: [
    {
        name: "web build"
        workdir: "./packages/web"
        implementations: [{
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "api build"
        workdir: "./packages/api"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "mobile build"
        workdir: "./packages/mobile"
        implementations: [{
            script: "flutter build"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'advanced/workdir-container-default': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            pwd  # /workspace
            ls   # Shows your project files
            """
        target: {
            runtimes: [{name: "container", image: "debian:bookworm-slim"}]
        }
    }]
}`,
  },

  'advanced/workdir-container-subdir': {
    language: 'cue',
    code: `{
    name: "build frontend"
    workdir: "./frontend"
    implementations: [{
        script: """
            pwd  # /workspace/frontend
            npm run build
            """
        target: {
            runtimes: [{name: "container", image: "node:20"}]
        }
    }]
}`,
  },

  'advanced/workdir-cross-platform': {
    language: 'cue',
    code: `// Good - works everywhere
workdir: "./src/app"

// Avoid - Windows-specific
workdir: ".\\\\src\\\\app"`,
  },

  'advanced/workdir-frontend-backend': {
    language: 'cue',
    code: `cmds: [
    {
        name: "start frontend"
        workdir: "./frontend"
        implementations: [{
            script: "npm run dev"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "start backend"
        workdir: "./backend"
        implementations: [{
            script: "go run ./cmd/server"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'advanced/workdir-tests': {
    language: 'cue',
    code: `cmds: [
    {
        name: "test unit"
        workdir: "./tests/unit"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test integration"
        workdir: "./tests/integration"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test e2e"
        workdir: "./tests/e2e"
        implementations: [{
            script: "cypress run"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'advanced/workdir-build-subdir': {
    language: 'cue',
    code: `{
    name: "build"
    workdir: "./src"
    implementations: [{
        script: """
            # Now in ./src
            go build -o ../bin/app ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  // Platform-specific page snippets
  'advanced/platform-open-browser': {
    language: 'cue',
    code: `{
    name: "open-browser"
    implementations: [
        {
            script: "xdg-open http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: "open http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: "start http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}`,
  },

  'advanced/platform-all-default': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "native"}]
            // No platforms = works everywhere
        }
    }]
}`,
  },

  'advanced/platform-unix-only': {
    language: 'cue',
    code: `{
    name: "check-permissions"
    implementations: [{
        script: """
            chmod +x ./scripts/*.sh
            ls -la ./scripts/
            """
        target: {
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    }]
}`,
  },

  'advanced/platform-env': {
    language: 'cue',
    code: `{
    name: "configure"
    implementations: [{
        script: "echo \\"Config: \$CONFIG_PATH\\""
        target: {
            runtimes: [{name: "native"}]
            platforms: [
                {
                    name: "linux"
                    env: {
                        CONFIG_PATH: "/etc/myapp/config.yaml"
                        CACHE_DIR: "/var/cache/myapp"
                    }
                },
                {
                    name: "macos"
                    env: {
                        CONFIG_PATH: "/usr/local/etc/myapp/config.yaml"
                        CACHE_DIR: "\${HOME}/Library/Caches/myapp"
                    }
                },
                {
                    name: "windows"
                    env: {
                        CONFIG_PATH: "%APPDATA%\\\\myapp\\\\config.yaml"
                        CACHE_DIR: "%LOCALAPPDATA%\\\\myapp\\\\cache"
                    }
                }
            ]
        }
    }]
}`,
  },

  'advanced/platform-cross-script': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            go build -o \${OUTPUT_NAME} ./...
            """
        target: {
            runtimes: [{name: "native"}]
            platforms: [
                {name: "linux", env: {OUTPUT_NAME: "bin/app"}},
                {name: "macos", env: {OUTPUT_NAME: "bin/app"}},
                {name: "windows", env: {OUTPUT_NAME: "bin/app.exe"}}
            ]
        }
    }]
}`,
  },

  'advanced/platform-cue-templates': {
    language: 'cue',
    code: `// Define platform templates
_linux: {name: "linux"}
_macos: {name: "macos"}
_windows: {name: "windows"}

_unix: [{name: "linux"}, {name: "macos"}]
_all: [{name: "linux"}, {name: "macos"}, {name: "windows"}]

cmds: [
    {
        name: "clean"
        implementations: [
            // Unix implementation
            {
                script: "rm -rf build/"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: _unix
                }
            },
            // Windows implementation
            {
                script: "rmdir /s /q build"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [_windows]
                }
            }
        ]
    }
]`,
  },

  'advanced/platform-list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*] (linux, macos, windows)
  myproject clean - Clean artifacts [native*] (linux, macos)
  myproject deploy - Deploy to cloud [native*] (linux)`,
  },

  'advanced/platform-unsupported-error': {
    language: 'text',
    code: `✗ Host not supported

Command 'deploy' cannot run on this host.

Current host:     windows
Supported hosts:  linux, macos

This command is only available on the platforms listed above.`,
  },

  'advanced/platform-sysinfo': {
    language: 'cue',
    code: `{
    name: "sysinfo"
    implementations: [
        {
            script: """
                echo "Hostname: \$(hostname)"
                echo "Kernel: \$(uname -r)"
                echo "Memory: \$(free -h | awk '/^Mem:/{print \$2}')"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: """
                echo "Hostname: \$(hostname)"
                echo "Kernel: \$(uname -r)"
                echo "Memory: \$(sysctl -n hw.memsize | awk '{print \$0/1024/1024/1024 "GB"}')"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: """
                echo Hostname: %COMPUTERNAME%
                systeminfo | findstr "Total Physical Memory"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}`,
  },

  'advanced/platform-install-deps': {
    language: 'cue',
    code: `{
    name: "install-deps"
    implementations: [
        {
            script: "apt-get install -y build-essential"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: "brew install coreutils"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: "choco install make"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}`,
  },

  // =============================================================================
  // PACKS
  // =============================================================================

  'packs/create': {
    language: 'bash',
    code: `invowk pack create com.example.mytools`,
  },

  'packs/validate': {
    language: 'bash',
    code: `invowk pack validate ./mypack.invkpack --deep`,
  },

  'packs/pack-invkfile': {
    language: 'cue',
    code: `group: "com.example.mytools"
version: "1.0.0"
description: "My reusable development tools"

cmds: [
    {
        name: "lint"
        description: "Run linters"
        implementations: [
            {
                script: "./scripts/lint.sh"
                target: {runtimes: [{name: "virtual"}]}
            }
        ]
    }
]`,
  },

  // Overview page snippets
  'packs/structure-basic': {
    language: 'text',
    code: `mytools.invkpack/
├── invkfile.cue          # Required: command definitions
├── scripts/               # Optional: script files
│   ├── build.sh
│   └── deploy.sh
└── templates/             # Optional: other resources
    └── config.yaml`,
  },

  'packs/quick-create': {
    language: 'bash',
    code: `invowk pack create mytools`,
  },

  'packs/quick-create-output': {
    language: 'text',
    code: `mytools.invkpack/
└── invkfile.cue`,
  },

  'packs/quick-use': {
    language: 'bash',
    code: `# List commands (pack commands appear automatically)
invowk cmd list

# Run a pack command
invowk cmd mytools hello`,
  },

  'packs/quick-share': {
    language: 'bash',
    code: `# Create a zip archive
invowk pack archive mytools.invkpack

# Share the zip file
# Recipients import with:
invowk pack import mytools.invkpack.zip`,
  },

  'packs/structure-example': {
    language: 'text',
    code: `com.example.devtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build.sh
│   ├── deploy.sh
│   └── utils/
│       └── helpers.sh
├── templates/
│   ├── Dockerfile.tmpl
│   └── config.yaml.tmpl
└── README.md`,
  },

  'packs/rdns-naming': {
    language: 'text',
    code: `com.company.projectname.invkpack
io.github.username.toolkit.invkpack
org.opensource.utilities.invkpack`,
  },

  'packs/script-paths': {
    language: 'cue',
    code: `// Inside mytools.invkpack/invkfile.cue
group: "mytools"

cmds: [
    {
        name: "build"
        implementations: [{
            script: "scripts/build.sh"  // Relative to pack root
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "deploy"
        implementations: [{
            script: "scripts/utils/helpers.sh"  // Nested path
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'packs/discovery-output': {
    language: 'text',
    code: `Available Commands

From current directory:
  mytools build - Build the project [native*]

From user commands (~/.invowk/cmds):
  com.example.utilities hello - Greeting [native*]`,
  },

  // Creating packs page snippets
  'packs/create-options': {
    language: 'bash',
    code: `# Simple pack
invowk pack create mytools

# RDNS naming
invowk pack create com.company.devtools

# In specific directory
invowk pack create mytools --path /path/to/packs

# With scripts directory
invowk pack create mytools --scripts`,
  },

  'packs/create-with-scripts': {
    language: 'text',
    code: `mytools.invkpack/
├── invkfile.cue
└── scripts/`,
  },

  'packs/template-invkfile': {
    language: 'cue',
    code: `group: "mytools"
version: "1.0"
description: "Commands for mytools"

cmds: [
    {
        name: "hello"
        description: "Say hello"
        implementations: [
            {
                script: """
                    echo "Hello from mytools!"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]`,
  },

  'packs/manual-create': {
    language: 'bash',
    code: `mkdir mytools.invkpack
touch mytools.invkpack/invkfile.cue`,
  },

  'packs/inline-vs-external': {
    language: 'cue',
    code: `cmds: [
    // Simple: inline script
    {
        name: "quick"
        implementations: [{
            script: "echo 'Quick task'"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Complex: external script
    {
        name: "complex"
        implementations: [{
            script: "scripts/complex-task.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'packs/script-organization': {
    language: 'text',
    code: `mytools.invkpack/
├── invkfile.cue
└── scripts/
    ├── build.sh           # Main scripts
    ├── deploy.sh
    ├── test.sh
    └── lib/               # Shared utilities
        ├── logging.sh
        └── validation.sh`,
  },

  'packs/script-paths-good-bad': {
    language: 'cue',
    code: `// Good
script: "scripts/build.sh"
script: "scripts/lib/logging.sh"

// Bad - will fail on some platforms
script: "scripts\\\\build.sh"

// Bad - escapes pack directory
script: "../outside.sh"`,
  },

  'packs/env-files-structure': {
    language: 'text',
    code: `mytools.invkpack/
├── invkfile.cue
├── .env                   # Default config
├── .env.example           # Template for users
└── scripts/`,
  },

  'packs/env-files-ref': {
    language: 'cue',
    code: `env: {
    files: [".env"]
}`,
  },

  'packs/docs-structure': {
    language: 'text',
    code: `mytools.invkpack/
├── invkfile.cue
├── README.md              # Usage instructions
├── CHANGELOG.md           # Version history
└── scripts/`,
  },

  'packs/buildtools-structure': {
    language: 'text',
    code: `com.company.buildtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build-go.sh
│   ├── build-node.sh
│   └── build-python.sh
├── templates/
│   ├── Dockerfile.go
│   ├── Dockerfile.node
│   └── Dockerfile.python
└── README.md`,
  },

  'packs/buildtools-invkfile': {
    language: 'cue',
    code: `group: "com.company.buildtools"
version: "1.0"
description: "Standardized build tools"

cmds: [
    {
        name: "go"
        description: "Build Go project"
        implementations: [{
            script: "scripts/build-go.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "node"
        description: "Build Node.js project"
        implementations: [{
            script: "scripts/build-node.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "python"
        description: "Build Python project"
        implementations: [{
            script: "scripts/build-python.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]`,
  },

  'packs/devops-structure': {
    language: 'text',
    code: `org.devops.k8s.invkpack/
├── invkfile.cue
├── scripts/
│   ├── deploy.sh
│   ├── rollback.sh
│   └── status.sh
├── manifests/
│   ├── deployment.yaml
│   └── service.yaml
└── .env.example`,
  },

  'packs/validate-before-share': {
    language: 'bash',
    code: `invowk pack validate mytools.invkpack --deep`,
  },

  // Validating page snippets
  'packs/validate-basic': {
    language: 'bash',
    code: `invowk pack validate ./mytools.invkpack`,
  },

  'packs/validate-basic-output': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present`,
  },

  'packs/validate-deep': {
    language: 'bash',
    code: `invowk pack validate ./mytools.invkpack --deep`,
  },

  'packs/validate-deep-output': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
✓ Invkfile parses successfully`,
  },

  'packs/error-missing-invkfile': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/bad.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] missing required invkfile.cue`,
  },

  'packs/error-invalid-name': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/my-tools.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [naming] pack name 'my-tools' contains invalid characters (hyphens not allowed)`,
  },

  'packs/error-nested': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/parent.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] nested.invkpack: nested packs are not allowed`,
  },

  'packs/error-parse': {
    language: 'text',
    code: `Pack Validation
• Path: /home/user/broken.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [invkfile] parse error at line 15: expected '}', found EOF`,
  },

  'packs/validate-batch': {
    language: 'bash',
    code: `# Validate all packs in a directory
for pack in ./packs/*.invkpack; do
    invowk pack validate "$pack" --deep
done`,
  },

  'packs/validate-ci': {
    language: 'yaml',
    code: `# GitHub Actions example
- name: Validate packs
  run: |
    for pack in packs/*.invkpack; do
      invowk pack validate "$pack" --deep
    done`,
  },

  'packs/path-separators-good-bad': {
    language: 'cue',
    code: `// Bad - Windows-style
script: "scripts\\\\build.sh"

// Good - Forward slashes
script: "scripts/build.sh"`,
  },

  'packs/escape-pack-dir': {
    language: 'cue',
    code: `// Bad - tries to access parent
script: "../outside/script.sh"

// Good - stays within pack
script: "scripts/script.sh"`,
  },

  'packs/absolute-paths': {
    language: 'cue',
    code: `// Bad - absolute path
script: "/usr/local/bin/script.sh"

// Good - relative path
script: "scripts/script.sh"`,
  },

  // Distributing page snippets
  'packs/archive-basic': {
    language: 'bash',
    code: `# Default output: <pack-name>.invkpack.zip
invowk pack archive ./mytools.invkpack

# Custom output path
invowk pack archive ./mytools.invkpack --output ./dist/mytools.zip`,
  },

  'packs/archive-output': {
    language: 'text',
    code: `Archive Pack

✓ Pack archived successfully

• Output: /home/user/dist/mytools.zip
• Size: 2.45 KB`,
  },

  'packs/import-local': {
    language: 'bash',
    code: `# Install to ~/.invowk/cmds/
invowk pack import ./mytools.invkpack.zip

# Install to custom directory
invowk pack import ./mytools.invkpack.zip --path ./local-packs

# Overwrite existing
invowk pack import ./mytools.invkpack.zip --overwrite`,
  },

  'packs/import-url': {
    language: 'bash',
    code: `# Download and install
invowk pack import https://example.com/packs/mytools.zip

# From GitHub release
invowk pack import https://github.com/user/repo/releases/download/v1.0/mytools.invkpack.zip`,
  },

  'packs/import-output': {
    language: 'text',
    code: `Import Pack

✓ Pack imported successfully

• Name: mytools
• Path: /home/user/.invowk/cmds/mytools.invkpack

• The pack commands are now available via invowk`,
  },

  'packs/list': {
    language: 'bash',
    code: `invowk pack list`,
  },

  'packs/list-output': {
    language: 'text',
    code: `Discovered Packs

• Found 3 pack(s)

• current directory:
   ✓ local.project
      /home/user/project/local.project.invkpack

• user commands (~/.invowk/cmds):
   ✓ com.company.devtools
      /home/user/.invowk/cmds/com.company.devtools.invkpack
   ✓ io.github.user.utilities
      /home/user/.invowk/cmds/io.github.user.utilities.invkpack`,
  },

  'packs/git-structure': {
    language: 'text',
    code: `my-project/
├── src/
├── packs/
│   ├── devtools.invkpack/
│   └── testing.invkpack/
└── invkfile.cue`,
  },

  'packs/github-release': {
    language: 'bash',
    code: `# Recipients install with:
invowk pack import https://github.com/org/repo/releases/download/v1.0.0/mytools.invkpack.zip`,
  },

  'packs/future-install': {
    language: 'bash',
    code: `invowk pack install com.company.devtools@1.0.0`,
  },

  'packs/install-user': {
    language: 'bash',
    code: `invowk pack import mytools.zip
# Installed to: ~/.invowk/cmds/mytools.invkpack/`,
  },

  'packs/install-project': {
    language: 'bash',
    code: `invowk pack import mytools.zip --path ./packs
# Installed to: ./packs/mytools.invkpack/`,
  },

  'packs/search-paths-config': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
search_paths: [
    "/shared/company-packs"
]`,
  },

  'packs/install-search-path': {
    language: 'bash',
    code: `invowk pack import mytools.zip --path /shared/company-packs`,
  },

  'packs/version-invkfile': {
    language: 'cue',
    code: `group: "com.company.tools"
version: "1.2.0"`,
  },

  'packs/archive-versioned': {
    language: 'bash',
    code: `invowk pack archive ./mytools.invkpack --output ./dist/mytools-1.2.0.zip`,
  },

  'packs/upgrade-process': {
    language: 'bash',
    code: `# Remove old version
rm -rf ~/.invowk/cmds/mytools.invkpack

# Install new version
invowk pack import mytools-1.2.0.zip

# Or use --overwrite
invowk pack import mytools-1.2.0.zip --overwrite`,
  },

  'packs/team-shared-location': {
    language: 'bash',
    code: `# Admin publishes
invowk pack archive ./devtools.invkpack --output /shared/packs/devtools.zip

# Team members import
invowk pack import /shared/packs/devtools.zip`,
  },

  'packs/internal-server': {
    language: 'bash',
    code: `# Team members import via URL
invowk pack import https://internal.company.com/packs/devtools.zip`,
  },

  'packs/workflow-example': {
    language: 'bash',
    code: `# 1. Create and develop pack
invowk pack create com.company.mytools --scripts
# ... add commands and scripts ...

# 2. Validate
invowk pack validate ./com.company.mytools.invkpack --deep

# 3. Create versioned archive
invowk pack archive ./com.company.mytools.invkpack \\
    --output ./releases/mytools-1.0.0.zip

# 4. Distribute (e.g., upload to GitHub release)

# 5. Team imports
invowk pack import https://github.com/company/mytools/releases/download/v1.0.0/mytools-1.0.0.zip`,
  },

  // =============================================================================
  // TUI COMPONENTS
  // =============================================================================

  'tui/input': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "What's your name?")
echo "Hello, $NAME!"`,
  },

  'tui/choose': {
    language: 'bash',
    code: `COLOR=$(invowk tui choose --title "Pick a color" red green blue)
echo "You picked: $COLOR"`,
  },

  'tui/confirm': {
    language: 'bash',
    code: `if invowk tui confirm --title "Are you sure?"; then
    echo "Proceeding..."
else
    echo "Cancelled"
fi`,
  },

  'tui/spin': {
    language: 'bash',
    code: `invowk tui spin --title "Installing dependencies..." -- npm install`,
  },

  'tui/filter': {
    language: 'bash',
    code: `SELECTED=$(ls | invowk tui filter --title "Select a file")`,
  },

  // =============================================================================
  // CONFIGURATION
  // =============================================================================

  'config/example': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
container_engine: "podman"
search_paths: [
    "~/.invowk/packs",
    "/usr/local/share/invowk/packs"
]
default_runtime: "virtual"`,
  },

  'config/container-engine': {
    language: 'cue',
    code: `container_engine: "docker"  // or "podman"`,
  },

  'config/search-paths': {
    language: 'cue',
    code: `search_paths: [
    "~/.invowk/packs",
    "~/my-company/shared-packs",
    "/opt/invowk/packs"
]`,
  },

  'config/cli-custom-path': {
    language: 'bash',
    code: `invowk --config /path/to/my/config.cue cmd list`,
  },

  'config/init': {
    language: 'bash',
    code: `invowk config init`,
  },

  'config/show': {
    language: 'bash',
    code: `invowk config show`,
  },

  'config/dump': {
    language: 'bash',
    code: `invowk config dump`,
  },

  'config/path': {
    language: 'bash',
    code: `invowk config path`,
  },

  'config/set-examples': {
    language: 'bash',
    code: `# Set the container engine
invowk config set container_engine podman

# Set the default runtime
invowk config set default_runtime virtual

# Set the color scheme
invowk config set ui.color_scheme dark`,
  },

  'config/edit-linux-macos': {
    language: 'bash',
    code: `# Linux/macOS
$EDITOR $(invowk config path)

# Windows PowerShell
notepad (invowk config path)`,
  },

  'config/full-example': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue

// Container engine: "podman" or "docker"
container_engine: "podman"

// Additional directories to search for invkfiles
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
]

// Default runtime for commands that don't specify one
default_runtime: "native"

// Virtual shell configuration
virtual_shell: {
    enable_uroot_utils: true
}

// UI preferences
ui: {
    color_scheme: "auto"  // "auto", "dark", or "light"
    verbose: false
    interactive: false    // Enable alternate screen buffer mode
}`,
  },

  'config/schema': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    search_paths?: [...string]
    default_runtime?: "native" | "virtual" | "container"
    virtual_shell?: #VirtualShellConfig
    ui?: #UIConfig
}

#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?: bool
    interactive?: bool
}`,
  },

  'config/default-runtime': {
    language: 'cue',
    code: `default_runtime: "virtual"`,
  },

  'config/virtual-shell': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'config/ui': {
    language: 'cue',
    code: `ui: {
    color_scheme: "dark"
    verbose: false
    interactive: false
}`,
  },

  'config/ui-color-scheme': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
}`,
  },

  'config/ui-verbose': {
    language: 'cue',
    code: `ui: {
    verbose: true
}`,
  },

  'config/ui-interactive': {
    language: 'cue',
    code: `ui: {
    interactive: true
}`,
  },

  'config/complete-example': {
    language: 'cue',
    code: `// Invowk Configuration File
// Located at: ~/.config/invowk/config.cue

// Use Podman as the container engine
container_engine: "podman"

// Search for invkfiles in these directories
search_paths: [
    "~/.invowk/cmds",          // Personal commands
    "~/work/shared-commands",   // Team shared commands
]

// Default to virtual shell for portability
default_runtime: "virtual"

// Virtual shell settings
virtual_shell: {
    // Enable u-root utilities for more shell commands
    enable_uroot_utils: true
}

// UI preferences
ui: {
    // Auto-detect color scheme from terminal
    color_scheme: "auto"
    
    // Don't be verbose by default
    verbose: false
    
    // Enable interactive mode for commands with stdin (e.g., password prompts)
    interactive: false
}`,
  },

  'config/env-override-examples': {
    language: 'bash',
    code: `# Example: Use Docker instead of configured Podman
INVOWK_CONTAINER_ENGINE=docker invowk cmd build

# Example: Enable verbose output for this run
INVOWK_VERBOSE=1 invowk cmd test`,
  },

  'config/cli-override-examples': {
    language: 'bash',
    code: `# Override config file
invowk --config /path/to/config.cue cmd list

# Override verbose setting
invowk --verbose cmd build

# Run command in interactive mode (alternate screen buffer)
invowk --interactive cmd myproject build

# Override runtime for a command
invowk cmd build --runtime container`,
  },

  // =============================================================================
  // INSTALLATION
  // =============================================================================

  'installation/build-from-source': {
    language: 'bash',
    code: `git clone https://github.com/invowk/invowk
cd invowk
go build -o invowk .`,
  },

  'installation/move-to-path': {
    language: 'bash',
    code: `# Linux/macOS
sudo mv invowk /usr/local/bin/

# Or add to your local bin
mv invowk ~/.local/bin/`,
  },

  'installation/make-options': {
    language: 'bash',
    code: `# Standard build (stripped binary, smaller size)
make build

# Development build (with debug symbols)
make build-dev

# Compressed build (requires UPX)
make build-upx

# Install to $GOPATH/bin
make install`,
  },

  'installation/verify': {
    language: 'bash',
    code: `invowk --version`,
  },

  'installation/completion-bash': {
    language: 'bash',
    code: `# Add to ~/.bashrc
eval "$(invowk completion bash)"

# Or install system-wide
invowk completion bash > /etc/bash_completion.d/invowk`,
  },

  'installation/completion-zsh': {
    language: 'bash',
    code: `# Add to ~/.zshrc
eval "$(invowk completion zsh)"

# Or install to fpath
invowk completion zsh > "\${fpath[1]}/_invowk"`,
  },

  'installation/completion-fish': {
    language: 'bash',
    code: `invowk completion fish > ~/.config/fish/completions/invowk.fish`,
  },

  'installation/completion-powershell': {
    language: 'powershell',
    code: `invowk completion powershell | Out-String | Invoke-Expression

# Or add to $PROFILE for persistence
invowk completion powershell >> $PROFILE`,
  },

  // =============================================================================
  // QUICKSTART
  // =============================================================================

  'quickstart/init': {
    language: 'bash',
    code: `cd my-project
invowk init`,
  },

  'quickstart/hello-invkfile': {
    language: 'cue',
    code: `group: "myproject"
version: "1.0"
description: "My project commands"

cmds: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]`,
  },

  'quickstart/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  myproject hello - Say hello! [native*] (linux, macos, windows)`,
  },

  'quickstart/run-hello': {
    language: 'bash',
    code: `invowk cmd myproject hello`,
  },

  'quickstart/hello-output': {
    language: 'text',
    code: `Hello from Invowk!`,
  },

  'quickstart/info-command': {
    language: 'cue',
    code: `group: "myproject"
version: "1.0"
description: "My project commands"

cmds: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },
    {
        name: "info"
        description: "Show system information"
        implementations: [
            {
                script: """
                    echo "=== System Info ==="
                    echo "User: $USER"
                    echo "Directory: $(pwd)"
                    echo "Date: $(date)"
                    """
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]`,
  },

  'quickstart/virtual-runtime': {
    language: 'cue',
    code: `{
    name: "cross-platform"
    description: "Works the same everywhere!"
    implementations: [
        {
            script: "echo 'This runs identically on Linux, Mac, and Windows!'"
            target: {
                runtimes: [{name: "virtual"}]
            }
        }
    ]
}`,
  },

  // =============================================================================
  // COMMANDS AND GROUPS
  // =============================================================================

  'commands-groups/basic-group': {
    language: 'cue',
    code: `group: "myproject"

cmds: [
    {name: "build"},
    {name: "test"},
    {name: "deploy"},
]`,
  },

  'commands-groups/valid-groups': {
    language: 'cue',
    code: `group: "frontend"
group: "backend"
group: "my.project"
group: "com.company.tools"
group: "io.github.username.cli"`,
  },

  'commands-groups/invalid-groups': {
    language: 'cue',
    code: `group: "my-project"     // Hyphens not allowed
group: "my_project"     // Underscores not allowed
group: ".project"       // Can't start with dot
group: "project."       // Can't end with dot
group: "my..project"    // No consecutive dots
group: "123project"     // Must start with letter`,
  },

  'commands-groups/nested-group': {
    language: 'cue',
    code: `group: "com.company.frontend"

cmds: [
    {name: "build"},
    {name: "test"},
]`,
  },

  'commands-groups/subcommand-names': {
    language: 'cue',
    code: `group: "myproject"

cmds: [
    {name: "test"},           // myproject test
    {name: "test unit"},      // myproject test unit
    {name: "test integration"}, // myproject test integration
    {name: "db migrate"},     // myproject db migrate
    {name: "db seed"},        // myproject db seed
]`,
  },

  'commands-groups/discovery-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*] (linux, macos, windows)

From user commands (~/.invowk/cmds):
  utils hello - A greeting [native*] (linux, macos)`,
  },

  'commands-groups/command-dependency': {
    language: 'cue',
    code: `group: "myproject"

cmds: [
    {
        name: "build"
        implementations: [...]
    },
    {
        name: "test"
        implementations: [...]
        depends_on: {
            cmds: [
                // Reference by full name (group + command name)
                {alternatives: ["myproject build"]}
            ]
        }
    },
    {
        name: "release"
        implementations: [...]
        depends_on: {
            cmds: [
                // Can depend on commands from other invkfiles too
                {alternatives: ["myproject build"]},
                {alternatives: ["myproject test"]},
                {alternatives: ["other.project lint"]},
            ]
        }
    }
]`,
  },

  'commands-groups/cross-invkfile-dep': {
    language: 'cue',
    code: `// In frontend/invkfile.cue
group: "frontend"

cmds: [
    {
        name: "build"
        depends_on: {
            cmds: [
                // Depends on backend build completing first
                {alternatives: ["backend build"]}
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
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
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
    code: `target: {
    runtimes: [
        {name: "native"},      // System shell
        {name: "virtual"},     // Built-in POSIX shell
        {name: "container", image: "debian:bookworm-slim"}  // Container
    ]
}`,
  },

  'implementations/platforms-list': {
    language: 'cue',
    code: `target: {
    runtimes: [{name: "native"}]
    platforms: [
        {name: "linux"},
        {name: "macos"},
        {name: "windows"}
    ]
}`,
  },

  'implementations/platform-specific': {
    language: 'cue',
    code: `{
    name: "clean"
    implementations: [
        // Unix implementation
        {
            script: "rm -rf build/"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows implementation
        {
            script: "rmdir /s /q build"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
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
            target: {
                runtimes: [{name: "native"}]
            }
        },
        // Reproducible container build
        {
            script: "go build -o /workspace/bin/app ./..."
            target: {
                runtimes: [{name: "container", image: "golang:1.21"}]
            }
        }
    ]
}`,
  },

  'implementations/platform-env': {
    language: 'cue',
    code: `{
    name: "deploy"
    implementations: [
        {
            script: "echo \\"Deploying to $PLATFORM with config at $CONFIG_PATH\\""
            target: {
                runtimes: [{name: "native"}]
                platforms: [
                    {
                        name: "linux"
                        env: {
                            PLATFORM: "Linux"
                            CONFIG_PATH: "/etc/app/config.yaml"
                        }
                    },
                    {
                        name: "macos"
                        env: {
                            PLATFORM: "macOS"
                            CONFIG_PATH: "/usr/local/etc/app/config.yaml"
                        }
                    }
                ]
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
            target: {
                runtimes: [{name: "native"}]
            }
            
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
            target: {
                runtimes: [{name: "native"}, {name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        {
            script: "msbuild"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}`,
  },

  'implementations/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual] (linux, macos)
  myproject clean - Clean artifacts [native*] (linux, macos, windows)
  myproject docker-build - Container build [container*] (linux, macos, windows)`,
  },

  'implementations/cue-templates': {
    language: 'cue',
    code: `// Define reusable templates
_unixNative: {
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
}

_allPlatforms: {
    target: {
        runtimes: [{name: "native"}]
    }
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

  // =============================================================================
  // RUNTIME MODES - ADDITIONAL
  // =============================================================================

  'runtime-modes/comparison': {
    language: 'cue',
    code: `cmds: [
    // Native: uses your system shell (bash, zsh, PowerShell, etc.)
    {
        name: "build native"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Virtual: uses built-in POSIX-compatible shell
    {
        name: "build virtual"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "virtual"}]}
        }]
    },
    
    // Container: runs inside a container
    {
        name: "build container"
        implementations: [{
            script: "go build -o /workspace/bin/app ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
        }]
    }
]`,
  },

  'runtime-modes/multiple-runtimes': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [
                {name: "native"},  // Default
                {name: "virtual"}, // Alternative
                {name: "container", image: "golang:1.21"}  // Reproducible
            ]
        }
    }]
}`,
  },

  'runtime-modes/override-cli': {
    language: 'bash',
    code: `# Use default (native)
invowk cmd myproject build

# Override to virtual
invowk cmd myproject build --runtime virtual

# Override to container
invowk cmd myproject build --runtime container`,
  },

  'runtime-modes/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual, container] (linux, macos)`,
  },

  'runtime-modes/container-containerfile': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    containerfile: "./Containerfile"  // Relative to invkfile
}]`,
  },

  'runtime-modes/containerfile-example': {
    language: 'dockerfile',
    code: `FROM golang:1.21

RUN apt-get update && apt-get install -y \\
    make \\
    git

WORKDIR /workspace`,
  },

  'runtime-modes/container-ports': {
    language: 'cue',
    code: `target: {
    runtimes: [{
        name: "container"
        image: "node:20"
        ports: [
            "3000:3000",      // Host:Container
            "8080:80"         // Map container port 80 to host port 8080
        ]
    }]
}`,
  },

  'runtime-modes/container-interpreter': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: """
            import sys
            print(f"Running on Python {sys.version_info.major}")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11"
                interpreter: "python3"
            }]
        }
    }]
}`,
  },

  'runtime-modes/container-ssh-example': {
    language: 'cue',
    code: `{
    name: "deploy from container"
    implementations: [{
        script: """
            # Connection credentials are provided via environment variables
            echo "SSH Host: $INVOWK_SSH_HOST"
            echo "SSH Port: $INVOWK_SSH_PORT"
            
            # Connect back to host
            sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no \\
                $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT \\
                'echo "Hello from host!"'
            """
        target: {
            runtimes: [{
                name: "container"
                image: "debian:bookworm-slim"
                enable_host_ssh: true  // Enable SSH server
            }]
        }
    }]
}`,
  },

  'runtime-modes/container-full-example': {
    language: 'cue',
    code: `{
    name: "build and test"
    description: "Build and test in isolated container"
    env: {
        vars: {
            GO_ENV: "test"
            CGO_ENABLED: "0"
        }
    }
    depends_on: {
        tools: [{alternatives: ["go"]}]
        filepaths: [{alternatives: ["go.mod"]}]
    }
    implementations: [{
        script: """
            echo "Go version: $(go version)"
            echo "Building..."
            go build -o /workspace/bin/app ./...
            echo "Testing..."
            go test -v ./...
            echo "Done!"
            """
        target: {
            runtimes: [{
                name: "container"
                image: "golang:1.21"
                volumes: [
                    "\${HOME}/go/pkg/mod:/go/pkg/mod:ro"  // Cache modules
                ]
            }]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    }]
}`,
  },

  // =============================================================================
  // DEPENDENCIES - ADDITIONAL
  // =============================================================================

  'dependencies/without-check': {
    language: 'bash',
    code: `$ invowk cmd myproject build
./scripts/build.sh: line 5: go: command not found`,
  },

  'dependencies/with-check': {
    language: 'text',
    code: `$ invowk cmd myproject build

✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.`,
  },

  'dependencies/basic-syntax': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"]}
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/alternatives-pattern': {
    language: 'cue',
    code: `// ANY of these tools satisfies the dependency
tools: [
    {alternatives: ["podman", "docker"]}
]

// ANY of these files satisfies the dependency
filepaths: [
    {alternatives: ["config.yaml", "config.json", "config.toml"]}
]`,
  },

  'dependencies/scope-root': {
    language: 'cue',
    code: `group: "myproject"

depends_on: {
    tools: [{alternatives: ["git"]}]  // Required by all commands
}

cmds: [...]`,
  },

  'dependencies/scope-command': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]  // Required by this command
    }
    implementations: [...]
}`,
  },

  'dependencies/scope-implementation': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
            depends_on: {
                // Validated INSIDE the container
                tools: [{alternatives: ["go"]}]
            }
        }
    ]
}`,
  },

  'dependencies/scope-inheritance': {
    language: 'cue',
    code: `group: "myproject"

// Root level: requires git
depends_on: {
    tools: [{alternatives: ["git"]}]
}

cmds: [
    {
        name: "build"
        // Command level: also requires go
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
        implementations: [
            {
                script: "go build ./..."
                target: {runtimes: [{name: "native"}]}
                // Implementation level: also requires make
                depends_on: {
                    tools: [{alternatives: ["make"]}]
                }
            }
        ]
    }
]

// Effective dependencies for "build": git + go + make`,
  },

  'dependencies/complete-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to production"
    depends_on: {
        // Check environment first
        env_vars: [
            {alternatives: [{name: "AWS_ACCESS_KEY_ID"}, {name: "AWS_PROFILE"}]}
        ]
        // Check required tools
        tools: [
            {alternatives: ["docker", "podman"]},
            {alternatives: ["kubectl"]}
        ]
        // Check required files
        filepaths: [
            {alternatives: ["Dockerfile"]},
            {alternatives: ["k8s/deployment.yaml"]}
        ]
        // Check network connectivity
        capabilities: [
            {alternatives: ["internet"]}
        ]
        // Run other commands first
        cmds: [
            {alternatives: ["myproject build"]},
            {alternatives: ["myproject test"]}
        ]
    }
    implementations: [
        {
            script: "./scripts/deploy.sh"
            target: {runtimes: [{name: "native"}]}
        }
    ]
}`,
  },

  'dependencies/error-output': {
    language: 'text',
    code: `✗ Dependencies not satisfied

Command 'deploy' has unmet dependencies:

Missing Tools:
  • docker - not found in PATH
  • kubectl - not found in PATH

Missing Files:
  • Dockerfile - file not found

Missing Environment Variables:
  • AWS_ACCESS_KEY_ID - not set in environment

Install the missing tools and try again.`,
  },

  // =============================================================================
  // TUI - ADDITIONAL
  // =============================================================================

  'tui/input-options': {
    language: 'bash',
    code: `# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With initial value
invowk tui input --title "Name" --value "John Doe"

# Limited length
invowk tui input --title "Username" --char-limit 20`,
  },

  'tui/input-in-script': {
    language: 'cue',
    code: `{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/write-basic': {
    language: 'bash',
    code: `invowk tui write --title "Enter description"`,
  },

  'tui/write-options': {
    language: 'bash',
    code: `# Basic editor
invowk tui write --title "Description:"

# With line numbers
invowk tui write --title "Code:" --show-line-numbers

# With initial content
invowk tui write --title "Edit message:" --value "Initial text here"`,
  },

  'tui/write-commit': {
    language: 'cue',
    code: `{
    name: "commit"
    description: "Interactive commit with editor"
    implementations: [{
        script: """
            # Show staged changes
            git diff --cached --stat
            
            # Get commit message
            MESSAGE=$(invowk tui write --title "Commit message:")
            
            if [ -z "$MESSAGE" ]; then
                echo "Commit cancelled (empty message)"
                exit 1
            fi
            
            git commit -m "$MESSAGE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/empty-validation': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi`,
  },

  'tui/validation-loop': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done`,
  },

  // Overview page
  'tui/overview-quick-examples': {
    language: 'bash',
    code: `# Get user input
NAME=$(invowk tui input --title "What's your name?")

# Choose from options
COLOR=$(invowk tui choose --title "Pick a color" red green blue)

# Confirm action
if invowk tui confirm "Continue?"; then
    echo "Proceeding..."
fi

# Show spinner during long task
invowk tui spin --title "Installing..." -- npm install

# Style output
echo "Success!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/overview-invkfile-example': {
    language: 'cue',
    code: `{
    name: "setup"
    description: "Interactive project setup"
    implementations: [{
        script: """
            #!/bin/bash
            
            # Gather information
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" cli library api)
            
            # Confirm
            echo "Creating $TYPE project: $NAME"
            if ! invowk tui confirm "Proceed?"; then
                echo "Cancelled."
                exit 0
            fi
            
            # Execute with spinner
            invowk tui spin --title "Creating project..." -- mkdir -p "$NAME"
            
            # Success message
            echo "Project created!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/overview-input-validation': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email address:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\\.[^@]+$'; then
        break
    fi
    echo "Invalid email format" | invowk tui style --foreground "#FF0000"
done`,
  },

  'tui/overview-menu-system': {
    language: 'bash',
    code: `ACTION=$(invowk tui choose --title "What would you like to do?" \\
    "Build project" \\
    "Run tests" \\
    "Deploy" \\
    "Exit")

case "$ACTION" in
    "Build project") make build ;;
    "Run tests") make test ;;
    "Deploy") make deploy ;;
    "Exit") exit 0 ;;
esac`,
  },

  'tui/overview-progress-feedback': {
    language: 'bash',
    code: `echo "Step 1: Installing dependencies..."
invowk tui spin --title "Installing..." -- npm install

echo "Step 2: Building..."
invowk tui spin --title "Building..." -- npm run build

echo "Done!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/overview-styled-headers': {
    language: 'bash',
    code: `invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""
# ... rest of script`,
  },

  'tui/overview-piping': {
    language: 'bash',
    code: `# Pipe to filter
ls | invowk tui filter --title "Select file"

# Capture output
SELECTED=$(invowk tui choose opt1 opt2 opt3)
echo "You selected: $SELECTED"

# Pipe for styling
echo "Important message" | invowk tui style --bold`,
  },

  'tui/overview-exit-codes': {
    language: 'bash',
    code: `if invowk tui confirm "Delete files?"; then
    rm -rf ./temp
fi`,
  },

  // Input component
  'tui/input-basic': {
    language: 'bash',
    code: `invowk tui input --title "What is your name?"`,
  },

  'tui/input-examples': {
    language: 'bash',
    code: `# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With initial value
invowk tui input --title "Name" --value "John Doe"

# Limited length
invowk tui input --title "Username" --char-limit 20`,
  },

  'tui/input-capture': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Enter your name:")
echo "Hello, $NAME!"`,
  },

  'tui/input-script': {
    language: 'cue',
    code: `{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  // Write component
  'tui/write-examples': {
    language: 'bash',
    code: `# Basic editor
invowk tui write --title "Description:"

# With line numbers
invowk tui write --title "Code:" --show-line-numbers

# With initial content
invowk tui write --title "Edit message:" --value "Initial text here"`,
  },

  'tui/write-git-commit': {
    language: 'bash',
    code: `MESSAGE=$(invowk tui write --title "Commit message:")
git commit -m "$MESSAGE"`,
  },

  'tui/write-yaml-config': {
    language: 'bash',
    code: `CONFIG=$(invowk tui write --title "Enter YAML config:" --show-line-numbers)
echo "$CONFIG" > config.yaml`,
  },

  'tui/write-release-notes': {
    language: 'bash',
    code: `NOTES=$(invowk tui write --title "Release notes:")
gh release create v1.0.0 --notes "$NOTES"`,
  },

  'tui/write-script': {
    language: 'cue',
    code: `{
    name: "commit"
    description: "Interactive commit with editor"
    implementations: [{
        script: """
            # Show staged changes
            git diff --cached --stat
            
            # Get commit message
            MESSAGE=$(invowk tui write --title "Commit message:")
            
            if [ -z "$MESSAGE" ]; then
                echo "Commit cancelled (empty message)"
                exit 1
            fi
            
            git commit -m "$MESSAGE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/input-empty-handling': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi`,
  },

  'tui/input-validation': {
    language: 'bash',
    code: `while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done`,
  },

  'tui/input-default-value': {
    language: 'bash',
    code: `# Use shell default if empty
NAME=$(invowk tui input --title "Name:" --placeholder "Anonymous")
NAME="\${NAME:-Anonymous}"`,
  },

  // Choose component
  'tui/choose-basic': {
    language: 'bash',
    code: `invowk tui choose "Option 1" "Option 2" "Option 3"`,
  },

  'tui/choose-single': {
    language: 'bash',
    code: `# Basic
COLOR=$(invowk tui choose red green blue)
echo "You chose: $COLOR"

# With title
ENV=$(invowk tui choose --title "Select environment" dev staging prod)`,
  },

  'tui/choose-multiple': {
    language: 'bash',
    code: `# Limited multi-select (up to 3)
ITEMS=$(invowk tui choose --limit 3 "One" "Two" "Three" "Four" "Five")

# Unlimited multi-select
ITEMS=$(invowk tui choose --no-limit "One" "Two" "Three" "Four" "Five")`,
  },

  'tui/choose-multi-process': {
    language: 'bash',
    code: `SERVICES=$(invowk tui choose --no-limit --title "Select services to deploy" \\
    api web worker scheduler)

echo "$SERVICES" | while read -r service; do
    echo "Deploying: $service"
done`,
  },

  'tui/choose-preselected': {
    language: 'bash',
    code: `invowk tui choose --selected "Two" "One" "Two" "Three"`,
  },

  'tui/choose-env-selection': {
    language: 'bash',
    code: `ENV=$(invowk tui choose --title "Deploy to which environment?" \\
    development staging production)

case "$ENV" in
    production)
        if ! invowk tui confirm "Are you sure? This is PRODUCTION!"; then
            exit 1
        fi
        ;;
esac

./deploy.sh "$ENV"`,
  },

  'tui/choose-service-selection': {
    language: 'bash',
    code: `SERVICES=$(invowk tui choose --no-limit --title "Which services?" \\
    api web worker cron)

for service in $SERVICES; do
    echo "Restarting $service..."
    systemctl restart "$service"
done`,
  },

  // Confirm component
  'tui/confirm-basic': {
    language: 'bash',
    code: `invowk tui confirm "Are you sure?"`,
  },

  'tui/confirm-examples': {
    language: 'bash',
    code: `# Basic confirmation
if invowk tui confirm "Continue?"; then
    echo "Continuing..."
else
    echo "Cancelled."
fi

# Custom labels
if invowk tui confirm --affirmative "Delete" --negative "Cancel" "Delete all files?"; then
    rm -rf ./temp/*
fi

# Default to yes (user just presses Enter)
if invowk tui confirm --default "Proceed with defaults?"; then
    echo "Using defaults..."
fi`,
  },

  'tui/confirm-conditional': {
    language: 'bash',
    code: `# Simple pattern
invowk tui confirm "Run tests?" && npm test

# Negation
invowk tui confirm "Skip build?" || npm run build`,
  },

  'tui/confirm-dangerous': {
    language: 'bash',
    code: `# Double confirmation for dangerous actions
if invowk tui confirm "Delete production database?"; then
    echo "This cannot be undone!" | invowk tui style --foreground "#FF0000" --bold
    if invowk tui confirm --affirmative "YES, DELETE IT" --negative "No, abort" "Type to confirm:"; then
        ./scripts/delete-production-db.sh
    fi
fi`,
  },

  'tui/confirm-script': {
    language: 'cue',
    code: `{
    name: "clean"
    description: "Clean build artifacts"
    implementations: [{
        script: """
            echo "This will delete:"
            echo "  - ./build/"
            echo "  - ./dist/"
            echo "  - ./node_modules/"
            
            if invowk tui confirm "Proceed with cleanup?"; then
                rm -rf build/ dist/ node_modules/
                echo "Cleaned!" | invowk tui style --foreground "#00FF00"
            else
                echo "Cancelled."
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/choose-confirm-combined': {
    language: 'bash',
    code: `ACTION=$(invowk tui choose --title "Select action" \\
    "Deploy to staging" \\
    "Deploy to production" \\
    "Rollback" \\
    "Cancel")

case "$ACTION" in
    "Cancel")
        exit 0
        ;;
    "Deploy to production")
        if ! invowk tui confirm --affirmative "Yes, deploy" "Deploy to PRODUCTION?"; then
            echo "Aborted."
            exit 1
        fi
        ;;
esac

echo "Executing: $ACTION"`,
  },

  'tui/multistep-wizard': {
    language: 'bash',
    code: `# Step 1: Choose action
ACTION=$(invowk tui choose --title "What would you like to do?" \\
    "Create new project" \\
    "Import existing" \\
    "Exit")

if [ "$ACTION" = "Exit" ]; then
    exit 0
fi

# Step 2: Get details
NAME=$(invowk tui input --title "Project name:")

# Step 3: Confirm
echo "Action: $ACTION"
echo "Name: $NAME"

if invowk tui confirm "Create project?"; then
    # proceed
fi`,
  },

  // Filter component
  'tui/filter-basic': {
    language: 'bash',
    code: `# From arguments
invowk tui filter "apple" "banana" "cherry" "date"

# From stdin
ls | invowk tui filter`,
  },

  'tui/filter-examples': {
    language: 'bash',
    code: `# Filter files
FILE=$(ls | invowk tui filter --title "Select file")

# Multi-select filter
FILES=$(ls *.go | invowk tui filter --no-limit --title "Select Go files")

# With placeholder
ITEM=$(invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3)`,
  },

  'tui/filter-git-branch': {
    language: 'bash',
    code: `BRANCH=$(git branch --list | tr -d '* ' | invowk tui filter --title "Checkout branch")
git checkout "$BRANCH"`,
  },

  'tui/filter-docker-container': {
    language: 'bash',
    code: `CONTAINER=$(docker ps --format "{{.Names}}" | invowk tui filter --title "Select container")
docker logs -f "$CONTAINER"`,
  },

  'tui/filter-kill-process': {
    language: 'bash',
    code: `PID=$(ps aux | invowk tui filter --title "Select process" | awk '{print $2}')
if [ -n "$PID" ]; then
    kill "$PID"
fi`,
  },

  'tui/filter-commands': {
    language: 'bash',
    code: `CMD=$(invowk cmd --list 2>/dev/null | grep "^  " | invowk tui filter --title "Run command")
# Extract command name and run it`,
  },

  // File component
  'tui/file-basic': {
    language: 'bash',
    code: `# Pick any file from current directory
invowk tui file

# Start in specific directory
invowk tui file /home/user/documents`,
  },

  'tui/file-examples': {
    language: 'bash',
    code: `# Pick a file
FILE=$(invowk tui file)
echo "Selected: $FILE"

# Only directories
DIR=$(invowk tui file --directory)

# Show hidden files
FILE=$(invowk tui file --hidden)

# Filter by extension
FILE=$(invowk tui file --allowed ".go,.md,.txt")

# Multiple extensions
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml")`,
  },

  'tui/file-config': {
    language: 'bash',
    code: `CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json" /etc/myapp)
echo "Using config: $CONFIG"
./myapp --config "$CONFIG"`,
  },

  'tui/file-project-dir': {
    language: 'bash',
    code: `PROJECT=$(invowk tui file --directory ~/projects)
cd "$PROJECT"
code .`,
  },

  'tui/file-script-run': {
    language: 'bash',
    code: `SCRIPT=$(invowk tui file --allowed ".sh,.bash" ./scripts)
if [ -n "$SCRIPT" ]; then
    chmod +x "$SCRIPT"
    "$SCRIPT"
fi`,
  },

  'tui/file-log': {
    language: 'bash',
    code: `LOG=$(invowk tui file --allowed ".log" /var/log)
less "$LOG"`,
  },

  'tui/file-script': {
    language: 'cue',
    code: `{
    name: "edit-config"
    description: "Edit a configuration file"
    implementations: [{
        script: """
            CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml" ./config)
            
            if [ -z "$CONFIG" ]; then
                echo "No file selected."
                exit 0
            fi
            
            # Open in default editor
            \${EDITOR:-vim} "$CONFIG"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/filter-file-search-edit': {
    language: 'bash',
    code: `# Find file by content, then pick from results
FILE=$(grep -l "TODO" *.go 2>/dev/null | invowk tui filter --title "Select file with TODOs")
if [ -n "$FILE" ]; then
    vim "$FILE"
fi`,
  },

  'tui/filter-file-dir-then-file': {
    language: 'bash',
    code: `# First pick directory
DIR=$(invowk tui file --directory ~/projects)

# Then pick file in that directory
FILE=$(invowk tui file "$DIR" --allowed ".go")

echo "Selected: $FILE"`,
  },

  // Table component
  'tui/table-basic': {
    language: 'bash',
    code: `# From a CSV file
invowk tui table --file data.csv

# From stdin with separator
echo -e "name|age|city\\nAlice|30|NYC\\nBob|25|LA" | invowk tui table --separator "|"`,
  },

  'tui/table-examples': {
    language: 'bash',
    code: `# Display CSV
invowk tui table --file users.csv

# Custom separator (TSV)
invowk tui table --file data.tsv --separator $'\\t'

# Pipe-separated
cat data.txt | invowk tui table --separator "|"`,
  },

  'tui/table-selectable': {
    language: 'bash',
    code: `# Select a row
SELECTED=$(invowk tui table --file servers.csv --selectable)
echo "Selected: $SELECTED"`,
  },

  'tui/table-servers': {
    language: 'bash',
    code: `# servers.csv:
# name,ip,status
# web-1,10.0.0.1,running
# web-2,10.0.0.2,running
# db-1,10.0.0.3,stopped

invowk tui table --file servers.csv`,
  },

  'tui/table-ssh': {
    language: 'bash',
    code: `# Select a server
SERVER=$(cat servers.csv | invowk tui table --selectable | cut -d',' -f2)
ssh "user@$SERVER"`,
  },

  'tui/table-process': {
    language: 'bash',
    code: `ps aux --no-headers | awk '{print $1","$2","$11}' | \\
    (echo "USER,PID,COMMAND"; cat) | \\
    invowk tui table --selectable`,
  },

  // Spin component
  'tui/spin-basic': {
    language: 'bash',
    code: `invowk tui spin --title "Installing..." -- npm install`,
  },

  'tui/spin-types': {
    language: 'bash',
    code: `invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file
invowk tui spin --type moon --title "Building..." -- make build
invowk tui spin --type pulse --title "Testing..." -- npm test`,
  },

  'tui/spin-examples': {
    language: 'bash',
    code: `# Basic spinner
invowk tui spin --title "Building..." -- go build ./...

# With specific type
invowk tui spin --type dot --title "Installing dependencies..." -- npm install

# Long-running task
invowk tui spin --title "Compiling assets..." -- webpack --mode production`,
  },

  'tui/spin-chained': {
    language: 'bash',
    code: `echo "Step 1/3: Dependencies"
invowk tui spin --title "Installing..." -- npm install

echo "Step 2/3: Build"
invowk tui spin --title "Building..." -- npm run build

echo "Step 3/3: Tests"
invowk tui spin --title "Testing..." -- npm test

echo "Done!" | invowk tui style --foreground "#00FF00" --bold`,
  },

  'tui/spin-exit-code': {
    language: 'bash',
    code: `if invowk tui spin --title "Testing..." -- npm test; then
    echo "Tests passed!"
else
    echo "Tests failed!"
    exit 1
fi`,
  },

  'tui/spin-script': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy with progress indication"
    implementations: [{
        script: """
            echo "Deploying application..."
            
            invowk tui spin --title "Building Docker image..." -- \\
                docker build -t myapp .
            
            invowk tui spin --title "Pushing to registry..." -- \\
                docker push myapp
            
            invowk tui spin --title "Updating Kubernetes..." -- \\
                kubectl rollout restart deployment/myapp
            
            invowk tui spin --title "Waiting for rollout..." -- \\
                kubectl rollout status deployment/myapp
            
            echo "Deployment complete!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/spin-select-execute': {
    language: 'bash',
    code: `# Choose what to build
PROJECT=$(invowk tui choose --title "Build which project?" api web worker)

# Build with spinner
invowk tui spin --title "Building $PROJECT..." -- make "build-$PROJECT"`,
  },

  'tui/table-spin-combined': {
    language: 'bash',
    code: `# Select server
SERVER=$(invowk tui table --file servers.csv --selectable | cut -d',' -f1)

# Restart with spinner
invowk tui spin --title "Restarting $SERVER..." -- ssh "$SERVER" "systemctl restart myapp"`,
  },

  // Format component
  'tui/format-basic': {
    language: 'bash',
    code: `echo "# Hello World" | invowk tui format --type markdown`,
  },

  'tui/format-markdown': {
    language: 'bash',
    code: `# From stdin
echo "# Heading\\n\\nSome **bold** and *italic* text" | invowk tui format --type markdown

# From file
cat README.md | invowk tui format --type markdown`,
  },

  'tui/format-code': {
    language: 'bash',
    code: `# Specify language
cat main.go | invowk tui format --type code --language go

# Python
cat script.py | invowk tui format --type code --language python

# JavaScript
cat app.js | invowk tui format --type code --language javascript`,
  },

  'tui/format-emoji': {
    language: 'bash',
    code: `echo "Hello :wave: World :smile:" | invowk tui format --type emoji
# Output: Hello \ud83d\udc4b World \ud83d\ude04`,
  },

  'tui/format-readme': {
    language: 'bash',
    code: `cat README.md | invowk tui format --type markdown`,
  },

  'tui/format-diff': {
    language: 'bash',
    code: `git diff | invowk tui format --type code --language diff`,
  },

  'tui/format-welcome': {
    language: 'bash',
    code: `echo ":rocket: Welcome to MyApp :sparkles:" | invowk tui format --type emoji`,
  },

  // Style component
  'tui/style-basic': {
    language: 'bash',
    code: `invowk tui style --foreground "#FF0000" "Red text"`,
  },

  'tui/style-colors': {
    language: 'bash',
    code: `# Hex colors
invowk tui style --foreground "#FF0000" "Red"
invowk tui style --foreground "#00FF00" "Green"
invowk tui style --foreground "#0000FF" "Blue"

# With background
invowk tui style --foreground "#FFFFFF" --background "#FF0000" "White on Red"`,
  },

  'tui/style-decorations': {
    language: 'bash',
    code: `# Bold
invowk tui style --bold "Bold text"

# Italic
invowk tui style --italic "Italic text"

# Combined
invowk tui style --bold --italic --underline "All decorations"

# Dimmed
invowk tui style --faint "Subtle text"`,
  },

  'tui/style-piping': {
    language: 'bash',
    code: `echo "Important message" | invowk tui style --bold --foreground "#FF0000"`,
  },

  'tui/style-borders': {
    language: 'bash',
    code: `# Simple border
invowk tui style --border normal "Boxed text"

# Rounded border
invowk tui style --border rounded "Rounded box"

# Double border
invowk tui style --border double "Double border"

# With padding
invowk tui style --border rounded --padding-left 2 --padding-right 2 "Padded"`,
  },

  'tui/style-layout': {
    language: 'bash',
    code: `# Fixed width
invowk tui style --width 40 --align center "Centered"

# With margins
invowk tui style --margin-left 4 "Indented text"

# Box with all options
invowk tui style \\
    --border rounded \\
    --foreground "#FFFFFF" \\
    --background "#333333" \\
    --padding-left 2 \\
    --padding-right 2 \\
    --width 50 \\
    --align center \\
    "Styled Box"`,
  },

  'tui/style-messages': {
    language: 'bash',
    code: `# Success
echo "Build successful!" | invowk tui style --foreground "#00FF00" --bold

# Error
echo "Build failed!" | invowk tui style --foreground "#FF0000" --bold

# Warning
echo "Deprecated feature" | invowk tui style --foreground "#FFA500" --italic`,
  },

  'tui/style-headers': {
    language: 'bash',
    code: `# Main header
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""

# Subheader
invowk tui style --foreground "#888888" "Configuration Options:"`,
  },

  'tui/style-boxes': {
    language: 'bash',
    code: `# Info box
invowk tui style \\
    --border rounded \\
    --foreground "#FFFFFF" \\
    --background "#0066CC" \\
    --padding-left 1 \\
    --padding-right 1 \\
    "Info: Server is running on port 3000"

# Warning box
invowk tui style \\
    --border rounded \\
    --foreground "#000000" \\
    --background "#FFCC00" \\
    --padding-left 1 \\
    --padding-right 1 \\
    "Warning: API key will expire soon"`,
  },

  'tui/style-script': {
    language: 'cue',
    code: `{
    name: "status"
    description: "Show system status"
    implementations: [{
        script: """
            invowk tui style --bold --foreground "#00BFFF" "System Status"
            echo ""
            
            # Check services
            if systemctl is-active nginx > /dev/null 2>&1; then
                echo "nginx: " | tr -d '\\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "nginx: " | tr -d '\\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            
            if systemctl is-active postgresql > /dev/null 2>&1; then
                echo "postgres: " | tr -d '\\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "postgres: " | tr -d '\\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/format-style-combined': {
    language: 'bash',
    code: `# Header
invowk tui style --bold --foreground "#FFD700" "Package Info"
echo ""

# Render package description as markdown
cat package.md | invowk tui format --type markdown`,
  },

  'tui/interactive-styled': {
    language: 'bash',
    code: `NAME=$(invowk tui input --title "Project name:")

if invowk tui confirm "Create $NAME?"; then
    invowk tui spin --title "Creating..." -- mkdir -p "$NAME"
    echo "" 
    invowk tui style --foreground "#00FF00" --bold "Created $NAME successfully!"
else
    invowk tui style --foreground "#FF0000" "Cancelled"
fi`,
  },

  // =============================================================================
  // CLI REFERENCE - ADDITIONAL
  // =============================================================================

  'cli/cmd-examples': {
    language: 'bash',
    code: `# List all available commands
invowk cmd --list
invowk cmd -l

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test.unit

# Run with a specific runtime
invowk cmd build --runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'cli/init-examples': {
    language: 'bash',
    code: `# Create a default invkfile
invowk init

# Create a minimal invkfile
invowk init --template minimal

# Overwrite existing invkfile
invowk init --force`,
  },

  'cli/config-examples': {
    language: 'bash',
    code: `# Set container engine
invowk config set container_engine podman

# Set default runtime
invowk config set default_runtime virtual

# Set nested value
invowk config set ui.color_scheme dark`,
  },

  'cli/pack-examples': {
    language: 'bash',
    code: `# Create a pack with RDNS naming
invowk pack create com.example.mytools

# Basic validation
invowk pack validate ./mypack.invkpack

# Deep validation
invowk pack validate ./mypack.invkpack --deep`,
  },

  'cli/completion-all': {
    language: 'bash',
    code: `# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression`,
  },

  // =============================================================================
  // REFERENCE - CLI
  // =============================================================================

  'reference/cli/invowk-syntax': {
    language: 'bash',
    code: `invowk [flags]
invowk [command]`,
  },

  'reference/cli/cmd-syntax': {
    language: 'bash',
    code: `invowk cmd [flags]
invowk cmd [command-name] [flags] [-- args...]`,
  },

  'reference/cli/cmd-examples': {
    language: 'bash',
    code: `# List all available commands
invowk cmd --list
invowk cmd -l

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test.unit

# Run with a specific runtime
invowk cmd build --runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'reference/cli/init-syntax': {
    language: 'bash',
    code: `invowk init [flags]`,
  },

  'reference/cli/init-examples': {
    language: 'bash',
    code: `# Create a default invkfile
invowk init

# Create a minimal invkfile
invowk init --template minimal

# Overwrite existing invkfile
invowk init --force`,
  },

  'reference/cli/config-syntax': {
    language: 'bash',
    code: `invowk config [command]`,
  },

  'reference/cli/config-show-syntax': {
    language: 'bash',
    code: `invowk config show`,
  },

  'reference/cli/config-dump-syntax': {
    language: 'bash',
    code: `invowk config dump`,
  },

  'reference/cli/config-path-syntax': {
    language: 'bash',
    code: `invowk config path`,
  },

  'reference/cli/config-init-syntax': {
    language: 'bash',
    code: `invowk config init`,
  },

  'reference/cli/config-set-syntax': {
    language: 'bash',
    code: `invowk config set <key> <value>`,
  },

  'reference/cli/config-set-examples': {
    language: 'bash',
    code: `# Set container engine
invowk config set container_engine podman

# Set default runtime
invowk config set default_runtime virtual

# Set nested value
invowk config set ui.color_scheme dark`,
  },

  'reference/cli/pack-syntax': {
    language: 'bash',
    code: `invowk pack [command]`,
  },

  'reference/cli/pack-create-syntax': {
    language: 'bash',
    code: `invowk pack create <name> [flags]`,
  },

  'reference/cli/pack-create-examples': {
    language: 'bash',
    code: `# Create a pack with RDNS naming
invowk pack create com.example.mytools`,
  },

  'reference/cli/pack-validate-syntax': {
    language: 'bash',
    code: `invowk pack validate <path> [flags]`,
  },

  'reference/cli/pack-validate-examples': {
    language: 'bash',
    code: `# Basic validation
invowk pack validate ./mypack.invkpack

# Deep validation
invowk pack validate ./mypack.invkpack --deep`,
  },

  'reference/cli/pack-list-syntax': {
    language: 'bash',
    code: `invowk pack list`,
  },

  'reference/cli/pack-archive-syntax': {
    language: 'bash',
    code: `invowk pack archive <path> [flags]`,
  },

  'reference/cli/pack-import-syntax': {
    language: 'bash',
    code: `invowk pack import <source> [flags]`,
  },

  'reference/cli/tui-syntax': {
    language: 'bash',
    code: `invowk tui [command] [flags]`,
  },

  'reference/cli/tui-input-syntax': {
    language: 'bash',
    code: `invowk tui input [flags]`,
  },

  'reference/cli/tui-write-syntax': {
    language: 'bash',
    code: `invowk tui write [flags]`,
  },

  'reference/cli/tui-choose-syntax': {
    language: 'bash',
    code: `invowk tui choose [options...] [flags]`,
  },

  'reference/cli/tui-confirm-syntax': {
    language: 'bash',
    code: `invowk tui confirm [prompt] [flags]`,
  },

  'reference/cli/tui-filter-syntax': {
    language: 'bash',
    code: `invowk tui filter [options...] [flags]`,
  },

  'reference/cli/tui-file-syntax': {
    language: 'bash',
    code: `invowk tui file [path] [flags]`,
  },

  'reference/cli/tui-table-syntax': {
    language: 'bash',
    code: `invowk tui table [flags]`,
  },

  'reference/cli/tui-spin-syntax': {
    language: 'bash',
    code: `invowk tui spin [flags] -- command [args...]`,
  },

  'reference/cli/tui-pager-syntax': {
    language: 'bash',
    code: `invowk tui pager [file] [flags]`,
  },

  'reference/cli/tui-format-syntax': {
    language: 'bash',
    code: `invowk tui format [text...] [flags]`,
  },

  'reference/cli/tui-style-syntax': {
    language: 'bash',
    code: `invowk tui style [text...] [flags]`,
  },

  'reference/cli/completion-syntax': {
    language: 'bash',
    code: `invowk completion [shell]`,
  },

  'reference/cli/completion-examples': {
    language: 'bash',
    code: `# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression`,
  },

  'reference/cli/help-syntax': {
    language: 'bash',
    code: `invowk help [command]`,
  },

  'reference/cli/help-examples': {
    language: 'bash',
    code: `invowk help
invowk help cmd
invowk help config set`,
  },

  // =============================================================================
  // REFERENCE - INVKFILE SCHEMA
  // =============================================================================

  'reference/invkfile/root-structure': {
    language: 'cue',
    code: `#Invkfile: {
    group:          string    // Required - prefix for all command names
    version?:       string    // Optional - schema version (e.g., "1.0")
    description?:   string    // Optional - describe this invkfile's purpose
    default_shell?: string    // Optional - override default shell
    workdir?:       string    // Optional - default working directory
    env?:           #EnvConfig      // Optional - global environment
    depends_on?:    #DependsOn      // Optional - global dependencies
    cmds:           [...#Command]   // Required - at least one command
}`,
  },

  'reference/invkfile/group-examples': {
    language: 'cue',
    code: `// Valid group names
group: "build"
group: "my.project"
group: "com.example.tools"

// Invalid
group: "123abc"     // Can't start with a number
group: ".build"     // Can't start with a dot
group: "build."     // Can't end with a dot
group: "my..tools"  // Can't have consecutive dots`,
  },

  'reference/invkfile/version-example': {
    language: 'cue',
    code: `version: "1.0"`,
  },

  'reference/invkfile/description-example': {
    language: 'cue',
    code: `description: "Build and deployment commands for the web application"`,
  },

  'reference/invkfile/default-shell-example': {
    language: 'cue',
    code: `default_shell: "/bin/bash"
default_shell: "pwsh"`,
  },

  'reference/invkfile/workdir-example': {
    language: 'cue',
    code: `workdir: "./src"
workdir: "/opt/app"`,
  },

  'reference/invkfile/command-structure': {
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

  'reference/invkfile/command-name-examples': {
    language: 'cue',
    code: `name: "build"
name: "test unit"     // Spaces allowed for subcommand-like behavior
name: "deploy-prod"`,
  },

  'reference/invkfile/command-description-example': {
    language: 'cue',
    code: `description: "Build the application for production"`,
  },

  'reference/invkfile/implementation-structure': {
    language: 'cue',
    code: `#Implementation: {
    script:      string       // Required - inline script or file path
    target:      #Target      // Required - runtime and platform constraints
    env?:        #EnvConfig   // Optional
    workdir?:    string       // Optional
    depends_on?: #DependsOn   // Optional
}`,
  },

  'reference/invkfile/script-examples': {
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

  'reference/invkfile/target-structure': {
    language: 'cue',
    code: `#Target: {
    runtimes:   [...#RuntimeConfig]   // Required - at least one
    platforms?: [...#PlatformConfig]  // Optional
}`,
  },

  'reference/invkfile/runtimes-examples': {
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
    image: "golang:1.22"
    volumes: ["./:/app"]
}]`,
  },

  'reference/invkfile/platforms-example': {
    language: 'cue',
    code: `// Linux and macOS only
platforms: [
    {name: "linux"},
    {name: "macos"},
]`,
  },

  'reference/invkfile/runtime-config-structure': {
    language: 'cue',
    code: `#RuntimeConfig: {
    name: "native" | "virtual" | "container"
    
    // For native and container:
    interpreter?: string
    
    // For container only:
    enable_host_ssh?: bool
    containerfile?:   string
    image?:           string
    volumes?:         [...string]
    ports?:           [...string]
}`,
  },

  'reference/invkfile/interpreter-examples': {
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

  'reference/invkfile/enable-host-ssh-example': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "debian:bookworm-slim"
    enable_host_ssh: true
}]`,
  },

  'reference/invkfile/containerfile-image-examples': {
    language: 'cue',
    code: `// Use a pre-built image
image: "debian:bookworm-slim"
image: "golang:1.22"

// Build from a Containerfile
containerfile: "./Containerfile"
containerfile: "./docker/Dockerfile.build"`,
  },

  'reference/invkfile/volumes-example': {
    language: 'cue',
    code: `volumes: [
    "./src:/app/src",
    "/tmp:/tmp:ro",
    "\${HOME}/.cache:/cache",
]`,
  },

  'reference/invkfile/ports-example': {
    language: 'cue',
    code: `ports: [
    "8080:80",
    "3000:3000",
]`,
  },

  'reference/invkfile/platform-config-structure': {
    language: 'cue',
    code: `#PlatformConfig: {
    name: "linux" | "macos" | "windows"
}`,
  },

  'reference/invkfile/env-config-structure': {
    language: 'cue',
    code: `#EnvConfig: {
    files?: [...string]         // Dotenv files to load
    vars?:  [string]: string    // Environment variables
}`,
  },

  'reference/invkfile/env-files-example': {
    language: 'cue',
    code: `env: {
    files: [
        ".env",
        ".env.local",
        ".env.\${ENVIRONMENT}?",  // '?' means optional
    ]
}`,
  },

  'reference/invkfile/env-vars-example': {
    language: 'cue',
    code: `env: {
    vars: {
        NODE_ENV: "production"
        DEBUG: "false"
    }
}`,
  },

  'reference/invkfile/depends-on-structure': {
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

  'reference/invkfile/tool-dependency-structure': {
    language: 'cue',
    code: `#ToolDependency: {
    alternatives: [...string]  // At least one - tool names
}`,
  },

  'reference/invkfile/tool-dependency-example': {
    language: 'cue',
    code: `depends_on: {
    tools: [
        {alternatives: ["go"]},
        {alternatives: ["podman", "docker"]},  // Either works
    ]
}`,
  },

  'reference/invkfile/command-dependency-structure': {
    language: 'cue',
    code: `#CommandDependency: {
    alternatives: [...string]  // Command names
}`,
  },

  'reference/invkfile/filepath-dependency-structure': {
    language: 'cue',
    code: `#FilepathDependency: {
    alternatives: [...string]  // File/directory paths
    readable?:    bool
    writable?:    bool
    executable?:  bool
}`,
  },

  'reference/invkfile/capability-dependency-structure': {
    language: 'cue',
    code: `#CapabilityDependency: {
    alternatives: [...("local-area-network" | "internet")]
}`,
  },

  'reference/invkfile/env-var-dependency-structure': {
    language: 'cue',
    code: `#EnvVarDependency: {
    alternatives: [...#EnvVarCheck]
}

#EnvVarCheck: {
    name:        string    // Environment variable name
    validation?: string    // Regex pattern
}`,
  },

  'reference/invkfile/custom-check-dependency-structure': {
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

  'reference/invkfile/flag-structure': {
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

  'reference/invkfile/flag-example': {
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

  'reference/invkfile/argument-structure': {
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

  'reference/invkfile/argument-example': {
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

  'reference/invkfile/complete-example': {
    language: 'cue',
    code: `group: "myapp"
version: "1.0"
description: "Build and deployment commands"

env: {
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
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            },
            {
                script: """
                    $flags = if ($env:INVOWK_FLAG_RELEASE -eq "true") { "-ldflags=-s -w" } else { "" }
                    go build $flags -o app.exe .
                    """
                target: {
                    runtimes: [{name: "native", interpreter: "pwsh"}]
                    platforms: [{name: "windows"}]
                }
            },
        ]
        
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
    },
]`,
  },

  // =============================================================================
  // REFERENCE - CONFIG SCHEMA
  // =============================================================================

  'reference/config/schema-definition': {
    language: 'cue',
    code: `// Root configuration structure
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}

// Virtual shell configuration
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

// UI configuration
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}`,
  },

  'reference/config/config-structure': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}`,
  },

  'reference/config/container-engine-example': {
    language: 'cue',
    code: `container_engine: "podman"`,
  },

  'reference/config/search-paths-example': {
    language: 'cue',
    code: `search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
    "/opt/company/invowk-commands",
]`,
  },

  'reference/config/default-runtime-example': {
    language: 'cue',
    code: `default_runtime: "virtual"`,
  },

  'reference/config/virtual-shell-example': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'reference/config/ui-example': {
    language: 'cue',
    code: `ui: {
    color_scheme: "dark"
    verbose: false
}`,
  },

  'reference/config/virtual-shell-config-structure': {
    language: 'cue',
    code: `#VirtualShellConfig: {
    enable_uroot_utils?: bool
}`,
  },

  'reference/config/enable-uroot-utils-example': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'reference/config/ui-config-structure': {
    language: 'cue',
    code: `#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}`,
  },

  'reference/config/color-scheme-example': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
}`,
  },

  'reference/config/verbose-example': {
    language: 'cue',
    code: `ui: {
    verbose: true
}`,
  },

  'reference/config/complete-example': {
    language: 'cue',
    code: `// Invowk Configuration File
// =========================
// Location: ~/.config/invowk/config.cue

// Container Engine
// ----------------
// Which container runtime to use: "podman" or "docker"
// If not specified, Invowk auto-detects (prefers Podman)
container_engine: "podman"

// Search Paths
// ------------
// Additional directories to search for invkfiles and packs
// Searched in order after the current directory
search_paths: [
    // Personal commands
    "~/.invowk/cmds",
    
    // Team shared commands
    "~/work/shared-commands",
    
    // Organization-wide commands
    "/opt/company/invowk-commands",
]

// Default Runtime
// ---------------
// The runtime to use when a command doesn't specify one
// Options: "native", "virtual", "container"
default_runtime: "native"

// Virtual Shell Configuration
// ---------------------------
// Settings for the virtual shell runtime (mvdan/sh)
virtual_shell: {
    // Enable u-root utilities for more shell commands
    // Provides ls, cat, grep, etc. in the virtual environment
    enable_uroot_utils: true
}

// UI Configuration
// ----------------
// User interface settings
ui: {
    // Color scheme: "auto", "dark", or "light"
    // "auto" detects from terminal settings
    color_scheme: "auto"
    
    // Enable verbose output by default
    // Same as always passing --verbose
    verbose: false
}`,
  },

  'reference/config/minimal-example': {
    language: 'cue',
    code: `// Just override what you need
container_engine: "docker"`,
  },

  'reference/config/empty-example': {
    language: 'cue',
    code: `// Empty config - use all defaults`,
  },

  'reference/config/cue-validate': {
    language: 'bash',
    code: `cue vet ~/.config/invowk/config.cue`,
  },

  'reference/config/invowk-validate': {
    language: 'bash',
    code: `invowk config show`,
  },

  // =============================================================================
  // TUI - PAGER COMPONENT
  // =============================================================================

  'tui/pager-basic': {
    language: 'bash',
    code: `# View a file
invowk tui pager README.md

# Pipe content
cat long-output.txt | invowk tui pager`,
  },

  'tui/pager-options': {
    language: 'bash',
    code: `# With title
invowk tui pager --title "Log Output" app.log

# Show line numbers
invowk tui pager --line-numbers main.go

# Soft wrap long lines
invowk tui pager --soft-wrap document.txt

# Combine options
git log | invowk tui pager --title "Git History" --line-numbers`,
  },

  'tui/pager-script': {
    language: 'cue',
    code: `{
    name: "view-logs"
    description: "View application logs interactively"
    implementations: [{
        script: """
            # Get recent logs and display in pager
            journalctl -u myapp --no-pager -n 500 | \\
                invowk tui pager --title "Application Logs" --soft-wrap
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'tui/pager-code-review': {
    language: 'bash',
    code: `# Review code with line numbers
invowk tui pager --line-numbers --title "Code Review" src/main.go

# View diff output
git diff HEAD~5 | invowk tui pager --title "Recent Changes"`,
  },

  // =============================================================================
  // INTERACTIVE MODE
  // =============================================================================

  'interactive/basic-usage': {
    language: 'bash',
    code: `# Run a command in interactive mode
invowk cmd myproject build --interactive

# Short form
invowk cmd myproject build -i`,
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
invowk cmd deploy --interactive

# Commands with sudo
invowk cmd system-update -i

# SSH sessions
invowk cmd remote-shell -i

# Any command with interactive input
invowk cmd database-cli -i`,
  },

  'interactive/embedded-tui': {
    language: 'cue',
    code: `{
    name: "interactive-setup"
    description: "Setup with embedded TUI prompts"
    implementations: [{
        script: """
            # When run with -i, TUI components appear as overlays
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" api cli library)
            
            if invowk tui confirm "Create $TYPE project '$NAME'?"; then
                mkdir -p "$NAME"
                echo "Created $NAME"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}`,
  },

  'interactive/key-bindings-executing': {
    language: 'text',
    code: `During command execution:
  All keys      → Forwarded to running command
  Ctrl+\\        → Emergency quit (force exit)`,
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
} as const;

// Type-safe snippet IDs
export type SnippetId = keyof typeof snippets;

// Helper to get all snippet IDs for documentation/tooling
export const snippetIds = Object.keys(snippets) as SnippetId[];
