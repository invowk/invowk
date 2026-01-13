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
	// Path is the file or directory path (can be absolute or relative to invowkfile)
	Path string `json:"path"`
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

// HostOS represents a supported operating system
type HostOS string

const (
	// HostLinux represents Linux operating system
	HostLinux HostOS = "linux"
	// HostMac represents macOS operating system
	HostMac HostOS = "mac"
	// HostWindows represents Windows operating system
	HostWindows HostOS = "windows"
)

// WorksOn defines where the command can run
type WorksOn struct {
	// Hosts lists the operating systems where this command can run
	Hosts []HostOS `json:"hosts"`
}

// Command represents a single command that can be executed
type Command struct {
	// Name is the command identifier (can include spaces for subcommand-like behavior, e.g., "test unit")
	Name string `json:"name"`
	// Description provides help text for the command
	Description string `json:"description,omitempty"`
	// Runtimes specifies the allowed execution modes for this command (required, at least one)
	// The first element is the default runtime used when no --runtime flag is specified
	Runtimes []RuntimeMode `json:"runtimes"`
	// Script contains the shell commands to execute OR a path to a script file
	// If Script starts with "./" or "/" or ends with common script extensions (.sh, .ps1, .bat, .cmd, .py, .rb),
	// it is treated as a file path. Otherwise, it is treated as inline script content.
	// Multi-line strings are fully supported in CUE.
	Script string `json:"script"`
	// Env contains environment variables to set for this command
	Env map[string]string `json:"env,omitempty"`
	// WorkDir specifies the working directory for command execution
	WorkDir string `json:"workdir,omitempty"`
	// DependsOn specifies dependencies that must be satisfied before running
	DependsOn *DependsOn `json:"depends_on,omitempty"`
	// WorksOn specifies where this command can run (required)
	WorksOn WorksOn `json:"works_on"`
	// HostSSH enables SSH access from container back to host (container runtime only)
	// When enabled, invowk starts an SSH server and provides connection credentials
	// to the container via environment variables: INVOWK_SSH_HOST, INVOWK_SSH_PORT,
	// INVOWK_SSH_USER, INVOWK_SSH_TOKEN
	HostSSH bool `json:"host_ssh,omitempty"`

	// resolvedScript caches the resolved script content
	resolvedScript string
	// scriptResolved indicates if the script has been resolved
	scriptResolved bool
}

// GetCurrentHostOS returns the current operating system as HostOS
func GetCurrentHostOS() HostOS {
	switch goruntime.GOOS {
	case "linux":
		return HostLinux
	case "darwin":
		return HostMac
	case "windows":
		return HostWindows
	default:
		// Default to linux for unknown OS
		return HostLinux
	}
}

// CanRunOnCurrentHost returns true if the command can run on the current host OS
func (c *Command) CanRunOnCurrentHost() bool {
	currentOS := GetCurrentHostOS()
	for _, host := range c.WorksOn.Hosts {
		if host == currentOS {
			return true
		}
	}
	return false
}

// GetHostsString returns a comma-separated string of supported hosts
func (c *Command) GetHostsString() string {
	if len(c.WorksOn.Hosts) == 0 {
		return ""
	}
	hosts := make([]string, len(c.WorksOn.Hosts))
	for i, h := range c.WorksOn.Hosts {
		hosts[i] = string(h)
	}
	return strings.Join(hosts, ", ")
}

// GetDefaultRuntime returns the default runtime for this command (first in the list)
func (c *Command) GetDefaultRuntime() RuntimeMode {
	if len(c.Runtimes) == 0 {
		return RuntimeNative
	}
	return c.Runtimes[0]
}

// GetRuntimesString returns a formatted string of runtimes with default highlighted
func (c *Command) GetRuntimesString() string {
	if len(c.Runtimes) == 0 {
		return ""
	}
	runtimes := make([]string, len(c.Runtimes))
	for i, r := range c.Runtimes {
		if i == 0 {
			runtimes[i] = string(r) + "*" // Mark default with asterisk
		} else {
			runtimes[i] = string(r)
		}
	}
	return strings.Join(runtimes, ", ")
}

// IsRuntimeAllowed checks if the given runtime is in the allowed list
func (c *Command) IsRuntimeAllowed(runtime RuntimeMode) bool {
	for _, r := range c.Runtimes {
		if r == runtime {
			return true
		}
	}
	return false
}

// GetAllowedRuntimesString returns a comma-separated list of allowed runtimes
func (c *Command) GetAllowedRuntimesString() string {
	if len(c.Runtimes) == 0 {
		return ""
	}
	runtimes := make([]string, len(c.Runtimes))
	for i, r := range c.Runtimes {
		runtimes[i] = string(r)
	}
	return strings.Join(runtimes, ", ")
}

// HasDependencies returns true if the command has any dependencies
func (c *Command) HasDependencies() bool {
	if c.DependsOn == nil {
		return false
	}
	return len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0
}

// GetCommandDependencies returns the list of command dependency names
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
		// Inline script - use directly (multi-line strings from CUE are already handled)
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
	Dockerfile string `json:"dockerfile,omitempty"`
	// Image specifies a pre-built image to use instead of building from Dockerfile
	Image string `json:"image,omitempty"`
	// Volumes specifies volume mounts in "host:container" format
	Volumes []string `json:"volumes,omitempty"`
	// Ports specifies port mappings in "host:container" format
	Ports []string `json:"ports,omitempty"`
}

// Invowkfile represents the complete parsed invowkfile
type Invowkfile struct {
	// Version specifies the invowkfile schema version
	Version string `json:"version,omitempty"`
	// Description provides a summary of this invowkfile's purpose
	Description string `json:"description,omitempty"`
	// DefaultRuntime sets the default runtime for all commands
	DefaultRuntime RuntimeMode `json:"default_runtime,omitempty"`
	// DefaultShell overrides the default shell for native runtime
	DefaultShell string `json:"default_shell,omitempty"`
	// Container holds container-specific configuration
	Container ContainerConfig `json:"container,omitempty"`
	// Env contains environment variables applied to all commands
	Env map[string]string `json:"env,omitempty"`
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

	// Apply default runtime if runtimes list is empty
	if len(cmd.Runtimes) == 0 {
		cmd.Runtimes = []RuntimeMode{inv.DefaultRuntime}
	}

	// Validate container config for commands that support container runtime
	for _, rt := range cmd.Runtimes {
		if rt == RuntimeContainer {
			if err := inv.validateContainerConfig(); err != nil {
				return fmt.Errorf("command '%s': %w", cmd.Name, err)
			}
			break
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
	if inv.DefaultRuntime != "" {
		sb.WriteString(fmt.Sprintf("default_runtime: %q\n", inv.DefaultRuntime))
	}
	if inv.DefaultShell != "" {
		sb.WriteString(fmt.Sprintf("default_shell: %q\n", inv.DefaultShell))
	}

	// Container config
	if inv.Container.Dockerfile != "" || inv.Container.Image != "" || len(inv.Container.Volumes) > 0 || len(inv.Container.Ports) > 0 {
		sb.WriteString("\ncontainer: {\n")
		if inv.Container.Dockerfile != "" {
			sb.WriteString(fmt.Sprintf("\tdockerfile: %q\n", inv.Container.Dockerfile))
		}
		if inv.Container.Image != "" {
			sb.WriteString(fmt.Sprintf("\timage: %q\n", inv.Container.Image))
		}
		if len(inv.Container.Volumes) > 0 {
			sb.WriteString("\tvolumes: [\n")
			for _, v := range inv.Container.Volumes {
				sb.WriteString(fmt.Sprintf("\t\t%q,\n", v))
			}
			sb.WriteString("\t]\n")
		}
		if len(inv.Container.Ports) > 0 {
			sb.WriteString("\tports: [\n")
			for _, p := range inv.Container.Ports {
				sb.WriteString(fmt.Sprintf("\t\t%q,\n", p))
			}
			sb.WriteString("\t]\n")
		}
		sb.WriteString("}\n")
	}

	// Environment variables
	if len(inv.Env) > 0 {
		sb.WriteString("\nenv: {\n")
		for k, v := range inv.Env {
			sb.WriteString(fmt.Sprintf("\t%s: %q\n", k, v))
		}
		sb.WriteString("}\n")
	}

	// Commands
	sb.WriteString("\ncommands: [\n")
	for _, cmd := range inv.Commands {
		sb.WriteString("\t{\n")
		sb.WriteString(fmt.Sprintf("\t\tname: %q\n", cmd.Name))
		if cmd.Description != "" {
			sb.WriteString(fmt.Sprintf("\t\tdescription: %q\n", cmd.Description))
		}
		// Generate runtimes list
		if len(cmd.Runtimes) > 0 {
			sb.WriteString("\t\truntimes: [")
			for i, r := range cmd.Runtimes {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%q", r))
			}
			sb.WriteString("]\n")
		}

		// Handle multi-line scripts with CUE's multi-line string syntax
		if strings.Contains(cmd.Script, "\n") {
			sb.WriteString("\t\tscript: \"\"\"\n")
			for _, line := range strings.Split(cmd.Script, "\n") {
				sb.WriteString(fmt.Sprintf("\t\t\t%s\n", line))
			}
			sb.WriteString("\t\t\t\"\"\"\n")
		} else {
			sb.WriteString(fmt.Sprintf("\t\tscript: %q\n", cmd.Script))
		}

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
					sb.WriteString("\t\t\t\t{")
					sb.WriteString(fmt.Sprintf("path: %q", fp.Path))
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
		// works_on is required
		if len(cmd.WorksOn.Hosts) > 0 {
			sb.WriteString("\t\tworks_on: {\n")
			sb.WriteString("\t\t\thosts: [")
			for i, host := range cmd.WorksOn.Hosts {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%q", host))
			}
			sb.WriteString("]\n")
			sb.WriteString("\t\t}\n")
		}
		if cmd.HostSSH {
			sb.WriteString("\t\thost_ssh: true\n")
		}
		sb.WriteString("\t},\n")
	}
	sb.WriteString("]\n")

	return sb.String()
}
