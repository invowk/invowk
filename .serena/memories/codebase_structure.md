# Codebase Structure

```
invowk/
├── main.go                 # Entry point
├── Makefile               # Build system
├── go.mod, go.sum         # Go modules
├── invkfile.cue           # Example invkfile
├── LICENSE                # EPL-2.0
├── AGENTS.md              # Agent instructions
├── TRADEMARK.md           # Trademark notice
├── Dockerfile             # Container image definition
├── .goreleaser.yaml       # GoReleaser config
│
├── .github/               # GitHub configuration
│   ├── workflows/
│   │   ├── ci.yml         # CI workflow
│   │   ├── lint.yml       # Linting workflow
│   │   ├── release.yml    # Release workflow
│   │   ├── deploy-website.yml
│   │   └── test-website.yml
│   └── dependabot.yml     # Dependabot config
│
├── cmd/invowk/            # CLI commands (Cobra)
│   ├── root.go            # Root command
│   ├── cmd.go             # `cmd` subcommand (run cmds)
│   ├── module.go          # `module` subcommand (module management)
│   ├── config.go          # `config` subcommand
│   ├── init.go            # `init` subcommand
│   ├── completion.go      # Shell completion
│   ├── exit_error.go      # Exit error handling
│   ├── internal*.go       # Internal commands
│   ├── tui_choose.go      # TUI choice selection
│   ├── tui_confirm.go     # TUI confirmation dialogs
│   ├── tui_file.go        # TUI file operations
│   ├── tui_filter.go      # TUI filtering
│   ├── tui_format.go      # TUI formatting utilities
│   ├── tui_input.go       # TUI text input
│   ├── tui_pager.go       # TUI pager/scrolling
│   ├── tui_spin.go        # TUI spinner
│   ├── tui_style.go       # TUI styling constants
│   ├── tui_table.go       # TUI table display
│   └── tui_write.go       # TUI output writing
│
├── internal/              # Private packages
│   ├── config/            # Configuration (Viper + CUE)
│   ├── container/         # Container runtime (Docker/Podman)
│   ├── discovery/         # Module/invkfile discovery
│   ├── issue/             # Issue/error handling
│   ├── platform/          # Platform-specific code
│   ├── runtime/           # Execution runtimes interface
│   ├── sshserver/         # SSH server for container
│   ├── testutil/          # Test utilities
│   ├── tui/               # TUI components (Charm)
│   └── tuiserver/         # TUI server
│
├── pkg/                   # Public packages
│   ├── invkfile/          # Invkfile parsing (CUE schema)
│   │   └── invkfile_schema.cue
│   └── invkmod/           # Invkmod parsing (CUE schema)
│       └── invkmod_schema.cue
│
├── modules/               # Sample invkmods
│   └── io.invowk.sample.invkmod/
│
├── docs/                  # Documentation
├── website/               # Website (if present)
├── examples/              # Examples
└── .claude/               # Claude Code config
```

## Architecture Flow
```
invkfile.cue -> CUE Parser -> pkg/invkfile -> Runtime Selection -> Execution
                                                  |
                                  +---------------+---------------+
                                  |               |               |
                               Native         Virtual        Container
                            (host shell)    (mvdan/sh)    (Docker/Podman)
```
