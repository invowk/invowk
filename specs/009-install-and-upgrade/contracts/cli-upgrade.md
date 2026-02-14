> **Status Update (2026-02-13):** This contract describes the `invowk upgrade` command which
> has been removed from this branch. The `internal/selfupdate/` package and `cmd/invowk/upgrade.go`
> were deleted. This document is retained as design history. See the branch commit log for details.

# CLI Contract: `invowk upgrade` — REMOVED

**Phase 1 Output** | **Date**: 2026-02-13

## Command Signature

```
invowk upgrade [version] [flags]
```

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `version` | No | Target version (e.g., `v1.2.0`). Defaults to latest stable release. |

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--check` | — | bool | `false` | Check for available upgrade without installing |
| `--yes` | `-y` | bool | `false` | Skip confirmation prompt |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (upgraded, already up-to-date, check completed, or managed installation detected with guidance) |
| `1` | User error (invalid version, permission denied) |
| `2` | Internal error (network failure, checksum mismatch, filesystem error) |

## Output Scenarios

### Upgrade Available (interactive)

```
Current version: v1.0.0
Latest version:  v1.1.0

Upgrade to v1.1.0? [y/N] y

Downloading invowk v1.1.0...
Verifying checksum... OK
Replacing binary...  OK

Successfully upgraded to v1.1.0
```

### Already Up-to-Date

```
Current version: v1.1.0
Latest version:  v1.1.0

Already up to date.
```

### Pre-Release Ahead of Stable

```
Current version: v1.2.0-alpha.1 (pre-release)
Latest stable:   v1.1.0

You are running a pre-release version ahead of the latest stable release.
No upgrade performed.
```

### Check Mode (`--check`)

```
Current version: v1.0.0
Latest version:  v1.1.0

An upgrade is available: v1.0.0 → v1.1.0
Run 'invowk upgrade' to install.
```

### Check Mode — Up-to-Date (`--check`)

```
Current version: v1.1.0
Latest version:  v1.1.0

Already up to date.
```

### Homebrew Detected

```
Detected Homebrew installation at /opt/homebrew/Cellar/invowk/1.0.0/bin/invowk

To upgrade, run:
  brew upgrade invowk
```

### Go Install Detected

```
Detected go install at /home/user/go/bin/invowk

To upgrade, run:
  go install github.com/invowk/invowk@latest
```

### Permission Denied

```
Error: insufficient permissions to replace /usr/local/bin/invowk

Try running with elevated privileges:
  sudo invowk upgrade
```

### Network Error

```
Error: failed to check for updates: Get "https://api.github.com/repos/invowk/invowk/releases": dial tcp: lookup api.github.com: no such host

Check your network connection and try again.
If behind a firewall, set GITHUB_TOKEN for authenticated access.
```

### Checksum Mismatch

```
Error: checksum verification failed for invowk_1.1.0_linux_amd64.tar.gz

Expected: a1b2c3d4...
Got:      e5f6g7h8...

The download may be corrupted. Please try again.
If this persists, report at https://github.com/invowk/invowk/issues
```

### Rate Limited

```
Error: GitHub API rate limit exceeded (0 remaining, resets at 14:30 UTC)

To increase your rate limit, set a GitHub token:
  export GITHUB_TOKEN=ghp_...
Then retry: invowk upgrade
```

### Specific Version

```
invowk upgrade v1.0.5

Current version: v1.0.0
Target version:  v1.0.5

Upgrade to v1.0.5? [y/N] y

Downloading invowk v1.0.5...
Verifying checksum... OK
Replacing binary...  OK

Successfully upgraded to v1.0.5
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub personal access token for authenticated API requests (higher rate limits) |

## Cobra Registration

```go
// In cmd/invowk/root.go, added alongside existing commands:
rootCmd.AddCommand(newUpgradeCommand())
```

## Help Text

```
Update invowk to the latest stable release or a specific version.

The upgrade command downloads the new binary from GitHub Releases,
verifies its SHA256 checksum, and atomically replaces the current binary.

If invowk was installed via Homebrew or go install, the command suggests
using the appropriate package manager instead.

Usage:
  invowk upgrade [version] [flags]

Examples:
  # Upgrade to latest stable
  invowk upgrade

  # Check for updates without installing
  invowk upgrade --check

  # Upgrade to a specific version
  invowk upgrade v1.2.0

  # Skip confirmation prompt
  invowk upgrade --yes

Flags:
      --check   Check for available upgrade without installing
  -y, --yes     Skip confirmation prompt
  -h, --help    help for upgrade

Global Flags:
  -c, --ivk-config string        config file (default is $HOME/.config/invowk/config.cue)
  -i, --ivk-interactive          run commands in alternate screen buffer (interactive mode)
  -v, --ivk-verbose              enable verbose output
```
