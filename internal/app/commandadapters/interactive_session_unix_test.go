// SPDX-License-Identifier: MPL-2.0

//go:build unix

package commandadapters

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/tui"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/unix"
)

const (
	// ptyTestTimeout bounds every wait on a command started on a test PTY.
	ptyTestTimeout = 10 * time.Second
	// ptyTestPollInterval is the poll period of the output and pid waits.
	ptyTestPollInterval = 20 * time.Millisecond
	// lastLineRuns is how many adapter runs must show LAST-LINE above the
	// completion banner.
	lastLineRuns = 20
	// hupProbeBound bounds the wait for the hang-up after the viewer ended.
	hupProbeBound = 5 * time.Second
)

var (
	bgPIDPattern  = regexp.MustCompile(`BG_PID=(\d+)`)
	cmdPIDPattern = regexp.MustCompile(`CMD_PID=(\d+) READY`)
)

type (
	// viewProbeMsg is a message the viewer ignores; it lets a test filter
	// look at the view.
	viewProbeMsg struct{}

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

	// syscallFile reaches a file's descriptor through its SyscallConn, as
	// xpty.UnixPty's Control does for the PTY master.
	syscallFile struct{ *os.File }
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

// TestInteractiveSessionLastLineAboveBanner runs `seq 1 50; echo LAST-LINE`
// through the real adapter (PTY start seam, session, waiter) and checks, on
// every run, that the completed view shows LAST-LINE above the banner (spec
// "Trailing output stays above the banner").
func TestInteractiveSessionLastLineAboveBanner(t *testing.T) {
	t.Parallel()

	for i := range lastLineRuns {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			t.Parallel()
			view := runAdapterToCompletion(t, `seq 1 50; echo LAST-LINE`)
			last := strings.LastIndex(view, "LAST-LINE")
			banner := strings.Index(view, "COMPLETED SUCCESSFULLY")
			if last < 0 || banner < 0 || last > banner {
				t.Fatalf("LAST-LINE at %d, banner at %d, want LAST-LINE above the banner; view:\n%s", last, banner, view)
			}
		})
	}
}

// runAdapterToCompletion runs script through runInteractiveCmd without a
// real terminal, presses Enter once the completed view is shown, and returns
// that view's text.
func runAdapterToCompletion(t *testing.T, script string) string {
	t.Helper()

	var (
		mu        sync.Mutex
		lastView  string
		completed = make(chan struct{})
		once      sync.Once
		program   = make(chan *tea.Program, 1)
	)
	filter := func(m tea.Model, msg tea.Msg) tea.Msg {
		content := m.View().Content
		mu.Lock()
		lastView = content
		mu.Unlock()
		if strings.Contains(content, "Press Enter to return to terminal") {
			once.Do(func() { close(completed) })
		}
		return msg
	}
	opts := tui.InteractiveOptions{
		Title:             "probe",
		StatusClearWriter: io.Discard,
		OnProgramReady:    func(p *tea.Program) { program <- p },
		ProgramOptions: []tea.ProgramOption{
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(100, 30),
			tea.WithFilter(filter),
		},
	}
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script)
	type outcome struct {
		result *tui.InteractiveResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		r, err := runInteractiveCmd(t.Context(), opts, cmd)
		done <- outcome{r, err}
	}()
	p := <-program
	// The filter sees the model only when a message arrives, so poke it
	// until the completed view shows.
	deadline := time.After(ptyTestTimeout)
	for waiting := true; waiting; {
		select {
		case <-completed:
			waiting = false
		case <-deadline:
			t.Fatalf("completed view not shown within %s", ptyTestTimeout)
		case <-time.After(ptyTestPollInterval):
			p.Send(viewProbeMsg{})
		}
	}
	p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	var o outcome
	select {
	case o = <-done:
	case <-time.After(ptyTestTimeout):
		t.Fatalf("session did not return within %s after Enter", ptyTestTimeout)
	}
	if o.err != nil || o.result == nil || o.result.ExitCode != 0 {
		t.Fatalf("runInteractiveCmd() = %+v, %v; want success", o.result, o.err)
	}
	mu.Lock()
	defer mu.Unlock()
	return lastView
}

// TestInteractiveForceQuitHangsUpNonReadingCommand is the probe of design
// D8 (spec "Force quit of a command that never reads its input"): the
// command never reads its input, a resize and a 64 KiB paste fill its
// terminal's input so the viewer's writer blocks on the master, and Ctrl+\
// ends the viewer. The command must get SIGHUP within hupProbeBound while
// this test process keeps running. With a blocking master, the blocked write
// (and the blocked read of a command that writes nothing) keep the master
// open after the adapter closes it, so SIGHUP came only when invowk exited.
func TestInteractiveForceQuitHangsUpNonReadingCommand(t *testing.T) {
	t.Parallel()

	hupFile := filepath.Join(t.TempDir(), "hup.txt")
	script := `stty raw -echo; trap 'echo HUP > "$HUP_FILE"; exit 1' HUP; echo "CMD_PID=$$ READY"; while :; do sleep 1; done`

	var (
		mu       sync.Mutex
		lastView string
		program  = make(chan *tea.Program, 1)
	)
	filter := func(m tea.Model, msg tea.Msg) tea.Msg {
		mu.Lock()
		lastView = m.View().Content
		mu.Unlock()
		return msg
	}
	view := func() string {
		mu.Lock()
		defer mu.Unlock()
		return lastView
	}
	opts := tui.InteractiveOptions{
		Title:             "hup probe",
		StatusClearWriter: io.Discard,
		OnProgramReady:    func(p *tea.Program) { program <- p },
		ProgramOptions: []tea.ProgramOption{
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(100, 30),
			tea.WithFilter(filter),
		},
	}
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script)
	cmd.Env = append(os.Environ(), "HUP_FILE="+hupFile)
	done := make(chan error, 1)
	go func() {
		_, err := runInteractiveCmd(t.Context(), opts, cmd)
		done <- err
	}()
	p := <-program

	var pid int
	deadline := time.Now().Add(ptyTestTimeout)
	for pid == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("command not ready within %s; view:\n%s", ptyTestTimeout, view())
		}
		p.Send(viewProbeMsg{})
		time.Sleep(ptyTestPollInterval)
		if m := cmdPIDPattern.FindStringSubmatch(view()); len(m) == 2 {
			pid, _ = strconv.Atoi(m[1])
		}
	}
	t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })

	p.Send(tea.WindowSizeMsg{Width: 90, Height: 28})
	p.Send(tea.PasteMsg{Content: strings.Repeat("x", 64<<10)})
	// Let the writer reach the master and block on the full input.
	time.Sleep(200 * time.Millisecond)
	p.Send(tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runInteractiveCmd() error = %v after Ctrl+\\", err)
		}
	case <-time.After(ptyTestTimeout):
		t.Fatalf("session did not return within %s after Ctrl+\\", ptyTestTimeout)
	}
	hupDeadline := time.Now().Add(hupProbeBound)
	for {
		if data, err := os.ReadFile(hupFile); err == nil && strings.Contains(string(data), "HUP") {
			break
		}
		if time.Now().After(hupDeadline) {
			t.Fatalf("command got no SIGHUP within %s of the viewer ending (process %d alive: %t)",
				hupProbeBound, pid, processAlive(pid))
		}
		time.Sleep(ptyTestPollInterval)
	}
	exitDeadline := time.Now().Add(hupProbeBound)
	for processAlive(pid) {
		if time.Now().After(exitDeadline) {
			t.Fatalf("command %d still running %s after its SIGHUP", pid, hupProbeBound)
		}
		time.Sleep(ptyTestPollInterval)
	}
}

// TestDuplicatePolledNonBlockingRefusesUnpollableFile checks the fallback of
// viewerTerminal: for a descriptor the runtime poller refuses, the helper
// returns nil and leaves the shared open file description blocking, so the
// original file still waits in Read and Write instead of failing with EAGAIN.
// Linux's epoll refuses a regular file; kqueue on the BSDs and Darwin accepts
// one, so the case cannot be built there from a plain file.
func TestDuplicatePolledNonBlockingRefusesUnpollableFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("only Linux's epoll is known to refuse a regular file")
	}

	file, err := os.CreateTemp(t.TempDir(), "unpollable")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer file.Close()
	if master := duplicatePolledNonBlocking(syscallFile{file}); master != nil {
		_ = master.Close()
		t.Fatal("duplicatePolledNonBlocking() returned a file the poller refused")
	}
	conn, err := file.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn() error = %v", err)
	}
	var flags int
	var flagsErr error
	if err := conn.Control(func(fd uintptr) {
		flags, flagsErr = unix.FcntlInt(fd, unix.F_GETFL, 0)
	}); err != nil || flagsErr != nil {
		t.Fatalf("reading the file's flags: %v, %v", err, flagsErr)
	}
	if flags&unix.O_NONBLOCK != 0 {
		t.Fatal("the shared open file description stayed non-blocking after the fallback")
	}
}

// TestDuplicatePolledNonBlockingPolledPtyMaster checks that a duplicate of a PTY
// master is registered with the poller (its deadline can be set), which the
// early hang-up of TestInteractiveForceQuitHangsUpNonReadingCommand needs.
func TestDuplicatePolledNonBlockingPolledPtyMaster(t *testing.T) {
	t.Parallel()

	pty, err := xpty.NewPty(80, 24)
	if err != nil {
		t.Fatalf("NewPty() error = %v", err)
	}
	defer pty.Close()
	terminal := viewerTerminal(pty)
	nonBlocking, ok := terminal.(*nonBlockingTerminal)
	if !ok {
		t.Fatalf("viewerTerminal() = %T, want *nonBlockingTerminal", terminal)
	}
	if err := nonBlocking.master.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("SetReadDeadline() on the duplicate error = %v", err)
	}
	if err := nonBlocking.master.Close(); err != nil {
		t.Fatalf("closing the duplicate: %v", err)
	}
}

func (f syscallFile) Control(fn func(fd uintptr)) error {
	conn, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("syscall conn: %w", err)
	}
	if err := conn.Control(fn); err != nil {
		return fmt.Errorf("control: %w", err)
	}
	return nil
}
