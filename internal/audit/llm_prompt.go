// SPDX-License-Identifier: MPL-2.0

package audit

import (
	_ "embed" // required for go:embed prompts/llm_system.md
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	//go:embed prompts/llm_system.md
	systemPrompt string

	// jsonFencePattern matches JSON inside markdown code fences.
	jsonFencePattern = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.+?)\\n\\s*```")
)

type (
	// llmFindingResponse is the expected JSON structure from the LLM.
	llmFindingResponse struct {
		Findings []llmFinding `json:"findings"`
	}

	// llmFinding is a single finding as reported by the LLM.
	llmFinding struct {
		ScriptID       string `json:"script_id,omitempty"`
		Severity       string `json:"severity"`
		Category       string `json:"category"`
		CommandName    string `json:"command_name"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		Recommendation string `json:"recommendation"`
		Line           int    `json:"line,omitempty"`
	}
)

// buildUserPrompt constructs the user message containing scripts to analyze.
// Each script is formatted with metadata (command name, file path, runtimes)
// to give the LLM context for its analysis.
func buildUserPrompt(scripts []ScriptRef) string {
	var b strings.Builder
	b.WriteString("Analyze the following scripts for security vulnerabilities:\n\n")

	for i := range scripts {
		ref := &scripts[i]
		fmt.Fprintf(&b, "Script ID: %s\n", scriptPromptID(ref))
		fmt.Fprintf(&b, "=== Script: %s ===\n", ref.CommandName)
		fmt.Fprintf(&b, "File: %s\n", ref.FilePath)

		if len(ref.Runtimes) > 0 {
			runtimeNames := make([]string, 0, len(ref.Runtimes))
			for ri := range ref.Runtimes {
				runtimeNames = append(runtimeNames, string(ref.Runtimes[ri].Name))
			}
			fmt.Fprintf(&b, "Runtime: %s\n", strings.Join(runtimeNames, ", "))
		}

		b.WriteString("---\n")
		script := ref.Content()
		b.WriteString(script)
		if !strings.HasSuffix(script, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("===\n\n")
	}

	return b.String()
}

// parseFindings extracts structured findings from the LLM response text.
// It attempts direct JSON parsing first, then falls back to extracting
// JSON from markdown code fences (a common LLM output quirk).
func parseFindings(raw string) ([]llmFinding, error) {
	// Try direct JSON parse.
	var resp llmFindingResponse
	if err := json.Unmarshal([]byte(raw), &resp); err == nil {
		return resp.Findings, nil
	}

	// Fall back to extracting JSON from markdown code fences.
	matches := jsonFencePattern.FindStringSubmatch(raw)
	if len(matches) >= 2 {
		if err := json.Unmarshal([]byte(matches[1]), &resp); err == nil {
			return resp.Findings, nil
		}
	}

	// Try to find any JSON object in the response as a last resort.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &resp); err == nil {
			return resp.Findings, nil
		}
	}

	return nil, &LLMMalformedResponseError{
		RawResponse: raw,
		Err:         errors.New("could not extract JSON findings from response"),
	}
}

// convertBatchFindings maps LLM findings to audit Findings for a batch.
// A non-empty findings array is an LLM response contract: unusable finding
// entries are reported as malformed responses so explicit LLM analysis cannot
// be silently interpreted as clean.
func convertBatchFindings(parsed []llmFinding, batch []ScriptRef) ([]Finding, error) {
	// Build lookups for efficient exact matching.
	byID := make(map[string]*ScriptRef, len(batch))
	byName := make(map[string]*ScriptRef, len(batch))
	nameCounts := make(map[string]int, len(batch))
	for i := range batch {
		ref := &batch[i]
		byID[scriptPromptID(ref)] = ref
		name := string(ref.CommandName)
		byName[name] = ref
		nameCounts[name]++
	}

	var findings []Finding
	var errs []error

	for i := range parsed {
		lf := &parsed[i]

		ref, ok := matchLLMFindingToScript(lf, byID, byName, nameCounts, batch)
		if !ok {
			errs = append(errs, malformedLLMFindingError(i, fmt.Sprintf("finding could not be matched to a script (script_id=%q, command_name=%q)", lf.ScriptID, lf.CommandName)))
			continue
		}

		f, err := buildFinding(lf, ref)
		if err != nil {
			errs = append(errs, malformedLLMFindingError(i, err.Error()))
			continue
		}
		findings = append(findings, f)
	}

	if len(errs) > 0 {
		return findings, errors.Join(errs...)
	}
	return findings, nil
}

func matchLLMFindingToScript(
	lf *llmFinding,
	byID map[string]*ScriptRef,
	byName map[string]*ScriptRef,
	nameCounts map[string]int,
	batch []ScriptRef,
) (*ScriptRef, bool) {
	if lf.ScriptID != "" {
		ref, ok := byID[lf.ScriptID]
		return ref, ok
	}
	if lf.CommandName != "" && nameCounts[lf.CommandName] == 1 {
		ref, ok := byName[lf.CommandName]
		return ref, ok
	}
	if len(batch) == 1 {
		return &batch[0], true
	}
	return nil, false
}

func scriptPromptID(ref *ScriptRef) string {
	return fmt.Sprintf(
		"surface=%s;file=%s;command=%s;impl=%d",
		url.QueryEscape(ref.SurfaceID),
		url.QueryEscape(string(ref.FilePath)),
		url.QueryEscape(string(ref.CommandName)),
		ref.ImplIndex,
	)
}

//goplint:ignore -- LLM response conversion builds provider-facing diagnostics from JSON finding ordinal and reason.
func malformedLLMFindingError(index int, reason string) error {
	return &LLMMalformedResponseError{
		Err: fmt.Errorf("invalid LLM finding %d: %s", index, reason),
	}
}

// buildFinding validates severity/category and constructs a Finding from an
// LLM-reported finding and its attributed ScriptRef.
func buildFinding(lf *llmFinding, ref *ScriptRef) (Finding, error) {
	sev, err := ParseSeverity(lf.Severity)
	if err != nil {
		return Finding{}, fmt.Errorf("invalid severity %q: %w", lf.Severity, err)
	}

	category := Category(lf.Category)
	if err := category.Validate(); err != nil {
		return Finding{}, fmt.Errorf("invalid category %q: %w", lf.Category, err)
	}

	return Finding{
		Severity:       sev,
		Category:       category,
		SurfaceID:      ref.SurfaceID,
		SurfaceKey:     ref.SurfaceKey,
		SurfaceKind:    ref.SurfaceKind,
		CheckerName:    llmCheckerName,
		FilePath:       ref.FilePath,
		Line:           lf.Line,
		Title:          lf.Title,
		Description:    lf.Description,
		Recommendation: lf.Recommendation,
	}, nil
}

// truncateScript limits a script's content to maxChars characters.
// Returns the original string if it's within limits.
func truncateScript(content string, maxChars int) (string, bool) {
	if len(content) <= maxChars {
		return content, false
	}
	return content[:maxChars] + fmt.Sprintf("\n[TRUNCATED at %d chars]", maxChars), true
}

// prepareScripts filters scripts suitable for LLM analysis and applies truncation.
func prepareScripts(scripts []ScriptRef, maxScriptChars int) []ScriptRef {
	result := make([]ScriptRef, 0, len(scripts))

	for i := range scripts {
		content := scripts[i].Content()
		if strings.TrimSpace(content) == "" {
			continue
		}

		truncated, _ := truncateScript(content, maxScriptChars)
		ref := scripts[i]
		ref.resolvedContent = truncated
		result = append(result, ref)
	}

	return result
}
