// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"

	"github.com/invowk/invowk/pkg/invowkmod"
)

type (
	// appendedModuleDirs records, for one discovery run, the top-level module
	// directories appended so far, so that the same physical directory reached
	// through another source (the current directory, the current-directory module
	// scan, configured includes, provisioned entries, the user commands directory)
	// is loaded once. Directories are compared with os.SameFile, which compares
	// device and inode (the file index on Windows), so different spellings of one
	// directory (letter case, short names, links) match.
	//
	//goplint:ignore -- private per-run bookkeeping over already discovered files.
	appendedModuleDirs struct {
		dirs []appendedModuleDir
	}

	//goplint:ignore -- private per-run bookkeeping over already discovered files.
	appendedModuleDir struct {
		info     os.FileInfo
		file     *DiscoveredFile
		vendored []*DiscoveredFile
	}
)

// Validate returns nil when every recorded module directory is valid.
func (a *appendedModuleDirs) Validate() error {
	errs := make([]error, 0, len(a.dirs))
	for i := range a.dirs {
		errs = append(errs, a.dirs[i].Validate())
	}
	return errors.Join(errs...)
}

// Validate returns nil when the kept file and its vendored children are valid.
func (d appendedModuleDir) Validate() error {
	var errs []error
	if d.file != nil {
		errs = append(errs, d.file.Validate())
	}
	for _, vf := range d.vendored {
		if vf != nil {
			errs = append(errs, vf.Validate())
		}
	}
	return errors.Join(errs...)
}

// topLevelModuleDirInfo returns the os.Stat result of a top-level module
// file's module directory. Vendored entries and non-module files are not
// deduplicated, and neither is a directory that cannot be stat'ed: it keeps the
// module collision check.
func topLevelModuleDirInfo(file *DiscoveredFile) (os.FileInfo, bool) {
	if file == nil || file.Module == nil || file.ParentModule != nil {
		return nil, false
	}
	info, err := os.Stat(string(file.Module.Path))
	if err != nil {
		return nil, false
	}
	return info, true
}

func (a *appendedModuleDirs) find(info os.FileInfo) *appendedModuleDir {
	for i := range a.dirs {
		if os.SameFile(a.dirs[i].info, info) {
			return &a.dirs[i]
		}
	}
	return nil
}

func (a *appendedModuleDirs) add(info os.FileInfo, file *DiscoveredFile, vendored []*DiscoveredFile) {
	a.dirs = append(a.dirs, appendedModuleDir{info: info, file: file, vendored: vendored})
}

// mergeDuplicateModule folds a dropped duplicate of a module directory into the
// kept, higher-precedence entry:
//   - the module is global when any of its sources is global, and its vendored
//     children follow it, so scope treats them like their parent;
//   - when the kept entry has no explicit command namespace, it adopts the first
//     explicit namespace (a provisioned namespace or a configured include alias)
//     of a dropped duplicate.
func (d *Discovery) mergeDuplicateModule(kept *appendedModuleDir, duplicate *DiscoveredFile) {
	if duplicate.IsGlobalModule && !kept.file.IsGlobalModule {
		kept.file.IsGlobalModule = true
		for _, vf := range kept.vendored {
			vf.IsGlobalModule = true
		}
	}
	if d.explicitCommandNamespace(kept.file) != "" {
		return
	}
	if namespace := d.explicitCommandNamespace(duplicate); namespace != "" {
		kept.file.CommandNamespace = namespace
	}
}

// explicitCommandNamespace returns the namespace a module file publishes under
// explicitly: its CommandNamespace, or the include alias configured for its
// module path. It is empty when the module publishes under its own name.
func (d *Discovery) explicitCommandNamespace(file *DiscoveredFile) invowkmod.ModuleNamespace {
	if file.CommandNamespace != "" {
		return file.CommandNamespace
	}
	if file.Module == nil {
		return ""
	}
	return invowkmod.ModuleNamespace(d.getAliasForModulePath(file.Module.Path))
}
