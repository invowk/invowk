# Issue: Update Website Documentation

**Category**: Documentation
**Priority**: Low
**Effort**: Medium (2-3 days)
**Labels**: `documentation`, `website`

## Summary

The website documentation is missing several sections that would help users understand and use invowk effectively. Add architecture overview, troubleshooting guide, and contributing guide.

## Problem

**Current documentation sections** (11 total in `website/docs/`):
- Getting started
- Configuration
- Commands
- Modules
- Runtimes
- Container execution
- CUE basics
- CLI reference
- (others)

**Missing sections**:
- Architecture overview
- Troubleshooting guide
- Contributing guide
- Performance tuning

## Solution

### 1. Architecture Overview (`website/docs/architecture.md`)

```markdown
---
sidebar_position: 15
title: Architecture
---

# Architecture Overview

This document explains how invowk works internally, which helps when
troubleshooting issues or contributing to the project.

## Execution Flow

```
invkfile.cue → CUE Parser → pkg/invkfile → Runtime Selection → Execution
                                                  │
                                  ┌───────────────┼───────────────┐
                                  │               │               │
                               Native         Virtual        Container
                            (host shell)    (mvdan/sh)    (Docker/Podman)
```

## Key Components

### CUE Parser
Invowk uses CUE for configuration files. The parser:
1. Reads `invkfile.cue` and validates against the schema
2. Unifies with default values
3. Produces a typed `Invkfile` structure

### Runtime Selection
When you run a command, invowk selects the appropriate runtime:
1. Check if command specifies a runtime
2. Check platform constraints (os, arch)
3. Check capability requirements
4. Fall back to configured default

### Module Resolution
Modules are discovered from:
1. Current directory
2. `~/.invowk/modules/`
3. System paths (`/usr/share/invowk/modules/`)
4. Custom paths from configuration

Dependencies are resolved with first-level visibility only.

## Configuration Layers

Configuration is loaded with this precedence (highest to lowest):
1. CLI flags
2. Environment variables
3. Project `config.cue`
4. User `~/.invowk/config.cue`
5. System defaults

## Environment Variable Precedence

When executing commands, environment variables follow a 10-level precedence:
1. Extra env from CLI (`-e KEY=value`)
2. Runtime env vars from command
3. Runtime env files (`.env`)
4. Command-level env
5. Invkfile-level env
6. Module-level env
7. User config env
8. System environment (if inherited)
9. Schema defaults
10. Filtered INVOWK_* variables
```

### 2. Troubleshooting Guide (`website/docs/troubleshooting.md`)

```markdown
---
sidebar_position: 20
title: Troubleshooting
---

# Troubleshooting

Common issues and their solutions.

## Container Issues

### "No container engine available"

**Problem**: invowk can't find Docker or Podman.

**Solutions**:
1. Install Docker or Podman
2. Ensure the daemon is running: `docker info` or `podman info`
3. Check your user has permission to access the socket
4. On Linux, add your user to the `docker` group

### "Image not found"

**Problem**: Container image doesn't exist locally or in registry.

**Solutions**:
1. Pull the image manually: `docker pull debian:stable-slim`
2. Check for typos in the image name
3. Ensure you have network access to the registry

### Container commands fail silently

**Problem**: Commands run but produce no output.

**Solutions**:
1. Use `--verbose` flag for detailed logging
2. Check container logs: `docker logs <container-id>`
3. Run interactively: `invowk cmd run --interactive my-cmd`

## Path Issues

### "Script not found"

**Problem**: invowk can't find the script file.

**Solutions**:
1. Use absolute paths or paths relative to invkfile location
2. Check file permissions
3. On Windows, ensure forward slashes in paths

### "Module not found"

**Problem**: Referenced module can't be located.

**Solutions**:
1. Check module exists in search paths
2. Verify module name matches directory (e.g., `com.example.tools.invkmod`)
3. Run `invowk module list` to see discovered modules

## Environment Issues

### Environment variables not inherited

**Problem**: System environment variables aren't available in commands.

**Solutions**:
1. Check `env_inherit` mode in your command
2. Use `env_inherit: "all"` to inherit all variables
3. Explicitly list variables with `env_inherit: "explicit"`

### .env file not loaded

**Problem**: Variables from .env file aren't available.

**Solutions**:
1. Ensure file is named `.env` (with leading dot)
2. Check file path is relative to invkfile location
3. Verify file format: `KEY=value` (no spaces around `=`)

## Debug Mode

Enable verbose logging for detailed information:

```bash
invowk --verbose cmd run my-command
```

For even more detail:

```bash
INVOWK_DEBUG=1 invowk cmd run my-command
```

## Getting Help

If you're still stuck:
1. Check [GitHub Issues](https://github.com/invowk/invowk/issues)
2. Search existing discussions
3. Open a new issue with:
   - invowk version (`invowk --version`)
   - OS and architecture
   - Minimal reproduction case
   - Full error output
```

### 3. Contributing Guide (`website/docs/contributing.md`)

```markdown
---
sidebar_position: 25
title: Contributing
---

# Contributing to invowk

We welcome contributions! This guide helps you get started.

## Development Setup

### Prerequisites

- Go 1.25+
- Make
- Docker or Podman (for container tests)
- Node.js 20+ (for website development)

### Clone and Build

```bash
git clone https://github.com/invowk/invowk.git
cd invowk
make build
```

### Run Tests

```bash
# All tests
make test

# Short tests (skip integration)
make test-short

# CLI integration tests
make test-cli

# Specific package
go test -v ./internal/runtime/...
```

## Code Style

We use several linters to maintain code quality:

```bash
make lint
```

Key conventions:
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` formatting
- Add doc comments to all exported symbols
- See `.claude/rules/go-patterns.md` for project-specific patterns

## Commit Messages

Use Conventional Commits format:

```
type(scope): description

- Bullet points describing changes
- ...
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**All commits must be GPG-signed** (`git commit -S`).

## Pull Requests

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linter
5. Submit a PR

PRs should:
- Have a clear description
- Include tests for new functionality
- Update documentation if needed
- Pass all CI checks

## Project Structure

```
cmd/invowk/     - CLI commands (Cobra)
internal/       - Private packages
  config/       - Configuration management
  container/    - Docker/Podman abstraction
  discovery/    - Module and command discovery
  runtime/      - Execution runtimes
  tui/          - Terminal UI components
pkg/            - Public packages
  invkfile/     - Invkfile parsing and types
  invkmod/      - Module parsing and types
website/        - Documentation site
```

## Testing

### Unit Tests

Add `_test.go` files alongside implementation:

```go
func TestMyFunction(t *testing.T) {
    // Table-driven tests preferred
    tests := []struct {
        name string
        input string
        want string
    }{
        {"basic", "hello", "HELLO"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := MyFunction(tt.input)
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

CLI tests use [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript):

```
# tests/cli/testdata/my_test.txtar
exec invowk cmd list
stdout 'hello'

-- invkfile.cue --
cmds: {
    hello: {
        desc: "Test command"
        impls: [{script: "echo hello"}]
    }
}
```

## License

By contributing, you agree that your contributions will be licensed under MPL-2.0.

All Go files must include the SPDX header:
```go
// SPDX-License-Identifier: MPL-2.0
```
```

## Implementation Steps

1. [ ] Create `website/docs/architecture.md`
2. [ ] Create `website/docs/troubleshooting.md`
3. [ ] Create `website/docs/contributing.md`
4. [ ] Update `website/sidebars.js` if needed
5. [ ] Add cross-links from existing docs
6. [ ] Verify build: `cd website && npm run build`
7. [ ] Preview locally: `cd website && npm start`

## Acceptance Criteria

- [ ] Architecture overview explains key components
- [ ] Troubleshooting covers common issues
- [ ] Contributing guide helps new developers
- [ ] All pages render correctly
- [ ] Navigation includes new pages
- [ ] `npm run build` passes
- [ ] Internal links work

## Testing

```bash
# Build website
cd website && npm run build

# Preview locally
cd website && npm start
# Open http://localhost:3000
```

## Notes

- Follow existing documentation style
- Include code examples where helpful
- Keep troubleshooting solutions actionable
- Update as new features are added
