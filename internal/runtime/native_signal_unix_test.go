// SPDX-License-Identifier: MPL-2.0

//go:build unix

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/xpty"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestNativeRuntime_Signal_ProcessGroupCleanup(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("SIGTERM signals non-TTY process group including grandchild", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "grandchild.pid")
		script := fmt.Sprintf(`sleep 30 &
child_pid=$!
printf '%%s\n' "$child_pid" > %q
wait "$child_pid"`, pidFile)

		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		}
		cmd := testCommandWithScript("pgid-term", script, invowkfile.RuntimeNative)

		cancelCtx, cancel := context.WithCancel(t.Context())
		defer cancel()

		rt := NewNativeRuntime()
		rt.termGracePeriod = 2 * time.Second
		execCtx := NewExecutionContext(cancelCtx, cmd, inv)
		var stdout, stderr bytes.Buffer
		execCtx.IO = IOContext{Stdout: &stdout, Stderr: &stderr}

		resCh := make(chan *Result, 1)
		go func() {
			resCh <- rt.Execute(execCtx)
		}()

		testutil.RequirePollUntil(t, 5*time.Second, 20*time.Millisecond, "grandchild pid file", func() bool {
			data, err := os.ReadFile(pidFile)
			return err == nil && strings.TrimSpace(string(data)) != ""
		})

		rawPID, err := os.ReadFile(pidFile)
		if err != nil {
			t.Fatalf("ReadFile(pidFile) error = %v", err)
		}
		grandchildPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
		if err != nil {
			t.Fatalf("Atoi(%q) error = %v", string(rawPID), err)
		}

		cancel()
		result := <-resCh
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("Execute() error = %v, want context.Canceled", result.Error)
		}

		testutil.RequirePollUntil(t, 3*time.Second, 20*time.Millisecond, "grandchild process exit", func() bool {
			err := syscall.Kill(grandchildPID, 0)
			return errors.Is(err, syscall.ESRCH)
		})
	})

	t.Run("SIGKILL escalates after grace period when process group traps SIGTERM", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "grandchild.pid")
		script := fmt.Sprintf(`trap '' TERM
sh -c "trap '' TERM; sleep 30" &
child_pid=$!
printf '%%s\n' "$child_pid" > %q
wait "$child_pid"`, pidFile)

		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		}
		cmd := testCommandWithScript("pgid-sigkill", script, invowkfile.RuntimeNative)

		cancelCtx, cancel := context.WithCancel(t.Context())
		defer cancel()

		rt := NewNativeRuntime()
		rt.termGracePeriod = 150 * time.Millisecond
		execCtx := NewExecutionContext(cancelCtx, cmd, inv)
		var stdout, stderr bytes.Buffer
		execCtx.IO = IOContext{Stdout: &stdout, Stderr: &stderr}

		resCh := make(chan *Result, 1)
		go func() {
			resCh <- rt.Execute(execCtx)
		}()

		testutil.RequirePollUntil(t, 5*time.Second, 20*time.Millisecond, "grandchild pid file", func() bool {
			data, err := os.ReadFile(pidFile)
			return err == nil && strings.TrimSpace(string(data)) != ""
		})

		rawPID, err := os.ReadFile(pidFile)
		if err != nil {
			t.Fatalf("ReadFile(pidFile) error = %v", err)
		}
		grandchildPID, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
		if err != nil {
			t.Fatalf("Atoi(%q) error = %v", string(rawPID), err)
		}

		start := time.Now()
		cancel()
		result := <-resCh
		if elapsed := time.Since(start); elapsed > 4*time.Second {
			t.Fatalf("Execute() took %v after cancel, want bounded SIGKILL escalation", elapsed)
		}
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("Execute() error = %v, want context.Canceled", result.Error)
		}

		testutil.RequirePollUntil(t, 3*time.Second, 20*time.Millisecond, "trapped grandchild killed by SIGKILL", func() bool {
			err := syscall.Kill(grandchildPID, 0)
			return errors.Is(err, syscall.ESRCH)
		})
	})
}

func TestNativeRuntime_Signal_TempScriptCleanup(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	scriptPathFile := filepath.Join(tmpDir, "script.path")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := &invowkfile.Command{
		Name: "temp-script-cancel",
		Implementations: []invowkfile.Implementation{{
			Script: invowkfile.ImplementationScript{
				Content:     invowkfile.ScriptContent(fmt.Sprintf("printf '%%s\\n' \"$0\" > %q\nsleep 30", scriptPathFile)),
				Interpreter: "sh",
			},
			Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}

	cancelCtx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt := NewNativeRuntime()
	execCtx := NewExecutionContext(cancelCtx, cmd, inv)
	var stdout, stderr bytes.Buffer
	execCtx.IO = IOContext{Stdout: &stdout, Stderr: &stderr}

	resCh := make(chan *Result, 1)
	go func() {
		resCh <- rt.Execute(execCtx)
	}()

	testutil.RequirePollUntil(t, 5*time.Second, 20*time.Millisecond, "temp script path file", func() bool {
		data, err := os.ReadFile(scriptPathFile)
		return err == nil && strings.TrimSpace(string(data)) != ""
	})

	rawPath, err := os.ReadFile(scriptPathFile)
	if err != nil {
		t.Fatalf("ReadFile(scriptPathFile) error = %v", err)
	}
	tempScriptPath := strings.TrimSpace(string(rawPath))
	if !strings.Contains(filepath.Base(tempScriptPath), "invowk-script-") {
		t.Fatalf("temp script path = %q, want invowk-script-*", tempScriptPath)
	}

	cancel()
	result := <-resCh
	if !errors.Is(result.Error, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", result.Error)
	}

	if _, statErr := os.Stat(tempScriptPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("temp script %q still exists after cancellation (stat err = %v)", tempScriptPath, statErr)
	}
}

func TestNativeRuntime_Signal_DirectTTYSafety(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pty, err := xpty.NewPty(80, 24)
	if err != nil {
		t.Fatalf("xpty.NewPty() error = %v", err)
	}
	t.Cleanup(func() { _ = pty.Close() })

	unixPty, ok := pty.(*xpty.UnixPty)
	if !ok {
		t.Fatalf("pty type = %T, want *xpty.UnixPty", pty)
	}
	slave := unixPty.Slave()

	if !hasDirectControllingTTY(slave, slave, slave) {
		t.Fatal("hasDirectControllingTTY(slave) = false, want true")
	}
	if hasDirectControllingTTY(nil, &bytes.Buffer{}, &bytes.Buffer{}) {
		t.Fatal("hasDirectControllingTTY(non-TTY) = true, want false")
	}

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "tty.out")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	// Reading from stdin on a PTY without Setpgid must not suspend on SIGTTIN.
	script := fmt.Sprintf("IFS= read -r line; printf 'line=%%s\\n' \"$line\" > %q", outFile)
	cmd := testCommandWithScript("direct-tty-read", script, invowkfile.RuntimeNative)

	execCtx := NewExecutionContext(t.Context(), cmd, inv)
	execCtx.IO = IOContext{
		Stdin:  slave,
		Stdout: slave,
		Stderr: slave,
	}

	rt := NewNativeRuntime()
	resCh := make(chan *Result, 1)
	go func() {
		resCh <- rt.Execute(execCtx)
	}()

	if _, writeErr := pty.Write([]byte("tty-ok\n")); writeErr != nil {
		t.Fatalf("pty.Write() error = %v", writeErr)
	}

	select {
	case result := <-resCh:
		if result.ExitCode != 0 || result.Error != nil {
			t.Fatalf("Execute() = exit %d err=%v, want exit 0", result.ExitCode, result.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("direct TTY read stalled (possible SIGTTIN from Setpgid)")
	}

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile(outFile) error = %v", err)
	}
	if strings.TrimSpace(string(got)) != "line=tty-ok" {
		t.Fatalf("tty output = %q, want %q", strings.TrimSpace(string(got)), "line=tty-ok")
	}
}
