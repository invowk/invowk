import type { Snippet } from '../snippets';

export const cliSnippets = {
  // =============================================================================
  // CLI COMMANDS
  // =============================================================================

  'cli/list-commands': {
    language: 'bash',
    code: `invowk cmd`,
  },

  'cli/run-command': {
    language: 'bash',
    code: `invowk cmd build`,
  },

  'cli/run-subcommands': {
    language: 'bash',
    code: `invowk cmd test unit
invowk cmd test coverage`,
  },

  'cli/runtime-override': {
    language: 'bash',
    code: `# Use the default (native)
invowk cmd test unit

# Explicitly use virtual runtime
invowk cmd test unit --ivk-runtime virtual`,
  },

  'cli/cue-validate': {
    language: 'bash',
    code: `cue vet invowkfile.cue path/to/invowkfile_schema.cue -d '#Invowkfile'`,
  },

  // =============================================================================
  // CLI OUTPUT EXAMPLES
  // =============================================================================

  'cli/output-list-commands': {
    language: 'text',
    code: `Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*] (linux, macos, windows)
  test unit - Run unit tests [native*, virtual] (linux, macos, windows)
  test coverage - Run tests with coverage [native*] (linux, macos, windows)
  clean - Remove build artifacts [native*] (linux, macos)`,
  },

  'cli/output-deps-not-satisfied': {
    language: 'text',
    code: `✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.`,
  },

  // =============================================================================
  // CLI REFERENCE - ADDITIONAL
  // =============================================================================

  'cli/cmd-examples': {
    language: 'bash',
    code: `# List all available commands
invowk cmd

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test unit

# Run with a specific runtime
invowk cmd build --ivk-runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'cli/init-examples': {
    language: 'bash',
    code: `# Create a default invowkfile
invowk init

# Create a minimal invowkfile
invowk init --template minimal

# Overwrite existing invowkfile
invowk init --force`,
  },

  'cli/config-examples': {
    language: 'bash',
    code: `# Set container engine
invowk config set container_engine podman

# Set default runtime
invowk config set default_runtime virtual

# Set nested value
invowk config set ui.color_scheme dark`,
  },

  'cli/module-examples': {
    language: 'bash',
    code: `# Create a module with RDNS naming
invowk module create com.example.mytools

# Basic validation
invowk module validate ./mymod.invowkmod

# Deep validation
invowk module validate ./mymod.invowkmod --deep`,
  },

  'cli/completion-all': {
    language: 'bash',
    code: `# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression`,
  },

  // =============================================================================
  // REFERENCE - CLI
  // =============================================================================

  'reference/cli/invowk-syntax': {
    language: 'bash',
    code: `invowk [flags]
invowk [command]`,
  },

  'reference/cli/cmd-syntax': {
    language: 'bash',
    code: `invowk cmd [flags]
invowk cmd [command-name] [flags] [-- args...]`,
  },

  'reference/cli/cmd-examples': {
    language: 'bash',
    code: `# List all available commands
invowk cmd

# Run a command
invowk cmd build

# Run a nested command
invowk cmd test unit

# Run with a specific runtime
invowk cmd build --ivk-runtime container

# Run with arguments
invowk cmd greet -- "World"

# Run with flags
invowk cmd deploy --env production`,
  },

  'reference/cli/init-syntax': {
    language: 'bash',
    code: `invowk init [flags] [filename]`,
  },

  'reference/cli/init-examples': {
    language: 'bash',
    code: `# Create a default invowkfile
invowk init

# Create a minimal invowkfile
invowk init --template minimal

# Overwrite existing invowkfile
invowk init --force`,
  },

  'reference/cli/config-syntax': {
    language: 'bash',
    code: `invowk config [command]`,
  },

  'reference/cli/config-show-syntax': {
    language: 'bash',
    code: `invowk config show`,
  },

  'reference/cli/config-dump-syntax': {
    language: 'bash',
    code: `invowk config dump`,
  },

  'reference/cli/config-path-syntax': {
    language: 'bash',
    code: `invowk config path`,
  },

  'reference/cli/config-init-syntax': {
    language: 'bash',
    code: `invowk config init`,
  },

  'reference/cli/config-set-syntax': {
    language: 'bash',
    code: `invowk config set <key> <value>`,
  },

  'reference/cli/config-set-examples': {
    language: 'bash',
    code: `# Set container engine
invowk config set container_engine podman

# Set default runtime
invowk config set default_runtime virtual

# Set UI options
invowk config set ui.color_scheme dark
invowk config set ui.verbose true
invowk config set ui.interactive false

# Set virtual shell options
invowk config set virtual_shell.enable_uroot_utils true`,
  },

  'reference/cli/module-syntax': {
    language: 'bash',
    code: `invowk module [command]`,
  },

  'reference/cli/module-create-syntax': {
    language: 'bash',
    code: `invowk module create <name> [flags]`,
  },

  'reference/cli/module-create-examples': {
    language: 'bash',
    code: `# Create a module with RDNS naming
invowk module create com.example.mytools

# Override module ID and description
invowk module create mytools --module-id com.example.tools --description "Shared tools"

# Create with scripts directory
invowk module create mytools --scripts`,
  },

  'reference/cli/module-validate-syntax': {
    language: 'bash',
    code: `invowk module validate <path> [flags]`,
  },

  'reference/cli/module-validate-examples': {
    language: 'bash',
    code: `# Basic validation
invowk module validate ./mymod.invowkmod

# Deep validation
invowk module validate ./mymod.invowkmod --deep`,
  },

  'reference/cli/module-list-syntax': {
    language: 'bash',
    code: `invowk module list`,
  },

  'reference/cli/module-archive-syntax': {
    language: 'bash',
    code: `invowk module archive <path> [flags]`,
  },

  'reference/cli/module-import-syntax': {
    language: 'bash',
    code: `invowk module import <source> [flags]`,
  },

  'reference/cli/module-add-syntax': {
    language: 'bash',
    code: `invowk module add <git-url> <version> [flags]`,
  },

  'reference/cli/module-add-examples': {
    language: 'bash',
    code: `invowk module add https://github.com/user/utils.invowkmod.git ^1.0.0
invowk module add git@github.com:user/tools.invowkmod.git ~2.1.0 --alias mytools
invowk module add https://github.com/user/monorepo.git ^1.0.0 --path packages/cli`,
  },

  'reference/cli/module-remove-syntax': {
    language: 'bash',
    code: `invowk module remove <identifier>`,
  },

  'reference/cli/module-sync-syntax': {
    language: 'bash',
    code: `invowk module sync`,
  },

  'reference/cli/module-update-syntax': {
    language: 'bash',
    code: `invowk module update [identifier]`,
  },

  'reference/cli/module-deps-syntax': {
    language: 'bash',
    code: `invowk module deps`,
  },

  'reference/cli/module-vendor-syntax': {
    language: 'bash',
    code: `invowk module vendor [module-path]`,
  },

  'reference/cli/tui-syntax': {
    language: 'bash',
    code: `invowk tui [command] [flags]`,
  },

  'reference/cli/tui-input-syntax': {
    language: 'bash',
    code: `invowk tui input [flags]`,
  },

  'reference/cli/tui-write-syntax': {
    language: 'bash',
    code: `invowk tui write [flags]`,
  },

  'reference/cli/tui-choose-syntax': {
    language: 'bash',
    code: `invowk tui choose [options...] [flags]`,
  },

  'reference/cli/tui-confirm-syntax': {
    language: 'bash',
    code: `invowk tui confirm [prompt] [flags]`,
  },

  'reference/cli/tui-filter-syntax': {
    language: 'bash',
    code: `invowk tui filter [options...] [flags]`,
  },

  'reference/cli/tui-file-syntax': {
    language: 'bash',
    code: `invowk tui file [path] [flags]`,
  },

  'reference/cli/tui-table-syntax': {
    language: 'bash',
    code: `invowk tui table [flags]`,
  },

  'reference/cli/tui-spin-syntax': {
    language: 'bash',
    code: `invowk tui spin [flags] -- command [args...]`,
  },

  'reference/cli/tui-pager-syntax': {
    language: 'bash',
    code: `invowk tui pager [file] [flags]`,
  },

  'reference/cli/tui-format-syntax': {
    language: 'bash',
    code: `invowk tui format [text...] [flags]`,
  },

  'reference/cli/tui-style-syntax': {
    language: 'bash',
    code: `invowk tui style [text...] [flags]`,
  },

  'reference/cli/completion-syntax': {
    language: 'bash',
    code: `invowk completion [shell]`,
  },

  'reference/cli/completion-examples': {
    language: 'bash',
    code: `# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression`,
  },

  'reference/cli/help-syntax': {
    language: 'bash',
    code: `invowk help [command]`,
  },

  'reference/cli/help-examples': {
    language: 'bash',
    code: `invowk help
invowk help cmd
invowk help config set`,
  },

  'reference/cli/error-no-invowkfile': {
    language: 'text',
    code: `# No invowkfile found!

We searched for an invowkfile but couldn't find one in the expected locations.

## Search locations (in order of precedence):
1. Current directory (invowkfile and sibling modules)
2. Configured includes (module paths from config)
3. ~/.invowk/cmds/ (modules only)

## Things you can try:
• Create an invowkfile in your current directory:
  $ invowk init

• Or specify a different directory:
  $ cd /path/to/your/project`,
  },

  'reference/cli/error-parse-failed': {
    language: 'text',
    code: `✗ Failed to parse /path/to/invowkfile.cue: invowkfile validation failed:
  #Invowkfile.cmds.0.implementations.0.runtimes.0.name: 3 errors in empty disjunction

# Failed to parse invowkfile!

Your invowkfile contains syntax errors or invalid configuration.

## Common issues:
- Invalid CUE syntax (missing quotes, braces, etc.)
- Unknown field names
- Invalid values for known fields
- Missing required fields (name, script for commands)

## Things you can try:
- Check the error message above for the specific line/column
- Validate your CUE syntax using the cue command-line tool
- Run with verbose mode for more details:
  $ invowk --ivk-verbose cmd`,
  },
} satisfies Record<string, Snippet>;
