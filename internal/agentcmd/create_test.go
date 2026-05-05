// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/llm"
)

const testCommandObject = `{
	name: "hello generated"
	description: "Print a generated greeting"
	implementations: [{
		script: "echo generated"
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}`

type (
	fakeCompleter struct {
		response string
	}

	sequenceCompleter struct {
		responses []string
		prompts   []string
	}

	structuredCompleter struct {
		response          string
		fallbackResponse  string
		structuredErr     error
		structuredFormats []llm.JSONSchemaFormat
		fallbackCalls     int
	}
)

func (c fakeCompleter) Complete(context.Context, string, string) (string, error) {
	return c.response, nil
}

func (c *sequenceCompleter) Complete(_ context.Context, _, userPrompt string) (string, error) {
	c.prompts = append(c.prompts, userPrompt)
	if len(c.responses) == 0 {
		return "", errors.New("unexpected completion call")
	}
	resp := c.responses[0]
	c.responses = c.responses[1:]
	return resp, nil
}

func (c *structuredCompleter) Complete(context.Context, string, string) (string, error) {
	c.fallbackCalls++
	return c.fallbackResponse, nil
}

func (c *structuredCompleter) CompleteJSONSchema(_ context.Context, _, _ string, format llm.JSONSchemaFormat) (string, error) {
	c.structuredFormats = append(c.structuredFormats, format)
	if c.structuredErr != nil {
		return "", c.structuredErr
	}
	return c.response, nil
}

func TestParseGenerationResponseDirectAndFenced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "direct", raw: `{"command_cue":"{name: \"x\"}","summary":"ok"}`},
		{name: "fenced", raw: "```json\n{\"command_cue\":\"{name: \\\"x\\\"}\",\"summary\":\"ok\"}\n```"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseGenerationResponse(tt.raw)
			if err != nil {
				t.Fatalf("ParseGenerationResponse() error = %v", err)
			}
			if got.CommandCUE == "" || got.Summary != "ok" {
				t.Fatalf("ParseGenerationResponse() = %#v", got)
			}
		})
	}
}

func TestValidateCommandCUERejectsCmdsList(t *testing.T) {
	t.Parallel()

	_, _, err := ValidateCommandCUE("cmds: []")
	if err == nil {
		t.Fatal("expected error for cmds list")
	}
}

func TestPatchInvowkfileAppendsAndReplacesCommand(t *testing.T) {
	t.Parallel()

	existing := `// keep this comment
cmds: [{
	name: "existing"
	description: "Existing command"
	implementations: [{
		script: "echo existing"
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`

	command, commandCUE, err := ValidateCommandCUE(testCommandObject)
	if err != nil {
		t.Fatalf("ValidateCommandCUE() error = %v", err)
	}

	appended, err := PatchInvowkfile(existing, true, commandCUE, command.Name, false, "invowkfile.cue")
	if err != nil {
		t.Fatalf("PatchInvowkfile() append error = %v", err)
	}
	for _, want := range []string{"keep this comment", "existing", "hello generated"} {
		if !strings.Contains(appended, want) {
			t.Fatalf("appended content missing %q:\n%s", want, appended)
		}
	}

	_, err = PatchInvowkfile(appended, true, commandCUE, command.Name, false, "invowkfile.cue")
	if err == nil {
		t.Fatal("expected duplicate command error")
	}

	replaced, err := PatchInvowkfile(appended, true, commandCUE, command.Name, true, "invowkfile.cue")
	if err != nil {
		t.Fatalf("PatchInvowkfile() replace error = %v", err)
	}
	if strings.Count(replaced, "hello generated") != 1 {
		t.Fatalf("replace should keep one generated command:\n%s", replaced)
	}
}

func TestCreateCommandDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "invowkfile.cue")
	response := `{"command_cue":` + quoteForJSON(testCommandObject) + `,"summary":"added"}`

	result, err := CreateCommand(t.Context(), CreateOptions{
		Description: "make a hello command",
		TargetPath:  target,
		DryRun:      true,
		Completer:   fakeCompleter{response: response},
	})
	if err != nil {
		t.Fatalf("CreateCommand() error = %v", err)
	}
	if result.CommandName != "hello generated" {
		t.Fatalf("CommandName = %q", result.CommandName)
	}
	if !strings.Contains(result.Diff, "--- /dev/null") || !strings.Contains(result.Diff, "hello generated") {
		t.Fatalf("dry-run diff missing expected content:\n%s", result.Diff)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote target file, stat err = %v", err)
	}
}

func TestCreateCommandRepairsInvalidModelResponse(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "invowkfile.cue")
	response := `{"command_cue":` + quoteForJSON(testCommandObject) + `,"summary":"repaired"}`
	completer := &sequenceCompleter{
		responses: []string{"not json", response},
	}

	result, err := CreateCommand(t.Context(), CreateOptions{
		Description: "make a hello command",
		TargetPath:  target,
		DryRun:      true,
		Completer:   completer,
	})
	if err != nil {
		t.Fatalf("CreateCommand() error = %v", err)
	}
	if result.CommandName != "hello generated" {
		t.Fatalf("CommandName = %q", result.CommandName)
	}
	if len(completer.prompts) != 2 {
		t.Fatalf("completion calls = %d, want 2", len(completer.prompts))
	}
	for _, want := range []string{"previous response was rejected", "could not extract generated command JSON", "not json"} {
		if !strings.Contains(completer.prompts[1], want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, completer.prompts[1])
		}
	}
}

func TestCreateCommandUsesStructuredCompletionWhenAvailable(t *testing.T) {
	t.Parallel()

	response := `{"command_cue":` + quoteForJSON(testCommandObject) + `,"summary":"structured"}`
	completer := &structuredCompleter{response: response}

	result, err := CreateCommand(t.Context(), CreateOptions{
		Description: "make a hello command",
		DryRun:      true,
		Completer:   completer,
	})
	if err != nil {
		t.Fatalf("CreateCommand() error = %v", err)
	}
	if result.CommandName != "hello generated" {
		t.Fatalf("CommandName = %q", result.CommandName)
	}
	if completer.fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0", completer.fallbackCalls)
	}
	if len(completer.structuredFormats) != 1 {
		t.Fatalf("structured formats = %d, want 1", len(completer.structuredFormats))
	}
	if completer.structuredFormats[0].Name != "invowk_command_generation" || !completer.structuredFormats[0].Strict {
		t.Fatalf("unexpected structured format: %#v", completer.structuredFormats[0])
	}
}

func TestCreateCommandFallsBackWhenStructuredCompletionUnsupported(t *testing.T) {
	t.Parallel()

	response := `{"command_cue":` + quoteForJSON(testCommandObject) + `,"summary":"fallback"}`
	completer := &structuredCompleter{
		fallbackResponse: response,
		structuredErr:    llm.ErrStructuredOutputUnsupported,
	}

	result, err := CreateCommand(t.Context(), CreateOptions{
		Description: "make a hello command",
		DryRun:      true,
		Completer:   completer,
	})
	if err != nil {
		t.Fatalf("CreateCommand() error = %v", err)
	}
	if result.CommandName != "hello generated" {
		t.Fatalf("CommandName = %q", result.CommandName)
	}
	if completer.fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d, want 1", completer.fallbackCalls)
	}
}

func TestCreateCommandPrintOnlyDoesNotPatchDuplicateTarget(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "invowkfile.cue")
	existing := wrapCommandObject(testCommandObject)
	if err := os.WriteFile(target, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target fixture: %v", err)
	}
	response := `{"command_cue":` + quoteForJSON(testCommandObject) + `,"summary":"print only"}`

	result, err := CreateCommand(t.Context(), CreateOptions{
		Description: "print a generated command",
		TargetPath:  target,
		PrintOnly:   true,
		Completer:   fakeCompleter{response: response},
	})
	if err != nil {
		t.Fatalf("CreateCommand() print-only error = %v", err)
	}
	if result.CommandName != "hello generated" || !strings.Contains(result.CommandCUE, "hello generated") {
		t.Fatalf("unexpected print-only result: %#v", result)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after print-only: %v", err)
	}
	if string(data) != existing {
		t.Fatalf("print-only changed target:\n%s", string(data))
	}
}

func quoteForJSON(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(data)
}
