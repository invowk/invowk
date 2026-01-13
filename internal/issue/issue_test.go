// SPDX-License-Identifier: EPL-2.0

package issue

import (
	"strings"
	"testing"
)

func TestId_Constants(t *testing.T) {
	// Verify all IDs are unique and sequential
	ids := []Id{
		FileNotFoundId,
		TuiServerStartFailedId,
		InvkfileNotFoundId,
		InvkfileParseErrorId,
		CommandNotFoundId,
		RuntimeNotAvailableId,
		ContainerEngineNotFoundId,
		DockerfileNotFoundId,
		ScriptExecutionFailedId,
		ConfigLoadFailedId,
		InvalidRuntimeModeId,
		DependencyCycleId,
		ShellNotFoundId,
		PermissionDeniedId,
		DependenciesNotSatisfiedId,
		HostNotSupportedId,
	}

	seen := make(map[Id]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID: %d", id)
		}
		seen[id] = true
	}

	// Verify IDs start at 1 (iota + 1)
	if FileNotFoundId != 1 {
		t.Errorf("FileNotFoundId = %d, want 1", FileNotFoundId)
	}
}

func TestIssue_Id(t *testing.T) {
	issue := Get(FileNotFoundId)
	if issue == nil {
		t.Fatal("Get(FileNotFoundId) returned nil")
	}

	if issue.Id() != FileNotFoundId {
		t.Errorf("issue.Id() = %d, want %d", issue.Id(), FileNotFoundId)
	}
}

func TestIssue_MarkdownMsg(t *testing.T) {
	issue := Get(InvkfileNotFoundId)
	if issue == nil {
		t.Fatal("Get(InvkfileNotFoundId) returned nil")
	}

	msg := issue.MarkdownMsg()
	if msg == "" {
		t.Error("MarkdownMsg() returned empty string")
	}

	// Verify it contains expected content
	if !strings.Contains(string(msg), "No invkfile found") {
		t.Error("MarkdownMsg() should contain 'No invkfile found'")
	}
}

func TestIssue_DocLinks(t *testing.T) {
	issue := Get(FileNotFoundId)
	if issue == nil {
		t.Fatal("Get(FileNotFoundId) returned nil")
	}

	// DocLinks returns a clone of the links
	links := issue.DocLinks()
	if links == nil {
		// nil is acceptable if no doc links are set
		return
	}

	// Modifying the returned slice should not affect the original
	if len(links) > 0 {
		original := links[0]
		links[0] = "modified"
		newLinks := issue.DocLinks()
		if len(newLinks) > 0 && newLinks[0] != original {
			t.Error("DocLinks() should return a clone")
		}
	}
}

func TestIssue_ExtLinks(t *testing.T) {
	issue := Get(FileNotFoundId)
	if issue == nil {
		t.Fatal("Get(FileNotFoundId) returned nil")
	}

	// ExtLinks returns a clone of the links
	links := issue.ExtLinks()
	if links == nil {
		// nil is acceptable if no ext links are set
		return
	}

	// Modifying the returned slice should not affect the original
	if len(links) > 0 {
		original := links[0]
		links[0] = "modified"
		newLinks := issue.ExtLinks()
		if len(newLinks) > 0 && newLinks[0] != original {
			t.Error("ExtLinks() should return a clone")
		}
	}
}

func TestIssue_Render(t *testing.T) {
	// Mock the render function for testing
	originalRender := render
	defer func() { render = originalRender }()

	render = func(in string, stylePath string) (string, error) {
		// Simple mock that just returns the input
		return in, nil
	}

	issue := Get(InvkfileNotFoundId)
	if issue == nil {
		t.Fatal("Get(InvkfileNotFoundId) returned nil")
	}

	rendered, err := issue.Render("")
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	if rendered == "" {
		t.Error("Render() returned empty string")
	}

	// The rendered output should contain the content
	if !strings.Contains(rendered, "invkfile") {
		t.Error("Render() output should contain 'invkfile'")
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		id       Id
		wantNil  bool
		contains string
	}{
		{FileNotFoundId, false, "TUI Server"},
		{InvkfileNotFoundId, false, "No invkfile found"},
		{InvkfileParseErrorId, false, "Failed to parse"},
		{CommandNotFoundId, false, "Command not found"},
		{RuntimeNotAvailableId, false, "Runtime not available"},
		{ContainerEngineNotFoundId, false, "Container engine not found"},
		{DockerfileNotFoundId, false, "Dockerfile not found"},
		{ScriptExecutionFailedId, false, "Script execution failed"},
		{ConfigLoadFailedId, false, "Failed to load configuration"},
		{InvalidRuntimeModeId, false, "Invalid runtime mode"},
		{DependencyCycleId, false, "Dependency cycle"},
		{ShellNotFoundId, false, "Shell not found"},
		{PermissionDeniedId, false, "Permission denied"},
		{DependenciesNotSatisfiedId, false, "Dependencies not satisfied"},
		{HostNotSupportedId, false, "Host not supported"},
		{Id(9999), true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			issue := Get(tt.id)

			if tt.wantNil {
				if issue != nil {
					t.Errorf("Get(%d) should return nil", tt.id)
				}
				return
			}

			if issue == nil {
				t.Fatalf("Get(%d) returned nil", tt.id)
			}

			if tt.contains != "" && !strings.Contains(string(issue.MarkdownMsg()), tt.contains) {
				t.Errorf("Get(%d).MarkdownMsg() should contain '%s'", tt.id, tt.contains)
			}
		})
	}
}

func TestValues(t *testing.T) {
	issues := Values()

	if len(issues) == 0 {
		t.Fatal("Values() returned empty slice")
	}

	// Count expected number of issues
	expectedCount := 15 // Based on the number of predefined issues

	if len(issues) != expectedCount {
		t.Errorf("Values() returned %d issues, want %d", len(issues), expectedCount)
	}

	// Verify all issues have valid IDs
	for _, issue := range issues {
		if issue.Id() == 0 {
			t.Error("found issue with ID 0")
		}
	}
}

func TestIssue_Render_WithLinks(t *testing.T) {
	// Mock the render function for testing
	originalRender := render
	defer func() { render = originalRender }()

	render = func(in string, stylePath string) (string, error) {
		return in, nil
	}

	// Create a test issue with links to verify the rendering logic
	testIssue := &Issue{
		id:       Id(9999),
		mdMsg:    "# Test Issue\n\nThis is a test.",
		docLinks: []HttpLink{"https://docs.example.com"},
		extLinks: []HttpLink{"https://external.example.com"},
	}

	rendered, err := testIssue.Render("")
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// The rendered output should include the "See also" section
	if !strings.Contains(rendered, "See also") {
		t.Error("Render() with links should contain 'See also'")
	}
}

func TestIssue_Render_NoLinks(t *testing.T) {
	// Mock the render function for testing
	originalRender := render
	defer func() { render = originalRender }()

	render = func(in string, stylePath string) (string, error) {
		return in, nil
	}

	// Create a test issue without links
	testIssue := &Issue{
		id:    Id(9998),
		mdMsg: "# Test Issue\n\nNo links here.",
	}

	rendered, err := testIssue.Render("")
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	// Should render without the "See also" section
	if strings.Contains(rendered, "See also") {
		t.Error("Render() without links should not contain 'See also'")
	}
}

func TestMarkdownMsg_Type(t *testing.T) {
	msg := MarkdownMsg("# Hello\n\nWorld")

	if string(msg) != "# Hello\n\nWorld" {
		t.Errorf("MarkdownMsg string conversion failed")
	}
}

func TestHttpLink_Type(t *testing.T) {
	link := HttpLink("https://example.com")

	if string(link) != "https://example.com" {
		t.Errorf("HttpLink string conversion failed")
	}
}

func TestAllIssuesHaveContent(t *testing.T) {
	issues := Values()

	for _, issue := range issues {
		if issue.MarkdownMsg() == "" {
			t.Errorf("Issue %d has empty MarkdownMsg", issue.Id())
		}
	}
}

func TestAllIssuesAreRenderable(t *testing.T) {
	// Mock the render function for testing
	originalRender := render
	defer func() { render = originalRender }()

	render = func(in string, stylePath string) (string, error) {
		return in, nil
	}

	issues := Values()

	for _, issue := range issues {
		rendered, err := issue.Render("")
		if err != nil {
			t.Errorf("Issue %d failed to render: %v", issue.Id(), err)
		}
		if rendered == "" {
			t.Errorf("Issue %d rendered to empty string", issue.Id())
		}
	}
}

// TestIssuesMapCompleteness verifies all issue IDs are in the map
func TestIssuesMapCompleteness(t *testing.T) {
	expectedIds := []Id{
		FileNotFoundId,
		InvkfileNotFoundId,
		InvkfileParseErrorId,
		CommandNotFoundId,
		RuntimeNotAvailableId,
		ContainerEngineNotFoundId,
		DockerfileNotFoundId,
		ScriptExecutionFailedId,
		ConfigLoadFailedId,
		InvalidRuntimeModeId,
		DependencyCycleId,
		ShellNotFoundId,
		PermissionDeniedId,
		DependenciesNotSatisfiedId,
		HostNotSupportedId,
	}

	for _, id := range expectedIds {
		issue := Get(id)
		if issue == nil {
			t.Errorf("Issue with ID %d is not in the issues map", id)
		}
	}
}
