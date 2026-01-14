// SPDX-License-Identifier: EPL-2.0

// Package invkfile defines the schema and parsing for invkfile CUE files.
package invkfile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// flagNameRegex validates POSIX-compliant flag names
var flagNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// envVarNameRegex validates environment variable names
var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

//go:embed invkfile_schema.cue
var invkfileSchema string

//go:embed invkpack_schema.cue
var invkpackSchema string

// RuntimeMode defines how commands are executed (the type of runtime)
type RuntimeMode string

const (
	// RuntimeNative executes commands using the system's default shell
	RuntimeNative RuntimeMode = "native"
	// RuntimeVirtual executes commands using mvdan/sh with u-root utilities
	RuntimeVirtual RuntimeMode = "virtual"
	// RuntimeContainer executes commands inside a disposable container
	RuntimeContainer RuntimeMode = "container"
)

// EnvInheritMode defines how host environment variables are inherited
type EnvInheritMode string

const (
	// EnvInheritNone disables host environment inheritance
	EnvInheritNone EnvInheritMode = "none"
	// EnvInheritAllow inherits only allowlisted host environment variables
	EnvInheritAllow EnvInheritMode = "allow"
	// EnvInheritAll inherits all host environment variables (filtered for invowk vars)
	EnvInheritAll EnvInheritMode = "all"
)

// IsValid returns true if the mode is a supported env inherit mode.
func (m EnvInheritMode) IsValid() bool {
	switch m {
	case EnvInheritNone, EnvInheritAllow, EnvInheritAll:
		return true
	default:
		return false
	}
}

// ParseEnvInheritMode parses a string into an EnvInheritMode.
func ParseEnvInheritMode(value string) (EnvInheritMode, error) {
	if value == "" {
		return "", nil
	}
	mode := EnvInheritMode(value)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid env_inherit_mode %q (expected: none, allow, all)", value)
	}
	return mode, nil
}

// ValidateEnvVarName validates a single environment variable name.
func ValidateEnvVarName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}
	if !envVarNameRegex.MatchString(name) {
		return fmt.Errorf("invalid environment variable name %q", name)
	}
	return nil
}

// RuntimeConfig represents a runtime configuration with type-specific options
type RuntimeConfig struct {
	// Name specifies the runtime type (required)
	Name RuntimeMode `json:"name"`
	// Interpreter specifies how to execute the script (native and container only)
	// - Omit field: defaults to "auto" (detect from shebang)
	// - "auto": detect interpreter from shebang (#!) in first line of script
	// - Specific value: use as interpreter (e.g., "python3", "node")
	// When declared, interpreter must be non-empty (cannot be "" or whitespace-only)
	// Not allowed for virtual runtime (CUE schema enforces this, Go validates as fallback)
	Interpreter string `json:"interpreter,omitempty"`
	// EnvInheritMode controls host environment inheritance (optional)
	// Allowed values: "none", "allow", "all"
	EnvInheritMode EnvInheritMode `json:"env_inherit_mode,omitempty"`
	// EnvInheritAllow lists host env vars to allow when EnvInheritMode is "allow"
	EnvInheritAllow []string `json:"env_inherit_allow,omitempty"`
	// EnvInheritDeny lists host env vars to block (applies to any mode)
	EnvInheritDeny []string `json:"env_inherit_deny,omitempty"`
	// EnableHostSSH enables SSH access from container back to host (container only)
	// Only valid when Name is "container". Default: false
	EnableHostSSH bool `json:"enable_host_ssh,omitempty"`
	// Containerfile specifies the path to Containerfile/Dockerfile (container only)
	// Mutually exclusive with Image
	Containerfile string `json:"containerfile,omitempty"`
	// Image specifies a pre-built container image to use (container only)
	// Mutually exclusive with Containerfile
	Image string `json:"image,omitempty"`
	// Volumes specifies volume mounts in "host:container" format (container only)
	Volumes []string `json:"volumes,omitempty"`
	// Ports specifies port mappings in "host:container" format (container only)
	Ports []string `json:"ports,omitempty"`
}

// PlatformType represents a target platform type
type PlatformType string

const (
	// PlatformLinux represents Linux operating system
	PlatformLinux PlatformType = "linux"
	// PlatformMac represents macOS operating system
	PlatformMac PlatformType = "macos"
	// PlatformWindows represents Windows operating system
	PlatformWindows PlatformType = "windows"
)

// PlatformConfig represents a platform configuration
type PlatformConfig struct {
	// Name specifies the platform type (required)
	Name PlatformType `json:"name"`
}

// ToolDependency represents a tool/binary that must be available in PATH
type ToolDependency struct {
	// Alternatives is a list of binary names where any match satisfies the dependency
	// If any of the provided tools is found in PATH, the validation succeeds (early return).
	// This allows specifying multiple possible tools (e.g., ["podman", "docker"]).
	Alternatives []string `json:"alternatives"`
}

// CustomCheck represents a custom validation script to verify system requirements
type CustomCheck struct {
	// Name is an identifier for this check (used for error reporting)
	Name string `json:"name"`
	// CheckScript is the script to execute for validation
	CheckScript string `json:"check_script"`
	// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
	ExpectedCode *int `json:"expected_code,omitempty"`
	// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
	ExpectedOutput string `json:"expected_output,omitempty"`
}

// CustomCheckDependency represents a custom check dependency that can be either:
// - A single CustomCheck (direct check with name, check_script, etc.)
// - An alternatives list of CustomChecks (OR semantics with early return)
type CustomCheckDependency struct {
	// Direct check fields (used when this is a single check)
	// Name is an identifier for this check (used for error reporting)
	Name string `json:"name,omitempty"`
	// CheckScript is the script to execute for validation
	CheckScript string `json:"check_script,omitempty"`
	// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
	ExpectedCode *int `json:"expected_code,omitempty"`
	// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
	ExpectedOutput string `json:"expected_output,omitempty"`

	// Alternatives is a list of custom checks where any passing check satisfies the dependency
	// If any of the provided checks passes, the validation succeeds (early return).
	// When Alternatives is set, the direct check fields above are ignored.
	Alternatives []CustomCheck `json:"alternatives,omitempty"`
}

// IsAlternatives returns true if this dependency uses the alternatives format
func (c *CustomCheckDependency) IsAlternatives() bool {
	return len(c.Alternatives) > 0
}

// GetChecks returns the list of CustomCheck to validate.
// If Alternatives is set, returns those; otherwise returns a single-element list with the direct check.
func (c *CustomCheckDependency) GetChecks() []CustomCheck {
	if c.IsAlternatives() {
		return c.Alternatives
	}
	// Return as a single-element list
	return []CustomCheck{{
		Name:           c.Name,
		CheckScript:    c.CheckScript,
		ExpectedCode:   c.ExpectedCode,
		ExpectedOutput: c.ExpectedOutput,
	}}
}

// CommandDependency represents another invowk command that must be discoverable.
type CommandDependency struct {
	// Alternatives is a list of command names where any match satisfies the dependency.
	// If any of the provided commands is discoverable, the dependency is satisfied (early return).
	// This allows specifying alternative commands (e.g., ["build-debug", "build-release"]).
	Alternatives []string `json:"alternatives"`
}

// CapabilityName represents a system capability type
type CapabilityName string

const (
	// CapabilityLocalAreaNetwork checks for Local Area Network presence
	CapabilityLocalAreaNetwork CapabilityName = "local-area-network"
	// CapabilityInternet checks for working Internet connectivity
	CapabilityInternet CapabilityName = "internet"
	// CapabilityContainers checks for available container engine (Docker or Podman)
	CapabilityContainers CapabilityName = "containers"
	// CapabilityTTY checks if invowk is running in an interactive TTY
	CapabilityTTY CapabilityName = "tty"
)

// CapabilityDependency represents a system capability that must be available
type CapabilityDependency struct {
	// Alternatives is a list of capability identifiers where any match satisfies the dependency
	// If any of the provided capabilities is available, the validation succeeds (early return).
	// Available capabilities: "local-area-network", "internet", "containers", "tty"
	Alternatives []CapabilityName `json:"alternatives"`
}

// EnvVarCheck represents a single environment variable check
type EnvVarCheck struct {
	// Name is the environment variable name to check (required, non-empty)
	// The check verifies that this env var exists in the user's environment
	Name string `json:"name"`
	// Validation is a regex pattern to validate the env var value (optional)
	// If specified, the env var must exist AND its value must match this pattern
	Validation string `json:"validation,omitempty"`
}

// EnvVarDependency represents an environment variable dependency with alternatives
type EnvVarDependency struct {
	// Alternatives is a list of env var checks where any match satisfies the dependency
	// If any of the provided env vars exists (and passes validation if specified), the dependency is satisfied
	// This allows specifying multiple possible env vars (e.g., ["AWS_ACCESS_KEY_ID", "AWS_PROFILE"])
	Alternatives []EnvVarCheck `json:"alternatives"`
}

// FilepathDependency represents a file or directory that must exist
type FilepathDependency struct {
	// Alternatives is a list of file or directory paths where any match satisfies the dependency
	// If any of the provided paths exists and satisfies the permission requirements,
	// the validation succeeds (early return). This allows specifying multiple
	// possible locations for a file (e.g., different paths on different systems).
	Alternatives []string `json:"alternatives"`
	// Readable checks if the path is readable
	Readable bool `json:"readable,omitempty"`
	// Writable checks if the path is writable
	Writable bool `json:"writable,omitempty"`
	// Executable checks if the path is executable
	Executable bool `json:"executable,omitempty"`
}

// FlagType represents the data type of a flag
type FlagType string

const (
	// FlagTypeString is the default flag type for string values
	FlagTypeString FlagType = "string"
	// FlagTypeBool is for boolean flags (true/false)
	FlagTypeBool FlagType = "bool"
	// FlagTypeInt is for integer flags
	FlagTypeInt FlagType = "int"
	// FlagTypeFloat is for floating-point flags
	FlagTypeFloat FlagType = "float"
)

// Flag represents a command-line flag for a command
type Flag struct {
	// Name is the flag name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
	Name string `json:"name"`
	// Description provides help text for the flag
	Description string `json:"description"`
	// DefaultValue is the default value for the flag (optional)
	DefaultValue string `json:"default_value,omitempty"`
	// Type specifies the data type of the flag (optional, defaults to "string")
	// Supported types: "string", "bool", "int", "float"
	Type FlagType `json:"type,omitempty"`
	// Required indicates whether this flag must be provided (optional, defaults to false)
	Required bool `json:"required,omitempty"`
	// Short is a single-character alias for the flag (optional)
	Short string `json:"short,omitempty"`
	// Validation is a regex pattern to validate the flag value (optional)
	Validation string `json:"validation,omitempty"`
}

// ArgumentType represents the data type of a positional argument
type ArgumentType string

const (
	// ArgumentTypeString is the default argument type for string values
	ArgumentTypeString ArgumentType = "string"
	// ArgumentTypeInt is for integer arguments
	ArgumentTypeInt ArgumentType = "int"
	// ArgumentTypeFloat is for floating-point arguments
	ArgumentTypeFloat ArgumentType = "float"
)

// Argument represents a positional command-line argument for a command
type Argument struct {
	// Name is the argument name (POSIX-compliant: starts with a letter, alphanumeric/hyphen/underscore)
	// Used for documentation and environment variable naming (INVOWK_ARG_<NAME>)
	Name string `json:"name"`
	// Description provides help text for the argument
	Description string `json:"description"`
	// Required indicates whether this argument must be provided (optional, defaults to false)
	Required bool `json:"required,omitempty"`
	// DefaultValue is the default value if the argument is not provided (optional)
	DefaultValue string `json:"default_value,omitempty"`
	// Type specifies the data type of the argument (optional, defaults to "string")
	// Supported types: "string", "int", "float"
	Type ArgumentType `json:"type,omitempty"`
	// Validation is a regex pattern to validate the argument value (optional)
	Validation string `json:"validation,omitempty"`
	// Variadic indicates this argument accepts multiple values (optional, defaults to false)
	// Only the last argument can be variadic
	Variadic bool `json:"variadic,omitempty"`
}

// GetType returns the effective type of the argument (defaults to "string" if not specified)
func (a *Argument) GetType() ArgumentType {
	if a.Type == "" {
		return ArgumentTypeString
	}
	return a.Type
}

// ValidateArgumentValue validates an argument value at runtime against type and validation regex.
// Returns nil if the value is valid, or an error describing the issue.
func (a *Argument) ValidateArgumentValue(value string) error {
	// Validate type
	if err := validateArgumentValueType(value, a.GetType()); err != nil {
		return fmt.Errorf("argument '%s' value '%s' is invalid: %s", a.Name, value, err.Error())
	}

	// Validate against regex pattern
	if a.Validation != "" {
		validationRegex, err := regexp.Compile(a.Validation)
		if err != nil {
			// This shouldn't happen as the regex is validated at parse time
			return fmt.Errorf("argument '%s' has invalid validation pattern: %s", a.Name, err.Error())
		}
		if !validationRegex.MatchString(value) {
			return fmt.Errorf("argument '%s' value '%s' does not match required pattern '%s'", a.Name, value, a.Validation)
		}
	}

	return nil
}

// validateArgumentValueType validates that a value is compatible with the specified argument type
func validateArgumentValueType(value string, argType ArgumentType) error {
	switch argType {
	case ArgumentTypeInt:
		// Check if value is a valid integer
		for i, c := range value {
			if i == 0 && c == '-' {
				continue // Allow negative sign at start
			}
			if c < '0' || c > '9' {
				return fmt.Errorf("must be a valid integer")
			}
		}
		if value == "" || value == "-" {
			return fmt.Errorf("must be a valid integer")
		}
	case ArgumentTypeFloat:
		// Check if value is a valid floating-point number
		if value == "" {
			return fmt.Errorf("must be a valid floating-point number")
		}
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("must be a valid floating-point number")
		}
	case ArgumentTypeString:
		// Any string is valid
	default:
		// Default to string (any value is valid)
	}
	return nil
}

// GetType returns the effective type of the flag (defaults to "string" if not specified)
func (f *Flag) GetType() FlagType {
	if f.Type == "" {
		return FlagTypeString
	}
	return f.Type
}

// DependsOn defines the dependencies for a command
type DependsOn struct {
	// Tools lists binaries that must be available in PATH before running
	// Uses OR semantics: if any alternative in the list is found, the dependency is satisfied
	Tools []ToolDependency `json:"tools,omitempty"`
	// Commands lists invowk commands that must be discoverable for this command to run (invkfile field: 'cmds')
	// Uses OR semantics: if any alternative in the list is discoverable, the dependency is satisfied
	Commands []CommandDependency `json:"cmds,omitempty"`
	// Filepaths lists files or directories that must exist before running
	// Uses OR semantics: if any alternative path exists, the dependency is satisfied
	Filepaths []FilepathDependency `json:"filepaths,omitempty"`
	// Capabilities lists system capabilities that must be available before running
	// Uses OR semantics: if any alternative capability is available, the dependency is satisfied
	Capabilities []CapabilityDependency `json:"capabilities,omitempty"`
	// CustomChecks lists custom validation scripts to verify system requirements
	// Each entry can be a single check or an alternatives list (OR semantics)
	CustomChecks []CustomCheckDependency `json:"custom_checks,omitempty"`
	// EnvVars lists environment variables that must exist before running
	// Uses OR semantics: if any alternative env var exists (and passes validation), the dependency is satisfied
	// IMPORTANT: Validated against the user's environment BEFORE invowk sets command-level env vars
	EnvVars []EnvVarDependency `json:"env_vars,omitempty"`
}

// PackRequirement represents a dependency on another pack from a Git repository.
type PackRequirement struct {
	// GitURL is the Git repository URL (HTTPS or SSH format).
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	GitURL string `json:"git_url"`
	// Version is the semver constraint for version selection.
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
	Version string `json:"version"`
	// Alias overrides the default namespace for imported commands (optional).
	// If not set, the namespace is: <pack>@<resolved-version>
	Alias string `json:"alias,omitempty"`
	// Path specifies a subdirectory containing the pack (optional).
	// Used for monorepos with multiple packs.
	Path string `json:"path,omitempty"`
}

// Invkpack represents pack metadata from invkpack.cue.
// This is analogous to Go's go.mod file - it contains pack identity and dependencies.
// Command definitions remain in invkfile.cue (separate file).
type Invkpack struct {
	// Pack is a MANDATORY identifier for this pack.
	// Acts as pack identity and command namespace prefix.
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
	// IMPORTANT: The pack value MUST match the folder name prefix (before .invkpack)
	Pack string `json:"pack"`
	// Version specifies the pack schema version (optional but recommended).
	// Current version: "1.0"
	Version string `json:"version,omitempty"`
	// Description provides a summary of this pack's purpose (optional).
	Description string `json:"description,omitempty"`
	// Requires declares dependencies on other packs from Git repositories (optional).
	// Dependencies are resolved at pack level.
	// All required packs are loaded and their commands made available.
	// IMPORTANT: Commands in this pack can ONLY call:
	//   1. Commands from globally installed packs (~/.invowk/packs/)
	//   2. Commands from packs declared directly in THIS requires list
	// Commands CANNOT call transitive dependencies (dependencies of dependencies).
	Requires []PackRequirement `json:"requires,omitempty"`
	// FilePath stores the path where this invkpack.cue was loaded from (not in CUE)
	FilePath string `json:"-"`
}

// ParsedPack combines pack metadata (from invkpack.cue) with commands (from invkfile.cue).
// This represents a fully parsed pack ready for use.
type ParsedPack struct {
	// Metadata contains pack identity and dependencies from invkpack.cue
	Metadata *Invkpack
	// Commands contains command definitions from invkfile.cue (nil for library-only packs)
	Commands *Invkfile
	// PackPath is the filesystem path to the pack directory
	PackPath string
	// IsLibraryOnly is true if the pack has no invkfile.cue (provides only dependencies)
	IsLibraryOnly bool
}

// CommandScope defines what commands a pack can access.
// Commands in a pack can ONLY call:
//  1. Commands from the same pack
//  2. Commands from globally installed packs (~/.invowk/packs/)
//  3. Commands from first-level requirements (direct dependencies in invkpack.cue:requires)
//
// Commands CANNOT call transitive dependencies (dependencies of dependencies).
type CommandScope struct {
	// PackID is the pack identifier that owns this scope
	PackID string
	// GlobalPacks are commands from globally installed packs (always accessible)
	GlobalPacks map[string]bool
	// DirectDeps are pack IDs from first-level requirements (from invkpack.cue:requires)
	DirectDeps map[string]bool
}

// NewCommandScope creates a CommandScope for a parsed pack.
// globalPackIDs should contain pack IDs from ~/.invowk/packs/
// directRequirements should be the requires list from the pack's invkpack.cue
func NewCommandScope(packID string, globalPackIDs []string, directRequirements []PackRequirement) *CommandScope {
	scope := &CommandScope{
		PackID:      packID,
		GlobalPacks: make(map[string]bool),
		DirectDeps:  make(map[string]bool),
	}

	for _, id := range globalPackIDs {
		scope.GlobalPacks[id] = true
	}

	for _, req := range directRequirements {
		// The direct dependency namespace uses either alias or the resolved pack ID
		if req.Alias != "" {
			scope.DirectDeps[req.Alias] = true
		}
		// Note: The actual resolved pack ID will be added during resolution
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (bool, string) {
	// Extract pack prefix from command name (format: "pack.name cmdname" or "pack.name@version cmdname")
	targetPack := ExtractPackFromCommand(targetCmd)

	// If no pack prefix, it's a local command (always allowed)
	if targetPack == "" {
		return true, ""
	}

	// Check if target is from same pack
	if targetPack == s.PackID {
		return true, ""
	}

	// Check if target is in global packs
	if s.GlobalPacks[targetPack] {
		return true, ""
	}

	// Check if target is in direct dependencies
	if s.DirectDeps[targetPack] {
		return true, ""
	}

	return false, fmt.Sprintf(
		"command from pack '%s' cannot call '%s': pack '%s' is not accessible\n"+
			"  Commands can only call:\n"+
			"  - Commands from the same pack (%s)\n"+
			"  - Commands from globally installed packs (~/.invowk/packs/)\n"+
			"  - Commands from direct dependencies declared in invkpack.cue:requires\n"+
			"  Add '%s' to your invkpack.cue requires list to use its commands",
		s.PackID, targetCmd, targetPack, s.PackID, targetPack)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual pack ID.
func (s *CommandScope) AddDirectDep(packID string) {
	s.DirectDeps[packID] = true
}

// ExtractPackFromCommand extracts the pack prefix from a fully qualified command name.
// Returns empty string if no pack prefix found.
// Examples:
//   - "io.invowk.sample hello" -> "io.invowk.sample"
//   - "utils@1.2.3 build" -> "utils@1.2.3"
//   - "build" -> ""
func ExtractPackFromCommand(cmd string) string {
	// Command format: "pack cmdname" where pack may contain dots and @version
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) < 2 {
		// No space means it's either a local command or just a pack with no command
		return ""
	}
	return parts[0]
}

// EnvConfig holds environment configuration for a command or implementation
type EnvConfig struct {
	// Files lists dotenv files to load (optional)
	// Files are loaded in order; later files override earlier ones.
	// Paths are relative to the invkfile location (or pack root for packs).
	// Files suffixed with '?' are optional and will not cause an error if missing.
	Files []string `json:"files,omitempty"`
	// Vars contains environment variables as key-value pairs (optional)
	// These override values loaded from Files.
	Vars map[string]string `json:"vars,omitempty"`
}

// GetFiles returns the files list, or an empty slice if EnvConfig is nil
func (e *EnvConfig) GetFiles() []string {
	if e == nil {
		return nil
	}
	return e.Files
}

// GetVars returns the vars map, or nil if EnvConfig is nil
func (e *EnvConfig) GetVars() map[string]string {
	if e == nil {
		return nil
	}
	return e.Vars
}

// HostOS represents a supported operating system (deprecated, use PlatformType)
type HostOS = PlatformType

const (
	// HostLinux represents Linux operating system
	HostLinux = PlatformLinux
	// HostMac represents macOS operating system
	HostMac = PlatformMac
	// HostWindows represents Windows operating system
	HostWindows = PlatformWindows
)

// Platform represents a target platform (alias for PlatformType for clarity)
type Platform = PlatformType

// Implementation represents an implementation with platform and runtime constraints
type Implementation struct {
	// Script contains the shell commands to execute OR a path to a script file
	Script string `json:"script"`
	// Runtimes specifies which runtimes can execute this implementation (required, at least one)
	// The first element is the default runtime for this platform combination
	// Each runtime is a struct with a Name field and optional type-specific fields
	Runtimes []RuntimeConfig `json:"runtimes"`
	// Platforms specifies which operating systems this implementation is for (optional)
	// If empty/nil, the implementation applies to all platforms
	// Each platform is a struct with a Name field
	Platforms []PlatformConfig `json:"platforms,omitempty"`
	// Env contains environment configuration for this implementation (optional)
	// Implementation-level env is merged with command-level env.
	// Implementation files are loaded after command-level files.
	// Implementation vars override command-level vars.
	Env *EnvConfig `json:"env,omitempty"`
	// WorkDir specifies the working directory for this implementation (optional)
	// Overrides both root-level and command-level workdir settings.
	// Can be absolute or relative to the invkfile location.
	// Forward slashes should be used for cross-platform compatibility.
	WorkDir string `json:"workdir,omitempty"`
	// DependsOn specifies dependencies that must be satisfied before running this implementation
	// These dependencies are validated according to the runtime being used
	DependsOn *DependsOn `json:"depends_on,omitempty"`

	// resolvedScript caches the resolved script content
	resolvedScript string
	// scriptResolved indicates if the script has been resolved
	scriptResolved bool
}

// Script is an alias for Implementation for backward compatibility
type Script = Implementation

// Command represents a single command that can be executed
type Command struct {
	// Name is the command identifier (can include spaces for subcommand-like behavior, e.g., "test unit")
	Name string `json:"name"`
	// Description provides help text for the command
	Description string `json:"description,omitempty"`
	// Implementations defines the executable implementations with platform/runtime constraints (required, at least one)
	Implementations []Implementation `json:"implementations"`
	// Env contains environment configuration for this command (optional)
	// Environment from files is loaded first, then vars override.
	// Command-level env is applied before implementation-level env.
	Env *EnvConfig `json:"env,omitempty"`
	// WorkDir specifies the working directory for command execution (optional)
	// Overrides root-level workdir but can be overridden by implementation-level workdir.
	// Can be absolute or relative to the invkfile location.
	// Forward slashes should be used for cross-platform compatibility.
	WorkDir string `json:"workdir,omitempty"`
	// DependsOn specifies dependencies that must be satisfied before running
	DependsOn *DependsOn `json:"depends_on,omitempty"`
	// Flags specifies command-line flags for this command
	// Note: 'env-file' (short 'e') and 'env-var' (short 'E') are reserved system flags and cannot be used.
	Flags []Flag `json:"flags,omitempty"`
	// Args specifies positional arguments for this command
	// Arguments are passed as environment variables: INVOWK_ARG_<NAME>
	// For variadic arguments: INVOWK_ARG_<NAME>_COUNT and INVOWK_ARG_<NAME>_1, _2, etc.
	Args []Argument `json:"args,omitempty"`
}

// GetCurrentHostOS returns the current operating system as HostOS
func GetCurrentHostOS() HostOS {
	switch goruntime.GOOS {
	case "linux":
		return HostLinux
	case "darwin":
		return HostMac // Returns "macos"
	case "windows":
		return HostWindows
	default:
		// Default to linux for unknown OS
		return HostLinux
	}
}

// PlatformRuntimeKey represents a unique combination of platform and runtime
type PlatformRuntimeKey struct {
	Platform Platform
	Runtime  RuntimeMode
}

// ScriptMatch represents a matched script for execution
type ScriptMatch struct {
	Script               *Script
	Platform             Platform
	Runtime              RuntimeMode
	IsDefaultForPlatform bool
}

// GetImplForPlatformRuntime finds the script that matches the given platform and runtime
func (c *Command) GetImplForPlatformRuntime(platform Platform, runtime RuntimeMode) *Script {
	for i := range c.Implementations {
		s := &c.Implementations[i]
		if s.MatchesPlatform(platform) && s.HasRuntime(runtime) {
			return s
		}
	}
	return nil
}

// GetImplsForPlatform returns all scripts that can run on the given platform
func (c *Command) GetImplsForPlatform(platform Platform) []*Script {
	var result []*Script
	for i := range c.Implementations {
		if c.Implementations[i].MatchesPlatform(platform) {
			result = append(result, &c.Implementations[i])
		}
	}
	return result
}

// GetDefaultImplForPlatform returns the first script that matches the platform (default)
func (c *Command) GetDefaultImplForPlatform(platform Platform) *Script {
	scripts := c.GetImplsForPlatform(platform)
	if len(scripts) == 0 {
		return nil
	}
	return scripts[0]
}

// GetDefaultRuntimeForPlatform returns the default runtime for the given platform
// The default runtime is the first runtime of the first script that matches the platform
func (c *Command) GetDefaultRuntimeForPlatform(platform Platform) RuntimeMode {
	script := c.GetDefaultImplForPlatform(platform)
	if script == nil || len(script.Runtimes) == 0 {
		return RuntimeNative
	}
	return script.Runtimes[0].Name
}

// CanRunOnCurrentHost returns true if the command can run on the current host OS
func (c *Command) CanRunOnCurrentHost() bool {
	currentOS := GetCurrentHostOS()
	return len(c.GetImplsForPlatform(currentOS)) > 0
}

// GetSupportedPlatforms returns all platforms that this command supports
func (c *Command) GetSupportedPlatforms() []Platform {
	platformSet := make(map[Platform]bool)
	allPlatforms := []Platform{HostLinux, HostMac, HostWindows}

	for _, s := range c.Implementations {
		if len(s.Platforms) == 0 {
			// Script applies to all platforms
			for _, p := range allPlatforms {
				platformSet[p] = true
			}
		} else {
			for _, p := range s.Platforms {
				platformSet[p.Name] = true
			}
		}
	}

	var result []Platform
	for _, p := range allPlatforms {
		if platformSet[p] {
			result = append(result, p)
		}
	}
	return result
}

// GetPlatformsString returns a comma-separated string of supported platforms
func (c *Command) GetPlatformsString() string {
	platforms := c.GetSupportedPlatforms()
	if len(platforms) == 0 {
		return ""
	}
	strs := make([]string, len(platforms))
	for i, p := range platforms {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

// GetAllowedRuntimesForPlatform returns all allowed runtimes for a given platform
func (c *Command) GetAllowedRuntimesForPlatform(platform Platform) []RuntimeMode {
	runtimeSet := make(map[RuntimeMode]bool)
	var orderedRuntimes []RuntimeMode

	for _, s := range c.Implementations {
		if s.MatchesPlatform(platform) {
			for _, r := range s.Runtimes {
				if !runtimeSet[r.Name] {
					runtimeSet[r.Name] = true
					orderedRuntimes = append(orderedRuntimes, r.Name)
				}
			}
		}
	}
	return orderedRuntimes
}

// GetRuntimesStringForPlatform returns a formatted string of runtimes for a platform with default highlighted
func (c *Command) GetRuntimesStringForPlatform(platform Platform) string {
	runtimes := c.GetAllowedRuntimesForPlatform(platform)
	if len(runtimes) == 0 {
		return ""
	}
	defaultRuntime := c.GetDefaultRuntimeForPlatform(platform)
	strs := make([]string, len(runtimes))
	for i, r := range runtimes {
		if r == defaultRuntime {
			strs[i] = string(r) + "*"
		} else {
			strs[i] = string(r)
		}
	}
	return strings.Join(strs, ", ")
}

// IsRuntimeAllowedForPlatform checks if the given runtime is allowed for the platform
func (c *Command) IsRuntimeAllowedForPlatform(platform Platform, runtime RuntimeMode) bool {
	for _, r := range c.GetAllowedRuntimesForPlatform(platform) {
		if r == runtime {
			return true
		}
	}
	return false
}

// ValidateScripts checks that there are no duplicate platform+runtime combinations
// Returns an error with a descriptive message if duplicates are found
func (c *Command) ValidateScripts() error {
	seen := make(map[PlatformRuntimeKey]int) // key -> script index (1-based for error messages)
	allPlatforms := []PlatformConfig{
		{Name: PlatformLinux},
		{Name: PlatformMac},
		{Name: PlatformWindows},
	}

	for i, s := range c.Implementations {
		platforms := s.Platforms
		if len(platforms) == 0 {
			platforms = allPlatforms // Applies to all platforms
		}

		for _, p := range platforms {
			for _, r := range s.Runtimes {
				key := PlatformRuntimeKey{Platform: p.Name, Runtime: r.Name}
				if existingIdx, exists := seen[key]; exists {
					return fmt.Errorf(
						"command '%s' has duplicate platform+runtime combination: platform=%s, runtime=%s (scripts #%d and #%d)",
						c.Name, p.Name, r.Name, existingIdx, i+1,
					)
				}
				seen[key] = i + 1
			}
		}
	}
	return nil
}

// MatchesPlatform returns true if the script can run on the given platform
func (s *Script) MatchesPlatform(platform Platform) bool {
	if len(s.Platforms) == 0 {
		return true // No platforms specified = all platforms
	}
	for _, p := range s.Platforms {
		if p.Name == platform {
			return true
		}
	}
	return false
}

// HasRuntime returns true if the script supports the given runtime
func (s *Script) HasRuntime(runtime RuntimeMode) bool {
	for _, r := range s.Runtimes {
		if r.Name == runtime {
			return true
		}
	}
	return false
}

// GetRuntimeConfig returns the RuntimeConfig for the given runtime type, or nil if not found
func (s *Script) GetRuntimeConfig(runtime RuntimeMode) *RuntimeConfig {
	for i := range s.Runtimes {
		if s.Runtimes[i].Name == runtime {
			return &s.Runtimes[i]
		}
	}
	return nil
}

// GetDefaultRuntime returns the default runtime type for this script (first runtime in the list)
func (s *Script) GetDefaultRuntime() RuntimeMode {
	if len(s.Runtimes) == 0 {
		return RuntimeNative
	}
	return s.Runtimes[0].Name
}

// GetDefaultRuntimeConfig returns the default RuntimeConfig for this script (first in the list)
func (s *Script) GetDefaultRuntimeConfig() *RuntimeConfig {
	if len(s.Runtimes) == 0 {
		return nil
	}
	return &s.Runtimes[0]
}

// HasHostSSH returns true if any runtime in this script has enable_host_ssh enabled
func (s *Script) HasHostSSH() bool {
	for _, r := range s.Runtimes {
		if r.Name == RuntimeContainer && r.EnableHostSSH {
			return true
		}
	}
	return false
}

// GetHostSSHForRuntime returns whether enable_host_ssh is enabled for the given runtime
func (s *Script) GetHostSSHForRuntime(runtime RuntimeMode) bool {
	if runtime != RuntimeContainer {
		return false // enable_host_ssh is only valid for container runtime
	}
	rc := s.GetRuntimeConfig(runtime)
	if rc == nil {
		return false
	}
	return rc.EnableHostSSH
}

// HasDependencies returns true if the command has any dependencies (at command or script level)
func (c *Command) HasDependencies() bool {
	// Check command-level dependencies
	if c.DependsOn != nil {
		if len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0 || len(c.DependsOn.Capabilities) > 0 || len(c.DependsOn.CustomChecks) > 0 || len(c.DependsOn.EnvVars) > 0 {
			return true
		}
	}
	// Check implementation-level dependencies
	for _, s := range c.Implementations {
		if s.HasDependencies() {
			return true
		}
	}
	return false
}

// HasCommandLevelDependencies returns true if the command has command-level dependencies only
func (c *Command) HasCommandLevelDependencies() bool {
	if c.DependsOn == nil {
		return false
	}
	return len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0 || len(c.DependsOn.Capabilities) > 0 || len(c.DependsOn.CustomChecks) > 0 || len(c.DependsOn.EnvVars) > 0
}

// GetCommandDependencies returns the list of command dependency names (from command level)
// For dependencies with alternatives, returns all alternatives flattened into a single list
func (c *Command) GetCommandDependencies() []string {
	if c.DependsOn == nil {
		return nil
	}
	var names []string
	for _, dep := range c.DependsOn.Commands {
		names = append(names, dep.Alternatives...)
	}
	return names
}

// HasDependencies returns true if the script has any dependencies
func (s *Script) HasDependencies() bool {
	if s.DependsOn == nil {
		return false
	}
	return len(s.DependsOn.Tools) > 0 || len(s.DependsOn.Commands) > 0 || len(s.DependsOn.Filepaths) > 0 || len(s.DependsOn.Capabilities) > 0 || len(s.DependsOn.CustomChecks) > 0 || len(s.DependsOn.EnvVars) > 0
}

// GetCommandDependencies returns the list of command dependency names from this script
// For dependencies with alternatives, returns all alternatives flattened into a single list
func (s *Script) GetCommandDependencies() []string {
	if s.DependsOn == nil {
		return nil
	}
	var names []string
	for _, dep := range s.DependsOn.Commands {
		names = append(names, dep.Alternatives...)
	}
	return names
}

// MergeDependsOn merges command-level and implementation-level dependencies
// Implementation-level dependencies are added to command-level dependencies
// Returns a new DependsOn struct with combined dependencies
func MergeDependsOn(cmdDeps, scriptDeps *DependsOn) *DependsOn {
	return MergeDependsOnAll(nil, cmdDeps, scriptDeps)
}

// MergeDependsOnAll merges root-level, command-level, and implementation-level dependencies
// Dependencies are combined in order: root -> command -> implementation
// Returns a new DependsOn struct with combined dependencies
func MergeDependsOnAll(rootDeps, cmdDeps, implDeps *DependsOn) *DependsOn {
	if rootDeps == nil && cmdDeps == nil && implDeps == nil {
		return nil
	}

	merged := &DependsOn{
		Tools:        make([]ToolDependency, 0),
		Commands:     make([]CommandDependency, 0),
		Filepaths:    make([]FilepathDependency, 0),
		Capabilities: make([]CapabilityDependency, 0),
		CustomChecks: make([]CustomCheckDependency, 0),
		EnvVars:      make([]EnvVarDependency, 0),
	}

	// Add root-level dependencies first (lowest priority)
	if rootDeps != nil {
		merged.Tools = append(merged.Tools, rootDeps.Tools...)
		merged.Commands = append(merged.Commands, rootDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, rootDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, rootDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, rootDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, rootDeps.EnvVars...)
	}

	// Add command-level dependencies
	if cmdDeps != nil {
		merged.Tools = append(merged.Tools, cmdDeps.Tools...)
		merged.Commands = append(merged.Commands, cmdDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, cmdDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, cmdDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, cmdDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, cmdDeps.EnvVars...)
	}

	// Add implementation-level dependencies
	if implDeps != nil {
		merged.Tools = append(merged.Tools, implDeps.Tools...)
		merged.Commands = append(merged.Commands, implDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, implDeps.Filepaths...)
		merged.Capabilities = append(merged.Capabilities, implDeps.Capabilities...)
		merged.CustomChecks = append(merged.CustomChecks, implDeps.CustomChecks...)
		merged.EnvVars = append(merged.EnvVars, implDeps.EnvVars...)
	}

	// Return nil if no dependencies after merging
	if len(merged.Tools) == 0 && len(merged.Commands) == 0 && len(merged.Filepaths) == 0 && len(merged.Capabilities) == 0 && len(merged.CustomChecks) == 0 && len(merged.EnvVars) == 0 {
		return nil
	}

	return merged
}

// HasRootLevelDependencies returns true if the invkfile has root-level dependencies
func (inv *Invkfile) HasRootLevelDependencies() bool {
	if inv.DependsOn == nil {
		return false
	}
	return len(inv.DependsOn.Tools) > 0 || len(inv.DependsOn.Commands) > 0 || len(inv.DependsOn.Filepaths) > 0 || len(inv.DependsOn.Capabilities) > 0 || len(inv.DependsOn.CustomChecks) > 0 || len(inv.DependsOn.EnvVars) > 0
}

// scriptFileExtensions contains extensions that indicate a script file
var scriptFileExtensions = []string{".sh", ".bash", ".ps1", ".bat", ".cmd", ".py", ".rb", ".pl", ".zsh", ".fish"}

// IsScriptFile returns true if the Script field appears to be a file path
func (s *Script) IsScriptFile() bool {
	script := strings.TrimSpace(s.Script)
	if script == "" {
		return false
	}

	// Check for explicit path indicators
	if strings.HasPrefix(script, "./") || strings.HasPrefix(script, "../") || strings.HasPrefix(script, "/") {
		return true
	}

	// On Windows, check for drive letter paths
	if len(script) >= 2 && script[1] == ':' {
		return true
	}

	// Check for known script file extensions
	lower := strings.ToLower(script)
	for _, ext := range scriptFileExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

// GetScriptFilePath returns the absolute path to the script file, if Script is a file reference.
// Returns empty string if Script is inline content.
// The invkfilePath parameter is used to resolve relative paths.
// If packPath is provided (non-empty), script paths are resolved relative to the pack root
// and are expected to use forward slashes for cross-platform compatibility.
func (s *Script) GetScriptFilePath(invkfilePath string) string {
	return s.GetScriptFilePathWithPack(invkfilePath, "")
}

// GetScriptFilePathWithPack returns the absolute path to the script file, if Script is a file reference.
// Returns empty string if Script is inline content.
// The invkfilePath parameter is used to resolve relative paths when not in a pack.
// The packPath parameter specifies the pack root directory for pack-relative paths.
// When packPath is non-empty, script paths are expected to use forward slashes for
// cross-platform compatibility and are resolved relative to the pack root.
func (s *Script) GetScriptFilePathWithPack(invkfilePath, packPath string) string {
	if !s.IsScriptFile() {
		return ""
	}

	script := strings.TrimSpace(s.Script)

	// If absolute path, return as-is
	if filepath.IsAbs(script) {
		return script
	}

	// If in a pack, resolve relative to pack root with cross-platform path conversion
	if packPath != "" {
		// Convert forward slashes to native path separator for cross-platform compatibility
		nativePath := filepath.FromSlash(script)
		return filepath.Join(packPath, nativePath)
	}

	// Resolve relative to invkfile directory
	invowkDir := filepath.Dir(invkfilePath)
	return filepath.Join(invowkDir, script)
}

// ResolveScript returns the actual script content to execute.
// If Script is a file path, it reads the file content.
// If Script is inline content (including multi-line), it returns it directly.
// The invkfilePath parameter is used to resolve relative paths.
func (s *Script) ResolveScript(invkfilePath string) (string, error) {
	return s.ResolveScriptWithPack(invkfilePath, "")
}

// ResolveScriptWithPack returns the actual script content to execute.
// If Script is a file path, it reads the file content.
// If Script is inline content (including multi-line), it returns it directly.
// The invkfilePath parameter is used to resolve relative paths when not in a pack.
// The packPath parameter specifies the pack root directory for pack-relative paths.
func (s *Script) ResolveScriptWithPack(invkfilePath, packPath string) (string, error) {
	if s.scriptResolved {
		return s.resolvedScript, nil
	}

	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithPack(invkfilePath, packPath)
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		s.resolvedScript = string(content)
	} else {
		// Inline script - use directly (multi-line strings from CUE are already handled)
		s.resolvedScript = script
	}

	s.scriptResolved = true
	return s.resolvedScript, nil
}

// ResolveScriptWithFS resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
func (s *Script) ResolveScriptWithFS(invkfilePath string, readFile func(path string) ([]byte, error)) (string, error) {
	return s.ResolveScriptWithFSAndPack(invkfilePath, "", readFile)
}

// ResolveScriptWithFSAndPack resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
// The packPath parameter specifies the pack root directory for pack-relative paths.
func (s *Script) ResolveScriptWithFSAndPack(invkfilePath, packPath string, readFile func(path string) ([]byte, error)) (string, error) {
	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePathWithPack(invkfilePath, packPath)
		content, err := readFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		return string(content), nil
	}

	// Inline script - use directly
	return script, nil
}

// Invkfile represents command definitions from invkfile.cue.
// Pack metadata (pack name, version, description, requires) is now in Invkpack.
// This separation follows Go's pattern: invkpack.cue is like go.mod, invkfile.cue is like .go files.
type Invkfile struct {
	// DefaultShell overrides the default shell for native runtime
	DefaultShell string `json:"default_shell,omitempty"`
	// WorkDir specifies the default working directory for all commands
	// Can be absolute or relative to the invkfile location.
	// Forward slashes should be used for cross-platform compatibility.
	// Individual commands or implementations can override this with their own workdir.
	WorkDir string `json:"workdir,omitempty"`
	// Env contains global environment configuration for all commands (optional)
	// Root-level env is applied first (lowest priority from invkfile).
	// Command-level and implementation-level env override root-level env.
	Env *EnvConfig `json:"env,omitempty"`
	// DependsOn specifies global dependencies that apply to all commands (optional)
	// Root-level depends_on is combined with command-level and implementation-level depends_on.
	// Root-level dependencies are validated first (lowest priority in the merge order).
	// This is useful for defining shared prerequisites like required tools or capabilities
	// that apply to all commands in this invkfile.
	DependsOn *DependsOn `json:"depends_on,omitempty"`
	// Commands defines the available commands (invkfile field: 'cmds')
	Commands []Command `json:"cmds"`

	// FilePath stores the path where this invkfile was loaded from (not in CUE)
	FilePath string `json:"-"`
	// PackPath stores the pack directory path if this invkfile is from a pack (not in CUE)
	// Empty string if not loaded from a pack
	PackPath string `json:"-"`
	// Metadata references the pack metadata from invkpack.cue (not in CUE)
	// This is set when parsing a pack via ParsePackFull
	Metadata *Invkpack `json:"-"`
}

// InvkfileName is the standard name for invkfile
const InvkfileName = "invkfile"

// InvkpackName is the standard name for pack metadata file
const InvkpackName = "invkpack"

// ValidExtensions lists valid file extensions for invkfile
var ValidExtensions = []string{".cue", ""}

// Parse reads and parses an invkfile from the given path
func Parse(path string) (*Invkfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkfile at %s: %w", path, err)
	}

	return ParseBytes(data, path)
}

// ParsePack reads and parses an invkfile from a pack directory.
// The packPath should be the path to the pack directory (ending in .invkpack).
// It parses both invkpack.cue (metadata) and invkfile.cue (commands).
// Returns the Invkfile with Metadata populated from invkpack.cue.
// Deprecated: Use ParsePackFull for full access to ParsedPack structure.
func ParsePack(packPath string) (*Invkfile, error) {
	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	invkfilePath := filepath.Join(packPath, "invkfile.cue")

	// Parse invkpack.cue (required for packs)
	var meta *Invkpack
	if _, statErr := os.Stat(invkpackPath); statErr == nil {
		var parseErr error
		meta, parseErr = ParseInvkpack(invkpackPath)
		if parseErr != nil {
			return nil, fmt.Errorf("pack at %s: %w", packPath, parseErr)
		}
	}

	// Parse invkfile.cue
	data, err := os.ReadFile(invkfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkfile at %s: %w", invkfilePath, err)
	}

	inv, err := ParseBytes(data, invkfilePath)
	if err != nil {
		return nil, err
	}

	// Set the pack path and metadata
	inv.PackPath = packPath
	inv.Metadata = meta

	return inv, nil
}

// ParseInvkpack reads and parses pack metadata from invkpack.cue at the given path.
func ParseInvkpack(path string) (*Invkpack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkpack at %s: %w", path, err)
	}

	return ParseInvkpackBytes(data, path)
}

// ParseInvkpackBytes parses pack metadata content from bytes.
func ParseInvkpackBytes(data []byte, path string) (*Invkpack, error) {
	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(invkpackSchema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile invkpack schema: %w", schemaValue.Err())
	}

	// Compile the user's invkpack file
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return nil, fmt.Errorf("failed to parse invkpack at %s: %w", path, userValue.Err())
	}

	// Unify with schema to validate
	schema := schemaValue.LookupPath(cue.ParsePath("#Invkpack"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("invkpack validation failed at %s: %w", path, err)
	}

	// Decode into struct
	var meta Invkpack
	if err := unified.Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode invkpack at %s: %w", path, err)
	}

	meta.FilePath = path

	return &meta, nil
}

// ParsePackFull reads and parses a complete pack from the given pack directory.
// It expects:
// - invkpack.cue (required): Pack metadata (pack name, version, description, requires)
// - invkfile.cue (optional): Command definitions (for library-only packs)
//
// The packPath should be the path to the pack directory (ending in .invkpack).
func ParsePackFull(packPath string) (*ParsedPack, error) {
	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	invkfilePath := filepath.Join(packPath, "invkfile.cue")

	// Parse invkpack.cue (required)
	meta, err := ParseInvkpack(invkpackPath)
	if err != nil {
		return nil, fmt.Errorf("pack at %s: %w", packPath, err)
	}

	// Create result
	result := &ParsedPack{
		Metadata: meta,
		PackPath: packPath,
	}

	// Parse invkfile.cue (optional - may be a library-only pack)
	if _, statErr := os.Stat(invkfilePath); statErr == nil {
		data, readErr := os.ReadFile(invkfilePath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read invkfile at %s: %w", invkfilePath, readErr)
		}

		inv, parseErr := ParseBytes(data, invkfilePath)
		if parseErr != nil {
			return nil, parseErr
		}

		// Set metadata reference and pack path
		inv.Metadata = meta
		inv.PackPath = packPath
		result.Commands = inv
	} else if os.IsNotExist(statErr) {
		// Library-only pack (no commands)
		result.IsLibraryOnly = true
	} else {
		return nil, fmt.Errorf("failed to check invkfile at %s: %w", invkfilePath, statErr)
	}

	return result, nil
}

// ParseBytes parses invkfile content from bytes
func ParseBytes(data []byte, path string) (*Invkfile, error) {
	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(invkfileSchema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	// Compile the user's invkfile
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return nil, fmt.Errorf("failed to parse invkfile at %s: %w", path, userValue.Err())
	}

	// Unify with schema to validate
	schema := schemaValue.LookupPath(cue.ParsePath("#Invkfile"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("invkfile validation failed at %s: %w", path, err)
	}

	// Decode into struct
	var inv Invkfile
	if err := unified.Decode(&inv); err != nil {
		return nil, fmt.Errorf("failed to decode invkfile at %s: %w", path, err)
	}

	inv.FilePath = path

	// Validate and apply command defaults
	if err := inv.validate(); err != nil {
		return nil, err
	}

	return &inv, nil
}

// validate checks the invkfile for errors and applies defaults
func (inv *Invkfile) validate() error {
	if len(inv.Commands) == 0 {
		return fmt.Errorf("invkfile at %s has no commands defined (missing required 'cmds' list)", inv.FilePath)
	}

	// Validate root-level custom_checks dependencies
	if inv.DependsOn != nil && len(inv.DependsOn.CustomChecks) > 0 {
		if err := validateCustomChecks(inv.DependsOn.CustomChecks, "root", inv.FilePath); err != nil {
			return err
		}
	}

	// Validate each command
	for i := range inv.Commands {
		if err := inv.validateCommand(&inv.Commands[i]); err != nil {
			return err
		}
	}

	return nil
}

// validateRuntimeConfig checks that a runtime configuration is valid
func validateRuntimeConfig(rt *RuntimeConfig, cmdName string, implIndex int) error {
	// Validate interpreter field: if declared, it must be non-empty (after trimming whitespace)
	// This is a Go-level validation as fallback to the CUE schema constraint
	if rt.Interpreter != "" && strings.TrimSpace(rt.Interpreter) == "" {
		return fmt.Errorf("command '%s' implementation #%d: interpreter cannot be empty or whitespace-only when declared; omit the field to use auto-detection", cmdName, implIndex)
	}

	// Validate env inherit mode and env var names
	if rt.EnvInheritMode != "" && !rt.EnvInheritMode.IsValid() {
		return fmt.Errorf("command '%s' implementation #%d: env_inherit_mode must be one of: none, allow, all", cmdName, implIndex)
	}
	for _, name := range rt.EnvInheritAllow {
		if err := ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_allow: %w", cmdName, implIndex, err)
		}
	}
	for _, name := range rt.EnvInheritDeny {
		if err := ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("command '%s' implementation #%d: env_inherit_deny: %w", cmdName, implIndex, err)
		}
	}

	// Container-specific fields are only valid for container runtime
	if rt.Name != RuntimeContainer {
		if rt.EnableHostSSH {
			return fmt.Errorf("command '%s' implementation #%d: enable_host_ssh is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Containerfile != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile is only valid for container runtime", cmdName, implIndex)
		}
		if rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: image is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Volumes) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: volumes is only valid for container runtime", cmdName, implIndex)
		}
		if len(rt.Ports) > 0 {
			return fmt.Errorf("command '%s' implementation #%d: ports is only valid for container runtime", cmdName, implIndex)
		}
	} else {
		// For container runtime, validate mutual exclusivity of containerfile and image
		if rt.Containerfile != "" && rt.Image != "" {
			return fmt.Errorf("command '%s' implementation #%d: containerfile and image are mutually exclusive - specify only one", cmdName, implIndex)
		}
		// At least one of containerfile or image must be specified for container runtime
		if rt.Containerfile == "" && rt.Image == "" {
			return fmt.Errorf("command '%s' implementation #%d: container runtime requires either containerfile or image to be specified", cmdName, implIndex)
		}
		// Validate container image name format
		if rt.Image != "" {
			if err := ValidateContainerImage(rt.Image); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: invalid image: %w", cmdName, implIndex, err)
			}
		}
		// Validate volume mounts
		for i, vol := range rt.Volumes {
			if err := ValidateVolumeMount(vol); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: volume #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
		// Validate port mappings
		for i, port := range rt.Ports {
			if err := ValidatePortMapping(port); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: port #%d: %w", cmdName, implIndex, i+1, err)
			}
		}
	}
	return nil
}

// validateCommand validates a single command
func (inv *Invkfile) validateCommand(cmd *Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("command must have a name in invkfile at %s", inv.FilePath)
	}

	// Validate command name length
	if err := ValidateStringLength(cmd.Name, "command name", MaxNameLength); err != nil {
		return fmt.Errorf("command '%s': %w in invkfile at %s", cmd.Name, err, inv.FilePath)
	}

	// Validate description length
	if err := ValidateStringLength(cmd.Description, "description", MaxDescriptionLength); err != nil {
		return fmt.Errorf("command '%s': %w in invkfile at %s", cmd.Name, err, inv.FilePath)
	}

	// Validate command-level custom_checks dependencies
	if cmd.DependsOn != nil && len(cmd.DependsOn.CustomChecks) > 0 {
		if err := validateCustomChecks(cmd.DependsOn.CustomChecks, fmt.Sprintf("command '%s'", cmd.Name), inv.FilePath); err != nil {
			return err
		}
	}

	if len(cmd.Implementations) == 0 {
		return fmt.Errorf("command '%s' must have at least one implementation in invkfile at %s", cmd.Name, inv.FilePath)
	}

	// Validate each implementation
	for i, impl := range cmd.Implementations {
		if impl.Script == "" {
			return fmt.Errorf("command '%s' implementation #%d must have a script in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate script length (only for inline scripts, not file paths)
		if !impl.IsScriptFile() {
			if err := ValidateStringLength(impl.Script, "script", MaxScriptLength); err != nil {
				return fmt.Errorf("command '%s' implementation #%d: %w in invkfile at %s", cmd.Name, i+1, err, inv.FilePath)
			}
		}

		if len(impl.Runtimes) == 0 {
			return fmt.Errorf("command '%s' implementation #%d must have at least one runtime in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate each runtime config
		for j := range impl.Runtimes {
			if err := validateRuntimeConfig(&impl.Runtimes[j], cmd.Name, i+1); err != nil {
				return err
			}
		}

		// Validate implementation-level custom_checks dependencies
		if impl.DependsOn != nil && len(impl.DependsOn.CustomChecks) > 0 {
			if err := validateCustomChecks(impl.DependsOn.CustomChecks, fmt.Sprintf("command '%s' implementation #%d", cmd.Name, i+1), inv.FilePath); err != nil {
				return err
			}
		}
	}

	// Validate that there are no duplicate platform+runtime combinations
	if err := cmd.ValidateScripts(); err != nil {
		return err
	}

	// Validate flags
	if err := inv.validateFlags(cmd); err != nil {
		return err
	}

	// Validate args
	if err := inv.validateArgs(cmd); err != nil {
		return err
	}

	return nil
}

// validateFlags validates the flags for a command
func (inv *Invkfile) validateFlags(cmd *Command) error {
	seenNames := make(map[string]bool)
	seenShorts := make(map[string]bool)

	for i, flag := range cmd.Flags {
		// Validate name is not empty
		if flag.Name == "" {
			return fmt.Errorf("command '%s' flag #%d must have a name in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate flag name length
		if err := ValidateStringLength(flag.Name, "flag name", MaxNameLength); err != nil {
			return fmt.Errorf("command '%s' flag '%s': %w in invkfile at %s", cmd.Name, flag.Name, err, inv.FilePath)
		}

		// Validate name is POSIX-compliant
		if !flagNameRegex.MatchString(flag.Name) {
			return fmt.Errorf("command '%s' flag '%s' has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate description is not empty (after trimming whitespace)
		if strings.TrimSpace(flag.Description) == "" {
			return fmt.Errorf("command '%s' flag '%s' must have a non-empty description in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate description length
		if err := ValidateStringLength(flag.Description, "flag description", MaxDescriptionLength); err != nil {
			return fmt.Errorf("command '%s' flag '%s': %w in invkfile at %s", cmd.Name, flag.Name, err, inv.FilePath)
		}

		// Check for duplicate flag names
		if seenNames[flag.Name] {
			return fmt.Errorf("command '%s' has duplicate flag name '%s' in invkfile at %s", cmd.Name, flag.Name, inv.FilePath)
		}
		seenNames[flag.Name] = true

		// Check for reserved flag names
		if flag.Name == "env-file" {
			return fmt.Errorf("command '%s' flag '%s': 'env-file' is a reserved system flag and cannot be used in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}
		if flag.Name == "env-var" {
			return fmt.Errorf("command '%s' flag '%s': 'env-var' is a reserved system flag and cannot be used in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate type is valid (if specified)
		if flag.Type != "" && flag.Type != FlagTypeString && flag.Type != FlagTypeBool && flag.Type != FlagTypeInt && flag.Type != FlagTypeFloat {
			return fmt.Errorf("command '%s' flag '%s' has invalid type '%s' (must be 'string', 'bool', 'int', or 'float') in invkfile at %s",
				cmd.Name, flag.Name, flag.Type, inv.FilePath)
		}

		// Validate that required flags don't have default values
		if flag.Required && flag.DefaultValue != "" {
			return fmt.Errorf("command '%s' flag '%s' cannot be both required and have a default_value in invkfile at %s",
				cmd.Name, flag.Name, inv.FilePath)
		}

		// Validate short alias format (single letter a-z or A-Z)
		if flag.Short != "" {
			if len(flag.Short) != 1 || !((flag.Short[0] >= 'a' && flag.Short[0] <= 'z') || (flag.Short[0] >= 'A' && flag.Short[0] <= 'Z')) {
				return fmt.Errorf("command '%s' flag '%s' has invalid short alias '%s' (must be a single letter a-z or A-Z) in invkfile at %s",
					cmd.Name, flag.Name, flag.Short, inv.FilePath)
			}
			// Check for reserved short aliases
			if flag.Short == "e" {
				return fmt.Errorf("command '%s' flag '%s': short alias 'e' is reserved for the system --env-file flag in invkfile at %s",
					cmd.Name, flag.Name, inv.FilePath)
			}
			if flag.Short == "E" {
				return fmt.Errorf("command '%s' flag '%s': short alias 'E' is reserved for the system --env-var flag in invkfile at %s",
					cmd.Name, flag.Name, inv.FilePath)
			}
			// Check for duplicate short aliases
			if seenShorts[flag.Short] {
				return fmt.Errorf("command '%s' has duplicate short alias '%s' in invkfile at %s",
					cmd.Name, flag.Short, inv.FilePath)
			}
			seenShorts[flag.Short] = true
		}

		// Validate default_value is compatible with type
		if flag.DefaultValue != "" {
			if err := validateFlagValueType(flag.DefaultValue, flag.GetType()); err != nil {
				return fmt.Errorf("command '%s' flag '%s' default_value '%s' is not compatible with type '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.DefaultValue, flag.GetType(), err.Error(), inv.FilePath)
			}
		}

		// Validate validation regex is valid and safe
		if flag.Validation != "" {
			// Check for regex complexity/safety issues first
			if err := ValidateRegexPattern(flag.Validation); err != nil {
				return fmt.Errorf("command '%s' flag '%s' has unsafe validation regex '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.Validation, err.Error(), inv.FilePath)
			}

			validationRegex, err := regexp.Compile(flag.Validation)
			if err != nil {
				return fmt.Errorf("command '%s' flag '%s' has invalid validation regex '%s': %s in invkfile at %s",
					cmd.Name, flag.Name, flag.Validation, err.Error(), inv.FilePath)
			}

			// Validate default_value matches validation regex (if both specified)
			if flag.DefaultValue != "" {
				if !validationRegex.MatchString(flag.DefaultValue) {
					return fmt.Errorf("command '%s' flag '%s' default_value '%s' does not match validation pattern '%s' in invkfile at %s",
						cmd.Name, flag.Name, flag.DefaultValue, flag.Validation, inv.FilePath)
				}
			}
		}
	}

	return nil
}

// argNameRegex validates POSIX-compliant argument names
var argNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// validateArgs validates the args for a command
func (inv *Invkfile) validateArgs(cmd *Command) error {
	if len(cmd.Args) == 0 {
		return nil
	}

	seenNames := make(map[string]bool)
	foundOptional := false
	foundVariadic := false

	for i, arg := range cmd.Args {
		// Validate name is not empty
		if arg.Name == "" {
			return fmt.Errorf("command '%s' argument #%d must have a name in invkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate argument name length
		if err := ValidateStringLength(arg.Name, "argument name", MaxNameLength); err != nil {
			return fmt.Errorf("command '%s' argument '%s': %w in invkfile at %s", cmd.Name, arg.Name, err, inv.FilePath)
		}

		// Validate name is POSIX-compliant
		if !argNameRegex.MatchString(arg.Name) {
			return fmt.Errorf("command '%s' argument '%s' has invalid name (must start with a letter, contain only alphanumeric, hyphens, and underscores) in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}

		// Validate description is not empty (after trimming whitespace)
		if strings.TrimSpace(arg.Description) == "" {
			return fmt.Errorf("command '%s' argument '%s' must have a non-empty description in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}

		// Validate description length
		if err := ValidateStringLength(arg.Description, "argument description", MaxDescriptionLength); err != nil {
			return fmt.Errorf("command '%s' argument '%s': %w in invkfile at %s", cmd.Name, arg.Name, err, inv.FilePath)
		}

		// Check for duplicate argument names
		if seenNames[arg.Name] {
			return fmt.Errorf("command '%s' has duplicate argument name '%s' in invkfile at %s", cmd.Name, arg.Name, inv.FilePath)
		}
		seenNames[arg.Name] = true

		// Validate type is valid (if specified) - note: bool is not allowed for args
		if arg.Type != "" && arg.Type != ArgumentTypeString && arg.Type != ArgumentTypeInt && arg.Type != ArgumentTypeFloat {
			return fmt.Errorf("command '%s' argument '%s' has invalid type '%s' (must be 'string', 'int', or 'float') in invkfile at %s",
				cmd.Name, arg.Name, arg.Type, inv.FilePath)
		}

		// Validate that required arguments don't have default values
		if arg.Required && arg.DefaultValue != "" {
			return fmt.Errorf("command '%s' argument '%s' cannot be both required and have a default_value in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}

		// Rule: Required arguments must come before optional arguments
		isOptional := !arg.Required
		if arg.Required && foundOptional {
			return fmt.Errorf("command '%s' argument '%s': required arguments must come before optional arguments in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}
		if isOptional {
			foundOptional = true
		}

		// Rule: Only the last argument can be variadic
		if foundVariadic {
			return fmt.Errorf("command '%s' argument '%s': only the last argument can be variadic (found after variadic argument) in invkfile at %s",
				cmd.Name, arg.Name, inv.FilePath)
		}
		if arg.Variadic {
			foundVariadic = true
		}

		// Validate default_value is compatible with type
		if arg.DefaultValue != "" {
			if err := validateArgumentValueType(arg.DefaultValue, arg.GetType()); err != nil {
				return fmt.Errorf("command '%s' argument '%s' default_value '%s' is not compatible with type '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.DefaultValue, arg.GetType(), err.Error(), inv.FilePath)
			}
		}

		// Validate validation regex is valid and safe
		if arg.Validation != "" {
			// Check for regex complexity/safety issues first
			if err := ValidateRegexPattern(arg.Validation); err != nil {
				return fmt.Errorf("command '%s' argument '%s' has unsafe validation regex '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.Validation, err.Error(), inv.FilePath)
			}

			validationRegex, err := regexp.Compile(arg.Validation)
			if err != nil {
				return fmt.Errorf("command '%s' argument '%s' has invalid validation regex '%s': %s in invkfile at %s",
					cmd.Name, arg.Name, arg.Validation, err.Error(), inv.FilePath)
			}

			// Validate default_value matches validation regex (if both specified)
			if arg.DefaultValue != "" {
				if !validationRegex.MatchString(arg.DefaultValue) {
					return fmt.Errorf("command '%s' argument '%s' default_value '%s' does not match validation pattern '%s' in invkfile at %s",
						cmd.Name, arg.Name, arg.DefaultValue, arg.Validation, inv.FilePath)
				}
			}
		}
	}

	return nil
}

// validateCustomChecks validates custom check dependencies for security and correctness
func validateCustomChecks(checks []CustomCheckDependency, context string, filePath string) error {
	for i, checkDep := range checks {
		// Get all checks (handles both direct and alternatives formats)
		for j, check := range checkDep.GetChecks() {
			// Validate name length
			if check.Name != "" {
				if err := ValidateStringLength(check.Name, "custom_check name", MaxNameLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// Validate check_script length (same limit as implementation scripts)
			if check.CheckScript != "" {
				if err := ValidateStringLength(check.CheckScript, "check_script", MaxScriptLength); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}

			// Validate expected_output regex pattern for safety
			if check.ExpectedOutput != "" {
				if err := ValidateRegexPattern(check.ExpectedOutput); err != nil {
					return fmt.Errorf("%s custom_check #%d alternative #%d: expected_output: %w in invkfile at %s", context, i+1, j+1, err, filePath)
				}
			}
		}
	}
	return nil
}

// validateFlagValueType validates that a value is compatible with the specified flag type
func validateFlagValueType(value string, flagType FlagType) error {
	switch flagType {
	case FlagTypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	case FlagTypeInt:
		// Check if value is a valid integer
		for i, c := range value {
			if i == 0 && c == '-' {
				continue // Allow negative sign at start
			}
			if c < '0' || c > '9' {
				return fmt.Errorf("must be a valid integer")
			}
		}
		if value == "" || value == "-" {
			return fmt.Errorf("must be a valid integer")
		}
	case FlagTypeFloat:
		// Check if value is a valid floating-point number
		if value == "" {
			return fmt.Errorf("must be a valid floating-point number")
		}
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("must be a valid floating-point number")
		}
	case FlagTypeString:
		// Any string is valid
	default:
		// Default to string (any value is valid)
	}
	return nil
}

// ValidateFlagValue validates a flag value at runtime against type and validation regex.
// Returns nil if the value is valid, or an error describing the issue.
func (f *Flag) ValidateFlagValue(value string) error {
	// Validate type
	if err := validateFlagValueType(value, f.GetType()); err != nil {
		return fmt.Errorf("flag '%s' value '%s' is invalid: %s", f.Name, value, err.Error())
	}

	// Validate against regex pattern
	if f.Validation != "" {
		validationRegex, err := regexp.Compile(f.Validation)
		if err != nil {
			// This shouldn't happen as the regex is validated at parse time
			return fmt.Errorf("flag '%s' has invalid validation pattern: %s", f.Name, err.Error())
		}
		if !validationRegex.MatchString(value) {
			return fmt.Errorf("flag '%s' value '%s' does not match required pattern '%s'", f.Name, value, f.Validation)
		}
	}

	return nil
}

// GetCommand finds a command by its name (supports names with spaces like "test unit")
func (inv *Invkfile) GetCommand(name string) *Command {
	if name == "" {
		return nil
	}

	for i := range inv.Commands {
		if inv.Commands[i].Name == name {
			return &inv.Commands[i]
		}
	}

	return nil
}

// IsFromPack returns true if this invkfile was loaded from a pack
func (inv *Invkfile) IsFromPack() bool {
	return inv.PackPath != ""
}

// GetScriptBasePath returns the base path for resolving script file references.
// For pack invkfiles, this is the pack path.
// For regular invkfiles, this is the directory containing the invkfile.
func (inv *Invkfile) GetScriptBasePath() string {
	if inv.PackPath != "" {
		return inv.PackPath
	}
	return filepath.Dir(inv.FilePath)
}

// GetEffectiveWorkDir resolves the effective working directory for command execution.
// It follows the precedence hierarchy (highest to lowest):
//  1. CLI override (cliOverride parameter)
//  2. Implementation-level workdir (impl.WorkDir)
//  3. Command-level workdir (cmd.WorkDir)
//  4. Root-level workdir (inv.WorkDir)
//  5. Default: invkfile directory
//
// All workdir paths in CUE should use forward slashes for cross-platform compatibility.
// Relative paths are resolved against the invkfile location.
func (inv *Invkfile) GetEffectiveWorkDir(cmd *Command, impl *Implementation, cliOverride string) string {
	invkfileDir := inv.GetScriptBasePath()

	// resolve converts a workdir path from CUE format (forward slashes) to native format
	// and resolves relative paths against the invkfile directory.
	resolve := func(workdir string) string {
		if workdir == "" {
			return ""
		}
		// Convert forward slashes to native path separator
		nativePath := filepath.FromSlash(workdir)
		if filepath.IsAbs(nativePath) {
			return nativePath
		}
		return filepath.Join(invkfileDir, nativePath)
	}

	// Priority 1: CLI override
	if cliOverride != "" {
		return resolve(cliOverride)
	}

	// Priority 2: Implementation-level
	if impl != nil && impl.WorkDir != "" {
		return resolve(impl.WorkDir)
	}

	// Priority 3: Command-level
	if cmd != nil && cmd.WorkDir != "" {
		return resolve(cmd.WorkDir)
	}

	// Priority 4: Root-level
	if inv.WorkDir != "" {
		return resolve(inv.WorkDir)
	}

	// Priority 5: Default (invkfile directory)
	return invkfileDir
}

// GetFullCommandName returns the fully qualified command name with the pack prefix.
// The format is "pack cmdname" where cmdname may have spaces for subcommands.
// Returns empty string for the pack prefix if no Metadata is set.
func (inv *Invkfile) GetFullCommandName(cmdName string) string {
	if inv.Metadata != nil {
		return inv.Metadata.Pack + " " + cmdName
	}
	return cmdName
}

// GetPack returns the pack identifier from Metadata, or empty string if not set.
func (inv *Invkfile) GetPack() string {
	if inv.Metadata != nil {
		return inv.Metadata.Pack
	}
	return ""
}

// ListCommands returns all command names at the top level (with pack prefix)
func (inv *Invkfile) ListCommands() []string {
	names := make([]string, len(inv.Commands))
	for i, cmd := range inv.Commands {
		names[i] = inv.GetFullCommandName(cmd.Name)
	}
	return names
}

// FlattenCommands returns all commands keyed by their fully qualified names (with pack prefix)
func (inv *Invkfile) FlattenCommands() map[string]*Command {
	result := make(map[string]*Command)
	for i := range inv.Commands {
		fullName := inv.GetFullCommandName(inv.Commands[i].Name)
		result[fullName] = &inv.Commands[i]
	}
	return result
}

// GenerateInvkpackCUE generates a CUE representation of pack metadata (invkpack.cue)
func GenerateInvkpackCUE(meta *Invkpack) string {
	var sb strings.Builder

	sb.WriteString("// Invkpack - Pack metadata for invowk\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation\n\n")

	// Pack is mandatory
	sb.WriteString(fmt.Sprintf("pack: %q\n", meta.Pack))

	if meta.Version != "" {
		sb.WriteString(fmt.Sprintf("version: %q\n", meta.Version))
	}
	if meta.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %q\n", meta.Description))
	}

	// Requires
	if len(meta.Requires) > 0 {
		sb.WriteString("\nrequires: [\n")
		for _, req := range meta.Requires {
			sb.WriteString("\t{\n")
			sb.WriteString(fmt.Sprintf("\t\tgit_url: %q\n", req.GitURL))
			sb.WriteString(fmt.Sprintf("\t\tversion: %q\n", req.Version))
			if req.Alias != "" {
				sb.WriteString(fmt.Sprintf("\t\talias: %q\n", req.Alias))
			}
			if req.Path != "" {
				sb.WriteString(fmt.Sprintf("\t\tpath: %q\n", req.Path))
			}
			sb.WriteString("\t},\n")
		}
		sb.WriteString("]\n")
	}

	return sb.String()
}

// GenerateCUE generates a CUE representation of an Invkfile (commands only)
func GenerateCUE(inv *Invkfile) string {
	var sb strings.Builder

	sb.WriteString("// Invkfile - Command definitions for invowk\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation\n\n")

	if inv.DefaultShell != "" {
		sb.WriteString(fmt.Sprintf("default_shell: %q\n", inv.DefaultShell))
	}
	if inv.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("workdir: %q\n", inv.WorkDir))
	}

	// Root-level env
	if inv.Env != nil && (len(inv.Env.Files) > 0 || len(inv.Env.Vars) > 0) {
		sb.WriteString("env: {\n")
		if len(inv.Env.Files) > 0 {
			sb.WriteString("\tfiles: [")
			for i, ef := range inv.Env.Files {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%q", ef))
			}
			sb.WriteString("]\n")
		}
		if len(inv.Env.Vars) > 0 {
			sb.WriteString("\tvars: {\n")
			for k, v := range inv.Env.Vars {
				sb.WriteString(fmt.Sprintf("\t\t%s: %q\n", k, v))
			}
			sb.WriteString("\t}\n")
		}
		sb.WriteString("}\n")
	}

	// Root-level depends_on
	if inv.DependsOn != nil && (len(inv.DependsOn.Tools) > 0 || len(inv.DependsOn.Commands) > 0 || len(inv.DependsOn.Filepaths) > 0 || len(inv.DependsOn.Capabilities) > 0 || len(inv.DependsOn.CustomChecks) > 0 || len(inv.DependsOn.EnvVars) > 0) {
		sb.WriteString("depends_on: {\n")
		if len(inv.DependsOn.Tools) > 0 {
			sb.WriteString("\ttools: [\n")
			for _, tool := range inv.DependsOn.Tools {
				sb.WriteString("\t\t{alternatives: [")
				for i, alt := range tool.Alternatives {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", alt))
				}
				sb.WriteString("]},\n")
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.DependsOn.Commands) > 0 {
			sb.WriteString("\tcmds: [\n")
			for _, dep := range inv.DependsOn.Commands {
				sb.WriteString("\t\t{alternatives: [")
				for i, alt := range dep.Alternatives {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", alt))
				}
				sb.WriteString("]},\n")
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.DependsOn.Filepaths) > 0 {
			sb.WriteString("\tfilepaths: [\n")
			for _, fp := range inv.DependsOn.Filepaths {
				sb.WriteString("\t\t{alternatives: [")
				for i, alt := range fp.Alternatives {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", alt))
				}
				sb.WriteString("]")
				if fp.Readable {
					sb.WriteString(", readable: true")
				}
				if fp.Writable {
					sb.WriteString(", writable: true")
				}
				if fp.Executable {
					sb.WriteString(", executable: true")
				}
				sb.WriteString("},\n")
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.DependsOn.Capabilities) > 0 {
			sb.WriteString("\tcapabilities: [\n")
			for _, cap := range inv.DependsOn.Capabilities {
				sb.WriteString("\t\t{alternatives: [")
				for i, alt := range cap.Alternatives {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", alt))
				}
				sb.WriteString("]},\n")
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.DependsOn.CustomChecks) > 0 {
			sb.WriteString("\tcustom_checks: [\n")
			for _, check := range inv.DependsOn.CustomChecks {
				if check.IsAlternatives() {
					sb.WriteString("\t\t{alternatives: [\n")
					for _, alt := range check.Alternatives {
						sb.WriteString("\t\t\t{")
						sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", alt.Name, alt.CheckScript))
						if alt.ExpectedCode != nil {
							sb.WriteString(fmt.Sprintf(", expected_code: %d", *alt.ExpectedCode))
						}
						if alt.ExpectedOutput != "" {
							sb.WriteString(fmt.Sprintf(", expected_output: %q", alt.ExpectedOutput))
						}
						sb.WriteString("},\n")
					}
					sb.WriteString("\t\t]},\n")
				} else {
					sb.WriteString("\t\t{")
					sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", check.Name, check.CheckScript))
					if check.ExpectedCode != nil {
						sb.WriteString(fmt.Sprintf(", expected_code: %d", *check.ExpectedCode))
					}
					if check.ExpectedOutput != "" {
						sb.WriteString(fmt.Sprintf(", expected_output: %q", check.ExpectedOutput))
					}
					sb.WriteString("},\n")
				}
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.DependsOn.EnvVars) > 0 {
			sb.WriteString("\tenv_vars: [\n")
			for _, envVar := range inv.DependsOn.EnvVars {
				sb.WriteString("\t\t{alternatives: [")
				for i, alt := range envVar.Alternatives {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString("{")
					sb.WriteString(fmt.Sprintf("name: %q", alt.Name))
					if alt.Validation != "" {
						sb.WriteString(fmt.Sprintf(", validation: %q", alt.Validation))
					}
					sb.WriteString("}")
				}
				sb.WriteString("]},\n")
			}
			sb.WriteString("\t]\n")
		}
		sb.WriteString("}\n")
	}

	// Commands
	sb.WriteString("\ncmds: [\n")
	for _, cmd := range inv.Commands {
		sb.WriteString("\t{\n")
		sb.WriteString(fmt.Sprintf("\t\tname: %q\n", cmd.Name))
		if cmd.Description != "" {
			sb.WriteString(fmt.Sprintf("\t\tdescription: %q\n", cmd.Description))
		}

		// Generate implementations list
		sb.WriteString("\t\timplementations: [\n")
		for _, impl := range cmd.Implementations {
			sb.WriteString("\t\t\t{\n")

			// Handle multi-line scripts with CUE's multi-line string syntax
			if strings.Contains(impl.Script, "\n") {
				sb.WriteString("\t\t\t\tscript: \"\"\"\n")
				for _, line := range strings.Split(impl.Script, "\n") {
					sb.WriteString(fmt.Sprintf("\t\t\t\t\t%s\n", line))
				}
				sb.WriteString("\t\t\t\t\t\"\"\"\n")
			} else {
				sb.WriteString(fmt.Sprintf("\t\t\t\tscript: %q\n", impl.Script))
			}

			// Runtimes (each is a struct with name and optional fields)
			sb.WriteString("\t\t\t\truntimes: [\n")
			for _, r := range impl.Runtimes {
				sb.WriteString("\t\t\t\t\t{")
				sb.WriteString(fmt.Sprintf("name: %q", r.Name))
				if r.Name == RuntimeContainer {
					if r.EnableHostSSH {
						sb.WriteString(", enable_host_ssh: true")
					}
					if r.Containerfile != "" {
						sb.WriteString(fmt.Sprintf(", containerfile: %q", r.Containerfile))
					}
					if r.Image != "" {
						sb.WriteString(fmt.Sprintf(", image: %q", r.Image))
					}
					if len(r.Volumes) > 0 {
						sb.WriteString(", volumes: [")
						for i, v := range r.Volumes {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", v))
						}
						sb.WriteString("]")
					}
					if len(r.Ports) > 0 {
						sb.WriteString(", ports: [")
						for i, p := range r.Ports {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", p))
						}
						sb.WriteString("]")
					}
				}
				sb.WriteString("},\n")
			}
			sb.WriteString("\t\t\t\t]\n")

			// Platforms (optional, each is a struct with name only)
			if len(impl.Platforms) > 0 {
				sb.WriteString("\t\t\t\tplatforms: [\n")
				for _, p := range impl.Platforms {
					sb.WriteString(fmt.Sprintf("\t\t\t\t\t{name: %q},\n", p.Name))
				}
				sb.WriteString("\t\t\t\t]\n")
			}

			// Implementation-level depends_on
			if impl.DependsOn != nil && (len(impl.DependsOn.Tools) > 0 || len(impl.DependsOn.Commands) > 0 || len(impl.DependsOn.Filepaths) > 0 || len(impl.DependsOn.Capabilities) > 0 || len(impl.DependsOn.CustomChecks) > 0 || len(impl.DependsOn.EnvVars) > 0) {
				sb.WriteString("\t\t\t\tdepends_on: {\n")
				if len(impl.DependsOn.Tools) > 0 {
					sb.WriteString("\t\t\t\t\ttools: [\n")
					for _, tool := range impl.DependsOn.Tools {
						sb.WriteString("\t\t\t\t\t\t{alternatives: [")
						for i, alt := range tool.Alternatives {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", alt))
						}
						sb.WriteString("]},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.Commands) > 0 {
					sb.WriteString("\t\t\t\t\tcmds: [\n")
					for _, dep := range impl.DependsOn.Commands {
						sb.WriteString("\t\t\t\t\t\t{alternatives: [")
						for i, alt := range dep.Alternatives {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", alt))
						}
						sb.WriteString("]},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.Filepaths) > 0 {
					sb.WriteString("\t\t\t\t\tfilepaths: [\n")
					for _, fp := range impl.DependsOn.Filepaths {
						sb.WriteString("\t\t\t\t\t\t{alternatives: [")
						for i, alt := range fp.Alternatives {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", alt))
						}
						sb.WriteString("]")
						if fp.Readable {
							sb.WriteString(", readable: true")
						}
						if fp.Writable {
							sb.WriteString(", writable: true")
						}
						if fp.Executable {
							sb.WriteString(", executable: true")
						}
						sb.WriteString("},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.Capabilities) > 0 {
					sb.WriteString("\t\t\t\t\tcapabilities: [\n")
					for _, cap := range impl.DependsOn.Capabilities {
						sb.WriteString("\t\t\t\t\t\t{alternatives: [")
						for i, alt := range cap.Alternatives {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%q", alt))
						}
						sb.WriteString("]},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.CustomChecks) > 0 {
					sb.WriteString("\t\t\t\t\tcustom_checks: [\n")
					for _, check := range impl.DependsOn.CustomChecks {
						if check.IsAlternatives() {
							sb.WriteString("\t\t\t\t\t\t{alternatives: [\n")
							for _, alt := range check.Alternatives {
								sb.WriteString("\t\t\t\t\t\t\t{")
								sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", alt.Name, alt.CheckScript))
								if alt.ExpectedCode != nil {
									sb.WriteString(fmt.Sprintf(", expected_code: %d", *alt.ExpectedCode))
								}
								if alt.ExpectedOutput != "" {
									sb.WriteString(fmt.Sprintf(", expected_output: %q", alt.ExpectedOutput))
								}
								sb.WriteString("},\n")
							}
							sb.WriteString("\t\t\t\t\t\t]},\n")
						} else {
							sb.WriteString("\t\t\t\t\t\t{")
							sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", check.Name, check.CheckScript))
							if check.ExpectedCode != nil {
								sb.WriteString(fmt.Sprintf(", expected_code: %d", *check.ExpectedCode))
							}
							if check.ExpectedOutput != "" {
								sb.WriteString(fmt.Sprintf(", expected_output: %q", check.ExpectedOutput))
							}
							sb.WriteString("},\n")
						}
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.EnvVars) > 0 {
					sb.WriteString("\t\t\t\t\tenv_vars: [\n")
					for _, envVar := range impl.DependsOn.EnvVars {
						sb.WriteString("\t\t\t\t\t\t{alternatives: [")
						for i, alt := range envVar.Alternatives {
							if i > 0 {
								sb.WriteString(", ")
							}
							sb.WriteString("{")
							sb.WriteString(fmt.Sprintf("name: %q", alt.Name))
							if alt.Validation != "" {
								sb.WriteString(fmt.Sprintf(", validation: %q", alt.Validation))
							}
							sb.WriteString("}")
						}
						sb.WriteString("]},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				sb.WriteString("\t\t\t\t}\n")
			}

			// Implementation-level env
			if impl.Env != nil && (len(impl.Env.Files) > 0 || len(impl.Env.Vars) > 0) {
				sb.WriteString("\t\t\t\tenv: {\n")
				if len(impl.Env.Files) > 0 {
					sb.WriteString("\t\t\t\t\tfiles: [")
					for i, ef := range impl.Env.Files {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", ef))
					}
					sb.WriteString("]\n")
				}
				if len(impl.Env.Vars) > 0 {
					sb.WriteString("\t\t\t\t\tvars: {\n")
					for k, v := range impl.Env.Vars {
						sb.WriteString(fmt.Sprintf("\t\t\t\t\t\t%s: %q\n", k, v))
					}
					sb.WriteString("\t\t\t\t\t}\n")
				}
				sb.WriteString("\t\t\t\t}\n")
			}

			// Implementation-level workdir
			if impl.WorkDir != "" {
				sb.WriteString(fmt.Sprintf("\t\t\t\tworkdir: %q\n", impl.WorkDir))
			}

			sb.WriteString("\t\t\t},\n")
		}
		sb.WriteString("\t\t]\n")

		// Command-level env
		if cmd.Env != nil && (len(cmd.Env.Files) > 0 || len(cmd.Env.Vars) > 0) {
			sb.WriteString("\t\tenv: {\n")
			if len(cmd.Env.Files) > 0 {
				sb.WriteString("\t\t\tfiles: [")
				for i, ef := range cmd.Env.Files {
					if i > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%q", ef))
				}
				sb.WriteString("]\n")
			}
			if len(cmd.Env.Vars) > 0 {
				sb.WriteString("\t\t\tvars: {\n")
				for k, v := range cmd.Env.Vars {
					sb.WriteString(fmt.Sprintf("\t\t\t\t%s: %q\n", k, v))
				}
				sb.WriteString("\t\t\t}\n")
			}
			sb.WriteString("\t\t}\n")
		}
		if cmd.WorkDir != "" {
			sb.WriteString(fmt.Sprintf("\t\tworkdir: %q\n", cmd.WorkDir))
		}
		if cmd.DependsOn != nil && (len(cmd.DependsOn.Tools) > 0 || len(cmd.DependsOn.Commands) > 0 || len(cmd.DependsOn.Filepaths) > 0 || len(cmd.DependsOn.Capabilities) > 0 || len(cmd.DependsOn.CustomChecks) > 0 || len(cmd.DependsOn.EnvVars) > 0) {
			sb.WriteString("\t\tdepends_on: {\n")
			if len(cmd.DependsOn.Tools) > 0 {
				sb.WriteString("\t\t\ttools: [\n")
				for _, tool := range cmd.DependsOn.Tools {
					sb.WriteString("\t\t\t\t{alternatives: [")
					for i, alt := range tool.Alternatives {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", alt))
					}
					sb.WriteString("]},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.Commands) > 0 {
				sb.WriteString("\t\t\tcmds: [\n")
				for _, dep := range cmd.DependsOn.Commands {
					sb.WriteString("\t\t\t\t{alternatives: [")
					for i, alt := range dep.Alternatives {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", alt))
					}
					sb.WriteString("]},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.Filepaths) > 0 {
				sb.WriteString("\t\t\tfilepaths: [\n")
				for _, fp := range cmd.DependsOn.Filepaths {
					sb.WriteString("\t\t\t\t{alternatives: [")
					for i, alt := range fp.Alternatives {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", alt))
					}
					sb.WriteString("]")
					if fp.Readable {
						sb.WriteString(", readable: true")
					}
					if fp.Writable {
						sb.WriteString(", writable: true")
					}
					if fp.Executable {
						sb.WriteString(", executable: true")
					}
					sb.WriteString("},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.Capabilities) > 0 {
				sb.WriteString("\t\t\tcapabilities: [\n")
				for _, cap := range cmd.DependsOn.Capabilities {
					sb.WriteString("\t\t\t\t{alternatives: [")
					for i, alt := range cap.Alternatives {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString(fmt.Sprintf("%q", alt))
					}
					sb.WriteString("]},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.CustomChecks) > 0 {
				sb.WriteString("\t\t\tcustom_checks: [\n")
				for _, check := range cmd.DependsOn.CustomChecks {
					if check.IsAlternatives() {
						sb.WriteString("\t\t\t\t{alternatives: [\n")
						for _, alt := range check.Alternatives {
							sb.WriteString("\t\t\t\t\t{")
							sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", alt.Name, alt.CheckScript))
							if alt.ExpectedCode != nil {
								sb.WriteString(fmt.Sprintf(", expected_code: %d", *alt.ExpectedCode))
							}
							if alt.ExpectedOutput != "" {
								sb.WriteString(fmt.Sprintf(", expected_output: %q", alt.ExpectedOutput))
							}
							sb.WriteString("},\n")
						}
						sb.WriteString("\t\t\t\t]},\n")
					} else {
						sb.WriteString("\t\t\t\t{")
						sb.WriteString(fmt.Sprintf("name: %q, check_script: %q", check.Name, check.CheckScript))
						if check.ExpectedCode != nil {
							sb.WriteString(fmt.Sprintf(", expected_code: %d", *check.ExpectedCode))
						}
						if check.ExpectedOutput != "" {
							sb.WriteString(fmt.Sprintf(", expected_output: %q", check.ExpectedOutput))
						}
						sb.WriteString("},\n")
					}
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.EnvVars) > 0 {
				sb.WriteString("\t\t\tenv_vars: [\n")
				for _, envVar := range cmd.DependsOn.EnvVars {
					sb.WriteString("\t\t\t\t{alternatives: [")
					for i, alt := range envVar.Alternatives {
						if i > 0 {
							sb.WriteString(", ")
						}
						sb.WriteString("{")
						sb.WriteString(fmt.Sprintf("name: %q", alt.Name))
						if alt.Validation != "" {
							sb.WriteString(fmt.Sprintf(", validation: %q", alt.Validation))
						}
						sb.WriteString("}")
					}
					sb.WriteString("]},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			sb.WriteString("\t\t}\n")
		}
		// Generate flags list
		if len(cmd.Flags) > 0 {
			sb.WriteString("\t\tflags: [\n")
			for _, flag := range cmd.Flags {
				sb.WriteString("\t\t\t{")
				sb.WriteString(fmt.Sprintf("name: %q, description: %q", flag.Name, flag.Description))
				if flag.DefaultValue != "" {
					sb.WriteString(fmt.Sprintf(", default_value: %q", flag.DefaultValue))
				}
				sb.WriteString("},\n")
			}
			sb.WriteString("\t\t]\n")
		}
		// Generate args list
		if len(cmd.Args) > 0 {
			sb.WriteString("\t\targs: [\n")
			for _, arg := range cmd.Args {
				sb.WriteString("\t\t\t{")
				sb.WriteString(fmt.Sprintf("name: %q, description: %q", arg.Name, arg.Description))
				if arg.Required {
					sb.WriteString(", required: true")
				}
				if arg.DefaultValue != "" {
					sb.WriteString(fmt.Sprintf(", default_value: %q", arg.DefaultValue))
				}
				if arg.Type != "" && arg.Type != ArgumentTypeString {
					sb.WriteString(fmt.Sprintf(", type: %q", arg.Type))
				}
				if arg.Validation != "" {
					sb.WriteString(fmt.Sprintf(", validation: %q", arg.Validation))
				}
				if arg.Variadic {
					sb.WriteString(", variadic: true")
				}
				sb.WriteString("},\n")
			}
			sb.WriteString("\t\t]\n")
		}
		sb.WriteString("\t},\n")
	}
	sb.WriteString("]\n")

	return sb.String()
}
