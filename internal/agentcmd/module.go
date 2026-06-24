// SPDX-License-Identifier: MPL-2.0

package agentcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cueformat "cuelang.org/go/cue/format"

	"github.com/invowk/invowk/internal/llm"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	defaultInvowkfileContent = "cmds: []\n"
	generatedModuleName      = "generated-module"
)

type (
	// ModuleCreateOptions configures LLM-assisted module creation.
	ModuleCreateOptions struct {
		ModuleID         invowkmod.ModuleID
		Description      string
		FromFile         string
		DryRun           bool
		PrintOnly        bool
		Verify           bool
		CreateScriptsDir bool
		Completer        llm.Completer
		// RepairAttempts is the number of validation-feedback retries after the
		// initial generation attempt. Zero uses the default bounded retry count.
		RepairAttempts int
	}

	// ModuleChangeOptions configures LLM-assisted module changes.
	ModuleChangeOptions struct {
		Target         string
		Description    string
		FromFile       string
		DryRun         bool
		PrintOnly      bool
		Verify         bool
		Completer      llm.Completer
		RepairAttempts int
	}

	// ModuleRemoveOptions configures deterministic module removal.
	ModuleRemoveOptions struct {
		Target string
		DryRun bool
		Force  bool
	}

	// ModuleResult contains a generated module file bundle and resulting plan.
	ModuleResult struct {
		Operation     AuthoringOperation
		ModuleID      invowkmod.ModuleID
		ModulePath    string
		InvowkmodCUE  string
		InvowkfileCUE string
		Summary       string
		Diff          string
		Changed       bool
		Verified      bool
	}

	moduleGenerationOptions struct {
		Operation      AuthoringOperation
		ModuleID       invowkmod.ModuleID
		Description    string
		ModulePath     string
		InvowkmodCUE   string
		InvowkfileCUE  string
		PrintOnly      bool
		Completer      llm.Completer
		RepairAttempts int
	}

	moduleGenerationResponse struct {
		InvowkmodCUE  string `json:"invowkmod_cue"`
		InvowkfileCUE string `json:"invowkfile_cue"`
		Summary       string `json:"summary"`
	}

	loadedModule struct {
		Path          string
		ModuleID      invowkmod.ModuleID
		InvowkmodCUE  string
		InvowkfileCUE string
	}
)

// CreateModule asks an LLM for a local module bundle and optionally writes it.
func CreateModule(ctx context.Context, opts ModuleCreateOptions) (*ModuleResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	description, err := loadModuleDescription(opts.Description, opts.FromFile)
	if err != nil {
		return nil, err
	}

	modulePath, err := modulePathForID(opts.ModuleID)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Lstat(modulePath); statErr == nil {
		return nil, fmt.Errorf("module %q already exists at %s; use `invowk agent mod change %s` to modify it", opts.ModuleID, modulePath, opts.ModuleID)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("check module path %s: %w", modulePath, statErr)
	}

	result, err := moduleGenerationOptions{
		Operation:      OperationCreate,
		ModuleID:       opts.ModuleID,
		Description:    description,
		ModulePath:     modulePath,
		Completer:      opts.Completer,
		PrintOnly:      opts.PrintOnly,
		RepairAttempts: opts.RepairAttempts,
	}.generateValidModule(ctx)
	if err != nil {
		return nil, err
	}

	if opts.PrintOnly {
		return result, nil
	}
	if opts.DryRun {
		result.Diff = buildModuleDiff(modulePath, "", result.InvowkmodCUE, "", result.InvowkfileCUE, false)
		return result, nil
	}
	if err := writeModuleBundle(modulePath, result.InvowkmodCUE, result.InvowkfileCUE, opts.CreateScriptsDir); err != nil {
		return nil, err
	}
	result.Changed = true
	if opts.Verify {
		if err := verifyModule(modulePath); err != nil {
			return nil, err
		}
		result.Verified = true
	}
	return result, nil
}

// ChangeModule asks an LLM to update invowkmod.cue and invowkfile.cue for an existing module.
func ChangeModule(ctx context.Context, opts ModuleChangeOptions) (*ModuleResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	description, err := loadModuleDescription(opts.Description, opts.FromFile)
	if err != nil {
		return nil, err
	}

	current, err := loadExistingModule(opts.Target)
	if err != nil {
		return nil, err
	}

	result, err := moduleGenerationOptions{
		Operation:      OperationChange,
		ModuleID:       current.ModuleID,
		Description:    description,
		ModulePath:     current.Path,
		InvowkmodCUE:   current.InvowkmodCUE,
		InvowkfileCUE:  current.InvowkfileCUE,
		Completer:      opts.Completer,
		PrintOnly:      opts.PrintOnly,
		RepairAttempts: opts.RepairAttempts,
	}.generateValidModule(ctx)
	if err != nil {
		return nil, err
	}

	if opts.PrintOnly {
		return result, nil
	}
	if opts.DryRun {
		result.Diff = buildModuleDiff(current.Path, current.InvowkmodCUE, result.InvowkmodCUE, current.InvowkfileCUE, result.InvowkfileCUE, true)
		return result, nil
	}
	if err := writeModuleFiles(current.Path, result.InvowkmodCUE, result.InvowkfileCUE); err != nil {
		return nil, err
	}
	result.Changed = current.InvowkmodCUE != result.InvowkmodCUE || current.InvowkfileCUE != result.InvowkfileCUE
	if opts.Verify {
		if err := verifyModule(current.Path); err != nil {
			return nil, err
		}
		result.Verified = true
	}
	return result, nil
}

// RemoveModule removes one validated local module directory without invoking an LLM.
func RemoveModule(ctx context.Context, opts ModuleRemoveOptions) (*ModuleResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("remove module: %w", err)
	}

	current, err := loadExistingModule(opts.Target)
	if err != nil {
		return nil, err
	}
	if !opts.DryRun && !opts.Force {
		return nil, errors.New("agent mod remove requires --force to delete a module directory")
	}

	result := &ModuleResult{
		Operation:     OperationRemove,
		ModuleID:      current.ModuleID,
		ModulePath:    current.Path,
		InvowkmodCUE:  current.InvowkmodCUE,
		InvowkfileCUE: current.InvowkfileCUE,
		Summary:       fmt.Sprintf("Removed module %q.", current.ModuleID),
		Diff:          buildModuleRemovePlan(current.Path),
		Changed:       !opts.DryRun,
	}
	if opts.DryRun {
		return result, nil
	}
	if err := os.RemoveAll(current.Path); err != nil {
		return nil, fmt.Errorf("remove module %s: %w", current.Path, err)
	}
	return result, nil
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts ModuleCreateOptions) Validate() error {
	if err := opts.ModuleID.Validate(); err != nil {
		return err
	}
	if opts.Completer == nil {
		return errors.New("LLM completer is required")
	}
	return validateModuleDescriptionInput(opts.Description, opts.FromFile)
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts ModuleChangeOptions) Validate() error {
	if strings.TrimSpace(opts.Target) == "" {
		return errors.New("module target is required")
	}
	if opts.Completer == nil {
		return errors.New("LLM completer is required")
	}
	return validateModuleDescriptionInput(opts.Description, opts.FromFile)
}

// Validate verifies option invariants that cannot be represented by Cobra alone.
func (opts ModuleRemoveOptions) Validate() error {
	if strings.TrimSpace(opts.Target) == "" {
		return errors.New("module target is required")
	}
	return nil
}

// PrintJSON renders the generated module bundle for --print output.
func (result ModuleResult) PrintJSON() (string, error) {
	payload := struct {
		ModuleID string            `json:"module_id"`
		Files    map[string]string `json:"files"`
		Summary  string            `json:"summary"`
	}{
		ModuleID: result.ModuleID.String(),
		Files: map[string]string{
			"invowkmod.cue":  result.InvowkmodCUE,
			"invowkfile.cue": result.InvowkfileCUE,
		},
		Summary: result.Summary,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode module print JSON: %w", err)
	}
	return string(data) + "\n", nil
}

func (opts moduleGenerationOptions) generateValidModule(ctx context.Context) (*ModuleResult, error) {
	systemPrompt := BuildModuleSystemPrompt(opts.Operation)
	promptOpts := ModuleUserPromptOptions{
		Operation:     opts.Operation,
		ModuleID:      opts.ModuleID,
		Description:   opts.Description,
		ModulePath:    opts.ModulePath,
		InvowkmodCUE:  opts.InvowkmodCUE,
		InvowkfileCUE: opts.InvowkfileCUE,
	}
	userPrompt := BuildModuleUserPrompt(promptOpts)
	attempts := opts.maxGenerationAttempts()
	var previousResponse string
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			userPrompt = BuildModuleRepairPrompt(promptOpts, previousResponse, lastErr)
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

	return nil, fmt.Errorf("generated module invalid after %d attempt(s): %w", attempts, lastErr)
}

func (opts moduleGenerationOptions) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if structured, ok := opts.Completer.(llm.StructuredCompleter); ok {
		raw, err := structured.CompleteJSONSchema(ctx, systemPrompt, userPrompt, llm.JSONSchemaFormat{
			Name:        "invowk_module_generation",
			Description: "Generated Invowk module files and summary.",
			Schema:      ModuleGenerationResponseSchema(),
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

func (opts moduleGenerationOptions) resultFromResponse(raw string) (*ModuleResult, error) {
	resp, err := ParseModuleGenerationResponse(raw)
	if err != nil {
		return nil, err
	}

	invowkmodCUE, invowkfileCUE, err := ValidateModuleBundle(opts.ModuleID, opts.ModulePath, resp.InvowkmodCUE, resp.InvowkfileCUE)
	if err != nil {
		return nil, err
	}

	return &ModuleResult{
		Operation:     opts.Operation,
		ModuleID:      opts.ModuleID,
		ModulePath:    opts.ModulePath,
		InvowkmodCUE:  invowkmodCUE,
		InvowkfileCUE: invowkfileCUE,
		Summary:       resp.Summary,
	}, nil
}

func (opts moduleGenerationOptions) maxGenerationAttempts() int {
	repairAttempts := opts.RepairAttempts
	if repairAttempts == 0 {
		repairAttempts = defaultRepairAttempts
	}
	if repairAttempts < 0 {
		repairAttempts = 0
	}
	return repairAttempts + 1
}

// ParseModuleGenerationResponse parses the model's JSON-only module response.
func ParseModuleGenerationResponse(raw string) (moduleGenerationResponse, error) {
	var resp moduleGenerationResponse
	if err := decodeStrictJSON(strings.TrimSpace(raw), &resp); err == nil {
		return validateModuleGenerationResponse(resp)
	}

	matches := jsonFencePattern.FindStringSubmatch(raw)
	if len(matches) >= 2 {
		if err := decodeStrictJSON(matches[1], &resp); err == nil {
			return validateModuleGenerationResponse(resp)
		}
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := decodeStrictJSON(raw[start:end+1], &resp); err == nil {
			return validateModuleGenerationResponse(resp)
		}
	}

	return moduleGenerationResponse{}, errors.New("could not extract generated module JSON")
}

// ValidateModuleBundle validates and formats a generated module two-file bundle.
func ValidateModuleBundle(moduleID invowkmod.ModuleID, modulePath, invowkmodCUE, invowkfileCUE string) (formattedMod, formattedFile string, err error) {
	formattedMod, err = formatCUEFile(invowkmodCUE, "invowkmod.cue")
	if err != nil {
		return "", "", err
	}
	formattedFile, err = formatCUEFile(invowkfileCUE, "invowkfile.cue")
	if err != nil {
		return "", "", err
	}

	meta, err := invowkmod.ParseInvowkmodBytes([]byte(formattedMod), types.FilesystemPath(filepath.Join(modulePath, "invowkmod.cue")))
	if err != nil {
		return "", "", fmt.Errorf("validate generated invowkmod.cue: %w", err)
	}
	if meta.Module != moduleID {
		return "", "", fmt.Errorf("generated module ID %q does not match requested module ID %q", meta.Module, moduleID)
	}
	if _, err := invowkfile.ParseBytes([]byte(formattedFile), filepath.Join(modulePath, "invowkfile.cue")); err != nil {
		return "", "", fmt.Errorf("validate generated invowkfile.cue: %w", err)
	}
	if err := validateModuleBundleInTemp(moduleID, formattedMod, formattedFile); err != nil {
		return "", "", err
	}
	return formattedMod, formattedFile, nil
}

// ModuleGenerationResponseSchema returns the JSON schema for module-generation responses.
func ModuleGenerationResponseSchema() map[string]any {
	return map[string]any{
		jsonSchemaType: "object",
		"properties": map[string]any{
			"invowkmod_cue": map[string]any{
				jsonSchemaType:        jsonSchemaTypeString,
				jsonSchemaDescription: "Complete invowkmod.cue content for the generated module.",
			},
			"invowkfile_cue": map[string]any{
				jsonSchemaType:        jsonSchemaTypeString,
				jsonSchemaDescription: "Complete invowkfile.cue content for the generated module.",
			},
			"summary": map[string]any{
				jsonSchemaType:        jsonSchemaTypeString,
				jsonSchemaDescription: "A short human summary of the generated module.",
			},
		},
		"required":             []string{"invowkmod_cue", "invowkfile_cue", "summary"},
		"additionalProperties": false,
	}
}

func validateModuleDescriptionInput(description, fromFile string) error {
	if strings.TrimSpace(description) == "" && fromFile == "" {
		return errors.New("module description is required")
	}
	if strings.TrimSpace(description) != "" && fromFile != "" {
		return errors.New("description arguments and --from-file are mutually exclusive")
	}
	return nil
}

func loadModuleDescription(description, fromFile string) (string, error) {
	return loadDescription(description, fromFile)
}

func modulePathForID(moduleID invowkmod.ModuleID) (string, error) {
	dirName, err := invowkmod.CanonicalModuleDirectoryName(moduleID)
	if err != nil {
		return "", err
	}
	return dirName.String(), nil
}

func resolveModulePath(target string) (string, error) {
	target = strings.TrimSpace(target)
	if strings.ContainsAny(target, `/\`) || strings.HasSuffix(filepath.Base(target), invowkmod.ModuleSuffix) || strings.HasPrefix(target, ".") {
		return filepath.Clean(target), nil
	}
	moduleID := invowkmod.ModuleID(target)
	if err := moduleID.Validate(); err != nil {
		return "", err
	}
	return modulePathForID(moduleID)
}

func loadExistingModule(target string) (loadedModule, error) {
	modulePath, err := resolveModulePath(target)
	if err != nil {
		return loadedModule{}, err
	}
	info, err := os.Lstat(modulePath)
	if err != nil {
		if os.IsNotExist(err) {
			return loadedModule{}, fmt.Errorf("module %q does not exist; use `invowk agent mod create %s` to add it", target, target)
		}
		return loadedModule{}, fmt.Errorf("check module path %s: %w", modulePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return loadedModule{}, fmt.Errorf("module path %s is a symlink; symlinked modules cannot be changed or removed", modulePath)
	}
	if !info.IsDir() {
		return loadedModule{}, fmt.Errorf("module path %s is not a directory", modulePath)
	}
	if !invowkmod.IsModule(types.FilesystemPath(modulePath)) {
		return loadedModule{}, fmt.Errorf("%s is not a valid local invowk module directory", modulePath)
	}
	if verifyErr := verifyModule(modulePath); verifyErr != nil {
		return loadedModule{}, verifyErr
	}

	invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
	invowkmodData, err := os.ReadFile(invowkmodPath)
	if err != nil {
		return loadedModule{}, fmt.Errorf("read %s: %w", invowkmodPath, err)
	}
	meta, err := invowkmod.ParseInvowkmodBytes(invowkmodData, types.FilesystemPath(invowkmodPath))
	if err != nil {
		return loadedModule{}, fmt.Errorf("parse %s: %w", invowkmodPath, err)
	}

	invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
	invowkfileData, err := os.ReadFile(invowkfilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return loadedModule{}, fmt.Errorf("read %s: %w", invowkfilePath, err)
		}
		invowkfileData = []byte(defaultInvowkfileContent)
	}
	return loadedModule{
		Path:          modulePath,
		ModuleID:      meta.Module,
		InvowkmodCUE:  string(invowkmodData),
		InvowkfileCUE: string(invowkfileData),
	}, nil
}

func formatCUEFile(content, name string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("%s is empty", name)
	}
	formatted, err := cueformat.Source([]byte(content + "\n"))
	if err != nil {
		return "", fmt.Errorf("format %s: %w", name, err)
	}
	return string(formatted), nil
}

func validateModuleBundleInTemp(moduleID invowkmod.ModuleID, invowkmodCUE, invowkfileCUE string) error {
	tempDir, err := os.MkdirTemp("", "invowk-agent-module-*")
	if err != nil {
		return fmt.Errorf("create temporary module validation directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	dirName, err := invowkmod.CanonicalModuleDirectoryName(moduleID)
	if err != nil {
		return err
	}
	modulePath := filepath.Join(tempDir, dirName.String())
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		return fmt.Errorf("create temporary module directory: %w", err)
	}
	if err := writeModuleFiles(modulePath, invowkmodCUE, invowkfileCUE); err != nil {
		return fmt.Errorf("write temporary module files: %w", err)
	}
	return verifyModule(modulePath)
}

func writeModuleBundle(modulePath, invowkmodCUE, invowkfileCUE string, createScriptsDir bool) error {
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		return fmt.Errorf("create module directory: %w", err)
	}
	if err := writeModuleFiles(modulePath, invowkmodCUE, invowkfileCUE); err != nil {
		_ = os.RemoveAll(modulePath)
		return err
	}
	if !createScriptsDir {
		return nil
	}
	scriptsDir := filepath.Join(modulePath, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		_ = os.RemoveAll(modulePath)
		return fmt.Errorf("create scripts directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, ".gitkeep"), nil, 0o644); err != nil {
		_ = os.RemoveAll(modulePath)
		return fmt.Errorf("create scripts .gitkeep: %w", err)
	}
	return nil
}

func writeModuleFiles(modulePath, invowkmodCUE, invowkfileCUE string) error {
	if err := os.WriteFile(filepath.Join(modulePath, "invowkmod.cue"), []byte(invowkmodCUE), 0o644); err != nil {
		return fmt.Errorf("write invowkmod.cue: %w", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "invowkfile.cue"), []byte(invowkfileCUE), 0o644); err != nil {
		return fmt.Errorf("write invowkfile.cue: %w", err)
	}
	return nil
}

func verifyModule(modulePath string) error {
	result, err := invowkmod.Validate(types.FilesystemPath(modulePath))
	if err != nil {
		return fmt.Errorf("validate module %s: %w", modulePath, err)
	}
	if result.Valid {
		return nil
	}
	messages := make([]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		messages = append(messages, issue.Error())
	}
	return fmt.Errorf("validate module %s: %s", modulePath, strings.Join(messages, "; "))
}

func buildModuleDiff(modulePath, oldMod, newMod, oldFile, newFile string, existed bool) string {
	var b strings.Builder
	b.WriteString(BuildUnifiedDiff(filepath.Join(modulePath, "invowkmod.cue"), oldMod, newMod, existed))
	b.WriteString(BuildUnifiedDiff(filepath.Join(modulePath, "invowkfile.cue"), oldFile, newFile, existed))
	return b.String()
}

func buildModuleRemovePlan(modulePath string) string {
	var b strings.Builder
	b.WriteString("delete ")
	b.WriteString(modulePath)
	b.WriteByte('\n')
	return b.String()
}

func validateModuleGenerationResponse(resp moduleGenerationResponse) (moduleGenerationResponse, error) {
	if strings.TrimSpace(resp.InvowkmodCUE) == "" {
		return moduleGenerationResponse{}, errors.New("LLM response missing invowkmod_cue")
	}
	if strings.TrimSpace(resp.InvowkfileCUE) == "" {
		return moduleGenerationResponse{}, errors.New("LLM response missing invowkfile_cue")
	}
	return resp, nil
}

func decodeStrictJSON(raw string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("unexpected trailing JSON tokens")
	}
	return nil
}
