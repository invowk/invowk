// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestTUISpinSIGINTWithoutTerminal sends SIGINT to the process group of a
// `tui spin` that has no terminal, as a terminal's Ctrl+C would, and expects
// exit status 1 with the command's output kept, the status a native
// `invowk cmd` gives (tui-clean-terminal-path, N14). Before the change spin
// forwarded the killed command's -1 and exited 255. Windows has no portable
// SIGINT; the unit tests in cmd/invowk cover its cancellation path.
func TestTUISpinSIGINTWithoutTerminal(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pidPath := filepath.Join(t.TempDir(), "pid")
	cmd := exec.CommandContext(t.Context(), binaryPath, "tui", "spin", "--",
		"sh", "-c", `echo $$ > "$1"; echo started; exec sleep 30`, "sh", pidPath)
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })
	var stdout, stderr bytes.Buffer
	cmd.Stdin = devNull
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	cmd.SysProcAttr = noTerminalProcAttr()
	cmd.WaitDelay = 5 * time.Second
	if err = cmd.Start(); err != nil {
		t.Fatalf("start invowk: %v", err)
	}

	if !waitForPath(pidPath, 20*time.Second) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("wrapped command did not start; stderr:\n%s", stderr.String())
	}
	// The new session makes invowk the leader of its own process group.
	start := time.Now()
	if err = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT to the process group: %v", err)
	}

	err = cmd.Wait()
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("spin exited %v after SIGINT, want well before the 30 s sleep", elapsed)
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("spin error = %v, want exit status 1; stdout %q, stderr:\n%s", err, stdout.String(), stderr.String())
	}
	if got := stdout.String(); got != "started\n" {
		t.Fatalf("stdout = %q, want %q", got, "started\n")
	}
}
