//go:build !windows

package sshserver

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
)

// startPty starts a command with a pseudo-terminal
func startPty(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

// setWinsize sets the window size for the PTY
func setWinsize(f *os.File, width, height int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct {
			h, w, x, y uint16
		}{uint16(height), uint16(width), 0, 0})))
}

// copyBuffer copies from src to dst
func copyBuffer(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
