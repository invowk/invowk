// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

// GetVendoredModulesDir returns the path to the vendored modules directory for a given module.
// Returns the path whether or not the directory exists.
func GetVendoredModulesDir(modulePath types.FilesystemPath) types.FilesystemPath {
	return fspath.JoinStr(modulePath, VendoredModulesDir)
}
