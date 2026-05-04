// SPDX-License-Identifier: MPL-2.0

package auditllm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/llm"
)

const (
	llmProviderNotFoundErrMsg      = "no LLM provider found"
	llmProviderModelRequiredErrMsg = "LLM provider model required"

	// ProviderAuto auto-detects the best available LLM provider.
	ProviderAuto = "auto"
	// ProviderClaude uses Anthropic's Claude via env var or Claude Code CLI.
	ProviderClaude = "claude"
	// ProviderCodex uses OpenAI via env var or Codex CLI.
	ProviderCodex = "codex"
	// ProviderGemini uses Google Gemini via env var or Gemini CLI.
	ProviderGemini = "gemini"
	// ProviderOllama uses a local Ollama instance.
	ProviderOllama = "ollama"

	ollamaProbeTimeout = 2 * time.Second

	anthropicBaseURL = "https://api.anthropic.com/v1/"
	openaiBaseURL    = "https://api.openai.com/v1"
	geminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta/openai/"
)

var (
	_ llm.Completer = (*CLICompleter)(nil) // compile-time interface assertion

	// ErrLLMProviderNotFound is the sentinel for when no LLM provider can be detected.
	ErrLLMProviderNotFound = errors.New(llmProviderNotFoundErrMsg)
	// ErrLLMProviderModelRequired is the sentinel for cloud API providers selected
	// from environment credentials without an explicit --llm-model.
	ErrLLMProviderModelRequired = errors.New(llmProviderModelRequiredErrMsg)

	// cloudProviders defines the cloud provider configurations in detection order.
	cloudProviders = map[string]cloudProvider{
		ProviderClaude: {envVars: []string{"ANTHROPIC_API_KEY"}, baseURL: anthropicBaseURL, name: ProviderClaude, cliTool: "claude"},
		ProviderCodex:  {envVars: []string{"OPENAI_API_KEY"}, baseURL: openaiBaseURL, name: ProviderCodex, cliTool: "codex"},
		ProviderGemini: {envVars: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, baseURL: geminiBaseURL, name: ProviderGemini, cliTool: "gemini"},
	}
)

type (
	// LLMProviderNotFoundError is returned when auto-detection finds no provider.
	LLMProviderNotFoundError struct {
		Tried []string
	}

	// LLMProviderModelRequiredError is returned when cloud API credentials are
	// available but the caller did not choose a model explicitly.
	LLMProviderModelRequiredError struct {
		Provider string
		EnvVar   string
	}

	// ProviderResult holds a detected provider's configuration.
	ProviderResult struct {
		completer llm.Completer
		name      string
		model     string
	}

	// CLICompleter implements llmCompleter by shelling out to an AI CLI tool.
	// It supports Claude Code, Codex CLI, and Gemini CLI, each with their
	// respective non-interactive flags and JSON output parsing.
	CLICompleter struct {
		tool  string
		model string
		// runCmd executes a command and returns its stdout. When nil,
		// defaults to exec.CommandContext(...).Output() in Complete().
		runCmd func(ctx context.Context, name string, args []string, input string) ([]byte, error)
	}

	// providerDeps holds injectable infrastructure dependencies for provider
	// detection. Production code uses defaultProviderDeps(); tests inject
	// stubs to isolate behavior from the host environment.
	providerDeps struct {
		getenv   func(string) string
		lookPath func(string) (string, error)
		httpDo   func(*http.Request) (*http.Response, error)
	}

	// cloudProvider bundles the fixed configuration for a cloud LLM provider.
	// Each provider has one or more env var names, a base URL, a display name,
	// and a CLI tool name for fallback.
	cloudProvider struct {
		envVars []string
		baseURL string
		name    string
		cliTool string
	}
)

// Completer returns the LLM completer for this provider.
func (r *ProviderResult) Completer() llm.Completer { return r.completer }

// Name returns the provider name (e.g., "claude", "ollama").
func (r *ProviderResult) Name() string { return r.name }

// Model returns the model that will be used, or an empty string when a CLI
// provider is delegating model selection to the tool.
func (r *ProviderResult) Model() string { return r.model }

// Error implements the error interface.
func (e *LLMProviderNotFoundError) Error() string {
	return fmt.Sprintf("%s (tried: %s)", llmProviderNotFoundErrMsg, strings.Join(e.Tried, ", "))
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMProviderNotFoundError) Unwrap() error { return ErrLLMProviderNotFound }

// Error implements the error interface.
func (e *LLMProviderModelRequiredError) Error() string {
	return fmt.Sprintf("%s: %s is set for %s; specify --llm-model when using cloud API credentials",
		llmProviderModelRequiredErrMsg, e.EnvVar, e.Provider)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMProviderModelRequiredError) Unwrap() error {
	return ErrLLMProviderModelRequired
}

// ValidProviders returns the list of valid --llm-provider values.
func ValidProviders() []string {
	return []string{ProviderAuto, ProviderClaude, ProviderCodex, ProviderGemini, ProviderOllama}
}

// defaultProviderDeps returns the production infrastructure dependencies.
func defaultProviderDeps() providerDeps {
	return providerDeps{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		httpDo:   http.DefaultClient.Do,
	}
}

// DetectProvider resolves an LLM provider by name. When name is "auto", it
// probes providers in local-first order: Ollama, then cloud env vars
// (Anthropic, OpenAI, Google), then CLI tools (claude, codex, gemini).
//
// The modelOverride parameter, when non-empty, selects the cloud/API model or
// passes an explicit model flag to CLI tools. Empty modelOverride delegates
// CLI model selection to the tool default. Cloud API credentials require a
// non-empty modelOverride so invowk never hardcodes a stale cloud default.
func DetectProvider(ctx context.Context, name, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	return detectProviderWith(ctx, defaultProviderDeps(), name, modelOverride, timeout)
}

// detectProviderWith is the injectable core of DetectProvider.
func detectProviderWith(ctx context.Context, deps providerDeps, name, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if name != ProviderAuto {
		return detectSpecificProvider(ctx, deps, name, modelOverride, timeout)
	}
	return detectAutoProvider(ctx, deps, modelOverride, timeout)
}

// detectAutoProvider probes all providers in local-first order.
func detectAutoProvider(ctx context.Context, deps providerDeps, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	var tried []string

	// 1. Local Ollama (free, private).
	if result, err := tryOllama(ctx, deps, modelOverride, timeout); err == nil {
		return result, nil
	}
	tried = append(tried, "ollama (localhost:11434)")

	// 2. Cloud env vars (no subprocess overhead).
	for _, name := range []string{ProviderClaude, ProviderCodex, ProviderGemini} {
		spec := cloudProviders[name]
		for _, envVar := range spec.envVars {
			result, err := tryEnvVar(deps, spec, envVar, modelOverride, timeout)
			if err == nil {
				return result, nil
			}
			if errors.Is(err, ErrLLMProviderModelRequired) {
				return nil, err
			}
			tried = append(tried, envVar)
		}
	}

	// 3. CLI tools (handles OAuth, keychain, ADC).
	for _, name := range []string{ProviderClaude, ProviderCodex, ProviderGemini} {
		spec := cloudProviders[name]
		if result, err := tryCLITool(deps, spec.cliTool, modelOverride); err == nil {
			return result, nil
		}
		tried = append(tried, spec.cliTool+" CLI")
	}

	return nil, &LLMProviderNotFoundError{Tried: tried}
}

// detectSpecificProvider resolves a named provider.
func detectSpecificProvider(ctx context.Context, deps providerDeps, name, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if name == ProviderOllama {
		return tryOllama(ctx, deps, modelOverride, timeout)
	}
	if spec, ok := cloudProviders[name]; ok {
		return detectCloudProvider(deps, spec, modelOverride, timeout)
	}
	return nil, &LLMClientConfigInvalidError{
		Reason: fmt.Sprintf("unknown provider %q; valid: %s", name, strings.Join(ValidProviders(), ", ")),
	}
}

// tryOllama probes the local Ollama server using the injected HTTP client.
func tryOllama(ctx context.Context, deps providerDeps, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, ollamaProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, DefaultLLMBaseURL+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating ollama probe request: %w", err)
	}

	resp, err := deps.httpDo(req)
	if err != nil {
		return nil, fmt.Errorf("probing ollama: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	model := DefaultLLMModel
	if modelOverride != "" {
		model = modelOverride
	}

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: DefaultLLMBaseURL,
		Model:   model,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}

	return &ProviderResult{completer: client, name: ProviderOllama, model: model}, nil
}

// tryEnvVar checks for an API key env var and creates an HTTP completer.
func tryEnvVar(deps providerDeps, spec cloudProvider, envVar, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	apiKey := deps.getenv(envVar)
	if apiKey == "" {
		return nil, fmt.Errorf("%s not set", envVar)
	}
	if modelOverride == "" {
		return nil, &LLMProviderModelRequiredError{Provider: spec.name, EnvVar: envVar}
	}

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: spec.baseURL,
		Model:   modelOverride,
		APIKey:  apiKey,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}

	return &ProviderResult{completer: client, name: spec.name, model: modelOverride}, nil
}

// detectCloudProvider tries env vars first, then falls back to the CLI tool.
func detectCloudProvider(deps providerDeps, spec cloudProvider, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	for _, envVar := range spec.envVars {
		result, err := tryEnvVar(deps, spec, envVar, modelOverride, timeout)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, ErrLLMProviderModelRequired) {
			return nil, err
		}
	}
	return tryCLITool(deps, spec.cliTool, modelOverride)
}

// tryCLITool checks if a CLI tool is in PATH and returns a CLI completer.
func tryCLITool(deps providerDeps, tool, modelOverride string) (*ProviderResult, error) {
	if _, err := deps.lookPath(tool); err != nil {
		return nil, fmt.Errorf("%s not found in PATH", tool)
	}

	providerName := tool
	if tool == "codex" {
		providerName = ProviderCodex
	}

	return &ProviderResult{
		completer: NewCLICompleter(tool, modelOverride),
		name:      providerName,
		model:     modelOverride,
	}, nil
}

// NewCLICompleter creates a completer that shells out to the named CLI tool.
func NewCLICompleter(tool, model string) *CLICompleter {
	return &CLICompleter{tool: tool, model: model}
}

// Complete sends a prompt to the CLI tool and returns the response text.
// CLI tools accept a single prompt string, so the system and user prompts
// are concatenated with a blank line separator. This loses the role-based
// separation that HTTP-based completers (LLMClient) maintain via separate
// chat messages.
//
// Each tool has different flags and output formats:
//   - claude: -p --output-format json [--model <model>] -> {"type":"result","result":"..."}
//   - codex: exec --json [-m <model>] -> JSONL with item.completed events
//   - gemini: -p --output-format json [--model <model>] -> {"response":"..."}
func (c *CLICompleter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	prompt := systemPrompt + "\n\n" + userPrompt

	args, err := c.buildArgs()
	if err != nil {
		return "", err
	}

	run := c.runCmd
	if run == nil {
		run = defaultRunCmd
	}

	output, err := run(ctx, c.tool, args, prompt)
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("%s CLI failed (exit %d); provider output withheld", c.tool, exitErr.ExitCode())
		}
		return "", fmt.Errorf("%s CLI failed: %w", c.tool, err)
	}

	return c.parseOutput(string(output))
}

// defaultRunCmd is the production implementation that shells out via exec.
func defaultRunCmd(ctx context.Context, name string, args []string, input string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return out, err //nolint:wrapcheck // caller wraps with tool-specific context
	}
	return out, nil
}

// buildArgs constructs the command-line arguments for the tool.
func (c *CLICompleter) buildArgs() ([]string, error) {
	switch c.tool {
	case "claude", "gemini":
		return c.promptFlagArgs(), nil
	case "codex":
		args := []string{"exec", "--json"}
		if c.model != "" {
			args = append(args, "-m", c.model)
		}
		return args, nil
	default:
		return nil, fmt.Errorf("unsupported CLI tool: %s", c.tool)
	}
}

func (c *CLICompleter) promptFlagArgs() []string {
	args := []string{"-p", "-", "--output-format", "json"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	return args
}

// parseOutput extracts the response text from tool-specific JSON output.
func (c *CLICompleter) parseOutput(raw string) (string, error) {
	switch c.tool {
	case "claude":
		return parseClaudeOutput(raw)
	case "codex":
		return parseCodexOutput(raw)
	case "gemini":
		return parseGeminiOutput(raw)
	default:
		return "", fmt.Errorf("unsupported CLI tool: %s", c.tool)
	}
}

// parseClaudeOutput extracts the result from Claude Code JSON output.
// Format: {"type":"result","result":"the response text","session_id":"..."}
func parseClaudeOutput(raw string) (string, error) {
	var resp struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", &audit.LLMMalformedResponseError{RawResponse: raw, Err: fmt.Errorf("parsing claude output: %w", err)}
	}
	if resp.Result == "" {
		return "", fmt.Errorf("%w: empty result from claude CLI", audit.ErrLLMEmptyResponse)
	}
	return resp.Result, nil
}

// parseCodexOutput extracts the last agent message from Codex JSONL output.
// Format: newline-delimited JSON events; we want item.completed with type=agent_message.
func parseCodexOutput(raw string) (string, error) {
	var lastMessage string
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" && event.Item.Text != "" {
			lastMessage = event.Item.Text
		}
	}
	if lastMessage == "" {
		return "", &audit.LLMMalformedResponseError{RawResponse: raw, Err: errors.New("no agent_message found in codex output")}
	}
	return lastMessage, nil
}

// parseGeminiOutput extracts the response from Gemini CLI JSON output.
// Format: {"response":"the response text","stats":{...}}
func parseGeminiOutput(raw string) (string, error) {
	var resp struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", &audit.LLMMalformedResponseError{RawResponse: raw, Err: fmt.Errorf("parsing gemini output: %w", err)}
	}
	if resp.Response == "" {
		return "", fmt.Errorf("%w: empty response from gemini CLI", audit.ErrLLMEmptyResponse)
	}
	return resp.Response, nil
}
