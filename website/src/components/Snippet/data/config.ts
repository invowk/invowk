import type { Snippet } from '../snippets';

export const configSnippets = {
  // =============================================================================
  // CONFIGURATION
  // =============================================================================

  'config/example': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
container_engine: "podman"
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/usr/local/share/invowk/shared.invowkmod"},
]
default_runtime: "native"`,
  },

  'config/container-engine': {
    language: 'cue',
    code: `container_engine: "docker"  // or "podman"`,
  },

  'config/includes': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/opt/company/shared.invowkmod", alias: "company"},
]`,
  },

  'config/cli-custom-path': {
    language: 'bash',
    code: `# Use a custom config file with --ivk-config (-c)
invowk --ivk-config /path/to/config.cue config show

# Or use the short flag
invowk -c /path/to/config.cue cmd build`,
  },

  'config/init': {
    language: 'bash',
    code: `invowk config init`,
  },

  'config/show': {
    language: 'bash',
    code: `invowk config show`,
  },

  'config/dump': {
    language: 'bash',
    code: `invowk config dump`,
  },

  'config/path': {
    language: 'bash',
    code: `invowk config path`,
  },

  'config/set-examples': {
    language: 'bash',
    code: `# Set the container engine
invowk config set container_engine podman

# Set the default runtime
invowk config set default_runtime virtual

# Set the color scheme
invowk config set ui.color_scheme dark`,
  },

  'config/edit-linux-macos': {
    language: 'bash',
    code: `# Linux
$EDITOR ~/.config/invowk/config.cue

# macOS
$EDITOR ~/Library/Application\ Support/invowk/config.cue

# Windows PowerShell
notepad "$env:APPDATA\invowk\config.cue"`,
  },

  'config/full-example': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue

// Container engine: "podman" or "docker"
container_engine: "podman"

// Additional modules to include in discovery
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
]

// Default runtime for commands that don't specify one
default_runtime: "native"

// Virtual shell configuration
virtual_shell: {
    enable_uroot_utils: true
}

// UI preferences
ui: {
    color_scheme: "auto"  // "auto", "dark", or "light"
    verbose: false
    interactive: false    // Enable alternate screen buffer mode
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'config/schema': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    includes?: [...#IncludeEntry]
    default_runtime?: "native" | "virtual" | "container"
    virtual_shell?: #VirtualShellConfig
    ui?: #UIConfig
    container?: #ContainerConfig
}

#IncludeEntry: {
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
}

#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?: bool
    interactive?: bool
}

#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}

#AutoProvisionConfig: {
    enabled?: bool
    strict?: bool
    binary_path?: string
    includes?: [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?: string
}`,
  },

  'config/default-runtime': {
    language: 'cue',
    code: `default_runtime: "native"`,
  },

  'config/virtual-shell': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'config/ui': {
    language: 'cue',
    code: `ui: {
    color_scheme: "dark"
    verbose: false
    interactive: false
}`,
  },

  'config/ui-color-scheme': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
}`,
  },

  'config/ui-verbose': {
    language: 'cue',
    code: `ui: {
    verbose: true
}`,
  },

  'config/ui-interactive': {
    language: 'cue',
    code: `ui: {
    interactive: true
}`,
  },

  'config/container-auto-provision': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
}`,
  },

  'config/complete-example': {
    language: 'cue',
    code: `// Invowk Configuration File
// Located at: ~/.config/invowk/config.cue

// Use Podman as the container engine
container_engine: "podman"

// Additional modules to include in discovery
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},       // Personal modules
    {path: "/home/user/work/shared.invowkmod", alias: "team"},   // Team shared module
]

// Keep the default runtime set to native shell
default_runtime: "native"

// Virtual shell settings
virtual_shell: {
    // Enable u-root utilities for more shell commands
    enable_uroot_utils: true
}

// UI preferences
ui: {
    // Auto-detect color scheme from terminal
    color_scheme: "auto"
    
    // Don't be verbose by default
    verbose: false
    
    // Enable interactive mode for commands with stdin (e.g., password prompts)
    interactive: false
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'config/env-override-examples': {
    language: 'bash',
    code: `# Invowk does not currently support config overrides via env vars`,
  },

  'config/cli-override-examples': {
    language: 'bash',
    code: `# Enable verbose output for a run
invowk --ivk-verbose cmd build

# Run command in interactive mode (alternate screen buffer)
invowk --ivk-interactive cmd build

# Override runtime for a command
invowk cmd build --ivk-runtime container`,
  },

  // =============================================================================
  // REFERENCE - CONFIG SCHEMA
  // =============================================================================

  'reference/config/schema-definition': {
    language: 'cue',
    code: `// Root configuration structure
#Config: {
    container_engine?: "podman" | "docker"
    includes?:         [...#IncludeEntry]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
    container?:        #ContainerConfig
}

// Include entry for modules
#IncludeEntry: {
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
}

// Virtual shell configuration
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

// UI configuration
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
    interactive?:  bool
}

// Container configuration
#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}

// Auto-provisioning configuration
#AutoProvisionConfig: {
    enabled?:          bool
    strict?:           bool
    binary_path?:      string
    includes?:         [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?:        string
}`,
  },

  'reference/config/config-structure': {
    language: 'cue',
    code: `#Config: {
    container_engine?: "podman" | "docker"
    includes?:         [...#IncludeEntry]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
    container?:        #ContainerConfig
}`,
  },

  'reference/config/container-engine-example': {
    language: 'cue',
    code: `container_engine: "podman"`,
  },

  'reference/config/includes-example': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
    {path: "/opt/company/tools.invowkmod"},
]`,
  },

  'reference/config/default-runtime-example': {
    language: 'cue',
    code: `default_runtime: "native"`,
  },

  'reference/config/virtual-shell-example': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'reference/config/ui-example': {
    language: 'cue',
    code: `ui: {
    color_scheme: "dark"
    verbose: false
    interactive: false
}`,
  },

  'reference/config/container-example': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'reference/config/virtual-shell-config-structure': {
    language: 'cue',
    code: `#VirtualShellConfig: {
    enable_uroot_utils?: bool
}`,
  },

  'reference/config/enable-uroot-utils-example': {
    language: 'cue',
    code: `virtual_shell: {
    enable_uroot_utils: true
}`,
  },

  'reference/config/ui-config-structure': {
    language: 'cue',
    code: `#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
    interactive?:  bool
}`,
  },

  'reference/config/container-config-structure': {
    language: 'cue',
    code: `#ContainerConfig: {
    auto_provision?: #AutoProvisionConfig
}`,
  },

  'reference/config/auto-provision-config-structure': {
    language: 'cue',
    code: `#AutoProvisionConfig: {
    enabled?:          bool
    strict?:           bool
    binary_path?:      string
    includes?:         [...#IncludeEntry]
    inherit_includes?: bool
    cache_dir?:        string
}`,
  },

  'reference/config/auto-provision-example': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
}`,
  },

  'reference/config/color-scheme-example': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
}`,
  },

  'reference/config/verbose-example': {
    language: 'cue',
    code: `ui: {
    verbose: true
}`,
  },

  'reference/config/interactive-example': {
    language: 'cue',
    code: `ui: {
    interactive: true
}`,
  },

  'reference/config/complete-example': {
    language: 'cue',
    code: `// Invowk Configuration File
// =========================
// Location: ~/.config/invowk/config.cue

// Container Engine
// ----------------
// Which container runtime to use: "podman" or "docker"
container_engine: "podman"

// Includes
// --------
// Additional modules to include in discovery.
// Each entry specifies a path to an *.invowkmod directory.
// Modules may have an optional alias for collision disambiguation.
includes: [
    // Personal modules
    {path: "/home/user/.invowk/modules/tools.invowkmod"},

    // Team shared module (with alias)
    {path: "/home/user/work/shared.invowkmod", alias: "team"},

    // Organization-wide module
    {path: "/opt/company/tools.invowkmod"},
]

// Default Runtime
// ---------------
// The runtime to use when a command doesn't specify one
// Options: "native", "virtual", "container"
default_runtime: "native"

// Virtual Shell Configuration
// ---------------------------
// Settings for the virtual shell runtime (mvdan/sh)
virtual_shell: {
    // Enable u-root utilities for more shell commands
    // Provides ls, cat, grep, etc. in the virtual environment
    enable_uroot_utils: true
}

// UI Configuration
// ----------------
// User interface settings
ui: {
    // Color scheme: "auto", "dark", or "light"
    // "auto" detects from terminal settings
    color_scheme: "auto"
    
    // Enable verbose output by default
    // Same as always passing --ivk-verbose
    verbose: false

    // Enable interactive mode by default
    interactive: false
}

// Container provisioning
// ----------------------
container: {
    auto_provision: {
        enabled: true
    }
}`,
  },

  'reference/config/minimal-example': {
    language: 'cue',
    code: `// Just override what you need
container_engine: "docker"`,
  },

  'reference/config/empty-example': {
    language: 'cue',
    code: `// Empty config - use all defaults`,
  },

  'reference/config/cue-validate': {
    language: 'bash',
    code: `cue vet ~/.config/invowk/config.cue`,
  },

  'reference/config/invowk-validate': {
    language: 'bash',
    code: `invowk config show`,
  },
} satisfies Record<string, Snippet>;
