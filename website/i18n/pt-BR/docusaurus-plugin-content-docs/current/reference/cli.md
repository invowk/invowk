---
sidebar_position: 1
---

# CLI Reference

Complete reference for all Invowk command-line commands and flags.

## Global Flags

These flags are available for all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | | Path to config file (default: OS-specific location) |
| `--help` | `-h` | Show help for the command |
| `--verbose` | `-v` | Enable verbose output |
| `--version` | | Show version information |

## Commands

### invowk

The root command. Running `invowk` without arguments shows the help message.

```bash
invowk [flags]
invowk [command]
```

---

### invowk cmd

Execute commands defined in invkfiles.

```bash
invowk cmd [flags]
invowk cmd [command-name] [flags] [-- args...]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--list` | `-l` | List all available commands |
| `--runtime` | `-r` | Override the runtime (must be allowed by the command) |

**Examples:**

```bash
# List all available commands
invowk cmd --list
invowk cmd -l

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test.unit

# Run with a specific runtime
invowk cmd build --runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production
```

**Command Discovery:**

Commands are discovered from (in priority order):
1. Current directory
2. `~/.invowk/cmds/`
3. Paths configured in `search_paths`

---

### invowk init

Create a new invkfile in the current directory.

```bash
invowk init [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Overwrite existing invkfile |
| `--template` | `-t` | Template to use: `default`, `minimal`, `full` |

**Templates:**

- `default` - A balanced template with a few example commands
- `minimal` - Bare minimum invkfile structure
- `full` - Comprehensive template showing all features

**Examples:**

```bash
# Create a default invkfile
invowk init

# Create a minimal invkfile
invowk init --template minimal

# Overwrite existing invkfile
invowk init --force
```

---

### invowk config

Manage Invowk configuration.

```bash
invowk config [command]
```

**Subcommands:**

#### invowk config show

Display current configuration in a readable format.

```bash
invowk config show
```

#### invowk config dump

Output raw configuration as CUE.

```bash
invowk config dump
```

#### invowk config path

Show the configuration file path.

```bash
invowk config path
```

#### invowk config init

Create a default configuration file.

```bash
invowk config init
```

#### invowk config set

Set a configuration value.

```bash
invowk config set <key> <value>
```

**Examples:**

```bash
# Set container engine
invowk config set container_engine podman

# Set default runtime
invowk config set default_runtime virtual

# Set nested value
invowk config set ui.color_scheme dark
```

---

### invowk pack

Manage invowk packs (self-contained command folders).

```bash
invowk pack [command]
```

**Subcommands:**

#### invowk pack create

Create a new invowk pack.

```bash
invowk pack create <name> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output directory (default: current directory) |

**Examples:**

```bash
# Create a pack with RDNS naming
invowk pack create com.example.mytools
```

#### invowk pack validate

Validate an invowk pack.

```bash
invowk pack validate <path> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--deep` | `-d` | Perform deep validation (checks script files, etc.) |

**Examples:**

```bash
# Basic validation
invowk pack validate ./mypack.invkpack

# Deep validation
invowk pack validate ./mypack.invkpack --deep
```

#### invowk pack list

List all discovered packs.

```bash
invowk pack list
```

#### invowk pack archive

Create a ZIP archive from a pack.

```bash
invowk pack archive <path> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output file path |

#### invowk pack import

Import a pack from a ZIP file or URL.

```bash
invowk pack import <source> [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Output directory |

---

### invowk tui

Interactive terminal UI components for shell scripts.

```bash
invowk tui [command] [flags]
```

:::tip
TUI components work great in invkfile scripts! They provide interactive prompts, spinners, file pickers, and more.
:::

**Subcommands:**

#### invowk tui input

Prompt for single-line text input.

```bash
invowk tui input [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--title` | Title for the input prompt |
| `--placeholder` | Placeholder text |
| `--default` | Default value |
| `--password` | Hide input (for passwords) |

#### invowk tui write

Multi-line text editor.

```bash
invowk tui write [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--title` | Title for the editor |
| `--placeholder` | Placeholder text |
| `--value` | Initial value |

#### invowk tui choose

Select from a list of options.

```bash
invowk tui choose [options...] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--title` | Title for the selection |
| `--limit` | Maximum number of selections (default: 1) |

#### invowk tui confirm

Prompt for yes/no confirmation.

```bash
invowk tui confirm [prompt] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--default` | Default value (true/false) |
| `--affirmative` | Text for "yes" option |
| `--negative` | Text for "no" option |

#### invowk tui filter

Fuzzy filter a list of options.

```bash
invowk tui filter [options...] [flags]
```

Options can also be provided via stdin.

#### invowk tui file

File picker.

```bash
invowk tui file [path] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--directory` | Only show directories |
| `--file` | Only show files |
| `--hidden` | Show hidden files |

#### invowk tui table

Display and select from a table.

```bash
invowk tui table [flags]
```

Reads CSV or TSV data from stdin.

**Flags:**

| Flag | Description |
|------|-------------|
| `--separator` | Column separator (default: `,`) |
| `--header` | First row is header |

#### invowk tui spin

Show a spinner while running a command.

```bash
invowk tui spin [flags] -- command [args...]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--title` | Spinner title |
| `--spinner` | Spinner style |

#### invowk tui pager

Scroll through content.

```bash
invowk tui pager [file] [flags]
```

Reads from file or stdin.

#### invowk tui format

Format text with markdown, code, or emoji.

```bash
invowk tui format [text...] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--type` | Format type: `markdown`, `code`, `emoji` |
| `--language` | Language for code highlighting |

#### invowk tui style

Apply styles to text.

```bash
invowk tui style [text...] [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--foreground` | Text color (hex or name) |
| `--background` | Background color |
| `--bold` | Bold text |
| `--italic` | Italic text |
| `--underline` | Underlined text |

---

### invowk completion

Generate shell completion scripts.

```bash
invowk completion [shell]
```

**Shells:** `bash`, `zsh`, `fish`, `powershell`

**Examples:**

```bash
# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression
```

---

### invowk help

Show help for any command.

```bash
invowk help [command]
```

**Examples:**

```bash
invowk help
invowk help cmd
invowk help config set
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Command not found |
| 3 | Dependency validation failed |
| 4 | Configuration error |
| 5 | Runtime error |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `INVOWK_CONFIG` | Override config file path |
| `INVOWK_VERBOSE` | Enable verbose output (`1` or `true`) |
| `INVOWK_CONTAINER_ENGINE` | Override container engine |
| `NO_COLOR` | Disable colored output |
| `FORCE_COLOR` | Force colored output |
