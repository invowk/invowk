// SPDX-License-Identifier: MPL-2.0

//go:build unix

package commandadapters

import (
	"bytes"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/xpty"
)

const (
	// ptyTestTimeout bounds every wait on a command started on a test PTY.
	ptyTestTimeout = 10 * time.Second
	// ptyTestPollInterval is the poll period of the output and pid waits.
	ptyTestPollInterval = 20 * time.Millisecond
)

var bgPIDPattern = regexp.MustCompile(`BG_PID=(\d+)`)

type (
	// ptyHarness runs one command on the start seam of runInteractiveCmd and
	// collects everything the command writes to its terminal.
	ptyHarness struct {
		t    *testing.T
		cmd  *exec.Cmd
		pty  xpty.Pty
		mu   sync.Mutex
		out  bytes.Buffer
		done chan struct{}
		wait chan error
	}
)

// startPtyHarness starts `sh -c script` on a PTY through startInteractivePty
// (a 80x27 command area of an 80x30 viewer) and reads its output in the
// background. Cleanup kills the command's process group and closes the PTY.
func startPtyHarness(t *testing.T, script string) *ptyHarness {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "sh", "-c", script)
	pty, err := startInteractivePty(cmd, 80, 30)
	if err != nil {
		t.Fatalf("startInteractivePty() error = %v", err)
	}
	h := &ptyHarness{t: t, cmd: cmd, pty: pty, done: make(chan struct{}), wait: make(chan error, 1)}
	go h.readLoop()
	go func() { h.wait <- cmd.Wait() }()
	t.Cleanup(func() {
		// The command is its own session and process group leader (Setsid),
		// so its pid is the group id.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = pty.Close()
		<-h.done
	})
	return h
}

func (h *ptyHarness) readLoop() {
	defer close(h.done)
	buf := make([]byte, 4096)
	for {
		n, err := h.pty.Read(buf)
		if n > 0 {
			h.mu.Lock()
			h.out.Write(buf[:n])
			h.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (h *ptyHarness) output() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.ReplaceAll(h.out.String(), "\r\n", "\n")
}

// waitOutput waits until the command's output contains want.
func (h *ptyHarness) waitOutput(want string) string {
	h.t.Helper()
	deadline := time.Now().Add(ptyTestTimeout)
	for time.Now().Before(deadline) {
		if out := h.output(); strings.Contains(out, want) {
			return out
		}
		time.Sleep(ptyTestPollInterval)
	}
	h.t.Fatalf("output does not contain %q within %s; output:\n%q", want, ptyTestTimeout, h.output())
	return ""
}

// waitExit waits for the command to end and returns its Wait error.
func (h *ptyHarness) waitExit() error {
	h.t.Helper()
	select {
	case err := <-h.wait:
		return err
	case <-time.After(ptyTestTimeout):
		h.t.Fatalf("command did not end within %s; output:\n%q", ptyTestTimeout, h.output())
		return nil
	}
}

func (h *ptyHarness) send(b ...byte) {
	h.t.Helper()
	if _, err := h.pty.Write(b); err != nil {
		h.t.Fatalf("write to PTY master: %v", err)
	}
}

// processAlive reports whether pid names a process. A killed orphan stays a
// zombie until its new parent reaps it, which the polling waits allow for.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func printedPID(t *testing.T, out string) int {
	t.Helper()
	m := bgPIDPattern.FindStringSubmatch(out)
	if len(m) < 2 {
		t.Fatalf("output has no BG_PID line:\n%q", out)
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil || pid <= 1 {
		t.Fatalf("BG_PID = %q, want a process id", m[1])
	}
	return pid
}

func TestPrepareInteractiveCmdRefusesPresetStreams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(*exec.Cmd)
		want    error
	}{
		{name: "stdin", prepare: func(c *exec.Cmd) { c.Stdin = strings.NewReader("") }, want: errInteractiveStdioPreset},
		{name: "stdout", prepare: func(c *exec.Cmd) { c.Stdout = &bytes.Buffer{} }, want: errInteractiveStdioPreset},
		{name: "stderr", prepare: func(c *exec.Cmd) { c.Stderr = &bytes.Buffer{} }, want: errInteractiveStdioPreset},
		{name: "sysprocattr", prepare: func(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{} }, want: errInteractiveSysProcAttrPreset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.CommandContext(t.Context(), "true")
			tt.prepare(cmd)
			pty, err := startInteractivePty(cmd, 80, 30)
			if pty != nil {
				_ = pty.Close()
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("startInteractivePty() error = %v, want %v", err, tt.want)
			}
			if cmd.Process != nil {
				t.Fatal("refused command was started")
			}
		})
	}
}

func TestPrepareInteractiveCmdUsesSlave(t *testing.T) {
	t.Parallel()

	pty, err := xpty.NewPty(80, 24)
	if err != nil {
		t.Fatalf("NewPty() error = %v", err)
	}
	defer pty.Close()
	cmd := exec.CommandContext(t.Context(), "true")
	if err := prepareInteractiveCmd(cmd, pty); err != nil {
		t.Fatalf("prepareInteractiveCmd() error = %v", err)
	}
	slave := pty.(*xpty.UnixPty).Slave()
	if cmd.Stdin != slave || cmd.Stdout != slave || cmd.Stderr != slave {
		t.Fatalf("stdio = %v/%v/%v, want the slave %v", cmd.Stdin, cmd.Stdout, cmd.Stderr, slave)
	}
	attr := cmd.SysProcAttr
	if attr == nil || !attr.Setsid || !attr.Setctty || attr.Ctty != 0 {
		t.Fatalf("SysProcAttr = %+v, want Setsid, Setctty and Ctty 0", attr)
	}
}

// TestInteractivePtyCtty: the PTY is the command's controlling terminal, so
// `tty` (stdin) and `ps -o tty=` (controlling terminal) name the same device.
func TestInteractivePtyCtty(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skip("ps not available")
	}

	h := startPtyHarness(t, `printf 'TTY=%s\n' "$(tty)"; printf 'CTTY=%s\n' "$(ps -o tty= -p $$)"; echo END`)
	out := h.waitOutput("END")
	ttyName := regexp.MustCompile(`TTY=(\S+)`).FindStringSubmatch(out)
	cttyName := regexp.MustCompile(`CTTY=\s*(\S+)`).FindStringSubmatch(out)
	if len(ttyName) < 2 || len(cttyName) < 2 {
		t.Fatalf("output lacks TTY/CTTY lines:\n%q", out)
	}
	if strings.TrimPrefix(ttyName[1], "/dev/") != strings.TrimPrefix(cttyName[1], "/dev/") {
		t.Fatalf("tty = %q, controlling terminal = %q, want the same device", ttyName[1], cttyName[1])
	}
	if err := h.waitExit(); err != nil {
		t.Fatalf("command error = %v", err)
	}
}

// TestInteractivePtyInterrupt: Ctrl+C written to the master reaches the
// command's foreground job as SIGINT.
func TestInteractivePtyInterrupt(t *testing.T) {
	t.Parallel()

	h := startPtyHarness(t, `echo ready; exec sleep 30`)
	h.waitOutput("ready")
	start := time.Now()
	h.send(0x03)
	err := h.waitExit()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("command ended after %s, want well before 30s", elapsed)
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		t.Fatalf("command error = %v, want an exit error", err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
		t.Fatalf("wait status = %v, want killed by SIGINT", exitErr)
	}
}

// TestInteractivePtyRawCtrlC: a raw-mode program reads Ctrl+C as a byte.
func TestInteractivePtyRawCtrlC(t *testing.T) {
	t.Parallel()

	h := startPtyHarness(t, `stty raw -echo; echo ready; od -An -c -N1; echo END`)
	h.waitOutput("ready")
	h.send(0x03)
	out := h.waitOutput("END")
	if !strings.Contains(out, "003") {
		t.Fatalf("od output = %q, want the byte 003", out)
	}
	if err := h.waitExit(); err != nil {
		t.Fatalf("command error = %v", err)
	}
}

// TestInteractivePtyWinch: a resize of the master delivers SIGWINCH to the
// command's foreground job, which then reads the new size.
func TestInteractivePtyWinch(t *testing.T) {
	t.Parallel()

	h := startPtyHarness(t, `trap 'echo "SIZE=$(stty size)"' WINCH; echo "START=$(stty size)"; while :; do sleep 1; done`)
	out := h.waitOutput("START=")
	if !strings.Contains(out, "START=27 80") {
		t.Fatalf("initial size output = %q, want the command area 27 80", out)
	}
	if err := h.pty.Resize(90, 20); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}
	h.waitOutput("SIZE=20 90")
}

// TestInteractivePtyBackgroundJob: when the command's main process exits,
// a background job it did not detach is hung up, and a nohup job survives.
func TestInteractivePtyBackgroundJob(t *testing.T) {
	t.Parallel()

	t.Run("attached", func(t *testing.T) {
		t.Parallel()
		h := startPtyHarness(t, `sleep 3017 & echo BG_PID=$!`)
		pid := printedPID(t, h.waitOutput("BG_PID="))
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
		if err := h.waitExit(); err != nil {
			t.Fatalf("command error = %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for processAlive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("background job %d still runs 5s after the command ended", pid)
			}
			time.Sleep(ptyTestPollInterval)
		}
	})

	t.Run("nohup", func(t *testing.T) {
		t.Parallel()
		// The pause lets nohup ignore SIGHUP before the main process exits;
		// without it the hang-up can reach the job before nohup runs.
		h := startPtyHarness(t, `nohup sleep 3019 >/dev/null 2>&1 & echo BG_PID=$!; sleep 1`)
		pid := printedPID(t, h.waitOutput("BG_PID="))
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
		if err := h.waitExit(); err != nil {
			t.Fatalf("command error = %v", err)
		}
		time.Sleep(time.Second)
		if !processAlive(pid) {
			t.Fatalf("nohup job %d ended with the command", pid)
		}
	})
}

// TestInteractivePtyCtrlZ: Ctrl+Z does not stop the command, whose process
// group is orphaned in its own session.
func TestInteractivePtyCtrlZ(t *testing.T) {
	t.Parallel()

	h := startPtyHarness(t, `echo ready; sleep 2; echo AFTER`)
	h.waitOutput("ready")
	h.send(0x1a)
	h.waitOutput("AFTER")
	if err := h.waitExit(); err != nil {
		t.Fatalf("command error = %v", err)
	}
}
