// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/watch"
)

func TestWatchPTYHelper(t *testing.T) {
	t.Parallel()

	mode := os.Getenv("INVOWK_TEST_WATCH_PTY_HELPER")
	switch mode {
	case "1":
		ctx, cancel := signal.NotifyContext(t.Context(), os.Interrupt)
		defer cancel()

		watchers := &fakeWatchFactory{
			run: func(runCtx context.Context, _ watch.Config) error {
				fmt.Println("WATCH_READY")
				<-runCtx.Done()
				return runCtx.Err()
			},
		}
		app, err := NewApp(Dependencies{
			Config:   &staticConfigProvider{cfg: config.DefaultConfig()},
			Watchers: watchers,
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		cmd := newWatchTestCmd(t)
		cmd.SetContext(contextWithDiscoveryScope(ctx, "", false))
		_ = runWatchMode(cmd, app, &rootFlagValues{}, &cmdFlagValues{watch: true}, []string{"check-token"})
	case "2":
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		rootCmd := NewRootCommand(app)
		rootCmd.SetArgs([]string{"cmd", "check-token"})
		if execErr := rootCmd.ExecuteContext(t.Context()); execErr != nil {
			t.Fatalf("rootCmd.ExecuteContext() error = %v", execErr)
		}
	default:
		return
	}
}

func TestRunWatchMode_PTYSignalsAndChildStripping(t *testing.T) {
	t.Parallel()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	tmpDir := t.TempDir()
	cueContent := `cmds: [
	{
		name: "check-token"
		implementations: [
			{
				script: {content: "printf 'CHILD_TOK=%s\\n' \"${OSC7577_TOKEN:-empty}\""}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
	}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(cueContent), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}

	tests := []struct {
		name      string
		sig       syscall.Signal
		wantClear bool
	}{
		{name: "SIGINT clears record", sig: syscall.SIGINT, wantClear: true},
		{name: "SIGHUP preserves record", sig: syscall.SIGHUP, wantClear: false},
		{name: "SIGTERM preserves record", sig: syscall.SIGTERM, wantClear: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestWatchPTYHelper$", "--", "cmd", "--ivk-watch", "check-token")
			cmd.Dir = tmpDir
			cleanEnv := make([]string, 0, len(os.Environ())+2)
			for _, kv := range os.Environ() {
				if !strings.HasPrefix(kv, ansi.ProgramResumeTokenEnv+"=") {
					cleanEnv = append(cleanEnv, kv)
				}
			}
			cleanEnv = append(cleanEnv,
				"INVOWK_TEST_WATCH_PTY_HELPER=1",
				ansi.ProgramResumeTokenEnv+"=pty-secret-token",
			)
			cmd.Env = cleanEnv

			ptmx, err := pty.Start(cmd)
			if err != nil {
				t.Fatalf("pty.Start() error = %v", err)
			}
			defer func() { _ = ptmx.Close() }()

			var output strings.Builder
			readDone := make(chan struct{})
			readyCh := make(chan struct{}, 1)
			go func() {
				defer close(readDone)
				buf := make([]byte, 4096)
				notified := false
				for {
					n, readErr := ptmx.Read(buf)
					if n > 0 {
						output.Write(buf[:n])
						if !notified && strings.Contains(output.String(), "WATCH_READY") {
							notified = true
							readyCh <- struct{}{}
						}
					}
					if readErr != nil {
						return
					}
				}
			}()

			select {
			case <-readyCh:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				<-readDone
				t.Fatalf("timed out waiting for WATCH_READY; output=%q", output.String())
			}

			if err := cmd.Process.Signal(tc.sig); err != nil {
				t.Fatalf("Signal(%v) error = %v", tc.sig, err)
			}

			waitCh := make(chan error, 1)
			go func() { waitCh <- cmd.Wait() }()
			select {
			case waitErr := <-waitCh:
				if waitErr != nil && !strings.Contains(waitErr.Error(), "exit status") {
					t.Fatalf("unexpected Wait() error = %v", waitErr)
				}
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatalf("timed out waiting for process exit after %v; output=%q", tc.sig, output.String())
			}
			<-readDone

			got := output.String()
			if !strings.Contains(got, "CHILD_TOK=empty") {
				t.Fatalf("PTY output missing CHILD_TOK=empty: %q", got)
			}
			if !strings.Contains(got, "\x1b]7577;") || !strings.Contains(got, ":argv=") {
				t.Fatalf("PTY output missing signed OSC 7577 argv report: %q", got)
			}
			for _, wantStatus := range []string{"\x1b]7501;state=working:app=invowk:", "\x1b]7501;state=idle:app=invowk:", ansi.ClearProgramStatus} {
				if !strings.Contains(got, wantStatus) {
					t.Fatalf("PTY output missing OSC 7501 %q in %q", wantStatus, got)
				}
			}
			hasClear := strings.Contains(got, ":clear=1:")
			if hasClear != tc.wantClear {
				t.Fatalf("hasClear = %v, want %v (sig=%v, output=%q)", hasClear, tc.wantClear, tc.sig, got)
			}
		})
	}

	t.Run("redirected stdout keeps OSC 7501 and OSC 7577 on /dev/tty only", func(t *testing.T) {
		t.Parallel()

		ptmx, pts, openErr := pty.Open()
		if openErr != nil {
			t.Fatalf("pty.Open() error = %v", openErr)
		}
		defer func() { _ = ptmx.Close() }()

		stdoutR, stdoutW, pipeErr := os.Pipe()
		if pipeErr != nil {
			_ = pts.Close()
			t.Fatalf("os.Pipe() error = %v", pipeErr)
		}

		cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestWatchPTYHelper$", "--", "cmd", "--ivk-watch", "check-token")
		cmd.Dir = tmpDir
		cleanEnv := make([]string, 0, len(os.Environ())+2)
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, ansi.ProgramResumeTokenEnv+"=") {
				cleanEnv = append(cleanEnv, kv)
			}
		}
		cleanEnv = append(cleanEnv,
			"INVOWK_TEST_WATCH_PTY_HELPER=1",
			ansi.ProgramResumeTokenEnv+"=pty-secret-token",
		)
		cmd.Env = cleanEnv
		cmd.Stdin = pts
		cmd.Stderr = pts
		cmd.Stdout = stdoutW
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0,
		}

		if startErr := cmd.Start(); startErr != nil {
			_ = pts.Close()
			_ = stdoutW.Close()
			_ = stdoutR.Close()
			t.Fatalf("cmd.Start() error = %v", startErr)
		}
		_ = pts.Close()
		_ = stdoutW.Close()
		defer func() { _ = stdoutR.Close() }()

		var stdoutBuf bytes.Buffer
		stdoutDone := make(chan struct{})
		readyCh := make(chan struct{}, 1)
		go func() {
			defer close(stdoutDone)
			buf := make([]byte, 4096)
			notified := false
			for {
				n, readErr := stdoutR.Read(buf)
				if n > 0 {
					stdoutBuf.Write(buf[:n])
					if !notified && strings.Contains(stdoutBuf.String(), "WATCH_READY") {
						notified = true
						readyCh <- struct{}{}
					}
				}
				if readErr != nil {
					return
				}
			}
		}()

		var ptyBuf bytes.Buffer
		ptyDone := make(chan struct{})
		go func() {
			defer close(ptyDone)
			_, _ = io.Copy(&ptyBuf, ptmx)
		}()

		select {
		case <-readyCh:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-stdoutDone
			<-ptyDone
			t.Fatalf("timed out waiting for WATCH_READY; stdout=%q pty=%q", stdoutBuf.String(), ptyBuf.String())
		}

		if sigErr := cmd.Process.Signal(syscall.SIGINT); sigErr != nil {
			t.Fatalf("Signal(SIGINT) error = %v", sigErr)
		}

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()
		select {
		case waitErr := <-waitCh:
			if waitErr != nil && !strings.Contains(waitErr.Error(), "exit status") {
				t.Fatalf("unexpected Wait() error = %v", waitErr)
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("timed out waiting for process exit")
		}
		<-stdoutDone
		<-ptyDone

		stdoutGot := stdoutBuf.String()
		if !strings.Contains(stdoutGot, "WATCH_READY") {
			t.Fatalf("redirected stdout missing WATCH_READY: %q", stdoutGot)
		}
		for _, forbidden := range []string{"\x1b]7501;", "\x1b]7577;"} {
			if strings.Contains(stdoutGot, forbidden) {
				t.Fatalf("redirected stdout leaked terminal sequence %q: %q", forbidden, stdoutGot)
			}
		}

		ptyGot := ptyBuf.String()
		for _, wantTTY := range []string{"\x1b]7501;state=working:app=invowk:", "\x1b]7501;state=idle:app=invowk:", ansi.ClearProgramStatus, "\x1b]7577;"} {
			if !strings.Contains(ptyGot, wantTTY) {
				t.Fatalf("/dev/tty PTY output missing %q in %q", wantTTY, ptyGot)
			}
		}
	})

	t.Run("non-interactive invowk cmd on PTY emits zero OSC 7501", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestWatchPTYHelper$")
		cmd.Dir = tmpDir
		cleanEnv := make([]string, 0, len(os.Environ())+2)
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, ansi.ProgramResumeTokenEnv+"=") {
				cleanEnv = append(cleanEnv, kv)
			}
		}
		cleanEnv = append(cleanEnv,
			"INVOWK_TEST_WATCH_PTY_HELPER=2",
			ansi.ProgramResumeTokenEnv+"=pty-secret-token",
		)
		cmd.Env = cleanEnv

		ptmx, startErr := pty.Start(cmd)
		if startErr != nil {
			t.Fatalf("pty.Start() error = %v", startErr)
		}
		defer func() { _ = ptmx.Close() }()

		var out bytes.Buffer
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			_, _ = io.Copy(&out, ptmx)
		}()

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()
		select {
		case waitErr := <-waitCh:
			if waitErr != nil {
				t.Fatalf("Wait() error = %v; output=%q", waitErr, out.String())
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("timed out waiting for non-interactive PTY command; output=%q", out.String())
		}
		<-readDone

		got := out.String()
		if !strings.Contains(got, "CHILD_TOK=pty-secret-token") {
			t.Fatalf("non-interactive PTY output missing command output CHILD_TOK=pty-secret-token: %q", got)
		}
		for _, forbidden := range []string{"\x1b]7501;", "\x1b]7577;"} {
			if strings.Contains(got, forbidden) {
				t.Fatalf("non-interactive PTY output unexpectedly emitted %q: %q", forbidden, got)
			}
		}
	})
}
