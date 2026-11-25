// SPDX-License-Identifier: MPL-2.0

//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package modulelock

import "os"

// tryLockFile fails closed where no advisory file lock is available.
func tryLockFile(*os.File) (busy bool, err error) { return false, ErrLockUnsupported }

func unlockFile(*os.File) error { return ErrLockUnsupported }
