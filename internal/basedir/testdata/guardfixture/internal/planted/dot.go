// SPDX-License-Identifier: MPL-2.0

package planted

import . "os"

// Dot plants the three lookups through a dot import of os.
func Dot() {
	_, _ = UserHomeDir()   // want: os.UserHomeDir
	_, _ = UserConfigDir() // want: os.UserConfigDir
	_, _ = UserCacheDir()  // want: os.UserCacheDir
}
