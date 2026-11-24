// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package runtime

import (
	"errors"
	"io/fs"
)

// isVirtualPathMissingVolume reports whether err, from an lstat of a volume
// root, means that the volume does not exist.
func isVirtualPathMissingVolume(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
