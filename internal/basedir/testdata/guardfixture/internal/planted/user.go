// SPDX-License-Identifier: MPL-2.0

package planted

import "os/user" // want: os/user import

// Current plants an os/user lookup.
func Current() { _, _ = user.Current() }
