import type { Snippet } from '../snippets';

export const environmentSnippets = {
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
            echo "Date: $BUILD_DATE"
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
                PLATFORM_CONFIG: "%APPDATA%\myapp"
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

  'environment/env-inherit-cli': {
    language: 'bash',
    code: `invowk cmd examples hello 
  --ivk-env-inherit-mode allow 
  --ivk-env-inherit-allow TERM 
  --ivk-env-inherit-allow LANG 
  --ivk-env-inherit-deny AWS_SECRET_ACCESS_KEY`,
  },
};
