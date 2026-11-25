// SPDX-License-Identifier: MPL-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/vt"
)

const (
	// interactiveLockMods are the lock modifiers some terminals report with
	// every key (Kitty protocol, Windows console); they never change the
	// bytes a key sends.
	interactiveLockMods = tea.ModCapsLock | tea.ModNumLock | tea.ModScrollLock
	// interactiveDroppedMods are the modifiers a special key loses when the
	// legacy encoding has no form for them (design D6 step 5); Alt stays, as
	// vt sends it as an ESC prefix.
	interactiveDroppedMods = vt.ModShift | vt.ModCtrl | vt.ModMeta | tea.ModHyper | tea.ModSuper
)

// vtEncodedSpecialKeys are the special (non-text) key codes vt encodes. Any
// other special key would reach vt's fallback, which writes the code's
// private-use rune, so it is dropped instead.
var vtEncodedSpecialKeys = map[rune]struct{}{
	vt.KeyEnter: {}, vt.KeyTab: {}, vt.KeyBackspace: {}, vt.KeyEscape: {}, vt.KeySpace: {},
	vt.KeyUp: {}, vt.KeyDown: {}, vt.KeyRight: {}, vt.KeyLeft: {},
	vt.KeyInsert: {}, vt.KeyDelete: {}, vt.KeyHome: {}, vt.KeyEnd: {}, vt.KeyPgUp: {}, vt.KeyPgDown: {},
	vt.KeyF1: {}, vt.KeyF2: {}, vt.KeyF3: {}, vt.KeyF4: {}, vt.KeyF5: {}, vt.KeyF6: {},
	vt.KeyF7: {}, vt.KeyF8: {}, vt.KeyF9: {}, vt.KeyF10: {}, vt.KeyF11: {}, vt.KeyF12: {},
	vt.KeyKp0: {}, vt.KeyKp1: {}, vt.KeyKp2: {}, vt.KeyKp3: {}, vt.KeyKp4: {},
	vt.KeyKp5: {}, vt.KeyKp6: {}, vt.KeyKp7: {}, vt.KeyKp8: {}, vt.KeyKp9: {},
	vt.KeyKpEnter: {}, vt.KeyKpEqual: {}, vt.KeyKpMultiply: {}, vt.KeyKpPlus: {},
	vt.KeyKpComma: {}, vt.KeyKpMinus: {}, vt.KeyKpDecimal: {},
}

// isForceQuitKey reports whether the key is Ctrl+\, the viewer's force
// quit, whatever lock modifiers the terminal reports with it.
func isForceQuitKey(msg tea.KeyPressMsg) bool {
	return msg.Code == '\\' && msg.Mod&^interactiveLockMods == tea.ModCtrl
}

// encodeInteractiveKey converts a Bubble Tea key press into the vt key
// event that sends the bytes a legacy terminal in the command's modes would
// send (design D6). It reports false for a key that sends nothing.
func encodeInteractiveKey(msg tea.KeyPressMsg) (vt.KeyPressEvent, bool) {
	mod := msg.Mod &^ interactiveLockMods
	code := msg.Code
	text := msg.Text

	if _, ok := vtEncodedSpecialKeys[code]; ok && code >= vt.KeyExtended {
		// Keypad keys can carry their digit as text (Kitty protocol); vt
		// encodes them for the keypad mode only without it.
		text = ""
	}
	if text == "" && !mod.Contains(vt.ModCtrl) && mod.Contains(vt.ModShift) {
		// Alt+Shift+letter arrives without text (legacy ESC A, Kitty):
		// send the shifted character, Alt kept as the ESC prefix.
		text = withShiftedText(msg).Text
	}

	if mod.Contains(vt.ModCtrl) {
		text = ""
		if code >= 'A' && code <= 'Z' {
			code += 'a' - 'A'
		}
		if code >= 'a' && code <= 'z' {
			mod &^= vt.ModShift // Ctrl+Shift+letter sends Ctrl+letter, as xterm.
		}
	}

	if text != "" {
		return vt.KeyPressEvent{Code: code, Text: text, Mod: mod}, true
	}

	key := vt.KeyPressEvent{Code: code, Mod: mod}
	special := isSpecialKey(key)
	if special {
		if _, ok := vtEncodedSpecialKeys[code]; !ok {
			return vt.KeyPressEvent{}, false
		}
	}
	if mod&interactiveDroppedMods == 0 || hasModifiedBinding(key) {
		return key, true
	}
	if !special {
		// Ctrl (or Meta) with a character that has no control form: Ctrl+1.
		return vt.KeyPressEvent{}, false
	}
	return vt.KeyPressEvent{Code: code, Mod: mod &^ interactiveDroppedMods}, true
}

// withShiftedText returns the key with Text set to the character a
// Shift-modified printable key types, or to "" for a non-printable key.
func withShiftedText(msg tea.KeyPressMsg) tea.KeyPressMsg {
	msg.Text = ""
	if msg.Code >= vt.KeyExtended {
		return msg
	}
	shifted := msg.ShiftedCode
	if shifted == 0 && msg.Code >= 'a' && msg.Code <= 'z' {
		shifted = msg.Code - ('a' - 'A')
	}
	if shifted <= ' ' || shifted == 0x7f || shifted >= vt.KeyExtended {
		return msg
	}
	msg.Text = string(shifted)
	return msg
}

// isSpecialKey reports whether the key is a non-printing key: one of the
// extended key codes, or Enter, Tab, Backspace or Escape.
func isSpecialKey(key vt.KeyPressEvent) bool {
	switch code := key.Code; code {
	case vt.KeyEnter, vt.KeyTab, vt.KeyBackspace, vt.KeyEscape:
		return true
	default:
		return code >= vt.KeyExtended
	}
}

// hasModifiedBinding reports whether vt encodes the key with exactly its
// modifiers, Alt aside (vt sends Alt as an ESC prefix first): Ctrl+Space,
// Ctrl+letter, Ctrl+[ \ ] ^ _ and Shift+Tab.
func hasModifiedBinding(key vt.KeyPressEvent) bool {
	code, mod := key.Code, key.Mod&^vt.ModAlt
	if mod == vt.ModShift {
		return code == vt.KeyTab
	}
	if mod != vt.ModCtrl {
		return false
	}
	switch {
	case code == vt.KeySpace, code >= 'a' && code <= 'z':
		return true
	default:
		return code == '[' || code == '\\' || code == ']' || code == '^' || code == '_'
	}
}
