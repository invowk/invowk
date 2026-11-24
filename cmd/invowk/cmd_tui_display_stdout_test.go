// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/tuiwire"

	"github.com/spf13/cobra"
)

// Display-only components whose stdout is not a terminal pass their content
// through (tui-clean-terminal-path, N15). A test buffer is never a terminal,
// so these runs never reach the terminal resolver.

const (
	displayPagerContent = "line one\n\tline two\nlast line without newline"
	displayTableInput   = "name|age|city\nAlice|30|NYC\nBob|25|LA\n"
	displayTableText    = "name   age  city\nAlice  30   NYC\nBob    25   LA\n"
)

func TestStdoutIsTerminal(t *testing.T) {
	t.Parallel()

	if stdoutIsTerminal(&strings.Builder{}) {
		t.Error("stdoutIsTerminal(buffer) = true, want false")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if stdoutIsTerminal(f) {
		t.Error("stdoutIsTerminal(regular file) = true, want false")
	}
}

func TestRunTuiPagerStdoutNotTerminal(t *testing.T) {
	t.Parallel()

	pagerFile := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(pagerFile, []byte(displayPagerContent), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	for _, tc := range []struct {
		name  string
		args  []string
		flags map[string]string
		stdin string
	}{
		{name: "file", args: []string{pagerFile}},
		{name: "file with title and line numbers", args: []string{pagerFile}, flags: map[string]string{"title": "Notes", "line-numbers": "true", "soft-wrap": "true"}},
		{name: "stdin", stdin: displayPagerContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := newTUIPagerCommand()
			setDisplayFlags(t, cmd, tc.flags)
			if tc.stdin != "" {
				cmd.SetIn(strings.NewReader(tc.stdin))
			}
			var stdout strings.Builder
			cmd.SetOut(&stdout)

			if err := runTuiPager(cmd, tc.args); err != nil {
				t.Fatalf("runTuiPager() error = %v", err)
			}
			if got := stdout.String(); got != displayPagerContent {
				t.Fatalf("stdout = %q, want the content unchanged %q", got, displayPagerContent)
			}
		})
	}
}

func TestRunTuiTableNotSelectableStdoutNotTerminal(t *testing.T) {
	t.Parallel()

	cmd := newTUITableCommand()
	setDisplayFlags(t, cmd, map[string]string{"separator": "|", "height": "1", "widths": "1,1,1"})
	cmd.SetIn(strings.NewReader(displayTableInput))
	var stdout strings.Builder
	cmd.SetOut(&stdout)

	if err := runTuiTable(cmd, nil); err != nil {
		t.Fatalf("runTuiTable() error = %v", err)
	}
	if got := stdout.String(); got != displayTableText {
		t.Fatalf("stdout = %q, want %q", got, displayTableText)
	}
}

// The stdout rule wins over a TUI server: no request reaches it.
func TestDisplayComponentsStdoutNotTerminalSkipServer(t *testing.T) {
	server, requests, asyncErrs := startTableTestServer(t, tuiwire.TableResult{SelectedIndex: -1})
	t.Setenv(tuiwire.EnvTUIAddr, string(server.URL()))
	t.Setenv(tuiwire.EnvTUIToken, server.Token().String())

	pager := newTUIPagerCommand()
	pager.SetIn(strings.NewReader(displayPagerContent))
	var pagerOut strings.Builder
	pager.SetOut(&pagerOut)
	if err := runTuiPager(pager, nil); err != nil {
		t.Fatalf("runTuiPager() error = %v", err)
	}
	if got := pagerOut.String(); got != displayPagerContent {
		t.Fatalf("pager stdout = %q, want %q", got, displayPagerContent)
	}

	table := newTUITableCommand()
	setDisplayFlags(t, table, map[string]string{"separator": "|"})
	table.SetIn(strings.NewReader(displayTableInput))
	var tableOut strings.Builder
	table.SetOut(&tableOut)
	if err := runTuiTable(table, nil); err != nil {
		t.Fatalf("runTuiTable() error = %v", err)
	}
	if got := tableOut.String(); got != displayTableText {
		t.Fatalf("table stdout = %q, want %q", got, displayTableText)
	}

	select {
	case req := <-requests:
		t.Fatalf("a request reached the TUI server: %+v", req)
	default:
	}
	assertNoAsyncError(t, asyncErrs)
}

func setDisplayFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("Set(%s): %v", name, err)
		}
	}
}
