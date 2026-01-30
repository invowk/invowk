// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	_ "embed"
	"fmt"
	"invowk-cli/pkg/invkmod"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
)

const (
	// DefaultMaxCUEFileSize is the maximum allowed size for CUE files (5MB).
	// This limit prevents OOM attacks from maliciously large configuration files.
	// The limit applies to invkfile.cue, invkmod.cue, and config.cue files.
	DefaultMaxCUEFileSize = 5 * 1024 * 1024
)

//go:embed invkfile_schema.cue
var invkfileSchema string

// Module represents a loaded invowk module, ready for use.
// This is a type alias for invkmod.Module.
type Module = invkmod.Module

// formatCUEError formats a CUE error with JSON path prefixes for clearer error messages.
// It extracts the path information from CUE errors and formats them consistently.
// For multiple errors, each error is listed on a separate line with its path.
func formatCUEError(err error, filePath string) error {
	if err == nil {
		return nil
	}

	// Extract all CUE errors
	cueErrors := errors.Errors(err)
	if len(cueErrors) == 0 {
		// Fallback: not a CUE error, return as-is
		return fmt.Errorf("%s: %w", filePath, err)
	}

	var lines []string
	for _, e := range cueErrors {
		// Get the path to the problematic field
		path := errors.Path(e)
		pathStr := formatPath(path)
		msg := e.Error()

		// Remove redundant path prefix from message if present
		// CUE sometimes includes the path in the message itself
		if pathStr != "" && strings.HasPrefix(msg, pathStr) {
			msg = strings.TrimPrefix(msg, pathStr)
			msg = strings.TrimPrefix(msg, ":")
			msg = strings.TrimSpace(msg)
		}

		if pathStr != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", pathStr, msg))
		} else {
			lines = append(lines, msg)
		}
	}

	if len(lines) == 1 {
		return fmt.Errorf("%s: %s", filePath, lines[0])
	}
	return fmt.Errorf("%s: validation failed:\n  %s", filePath, strings.Join(lines, "\n  "))
}

// formatPath converts a CUE path (slice of selectors) to a JSON-like path string.
// Example: [cmds, 0, implementations, 2, script] -> "cmds[0].implementations[2].script"
func formatPath(path []string) string {
	if len(path) == 0 {
		return ""
	}

	// Join with dots but handle array indices specially
	// The path from CUE is already in a good format like ["cmds", "0", "script"]
	// We want to produce "cmds[0].script"
	var result strings.Builder
	for i, part := range path {
		// Check if this looks like an array index (purely numeric)
		isIndex := true
		for _, c := range part {
			if c < '0' || c > '9' {
				isIndex = false
				break
			}
		}

		if isIndex && i > 0 {
			result.WriteString("[")
			result.WriteString(part)
			result.WriteString("]")
		} else {
			if i > 0 {
				result.WriteString(".")
			}
			result.WriteString(part)
		}
	}

	return result.String()
}

// Parse reads and parses an invkfile from the given path.
func Parse(path string) (*Invkfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkfile at %s: %w", path, err)
	}

	return ParseBytes(data, path)
}

// ParseBytes parses invkfile content from bytes.
func ParseBytes(data []byte, path string) (*Invkfile, error) {
	// Early file size check to prevent OOM attacks from large files
	if len(data) > DefaultMaxCUEFileSize {
		return nil, fmt.Errorf("%s: file size %d bytes exceeds maximum %d bytes",
			path, len(data), DefaultMaxCUEFileSize)
	}

	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(invkfileSchema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
	}

	// Compile the user's invkfile
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return nil, formatCUEError(userValue.Err(), path)
	}

	// Unify with schema to validate
	schema := schemaValue.LookupPath(cue.ParsePath("#Invkfile"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, formatCUEError(err, path)
	}

	// Decode into struct
	var inv Invkfile
	if err := unified.Decode(&inv); err != nil {
		return nil, formatCUEError(err, path)
	}

	inv.FilePath = path

	// Validate and apply command defaults
	if err := inv.validate(); err != nil {
		return nil, err
	}

	return &inv, nil
}

// ParseInvkmod reads and parses module metadata from invkmod.cue at the given path.
// This is a wrapper for invkmod.ParseInvkmod.
func ParseInvkmod(path string) (*Invkmod, error) {
	return invkmod.ParseInvkmod(path)
}

// ParseInvkmodBytes parses module metadata content from bytes.
// This is a wrapper for invkmod.ParseInvkmodBytes.
func ParseInvkmodBytes(data []byte, path string) (*Invkmod, error) {
	return invkmod.ParseInvkmodBytes(data, path)
}

// ParseModule reads and parses a complete module from the given module directory.
// It expects:
// - invkmod.cue (required): Module metadata (module name, version, description, requires)
// - invkfile.cue (optional): Command definitions (for library-only modules)
//
// The modulePath should be the path to the module directory (ending in .invkmod).
// Returns a Module with Metadata from invkmod.cue and Commands from invkfile.cue.
// Note: Commands is stored as any but is always *Invkfile when present.
// Use GetModuleCommands() for typed access.
func ParseModule(modulePath string) (*Module, error) {
	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	invkfilePath := filepath.Join(modulePath, "invkfile.cue")

	// Parse invkmod.cue (required)
	meta, err := ParseInvkmod(invkmodPath)
	if err != nil {
		return nil, fmt.Errorf("module at %s: %w", modulePath, err)
	}

	// Create result
	result := &Module{
		Metadata: meta,
		Path:     modulePath,
	}

	// Parse invkfile.cue (optional - may be a library-only module)
	if _, statErr := os.Stat(invkfilePath); statErr == nil {
		data, readErr := os.ReadFile(invkfilePath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read invkfile at %s: %w", invkfilePath, readErr)
		}

		inv, parseErr := ParseBytes(data, invkfilePath)
		if parseErr != nil {
			return nil, parseErr
		}

		// Set metadata reference and module path
		inv.Metadata = meta
		inv.ModulePath = modulePath

		result.Commands = inv
	} else {
		result.IsLibraryOnly = true
	}

	return result, nil
}

// GetModuleCommands extracts the Invkfile from a Module.
// Returns nil if the module has no commands (library-only module) or if m is nil.
func GetModuleCommands(m *Module) *Invkfile {
	if m == nil || m.Commands == nil {
		return nil
	}
	if inv, ok := m.Commands.(*Invkfile); ok {
		return inv
	}
	return nil
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
