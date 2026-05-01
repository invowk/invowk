// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestBuildUserPrompt_SingleScript(t *testing.T) {
	t.Parallel()

	scripts := []ScriptRef{
		{
			CommandName:     "build",
			FilePath:        types.FilesystemPath("/path/to/invowkfile.cue"),
			Script:          invowkfile.ScriptContent("scripts/build.sh"),
			IsFile:          true,
			resolvedContent: "#!/bin/bash\necho hello",
			Runtimes: []invowkfile.RuntimeConfig{
				{Name: invowkfile.RuntimeNative},
			},
		},
	}

	prompt := buildUserPrompt(scripts)

	if !strings.Contains(prompt, "=== Script: build ===") {
		t.Error("prompt should contain script header with command name")
	}
	if !strings.Contains(prompt, "Script ID: ") {
		t.Error("prompt should contain stable script ID")
	}
	if !strings.Contains(prompt, "File: /path/to/invowkfile.cue") {
		t.Error("prompt should contain file path")
	}
	if !strings.Contains(prompt, "Runtime: native") {
		t.Error("prompt should contain runtime name")
	}
	if !strings.Contains(prompt, "echo hello") {
		t.Error("prompt should contain script content")
	}
}

func TestBuildUserPrompt_MultipleScripts(t *testing.T) {
	t.Parallel()

	scripts := []ScriptRef{
		{CommandName: "build", Script: invowkfile.ScriptContent("make build")},
		{CommandName: "test", Script: invowkfile.ScriptContent("make test")},
	}

	prompt := buildUserPrompt(scripts)

	if !strings.Contains(prompt, "=== Script: build ===") {
		t.Error("prompt should contain first script header")
	}
	if !strings.Contains(prompt, "=== Script: test ===") {
		t.Error("prompt should contain second script header")
	}
}

func TestSystemPromptEmbeddedMarkdownContract(t *testing.T) {
	t.Parallel()

	required := []string{
		"You are a security auditor for Invowk",
		"The deterministic audit scanner already checks obvious patterns",
		"module supply-chain scenarios",
		"The virtual runtime is a portable shell interpreter, not a security sandbox",
		"Command scope enforcement is static validation",
		`"info", "low", "medium", "high", "critical"`,
		`"integrity"`,
		`"path-traversal"`,
		`"exfiltration"`,
		`"execution"`,
		`"trust"`,
		`"obfuscation"`,
		`{"findings": [{"script_id": "..."`,
		`{"findings": []}`,
		"Return ONLY a JSON object",
		"Include a concrete exploit path in description",
		"Prefer no finding over a speculative finding",
	}

	for _, want := range required {
		if !strings.Contains(systemPrompt, want) {
			t.Errorf("systemPrompt should contain %q", want)
		}
	}
}

func TestParseFindings_ValidJSON(t *testing.T) {
	t.Parallel()

	raw := `{"findings": [{"severity": "high", "category": "execution", "command_name": "deploy", "title": "Remote code execution", "description": "Downloads and executes script", "recommendation": "Pin URLs", "line": 5}]}`

	findings, err := parseFindings(raw)
	if err != nil {
		t.Fatalf("parseFindings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != "high" {
		t.Errorf("severity = %q, want %q", findings[0].Severity, "high")
	}
	if findings[0].Category != "execution" {
		t.Errorf("category = %q, want %q", findings[0].Category, "execution")
	}
	if findings[0].Line != 5 {
		t.Errorf("line = %d, want %d", findings[0].Line, 5)
	}
}

func TestParseFindings_EmptyFindings(t *testing.T) {
	t.Parallel()

	raw := `{"findings": []}`
	findings, err := parseFindings(raw)
	if err != nil {
		t.Fatalf("parseFindings: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestParseFindings_MarkdownFenced(t *testing.T) {
	t.Parallel()

	raw := "Here are the findings:\n```json\n{\"findings\": [{\"severity\": \"medium\", \"category\": \"exfiltration\", \"command_name\": \"sync\", \"title\": \"Network access\", \"description\": \"Sends data\", \"recommendation\": \"Review\"}]}\n```"

	findings, err := parseFindings(raw)
	if err != nil {
		t.Fatalf("parseFindings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != "medium" {
		t.Errorf("severity = %q, want %q", findings[0].Severity, "medium")
	}
}

func TestParseFindings_ExtractFromSurroundingText(t *testing.T) {
	t.Parallel()

	raw := `I analyzed the scripts. Here are my findings: {"findings": [{"severity": "low", "category": "trust", "command_name": "init", "title": "Unpinned dep", "description": "Version not pinned", "recommendation": "Pin version"}]} That's all.`

	findings, err := parseFindings(raw)
	if err != nil {
		t.Fatalf("parseFindings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestParseFindings_Malformed(t *testing.T) {
	t.Parallel()

	raw := "This is not JSON at all, SECRET_TOKEN=hunter2"

	_, err := parseFindings(raw)
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
	if !errors.Is(err, ErrLLMMalformedResponse) {
		t.Errorf("expected ErrLLMMalformedResponse, got %v", err)
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("malformed response error leaked raw response: %v", err)
	}
	var malformed *LLMMalformedResponseError
	if !errors.As(err, &malformed) {
		t.Fatalf("errors.As(*LLMMalformedResponseError) = false for %T", err)
	}
	if !strings.Contains(malformed.RawResponsePreview(), "SECRET_TOKEN") {
		t.Fatalf("RawResponsePreview() did not retain bounded debug response")
	}
}

func TestConvertBatchFindings_ValidSingleScript(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{
			Severity:       "high",
			Category:       "execution",
			CommandName:    "deploy",
			Title:          "Remote code execution",
			Description:    "Downloads and executes script",
			Recommendation: "Pin URLs",
			Line:           5,
		},
	}
	batch := []ScriptRef{
		{SurfaceID: "test-surface", FilePath: types.FilesystemPath("/test/invowkfile.cue"), CommandName: "deploy"},
	}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("severity = %v, want %v", findings[0].Severity, SeverityHigh)
	}
	if findings[0].Category != CategoryExecution {
		t.Errorf("category = %v, want %v", findings[0].Category, CategoryExecution)
	}
	if findings[0].CheckerName != llmCheckerName {
		t.Errorf("checker = %q, want %q", findings[0].CheckerName, llmCheckerName)
	}
	if findings[0].SurfaceID != "test-surface" {
		t.Errorf("surface = %q, want %q", findings[0].SurfaceID, "test-surface")
	}
}

func TestConvertBatchFindings_InvalidSeverity(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{Severity: "extreme", Category: "execution", Title: "Test"},
	}
	batch := []ScriptRef{{SurfaceID: "test"}}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (invalid severity discarded), got %d", len(findings))
	}
}

func TestConvertBatchFindings_InvalidCategory(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{Severity: "high", Category: "nonexistent-category", Title: "Test"},
	}
	batch := []ScriptRef{{SurfaceID: "test"}}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (invalid category discarded), got %d", len(findings))
	}
}

func TestConvertBatchFindings_MatchesByCommandName(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{Severity: "high", Category: "execution", CommandName: "deploy", Title: "RCE"},
	}
	batch := []ScriptRef{
		{CommandName: "build", SurfaceID: "build-surface", FilePath: "build.cue"},
		{CommandName: "deploy", SurfaceID: "deploy-surface", FilePath: "deploy.cue"},
	}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].SurfaceID != "deploy-surface" {
		t.Errorf("surface = %q, want %q", findings[0].SurfaceID, "deploy-surface")
	}
}

func TestConvertBatchFindings_MatchesByScriptID(t *testing.T) {
	t.Parallel()

	batch := []ScriptRef{
		{CommandName: "deploy", SurfaceID: "surface-a", FilePath: "a.cue"},
		{CommandName: "deploy", SurfaceID: "surface-b", FilePath: "b.cue"},
	}
	parsed := []llmFinding{
		{ScriptID: scriptPromptID(&batch[1]), Severity: "high", Category: "execution", CommandName: "deploy", Title: "RCE"},
	}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].SurfaceID != "surface-b" {
		t.Errorf("surface = %q, want %q", findings[0].SurfaceID, "surface-b")
	}
}

func TestConvertBatchFindings_DropsUnknownMultiScriptAttribution(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{Severity: "medium", Category: "trust", CommandName: "unknown", Title: "Issue"},
	}
	batch := []ScriptRef{
		{CommandName: "build", SurfaceID: "build-surface", FilePath: "build.cue"},
		{CommandName: "deploy", SurfaceID: "deploy-surface", FilePath: "deploy.cue"},
	}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for unknown multi-script attribution, got %d", len(findings))
	}
}

func TestConvertBatchFindings_SingleScriptFallback(t *testing.T) {
	t.Parallel()

	parsed := []llmFinding{
		{Severity: "medium", Category: "trust", CommandName: "unknown", Title: "Issue"},
	}
	batch := []ScriptRef{
		{CommandName: "build", SurfaceID: "build-surface", FilePath: "build.cue"},
	}

	findings := convertBatchFindings(parsed, batch)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	// Single-script batch: attribute to the only script.
	if findings[0].SurfaceID != "build-surface" {
		t.Errorf("surface = %q, want %q", findings[0].SurfaceID, "build-surface")
	}
}

func TestTruncateScript(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()
		result, truncated := truncateScript("short", 100)
		if truncated {
			t.Error("should not be truncated")
		}
		if result != "short" {
			t.Errorf("result = %q, want %q", result, "short")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("x", 200)
		result, truncated := truncateScript(long, 50)
		if !truncated {
			t.Error("should be truncated")
		}
		if !strings.Contains(result, "[TRUNCATED at 50 chars]") {
			t.Error("should contain truncation marker")
		}
		if len(result) > 100 {
			t.Errorf("truncated result too long: %d chars", len(result))
		}
	})
}

func TestPrepareScripts_FiltersAndTruncates(t *testing.T) {
	t.Parallel()

	scripts := []ScriptRef{
		{CommandName: "build", Script: "make build", IsFile: false},
		{CommandName: "deploy", Script: "", IsFile: false},
		{CommandName: "test", Script: "scripts/test.sh", IsFile: true, resolvedContent: "make test"},
		{CommandName: "lint", Script: "  \t\n  ", IsFile: false},
		{CommandName: "clean", Script: "rm -rf dist", IsFile: false},
	}

	prepared := prepareScripts(scripts, maxScriptChars)
	if len(prepared) != 3 {
		t.Fatalf("expected 3 analyzable scripts, got %d", len(prepared))
	}
	if prepared[0].CommandName != "build" {
		t.Errorf("first = %q, want %q", prepared[0].CommandName, "build")
	}
	if prepared[1].CommandName != "test" {
		t.Errorf("second = %q, want %q", prepared[1].CommandName, "test")
	}
	if prepared[1].Content() != "make test" {
		t.Errorf("file script content = %q, want %q", prepared[1].Content(), "make test")
	}
	if prepared[2].CommandName != "clean" {
		t.Errorf("third = %q, want %q", prepared[2].CommandName, "clean")
	}
}
