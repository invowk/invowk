// Package invowkfile defines the schema and parsing for invowkfile TOML files.
package invowkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// RuntimeMode defines how commands are executed
type RuntimeMode string

const (
	// RuntimeNative executes commands using the system's default shell
	RuntimeNative RuntimeMode = "native"
	// RuntimeVirtual executes commands using mvdan/sh with u-root utilities
	RuntimeVirtual RuntimeMode = "virtual"
	// RuntimeContainer executes commands inside a disposable container
	RuntimeContainer RuntimeMode = "container"
)

// Command represents a single command that can be executed
type Command struct {
	// Name is the command identifier (can include spaces for subcommand-like behavior, e.g., "test unit")
	Name string `toml:"name"`
	// Description provides help text for the command
	Description string `toml:"description,omitempty"`
	// Runtime specifies which runtime mode to use (defaults to file-level setting)
	Runtime RuntimeMode `toml:"runtime,omitempty"`
	// Script contains the shell commands to execute OR a path to a script file
	// If Script starts with "./" or "/" or ends with common script extensions (.sh, .ps1, .bat, .cmd, .py, .rb),
	// it is treated as a file path. Otherwise, it is treated as inline script content.
	// Multi-line literal strings (using ''' in TOML) are fully supported.
	Script string `toml:"script"`
	// Env contains environment variables to set for this command
	Env map[string]string `toml:"env,omitempty"`
	// WorkDir specifies the working directory for command execution
	WorkDir string `toml:"workdir,omitempty"`
	// DependsOn lists commands that must run before this one
	DependsOn []string `toml:"depends_on,omitempty"`

	// resolvedScript caches the resolved script content
	resolvedScript string
	// scriptResolved indicates if the script has been resolved
	scriptResolved bool
}

// scriptFileExtensions contains extensions that indicate a script file
var scriptFileExtensions = []string{".sh", ".bash", ".ps1", ".bat", ".cmd", ".py", ".rb", ".pl", ".zsh", ".fish"}

// IsScriptFile returns true if the Script field appears to be a file path
func (c *Command) IsScriptFile() bool {
	script := strings.TrimSpace(c.Script)
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
func (c *Command) GetScriptFilePath(invowkfilePath string) string {
	if !c.IsScriptFile() {
		return ""
	}

	script := strings.TrimSpace(c.Script)

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
func (c *Command) ResolveScript(invowkfilePath string) (string, error) {
	if c.scriptResolved {
		return c.resolvedScript, nil
	}

	script := c.Script
	if script == "" {
		return "", fmt.Errorf("command '%s' has no script", c.Name)
	}

	if c.IsScriptFile() {
		scriptPath := c.GetScriptFilePath(invowkfilePath)
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		c.resolvedScript = string(content)
	} else {
		// Inline script - use directly (multi-line strings from TOML are already handled)
		c.resolvedScript = script
	}

	c.scriptResolved = true
	return c.resolvedScript, nil
}

// ResolveScriptWithFS resolves the script using a custom filesystem reader function.
// This is useful for testing with virtual filesystems.
func (c *Command) ResolveScriptWithFS(invowkfilePath string, readFile func(path string) ([]byte, error)) (string, error) {
	script := c.Script
	if script == "" {
		return "", fmt.Errorf("command '%s' has no script", c.Name)
	}

	if c.IsScriptFile() {
		scriptPath := c.GetScriptFilePath(invowkfilePath)
		content, err := readFile(scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to read script file '%s': %w", scriptPath, err)
		}
		return string(content), nil
	}

	// Inline script - use directly
	return script, nil
}

// ContainerConfig defines container-specific settings
type ContainerConfig struct {
	// Dockerfile specifies the path to Dockerfile (relative to invowkfile)
	Dockerfile string `toml:"dockerfile,omitempty"`
	// Image specifies a pre-built image to use instead of building from Dockerfile
	Image string `toml:"image,omitempty"`
	// Volumes specifies volume mounts in "host:container" format
	Volumes []string `toml:"volumes,omitempty"`
	// Ports specifies port mappings in "host:container" format
	Ports []string `toml:"ports,omitempty"`
}

// Invowkfile represents the complete parsed invowkfile
type Invowkfile struct {
	// Version specifies the invowkfile schema version
	Version string `toml:"version,omitempty"`
	// Description provides a summary of this invowkfile's purpose
	Description string `toml:"description,omitempty"`
	// DefaultRuntime sets the default runtime for all commands
	DefaultRuntime RuntimeMode `toml:"default_runtime,omitempty"`
	// DefaultShell overrides the default shell for native runtime
	DefaultShell string `toml:"default_shell,omitempty"`
	// Container holds container-specific configuration
	Container ContainerConfig `toml:"container,omitempty"`
	// Env contains environment variables applied to all commands
	Env map[string]string `toml:"env,omitempty"`
	// Commands defines the available commands
	Commands []Command `toml:"commands"`

	// FilePath stores the path where this invowkfile was loaded from (not in TOML)
	FilePath string `toml:"-"`
}

// InvowkfileName is the standard name for invowkfile
const InvowkfileName = "invowkfile"

// ValidExtensions lists valid file extensions for invowkfile
var ValidExtensions = []string{".toml", ""}

// Parse reads and parses an invowkfile from the given path
func Parse(path string) (*Invowkfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invowkfile at %s: %w", path, err)
	}

	var inv Invowkfile
	if err := toml.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("failed to parse invowkfile at %s: %w", path, err)
	}

	inv.FilePath = path

	// Apply defaults
	if inv.DefaultRuntime == "" {
		inv.DefaultRuntime = RuntimeNative
	}

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

	// Check container runtime requirements
	if inv.DefaultRuntime == RuntimeContainer {
		if err := inv.validateContainerConfig(); err != nil {
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

// validateContainerConfig checks that container configuration is valid
// Note: This does NOT validate Dockerfile existence - that's done at runtime
func (inv *Invowkfile) validateContainerConfig() error {
	// Container config validation is deferred to runtime
	// to allow invowkfiles to be parsed even when container runtime won't be used
	return nil
}

// validateCommand validates a single command
func (inv *Invowkfile) validateCommand(cmd *Command) error {
	if cmd.Name == "" {
		return fmt.Errorf("command must have a name in invowkfile at %s", inv.FilePath)
	}

	if cmd.Script == "" {
		return fmt.Errorf("command '%s' must have a script in invowkfile at %s", cmd.Name, inv.FilePath)
	}

	// Apply default runtime if not specified
	if cmd.Runtime == "" {
		cmd.Runtime = inv.DefaultRuntime
	}

	// Validate container config for commands using container runtime
	if cmd.Runtime == RuntimeContainer {
		if err := inv.validateContainerConfig(); err != nil {
			return fmt.Errorf("command '%s': %w", cmd.Name, err)
		}
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
