// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// tmuxSession wraps a tmux session for TUI testing.
// Each test gets a unique session name to avoid conflicts in parallel execution.
type tmuxSession struct {
	name string
	t    *testing.T
}

func newTmuxSession(t *testing.T, suffix string) *tmuxSession {
	t.Helper()
	name := fmt.Sprintf("invowk-test-%s-%d", suffix, os.Getpid())
	ctx := context.Background()

	// Ensure any stale session is cleaned up first
	_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).Run()

	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", name, "-x", "100", "-y", "30")
	if err := cmd.Run(); err != nil {
		t.Skipf("tmux not available or cannot create session: %v", err)
	}

	s := &tmuxSession{name: name, t: t}
	t.Cleanup(s.kill)
	return s
}

func (s *tmuxSession) sendKeys(keys ...string) {
	s.t.Helper()
	ctx := context.Background()
	args := append([]string{"send-keys", "-t", s.name}, keys...)
	if err := exec.CommandContext(ctx, "tmux", args...).Run(); err != nil {
		s.t.Fatalf("tmux send-keys failed: %v", err)
	}
}

func (s *tmuxSession) capturePlain() string {
	s.t.Helper()
	ctx := context.Background()
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

func (s *tmuxSession) kill() {
	ctx := context.Background()
	_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", s.name).Run()
}

func requireTmux(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping TUI tmux test in short mode")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}
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

		// Launch confirm; rejected (No) returns exit 1. Use && / || to emit
		// a deterministic marker regardless of Cobra's styled error rendering.
		s.sendKeys(binaryPath+" tui confirm 'Proceed?' && echo INVOWK_CONFIRMED || echo INVOWK_REJECTED", "Enter")

		// Wait for TUI to fully render (help text only appears after TUI init)
		if !s.waitFor("enter submit", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		// Use "n" shortcut key to select No and submit (exit 1)
		time.Sleep(200 * time.Millisecond)
		s.sendKeys("n")

		// Wait for the rejection marker
		if !s.waitFor("INVOWK_REJECTED", 5*time.Second) {
			t.Fatal("command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "INVOWK_REJECTED") {
			t.Errorf("expected INVOWK_REJECTED marker, got:\n%s", output)
		}
	})

	t.Run("accept_with_shortcut", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "confirm-accept")

		// Launch confirm; accepted (Yes) returns exit 0.
		s.sendKeys(binaryPath+" tui confirm 'Proceed?' && echo INVOWK_CONFIRMED || echo INVOWK_REJECTED", "Enter")

		// Wait for TUI to fully render
		if !s.waitFor("enter submit", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		// Use "y" shortcut key to select Yes and submit (exit 0)
		time.Sleep(200 * time.Millisecond)
		s.sendKeys("y")

		// Wait for the confirmation marker
		if !s.waitFor("INVOWK_CONFIRMED", 5*time.Second) {
			t.Fatal("command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "INVOWK_CONFIRMED") {
			t.Errorf("expected INVOWK_CONFIRMED marker, got:\n%s", output)
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

		s := newTmuxSession(t, "choose-nav")

		// Launch choose with done marker
		s.sendKeys(binaryPath+" tui choose 'Apple' 'Banana' 'Cherry'; echo INVOWK_EXIT:$?", "Enter")

		// Wait for TUI to render
		if !s.waitFor("Apple", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		// Navigate down to Banana
		time.Sleep(300 * time.Millisecond)
		s.sendKeys("Down")
		time.Sleep(200 * time.Millisecond)

		// Verify Banana is visible in the TUI
		output := s.capturePlain()
		if !strings.Contains(output, "Banana") {
			t.Fatal("Banana not visible in TUI")
		}

		// Select Banana
		s.sendKeys("Enter")

		// Wait for command to finish — "Banana" should be printed to stdout
		if !s.waitFor("INVOWK_EXIT:", 5*time.Second) {
			t.Fatal("command did not exit within timeout")
		}

		output = s.capturePlain()
		if !strings.Contains(output, "Banana") {
			t.Errorf("expected output to contain 'Banana' (selected option), got:\n%s", output)
		}
	})
}

// TestTUI_Input tests `invowk tui input` via tmux.
// The input command prints the entered text to stdout.
func TestTUI_Input(t *testing.T) {
	t.Parallel()

	requireTmux(t)

	t.Run("type_and_submit", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "input-type")

		// Launch input with done marker
		s.sendKeys(binaryPath+" tui input --header 'Enter name:'; echo INVOWK_EXIT:$?", "Enter")

		// Wait for TUI to render
		if !s.waitFor("name", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		time.Sleep(300 * time.Millisecond)

		// Type some text
		s.sendKeys("Hello World")
		time.Sleep(200 * time.Millisecond)

		// Submit
		s.sendKeys("Enter")

		// Wait for command to finish — "Hello World" should be printed to stdout
		if !s.waitFor("INVOWK_EXIT:", 5*time.Second) {
			t.Fatal("command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "Hello World") {
			t.Errorf("expected output to contain 'Hello World' (entered text), got:\n%s", output)
		}
	})
}

// TestTUI_Write exercises `invowk tui write` end-to-end via tmux.
func TestTUI_Write(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("multiline_submit", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "write-submit")
		s.sendKeys(binaryPath+" tui write --title 'Enter notes'; echo INVOWK_EXIT:$?", "Enter")

		if !s.waitFor("ctrl+d submit", 5*time.Second) {
			t.Fatal("write TUI did not render within timeout")
		}

		s.sendKeys("Line 1")
		s.sendKeys("Enter")
		s.sendKeys("Line 2")
		s.sendKeys("C-d")

		if !s.waitFor("INVOWK_EXIT:0", 5*time.Second) {
			t.Fatal("write command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "Line 1") || !strings.Contains(output, "Line 2") {
			t.Errorf("expected submitted text in output, got:\n%s", output)
		}
	})
}

// TestTUI_Filter exercises `invowk tui filter` end-to-end via tmux.
func TestTUI_Filter(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("dismiss_with_escape", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "filter-select")
		s.sendKeys("export INVOWK_FILTER_ONE=alpha_opt INVOWK_FILTER_TWO=beta_opt INVOWK_FILTER_THREE=gamma_opt", "Enter")
		s.sendKeys(binaryPath+" tui filter \"$INVOWK_FILTER_ONE\" \"$INVOWK_FILTER_TWO\" \"$INVOWK_FILTER_THREE\"; echo INVOWK_EXIT:$?", "Enter")

		if !s.waitFor("alpha_opt", 5*time.Second) {
			t.Fatal("filter TUI did not render within timeout")
		}

		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_EXIT:0", 5*time.Second) {
			t.Fatal("filter command did not exit within timeout")
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
		command := fmt.Sprintf("%s tui file --allowed .txt %s && echo INVOWK_FILE_SELECTED || echo INVOWK_FILE_ABORTED", binaryPath, shellQuote(tmpDir))
		s.sendKeys(command, "Enter")

		if !s.waitFor("enter select", 5*time.Second) {
			t.Fatal("file picker did not render within timeout")
		}

		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_FILE_ABORTED", 5*time.Second) {
			t.Fatal("file picker command did not abort within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "INVOWK_FILE_ABORTED") {
			t.Errorf("expected cancellation marker in output, got:\n%s", output)
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
		csvData := "name,age\nAlice,30\nBob,25\n"
		if err := os.WriteFile(csvPath, []byte(csvData), 0o644); err != nil {
			t.Fatalf("failed to create csv file: %v", err)
		}

		s := newTmuxSession(t, "table-select")
		command := fmt.Sprintf("%s tui table --file %s --selectable; echo INVOWK_EXIT:$?", binaryPath, shellQuote(csvPath))
		s.sendKeys(command, "Enter")

		time.Sleep(500 * time.Millisecond)
		s.sendKeys("Escape")

		if !s.waitFor("INVOWK_EXIT:0", 5*time.Second) {
			t.Fatal("table command did not exit within timeout")
		}
	})
}

// TestTUI_Spin exercises `invowk tui spin` end-to-end via tmux.
func TestTUI_Spin(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("run_command", func(t *testing.T) {
		t.Parallel()

		s := newTmuxSession(t, "spin-run")
		spinMarker := fmt.Sprintf("INVOWK_SPIN_DONE_%d", time.Now().UnixNano())
		s.sendKeys("export INVOWK_SPIN_MARKER="+spinMarker, "Enter")
		s.sendKeys(binaryPath+" tui spin --title 'Running...' -- sh -c \"echo $INVOWK_SPIN_MARKER\"; echo INVOWK_EXIT:$?", "Enter")

		if !s.waitFor("INVOWK_EXIT:0", 5*time.Second) {
			t.Fatal("spin command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, spinMarker) {
			t.Errorf("expected spin command output in pane, got:\n%s", output)
		}
	})
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

		if !s.waitFor("Line 01", 5*time.Second) {
			t.Fatal("pager did not render within timeout")
		}

		s.sendKeys("q")

		if !s.waitFor("INVOWK_EXIT:0", 5*time.Second) {
			t.Fatal("pager command did not exit within timeout")
		}

		output := s.capturePlain()
		if !strings.Contains(output, "INVOWK_EXIT:0") {
			t.Errorf("expected zero exit marker in output, got:\n%s", output)
		}
	})
}
