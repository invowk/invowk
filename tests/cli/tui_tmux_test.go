// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// tmuxWaitTimeout is the default timeout for tmux waitFor polls.
	// 30s gives heavily loaded CI runners room to render full-screen widgets.
	tmuxWaitTimeout = 30 * time.Second

	// tuiInputSettleDelay is a small pause after waitFor succeeds and before
	// sendKeys. Tmux rendering is asynchronous — even after content is visible in
	// the pane, the TUI process may not yet be ready to accept input. This delay
	// is a pragmatic minimum; increasing it reduces flakiness on slow CI runners.
	tuiInputSettleDelay = 100 * time.Millisecond

	tmuxCleanupTimeout = 5 * time.Second

	// requireTmuxEnv names the variable that turns a missing tmux into a
	// test failure instead of a skip.
	requireTmuxEnv = "INVOWK_TEST_REQUIRE_TMUX"
)

var tmuxSessionMu sync.Mutex

// tmuxSession wraps a tmux session for TUI testing.
// Each test gets a unique session name, but sessions are serialized to keep
// CI runner load and tmux server state deterministic.
type tmuxSession struct {
	name string
	t    *testing.T
}

func newTmuxSession(t *testing.T, suffix string) *tmuxSession {
	t.Helper()

	tmuxSessionMu.Lock()
	t.Cleanup(tmuxSessionMu.Unlock)

	name := fmt.Sprintf("invowk-test-%s-%d", suffix, os.Getpid())
	ctx := t.Context()

	// Ensure any stale session is cleaned up first
	_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).Run()

	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", name, "-x", "100", "-y", "30")
	if err := cmd.Run(); err != nil {
		skipOrFailWithoutTmux(t, fmt.Sprintf("tmux cannot create session: %v", err))
	}

	s := &tmuxSession{name: name, t: t}
	t.Cleanup(s.kill)
	return s
}

func (s *tmuxSession) sendKeys(keys ...string) {
	s.t.Helper()
	ctx := s.t.Context()
	args := append([]string{"send-keys", "-t", s.name}, keys...)
	if err := exec.CommandContext(ctx, "tmux", args...).Run(); err != nil {
		s.t.Fatalf("tmux send-keys failed: %v", err)
	}
}

func (s *tmuxSession) capturePlain() string {
	s.t.Helper()
	ctx := s.t.Context()
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-t", s.name, "-p").Output()
	if err != nil {
		s.t.Fatalf("tmux capture-pane failed: %v", err)
	}
	return string(out)
}

// waitFor polls the tmux pane for a pattern, with timeout.
func (s *tmuxSession) waitFor(pattern string, timeout time.Duration) bool {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.capturePlain(), pattern) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// waitForLineRegexp polls the tmux pane until at least one line matches re.
func (s *tmuxSession) waitForLineRegexp(re *regexp.Regexp, timeout time.Duration) bool {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for line := range strings.SplitSeq(s.capturePlain(), "\n") {
			if re.MatchString(line) {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func (s *tmuxSession) kill() {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxCleanupTimeout)
	defer cancel()

	_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", s.name).Run()
}

// requireTmux skips the test in short mode and when tmux is missing, unless
// INVOWK_TEST_REQUIRE_TMUX=1, which CI sets on the Linux full legs so the tmux
// tests there cannot skip silently.
func requireTmux(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		skipOrFailWithoutTmux(t, fmt.Sprintf("tmux not available: %v", err))
	}
}

// skipOrFailWithoutTmux fails the test when INVOWK_TEST_REQUIRE_TMUX=1 and
// skips it otherwise.
func skipOrFailWithoutTmux(t *testing.T, reason string) {
	t.Helper()

	if os.Getenv(requireTmuxEnv) == "1" {
		t.Fatalf("%s, and %s=1 requires tmux", reason, requireTmuxEnv)
	}
	t.Skip(reason)
}

func shellQuote(path string) string {
	return strconv.Quote(path)
}

// TestTUI_Confirm tests `invowk tui confirm` via tmux.
// The confirm command communicates its result via exit code (0=yes, 1=no),
// not stdout. We verify the exit code using an echo marker after the command.
func TestTUI_Confirm(t *testing.T) {
	t.Parallel()

	requireTmux(t)

	t.Run("reject_with_shortcut", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "confirm-reject")

		// Launch confirm; rejected (No) returns exit 1. Assemble the marker
		// dynamically so the echoed command line cannot satisfy waitFor.
		s.sendKeys(binaryPath+" tui confirm 'Proceed?'; st=$?; [ $st -eq 0 ] && rc=CONFIRMED || rc=REJECTED:$st; echo \"INVOWK_CONFIRM:$rc\"", "Enter")

		// Wait for TUI chrome (help text only appears after TUI init).
		if !s.waitFor("enter submit • y yes • n no • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("TUI did not render within timeout, pane:\n%s", s.capturePlain())
		}

		// Use "n" shortcut key to select No and submit (exit 1).
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("n")

		// Wait for the dynamically expanded rejection marker.
		if !s.waitFor("INVOWK_CONFIRM:REJECTED:1", tmuxWaitTimeout) {
			t.Fatalf("command did not exit 1 within timeout, pane:\n%s", s.capturePlain())
		}
	})

	t.Run("accept_with_shortcut", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "confirm-accept")

		// Launch confirm; accepted (Yes) returns exit 0.
		s.sendKeys(binaryPath+" tui confirm 'Proceed?'; st=$?; [ $st -eq 0 ] && rc=CONFIRMED || rc=REJECTED:$st; echo \"INVOWK_CONFIRM:$rc\"", "Enter")

		// Wait for TUI chrome.
		if !s.waitFor("enter submit • y yes • n no • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("TUI did not render within timeout, pane:\n%s", s.capturePlain())
		}

		// Use "y" shortcut key to select Yes and submit (exit 0).
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("y")

		// Wait for the dynamically expanded confirmation marker.
		if !s.waitFor("INVOWK_CONFIRM:CONFIRMED", tmuxWaitTimeout) {
			t.Fatalf("command did not confirm within timeout, pane:\n%s", s.capturePlain())
		}
	})
}

// TestTUI_Choose tests `invowk tui choose` via tmux.
// The choose command prints the selected option to stdout.
func TestTUI_Choose(t *testing.T) {
	t.Parallel()

	requireTmux(t)

	t.Run("navigate_and_select", func(t *testing.T) {
		t.Parallel()

		outPath := filepath.Join(t.TempDir(), "out")
		s := newTmuxSession(t, "choose-nav")

		command := fmt.Sprintf("%s tui choose 'Apple' 'Banana' 'Cherry' > %s; echo \"INVOWK_EXIT:$? INVOWK_CHOSEN:$(cat %s)\"", binaryPath, shellQuote(outPath), shellQuote(outPath))
		s.sendKeys(command, "Enter")

		// Wait for rendered TUI help line and focused Apple row (immune to
		// echoed CLI args and column-100 wrapping).
		if !s.waitFor("↑/k up • ↓/j down • enter submit • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("choose TUI help line did not render within timeout, pane:\n%s", s.capturePlain())
		}
		if !s.waitForLineRegexp(regexp.MustCompile(`│\s*Apple\s*$`), tmuxWaitTimeout) {
			t.Fatalf("Apple focus row did not render within timeout, pane:\n%s", s.capturePlain())
		}

		// Navigate down to Banana and wait for the TUI focus marker to move.
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Down")
		if !s.waitForLineRegexp(regexp.MustCompile(`│\s*Banana\s*$`), tmuxWaitTimeout) {
			t.Fatalf("Banana focus row not visible in TUI, pane:\n%s", s.capturePlain())
		}

		// Select Banana.
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Enter")

		// Wait for dynamically expanded exit code and stdout value.
		if !s.waitFor("INVOWK_EXIT:0 INVOWK_CHOSEN:Banana", tmuxWaitTimeout) {
			t.Fatalf("expected INVOWK_EXIT:0 INVOWK_CHOSEN:Banana within timeout, pane:\n%s", s.capturePlain())
		}
		assertFileContent(t, outPath, "Banana\n")
	})
}

// TestTUI_Input tests `invowk tui input` via tmux.
// The input command prints the entered text to stdout.
func TestTUI_Input(t *testing.T) {
	t.Parallel()

	requireTmux(t)

	t.Run("type_and_submit", func(t *testing.T) {
		t.Parallel()

		outPath := filepath.Join(t.TempDir(), "out")
		s := newTmuxSession(t, "input-type")

		command := fmt.Sprintf("%s tui input --title 'Enter name:' > %s; echo \"INVOWK_EXIT:$? INVOWK_VAL:$(cat %s)\"", binaryPath, shellQuote(outPath), shellQuote(outPath))
		s.sendKeys(command, "Enter")

		// Wait for rendered TUI help chrome (absent from command line).
		if !s.waitFor("enter submit • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("input TUI did not render within timeout, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Hello World")
		if !s.waitForLineRegexp(regexp.MustCompile(`^\s*>\s*Hello World\s*$`), tmuxWaitTimeout) {
			t.Fatalf("typed text did not render on input prompt row, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Enter")

		if !s.waitFor("INVOWK_EXIT:0 INVOWK_VAL:Hello World", tmuxWaitTimeout) {
			t.Fatalf("expected INVOWK_EXIT:0 INVOWK_VAL:Hello World within timeout, pane:\n%s", s.capturePlain())
		}
		assertFileContent(t, outPath, "Hello World\n")
	})
}

// TestTUI_Write exercises `invowk tui write` end-to-end via tmux.
func TestTUI_Write(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("multiline_submit", func(t *testing.T) {
		t.Parallel()

		outPath := filepath.Join(t.TempDir(), "out")
		s := newTmuxSession(t, "write-submit")
		command := fmt.Sprintf("%s tui write --title 'Enter notes' > %s; echo \"INVOWK_EXIT:$? INVOWK_WRITE:$(tr '\\n' '|' < %s)\"", binaryPath, shellQuote(outPath), shellQuote(outPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("ctrl+d submit • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("write TUI did not render within timeout, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Line 1")
		s.sendKeys("Enter")
		s.sendKeys("Line 2")
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("C-d")

		if !s.waitFor("INVOWK_EXIT:0 INVOWK_WRITE:Line 1|Line 2|", tmuxWaitTimeout) {
			t.Fatalf("expected INVOWK_EXIT:0 INVOWK_WRITE:Line 1|Line 2| within timeout, pane:\n%s", s.capturePlain())
		}
		assertFileContent(t, outPath, "Line 1\nLine 2\n")
	})
}

// TestTUI_Filter exercises `invowk tui filter` end-to-end via tmux.
func TestTUI_Filter(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("dismiss_with_escape", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "filter-select")
		// Assemble options via split env vars so the echoed export line cannot match waitFor.
		s.sendKeys("export INVOWK_F1A=alpha INVOWK_F1B=_opt INVOWK_F2A=beta INVOWK_F2B=_opt INVOWK_F3A=gamma INVOWK_F3B=_opt", "Enter")
		s.sendKeys(binaryPath+" tui filter \"$INVOWK_F1A$INVOWK_F1B\" \"$INVOWK_F2A$INVOWK_F2B\" \"$INVOWK_F3A$INVOWK_F3B\"; echo INVOWK_EXIT:$?", "Enter")

		if !s.waitForLineRegexp(regexp.MustCompile(`│\s*alpha_opt\s*$`), tmuxWaitTimeout) {
			t.Fatalf("filter TUI did not render focused option within timeout, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("filter command did not exit within timeout, pane:\n%s", s.capturePlain())
		}
	})
}

// TestTUI_File exercises `invowk tui file` cancellation flow via tmux.
func TestTUI_File(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("cancel_with_escape", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "picked.txt"), []byte("picked"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		s := newTmuxSession(t, "file-cancel")
		command := fmt.Sprintf("%s tui file --allowed .txt %s && rc=SELECTED || rc=ABORTED; echo \"INVOWK_FILE:$rc\"", binaryPath, shellQuote(tmpDir))
		s.sendKeys(command, "Enter")

		if !s.waitFor("enter select • esc cancel", tmuxWaitTimeout) {
			t.Fatalf("file picker did not render help chrome within timeout, pane:\n%s", s.capturePlain())
		}
		if !s.waitFor("picked.txt", tmuxWaitTimeout) {
			t.Fatalf("file picker did not list picked.txt within timeout, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_FILE:ABORTED", tmuxWaitTimeout) {
			t.Fatalf("file picker command did not abort within timeout, pane:\n%s", s.capturePlain())
		}
	})
}

// TestTUI_Table exercises `invowk tui table` selectable flow via tmux.
func TestTUI_Table(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("select_row", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "rows.csv")
		csvData := "tblhdr_name,tblhdr_age\nAlice,30\nBob,25\n"
		if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
			t.Fatalf("failed to create csv file: %v", err)
		}

		s := newTmuxSession(t, "table-select")
		command := fmt.Sprintf("%s tui table --file %s --selectable; echo INVOWK_EXIT:$?", binaryPath, shellQuote(csvPath))
		s.sendKeys(command, "Enter")

		waitForTableContent(t, s, []string{"tblhdr_name", "tblhdr_age"}, []string{"Alice", "30"}, []string{"Bob", "25"})
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("table command did not exit within timeout, pane:\n%s", s.capturePlain())
		}
	})

	// With stdout on the terminal, a non-selectable table still draws there
	// and waits for a key (tui-clean-terminal-path, N15).
	t.Run("display_on_terminal", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "rows.csv")
		if err := os.WriteFile(csvPath, []byte("tblhdr_name,tblhdr_age\nAlice,30\nBob,25\n"), 0o644); err != nil {
			t.Fatalf("failed to create csv file: %v", err)
		}

		s := newTmuxSession(t, "table-display")
		command := fmt.Sprintf("%s tui table --file %s; echo INVOWK_EXIT:$?", binaryPath, shellQuote(csvPath))
		s.sendKeys(command, "Enter")

		waitForTableContent(t, s, []string{"tblhdr_name", "tblhdr_age"}, []string{"Alice", "30"}, []string{"Bob", "25"})
		time.Sleep(tuiInputSettleDelay)
		if pane := s.capturePlain(); strings.Contains(pane, "INVOWK_EXIT:0") {
			t.Fatalf("table exited before a key press, pane:\n%s", pane)
		}
		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("table command did not exit within timeout, pane:\n%s", s.capturePlain())
		}
	})

	// A drawn table shows its data rows before any key press, each on its own
	// line (decision 98a): the base drew only the header and its rule, since
	// the table's row viewport had no width.
	t.Run("display_draws_rows", func(t *testing.T) {
		t.Parallel()

		csvPath := writeTableRowsCSV(t)
		s := newTmuxSession(t, "table-display-rows")
		command := fmt.Sprintf("%s tui table --file %s; echo INVOWK_EXIT:$?", binaryPath, shellQuote(csvPath))
		s.sendKeys(command, "Enter")

		waitForTableRows(t, s)
		s.sendKeys("Escape")
		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("table command did not exit within timeout, pane:\n%s", s.capturePlain())
		}
	})

	t.Run("selectable_draws_rows", func(t *testing.T) {
		t.Parallel()

		csvPath := writeTableRowsCSV(t)
		s := newTmuxSession(t, "table-select-rows")
		command := fmt.Sprintf("row=$(%s tui table --file %s --selectable); echo INVOWK_EXIT:$? INVOWK_ROW:$row", binaryPath, shellQuote(csvPath))
		s.sendKeys(command, "Enter")

		waitForTableRows(t, s)
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Down")
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("Enter")
		// Wait for the printed result itself: the typed command line already
		// holds "INVOWK_ROW:", so waiting for that marker alone raced the exit.
		if !s.waitFor("INVOWK_EXIT:0 INVOWK_ROW:Bob,25,LA", tmuxWaitTimeout) {
			t.Fatalf("expected the selected row Bob,25,LA within timeout, pane:\n%s", s.capturePlain())
		}
	})

	// With a terminal present and stdout piped, a non-selectable table prints
	// plain-text rows and exits without a key press (tui-clean-terminal-path,
	// N15).
	t.Run("stdout_piped", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		csvPath := filepath.Join(tmpDir, "rows.csv")
		outPath := filepath.Join(tmpDir, "out")
		if err := os.WriteFile(csvPath, []byte("name,age,city\nAlice,30,NYC\nBob,25,LA\n"), 0o644); err != nil {
			t.Fatalf("failed to create csv file: %v", err)
		}

		s := newTmuxSession(t, "table-piped")
		command := fmt.Sprintf("%s tui table --file %s | cat > %s; echo INVOWK_EXIT:$?", binaryPath, shellQuote(csvPath), shellQuote(outPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("piped table did not exit without a key press, pane:\n%s", s.capturePlain())
		}
		assertFileContent(t, outPath, "name   age  city\nAlice  30   NYC\nBob    25   LA\n")
	})
}

// TestTUI_Spin exercises `invowk tui spin` end-to-end via tmux.
func TestTUI_Spin(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("run_command", func(t *testing.T) {
		t.Parallel()

		goPath := filepath.Join(t.TempDir(), "go")
		s := newTmuxSession(t, "spin-run")
		suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
		spinMarker := "INVOWK_SPIN_DONE_" + suffix
		// Split title and marker across env vars so the echoed command line cannot
		// satisfy checks, and hold exit via goPath until the spinner is rendered
		// and startup terminal replies have drained cleanly.
		s.sendKeys("export INVOWK_RT1=Spin INVOWK_RT2=Running INVOWK_SM1=INVOWK_SPIN_DONE_ INVOWK_SM2="+suffix, "Enter")
		command := fmt.Sprintf(`%s tui spin --title "$INVOWK_RT1$INVOWK_RT2" -- sh -c 'while [ ! -f %s ]; do sleep 0.05; done; echo "$INVOWK_SM1$INVOWK_SM2"'; echo INVOWK_EXIT:$?`,
			binaryPath, shellQuote(goPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("SpinRunning", tmuxWaitTimeout) {
			t.Fatalf("spinner did not render within timeout, pane:\n%s", s.capturePlain())
		}
		time.Sleep(tuiInputSettleDelay)
		if err := os.WriteFile(goPath, nil, 0o644); err != nil {
			t.Fatalf("failed to write spin gate file: %v", err)
		}

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("spin command did not exit within timeout, pane:\n%s", s.capturePlain())
		}

		output := s.capturePlain()
		if !paneHasLine(output, spinMarker) {
			t.Errorf("expected spin command output line %q in pane, got:\n%s", spinMarker, output)
		}
	})

	// Ctrl+C stops the wrapped command, keeps its output and exits 1
	// (tui-clean-terminal-path, N14). `exec sleep` makes the sleeper the
	// direct child, so the kill reaches it. The pid file is written after the
	// output line, so once it exists the line is already in spin's pipe.
	// Markers are assembled by the shell so the typed command line cannot
	// match them.
	t.Run("ctrl_c_cancels_command", func(t *testing.T) {
		t.Parallel()

		pidPath := filepath.Join(t.TempDir(), "pid")
		s := newTmuxSession(t, "spin-ctrl-c")
		s.sendKeys("export INVOWK_T1=Spin INVOWK_T2=Wait INVOWK_S1=sta INVOWK_S2=rted", "Enter")
		command := fmt.Sprintf(`%s tui spin --title "$INVOWK_T1$INVOWK_T2" -- sh -c 'echo "$INVOWK_S1$INVOWK_S2"; echo $$ > %s; exec sleep 30'; echo INVOWK_EXIT:$?`,
			binaryPath, shellQuote(pidPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("SpinWait", tmuxWaitTimeout) {
			t.Fatalf("spinner did not render within timeout, pane:\n%s", s.capturePlain())
		}
		if !waitForPath(pidPath, tmuxWaitTimeout) {
			t.Fatal("wrapped command did not start within timeout")
		}
		time.Sleep(tuiInputSettleDelay)
		start := time.Now()
		s.sendKeys("C-c")

		// Well before the 30 s sleep: the command was stopped, not awaited.
		if !s.waitFor("INVOWK_EXIT:1", 20*time.Second) {
			t.Fatalf("spin did not exit 1 after Ctrl+C, pane:\n%s", s.capturePlain())
		}
		if elapsed := time.Since(start); elapsed > 20*time.Second {
			t.Fatalf("spin exited %v after Ctrl+C, want well before the 30 s sleep", elapsed)
		}
		if output := s.capturePlain(); !paneHasLine(output, "started") {
			t.Errorf("expected the command output line 'started' in the pane, got:\n%s", output)
		}

		s.sendKeys(fmt.Sprintf("echo INVOWK_PID_$(kill -0 $(cat %s) 2>/dev/null && echo ALIVE || echo GONE)", shellQuote(pidPath)), "Enter")
		if !s.waitFor("INVOWK_PID_GONE", 5*time.Second) {
			t.Errorf("the wrapped command is still running after Ctrl+C, pane:\n%s", s.capturePlain())
		}
	})
}

// waitForPath polls until path exists and reports whether it appeared within
// timeout.
func waitForPath(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for _, err := os.Stat(path); err != nil; _, err = os.Stat(path) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// writeTableRowsCSV writes the three-column table of the drawn-table tests and
// returns its path.
func writeTableRowsCSV(t *testing.T) string {
	t.Helper()

	csvPath := filepath.Join(t.TempDir(), "rows.csv")
	if err := os.WriteFile(csvPath, []byte("name,age,city\nAlice,30,NYC\nBob,25,LA\n"), 0o644); err != nil {
		t.Fatalf("failed to create csv file: %v", err)
	}
	return csvPath
}

// waitForTableRows waits until the pane shows the rows of writeTableRowsCSV
// below their titles, each row on its own line, and fails the test otherwise.
func waitForTableRows(t *testing.T, s *tmuxSession) {
	t.Helper()
	waitForTableContent(t, s, []string{"name", "age", "city"}, []string{"Alice", "30", "NYC"}, []string{"Bob", "25", "LA"})
}

// waitForTableContent polls until the pane displays every expected table row
// strictly in order, each on its own line.
func waitForTableContent(t *testing.T, s *tmuxSession, expectedRows ...[]string) {
	t.Helper()

	deadline := time.Now().Add(tmuxWaitTimeout)
	for {
		pane := s.capturePlain()
		prev := -1
		matched := true
		for _, fields := range expectedRows {
			idx := paneLineWithFields(pane, fields...)
			if idx <= prev {
				matched = false
				break
			}
			prev = idx
		}
		if matched {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("table rows %v not drawn in order on separate lines, pane:\n%s", expectedRows, pane)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// paneLineWithFields returns the index of the first pane line whose
// whitespace-separated fields are exactly fields, or -1.
func paneLineWithFields(pane string, fields ...string) int {
	for i, line := range strings.Split(pane, "\n") {
		if slices.Equal(strings.Fields(line), fields) {
			return i
		}
	}
	return -1
}

// paneHasLine reports whether a line of the captured pane, without trailing
// spaces, equals want.
func paneHasLine(pane, want string) bool {
	for line := range strings.SplitSeq(pane, "\n") {
		if strings.TrimRight(line, " ") == want {
			return true
		}
	}
	return false
}

// TestTUI_Pager exercises `invowk tui pager` dismissal flow via tmux.
func TestTUI_Pager(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("dismiss_with_q", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		pagerPath := filepath.Join(tmpDir, "pager.txt")
		content := strings.Join([]string{
			"Line 01", "Line 02", "Line 03", "Line 04", "Line 05",
		}, "\n") + "\n"
		if err := os.WriteFile(pagerPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create pager file: %v", err)
		}

		s := newTmuxSession(t, "pager-dismiss")
		command := fmt.Sprintf("%s tui pager %s; echo INVOWK_EXIT:$?", binaryPath, shellQuote(pagerPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("↑/↓: navigate • q/Enter: close", tmuxWaitTimeout) {
			t.Fatalf("pager help chrome did not render within timeout, pane:\n%s", s.capturePlain())
		}
		if !s.waitFor("Line 01", tmuxWaitTimeout) {
			t.Fatalf("pager content did not render within timeout, pane:\n%s", s.capturePlain())
		}

		time.Sleep(tuiInputSettleDelay)
		s.sendKeys("q")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("pager command did not exit within timeout, pane:\n%s", s.capturePlain())
		}
	})

	// With a terminal present and stdout redirected, the pager passes the
	// content through like less and exits without a key press
	// (tui-clean-terminal-path, N15).
	t.Run("stdout_redirected", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		pagerPath := filepath.Join(tmpDir, "pager.txt")
		outPath := filepath.Join(tmpDir, "out")
		content := "Line 01\nLine 02\n"
		if err := os.WriteFile(pagerPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create pager file: %v", err)
		}

		s := newTmuxSession(t, "pager-redirected")
		command := fmt.Sprintf("%s tui pager --title Notes --line-numbers %s > %s; echo INVOWK_EXIT:$?", binaryPath, shellQuote(pagerPath), shellQuote(outPath))
		s.sendKeys(command, "Enter")

		if !s.waitFor("INVOWK_EXIT:0", tmuxWaitTimeout) {
			t.Fatalf("redirected pager did not exit without a key press, pane:\n%s", s.capturePlain())
		}
		assertFileContent(t, outPath, content)
	})
}

// assertFileContent fails unless the file at path holds exactly want.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}
