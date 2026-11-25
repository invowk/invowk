// SPDX-License-Identifier: MPL-2.0

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package modulelock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLockFile takes flock(LOCK_EX|LOCK_NB); busy reports another holder.
func tryLockFile(f *os.File) (busy bool, err error) {
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		switch {
		case err == nil:
			return false, nil
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.EWOULDBLOCK):
			return true, nil
		default:
			return false, err
		}
	}
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
