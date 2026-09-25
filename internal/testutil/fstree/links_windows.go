// SPDX-License-Identifier: MPL-2.0

//go:build windows

package fstree

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// createSymlink creates a symbolic link. os.Symlink picks the directory flag
// from the existing target; a dangling target yields a file link, which is what
// the model's dangling links mean.
func createSymlink(target, link string) error {
	return os.Symlink(target, link)
}

// createJunction creates an NTFS directory junction (mount point) at link
// pointing at target, matching `mklink /J` semantics. os.Lstat does not report a
// junction with ModeSymlink (Go 1.23+), so the transcribed symlink checks do not
// see it.
func createJunction(target, link string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return err
	}
	return setMountPoint(link, target)
}

func probeJunction(base string) bool {
	target := filepath.Join(base, "probe-jun-target")
	link := filepath.Join(base, "probe-jun-link")
	if createJunction(target, link) != nil {
		return false
	}
	_ = os.Remove(link)
	return true
}

// setMountPoint writes an IO_REPARSE_TAG_MOUNT_POINT reparse point on the empty
// directory at link, pointing at the absolute target path.
func setMountPoint(link, target string) error {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	handle, err := openReparseHandle(link)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return writeMountPointReparse(handle, `\??\`+absTarget)
}

func openReparseHandle(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(
		p,
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
}

func writeMountPointReparse(handle windows.Handle, subst string) error {
	subUTF16 := windows.StringToUTF16(subst)
	subBytes := (len(subUTF16) - 1) * 2 // exclude the trailing NUL from the length
	buf := make([]byte, 8+8+subBytes+2)
	// REPARSE_DATA_BUFFER header
	putUint32(buf[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	putUint16(buf[4:], uint16(8+subBytes+2)) // ReparseDataLength
	// MountPointReparseBuffer: SubstituteNameOffset, Length, PrintNameOffset, Length
	putUint16(buf[8:], 0)
	putUint16(buf[10:], uint16(subBytes))
	putUint16(buf[12:], uint16(subBytes+2))
	putUint16(buf[14:], 0)
	for i, w := range subUTF16[:len(subUTF16)-1] {
		putUint16(buf[16+i*2:], w)
	}
	var returned uint32
	return windows.DeviceIoControl(
		handle,
		windows.FSCTL_SET_REPARSE_POINT,
		&buf[0], uint32(len(buf)),
		nil, 0,
		&returned, nil,
	)
}

func putUint16(b []byte, v uint16) { b[0] = byte(v); b[1] = byte(v >> 8) }
func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
