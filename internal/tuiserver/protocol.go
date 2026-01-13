// SPDX-License-Identifier: EPL-2.0

// Package tuiserver provides an HTTP server for TUI component rendering.
//
// When invowk runs in interactive mode (-i flag), it starts an HTTP server
// that listens for TUI rendering requests from child processes. This allows
// nested `invowk tui *` commands to render their TUI components through the
// parent's terminal connection, avoiding conflicts between multiple TUI
// instances fighting for terminal control.
//
// # Architecture
//
// Parent invowk (with -i flag):
//   - Starts HTTP server on localhost with random port
//   - Sets INVOWK_TUI_ADDR and INVOWK_TUI_TOKEN environment variables
//   - Enters alternate screen buffer
//   - Executes user command with stdout/stderr capture
//   - Handles incoming TUI requests by rendering components to terminal
//
// Child invowk tui commands:
//   - Detect INVOWK_TUI_ADDR environment variable
//   - Send HTTP POST with TUI parameters to parent server
//   - Receive result in HTTP response body
//   - Print result to stdout (for shell variable capture)
//
// # Security
//
// The server uses a random token for authentication. Only requests with
// the correct token (passed via INVOWK_TUI_TOKEN) are accepted. The server
// only listens on localhost to prevent remote access.
package tuiserver

import "encoding/json"

// Environment variable names for TUI server communication.
const (
	// EnvTUIAddr is the environment variable containing the TUI server address.
	// Example: "http://127.0.0.1:54321"
	EnvTUIAddr = "INVOWK_TUI_ADDR"

	// EnvTUIToken is the environment variable containing the authentication token.
	EnvTUIToken = "INVOWK_TUI_TOKEN"
)

// Component represents a TUI component type.
type Component string

const (
	ComponentInput    Component = "input"
	ComponentConfirm  Component = "confirm"
	ComponentChoose   Component = "choose"
	ComponentFilter   Component = "filter"
	ComponentFile     Component = "file"
	ComponentWrite    Component = "write"
	ComponentTextArea Component = "textarea"
	ComponentSpin     Component = "spin"
	ComponentPager    Component = "pager"
	ComponentTable    Component = "table"
)

// Request is the common wrapper for all TUI requests.
type Request struct {
	// Component is the TUI component type (input, confirm, choose, etc.).
	Component Component `json:"component"`
	// Options contains component-specific options as raw JSON.
	Options json.RawMessage `json:"options"`
}

// Response is the common wrapper for all TUI responses.
type Response struct {
	// Result contains the component-specific result as raw JSON.
	// For input: {"value": "user input"}
	// For confirm: {"confirmed": true}
	// For choose: {"selected": "option1"} or {"selected": ["opt1", "opt2"]}
	Result json.RawMessage `json:"result,omitempty"`
	// Cancelled is true if the user cancelled the prompt (Ctrl+C, Esc).
	Cancelled bool `json:"cancelled,omitempty"`
	// Error contains an error message if the request failed.
	Error string `json:"error,omitempty"`
}

// InputRequest contains options for the input component.
type InputRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Value       string `json:"value,omitempty"`
	CharLimit   int    `json:"char_limit,omitempty"`
	Width       int    `json:"width,omitempty"`
	Password    bool   `json:"password,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
}

// InputResult contains the result of an input prompt.
type InputResult struct {
	Value string `json:"value"`
}

// ConfirmRequest contains options for the confirm component.
type ConfirmRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Affirmative string `json:"affirmative,omitempty"`
	Negative    string `json:"negative,omitempty"`
	Default     bool   `json:"default,omitempty"`
}

// ConfirmResult contains the result of a confirm prompt.
type ConfirmResult struct {
	Confirmed bool `json:"confirmed"`
}

// ChooseRequest contains options for the choose component.
type ChooseRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options"`
	Selected    string   `json:"selected,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	NoLimit     bool     `json:"no_limit,omitempty"`
	Ordered     bool     `json:"ordered,omitempty"`
	Height      int      `json:"height,omitempty"`
	Cursor      string   `json:"cursor,omitempty"`
}

// ChooseResult contains the result of a choose prompt.
type ChooseResult struct {
	// Selected contains the selected option (single select) or options (multi-select).
	Selected interface{} `json:"selected"`
}

// FilterRequest contains options for the filter component.
type FilterRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options"`
	Limit       int      `json:"limit,omitempty"`
	NoLimit     bool     `json:"no_limit,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	Value       string   `json:"value,omitempty"`
	Reverse     bool     `json:"reverse,omitempty"`
	Fuzzy       bool     `json:"fuzzy,omitempty"`
	Sort        bool     `json:"sort,omitempty"`
	Strict      bool     `json:"strict,omitempty"`
}

// FilterResult contains the result of a filter prompt.
type FilterResult struct {
	Selected []string `json:"selected"`
}

// FileRequest contains options for the file picker component.
type FileRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Path        string   `json:"path,omitempty"`
	AllowedExts []string `json:"allowed_exts,omitempty"`
	ShowHidden  bool     `json:"show_hidden,omitempty"`
	ShowDirs    bool     `json:"show_dirs,omitempty"`
	ShowFiles   bool     `json:"show_files,omitempty"`
	Height      int      `json:"height,omitempty"`
}

// FileResult contains the result of a file picker.
type FileResult struct {
	Path string `json:"path"`
}

// WriteRequest contains options for the write component (styled text output).
type WriteRequest struct {
	Text string `json:"text"`
	// Style options
	Foreground       string `json:"foreground,omitempty"`
	Background       string `json:"background,omitempty"`
	Bold             bool   `json:"bold,omitempty"`
	Italic           bool   `json:"italic,omitempty"`
	Underline        bool   `json:"underline,omitempty"`
	Strikethrough    bool   `json:"strikethrough,omitempty"`
	Faint            bool   `json:"faint,omitempty"`
	Blink            bool   `json:"blink,omitempty"`
	Border           string `json:"border,omitempty"`
	BorderForeground string `json:"border_foreground,omitempty"`
	Width            int    `json:"width,omitempty"`
	Align            string `json:"align,omitempty"`
	Padding          []int  `json:"padding,omitempty"`
	Margin           []int  `json:"margin,omitempty"`
}

// WriteResult is empty (write doesn't return a value).
type WriteResult struct{}

// TextAreaRequest contains options for the textarea component (multi-line text input).
type TextAreaRequest struct {
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	Placeholder     string `json:"placeholder,omitempty"`
	Value           string `json:"value,omitempty"`
	CharLimit       int    `json:"char_limit,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	ShowLineNumbers bool   `json:"show_line_numbers,omitempty"`
}

// TextAreaResult contains the result of a textarea prompt.
type TextAreaResult struct {
	Value string `json:"value"`
}

// SpinRequest contains options for the spin component.
type SpinRequest struct {
	Title   string   `json:"title,omitempty"`
	Spinner string   `json:"spinner,omitempty"`
	Command []string `json:"command,omitempty"`
}

// SpinResult contains the result of a spin operation.
type SpinResult struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// PagerRequest contains options for the pager component.
type PagerRequest struct {
	Content     string `json:"content"`
	ShowLineNum bool   `json:"show_line_num,omitempty"`
	SoftWrap    bool   `json:"soft_wrap,omitempty"`
}

// PagerResult is empty (pager doesn't return a value).
type PagerResult struct{}

// TableRequest contains options for the table component.
type TableRequest struct {
	Columns   []string   `json:"columns,omitempty"`
	Rows      [][]string `json:"rows"`
	Widths    []int      `json:"widths,omitempty"`
	Height    int        `json:"height,omitempty"`
	Separator string     `json:"separator,omitempty"`
	Border    string     `json:"border,omitempty"`
	Print     bool       `json:"print,omitempty"`
}

// TableResult contains the result of a table selection.
type TableResult struct {
	SelectedRow   []string `json:"selected_row,omitempty"`
	SelectedIndex int      `json:"selected_index"`
}
