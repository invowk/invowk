import type { Snippet } from '../snippets';

export const runtimeModesSnippets = {
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
// The "interpreter" field is not allowed on #RuntimeConfigVirtual —
// CUE schema validation will reject this at parse time.
{
    name: "bad-example"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            print("This won't work!")
            """
        runtimes: [{
            name: "virtual"
            interpreter: "python3"  // CUE validation error: field not allowed
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
// %APPDATA%\invowk\config.cue (Windows)

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

RUN apt-get update && apt-get install -y 
    make 
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
            sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no 
                $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT 
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
};
