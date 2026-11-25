// SPDX-License-Identifier: MPL-2.0

//go:build unix

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	// viewerKeySettle is the pause before each key sent to a command inside
	// the -i viewer: the key crosses the viewer and the command's PTY.
	viewerKeySettle = 300 * time.Millisecond
	// viewerExitMarker is printed by the run script after invowk returns.
	viewerExitMarker = "INVOWK_VIEWER_EXIT:"
	// viewerPickRuns is how many times the no-server pick recipe must pass.
	viewerPickRuns = 5
	// viewerPidGoneTimeout bounds the wait for a hung-up process to end.
	viewerPidGoneTimeout = 5 * time.Second
)

var viewerPIDPattern = regexp.MustCompile(`SLEEP_PID=(\d+)`)

// viewerPickScript is the diagnosis recipe (program-notes viewer-diagnosis):
// two value components run from a script inside the viewer.
func viewerPickScript(unsetServer bool) string {
	var b strings.Builder
	if unsetServer {
		b.WriteString("unset INVOWK_TUI_ADDR INVOWK_TUI_TOKEN\n")
	}
	inv := shellQuote(binaryPath)
	fmt.Fprintf(&b, "c=$(%s tui choose --title \"Pick one\" alpha beta gamma); echo \"CHOSEN=[$c]\"\n", inv)
	fmt.Fprintf(&b, "n=$(%s tui input --title \"Your name\"); echo \"NAME=[$n]\"\n", inv)
	return b.String()
}

// startViewer writes an invowkfile whose native command `probe` runs script,
// starts `invowk -i cmd probe` in a fresh 100x30 tmux pane through a run
// script (so the pane never shows the script text), and returns the session.
func startViewer(t *testing.T, suffix, script string) *tmuxSession {
	t.Helper()

	dir := t.TempDir()
	invowkfile := fmt.Sprintf(`cmds: [
	{
		name:        "probe"
		description: "Interactive viewer probe"
		implementations: [
			{
				script: {content: %s}
				runtimes:  [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},
]
`, strconv.Quote(script))
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(invowkfile), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}
	run := strings.Join([]string{
		"cd " + shellQuote(dir) + " || exit 99",
		shellQuote(binaryPath) + " -i cmd probe",
		`echo "` + viewerExitMarker + `$?"`,
	}, "\n") + "\n"
	runPath := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(runPath, []byte(run), 0o644); err != nil {
		t.Fatalf("write run script: %v", err)
	}

	s := newTmuxSession(t, "viewer-"+suffix)
	s.sendKeys("sh "+shellQuote(runPath), "Enter")
	return s
}

// mustWait fails the test with the pane when pattern does not appear.
func (s *tmuxSession) mustWait(pattern string) string {
	s.t.Helper()
	if !s.waitFor(pattern, tmuxWaitTimeout) {
		s.t.Fatalf("pane does not show %q within %s; pane:\n%s", pattern, tmuxWaitTimeout, s.capturePlain())
	}
	return s.capturePlain()
}

// key sends one key to the command inside the viewer after the settle delay.
func (s *tmuxSession) key(keys ...string) {
	s.t.Helper()
	time.Sleep(viewerKeySettle)
	s.sendKeys(keys...)
}

// resize resizes the session's window, as a user resizing the terminal.
func (s *tmuxSession) resize(width, height int) {
	s.t.Helper()
	cmd := exec.CommandContext(s.t.Context(), "tmux", "resize-window", "-t", s.name, "-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	if out, err := cmd.CombinedOutput(); err != nil {
		s.t.Fatalf("tmux resize-window failed: %v: %s", err, out)
	}
}

// exitViewer leaves the completed viewer and waits for invowk to return.
func (s *tmuxSession) exitViewer() string {
	s.t.Helper()
	s.key("Enter")
	return s.mustWait(viewerExitMarker)
}

// runPick drives the pick recipe: Down, Enter, "Ana", Enter.
func runPick(t *testing.T, s *tmuxSession) string {
	t.Helper()
	s.mustWait("Pick one")
	s.key("Down")
	s.key("Enter")
	s.mustWait("Your name")
	s.key("Ana")
	s.key("Enter")
	return s.mustWait("COMPLETED SUCCESSFULLY")
}

// TestInteractiveViewerPick: value components that a script runs without the
// TUI server read their keys from the viewer's terminal (spec "Components
// without a TUI server answer inside the viewer"), on every run, and the
// server-delegated control still passes.
func TestInteractiveViewerPick(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	for i := range viewerPickRuns {
		t.Run(fmt.Sprintf("no_server_%d", i+1), func(t *testing.T) {
			t.Parallel()
			s := startViewer(t, fmt.Sprintf("pick-%d", i+1), viewerPickScript(true))
			pane := runPick(t, s)
			assertPickResult(t, pane)
			s.exitViewer()
		})
	}

	t.Run("server_control", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "pick-server", viewerPickScript(false))
		pane := runPick(t, s)
		assertPickResult(t, pane)
		s.exitViewer()
	})
}

func assertPickResult(t *testing.T, pane string) {
	t.Helper()
	for _, want := range []string{"CHOSEN=[beta]", "NAME=[Ana]"} {
		if !strings.Contains(pane, want) {
			t.Errorf("pane lacks %q; pane:\n%s", want, pane)
		}
	}
	if strings.Contains(pane, "^[[B") {
		t.Errorf("pane echoes a stolen key as ^[[B; pane:\n%s", pane)
	}
}

// TestInteractiveViewerTerminal: the command's terminal identity and size
// (spec "Terminal identity inside the viewer", "Size at start", "Resize
// while running").
func TestInteractiveViewerTerminal(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("identity", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "identity", `printf 'TTY=%s\n' "$(tty)"; printf 'CTTY=%s\n' "$(ps -o tty= -p $$)"; echo IDENT_DONE`)
		pane := s.mustWait("IDENT_DONE")
		ttyName := regexp.MustCompile(`TTY=(\S+)`).FindStringSubmatch(pane)
		cttyName := regexp.MustCompile(`CTTY=\s*(\S+)`).FindStringSubmatch(pane)
		if len(ttyName) < 2 || len(cttyName) < 2 {
			t.Fatalf("pane lacks TTY/CTTY lines:\n%s", pane)
		}
		if strings.TrimPrefix(ttyName[1], "/dev/") != strings.TrimPrefix(cttyName[1], "/dev/") {
			t.Fatalf("tty = %q, ps tty = %q, want the same device; pane:\n%s", ttyName[1], cttyName[1], pane)
		}
		s.mustWait("COMPLETED SUCCESSFULLY")
		s.exitViewer()
	})

	t.Run("size", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "size", `echo "SIZE=[$(stty size)]"`)
		pane := s.mustWait("COMPLETED SUCCESSFULLY")
		if !strings.Contains(pane, "SIZE=[27 100]") {
			t.Fatalf("pane lacks SIZE=[27 100]; pane:\n%s", pane)
		}
		s.exitViewer()
	})

	t.Run("resize", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "resize", `trap 'echo "WINCH=[$(stty size)]"' WINCH; echo WINCH_READY; while :; do sleep 1; done`)
		s.mustWait("WINCH_READY")
		s.resize(90, 25)
		s.mustWait("WINCH=[22 90]")
		s.key("C-\\")
		s.mustWait(viewerExitMarker)
	})
}

// TestInteractiveViewerSignals: Ctrl+C interrupts the command and Ctrl+\
// hangs it up (spec "Interrupt a sleeping command", "Force quit").
func TestInteractiveViewerSignals(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("ctrl_c", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "ctrlc", `echo SLEEP_READY; sleep 30`)
		s.mustWait("SLEEP_READY")
		start := time.Now()
		s.key("C-c")
		pane := s.mustWait("Duration:")
		if elapsed := time.Since(start); elapsed > 20*time.Second {
			t.Fatalf("command ended %s after Ctrl+C, want well before 30s", elapsed)
		}
		if strings.Contains(pane, "COMPLETED SUCCESSFULLY") || !strings.Contains(pane, "Failed") {
			t.Fatalf("pane does not show a failed completion; pane:\n%s", pane)
		}
		s.exitViewer()
	})

	t.Run("force_quit_hangs_up", func(t *testing.T) {
		t.Parallel()
		s := startViewer(t, "forcequit", `sleep 3011 & echo SLEEP_PID=$!; wait`)
		pane := s.mustWait("SLEEP_PID=")
		m := viewerPIDPattern.FindStringSubmatch(pane)
		if len(m) < 2 {
			t.Fatalf("pane lacks SLEEP_PID; pane:\n%s", pane)
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil || pid <= 1 {
			t.Fatalf("SLEEP_PID = %q, want a process id", m[1])
		}
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
		s.key("C-\\")
		s.mustWait(viewerExitMarker)
		deadline := time.Now().Add(viewerPidGoneTimeout)
		for viewerProcessAlive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("process %d still runs %s after the force quit", pid, viewerPidGoneTimeout)
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// viewerProcessAlive reports whether pid names a process (kill -0).
func viewerProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
