# VHS Demo Recordings

This directory contains VHS tape files for generating demo GIFs and recordings. These are used for documentation, website, and promotional materials.

**Note:** VHS is no longer used for CI testing. Integration tests are handled by testscript in `tests/cli/`.

## Directory Structure

```
vhs/
├── demos/              # VHS tape files (demo recordings)
│   ├── 01-simple.tape
│   ├── 02-virtual.tape
│   └── ...
├── golden/             # Historical golden files (deprecated)
├── output/             # Generated output (gitignored)
└── README.md           # This file
```

## Requirements

- [VHS](https://github.com/charmbracelet/vhs) v0.10.0 or later
- Built `invowk` binary in `bin/` directory
- `ffmpeg` and `ttyd` for full functionality

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

VHS requires `ffmpeg` and `ttyd`:

```bash
# Ubuntu/Debian
sudo apt-get install -y ffmpeg
TTYD_VERSION="1.7.7"
wget -q "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.x86_64" -O ttyd
chmod +x ttyd && sudo mv ttyd /usr/local/bin/

# macOS
brew install ffmpeg ttyd

# Fedora
sudo dnf install -y ffmpeg ttyd
```

## Generating Demos

Generate a specific demo GIF:

```bash
# From project root
make build
cd vhs/demos
vhs 01-simple.tape
```

Generate all demos:

```bash
cd vhs/demos
for tape in *.tape; do vhs "$tape"; done
```

## Demo Categories

| File | Description |
|------|-------------|
| `01-simple.tape` | Basic hello world |
| `02-virtual.tape` | Virtual shell runtime |
| `03-deps-tools.tape` | Tool dependency checks |
| `04-deps-files.tape` | File dependency checks |
| `05-deps-caps.tape` | Capability checks |
| `06-deps-custom.tape` | Custom validation |
| `07-deps-env.tape` | Environment dependencies |
| `08-flags.tape` | Command flags |
| `09-args.tape` | Positional arguments |
| `10-env.tape` | Environment configuration |
| `11-isolation.tape` | Variable isolation |

## Creating New Demos

1. Create a new tape in `demos/`:

```tape
# NN-name.tape - Description
Output demos/NN-name.gif

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set TypingSpeed 50ms

Type "./bin/invowk cmd 'command'"
Enter
Sleep 500ms
```

2. Generate the GIF:

```bash
cd vhs/demos && vhs NN-name.tape
```

## Integration Testing

For CI-grade integration testing, see `tests/cli/` which uses [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) for deterministic output verification.

```bash
# Run integration tests
go test -v ./tests/cli/...
```
