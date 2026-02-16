import type { Snippet } from '../snippets';

export const modulesSnippets = {
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
  build - Build the project [native*]

From com.example.utilities.invowkmod:
  hello - Greeting [native*]`,
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
script: "scripts\build.sh"

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
script: "scripts\build.sh"

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
invowk module archive ./com.company.mytools.invowkmod 
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
} satisfies Record<string, Snippet>;
