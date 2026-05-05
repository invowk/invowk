// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/invowk/invowk/internal/llm"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	defaultInvowkfileName = "invowkfile.cue"
	generatedCommandPath  = "generated-command.cue"
	defaultRepairAttempts = 1
)

var jsonFencePattern = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.+?)\\n\\s*```")

type (
	// CreateOptions configures LLM-assisted command creation.
	CreateOptions struct {
		Description string
		TargetPath  string
		FromFile    string
		DryRun      bool
		PrintOnly   bool
		Replace     bool
		Completer   llm.Completer
		// RepairAttempts is the number of validation-feedback retries after the
		// initial generation attempt. Zero uses the default bounded retry count.
		RepairAttempts int
	}

	// CreateResult contains the validated generated command and resulting file content.
	CreateResult struct {
		CommandName invowkfile.CommandName
		CommandCUE  string
		TargetPath  string
		Summary     string
		Content     string
		Diff        string
		Changed     bool
	}

	generationResponse struct {
		CommandCUE string `json:"command_cue"`
		Summary    string `json:"summary"`
	}
)

// CreateCommand asks an LLM for one command, validates it, and optionally patches the target file.
func CreateCommand(ctx context.Context, opts CreateOptions) (*CreateResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	description, err := opts.LoadDescription()
	if err != nil {
		return nil, err
	}

	targetPath := opts.TargetPath
	if targetPath == "" {
		targetPath = defaultInvowkfileName
	}

	existing, exists, err := readTarget(targetPath)
	if err != nil {
		return nil, err
	}

	result, err := opts.generateValidCommand(ctx, description, targetPath, existing, exists)
	if err != nil {
		return nil, err
	}

	if opts.PrintOnly {
		return result, nil
	}
	if opts.DryRun {
		result.Diff = BuildUnifiedDiff(targetPath, existing, result.Content, exists)
		return result, nil
	}
	if targetDir := filepath.Dir(targetPath); targetDir != "." {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return nil, fmt.Errorf("create target directory: %w", err)
		}
	}
	if err := os.WriteFile(targetPath, []byte(result.Content), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", targetPath, err)
	}

	return result, nil
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts CreateOptions) Validate() error {
	if opts.Completer == nil {
		return errors.New("LLM completer is required")
	}
	if strings.TrimSpace(opts.Description) == "" && opts.FromFile == "" {
		return errors.New("command description is required")
	}
	if strings.TrimSpace(opts.Description) != "" && opts.FromFile != "" {
		return errors.New("description arguments and --from-file are mutually exclusive")
	}
	if opts.DryRun && opts.PrintOnly {
		return errors.New("--dry-run and --print are mutually exclusive")
	}
	return nil
}

// LoadDescription returns the inline or file-backed command description.
func (opts CreateOptions) LoadDescription() (string, error) {
	if opts.FromFile == "" {
		return strings.TrimSpace(opts.Description), nil
	}
	data, err := os.ReadFile(opts.FromFile)
	if err != nil {
		return "", fmt.Errorf("read description file: %w", err)
	}
	description := strings.TrimSpace(string(data))
	if description == "" {
		return "", errors.New("description file is empty")
	}
	return description, nil
}

func (opts CreateOptions) generateValidCommand(ctx context.Context, description, targetPath, existing string, exists bool) (*CreateResult, error) {
	systemPrompt := BuildSystemPrompt()
	userPrompt := BuildUserPrompt(description, targetPath, existing)
	attempts := opts.maxGenerationAttempts()
	var previousResponse string
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			userPrompt = BuildRepairPrompt(description, targetPath, existing, previousResponse, lastErr)
		}

		raw, err := opts.complete(ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("LLM completion failed: %w", err)
		}
		previousResponse = raw

		result, err := opts.resultFromResponse(raw, targetPath, existing, exists)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("generated command invalid after %d attempt(s): %w", attempts, lastErr)
}

func (opts CreateOptions) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if structured, ok := opts.Completer.(llm.StructuredCompleter); ok {
		raw, err := structured.CompleteJSONSchema(ctx, systemPrompt, userPrompt, llm.JSONSchemaFormat{
			Name:        "invowk_command_generation",
			Description: "Generated Invowk command object and summary.",
			Schema:      GenerationResponseSchema(),
			Strict:      true,
		})
		if err == nil {
			return raw, nil
		}
		if !errors.Is(err, llm.ErrStructuredOutputUnsupported) {
			return "", err
		}
	}
	return opts.Completer.Complete(ctx, systemPrompt, userPrompt)
}

func (opts CreateOptions) resultFromResponse(raw, targetPath, existing string, exists bool) (*CreateResult, error) {
	resp, err := ParseGenerationResponse(raw)
	if err != nil {
		return nil, err
	}

	command, commandCUE, err := ValidateCommandCUE(resp.CommandCUE)
	if err != nil {
		return nil, err
	}

	result := &CreateResult{
		CommandName: command.Name,
		CommandCUE:  commandCUE,
		TargetPath:  targetPath,
		Summary:     resp.Summary,
	}
	if opts.PrintOnly {
		return result, nil
	}

	content, err := PatchInvowkfile(existing, exists, commandCUE, command.Name, opts.Replace, targetPath)
	if err != nil {
		return nil, err
	}
	result.Content = content
	result.Changed = content != existing
	return result, nil
}

func (opts CreateOptions) maxGenerationAttempts() int {
	repairAttempts := opts.RepairAttempts
	if repairAttempts == 0 {
		repairAttempts = defaultRepairAttempts
	}
	if repairAttempts < 0 {
		repairAttempts = 0
	}
	return repairAttempts + 1
}

// ParseGenerationResponse parses the model's JSON-only response.
func ParseGenerationResponse(raw string) (generationResponse, error) {
	var resp generationResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &resp); err == nil {
		return validateGenerationResponse(resp)
	}

	matches := jsonFencePattern.FindStringSubmatch(raw)
	if len(matches) >= 2 {
		if err := json.Unmarshal([]byte(matches[1]), &resp); err == nil {
			return validateGenerationResponse(resp)
		}
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &resp); err == nil {
			return validateGenerationResponse(resp)
		}
	}

	return generationResponse{}, errors.New("could not extract generated command JSON")
}

// ValidateCommandCUE validates that commandCUE is exactly one command object.
func ValidateCommandCUE(commandCUE string) (invowkfile.Command, string, error) {
	commandCUE = strings.TrimSpace(commandCUE)
	if commandCUE == "" {
		return invowkfile.Command{}, "", errors.New("command_cue is empty")
	}
	if strings.Contains(commandCUE, "cmds:") {
		return invowkfile.Command{}, "", errors.New("command_cue must be one command object, not a cmds list")
	}

	formatted, err := formatCommandObject(commandCUE)
	if err != nil {
		return invowkfile.Command{}, "", err
	}

	wrapped := wrapCommandObject(formatted)
	inv, err := invowkfile.ParseBytes([]byte(wrapped), generatedCommandPath)
	if err != nil {
		return invowkfile.Command{}, "", fmt.Errorf("validate generated command: %w", err)
	}
	if len(inv.Commands) != 1 {
		return invowkfile.Command{}, "", fmt.Errorf("generated %d commands; expected exactly 1", len(inv.Commands))
	}
	return inv.Commands[0], formatted, nil
}

func validateGenerationResponse(resp generationResponse) (generationResponse, error) {
	if strings.TrimSpace(resp.CommandCUE) == "" {
		return generationResponse{}, errors.New("LLM response missing command_cue")
	}
	return resp, nil
}

func readTarget(path string) (content string, exists bool, err error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), true, nil
	}
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("read %s: %w", path, err)
}
