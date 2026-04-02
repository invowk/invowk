// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

type lockHashEntry struct {
	key  ModuleRefKey
	hash ContentHash
}

// VerifyVendoredModuleHashes checks that vendored modules in invowk_modules/
// match the content hashes recorded in the lock file. This detects tampering
// of vendored module content (e.g., via malicious commits or CI artifact cache
// poisoning) before the modules are loaded for execution.
//
// Returns nil if all hashes match, no vendored modules exist, or no lock file
// exists. Returns a ContentHashMismatchError on the first mismatched module.
func VerifyVendoredModuleHashes(modulePath types.FilesystemPath) error {
	vendorDir := GetVendoredModulesDir(modulePath)
	vendorDirStr := string(vendorDir)

	// No vendor directory is common and not an error.
	if _, err := os.Stat(vendorDirStr); os.IsNotExist(err) {
		return nil
	}

	// Load the lock file for expected content hashes.
	lockPath := filepath.Join(string(modulePath), LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("loading lock file for hash verification: %w", err)
	}

	// Build a lookup from module ID to lock entry for matching vendored modules.
	hashByID := make(map[ModuleID]lockHashEntry, len(lock.Modules))
	for key := range lock.Modules {
		mod := lock.Modules[key]
		if mod.ContentHash == "" {
			continue
		}
		hashByID[ModuleID(mod.Namespace)] = lockHashEntry{
			key:  key,
			hash: mod.ContentHash,
		}
	}

	// If the lock file has no content hashes (v1.0 or empty), skip verification.
	if len(hashByID) == 0 {
		return nil
	}

	// Scan vendored modules and verify hashes.
	entries, err := os.ReadDir(vendorDirStr)
	if err != nil {
		return fmt.Errorf("reading vendor directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ModuleSuffix) {
			continue
		}

		entryPath := filepath.Join(vendorDirStr, entry.Name())
		vendoredModPath := types.FilesystemPath(entryPath) //goplint:ignore -- filepath.Join from OS-listed entry

		// Load the module to get its ID/namespace for matching.
		m, loadErr := Load(vendoredModPath)
		if loadErr != nil {
			continue // Skip invalid modules (discovery will also skip them)
		}

		// Try to find a matching lock entry by namespace or module ID.
		lockEntry, found := findLockEntryForModule(m, hashByID, lock)
		if !found {
			continue // No lock entry — nothing to verify
		}

		actualHash, hashErr := computeModuleHash(entryPath)
		if hashErr != nil {
			return fmt.Errorf("computing hash for vendored module %s: %w", entry.Name(), hashErr)
		}

		if actualHash != lockEntry.hash {
			return &ContentHashMismatchError{
				ModuleKey: lockEntry.key,
				Expected:  lockEntry.hash,
				Actual:    actualHash,
			}
		}
	}

	return nil
}

// findLockEntryForModule matches a vendored module to its lock file entry.
// Tries namespace match first, then falls back to module ID matching.
func findLockEntryForModule(m *Module, hashByID map[ModuleID]lockHashEntry, lock *LockFile) (lockHashEntry, bool) {
	// Match by module ID in the hashByID map (keyed by namespace).
	if entry, ok := hashByID[m.Metadata.Module]; ok {
		return entry, true
	}

	// Fallback: match by scanning lock entries for matching module ID.
	for key := range lock.Modules {
		mod := lock.Modules[key]
		if mod.ContentHash == "" {
			continue
		}
		// The lock entry's namespace may be "name@version" or an alias.
		// Check if the vendored module's ID matches via the lock entry's git URL pattern.
		if mod.Namespace != "" && ModuleID(mod.Namespace) == m.Metadata.Module {
			return lockHashEntry{key: key, hash: mod.ContentHash}, true
		}
	}

	return lockHashEntry{}, false
}
