// SPDX-License-Identifier: MPL-2.0

package nested

import "os"

// testdata below the scan root is skipped.
var _, _ = os.UserHomeDir()
