---
sidebar_position: 3
---

# Environment Variables

Set environment variables directly in your invkfile. These are available to your scripts during execution.

## Basic Usage

```cue
{
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
}
```

## Variable Syntax

Variables are key-value string pairs:

```cue
vars: {
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
}
```

All values are strings. The shell interprets them as needed.

## Referencing Other Variables

Reference system environment variables:

```cue
vars: {
    // Use system variable
    HOME_CONFIG: "${HOME}/.config/myapp"
    
    // With default value
    LOG_LEVEL: "${LOG_LEVEL:-info}"
    
    // Combine variables
    FULL_PATH: "${HOME}/projects/${PROJECT_NAME}"
}
```

Note: References are expanded at runtime, not definition time.

## Scope Levels

### Root Level

Available to all commands:

```cue
group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
        VERSION: "1.0.0"
    }
}

commands: [
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
]
```

### Command Level

Available to specific command:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [...]
}
```

### Implementation Level

Available to specific implementation:

```cue
{
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
}
```

### Platform Level

Per-platform variables:

```cue
implementations: [{
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
                    PLATFORM_CONFIG: "%APPDATA%\\myapp"
                    PLATFORM_NAME: "Windows"
                }
            }
        ]
    }
}]
```

## Combined with Files

Variables override values from env files:

```cue
env: {
    files: [".env"]  // Loaded first
    vars: {
        // These override .env values
        OVERRIDE: "from-invkfile"
    }
}
```

## CLI Override

Override at runtime:

```bash
# Single variable
invowk cmd myproject build --env-var NODE_ENV=development

# Short form
invowk cmd myproject build -E NODE_ENV=development

# Multiple variables
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true -E PORT=8080
```

CLI variables have the highest priority.

## Real-World Examples

### Build Configuration

```cue
{
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
}
```

### API Configuration

```cue
{
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
}
```

### Dynamic Values

```cue
{
    name: "release"
    env: {
        vars: {
            // Git-based version
            GIT_SHA: "$(git rev-parse --short HEAD)"
            GIT_BRANCH: "$(git branch --show-current)"
            
            // Timestamp
            BUILD_TIME: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            
            // Combine values
            BUILD_ID: "${GIT_BRANCH}-${GIT_SHA}"
        }
    }
    implementations: [{
        script: """
            echo "Building $BUILD_ID at $BUILD_TIME"
            go build -ldflags="-X main.version=$BUILD_ID" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Database Configuration

```cue
{
    name: "db migrate"
    env: {
        vars: {
            DB_HOST: "${DB_HOST:-localhost}"
            DB_PORT: "${DB_PORT:-5432}"
            DB_NAME: "${DB_NAME:-myapp}"
            DB_USER: "${DB_USER:-postgres}"
            // Construct URL from parts
            DATABASE_URL: "postgres://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
        }
    }
    implementations: [{
        script: "migrate -database $DATABASE_URL up"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Container Environment

Variables are passed into containers:

```cue
{
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
}
```

## Best Practices

1. **Use defaults**: `${VAR:-default}` for optional config
2. **Keep secrets out**: Don't hardcode secrets; use env files or external secrets
3. **Document variables**: Add comments explaining each variable
4. **Use consistent naming**: `UPPER_SNAKE_CASE` convention
5. **Scope appropriately**: Root for shared, command for specific

## Next Steps

- [Env Files](./env-files) - Load from .env files
- [Precedence](./precedence) - Understand override order
