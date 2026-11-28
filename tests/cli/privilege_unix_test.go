// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cli

import "os"

func isTestProcessElevated() bool {
	return os.Geteuid() == 0
}
