// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/invowkmod"
)

var (
	//go:embed invowkfile_schema.cue
	invowkfileSchema string

	// Ensure Invowkfile satisfies the typed module command contract.
	_ invowkmod.ModuleCommands = (*Invowkfile)(nil)
)

// Module represents a loaded invowk module, ready for use.
// This is a type alias for invowkmod.Module.
type Module = invowkmod.Module

// Parse reads and parses an invowkfile from the given path.
func Parse(path FilesystemPath) (*Invowkfile, error) {
	pathStr := string(path)
	data, err := os.ReadFile(pathStr)
	if err != nil {
		return nil, fmt.Errorf("failed to read invowkfile at %s: %w", path, err)
	}

	return ParseBytes(data, pathStr)
}

// ParseBytes parses invowkfile content from bytes.
// Uses cueutil.ParseAndDecodeString for the 3-step CUE parsing flow:
// compile schema → compile user data → validate and decode.
func ParseBytes(data []byte, path string) (*Invowkfile, error) {
	result, err := cueutil.ParseAndDecodeString[Invowkfile](
		invowkfileSchema,
		data,
		"#Invowkfile",
		cueutil.WithFilename(path),
	)
	if err != nil {
		return nil, err
	}

	inv := result.Value
	inv.FilePath = FilesystemPath(path)

	// Validate and collect all errors
	if errs := inv.Validate(); len(errs) > 0 {
		// Return ValidationErrors which implements error interface
		return nil, errs
	}

	return inv, nil
}

// ParseInvowkmod reads and parses module metadata from invowkmod.cue at the given path.
// This is a wrapper for invowkmod.ParseInvowkmod.
func ParseInvowkmod(path FilesystemPath) (*Invowkmod, error) {
	return invowkmod.ParseInvowkmod(path)
}

// ParseInvowkmodBytes parses module metadata content from bytes.
// This is a wrapper for invowkmod.ParseInvowkmodBytes.
func ParseInvowkmodBytes(data []byte, path FilesystemPath) (*Invowkmod, error) {
	return invowkmod.ParseInvowkmodBytes(data, path)
}

// ParseModule reads and parses a complete module from the given module directory.
// It loads invowkmod.cue for module metadata (name, version, requires) and optionally
// invowkfile.cue for command definitions. Modules without invowkfile.cue are marked as
// library-only — they provide scripts and files for other modules to reference via
// `requires` but contribute no commands to the CLI.
//
// The modulePath should be the path to the module directory (ending in .invowkmod).
// Returns a Module with Metadata from invowkmod.cue and Commands from invowkfile.cue.
func ParseModule(modulePath FilesystemPath) (*Module, error) {
	modulePathStr := string(modulePath)
	invowkmodPath := filepath.Join(modulePathStr, "invowkmod.cue")
	invowkfilePath := filepath.Join(modulePathStr, "invowkfile.cue")

	// Parse invowkmod.cue (required)
	meta, err := ParseInvowkmod(FilesystemPath(invowkmodPath))
	if err != nil {
		return nil, fmt.Errorf("module at %s: %w", modulePath, err)
	}

	// Create result
	result := &Module{
		Metadata: meta,
		Path:     modulePath,
	}

	// Parse invowkfile.cue (optional - may be a library-only module)
	if _, statErr := os.Stat(invowkfilePath); statErr == nil {
		data, readErr := os.ReadFile(invowkfilePath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read invowkfile at %s: %w", invowkfilePath, readErr)
		}

		inv, parseErr := ParseBytes(data, invowkfilePath)
		if parseErr != nil {
			return nil, parseErr
		}

		// Attach local metadata snapshot and module path
		inv.Metadata = NewModuleMetadataFromInvowkmod(meta)
		inv.ModulePath = FilesystemPath(modulePath)

		result.Commands = inv
	} else {
		result.IsLibraryOnly = true
	}

	return result, nil
}

// ParseEnvInheritMode parses a string into an EnvInheritMode.
func ParseEnvInheritMode(value string) (EnvInheritMode, error) {
	if value == "" {
		return "", nil
	}
	mode := EnvInheritMode(value)
	if isValid, errs := mode.IsValid(); !isValid {
		return "", errs[0]
	}
	return mode, nil
}

// ParseRuntimeMode parses a string into a RuntimeMode.
// Returns zero value ("") for empty input, which serves as the "no override" sentinel.
func ParseRuntimeMode(value string) (RuntimeMode, error) {
	if value == "" {
		return "", nil
	}
	mode := RuntimeMode(value)
	if isValid, errs := mode.IsValid(); !isValid {
		return "", errs[0]
	}
	return mode, nil
}
