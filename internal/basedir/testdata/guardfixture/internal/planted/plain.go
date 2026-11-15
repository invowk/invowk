// SPDX-License-Identifier: MPL-2.0

package planted

import "os"

// Plain plants the three lookups under the plain os name.
func Plain() {
	_, _ = os.UserHomeDir()   // want: os.UserHomeDir
	_, _ = os.UserConfigDir() // want: os.UserConfigDir
	_, _ = os.UserCacheDir()  // want: os.UserCacheDir
}
