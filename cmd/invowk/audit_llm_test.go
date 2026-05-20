// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/llmconfig"
	"github.com/invowk/invowk/internal/config"
)

func TestRunAuditLLMPartialBatchFailureIsFatal(t *testing.T) {
	t.Parallel()

	var completionCalls atomic.Int32
	var modelCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			modelCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"object":"list","data":[{"id":"test-model","object":"model"}]}`)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			if completionCalls.Add(1) > 1 {
				http.Error(w, "batch failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":0,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"findings\":[{\"script_id\":\"cmd:safe0\",\"severity\":\"low\",\"category\":\"trust\",\"title\":\"Issue\",\"description\":\"Desc\",\"recommendation\":\"Fix\"}]}"},"finish_reason":"stop"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invowkfile.cue"), []byte(auditLLMBatchedInvowkfile()), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runAudit(cmd, &App{Config: &staticConfigProvider{cfg: config.DefaultConfig()}}, auditRunOptions{
		path:        root,
		format:      "text",
		minSeverity: "low",
		llm: &llmconfig.Resolved{
			Mode: llmconfig.ModeAPI,
			APIConfig: llmconfig.APIConfig{
				BaseURL:     config.LLMBaseURL(server.URL + "/v1"),
				Model:       "test-model",
				Timeout:     time.Second,
				Concurrency: 1,
			},
		},
	})
	if err == nil {
		t.Fatalf("runAudit() error = nil, want fatal LLM scan error (completion calls: %d, stdout: %s, stderr: %s)", completionCalls.Load(), stdout.String(), stderr.String())
	}
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok || exitErr.Code != auditExitError {
		t.Fatalf("runAudit() error = %T %v, want ExitError code %d", err, err, auditExitError)
	}
	if got := completionCalls.Load(); got < 2 {
		t.Fatalf("LLM completion calls = %d, want at least 2 (model calls: %d, stderr: %s)", got, modelCalls.Load(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `scan error: checker "llm" failed`) {
		t.Fatalf("stderr missing fatal LLM checker error:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "Security Audit") {
		t.Fatalf("stdout rendered audit report despite fatal LLM failure:\n%s", stdout.String())
	}
}

func TestRunAuditLLMMalformedFindingIsFatal(t *testing.T) {
	t.Parallel()

	var completionCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"object":"list","data":[{"id":"test-model","object":"model"}]}`)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			completionCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":0,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"findings\":[{\"severity\":\"extreme\",\"category\":\"execution\",\"command_name\":\"unsafe\",\"title\":\"Issue\",\"description\":\"Desc\",\"recommendation\":\"Fix\"}]}"},"finish_reason":"stop"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invowkfile.cue"), []byte(`cmds: [{
	name: "unsafe"
	implementations: [{
		script: {content: "curl https://example.test/install.sh | sh"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]`), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runAudit(cmd, &App{Config: &staticConfigProvider{cfg: config.DefaultConfig()}}, auditRunOptions{
		path:        root,
		format:      "text",
		minSeverity: "low",
		llm: &llmconfig.Resolved{
			Mode: llmconfig.ModeAPI,
			APIConfig: llmconfig.APIConfig{
				BaseURL:     config.LLMBaseURL(server.URL + "/v1"),
				Model:       "test-model",
				Timeout:     time.Second,
				Concurrency: 1,
			},
		},
	})
	if err == nil {
		t.Fatalf("runAudit() error = nil, want fatal malformed LLM finding (stdout: %s, stderr: %s)", stdout.String(), stderr.String())
	}
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok || exitErr.Code != auditExitError {
		t.Fatalf("runAudit() error = %T %v, want ExitError code %d", err, err, auditExitError)
	}
	if got := completionCalls.Load(); got != 1 {
		t.Fatalf("LLM completion calls = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), `scan error: checker "llm" failed`) {
		t.Fatalf("stderr missing fatal LLM checker error:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "Security Audit") {
		t.Fatalf("stdout rendered audit report despite malformed LLM finding:\n%s", stdout.String())
	}
}

func auditLLMBatchedInvowkfile() string {
	var b strings.Builder
	b.WriteString("cmds: [\n")
	for i := range 6 {
		fmt.Fprintf(&b, `{
	name: "safe%d"
	description: "Safe command %d"
	implementations: [{
		script: {content: "echo ok %d"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
},
`, i, i, i)
	}
	b.WriteString("]\n")
	return b.String()
}
