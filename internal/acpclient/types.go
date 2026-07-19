// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

const (
	defaultClientName      clientName    = "invowk"
	defaultClientVersion   clientVersion = "internal-acp-foundation"
	defaultShutdownTimeout               = 5 * time.Second
	cancelNotifyTimeout                  = time.Second
	cancelDrainTimeout                   = 100 * time.Millisecond

	operationStart               Operation = "start ACP agent"
	operationInitialize          Operation = "initialize ACP session"
	operationNewSession          Operation = "create ACP session"
	operationPrompt              Operation = "run ACP prompt"
	operationClose               Operation = "close ACP session"
	operationCancel              Operation = "cancel ACP prompt"
	operationProcessExit         Operation = "wait for ACP agent process"
	operationRequestPermission   Operation = "handle ACP permission request"
	operationReadTextFile        Operation = "handle ACP file read request"
	operationWriteTextFile       Operation = "handle ACP file write request"
	operationCreateTerminal      Operation = "handle ACP terminal create request"
	operationTerminalOutput      Operation = "handle ACP terminal output request"
	operationReleaseTerminal     Operation = "handle ACP terminal release request"
	operationWaitForTerminalExit Operation = "handle ACP terminal wait request"
	operationKillTerminal        Operation = "handle ACP terminal kill request"

	// EventKindAgentMessage indicates streamed assistant text from the ACP agent.
	EventKindAgentMessage EventKind = "agent_message"
	// EventKindAgentThought indicates streamed thought/reasoning text from the ACP agent.
	EventKindAgentThought EventKind = "agent_thought"
	// EventKindToolCall indicates a tool call started.
	EventKindToolCall EventKind = "tool_call"
	// EventKindToolCallUpdate indicates a tool call status/content update.
	EventKindToolCallUpdate EventKind = "tool_call_update"
	// EventKindPlan indicates an ACP plan update.
	EventKindPlan EventKind = "plan"
	// EventKindUnknown indicates an ACP update variant this foundation does not yet model.
	EventKindUnknown EventKind = "unknown"

	// PermissionOutcomeKindSelected means the policy selected one ACP permission option.
	PermissionOutcomeKindSelected PermissionOutcomeKind = "selected"
	// PermissionOutcomeKindCancelled means the policy cancelled the permission request.
	PermissionOutcomeKindCancelled PermissionOutcomeKind = "cancelled"
)

var (
	// ErrInvalidOptions is the sentinel wrapped when ACP foundation options are invalid.
	ErrInvalidOptions = errors.New("invalid ACP client options")
	// ErrAgentProcess is the sentinel wrapped when the ACP agent process fails.
	ErrAgentProcess = errors.New("ACP agent process error")
	// ErrACPProtocol is the sentinel wrapped when ACP negotiation or prompt calls fail.
	ErrACPProtocol = errors.New("ACP protocol error")
	// ErrPolicyDenied is the sentinel wrapped when the configured policy denies an ACP callback.
	ErrPolicyDenied = errors.New("ACP policy denied operation")
	// ErrUnsupportedOperation is the sentinel wrapped when an ACP callback is not supported.
	ErrUnsupportedOperation = errors.New("unsupported ACP operation")
)

//goplint:ignore -- ACP protocol adapter values mirror external protocol labels; DTOs are translated and validated at package boundaries.
type (
	clientName    string
	clientVersion string

	// CommandName is an executable path or name supplied by a future ACP caller.
	CommandName string
	// CommandArgument is one raw argv item supplied to an ACP agent process.
	CommandArgument string
	// PromptText is the user prompt text sent for a bounded ACP turn.
	PromptText string
	// SessionID is an ACP session identifier translated into an Invowk-owned type.
	SessionID string
	// StopReason is the ACP stop reason translated into an Invowk-owned type.
	StopReason string
	// EventKind categorizes an ACP streamed update.
	EventKind string
	// EventText contains display text extracted from ACP update content.
	EventText string
	// ToolCallID identifies an ACP tool call.
	ToolCallID string
	// ToolTitle is display text for an ACP tool call.
	ToolTitle string
	// ToolKind is the ACP tool category translated into an Invowk-owned type.
	ToolKind string
	// ToolStatus is the ACP tool status translated into an Invowk-owned type.
	ToolStatus string
	// PermissionOptionID identifies a permission option offered by an ACP agent.
	PermissionOptionID string
	// PermissionOptionKind categorizes an ACP permission option.
	PermissionOptionKind string
	// PermissionOptionName is display text for a permission option.
	PermissionOptionName string
	// PermissionOutcomeKind identifies the kind of permission response.
	PermissionOutcomeKind string
	// FileContent carries text content mediated by the filesystem policy.
	FileContent string
	// LineNumber is a one-based file line number requested by ACP.
	LineNumber int
	// LineLimit is the maximum number of file lines requested by ACP.
	LineLimit int
	// OptionalLineNumber preserves ACP field absence without exposing a mutable
	// pointer to a validated LineNumber.
	OptionalLineNumber struct {
		value   LineNumber //goplint:ignore -- immutable private value guarded by present.
		present bool
	}
	// OptionalLineLimit preserves ACP field absence without exposing a mutable
	// pointer to a validated LineLimit.
	OptionalLineLimit struct {
		value   LineLimit //goplint:ignore -- immutable private value guarded by present.
		present bool
	}
	// TerminalCommand is a command requested through an ACP terminal callback.
	TerminalCommand string
	// TerminalArgument is one raw argv item requested through an ACP terminal callback.
	TerminalArgument string
	// TerminalEnvName is a terminal environment variable name.
	TerminalEnvName string
	// TerminalEnvValue is a terminal environment variable value.
	TerminalEnvValue string
	// TerminalID identifies a terminal created through ACP.
	TerminalID string
	// TerminalOutput contains output returned by a terminal callback.
	TerminalOutput string
	// TerminalOutputLimit is an ACP terminal output byte limit.
	TerminalOutputLimit int
	// TerminalSignal is a signal name returned by a terminal callback.
	TerminalSignal string
	// TerminalOutputTruncated records whether terminal output was truncated.
	TerminalOutputTruncated bool
	// Operation names the ACP foundation lifecycle step that failed.
	Operation string
	// AgentStderr contains captured stderr from the ACP agent process.
	AgentStderr string

	// Command describes the ACP-compatible process to launch.
	Command struct {
		Name CommandName
		Args []CommandArgument
	}

	// Options configures one bounded ACP prompt run.
	Options struct {
		Command         Command
		WorkDir         types.FilesystemPath //goplint:ignore -- ACP session cwd is required by options validation.
		Policies        Policies
		ShutdownTimeout time.Duration
		Stderr          io.Writer //goplint:ignore -- optional adapter stream mirrors os/exec stderr plumbing.
	}

	// Prompt is the bounded user turn sent to the ACP agent.
	Prompt struct {
		Text PromptText
	}

	// Result is the normalized result of one ACP prompt turn.
	Result struct {
		SessionID  SessionID
		StopReason StopReason
		Events     []Event
	}

	// Event is a normalized ACP streamed update.
	Event struct {
		Kind       EventKind
		Text       EventText
		ToolCallID ToolCallID
		ToolTitle  ToolTitle
		ToolKind   ToolKind
		ToolStatus ToolStatus
	}

	// ToolCall is the normalized tool-call detail for permission requests.
	ToolCall struct {
		ID        ToolCallID
		Title     ToolTitle
		Kind      ToolKind
		Status    ToolStatus
		Locations []types.FilesystemPath
	}

	// PermissionOption is one policy choice offered by an ACP agent.
	PermissionOption struct {
		ID   PermissionOptionID
		Kind PermissionOptionKind
		Name PermissionOptionName
	}

	// PermissionOutcome is the selected or cancelled result returned by policy.
	PermissionOutcome struct {
		Kind             PermissionOutcomeKind
		SelectedOptionID PermissionOptionID
	}

	// PermissionRequest is the Invowk-owned permission callback request.
	PermissionRequest struct {
		SessionID SessionID
		ToolCall  ToolCall
		Options   []PermissionOption
	}

	// PermissionResponse is the Invowk-owned permission callback response.
	PermissionResponse struct {
		Outcome PermissionOutcome
	}

	// ReadTextFileRequest is a mediated ACP filesystem read request.
	ReadTextFileRequest struct {
		SessionID SessionID
		Path      types.FilesystemPath //goplint:ignore -- ACP read path is required by protocol validation.
		Line      OptionalLineNumber
		Limit     OptionalLineLimit
	}

	// ReadTextFileResponse is a mediated ACP filesystem read response.
	ReadTextFileResponse struct {
		Content FileContent
	}

	// WriteTextFileRequest is a mediated ACP filesystem write request.
	WriteTextFileRequest struct {
		SessionID SessionID
		Path      types.FilesystemPath //goplint:ignore -- ACP write path is required by protocol validation.
		Content   FileContent
	}

	// WriteTextFileResponse is a mediated ACP filesystem write response.
	WriteTextFileResponse struct{}

	// TerminalEnv is one environment variable requested for an ACP terminal.
	TerminalEnv struct {
		Name  TerminalEnvName
		Value TerminalEnvValue
	}

	// CreateTerminalRequest is a mediated ACP terminal creation request.
	CreateTerminalRequest struct {
		SessionID       SessionID
		Command         TerminalCommand
		Args            []TerminalArgument
		WorkDir         *types.FilesystemPath
		Env             []TerminalEnv
		OutputByteLimit *TerminalOutputLimit
	}

	// CreateTerminalResponse is a mediated ACP terminal creation response.
	CreateTerminalResponse struct {
		TerminalID TerminalID
	}

	// TerminalOutputRequest is a mediated ACP terminal output request.
	TerminalOutputRequest struct {
		SessionID  SessionID
		TerminalID TerminalID
	}

	// TerminalOutputResponse is a mediated ACP terminal output response.
	TerminalOutputResponse struct {
		Output    TerminalOutput
		Truncated TerminalOutputTruncated
	}

	// ReleaseTerminalRequest is a mediated ACP terminal release request.
	ReleaseTerminalRequest struct {
		SessionID  SessionID
		TerminalID TerminalID
	}

	// ReleaseTerminalResponse is a mediated ACP terminal release response.
	ReleaseTerminalResponse struct{}

	// WaitForTerminalExitRequest is a mediated ACP terminal wait request.
	WaitForTerminalExitRequest struct {
		SessionID  SessionID
		TerminalID TerminalID
	}

	// WaitForTerminalExitResponse is a mediated ACP terminal wait response.
	WaitForTerminalExitResponse struct {
		ExitCode *types.ExitCode
		Signal   TerminalSignal
	}

	// KillTerminalRequest is a mediated ACP terminal kill request.
	KillTerminalRequest struct {
		SessionID  SessionID
		TerminalID TerminalID
	}

	// KillTerminalResponse is a mediated ACP terminal kill response.
	KillTerminalResponse struct{}

	// OperationError wraps an ACP lifecycle, protocol, or process failure.
	OperationError struct {
		Operation Operation
		Kind      error
		Err       error
		Stderr    AgentStderr
	}

	// InvalidOptionsError is returned when ACP options or prompt fields are invalid.
	InvalidOptionsError struct {
		FieldErrors []error
	}
)

// SelectedPermissionOutcome returns a permission outcome selecting optionID.
func SelectedPermissionOutcome(optionID PermissionOptionID) PermissionOutcome {
	return PermissionOutcome{
		Kind:             PermissionOutcomeKindSelected,
		SelectedOptionID: optionID,
	}
}

// CancelledPermissionOutcome returns a cancelled permission outcome.
func CancelledPermissionOutcome() PermissionOutcome {
	return PermissionOutcome{Kind: PermissionOutcomeKindCancelled}
}

// String returns the command name as text.
func (n CommandName) String() string { return string(n) }

// Validate returns an error if the command name is empty.
func (n CommandName) Validate() error {
	if strings.TrimSpace(string(n)) == "" {
		return fmt.Errorf("%w: command name must be non-empty", ErrInvalidOptions)
	}
	return nil
}

// String returns the command argument as text.
func (a CommandArgument) String() string { return string(a) }

// String returns the prompt text.
func (p PromptText) String() string { return string(p) }

// Validate returns an error if the prompt text is empty.
func (p PromptText) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return fmt.Errorf("%w: prompt text must be non-empty", ErrInvalidOptions)
	}
	return nil
}

// String returns the session ID.
func (id SessionID) String() string { return string(id) }

// String returns the stop reason.
func (r StopReason) String() string { return string(r) }

// String returns the event kind.
func (k EventKind) String() string { return string(k) }

// String returns the event text.
func (t EventText) String() string { return string(t) }

// String returns the tool-call ID.
func (id ToolCallID) String() string { return string(id) }

// String returns the tool title.
func (t ToolTitle) String() string { return string(t) }

// String returns the tool kind.
func (k ToolKind) String() string { return string(k) }

// String returns the tool status.
func (s ToolStatus) String() string { return string(s) }

// String returns the permission option ID.
func (id PermissionOptionID) String() string { return string(id) }

// Validate returns an error if the permission option ID is empty.
func (id PermissionOptionID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("%w: permission option id must be non-empty", ErrInvalidOptions)
	}
	return nil
}

// String returns the permission option kind.
func (k PermissionOptionKind) String() string { return string(k) }

// String returns the permission option name.
func (n PermissionOptionName) String() string { return string(n) }

// String returns the permission outcome kind.
func (k PermissionOutcomeKind) String() string { return string(k) }

// Validate returns an error if the permission outcome is invalid.
func (o PermissionOutcome) Validate() error {
	switch o.Kind {
	case PermissionOutcomeKindSelected:
		return o.SelectedOptionID.Validate()
	case PermissionOutcomeKindCancelled:
		return nil
	default:
		return fmt.Errorf("%w: unknown permission outcome %q", ErrInvalidOptions, o.Kind)
	}
}

// String returns the file content.
func (c FileContent) String() string { return string(c) }

// Validate returns an error if the line number is invalid.
func (n LineNumber) Validate() error {
	if n < 1 {
		return fmt.Errorf("%w: line number must be positive", ErrInvalidOptions)
	}
	return nil
}

// Validate returns an error if the line limit is invalid.
func (l LineLimit) Validate() error {
	if l < 1 {
		return fmt.Errorf("%w: line limit must be positive", ErrInvalidOptions)
	}
	return nil
}

// Value returns the optional line number and whether the ACP request supplied it.
func (o OptionalLineNumber) Value() (LineNumber, bool) {
	return o.value, o.present
}

// Value returns the optional line limit and whether the ACP request supplied it.
func (o OptionalLineLimit) Value() (LineLimit, bool) {
	return o.value, o.present
}

// String returns the terminal command.
func (c TerminalCommand) String() string { return string(c) }

// String returns the terminal argument.
func (a TerminalArgument) String() string { return string(a) }

// String returns the terminal environment variable name.
func (n TerminalEnvName) String() string { return string(n) }

// String returns the terminal environment variable value.
func (v TerminalEnvValue) String() string { return string(v) }

// String returns the terminal ID.
func (id TerminalID) String() string { return string(id) }

// String returns the terminal output.
func (o TerminalOutput) String() string { return string(o) }

// String returns the terminal signal.
func (s TerminalSignal) String() string { return string(s) }

// String returns the operation name.
func (o Operation) String() string { return string(o) }

// String returns captured agent stderr.
func (s AgentStderr) String() string { return string(s) }

// Validate returns an error if the command is invalid.
func (c Command) Validate() error {
	return c.Name.Validate()
}

// ArgsAsStrings returns raw argv strings for os/exec.
//
//goplint:ignore -- os/exec requires raw argv strings at the process adapter boundary.
func (c Command) ArgsAsStrings() []string {
	args := make([]string, 0, len(c.Args))
	for _, arg := range c.Args {
		args = append(args, arg.String())
	}
	return args
}

// Validate returns an error if options are invalid.
func (o Options) Validate() error {
	var fieldErrors []error
	if err := o.Command.Validate(); err != nil {
		fieldErrors = append(fieldErrors, fmt.Errorf("command: %w", err))
	}
	if err := o.WorkDir.Validate(); err != nil {
		fieldErrors = append(fieldErrors, fmt.Errorf("workdir: %w", err))
	}
	if len(fieldErrors) > 0 {
		return &InvalidOptionsError{FieldErrors: fieldErrors}
	}
	return nil
}

// Validate returns an error if the prompt is invalid.
func (p Prompt) Validate() error {
	if err := p.Text.Validate(); err != nil {
		return &InvalidOptionsError{FieldErrors: []error{fmt.Errorf("prompt: %w", err)}}
	}
	return nil
}

// ShutdownDuration returns the configured process shutdown timeout.
func (o Options) ShutdownDuration() time.Duration {
	if o.ShutdownTimeout <= 0 {
		return defaultShutdownTimeout
	}
	return o.ShutdownTimeout
}

// Error implements error.
func (e *OperationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := fmt.Sprintf("%s: %v", e.Operation, e.Err)
	if e.Stderr != "" {
		msg += fmt.Sprintf(" (agent stderr: %s)", e.Stderr)
	}
	return msg
}

// Unwrap returns the sentinel and underlying cause for errors.Is compatibility.
func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return errors.Join(e.Kind, e.Err)
}

// Error implements error.
func (e *InvalidOptionsError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return types.FormatFieldErrors("ACP client options", e.FieldErrors)
}

// Unwrap returns ErrInvalidOptions plus field errors for errors.Is compatibility.
func (e *InvalidOptionsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return errors.Join(append([]error{ErrInvalidOptions}, e.FieldErrors...)...)
}
