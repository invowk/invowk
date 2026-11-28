// SPDX-License-Identifier: MPL-2.0

//go:build windows

package audit

import (
	"io/fs"
	"os"
)

// classifyFileWritableRisk is a no-op on Windows because POSIX 0o022 mode bits
// do not model NTFS discretionary ACLs.
//
//goplint:ignore -- platform stub matches Unix signature for OS permission inspection.
func classifyFileWritableRisk(string, func(os.FileInfo) bool) (writableRisk, fs.FileMode) {
	return writableRiskNone, 0
}

func isUnixSharedGroup(os.FileInfo) bool {
	return false
}
