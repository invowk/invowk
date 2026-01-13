// Package invowkfile defines the schema and parsing for invowkfile CUE files.
package invowkfile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed invowkfile_schema.cue
var invowkfileSchema string

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

// RuntimeConfig represents a runtime configuration with type-specific options
type RuntimeConfig struct {
	// Name specifies the runtime type (required)
	Name RuntimeMode `json:"name"`
	// HostSSH enables SSH access from container back to host (container only)
	// Only valid when Name is "container". Default: false
	HostSSH bool `json:"host_ssh,omitempty"`
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

// PlatformConfig represents a platform configuration with optional environment variables
type PlatformConfig struct {
	// Name specifies the platform type (required)
	Name PlatformType `json:"name"`
	// Env contains environment variables specific to this platform (optional)
	Env map[string]string `json:"env,omitempty"`
}

// Target defines the runtime and platform constraints for an implementation
type Target struct {
	// Runtimes specifies which runtimes can execute this implementation (required, at least one)
	// The first element is the default runtime for this platform combination
	// Each runtime is a struct with a Name field and optional type-specific fields
	Runtimes []RuntimeConfig `json:"runtimes"`
	// Platforms specifies which operating systems this implementation is for (optional)
	// If empty/nil, the implementation applies to all platforms
	// Each platform is a struct with a Name field
	Platforms []PlatformConfig `json:"platforms,omitempty"`
}

// ToolDependency represents a tool/binary that must be available in PATH
type ToolDependency struct {
	// Name is the binary name that must be in PATH
	Name string `json:"name"`
	// CheckScript is a custom script to validate the tool (optional)
	// If provided, this script is executed instead of just checking PATH
	CheckScript string `json:"check_script,omitempty"`
	// ExpectedCode is the expected exit code from CheckScript (optional, default: 0)
	// Only used when CheckScript is provided
	ExpectedCode *int `json:"expected_code,omitempty"`
	// ExpectedOutput is a regex pattern to match against CheckScript output (optional)
	// Only used when CheckScript is provided
	// Can be used together with ExpectedCode
	ExpectedOutput string `json:"expected_output,omitempty"`
}

// CommandDependency represents another invowk command that must run first
type CommandDependency struct {
	// Name is the command name that must run before this one
	Name string `json:"name"`
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

// DependsOn defines the dependencies for a command
type DependsOn struct {
	// Tools lists binaries that must be available in PATH before running
	Tools []ToolDependency `json:"tools,omitempty"`
	// Commands lists invowk commands that must run before this one
	Commands []CommandDependency `json:"commands,omitempty"`
	// Filepaths lists files or directories that must exist before running
	Filepaths []FilepathDependency `json:"filepaths,omitempty"`
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
	// Target defines the runtime and platform constraints (required)
	Target Target `json:"target"`
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
	// Env contains environment variables to set for this command
	Env map[string]string `json:"env,omitempty"`
	// WorkDir specifies the working directory for command execution
	WorkDir string `json:"workdir,omitempty"`
	// DependsOn specifies dependencies that must be satisfied before running
	DependsOn *DependsOn `json:"depends_on,omitempty"`
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
	if script == nil || len(script.Target.Runtimes) == 0 {
		return RuntimeNative
	}
	return script.Target.Runtimes[0].Name
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
		if len(s.Target.Platforms) == 0 {
			// Script applies to all platforms
			for _, p := range allPlatforms {
				platformSet[p] = true
			}
		} else {
			for _, p := range s.Target.Platforms {
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
			for _, r := range s.Target.Runtimes {
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
		platforms := s.Target.Platforms
		if len(platforms) == 0 {
			platforms = allPlatforms // Applies to all platforms
		}

		for _, p := range platforms {
			for _, r := range s.Target.Runtimes {
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
	if len(s.Target.Platforms) == 0 {
		return true // No platforms specified = all platforms
	}
	for _, p := range s.Target.Platforms {
		if p.Name == platform {
			return true
		}
	}
	return false
}

// HasRuntime returns true if the script supports the given runtime
func (s *Script) HasRuntime(runtime RuntimeMode) bool {
	for _, r := range s.Target.Runtimes {
		if r.Name == runtime {
			return true
		}
	}
	return false
}

// GetRuntimeConfig returns the RuntimeConfig for the given runtime type, or nil if not found
func (s *Script) GetRuntimeConfig(runtime RuntimeMode) *RuntimeConfig {
	for i := range s.Target.Runtimes {
		if s.Target.Runtimes[i].Name == runtime {
			return &s.Target.Runtimes[i]
		}
	}
	return nil
}

// GetDefaultRuntime returns the default runtime type for this script (first runtime in the list)
func (s *Script) GetDefaultRuntime() RuntimeMode {
	if len(s.Target.Runtimes) == 0 {
		return RuntimeNative
	}
	return s.Target.Runtimes[0].Name
}

// GetDefaultRuntimeConfig returns the default RuntimeConfig for this script (first in the list)
func (s *Script) GetDefaultRuntimeConfig() *RuntimeConfig {
	if len(s.Target.Runtimes) == 0 {
		return nil
	}
	return &s.Target.Runtimes[0]
}

// HasHostSSH returns true if any runtime in this script has host_ssh enabled
func (s *Script) HasHostSSH() bool {
	for _, r := range s.Target.Runtimes {
		if r.Name == RuntimeContainer && r.HostSSH {
			return true
		}
	}
	return false
}

// GetHostSSHForRuntime returns whether host_ssh is enabled for the given runtime
func (s *Script) GetHostSSHForRuntime(runtime RuntimeMode) bool {
	if runtime != RuntimeContainer {
		return false // host_ssh is only valid for container runtime
	}
	rc := s.GetRuntimeConfig(runtime)
	if rc == nil {
		return false
	}
	return rc.HostSSH
}

// HasDependencies returns true if the command has any dependencies (at command or script level)
func (c *Command) HasDependencies() bool {
	// Check command-level dependencies
	if c.DependsOn != nil {
		if len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0 {
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
	return len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0
}

// GetCommandDependencies returns the list of command dependency names (from command level)
func (c *Command) GetCommandDependencies() []string {
	if c.DependsOn == nil {
		return nil
	}
	names := make([]string, len(c.DependsOn.Commands))
	for i, dep := range c.DependsOn.Commands {
		names[i] = dep.Name
	}
	return names
}

// HasDependencies returns true if the script has any dependencies
func (s *Script) HasDependencies() bool {
	if s.DependsOn == nil {
		return false
	}
	return len(s.DependsOn.Tools) > 0 || len(s.DependsOn.Commands) > 0 || len(s.DependsOn.Filepaths) > 0
}

// GetCommandDependencies returns the list of command dependency names from this script
func (s *Script) GetCommandDependencies() []string {
	if s.DependsOn == nil {
		return nil
	}
	names := make([]string, len(s.DependsOn.Commands))
	for i, dep := range s.DependsOn.Commands {
		names[i] = dep.Name
	}
	return names
}

// MergeDependsOn merges command-level and implementation-level dependencies
// Implementation-level dependencies are added to command-level dependencies
// Returns a new DependsOn struct with combined dependencies
func MergeDependsOn(cmdDeps, scriptDeps *DependsOn) *DependsOn {
	if cmdDeps == nil && scriptDeps == nil {
		return nil
	}

	merged := &DependsOn{
		Tools:     make([]ToolDependency, 0),
		Commands:  make([]CommandDependency, 0),
		Filepaths: make([]FilepathDependency, 0),
	}

	// Add command-level dependencies first
	if cmdDeps != nil {
		merged.Tools = append(merged.Tools, cmdDeps.Tools...)
		merged.Commands = append(merged.Commands, cmdDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, cmdDeps.Filepaths...)
	}

	// Add implementation-level dependencies
	if scriptDeps != nil {
		merged.Tools = append(merged.Tools, scriptDeps.Tools...)
		merged.Commands = append(merged.Commands, scriptDeps.Commands...)
		merged.Filepaths = append(merged.Filepaths, scriptDeps.Filepaths...)
	}

	// Return nil if no dependencies after merging
	if len(merged.Tools) == 0 && len(merged.Commands) == 0 && len(merged.Filepaths) == 0 {
		return nil
	}

	return merged
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
// The invowkfilePath parameter is used to resolve relative paths.
func (s *Script) GetScriptFilePath(invowkfilePath string) string {
	if !s.IsScriptFile() {
		return ""
	}

	script := strings.TrimSpace(s.Script)

	// If absolute path, return as-is
	if filepath.IsAbs(script) {
		return script
	}

	// Resolve relative to invowkfile directory
	invowkDir := filepath.Dir(invowkfilePath)
	return filepath.Join(invowkDir, script)
}

// ResolveScript returns the actual script content to execute.
// If Script is a file path, it reads the file content.
// If Script is inline content (including multi-line), it returns it directly.
// The invowkfilePath parameter is used to resolve relative paths.
func (s *Script) ResolveScript(invowkfilePath string) (string, error) {
	if s.scriptResolved {
		return s.resolvedScript, nil
	}

	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePath(invowkfilePath)
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
func (s *Script) ResolveScriptWithFS(invowkfilePath string, readFile func(path string) ([]byte, error)) (string, error) {
	script := s.Script
	if script == "" {
		return "", fmt.Errorf("script has no content")
	}

	if s.IsScriptFile() {
		scriptPath := s.GetScriptFilePath(invowkfilePath)
		content, err := readFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		return string(content), nil
	}

	// Inline script - use directly
	return script, nil
}

// Invowkfile represents the complete parsed invowkfile
type Invowkfile struct {
	// Version specifies the invowkfile schema version
	Version string `json:"version,omitempty"`
	// Description provides a summary of this invowkfile's purpose
	Description string `json:"description,omitempty"`
	// DefaultShell overrides the default shell for native runtime
	DefaultShell string `json:"default_shell,omitempty"`
	// Commands defines the available commands
	Commands []Command `json:"commands"`

	// FilePath stores the path where this invowkfile was loaded from (not in CUE)
	FilePath string `json:"-"`
}

// InvowkfileName is the standard name for invowkfile
const InvowkfileName = "invowkfile"

// ValidExtensions lists valid file extensions for invowkfile
var ValidExtensions = []string{".cue", ""}

// Parse reads and parses an invowkfile from the given path
func Parse(path string) (*Invowkfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invowkfile at %s: %w", path, err)
	}

	return ParseBytes(data, path)
}

// ParseBytes parses invowkfile content from bytes
func ParseBytes(data []byte, path string) (*Invowkfile, error) {
	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(invowkfileSchema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	// Compile the user's invowkfile
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return nil, fmt.Errorf("failed to parse invowkfile at %s: %w", path, userValue.Err())
	}

	// Unify with schema to validate
	schema := schemaValue.LookupPath(cue.ParsePath("#Invowkfile"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("invowkfile validation failed at %s: %w", path, err)
	}

	// Decode into struct
	var inv Invowkfile
	if err := unified.Decode(&inv); err != nil {
		return nil, fmt.Errorf("failed to decode invowkfile at %s: %w", path, err)
	}

	inv.FilePath = path

	// Validate and apply command defaults
	if err := inv.validate(); err != nil {
		return nil, err
	}

	return &inv, nil
}

// validate checks the invowkfile for errors and applies defaults
func (inv *Invowkfile) validate() error {
	if len(inv.Commands) == 0 {
		return fmt.Errorf("invowkfile at %s has no commands defined", inv.FilePath)
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
	// Container-specific fields are only valid for container runtime
	if rt.Name != RuntimeContainer {
		if rt.HostSSH {
			return fmt.Errorf("command '%s' implementation #%d: host_ssh is only valid for container runtime", cmdName, implIndex)
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
	}
	return nil
}

// validateCommand validates a single command
func (inv *Invowkfile) validateCommand(cmd *Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("command must have a name in invowkfile at %s", inv.FilePath)
	}

	if len(cmd.Implementations) == 0 {
		return fmt.Errorf("command '%s' must have at least one implementation in invowkfile at %s", cmd.Name, inv.FilePath)
	}

	// Validate each implementation
	for i, impl := range cmd.Implementations {
		if impl.Script == "" {
			return fmt.Errorf("command '%s' implementation #%d must have a script in invowkfile at %s", cmd.Name, i+1, inv.FilePath)
		}
		if len(impl.Target.Runtimes) == 0 {
			return fmt.Errorf("command '%s' implementation #%d must have at least one runtime in invowkfile at %s", cmd.Name, i+1, inv.FilePath)
		}

		// Validate each runtime config
		for j := range impl.Target.Runtimes {
			if err := validateRuntimeConfig(&impl.Target.Runtimes[j], cmd.Name, i+1); err != nil {
				return err
			}
		}
	}

	// Validate that there are no duplicate platform+runtime combinations
	if err := cmd.ValidateScripts(); err != nil {
		return err
	}

	return nil
}

// GetCommand finds a command by its name (supports names with spaces like "test unit")
func (inv *Invowkfile) GetCommand(name string) *Command {
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

// ListCommands returns all command names at the top level
func (inv *Invowkfile) ListCommands() []string {
	names := make([]string, len(inv.Commands))
	for i, cmd := range inv.Commands {
		names[i] = cmd.Name
	}
	return names
}

// FlattenCommands returns all commands keyed by their names
func (inv *Invowkfile) FlattenCommands() map[string]*Command {
	result := make(map[string]*Command)
	for i := range inv.Commands {
		result[inv.Commands[i].Name] = &inv.Commands[i]
	}
	return result
}

// GenerateCUE generates a CUE representation of an Invowkfile
func GenerateCUE(inv *Invowkfile) string {
	var sb strings.Builder

	sb.WriteString("// Invowkfile - Command definitions for invowk\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation\n\n")

	if inv.Version != "" {
		sb.WriteString(fmt.Sprintf("version: %q\n", inv.Version))
	}
	if inv.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %q\n", inv.Description))
	}
	if inv.DefaultShell != "" {
		sb.WriteString(fmt.Sprintf("default_shell: %q\n", inv.DefaultShell))
	}

	// Commands
	sb.WriteString("\ncommands: [\n")
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

			// Target with runtimes and platforms
			sb.WriteString("\t\t\t\ttarget: {\n")

			// Runtimes (each is a struct with name and optional fields)
			sb.WriteString("\t\t\t\t\truntimes: [\n")
			for _, r := range impl.Target.Runtimes {
				sb.WriteString("\t\t\t\t\t\t{")
				sb.WriteString(fmt.Sprintf("name: %q", r.Name))
				if r.Name == RuntimeContainer {
					if r.HostSSH {
						sb.WriteString(", host_ssh: true")
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
			sb.WriteString("\t\t\t\t\t]\n")

			// Platforms (optional, each is a struct with name and optional env)
			if len(impl.Target.Platforms) > 0 {
				sb.WriteString("\t\t\t\t\tplatforms: [\n")
				for _, p := range impl.Target.Platforms {
					if len(p.Env) > 0 {
						sb.WriteString(fmt.Sprintf("\t\t\t\t\t\t{name: %q, env: {", p.Name))
						first := true
						for k, v := range p.Env {
							if !first {
								sb.WriteString(", ")
							}
							sb.WriteString(fmt.Sprintf("%s: %q", k, v))
							first = false
						}
						sb.WriteString("}},\n")
					} else {
						sb.WriteString(fmt.Sprintf("\t\t\t\t\t\t{name: %q},\n", p.Name))
					}
				}
				sb.WriteString("\t\t\t\t\t]\n")
			}

			sb.WriteString("\t\t\t\t}\n") // close target

			// Implementation-level depends_on
			if impl.DependsOn != nil && (len(impl.DependsOn.Tools) > 0 || len(impl.DependsOn.Commands) > 0 || len(impl.DependsOn.Filepaths) > 0) {
				sb.WriteString("\t\t\t\tdepends_on: {\n")
				if len(impl.DependsOn.Tools) > 0 {
					sb.WriteString("\t\t\t\t\ttools: [\n")
					for _, tool := range impl.DependsOn.Tools {
						sb.WriteString("\t\t\t\t\t\t{")
						sb.WriteString(fmt.Sprintf("name: %q", tool.Name))
						if tool.CheckScript != "" {
							sb.WriteString(fmt.Sprintf(", check_script: %q", tool.CheckScript))
						}
						if tool.ExpectedCode != nil {
							sb.WriteString(fmt.Sprintf(", expected_code: %d", *tool.ExpectedCode))
						}
						if tool.ExpectedOutput != "" {
							sb.WriteString(fmt.Sprintf(", expected_output: %q", tool.ExpectedOutput))
						}
						sb.WriteString("},\n")
					}
					sb.WriteString("\t\t\t\t\t]\n")
				}
				if len(impl.DependsOn.Commands) > 0 {
					sb.WriteString("\t\t\t\t\tcommands: [\n")
					for _, dep := range impl.DependsOn.Commands {
						sb.WriteString(fmt.Sprintf("\t\t\t\t\t\t{name: %q},\n", dep.Name))
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
				sb.WriteString("\t\t\t\t}\n")
			}

			sb.WriteString("\t\t\t},\n")
		}
		sb.WriteString("\t\t]\n")

		if len(cmd.Env) > 0 {
			sb.WriteString("\t\tenv: {\n")
			for k, v := range cmd.Env {
				sb.WriteString(fmt.Sprintf("\t\t\t%s: %q\n", k, v))
			}
			sb.WriteString("\t\t}\n")
		}
		if cmd.WorkDir != "" {
			sb.WriteString(fmt.Sprintf("\t\tworkdir: %q\n", cmd.WorkDir))
		}
		if cmd.DependsOn != nil && (len(cmd.DependsOn.Tools) > 0 || len(cmd.DependsOn.Commands) > 0 || len(cmd.DependsOn.Filepaths) > 0) {
			sb.WriteString("\t\tdepends_on: {\n")
			if len(cmd.DependsOn.Tools) > 0 {
				sb.WriteString("\t\t\ttools: [\n")
				for _, tool := range cmd.DependsOn.Tools {
					sb.WriteString("\t\t\t\t{")
					sb.WriteString(fmt.Sprintf("name: %q", tool.Name))
					if tool.CheckScript != "" {
						sb.WriteString(fmt.Sprintf(", check_script: %q", tool.CheckScript))
					}
					if tool.ExpectedCode != nil {
						sb.WriteString(fmt.Sprintf(", expected_code: %d", *tool.ExpectedCode))
					}
					if tool.ExpectedOutput != "" {
						sb.WriteString(fmt.Sprintf(", expected_output: %q", tool.ExpectedOutput))
					}
					sb.WriteString("},\n")
				}
				sb.WriteString("\t\t\t]\n")
			}
			if len(cmd.DependsOn.Commands) > 0 {
				sb.WriteString("\t\t\tcommands: [\n")
				for _, dep := range cmd.DependsOn.Commands {
					sb.WriteString(fmt.Sprintf("\t\t\t\t{name: %q},\n", dep.Name))
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
			sb.WriteString("\t\t}\n")
		}
		sb.WriteString("\t},\n")
	}
	sb.WriteString("]\n")

	return sb.String()
}
