# VHS Integration Tests

This directory contains VHS-based integration tests for the Invowk CLI. [VHS](https://github.com/charmbracelet/vhs) is a terminal recording tool that we use to capture and verify CLI output.

## Directory Structure

```
vhs/
├── tapes/              # VHS tape files (test definitions)
│   ├── 01-simple.tape
│   ├── 02-virtual.tape
│   └── ...
├── golden/             # Expected normalized output (committed to repo)
│   ├── 01-simple.golden
│   └── ...
├── output/             # Generated output (gitignored)
├── scripts/
│   ├── run-tests.sh    # Test runner
│   └── update-golden.sh # Golden file updater
├── normalize.cue       # Normalization rules configuration
└── README.md           # This file
```

## Requirements

- [VHS](https://github.com/charmbracelet/vhs) v0.10.0 or later
- Built `invowk` binary in `bin/` directory

### Installing VHS

```bash
# macOS
brew install charmbracelet/tap/vhs

# Arch Linux
pacman -S vhs

# Debian/Ubuntu (via snap)
sudo snap install vhs

# From source
go install github.com/charmbracelet/vhs@latest
```

### System Dependencies

VHS requires `ffmpeg` and `ttyd` to be installed for full functionality:

**Ubuntu/Debian:**

```bash
sudo apt-get install -y ffmpeg
# ttyd from GitHub releases (apt version may be outdated)
TTYD_VERSION="1.7.7"
wget -q "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64" -O ttyd
chmod +x ttyd && sudo mv ttyd /usr/local/bin/
```

**macOS (Homebrew):**

```bash
brew install ffmpeg ttyd
```

**Fedora:**

```bash
sudo dnf install -y ffmpeg ttyd
```

**Arch Linux:**

```bash
pacman -S ffmpeg ttyd
```

## Usage

### Running Tests

Run all VHS integration tests:

```bash
make test-vhs
```

Or run tests directly:

```bash
./vhs/scripts/run-tests.sh
```

Run a specific test category:

```bash
./vhs/scripts/run-tests.sh "01-*.tape"
```

### Updating Golden Files

When command output changes intentionally, update the golden files:

```bash
make test-vhs-update
```

**Important**: Always review the diff before committing:

```bash
git diff vhs/golden/
```

### Validating Tape Syntax

Check that all tape files have valid syntax:

```bash
make test-vhs-validate
```

## Test Categories

| File | Description | Commands Tested |
|------|-------------|-----------------|
| `01-simple.tape` | Basic command execution | `hello`, `env hierarchy` |
| `02-virtual.tape` | Virtual shell runtime | `virtual hello`, `multi runtime` |
| `03-deps-tools.tape` | Tool dependencies | `deps tool single/alternatives/mixed` |
| `04-deps-files.tape` | File dependencies | `deps file single/alternatives/permissions` |
| `05-deps-caps.tape` | Capability dependencies | `deps cap single/alternatives` |
| `06-deps-custom.tape` | Custom check dependencies | `deps check exitcode/output` |
| `07-deps-env.tape` | Environment var dependencies | `deps env single/multiple` |
| `08-flags.tape` | Command flags | `flags simple/defaults/typed/short/validation` |
| `09-args.tape` | Positional arguments | `args simple/optional/typed/validated` |
| `10-env.tape` | Environment configuration | `env files basic`, `env vars override` |
| `11-isolation.tape` | Env var isolation | `isolation virtual` |

## How It Works

1. **Tape Execution**: VHS runs each `.tape` file, which simulates typing commands and captures terminal output to `.txt` files in `output/`.

2. **Normalization**: The Go-based normalizer (`invowk internal normalize`) processes the output according to rules defined in `normalize.cue`, filtering variable content and VHS artifacts to make output deterministic.

3. **Comparison**: Normalized output is compared against golden files. Any differences indicate a regression or intentional change.

## Output Normalization

The normalizer is implemented in Go (`internal/vhsnorm/`) and configured via `normalize.cue`. It handles:

### VHS Artifact Filtering

| Artifact | Action |
|----------|--------|
| Frame separators (`─────...`) | Removed |
| Empty prompts (lone `>`) | Removed |
| Consecutive duplicate lines | Collapsed |
| ANSI escape codes | Stripped |
| Empty lines | Removed |

### Variable Content Substitution

| Pattern | Replacement |
|---------|-------------|
| ISO 8601 timestamps | `[TIMESTAMP]` |
| Home directory paths | `[HOME]` |
| Temp directory paths | `[TMPDIR]` |
| Hostname values | `[HOSTNAME]` |
| Version strings | `[VERSION]` |
| USER environment variable | `[USER]` |
| PATH environment variable | `[PATH]` |

### Customizing Normalization

Edit `normalize.cue` to add new substitution rules:

```cue
substitutions: [
    // Existing rules...
    {
        name:        "my_pattern"
        pattern:     "regex-pattern"
        replacement: "[REPLACEMENT]"
    },
]
```

## Writing New Tests

1. Create a new tape file in `tapes/`:

```tape
# NN-category.tape - Description
# Tests: command1, command2

Output vhs/output/NN-category.txt

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

# Test 1: description
Type "./bin/invowk cmd 'command name'"
Enter
Sleep 500ms
```

2. Generate the golden file:

```bash
make test-vhs-update
```

3. Review and commit both files:

```bash
git add vhs/tapes/NN-category.tape vhs/golden/NN-category.golden
```

## CI Integration

VHS tests run automatically on:
- Push to `main` branch
- Pull requests targeting `main`

The GitHub Actions workflow:
1. Builds the `invowk` binary
2. Installs VHS via `charmbracelet/vhs-action@v2`
3. Validates tape syntax
4. Runs all tests
5. Uploads output artifacts on failure for debugging

## Troubleshooting

### Tests fail locally but pass in CI

Check for environment differences:
- Different shell versions
- Different locale settings
- Missing tools or capabilities

### Output contains unexpected content

1. Run the command manually to see raw output
2. Check if new variable content needs normalization
3. Update `normalize.cue` to add new substitution rules

### VHS hangs or times out

- Ensure commands complete within the `Sleep` timeouts
- Avoid commands that require interactive input
- Skip tests requiring network if running offline

## Design Decisions

1. **Native and Virtual runtimes only**: Container tests are skipped to avoid Docker/Podman dependencies in CI and reduce test complexity.

2. **Text output, not video**: We use VHS for text capture, not video recording. This keeps golden files small and diff-friendly.

3. **Deterministic output**: All variable content is normalized to ensure tests pass across different environments.

4. **Graceful degradation**: Tests that may fail due to missing capabilities (network, container runtime) include fallback handling.
