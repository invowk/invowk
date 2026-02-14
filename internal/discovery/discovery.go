// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ErrNoInvowkfileFound is returned when no invowkfile.cue is found in any search path.
// Callers can check for this error using errors.Is(err, ErrNoInvowkfileFound).
var ErrNoInvowkfileFound = errors.New("no invowkfile found")

type (
	// ModuleCollisionError is returned when two modules have the same module identifier.
	ModuleCollisionError struct {
		ModuleID     string
		FirstSource  string
		SecondSource string
	}

	// Discovery is the stateless entry point for file discovery, command set building,
	// and single-command lookup. Each method creates fresh state from the config rather
	// than caching results, which is appropriate for a short-lived CLI process.
	Discovery struct {
		cfg *config.Config
	}
)

// Error implements the error interface.
func (e *ModuleCollisionError) Error() string {
	return fmt.Sprintf(
		"module name collision: '%s' defined in both:\n"+
			"  - %s\n"+
			"  - %s\n\n"+
			"Add an alias to disambiguate in your config:\n"+
			"  includes: [{path: %q, alias: \"<new-alias>\"}]",
		e.ModuleID, e.FirstSource, e.SecondSource,
		e.SecondSource)
}

// Unwrap returns nil; ModuleCollisionError has no underlying cause.
func (e *ModuleCollisionError) Unwrap() error { return nil }

// New creates a new Discovery instance
func New(cfg *config.Config) *Discovery {
	return &Discovery{cfg: cfg}
}

// LoadAll parses all discovered files into Invowkfile structs. Library-only modules
// (those without an invowkfile.cue) are skipped because they provide scripts and files
// for other modules via `requires` but contribute no commands. Module metadata is
// reattached to parsed Invowkfiles so downstream scope/dependency checks can identify
// the owning module.
func (d *Discovery) LoadAll() ([]*DiscoveredFile, error) {
	files, _, err := d.loadAllWithDiagnostics()
	if err != nil {
		return nil, err
	}

	return files, nil
}

// LoadFirst loads the first valid invowkfile found (respecting precedence).
// This method is currently used only in tests to verify single-file
// precedence behavior; production code uses LoadAll or the command set methods.
func (d *Discovery) LoadFirst() (*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrNoInvowkfileFound
	}

	file := files[0]
	var inv *invowkfile.Invowkfile
	var parseErr error

	if file.Module != nil {
		// Library-only modules can be discovered first; they are valid, but have
		// no command-bearing invowkfile to load.
		if file.Module.IsLibraryOnly || file.Path == "" {
			file.Invowkfile = nil
			return file, nil
		}

		inv, parseErr = invowkfile.Parse(file.Path)
		if parseErr == nil {
			inv.Metadata = file.Module.Metadata
			inv.ModulePath = file.Module.Path
		}
	} else {
		inv, parseErr = invowkfile.Parse(file.Path)
	}

	if parseErr != nil {
		file.Error = parseErr
		return file, parseErr
	}

	file.Invowkfile = inv
	return file, nil
}

// CheckModuleCollisions checks for module ID collisions among discovered files.
// It returns a ModuleCollisionError if two modules have the same module identifier
// and neither has an alias configured via includes.
func (d *Discovery) CheckModuleCollisions(files []*DiscoveredFile) error {
	// Map effective module IDs to their source paths for collision detection.
	moduleSources := make(map[string]string)

	for _, file := range files {
		if file.Error != nil || file.Invowkfile == nil {
			continue
		}

		moduleID := d.GetEffectiveModuleID(file)
		if moduleID == "" {
			continue
		}

		// Use the module directory path (not the invowkfile inside it) so
		// the error message shows the path users need for their includes config.
		sourcePath := file.Path
		if file.Module != nil {
			sourcePath = file.Module.Path
		}

		if existingSource, exists := moduleSources[moduleID]; exists {
			return &ModuleCollisionError{
				ModuleID:     moduleID,
				FirstSource:  existingSource,
				SecondSource: sourcePath,
			}
		}

		moduleSources[moduleID] = sourcePath
	}

	return nil
}

// GetEffectiveModuleID returns the effective module ID for a file, considering
// aliases from the includes config. For module-backed files, if the module's
// directory matches an include entry with an alias, the alias overrides the
// module's declared ID.
func (d *Discovery) GetEffectiveModuleID(file *DiscoveredFile) string {
	if file.Invowkfile == nil {
		return ""
	}

	moduleID := file.Invowkfile.GetModule()

	// Module-backed files can have aliases configured in includes.
	// Match against Module.Path (the module directory), not file.Path
	// (the invowkfile inside the module), because includes reference
	// module directories.
	if file.Module != nil {
		if alias := d.getAliasForModulePath(file.Module.Path); alias != "" {
			return alias
		}
	}

	return moduleID
}

// getAliasForModulePath looks up an alias for the given module directory path
// from the includes config. Paths are normalized with filepath.Clean before
// comparison to handle trailing slashes and redundant separators. Returns the
// alias if found, or empty string if no alias is configured.
func (d *Discovery) getAliasForModulePath(modulePath string) string {
	if d.cfg == nil {
		return ""
	}

	cleanPath := filepath.Clean(modulePath)

	for _, inc := range d.cfg.Includes {
		if inc.Alias != "" && filepath.Clean(inc.Path) == cleanPath {
			return inc.Alias
		}
	}

	return ""
}

// loadAllWithDiagnostics parses discovered files and returns non-fatal
// discovery diagnostics (e.g., skipped includes/modules) alongside files.
func (d *Discovery) loadAllWithDiagnostics() ([]*DiscoveredFile, []Diagnostic, error) {
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		return nil, diagnostics, err
	}

	for _, file := range files {
		var inv *invowkfile.Invowkfile
		var parseErr error

		if file.Module != nil {
			// Library-only modules provide scripts and files for other modules to
			// reference via `requires`, but don't contribute their own command
			// definitions to the CLI command tree.
			if file.Module.IsLibraryOnly || file.Path == "" {
				continue
			}

			// Parse module invowkfile.cue and reattach module metadata so downstream
			// logic (scope/dependency checks) can treat it as module-backed input.
			inv, parseErr = invowkfile.Parse(file.Path)
			if parseErr == nil {
				inv.Metadata = file.Module.Metadata
				inv.ModulePath = file.Module.Path
			}
		} else {
			inv, parseErr = invowkfile.Parse(file.Path)
		}

		if parseErr != nil {
			file.Error = parseErr
		} else {
			file.Invowkfile = inv
		}
	}

	// Detect module ID collisions after all files are parsed. This catches
	// two modules that declare the same module identifier and neither has
	// an alias to disambiguate. Callers receive a ModuleCollisionError with
	// actionable remediation (add an alias in the includes config).
	if err := d.CheckModuleCollisions(files); err != nil {
		return nil, diagnostics, err
	}

	return files, diagnostics, nil
}
