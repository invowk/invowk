---
sidebar_position: 4
---

# Container Runtime

The **container** runtime executes commands inside a Docker or Podman container. It provides complete isolation and reproducibility - your command runs in the exact same environment every time.

## How It Works

When you run a command with the container runtime, Invowk:

1. Pulls or builds the container image (if needed)
2. Mounts the current directory into the container
3. Executes the script inside the container
4. Streams output back to your terminal

## Basic Usage

```cue
{
    name: "build"
    implementations: [{
        script: "go build -o /workspace/bin/app ./..."
        target: {
            runtimes: [{
                name: "container"
                image: "golang:1.21"  // Required: specify an image
            }]
        }
    }]
}
```

```bash
invowk cmd myproject build
```

## Container Image Sources

You must specify either an `image` or a `containerfile` - they're mutually exclusive.

### Pre-built Images

```cue
runtimes: [{
    name: "container"
    image: "golang:1.21"
}]
```

Common images:
- `alpine:latest` - Minimal Linux
- `ubuntu:22.04` - Full Ubuntu
- `golang:1.21` - Go development
- `node:20` - Node.js development
- `python:3.11` - Python development

### Custom Containerfile

Build from a local Containerfile/Dockerfile:

```cue
runtimes: [{
    name: "container"
    containerfile: "./Containerfile"  // Relative to invkfile
}]
```

Example `Containerfile`:

```dockerfile
FROM golang:1.21

RUN apt-get update && apt-get install -y \
    make \
    git

WORKDIR /workspace
```

## Volume Mounts

Mount additional directories into the container:

```cue
runtimes: [{
    name: "container"
    image: "golang:1.21"
    volumes: [
        "./data:/data",           // Relative path
        "/tmp:/tmp:ro",           // Absolute path, read-only
        "${HOME}/.cache:/cache"   // Environment variable
    ]
}]
```

The current working directory is automatically mounted to `/workspace`.

## Port Mappings

Expose container ports to the host:

```cue
runtimes: [{
    name: "container"
    image: "node:20"
    ports: [
        "3000:3000",      // Host:Container
        "8080:80"         // Map container port 80 to host port 8080
    ]
}]
```

## Using Interpreters

Like native runtime, containers support custom interpreters:

### Auto-Detection from Shebang

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            print(f"Python {sys.version} in container!")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11"
            }]
        }
    }]
}
```

### Explicit Interpreter

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            import sys
            print(f"Running on Python {sys.version_info.major}")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11"
                interpreter: "python3"
            }]
        }
    }]
}
```

## Environment Variables

Environment variables are passed into the container:

```cue
{
    name: "deploy"
    env: {
        vars: {
            DEPLOY_ENV: "production"
            API_URL: "https://api.example.com"
        }
    }
    implementations: [{
        script: """
            echo "Deploying to $DEPLOY_ENV"
            echo "API: $API_URL"
            """
        target: {
            runtimes: [{
                name: "container"
                image: "alpine:latest"
            }]
        }
    }]
}
```

## Host SSH Access

Sometimes your container needs to execute commands on the host system. Enable SSH access back to the host:

```cue
{
    name: "deploy from container"
    implementations: [{
        script: """
            # Connection credentials are provided via environment variables
            echo "SSH Host: $INVOWK_SSH_HOST"
            echo "SSH Port: $INVOWK_SSH_PORT"
            
            # Connect back to host
            sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no \
                $INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT \
                'echo "Hello from host!"'
            """
        target: {
            runtimes: [{
                name: "container"
                image: "alpine:latest"
                enable_host_ssh: true  // Enable SSH server
            }]
        }
    }]
}
```

### SSH Environment Variables

When `enable_host_ssh: true`, these variables are available:

| Variable | Description |
|----------|-------------|
| `INVOWK_SSH_HOST` | Host address (e.g., `host.docker.internal`) |
| `INVOWK_SSH_PORT` | SSH server port |
| `INVOWK_SSH_USER` | Username (`invowk`) |
| `INVOWK_SSH_TOKEN` | One-time authentication token |

### Security

- Each command execution gets a unique token
- Tokens are revoked when the command completes
- The SSH server only accepts token-based authentication
- The server shuts down after command execution

### Container Requirements

Your container needs `sshpass` or similar for password-based SSH:

```dockerfile
FROM alpine:latest
RUN apk add --no-cache openssh-client sshpass
```

## Dependencies

Container dependencies are validated **inside the container**:

```cue
{
    name: "build"
    depends_on: {
        tools: [
            // Checked inside the container, not on host
            {alternatives: ["go"]},
            {alternatives: ["make"]}
        ]
        filepaths: [
            // Paths relative to container's /workspace
            {alternatives: ["go.mod"]}
        ]
    }
    implementations: [{
        script: "make build"
        target: {
            runtimes: [{
                name: "container"
                image: "golang:1.21"
            }]
        }
    }]
}
```

## Container Engine

Invowk supports both Docker and Podman. Configure your preference:

```cue
// ~/.config/invowk/config.cue
container_engine: "podman"  // or "docker"
```

If not configured, Invowk tries:
1. `podman` (if available)
2. `docker` (fallback)

## Working Directory

By default, the current directory is mounted to `/workspace` and used as the working directory:

```cue
{
    name: "build"
    implementations: [{
        script: """
            pwd  # Outputs: /workspace
            ls   # Shows your project files
            """
        target: {
            runtimes: [{
                name: "container"
                image: "alpine:latest"
            }]
        }
    }]
}
```

Override with `workdir`:

```cue
{
    name: "build frontend"
    workdir: "./frontend"  // Mounted and used as workdir
    implementations: [{
        script: "npm run build"
        target: {
            runtimes: [{
                name: "container"
                image: "node:20"
            }]
        }
    }]
}
```

## Complete Example

Here's a full-featured container command:

```cue
{
    name: "build and test"
    description: "Build and test in isolated container"
    env: {
        vars: {
            GO_ENV: "test"
            CGO_ENABLED: "0"
        }
    }
    depends_on: {
        tools: [{alternatives: ["go"]}]
        filepaths: [{alternatives: ["go.mod"]}]
    }
    implementations: [{
        script: """
            echo "Go version: $(go version)"
            echo "Building..."
            go build -o /workspace/bin/app ./...
            echo "Testing..."
            go test -v ./...
            echo "Done!"
            """
        target: {
            runtimes: [{
                name: "container"
                image: "golang:1.21-alpine"
                volumes: [
                    "${HOME}/go/pkg/mod:/go/pkg/mod:ro"  // Cache modules
                ]
            }]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    }]
}
```

## Advantages

- **Reproducibility**: Same environment everywhere
- **Isolation**: No host system pollution
- **Version control**: Pin exact tool versions
- **CI/CD parity**: Local builds match CI builds
- **Clean builds**: Fresh environment each time

## Limitations

- **Performance**: Container startup overhead
- **Disk space**: Images consume storage
- **Complexity**: Need to manage images
- **Host access**: Limited without SSH bridge

## When to Use Container

- **Reproducible builds**: When consistency matters
- **CI/CD pipelines**: Match local and CI environments
- **Legacy projects**: Isolate old tool versions
- **Team onboarding**: No local tool installation needed
- **Clean-room builds**: Test without host pollution

## Troubleshooting

### Container Not Starting

```bash
# Check if container engine is available
docker --version  # or: podman --version

# Check if image exists
docker images | grep golang
```

### Slow First Run

The first run pulls the image. Subsequent runs are faster:

```bash
# Pre-pull images
docker pull golang:1.21
docker pull node:20
```

### Permission Issues

On Linux, you may need to configure container permissions:

```bash
# For Docker
sudo usermod -aG docker $USER

# For Podman (rootless)
# Usually works out of the box
```

## Next Steps

- [Native Runtime](./native) - For development speed
- [Virtual Runtime](./virtual) - For cross-platform scripts
- [Dependencies](../dependencies/overview) - Declare command requirements
