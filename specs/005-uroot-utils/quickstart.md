# Quickstart: u-root Utils Integration

**Feature Branch**: `005-uroot-utils`
**Date**: 2026-01-30

## Overview

This feature enables built-in file utilities in Invowk's virtual shell runtime, making scripts portable across systems without requiring external binaries like `cp`, `mv`, `cat`, etc.

## Enabling u-root Utils

The u-root utils are **enabled by default** as of this feature. To verify or change the setting:

### Check Current Config

```bash
invowk config show
```

Look for:
```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

### Enable/Disable via Config File

Edit `~/.config/invowk/config.cue`:

```cue
virtual_shell: {
    enable_uroot_utils: true   // or false to disable
}
```

## Supported Commands

### From u-root pkg/core (7 utilities)

| Command | Description | Common Flags |
|---------|-------------|--------------|
| `cat` | Concatenate files | `-n` (number lines) |
| `cp` | Copy files | `-r` (recursive), `-f` (force) |
| `ls` | List directory | `-l` (long), `-a` (all), `-R` (recursive) |
| `mkdir` | Create directory | `-p` (parents) |
| `mv` | Move/rename files | `-f` (force), `-n` (no clobber) |
| `rm` | Remove files | `-r` (recursive), `-f` (force) |
| `touch` | Create/update file | `-c` (no create) |

### Custom Implementations (8 utilities)

| Command | Description | Common Flags |
|---------|-------------|--------------|
| `head` | First N lines | `-n <num>` (default 10) |
| `tail` | Last N lines | `-n <num>` (default 10) |
| `wc` | Word/line/byte count | `-l` (lines), `-w` (words), `-c` (bytes) |
| `grep` | Pattern matching | `-i` (ignore case), `-v` (invert), `-n` (line numbers) |
| `sort` | Sort lines | `-r` (reverse), `-n` (numeric) |
| `uniq` | Unique lines | `-c` (count), `-d` (duplicates only) |
| `cut` | Select fields | `-d` (delimiter), `-f` (fields) |
| `tr` | Translate characters | `SET1 SET2` (character mapping) |

### Built into mvdan/sh (no implementation needed)

| Command | Description |
|---------|-------------|
| `echo` | Print text |
| `pwd` | Print working directory |

## Example Invowkfile

```cue
cmds: [
    {
        name: "process-logs"
        description: "Process log files using built-in utilities"
        implementations: [
            {
                script: """
                    # Works on any system with u-root enabled
                    cat /var/log/app.log | grep ERROR | head -n 20

                    # Copy results to output directory
                    mkdir -p /tmp/reports
                    grep ERROR /var/log/app.log > /tmp/reports/errors.txt
                    wc -l /tmp/reports/errors.txt
                    """
                runtimes: [{name: "virtual"}]
            },
        ]
    },
]
```

## Behavior Notes

### Unsupported Flags

GNU-specific flags not supported by u-root (like `--color`, `--time-style`) are **silently ignored**. The command executes with supported flags only.

```bash
# --color is ignored, ls still works
ls --color=auto -la /tmp
```

### Error Identification

All errors from u-root utilities are prefixed with `[uroot]`:

```
[uroot] cp: /source/file: no such file or directory
[uroot] rm: /protected: permission denied
```

### No Silent Fallback

If a u-root command fails, it returns an error immediately. The system does **not** fall back to host binaries. This ensures bugs in u-root implementations are caught rather than masked.

### Streaming I/O

All file operations use streaming I/O. This means:
- Constant memory usage regardless of file size
- Safe to process large files (e.g., multi-GB logs)
- No OOM conditions from file buffering

## Testing Your Setup

Create a test invowkfile:

```cue
cmds: [
    {
        name: "test-uroot"
        description: "Test u-root utilities"
        implementations: [
            {
                script: """
                    echo "Testing u-root utilities..."

                    # Create test files
                    mkdir -p /tmp/uroot-test
                    echo "line 1" > /tmp/uroot-test/file1.txt
                    echo "line 2" >> /tmp/uroot-test/file1.txt
                    echo "line 3" >> /tmp/uroot-test/file1.txt

                    # Test cat
                    echo "--- cat ---"
                    cat /tmp/uroot-test/file1.txt

                    # Test head
                    echo "--- head -n 2 ---"
                    head -n 2 /tmp/uroot-test/file1.txt

                    # Test wc
                    echo "--- wc -l ---"
                    wc -l /tmp/uroot-test/file1.txt

                    # Test cp
                    echo "--- cp ---"
                    cp /tmp/uroot-test/file1.txt /tmp/uroot-test/file2.txt
                    ls -la /tmp/uroot-test/

                    # Cleanup
                    rm -rf /tmp/uroot-test
                    echo "Done!"
                    """
                runtimes: [{name: "virtual"}]
            },
        ]
    },
]
```

Run it:

```bash
invowk cmd test-uroot
```

## Disabling for Specific Commands

If you need system utilities for a specific command (e.g., for GNU-specific features), use the native runtime:

```cue
cmds: [
    {
        name: "use-system-ls"
        description: "Uses system ls with all GNU features"
        implementations: [
            {
                script: "ls --color=always --time-style=long-iso -la"
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            },
        ]
    },
]
```

## Troubleshooting

### "command not found" for u-root commands

1. Check config: `invowk config show | grep enable_uroot_utils`
2. Ensure using virtual runtime: `runtimes: [{name: "virtual"}]`

### Unexpected behavior compared to GNU coreutils

The u-root library implements POSIX behavior, not GNU extensions. Common differences:
- `ls` doesn't support `--time-style`
- `grep` doesn't support `--color`
- `sort` doesn't support `--version-sort`

### Error messages differ from system utilities

The u-root errors are prefixed with `[uroot]`. This is intentional to help identify the error source.
