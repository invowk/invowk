// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"os"
	"path/filepath"
)

// moduleError wraps errors from vendor module operations for consistent error messages.
type moduleError struct {
	op  string
	err error
}

func (e *moduleError) Error() string {
	return "failed to " + e.op + ": " + e.err.Error()
}

func (e *moduleError) Unwrap() error {
	return e.err
}

// GetVendoredModulesDir returns the path to the vendored modules directory for a given module.
// Returns the path whether or not the directory exists.
func GetVendoredModulesDir(modulePath string) string {
	return filepath.Join(modulePath, VendoredModulesDir)
}

// HasVendoredModules checks if a module has vendored dependencies.
// Returns true only if the invk_modules/ directory exists AND contains at least one valid module.
func HasVendoredModules(modulePath string) bool {
	modules, err := ListVendoredModules(modulePath)
	if err != nil {
		return false
	}
	return len(modules) > 0
}

// ListVendoredModules returns a list of vendored modules in the given module directory.
// Returns nil if no invk_modules/ directory exists or it's empty.
func ListVendoredModules(modulePath string) ([]*Module, error) {
	vendorDir := GetVendoredModulesDir(modulePath)

	// Check if vendor directory exists
	info, err := os.Stat(vendorDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &moduleError{op: "stat vendor directory", err: err}
	}
	if !info.IsDir() {
		return nil, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		return nil, &moduleError{op: "read vendor directory", err: err}
	}

	var modules []*Module
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it's a module
		entryPath := filepath.Join(vendorDir, entry.Name())
		if !IsModule(entryPath) {
			continue
		}

		// Load the module
		m, err := Load(entryPath)
		if err != nil {
			// Skip invalid modules
			continue
		}

		modules = append(modules, m)
	}

	return modules, nil
}
