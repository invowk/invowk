// SPDX-License-Identifier: MPL-2.0

package planted

import goos "os"

// Alias plants the three lookups under an aliased os import.
func Alias() {
	_, _ = goos.UserHomeDir()   // want: os.UserHomeDir
	_, _ = goos.UserConfigDir() // want: os.UserConfigDir
	lookup := goos.UserCacheDir // want: os.UserCacheDir
	_ = lookup
}
