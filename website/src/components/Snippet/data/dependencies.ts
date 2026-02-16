import type { Snippet } from '../snippets';

export const dependenciesSnippets = {
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
            validation: "^[0-9]+\.[0-9]+\.[0-9]+$"
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
                validation: "^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$"
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
    code: `validation: "^[0-9]+\.[0-9]+\.[0-9]+$"
// Matches: 1.0.0, 2.1.3
// Rejects: v1.0.0, 1.0, latest`,
  },

  'dependencies/env-vars-pattern-url': {
    language: 'cue',
    code: `validation: "^https?://[^\s]+$"
// Matches: http://localhost, https://example.com/path
// Rejects: ftp://server, not-a-url`,
  },

  'dependencies/env-vars-pattern-email': {
    language: 'cue',
    code: `validation: "^[^@]+@[^@]+\.[^@]+$"
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
        check_script: "test "$ENV" = 'production'"
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
        expected_output: "^v(18|20|22)\."  // Major version 18, 20, or 22
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
        expected_output: "go1\.2[6-9]"  // Must be Go 1.26+
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
                    version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
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
                check_script: "test -z "$(git status --porcelain)""
            },
            {
                name: "on-main-branch"
                check_script: "test "$(git branch --show-current)" = 'main'"
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
                check_script: "python3 -c 'import yaml; yaml.safe_load(open("config.yaml"))'"
            },
            {
                name: "has-required-fields"
                check_script: """
                    grep -q 'database:' config.yaml && 
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
                        expected_output: "^Python 3\.11"
                    },
                    {
                        name: "python-3.12"
                        check_script: "python3 --version"
                        expected_output: "^Python 3\.12"
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
                    expected_output: "^v20\."
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
    echo "$version" | grep -qE 'go1\.(2[6-9]|[3-9][0-9])'
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
} satisfies Record<string, Snippet>;
