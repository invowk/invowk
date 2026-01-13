---
sidebar_position: 2
---

# Env Files

Load environment variables from `.env` files. This is a common pattern for managing configuration, especially secrets that shouldn't be committed to version control.

## Basic Usage

```cue
{
    name: "build"
    env: {
        files: [".env"]
    }
    implementations: [...]
}
```

With a `.env` file:
```bash
# .env
API_KEY=secret123
DATABASE_URL=postgres://localhost/mydb
DEBUG=false
```

Variables are loaded and available in your script.

## File Format

Standard `.env` format:

```bash
# Comments start with #
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
line3"
```

## Optional Files

Suffix with `?` to make a file optional:

```cue
env: {
    files: [
        ".env",           // Required - error if missing
        ".env.local?",    // Optional - ignored if missing
        ".env.secrets?",  // Optional
    ]
}
```

This is useful for:
- Local overrides that may not exist
- Environment-specific files
- Developer-specific settings

## File Order

Files are loaded in order; later files override earlier ones:

```cue
env: {
    files: [
        ".env",           // Base config
        ".env.${ENV}?",   // Environment-specific overrides
        ".env.local?",    // Local overrides (highest priority)
    ]
}
```

Example with `ENV=production`:

```bash
# .env
API_URL=http://localhost:3000
DEBUG=true

# .env.production
API_URL=https://api.example.com
DEBUG=false

# .env.local (developer override)
DEBUG=true
```

Result:
- `API_URL=https://api.example.com` (from .env.production)
- `DEBUG=true` (from .env.local)

## Path Resolution

Paths are relative to the invkfile location:

```
project/
├── invkfile.cue
├── .env                  # files: [".env"]
├── config/
│   └── .env.prod         # files: ["config/.env.prod"]
└── src/
```

For packs, paths are relative to the pack root.

## Variable Interpolation

Use `${VAR}` to include other environment variables:

```cue
env: {
    files: [
        ".env",
        ".env.${NODE_ENV}?",    // Uses NODE_ENV value
        ".env.${USER}?",        // Uses current user
    ]
}
```

```bash
# If NODE_ENV=production, loads:
# - .env
# - .env.production (if exists)
# - .env.john (if exists and USER=john)
```

## Scope Levels

Env files can be loaded at multiple levels:

### Root Level

```cue
group: "myproject"

env: {
    files: [".env"]  // Loaded for all commands
}

commands: [...]
```

### Command Level

```cue
{
    name: "build"
    env: {
        files: [".env.build"]  // Only for this command
    }
    implementations: [...]
}
```

### Implementation Level

```cue
{
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
}
```

## Combined with Variables

Use both files and direct variables:

```cue
env: {
    files: [".env"]
    vars: {
        // These override values from .env
        OVERRIDE_VALUE: "from-invkfile"
    }
}
```

Variables in `vars` always override values from files at the same level.

## CLI Override

Load additional files at runtime:

```bash
# Load extra file
invowk cmd myproject build --env-file .env.custom

# Short form
invowk cmd myproject build -e .env.custom

# Multiple files
invowk cmd myproject build -e .env.custom -e .env.secrets
```

CLI files have highest priority and override all invkfile-defined sources.

## Real-World Examples

### Development vs Production

```cue
{
    name: "start"
    env: {
        files: [
            ".env",                    // Base config
            ".env.${NODE_ENV:-dev}?",  // Environment-specific
            ".env.local?",             // Local overrides
        ]
    }
    implementations: [{
        script: "node server.js"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Secrets Management

```cue
{
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
}
```

`.gitignore`:
```
.env.secrets
.env.local
```

### Multi-Environment

```
project/
├── invkfile.cue
├── .env                  # Shared defaults
├── .env.development      # Dev settings
├── .env.staging          # Staging settings
└── .env.production       # Production settings
```

```cue
{
    name: "deploy"
    env: {
        files: [
            ".env",
            ".env.${DEPLOY_ENV}",  // DEPLOY_ENV must be set
        ]
    }
    depends_on: {
        env_vars: [
            {alternatives: [{name: "DEPLOY_ENV", validation: "^(development|staging|production)$"}]}
        ]
    }
    implementations: [...]
}
```

## Best Practices

1. **Use `.env` for defaults**: Base configuration that works for everyone
2. **Use `.env.local` for overrides**: Developer-specific settings, not in git
3. **Use `.env.{environment}` for environments**: Production, staging, etc.
4. **Mark sensitive files optional**: They may not exist in all environments
5. **Don't commit secrets**: Add `.env.secrets`, `.env.local` to `.gitignore`

## Next Steps

- [Env Vars](./env-vars) - Set variables directly
- [Precedence](./precedence) - Understand override order
