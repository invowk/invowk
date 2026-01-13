// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"context"
	"os/exec"

	"github.com/charmbracelet/huh/spinner"
)

// SpinnerType represents the type of spinner animation.
type SpinnerType int

const (
	// SpinnerLine is a simple line spinner.
	SpinnerLine SpinnerType = iota
	// SpinnerDot is a dot spinner.
	SpinnerDot
	// SpinnerMiniDot is a mini dot spinner.
	SpinnerMiniDot
	// SpinnerJump is a jumping spinner.
	SpinnerJump
	// SpinnerPulse is a pulsing spinner.
	SpinnerPulse
	// SpinnerPoints is a points spinner.
	SpinnerPoints
	// SpinnerGlobe is a globe spinner.
	SpinnerGlobe
	// SpinnerMoon is a moon phases spinner.
	SpinnerMoon
	// SpinnerMonkey is a monkey spinner.
	SpinnerMonkey
	// SpinnerMeter is a meter spinner.
	SpinnerMeter
	// SpinnerHamburger is a hamburger spinner.
	SpinnerHamburger
	// SpinnerEllipsis is an ellipsis spinner.
	SpinnerEllipsis
)

// ParseSpinnerType parses a string into a SpinnerType.
func ParseSpinnerType(s string) SpinnerType {
	switch s {
	case "line":
		return SpinnerLine
	case "dot":
		return SpinnerDot
	case "minidot":
		return SpinnerMiniDot
	case "jump":
		return SpinnerJump
	case "pulse":
		return SpinnerPulse
	case "points":
		return SpinnerPoints
	case "globe":
		return SpinnerGlobe
	case "moon":
		return SpinnerMoon
	case "monkey":
		return SpinnerMonkey
	case "meter":
		return SpinnerMeter
	case "hamburger":
		return SpinnerHamburger
	case "ellipsis":
		return SpinnerEllipsis
	default:
		return SpinnerLine
	}
}

// SpinnerTypeNames returns the list of available spinner type names.
func SpinnerTypeNames() []string {
	return []string{
		"line", "dot", "minidot", "jump", "pulse", "points",
		"globe", "moon", "monkey", "meter", "hamburger", "ellipsis",
	}
}

// SpinOptions configures the Spin component.
type SpinOptions struct {
	// Title is the text displayed next to the spinner.
	Title string
	// Type specifies the spinner animation type.
	Type SpinnerType
	// Config holds common TUI configuration.
	Config Config
}

// getSpinnerType converts SpinnerType to spinner.Type.
func getSpinnerType(t SpinnerType) spinner.Type {
	switch t {
	case SpinnerLine:
		return spinner.Line
	case SpinnerDot:
		return spinner.Dots
	case SpinnerMiniDot:
		return spinner.MiniDot
	case SpinnerJump:
		return spinner.Jump
	case SpinnerPulse:
		return spinner.Pulse
	case SpinnerPoints:
		return spinner.Points
	case SpinnerGlobe:
		return spinner.Globe
	case SpinnerMoon:
		return spinner.Moon
	case SpinnerMonkey:
		return spinner.Monkey
	case SpinnerMeter:
		return spinner.Meter
	case SpinnerHamburger:
		return spinner.Hamburger
	case SpinnerEllipsis:
		return spinner.Ellipsis
	default:
		return spinner.Line
	}
}

// SpinWithAction displays a spinner while running an action function.
// The spinner stops when the action completes.
func SpinWithAction(opts SpinOptions, action func()) error {
	s := spinner.New().
		Title(opts.Title).
		Type(getSpinnerType(opts.Type)).
		Action(action)

	if shouldUseAccessible(opts.Config) {
		s = s.Accessible(true)
	}

	return s.Run()
}

// SpinWithContext displays a spinner until the context is cancelled.
func SpinWithContext(opts SpinOptions, ctx context.Context) error {
	s := spinner.New().
		Title(opts.Title).
		Type(getSpinnerType(opts.Type)).
		Context(ctx)

	if shouldUseAccessible(opts.Config) {
		s = s.Accessible(true)
	}

	return s.Run()
}

// SpinWithCommand displays a spinner while running a shell command.
// Returns the command output and any error.
func SpinWithCommand(opts SpinOptions, command string, args ...string) ([]byte, error) {
	var output []byte
	var cmdErr error

	action := func() {
		cmd := exec.Command(command, args...)
		output, cmdErr = cmd.CombinedOutput()
	}

	if err := SpinWithAction(opts, action); err != nil {
		return nil, err
	}

	return output, cmdErr
}

// SpinBuilder provides a fluent API for building Spin prompts.
type SpinBuilder struct {
	opts   SpinOptions
	action func()
	ctx    context.Context
}

// NewSpin creates a new SpinBuilder with default options.
func NewSpin() *SpinBuilder {
	return &SpinBuilder{
		opts: SpinOptions{
			Type:   SpinnerLine,
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title displayed next to the spinner.
func (b *SpinBuilder) Title(title string) *SpinBuilder {
	b.opts.Title = title
	return b
}

// Type sets the spinner animation type.
func (b *SpinBuilder) Type(t SpinnerType) *SpinBuilder {
	b.opts.Type = t
	return b
}

// TypeString sets the spinner type by name.
func (b *SpinBuilder) TypeString(t string) *SpinBuilder {
	b.opts.Type = ParseSpinnerType(t)
	return b
}

// Theme sets the visual theme.
func (b *SpinBuilder) Theme(theme Theme) *SpinBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *SpinBuilder) Accessible(accessible bool) *SpinBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Action sets the action to run while the spinner is displayed.
func (b *SpinBuilder) Action(action func()) *SpinBuilder {
	b.action = action
	return b
}

// Context sets a context that controls when the spinner stops.
func (b *SpinBuilder) Context(ctx context.Context) *SpinBuilder {
	b.ctx = ctx
	return b
}

// Run executes the spinner with the configured action or context.
func (b *SpinBuilder) Run() error {
	if b.ctx != nil {
		return SpinWithContext(b.opts, b.ctx)
	}
	if b.action != nil {
		return SpinWithAction(b.opts, b.action)
	}
	// If no action or context, just return immediately
	return nil
}
