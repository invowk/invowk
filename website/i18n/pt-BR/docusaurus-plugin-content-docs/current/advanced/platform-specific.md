---
sidebar_position: 3
---

# Platform-Specific Commands

Create commands that behave differently on Linux, macOS, and Windows. Invowk automatically selects the right implementation for the current platform.

## Supported Platforms

| Value | Description |
|-------|-------------|
| `linux` | Linux distributions |
| `macos` | macOS (Darwin) |
| `windows` | Windows |

## Basic Platform Targeting

Restrict an implementation to specific platforms:

```cue
{
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
}
```

## All Platforms (Default)

If `platforms` is omitted, the implementation works on all platforms:

```cue
{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "native"}]
            // No platforms = works everywhere
        }
    }]
}
```

## Unix-Only Commands

Target both Linux and macOS:

```cue
{
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
}
```

## Platform-Specific Environment

Set different environment variables per platform:

```cue
{
    name: "configure"
    implementations: [{
        script: "echo \"Config: $CONFIG_PATH\""
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
                        CACHE_DIR: "${HOME}/Library/Caches/myapp"
                    }
                },
                {
                    name: "windows"
                    env: {
                        CONFIG_PATH: "%APPDATA%\\myapp\\config.yaml"
                        CACHE_DIR: "%LOCALAPPDATA%\\myapp\\cache"
                    }
                }
            ]
        }
    }]
}
```

## Cross-Platform Scripts

Write one script that works everywhere:

```cue
{
    name: "build"
    implementations: [{
        script: """
            go build -o ${OUTPUT_NAME} ./...
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
}
```

## CUE Templates for Platforms

Use CUE templates to reduce repetition:

```cue
// Define platform templates
_linux: {name: "linux"}
_macos: {name: "macos"}
_windows: {name: "windows"}

_unix: [{name: "linux"}, {name: "macos"}]
_all: [{name: "linux"}, {name: "macos"}, {name: "windows"}]

commands: [
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
]
```

## Command Listing

The command list shows supported platforms:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*] (linux, macos, windows)
  myproject clean - Clean artifacts [native*] (linux, macos)
  myproject deploy - Deploy to cloud [native*] (linux)
```

## Unsupported Platform Error

Running a command on an unsupported platform shows a clear error:

```
✗ Host not supported

Command 'deploy' cannot run on this host.

Current host:     windows
Supported hosts:  linux, macos

This command is only available on the platforms listed above.
```

## Real-World Examples

### System Information

```cue
{
    name: "sysinfo"
    implementations: [
        {
            script: """
                echo "Hostname: $(hostname)"
                echo "Kernel: $(uname -r)"
                echo "Memory: $(free -h | awk '/^Mem:/{print $2}')"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: """
                echo "Hostname: $(hostname)"
                echo "Kernel: $(uname -r)"
                echo "Memory: $(sysctl -n hw.memsize | awk '{print $0/1024/1024/1024 "GB"}')"
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
}
```

### Package Installation

```cue
{
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
}
```

## Best Practices

1. **Start with cross-platform**: Add platform-specific only when needed
2. **Use environment variables**: Abstract platform differences
3. **Test on all platforms**: CI should cover all supported platforms
4. **Document limitations**: Note which platforms are supported

## Next Steps

- [Interpreters](./interpreters) - Use non-shell interpreters
- [Working Directory](./workdir) - Control execution location
