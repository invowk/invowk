// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"path/filepath"
)

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
// declares moduleID through the root requirements, if one exists.
func DeclaredLockedModuleEntry(requirements []ModuleRequirement, lock *LockFile, moduleID ModuleID) (ModuleRefKey, LockedModule, bool) {
	if lock == nil || moduleID == "" {
		return "", LockedModule{}, false
	}
	for _, req := range requirements {
		key := ModuleRef(req).Key()
		locked, ok := lock.Modules[key]
		if !ok {
			continue
		}
		if locked.IdentityModuleID() == moduleID {
			return key, locked, true
		}
	}
	return "", LockedModule{}, false
}
