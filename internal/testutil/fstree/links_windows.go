// SPDX-License-Identifier: MPL-2.0

//go:build windows

package fstree

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// createSymlink creates a symbolic link. os.Symlink picks the directory flag
// from the existing target; a dangling target yields a file link, which is what
// the model's dangling links mean.
func createSymlink(target, link string) error {
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}
	return nil
}

// createJunction creates an NTFS directory junction (mount point) at link
// pointing at target, matching `mklink /J` semantics. os.Lstat does not report a
// junction with ModeSymlink (Go 1.23+), so the transcribed symlink checks do not
// see it.
//
// It writes the reparse point itself and has no fallback writer: probeJunction
// shares this path, so a fallback would let a broken reparse writer pass the
// capability policy silently (decision 12 of the harden-formal-instruments
// change).
func createJunction(target, link string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("junction target: %w", err)
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return fmt.Errorf("junction directory: %w", err)
	}
	if err := setMountPoint(link, target); err != nil {
		return fmt.Errorf("reparse point: %w", err)
	}
	return nil
}

func probeJunction(base string) error {
	target := filepath.Join(base, "probe-jun-target")
	link := filepath.Join(base, "probe-jun-link")
	if err := createJunction(target, link); err != nil {
		return err
	}
	_ = os.Remove(link)
	return nil
}

// setMountPoint writes an IO_REPARSE_TAG_MOUNT_POINT reparse point on the empty
// directory at link, pointing at the absolute target path.
func setMountPoint(link, target string) (err error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("absolute junction target: %w", err)
	}
	handle, err := openReparseHandle(link)
	if err != nil {
		return fmt.Errorf("open %s for FSCTL_SET_REPARSE_POINT: %w", link, err)
	}
	defer func() {
		if closeErr := windows.CloseHandle(handle); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", link, closeErr)
		}
	}()
	if err = writeMountPointReparse(handle, `\??\`+absTarget, absTarget); err != nil {
		return fmt.Errorf("FSCTL_SET_REPARSE_POINT on %s: %w", link, err)
	}
	return nil
}

// openReparseHandle opens the directory as Go's own os_windows_test.go
// createDirLink does: GENERIC_WRITE, no sharing, OPEN_EXISTING, and
// FILE_FLAG_OPEN_REPARSE_POINT|FILE_FLAG_BACKUP_SEMANTICS.
func openReparseHandle(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(
		p,
		windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
}

// writeMountPointReparse writes a REPARSE_DATA_BUFFER with a
// MountPointReparseBuffer laid out exactly as Go 1.27's src/os/os_windows_test.go
// createMountPoint does for its "standard" (mklink-like) case, compared field
// by field (harden-formal-instruments task 4.2):
//   - PathBuffer = substitute name + NUL, then print name + NUL. The previous
//     writer stored no print name and no print-name NUL (PrintNameOffset
//     pointed past ReparseDataLength), and junction creation failed on
//     windows-latest. With this layout TestCreateJunction passes there and
//     reports "created via reparse point" (CI run 43660058330).
//   - SubstituteNameLength and PrintNameLength exclude their NULs; the offsets
//     are byte offsets into PathBuffer (0 and len(substitute)+2).
//   - ReparseDataLength = offsetof(MountPointReparseBuffer.PathBuffer) (8) +
//     len(PathBuffer) in bytes.
//   - The ioctl input length = ReparseDataLength + the 8-byte header (tag,
//     ReparseDataLength, Reserved).
//   - CreateFile: GENERIC_WRITE, share mode 0 (the previous writer shared
//     read and write), OPEN_EXISTING, OPEN_REPARSE_POINT|BACKUP_SEMANTICS.
//
// The print name is the absolute target, as mklink writes it.
func writeMountPointReparse(handle windows.Handle, subst, printName string) error {
	subUTF16 := windows.StringToUTF16(subst) // includes the NUL
	printUTF16 := windows.StringToUTF16(printName)
	subBytes := (len(subUTF16) - 1) * 2
	printBytes := (len(printUTF16) - 1) * 2
	pathBuf := make([]uint16, 0, len(subUTF16)+len(printUTF16))
	pathBuf = append(pathBuf, subUTF16...)
	pathBuf = append(pathBuf, printUTF16...)
	dataLength := 8 + len(pathBuf)*2
	buf := make([]byte, 8+dataLength)
	// REPARSE_DATA_BUFFER header; buf[6:8] (Reserved) stays 0.
	binary.LittleEndian.PutUint32(buf[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:], uint16(dataLength)) // ReparseDataLength
	// MountPointReparseBuffer: SubstituteNameOffset, Length, PrintNameOffset, Length
	binary.LittleEndian.PutUint16(buf[8:], 0)
	binary.LittleEndian.PutUint16(buf[10:], uint16(subBytes))
	binary.LittleEndian.PutUint16(buf[12:], uint16(subBytes+2))
	binary.LittleEndian.PutUint16(buf[14:], uint16(printBytes))
	for i, w := range pathBuf {
		binary.LittleEndian.PutUint16(buf[16+i*2:], w)
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
