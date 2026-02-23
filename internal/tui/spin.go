// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
)

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

// ErrInvalidSpinnerType is the sentinel error wrapped by InvalidSpinnerTypeError.
var ErrInvalidSpinnerType = errors.New("invalid spinner type")

// All type declarations consolidated in a single block.
type (
	// SpinnerType represents the type of spinner animation.
	SpinnerType int

	// InvalidSpinnerTypeError is returned when a SpinnerType value is not
	// one of the defined spinner types.
	InvalidSpinnerTypeError struct {
		Value SpinnerType
	}

	// SpinOptions configures the Spin component.
	SpinOptions struct {
		// Title is the text displayed next to the spinner.
		Title string
		// Type specifies the spinner animation type.
		Type SpinnerType
		// Config holds common TUI configuration.
		Config Config
	}

	// SpinCommandOptions configures an embeddable spin component with a command.
	SpinCommandOptions struct {
		// Title is the text displayed next to the spinner.
		Title string
		// Command is the command and arguments to execute.
		Command []string
		// Type specifies the spinner animation type.
		Type SpinnerType
		// Config holds common TUI configuration.
		Config Config
	}

	// spinModel implements EmbeddableComponent for spinner with command execution.
	spinModel struct {
		title   string
		command []string
		done    bool
		result  SpinResult
		width   int
		height  int
		spinner int
		frames  []string
	}

	// spinnerTickMsg is sent to animate the spinner.
	spinnerTickMsg struct{}

	// spinnerDoneMsg is sent when the command completes.
	spinnerDoneMsg struct {
		result SpinResult
	}

	// SpinBuilder provides a fluent API for building Spin prompts.
	SpinBuilder struct {
		opts   SpinOptions
		action func()
		ctx    context.Context
	}
)

// Error implements the error interface.
func (e *InvalidSpinnerTypeError) Error() string {
	return fmt.Sprintf("invalid spinner type %d (valid: %s)",
		e.Value, strings.Join(SpinnerTypeNames(), ", "))
}

// Unwrap returns ErrInvalidSpinnerType so callers can use errors.Is for programmatic detection.
func (e *InvalidSpinnerTypeError) Unwrap() error { return ErrInvalidSpinnerType }

// IsValid returns whether the SpinnerType is one of the defined spinner types,
// and a list of validation errors if it is not.
func (t SpinnerType) IsValid() (bool, []error) {
	switch t {
	case SpinnerLine, SpinnerDot, SpinnerMiniDot, SpinnerJump,
		SpinnerPulse, SpinnerPoints, SpinnerGlobe, SpinnerMoon,
		SpinnerMonkey, SpinnerMeter, SpinnerHamburger, SpinnerEllipsis:
		return true, nil
	default:
		return false, []error{&InvalidSpinnerTypeError{Value: t}}
	}
}

// ParseSpinnerType parses a string into a SpinnerType.
// Returns an error if the string does not match any known spinner type name.
func ParseSpinnerType(s string) (SpinnerType, error) {
	switch s {
	case "line":
		return SpinnerLine, nil
	case "dot":
		return SpinnerDot, nil
	case "minidot":
		return SpinnerMiniDot, nil
	case "jump":
		return SpinnerJump, nil
	case "pulse":
		return SpinnerPulse, nil
	case "points":
		return SpinnerPoints, nil
	case "globe":
		return SpinnerGlobe, nil
	case "moon":
		return SpinnerMoon, nil
	case "monkey":
		return SpinnerMonkey, nil
	case "meter":
		return SpinnerMeter, nil
	case "hamburger":
		return SpinnerHamburger, nil
	case "ellipsis":
		return SpinnerEllipsis, nil
	default:
		return 0, fmt.Errorf("unknown spinner type %q (valid: %s)", s, strings.Join(SpinnerTypeNames(), ", "))
	}
}

// SpinnerTypeNames returns the list of available spinner type names.
func SpinnerTypeNames() []string {
	return []string{
		"line", "dot", "minidot", "jump", "pulse", "points",
		"globe", "moon", "monkey", "meter", "hamburger", "ellipsis",
	}
}

// NewSpinModel creates an embeddable spinner component.
func NewSpinModel(opts SpinCommandOptions) *spinModel {
	if len(opts.Command) == 0 {
		// No command - return immediately done
		return &spinModel{
			done: true,
		}
	}

	return &spinModel{
		title:   opts.Title,
		command: opts.Command,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Init implements tea.Model.
func (m *spinModel) Init() tea.Cmd {
	// Start the command and spinner tick
	return tea.Batch(
		m.runCommand(),
		m.tick(),
	)
}

// Update implements tea.Model.
func (m *spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerTickMsg:
		if !m.done {
			m.spinner = (m.spinner + 1) % len(m.frames)
			cmd := m.tick()
			return m, cmd
		}
	case spinnerDoneMsg:
		m.done = true
		m.result = msg.result
		return m, nil
	case tea.KeyMsg:
		if msg.String() == keyCtrlC {
			m.done = true
			return m, nil
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *spinModel) View() string {
	if m.done {
		return ""
	}

	frame := m.frames[m.spinner]
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	content := spinnerStyle.Render(frame) + " " + titleStyle.Render(m.title)

	// Constrain the view to the configured width to prevent overflow in modal overlays
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(content)
	}
	return content
}

// IsDone implements EmbeddableComponent.
func (m *spinModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
func (m *spinModel) Result() (any, error) {
	return m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *spinModel) Cancelled() bool {
	return false
}

// SetSize implements EmbeddableComponent.
func (m *spinModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// runCommand starts the command execution and returns the result.
func (m *spinModel) runCommand() tea.Cmd {
	return func() tea.Msg {
		if len(m.command) == 0 {
			return spinnerDoneMsg{result: SpinResult{}}
		}

		cmd := exec.CommandContext(context.Background(), m.command[0], m.command[1:]...)
		output, err := cmd.CombinedOutput()

		result := SpinResult{
			Stdout: string(output),
		}

		if err != nil {
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				result.ExitCode = types.ExitCode(exitErr.ExitCode())
			} else {
				result.ExitCode = 1
			}
		}

		return spinnerDoneMsg{result: result}
	}
}

// tick returns a command that sends a spinnerTickMsg after a short delay.
func (m *spinModel) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// SpinWithAction displays a spinner while running an action function.
// The spinner stops when the action completes.
func SpinWithAction(opts SpinOptions, action func()) error {
	s := spinner.New().
		Title(opts.Title).
		Type(getSpinnerType(opts.Type)).
		Action(action)

	if opts.Config.Accessible {
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

	if opts.Config.Accessible {
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
		cmd := exec.CommandContext(context.Background(), command, args...)
		output, cmdErr = cmd.CombinedOutput()
	}

	if err := SpinWithAction(opts, action); err != nil {
		return nil, err
	}

	return output, cmdErr
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
// Unknown names are silently ignored, keeping the current type.
func (b *SpinBuilder) TypeString(t string) *SpinBuilder {
	if parsed, err := ParseSpinnerType(t); err == nil {
		b.opts.Type = parsed
	}
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
