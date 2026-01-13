---
sidebar_position: 4
---

# Distributing Packs

Share your packs with teammates, across organizations, or with the world.

## Creating Archives

Create a zip archive for distribution:

```bash
# Default output: <pack-name>.invkpack.zip
invowk pack archive ./mytools.invkpack

# Custom output path
invowk pack archive ./mytools.invkpack --output ./dist/mytools.zip
```

Output:
```
Archive Pack

✓ Pack archived successfully

• Output: /home/user/dist/mytools.zip
• Size: 2.45 KB
```

## Importing Packs

### From Local File

```bash
# Install to ~/.invowk/cmds/
invowk pack import ./mytools.invkpack.zip

# Install to custom directory
invowk pack import ./mytools.invkpack.zip --path ./local-packs

# Overwrite existing
invowk pack import ./mytools.invkpack.zip --overwrite
```

### From URL

```bash
# Download and install
invowk pack import https://example.com/packs/mytools.zip

# From GitHub release
invowk pack import https://github.com/user/repo/releases/download/v1.0/mytools.invkpack.zip
```

Output:
```
Import Pack

✓ Pack imported successfully

• Name: mytools
• Path: /home/user/.invowk/cmds/mytools.invkpack

• The pack commands are now available via invowk
```

## Listing Installed Packs

```bash
invowk pack list
```

Output:
```
Discovered Packs

• Found 3 pack(s)

• current directory:
   ✓ local.project
      /home/user/project/local.project.invkpack

• user commands (~/.invowk/cmds):
   ✓ com.company.devtools
      /home/user/.invowk/cmds/com.company.devtools.invkpack
   ✓ io.github.user.utilities
      /home/user/.invowk/cmds/io.github.user.utilities.invkpack
```

## Distribution Methods

### Direct Sharing

1. Create archive: `invowk pack archive`
2. Share the zip file (email, Slack, etc.)
3. Recipient imports: `invowk pack import`

### Git Repository

Include packs in your repo:

```
my-project/
├── src/
├── packs/
│   ├── devtools.invkpack/
│   └── testing.invkpack/
└── invkfile.cue
```

Team members get packs when they clone the repo.

### GitHub Releases

1. Create archive
2. Attach to GitHub release
3. Share the download URL

```bash
# Recipients install with:
invowk pack import https://github.com/org/repo/releases/download/v1.0.0/mytools.invkpack.zip
```

### Package Registry (Future)

Future versions may support:
```bash
invowk pack install com.company.devtools@1.0.0
```

## Install Locations

### User Commands (Default)

```bash
invowk pack import mytools.zip
# Installed to: ~/.invowk/cmds/mytools.invkpack/
```

Available globally for the user.

### Project-Local

```bash
invowk pack import mytools.zip --path ./packs
# Installed to: ./packs/mytools.invkpack/
```

Only available in this project.

### Custom Search Path

Configure additional search paths:

```cue
// ~/.config/invowk/config.cue
search_paths: [
    "/shared/company-packs"
]
```

Install there:
```bash
invowk pack import mytools.zip --path /shared/company-packs
```

## Version Management

### Semantic Versioning

Use version in your invkfile:

```cue
group: "com.company.tools"
version: "1.2.0"
```

### Archive Naming

Include version in archive name:

```bash
invowk pack archive ./mytools.invkpack --output ./dist/mytools-1.2.0.zip
```

### Upgrade Process

```bash
# Remove old version
rm -rf ~/.invowk/cmds/mytools.invkpack

# Install new version
invowk pack import mytools-1.2.0.zip

# Or use --overwrite
invowk pack import mytools-1.2.0.zip --overwrite
```

## Team Distribution

### Shared Network Location

```bash
# Admin publishes
invowk pack archive ./devtools.invkpack --output /shared/packs/devtools.zip

# Team members import
invowk pack import /shared/packs/devtools.zip
```

### Internal Package Server

Host packs on internal HTTP server:

```bash
# Team members import via URL
invowk pack import https://internal.company.com/packs/devtools.zip
```

## Best Practices

1. **Validate before archiving**: `invowk pack validate --deep`
2. **Use semantic versioning**: Track changes clearly
3. **Include README**: Document pack usage
4. **RDNS naming**: Prevent conflicts
5. **Changelog**: Document what changed between versions

## Example Workflow

```bash
# 1. Create and develop pack
invowk pack create com.company.mytools --scripts
# ... add commands and scripts ...

# 2. Validate
invowk pack validate ./com.company.mytools.invkpack --deep

# 3. Create versioned archive
invowk pack archive ./com.company.mytools.invkpack \
    --output ./releases/mytools-1.0.0.zip

# 4. Distribute (e.g., upload to GitHub release)

# 5. Team imports
invowk pack import https://github.com/company/mytools/releases/download/v1.0.0/mytools-1.0.0.zip
```

## Next Steps

- [Overview](./overview) - Pack concepts
- [Creating Packs](./creating-packs) - Build your pack
- [Validating](./validating) - Ensure pack integrity
