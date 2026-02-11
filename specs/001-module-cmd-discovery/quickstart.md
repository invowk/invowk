# Quickstart: Module-Aware Command Discovery

**Date**: 2026-01-21
**Feature**: 001-module-cmd-discovery

## Overview

This feature extends `invowk cmd` to discover commands from multiple sources in a single directory:
- The root `invkfile.cue` (if present)
- All sibling `*.invkmod` module directories (excluding their dependencies)

Commands are displayed with simple names when unique, or with source annotations when names conflict.

## Setup for Development

### Prerequisites

```bash
# Ensure Go 1.26+ is installed
go version

# Clone and build
cd /var/home/danilo/Workspace/github/invowk/invowk
make build
```

### Test Workspace Setup

Create a test directory with multiple sources:

```bash
mkdir -p /tmp/multi-source-test
cd /tmp/multi-source-test

# Create root invkfile
cat > invkfile.cue << 'EOF'
cmds: [
    {
        name: "hello"
        description: "Hello from root invkfile"
        implementations: [{script: "echo 'Hello from invkfile!'", runtimes: [{name: "virtual"}]}]
    },
    {
        name: "deploy"
        description: "Deploy from root"
        implementations: [{script: "echo 'Deploying from root...'", runtimes: [{name: "virtual"}]}]
    },
]
EOF

# Create first module
mkdir -p foo.invkmod
cat > foo.invkmod/invkmod.cue << 'EOF'
module: "foo"
version: "1.0"
description: "Foo module"
EOF

cat > foo.invkmod/invkfile.cue << 'EOF'
cmds: [
    {
        name: "build"
        description: "Build from foo module"
        implementations: [{script: "echo 'Building from foo...'", runtimes: [{name: "virtual"}]}]
    },
    {
        name: "deploy"
        description: "Deploy from foo module"
        implementations: [{script: "echo 'Deploying from foo...'", runtimes: [{name: "virtual"}]}]
    },
]
EOF

# Create second module
mkdir -p bar.invkmod
cat > bar.invkmod/invkmod.cue << 'EOF'
module: "bar"
version: "1.0"
description: "Bar module"
EOF

cat > bar.invkmod/invkfile.cue << 'EOF'
cmds: [
    {
        name: "test"
        description: "Test from bar module"
        implementations: [{script: "echo 'Testing from bar...'", runtimes: [{name: "virtual"}]}]
    },
]
EOF
```

## Usage Examples

### Listing Commands

```bash
# List all available commands (grouped by source)
invowk cmd

# Expected output:
# Available Commands
#   (* = default runtime)
#
# From invkfile:
#   hello          - Hello from root invkfile [virtual*]
#   deploy         - Deploy from root (@invkfile) [virtual*]
#
# From bar.invkmod:
#   test           - Test from bar module [virtual*]
#
# From foo.invkmod:
#   build          - Build from foo module [virtual*]
#   deploy         - Deploy from foo module (@foo) [virtual*]
```

Note: `(@invkfile)` and `(@foo)` annotations appear because `deploy` exists in multiple sources.

### Running Unambiguous Commands

```bash
# Commands with unique names work as before
invowk cmd hello     # Runs hello from invkfile
invowk cmd build     # Runs build from foo module
invowk cmd test      # Runs test from bar module
```

### Running Ambiguous Commands

```bash
# Ambiguous command without disambiguation - shows error
invowk cmd deploy
# Error: 'deploy' is ambiguous. Found in:
#   - @invkfile: Deploy from root
#   - @foo: Deploy from foo module
# Use 'invowk cmd @<source> deploy' or 'invowk cmd --from <source> deploy'

# Using @ prefix syntax
invowk cmd @invkfile deploy    # Runs deploy from invkfile
invowk cmd @foo deploy         # Runs deploy from foo module
invowk cmd @foo.invkmod deploy # Also works with full name

# Using --from flag (must be before command name)
invowk cmd --from invkfile deploy
invowk cmd --from foo deploy
```

### Explicit Source for Non-Ambiguous Commands

```bash
# You can always specify source explicitly (FR-009c)
invowk cmd @foo build   # Works even though 'build' is unambiguous
invowk cmd --from bar test
```

### Verbose Mode

```bash
# See discovery details
invowk cmd --verbose

# Shows which sources were scanned and command counts
```

## Key Behaviors

| Scenario | Behavior |
|----------|----------|
| Single invkfile, no modules | Unchanged from current behavior |
| Multiple sources, unique names | Simple names work, listing grouped by source |
| Multiple sources, conflicting names | Listing shows source annotations, execution requires disambiguation |
| `@source` with non-existent source | Error with suggestion of valid sources |
| `invkfile.invkmod` module | Rejected with warning (reserved name) |

## Running Tests

```bash
# Unit tests
go test ./internal/discovery/... -v

# CLI integration tests
make test-cli

# Specific test files (when created)
go test ./tests/cli/... -run TestMultiSource
```

## Debugging

```bash
# Check what sources are discovered
invowk cmd --verbose

# Validate module structure
invowk module validate foo.invkmod --deep

# Check for naming conflicts
invowk cmd 2>&1 | grep -i ambiguous
```
