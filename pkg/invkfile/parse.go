// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/internal/cueutil"
	"invowk-cli/pkg/invkmod"
)

//go:embed invkfile_schema.cue
var invkfileSchema string

// Module represents a loaded invowk module, ready for use.
// This is a type alias for invkmod.Module.
type Module = invkmod.Module

// Parse reads and parses an invkfile from the given path.
func Parse(path string) (*Invkfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkfile at %s: %w", path, err)
	}

	return ParseBytes(data, path)
}

// ParseBytes parses invkfile content from bytes.
// Uses cueutil.ParseAndDecodeString for the 3-step CUE parsing flow:
// compile schema → compile user data → validate and decode.
func ParseBytes(data []byte, path string) (*Invkfile, error) {
	result, err := cueutil.ParseAndDecodeString[Invkfile](
		invkfileSchema,
		data,
		"#Invkfile",
		cueutil.WithFilename(path),
	)
	if err != nil {
		return nil, err
	}

	inv := result.Value
	inv.FilePath = path

	// Validate and apply command defaults
	if err := inv.validate(); err != nil {
		return nil, err
	}

	return inv, nil
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
