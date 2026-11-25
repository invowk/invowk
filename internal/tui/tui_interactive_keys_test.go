// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type (
	keyEncodingCase struct {
		name string
		key  tea.KeyPressMsg
		// appCursor enables application cursor keys (DECCKM) first.
		appCursor bool
		want      string
	}
)

func keyEncodingCases() []keyEncodingCase {
	cases := make([]keyEncodingCase, 0, 64)
	for c := 'a'; c <= 'z'; c++ {
		cases = append(cases, keyEncodingCase{
			name: "ctrl+" + string(c),
			key:  tea.KeyPressMsg{Code: c, Mod: tea.ModCtrl},
			want: string(c - 'a' + 1),
		})
	}
	return append(cases,
		keyEncodingCase{name: "ctrl+shift+a", key: tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModShift}, want: "\x01"},
		keyEncodingCase{name: "ctrl+A upper code", key: tea.KeyPressMsg{Code: 'A', Mod: tea.ModCtrl}, want: "\x01"},
		keyEncodingCase{name: "ctrl+a with text", key: tea.KeyPressMsg{Code: 'a', Text: "a", Mod: tea.ModCtrl}, want: "\x01"},
		keyEncodingCase{name: "ctrl+space", key: tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}, want: "\x00"},
		keyEncodingCase{name: "ctrl+]", key: tea.KeyPressMsg{Code: ']', Mod: tea.ModCtrl}, want: "\x1d"},
		keyEncodingCase{name: "alt+x", key: tea.KeyPressMsg{Code: 'x', Text: "x", Mod: tea.ModAlt}, want: "\x1bx"},
		keyEncodingCase{name: "enter", key: tea.KeyPressMsg{Code: tea.KeyEnter}, want: "\r"},
		keyEncodingCase{name: "tab", key: tea.KeyPressMsg{Code: tea.KeyTab}, want: "\t"},
		keyEncodingCase{name: "shift+tab", key: tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, want: "\x1b[Z"},
		keyEncodingCase{name: "backspace", key: tea.KeyPressMsg{Code: tea.KeyBackspace}, want: "\x7f"},
		keyEncodingCase{name: "esc", key: tea.KeyPressMsg{Code: tea.KeyEscape}, want: "\x1b"},
		keyEncodingCase{name: "space", key: tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, want: " "},
		keyEncodingCase{name: "up", key: tea.KeyPressMsg{Code: tea.KeyUp}, want: "\x1b[A"},
		keyEncodingCase{name: "down", key: tea.KeyPressMsg{Code: tea.KeyDown}, want: "\x1b[B"},
		keyEncodingCase{name: "right", key: tea.KeyPressMsg{Code: tea.KeyRight}, want: "\x1b[C"},
		keyEncodingCase{name: "left", key: tea.KeyPressMsg{Code: tea.KeyLeft}, want: "\x1b[D"},
		keyEncodingCase{name: "app up", key: tea.KeyPressMsg{Code: tea.KeyUp}, appCursor: true, want: "\x1bOA"},
		keyEncodingCase{name: "app down", key: tea.KeyPressMsg{Code: tea.KeyDown}, appCursor: true, want: "\x1bOB"},
		keyEncodingCase{name: "app right", key: tea.KeyPressMsg{Code: tea.KeyRight}, appCursor: true, want: "\x1bOC"},
		keyEncodingCase{name: "app left", key: tea.KeyPressMsg{Code: tea.KeyLeft}, appCursor: true, want: "\x1bOD"},
		keyEncodingCase{name: "home", key: tea.KeyPressMsg{Code: tea.KeyHome}, want: "\x1b[H"},
		keyEncodingCase{name: "end", key: tea.KeyPressMsg{Code: tea.KeyEnd}, want: "\x1b[F"},
		keyEncodingCase{name: "pgup", key: tea.KeyPressMsg{Code: tea.KeyPgUp}, want: "\x1b[5~"},
		keyEncodingCase{name: "pgdown", key: tea.KeyPressMsg{Code: tea.KeyPgDown}, want: "\x1b[6~"},
		keyEncodingCase{name: "delete", key: tea.KeyPressMsg{Code: tea.KeyDelete}, want: "\x1b[3~"},
		keyEncodingCase{name: "f1", key: tea.KeyPressMsg{Code: tea.KeyF1}, want: "\x1bOP"},
		keyEncodingCase{name: "shifted text", key: tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}, want: "A"},
		keyEncodingCase{name: "shifted symbol", key: tea.KeyPressMsg{Code: '1', Text: "!", Mod: tea.ModShift}, want: "!"},
		keyEncodingCase{name: "non-ascii text", key: tea.KeyPressMsg{Code: 'é', Text: "é"}, want: "é"},
		keyEncodingCase{name: "repeated key", key: tea.KeyPressMsg{Code: 'x', Text: "x", IsRepeat: true}, want: "x"},
		keyEncodingCase{name: "base and shifted codes", key: tea.KeyPressMsg{Code: tea.KeyEnter, BaseCode: tea.KeyEnter, ShiftedCode: tea.KeyEnter}, want: "\r"},
		keyEncodingCase{name: "capslock ctrl+a", key: tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl | tea.ModCapsLock}, want: "\x01"},
		keyEncodingCase{name: "numlock enter", key: tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModNumLock}, want: "\r"},
		keyEncodingCase{name: "capslock up", key: tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCapsLock}, want: "\x1b[A"},
		keyEncodingCase{name: "scrolllock backspace", key: tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModScrollLock}, want: "\x7f"},
		keyEncodingCase{name: "ctrl+left", key: tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl}, want: "\x1b[D"},
		keyEncodingCase{name: "shift+up", key: tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}, want: "\x1b[A"},
		keyEncodingCase{name: "shift+enter", key: tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}, want: "\r"},
		keyEncodingCase{name: "ctrl+backspace", key: tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModCtrl}, want: "\x7f"},
		keyEncodingCase{name: "alt+ctrl+left", key: tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt | tea.ModCtrl}, want: "\x1b\x1b[D"},
		keyEncodingCase{name: "super+left", key: tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModSuper}, want: "\x1b[D"},
		keyEncodingCase{name: "ctrl+1 dropped", key: tea.KeyPressMsg{Code: '1', Mod: tea.ModCtrl}, want: ""},
		keyEncodingCase{name: "unencoded special dropped", key: tea.KeyPressMsg{Code: tea.KeyMediaPlay}, want: ""},
		keyEncodingCase{name: "f13 dropped", key: tea.KeyPressMsg{Code: tea.KeyF13}, want: ""},
	)
}

// TestInteractiveKeyEncoding sends each key through the model and reads the
// bytes the command's terminal receives (design D6).
func TestInteractiveKeyEncoding(t *testing.T) {
	t.Parallel()

	for _, tt := range keyEncodingCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := newRecordingTerminal()
			model := newSizedInteractiveModel(t, term)
			if tt.appCursor {
				model.Update(outputMsg{data: []byte("\x1b[?1h")})
			}
			_, cmd := model.Update(tt.key)
			if cmd != nil {
				t.Fatalf("key returned a command, want it forwarded")
			}
			if tt.want == "" {
				// A dropped key sends nothing; a following marker key arrives alone.
				model.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
				term.waitFor(t, "z")
				if got := term.written(); got != "z" {
					t.Fatalf("terminal received %q, want only the marker", got)
				}
				return
			}
			term.waitFor(t, tt.want)
			if got := term.written(); got != tt.want {
				t.Fatalf("terminal received %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInteractiveForceQuitKey(t *testing.T) {
	t.Parallel()

	for _, key := range []tea.KeyPressMsg{
		{Code: '\\', Mod: tea.ModCtrl},
		{Code: '\\', Mod: tea.ModCtrl | tea.ModCapsLock},
		{Code: '\\', Mod: tea.ModCtrl | tea.ModNumLock},
	} {
		term := newRecordingTerminal()
		model := newSizedInteractiveModel(t, term)
		_, cmd := model.Update(key)
		if cmd == nil {
			t.Fatalf("key %v: command = nil, want force quit", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("key %v: command message is not tea.QuitMsg", key)
		}
		if got := term.written(); got != "" {
			t.Fatalf("key %v reached the terminal as %q", key, got)
		}
	}
}

func TestInteractivePaste(t *testing.T) {
	t.Parallel()

	t.Run("plain", func(t *testing.T) {
		t.Parallel()
		term := newRecordingTerminal()
		model := newSizedInteractiveModel(t, term)
		model.Update(tea.PasteMsg{Content: "pasted"})
		term.waitFor(t, "pasted")
		if got := term.written(); got != "pasted" {
			t.Fatalf("terminal received %q, want plain paste", got)
		}
	})

	t.Run("bracketed", func(t *testing.T) {
		t.Parallel()
		term := newRecordingTerminal()
		model := newSizedInteractiveModel(t, term)
		model.Update(outputMsg{data: []byte("\x1b[?2004h")})
		model.Update(tea.PasteMsg{Content: "pasted"})
		want := "\x1b[200~pasted\x1b[201~"
		term.waitFor(t, want)
		if got := term.written(); got != want {
			t.Fatalf("terminal received %q, want %q", got, want)
		}
	})
}
