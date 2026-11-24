// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"errors"
	"io/fs"
	"syscall"
)

// errorBadNetName is ERROR_BAD_NET_NAME: the server answers, but the share
// does not exist. Go maps ERROR_BAD_NETPATH (a missing server) to
// fs.ErrNotExist, but not this code.
const errorBadNetName = syscall.Errno(67)

// isVirtualPathMissingVolume reports whether err, from an lstat of a volume
// root, means that the volume does not exist: a missing drive, server or share.
func isVirtualPathMissingVolume(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, errorBadNetName)
}
