// SPDX-License-Identifier: MPL-2.0

package runtime

import "github.com/invowk/invowk/pkg/invowkfile"

// fullAccess reports whether the validator's access is "full". The runtimes
// hand it to the u-root registry as HandlerContext.FullAccess, which turns off
// the checks of the paths cp and tar -x derive from their operands; every
// other access mode keeps them.
func (v virtualPathValidator) fullAccess() bool {
	return v.resolver.access == invowkfile.VirtualFilesystemAccessFull
}
