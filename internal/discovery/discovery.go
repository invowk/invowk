// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"

	"invowk-cli/internal/config"
	"invowk-cli/pkg/invkfile"
)

// ErrNoInvkfileFound is returned when no invkfile.cue is found in any search path.
// Callers can check for this error using errors.Is(err, ErrNoInvkfileFound).
var ErrNoInvkfileFound = errors.New("no invkfile found")

type (
	// ModuleCollisionError is returned when two modules have the same module identifier.
	ModuleCollisionError struct {
		ModuleID     string
		FirstSource  string
		SecondSource string
	}

	// Discovery handles finding invkfiles
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
			"Use an alias to disambiguate:\n"+
			"  invowk module alias %q <new-alias>\n"+
			"  invowk module alias %q <new-alias>",
		e.ModuleID, e.FirstSource, e.SecondSource,
		e.FirstSource, e.SecondSource)
}

// New creates a new Discovery instance
func New(cfg *config.Config) *Discovery {
	return &Discovery{cfg: cfg}
}

// LoadAll parses all discovered files
func (d *Discovery) LoadAll() ([]*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		var inv *invkfile.Invkfile
		var parseErr error

		if file.Module != nil {
			// Use module-aware parsing
			parsed, err := invkfile.ParseModule(file.Module.Path)
			if err != nil {
				parseErr = err
			} else {
				inv = invkfile.GetModuleCommands(parsed)
			}
		} else {
			inv, parseErr = invkfile.Parse(file.Path)
		}

		if parseErr != nil {
			file.Error = parseErr
		} else {
			file.Invkfile = inv
		}
	}

	return files, nil
}

// LoadFirst loads the first valid invkfile found (respecting precedence)
func (d *Discovery) LoadFirst() (*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrNoInvkfileFound
	}

	file := files[0]
	var inv *invkfile.Invkfile
	var parseErr error

	if file.Module != nil {
		// Use module-aware parsing
		parsed, err := invkfile.ParseModule(file.Module.Path)
		if err != nil {
			parseErr = err
		} else {
			inv = invkfile.GetModuleCommands(parsed)
		}
	} else {
		inv, parseErr = invkfile.Parse(file.Path)
	}

	if parseErr != nil {
		file.Error = parseErr
		return file, parseErr
	}

	file.Invkfile = inv
	return file, nil
}

// CheckModuleCollisions checks for module ID collisions among discovered files.
// It returns a ModuleCollisionError if two modules have the same module identifier
// and neither has an alias configured.
func (d *Discovery) CheckModuleCollisions(files []*DiscoveredFile) error {
	// Map module IDs to their sources (considering aliases)
	moduleSources := make(map[string]string)

	for _, file := range files {
		if file.Error != nil || file.Invkfile == nil {
			continue
		}

		moduleID := file.Invkfile.GetModule()
		if moduleID == "" {
			continue
		}

		// Check if there's an alias configured for this path
		if d.cfg != nil && d.cfg.ModuleAliases != nil {
			if alias, ok := d.cfg.ModuleAliases[file.Path]; ok {
				moduleID = alias
			}
		}

		// Check for collision
		if existingSource, exists := moduleSources[moduleID]; exists {
			return &ModuleCollisionError{
				ModuleID:     moduleID,
				FirstSource:  existingSource,
				SecondSource: file.Path,
			}
		}

		moduleSources[moduleID] = file.Path
	}

	return nil
}

// GetEffectiveModuleID returns the effective module ID for a file, considering aliases.
func (d *Discovery) GetEffectiveModuleID(file *DiscoveredFile) string {
	if file.Invkfile == nil {
		return ""
	}

	moduleID := file.Invkfile.GetModule()

	// Check if there's an alias configured for this path
	if d.cfg != nil && d.cfg.ModuleAliases != nil {
		if alias, ok := d.cfg.ModuleAliases[file.Path]; ok {
			return alias
		}
	}

	return moduleID
}
