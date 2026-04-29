// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// VendoredHashMatched means the vendored module hash matches one lock entry.
	VendoredHashMatched VendoredHashStatus = "matched"
	// VendoredHashMissing means no lock entry matched the vendored module ID.
	VendoredHashMissing VendoredHashStatus = "missing"
	// VendoredHashAmbiguous means multiple lock entries matched the vendored module ID.
	VendoredHashAmbiguous VendoredHashStatus = "ambiguous"
	// VendoredHashMismatch means the computed vendored hash differs from the lock entry.
	VendoredHashMismatch VendoredHashStatus = "mismatch"
	// VendoredHashUnavailable means the matching lock entry lacks a hash or the hash could not be computed.
	VendoredHashUnavailable VendoredHashStatus = "unavailable"
)

type (
	lockHashEntry struct {
		key  ModuleRefKey
		hash ContentHash
	}

	// VendoredHashStatus classifies a vendored module integrity evaluation.
	VendoredHashStatus string

	// VendoredHashEvaluation is the policy result for matching one vendored
	// module to its lock-file content hash.
	VendoredHashEvaluation struct {
		Status    VendoredHashStatus
		ModuleID  ModuleID
		ModuleKey ModuleRefKey
		LockKeys  []ModuleRefKey
		Expected  ContentHash
		Actual    ContentHash
		Err       error
	}
)

// String returns the string representation of the VendoredHashStatus.
func (s VendoredHashStatus) String() string { return string(s) }

// Validate returns nil when the status is one of the known vendored hash states.
func (s VendoredHashStatus) Validate() error {
	switch s {
	case VendoredHashMatched, VendoredHashMissing, VendoredHashAmbiguous, VendoredHashMismatch, VendoredHashUnavailable:
		return nil
	default:
		return fmt.Errorf("invalid vendored hash status %q", s)
	}
}

// Validate returns nil when the evaluation's typed fields are valid.
func (e VendoredHashEvaluation) Validate() error {
	var errs []error
	if err := e.Status.Validate(); err != nil {
		errs = append(errs, err)
	}
	if e.ModuleID != "" {
		if err := e.ModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.ModuleKey != "" {
		if err := e.ModuleKey.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, key := range e.LockKeys {
		if err := key.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.Expected != "" {
		if err := e.Expected.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.Actual != "" {
		if err := e.Actual.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

	// If the lock file has no content hashes (v1.0 or empty), skip verification.
	// Security note: v1.0 lock files provide no tamper detection — the
	// LockFileChecker emits a SeverityMedium finding for v1.0 deprecation,
	// but discovery proceeds without hash verification for these modules.
	if len(lock.ContentHashes()) == 0 {
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

		evaluation := EvaluateVendoredModuleHash(lock, m)
		switch evaluation.Status {
		case VendoredHashMatched, VendoredHashMissing:
			continue
		case VendoredHashAmbiguous:
			return fmt.Errorf("ambiguous lock file entries for vendored module %s: %s", m.Metadata.Module, joinModuleRefKeys(evaluation.LockKeys))
		case VendoredHashUnavailable:
			return fmt.Errorf("computing hash for vendored module %s: %w", entry.Name(), evaluation.Err)
		case VendoredHashMismatch:
			return &ContentHashMismatchError{
				ModuleKey: evaluation.ModuleKey,
				Expected:  evaluation.Expected,
				Actual:    evaluation.Actual,
			}
		default:
			return fmt.Errorf("unknown vendored hash status %q for module %s", evaluation.Status, m.Metadata.Module)
		}
	}

	return nil
}

// EvaluateVendoredModuleHash matches a vendored module to its lock file entry
// using the resolved module ID and computes the vendored content-hash status.
func EvaluateVendoredModuleHash(lock *LockFile, m *Module) VendoredHashEvaluation {
	if m == nil || m.Metadata == nil {
		return VendoredHashEvaluation{Status: VendoredHashMissing}
	}

	moduleID := m.Metadata.Module
	entries := lockHashEntriesForModule(lock, moduleID)
	evaluation := VendoredHashEvaluation{Status: VendoredHashMissing, ModuleID: moduleID}
	if len(entries) == 0 {
		return evaluation
	}

	evaluation.LockKeys = lockEntryKeys(entries)
	if len(entries) > 1 {
		evaluation.Status = VendoredHashAmbiguous
		return evaluation
	}

	entry := entries[0]
	return EvaluateModuleContentHash(entry.key, moduleID, m.Path, entry.hash)
}

// EvaluateModuleContentHash computes modulePath's hash and compares it to the
// expected hash from module dependency metadata.
func EvaluateModuleContentHash(moduleKey ModuleRefKey, moduleID ModuleID, modulePath types.FilesystemPath, expected ContentHash) VendoredHashEvaluation {
	evaluation := VendoredHashEvaluation{
		Status:    VendoredHashUnavailable,
		ModuleID:  moduleID,
		ModuleKey: moduleKey,
		Expected:  expected,
	}
	if expected == "" {
		return evaluation
	}

	actualHash, err := computeModuleHash(string(modulePath))
	if err != nil {
		evaluation.Err = err
		return evaluation
	}

	evaluation.Actual = actualHash
	if actualHash != expected {
		evaluation.Status = VendoredHashMismatch
		return evaluation
	}

	evaluation.Status = VendoredHashMatched
	return evaluation
}

func lockHashEntriesForModule(lock *LockFile, moduleID ModuleID) []lockHashEntry {
	if lock == nil {
		return nil
	}

	var entries []lockHashEntry
	for key := range lock.Modules {
		mod := lock.Modules[key]
		if mod.IdentityModuleID() != moduleID {
			continue
		}
		entries = append(entries, lockHashEntry{key: key, hash: mod.ContentHash})
	}
	return entries
}

func lockEntryKeys(entries []lockHashEntry) []ModuleRefKey {
	keys := make([]ModuleRefKey, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.key)
	}
	return keys
}

//plint:render
func joinModuleRefKeys(keys []ModuleRefKey) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, string(key))
	}
	return strings.Join(parts, ", ")
}

// IdentityModuleID returns the stable module identity for hash verification.
// It prefers the lock file's persisted ModuleID and falls back to the historical
// namespace-derived identity for older lock files.
func (m LockedModule) IdentityModuleID() ModuleID {
	if m.ModuleID != "" {
		return m.ModuleID
	}
	return ExtractModuleIDFromNamespace(m.Namespace)
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
