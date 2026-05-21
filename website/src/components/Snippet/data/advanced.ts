import type { Snippet } from '../snippets';

export const advancedSnippets = {
  // =============================================================================
  // ADVANCED
  // =============================================================================

  'advanced/interpreter-python': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [
        {
            script: {
                content: """
                import json
                import sys

                data = json.load(open('data.json'))
                print(f"Found {len(data)} records")
                """
                interpreter: "python3"
            }
            runtimes: [{name: "native"}]
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
            script: {
                content: """
                const fs = require('fs');
                const data = JSON.parse(fs.readFileSync('data.json'));
                console.log(\`Processing \${data.length} items\`);
                """
                interpreter: "node"
            }
            runtimes: [{name: "native"}]
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
            script: {content: "xdg-open $URL"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: {content: "open $URL"}
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: {content: "start $URL"}
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
        script: {content: """
            #!/usr/bin/env python3
            import sys
            import json
            
            data = {"status": "ok", "python": sys.version}
            print(json.dumps(data, indent=2))
            """}
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
        script: {
            content: """
            import sys
            print(f"Python {sys.version_info.major}.{sys.version_info.minor}")
            """
            interpreter: "python3"  // Explicit
        }
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'advanced/interpreter-args': {
    language: 'cue',
    code: `{
    name: "unbuffered"
    implementations: [{
        script: {
            content: """
            import time
            for i in range(5):
                print(f"Count: {i}")
                time.sleep(1)
            """
            interpreter: "python3 -u"  // Unbuffered output
        }
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'advanced/interpreter-more-args': {
    language: 'cue',
    code: `// Perl with warnings
script: {content: "...", interpreter: "perl -w"}

// Ruby with debug mode
script: {content: "...", interpreter: "ruby -d"}

// Node with specific options
script: {content: "...", interpreter: "node --max-old-space-size=4096"}`,
  },

  'advanced/interpreter-container-shebang': {
    language: 'cue',
    code: `{
    name: "analyze"
    implementations: [{
        script: {content: """
            #!/usr/bin/env python3
            import os
            print(f"Running in container at {os.getcwd()}")
            """}
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
        script: {
            content: """
            console.log('Hello from Node in container!')
            console.log('Node version:', process.version)
            """
            interpreter: "node"
        }
        runtimes: [{
            name: "container"
            image: "node:22-slim"
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
        script: {content: """
            #!/usr/bin/env python3
            import sys
            import os
            
            # Via command line args
            name = sys.argv[1] if len(sys.argv) > 1 else "World"
            print(f"Hello, {name}!")
            
            # Or via environment variable
            name = os.environ.get("INVOWK_ARG_NAME", "World")
            print(f"Hello again, {name}!")
            """}
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }]
}`,
  },

  'advanced/interpreter-virtual-error': {
    language: 'cue',
    code: `// This will NOT work with the virtual-sh runtime.
// Runtime validation error: virtual-sh uses mvdan/sh and
// cannot execute non-shell script interpreters.
{
    name: "bad"
    implementations: [{
        script: {content: "print('hello')", interpreter: "python3"}
        runtimes: [{name: "virtual-sh"}]
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
        script: {content: "npm run build"}
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
            script: {content: "npm run build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            workdir: "./web"  // This implementation runs in ./web
        },
        {
            script: {content: "go build ./..."}
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
                script: {content: "make"}
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
            script: {content: "npm run build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "api build"
        workdir: "./packages/api"
        implementations: [{
            script: {content: "go build ./..."}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "mobile build"
        workdir: "./packages/mobile"
        implementations: [{
            script: {content: "flutter build"}
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
        script: {content: """
            pwd  # /workspace
            ls   # Shows your project files
            """}
        runtimes: [{name: "container", image: "debian:stable-slim"}]
        platforms: [{name: "linux"}]
    }]
}`,
  },

  'advanced/workdir-container-subdir': {
    language: 'cue',
    code: `{
    name: "build package"
    workdir: "./frontend"
    implementations: [{
        script: {content: """
            pwd  # /workspace/frontend
            ./build.sh
            """}
        runtimes: [{name: "container", image: "debian:stable-slim"}]
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
            script: {content: "npm run dev"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
        }]
    },
    {
        name: "start backend"
        workdir: "./backend"
        implementations: [{
            script: {content: "go run ./cmd/server"}
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
            script: {content: "pytest"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "test integration"
        workdir: "./tests/integration"
        implementations: [{
            script: {content: "pytest"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }]
    },
    {
        name: "test e2e"
        workdir: "./tests/e2e"
        implementations: [{
            script: {content: "cypress run"}
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
        script: {content: """
            # Now in ./src
            go build -o ../bin/app ./...
            """}
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
            script: {content: "xdg-open http://localhost:3000"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: {content: "open http://localhost:3000"}
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: {content: "start http://localhost:3000"}
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
        script: {content: "go build ./..."}
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
        script: {content: """
            chmod +x ./scripts/*.sh
            ls -la ./scripts/
            """}
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
        // Linux implementation with platform-specific env
        {
            script: {content: "echo \\"Config: $CONFIG_PATH\\""}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
            env: {
                vars: {
                    CONFIG_PATH: "/etc/myapp/config.yaml"
                    CACHE_DIR: "/var/cache/myapp"
                }
            }
        },
        // macOS implementation with platform-specific env
        {
            script: {content: "echo \\"Config: $CONFIG_PATH\\""}
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
            script: {content: "echo \\"Config: %CONFIG_PATH%\\""}
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
            script: {content: "go build -o bin/app ./..."}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        },
        // Windows (different output name)
        {
            script: {content: "go build -o bin/app.exe ./..."}
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
                script: {content: "rm -rf build/"}
                runtimes: [{name: "native"}]
                platforms: _unix
            },
            // Windows implementation
            {
                script: {content: "rmdir /s /q build"}
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
            script: {content: """
                echo "Hostname: \$(hostname)"
                echo "Kernel: \$(uname -r)"
                echo "Memory: \$(free -h | awk '/^Mem:/{print \$2}')"
                """}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: {content: """
                echo "Hostname: \$(hostname)"
                echo "Kernel: \$(uname -r)"
                echo "Memory: \$(sysctl -n hw.memsize | awk '{print \$0/1024/1024/1024 "GB"}')"
                """}
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: {content: """
                echo Hostname: %COMPUTERNAME%
                systeminfo | findstr "Total Physical Memory"
                """}
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
            script: {content: "apt-get install -y build-essential"}
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}]
        },
        {
            script: {content: "brew install coreutils"}
            runtimes: [{name: "native"}]
            platforms: [{name: "macos"}]
        },
        {
            script: {content: "choco install make"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        }
    ]
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
            script: {content: "nmake build"}
            runtimes: [{name: "native"}]
            platforms: [{name: "windows"}]
        },
        {
            script: {content: "make build"}
            runtimes: [{name: "container", image: "debian:stable-slim"}]
            platforms: [{name: "linux"}]
        },
        {
            script: {content: "make build"}
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
        git_url: "https://github.com/org/repo.git"
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
} satisfies Record<string, Snippet>;
