---
sidebar_position: 1
---

# Packs Overview

Packs are self-contained folders that bundle an invkfile together with its script files. They're perfect for sharing commands, creating reusable toolkits, and distributing automation across teams.

## What is a Pack?

A pack is a directory with the `.invkpack` suffix:

```
mytools.invkpack/
├── invkfile.cue          # Required: command definitions
├── scripts/               # Optional: script files
│   ├── build.sh
│   └── deploy.sh
└── templates/             # Optional: other resources
    └── config.yaml
```

## Why Use Packs?

- **Portability**: Share a complete command set as a single folder
- **Self-contained**: Scripts are bundled with the invkfile
- **Cross-platform**: Forward slash paths work everywhere
- **Namespace isolation**: RDNS naming prevents conflicts
- **Easy distribution**: Zip, share, unzip

## Quick Start

### Create a Pack

```bash
invowk pack create mytools
```

Creates:
```
mytools.invkpack/
└── invkfile.cue
```

### Use the Pack

Packs are automatically discovered from:
1. Current directory
2. `~/.invowk/cmds/` (user commands)
3. Configured search paths

```bash
# List commands (pack commands appear automatically)
invowk cmd list

# Run a pack command
invowk cmd mytools hello
```

### Share the Pack

```bash
# Create a zip archive
invowk pack archive mytools.invkpack

# Share the zip file
# Recipients import with:
invowk pack import mytools.invkpack.zip
```

## Pack Structure

### Required Files

- **`invkfile.cue`**: Command definitions (must be at pack root)

### Optional Contents

- **Scripts**: Shell scripts, Python files, etc.
- **Templates**: Configuration templates
- **Data**: Any supporting files

### Example Structure

```
com.example.devtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build.sh
│   ├── deploy.sh
│   └── utils/
│       └── helpers.sh
├── templates/
│   ├── Dockerfile.tmpl
│   └── config.yaml.tmpl
└── README.md
```

## Pack Naming

Pack folder names follow these rules:

| Rule | Valid | Invalid |
|------|-------|---------|
| End with `.invkpack` | `mytools.invkpack` | `mytools` |
| Start with letter | `mytools.invkpack` | `123tools.invkpack` |
| Alphanumeric + dots | `com.example.invkpack` | `my-tools.invkpack` |

### RDNS Naming

Recommended for shared packs:

```
com.company.projectname.invkpack
io.github.username.toolkit.invkpack
org.opensource.utilities.invkpack
```

## Script Paths

Reference scripts relative to pack root with **forward slashes**:

```cue
// Inside mytools.invkpack/invkfile.cue
group: "mytools"

commands: [
    {
        name: "build"
        implementations: [{
            script: "scripts/build.sh"  // Relative to pack root
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "deploy"
        implementations: [{
            script: "scripts/utils/helpers.sh"  // Nested path
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

**Important:**
- Always use forward slashes (`/`)
- Paths are relative to pack root
- No absolute paths allowed
- Can't escape pack directory (`../` is invalid)

## Pack Commands

| Command | Description |
|---------|-------------|
| `invowk pack create` | Create a new pack |
| `invowk pack validate` | Validate pack structure |
| `invowk pack list` | List discovered packs |
| `invowk pack archive` | Create zip archive |
| `invowk pack import` | Install from zip/URL |

## Discovery

Packs are discovered from these locations:

1. **Current directory** (highest priority)
2. **User commands** (`~/.invowk/cmds/`)
3. **Search paths** (from config)

Commands appear in `invowk cmd list` with their source:

```
Available Commands

From current directory:
  mytools build - Build the project [native*]

From user commands (~/.invowk/cmds):
  com.example.utilities hello - Greeting [native*]
```

## Next Steps

- [Creating Packs](./creating-packs) - Scaffold and structure packs
- [Validating](./validating) - Ensure pack integrity
- [Distributing](./distributing) - Share packs with others
