---
sidebar_position: 3
---

# Validating Packs

Validate your packs to ensure they're correctly structured and ready for distribution.

## Basic Validation

```bash
invowk pack validate ./mytools.invkpack
```

Output for a valid pack:
```
Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
```

## Deep Validation

Add `--deep` to also parse and validate the invkfile:

```bash
invowk pack validate ./mytools.invkpack --deep
```

Output:
```
Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
✓ Invkfile parses successfully
```

## What Gets Validated

### Structure Checks

- Pack directory exists
- `invkfile.cue` is present at root
- No nested packs (packs can't contain other packs)

### Naming Checks

- Folder name ends with `.invkpack`
- Name prefix follows rules (starts with letter, alphanumeric + dots)
- No invalid characters (hyphens, underscores)

### Deep Checks (with `--deep`)

- Invkfile parses without errors
- CUE syntax is valid
- Schema constraints are met
- Script paths are valid (relative, within pack)

## Validation Errors

### Missing Invkfile

```
Pack Validation
• Path: /home/user/bad.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] missing required invkfile.cue
```

### Invalid Name

```
Pack Validation
• Path: /home/user/my-tools.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [naming] pack name 'my-tools' contains invalid characters (hyphens not allowed)
```

### Nested Pack

```
Pack Validation
• Path: /home/user/parent.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] nested.invkpack: nested packs are not allowed
```

### Invalid Invkfile (deep)

```
Pack Validation
• Path: /home/user/broken.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [invkfile] parse error at line 15: expected '}', found EOF
```

## Batch Validation

Validate multiple packs:

```bash
# Validate all packs in a directory
for pack in ./packs/*.invkpack; do
    invowk pack validate "$pack" --deep
done
```

## CI Integration

Add pack validation to your CI pipeline:

```yaml
# GitHub Actions example
- name: Validate packs
  run: |
    for pack in packs/*.invkpack; do
      invowk pack validate "$pack" --deep
    done
```

## Common Issues

### Wrong Path Separators

```cue
// Bad - Windows-style
script: "scripts\\build.sh"

// Good - Forward slashes
script: "scripts/build.sh"
```

### Escaping Pack Directory

```cue
// Bad - tries to access parent
script: "../outside/script.sh"

// Good - stays within pack
script: "scripts/script.sh"
```

### Absolute Paths

```cue
// Bad - absolute path
script: "/usr/local/bin/script.sh"

// Good - relative path
script: "scripts/script.sh"
```

## Best Practices

1. **Validate before committing**: Catch issues early
2. **Use `--deep`**: Catches invkfile errors
3. **Validate in CI**: Prevent broken packs from shipping
4. **Fix issues immediately**: Don't let validation debt accumulate

## Next Steps

- [Creating Packs](./creating-packs) - Structure your pack
- [Distributing](./distributing) - Share your pack
