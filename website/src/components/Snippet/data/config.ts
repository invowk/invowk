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
    code: `container_engine: "podman"`,
  },

  'config/includes': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
    {path: "/opt/company/tools.invowkmod"},
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
invowk config set default_runtime virtual-sh

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

// Virtual runtime family configuration
virtual: {
    utilities: {
        enabled: true
    }
}

// UI preferences
ui: {
    color_scheme: "auto"  // "auto", "dark", or "light"
    verbose: false
    interactive: false    // Enable alternate screen buffer mode
}

// Example LLM backend override for invowk agent and audit --llm.
// Default config uses llm: {} until a provider or API is configured.
llm: {
    provider: "codex"
    model: "gpt-5.1-codex"
    timeout: "2m"
    concurrency: 2
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: ""
        includes: []
        inherit_includes: true
        cache_dir: ""
    }
}`,
  },

  'config/schema': {
    language: 'cue',
    code: `import "strings"

#ContainerEngineType: "podman" | "docker"
#ConfigRuntimeType: "native" | "virtual-sh" | "virtual-lua" | "container"
#ColorSchemeType: "auto" | "dark" | "light"
#LLMProviderType: "auto" | "claude" | "codex" | "gemini" | "ollama"
#LLMTimeoutDurationString: string & =~"^([0-9]+(\\\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(64)

#Config: close({
    container_engine: *"podman" | #ContainerEngineType
    includes: *([]) | [...#IncludeEntry]
    default_runtime: *"native" | #ConfigRuntimeType
    virtual: *#VirtualConfig | #VirtualConfig
    ui: *#UIConfig | #UIConfig
    container: *#ContainerConfig | #ContainerConfig
    llm: *#LLMDefaultsConfig | #LLMConfig
})

#IncludeEntry: close({
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
})

#VirtualConfig: close({
    utilities: *#VirtualUtilitiesConfig | #VirtualUtilitiesConfig
})

#VirtualUtilitiesConfig: close({
    enabled: *true | bool
})

#UIConfig: close({
    color_scheme: *"auto" | #ColorSchemeType
    verbose: *false | bool
    interactive: *false | bool
})

#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
    model?: string & !="" & strings.MaxRunes(256)
    timeout?: #LLMTimeoutDurationString
    concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
    #LLMCommonConfig
})

#LLMProviderConfig: close({
    #LLMCommonConfig
    provider: #LLMProviderType
})

#LLMAPIBackendConfig: close({
    #LLMCommonConfig
    api: #LLMAPIConfig
})

#LLMAPIConfig: close({
    base_url?: string & !="" & strings.MaxRunes(2048)
    model?: string & !="" & strings.MaxRunes(256)
    api_key_env?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)
})

#ContainerConfig: close({
    auto_provision: *#AutoProvisionConfig | #AutoProvisionConfig
})

#AutoProvisionConfig: close({
    enabled: *true | bool
    strict: *false | bool
    binary_path: *"" | (string & !="")
    includes: *([]) | [...#IncludeEntry]
    inherit_includes: *true | bool
    cache_dir: *"" | (string & !="")
})`,
  },

  'config/default-runtime': {
    language: 'cue',
    code: `default_runtime: "native"`,
  },

  'config/virtual-shell': {
    language: 'cue',
    code: `virtual: {
    utilities: {
        enabled: true
    }
}`,
  },

  'config/ui': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
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

  'config/llm-provider': {
    language: 'cue',
    code: `llm: {
    provider: "codex"
    model: "gpt-5.1-codex" // optional for CLI harnesses
    timeout: "2m"
    concurrency: 2
}`,
  },

  'config/llm-api': {
    language: 'cue',
    code: `llm: {
    api: {
        base_url: "https://api.openai.com/v1"
        model: "gpt-5.1"
        api_key_env: "OPENAI_API_KEY"
    }
}`,
  },

  'config/container-auto-provision': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
}`,
  },

  'config/minimal-example': {
    language: 'cue',
    code: `// Just override what you need
container_engine: "docker"`,
  },

  'config/empty-example': {
    language: 'cue',
    code: `// Empty config - use all defaults`,
  },

  'config/cue-validate': {
    language: 'bash',
    code: `cue vet ~/.config/invowk/config.cue`,
  },

  'config/invowk-validate': {
    language: 'bash',
    code: `invowk config show`,
  },

  'config/complete-example': {
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
// Options: "native", "virtual-sh", "virtual-lua", "container"
default_runtime: "native"

// Virtual Runtime Configuration
// ---------------------------
// Settings for the virtual runtime family
virtual: {
    utilities: {
        // Enable built-in utilities for virtual runtimes
        // Provides ls, cat, grep, etc. in virtual-sh and command helpers in virtual-lua
        enabled: true
    }
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

// Example LLM backend override
// ----------------------------
// Used by invowk agent cmd create and by invowk audit --llm.
// Default config uses llm: {} until a provider or API is configured.
// Bare invowk audit remains deterministic and does not call LLMs.
llm: {
    provider: "codex"
    model: "gpt-5.1-codex"
    timeout: "2m"
    concurrency: 2
}

// Container provisioning
// ----------------------
container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: ""
        includes: []
        inherit_includes: true
        cache_dir: ""
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

  'reference/config/example': {
    language: 'cue',
    code: `// ~/.config/invowk/config.cue
container_engine: "podman"
includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/usr/local/share/invowk/shared.invowkmod"},
]
default_runtime: "native"`,
  },

  'reference/config/cli-custom-path': {
    language: 'bash',
    code: `# Use a custom config file with --ivk-config (-c)
invowk --ivk-config /path/to/config.cue config show

# Or use the short flag
invowk -c /path/to/config.cue cmd build`,
  },

  'reference/config/init': {
    language: 'bash',
    code: `invowk config init`,
  },

  'reference/config/show': {
    language: 'bash',
    code: `invowk config show`,
  },

  'reference/config/dump': {
    language: 'bash',
    code: `invowk config dump`,
  },

  'reference/config/path': {
    language: 'bash',
    code: `invowk config path`,
  },

  'reference/config/set-examples': {
    language: 'bash',
    code: `# Set the container engine
invowk config set container_engine podman

# Set the default runtime
invowk config set default_runtime virtual-sh

# Set the color scheme
invowk config set ui.color_scheme dark`,
  },

  'reference/config/edit-linux-macos': {
    language: 'bash',
    code: `# Linux
$EDITOR ~/.config/invowk/config.cue

# macOS
$EDITOR ~/Library/Application\ Support/invowk/config.cue

# Windows PowerShell
notepad "$env:APPDATA\invowk\config.cue"`,
  },

  'reference/config/full-example': {
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

// Virtual runtime family configuration
virtual: {
    utilities: {
        enabled: true
    }
}

// UI preferences
ui: {
    color_scheme: "auto"  // "auto", "dark", or "light"
    verbose: false
    interactive: false    // Enable alternate screen buffer mode
}

// Example LLM backend override for invowk agent and audit --llm.
// Default config uses llm: {} until a provider or API is configured.
llm: {
    provider: "codex"
    model: "gpt-5.1-codex"
    timeout: "2m"
    concurrency: 2
}

// Container provisioning
container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: ""
        includes: []
        inherit_includes: true
        cache_dir: ""
    }
}`,
  },

  'reference/config/env-override-examples': {
    language: 'bash',
    code: `# Invowk does not currently support config overrides via env vars`,
  },

  'reference/config/cli-override-examples': {
    language: 'bash',
    code: `# Enable verbose output for a run
invowk --ivk-verbose cmd build

# Run command in interactive mode (alternate screen buffer)
invowk --ivk-interactive cmd build

# Override runtime for a command
invowk cmd build --ivk-runtime container`,
  },

  'reference/config/schema-definition': {
    language: 'cue',
    code: `// Root configuration structure
import "strings"

#ContainerEngineType: "podman" | "docker"
#ConfigRuntimeType: "native" | "virtual-sh" | "virtual-lua" | "container"
#ColorSchemeType: "auto" | "dark" | "light"
#LLMProviderType: "auto" | "claude" | "codex" | "gemini" | "ollama"
#LLMTimeoutDurationString: string & =~"^([0-9]+(\\\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(64)

#Config: close({
    container_engine: *"podman" | #ContainerEngineType
    includes:         *([]) | [...#IncludeEntry]
    default_runtime:  *"native" | #ConfigRuntimeType
    virtual: *#VirtualConfig | #VirtualConfig
    ui:               *#UIConfig | #UIConfig
    container:        *#ContainerConfig | #ContainerConfig
    llm:              *#LLMDefaultsConfig | #LLMConfig
})

// Include entry for modules
#IncludeEntry: close({
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
})

// Virtual runtime family configuration
#VirtualConfig: close({
    utilities: *#VirtualUtilitiesConfig | #VirtualUtilitiesConfig
})

#VirtualUtilitiesConfig: close({
    enabled: *true | bool
})

// UI configuration
#UIConfig: close({
    color_scheme: *"auto" | #ColorSchemeType
    verbose:      *false | bool
    interactive:  *false | bool
})

// LLM configuration
#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
    model?:       string & !="" & strings.MaxRunes(256)
    timeout?:     #LLMTimeoutDurationString
    concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
    #LLMCommonConfig
})

#LLMProviderConfig: close({
    #LLMCommonConfig
    provider: #LLMProviderType
})

#LLMAPIBackendConfig: close({
    #LLMCommonConfig
    api: #LLMAPIConfig
})

// OpenAI-compatible API configuration
#LLMAPIConfig: close({
    base_url?:    string & !="" & strings.MaxRunes(2048)
    model?:       string & !="" & strings.MaxRunes(256)
    api_key_env?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)
})

// Container configuration
#ContainerConfig: close({
    auto_provision: *#AutoProvisionConfig | #AutoProvisionConfig
})

// Auto-provisioning configuration
#AutoProvisionConfig: close({
    enabled:          *true | bool
    strict:           *false | bool
    binary_path:      *"" | (string & !="")
    includes:         *([]) | [...#IncludeEntry]
    inherit_includes: *true | bool
    cache_dir:        *"" | (string & !="")
})`,
  },

  'reference/config/schema': {
    language: 'cue',
    code: `import "strings"

#ContainerEngineType: "podman" | "docker"
#ConfigRuntimeType: "native" | "virtual-sh" | "virtual-lua" | "container"
#ColorSchemeType: "auto" | "dark" | "light"
#LLMProviderType: "auto" | "claude" | "codex" | "gemini" | "ollama"
#LLMTimeoutDurationString: string & =~"^([0-9]+(\\\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(64)

#Config: close({
    container_engine: *"podman" | #ContainerEngineType
    includes: *([]) | [...#IncludeEntry]
    default_runtime: *"native" | #ConfigRuntimeType
    virtual: *#VirtualConfig | #VirtualConfig
    ui: *#UIConfig | #UIConfig
    container: *#ContainerConfig | #ContainerConfig
    llm: *#LLMDefaultsConfig | #LLMConfig
})

#IncludeEntry: close({
    path:   string  // Must be absolute and end with .invowkmod
    alias?: string  // Optional, for collision disambiguation
})

#VirtualConfig: close({
    utilities: *#VirtualUtilitiesConfig | #VirtualUtilitiesConfig
})

#VirtualUtilitiesConfig: close({
    enabled: *true | bool
})

#UIConfig: close({
    color_scheme: *"auto" | #ColorSchemeType
    verbose: *false | bool
    interactive: *false | bool
})

#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
    model?: string & !="" & strings.MaxRunes(256)
    timeout?: #LLMTimeoutDurationString
    concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
    #LLMCommonConfig
})

#LLMProviderConfig: close({
    #LLMCommonConfig
    provider: #LLMProviderType
})

#LLMAPIBackendConfig: close({
    #LLMCommonConfig
    api: #LLMAPIConfig
})

#LLMAPIConfig: close({
    base_url?: string & !="" & strings.MaxRunes(2048)
    model?: string & !="" & strings.MaxRunes(256)
    api_key_env?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)
})

#ContainerConfig: close({
    auto_provision: *#AutoProvisionConfig | #AutoProvisionConfig
})

#AutoProvisionConfig: close({
    enabled: *true | bool
    strict: *false | bool
    binary_path: *"" | (string & !="")
    includes: *([]) | [...#IncludeEntry]
    inherit_includes: *true | bool
    cache_dir: *"" | (string & !="")
})`,
  },

  'reference/config/config-structure': {
    language: 'cue',
    code: `#Config: close({
    container_engine: *"podman" | #ContainerEngineType
    includes:         *([]) | [...#IncludeEntry]
    default_runtime:  *"native" | #ConfigRuntimeType
    virtual: *#VirtualConfig | #VirtualConfig
    ui:               *#UIConfig | #UIConfig
    container:        *#ContainerConfig | #ContainerConfig
    llm:              *#LLMDefaultsConfig | #LLMConfig
})`,
  },

  'reference/config/container-engine': {
    language: 'cue',
    code: `container_engine: "podman"`,
  },

  'reference/config/includes': {
    language: 'cue',
    code: `includes: [
    {path: "/home/user/.invowk/modules/tools.invowkmod"},
    {path: "/home/user/projects/shared.invowkmod", alias: "shared"},
    {path: "/opt/company/tools.invowkmod"},
]`,
  },

  'reference/config/default-runtime': {
    language: 'cue',
    code: `default_runtime: "native"`,
  },

  'reference/config/virtual-shell': {
    language: 'cue',
    code: `virtual: {
    utilities: {
        enabled: true
    }
}`,
  },

  'reference/config/ui': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
    verbose: false
    interactive: false
}`,
  },

  'reference/config/container': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: ""
        includes: []
        inherit_includes: true
        cache_dir: ""
    }
}`,
  },

  'reference/config/virtual-shell-config-structure': {
    language: 'cue',
    code: `#VirtualConfig: close({
    utilities: *#VirtualUtilitiesConfig | #VirtualUtilitiesConfig
})

#VirtualUtilitiesConfig: close({
    enabled: *true | bool
})`,
  },

  'reference/config/enable-uroot-utils': {
    language: 'cue',
    code: `virtual: {
    utilities: {
        enabled: true
    }
}`,
  },

  'reference/config/ui-config-structure': {
    language: 'cue',
    code: `#UIConfig: close({
    color_scheme: *"auto" | #ColorSchemeType
    verbose:      *false | bool
    interactive:  *false | bool
})`,
  },

  'reference/config/llm-config-structure': {
    language: 'cue',
    code: `import "strings"

#LLMTimeoutDurationString: string & =~"^([0-9]+(\\\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$" & strings.MaxRunes(64)

#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
    model?:       string & !="" & strings.MaxRunes(256)
    timeout?:     #LLMTimeoutDurationString
    concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
    #LLMCommonConfig
})

#LLMProviderConfig: close({
    #LLMCommonConfig
    provider: #LLMProviderType
})

#LLMAPIBackendConfig: close({
    #LLMCommonConfig
    api: #LLMAPIConfig
})`,
  },

  'reference/config/llm-api-config-structure': {
    language: 'cue',
    code: `import "strings"

#LLMAPIConfig: close({
    base_url?:    string & !="" & strings.MaxRunes(2048)
    model?:       string & !="" & strings.MaxRunes(256)
    api_key_env?: string & =~"^[A-Za-z_][A-Za-z0-9_]*$" & strings.MaxRunes(256)
})`,
  },

  'reference/config/container-config-structure': {
    language: 'cue',
    code: `#ContainerConfig: close({
    auto_provision: *#AutoProvisionConfig | #AutoProvisionConfig
})`,
  },

  'reference/config/auto-provision-config-structure': {
    language: 'cue',
    code: `#AutoProvisionConfig: close({
    enabled:          *true | bool
    strict:           *false | bool
    binary_path:      *"" | (string & !="")
    includes:         *([]) | [...#IncludeEntry]
    inherit_includes: *true | bool
    cache_dir:        *"" | (string & !="")
})`,
  },

  'reference/config/container-auto-provision': {
    language: 'cue',
    code: `container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: "/usr/local/bin/invowk"
        includes: [
            {path: "/opt/company/modules/tools.invowkmod"},
        ]
        inherit_includes: true
        cache_dir: "/tmp/invowk/provision"
    }
}`,
  },

  'reference/config/ui-color-scheme': {
    language: 'cue',
    code: `ui: {
    color_scheme: "auto"
}`,
  },

  'reference/config/ui-verbose': {
    language: 'cue',
    code: `ui: {
    verbose: true
}`,
  },

  'reference/config/ui-interactive': {
    language: 'cue',
    code: `ui: {
    interactive: true
}`,
  },

  'reference/config/llm-provider': {
    language: 'cue',
    code: `llm: {
    provider: "codex"
    model: "gpt-5.1-codex" // optional for CLI harnesses
    timeout: "2m"
    concurrency: 2
}`,
  },

  'reference/config/llm-api': {
    language: 'cue',
    code: `llm: {
    api: {
        base_url: "https://api.openai.com/v1"
        model: "gpt-5.1"
        api_key_env: "OPENAI_API_KEY"
    }
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
// Options: "native", "virtual-sh", "virtual-lua", "container"
default_runtime: "native"

// Virtual Runtime Configuration
// ---------------------------
// Settings for the virtual runtime family
virtual: {
    utilities: {
        // Enable built-in utilities for virtual runtimes
        // Provides ls, cat, grep, etc. in virtual-sh and command helpers in virtual-lua
        enabled: true
    }
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

// Example LLM backend override
// ----------------------------
// Used by invowk agent cmd create and by invowk audit --llm.
// Default config uses llm: {} until a provider or API is configured.
// Bare invowk audit remains deterministic and does not call LLMs.
llm: {
    provider: "codex"
    model: "gpt-5.1-codex"
    timeout: "2m"
    concurrency: 2
}

// Container provisioning
// ----------------------
container: {
    auto_provision: {
        enabled: true
        strict: false
        binary_path: ""
        includes: []
        inherit_includes: true
        cache_dir: ""
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
