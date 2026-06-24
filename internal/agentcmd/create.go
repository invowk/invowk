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
		Name        invowkfile.CommandName
		Description string
		TargetPath  string
		FromFile    string
		DryRun      bool
		PrintOnly   bool
		Completer   llm.Completer
		// RepairAttempts is the number of validation-feedback retries after the
		// initial generation attempt. Zero uses the default bounded retry count.
		RepairAttempts int
	}

	// ChangeOptions configures LLM-assisted command changes.
	ChangeOptions struct {
		Name        invowkfile.CommandName
		Description string
		TargetPath  string
		FromFile    string
		DryRun      bool
		PrintOnly   bool
		Completer   llm.Completer
		// RepairAttempts is the number of validation-feedback retries after the
		// initial generation attempt. Zero uses the default bounded retry count.
		RepairAttempts int
	}

	// RemoveOptions configures deterministic command removal.
	RemoveOptions struct {
		Name       invowkfile.CommandName
		TargetPath string
		DryRun     bool
	}

	// CommandResult contains the validated command and resulting file content.
	CommandResult struct {
		Operation   AuthoringOperation
		CommandName invowkfile.CommandName
		CommandCUE  string
		TargetPath  string
		Summary     string
		Content     string
		Diff        string
		Changed     bool
	}

	// CreateResult contains the validated generated command and resulting file content.
	CreateResult = CommandResult

	// ChangeResult contains the validated changed command and resulting file content.
	ChangeResult = CommandResult

	// RemoveResult contains the deterministic removal result and resulting file content.
	RemoveResult = CommandResult

	commandGenerationOptions struct {
		Operation       AuthoringOperation
		Name            invowkfile.CommandName
		Description     string
		TargetPath      string
		Existing        string
		ExistingCommand string
		TargetExists    bool
		PrintOnly       bool
		Completer       llm.Completer
		RepairAttempts  int
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

	description, err := loadDescription(opts.Description, opts.FromFile)
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

	if _, found, findErr := FindCommandCUE(existing, targetPath, opts.Name); findErr != nil {
		return nil, findErr
	} else if found {
		return nil, fmt.Errorf("command %q already exists; use `invowk agent cmd change %s` to modify it", opts.Name, opts.Name)
	}

	result, err := commandGenerationOptions{
		Operation:      OperationCreate,
		Name:           opts.Name,
		Description:    description,
		TargetPath:     targetPath,
		Existing:       existing,
		TargetExists:   exists,
		PrintOnly:      opts.PrintOnly,
		Completer:      opts.Completer,
		RepairAttempts: opts.RepairAttempts,
	}.generateValidCommand(ctx)
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

// ChangeCommand asks an LLM to update one existing command and optionally patches the target file.
func ChangeCommand(ctx context.Context, opts ChangeOptions) (*ChangeResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	description, err := loadDescription(opts.Description, opts.FromFile)
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
	if !exists {
		return nil, fmt.Errorf("command %q does not exist in %s; use `invowk agent cmd create %s` to add it", opts.Name, targetPath, opts.Name)
	}

	existingCommand, found, err := FindCommandCUE(existing, targetPath, opts.Name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("command %q does not exist in %s; use `invowk agent cmd create %s` to add it", opts.Name, targetPath, opts.Name)
	}

	result, err := commandGenerationOptions{
		Operation:       OperationChange,
		Name:            opts.Name,
		Description:     description,
		TargetPath:      targetPath,
		Existing:        existing,
		ExistingCommand: existingCommand,
		TargetExists:    exists,
		PrintOnly:       opts.PrintOnly,
		Completer:       opts.Completer,
		RepairAttempts:  opts.RepairAttempts,
	}.generateValidCommand(ctx)
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
	if err := os.WriteFile(targetPath, []byte(result.Content), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", targetPath, err)
	}

	return result, nil
}

// RemoveCommand removes one command from the target invowkfile without invoking an LLM.
func RemoveCommand(ctx context.Context, opts RemoveOptions) (*RemoveResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("remove command: %w", err)
	}

	targetPath := opts.TargetPath
	if targetPath == "" {
		targetPath = defaultInvowkfileName
	}

	existing, exists, err := readTarget(targetPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("command %q does not exist in %s", opts.Name, targetPath)
	}

	removedCommand, content, err := RemoveCommandFromInvowkfile(existing, opts.Name, targetPath)
	if err != nil {
		return nil, err
	}
	result := &CommandResult{
		Operation:   OperationRemove,
		CommandName: opts.Name,
		CommandCUE:  removedCommand,
		TargetPath:  targetPath,
		Summary:     fmt.Sprintf("Removed command %q.", opts.Name),
		Content:     content,
		Changed:     content != existing,
	}
	if opts.DryRun {
		result.Diff = BuildUnifiedDiff(targetPath, existing, content, exists)
		return result, nil
	}
	if content == "" {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove empty %s: %w", targetPath, err)
		}
	} else if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return result, nil
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts CreateOptions) Validate() error {
	if err := opts.Name.Validate(); err != nil {
		return err
	}
	if opts.Completer == nil {
		return errors.New("LLM completer is required")
	}
	return validateDescriptionInput(opts.Description, opts.FromFile)
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts ChangeOptions) Validate() error {
	if err := opts.Name.Validate(); err != nil {
		return err
	}
	if opts.Completer == nil {
		return errors.New("LLM completer is required")
	}
	return validateDescriptionInput(opts.Description, opts.FromFile)
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts RemoveOptions) Validate() error {
	return opts.Name.Validate()
}

func validateDescriptionInput(description, fromFile string) error {
	if strings.TrimSpace(description) == "" && fromFile == "" {
		return errors.New("command description is required")
	}
	if strings.TrimSpace(description) != "" && fromFile != "" {
		return errors.New("description arguments and --from-file are mutually exclusive")
	}
	return nil
}

// LoadDescription returns the inline or file-backed command description.
func (opts CreateOptions) LoadDescription() (string, error) {
	return loadDescription(opts.Description, opts.FromFile)
}

// LoadDescription returns the inline or file-backed command description.
func (opts ChangeOptions) LoadDescription() (string, error) {
	return loadDescription(opts.Description, opts.FromFile)
}

func loadDescription(description, fromFile string) (string, error) {
	if fromFile == "" {
		return strings.TrimSpace(description), nil
	}
	data, err := os.ReadFile(fromFile)
	if err != nil {
		return "", fmt.Errorf("read description file: %w", err)
	}
	loaded := strings.TrimSpace(string(data))
	if loaded == "" {
		return "", errors.New("description file is empty")
	}
	return loaded, nil
}

func (opts commandGenerationOptions) generateValidCommand(ctx context.Context) (*CommandResult, error) {
	systemPrompt := BuildCommandSystemPrompt(opts.Operation)
	userPrompt := BuildCommandUserPrompt(CommandUserPromptOptions{
		Operation:       opts.Operation,
		Name:            opts.Name,
		Description:     opts.Description,
		TargetPath:      opts.TargetPath,
		Existing:        opts.Existing,
		ExistingCommand: opts.ExistingCommand,
	})
	attempts := opts.maxGenerationAttempts()
	var previousResponse string
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			userPrompt = BuildCommandRepairPrompt(CommandUserPromptOptions{
				Operation:       opts.Operation,
				Name:            opts.Name,
				Description:     opts.Description,
				TargetPath:      opts.TargetPath,
				Existing:        opts.Existing,
				ExistingCommand: opts.ExistingCommand,
			}, previousResponse, lastErr)
		}

		raw, err := opts.complete(ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("LLM completion failed: %w", err)
		}
		previousResponse = raw

		result, err := opts.resultFromResponse(raw)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("generated command invalid after %d attempt(s): %w", attempts, lastErr)
}

func (opts commandGenerationOptions) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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

func (opts commandGenerationOptions) resultFromResponse(raw string) (*CommandResult, error) {
	resp, err := ParseGenerationResponse(raw)
	if err != nil {
		return nil, err
	}

	command, commandCUE, err := ValidateCommandCUE(resp.CommandCUE)
	if err != nil {
		return nil, err
	}
	if command.Name != opts.Name {
		return nil, fmt.Errorf("generated command name %q does not match requested name %q", command.Name, opts.Name)
	}

	result := &CommandResult{
		Operation:   opts.Operation,
		CommandName: command.Name,
		CommandCUE:  commandCUE,
		TargetPath:  opts.TargetPath,
		Summary:     resp.Summary,
	}
	if opts.PrintOnly {
		return result, nil
	}

	replace := opts.Operation == OperationChange
	content, err := PatchInvowkfile(opts.Existing, opts.TargetExists, commandCUE, command.Name, replace, opts.TargetPath)
	if err != nil {
		return nil, err
	}
	result.Content = content
	result.Changed = content != opts.Existing
	return result, nil
}

func (opts commandGenerationOptions) maxGenerationAttempts() int {
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
