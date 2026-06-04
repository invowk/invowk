// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"context"
	"errors"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

var (
	_ PermissionRequester = (*recordingPermissionRequester)(nil)
	_ FilesystemPolicy    = (*recordingFilesystemPolicy)(nil)
)

type (
	recordingPermissionRequester struct {
		request PermissionRequest
	}

	recordingFilesystemPolicy struct {
		readRequest  ReadTextFileRequest
		writeRequest WriteTextFileRequest
	}
)

func TestCallbackClientDelegatesPermissionRequester(t *testing.T) {
	t.Parallel()

	policy := &recordingPermissionRequester{}
	client := &callbackClient{policies: Policies{Permission: policy}.Normalize()}
	title := "Edit config"
	kind := acp.ToolKindEdit
	status := acp.ToolCallStatusPending

	resp, err := client.RequestPermission(t.Context(), acp.RequestPermissionRequest{
		SessionId: "session-1",
		ToolCall: acp.ToolCallUpdate{
			ToolCallId: "tool-1",
			Title:      &title,
			Kind:       &kind,
			Status:     &status,
			Locations:  []acp.ToolCallLocation{{Path: "/tmp/config.cue"}},
		},
		Options: []acp.PermissionOption{
			{OptionId: "allow", Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow"},
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce, Name: "Reject"},
		},
	})
	if err != nil {
		t.Fatalf("RequestPermission() error = %v", err)
	}
	if policy.request.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", policy.request.SessionID)
	}
	if policy.request.ToolCall.ID != "tool-1" {
		t.Fatalf("ToolCall.ID = %q, want tool-1", policy.request.ToolCall.ID)
	}
	if got := policy.request.ToolCall.Locations[0]; got != "/tmp/config.cue" {
		t.Fatalf("ToolCall.Locations[0] = %q, want /tmp/config.cue", got)
	}
	if resp.Outcome.Selected == nil {
		t.Fatal("response outcome selected = nil, want selected")
	}
	if resp.Outcome.Selected.OptionId != "allow" {
		t.Fatalf("selected option = %q, want allow", resp.Outcome.Selected.OptionId)
	}
}

func TestCallbackClientDelegatesFilesystemPolicy(t *testing.T) {
	t.Parallel()

	policy := &recordingFilesystemPolicy{}
	client := &callbackClient{policies: Policies{Filesystem: policy}.Normalize()}
	line := 2
	limit := 3

	readResp, err := client.ReadTextFile(t.Context(), acp.ReadTextFileRequest{
		SessionId: "session-1",
		Path:      "/tmp/readme.md",
		Line:      &line,
		Limit:     &limit,
	})
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if readResp.Content != "file content from policy" {
		t.Fatalf("read content = %q, want policy content", readResp.Content)
	}
	if policy.readRequest.Path != "/tmp/readme.md" {
		t.Fatalf("read path = %q, want /tmp/readme.md", policy.readRequest.Path)
	}
	if policy.readRequest.Line == nil || *policy.readRequest.Line != 2 {
		t.Fatalf("read line = %v, want 2", policy.readRequest.Line)
	}
	if policy.readRequest.Limit == nil || *policy.readRequest.Limit != 3 {
		t.Fatalf("read limit = %v, want 3", policy.readRequest.Limit)
	}

	_, err = client.WriteTextFile(t.Context(), acp.WriteTextFileRequest{
		SessionId: "session-1",
		Path:      "/tmp/readme.md",
		Content:   "updated by policy",
	})
	if err != nil {
		t.Fatalf("WriteTextFile() error = %v", err)
	}
	if policy.writeRequest.Path != "/tmp/readme.md" {
		t.Fatalf("write path = %q, want /tmp/readme.md", policy.writeRequest.Path)
	}
	if policy.writeRequest.Content != "updated by policy" {
		t.Fatalf("write content = %q, want updated by policy", policy.writeRequest.Content)
	}
}

func TestCallbackClientReportsUnsupportedTerminalWithoutPolicy(t *testing.T) {
	t.Parallel()

	client := &callbackClient{policies: Policies{}.Normalize()}

	_, err := client.CreateTerminal(t.Context(), acp.CreateTerminalRequest{
		SessionId: "session-1",
		Command:   "echo",
		Args:      []string{"hello"},
	})
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("CreateTerminal() error = %v, want ErrUnsupportedOperation", err)
	}
}

func TestRestrictivePolicyDeniesByDefault(t *testing.T) {
	t.Parallel()

	policy := RestrictivePolicy{}

	permissionResp, err := policy.RequestPermission(t.Context(), PermissionRequest{})
	if err != nil {
		t.Fatalf("RequestPermission() error = %v", err)
	}
	if permissionResp.Outcome.Kind != PermissionOutcomeKindCancelled {
		t.Fatalf("permission outcome = %q, want cancelled", permissionResp.Outcome.Kind)
	}
	_, err = policy.ReadTextFile(t.Context(), ReadTextFileRequest{Path: "/tmp/nope"})
	if !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("ReadTextFile() error = %v, want ErrPolicyDenied", err)
	}
	_, err = policy.CreateTerminal(t.Context(), CreateTerminalRequest{Command: "echo"})
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("CreateTerminal() error = %v, want ErrUnsupportedOperation", err)
	}
}

func (p *recordingPermissionRequester) RequestPermission(
	_ context.Context,
	request PermissionRequest,
) (PermissionResponse, error) {
	p.request = request
	return PermissionResponse{Outcome: SelectedPermissionOutcome("allow")}, nil
}

func (p *recordingFilesystemPolicy) ReadTextFile(
	_ context.Context,
	request ReadTextFileRequest,
) (ReadTextFileResponse, error) {
	p.readRequest = request
	return ReadTextFileResponse{Content: "file content from policy"}, nil
}

func (p *recordingFilesystemPolicy) WriteTextFile(
	_ context.Context,
	request WriteTextFileRequest,
) (WriteTextFileResponse, error) {
	p.writeRequest = request
	return WriteTextFileResponse{}, nil
}
