// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/invowk/invowk/pkg/types"
)

var _ acp.Client = (*callbackClient)(nil)

type (
	callbackClient struct {
		policies Policies

		mu     sync.Mutex
		events []Event
	}

	agentProcess struct {
		cancel          context.CancelFunc
		stdin           io.WriteCloser
		wait            <-chan error
		shutdownTimeout time.Duration
	}

	stderrCapture struct {
		mu     sync.Mutex
		buffer bytes.Buffer
	}
)

// RunPrompt launches the configured ACP process, drives one bounded prompt turn,
// collects streamed updates, and closes the process.
func RunPrompt(ctx context.Context, opts Options, prompt Prompt) (result Result, err error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, newOperationError(operationCancel, ErrACPProtocol, ctxErr, "")
	}
	if validateErr := opts.Validate(); validateErr != nil {
		return Result{}, validateErr
	}
	if validateErr := prompt.Validate(); validateErr != nil {
		return Result{}, validateErr
	}

	process, stdout, stderr, err := startAgent(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if closeErr := process.close(); err == nil && closeErr != nil {
			err = newOperationError(operationProcessExit, ErrAgentProcess, closeErr, stderr.String())
		}
	}()

	client := &callbackClient{policies: opts.Policies.Normalize()}
	conn := acp.NewClientSideConnection(client, process.stdin, stdout)

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: opts.Policies.Terminal != nil,
		},
		ClientInfo: &acp.Implementation{
			Name:    defaultClientName.String(),
			Version: defaultClientVersion.String(),
		},
	})
	if err != nil {
		return Result{}, newOperationError(operationInitialize, ErrACPProtocol, err, stderr.String())
	}

	session, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        opts.WorkDir.String(),
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return Result{}, newOperationError(operationNewSession, ErrACPProtocol, err, stderr.String())
	}

	promptResp, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: session.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt.Text.String())},
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			cancelErr := sendCancel(conn, session.SessionId)
			return Result{}, newOperationError(operationCancel, ErrACPProtocol, errors.Join(ctxErr, cancelErr), stderr.String())
		}
		return Result{}, newOperationError(operationPrompt, ErrACPProtocol, err, stderr.String())
	}

	if initResp.AgentCapabilities.SessionCapabilities.Close != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opts.ShutdownDuration())
		defer cancel()
		if _, err := conn.CloseSession(closeCtx, acp.CloseSessionRequest{SessionId: session.SessionId}); err != nil {
			return Result{}, newOperationError(operationClose, ErrACPProtocol, err, stderr.String())
		}
	}

	return Result{
		SessionID:  SessionID(session.SessionId),
		StopReason: StopReason(promptResp.StopReason),
		Events:     client.Events(),
	}, nil
}

func (n clientName) String() string { return string(n) }

func (v clientVersion) String() string { return string(v) }

func startAgent(ctx context.Context, opts Options) (*agentProcess, io.Reader, *stderrCapture, error) {
	processCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(processCtx, opts.Command.Name.String(), opts.Command.ArgsAsStrings()...)
	cmd.Dir = opts.WorkDir.String()
	cmd.WaitDelay = opts.ShutdownDuration()
	stderr := &stderrCapture{}
	if opts.Stderr != nil {
		cmd.Stderr = io.MultiWriter(stderr, opts.Stderr)
	} else {
		cmd.Stderr = stderr
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancelProcess()
		return nil, nil, stderr, newOperationError(operationStart, ErrAgentProcess, err, stderr.String())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancelProcess()
		return nil, nil, stderr, newOperationError(operationStart, ErrAgentProcess, err, stderr.String())
	}
	if err := cmd.Start(); err != nil {
		cancelProcess()
		return nil, nil, stderr, newOperationError(operationStart, ErrAgentProcess, err, stderr.String())
	}

	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
	}()

	return &agentProcess{
		cancel:          cancelProcess,
		stdin:           stdin,
		wait:            wait,
		shutdownTimeout: opts.ShutdownDuration(),
	}, stdout, stderr, nil
}

func (s *stderrCapture) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buffer.Write(p)
}

func (s *stderrCapture) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buffer.String()
}

func sendCancel(conn *acp.ClientSideConnection, sessionID acp.SessionId) error {
	cancelCtx, cancel := context.WithTimeout(context.Background(), cancelNotifyTimeout)
	defer cancel()
	err := conn.Cancel(cancelCtx, acp.CancelNotification{SessionId: sessionID})
	timer := time.NewTimer(cancelDrainTimeout)
	defer timer.Stop()
	select {
	case <-conn.Done():
	case <-timer.C:
	}
	if err != nil {
		return fmt.Errorf("send ACP cancel notification: %w", err)
	}
	return nil
}

func (p *agentProcess) close() error {
	if p == nil {
		return nil
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	select {
	case err := <-p.wait:
		p.cancel()
		return err
	case <-time.After(p.shutdownTimeout):
		p.cancel()
		err := <-p.wait
		if err != nil {
			return fmt.Errorf("ACP agent did not exit before shutdown timeout: %w", err)
		}
		return errors.New("ACP agent did not exit before shutdown timeout")
	}
}

// Events returns a snapshot of streamed ACP events.
func (c *callbackClient) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	events := make([]Event, len(c.events))
	copy(events, c.events)
	return events
}

func (c *callbackClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = append(c.events, eventFromSessionUpdate(params.Update))
	return nil
}

func (c *callbackClient) RequestPermission(
	ctx context.Context,
	params acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.RequestPermissionResponse{}, newOperationError(operationRequestPermission, ErrACPProtocol, err, "")
	}
	toolCall, err := toolCallFromACP(params.ToolCall)
	if err != nil {
		return acp.RequestPermissionResponse{}, err
	}
	req := PermissionRequest{
		SessionID: SessionID(params.SessionId),
		ToolCall:  toolCall,
		Options:   permissionOptionsFromACP(params.Options),
	}
	resp, err := c.policies.Permission.RequestPermission(ctx, req)
	if err != nil {
		return acp.RequestPermissionResponse{}, err
	}
	if err := resp.Outcome.Validate(); err != nil {
		return acp.RequestPermissionResponse{}, err
	}
	return permissionResponseToACP(resp), nil
}

func (c *callbackClient) ReadTextFile(
	ctx context.Context,
	params acp.ReadTextFileRequest,
) (acp.ReadTextFileResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.ReadTextFileResponse{}, newOperationError(operationReadTextFile, ErrACPProtocol, err, "")
	}
	path, err := filesystemPathFromACP(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	line, err := lineNumberFromPtr(params.Line)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	limit, err := lineLimitFromPtr(params.Limit)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	req := ReadTextFileRequest{
		SessionID: SessionID(params.SessionId),
		Path:      path,
		Line:      line,
		Limit:     limit,
	}
	resp, err := c.policies.Filesystem.ReadTextFile(ctx, req)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	return acp.ReadTextFileResponse{Content: resp.Content.String()}, nil
}

func (c *callbackClient) WriteTextFile(
	ctx context.Context,
	params acp.WriteTextFileRequest,
) (acp.WriteTextFileResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.WriteTextFileResponse{}, newOperationError(operationWriteTextFile, ErrACPProtocol, err, "")
	}
	path, err := filesystemPathFromACP(params.Path)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	req := WriteTextFileRequest{
		SessionID: SessionID(params.SessionId),
		Path:      path,
		Content:   FileContent(params.Content),
	}
	if _, err := c.policies.Filesystem.WriteTextFile(ctx, req); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return acp.WriteTextFileResponse{}, nil
}

func (c *callbackClient) CreateTerminal(
	ctx context.Context,
	params acp.CreateTerminalRequest,
) (acp.CreateTerminalResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.CreateTerminalResponse{}, newOperationError(operationCreateTerminal, ErrACPProtocol, err, "")
	}
	policy := c.terminalPolicy()
	req, err := createTerminalRequestFromACP(params)
	if err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	resp, err := policy.CreateTerminal(ctx, req)
	if err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	return acp.CreateTerminalResponse{TerminalId: resp.TerminalID.String()}, nil
}

func (c *callbackClient) TerminalOutput(
	ctx context.Context,
	params acp.TerminalOutputRequest,
) (acp.TerminalOutputResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.TerminalOutputResponse{}, newOperationError(operationTerminalOutput, ErrACPProtocol, err, "")
	}
	policy := c.terminalPolicy()
	resp, err := policy.TerminalOutput(ctx, TerminalOutputRequest{
		SessionID:  SessionID(params.SessionId),
		TerminalID: TerminalID(params.TerminalId),
	})
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	return acp.TerminalOutputResponse{
		Output:    resp.Output.String(),
		Truncated: bool(resp.Truncated),
	}, nil
}

func (c *callbackClient) ReleaseTerminal(
	ctx context.Context,
	params acp.ReleaseTerminalRequest,
) (acp.ReleaseTerminalResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.ReleaseTerminalResponse{}, newOperationError(operationReleaseTerminal, ErrACPProtocol, err, "")
	}
	policy := c.terminalPolicy()
	_, err := policy.ReleaseTerminal(ctx, ReleaseTerminalRequest{
		SessionID:  SessionID(params.SessionId),
		TerminalID: TerminalID(params.TerminalId),
	})
	if err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *callbackClient) WaitForTerminalExit(
	ctx context.Context,
	params acp.WaitForTerminalExitRequest,
) (acp.WaitForTerminalExitResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.WaitForTerminalExitResponse{}, newOperationError(operationWaitForTerminalExit, ErrACPProtocol, err, "")
	}
	policy := c.terminalPolicy()
	resp, err := policy.WaitForTerminalExit(ctx, WaitForTerminalExitRequest{
		SessionID:  SessionID(params.SessionId),
		TerminalID: TerminalID(params.TerminalId),
	})
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	return acp.WaitForTerminalExitResponse{
		ExitCode: exitCodeToPtr(resp.ExitCode),
		Signal:   signalToPtr(resp.Signal),
	}, nil
}

func (c *callbackClient) KillTerminal(
	ctx context.Context,
	params acp.KillTerminalRequest,
) (acp.KillTerminalResponse, error) {
	if err := params.Validate(); err != nil {
		return acp.KillTerminalResponse{}, newOperationError(operationKillTerminal, ErrACPProtocol, err, "")
	}
	policy := c.terminalPolicy()
	_, err := policy.KillTerminal(ctx, KillTerminalRequest{
		SessionID:  SessionID(params.SessionId),
		TerminalID: TerminalID(params.TerminalId),
	})
	if err != nil {
		return acp.KillTerminalResponse{}, err
	}
	return acp.KillTerminalResponse{}, nil
}

func (c *callbackClient) terminalPolicy() TerminalPolicy {
	if c.policies.Terminal != nil {
		return c.policies.Terminal
	}
	return RestrictivePolicy{}
}

func eventFromSessionUpdate(update acp.SessionUpdate) Event {
	switch {
	case update.AgentMessageChunk != nil:
		return Event{
			Kind: EventKindAgentMessage,
			Text: textFromContentBlock(update.AgentMessageChunk.Content),
		}
	case update.AgentThoughtChunk != nil:
		return Event{
			Kind: EventKindAgentThought,
			Text: textFromContentBlock(update.AgentThoughtChunk.Content),
		}
	case update.ToolCall != nil:
		return Event{
			Kind:       EventKindToolCall,
			ToolCallID: ToolCallID(update.ToolCall.ToolCallId),
			ToolTitle:  ToolTitle(update.ToolCall.Title),
			ToolKind:   ToolKind(update.ToolCall.Kind),
			ToolStatus: ToolStatus(update.ToolCall.Status),
		}
	case update.ToolCallUpdate != nil:
		return Event{
			Kind:       EventKindToolCallUpdate,
			Text:       textFromToolContent(update.ToolCallUpdate.Content),
			ToolCallID: ToolCallID(update.ToolCallUpdate.ToolCallId),
			ToolTitle:  toolTitleFromPtr(update.ToolCallUpdate.Title),
			ToolKind:   toolKindFromPtr(update.ToolCallUpdate.Kind),
			ToolStatus: toolStatusFromPtr(update.ToolCallUpdate.Status),
		}
	case update.Plan != nil:
		return Event{Kind: EventKindPlan}
	default:
		return Event{Kind: EventKindUnknown}
	}
}

func textFromContentBlock(block acp.ContentBlock) EventText {
	if block.Text == nil {
		return ""
	}
	return EventText(block.Text.Text)
}

func textFromToolContent(content []acp.ToolCallContent) EventText {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if item.Content != nil && item.Content.Content.Text != nil {
			parts = append(parts, item.Content.Content.Text.Text)
		}
	}
	return EventText(strings.Join(parts, "\n"))
}

//goplint:ignore -- ACP generated protocol uses primitive string pointers for optional labels.
func toolTitleFromPtr(title *string) ToolTitle {
	if title == nil {
		return ""
	}
	return ToolTitle(*title)
}

func toolKindFromPtr(kind *acp.ToolKind) ToolKind {
	if kind == nil {
		return ""
	}
	return ToolKind(*kind)
}

func toolStatusFromPtr(status *acp.ToolCallStatus) ToolStatus {
	if status == nil {
		return ""
	}
	return ToolStatus(*status)
}

func toolCallFromACP(tool acp.ToolCallUpdate) (ToolCall, error) {
	locations := make([]types.FilesystemPath, 0, len(tool.Locations))
	for _, location := range tool.Locations {
		path, err := filesystemPathFromACP(location.Path)
		if err != nil {
			return ToolCall{}, err
		}
		locations = append(locations, path)
	}
	return ToolCall{
		ID:        ToolCallID(tool.ToolCallId),
		Title:     toolTitleFromPtr(tool.Title),
		Kind:      toolKindFromPtr(tool.Kind),
		Status:    toolStatusFromPtr(tool.Status),
		Locations: locations,
	}, nil
}

func permissionOptionsFromACP(options []acp.PermissionOption) []PermissionOption {
	result := make([]PermissionOption, 0, len(options))
	for _, option := range options {
		result = append(result, PermissionOption{
			ID:   PermissionOptionID(option.OptionId),
			Kind: PermissionOptionKind(option.Kind),
			Name: PermissionOptionName(option.Name),
		})
	}
	return result
}

func permissionResponseToACP(resp PermissionResponse) acp.RequestPermissionResponse {
	outcome := acp.RequestPermissionOutcome{}
	switch resp.Outcome.Kind {
	case PermissionOutcomeKindSelected:
		outcome.Selected = &acp.RequestPermissionOutcomeSelected{
			OptionId: acp.PermissionOptionId(resp.Outcome.SelectedOptionID),
		}
	case PermissionOutcomeKindCancelled:
		outcome.Cancelled = &acp.RequestPermissionOutcomeCancelled{}
	}
	return acp.RequestPermissionResponse{Outcome: outcome}
}

//goplint:ignore -- ACP generated protocol uses primitive int pointers for optional line numbers.
//nolint:nilnil // A nil line with nil error is the ACP "field absent" value.
func lineNumberFromPtr(line *int) (*LineNumber, error) {
	if line == nil {
		return nil, nil
	}
	value := LineNumber(*line)
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return &value, nil
}

//goplint:ignore -- ACP generated protocol uses primitive int pointers for optional line limits.
//nolint:nilnil // A nil limit with nil error is the ACP "field absent" value.
func lineLimitFromPtr(limit *int) (*LineLimit, error) {
	if limit == nil {
		return nil, nil
	}
	value := LineLimit(*limit)
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return &value, nil
}

func createTerminalRequestFromACP(params acp.CreateTerminalRequest) (CreateTerminalRequest, error) {
	args := make([]TerminalArgument, 0, len(params.Args))
	for _, arg := range params.Args {
		args = append(args, TerminalArgument(arg))
	}
	env := make([]TerminalEnv, 0, len(params.Env))
	for _, item := range params.Env {
		env = append(env, TerminalEnv{
			Name:  TerminalEnvName(item.Name),
			Value: TerminalEnvValue(item.Value),
		})
	}
	workDir, err := workDirFromPtr(params.Cwd)
	if err != nil {
		return CreateTerminalRequest{}, err
	}
	return CreateTerminalRequest{
		SessionID:       SessionID(params.SessionId),
		Command:         TerminalCommand(params.Command),
		Args:            args,
		WorkDir:         workDir,
		Env:             env,
		OutputByteLimit: terminalOutputLimitFromPtr(params.OutputByteLimit),
	}, nil
}

//goplint:ignore -- ACP generated protocol uses primitive string pointers for optional cwd.
//nolint:nilnil // A nil cwd with nil error is the ACP "field absent" value.
func workDirFromPtr(path *string) (*types.FilesystemPath, error) {
	if path == nil {
		return nil, nil
	}
	value, err := filesystemPathFromACP(*path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

//goplint:ignore -- ACP generated protocol uses primitive int pointers for optional output limits.
func terminalOutputLimitFromPtr(limit *int) *TerminalOutputLimit {
	if limit == nil {
		return nil
	}
	value := TerminalOutputLimit(*limit)
	return &value
}

//goplint:ignore -- ACP generated protocol uses primitive int pointers for optional exit codes.
func exitCodeToPtr(exitCode *types.ExitCode) *int {
	if exitCode == nil {
		return nil
	}
	value := int(*exitCode)
	return &value
}

//goplint:ignore -- ACP generated protocol uses primitive string pointers for optional signals.
func signalToPtr(signal TerminalSignal) *string {
	if signal == "" {
		return nil
	}
	value := signal.String()
	return &value
}

func filesystemPathFromACP(path string) (types.FilesystemPath, error) {
	value := types.FilesystemPath(path) //goplint:ignore -- ACP protocol path is validated immediately below.
	if err := value.Validate(); err != nil {
		return "", err
	}
	return value, nil
}

//goplint:ignore -- stderr is captured from os/exec as raw process text at the adapter boundary.
func newOperationError(operation Operation, kind, err error, stderr string) error {
	return &OperationError{
		Operation: operation,
		Kind:      kind,
		Err:       err,
		Stderr:    AgentStderr(strings.TrimSpace(stderr)),
	}
}
