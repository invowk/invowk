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
 * - modules/* - Snippets for modules section
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

  'getting-started/invowkfile-basic-structure': {
    language: 'cue',
    code: `// invowkfile.cue (commands only)
cmds: [                  // Required: list of commands
    // ... your commands here
]`,
  },

  'getting-started/go-project-full': {
    language: 'cue',
    code: `cmds: [
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
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
                runtimes: [{name: "native"}, {name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
    code: `// Root-level env applies to ALL commands
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
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
    code: `invowk cmd`,
  },

  'cli/run-command': {
    language: 'bash',
    code: `invowk cmd build`,
  },

  'cli/run-subcommands': {
    language: 'bash',
    code: `invowk cmd test unit
invowk cmd test coverage`,
  },

  'cli/runtime-override': {
    language: 'bash',
    code: `# Use the default (native)
invowk cmd test unit

# Explicitly use virtual runtime
invowk cmd test unit --ivk-runtime virtual`,
  },

  'cli/cue-validate': {
    language: 'bash',
    code: `cue vet invowkfile.cue path/to/invowkfile_schema.cue -d '#Invowkfile'`,
  },

  // =============================================================================
  // CLI OUTPUT EXAMPLES
  // =============================================================================

  'cli/output-list-commands': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*]
  test unit - Run unit tests [native*, virtual]
  test coverage - Run tests with coverage [native*]
  clean - Remove build artifacts [native*] (linux, macos)`,
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
  // RUNTIME MODES
  // =============================================================================

  'runtime-modes/native-basic': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }
    ]
}`,
  },

  'runtime-modes/native-run': {
    language: 'bash',
    code: `invowk cmd build`,
  },

  'runtime-modes/native-default-shell': {
    language: 'cue',
    code: `default_shell: "/bin/bash"

cmds: [
    // Commands...
]`,
  },

  'runtime-modes/native-bash-script': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            #!/bin/bash
            set -euo pipefail  # Bash strict mode
            
            # Bash-specific features
            declare -A config=(
                ["env"]="production"
                ["debug"]="false"
            )
            
            echo "Building for \${config[env]}..."
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-powershell-script': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            $ErrorActionPreference = "Stop"
            
            Write-Host "Building..." -ForegroundColor Green
            dotnet build --configuration Release
            Write-Host "Done!" -ForegroundColor Green
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "windows"}]
    }]
}`,
  },

  'runtime-modes/native-cmd-script': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: """
            @echo off
            echo Building...
            msbuild /p:Configuration=Release
            echo Done!
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "windows"}]
    }]
}`,
  },

  'runtime-modes/native-shebang-script': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import json

            print(f"Python {sys.version}")
            data = {"status": "ok", "items": [1, 2, 3]}
            print(json.dumps(data, indent=2))
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-explicit-interpreter': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: """
            import sys
            print(f"Hello from Python {sys.version_info.major}!")
            """
        runtimes: [{
            name: "native"
            interpreter: "python3"  // Explicit interpreter
        }]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-interpreter-args': {
    language: 'cue',
    code: `{
    name: "script"
    implementations: [{
        script: """
            print("Unbuffered output!")
            """
        runtimes: [{
            name: "native"
            interpreter: "python3 -u"  // With arguments
        }]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-env-vars': {
    language: 'cue',
    code: `{
    name: "deploy"
    env: {
        vars: {
            DEPLOY_ENV: "production"
        }
    }
    implementations: [{
        script: """
            echo "Home: $HOME"
            echo "User: $USER"
            echo "Deploy to: $DEPLOY_ENV"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-flags-args': {
    language: 'cue',
    code: `{
    name: "greet"
    flags: [
        {name: "loud", description: "Use uppercase greeting", type: "bool", default_value: "false"}
    ]
    args: [
        {name: "name", description: "Name to greet", default_value: "World"}
    ]
    implementations: [{
        script: """
            if [ "$INVOWK_FLAG_LOUD" = "true" ]; then
                echo "HELLO, $INVOWK_ARG_NAME!"
            else
                echo "Hello, $INVOWK_ARG_NAME!"
            fi
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'runtime-modes/native-flags-run': {
    language: 'bash',
    code: `invowk cmd greet Alice --loud
# Output: HELLO, ALICE!`,
  },

  'runtime-modes/native-workdir': {
    language: 'cue',
    code: `{
    name: "build frontend"
    workdir: "./frontend"  // Run in frontend subdirectory
    implementations: [{
        script: "npm run build"
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}`,
  },

  'runtime-modes/native-deps': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        tools: [
            {alternatives: ["docker", "podman"]},
            {alternatives: ["kubectl"]}
        ]
        filepaths: [
            {alternatives: ["Dockerfile"]}
        ]
    }
    implementations: [{
        script: "docker build -t myapp . && kubectl apply -f k8s/"
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
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
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}`,
  },

  'runtime-modes/virtual-run': {
    language: 'bash',
    code: `invowk cmd build --ivk-runtime virtual`,
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
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}`,
  },

  'runtime-modes/virtual-args': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [{name: "name", description: "Name to greet", default_value: "World"}]
    implementations: [{
        script: """
            # Using environment variable
            echo "Hello, $INVOWK_ARG_NAME!"

            # Or positional parameter
            echo "Hello, $1!"
            """
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{
            name: "virtual"
            interpreter: "python3"  // ERROR: Not supported
        }]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "virtual"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
            runtimes: [{
                name: "container"
                image: "golang:1.26"
            }]
            platforms: [{name: "linux"}]
        }
    ]
}`,
  },

  'runtime-modes/container-volumes': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "golang:1.26"
    volumes: [
        "./src:/workspace/src",
        "./data:/data:ro"
    ]
}]`,
  },

  'runtime-modes/container-env': {
    language: 'cue',
    code: `// env belongs on the implementation, not on the runtime
implementations: [{
    script: "node index.js"
    runtimes: [{name: "container", image: "node:20"}]
    platforms: [{name: "linux"}]
    env: {
        vars: {
            NODE_ENV: "production"
            DEBUG: "app:*"
        }
    }
}]`,
  },

  'runtime-modes/container-workdir': {
    language: 'cue',
    code: `// workdir belongs on the implementation, not on the runtime
implementations: [{
    script: "python main.py"
    runtimes: [{name: "container", image: "python:3-slim"}]
    platforms: [{name: "linux"}]
    workdir: "/app"
}]`,
  },

  'runtime-modes/container-ssh': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "debian:stable-slim"
    enable_host_ssh: true
}]`,
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
        {alternatives: [".env"], readable: true}
    ]
}`,
  },

  'dependencies/commands-basic': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        {alternatives: ["build"]}
    ]
}`,
  },

  'dependencies/commands-alternatives': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        // Either command being discoverable satisfies this dependency
        {alternatives: ["build debug", "build release"]},
    ]
}`,
  },

  'dependencies/commands-multiple': {
    language: 'cue',
    code: `depends_on: {
    cmds: [
        {alternatives: ["build"]},
        {alternatives: ["test unit", "test integration"]},
    ]
}`,
  },

  'dependencies/commands-cross-invowkfile': {
    language: 'cue',
    code: `depends_on: {
    cmds: [{alternatives: ["shared generate-types"]}]
}`,
  },

  'dependencies/commands-workflow': {
    language: 'bash',
    code: `invowk cmd build && invowk cmd deploy`,
  },

  'dependencies/capabilities-basic': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["internet"]},
        {alternatives: ["local-area-network"]}
    ]
}`,
  },

  'dependencies/capabilities-containers': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["containers"]}
    ]
}`,
  },

  'dependencies/capabilities-tty': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["tty"]}
    ]
}`,
  },

  'dependencies/env-vars-basic': {
    language: 'cue',
    code: `depends_on: {
    env_vars: [
        {alternatives: [{name: "API_KEY"}]},
        {alternatives: [{name: "DATABASE_URL"}, {name: "DB_URL"}]}
    ]
}`,
  },

  'dependencies/custom-checks': {
    language: 'cue',
    code: `depends_on: {
    custom_checks: [
        {
            alternatives: [{
                name: "docker-running"
                check_script: "docker info > /dev/null 2>&1"
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
        OVERRIDE_VALUE: "from-invowkfile"
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'environment/scope-root': {
    language: 'cue',
    code: `env: {
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
    code: `// Platform-specific env requires separate implementations
implementations: [
    {
        script: "echo $CONFIG_PATH"
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}]
        env: {vars: {CONFIG_PATH: "/etc/myapp/config"}}
    },
    {
        script: "echo $CONFIG_PATH"
        runtimes: [{name: "native"}]
        platforms: [{name: "macos"}]
        env: {vars: {CONFIG_PATH: "/usr/local/etc/myapp/config"}}
    }
]`,
  },

  'environment/cli-overrides': {
    language: 'bash',
    code: `# Set a single variable
invowk cmd build --ivk-env-var NODE_ENV=development

# Set multiple variables
invowk cmd build --ivk-env-var NODE_ENV=dev --ivk-env-var DEBUG=true

# Load from a file
invowk cmd build --ivk-env-file .env.local

# Combine
invowk cmd build --ivk-env-file .env.local --ivk-env-var OVERRIDE=value`,
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
        runtimes: [{name: "container", image: "debian:stable-slim"}]
        platforms: [{name: "linux"}]
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
├── invowkfile.cue
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
    code: `env: {
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
invowk cmd build --ivk-env-file .env.custom

# Multiple files
invowk cmd build --ivk-env-file .env.custom --ivk-env-file .env.secrets`,
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
├── invowkfile.cue
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
    code: `env: {
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
        },
        {
            script: "go build ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
    code: `// Platform-specific env requires separate implementations
implementations: [
    {
        script: "echo $PLATFORM_CONFIG"
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}]
        env: {
            vars: {
                PLATFORM_CONFIG: "/etc/myapp"
                PLATFORM_NAME: "Linux"
            }
        }
    },
    {
        script: "echo $PLATFORM_CONFIG"
        runtimes: [{name: "native"}]
        platforms: [{name: "macos"}]
        env: {
            vars: {
                PLATFORM_CONFIG: "/usr/local/etc/myapp"
                PLATFORM_NAME: "macOS"
            }
        }
    },
    {
        script: "echo %PLATFORM_CONFIG%"
        runtimes: [{name: "native"}]
        platforms: [{name: "windows"}]
        env: {
            vars: {
                PLATFORM_CONFIG: "%APPDATA%\\\\myapp"
                PLATFORM_NAME: "Windows"
            }
        }
    }
]`,
  },

  'environment/env-vars-combined-files': {
    language: 'cue',
    code: `env: {
    files: [".env"]  // Loaded first
    vars: {
        // These override .env values
        OVERRIDE: "from-invowkfile"
    }
}`,
  },

  'environment/env-vars-cli-override': {
    language: 'bash',
    code: `# Single variable
invowk cmd build --ivk-env-var NODE_ENV=development

# Multiple variables
invowk cmd build --ivk-env-var NODE_ENV=dev --ivk-env-var DEBUG=true --ivk-env-var PORT=8080`,
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "container", image: "golang:1.26"}]
        platforms: [{name: "linux"}]
    }]
}`,
  },

  // Precedence page snippets
  'environment/precedence-hierarchy': {
    language: 'text',
    code: `CLI (highest priority)
├── --ivk-env-var KEY=value
└── --ivk-env-file .env.local
    │
Invowk Vars
├── INVOWK_FLAG_*
└── INVOWK_ARG_*
    │
Vars (by scope, highest to lowest)
├── Implementation env.vars
├── Command env.vars
└── Root env.vars
    │
Files (by scope, highest to lowest)
├── Implementation env.files
├── Command env.files
└── Root env.files
    │
System Environment (lowest priority)`,
  },

  'environment/precedence-invowkfile': {
    language: 'cue',
    code: `// Root level
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
    code: `invowk cmd build --ivk-env-var API_URL=http://cli.example.com`,
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
    code: `// Platform-specific env requires separate implementations
implementations: [
    {
        script: "echo $CONFIG_PATH"
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}]
        env: {
            vars: {
                CONFIG_PATH: "/etc/app"
                OTHER_VAR: "value"
            }
        }
    },
    {
        script: "echo $CONFIG_PATH"
        runtimes: [{name: "native"}]
        platforms: [{name: "macos"}]
        env: {
            vars: {
                CONFIG_PATH: "/usr/local/etc/app"
                OTHER_VAR: "value"
            }
        }
    }
]`,
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
    runtimes: [{name: "container", image: "node:20"}]
    platforms: [{name: "linux"}]
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
invowk cmd build --ivk-env-var DEBUG=true --ivk-env-var LOG_LEVEL=debug`,
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
        {name: "dry-run", description: "Preview without applying changes", type: "bool", default_value: "false"},
        {name: "replicas", description: "Number of replicas", type: "int", default_value: "1"},
    ]

    // Arguments - positional values
    args: [
        {name: "environment", description: "Target environment", required: true},
        {name: "services", description: "Services to deploy", variadic: true},
    ]
    
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENVIRONMENT"
            echo "Replicas: $INVOWK_FLAG_REPLICAS"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Services: $INVOWK_ARG_SERVICES"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/overview-usage': {
    language: 'bash',
    code: `invowk cmd deploy production api web --dry-run --replicas=3`,
  },

  'flags-args/overview-flags-example': {
    language: 'cue',
    code: `flags: [
    {name: "verbose", description: "Enable verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output directory", type: "string", short: "o", default_value: "./dist"},
    {name: "count", description: "Number of iterations", type: "int", default_value: "1"},
]`,
  },

  'flags-args/overview-flags-usage': {
    language: 'bash',
    code: `# Long form
invowk cmd build --verbose --output=./build --count=5

# Short form
invowk cmd build -v -o=./build`,
  },

  'flags-args/overview-args-example': {
    language: 'cue',
    code: `args: [
    {name: "source", description: "Source directory", required: true},
    {name: "destination", description: "Destination path", default_value: "./output"},
    {name: "files", description: "Files to copy", variadic: true},
]`,
  },

  'flags-args/overview-args-usage': {
    language: 'bash',
    code: `invowk cmd copy ./src ./dest file1.txt file2.txt`,
  },

  'flags-args/overview-shell-positional': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name", description: "First name"},
        {name: "last-name", description: "Last name"},
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/overview-mixing': {
    language: 'bash',
    code: `# All equivalent
invowk cmd deploy production --dry-run api web
invowk cmd deploy --dry-run production api web
invowk cmd deploy production api web --dry-run`,
  },

  'flags-args/overview-help': {
    language: 'bash',
    code: `invowk cmd deploy --help`,
  },

  'flags-args/overview-help-output': {
    language: 'text',
    code: `Usage:
  invowk cmd deploy <environment> [services]... [flags]

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
    code: `invowk cmd run --message="Hello World"`,
  },

  'flags-args/flags-type-bool': {
    language: 'cue',
    code: `{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"}`,
  },

  'flags-args/flags-type-bool-usage': {
    language: 'bash',
    code: `# Enable
invowk cmd run --verbose
invowk cmd run --verbose=true

# Disable (explicit)
invowk cmd run --verbose=false`,
  },

  'flags-args/flags-type-int': {
    language: 'cue',
    code: `{name: "count", description: "Number of iterations", type: "int", default_value: "5"}`,
  },

  'flags-args/flags-type-int-usage': {
    language: 'bash',
    code: `invowk cmd run --count=10
invowk cmd run --count=-1  # Negative allowed`,
  },

  'flags-args/flags-type-float': {
    language: 'cue',
    code: `{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.95"}`,
  },

  'flags-args/flags-type-float-usage': {
    language: 'bash',
    code: `invowk cmd run --threshold=0.8
invowk cmd run --threshold=1.5e-3  # Scientific notation`,
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
invowk cmd deploy
# Error: flag 'target' is required

# Success
invowk cmd deploy --target=production`,
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
invowk cmd request

# Override
invowk cmd request --timeout=60`,
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
invowk cmd build --verbose --output=./dist --force

# Short form
invowk cmd build -v -o=./dist -f

# Mixed
invowk cmd build -v --output=./dist -f`,
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
invowk cmd deploy --env=prod --version=1.2.3

# Invalid - fails before execution
invowk cmd deploy --env=production
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
    code: `invowk cmd copy ./src ./dest`,
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
    code: `invowk cmd generate 5`,
  },

  'flags-args/args-type-float': {
    language: 'cue',
    code: `{name: "ratio", description: "Scaling ratio", type: "float", default_value: "1.0"}`,
  },

  'flags-args/args-type-float-usage': {
    language: 'bash',
    code: `invowk cmd scale 0.5`,
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
invowk cmd convert input.txt
# Error: argument 'output' is required

# Success
invowk cmd convert input.txt output.txt`,
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
invowk cmd parse input.txt

# Override format
invowk cmd parse input.txt yaml`,
  },

  'flags-args/args-ordering': {
    language: 'cue',
    code: `// Good
args: [
    {name: "input", description: "Input file", required: true},      // Required first
    {name: "output", description: "Output file", required: true},     // Required second
    {name: "format", description: "Output format", default_value: "json"}, // Optional last
]

// Bad - will cause validation error
args: [
    {name: "format", description: "Output format", default_value: "json"}, // Optional can't come first
    {name: "input", description: "Input file", required: true},
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-variadic-usage': {
    language: 'bash',
    code: `invowk cmd process out.txt a.txt b.txt c.txt
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
invowk cmd deploy prod 1.2.3

# Invalid
invowk cmd deploy production
# Error: argument 'environment' value 'production' does not match pattern '^(dev|staging|prod)$'`,
  },

  'flags-args/args-accessing': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [
        {name: "first-name", description: "First name", required: true},
        {name: "last-name", description: "Last name", default_value: "User"},
    ]
    implementations: [{
        script: """
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            """
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-positional-params': {
    language: 'cue',
    code: `{
    name: "copy"
    args: [
        {name: "source", description: "Source path", required: true},
        {name: "dest", description: "Destination path", required: true},
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'flags-args/args-mixing-flags': {
    language: 'bash',
    code: `# All equivalent
invowk cmd deploy prod 3 --dry-run
invowk cmd deploy --dry-run prod 3
invowk cmd deploy prod --dry-run 3`,
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
            script: """
                import json
                import sys

                data = json.load(open('data.json'))
                print(f"Found {len(data)} records")
                """
            runtimes: [{name: "native", interpreter: "python3"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
            script: """
                const fs = require('fs');
                const data = JSON.parse(fs.readFileSync('data.json'));
                console.log(\`Processing \${data.length} items\`);
                """
            runtimes: [{name: "native", interpreter: "node"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: "open $URL"
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: "start $URL"
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{
            name: "native"
            interpreter: "python3"  // Explicit
        }]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{
            name: "native"
            interpreter: "python3 -u"  // Unbuffered output
        }]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{
            name: "container"
            image: "python:3-slim"
        }]
        platforms: [{name: "linux"}]
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
        runtimes: [{
            name: "container"
            image: "node:20-slim"
            interpreter: "node"
        }]
        platforms: [{name: "linux"}]
    }]
}`,
  },

  'advanced/interpreter-args-access': {
    language: 'cue',
    code: `{
    name: "greet"
    args: [{name: "name", description: "Name to greet", default_value: "World"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{
            name: "virtual"
            interpreter: "python3"  // ERROR!
        }]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            workdir: "./web"  // This implementation runs in ./web
        },
        {
            script: "go build ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            workdir: "./api"  // This implementation runs in ./api
        }
    ]
}`,
  },

  'advanced/workdir-root': {
    language: 'cue',
    code: `workdir: "./src"  // All commands default to ./src

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

  'advanced/workdir-cli': {
    language: 'bash',
    code: `invowk cmd build --ivk-workdir ./frontend`,
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
    code: `// --ivk-workdir (CLI) > implementation > command > root
workdir: "./root"  // Lowest: ./root

cmds: [
    {
        name: "build"
        workdir: "./command"  // Override: ./command
        implementations: [
            {
                script: "make"
                workdir: "./implementation"  // Override: ./implementation
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
    }
]`,
  },

  'advanced/workdir-monorepo': {
    language: 'cue',
    code: `cmds: [
    {
        name: "web build"
        workdir: "./packages/web"
        implementations: [{
            script: "npm run build"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "api build"
        workdir: "./packages/api"
        implementations: [{
            script: "go build ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "mobile build"
        workdir: "./packages/mobile"
        implementations: [{
            script: "flutter build"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "container", image: "debian:stable-slim"}]
        platforms: [{name: "linux"}]
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
        runtimes: [{name: "container", image: "node:20"}]
        platforms: [{name: "linux"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "start backend"
        workdir: "./backend"
        implementations: [{
            script: "go run ./cmd/server"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "test integration"
        workdir: "./tests/integration"
        implementations: [{
            script: "pytest"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "test e2e"
        workdir: "./tests/e2e"
        implementations: [{
            script: "cypress run"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: "open http://localhost:3000"
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: "start http://localhost:3000"
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'advanced/platform-env': {
    language: 'cue',
    code: `{
    name: "configure"
    implementations: [
        // Linux implementation
        {
            script: "echo \\"Config: \$CONFIG_PATH\\""
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
            env: {
                vars: {
                    CONFIG_PATH: "/etc/myapp/config.yaml"
                    CACHE_DIR: "/var/cache/myapp"
                }
            }
        },
        // macOS implementation
        {
            script: "echo \\"Config: \$CONFIG_PATH\\""
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
            env: {
                vars: {
                    CONFIG_PATH: "/usr/local/etc/myapp/config.yaml"
                    CACHE_DIR: "\${HOME}/Library/Caches/myapp"
                }
            }
        },
        // Windows implementation
        {
            script: "echo \\"Config: %CONFIG_PATH%\\""
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
            env: {
                vars: {
                    CONFIG_PATH: "%APPDATA%\\\\myapp\\\\config.yaml"
                    CACHE_DIR: "%LOCALAPPDATA%\\\\myapp\\\\cache"
                }
            }
        }
    ]
}`,
  },

  'advanced/platform-cross-script': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        // Unix platforms (same output name)
        {
            script: "go build -o bin/app ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows (different output name)
        {
            script: "go build -o bin/app.exe ./..."
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
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
                runtimes: [{name: "native"}]
                platforms: _unix
            },
            // Windows implementation
            {
                script: "rmdir /s /q build"
                runtimes: [{name: "native"}]
                platforms: [_windows]
            }
        ]
    }
]`,
  },

  'advanced/platform-list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*] (linux, macos, windows)
  clean - Clean artifacts [native*] (linux, macos)
  deploy - Deploy to cloud [native*] (linux)`,
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: """
                echo "Hostname: \$(hostname)"
                echo "Kernel: \$(uname -r)"
                echo "Memory: \$(sysctl -n hw.memsize | awk '{print \$0/1024/1024/1024 "GB"}')"
                """
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: """
                echo Hostname: %COMPUTERNAME%
                systeminfo | findstr "Total Physical Memory"
                """
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: "brew install coreutils"
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: "choco install make"
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
}`,
  },

  // =============================================================================
  // MODULES
  // =============================================================================

  'modules/create': {
    language: 'bash',
    code: `invowk module create com.example.mytools`,
  },

  'modules/validate': {
    language: 'bash',
    code: `invowk module validate ./mymod.invowkmod --deep`,
  },

  'modules/module-invowkfile': {
    language: 'cue',
    code: `cmds: [
    {
        name: "lint"
        description: "Run linters"
        implementations: [
            {
                script: "./scripts/lint.sh"
                runtimes: [{name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
    }
]`,
  },

  // Overview page snippets
  'modules/structure-basic': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue          # Required: module metadata
├── invowkfile.cue          # Optional: command definitions
├── scripts/              # Optional: script files
│   ├── build.sh
│   └── deploy.sh
└── templates/             # Optional: other resources
    └── config.yaml`,
  },

  'modules/quick-create': {
    language: 'bash',
    code: `invowk module create mytools`,
  },

  'modules/quick-create-output': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue
└── invowkfile.cue`,
  },

'modules/quick-use': {
    language: 'bash',
    code: `# List commands (module commands appear automatically)
invowk cmd

# Run a module command
invowk cmd mytools hello`,
  },

  'modules/quick-share': {
    language: 'bash',
    code: `# Create a zip archive
invowk module archive mytools.invowkmod

# Share the zip file
# Recipients import with:
invowk module import mytools.invowkmod.zip`,
  },

  'modules/structure-example': {
    language: 'text',
    code: `com.example.devtools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
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

  'modules/rdns-naming': {
    language: 'text',
    code: `com.company.projectname.invowkmod
io.github.username.toolkit.invowkmod
org.opensource.utilities.invowkmod`,
  },

  'modules/script-paths': {
    language: 'cue',
    code: `// Inside mytools.invowkmod/invowkfile.cue
cmds: [
    {
        name: "build"
        implementations: [{
            script: "scripts/build.sh"  // Relative to module root
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "deploy"
        implementations: [{
            script: "scripts/utils/helpers.sh"  // Nested path
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    }
]`,
  },

  'modules/discovery-output': {
    language: 'text',
    code: `Available Commands

From invowkfile:
  mytools build - Build the project [native*]

From user modules (~/.invowk/cmds):
  com.example.utilities hello - Greeting [native*]`,
  },

  // Creating modules page snippets
  'modules/create-options': {
    language: 'bash',
    code: `# Simple module
invowk module create mytools

# RDNS naming
invowk module create com.company.devtools

# In specific directory
invowk module create mytools --path /path/to/modules

# Custom module ID + description
invowk module create mytools --module-id com.company.tools --description "Shared tools"

# With scripts directory
invowk module create mytools --scripts`,
  },

  'modules/create-with-scripts': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
└── scripts/`,
  },

  'modules/template-invowkmod': {
    language: 'cue',
    code: `module: "mytools"
version: "1.0.0"
description: "Commands for mytools"

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invowkmod.git"
//         version: "^1.0.0"
//     },
// ]`,
  },

  'modules/template-invowkfile': {
    language: 'cue',
    code: `cmds: [
    {
        name: "hello"
        description: "Say hello"
        implementations: [
            {
                script: """
                    echo "Hello from mytools!"
                    """
                runtimes: [{name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
    }
]`,
  },

  'modules/manual-create': {
    language: 'bash',
    code: `mkdir mytools.invowkmod
touch mytools.invowkmod/invowkmod.cue
touch mytools.invowkmod/invowkfile.cue`,
  },

  'modules/inline-vs-external': {
    language: 'cue',
    code: `cmds: [
    // Simple: inline script
    {
        name: "quick"
        implementations: [{
            script: "echo 'Quick task'"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },

    // Complex: external script
    {
        name: "complex"
        implementations: [{
            script: "scripts/complex-task.sh"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    }
]`,
  },

  'modules/script-organization': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
└── scripts/
    ├── build.sh           # Main scripts
    ├── deploy.sh
    ├── test.sh
    └── lib/               # Shared utilities
        ├── logging.sh
        └── validation.sh`,
  },

  'modules/script-paths-good-bad': {
    language: 'cue',
    code: `// Good
script: "scripts/build.sh"
script: "scripts/lib/logging.sh"

// Bad - will fail on some platforms
script: "scripts\\\\build.sh"

// Bad - escapes module directory
script: "../outside.sh"`,
  },

  'modules/env-files-structure': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
├── .env                   # Default config
├── .env.example           # Template for users
└── scripts/`,
  },

  'modules/env-files-ref': {
    language: 'cue',
    code: `env: {
    files: [".env"]
}`,
  },

  'modules/docs-structure': {
    language: 'text',
    code: `mytools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
├── README.md              # Usage instructions
├── CHANGELOG.md           # Version history
└── scripts/`,
  },

  'modules/buildtools-structure': {
    language: 'text',
    code: `com.company.buildtools.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
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

  'modules/buildtools-invowkfile': {
    language: 'cue',
    code: `cmds: [
    {
        name: "go"
        description: "Build Go project"
        implementations: [{
            script: "scripts/build-go.sh"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "node"
        description: "Build Node.js project"
        implementations: [{
            script: "scripts/build-node.sh"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "python"
        description: "Build Python project"
        implementations: [{
            script: "scripts/build-python.sh"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    }
]`,
  },

  'modules/devops-structure': {
    language: 'text',
    code: `org.devops.k8s.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
├── scripts/
│   ├── deploy.sh
│   ├── rollback.sh
│   └── status.sh
├── manifests/
│   ├── deployment.yaml
│   └── service.yaml
└── .env.example`,
  },

  'modules/validate-before-share': {
    language: 'bash',
    code: `invowk module validate mytools.invowkmod --deep`,
  },

  // Validating page snippets
  'modules/validate-basic': {
    language: 'bash',
    code: `invowk module validate ./mytools.invowkmod`,
  },

  'modules/validate-basic-output': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/mytools.invowkmod
• Name: mytools

✓ Module is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present`,
  },

  'modules/validate-deep': {
    language: 'bash',
    code: `invowk module validate ./mytools.invowkmod --deep`,
  },

  'modules/validate-deep-output': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/mytools.invowkmod
• Name: mytools

✓ Module is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
✓ Invowkfile parses successfully`,
  },

  'modules/error-missing-invowkfile': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/bad.invowkmod

✗ Module validation failed with 1 issue(s)

  1. [structure] missing required invowkmod.cue`,
  },

  'modules/error-invalid-name': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/my-tools.invowkmod

✗ Module validation failed with 1 issue(s)

  1. [naming] module name 'my-tools' contains invalid characters (hyphens not allowed)`,
  },

  'modules/error-nested': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/parent.invowkmod

✗ Module validation failed with 1 issue(s)

  1. [structure] nested.invowkmod: nested modules are not allowed (except in invowk_modules/)`,
  },

  'modules/error-parse': {
    language: 'text',
    code: `Module Validation
• Path: /home/user/broken.invowkmod

✗ Module validation failed with 1 issue(s)

  1. [invowkfile] parse error at line 15: expected '}', found EOF`,
  },

  'modules/validate-batch': {
    language: 'bash',
    code: `# Validate all modules in a directory
for mod in ./modules/*.invowkmod; do
    invowk module validate "$mod" --deep
done`,
  },

  'modules/validate-ci': {
    language: 'yaml',
    code: `# GitHub Actions example
- name: Validate modules
  run: |
    for mod in modules/*.invowkmod; do
      invowk module validate "$mod" --deep
    done`,
  },

  'modules/path-separators-good-bad': {
    language: 'cue',
    code: `// Bad - Windows-style
script: "scripts\\\\build.sh"

// Good - Forward slashes
script: "scripts/build.sh"`,
  },

  'modules/escape-module-dir': {
    language: 'cue',
    code: `// Bad - tries to access parent
script: "../outside/script.sh"

// Good - stays within module
script: "scripts/script.sh"`,
  },

  'modules/absolute-paths': {
    language: 'cue',
    code: `// Bad - absolute path
script: "/usr/local/bin/script.sh"

// Good - relative path
script: "scripts/script.sh"`,
  },

  // Distributing page snippets
  'modules/archive-basic': {
    language: 'bash',
    code: `# Default output: <module-name>.invowkmod.zip
invowk module archive ./mytools.invowkmod

# Custom output path
invowk module archive ./mytools.invowkmod --output ./dist/mytools.zip`,
  },

  'modules/archive-output': {
    language: 'text',
    code: `Archive Module

✓ Module archived successfully

• Output: /home/user/dist/mytools.zip
• Size: 2.45 KB`,
  },

  'modules/import-local': {
    language: 'bash',
    code: `# Install to ~/.invowk/cmds/
invowk module import ./mytools.invowkmod.zip

# Install to custom directory
invowk module import ./mytools.invowkmod.zip --path ./local-modules

# Overwrite existing
invowk module import ./mytools.invowkmod.zip --overwrite`,
  },

  'modules/import-url': {
    language: 'bash',
    code: `# Download and install
invowk module import https://example.com/modules/mytools.zip

# From GitHub release
invowk module import https://github.com/user/repo/releases/download/v1.0/mytools.invowkmod.zip`,
  },

  'modules/import-output': {
    language: 'text',
    code: `Import Module

✓ Module imported successfully

• Name: mytools
• Path: /home/user/.invowk/cmds/mytools.invowkmod

• The module commands are now available via invowk`,
  },

  'modules/list': {
    language: 'bash',
    code: `invowk module list`,
  },

  'modules/list-output': {
    language: 'text',
    code: `Discovered Modules

• Found 3 module(s)

• current directory:
   ✓ local.project
      /home/user/project/local.project.invowkmod

• user modules (~/.invowk/cmds):
   ✓ com.company.devtools
      /home/user/.invowk/cmds/com.company.devtools.invowkmod
   ✓ io.github.user.utilities
      /home/user/.invowk/cmds/io.github.user.utilities.invowkmod`,
  },

  'modules/git-structure': {
    language: 'text',
    code: `my-project/
├── src/
├── modules/
│   ├── devtools.invowkmod/
│   │   ├── invowkmod.cue
│   │   └── invowkfile.cue
│   └── testing.invowkmod/
│       ├── invowkmod.cue
│       └── invowkfile.cue
└── invowkfile.cue`,
  },

  'modules/github-release': {
    language: 'bash',
    code: `# Recipients install with:
invowk module import https://github.com/org/repo/releases/download/v1.0.0/mytools.invowkmod.zip`,
  },

  'modules/future-install': {
    language: 'bash',
    code: `invowk module install com.company.devtools@1.0.0`,
  },

  'modules/install-user': {
    language: 'bash',
    code: `invowk module import mytools.zip
# Installed to: ~/.invowk/cmds/mytools.invowkmod/`,
  },

  'modules/install-project': {
    language: 'bash',
    code: `invowk module import mytools.zip --path ./modules
# Installed to: ./modules/mytools.invowkmod/`,
  },

  'modules/includes-config': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
includes: [
    {path: "/shared/company-modules/tools.invowkmod"},
]`,
  },

  'modules/install-custom-path': {
    language: 'bash',
    code: `invowk module import mytools.zip --path /shared/company-modules`,
  },

  'modules/version-invowkfile': {
    language: 'cue',
    code: `module: "com.company.tools"
version: "1.2.0"`,
  },

  'modules/archive-versioned': {
    language: 'bash',
    code: `invowk module archive ./mytools.invowkmod --output ./dist/mytools-1.2.0.zip`,
  },

  'modules/upgrade-process': {
    language: 'bash',
    code: `# Remove old version
rm -rf ~/.invowk/cmds/mytools.invowkmod

# Install new version
invowk module import mytools-1.2.0.zip

# Or use --overwrite
invowk module import mytools-1.2.0.zip --overwrite`,
  },

  'modules/team-shared-location': {
    language: 'bash',
    code: `# Admin publishes
invowk module archive ./devtools.invowkmod --output /shared/modules/devtools.zip

# Team members import
invowk module import /shared/modules/devtools.zip`,
  },

  'modules/internal-server': {
    language: 'bash',
    code: `# Team members import via URL
invowk module import https://internal.company.com/modules/devtools.zip`,
  },

  'modules/workflow-example': {
    language: 'bash',
    code: `# 1. Create and develop module
invowk module create com.company.mytools --scripts
# ... add commands and scripts ...

# 2. Validate
invowk module validate ./com.company.mytools.invowkmod --deep

# 3. Create versioned archive
invowk module archive ./com.company.mytools.invowkmod \\
    --output ./releases/mytools-1.0.0.zip

# 4. Distribute (e.g., upload to GitHub release)

# 5. Team imports
invowk module import https://github.com/company/mytools/releases/download/v1.0.0/mytools-1.0.0.zip`,
  },

  // =============================================================================
  // MODULE DEPENDENCIES
  // =============================================================================

  'modules/dependencies/quick-add': {
    language: 'bash',
    code: `invowk module add https://github.com/example/common.invowkmod.git ^1.0.0`,
  },

  'modules/dependencies/quick-invowkmod': {
    language: 'cue',
    code: `module: "com.example.mytools"
version: "1.0.0"
description: "My tools"

requires: [
    {
        git_url: "https://github.com/example/common.invowkmod.git"
        version: "^1.0.0"
        alias: "common"
    },
]`,
  },

  'modules/dependencies/quick-sync': {
    language: 'bash',
    code: `invowk module sync`,
  },

  'modules/dependencies/quick-deps': {
    language: 'bash',
    code: `invowk module deps`,
  },

  'modules/dependencies/namespace-usage': {
    language: 'bash',
    code: `# Default namespace includes the resolved version
invowk cmd com.example.common@1.2.3 build

# With alias
invowk cmd common build`,
  },

  'modules/dependencies/cache-structure': {
    language: 'text',
    code: `~/.invowk/modules/
├── sources/
│   └── github.com/
│       └── example/
│           └── common.invowkmod/
└── github.com/
    └── example/
        └── common.invowkmod/
            └── 1.2.3/
                ├── invowkmod.cue
                └── invowkfile.cue`,
  },

  'modules/dependencies/cache-env': {
    language: 'bash',
    code: `export INVOWK_MODULES_PATH=/custom/cache/path`,
  },

  'modules/dependencies/basic-invowkmod': {
    language: 'cue',
    code: `module: "com.example.mytools"
version: "1.0.0"

requires: [
    {
        git_url: "https://github.com/example/common.invowkmod.git"
        version: "^1.0.0"
    },
]`,
  },

  'modules/dependencies/git-url-examples': {
    language: 'cue',
    code: `requires: [
    // HTTPS (works with public repos or GITHUB_TOKEN)
    {git_url: "https://github.com/user/tools.invowkmod.git", version: "^1.0.0"},

    // SSH (requires SSH key in ~/.ssh/)
    {git_url: "git@github.com:user/tools.invowkmod.git", version: "^1.0.0"},

    // GitLab
    {git_url: "https://gitlab.com/user/tools.invowkmod.git", version: "^1.0.0"},

    // Self-hosted
    {git_url: "https://git.example.com/user/tools.invowkmod.git", version: "^1.0.0"},
]`,
  },

  'modules/dependencies/version-example': {
    language: 'cue',
    code: `requires: [
    // Invowk tries both v1.0.0 and 1.0.0
    {git_url: "https://github.com/user/tools.invowkmod.git", version: "^1.0.0"},
]`,
  },

  'modules/dependencies/alias-example': {
    language: 'cue',
    code: `requires: [
    // Default namespace: common@1.2.3
    {git_url: "https://github.com/user/common.invowkmod.git", version: "^1.0.0"},

    // Custom namespace: tools
    {
        git_url: "https://github.com/user/common.invowkmod.git"
        version: "^1.0.0"
        alias: "tools"
    },
]`,
  },

  'modules/dependencies/alias-usage': {
    language: 'bash',
    code: `# Instead of: invowk cmd common@1.2.3 build
invowk cmd tools build`,
  },

  'modules/dependencies/path-example': {
    language: 'cue',
    code: `requires: [
    {
        git_url: "https://github.com/user/monorepo.invowkmod.git"
        version: "^1.0.0"
        path: "modules/cli-tools"
    },
    {
        git_url: "https://github.com/user/monorepo.invowkmod.git"
        version: "^1.0.0"
        path: "modules/deploy-utils"
        alias: "deploy"
    },
]`,
  },

  'modules/dependencies/multiple-requires': {
    language: 'cue',
    code: `requires: [
    {
        git_url: "https://github.com/company/build-tools.invowkmod.git"
        version: "^2.0.0"
        alias: "build"
    },
    {
        git_url: "https://github.com/company/deploy-tools.invowkmod.git"
        version: "~1.5.0"
        alias: "deploy"
    },
    {
        git_url: "https://github.com/company/test-utils.invowkmod.git"
        version: ">=1.0.0 <2.0.0"
    },
]`,
  },

  'modules/dependencies/auth-tokens': {
    language: 'bash',
    code: `# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab
export GITLAB_TOKEN=glpat-xxxx

# Generic (any Git server)
export GIT_TOKEN=your-token`,
  },

  'modules/dependencies/transitive-tree': {
    language: 'text',
    code: `com.example.app
├── common-tools@1.2.3
│   └── logging-utils@2.0.0
└── deploy-utils@1.5.0
    └── common-tools@1.2.3  (shared)`,
  },

  'modules/dependencies/circular-error': {
    language: 'text',
    code: `Error: circular dependency detected: https://github.com/user/module-a.invowkmod.git`,
  },

  'modules/dependencies/best-practices-version': {
    language: 'cue',
    code: `// Good: allows patch and minor updates
{git_url: "...", version: "^1.0.0"}

// Too strict: no updates allowed
{git_url: "...", version: "1.0.0"}`,
  },

  'modules/dependencies/best-practices-alias': {
    language: 'cue',
    code: `{
    git_url: "https://github.com/company/company-internal-build-tools.invowkmod.git"
    version: "^2.0.0"
    alias: "build"
}`,
  },

  'modules/dependencies/update-command': {
    language: 'bash',
    code: `invowk module update`,
  },

  'modules/dependencies/lockfile-example': {
    language: 'cue',
    code: `version: "1.0"
generated: "2025-01-10T12:34:56Z"

modules: {
    "https://github.com/example/common.invowkmod.git": {
        git_url:          "https://github.com/example/common.invowkmod.git"
        version:          "^1.0.0"
        resolved_version: "1.2.3"
        git_commit:       "abc123def4567890"
        alias:            "common"
        namespace:        "common"
    }
}`,
  },

  'modules/dependencies/lockfile-key': {
    language: 'cue',
    code: `modules: {
    "https://github.com/example/monorepo.invowkmod.git#modules/cli": {
        git_url: "https://github.com/example/monorepo.invowkmod.git"
        path:    "modules/cli"
    }
}`,
  },

  'modules/dependencies/lockfile-workflow': {
    language: 'bash',
    code: `# Resolve and lock
invowk module sync

# Commit the lock file
git add invowkmod.lock.cue
git commit -m "Lock module dependencies"`,
  },

  'modules/dependencies/cli/add-usage': {
    language: 'bash',
    code: `invowk module add <git-url> <version> [flags]`,
  },

  'modules/dependencies/cli/add-examples': {
    language: 'bash',
    code: `# Add a dependency with caret version
invowk module add https://github.com/user/mod.invowkmod.git ^1.0.0

# Add with SSH URL
invowk module add git@github.com:user/mod.invowkmod.git ~2.0.0

# Add with custom alias
invowk module add https://github.com/user/common.invowkmod.git ^1.0.0 --alias tools

# Add from monorepo subdirectory
invowk module add https://github.com/user/monorepo.invowkmod.git ^1.0.0 --path modules/cli`,
  },

  'modules/dependencies/cli/add-output': {
    language: 'text',
    code: `Add Module Dependency

ℹ Resolving https://github.com/user/mod.invowkmod.git@^1.0.0...
✓ Module resolved and lock file updated

ℹ Git URL:   https://github.com/user/mod.invowkmod.git
ℹ Version:   ^1.0.0 → 1.2.3
ℹ Namespace: mod@1.2.3
ℹ Cache:     /home/user/.invowk/modules/github.com/user/mod.invowkmod/1.2.3
✓ Updated invowkmod.cue with new requires entry`,
  },

  'modules/dependencies/cli/remove-usage': {
    language: 'bash',
    code: `invowk module remove <identifier>`,
  },

  'modules/dependencies/cli/remove-example': {
    language: 'bash',
    code: `invowk module remove https://github.com/user/mod.invowkmod.git`,
  },

  'modules/dependencies/cli/remove-output': {
    language: 'text',
    code: `Remove Module Dependency

ℹ Removing https://github.com/user/mod.invowkmod.git...
✓ Removed mod@1.2.3

✓ Lock file and invowkmod.cue updated`,
  },

  'modules/dependencies/cli/deps-usage': {
    language: 'bash',
    code: `invowk module deps`,
  },

  'modules/dependencies/cli/deps-output': {
    language: 'text',
    code: `Module Dependencies

• Found 2 module dependency(ies)

✓ build-tools@2.3.1
   Git URL:  https://github.com/company/build-tools.invowkmod.git
   Version:  ^2.0.0 → 2.3.1
   Commit:   abc123def456
   Cache:    /home/user/.invowk/modules/github.com/company/build-tools.invowkmod/2.3.1

✓ deploy-utils@1.5.2
   Git URL:  https://github.com/company/deploy-tools.invowkmod.git
   Version:  ~1.5.0 → 1.5.2
   Commit:   789xyz012abc
   Cache:    /home/user/.invowk/modules/github.com/company/deploy-tools.invowkmod/1.5.2`,
  },

  'modules/dependencies/cli/deps-empty': {
    language: 'text',
    code: `Module Dependencies

• No module dependencies found

• To add modules, use: invowk module add <git-url> <version>`,
  },

  'modules/dependencies/cli/sync-usage': {
    language: 'bash',
    code: `invowk module sync`,
  },

  'modules/dependencies/cli/sync-output': {
    language: 'text',
    code: `Sync Module Dependencies

• Found 2 requirement(s) in invowkmod.cue

✓ build-tools@2.3.1 → 2.3.1
✓ deploy-utils@1.5.2 → 1.5.2

✓ Lock file updated: invowkmod.lock.cue`,
  },

  'modules/dependencies/cli/sync-empty': {
    language: 'text',
    code: `Sync Module Dependencies

• No requires field found in invowkmod.cue`,
  },

  'modules/dependencies/cli/update-usage': {
    language: 'bash',
    code: `invowk module update [identifier]`,
  },

  'modules/dependencies/cli/update-examples': {
    language: 'bash',
    code: `# Update all modules
invowk module update

# Update a specific module
invowk module update https://github.com/user/mod.invowkmod.git`,
  },

  'modules/dependencies/cli/update-output': {
    language: 'text',
    code: `Update Module Dependencies

• Updating all modules...

✓ build-tools@2.3.1 → 2.4.0
✓ deploy-utils@1.5.2 → 1.5.3

✓ Lock file updated: invowkmod.lock.cue`,
  },

  'modules/dependencies/cli/update-empty': {
    language: 'text',
    code: `Update Module Dependencies

• Updating all modules...
• No modules to update`,
  },

  'modules/dependencies/cli/vendor-usage': {
    language: 'bash',
    code: `invowk module vendor [module-path]`,
  },

  'modules/dependencies/cli/vendor-output': {
    language: 'text',
    code: `Vendor Module Dependencies

• Found 2 requirement(s) in invowkmod.cue
• Vendor directory: /home/user/project/invowk_modules

! Vendoring is not yet fully implemented

• The following dependencies would be vendored:
   • https://github.com/example/common.invowkmod.git@^1.0.0
   • https://github.com/example/deploy.invowkmod.git@~1.5.0`,
  },

  'modules/dependencies/cli/workflow-init': {
    language: 'bash',
    code: `# 1. Resolve dependencies
invowk module add https://github.com/company/build-tools.invowkmod.git ^2.0.0 --alias build
invowk module add https://github.com/company/deploy-tools.invowkmod.git ~1.5.0 --alias deploy

# 2. Add requires to invowkmod.cue manually or verify
cat invowkmod.cue

# 3. Sync to generate lock file
invowk module sync

# 4. Commit the lock file
git add invowkmod.lock.cue
git commit -m "Add module dependencies"`,
  },

  'modules/dependencies/cli/workflow-fresh': {
    language: 'bash',
    code: `# On a fresh clone, sync downloads all modules
git clone https://github.com/yourorg/project.git
cd project
invowk module sync`,
  },

  'modules/dependencies/cli/workflow-update': {
    language: 'bash',
    code: `# Update all modules periodically
invowk module update

# Review changes
git diff invowkmod.lock.cue

# Commit if tests pass
git add invowkmod.lock.cue
git commit -m "Update module dependencies"`,
  },

  'modules/dependencies/cli/workflow-troubleshoot': {
    language: 'bash',
    code: `# List current dependencies to verify state
invowk module deps

# Re-sync if something seems wrong
invowk module sync

# Check commands are available
invowk cmd`,
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
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/usr/local/share/invowk/shared.invowkmod"},
]
default_runtime: "virtual"`,
  },

  'config/container-engine': {
    language: 'cue',
    code: `container_engine: "docker"  // or "podman"`,
  },

  'config/includes': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/opt/company/shared.invowkmod", alias: "company"},
]`,
  },

  'config/cli-custom-path': {
    language: 'bash',
    code: `# Use a project-local config
cp /path/to/config.cue ./config.cue
invowk config show`,
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

// Additional modules to include in discovery
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
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
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'config/schema': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    includes?: [...#IncludeEntry]
    default_runtime?: "native" | "virtual" | "container"
    virtual_shell?: #VirtualShellConfig
    ui?: #UIConfig
    container?: #ContainerConfig
}

#IncludeEntry: {
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
}

#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?: bool
    interactive?: bool
}

#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}

#AutoProvisionConfig: {
    enabled?: bool
    strict?: bool
    binary_path?: string
    includes?: [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?: string
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

  'config/container-auto-provision': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
}`,
  },

  'config/complete-example': {
    language: 'cue',
    code: `// Invowk Configuration File
// Located at: ~/.config/invowk/config.cue

// Use Podman as the container engine
container_engine: "podman"

// Additional modules to include in discovery
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},       // Personal modules
    {path: "/home/user/work/shared.invowkmod", alias: "team"},   // Team shared module
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
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'config/env-override-examples': {
    language: 'bash',
    code: `# Invowk does not currently support config overrides via env vars`,
  },

  'config/cli-override-examples': {
    language: 'bash',
    code: `# Enable verbose output for a run
invowk --ivk-verbose cmd build

# Run command in interactive mode (alternate screen buffer)
invowk --ivk-interactive cmd build

# Override runtime for a command
invowk cmd build --ivk-runtime container`,
  },

  // =============================================================================
  // INSTALLATION
  // =============================================================================

  'installation/shell-script': {
    language: 'bash',
    code: `curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh`,
  },

  'installation/shell-script-custom': {
    language: 'bash',
    code: `# Install to a custom directory
INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh

# Install a specific version
INVOWK_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh`,
  },

  'installation/powershell-script': {
    language: 'powershell',
    code: `irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex`,
  },

  'installation/powershell-script-custom': {
    language: 'powershell',
    code: `# Install to a custom directory
$env:INSTALL_DIR='C:\\tools\\invowk'; irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex

# Install a specific version
$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex

# Skip automatic PATH modification
$env:INVOWK_NO_MODIFY_PATH='1'; irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex`,
  },

  'installation/winget': {
    language: 'powershell',
    code: `winget install Invowk.Invowk`,
  },

  'installation/winget-upgrade': {
    language: 'powershell',
    code: `winget upgrade Invowk.Invowk`,
  },

  'installation/homebrew': {
    language: 'bash',
    code: `brew install invowk/tap/invowk`,
  },

  'installation/go-install': {
    language: 'bash',
    code: `go install github.com/invowk/invowk@latest`,
  },

  'installation/build-from-source': {
    language: 'bash',
    code: `git clone https://github.com/invowk/invowk
cd invowk
make build`,
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

  'quickstart/default-invowkfile': {
    language: 'cue',
    code: `// Invowkfile - Command definitions for invowk
// See https://github.com/invowk/invowk for documentation

cmds: [
    {
        name: "hello"
        description: "Print a greeting"
        implementations: [
            {
                script: "echo \\"Hello, $INVOWK_ARG_NAME!\\""
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            },
            {
                script: "Write-Output \\"Hello, $($env:INVOWK_ARG_NAME)!\\""
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            },
            {
                script: "echo \\"Hello, $INVOWK_ARG_NAME!\\""
                runtimes: [{name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            },
            {
                script: "echo \\"Hello from container, $INVOWK_ARG_NAME!\\""
                runtimes: [{name: "container", image: "debian:stable-slim"}]
                platforms: [{name: "linux"}]
            },
        ]
        args: [
            {name: "name", description: "Who to greet", default_value: "World"},
        ]
    },
]`,
  },

  'quickstart/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  hello - Print a greeting [native*, virtual, container] (linux, macos, windows)`,
  },

  'quickstart/run-hello': {
    language: 'bash',
    code: `invowk cmd hello`,
  },

  'quickstart/hello-output': {
    language: 'text',
    code: `Hello, World!`,
  },

  'quickstart/info-command': {
    language: 'bash',
    code: `# Pass an argument to the hello command
invowk cmd hello Alice

# Use a different runtime
invowk cmd hello --ivk-runtime virtual`,
  },

  'quickstart/run-info': {
    language: 'text',
    code: `Hello, Alice!`,
  },

  'quickstart/virtual-runtime': {
    language: 'cue',
    code: `// The virtual runtime uses the built-in mvdan/sh interpreter
// It works identically on Linux, macOS, and Windows
{
    script: "echo \\"Hello, $INVOWK_ARG_NAME!\\""
    runtimes: [{name: "virtual"}]
    platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
}`,
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

From user modules (~/.invowk/cmds):
  com.example.tools hello - A greeting [native*] (linux, macos)`,
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
            script: "echo \\"Deploying to \$PLATFORM with config at \$CONFIG_PATH\\""
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
            script: "echo \\"Deploying to \$PLATFORM with config at \$CONFIG_PATH\\""
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
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },

    // Virtual: uses built-in POSIX-compatible shell
    {
        name: "build virtual"
        implementations: [{
            script: "go build ./..."
            runtimes: [{name: "virtual"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },

    // Container: runs inside a container
    {
        name: "build container"
        implementations: [{
            script: "go build -o /workspace/bin/app ./..."
            runtimes: [{name: "container", image: "golang:1.26"}]
            platforms: [{name: "linux"}]
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
        runtimes: [
            {name: "native"},  // Default
            {name: "virtual"}, // Alternative
            {name: "container", image: "golang:1.26"}  // Reproducible
        ]
        platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
    }]
}`,
  },

  'runtime-modes/override-cli': {
    language: 'bash',
    code: `# Use default (native)
invowk cmd build

# Override to virtual
invowk cmd build --ivk-runtime virtual

# Override to container
invowk cmd build --ivk-runtime container`,
  },

  'runtime-modes/list-output': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*, virtual, container] (linux, macos)`,
  },

  'runtime-modes/container-containerfile': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    containerfile: "./Containerfile"  // Relative to invowkfile
}]`,
  },

  'runtime-modes/containerfile-example': {
    language: 'dockerfile',
    code: `FROM golang:1.26

RUN apt-get update && apt-get install -y \\
    make \\
    git

WORKDIR /workspace`,
  },

  'runtime-modes/container-ports': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "node:20"
    ports: [
        "3000:3000",      // Host:Container
        "8080:80"         // Map container port 80 to host port 8080
    ]
}]`,
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
        runtimes: [{
            name: "container"
            image: "python:3-slim"
            interpreter: "python3"
        }]
        platforms: [{name: "linux"}]
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
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
            enable_host_ssh: true  // Enable SSH server
        }]
        platforms: [{name: "linux"}]
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
        runtimes: [{
            name: "container"
            image: "golang:1.26"
            volumes: [
                "\${HOME}/go/pkg/mod:/go/pkg/mod:ro"  // Cache Go dependencies
            ]
        }]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  // =============================================================================
  // RUNTIME MODES - CONTAINER (extracted from inline blocks)
  // =============================================================================

  'runtime-modes/container-prebuilt-image': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "golang:1.26"
}]`,
  },

  'runtime-modes/container-volumes-full': {
    language: 'cue',
    code: `runtimes: [{
    name: "container"
    image: "golang:1.26"
    volumes: [
        "./data:/data",           // Relative path
        "/tmp:/tmp:ro",           // Absolute path, read-only
        "\${HOME}/.cache:/cache"   // Environment variable
    ]
}]`,
  },

  'runtime-modes/container-shebang': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        platforms: [{name: "linux"}]
        script: """
            #!/usr/bin/env python3
            import sys
            print(f"Python {sys.version} in container!")
            """
        runtimes: [{
            name: "container"
            image: "python:3-slim"
        }]
    }]
}`,
  },

  'runtime-modes/container-env-vars': {
    language: 'cue',
    code: `{
    name: "deploy"
    env: {
        vars: {
            DEPLOY_ENV: "production"
            API_URL: "https://api.example.com"
        }
    }
    implementations: [{
        platforms: [{name: "linux"}]
        script: """
            echo "Deploying to $DEPLOY_ENV"
            echo "API: $API_URL"
            """
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
        }]
    }]
}`,
  },

  'runtime-modes/container-dependencies': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [
            // Checked inside the container, not on host
            {alternatives: ["go"]},
            {alternatives: ["make"]}
        ]
        filepaths: [
            // Paths relative to container's /workspace
            {alternatives: ["go.mod"]}
        ]
    }
    implementations: [{
        platforms: [{name: "linux"}]
        script: "make build"
        runtimes: [{
            name: "container"
            image: "golang:1.26"
        }]
    }]
}`,
  },

  'runtime-modes/container-engine-config': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
container_engine: "podman"  // or "docker"`,
  },

  'runtime-modes/container-workdir-default': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        platforms: [{name: "linux"}]
        script: """
            pwd  # Outputs: /workspace
            ls   # Shows your project files
            """
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
        }]
    }]
}`,
  },

  'runtime-modes/container-workdir-override': {
    language: 'cue',
    code: `{
    name: "build frontend"
    workdir: "./frontend"  // Mounted and used as workdir
    implementations: [{
        platforms: [{name: "linux"}]
        script: "npm run build"
        runtimes: [{
            name: "container"
            image: "node:20"
        }]
    }]
}`,
  },

  'runtime-modes/container-interactive-tui': {
    language: 'cue',
    code: `{
    name: "deploy container"
    implementations: [{
        platforms: [{name: "linux"}]
        script: """
            # This TUI confirm appears as an overlay on your terminal
            if invowk tui confirm "Deploy to production?"; then
                echo "Deploying..."
                ./deploy.sh
            fi
            """
        runtimes: [{
            name: "container"
            image: "debian:stable-slim"
        }]
    }]
}`,
  },

  // =============================================================================
  // DEPENDENCIES - ADDITIONAL
  // =============================================================================

  'dependencies/without-check': {
    language: 'bash',
    code: `$ invowk cmd build
./scripts/build.sh: line 5: go: command not found`,
  },

  'dependencies/with-check': {
    language: 'text',
    code: `$ invowk cmd build

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
    code: `depends_on: {
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
            runtimes: [{name: "container", image: "golang:1.26"}]
            platforms: [{name: "linux"}]
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
    code: `// Root level: requires git
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
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
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
            {alternatives: ["build"]},
            {alternatives: ["test"]}
        ]
    }
    implementations: [
        {
            script: "./scripts/deploy.sh"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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

  'tui/overview-invowkfile-example': {
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
    code: `CMD=$(invowk cmd 2>/dev/null | grep "^  " | invowk tui filter --title "Run command")
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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
invowk cmd

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test unit

# Run with a specific runtime
invowk cmd build --ivk-runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'cli/init-examples': {
    language: 'bash',
    code: `# Create a default invowkfile
invowk init

# Create a minimal invowkfile
invowk init --template minimal

# Overwrite existing invowkfile
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

  'cli/module-examples': {
    language: 'bash',
    code: `# Create a module with RDNS naming
invowk module create com.example.mytools

# Basic validation
invowk module validate ./mymod.invowkmod

# Deep validation
invowk module validate ./mymod.invowkmod --deep`,
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
invowk cmd

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test unit

# Run with a specific runtime
invowk cmd build --ivk-runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'reference/cli/init-syntax': {
    language: 'bash',
    code: `invowk init [flags] [filename]`,
  },

  'reference/cli/init-examples': {
    language: 'bash',
    code: `# Create a default invowkfile
invowk init

# Create a minimal invowkfile
invowk init --template minimal

# Overwrite existing invowkfile
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

# Set UI options
invowk config set ui.color_scheme dark
invowk config set ui.verbose true
invowk config set ui.interactive false

# Set virtual shell options
invowk config set virtual_shell.enable_uroot_utils true`,
  },

  'reference/cli/module-syntax': {
    language: 'bash',
    code: `invowk module [command]`,
  },

  'reference/cli/module-create-syntax': {
    language: 'bash',
    code: `invowk module create <name> [flags]`,
  },

  'reference/cli/module-create-examples': {
    language: 'bash',
    code: `# Create a module with RDNS naming
invowk module create com.example.mytools

# Override module ID and description
invowk module create mytools --module-id com.example.tools --description "Shared tools"

# Create with scripts directory
invowk module create mytools --scripts`,
  },

  'reference/cli/module-validate-syntax': {
    language: 'bash',
    code: `invowk module validate <path> [flags]`,
  },

  'reference/cli/module-validate-examples': {
    language: 'bash',
    code: `# Basic validation
invowk module validate ./mymod.invowkmod

# Deep validation
invowk module validate ./mymod.invowkmod --deep`,
  },

  'reference/cli/module-list-syntax': {
    language: 'bash',
    code: `invowk module list`,
  },

  'reference/cli/module-archive-syntax': {
    language: 'bash',
    code: `invowk module archive <path> [flags]`,
  },

  'reference/cli/module-import-syntax': {
    language: 'bash',
    code: `invowk module import <source> [flags]`,
  },

  'reference/cli/module-add-syntax': {
    language: 'bash',
    code: `invowk module add <git-url> <version> [flags]`,
  },

  'reference/cli/module-remove-syntax': {
    language: 'bash',
    code: `invowk module remove <identifier>`,
  },

  'reference/cli/module-sync-syntax': {
    language: 'bash',
    code: `invowk module sync`,
  },

  'reference/cli/module-update-syntax': {
    language: 'bash',
    code: `invowk module update [identifier]`,
  },

  'reference/cli/module-deps-syntax': {
    language: 'bash',
    code: `invowk module deps`,
  },

  'reference/cli/module-vendor-syntax': {
    language: 'bash',
    code: `invowk module vendor [module-path]`,
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

  'reference/cli/error-no-invowkfile': {
    language: 'text',
    code: `# No invowkfile found!

We searched for an invowkfile but couldn't find one in the expected locations.

## Search locations (in order of precedence):
1. Current directory (invowkfile and sibling modules)
2. Configured includes (module paths from config)
3. ~/.invowk/cmds/ (modules only)

## Things you can try:
• Create an invowkfile in your current directory:
  $ invowk init

• Or specify a different directory:
  $ cd /path/to/your/project`,
  },

  'reference/cli/error-parse-failed': {
    language: 'text',
    code: `✗ Failed to parse /path/to/invowkfile.cue: invowkfile validation failed:
  #Invowkfile.cmds.0.implementations.0.runtimes.0.name: 3 errors in empty disjunction

# Failed to parse invowkfile!

Your invowkfile contains syntax errors or invalid configuration.

## Common issues:
- Invalid CUE syntax (missing quotes, braces, etc.)
- Unknown field names
- Invalid values for known fields
- Missing required fields (name, script for commands)

## Things you can try:
- Check the error message above for the specific line/column
- Validate your CUE syntax using the cue command-line tool
- Run with verbose mode for more details:
  $ invowk --ivk-verbose cmd`,
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
    code: `#RuntimeConfig: {
    name: "native" | "virtual" | "container"
    
    // Host environment inheritance:
    env_inherit_mode?:  "none" | "allow" | "all"
    env_inherit_allow?: [...string]
    env_inherit_deny?:  [...string]

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

  'environment/env-inherit-cli': {
    language: 'bash',
    code: `invowk cmd examples hello \\
  --ivk-env-inherit-mode allow \\
  --ivk-env-inherit-allow TERM \\
  --ivk-env-inherit-allow LANG \\
  --ivk-env-inherit-deny AWS_SECRET_ACCESS_KEY`,
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

  // =============================================================================
  // REFERENCE - INVKMOD SCHEMA
  // =============================================================================

  'reference/invowkmod/root-structure': {
    language: 'cue',
    code: `#Invowkmod: {
    module:       string               // Required - module identifier
    version:      string               // Required - semver version (e.g., "1.0.0")
    description?: string               // Optional - module description
    requires?:    [...#ModuleRequirement] // Optional - dependencies
}`,
  },

  'reference/invowkmod/module-examples': {
    language: 'cue',
    code: `module: "mytools"
module: "com.company.devtools"
module: "io.github.username.cli"`,
  },

  'reference/invowkmod/requires-example': {
    language: 'cue',
    code: `requires: [
    {
        git_url: "https://github.com/example/common.invowkmod.git"
        version: "^1.0.0"
        alias: "common"
    },
]`,
  },

  'reference/invowkmod/requirement-structure': {
    language: 'cue',
    code: `#ModuleRequirement: {
    git_url: string
    version: string
    alias?:  string
    path?:   string
}`,
  },

  // =============================================================================
  // REFERENCE - CONFIG SCHEMA
  // =============================================================================

  'reference/config/schema-definition': {
    language: 'cue',
    code: `// Root configuration structure
#Config: {
    container_engine?: "podman" | "docker"
    includes?:         [...#IncludeEntry]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
    container?:        #ContainerConfig
}

// Include entry for modules
#IncludeEntry: {
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
}

// Virtual shell configuration
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

// UI configuration
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
    interactive?:  bool
}

// Container configuration
#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}

// Auto-provisioning configuration
#AutoProvisionConfig: {
    enabled?:          bool
    strict?:           bool
    binary_path?:      string
    includes?:         [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?:        string
}`,
  },

  'reference/config/config-structure': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    includes?:         [...#IncludeEntry]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
    container?:        #ContainerConfig
}`,
  },

  'reference/config/container-engine-example': {
    language: 'cue',
    code: `container_engine: "podman"`,
  },

  'reference/config/includes-example': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
    {path: "/opt/company/tools.invowkmod"},
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
    interactive: false
}`,
  },

  'reference/config/container-example': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
    }
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
    interactive?:  bool
}`,
  },

  'reference/config/container-config-structure': {
    language: 'cue',
    code: `#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}`,
  },

  'reference/config/auto-provision-config-structure': {
    language: 'cue',
    code: `#AutoProvisionConfig: {
    enabled?:          bool
    strict?:           bool
    binary_path?:      string
    includes?:         [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?:        string
}`,
  },

  'reference/config/auto-provision-example': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
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

  'reference/config/interactive-example': {
    language: 'cue',
    code: `ui: {
    interactive: true
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
container_engine: "podman"

// Includes
// --------
// Additional modules to include in discovery.
// Each entry specifies a path to an *.invowkmod directory.
// Modules may have an optional alias for collision disambiguation.
includes: [
    // Personal modules
    {path: "/home/user/.invowk/modules/tools.invowkmod"},

    // Team shared module (with alias)
    {path: "/home/user/work/shared.invowkmod", alias: "team"},

    // Organization-wide module
    {path: "/opt/company/tools.invowkmod"},
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
    // Same as always passing --ivk-verbose
    verbose: false

    // Enable interactive mode by default
    interactive: false
}

// Container provisioning
// ----------------------
container: {
    auto_provision: {
        enabled: true
    }
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
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
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

  'interactive/container-tui-example': {
    language: 'cue',
    code: `{
    name: "deploy"
    implementations: [{
        script: """
            # TUI components work inside containers!
            ENV=$(invowk tui choose "Select environment" "dev" "staging" "prod")

            if invowk tui confirm "Deploy to \\$ENV?"; then
                ./deploy.sh "\\$ENV"
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
  // ARCHITECTURE
  // =============================================================================

  'architecture/runtime-selection-cross-platform': {
    language: 'cue',
    code: `cmds: [{
    name: "build"
    implementations: [
        {
            script: "nmake build"
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        },
        {
            script: "make build"
            runtimes: [{name: "container", image: "golang:1.26"}]
            platforms: [{name: "linux"}]
        },
        {
            script: "make build"
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
    ]
}]`,
  },

  'architecture/discovery-module-fields': {
    language: 'cue',
    code: `// invowkmod.cue
module: "com.example.mymodule"  // RDNS naming convention
version: "1.0.0"                // Semantic version

// Optional
description: "My useful module"
requires: [
    {
        git_url: "https://github.com/org/repo.invowkmod.git"
        version: "^1.0.0"
    }
]`,
  },

  'architecture/discovery-includes-config': {
    language: 'cue',
    code: `includes: [
    {path: "/opt/company-invowk-modules/tools.invowkmod"},
    {path: "/home/shared/invowk/shared.invowkmod"},
]`,
  },

  'dependencies/overview-runtime-aware': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            runtimes: [{name: "container", image: "golang:1.26"}]
            depends_on: {
                // Checked INSIDE the container, not on host
                tools: [{alternatives: ["go"]}]
                filepaths: [{alternatives: ["/workspace/go.mod"]}]
            }
        }
    ]
}`,
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
  // DEPENDENCIES - TOOLS (extracted from inline blocks)
  // =============================================================================

  'dependencies/tools-multiple-and': {
    language: 'cue',
    code: `depends_on: {
    tools: [
        // Need (podman OR docker) AND kubectl AND helm
        {alternatives: ["podman", "docker"]},
        {alternatives: ["kubectl"]},
        {alternatives: ["helm"]},
    ]
}`,
  },

  'dependencies/tools-go-project': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [
            {alternatives: ["go"]},
            {alternatives: ["git"]},  // For version info
        ]
    }
    implementations: [{
        script: """
            VERSION=$(git describe --tags --always)
            go build -ldflags="-X main.version=$VERSION" ./...
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-nodejs-project': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [
            // Prefer pnpm, but npm works too
            {alternatives: ["pnpm", "npm", "yarn"]},
            {alternatives: ["node"]},
        ]
    }
    implementations: [{
        script: "pnpm run build || npm run build"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-kubernetes-deploy': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        tools: [
            {alternatives: ["kubectl"]},
            {alternatives: ["helm"]},
            {alternatives: ["podman", "docker"]},
        ]
    }
    implementations: [{
        script: """
            helm upgrade --install myapp ./charts/myapp
            kubectl rollout status deployment/myapp
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-python-project': {
    language: 'cue',
    code: `{
    name: "run"
    depends_on: {
        tools: [
            // Python 3 with various possible names
            {alternatives: ["python3", "python"]},
            // Virtual environment tool
            {alternatives: ["poetry", "pipenv", "pip"]},
        ]
    }
    implementations: [{
        script: "poetry run python main.py"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-runtime-native': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]  // Checked on host
    }
    implementations: [{
        script: "go build ./..."
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-runtime-virtual': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]  // Checked in virtual shell
    }
    implementations: [{
        script: "go build ./..."
        runtimes: [{name: "virtual"}]
    }]
}`,
  },

  'dependencies/tools-runtime-container': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: "go build ./..."
        runtimes: [{name: "container", image: "golang:1.26"}]
        depends_on: {
            // This checks for 'go' INSIDE the container
            tools: [{alternatives: ["go"]}]
        }
    }]
}`,
  },

  'dependencies/tools-external-call': {
    language: 'cue',
    code: `{
    name: "upload"
    depends_on: {
        tools: [{alternatives: ["aws", "aws-cli"]}]
    }
    implementations: [{
        script: "aws s3 sync ./dist s3://my-bucket"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-database': {
    language: 'cue',
    code: `{
    name: "db migrate"
    depends_on: {
        tools: [
            {alternatives: ["psql", "pgcli"]},  // PostgreSQL client
            {alternatives: ["migrate", "goose", "flyway"]},  // Migration tool
        ]
    }
    implementations: [{
        script: "migrate -path ./migrations -database $DATABASE_URL up"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/tools-cross-platform': {
    language: 'cue',
    code: `{
    name: "open docs"
    depends_on: {
        tools: [
            // Platform-specific openers
            {alternatives: ["xdg-open", "open", "start"]},
        ]
    }
    implementations: [{
        script: "xdg-open http://localhost:3000/docs || open http://localhost:3000/docs"
        runtimes: [{name: "native"}]
    }]
}`,
  },
  // =============================================================================
  // DEPENDENCIES - CAPABILITIES (extracted from inline blocks)
  // =============================================================================

  'dependencies/capabilities-internet': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["internet"]}
    ]
}`,
  },

  'dependencies/capabilities-internet-usecases': {
    language: 'cue',
    code: `// Download dependencies
{
    name: "deps"
    depends_on: {
        capabilities: [{alternatives: ["internet"]}]
    }
    implementations: [{
        script: "go mod download"
        runtimes: [{name: "native"}]
    }]
}

// Deploy to cloud
{
    name: "deploy"
    depends_on: {
        capabilities: [{alternatives: ["internet"]}]
    }
    implementations: [{
        script: "kubectl apply -f k8s/"
        runtimes: [{name: "native"}]
    }]
}

// Fetch remote data
{
    name: "sync"
    depends_on: {
        capabilities: [{alternatives: ["internet"]}]
    }
    implementations: [{
        script: "curl -o data.json https://api.example.com/data"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/capabilities-lan': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        {alternatives: ["local-area-network"]}
    ]
}`,
  },

  'dependencies/capabilities-lan-usecases': {
    language: 'cue',
    code: `// Connect to local database
{
    name: "db connect"
    depends_on: {
        capabilities: [{alternatives: ["local-area-network"]}]
    }
    implementations: [{
        script: "psql -h db.local -U admin"
        runtimes: [{name: "native"}]
    }]
}

// Access local services
{
    name: "check services"
    depends_on: {
        capabilities: [{alternatives: ["local-area-network"]}]
    }
    implementations: [{
        script: "curl http://service.local:8080/health"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/capabilities-alternatives': {
    language: 'cue',
    code: `depends_on: {
    capabilities: [
        // Either internet OR LAN is fine
        {alternatives: ["internet", "local-area-network"]}
    ]
}`,
  },

  'dependencies/capabilities-package-install': {
    language: 'cue',
    code: `{
    name: "install"
    description: "Install project dependencies"
    depends_on: {
        capabilities: [{alternatives: ["internet"]}]
        tools: [{alternatives: ["npm", "pnpm", "yarn"]}]
    }
    implementations: [{
        script: "npm install"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/capabilities-ci-pipeline': {
    language: 'cue',
    code: `{
    name: "ci"
    description: "Run CI pipeline with remote checks"
    depends_on: {
        capabilities: [
            {alternatives: ["internet"]}  // For dependency download
        ]
        tools: [
            {alternatives: ["go"]},
            {alternatives: ["git"]},
        ]
    }
    implementations: [{
        script: """
            go mod download
            go build ./...
            go test ./...
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/capabilities-hybrid': {
    language: 'cue',
    code: `{
    name: "backup"
    description: "Backup to available storage"
    depends_on: {
        // Can backup to cloud (internet) or NAS (LAN)
        capabilities: [{alternatives: ["internet", "local-area-network"]}]
    }
    implementations: [{
        script: """
            if ping -c 1 nas.local > /dev/null 2>&1; then
                rsync -av ./data nas.local:/backup/
            else
                aws s3 sync ./data s3://my-backup-bucket/
            fi
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/capabilities-offline-first': {
    language: 'cue',
    code: `cmds: [
    // Online version - downloads latest
    {
        name: "update deps"
        depends_on: {
            capabilities: [{alternatives: ["internet"]}]
        }
        implementations: [{
            script: "go mod download"
            runtimes: [{name: "native"}]
        }]
    },

    // Offline version - uses cache
    {
        name: "build"
        // No internet requirement - uses cached dependencies
        depends_on: {
            filepaths: [{alternatives: ["go.mod"]}]
        }
        implementations: [{
            script: "go build -mod=readonly ./..."
            runtimes: [{name: "native"}]
        }]
    }
]`,
  },

  'dependencies/capabilities-container-context': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        // Internet check happens on HOST
        capabilities: [{alternatives: ["internet"]}]
    }
    implementations: [{
        script: """
            apt-get update && apt-get install -y kubectl
            kubectl apply -f k8s/
            """
        runtimes: [{name: "container", image: "debian:stable-slim"}]
    }]
}`,
  },

  // =============================================================================
  // DEPENDENCIES - ENV VARS (extracted from inline blocks)
  // =============================================================================

  'dependencies/env-vars-alternatives': {
    language: 'cue',
    code: `depends_on: {
    env_vars: [
        // Either AWS_ACCESS_KEY_ID OR AWS_PROFILE
        {alternatives: [
            {name: "AWS_ACCESS_KEY_ID"},
            {name: "AWS_PROFILE"}
        ]}
    ]
}`,
  },

  'dependencies/env-vars-regex': {
    language: 'cue',
    code: `depends_on: {
    env_vars: [
        // Must be set AND match semver format
        {alternatives: [{
            name: "VERSION"
            validation: "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+$"
        }]}
    ]
}`,
  },

  'dependencies/env-vars-aws': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to AWS"
    depends_on: {
        env_vars: [
            // Need either access key or profile
            {alternatives: [
                {name: "AWS_ACCESS_KEY_ID"},
                {name: "AWS_PROFILE"}
            ]},
            // Region is required
            {alternatives: [{name: "AWS_REGION"}]}
        ]
        tools: [{alternatives: ["aws"]}]
    }
    implementations: [{
        script: "aws s3 sync ./dist s3://my-bucket"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/env-vars-database': {
    language: 'cue',
    code: `{
    name: "db migrate"
    description: "Run database migrations"
    depends_on: {
        env_vars: [
            {alternatives: [{
                name: "DATABASE_URL"
                // Validate it looks like a connection string
                validation: "^postgres(ql)?://.*$"
            }]}
        ]
        tools: [{alternatives: ["migrate", "goose"]}]
    }
    implementations: [{
        script: "migrate -path ./migrations -database $DATABASE_URL up"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/env-vars-api-keys': {
    language: 'cue',
    code: `{
    name: "publish"
    description: "Publish package to registry"
    depends_on: {
        env_vars: [
            // NPM token for publishing
            {alternatives: [{name: "NPM_TOKEN"}]},
        ]
        tools: [{alternatives: ["npm"]}]
    }
    implementations: [{
        script: """
            echo "//registry.npmjs.org/:_authToken=\${NPM_TOKEN}" > ~/.npmrc
            npm publish
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/env-vars-env-config': {
    language: 'cue',
    code: `{
    name: "deploy"
    description: "Deploy to target environment"
    depends_on: {
        env_vars: [
            // DEPLOY_ENV must be one of: dev, staging, prod
            {alternatives: [{
                name: "DEPLOY_ENV"
                validation: "^(dev|staging|prod)$"
            }]}
        ]
    }
    implementations: [{
        script: """
            echo "Deploying to $DEPLOY_ENV..."
            ./scripts/deploy-$DEPLOY_ENV.sh
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/env-vars-version': {
    language: 'cue',
    code: `{
    name: "release"
    description: "Create a release"
    depends_on: {
        env_vars: [
            // Version must be semantic
            {alternatives: [{
                name: "VERSION"
                validation: "^v?[0-9]+\\\\.[0-9]+\\\\.[0-9]+(-[a-zA-Z0-9]+)?$"
            }]},
            // Git tag message
            {alternatives: [{name: "RELEASE_NOTES"}]}
        ]
    }
    implementations: [{
        script: """
            git tag -a "$VERSION" -m "$RELEASE_NOTES"
            git push origin "$VERSION"
            """
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/env-vars-pattern-semver': {
    language: 'cue',
    code: `validation: "^[0-9]+\\\\.[0-9]+\\\\.[0-9]+$"
// Matches: 1.0.0, 2.1.3
// Rejects: v1.0.0, 1.0, latest`,
  },

  'dependencies/env-vars-pattern-url': {
    language: 'cue',
    code: `validation: "^https?://[^\\\\s]+$"
// Matches: http://localhost, https://example.com/path
// Rejects: ftp://server, not-a-url`,
  },

  'dependencies/env-vars-pattern-email': {
    language: 'cue',
    code: `validation: "^[^@]+@[^@]+\\\\.[^@]+$"
// Matches: user@example.com
// Rejects: invalid, @example.com`,
  },

  'dependencies/env-vars-pattern-alphanum': {
    language: 'cue',
    code: `validation: "^[a-zA-Z0-9_-]+$"
// Matches: my-project_123, ABC
// Rejects: my project, name@here`,
  },

  'dependencies/env-vars-pattern-aws-region': {
    language: 'cue',
    code: `validation: "^[a-z]{2}-[a-z]+-[0-9]+$"
// Matches: us-east-1, eu-west-2
// Rejects: US-EAST-1, us_east_1`,
  },

  'dependencies/env-vars-multiple': {
    language: 'cue',
    code: `depends_on: {
    env_vars: [
        // Need API_KEY AND API_SECRET AND API_URL
        {alternatives: [{name: "API_KEY"}]},
        {alternatives: [{name: "API_SECRET"}]},
        {alternatives: [{
            name: "API_URL"
            validation: "^https://.*$"
        }]},
    ]
}`,
  },

  'dependencies/env-vars-user-env': {
    language: 'cue',
    code: `{
    name: "example"
    env: {
        vars: {
            // This is set by Invowk during execution
            MY_VAR: "value"
        }
    }
    depends_on: {
        env_vars: [
            // This checks the USER's environment, BEFORE Invowk sets MY_VAR
            // So it will fail if the user hasn't set MY_VAR themselves
            {alternatives: [{name: "MY_VAR"}]}
        ]
    }
}`,
  },

  // =============================================================================
  // DEPENDENCIES - CUSTOM CHECKS (extracted from inline blocks)
  // =============================================================================

  'dependencies/custom-checks-exit-code-default': {
    language: 'cue',
    code: `custom_checks: [
    {
        name: "docker-running"
        check_script: "docker info > /dev/null 2>&1"
        // Passes if exit code is 0
    }
]`,
  },

  'dependencies/custom-checks-exit-code-custom': {
    language: 'cue',
    code: `custom_checks: [
    {
        name: "not-production"
        check_script: "test \\"$ENV\\" = 'production'"
        expected_code: 1  // Should fail (not be production)
    }
]`,
  },

  'dependencies/custom-checks-output-validation': {
    language: 'cue',
    code: `custom_checks: [
    {
        name: "node-version"
        check_script: "node --version"
        expected_output: "^v(18|20|22)\\\\."  // Major version 18, 20, or 22
    }
]`,
  },

  'dependencies/custom-checks-output-and-exit-code': {
    language: 'cue',
    code: `custom_checks: [
    {
        name: "go-version"
        check_script: "go version"
        expected_code: 0  // Must succeed
        expected_output: "go1\\\\.2[6-9]"  // Must be Go 1.26+
    }
]`,
  },

  'dependencies/custom-checks-alternatives': {
    language: 'cue',
    code: `custom_checks: [
    {
        alternatives: [
            {
                name: "go-1.26"
                check_script: "go version | grep -q 'go1.26'"
            },
            {
                name: "go-1.27"
                check_script: "go version | grep -q 'go1.27'"
            }
        ]
    }
]`,
  },

  'dependencies/custom-checks-example-tool-version': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]
        custom_checks: [
            {
                name: "go-1.26-or-higher"
                check_script: """
                    version=$(go version | grep -oE 'go[0-9]+\\\\.[0-9]+' | head -1)
                    major=$(echo $version | cut -d. -f1 | tr -d 'go')
                    minor=$(echo $version | cut -d. -f2)
                    [ "$major" -ge 1 ] && [ "$minor" -ge 26 ]
                    """
            }
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/custom-checks-example-docker-running': {
    language: 'cue',
    code: `{
    name: "docker-build"
    depends_on: {
        tools: [{alternatives: ["docker"]}]
        custom_checks: [
            {
                name: "docker-daemon"
                check_script: "docker info > /dev/null 2>&1"
            }
        ]
    }
    implementations: [{
        script: "docker build -t myapp ."
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/custom-checks-example-git-status': {
    language: 'cue',
    code: `{
    name: "release"
    depends_on: {
        tools: [{alternatives: ["git"]}]
        custom_checks: [
            {
                name: "clean-working-tree"
                check_script: "test -z \\"$(git status --porcelain)\\""
            },
            {
                name: "on-main-branch"
                check_script: "test \\"$(git branch --show-current)\\" = 'main'"
            }
        ]
    }
    implementations: [{
        script: "./scripts/release.sh"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/custom-checks-example-config-validation': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        filepaths: [{alternatives: ["config.yaml"]}]
        custom_checks: [
            {
                name: "valid-yaml"
                check_script: "python3 -c 'import yaml; yaml.safe_load(open(\\"config.yaml\\"))'"
            },
            {
                name: "has-required-fields"
                check_script: """
                    grep -q 'database:' config.yaml && \\
                    grep -q 'server:' config.yaml
                    """
            }
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/custom-checks-example-memory-resource': {
    language: 'cue',
    code: `{
    name: "build heavy"
    depends_on: {
        custom_checks: [
            {
                name: "enough-memory"
                check_script: """
                    # Check for at least 4GB free memory
                    free_mb=$(free -m | awk '/^Mem:/{print $7}')
                    [ "$free_mb" -ge 4096 ]
                    """
            },
            {
                name: "enough-disk"
                check_script: """
                    # Check for at least 10GB free disk
                    free_gb=$(df -BG . | awk 'NR==2{print $4}' | tr -d 'G')
                    [ "$free_gb" -ge 10 ]
                    """
            }
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/custom-checks-example-kubernetes': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        tools: [{alternatives: ["kubectl"]}]
        custom_checks: [
            {
                name: "correct-context"
                check_script: "kubectl config current-context"
                expected_output: "^production-cluster$"
            },
            {
                name: "cluster-reachable"
                check_script: "kubectl cluster-info > /dev/null 2>&1"
            }
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/custom-checks-example-multiple-versions': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        custom_checks: [
            {
                alternatives: [
                    {
                        name: "python-3.11"
                        check_script: "python3 --version"
                        expected_output: "^Python 3\\\\.11"
                    },
                    {
                        name: "python-3.12"
                        check_script: "python3 --version"
                        expected_output: "^Python 3\\\\.12"
                    }
                ]
            }
        ]
    }
    implementations: [...]
}`,
  },

  'dependencies/custom-checks-container-context': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: "npm run build"
        runtimes: [{name: "container", image: "node:20"}]
        depends_on: {
            custom_checks: [
                {
                    name: "node-version"
                    // This runs INSIDE the container
                    check_script: "node --version"
                    expected_output: "^v20\\\\."
                }
            ]
        }
    }]
}`,
  },

  'dependencies/custom-checks-tip-keep-simple': {
    language: 'cue',
    code: `// Good - simple and clear
check_script: "go version | grep -q 'go1.26'"

// Avoid - complex and fragile
check_script: """
    set -e
    version=$(go version 2>&1)
    if [ $? -ne 0 ]; then exit 1; fi
    echo "$version" | grep -qE 'go1\\\\.(2[6-9]|[3-9][0-9])'
    """`,
  },

  'dependencies/custom-checks-tip-exit-codes': {
    language: 'cue',
    code: `// Script should exit 0 for success, non-zero for failure
check_script: """
    if [ -f "required-file" ]; then
        exit 0
    else
        exit 1
    fi
    """`,
  },

  'dependencies/custom-checks-tip-handle-missing': {
    language: 'cue',
    code: `check_script: "command -v mytools > /dev/null && mytool --check"`,
  },

  // =============================================================================
  // DEPENDENCIES - FILEPATHS (extracted from inline blocks)
  // =============================================================================

  'dependencies/filepaths-relative': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["./src/main.go"]},
        {alternatives: ["../shared/utils.go"]},
        {alternatives: ["scripts/build.sh"]},
    ]
}`,
  },

  'dependencies/filepaths-absolute': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["/etc/myapp/config.yaml"]},
        {alternatives: ["/usr/local/bin/mytool"]},
    ]
}`,
  },

  'dependencies/filepaths-envvars': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["\${HOME}/.config/myapp/config.yaml"]},
        {alternatives: ["\${XDG_CONFIG_HOME}/myapp/config.yaml", "\${HOME}/.myapprc"]},
    ]
}`,
  },

  'dependencies/filepaths-readable': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["secrets.env"], readable: true}
    ]
}`,
  },

  'dependencies/filepaths-writable': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["./output", "./dist"], writable: true}
    ]
}`,
  },

  'dependencies/filepaths-executable': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        {alternatives: ["./scripts/deploy.sh"], executable: true}
    ]
}`,
  },

  'dependencies/filepaths-combined-permissions': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        // Script must be readable AND executable
        {
            alternatives: ["./scripts/run.sh"]
            readable: true
            executable: true
        }
    ]
}`,
  },

  'dependencies/filepaths-dirs-vs-files': {
    language: 'cue',
    code: `depends_on: {
    filepaths: [
        // Check for a file
        {alternatives: ["package.json"]},

        // Check for a directory
        {alternatives: ["node_modules"]},

        // Check directory is writable
        {alternatives: ["./build"], writable: true},
    ]
}`,
  },

  'dependencies/filepaths-go-project': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        filepaths: [
            {alternatives: ["go.mod"]},
            {alternatives: ["go.sum"]},
            {alternatives: ["cmd/main.go", "main.go"]},
        ]
    }
    implementations: [{
        script: "go build ./..."
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/filepaths-nodejs-project': {
    language: 'cue',
    code: `{
    name: "build"
    depends_on: {
        filepaths: [
            {alternatives: ["package.json"]},
            // Any lock file is fine
            {alternatives: ["pnpm-lock.yaml", "package-lock.json", "yarn.lock"]},
            // Dependencies must be installed
            {alternatives: ["node_modules"]},
        ]
    }
    implementations: [{
        script: "npm run build"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/filepaths-docker-build': {
    language: 'cue',
    code: `{
    name: "docker-build"
    depends_on: {
        filepaths: [
            // Need either Dockerfile or Containerfile
            {alternatives: ["Dockerfile", "Containerfile"]},
            // And a build script
            {alternatives: ["scripts/build.sh"], executable: true},
        ]
    }
    implementations: [{
        script: "docker build -t myapp ."
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/filepaths-config-files': {
    language: 'cue',
    code: `{
    name: "deploy"
    depends_on: {
        filepaths: [
            // Check for config in order of preference
            {
                alternatives: [
                    "./config/production.yaml",
                    "./config/default.yaml",
                    "\${HOME}/.myapp/config.yaml"
                ]
                readable: true
            },
            // Writable output directory
            {alternatives: ["./deploy-output"], writable: true},
        ]
    }
    implementations: [{
        script: "./scripts/deploy.sh"
        runtimes: [{name: "native"}]
    }]
}`,
  },

  'dependencies/filepaths-container': {
    language: 'cue',
    code: `{
    name: "build"
    implementations: [{
        script: "go build ./..."
        runtimes: [{name: "container", image: "golang:1.26"}]
        depends_on: {
            filepaths: [
                // These are checked INSIDE the container
                // /workspace is where your project is mounted
                {alternatives: ["/workspace/go.mod"]},
                {alternatives: ["/workspace/go.sum"]},
            ]
        }
    }]
}`,
  },

  'dependencies/filepaths-platform': {
    language: 'cue',
    code: `{
    name: "read-config"
    implementations: [
        {
            script: "cat $CONFIG_PATH"
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
            depends_on: {
                filepaths: [{alternatives: ["/etc/myapp/config.yaml"]}]
            }
        },
        {
            script: "cat $CONFIG_PATH"
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
            depends_on: {
                filepaths: [{alternatives: ["/usr/local/etc/myapp/config.yaml"]}]
            }
        }
    ]
}`,
  },
} as const;

// Type-safe snippet IDs
export type SnippetId = keyof typeof snippets;

// Helper to get all snippet IDs for documentation/tooling
export const snippetIds = Object.keys(snippets) as SnippetId[];
