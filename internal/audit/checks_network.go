// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"regexp"
)

const (
	networkCheckerName = "network"
)

// Regex patterns for network access and exfiltration detection.
var (
	// Network commands (Medium).
	networkCommandPattern = regexp.MustCompile(
		`\b(curl|wget|nc|ncat|socat)\b`)

	// DNS exfiltration (High).
	dnsExfilPattern = regexp.MustCompile(
		`(dig|nslookup|host)\s+.*\$[{(]?[A-Z_]`)

	// Reverse shell patterns (Critical).
	reverseShellBashPattern = regexp.MustCompile(
		`bash\s+-i\s+>&\s*/dev/tcp/`)
	reverseShellNcPattern = regexp.MustCompile(
		`\bnc\b.*-e\s*/bin/(ba)?sh`)
	reverseShellPythonPattern = regexp.MustCompile(
		`python[23]?\s+-c\s+.*socket.*connect`)

	// Encoded URL indicators (High).
	base64HTTPPattern = regexp.MustCompile(`aHR0c`) // base64 of "http"
)

// NetworkChecker scans script content for network access commands, encoded URLs,
// DNS exfiltration patterns, and reverse shell patterns.
type NetworkChecker struct{}

// NewNetworkChecker creates a NetworkChecker.
func NewNetworkChecker() *NetworkChecker { return &NetworkChecker{} }

// Name returns the checker identifier.
func (c *NetworkChecker) Name() string { return networkCheckerName }

// Category returns CategoryExfiltration.
func (c *NetworkChecker) Category() Category { return CategoryExfiltration }

// Check scans all scripts for network access patterns.
func (c *NetworkChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	for _, ref := range sc.AllScripts() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("network checker cancelled: %w", ctx.Err())
		default:
		}

		content := string(ref.Script)
		if content == "" {
			continue
		}

		reverseShellFindings := c.checkReverseShell(ref, content)
		findings = append(findings, reverseShellFindings...)
		findings = append(findings, c.checkDNSExfiltration(ref, content)...)
		findings = append(findings, c.checkEncodedURLs(ref, content)...)
		// Skip generic network command finding when a more specific reverse shell was found.
		if len(reverseShellFindings) == 0 {
			findings = append(findings, c.checkNetworkCommands(ref, content)...)
		}
	}

	return findings, nil
}

func (c *NetworkChecker) checkReverseShell(ref ScriptRef, content string) []Finding {
	var findings []Finding

	patterns := []struct {
		re   *regexp.Regexp
		desc string
	}{
		{reverseShellBashPattern, "bash reverse shell via /dev/tcp"},
		{reverseShellNcPattern, "netcat reverse shell with -e flag"},
		{reverseShellPythonPattern, "Python socket reverse shell"},
	}

	for _, p := range patterns {
		if p.re.MatchString(content) {
			findings = append(findings, Finding{
				Severity:       SeverityCritical,
				Category:       CategoryExfiltration,
				SurfaceID:      ref.SurfaceID,
				CheckerName:    networkCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Reverse shell pattern detected",
				Description:    fmt.Sprintf("Command %q contains a %s pattern", ref.CommandName, p.desc),
				Recommendation: "Remove the reverse shell command immediately; this is a strong indicator of compromise",
			})
		}
	}

	return findings
}

func (c *NetworkChecker) checkDNSExfiltration(ref ScriptRef, content string) []Finding {
	if !dnsExfilPattern.MatchString(content) {
		return nil
	}

	return []Finding{{
		Severity:       SeverityHigh,
		Category:       CategoryExfiltration,
		SurfaceID:      ref.SurfaceID,
		CheckerName:    networkCheckerName,
		FilePath:       ref.FilePath,
		Title:          "Possible DNS exfiltration pattern",
		Description:    fmt.Sprintf("Command %q uses DNS lookup commands with variable interpolation — may encode data in DNS queries", ref.CommandName),
		Recommendation: "Review the DNS lookups to ensure they are not encoding sensitive data in query names",
	}}
}

func (c *NetworkChecker) checkEncodedURLs(ref ScriptRef, content string) []Finding {
	if !base64HTTPPattern.MatchString(content) {
		return nil
	}

	return []Finding{{
		Severity:       SeverityHigh,
		Category:       CategoryExfiltration,
		SurfaceID:      ref.SurfaceID,
		CheckerName:    networkCheckerName,
		FilePath:       ref.FilePath,
		Title:          "Script contains encoded URL",
		Description:    fmt.Sprintf("Command %q contains base64-encoded HTTP URL indicator — may be hiding network destinations", ref.CommandName),
		Recommendation: "Decode and review the encoded content; replace with plain URLs",
	}}
}

func (c *NetworkChecker) checkNetworkCommands(ref ScriptRef, content string) []Finding {
	if !networkCommandPattern.MatchString(content) {
		return nil
	}

	return []Finding{{
		Severity:       SeverityMedium,
		Category:       CategoryExfiltration,
		SurfaceID:      ref.SurfaceID,
		CheckerName:    networkCheckerName,
		FilePath:       ref.FilePath,
		Title:          "Script uses network access command",
		Description:    fmt.Sprintf("Command %q uses curl, wget, nc, or similar network tools", ref.CommandName),
		Recommendation: "Verify that network access is expected for this command's purpose",
	}}
}
