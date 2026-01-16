// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"invowk-cli/pkg/invkpack"
)

//go:embed invkfile_schema.cue
var invkfileSchema string

// Pack represents a loaded invowk pack, ready for use.
// This is a type alias for invkpack.Pack.
type Pack = invkpack.Pack

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

// ParseInvkpack reads and parses pack metadata from invkpack.cue at the given path.
// This is a wrapper for invkpack.ParseInvkpack.
func ParseInvkpack(path string) (*Invkpack, error) {
	return invkpack.ParseInvkpack(path)
}

// ParseInvkpackBytes parses pack metadata content from bytes.
// This is a wrapper for invkpack.ParseInvkpackBytes.
func ParseInvkpackBytes(data []byte, path string) (*Invkpack, error) {
	return invkpack.ParseInvkpackBytes(data, path)
}

// ParsePack reads and parses a complete pack from the given pack directory.
// It expects:
// - invkpack.cue (required): Pack metadata (pack name, version, description, requires)
// - invkfile.cue (optional): Command definitions (for library-only packs)
//
// The packPath should be the path to the pack directory (ending in .invkpack).
// Returns a Pack with Metadata from invkpack.cue and Commands from invkfile.cue.
// Note: Commands is stored as any but is always *Invkfile when present.
// Use GetPackCommands() for typed access.
func ParsePack(packPath string) (*Pack, error) {
	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	invkfilePath := filepath.Join(packPath, "invkfile.cue")

	// Parse invkpack.cue (required)
	meta, err := ParseInvkpack(invkpackPath)
	if err != nil {
		return nil, fmt.Errorf("pack at %s: %w", packPath, err)
	}

	// Create result
	result := &Pack{
		Metadata: meta,
		Path:     packPath,
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
	} else {
		result.IsLibraryOnly = true
	}

	return result, nil
}

// GetPackCommands extracts the Invkfile from a Pack.
// Returns nil if the pack has no commands (library-only pack) or if p is nil.
func GetPackCommands(p *Pack) *Invkfile {
	if p == nil || p.Commands == nil {
		return nil
	}
	if inv, ok := p.Commands.(*Invkfile); ok {
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
