---
sidebar_position: 1
---

# Installation

Welcome to Invowk! Let's get you set up and running commands in no time.

## Requirements

Before installing Invowk, make sure you have:

- **Go 1.25+** (only if building from source)
- **Linux, macOS, or Windows** - Invowk works on all three!

For container runtime features, you'll also need:
- **Docker** or **Podman** installed and running

## Installation Methods

### From Source (Recommended for now)

If you have Go installed, building from source is straightforward:

```bash
git clone https://github.com/invowk/invowk
cd invowk
go build -o invowk .
```

Then move the binary to your PATH:

```bash
# Linux/macOS
sudo mv invowk /usr/local/bin/

# Or add to your local bin
mv invowk ~/.local/bin/
```

### Using Make (with more options)

The project includes a Makefile with several build options:

```bash
# Standard build (stripped binary, smaller size)
make build

# Development build (with debug symbols)
make build-dev

# Compressed build (requires UPX)
make build-upx

# Install to $GOPATH/bin
make install
```

### Verify Installation

Once installed, verify everything works:

```bash
invowk --version
```

You should see the version information. If you get a "command not found" error, make sure the binary is in your PATH.

## Shell Completion

Invowk supports tab completion for bash, zsh, fish, and PowerShell. This makes typing commands much faster!

### Bash

```bash
# Add to ~/.bashrc
eval "$(invowk completion bash)"

# Or install system-wide
invowk completion bash > /etc/bash_completion.d/invowk
```

### Zsh

```bash
# Add to ~/.zshrc
eval "$(invowk completion zsh)"

# Or install to fpath
invowk completion zsh > "${fpath[1]}/_invowk"
```

### Fish

```bash
invowk completion fish > ~/.config/fish/completions/invowk.fish
```

### PowerShell

```powershell
invowk completion powershell | Out-String | Invoke-Expression

# Or add to $PROFILE for persistence
invowk completion powershell >> $PROFILE
```

## What's Next?

Now that you have Invowk installed, head over to the [Quickstart](./quickstart) guide to run your first command!
