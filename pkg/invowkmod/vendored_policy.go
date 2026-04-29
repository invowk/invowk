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
	if lock == nil || moduleID == "" {
		return false
	}
	for _, req := range requirements {
		key := ModuleRef(req).Key()
		locked, ok := lock.Modules[key]
		if !ok {
			continue
		}
		if locked.IdentityModuleID() == moduleID {
			return true
		}
	}
	return false
}
