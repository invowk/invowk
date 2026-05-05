// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

const (
	promptFormatText = "text"
	promptFormatJSON = "json"
)

type (
	// PromptDocument is the JSON representation of the command authoring prompt.
	PromptDocument struct {
		SystemPrompt string            `json:"system_prompt"`
		Schemas      map[string]string `json:"schemas"`
		Response     map[string]any    `json:"response_schema"`
	}
)

// BuildSystemPrompt returns the system prompt used for LLM command authoring.
func BuildSystemPrompt() string {
	return strings.TrimSpace(fmt.Sprintf(`You are helping a user create a custom Invowk command.

Invowk is a command runner configured with CUE. User commands live in invowkfile.cue under the top-level "cmds" list. Module metadata lives separately in invowkmod.cue.

Return only JSON with this exact shape:
{"command_cue":"{ name: \"...\", description: \"...\", implementations: [...] }","summary":"short human summary"}

Authoring rules:
- Generate exactly one #Command object in command_cue, not a full invowkfile and not a cmds array.
- Use "cmds" only when discussing the surrounding invowkfile; never put it inside command_cue.
- Every command needs at least one implementation with non-empty script, runtimes, and platforms.
- Prefer the virtual runtime for portable shell-like commands, but do not describe it as sandboxed or isolated. The virtual runtime is not a security sandbox and can still execute unknown commands from the host PATH.
- Use the container runtime when execution isolation is needed. Container examples must use debian:stable-slim unless a language-specific slim image is necessary.
- Use native runtime only for host-specific behavior or when an explicit host interpreter is required.
- Use flags for named options and args for positional inputs. Required args must come before optional args, and only the last arg may be variadic.
- Use depends_on for real prerequisites: tools, cmds, filepaths, capabilities, custom_checks, and env_vars.
- Command dependency visibility is static: a command may declare dependencies on the same invowkfile/module, global modules, or direct module dependencies only.
- Keep scripts concrete and runnable. Do not create external files; reference existing scripts only when the user explicitly asked for that.
- Do not include markdown fences, explanations, or comments outside the JSON object.

invowkfile.cue schema:
%s

invowkmod.cue schema:
%s`, invowkfile.SchemaCUE(), invowkmod.SchemaCUE()))
}

// BuildPromptDocument returns the structured prompt plus schema map.
func BuildPromptDocument() PromptDocument {
	return PromptDocument{
		SystemPrompt: BuildSystemPrompt(),
		Schemas: map[string]string{
			"invowkfile.cue": invowkfile.SchemaCUE(),
			"invowkmod.cue":  invowkmod.SchemaCUE(),
		},
		Response: GenerationResponseSchema(),
	}
}

// RenderPrompt renders the authoring prompt as text or JSON.
func RenderPrompt(format string) (string, error) {
	switch strings.ToLower(format) {
	case "", promptFormatText:
		return BuildSystemPrompt() + "\n", nil
	case promptFormatJSON:
		data, err := json.MarshalIndent(BuildPromptDocument(), "", "  ")
		if err != nil {
			return "", fmt.Errorf("encoding prompt JSON: %w", err)
		}
		return string(data) + "\n", nil
	default:
		return "", fmt.Errorf("unknown prompt format %q (must be \"text\" or \"json\")", format)
	}
}

// BuildUserPrompt returns the user prompt for a command creation request.
func BuildUserPrompt(description, targetPath, existing string) string {
	var b strings.Builder
	b.WriteString("Create one Invowk custom command for this user request:\n")
	b.WriteString(description)
	b.WriteString("\n\nTarget invowkfile path:\n")
	b.WriteString(targetPath)
	b.WriteString("\n\n")
	if strings.TrimSpace(existing) == "" {
		b.WriteString("The target invowkfile is missing or empty. Generate a command that can become the first entry in cmds.\n")
	} else {
		b.WriteString("Current target invowkfile content:\n```cue\n")
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	b.WriteString("\nReturn the JSON object only.")
	return b.String()
}

// BuildRepairPrompt returns a follow-up prompt that asks the model to correct
// an invalid generated response without changing the original task.
func BuildRepairPrompt(description, targetPath, existing, previousResponse string, failure error) string {
	var b strings.Builder
	b.WriteString(BuildUserPrompt(description, targetPath, existing))
	b.WriteString("\n\nThe previous response was rejected by Invowk validation.\n")
	b.WriteString("Validation error:\n")
	b.WriteString(failure.Error())
	b.WriteString("\n\nPrevious response:\n")
	b.WriteString(previousResponse)
	if !strings.HasSuffix(previousResponse, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("\nReturn corrected JSON only, preserving the original user request.")
	return b.String()
}

// GenerationResponseSchema returns the JSON schema for command-generation
// responses requested from structured-output providers.
func GenerationResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command_cue": map[string]any{
				"type":        "string",
				"description": "A single Invowk #Command object in CUE syntax, without a surrounding cmds list.",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "A short human summary of the generated command.",
			},
		},
		"required":             []string{"command_cue", "summary"},
		"additionalProperties": false,
	}
}
