// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/ssh"
)

type (
	testSSHSession struct {
		ssh.Session
		ctx      ssh.Context
		command  []string
		environ  []string
		stdin    *bytes.Reader
		stdout   bytes.Buffer
		stderr   bytes.Buffer
		exitCode int
		pty      ssh.Pty
		windows  <-chan ssh.Window
		hasPTY   bool
	}

	interactiveStartCall struct {
		env       []string
		waitDelay time.Duration
	}

	interactiveCopyCall struct {
		dst io.Writer
		src io.Reader
	}

	interactiveResizeCall struct {
		width  int
		height int
	}

	interactiveShellCase struct {
		name     string
		exitCode string
		wantExit int
	}

	interactiveShellFixture struct {
		t           *testing.T
		srv         *Server
		ptyFile     *os.File
		startCalls  chan interactiveStartCall
		copyCalls   chan interactiveCopyCall
		resizeCalls chan interactiveResizeCall
		commandName string
	}
)

func newTestSSHSession(t *testing.T, command, environ []string, stdin string) *testSSHSession {
	t.Helper()
	return &testSSHSession{
		ctx: newStubSSHContext(t.Context()), command: command, environ: environ,
		stdin: bytes.NewReader([]byte(stdin)), exitCode: -1,
	}
}

func (s *testSSHSession) Read(p []byte) (int, error)  { return s.stdin.Read(p) }
func (s *testSSHSession) Write(p []byte) (int, error) { return s.stdout.Write(p) }
func (s *testSSHSession) Stderr() io.ReadWriter       { return &s.stderr }
func (s *testSSHSession) Context() ssh.Context        { return s.ctx }
func (s *testSSHSession) Command() []string           { return s.command }
func (s *testSSHSession) Environ() []string           { return s.environ }
func (s *testSSHSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	return s.pty, s.windows, s.hasPTY
}

func (s *testSSHSession) Exit(code int) error {
	s.exitCode = code
	return nil
}

func TestCommandMiddlewareDispatch(t *testing.T) {
	t.Parallel()

	t.Run("interactive shell", func(t *testing.T) {
		t.Parallel()
		var gotName string
		srv, err := newWithDependencies(DefaultConfig(), realClock{}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
			gotName = name
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-shell"), args...)
		})
		if err != nil {
			t.Fatalf("newWithDependencies() error = %v", err)
		}
		session := newTestSSHSession(t, nil, nil, "")
		srv.commandMiddleware()(nil)(session)
		if gotName != string(DefaultConfig().DefaultShell) {
			t.Errorf("interactive command = %q, want %q", gotName, DefaultConfig().DefaultShell)
		}
		if session.exitCode != 1 || !strings.Contains(session.stderr.String(), "Error starting shell:") {
			t.Errorf("interactive failure = exit %d, stderr %q; want exit 1 launch diagnostic", session.exitCode, session.stderr.String())
		}
	})

	t.Run("direct command", func(t *testing.T) {
		t.Parallel()
		srv := newCommandTestServer(t, nil)
		session := newTestSSHSession(t, []string{"tool", "arg"}, commandHelperEnv("ok", "", "0"), "")
		srv.commandMiddleware()(nil)(session)
		if session.exitCode != 0 || session.stdout.String() != "ok" {
			t.Errorf("direct command = exit %d, stdout %q; want exit 0 and output", session.exitCode, session.stdout.String())
		}
	})
}

func TestRunCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      string
		stdout     string
		stderr     string
		exit       string
		wantName   string
		wantArgs   []string
		wantExit   int
		wantStderr string
	}{
		{name: "single argument uses shell", args: []string{"echo hi"}, stdout: "shell", exit: "0", wantName: string(DefaultConfig().DefaultShell), wantArgs: []string{"-c", "echo hi"}, wantExit: 0},
		{name: "multiple arguments execute directly", args: []string{"tool", "one", "two"}, stdin: "input", stdout: "direct:", exit: "0", wantName: "tool", wantArgs: []string{"one", "two"}, wantExit: 0},
		{name: "exit status propagates", args: []string{"tool", "bad"}, stderr: "failed", exit: "7", wantName: "tool", wantArgs: []string{"bad"}, wantExit: 7, wantStderr: "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var gotName string
			var gotArgs []string
			srv := newCommandTestServer(t, func(name string, args []string) {
				gotName = name
				gotArgs = append([]string(nil), args...)
			})
			session := newTestSSHSession(t, tt.args, commandHelperEnv(tt.stdout, tt.stderr, tt.exit), tt.stdin)
			srv.runCommand(session, tt.args)

			if gotName != tt.wantName || strings.Join(gotArgs, "\x00") != strings.Join(tt.wantArgs, "\x00") {
				t.Errorf("command = %q %q, want %q %q", gotName, gotArgs, tt.wantName, tt.wantArgs)
			}
			wantStdout := tt.stdout + tt.stdin
			if got := session.stdout.String(); got != wantStdout {
				t.Errorf("stdout = %q, want %q", got, wantStdout)
			}
			if got := session.stderr.String(); got != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", got, tt.wantStderr)
			}
			if session.exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", session.exitCode, tt.wantExit)
			}
		})
	}
}

func TestRunCommandLaunchFailure(t *testing.T) {
	t.Parallel()

	srv, err := newWithDependencies(DefaultConfig(), realClock{}, func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-command"))
	})
	if err != nil {
		t.Fatalf("newWithDependencies() error = %v", err)
	}
	session := newTestSSHSession(t, []string{"tool", "arg"}, nil, "")
	srv.runCommand(session, session.command)
	if session.exitCode != 1 || !strings.Contains(session.stderr.String(), "Error:") {
		t.Errorf("launch failure = exit %d, stderr %q; want exit 1 diagnostic", session.exitCode, session.stderr.String())
	}
}

func TestRunInteractiveShell(t *testing.T) {
	t.Parallel()

	tests := []interactiveShellCase{
		{name: "successful shell", exitCode: "0", wantExit: 0},
		{name: "nonzero shell exit", exitCode: "7", wantExit: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runInteractiveShellCase(t, tt)
		})
	}
}

func runInteractiveShellCase(t *testing.T, tt interactiveShellCase) {
	t.Helper()

	fixture := newInteractiveShellFixture(t)
	session := newInteractiveShellSession(t, tt.exitCode)
	fixture.srv.runInteractiveShell(session)

	assertInteractiveShellCommand(t, fixture.commandName, session.exitCode, tt.wantExit)
	assertInteractiveShellStart(t, receiveInteractiveCall(t, fixture.startCalls, "PTY start"))
	assertInteractiveShellResize(t, receiveInteractiveCall(t, fixture.resizeCalls, "window resize"))
	assertInteractiveShellCopies(t, fixture, session)
	assertPTYClosed(t, fixture.ptyFile)
}

func newInteractiveShellFixture(t *testing.T) *interactiveShellFixture {
	t.Helper()

	fixture := &interactiveShellFixture{
		t:           t,
		startCalls:  make(chan interactiveStartCall, 1),
		copyCalls:   make(chan interactiveCopyCall, 2),
		resizeCalls: make(chan interactiveResizeCall, 1),
	}
	fixture.srv = newCommandTestServer(t, fixture.recordCommand)
	ptyFile, err := os.CreateTemp(t.TempDir(), "interactive-pty-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	fixture.ptyFile = ptyFile
	fixture.srv.startPTY = fixture.startPTY
	fixture.srv.copyBuffer = fixture.copyBuffer
	fixture.srv.setWinsize = fixture.setWinsize
	return fixture
}

func (f *interactiveShellFixture) recordCommand(name string, args []string) {
	f.commandName = name
	if len(args) != 0 {
		f.t.Errorf("interactive shell args = %q, want none", args)
	}
}

func (f *interactiveShellFixture) startPTY(cmd *exec.Cmd) (*os.File, error) {
	f.startCalls <- interactiveStartCall{env: slices.Clone(cmd.Env), waitDelay: cmd.WaitDelay}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return f.ptyFile, nil
}

func (f *interactiveShellFixture) copyBuffer(dst io.Writer, src io.Reader) (int64, error) {
	f.copyCalls <- interactiveCopyCall{dst: dst, src: src}
	return 0, nil
}

func (f *interactiveShellFixture) setWinsize(_ *os.File, width, height int) {
	f.resizeCalls <- interactiveResizeCall{width: width, height: height}
}

func newInteractiveShellSession(t *testing.T, exitCode string) *testSSHSession {
	t.Helper()

	windows := make(chan ssh.Window, 1)
	windows <- ssh.Window{Width: 132, Height: 43}
	close(windows)
	session := newTestSSHSession(t, nil, commandHelperEnv("", "", exitCode), "input")
	session.environ = append(session.environ, "SESSION_VALUE=present")
	session.pty = ssh.Pty{Term: "xterm-256color"}
	session.windows = windows
	session.hasPTY = true
	return session
}

func assertInteractiveShellCommand(t *testing.T, commandName string, gotExit, wantExit int) {
	t.Helper()

	if commandName != string(DefaultConfig().DefaultShell) {
		t.Errorf("interactive shell command = %q, want %q", commandName, DefaultConfig().DefaultShell)
	}
	if gotExit != wantExit {
		t.Errorf("interactive shell exit = %d, want %d", gotExit, wantExit)
	}
}

func assertInteractiveShellStart(t *testing.T, start interactiveStartCall) {
	t.Helper()

	if start.waitDelay != cmdWaitDelay {
		t.Errorf("WaitDelay = %v, want %v", start.waitDelay, cmdWaitDelay)
	}
	for _, env := range []string{"SESSION_VALUE=present", "TERM=xterm-256color"} {
		if !slices.Contains(start.env, env) {
			t.Errorf("command env missing %q: %v", env, start.env)
		}
	}
}

func assertInteractiveShellResize(t *testing.T, resize interactiveResizeCall) {
	t.Helper()
	if resize.width != 132 || resize.height != 43 {
		t.Errorf("resize = %dx%d, want 132x43", resize.width, resize.height)
	}
}

func assertInteractiveShellCopies(t *testing.T, fixture *interactiveShellFixture, session *testSSHSession) {
	t.Helper()

	copies := []interactiveCopyCall{
		receiveInteractiveCall(t, fixture.copyCalls, "first stream copy"),
		receiveInteractiveCall(t, fixture.copyCalls, "second stream copy"),
	}
	if !hasInteractiveCopy(copies, fixture.ptyFile, session) || !hasInteractiveCopy(copies, session, fixture.ptyFile) {
		t.Errorf("copy calls = %#v, want session->PTY and PTY->session", copies)
	}
}

func assertPTYClosed(t *testing.T, ptyFile *os.File) {
	t.Helper()
	if _, err := ptyFile.WriteString("closed"); !errors.Is(err, os.ErrClosed) {
		t.Errorf("PTY write after run error = %v, want os.ErrClosed", err)
	}
}

func receiveInteractiveCall[T any](t *testing.T, calls <-chan T, description string) T {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		var zero T
		return zero
	}
}

func hasInteractiveCopy(calls []interactiveCopyCall, dst io.Writer, src io.Reader) bool {
	return slices.ContainsFunc(calls, func(call interactiveCopyCall) bool {
		return call.dst == dst && call.src == src
	})
}

func newCommandTestServer(t *testing.T, record func(string, []string)) *Server {
	t.Helper()
	srv, err := newWithDependencies(DefaultConfig(), realClock{}, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if record != nil {
			record(name, args)
		}
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSSHCommandHelper$")
	})
	if err != nil {
		t.Fatalf("newWithDependencies() error = %v", err)
	}
	return srv
}

func commandHelperEnv(stdout, stderr, exit string) []string {
	return []string{
		"GO_WANT_SSH_COMMAND_HELPER=1",
		"SSH_COMMAND_HELPER_STDOUT=" + stdout,
		"SSH_COMMAND_HELPER_STDERR=" + stderr,
		"SSH_COMMAND_HELPER_EXIT=" + exit,
	}
}

func TestSSHCommandHelper(_ *testing.T) { //nolint:paralleltest // helper process entrypoint exits the subprocess.
	if os.Getenv("GO_WANT_SSH_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, os.Getenv("SSH_COMMAND_HELPER_STDOUT"))
	_, _ = io.Copy(os.Stdout, os.Stdin)
	_, _ = fmt.Fprint(os.Stderr, os.Getenv("SSH_COMMAND_HELPER_STDERR"))
	exitCode := 0
	if _, err := fmt.Sscanf(os.Getenv("SSH_COMMAND_HELPER_EXIT"), "%d", &exitCode); err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}
