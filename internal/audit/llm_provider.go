// SPDX-License-Identifier: MPL-2.0

package audit

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
)

const (
	llmProviderNotFoundErrMsg = "no LLM provider found"

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

	defaultClaudeModel = "claude-sonnet-4-6"
	defaultOpenAIModel = "gpt-4o"
	defaultGeminiModel = "gemini-2.5-flash"
)

// ErrLLMProviderNotFound is the sentinel for when no LLM provider can be detected.
var ErrLLMProviderNotFound = errors.New(llmProviderNotFoundErrMsg)

type (
	// LLMProviderNotFoundError is returned when auto-detection finds no provider.
	LLMProviderNotFoundError struct {
		Tried []string
	}

	// ProviderResult holds a detected provider's configuration.
	ProviderResult struct {
		completer llmCompleter
		name      string
		model     string
	}

	// CLICompleter implements llmCompleter by shelling out to an AI CLI tool.
	// It supports Claude Code, Codex CLI, and Gemini CLI, each with their
	// respective non-interactive flags and JSON output parsing.
	CLICompleter struct {
		tool  string
		model string
	}
)

// Completer returns the llmCompleter for this provider.
func (r *ProviderResult) Completer() llmCompleter { return r.completer }

// Name returns the provider name (e.g., "claude", "ollama").
func (r *ProviderResult) Name() string { return r.name }

// Model returns the model that will be used.
func (r *ProviderResult) Model() string { return r.model }

// Error implements the error interface.
func (e *LLMProviderNotFoundError) Error() string {
	return fmt.Sprintf("%s (tried: %s)", llmProviderNotFoundErrMsg, strings.Join(e.Tried, ", "))
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMProviderNotFoundError) Unwrap() error { return ErrLLMProviderNotFound }

// ValidProviders returns the list of valid --llm-provider values.
func ValidProviders() []string {
	return []string{ProviderAuto, ProviderClaude, ProviderCodex, ProviderGemini, ProviderOllama}
}

// DetectProvider resolves an LLM provider by name. When name is "auto", it
// probes providers in local-first order: Ollama, then cloud env vars
// (Anthropic, OpenAI, Google), then CLI tools (claude, codex, gemini).
//
// The modelOverride parameter, when non-empty, replaces the provider's
// default model selection.
func DetectProvider(ctx context.Context, name, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if name != ProviderAuto {
		return detectSpecificProvider(ctx, name, modelOverride, timeout)
	}
	return detectAutoProvider(ctx, modelOverride, timeout)
}

// detectAutoProvider probes all providers in local-first order.
func detectAutoProvider(ctx context.Context, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	var tried []string

	// 1. Local Ollama (free, private).
	if result, err := tryOllama(ctx, modelOverride, timeout); err == nil {
		return result, nil
	}
	tried = append(tried, "ollama (localhost:11434)")

	// 2. Cloud env vars (no subprocess overhead).
	if result, err := tryEnvVar("ANTHROPIC_API_KEY", anthropicBaseURL, defaultClaudeModel, ProviderClaude, modelOverride, timeout); err == nil {
		return result, nil
	}
	tried = append(tried, "ANTHROPIC_API_KEY")

	if result, err := tryEnvVar("OPENAI_API_KEY", openaiBaseURL, defaultOpenAIModel, ProviderCodex, modelOverride, timeout); err == nil {
		return result, nil
	}
	tried = append(tried, "OPENAI_API_KEY")

	if result, err := tryGeminiEnvVar(modelOverride, timeout); err == nil {
		return result, nil
	}
	tried = append(tried, "GEMINI_API_KEY/GOOGLE_API_KEY")

	// 3. CLI tools (handles OAuth, keychain, ADC).
	for _, tool := range []string{"claude", "codex", "gemini"} {
		if result, err := tryCLITool(tool, modelOverride); err == nil {
			return result, nil
		}
		tried = append(tried, tool+" CLI")
	}

	return nil, &LLMProviderNotFoundError{Tried: tried}
}

// detectSpecificProvider resolves a named provider.
func detectSpecificProvider(ctx context.Context, name, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	switch name {
	case ProviderOllama:
		return tryOllama(ctx, modelOverride, timeout)
	case ProviderClaude:
		return detectClaude(modelOverride, timeout)
	case ProviderCodex:
		return detectCodex(modelOverride, timeout)
	case ProviderGemini:
		return detectGemini(modelOverride, timeout)
	default:
		return nil, &LLMClientConfigInvalidError{
			Reason: fmt.Sprintf("unknown provider %q; valid: %s", name, strings.Join(ValidProviders(), ", ")),
		}
	}
}

// tryOllama probes the local Ollama server.
func tryOllama(ctx context.Context, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, ollamaProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, DefaultLLMBaseURL+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating ollama probe request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
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
func tryEnvVar(envVar, baseURL, defaultModel, providerName, modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	apiKey := os.Getenv(envVar)
	if apiKey == "" {
		return nil, fmt.Errorf("%s not set", envVar)
	}

	model := defaultModel
	if modelOverride != "" {
		model = modelOverride
	}

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}

	return &ProviderResult{completer: client, name: providerName, model: model}, nil
}

// tryGeminiEnvVar checks both GEMINI_API_KEY and GOOGLE_API_KEY.
func tryGeminiEnvVar(modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	for _, envVar := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		if result, err := tryEnvVar(envVar, geminiBaseURL, defaultGeminiModel, ProviderGemini, modelOverride, timeout); err == nil {
			return result, nil
		}
	}
	return nil, errors.New("no Gemini API key found")
}

// detectClaude tries env var first, then the claude CLI.
func detectClaude(modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if result, err := tryEnvVar("ANTHROPIC_API_KEY", anthropicBaseURL, defaultClaudeModel, ProviderClaude, modelOverride, timeout); err == nil {
		return result, nil
	}
	return tryCLITool("claude", modelOverride)
}

// detectCodex tries env var first, then the codex CLI.
func detectCodex(modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if result, err := tryEnvVar("OPENAI_API_KEY", openaiBaseURL, defaultOpenAIModel, ProviderCodex, modelOverride, timeout); err == nil {
		return result, nil
	}
	return tryCLITool("codex", modelOverride)
}

// detectGemini tries env vars first, then the gemini CLI.
func detectGemini(modelOverride string, timeout time.Duration) (*ProviderResult, error) {
	if result, err := tryGeminiEnvVar(modelOverride, timeout); err == nil {
		return result, nil
	}
	return tryCLITool("gemini", modelOverride)
}

// tryCLITool checks if a CLI tool is in PATH and returns a CLI completer.
func tryCLITool(tool, modelOverride string) (*ProviderResult, error) {
	if _, err := exec.LookPath(tool); err != nil {
		return nil, fmt.Errorf("%s not found in PATH", tool)
	}

	providerName := tool
	if tool == "codex" {
		providerName = ProviderCodex
	}

	model := cliDefaultModel(tool)
	if modelOverride != "" {
		model = modelOverride
	}

	return &ProviderResult{
		completer: NewCLICompleter(tool, model),
		name:      providerName,
		model:     model,
	}, nil
}

// cliDefaultModel returns the default model name for a CLI tool.
func cliDefaultModel(tool string) string {
	switch tool {
	case "claude":
		return defaultClaudeModel
	case "codex":
		return defaultOpenAIModel
	case "gemini":
		return defaultGeminiModel
	default:
		return DefaultLLMModel
	}
}

// NewCLICompleter creates a completer that shells out to the named CLI tool.
func NewCLICompleter(tool, model string) *CLICompleter {
	return &CLICompleter{tool: tool, model: model}
}

// Complete sends a prompt to the CLI tool and returns the response text.
// Each tool has different flags and output formats:
//   - claude: -p --output-format json --bare -> {"type":"result","result":"..."}
//   - codex: exec --json -m <model> -> JSONL with item.completed events
//   - gemini: -p --output-format json -> {"response":"..."}
func (c *CLICompleter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	prompt := systemPrompt + "\n\n" + userPrompt

	args, err := c.buildArgs(prompt)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, c.tool, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("%s CLI failed (exit %d): %s", c.tool, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("%s CLI failed: %w", c.tool, err)
	}

	return c.parseOutput(string(output))
}

// buildArgs constructs the command-line arguments for the tool.
func (c *CLICompleter) buildArgs(prompt string) ([]string, error) {
	switch c.tool {
	case "claude":
		return []string{"-p", prompt, "--output-format", "json", "--bare"}, nil
	case "codex":
		args := []string{"exec", prompt, "--json"}
		if c.model != "" {
			args = append(args, "-m", c.model)
		}
		return args, nil
	case "gemini":
		return []string{"-p", prompt, "--output-format", "json"}, nil
	default:
		return nil, fmt.Errorf("unsupported CLI tool: %s", c.tool)
	}
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
		return "", &LLMMalformedResponseError{RawResponse: raw, Err: fmt.Errorf("parsing claude output: %w", err)}
	}
	if resp.Result == "" {
		return "", fmt.Errorf("%w: empty result from claude CLI", ErrLLMEmptyResponse)
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
		return "", &LLMMalformedResponseError{RawResponse: raw, Err: errors.New("no agent_message found in codex output")}
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
		return "", &LLMMalformedResponseError{RawResponse: raw, Err: fmt.Errorf("parsing gemini output: %w", err)}
	}
	if resp.Response == "" {
		return "", fmt.Errorf("%w: empty response from gemini CLI", ErrLLMEmptyResponse)
	}
	return resp.Response, nil
}
