// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/watch"
	"github.com/invowk/invowk/pkg/types"
)

type watchTerminalFunc func() (io.Writer, func(), bool)

func (f watchTerminalFunc) OpenWatchTerminal() (io.Writer, func(), bool) {
	return f()
}

func TestWatchResumeReporter_ValidSignedSequenceAndClear(t *testing.T) {
	t.Parallel()

	var termBuf bytes.Buffer
	closed := false
	dir := filepath.Join(t.TempDir(), "work")
	exePath := filepath.Join(t.TempDir(), "invowk")
	token := "test-resume-secret"
	fixedNow := time.Unix(1700000100, 0)

	app := &App{
		WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
			return &termBuf, func() { closed = true }, true
		}),
		Getenv: func(key string) string {
			if key == ansi.ProgramResumeTokenEnv {
				return token
			}
			return ""
		},
	}

	reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
		rawArgs:    []string{"cmd", "--ivk-watch", "build", "--flag=1"},
		executable: func() (string, error) { return exePath, nil },
		getwd:      func() (string, error) { return dir, nil },
		getpid:     func() int { return 4242 },
		now:        func() time.Time { return fixedNow },
	})

	reporter.Report()
	gotReport := termBuf.String()
	wantReport := ansi.SetProgramResume(token, 4242, fixedNow, ansi.ProgramResume{
		Args: []string{exePath, "cmd", "--ivk-watch", "build", "--flag=1"},
		Dir:  dir,
	})
	if gotReport != wantReport {
		t.Fatalf("Report() = %q, want %q", gotReport, wantReport)
	}
	verifyOSC7577MAC(t, gotReport, token)

	termBuf.Reset()
	reporter.Clear()
	gotClear := termBuf.String()
	wantClear := ansi.ClearProgramResume(token, 4242, fixedNow)
	if gotClear != wantClear {
		t.Fatalf("Clear() = %q, want %q", gotClear, wantClear)
	}
	verifyOSC7577MAC(t, gotClear, token)

	// Calling Clear again after clearing must be a no-op.
	termBuf.Reset()
	reporter.Clear()
	if termBuf.Len() != 0 {
		t.Fatalf("second Clear() wrote %q, want empty", termBuf.String())
	}

	reporter.Close()
	reporter.Close()
	if !closed {
		t.Fatal("Close() did not run cleanup")
	}
}

func TestWatchResumeReporter_MaterializesEnvScopeAndSkipsDuplicates(t *testing.T) {
	t.Parallel()

	tmpRoot := t.TempDir()
	exePath := filepath.Join(tmpRoot, "invowk")
	cfgPath := filepath.Join(tmpRoot, "custom.cue")
	cwdPath := filepath.Join(tmpRoot, "proj")
	now := time.Unix(1700000200, 0)

	var termBuf bytes.Buffer
	app := &App{
		WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
			return &termBuf, func() {}, true
		}),
		Getenv: func(key string) string {
			switch key {
			case ansi.ProgramResumeTokenEnv:
				return "tok"
			case config.EnvConfigPath:
				return cfgPath
			default:
				return ""
			}
		},
	}

	cmd := newWatchTestCmd(t)
	ctx := contextWithDiscoveryScope(t.Context(), "", true)
	reporter := newWatchResumeReporter(ctx, app, cmd, &rootFlagValues{}, watchResumeOptions{
		rawArgs:    []string{"cmd", "-W", "dev"},
		executable: func() (string, error) { return exePath, nil },
		getwd:      func() (string, error) { return cwdPath, nil },
		getpid:     func() int { return 99 },
		now:        func() time.Time { return now },
	})
	reporter.Report()

	want := ansi.SetProgramResume("tok", 99, now, ansi.ProgramResume{
		Args: []string{exePath, "cmd", "--ivk-config=" + cfgPath, "--ivk-no-cwd", "-W", "dev"},
		Dir:  cwdPath,
	})
	if got := termBuf.String(); got != want {
		t.Fatalf("Report() with env scope = %q, want %q", got, want)
	}

	// Explicit CLI flags must not be duplicated.
	noDup := buildWatchResumeArgs(
		[]string{"cmd", "--ivk-config", cfgPath, "--ivk-no-cwd", "-W", "dev"},
		exePath,
		cfgPath,
		cfgPath,
		true,
	)
	wantNoDup := []string{exePath, "cmd", "--ivk-config", cfgPath, "--ivk-no-cwd", "-W", "dev"}
	if strings.Join(noDup, "\x00") != strings.Join(wantNoDup, "\x00") {
		t.Fatalf("buildWatchResumeArgs duplicate check = %q, want %q", noDup, wantNoDup)
	}
}

func TestWatchResumeReporter_Guardrails(t *testing.T) {
	t.Parallel()

	tmpRoot := t.TempDir()
	exePath := filepath.Join(tmpRoot, "invowk")
	cwdPath := filepath.Join(tmpRoot, "work")
	now := time.Unix(1700000300, 0)

	t.Run("skips when OSC7577_TOKEN missing", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() {}, true
			}),
			Getenv: func(string) string { return "" },
		}
		reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
			rawArgs:    []string{"cmd", "--ivk-watch", "build"},
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})
		reporter.Report()
		reporter.Clear()
		if termBuf.Len() != 0 {
			t.Fatalf("expected no output without token, got %q", termBuf.String())
		}
	})

	t.Run("skips on non-TTY writer", func(t *testing.T) {
		t.Parallel()

		var stdoutBuf bytes.Buffer
		app := &App{
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "secret"
				}
				return ""
			},
			stdout: &stdoutBuf,
		}
		reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
			rawArgs:    []string{"cmd", "--ivk-watch", "build"},
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})
		reporter.Report()
		reporter.Clear()
		if stdoutBuf.Len() != 0 {
			t.Fatalf("non-TTY stdout received %q", stdoutBuf.String())
		}
	})

	t.Run("skips when --ivk-env-var is passed", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetContext(contextWithDiscoveryScope(t.Context(), "", false))
		cmd.Flags().StringArray("ivk-env-var", nil, "")
		if err := cmd.Flags().Set("ivk-env-var", "API_TOKEN=secret"); err != nil {
			t.Fatalf("Flags().Set() error = %v", err)
		}

		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() {}, true
			}),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "secret"
				}
				return ""
			},
		}
		reporter := newWatchResumeReporter(t.Context(), app, cmd, &rootFlagValues{}, watchResumeOptions{
			rawArgs:    []string{"cmd", "--ivk-watch", "--ivk-env-var", "API_TOKEN=secret", "build"},
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})
		reporter.Report()
		if termBuf.Len() != 0 {
			t.Fatalf("expected --ivk-env-var to suppress OSC 7577, got %q", termBuf.String())
		}
	})

	t.Run("skips control-char and oversize args and clears prior active report", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		args := []string{"cmd", "--ivk-watch", "ok"}
		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() {}, true
			}),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "secret"
				}
				return ""
			},
		}
		reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
			rawArgs:    args,
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})

		// Emit valid first, then swap buildResume to control-char/oversized and assert clear=1 without argv.
		reporter.Report()
		if !strings.Contains(termBuf.String(), ":argv=") {
			t.Fatalf("expected initial argv report, got %q", termBuf.String())
		}

		termBuf.Reset()
		reporter.buildResume = func() ansi.ProgramResume {
			return ansi.ProgramResume{Args: []string{exePath, "cmd", "bad\narg"}, Dir: cwdPath}
		}
		reporter.Report()
		if got := termBuf.String(); !strings.Contains(got, ":clear=1:") || strings.Contains(got, ":argv=") {
			t.Fatalf("expected clear=1 only after invalid update, got %q", got)
		}

		// Oversized sequence when no active report must emit nothing at all.
		termBuf.Reset()
		huge := strings.Repeat("a", 2048)
		reporter.buildResume = func() ansi.ProgramResume {
			return ansi.ProgramResume{Args: []string{exePath, huge}, Dir: cwdPath}
		}
		reporter.Report()
		if termBuf.Len() != 0 {
			t.Fatalf("expected oversized un-reported update to emit nothing, got %q", termBuf.String())
		}
	})

	t.Run("skips more than 64 arguments and invalid cwd", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		app := &App{
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) {
				return &termBuf, func() {}, true
			}),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "secret"
				}
				return ""
			},
		}
		reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
			rawArgs:    []string{"cmd", "--ivk-watch", "ok"},
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})

		tooManyArgs := make([]string, 65)
		tooManyArgs[0] = exePath
		for i := 1; i < len(tooManyArgs); i++ {
			tooManyArgs[i] = "a"
		}
		reporter.buildResume = func() ansi.ProgramResume {
			return ansi.ProgramResume{Args: tooManyArgs, Dir: cwdPath}
		}
		reporter.Report()
		if termBuf.Len() != 0 {
			t.Fatalf(">64 args without prior report emitted %q, want empty", termBuf.String())
		}

		// Valid first, then invalid cwd clears active report.
		reporter.buildResume = func() ansi.ProgramResume {
			return ansi.ProgramResume{Args: []string{exePath, "cmd", "--ivk-watch", "ok"}, Dir: cwdPath}
		}
		reporter.Report()
		if !strings.Contains(termBuf.String(), ":argv=") {
			t.Fatalf("expected valid report, got %q", termBuf.String())
		}

		termBuf.Reset()
		reporter.buildResume = func() ansi.ProgramResume {
			return ansi.ProgramResume{Args: []string{exePath, "cmd"}, Dir: cwdPath + "_bad\ncwd"}
		}
		reporter.Report()
		if got := termBuf.String(); !strings.Contains(got, ":clear=1:") || strings.Contains(got, ":argv=") {
			t.Fatalf("expected clear=1 only after invalid cwd, got %q", got)
		}
	})

	t.Run("skips non-TTY os.File pipe via defaultWatchTerminalOpener", func(t *testing.T) {
		t.Parallel()

		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			_ = pr.Close()
			_ = pw.Close()
		})

		app := &App{
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "secret"
				}
				return ""
			},
			stdout: pw,
		}
		reporter := newWatchResumeReporter(t.Context(), app, newWatchTestCmd(t), &rootFlagValues{}, watchResumeOptions{
			rawArgs:    []string{"cmd", "--ivk-watch", "build"},
			executable: func() (string, error) { return exePath, nil },
			getwd:      func() (string, error) { return cwdPath, nil },
			getpid:     func() int { return 10 },
			now:        func() time.Time { return now },
		})
		reporter.Report()
		if closeErr := pw.Close(); closeErr != nil {
			t.Fatalf("pw.Close() error = %v", closeErr)
		}
		pipeOut, err := io.ReadAll(pr)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		if len(pipeOut) != 0 {
			t.Fatalf("non-TTY *os.File pipe received %q", string(pipeOut))
		}
	})
}

func TestRunWatchMode_ResumeLifecycleAndChildTokenStripping(t *testing.T) {
	t.Parallel()

	t.Run("strips token from child requests and clears on voluntary cancel", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		commands := &fakeWatchCommandService{
			cmdInfo:   newResolvedWatchCommand(t),
			exitCodes: []types.ExitCode{0, 0},
		}
		watchers := &fakeWatchFactory{
			run: func(ctx context.Context, cfg watch.Config) error {
				if err := cfg.OnChange(ctx, []string{"main.go"}); err != nil {
					return err
				}
				return context.Canceled
			},
		}
		app := &App{
			Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
			Commands:      commands,
			Watchers:      watchers,
			Diagnostics:   &defaultDiagnosticRenderer{},
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "watch-secret"
				}
				return ""
			},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		err := runWatchMode(
			newWatchTestCmd(t),
			app,
			&rootFlagValues{interactive: true},
			&cmdFlagValues{},
			[]string{"build"},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runWatchMode() error = %v, want context.Canceled", err)
		}
		if len(commands.requests) != 2 {
			t.Fatalf("execute requests = %d, want 2", len(commands.requests))
		}
		for i, req := range commands.requests {
			if !req.StripProgramResumeToken {
				t.Fatalf("requests[%d].StripProgramResumeToken = false, want true", i)
			}
		}

		out := termBuf.String()
		if count := strings.Count(out, ":argv="); count < 2 {
			t.Fatalf("expected initial and interactive re-report argv count >= 2, got %d in %q", count, out)
		}
		if !strings.Contains(out, ":clear=1:") {
			t.Fatalf("expected clear=1 on voluntary/cancelled exit, got %q", out)
		}
	})

	t.Run("clears active record on post-setup error exits", func(t *testing.T) {
		t.Parallel()

		watcherErr := errors.New("watcher failed")
		var termBuf bytes.Buffer
		commands := &fakeWatchCommandService{cmdInfo: newResolvedWatchCommand(t)}
		watchers := &fakeWatchFactory{
			run: func(_ context.Context, _ watch.Config) error { return watcherErr },
		}
		app := &App{
			Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
			Commands:      commands,
			Watchers:      watchers,
			Diagnostics:   &defaultDiagnosticRenderer{},
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "watch-secret"
				}
				return ""
			},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		err := runWatchMode(newWatchTestCmd(t), app, &rootFlagValues{}, &cmdFlagValues{}, []string{"build"})
		if !errors.Is(err, watcherErr) {
			t.Fatalf("runWatchMode() error = %v, want %v", err, watcherErr)
		}
		if !strings.Contains(termBuf.String(), ":argv=") || !strings.Contains(termBuf.String(), ":clear=1:") {
			t.Fatalf("expected argv then clear=1 on watcher error exit, got %q", termBuf.String())
		}
	})

	t.Run("suppresses OSC 7577 when --ivk-env-var is set", func(t *testing.T) {
		t.Parallel()

		var termBuf bytes.Buffer
		cmd := newWatchTestCmd(t)
		cmd.Flags().StringArrayP("ivk-env-var", "E", nil, "")
		if err := cmd.Flags().Set("ivk-env-var", "SECRET=value"); err != nil {
			t.Fatalf("Flags().Set() error = %v", err)
		}

		app := &App{
			Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
			Commands:      &fakeWatchCommandService{cmdInfo: newResolvedWatchCommand(t)},
			Watchers:      &fakeWatchFactory{},
			Diagnostics:   &defaultDiagnosticRenderer{},
			WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
			Getenv: func(key string) string {
				if key == ansi.ProgramResumeTokenEnv {
					return "watch-secret"
				}
				return ""
			},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		if err := runWatchMode(cmd, app, &rootFlagValues{}, &cmdFlagValues{}, []string{"build"}); err != nil {
			t.Fatalf("runWatchMode() error = %v", err)
		}
		if termBuf.Len() != 0 {
			t.Fatalf("expected --ivk-env-var to suppress OSC 7577, got %q", termBuf.String())
		}
	})
}

func TestRunWatchMode_RealNativeChildStripsHostToken(t *testing.T) {
	tmpDir := t.TempDir()
	cueContent := `cmds: [
	{
		name: "check-token"
		implementations: [
			{
				script: {content: "printf 'TOK=%s\\n' \"${OSC7577_TOKEN:-empty}\" >> child.out"}
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
			{
				script: {content: "$v = if ($env:OSC7577_TOKEN) { \"TOK=$($env:OSC7577_TOKEN)\" } else { 'TOK=empty' }; Add-Content -Path child.out -Value $v"}
				runtimes: [{name: "native"}]
				platforms: [{name: "windows"}]
			}
		]
	}
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(cueContent), 0o600); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}

	t.Setenv(ansi.ProgramResumeTokenEnv, "watch-host-token")
	t.Chdir(tmpDir)

	var termBuf bytes.Buffer
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	watchers := &fakeWatchFactory{
		run: func(ctx context.Context, cfg watch.Config) error {
			if err := cfg.OnChange(ctx, []string{"invowkfile.cue"}); err != nil {
				return err
			}
			return context.Canceled
		},
	}

	app, err := NewApp(Dependencies{
		Config:        &staticConfigProvider{cfg: config.DefaultConfig()},
		Watchers:      watchers,
		WatchTerminal: watchTerminalFunc(func() (io.Writer, func(), bool) { return &termBuf, func() {}, true }),
		Stdout:        &stdoutBuf,
		Stderr:        &stderrBuf,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	err = runWatchMode(
		newWatchTestCmd(t),
		app,
		&rootFlagValues{},
		&cmdFlagValues{watch: true},
		[]string{"check-token"},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runWatchMode() error = %v, want context.Canceled", err)
	}
	childOut, readErr := os.ReadFile(filepath.Join(tmpDir, "child.out"))
	if readErr != nil {
		t.Fatalf("ReadFile(child.out) error = %v (stdout=%q, stderr=%q)", readErr, stdoutBuf.String(), stderrBuf.String())
	}
	if got := strings.Count(string(childOut), "TOK=empty"); got != 2 {
		t.Fatalf("watched native child TOK=empty count = %d, want 2 (childOut=%q, stderr=%q)", got, string(childOut), stderrBuf.String())
	}
	if !strings.Contains(termBuf.String(), ":argv=") || !strings.Contains(termBuf.String(), ":clear=1:") {
		t.Fatalf("watch terminal output = %q, want argv and clear=1 sequences", termBuf.String())
	}
}

func verifyOSC7577MAC(t *testing.T, seq, token string) {
	t.Helper()

	if !strings.HasPrefix(seq, "\x1b]7577;") || !strings.HasSuffix(seq, "\x07") {
		t.Fatalf("malformed OSC 7577 frame: %q", seq)
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b]7577;"), "\x07")
	body, macPart, ok := strings.Cut(payload, ":mac=")
	if !ok {
		t.Fatalf("missing :mac= in %q", payload)
	}
	sum := sha256.Sum256([]byte(body + ":" + token))
	wantMAC := hex.EncodeToString(sum[:])
	if macPart != wantMAC {
		t.Fatalf("mac = %q, want %q (body=%q)", macPart, wantMAC, body)
	}
	if _, argv, hasArgv := strings.Cut(body, ":argv="); hasArgv {
		argvField, _, _ := strings.Cut(argv, ":")
		for tok := range strings.SplitSeq(argvField, ",") {
			if _, err := base64.StdEncoding.DecodeString(tok); err != nil {
				t.Fatalf("invalid base64 argv token %q: %v", tok, err)
			}
		}
	}
	pidPrefix := "pid="
	if pidVal, _, hasPID := strings.Cut(strings.TrimPrefix(body, pidPrefix), ":"); hasPID {
		if n, err := strconv.Atoi(pidVal); err != nil || n <= 0 {
			t.Fatalf("invalid pid %q", pidVal)
		}
	}
}
