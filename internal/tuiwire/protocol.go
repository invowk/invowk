// SPDX-License-Identifier: MPL-2.0

// Package tuiwire defines the shared wire vocabulary for delegated TUI
// components.
package tuiwire

import (
	"encoding/json"

	"github.com/invowk/invowk/internal/tui/component"
)

// Environment variable names for TUI server communication.
const (
	// EnvTUIAddr is the environment variable containing the TUI server address.
	EnvTUIAddr = "INVOWK_TUI_ADDR"

	// EnvTUIToken is the environment variable containing the authentication token.
	//nolint:gosec // G101: This is an env var name, not a hardcoded credential
	EnvTUIToken = "INVOWK_TUI_TOKEN"

	// ComponentInput represents the text input component.
	ComponentInput Component = component.TypeInput
	// ComponentConfirm represents the yes/no confirmation component.
	ComponentConfirm Component = component.TypeConfirm
	// ComponentChoose represents the single/multi-select component.
	ComponentChoose Component = component.TypeChoose
	// ComponentFilter represents the filterable list component.
	ComponentFilter Component = component.TypeFilter
	// ComponentFile represents the file picker component.
	ComponentFile Component = component.TypeFile
	// ComponentWrite represents the styled text output component.
	ComponentWrite Component = component.TypeWrite
	// ComponentTextArea represents the multi-line text input component.
	ComponentTextArea Component = component.TypeTextArea
	// ComponentSpin represents the spinner/loading component.
	ComponentSpin Component = component.TypeSpin
	// ComponentPager represents the scrollable text viewer component.
	ComponentPager Component = component.TypePager
	// ComponentTable represents the table selection component.
	ComponentTable Component = component.TypeTable
)

// ErrInvalidComponent is returned when a Component value is not one of the defined types.
var ErrInvalidComponent = component.ErrInvalidType

type (
	// Component represents a delegated TUI component type.
	Component = component.Type

	// Request is the common wrapper for all TUI requests.
	Request struct {
		Component Component       `json:"component"`
		Options   json.RawMessage `json:"options"`
	}

	// Response is the common wrapper for all TUI responses.
	Response struct {
		Result    json.RawMessage `json:"result,omitempty"`
		Cancelled bool            `json:"cancelled,omitempty"`
		Error     string          `json:"error,omitempty"`
	}

	// InputRequest contains options for the input component.
	InputRequest struct {
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
	InputResult struct {
		Value string `json:"value"`
	}

	// ConfirmRequest contains options for the confirm component.
	ConfirmRequest struct {
		Title       string `json:"title,omitempty"`
		Description string `json:"description,omitempty"`
		Affirmative string `json:"affirmative,omitempty"`
		Negative    string `json:"negative,omitempty"`
		Default     bool   `json:"default,omitempty"`
	}

	// ConfirmResult contains the result of a confirm prompt.
	ConfirmResult struct {
		Confirmed bool `json:"confirmed"`
	}

	// ChooseRequest contains options for the choose component.
	ChooseRequest struct {
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
	ChooseResult struct {
		Selected any `json:"selected"`
	}

	// FilterRequest contains options for the filter component.
	FilterRequest struct {
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
	FilterResult struct {
		Selected []string `json:"selected"`
	}

	// FileRequest contains options for the file picker component.
	FileRequest struct {
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
	FileResult struct {
		Path string `json:"path"`
	}

	// WriteRequest contains options for the write component.
	WriteRequest struct {
		Text             string `json:"text"`
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

	// WriteResult is empty because write does not return a value.
	WriteResult struct{}

	// TextAreaRequest contains options for the textarea component.
	TextAreaRequest struct {
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
	TextAreaResult struct {
		Value string `json:"value"`
	}

	// SpinRequest contains options for the spin component.
	SpinRequest struct {
		Title   string `json:"title,omitempty"`
		Spinner string `json:"spinner,omitempty"`
	}

	// SpinResult is empty because delegated spin is a presentation-only overlay.
	SpinResult struct{}

	// PagerRequest contains options for the pager component.
	PagerRequest struct {
		Title       string `json:"title,omitempty"`
		Content     string `json:"content"`
		ShowLineNum bool   `json:"show_line_num,omitempty"`
		SoftWrap    bool   `json:"soft_wrap,omitempty"`
	}

	// PagerResult is empty because pager does not return a value.
	PagerResult struct{}

	// TableRequest contains options for the table component.
	TableRequest struct {
		Columns   []string   `json:"columns,omitempty"`
		Rows      [][]string `json:"rows"`
		Widths    []int      `json:"widths,omitempty"`
		Height    int        `json:"height,omitempty"`
		Separator string     `json:"separator,omitempty"`
		Border    string     `json:"border,omitempty"`
		Print     bool       `json:"print,omitempty"`
	}

	// TableResult contains the result of a table selection.
	TableResult struct {
		SelectedRow   []string `json:"selected_row,omitempty"`
		SelectedIndex int      `json:"selected_index"`
	}

	// InvalidComponentError is returned when a Component value is not recognized.
	// It wraps ErrInvalidComponent for errors.Is() compatibility.
	InvalidComponentError = component.InvalidTypeError
)
