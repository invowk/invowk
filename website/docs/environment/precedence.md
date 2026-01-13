---
sidebar_position: 4
---

# Environment Precedence

When the same variable is defined in multiple places, Invowk follows a specific precedence order. Higher precedence sources override lower ones.

## Precedence Order

From highest to lowest priority:

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI env vars | `--env-var KEY=value` |
| 2 | CLI env files | `--env-file .env.local` |
| 3 | Implementation vars | `implementations[].env.vars` |
| 4 | Implementation files | `implementations[].env.files` |
| 5 | Command vars | `command.env.vars` |
| 6 | Command files | `command.env.files` |
| 7 | Root vars | `root.env.vars` |
| 8 | Root files | `root.env.files` |
| 9 | System environment | Host's environment |
| 10 | Platform vars | `platforms[].env` |

## Visual Hierarchy

```
CLI (highest priority)
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
Platform-specific env
```

## Example Walkthrough

Given this invkfile:

```cue
group: "myproject"

// Root level
env: {
    files: [".env"]
    vars: {
        API_URL: "http://root.example.com"
        LOG_LEVEL: "info"
    }
}

commands: [
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
]
```

And these files:

```bash
# .env
API_URL=http://envfile.example.com
DATABASE_URL=postgres://localhost/db

# .env.build
BUILD_MODE=release
CACHE_DIR=./cache
```

### Resolution Order

1. **Start with system environment** (e.g., `PATH`, `HOME`)

2. **Load root files** (`.env`):
   - `API_URL=http://envfile.example.com`
   - `DATABASE_URL=postgres://localhost/db`

3. **Apply root vars** (override files):
   - `API_URL=http://root.example.com` ← overrides `.env`
   - `LOG_LEVEL=info`

4. **Load command files** (`.env.build`):
   - `BUILD_MODE=release`
   - `CACHE_DIR=./cache`

5. **Apply command vars** (override files):
   - `API_URL=http://command.example.com` ← overrides root
   - `BUILD_MODE=development` ← overrides `.env.build`

6. **Apply implementation vars**:
   - `BUILD_MODE=production` ← overrides command
   - `NODE_ENV=production`

### Final Result

```bash
API_URL=http://command.example.com    # From command vars
LOG_LEVEL=info                         # From root vars
BUILD_MODE=production                  # From implementation vars
NODE_ENV=production                    # From implementation vars
DATABASE_URL=postgres://localhost/db   # From .env file
CACHE_DIR=./cache                      # From .env.build file
```

### With CLI Override

```bash
invowk cmd myproject build --env-var API_URL=http://cli.example.com
```

Now `API_URL=http://cli.example.com` because CLI has highest priority.

## Files vs Vars at Same Level

Within the same level, `vars` override `files`:

```cue
env: {
    files: [".env"]  // API_URL=from-file
    vars: {
        API_URL: "from-vars"  // This wins
    }
}
```

## Multiple Files at Same Level

Files are loaded in order; later files override earlier:

```cue
env: {
    files: [
        ".env",           // API_URL=base
        ".env.local",     // API_URL=local (wins)
    ]
}
```

## Platform-Specific Variables

Platform variables are applied after everything else:

```cue
implementations: [{
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
}]
```

Platform `env` is only applied if that platform matches and the variable isn't already set.

## Best Practices

### Use Appropriate Levels

```cue
// Root: shared across all commands
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
}]
```

### Override Pattern

Base config in files, overrides in vars:

```cue
env: {
    files: [".env"]              // Defaults
    vars: {
        OVERRIDE_THIS: "value"   // Specific override
    }
}
```

### Local Development

Use optional local files for developer overrides:

```cue
env: {
    files: [
        ".env",          // Committed defaults
        ".env.local?",   // Not committed, personal overrides
    ]
}
```

### CLI for Temporary Overrides

```bash
# Quick test with different config
invowk cmd myproject build -E DEBUG=true -E LOG_LEVEL=debug
```

## Debugging Precedence

To see final values, add debug output:

```cue
{
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
}
```

## Next Steps

- [Env Files](./env-files) - Load from .env files
- [Env Vars](./env-vars) - Set variables directly
