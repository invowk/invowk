// SPDX-License-Identifier: MPL-2.0

package planted

import . "os"

// Dot plants the five lookups through a dot import of os.
func Dot() {
	_, _ = UserHomeDir()   // want: os.UserHomeDir
	_, _ = UserConfigDir() // want: os.UserConfigDir
	_, _ = UserCacheDir()  // want: os.UserCacheDir
	_ = ExpandEnv("x")     // want: os.ExpandEnv
	_ = Expand("x", nil)   // want: os.Expand
}
