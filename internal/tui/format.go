package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// FormatType specifies the type of content to format.
type FormatType string

const (
	// FormatMarkdown formats content as Markdown.
	FormatMarkdown FormatType = "markdown"
	// FormatCode formats content as code with syntax highlighting.
	FormatCode FormatType = "code"
	// FormatTemplate formats content using Go templates.
	FormatTemplate FormatType = "template"
	// FormatEmoji formats content with emoji substitution.
	FormatEmoji FormatType = "emoji"
)

// FormatOptions configures the Format component.
type FormatOptions struct {
	// Content is the text content to format.
	Content string
	// Type specifies how to format the content.
	Type FormatType
	// Language is the language for code syntax highlighting.
	Language string
	// Theme is the glamour theme for markdown rendering.
	GlamourTheme string
	// Width is the word wrap width (0 for no wrap).
	Width int
	// Config holds common TUI configuration.
	Config Config
}

// Format formats content according to the specified type.
func Format(opts FormatOptions) (string, error) {
	switch opts.Type {
	case FormatMarkdown:
		return formatMarkdown(opts)
	case FormatCode:
		return formatCode(opts)
	case FormatEmoji:
		return formatEmoji(opts)
	case FormatTemplate:
		// Template formatting would require additional templating logic
		return opts.Content, nil
	default:
		return opts.Content, nil
	}
}

// formatMarkdown renders markdown content using glamour.
func formatMarkdown(opts FormatOptions) (string, error) {
	theme := opts.GlamourTheme
	if theme == "" {
		theme = "auto"
	}

	var rendererOpts []glamour.TermRendererOption
	rendererOpts = append(rendererOpts, glamour.WithAutoStyle())

	if opts.Width > 0 {
		rendererOpts = append(rendererOpts, glamour.WithWordWrap(opts.Width))
	}

	renderer, err := glamour.NewTermRenderer(rendererOpts...)
	if err != nil {
		return "", err
	}

	return renderer.Render(opts.Content)
}

// formatCode wraps content in a code block for markdown rendering.
func formatCode(opts FormatOptions) (string, error) {
	lang := opts.Language
	if lang == "" {
		lang = ""
	}

	// Wrap in markdown code block
	content := "```" + lang + "\n" + opts.Content + "\n```"

	return formatMarkdown(FormatOptions{
		Content:      content,
		Type:         FormatMarkdown,
		GlamourTheme: opts.GlamourTheme,
		Width:        opts.Width,
		Config:       opts.Config,
	})
}

// formatEmoji substitutes emoji shortcodes with actual emoji.
// Supports common shortcodes like :heart:, :star:, :check:, etc.
func formatEmoji(opts FormatOptions) (string, error) {
	content := opts.Content

	// Common emoji replacements
	emojiMap := map[string]string{
		":heart:":           "❤️",
		":star:":            "⭐",
		":check:":           "✓",
		":x:":               "✗",
		":warning:":         "⚠️",
		":info:":            "ℹ️",
		":question:":        "❓",
		":exclamation:":     "❗",
		":thumbsup:":        "👍",
		":thumbsdown:":      "👎",
		":fire:":            "🔥",
		":rocket:":          "🚀",
		":sparkles:":        "✨",
		":tada:":            "🎉",
		":bug:":             "🐛",
		":wrench:":          "🔧",
		":hammer:":          "🔨",
		":gear:":            "⚙️",
		":lock:":            "🔒",
		":unlock:":          "🔓",
		":key:":             "🔑",
		":clipboard:":       "📋",
		":memo:":            "📝",
		":book:":            "📖",
		":folder:":          "📁",
		":file:":            "📄",
		":trash:":           "🗑️",
		":magnifying_glass:": "🔍",
		":hourglass:":       "⏳",
		":clock:":           "🕐",
		":calendar:":        "📅",
		":chart:":           "📊",
		":link:":            "🔗",
		":email:":           "📧",
		":phone:":           "📞",
		":computer:":        "💻",
		":keyboard:":        "⌨️",
		":mouse:":           "🖱️",
		":printer:":         "🖨️",
		":cloud:":           "☁️",
		":sun:":             "☀️",
		":moon:":            "🌙",
		":rainbow:":         "🌈",
		":umbrella:":        "☂️",
		":snowflake:":       "❄️",
		":zap:":             "⚡",
		":droplet:":         "💧",
		":tree:":            "🌲",
		":flower:":          "🌸",
		":candy:":           "🍬",
		":pizza:":           "🍕",
		":coffee:":          "☕",
		":beer:":            "🍺",
		":wine:":            "🍷",
		":cake:":            "🎂",
		":gift:":            "🎁",
		":balloon:":         "🎈",
		":trophy:":          "🏆",
		":medal:":           "🏅",
		":crown:":           "👑",
		":gem:":             "💎",
		":money:":           "💰",
		":dollar:":          "💵",
		":credit_card:":     "💳",
		":bell:":            "🔔",
		":speaker:":         "🔊",
		":mute:":            "🔇",
		":musical_note:":    "🎵",
		":microphone:":      "🎤",
		":headphones:":      "🎧",
		":camera:":          "📷",
		":video_camera:":    "📹",
		":movie:":           "🎬",
		":tv:":              "📺",
		":radio:":           "📻",
		":battery:":         "🔋",
		":plug:":            "🔌",
		":bulb:":            "💡",
		":flashlight:":      "🔦",
		":satellite:":       "📡",
		":package:":         "📦",
		":mailbox:":         "📬",
		":pencil:":          "✏️",
		":pen:":             "🖊️",
		":paintbrush:":      "🖌️",
		":ruler:":           "📏",
		":scissors:":        "✂️",
		":pushpin:":         "📌",
		":paperclip:":       "📎",
	}

	for code, emoji := range emojiMap {
		content = strings.ReplaceAll(content, code, emoji)
	}

	return content, nil
}

// FormatBuilder provides a fluent API for building Format operations.
type FormatBuilder struct {
	opts FormatOptions
}

// NewFormat creates a new FormatBuilder with default options.
func NewFormat() *FormatBuilder {
	return &FormatBuilder{
		opts: FormatOptions{
			Type:   FormatMarkdown,
			Config: DefaultConfig(),
		},
	}
}

// Content sets the content to format.
func (b *FormatBuilder) Content(content string) *FormatBuilder {
	b.opts.Content = content
	return b
}

// Type sets the format type.
func (b *FormatBuilder) Type(t FormatType) *FormatBuilder {
	b.opts.Type = t
	return b
}

// Markdown sets the format type to markdown.
func (b *FormatBuilder) Markdown() *FormatBuilder {
	b.opts.Type = FormatMarkdown
	return b
}

// Code sets the format type to code.
func (b *FormatBuilder) Code() *FormatBuilder {
	b.opts.Type = FormatCode
	return b
}

// Emoji sets the format type to emoji.
func (b *FormatBuilder) Emoji() *FormatBuilder {
	b.opts.Type = FormatEmoji
	return b
}

// Language sets the language for code highlighting.
func (b *FormatBuilder) Language(lang string) *FormatBuilder {
	b.opts.Language = lang
	return b
}

// GlamourTheme sets the glamour theme.
func (b *FormatBuilder) GlamourTheme(theme string) *FormatBuilder {
	b.opts.GlamourTheme = theme
	return b
}

// Width sets the word wrap width.
func (b *FormatBuilder) Width(width int) *FormatBuilder {
	b.opts.Width = width
	return b
}

// Run formats the content and returns the result.
func (b *FormatBuilder) Run() (string, error) {
	return Format(b.opts)
}
