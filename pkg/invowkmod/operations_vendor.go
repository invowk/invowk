// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type (
	// moduleError wraps errors from vendor module operations for consistent error messages.
	moduleError struct {
		op  string
		err error
	}

	// VendorOptions configures a VendorModules operation.
	VendorOptions struct {
		// ModulePath is the absolute path to the module being vendored.
		ModulePath string
		// Modules are the resolved modules to copy into invowk_modules/.
		Modules []*ResolvedModule
		// Prune removes vendored modules not present in the Modules list.
		Prune bool
	}

	// VendorResult contains the outcome of a VendorModules operation.
	VendorResult struct {
		// Vendored lists the modules copied to invowk_modules/.
		Vendored []VendoredEntry
		// Pruned lists directory names removed during pruning.
		Pruned []string
		// VendorDir is the absolute path to the invowk_modules/ directory.
		VendorDir string
	}

	// VendoredEntry describes a single module copied to the vendor directory.
	VendoredEntry struct {
		// Namespace is the module's command namespace (e.g., "tools@1.2.3").
		Namespace string
		// SourcePath is the cache path the module was copied from.
		SourcePath string
		// VendorPath is the destination path in invowk_modules/.
		VendorPath string
	}
)

func (e *moduleError) Error() string {
	return "failed to " + e.op + ": " + e.err.Error()
}

func (e *moduleError) Unwrap() error {
	return e.err
}

// VendorModules copies resolved modules into the invowk_modules/ directory of the
// target module. If Prune is set, vendored modules not in the Modules list are removed.
// The operation is fail-fast: if any module fails to resolve or copy, the entire
// operation fails, leaving a partially-vendored directory rather than silently corrupt.
func VendorModules(opts VendorOptions) (*VendorResult, error) {
	vendorDir := GetVendoredModulesDir(opts.ModulePath)

	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create vendor directory: %w", err)
	}

	result := &VendorResult{
		VendorDir: vendorDir,
	}

	// Track expected module directory basenames for pruning.
	expectedDirs := make(map[string]bool)

	for _, mod := range opts.Modules {
		// Locate the .invowkmod directory within the cache path.
		moduleDir, _, err := findModuleInDir(mod.CachePath)
		if err != nil {
			return nil, fmt.Errorf("failed to locate module in cache path %s: %w", mod.CachePath, err)
		}

		dirBase := filepath.Base(moduleDir)
		destPath := filepath.Join(vendorDir, dirBase)
		expectedDirs[dirBase] = true

		// Remove any previous vendored copy so we get a clean state.
		if err := os.RemoveAll(destPath); err != nil {
			return nil, fmt.Errorf("failed to remove existing vendored module at %s: %w", destPath, err)
		}

		if err := copyDir(moduleDir, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy module to %s: %w", destPath, err)
		}

		result.Vendored = append(result.Vendored, VendoredEntry{
			Namespace:  mod.Namespace,
			SourcePath: moduleDir,
			VendorPath: destPath,
		})
	}

	// Prune vendored modules that are no longer in the resolved set.
	if opts.Prune {
		pruned, err := pruneVendorDir(vendorDir, expectedDirs)
		if err != nil {
			return nil, fmt.Errorf("failed to prune vendor directory: %w", err)
		}
		result.Pruned = pruned
	}

	return result, nil
}

// GetVendoredModulesDir returns the path to the vendored modules directory for a given module.
// Returns the path whether or not the directory exists.
func GetVendoredModulesDir(modulePath string) string {
	return filepath.Join(modulePath, VendoredModulesDir)
}

// HasVendoredModules checks if a module has vendored dependencies.
// Returns true only if the invowk_modules/ directory exists AND contains at least one valid module.
func HasVendoredModules(modulePath string) bool {
	modules, err := ListVendoredModules(modulePath)
	if err != nil {
		return false
	}
	return len(modules) > 0
}

// ListVendoredModules returns a list of vendored modules in the given module directory.
// Returns nil if no invowk_modules/ directory exists or it's empty.
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

// pruneVendorDir removes *.invowkmod entries from vendorDir that are not in expectedDirs.
func pruneVendorDir(vendorDir string, expectedDirs map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		return nil, err
	}

	var pruned []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ModuleSuffix) {
			continue
		}
		if expectedDirs[entry.Name()] {
			continue
		}

		entryPath := filepath.Join(vendorDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return pruned, fmt.Errorf("failed to remove stale vendored module %s: %w", entry.Name(), err)
		}
		pruned = append(pruned, entry.Name())
	}

	return pruned, nil
}
