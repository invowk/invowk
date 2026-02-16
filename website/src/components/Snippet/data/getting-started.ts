import type { Snippet } from '../snippets';

export const gettingStartedSnippets = {
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
$env:INSTALL_DIR='C:	ools\invowk'; irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex

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
                script: "echo "Hello, $INVOWK_ARG_NAME!""
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            },
            {
                script: "Write-Output "Hello, $($env:INVOWK_ARG_NAME)!""
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            },
            {
                script: "echo "Hello, $INVOWK_ARG_NAME!""
                runtimes: [{name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            },
            {
                script: "echo "Hello from container, $INVOWK_ARG_NAME!""
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
    script: "echo "Hello, $INVOWK_ARG_NAME!""
    runtimes: [{name: "virtual"}]
    platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
}`,
  },
};
