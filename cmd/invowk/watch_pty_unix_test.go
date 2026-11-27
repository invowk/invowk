// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cmd

import (
	"context"
	"fmt"
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

	if os.Getenv("INVOWK_TEST_WATCH_PTY_HELPER") != "1" {
		return
	}

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
			hasClear := strings.Contains(got, ":clear=1:")
			if hasClear != tc.wantClear {
				t.Fatalf("hasClear = %v, want %v (sig=%v, output=%q)", hasClear, tc.wantClear, tc.sig, got)
			}
		})
	}
}
