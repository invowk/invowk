// SPDX-License-Identifier: EPL-2.0

//go:build windows

package sshserver

import (
	"io"
	"os"
	"os/exec"
)

// startPty starts a command - on Windows we don't use PTY
func startPty(cmd *exec.Cmd) (*os.File, error) {
	// On Windows, we use pipes instead of PTY
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Return stdin pipe as the "file" to write to
	// This is a simplified approach for Windows
	return os.NewFile(stdin.(*os.File).Fd(), "stdin"), nil
}

// setWinsize is a no-op on Windows
func setWinsize(f *os.File, width, height int) {
	// No-op on Windows
}

// copyBuffer copies from src to dst
func copyBuffer(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
