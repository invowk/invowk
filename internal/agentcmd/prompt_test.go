// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSystemPromptIncludesSchemasAndRuntimeGuidance(t *testing.T) {
	t.Parallel()

	prompt := BuildSystemPrompt()
	for _, want := range []string{
		"invowkfile.cue schema:",
		"#Invowkfile",
		"invowkmod.cue schema:",
		"#Invowkmod",
		"not a security sandbox",
		"debian:stable-slim",
		"Return only JSON",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("BuildSystemPrompt() missing %q", want)
		}
	}
}

func TestRenderPromptJSON(t *testing.T) {
	t.Parallel()

	rendered, err := RenderPrompt("json")
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}

	var doc PromptDocument
	if err := json.Unmarshal([]byte(rendered), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if doc.SystemPrompt == "" {
		t.Fatal("SystemPrompt is empty")
	}
	if doc.Schemas["invowkfile.cue"] == "" || doc.Schemas["invowkmod.cue"] == "" {
		t.Fatalf("schemas missing: %#v", doc.Schemas)
	}
}

func TestRenderPromptRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	_, err := RenderPrompt("yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
