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

	raw, err := opts.Completer.Complete(ctx, BuildSystemPrompt(), BuildUserPrompt(description, targetPath, existing))
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

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

	if opts.DryRun {
		result.Diff = BuildUnifiedDiff(targetPath, existing, content, exists)
		return result, nil
	}
	if targetDir := filepath.Dir(targetPath); targetDir != "." {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return nil, fmt.Errorf("create target directory: %w", err)
		}
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
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
