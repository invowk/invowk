// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	scriptCheckerName = "script"
	// maxScriptFileSize is the threshold for flagging unusually large scripts (5 MiB).
	maxScriptFileSize = 5 * 1024 * 1024
)

// Regex patterns for script content scanning.
var (
	// Remote execution patterns (Critical): download-and-pipe-to-interpreter.
	// Also matches process substitution: bash <(curl ...).
	remoteExecPattern = regexp.MustCompile(
		`(curl|wget|fetch)\s+[^\|]*\|\s*(sh|bash|zsh|ash|dash|python[23]?|perl|ruby|node(js)?|php|pwsh|powershell)`)
	processSubstitutionPattern = regexp.MustCompile(
		`\b(sh|bash|zsh|ash|dash|python[23]?|perl|ruby|node(js)?|php|pwsh|powershell)\s+<\(\s*(curl|wget|fetch)`)
	// Download-then-execute pattern (Critical): download to file then run.
	// Matches &&, ;, and newline separators. Also matches wget --output-document.
	downloadExecPattern = regexp.MustCompile(
		`(curl|wget|fetch)\s+.*(-[oO]\s+\S+|--output-document[= ]\S+).*([;&]\s*|&&\s*)(sh|bash|chmod\s+\+x)`)

	// Obfuscation patterns (High).
	base64DecodePattern = regexp.MustCompile(`base64\s+(-d|--decode)`)
	evalPattern         = regexp.MustCompile("\\beval\\b\\s*[$\"'`]")
	base64SubshPattern  = regexp.MustCompile("(\\$\\(|`).*?base64")
	encodedPipePattern  = regexp.MustCompile(`echo\s+["']?[A-Za-z0-9+/=]{20,}["']?\s*\|\s*base64`)

	// Hex encoding patterns (High).
	hexSequencePattern = regexp.MustCompile(`\\x[0-9a-fA-F]{2}.*\\x[0-9a-fA-F]{2}.*\\x[0-9a-fA-F]{2}`)

	// Path traversal in script content.
	pathTraversalPattern = regexp.MustCompile(`\.\./`)
)

// ScriptChecker analyzes script content and paths for path traversal,
// obfuscation, remote execution, and other suspicious patterns.
type ScriptChecker struct{}

// NewScriptChecker creates a ScriptChecker.
func NewScriptChecker() *ScriptChecker { return &ScriptChecker{} }

// Name returns the checker identifier.
func (c *ScriptChecker) Name() string { return scriptCheckerName }

// Category returns CategoryExecution as the primary category.
func (c *ScriptChecker) Category() Category { return CategoryExecution }

// Check analyzes all scripts in the scan context.
func (c *ScriptChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	allScripts := sc.AllScripts()
	for i := range allScripts {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("script checker cancelled: %w", ctx.Err())
		default:
		}

		ref := &allScripts[i]
		findings = append(findings, c.checkScriptPath(*ref)...)

		// For file-based scripts, check file size and read content.
		if ref.IsFile {
			findings = append(findings, c.checkScriptFileSize(*ref)...)
		}

		// Analyze script content (both inline and file-based).
		// Content() returns the actual script body — for file-based scripts
		// this is the file contents read during context building (not the path).
		content := ref.Content()
		if content != "" {
			findings = append(findings, c.checkRemoteExecution(*ref, content)...)
			findings = append(findings, c.checkObfuscation(*ref, content)...)
		}
	}

	return findings, nil
}

func (c *ScriptChecker) checkScriptPath(ref ScriptRef) []Finding {
	if !ref.IsFile {
		return nil
	}

	var findings []Finding
	script := strings.TrimSpace(string(ref.Script))

	// Check for path traversal in module context.
	if ref.ModulePath != "" {
		if strings.Contains(script, "../") || strings.Contains(script, "..\\") {
			findings = append(findings, Finding{
				Code:           codeScriptPathOutsideModule,
				Severity:       SeverityHigh,
				Category:       CategoryPathTraversal,
				SurfaceID:      ref.SurfaceID,
				SurfaceKind:    ref.SurfaceKind,
				CheckerName:    scriptCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Script references path outside module boundary",
				Description:    fmt.Sprintf("Script path %q in command %q uses path traversal — may escape module boundary", script, ref.CommandName),
				Recommendation: "Use paths relative to the module root without '..' components",
			})
		}

		// Absolute paths in module context.
		if strings.HasPrefix(script, "/") {
			findings = append(findings, Finding{
				Code:           codeScriptAbsolutePath,
				Severity:       SeverityHigh,
				Category:       CategoryPathTraversal,
				SurfaceID:      ref.SurfaceID,
				SurfaceKind:    ref.SurfaceKind,
				CheckerName:    scriptCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Module script uses absolute path",
				Description:    fmt.Sprintf("Script path %q in command %q is absolute — bypasses module boundary containment", script, ref.CommandName),
				Recommendation: "Use a relative path within the module directory",
			})
		}
	}

	return findings
}

func (c *ScriptChecker) checkScriptFileSize(ref ScriptRef) []Finding {
	if ref.ModulePath == "" {
		return nil
	}

	if ref.FileStatErr != nil || ref.ScriptPath == "" {
		return nil
	}

	if ref.FileSize > maxScriptFileSize {
		return []Finding{{
			Code:           codeScriptFileLarge,
			Severity:       SeverityMedium,
			Category:       CategoryExecution,
			SurfaceID:      ref.SurfaceID,
			SurfaceKind:    ref.SurfaceKind,
			CheckerName:    scriptCheckerName,
			FilePath:       ref.ScriptPath,
			Title:          "Script file unusually large",
			Description:    fmt.Sprintf("Script file is %d bytes — may contain embedded binaries or obfuscated content", ref.FileSize),
			Recommendation: "Review the script contents; large scripts warrant extra scrutiny",
		}}
	}
	return nil
}

func (c *ScriptChecker) checkRemoteExecution(ref ScriptRef, content string) []Finding {
	var findings []Finding

	if remoteExecPattern.MatchString(content) || downloadExecPattern.MatchString(content) || processSubstitutionPattern.MatchString(content) {
		findings = append(findings, Finding{
			Code:           codeScriptRemoteExecution,
			Severity:       SeverityCritical,
			Category:       CategoryExecution,
			SurfaceID:      ref.SurfaceID,
			CheckerName:    scriptCheckerName,
			FilePath:       ref.FilePath,
			Title:          "Script downloads and executes remote code",
			Description:    fmt.Sprintf("Command %q contains a remote code download and execution pattern (pipe, process substitution, or download-then-execute)", ref.CommandName),
			Recommendation: "Download to a temporary file, verify its checksum, then execute",
		})
	}

	return findings
}

func (c *ScriptChecker) checkObfuscation(ref ScriptRef, content string) []Finding {
	var findings []Finding

	patterns := []struct {
		code  FindingCode
		re    *regexp.Regexp
		title string
		desc  string
	}{
		{codeScriptBase64Decode, base64DecodePattern, "Script contains base64 decode", "base64 -d/--decode pattern detected"},
		{codeScriptEvalDynamic, evalPattern, "Script uses eval with dynamic content", "eval with variable/string interpolation detected"},
		{codeScriptBase64Subshell, base64SubshPattern, "Script uses base64 in subshell", "base64 encoding in $() subshell detected"},
		{codeScriptEncodedPipe, encodedPipePattern, "Script pipes encoded content to base64", "Long encoded string piped to base64 detected"},
		{codeScriptHexEscapes, hexSequencePattern, "Script contains hex escape sequences", "Multiple hex escape sequences detected"},
	}

	for _, p := range patterns {
		if p.re.MatchString(content) {
			findings = append(findings, Finding{
				Code:           p.code,
				Severity:       SeverityHigh,
				Category:       CategoryObfuscation,
				SurfaceID:      ref.SurfaceID,
				CheckerName:    scriptCheckerName,
				FilePath:       ref.FilePath,
				Title:          p.title,
				Description:    fmt.Sprintf("Command %q: %s — may be hiding malicious content", ref.CommandName, p.desc),
				Recommendation: "Decode and review the obfuscated content; replace with clear, readable commands",
			})
		}
	}

	// Path traversal in script content (not just path field).
	if ref.ModulePath != "" && pathTraversalPattern.MatchString(content) {
		// Only flag if not already caught by checkScriptPath.
		if !ref.IsFile || !strings.Contains(strings.TrimSpace(string(ref.Script)), "../") {
			findings = append(findings, Finding{
				Code:           codeScriptContentPathTraversal,
				Severity:       SeverityMedium,
				Category:       CategoryPathTraversal,
				SurfaceID:      ref.SurfaceID,
				SurfaceKind:    ref.SurfaceKind,
				CheckerName:    scriptCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Script content contains path traversal",
				Description:    fmt.Sprintf("Command %q script body references '../' — may access files outside the expected directory", ref.CommandName),
				Recommendation: "Use absolute paths or paths relative to the working directory without '..' components",
			})
		}
	}

	return findings
}
