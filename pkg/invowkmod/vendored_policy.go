// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"path/filepath"
	"slices"
)

type declaredLockedModuleEntry struct {
	key    ModuleRefKey
	locked LockedModule
}

func (e declaredLockedModuleEntry) Validate() error {
	return errors.Join(e.key.Validate(), e.locked.Validate())
}

// IsDeclaredLockedVendoredModule reports whether childModule is an explicit,
// locked dependency of parentModule.
func IsDeclaredLockedVendoredModule(parentModule, childModule *Module) bool {
	if parentModule == nil || parentModule.Metadata == nil || childModule == nil || childModule.Metadata == nil {
		return false
	}

	lockPath := filepath.Join(string(parentModule.Path), LockFileName)
	lock, err := LoadLockFile(lockPath)
	if err != nil {
		return false
	}

	return IsDeclaredLockedModule(parentModule.Metadata.Requires, lock, childModule.Metadata.Module)
}

// IsDeclaredLockedModule reports whether moduleID is present in both the
// declared requirements and the lock file.
func IsDeclaredLockedModule(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) bool {
	_, _, ok := DeclaredLockedModuleEntry(requirements, lock, moduleID)
	return ok
}

// DeclaredLockedModule returns the lock entry that declares moduleID through the
// root requirements, if one exists.
func DeclaredLockedModule(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) (LockedModule, bool) {
	_, locked, ok := DeclaredLockedModuleEntry(requirements, lock, moduleID)
	return locked, ok
}

// DeclaredLockedModuleEntry returns the requirement key and lock entry that
// declares moduleID through the root requirements, if exactly one exists.
func DeclaredLockedModuleEntry(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) (ModuleRefKey, LockedModule, bool) {
	matches := declaredLockedModuleEntryMatches(requirements, lock, moduleID)
	if len(matches) != 1 {
		return "", LockedModule{}, false
	}
	return matches[0].key, matches[0].locked, true
}

// AmbiguousDeclaredLockedModuleEntries returns declared lock keys that resolve
// to the same stable module identity. Such a module cannot be safely matched to
// one trusted lock entry.
func AmbiguousDeclaredLockedModuleEntries(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) []ModuleRefKey {
	matches := declaredLockedModuleEntryMatches(requirements, lock, moduleID)
	if len(matches) < 2 {
		return nil
	}
	keys := make([]ModuleRefKey, 0, len(matches))
	for i := range matches {
		keys = append(keys, matches[i].key)
	}
	return keys
}

func declaredLockedModuleEntryMatches(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) []declaredLockedModuleEntry {
	if lock == nil || moduleID == "" {
		return nil
	}

	matchesByKey := make(map[ModuleRefKey]LockedModule)
	for _, req := range requirements {
		key := ModuleRef(req).Key()
		locked, ok := lock.Modules[key]
		if !ok {
			continue
		}
		if locked.IdentityModuleID() == moduleID {
			matchesByKey[key] = locked
		}
	}

	keys := make([]ModuleRefKey, 0, len(matchesByKey))
	for key := range matchesByKey {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	matches := make([]declaredLockedModuleEntry, 0, len(keys))
	for _, key := range keys {
		matches = append(matches, declaredLockedModuleEntry{key: key, locked: matchesByKey[key]})
	}
	return matches
}

// IsDeclaredLockedCommandSource reports whether the discovered command source
// is a direct requirement whose lock entry resolves to the same module identity
// and command namespace.
func IsDeclaredLockedCommandSource(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID, sourceID ModuleSourceID) bool {
	if lock == nil || moduleID == "" || sourceID == "" {
		return false
	}
	for _, req := range requirements {
		locked, ok := lock.Modules[ModuleRef(req).Key()]
		if !ok {
			continue
		}
		if locked.IdentityModuleID() == moduleID && locked.EffectiveCommandSourceID() == sourceID {
			return true
		}
	}
	return false
}

// MissingLockedModuleRequirementKeys returns requirement keys without exact
// lock-file entries. It uses ModuleRef.Key() so subdirectory requirements match
// the same normalized key policy used by module sync and lock parsing.
func MissingLockedModuleRequirementKeys(requirements []ModuleRequirement, lock *LockFile) []ModuleRefKey {
	missing := make([]ModuleRefKey, 0)
	for _, req := range requirements {
		key := ModuleRef(req).Key()
		if lock == nil || !lock.HasModule(key) {
			missing = append(missing, key)
		}
	}
	return missing
}

// OrphanedLockedModuleEntries returns lock entries that are not declared by the
// current root requirements. The lock file may contain stale entries after
// requirements are removed; vendored module presence is intentionally not part
// of this classification.
func OrphanedLockedModuleEntries(requirements []ModuleRequirement, lock *LockFile) []ModuleRefKey {
	if lock == nil {
		return nil
	}

	declared := make(map[ModuleRefKey]bool, len(requirements))
	for _, req := range requirements {
		declared[ModuleRef(req).Key()] = true
	}

	var orphaned []ModuleRefKey
	for key := range lock.Modules {
		if !declared[key] {
			orphaned = append(orphaned, key)
		}
	}
	slices.Sort(orphaned)
	return orphaned
}
