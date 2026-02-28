// SPDX-License-Identifier: MPL-2.0

package issue

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestId_Constants(t *testing.T) {
	t.Parallel()

	// Verify all IDs are unique and sequential
	ids := []Id{
		FileNotFoundId,
		InvowkfileNotFoundId,
		InvowkfileParseErrorId,
		CommandNotFoundId,
		RuntimeNotAvailableId,
		ContainerEngineNotFoundId,
		DockerfileNotFoundId,
		ScriptExecutionFailedId,
		ConfigLoadFailedId,
		InvalidRuntimeModeId,
		ShellNotFoundId,
		PermissionDeniedId,
		DependenciesNotSatisfiedId,
		HostNotSupportedId,
		InvalidArgumentId,
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
	t.Parallel()

	issue := Get(FileNotFoundId)
	if issue == nil {
		t.Fatal("Get(FileNotFoundId) returned nil")
	}

	if issue.Id() != FileNotFoundId {
		t.Errorf("issue.Id() = %d, want %d", issue.Id(), FileNotFoundId)
	}
}

func TestIssue_MarkdownMsg(t *testing.T) {
	t.Parallel()

	issue := Get(InvowkfileNotFoundId)
	if issue == nil {
		t.Fatal("Get(InvowkfileNotFoundId) returned nil")
	}

	msg := issue.MarkdownMsg()
	if msg == "" {
		t.Error("MarkdownMsg() returned empty string")
	}

	// Verify it contains expected content
	if !strings.Contains(string(msg), "No invowkfile found") {
		t.Error("MarkdownMsg() should contain 'No invowkfile found'")
	}
}

func TestIssue_DocLinks(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	mockRender := func(in string, _ string) (string, error) {
		return in, nil
	}

	issue := Get(InvowkfileNotFoundId)
	if issue == nil {
		t.Fatal("Get(InvowkfileNotFoundId) returned nil")
	}

	rendered, err := issue.RenderWith(mockRender, "")
	if err != nil {
		t.Fatalf("RenderWith() returned error: %v", err)
	}

	if rendered == "" {
		t.Error("RenderWith() returned empty string")
	}

	// The rendered output should contain the content
	if !strings.Contains(rendered, "invowkfile") {
		t.Error("RenderWith() output should contain 'invowkfile'")
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       Id
		wantNil  bool
		contains string
	}{
		{FileNotFoundId, false, "File not found"},
		{InvowkfileNotFoundId, false, "No invowkfile found"},
		{InvowkfileParseErrorId, false, "Failed to parse"},
		{CommandNotFoundId, false, "Command not found"},
		{RuntimeNotAvailableId, false, "Runtime not available"},
		{ContainerEngineNotFoundId, false, "Container engine not found"},
		{DockerfileNotFoundId, false, "Dockerfile not found"},
		{ScriptExecutionFailedId, false, "Script execution failed"},
		{ConfigLoadFailedId, false, "Failed to load configuration"},
		{InvalidRuntimeModeId, false, "Invalid runtime mode"},
		{ShellNotFoundId, false, "Shell not found"},
		{PermissionDeniedId, false, "Permission denied"},
		{DependenciesNotSatisfiedId, false, "Dependencies not satisfied"},
		{HostNotSupportedId, false, "Host not supported"},
		{Id(9999), true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	mockRender := func(in string, _ string) (string, error) {
		return in, nil
	}

	// Create a test issue with links to verify the rendering logic
	testIssue := &Issue{
		id:       Id(9999),
		mdMsg:    "# Test Issue\n\nThis is a test.",
		docLinks: []HttpLink{"https://docs.example.com"},
		extLinks: []HttpLink{"https://external.example.com"},
	}

	rendered, err := testIssue.RenderWith(mockRender, "")
	if err != nil {
		t.Fatalf("RenderWith() returned error: %v", err)
	}

	// The rendered output should include the "See also" section
	if !strings.Contains(rendered, "See also") {
		t.Error("RenderWith() with links should contain 'See also'")
	}
}

func TestIssue_Render_NoLinks(t *testing.T) {
	t.Parallel()

	mockRender := func(in string, _ string) (string, error) {
		return in, nil
	}

	// Create a test issue without links
	testIssue := &Issue{
		id:    Id(9998),
		mdMsg: "# Test Issue\n\nNo links here.",
	}

	rendered, err := testIssue.RenderWith(mockRender, "")
	if err != nil {
		t.Fatalf("RenderWith() returned error: %v", err)
	}

	// Should render without the "See also" section
	if strings.Contains(rendered, "See also") {
		t.Error("RenderWith() without links should not contain 'See also'")
	}
}

func TestMarkdownMsg_Type(t *testing.T) {
	t.Parallel()

	msg := MarkdownMsg("# Hello\n\nWorld")

	if string(msg) != "# Hello\n\nWorld" {
		t.Errorf("MarkdownMsg string conversion failed")
	}
}

func TestHttpLink_Type(t *testing.T) {
	t.Parallel()

	link := HttpLink("https://example.com")

	if string(link) != "https://example.com" {
		t.Errorf("HttpLink string conversion failed")
	}
}

func TestAllIssuesHaveContent(t *testing.T) {
	t.Parallel()

	issues := Values()

	for _, issue := range issues {
		if issue.MarkdownMsg() == "" {
			t.Errorf("Issue %d has empty MarkdownMsg", issue.Id())
		}
	}
}

func TestAllIssuesAreRenderable(t *testing.T) {
	t.Parallel()

	mockRender := func(in string, _ string) (string, error) {
		return in, nil
	}

	issues := Values()

	for _, issue := range issues {
		rendered, err := issue.RenderWith(mockRender, "")
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
	t.Parallel()

	expectedIds := []Id{
		FileNotFoundId,
		InvowkfileNotFoundId,
		InvowkfileParseErrorId,
		CommandNotFoundId,
		RuntimeNotAvailableId,
		ContainerEngineNotFoundId,
		DockerfileNotFoundId,
		ScriptExecutionFailedId,
		ConfigLoadFailedId,
		InvalidRuntimeModeId,
		ShellNotFoundId,
		PermissionDeniedId,
		DependenciesNotSatisfiedId,
		HostNotSupportedId,
		InvalidArgumentId,
	}

	for _, id := range expectedIds {
		issue := Get(id)
		if issue == nil {
			t.Errorf("Issue with ID %d is not in the issues map", id)
		}
	}
}

func TestIssueTemplates_NoStaleGuidance(t *testing.T) {
	t.Parallel()

	tokens := []string{
		"invowk fix",
		"apk add --no-cache",
	}

	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		t.Fatalf("failed to read embedded issue templates: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, readErr := templateFS.ReadFile("templates/" + entry.Name())
		if readErr != nil {
			t.Fatalf("failed to read embedded template %s: %v", entry.Name(), readErr)
		}

		content := strings.ToLower(string(data))
		for _, token := range tokens {
			if strings.Contains(content, token) {
				t.Errorf("template %s contains stale guidance token %q", entry.Name(), token)
			}
		}
	}
}

func TestId_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   Id
		wantOK  bool
		wantErr bool
	}{
		{"FileNotFoundId", FileNotFoundId, true, false},
		{"InvowkfileNotFoundId", InvowkfileNotFoundId, true, false},
		{"InvowkfileParseErrorId", InvowkfileParseErrorId, true, false},
		{"CommandNotFoundId", CommandNotFoundId, true, false},
		{"RuntimeNotAvailableId", RuntimeNotAvailableId, true, false},
		{"ContainerEngineNotFoundId", ContainerEngineNotFoundId, true, false},
		{"DockerfileNotFoundId", DockerfileNotFoundId, true, false},
		{"ScriptExecutionFailedId", ScriptExecutionFailedId, true, false},
		{"ConfigLoadFailedId", ConfigLoadFailedId, true, false},
		{"InvalidRuntimeModeId", InvalidRuntimeModeId, true, false},
		{"ShellNotFoundId", ShellNotFoundId, true, false},
		{"PermissionDeniedId", PermissionDeniedId, true, false},
		{"DependenciesNotSatisfiedId", DependenciesNotSatisfiedId, true, false},
		{"HostNotSupportedId", HostNotSupportedId, true, false},
		{"InvalidArgumentId", InvalidArgumentId, true, false},
		{"zero value", Id(0), false, true},
		{"out of range positive", Id(9999), false, true},
		{"negative", Id(-1), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Id(%d).Validate() error = %v, wantOK %v", tt.value, err, tt.wantOK)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error, got nil")
				} else if !errors.Is(err, ErrInvalidId) {
					t.Errorf("expected errors.Is(err, ErrInvalidId), got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestMarkdownMsg_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   MarkdownMsg
		wantOK  bool
		wantErr bool
	}{
		{"valid markdown", MarkdownMsg("# Hello\n\nWorld"), true, false},
		{"simple content", MarkdownMsg("content"), true, false},
		{"single character", MarkdownMsg("x"), true, false},
		{"empty string", MarkdownMsg(""), false, true},
		{"whitespace only", MarkdownMsg("   "), false, true},
		{"newlines and tabs", MarkdownMsg("\n\t\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("MarkdownMsg(%q).Validate() error = %v, wantOK %v", tt.value, err, tt.wantOK)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error, got nil")
				} else if !errors.Is(err, ErrInvalidMarkdownMsg) {
					t.Errorf("expected errors.Is(err, ErrInvalidMarkdownMsg), got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestHttpLink_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   HttpLink
		wantOK  bool
		wantErr bool
	}{
		{"https URL", HttpLink("https://example.com"), true, false},
		{"http URL", HttpLink("http://docs.invowk.io/path"), true, false},
		{"https with path and query", HttpLink("https://example.com/path?q=1"), true, false},
		{"empty string", HttpLink(""), false, true},
		{"ftp scheme", HttpLink("ftp://bad.example.com"), false, true},
		{"no scheme", HttpLink("not-a-url"), false, true},
		{"javascript scheme", HttpLink("javascript:alert(1)"), false, true},
		{"bare domain", HttpLink("example.com"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("HttpLink(%q).Validate() error = %v, wantOK %v", tt.value, err, tt.wantOK)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error, got nil")
				} else if !errors.Is(err, ErrInvalidHttpLink) {
					t.Errorf("expected errors.Is(err, ErrInvalidHttpLink), got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestId_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   Id
		want string
	}{
		{FileNotFoundId, strconv.Itoa(int(FileNotFoundId))},
		{InvalidArgumentId, strconv.Itoa(int(InvalidArgumentId))},
		{Id(0), "0"},
		{Id(999), "999"},
	}

	for _, tt := range tests {
		if got := tt.id.String(); got != tt.want {
			t.Errorf("Id(%d).String() = %q, want %q", int(tt.id), got, tt.want)
		}
	}
}

func TestMarkdownMsg_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg  MarkdownMsg
		want string
	}{
		{"# Hello", "# Hello"},
		{"", ""},
		{"multi\nline", "multi\nline"},
	}

	for _, tt := range tests {
		if got := tt.msg.String(); got != tt.want {
			t.Errorf("MarkdownMsg(%q).String() = %q, want %q", string(tt.msg), got, tt.want)
		}
	}
}

func TestHttpLink_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		link HttpLink
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := tt.link.String(); got != tt.want {
			t.Errorf("HttpLink(%q).String() = %q, want %q", string(tt.link), got, tt.want)
		}
	}
}
