// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// captureExitPattern matches the marker the capture script prints after the
// component exits.
var captureExitPattern = regexp.MustCompile(`INVOWK_CAPTURE_EXIT:(\d+)`)

type (
	// tuiCaptureCase drives one captured component run in a tmux pane
	// (fix-tui-capture-output, spec tui-component-terminal-io).
	tuiCaptureCase struct {
		// name is the subtest name and the tmux session suffix.
		name string
		// assign is a POSIX shell assignment that captures the component's
		// stdout into $v, for example `v=$("$INVOWK" tui input)`. $INVOWK is
		// the binary under test and the working directory is the case's
		// temporary directory.
		assign string
		// files are written into the temporary directory before the run.
		files map[string]string
		// ready is text that must be visible in the pane before any key is
		// sent: proof that the UI drew on the terminal while stdout was
		// captured.
		ready string
		// keys are sent one tmux send-keys call per element.
		keys [][]string
		// want is the exact captured value, given the temporary directory.
		// $(...) strips the trailing newline.
		want func(dir string) string
	}
)

// wantValue returns a want function for a value that does not depend on the
// temporary directory.
func wantValue(v string) func(string) string {
	return func(string) string { return v }
}

// runTUICapture runs c in a tmux pane through a POSIX sh script, so the result
// does not depend on the login shell, fails unless the captured bytes (as
// `od -An -tx1` reports them) equal c.want, and returns the case's temporary
// directory.
func runTUICapture(t *testing.T, c tuiCaptureCase) (dir string) {
	t.Helper()

	dir = t.TempDir()
	for name, content := range c.files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	script := strings.Join([]string{
		"cd " + shellQuote(dir) + " || exit 99",
		"INVOWK=" + shellQuote(binaryPath),
		c.assign,
		"rc=$?",
		"printf '%s' \"$v\" | od -An -tx1 > capture.hex",
		"echo \"INVOWK_CAPTURE_EXIT:$rc\"",
	}, "\n") + "\n"
	scriptPath := filepath.Join(dir, "capture.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatalf("write capture script: %v", err)
	}

	s := newTmuxSession(t, "capture-"+c.name)
	s.sendKeys("sh "+shellQuote(scriptPath), "Enter")

	if !s.waitFor(c.ready, tmuxWaitTimeout) {
		t.Fatalf("UI did not draw %q on the terminal while stdout was captured; pane:\n%s", c.ready, s.capturePlain())
	}
	for _, keys := range c.keys {
		time.Sleep(tuiInputSettleDelay)
		s.sendKeys(keys...)
	}

	if !s.waitFor("INVOWK_CAPTURE_EXIT:", tmuxWaitTimeout) {
		t.Fatalf("component did not exit within timeout; pane:\n%s", s.capturePlain())
	}
	pane := s.capturePlain()
	m := captureExitPattern.FindStringSubmatch(pane)
	if len(m) < 2 || m[1] != "0" {
		t.Fatalf("component exit marker = %v, want status 0; pane:\n%s", m, pane)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "capture.hex"))
	if err != nil {
		t.Fatalf("read capture.hex: %v", err)
	}
	gotHex := strings.Join(strings.Fields(string(raw)), "")
	want := c.want(dir)
	if wantHex := hex.EncodeToString([]byte(want)); gotHex != wantHex {
		t.Fatalf("captured value hex = %s, want %s (%q)", gotHex, wantHex, want)
	}
	return dir
}

// TestTUI_CaptureValue verifies that each value component, with stdout
// captured, draws on the terminal and that the capture holds exactly the
// value: no control sequences (spec "Captured value is byte-exact" and
// "Every value component"). The tests need tmux, so they run on the Linux CI
// legs only, where INVOWK_TEST_REQUIRE_TMUX makes a missing tmux fail.
func TestTUI_CaptureValue(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	cases := []tuiCaptureCase{
		{
			name:   "input",
			assign: `v=$("$INVOWK" tui input --title 'Capture name')`,
			ready:  "Capture name",
			keys:   [][]string{{"Hello World"}, {"Enter"}},
			want:   wantValue("Hello World"),
		},
		{
			name:   "input_password",
			assign: `v=$("$INVOWK" tui input --title 'Capture secret' --password)`,
			ready:  "Capture secret",
			keys:   [][]string{{"s3cr3t"}, {"Enter"}},
			want:   wantValue("s3cr3t"),
		},
		{
			name:   "choose",
			assign: `v=$("$INVOWK" tui choose --title 'Pick fruit' Apple Banana Cherry)`,
			ready:  "Pick fruit",
			keys:   [][]string{{"Down"}, {"Enter"}},
			want:   wantValue("Banana"),
		},
		{
			name:   "filter_piped_options",
			assign: `v=$(printf 'alpha_opt\nbeta_opt\n' | "$INVOWK" tui filter --title 'Filter opts')`,
			ready:  "alpha_opt",
			keys:   [][]string{{"Down"}, {"Enter"}},
			want:   wantValue("beta_opt"),
		},
		{
			name:   "write",
			assign: `v=$("$INVOWK" tui write --title 'Enter notes')`,
			ready:  "ctrl+d submit",
			keys:   [][]string{{"Line 1"}, {"Enter"}, {"Line 2"}, {"C-d"}},
			want:   wantValue("Line 1\nLine 2"),
		},
		{
			name:   "file",
			assign: `v=$("$INVOWK" tui file --title 'Pick file' "$PWD/pick")`,
			files:  map[string]string{"pick/picked.txt": "picked"},
			ready:  "picked.txt",
			keys:   [][]string{{"Enter"}},
			want:   func(dir string) string { return filepath.Join(dir, "pick", "picked.txt") },
		},
		{
			name:   "table_selectable",
			assign: `v=$("$INVOWK" tui table --file rows.csv --selectable)`,
			files:  map[string]string{"rows.csv": "tblhdr_name,tblhdr_age\nAlice,30\nBob,25\n"},
			// The auto height (rows+1) leaves the data rows scrolled out of
			// view, so readiness is the header line. Its distinctive text
			// cannot match the prompt or the typed command.
			ready: "tblhdr_name",
			keys:  [][]string{{"Enter"}},
			want:  wantValue("Alice,30"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runTUICapture(t, c)
		})
	}
}

// TestTUI_CaptureStreams verifies the stream cases of the terminal rule:
// spin's captured output, input with stderr redirected, and input with stdin
// from /dev/null (spec "Captured spin output", "stderr redirected" and
// "stdin is a pipe").
func TestTUI_CaptureStreams(t *testing.T) {
	t.Parallel()
	requireTmux(t)

	t.Run("spin_output", func(t *testing.T) {
		t.Parallel()
		runTUICapture(t, tuiCaptureCase{
			name:   "spin",
			assign: `v=$("$INVOWK" tui spin --title 'Spin wait' -- sh -c 'sleep 1; echo done')`,
			ready:  "Spin wait",
			want:   wantValue("done"),
		})
	})

	t.Run("input_stderr_redirected", func(t *testing.T) {
		t.Parallel()
		dir := runTUICapture(t, tuiCaptureCase{
			name:   "stderr",
			assign: `v=$("$INVOWK" tui input --title 'Stderr name' 2>err.log)`,
			ready:  "Stderr name",
			keys:   [][]string{{"hey"}, {"Enter"}},
			want:   wantValue("hey"),
		})
		errLog, err := os.ReadFile(filepath.Join(dir, "err.log"))
		if err != nil {
			t.Fatalf("read err.log: %v", err)
		}
		if bytes.ContainsRune(errLog, '\x1b') {
			t.Fatalf("err.log holds UI bytes: %q", errLog)
		}
	})

	t.Run("input_stdin_devnull", func(t *testing.T) {
		t.Parallel()
		runTUICapture(t, tuiCaptureCase{
			name:   "devnull",
			assign: `v=$("$INVOWK" tui input --title 'Null name' </dev/null)`,
			ready:  "Null name",
			keys:   [][]string{{"hey"}, {"Enter"}},
			want:   wantValue("hey"),
		})
	})
}
