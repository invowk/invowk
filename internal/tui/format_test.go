// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestFormat_Markdown(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "# Hello World",
		Type:    FormatMarkdown,
		Config:  DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result should contain the header text (rendering may add ANSI codes)
	if !strings.Contains(result, "Hello World") {
		t.Errorf("expected result to contain 'Hello World', got %q", result)
	}
}

func TestFormat_Code(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content:  "func main() {}",
		Type:     FormatCode,
		Language: "go",
		Config:   DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result should contain the code (rendering may add ANSI codes)
	if !strings.Contains(result, "func") {
		t.Errorf("expected result to contain 'func', got %q", result)
	}
}

func TestFormat_Emoji(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"heart", ":heart:", "❤️"},
		{"star", ":star:", "⭐"},
		{"check", ":check:", "✓"},
		{"x", ":x:", "✗"},
		{"warning", ":warning:", "⚠️"},
		{"rocket", ":rocket:", "🚀"},
		{"fire", ":fire:", "🔥"},
		{"bug", ":bug:", "🐛"},
		{"wrench", ":wrench:", "🔧"},
		{"gear", ":gear:", "⚙️"},
		{"lock", ":lock:", "🔒"},
		{"key", ":key:", "🔑"},
		{"file", ":file:", "📄"},
		{"folder", ":folder:", "📁"},
		{"coffee", ":coffee:", "☕"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := FormatOptions{
				Content: tt.input,
				Type:    FormatEmoji,
				Config:  DefaultConfig(),
			}

			result, err := Format(opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormat_Emoji_MultipleReplacements(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "Status: :check: Done :rocket:",
		Type:    FormatEmoji,
		Config:  DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Status: ✓ Done 🚀"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormat_Emoji_NoReplacements(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "Plain text without emoji codes",
		Type:    FormatEmoji,
		Config:  DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != opts.Content {
		t.Errorf("expected unchanged content %q, got %q", opts.Content, result)
	}
}

func TestFormat_Template(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "Template content",
		Type:    FormatTemplate,
		Config:  DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Template formatting currently returns content unchanged
	if result != opts.Content {
		t.Errorf("expected unchanged content for template type")
	}
}

func TestFormat_UnknownType(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "Some content",
		Type:    "unknown",
		Config:  DefaultConfig(),
	}

	_, err := Format(opts)
	if err == nil {
		t.Fatal("Format() with unknown type should return error")
	}
	if !errors.Is(err, ErrInvalidFormatType) {
		t.Errorf("error should wrap ErrInvalidFormatType, got: %v", err)
	}
}

func TestFormat_MarkdownWithWidth(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content: "This is a long line of text that should be wrapped at a certain width for readability.",
		Type:    FormatMarkdown,
		Width:   40,
		Config:  DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be non-empty
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestFormatBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewFormat().
		Content("# Title").
		Type(FormatMarkdown).
		GlamourTheme("dark").
		Width(80)

	if builder.opts.Content != "# Title" {
		t.Errorf("expected content '# Title', got %q", builder.opts.Content)
	}
	if builder.opts.Type != FormatMarkdown {
		t.Errorf("expected type FormatMarkdown, got %q", builder.opts.Type)
	}
	if builder.opts.GlamourTheme != "dark" {
		t.Errorf("expected glamour theme 'dark', got %q", builder.opts.GlamourTheme)
	}
	if builder.opts.Width != 80 {
		t.Errorf("expected width 80, got %d", builder.opts.Width)
	}
}

func TestFormatBuilder_Markdown(t *testing.T) {
	t.Parallel()

	builder := NewFormat().
		Content("text").
		Markdown()

	if builder.opts.Type != FormatMarkdown {
		t.Errorf("expected type FormatMarkdown, got %q", builder.opts.Type)
	}
}

func TestFormatBuilder_Code(t *testing.T) {
	t.Parallel()

	builder := NewFormat().
		Content("code").
		Code().
		Language("python")

	if builder.opts.Type != FormatCode {
		t.Errorf("expected type FormatCode, got %q", builder.opts.Type)
	}
	if builder.opts.Language != "python" {
		t.Errorf("expected language 'python', got %q", builder.opts.Language)
	}
}

func TestFormatBuilder_Emoji(t *testing.T) {
	t.Parallel()

	builder := NewFormat().
		Content(":star:").
		Emoji()

	if builder.opts.Type != FormatEmoji {
		t.Errorf("expected type FormatEmoji, got %q", builder.opts.Type)
	}
}

func TestFormatBuilder_Run(t *testing.T) {
	t.Parallel()

	result, err := NewFormat().
		Content(":heart:").
		Emoji().
		Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "❤️" {
		t.Errorf("expected '❤️', got %q", result)
	}
}

func TestFormatBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewFormat()

	if builder.opts.Type != FormatMarkdown {
		t.Errorf("expected default type FormatMarkdown, got %q", builder.opts.Type)
	}
}

func TestFormatType_Constants(t *testing.T) {
	t.Parallel()

	if FormatMarkdown != "markdown" {
		t.Errorf("expected FormatMarkdown to be 'markdown', got %q", FormatMarkdown)
	}
	if FormatCode != "code" {
		t.Errorf("expected FormatCode to be 'code', got %q", FormatCode)
	}
	if FormatTemplate != "template" {
		t.Errorf("expected FormatTemplate to be 'template', got %q", FormatTemplate)
	}
	if FormatEmoji != "emoji" {
		t.Errorf("expected FormatEmoji to be 'emoji', got %q", FormatEmoji)
	}
}

func TestFormatOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content:      "test content",
		Type:         FormatCode,
		Language:     "go",
		GlamourTheme: "dark",
		Width:        100,
		Config: Config{
			Theme:      ThemeCharm,
			Accessible: true,
		},
	}

	if opts.Content != "test content" {
		t.Errorf("expected content 'test content', got %q", opts.Content)
	}
	if opts.Type != FormatCode {
		t.Errorf("expected type FormatCode, got %q", opts.Type)
	}
	if opts.Language != "go" {
		t.Errorf("expected language 'go', got %q", opts.Language)
	}
	if opts.GlamourTheme != "dark" {
		t.Errorf("expected glamour theme 'dark', got %q", opts.GlamourTheme)
	}
	if opts.Width != 100 {
		t.Errorf("expected width 100, got %d", opts.Width)
	}
	if opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
}

func TestFormatCode_EmptyLanguage(t *testing.T) {
	t.Parallel()

	opts := FormatOptions{
		Content:  "some code",
		Type:     FormatCode,
		Language: "", // Empty language
		Config:   DefaultConfig(),
	}

	result, err := Format(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work with empty language
	if !strings.Contains(result, "some code") {
		t.Errorf("expected result to contain 'some code', got %q", result)
	}
}

func TestEmojiMap_Completeness(t *testing.T) {
	t.Parallel()

	// Test that all documented emoji codes work
	codes := []string{
		":heart:", ":star:", ":check:", ":x:", ":warning:", ":info:",
		":question:", ":exclamation:", ":thumbsup:", ":thumbsdown:",
		":fire:", ":rocket:", ":sparkles:", ":tada:", ":bug:", ":wrench:",
		":hammer:", ":gear:", ":lock:", ":unlock:", ":key:", ":clipboard:",
		":memo:", ":book:", ":folder:", ":file:", ":trash:",
		":magnifying_glass:", ":hourglass:", ":clock:", ":calendar:",
		":chart:", ":link:", ":email:", ":phone:", ":computer:",
		":keyboard:", ":mouse:", ":printer:", ":cloud:", ":sun:", ":moon:",
		":rainbow:", ":umbrella:", ":snowflake:", ":zap:", ":droplet:",
		":tree:", ":flower:", ":candy:", ":pizza:", ":coffee:", ":beer:",
		":wine:", ":cake:", ":gift:", ":balloon:", ":trophy:", ":medal:",
		":crown:", ":gem:", ":money:", ":dollar:", ":credit_card:", ":bell:",
		":speaker:", ":mute:", ":musical_note:", ":microphone:", ":headphones:",
		":camera:", ":video_camera:", ":movie:", ":tv:", ":radio:",
		":battery:", ":plug:", ":bulb:", ":flashlight:", ":satellite:",
		":package:", ":mailbox:", ":pencil:", ":pen:", ":paintbrush:",
		":ruler:", ":scissors:", ":pushpin:", ":paperclip:",
	}

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			opts := FormatOptions{
				Content: code,
				Type:    FormatEmoji,
				Config:  DefaultConfig(),
			}

			result, err := Format(opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Result should be different from input (emoji was replaced)
			if result == code {
				t.Errorf("emoji code %s was not replaced", code)
			}
		})
	}
}

func TestFormatType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ft   FormatType
		want string
	}{
		{FormatMarkdown, "markdown"},
		{FormatCode, "code"},
		{FormatTemplate, "template"},
		{FormatEmoji, "emoji"},
		{FormatType("custom"), "custom"},
		{FormatType(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.ft.String()
			if got != tt.want {
				t.Errorf("FormatType(%q).String() = %q, want %q", tt.ft, got, tt.want)
			}
		})
	}
}

func TestFormatType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ft      FormatType
		want    bool
		wantErr bool
	}{
		{FormatMarkdown, true, false},
		{FormatCode, true, false},
		{FormatTemplate, true, false},
		{FormatEmoji, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"MARKDOWN", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			t.Parallel()
			err := tt.ft.Validate()
			if (err == nil) != tt.want {
				t.Errorf("FormatType(%q).Validate() err = %v, wantValid %v", tt.ft, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FormatType(%q).Validate() returned nil, want error", tt.ft)
				}
				if !errors.Is(err, ErrInvalidFormatType) {
					t.Errorf("error should wrap ErrInvalidFormatType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("FormatType(%q).Validate() returned unexpected error: %v", tt.ft, err)
			}
		})
	}
}

func TestFormat_InvalidType(t *testing.T) {
	t.Parallel()

	_, err := Format(FormatOptions{
		Content: "test",
		Type:    FormatType("invalid"),
	})
	if err == nil {
		t.Fatal("Format() with invalid type should return error")
	}
	if !errors.Is(err, ErrInvalidFormatType) {
		t.Errorf("error should wrap ErrInvalidFormatType, got: %v", err)
	}
}
