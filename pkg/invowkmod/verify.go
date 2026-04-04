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
// Security boundary (L-02): This is a point-in-time integrity check, not a
// runtime enforcement gate. There is a TOCTOU gap between this verification
// and the actual module loading for execution — an attacker with filesystem
// write access could modify module files after verification passes. For
// stronger guarantees, combine with read-only filesystem mounts or container
// execution where the module tree is copied into an immutable layer.
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
	// The namespace format is "<module_id>@<version>" or an alias — extract
	// the module ID by stripping the version suffix so lookups by ModuleID
	// actually match (CT-03: fixes silent verification bypass).
	hashByID := make(map[ModuleID]lockHashEntry, len(lock.Modules))
	for key := range lock.Modules {
		mod := lock.Modules[key]
		if mod.ContentHash == "" {
			continue
		}
		moduleID := ExtractModuleIDFromNamespace(mod.Namespace)
		hashByID[moduleID] = lockHashEntry{
			key:  key,
			hash: mod.ContentHash,
		}
	}

	// If the lock file has no content hashes (v1.0 or empty), skip verification.
	// Security note: v1.0 lock files provide no tamper detection — the
	// LockFileChecker emits a SeverityMedium finding for v1.0 deprecation,
	// but discovery proceeds without hash verification for these modules.
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

// findLockEntryForModule matches a vendored module to its lock file entry
// using the module ID extracted from the namespace (CT-03).
func findLockEntryForModule(m *Module, hashByID map[ModuleID]lockHashEntry, _ *LockFile) (lockHashEntry, bool) {
	entry, ok := hashByID[m.Metadata.Module]
	return entry, ok
}

// ExtractModuleIDFromNamespace extracts the module ID from a namespace string.
// Namespace format is "<module_id>@<version>" or an alias. If no "@" separator
// is found, the entire namespace is returned as the module ID.
func ExtractModuleIDFromNamespace(ns ModuleNamespace) ModuleID {
	nsStr := string(ns)
	if moduleID, _, found := strings.Cut(nsStr, "@"); found {
		return ModuleID(moduleID) //goplint:ignore -- extracted from validated ModuleNamespace
	}
	return ModuleID(nsStr) //goplint:ignore -- identity conversion from validated ModuleNamespace
}
