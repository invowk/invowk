// SPDX-License-Identifier: MPL-2.0

package audit

import (
	_ "embed" // required for go:embed prompts/llm_system.md
	"encoding/json"
	"errors"
	"fmt"
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

// convertBatchFindings maps LLM findings to audit Findings for a batch
// of scripts, matching findings to scripts by command name. Findings with
// invalid severity or category values are silently discarded as a defense
// against LLM hallucination.
func convertBatchFindings(parsed []llmFinding, batch []ScriptRef) []Finding {
	// Build lookup by command name for efficient matching.
	byName := make(map[string]*ScriptRef, len(batch))
	for i := range batch {
		byName[string(batch[i].CommandName)] = &batch[i]
	}

	var findings []Finding

	for i := range parsed {
		lf := &parsed[i]

		// Match finding to script by command name; fall back to first
		// script when the LLM returns an unrecognized command name.
		ref, ok := byName[lf.CommandName]
		if !ok {
			ref = &batch[0]
		}

		f, valid := buildFinding(lf, ref)
		if !valid {
			continue
		}
		findings = append(findings, f)
	}

	return findings
}

// buildFinding validates severity/category and constructs a Finding from an
// LLM-reported finding and its attributed ScriptRef. Returns false when the
// finding has invalid severity or category (hallucination defense).
func buildFinding(lf *llmFinding, ref *ScriptRef) (Finding, bool) {
	sev, err := ParseSeverity(lf.Severity)
	if err != nil {
		return Finding{}, false
	}

	category := Category(lf.Category)
	if category.Validate() != nil {
		return Finding{}, false
	}

	return Finding{
		Severity:       sev,
		Category:       category,
		SurfaceID:      ref.SurfaceID,
		CheckerName:    llmCheckerName,
		FilePath:       ref.FilePath,
		Line:           lf.Line,
		Title:          lf.Title,
		Description:    lf.Description,
		Recommendation: lf.Recommendation,
	}, true
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
