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
	promptFormatText      = "text"
	promptFormatJSON      = "json"
	promptTargetCommand   = "cmd"
	promptTargetModule    = "mod"
	jsonSchemaType        = "type"
	jsonSchemaTypeString  = "string"
	jsonSchemaDescription = "description"
	// OperationCreate creates a new authoring target.
	OperationCreate AuthoringOperation = "create"
	// OperationChange changes an existing authoring target.
	OperationChange AuthoringOperation = "change"
	// OperationRemove removes an existing authoring target.
	OperationRemove AuthoringOperation = "remove"
)

type (
	// AuthoringOperation is an agent-authoring operation.
	AuthoringOperation string

	// PromptDocument is the JSON representation of one authoring prompt.
	PromptDocument struct {
		Target       string            `json:"target"`
		Operation    string            `json:"operation"`
		SystemPrompt string            `json:"system_prompt"`
		Schemas      map[string]string `json:"schemas"`
		Response     map[string]any    `json:"response_schema,omitempty"`
	}

	// PromptCatalog is the JSON representation returned when no operation is requested.
	PromptCatalog struct {
		Target     string           `json:"target"`
		Operations []PromptDocument `json:"operations"`
	}

	// CommandUserPromptOptions configures the per-request command prompt.
	CommandUserPromptOptions struct {
		Operation       AuthoringOperation
		Name            invowkfile.CommandName
		Description     string
		TargetPath      string
		Existing        string
		ExistingCommand string
	}

	// ModuleUserPromptOptions configures the per-request module prompt.
	ModuleUserPromptOptions struct {
		Operation     AuthoringOperation
		ModuleID      invowkmod.ModuleID
		Description   string
		ModulePath    string
		InvowkmodCUE  string
		InvowkfileCUE string
	}
)

// BuildCommandSystemPrompt returns the system prompt used for LLM command authoring.
func BuildCommandSystemPrompt(operation AuthoringOperation) string {
	if operation == OperationRemove {
		return strings.TrimSpace(`Invowk command removal is deterministic.

No LLM response is needed. The caller should remove exactly one matching command object from the top-level "cmds" list in invowkfile.cue, validate the resulting invowkfile, and make no other changes.`)
	}

	return strings.TrimSpace(fmt.Sprintf(`You are helping a user %s a custom Invowk command.

Invowk is a command runner configured with CUE. User commands live in invowkfile.cue under the top-level "cmds" list. Module metadata lives separately in invowkmod.cue.

Return only JSON with this exact shape:
{"command_cue":"{ name: \"...\", description: \"...\", implementations: [...] }","summary":"short human summary"}

Authoring rules:
- Generate exactly one #Command object in command_cue, not a full invowkfile and not a cmds array.
- The command name is owned by the caller. The generated name field must exactly match the requested command name.
- Use "cmds" only when discussing the surrounding invowkfile; never put it inside command_cue.
- Every command needs at least one implementation with non-empty script, runtimes, and platforms.
- Prefer the virtual-sh runtime for portable shell-like commands, but do not describe it as sandboxed or isolated. virtual-sh is not a security sandbox; host binaries are denied by default and run only when explicitly allowed with allowed_binaries.
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
%s`, operation, invowkfile.SchemaCUE(), invowkmod.SchemaCUE()))
}

// BuildModuleSystemPrompt returns the system prompt used for LLM module authoring.
func BuildModuleSystemPrompt(operation AuthoringOperation) string {
	if operation == OperationRemove {
		return strings.TrimSpace(`Invowk module removal is deterministic.

No LLM response is needed. The caller should validate the exact local module directory, require explicit confirmation for deletion, reject symlinked module paths, and remove only that module directory without editing dependencies or lock files.`)
	}

	return strings.TrimSpace(fmt.Sprintf(`You are helping a user %s a local Invowk module.

Invowk modules are local directories named "<module-id>.invowkmod" containing invowkmod.cue metadata and invowkfile.cue command definitions.

Return only JSON with this exact shape:
{"invowkmod_cue":"module: \"...\"\nversion: \"1.0.0\"\ndescription: \"...\"","invowkfile_cue":"cmds: [...]","summary":"short human summary"}

Authoring rules:
- The module identity is owned by the caller. The invowkmod.cue module field must exactly match the requested module ID.
- Generate only invowkmod.cue and invowkfile.cue content. Do not request or describe arbitrary extra file writes.
- For change operations, update only those two files and keep the existing module ID unchanged.
- invowkmod.cue must satisfy the #Invowkmod schema and should include a valid semantic version.
- invowkfile.cue must satisfy the #Invowkfile schema. It may contain zero or more commands under the top-level cmds list.
- Prefer the virtual-sh runtime for portable shell-like commands, but do not describe it as sandboxed or isolated. virtual-sh is not a security sandbox; host binaries are denied by default and run only when explicitly allowed with allowed_binaries.
- Use the container runtime when execution isolation is needed. Container examples must use debian:stable-slim unless a language-specific slim image is necessary.
- Keep scripts inline and concrete. Do not create external script files; reference existing scripts only when the user explicitly asked for that.
- Do not include markdown fences, explanations, or comments outside the JSON object.

invowkfile.cue schema:
%s

invowkmod.cue schema:
%s`, operation, invowkfile.SchemaCUE(), invowkmod.SchemaCUE()))
}

// BuildPromptDocument returns the structured prompt plus schema map.
func BuildPromptDocument() PromptDocument {
	return BuildCommandPromptDocument(OperationCreate)
}

// BuildCommandPromptDocument returns the structured prompt for one command operation.
func BuildCommandPromptDocument(operation AuthoringOperation) PromptDocument {
	doc := PromptDocument{
		Target:       promptTargetCommand,
		Operation:    operation.String(),
		SystemPrompt: BuildCommandSystemPrompt(operation),
		Schemas:      promptSchemas(),
	}
	if operation != OperationRemove {
		doc.Response = GenerationResponseSchema()
	}
	return doc
}

// BuildModulePromptDocument returns the structured prompt for one module operation.
func BuildModulePromptDocument(operation AuthoringOperation) PromptDocument {
	doc := PromptDocument{
		Target:       promptTargetModule,
		Operation:    operation.String(),
		SystemPrompt: BuildModuleSystemPrompt(operation),
		Schemas:      promptSchemas(),
	}
	if operation != OperationRemove {
		doc.Response = ModuleGenerationResponseSchema()
	}
	return doc
}

// RenderPrompt renders the authoring prompt as text or JSON.
func RenderPrompt(format string) (string, error) {
	return RenderCommandPrompt(format, OperationCreate.String())
}

// RenderCommandPrompt renders command authoring prompts as text or JSON.
func RenderCommandPrompt(format, operation string) (string, error) {
	return renderPrompt(format, promptTargetCommand, operation, BuildCommandPromptDocument)
}

// RenderModulePrompt renders module authoring prompts as text or JSON.
func RenderModulePrompt(format, operation string) (string, error) {
	return renderPrompt(format, promptTargetModule, operation, BuildModulePromptDocument)
}

func renderPrompt(
	format string,
	target string,
	operation string,
	build func(AuthoringOperation) PromptDocument,
) (string, error) {
	ops, err := promptOperations(operation)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(format) {
	case "", promptFormatText:
		return renderPromptText(ops, build), nil
	case promptFormatJSON:
		var value any
		if len(ops) == 1 {
			value = build(ops[0])
		} else {
			docs := make([]PromptDocument, 0, len(ops))
			for _, op := range ops {
				docs = append(docs, build(op))
			}
			value = PromptCatalog{Target: target, Operations: docs}
		}
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return "", fmt.Errorf("encoding prompt JSON: %w", err)
		}
		return string(data) + "\n", nil
	default:
		return "", fmt.Errorf("unknown prompt format %q (must be \"text\" or \"json\")", format)
	}
}

// BuildSystemPrompt returns the legacy default command-create system prompt.
func BuildSystemPrompt() string {
	return BuildCommandSystemPrompt(OperationCreate)
}

// BuildUserPrompt returns the legacy default command-create user prompt.
func BuildUserPrompt(description, targetPath, existing string) string {
	return BuildCommandUserPrompt(CommandUserPromptOptions{
		Operation:   OperationCreate,
		Name:        invowkfile.CommandName("requested command"),
		Description: description,
		TargetPath:  targetPath,
		Existing:    existing,
	})
}

// BuildCommandUserPrompt returns the user prompt for a command authoring request.
func BuildCommandUserPrompt(opts CommandUserPromptOptions) string {
	var b strings.Builder
	b.WriteString(commandPromptVerb(opts.Operation))
	b.WriteString(" one Invowk custom command.\n\nRequested command name:\n")
	b.WriteString(opts.Name.String())
	b.WriteString("\n\nUser request:\n")
	b.WriteString(opts.Description)
	b.WriteString("\n\nTarget invowkfile path:\n")
	b.WriteString(opts.TargetPath)
	b.WriteString("\n\n")
	if opts.Operation == OperationChange && strings.TrimSpace(opts.ExistingCommand) != "" {
		b.WriteString("Current command to change:\n```cue\n")
		b.WriteString(opts.ExistingCommand)
		if !strings.HasSuffix(opts.ExistingCommand, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}
	if strings.TrimSpace(opts.Existing) == "" {
		b.WriteString("The target invowkfile is missing or empty. Generate a command that can become the first entry in cmds.\n")
	} else {
		b.WriteString("Current target invowkfile content:\n```cue\n")
		b.WriteString(opts.Existing)
		if !strings.HasSuffix(opts.Existing, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	b.WriteString("\nReturn the JSON object only. The command_cue name must exactly match the requested command name.")
	return b.String()
}

// BuildModuleUserPrompt returns the user prompt for a module authoring request.
func BuildModuleUserPrompt(opts ModuleUserPromptOptions) string {
	var b strings.Builder
	b.WriteString(commandPromptVerb(opts.Operation))
	b.WriteString(" one local Invowk module.\n\nRequested module ID:\n")
	b.WriteString(opts.ModuleID.String())
	b.WriteString("\n\nUser request:\n")
	b.WriteString(opts.Description)
	b.WriteString("\n\nTarget module path:\n")
	b.WriteString(opts.ModulePath)
	b.WriteString("\n\n")
	if opts.Operation == OperationChange {
		b.WriteString("Current invowkmod.cue:\n```cue\n")
		b.WriteString(opts.InvowkmodCUE)
		if !strings.HasSuffix(opts.InvowkmodCUE, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\nCurrent invowkfile.cue:\n```cue\n")
		b.WriteString(opts.InvowkfileCUE)
		if !strings.HasSuffix(opts.InvowkfileCUE, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}
	b.WriteString("Return the JSON object only. The invowkmod_cue module field must exactly match the requested module ID.")
	return b.String()
}

// BuildRepairPrompt returns the legacy default command-create repair prompt.
func BuildRepairPrompt(description, targetPath, existing, previousResponse string, failure error) string {
	return BuildCommandRepairPrompt(CommandUserPromptOptions{
		Operation:   OperationCreate,
		Name:        invowkfile.CommandName("requested command"),
		Description: description,
		TargetPath:  targetPath,
		Existing:    existing,
	}, previousResponse, failure)
}

// BuildCommandRepairPrompt asks the model to correct an invalid command response.
func BuildCommandRepairPrompt(opts CommandUserPromptOptions, previousResponse string, failure error) string {
	var b strings.Builder
	b.WriteString(BuildCommandUserPrompt(opts))
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

// BuildModuleRepairPrompt asks the model to correct an invalid module response.
func BuildModuleRepairPrompt(opts ModuleUserPromptOptions, previousResponse string, failure error) string {
	var b strings.Builder
	b.WriteString(BuildModuleUserPrompt(opts))
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
		jsonSchemaType: "object",
		"properties": map[string]any{
			"command_cue": map[string]any{
				jsonSchemaType:        jsonSchemaTypeString,
				jsonSchemaDescription: "A single Invowk #Command object in CUE syntax, without a surrounding cmds list.",
			},
			"summary": map[string]any{
				jsonSchemaType:        jsonSchemaTypeString,
				jsonSchemaDescription: "A short human summary of the generated command.",
			},
		},
		"required":             []string{"command_cue", "summary"},
		"additionalProperties": false,
	}
}

// String returns the operation text.
func (op AuthoringOperation) String() string {
	return string(op)
}

func promptOperations(operation string) ([]AuthoringOperation, error) {
	switch AuthoringOperation(strings.ToLower(strings.TrimSpace(operation))) {
	case "":
		return []AuthoringOperation{OperationCreate, OperationChange, OperationRemove}, nil
	case OperationCreate:
		return []AuthoringOperation{OperationCreate}, nil
	case OperationChange:
		return []AuthoringOperation{OperationChange}, nil
	case OperationRemove:
		return []AuthoringOperation{OperationRemove}, nil
	default:
		return nil, fmt.Errorf("unknown prompt operation %q (must be \"create\", \"change\", or \"remove\")", operation)
	}
}

func renderPromptText(ops []AuthoringOperation, build func(AuthoringOperation) PromptDocument) string {
	if len(ops) == 1 {
		return build(ops[0]).SystemPrompt + "\n"
	}

	var b strings.Builder
	for i, op := range ops {
		if i > 0 {
			b.WriteByte('\n')
		}
		doc := build(op)
		b.WriteString("# ")
		b.WriteString(doc.Target)
		b.WriteByte(' ')
		b.WriteString(doc.Operation)
		b.WriteString("\n\n")
		b.WriteString(doc.SystemPrompt)
		b.WriteByte('\n')
	}
	return b.String()
}

func promptSchemas() map[string]string {
	return map[string]string{
		"invowkfile.cue": invowkfile.SchemaCUE(),
		"invowkmod.cue":  invowkmod.SchemaCUE(),
	}
}

func commandPromptVerb(operation AuthoringOperation) string {
	switch operation {
	case OperationCreate:
		return "Create"
	case OperationChange:
		return "Change"
	case OperationRemove:
		return "Remove"
	default:
		return "Create"
	}
}
