// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"os"
	"regexp"

	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/charmbracelet/x/xpty"
)

// oscSequenceRe matches all OSC (Operating System Command) escape sequences.
// OSC sequences are used for terminal features like window titles, hyperlinks,
// color queries, clipboard operations, etc.
//
// In the context of invowk's interactive pager, these sequences don't function
// (hyperlinks aren't clickable, window titles don't apply, etc.) and can appear
// as visual garbage when fragmented across PTY read buffers.
//
// Format: ESC ] <content> <terminator>
// Terminators: BEL (\x07), ST (\x1b\\), or backslash alone (\)
//
// Also matches partial/fragmented sequences where leading ESC was consumed.
var oscSequenceRe = regexp.MustCompile(
	`\x1b\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)` + // Full OSC: ESC ] ... terminator
		`|` +
		`\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)`, // Partial: ] ... terminator (missing ESC)
)

// stripOSCSequences removes all OSC escape sequences from output.
// In invowk's interactive pager context, OSC features don't function anyway
// (the pager is a text viewport, not a full terminal emulator), and fragmented
// sequences appear as visual garbage.
func stripOSCSequences(s string) string {
	return oscSequenceRe.ReplaceAllString(s, "")
}

func newInteractiveModel(opts InteractiveOptions, pty xpty.Pty) *interactiveModel {
	return &interactiveModel{
		title:   opts.Title,
		cmdName: string(opts.CommandName),
		state:   stateExecuting,
		pty:     pty,
	}
}

// convertToProtocolResult converts a raw component result to a protocol-compliant struct.
// The tuiserver client expects specific JSON structures for each component type.
func convertToProtocolResult(componentType ComponentType, result any) any {
	switch componentType {
	case ComponentTypeInput, ComponentTypeTextArea, ComponentTypeWrite:
		// Input, TextArea, and Write return a string
		if s, ok := result.(string); ok {
			return tuiserver.InputResult{Value: s}
		}
		return tuiserver.InputResult{}

	case ComponentTypeConfirm:
		// Confirm returns a bool
		if b, ok := result.(bool); ok {
			return tuiserver.ConfirmResult{Confirmed: b}
		}
		return tuiserver.ConfirmResult{}

	case ComponentTypeChoose:
		// Choose returns []string
		if selected, ok := result.([]string); ok {
			return tuiserver.ChooseResult{Selected: selected}
		}
		return tuiserver.ChooseResult{Selected: []string{}}

	case ComponentTypeFilter:
		// Filter returns []string
		if selected, ok := result.([]string); ok {
			return tuiserver.FilterResult{Selected: selected}
		}
		return tuiserver.FilterResult{Selected: []string{}}

	case ComponentTypeFile:
		// File returns a string path
		if path, ok := result.(string); ok {
			return tuiserver.FileResult{Path: path}
		}
		return tuiserver.FileResult{}

	case ComponentTypeTable:
		// Table returns TableSelectionResult
		if tableResult, ok := result.(TableSelectionResult); ok {
			return tuiserver.TableResult{
				SelectedRow:   tableResult.SelectedRow,
				SelectedIndex: tableResult.SelectedIndex,
			}
		}
		return tuiserver.TableResult{SelectedIndex: -1}

	case ComponentTypePager:
		// Pager has no result
		return tuiserver.PagerResult{}

	case ComponentTypeSpin:
		// Spin returns SpinResult
		if spinResult, ok := result.(SpinResult); ok {
			return tuiserver.SpinResult{
				Stdout:   spinResult.Stdout,
				Stderr:   spinResult.Stderr,
				ExitCode: spinResult.ExitCode,
			}
		}
		return tuiserver.SpinResult{}

	default:
		// Unknown component type, return as-is
		return result
	}
}

// getTerminalSize attempts to get the current terminal size.
func getTerminalSize() (width, height int, err error) {
	// Try to get size from stdout
	fd := int(os.Stdout.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stderr
	fd = int(os.Stderr.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stdin
	fd = int(os.Stdin.Fd())
	return getTerminalSizeFromFd(fd)
}
