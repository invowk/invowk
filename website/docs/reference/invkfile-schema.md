---
sidebar_position: 2
---

# Invkfile Schema Reference

:::warning Alpha — Schema May Change
The invkfile schema is still evolving. Fields, types, and validation rules **may change between releases** as we stabilize the format. Always check the [changelog](https://github.com/invowk/invowk/releases) when upgrading.
:::

Complete reference for the invkfile schema. Invkfiles use [CUE](https://cuelang.org/) format for defining commands.

## Root Structure

Every invkfile must have a `group` and at least one command:

```cue
#Invkfile: {
    group:          string    // Required - prefix for all command names
    version?:       string    // Optional - schema version (e.g., "1.0")
    description?:   string    // Optional - describe this invkfile's purpose
    default_shell?: string    // Optional - override default shell
    workdir?:       string    // Optional - default working directory
    env?:           #EnvConfig      // Optional - global environment
    depends_on?:    #DependsOn      // Optional - global dependencies
    commands:       [...#Command]   // Required - at least one command
}
```

### group

**Type:** `string`  
**Required:** Yes

A mandatory prefix for all command names from this invkfile. Must start with a letter and can contain dot-separated segments.

```cue
// Valid group names
group: "build"
group: "my.project"
group: "com.example.tools"

// Invalid
group: "123abc"     // Can't start with a number
group: ".build"     // Can't start with a dot
group: "build."     // Can't end with a dot
group: "my..tools"  // Can't have consecutive dots
```

### version

**Type:** `string` (pattern: `^[0-9]+\.[0-9]+$`)  
**Required:** No  
**Default:** None

The invkfile schema version. Current version is `"1.0"`.

```cue
version: "1.0"
```

### description

**Type:** `string`  
**Required:** No

A summary of this invkfile's purpose. Shown when listing commands.

```cue
description: "Build and deployment commands for the web application"
```

### default_shell

**Type:** `string`  
**Required:** No  
**Default:** System default

Override the default shell for native runtime execution.

```cue
default_shell: "/bin/bash"
default_shell: "pwsh"
```

### workdir

**Type:** `string`  
**Required:** No  
**Default:** Invkfile directory

Default working directory for all commands. Can be absolute or relative to the invkfile location.

```cue
workdir: "./src"
workdir: "/opt/app"
```

### env

**Type:** `#EnvConfig`  
**Required:** No

Global environment configuration applied to all commands. See [EnvConfig](#envconfig).

### depends_on

**Type:** `#DependsOn`  
**Required:** No

Global dependencies that apply to all commands. See [DependsOn](#dependson).

### commands

**Type:** `[...#Command]`  
**Required:** Yes (at least one)

List of commands defined in this invkfile. See [Command](#command).

---

## Command

Defines an executable command:

```cue
#Command: {
    name:            string               // Required
    description?:    string               // Optional
    implementations: [...#Implementation] // Required - at least one
    env?:            #EnvConfig           // Optional
    workdir?:        string               // Optional
    depends_on?:     #DependsOn           // Optional
    flags?:          [...#Flag]           // Optional
    args?:           [...#Argument]       // Optional
}
```

### name

**Type:** `string` (pattern: `^[a-zA-Z][a-zA-Z0-9_ -]*$`)  
**Required:** Yes

The command identifier. Must start with a letter.

```cue
name: "build"
name: "test unit"     // Spaces allowed for subcommand-like behavior
name: "deploy-prod"
```

### description

**Type:** `string`  
**Required:** No

Help text for the command.

```cue
description: "Build the application for production"
```

### implementations

**Type:** `[...#Implementation]`  
**Required:** Yes (at least one)

The executable implementations. See [Implementation](#implementation).

### flags

**Type:** `[...#Flag]`  
**Required:** No

Command-line flags for this command. See [Flag](#flag).

:::warning Reserved Flags
`env-file` (short `e`) and `env-var` (short `E`) are reserved system flags and cannot be used.
:::

### args

**Type:** `[...#Argument]`  
**Required:** No

Positional arguments for this command. See [Argument](#argument).

---

## Implementation

Defines how a command is executed:

```cue
#Implementation: {
    script:      string       // Required - inline script or file path
    target:      #Target      // Required - runtime and platform constraints
    env?:        #EnvConfig   // Optional
    workdir?:    string       // Optional
    depends_on?: #DependsOn   // Optional
}
```

### script

**Type:** `string` (non-empty)  
**Required:** Yes

The shell commands to execute OR a path to a script file.

```cue
// Inline script
script: "echo 'Hello, World!'"

// Multi-line script
script: """
    echo "Building..."
    go build -o app .
    echo "Done!"
    """

// Script file reference
script: "./scripts/build.sh"
script: "deploy.py"
```

**Recognized extensions:** `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

### target

**Type:** `#Target`  
**Required:** Yes

Defines runtime and platform constraints. See [Target](#target-1).

---

## Target

Specifies where an implementation can run:

```cue
#Target: {
    runtimes:   [...#RuntimeConfig]   // Required - at least one
    platforms?: [...#PlatformConfig]  // Optional
}
```

### runtimes

**Type:** `[...#RuntimeConfig]`  
**Required:** Yes (at least one)

The runtimes that can execute this implementation. The first runtime is the default.

```cue
// Native only
runtimes: [{name: "native"}]

// Multiple runtimes
runtimes: [
    {name: "native"},
    {name: "virtual"},
]

// Container with options
runtimes: [{
    name: "container"
    image: "golang:1.22"
    volumes: ["./:/app"]
}]
```

### platforms

**Type:** `[...#PlatformConfig]`  
**Required:** No  
**Default:** All platforms

Restrict this implementation to specific operating systems.

```cue
// Linux and macOS only
platforms: [
    {name: "linux"},
    {name: "macos"},
]
```

---

## RuntimeConfig

Configuration for a specific runtime:

```cue
#RuntimeConfig: {
    name: "native" | "virtual" | "container"
    
    // For native and container:
    interpreter?: string
    
    // For container only:
    enable_host_ssh?: bool
    containerfile?:   string
    image?:           string
    volumes?:         [...string]
    ports?:           [...string]
}
```

### name

**Type:** `"native" | "virtual" | "container"`  
**Required:** Yes

The runtime type.

### interpreter

**Type:** `string`  
**Available for:** `native`, `container`  
**Default:** `"auto"` (detect from shebang)

Specifies how to execute the script.

```cue
// Auto-detect from shebang
interpreter: "auto"

// Specific interpreter
interpreter: "python3"
interpreter: "node"
interpreter: "/usr/bin/ruby"

// With arguments
interpreter: "python3 -u"
interpreter: "/usr/bin/env perl -w"
```

:::note
Virtual runtime uses mvdan/sh which cannot execute non-shell interpreters.
:::

### enable_host_ssh

**Type:** `bool`  
**Available for:** `container`  
**Default:** `false`

Enable SSH access from container back to the host. When enabled, Invowk starts an SSH server and provides credentials via environment variables:

- `INVOWK_SSH_HOST`
- `INVOWK_SSH_PORT`
- `INVOWK_SSH_USER`
- `INVOWK_SSH_TOKEN`

```cue
runtimes: [{
    name: "container"
    image: "alpine:latest"
    enable_host_ssh: true
}]
```

### containerfile / image

**Type:** `string`  
**Available for:** `container`

Specify the container source. These are **mutually exclusive**.

```cue
// Use a pre-built image
image: "alpine:latest"
image: "golang:1.22"

// Build from a Containerfile
containerfile: "./Containerfile"
containerfile: "./docker/Dockerfile.build"
```

### volumes

**Type:** `[...string]`  
**Available for:** `container`

Volume mounts in `host:container[:options]` format.

```cue
volumes: [
    "./src:/app/src",
    "/tmp:/tmp:ro",
    "${HOME}/.cache:/cache",
]
```

### ports

**Type:** `[...string]`  
**Available for:** `container`

Port mappings in `host:container` format.

```cue
ports: [
    "8080:80",
    "3000:3000",
]
```

---

## PlatformConfig

```cue
#PlatformConfig: {
    name: "linux" | "macos" | "windows"
}
```

---

## EnvConfig

Environment configuration:

```cue
#EnvConfig: {
    files?: [...string]         // Dotenv files to load
    vars?:  [string]: string    // Environment variables
}
```

### files

Dotenv files to load. Files are loaded in order; later files override earlier ones.

```cue
env: {
    files: [
        ".env",
        ".env.local",
        ".env.${ENVIRONMENT}?",  // '?' means optional
    ]
}
```

### vars

Environment variables as key-value pairs.

```cue
env: {
    vars: {
        NODE_ENV: "production"
        DEBUG: "false"
    }
}
```

---

## DependsOn

Dependency specification:

```cue
#DependsOn: {
    tools?:         [...#ToolDependency]
    commands?:      [...#CommandDependency]
    filepaths?:     [...#FilepathDependency]
    capabilities?:  [...#CapabilityDependency]
    custom_checks?: [...#CustomCheckDependency]
    env_vars?:      [...#EnvVarDependency]
}
```

### ToolDependency

```cue
#ToolDependency: {
    alternatives: [...string]  // At least one - tool names
}
```

```cue
depends_on: {
    tools: [
        {alternatives: ["go"]},
        {alternatives: ["podman", "docker"]},  // Either works
    ]
}
```

### CommandDependency

```cue
#CommandDependency: {
    alternatives: [...string]  // Command names
}
```

### FilepathDependency

```cue
#FilepathDependency: {
    alternatives: [...string]  // File/directory paths
    readable?:    bool
    writable?:    bool
    executable?:  bool
}
```

### CapabilityDependency

```cue
#CapabilityDependency: {
    alternatives: [...("local-area-network" | "internet")]
}
```

### EnvVarDependency

```cue
#EnvVarDependency: {
    alternatives: [...#EnvVarCheck]
}

#EnvVarCheck: {
    name:        string    // Environment variable name
    validation?: string    // Regex pattern
}
```

### CustomCheckDependency

```cue
#CustomCheckDependency: #CustomCheck | #CustomCheckAlternatives

#CustomCheck: {
    name:             string  // Check identifier
    check_script:     string  // Script to run
    expected_code?:   int     // Expected exit code (default: 0)
    expected_output?: string  // Regex to match output
}

#CustomCheckAlternatives: {
    alternatives: [...#CustomCheck]
}
```

---

## Flag

Command-line flag definition:

```cue
#Flag: {
    name:          string    // POSIX-compliant name
    description:   string    // Help text
    default_value?: string   // Default value
    type?:         "string" | "bool" | "int" | "float"
    required?:     bool
    short?:        string    // Single character alias
    validation?:   string    // Regex pattern
}
```

```cue
flags: [
    {
        name: "output"
        short: "o"
        description: "Output file path"
        default_value: "./build"
    },
    {
        name: "verbose"
        short: "v"
        description: "Enable verbose output"
        type: "bool"
    },
]
```

---

## Argument

Positional argument definition:

```cue
#Argument: {
    name:          string    // POSIX-compliant name
    description:   string    // Help text
    required?:     bool      // Must be provided
    default_value?: string   // Default if not provided
    type?:         "string" | "int" | "float"
    validation?:   string    // Regex pattern
    variadic?:     bool      // Accepts multiple values (last arg only)
}
```

```cue
args: [
    {
        name: "target"
        description: "Build target"
        required: true
    },
    {
        name: "files"
        description: "Files to process"
        variadic: true
    },
]
```

**Environment Variables for Arguments:**
- `INVOWK_ARG_<NAME>` - The argument value
- For variadic: `INVOWK_ARG_<NAME>_COUNT`, `INVOWK_ARG_<NAME>_1`, `INVOWK_ARG_<NAME>_2`, etc.

---

## Complete Example

```cue
group: "myapp"
version: "1.0"
description: "Build and deployment commands"

env: {
    files: [".env"]
    vars: {
        APP_NAME: "myapp"
    }
}

commands: [
    {
        name: "build"
        description: "Build the application"
        
        flags: [
            {
                name: "release"
                short: "r"
                description: "Build for release"
                type: "bool"
            },
        ]
        
        implementations: [
            {
                script: """
                    if [ "$INVOWK_FLAG_RELEASE" = "true" ]; then
                        go build -ldflags="-s -w" -o app .
                    else
                        go build -o app .
                    fi
                    """
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            },
            {
                script: """
                    $flags = if ($env:INVOWK_FLAG_RELEASE -eq "true") { "-ldflags=-s -w" } else { "" }
                    go build $flags -o app.exe .
                    """
                target: {
                    runtimes: [{name: "native", interpreter: "pwsh"}]
                    platforms: [{name: "windows"}]
                }
            },
        ]
        
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
    },
]
```
