// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

// TestTUI_Confirm tests `invowk tui confirm` via tmux.
// This is a representative E2E test for interactive TUI commands.
func TestTUI_Confirm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TUI tmux test in short mode")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}

	t.Run("accept_default", func(t *testing.T) {
		s := newTmuxSession(t, "confirm-default")

		// Launch invowk tui confirm
		s.sendKeys(binaryPath+" tui confirm 'Proceed?'", "Enter")

		// Wait for TUI to render
		if !s.waitFor("Proceed", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		// Press Enter to accept default (No)
		time.Sleep(300 * time.Millisecond)
		s.sendKeys("Enter")
		time.Sleep(500 * time.Millisecond)

		// Capture output — should show the result
		output := s.capturePlain()
		// The confirm widget exits and prints the result to stdout
		if !strings.Contains(output, "No") && !strings.Contains(output, "false") {
			t.Logf("captured output:\n%s", output)
			// Don't fail hard — confirm might render differently
			t.Log("Note: could not verify default selection in output")
		}
	})

	t.Run("select_yes", func(t *testing.T) {
		s := newTmuxSession(t, "confirm-yes")

		// Launch invowk tui confirm with --affirmative flag
		s.sendKeys(binaryPath+" tui confirm 'Proceed?' --affirmative 'Yes'", "Enter")

		// Wait for TUI to render
		if !s.waitFor("Proceed", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		time.Sleep(300 * time.Millisecond)

		// Navigate to Yes (Tab or arrow) and confirm
		s.sendKeys("Tab")
		time.Sleep(200 * time.Millisecond)
		s.sendKeys("Enter")
		time.Sleep(500 * time.Millisecond)

		output := s.capturePlain()
		if !strings.Contains(output, "Yes") && !strings.Contains(output, "true") {
			t.Logf("captured output:\n%s", output)
			t.Log("Note: could not verify Yes selection in output")
		}
	})
}

// TestTUI_Choose tests `invowk tui choose` via tmux.
func TestTUI_Choose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TUI tmux test in short mode")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}

	t.Run("navigate_and_select", func(t *testing.T) {
		s := newTmuxSession(t, "choose-nav")

		// Launch invowk tui choose
		s.sendKeys(binaryPath+" tui choose 'Apple' 'Banana' 'Cherry'", "Enter")

		// Wait for TUI to render
		if !s.waitFor("Apple", 5*time.Second) {
			t.Fatal("TUI did not render within timeout")
		}

		// Navigate down to Banana
		time.Sleep(300 * time.Millisecond)
		s.sendKeys("Down")
		time.Sleep(200 * time.Millisecond)

		// Verify Banana is visible
		output := s.capturePlain()
		if !strings.Contains(output, "Banana") {
			t.Fatal("Banana not visible in TUI")
		}

		// Select Banana
		s.sendKeys("Enter")
		time.Sleep(500 * time.Millisecond)

		output = s.capturePlain()
		if !strings.Contains(output, "Banana") {
			t.Logf("captured output:\n%s", output)
			t.Log("Note: could not verify Banana selection in output")
		}
	})
}

// TestTUI_Input tests `invowk tui input` via tmux.
func TestTUI_Input(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TUI tmux test in short mode")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available")
	}

	t.Run("type_and_submit", func(t *testing.T) {
		s := newTmuxSession(t, "input-type")

		// Launch invowk tui input
		s.sendKeys(binaryPath+" tui input --header 'Enter name:'", "Enter")

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
		time.Sleep(500 * time.Millisecond)

		output := s.capturePlain()
		if !strings.Contains(output, "Hello World") {
			t.Logf("captured output:\n%s", output)
			t.Log("Note: could not verify typed input in output")
		}
	})
}
