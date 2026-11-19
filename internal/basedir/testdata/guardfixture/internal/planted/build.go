// SPDX-License-Identifier: MPL-2.0

package planted

import "os"

// xdgPrefix builds a base-variable name by concatenation.
const xdgPrefix = "XDG_" // want: literal XDG_*

// Built reads an XDG variable through a built name.
func Built() string { return os.Getenv(xdgPrefix + "CACHE_HOME") }
