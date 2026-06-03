// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/invowk/invowk/pkg/types"
)

const (
	fakeAgentEnv           = "INVOWK_ACPCLIENT_FAKE_AGENT"
	fakeAgentScenarioEnv   = "INVOWK_ACPCLIENT_FAKE_SCENARIO"
	fakeAgentStartFileEnv  = "INVOWK_ACPCLIENT_FAKE_START_FILE"
	fakeAgentCancelFileEnv = "INVOWK_ACPCLIENT_FAKE_CANCEL_FILE"

	fakeAgentScenarioSuccess = "success"
	fakeAgentScenarioCancel  = "cancel"
)

var _ acp.Agent = (*fakeAgent)(nil)

type fakeAgent struct {
	conn         *acp.AgentSideConnection
	ready        chan struct{}
	scenario     string
	startMarker  types.FilesystemPath
	cancelMarker types.FilesystemPath
}

func TestMain(m *testing.M) {
	if os.Getenv(fakeAgentEnv) == "1" {
		os.Exit(runFakeAgentProcess())
	}
	os.Exit(m.Run())
}

func TestRunPromptWithFakeAgent(t *testing.T) {
	t.Setenv(fakeAgentEnv, "1")
	t.Setenv(fakeAgentScenarioEnv, fakeAgentScenarioSuccess)

	result, err := RunPrompt(t.Context(), fakeOptions(t), Prompt{Text: PromptText("hello")})
	if err != nil {
		t.Fatalf("RunPrompt() error = %v", err)
	}
	if result.SessionID != "fake-session" {
		t.Fatalf("SessionID = %q, want fake-session", result.SessionID)
	}
	if result.StopReason != StopReason(acp.StopReasonEndTurn) {
		t.Fatalf("StopReason = %q, want %q", result.StopReason, acp.StopReasonEndTurn)
	}
	if len(result.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(result.Events))
	}
	event := result.Events[0]
	if event.Kind != EventKindAgentMessage {
		t.Fatalf("event.Kind = %q, want %q", event.Kind, EventKindAgentMessage)
	}
	if event.Text != "hello from fake agent" {
		t.Fatalf("event.Text = %q, want fake agent text", event.Text)
	}
}

func TestRunPromptCancellationForwardsCancel(t *testing.T) {
	startFile := types.FilesystemPath(t.TempDir() + "/started")
	cancelFile := types.FilesystemPath(t.TempDir() + "/cancelled")
	t.Setenv(fakeAgentEnv, "1")
	t.Setenv(fakeAgentScenarioEnv, fakeAgentScenarioCancel)
	t.Setenv(fakeAgentStartFileEnv, startFile.String())
	t.Setenv(fakeAgentCancelFileEnv, cancelFile.String())

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		_, err := RunPrompt(ctx, fakeOptions(t), Prompt{Text: PromptText("cancel me")})
		errCh <- err
	}()

	waitForFile(t, startFile)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunPrompt() error = %v, want context.Canceled", err)
		}
		if !errors.Is(err, ErrACPProtocol) {
			t.Fatalf("RunPrompt() error = %v, want ErrACPProtocol", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunPrompt() did not return after cancellation")
	}
	waitForFile(t, cancelFile)
}

func fakeOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		Command: Command{
			Name: CommandName(os.Args[0]),
			Args: []CommandArgument{"-test.run=TestFakeACPAgentProcess"},
		},
		WorkDir: types.FilesystemPath(t.TempDir()),
	}
}

func runFakeAgentProcess() int {
	agent := &fakeAgent{
		ready:        make(chan struct{}),
		scenario:     os.Getenv(fakeAgentScenarioEnv),
		startMarker:  types.FilesystemPath(os.Getenv(fakeAgentStartFileEnv)),
		cancelMarker: types.FilesystemPath(os.Getenv(fakeAgentCancelFileEnv)),
	}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	close(agent.ready)
	<-conn.Done()
	return 0
}

func (a *fakeAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *fakeAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo: &acp.Implementation{
			Name:    "fake-agent",
			Version: "test",
		},
		AgentCapabilities: acp.AgentCapabilities{
			SessionCapabilities: acp.SessionCapabilities{
				Close: &acp.SessionCloseCapabilities{},
			},
		},
	}, nil
}

func (a *fakeAgent) Cancel(context.Context, acp.CancelNotification) error {
	if a.cancelMarker != "" {
		return os.WriteFile(a.cancelMarker.String(), []byte("cancelled"), 0o600)
	}
	return nil
}

func (a *fakeAgent) CloseSession(context.Context, acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (a *fakeAgent) ListSessions(context.Context, acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionList)
}

func (a *fakeAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "fake-session"}, nil
}

func (a *fakeAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	switch a.scenario {
	case fakeAgentScenarioCancel:
		if a.startMarker != "" {
			if err := os.WriteFile(a.startMarker.String(), []byte("started"), 0o600); err != nil {
				return acp.PromptResponse{}, err
			}
		}
		<-ctx.Done()
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, fmt.Errorf("fake prompt canceled: %w", ctx.Err())
	default:
		conn, err := a.connection(ctx)
		if err != nil {
			return acp.PromptResponse{}, err
		}
		if err := conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    acp.UpdateAgentMessageText("hello from fake agent"),
		}); err != nil {
			return acp.PromptResponse{}, fmt.Errorf("send fake session update: %w", err)
		}
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}
}

func (a *fakeAgent) ResumeSession(context.Context, acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionResume)
}

func (a *fakeAgent) SetSessionConfigOption(
	context.Context,
	acp.SetSessionConfigOptionRequest,
) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}

func (a *fakeAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode)
}

func (a *fakeAgent) connection(ctx context.Context) (*acp.AgentSideConnection, error) {
	select {
	case <-a.ready:
		return a.conn, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for fake agent connection: %w", ctx.Err())
	}
}

func waitForFile(t *testing.T, path types.FilesystemPath) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path.String()); err == nil {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", path)
		case <-ticker.C:
		}
	}
}
