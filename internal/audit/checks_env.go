// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"regexp"

	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	envCheckerName = "env"
)

// Patterns for sensitive environment variable detection.
var (
	// Credential variables (Medium, escalated to High when correlated with network).
	sensitiveVarPattern = regexp.MustCompile(
		`\$(AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|GITHUB_TOKEN|GH_TOKEN|` +
			`SSH_AUTH_SOCK|GPG_AGENT_INFO|DATABASE_URL|REDIS_URL|MONGODB_URI)`)
	genericSecretPattern = regexp.MustCompile(
		`\$\{?(API_KEY|SECRET_KEY|PRIVATE_KEY|ACCESS_TOKEN|AUTH_TOKEN|PASSWORD)\}?`)

	// Token extraction to files or pipes (High).
	tokenExtractionPattern = regexp.MustCompile(
		`\$(API_KEY|SECRET_KEY|TOKEN|PASSWORD|ACCESS_TOKEN)[^|>]*[|>]`)
)

// EnvChecker scans environment configurations and script content for sensitive
// variable access, risky inheritance modes, and credential extraction patterns.
type EnvChecker struct{}

// NewEnvChecker creates an EnvChecker.
func NewEnvChecker() *EnvChecker { return &EnvChecker{} }

// Name returns the checker identifier.
func (c *EnvChecker) Name() string { return envCheckerName }

// Category returns CategoryExfiltration.
func (c *EnvChecker) Category() Category { return CategoryExfiltration }

// Check analyzes env configs and script content for credential risks.
func (c *EnvChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	// Check runtime env inheritance mode.
	allScripts := sc.AllScripts()
	for i := range allScripts {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("env checker cancelled: %w", ctx.Err())
		default:
		}

		findings = append(findings, c.checkEnvInheritMode(allScripts[i])...)

		// Scan script content for sensitive variable access.
		content := string(allScripts[i].Script)
		if content != "" {
			findings = append(findings, c.checkSensitiveVars(allScripts[i], content)...)
			findings = append(findings, c.checkTokenExtraction(allScripts[i], content)...)
		}
	}

	return findings, nil
}

func (c *EnvChecker) checkEnvInheritMode(ref ScriptRef) []Finding {
	var findings []Finding

	for i := range ref.Runtimes {
		if ref.Runtimes[i].EnvInheritMode == invowkfile.EnvInheritAll {
			findings = append(findings, Finding{
				Severity:       SeverityLow,
				Category:       CategoryExfiltration,
				SurfaceID:      ref.SurfaceID,
				CheckerName:    envCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Command inherits all host environment variables",
				Description:    fmt.Sprintf("Command %q runtime %q uses env_inherit_mode: \"all\" — all host env vars including credentials are visible", ref.CommandName, ref.Runtimes[i].Name),
				Recommendation: "Use env_inherit_mode: \"allow\" with an explicit allowlist, or env_inherit_mode: \"none\"",
			})
		}
	}

	return findings
}

func (c *EnvChecker) checkSensitiveVars(ref ScriptRef, content string) []Finding {
	var findings []Finding

	if sensitiveVarPattern.MatchString(content) {
		findings = append(findings, Finding{
			Severity:       SeverityMedium,
			Category:       CategoryExfiltration,
			SurfaceID:      ref.SurfaceID,
			CheckerName:    envCheckerName,
			FilePath:       ref.FilePath,
			Title:          "Script accesses sensitive environment variable",
			Description:    fmt.Sprintf("Command %q references known credential environment variables (AWS, GitHub, SSH, database)", ref.CommandName),
			Recommendation: "Ensure credential access is intentional; prefer scoped tokens over broad credential variables",
		})
	}

	if genericSecretPattern.MatchString(content) {
		findings = append(findings, Finding{
			Severity:       SeverityMedium,
			Category:       CategoryExfiltration,
			SurfaceID:      ref.SurfaceID,
			CheckerName:    envCheckerName,
			FilePath:       ref.FilePath,
			Title:          "Script accesses generic secret variable",
			Description:    fmt.Sprintf("Command %q references generic secret variable names (API_KEY, SECRET_KEY, PASSWORD, etc.)", ref.CommandName),
			Recommendation: "Review whether these credentials should be accessible to this command",
		})
	}

	return findings
}

func (c *EnvChecker) checkTokenExtraction(ref ScriptRef, content string) []Finding {
	if !tokenExtractionPattern.MatchString(content) {
		return nil
	}

	return []Finding{{
		Severity:       SeverityHigh,
		Category:       CategoryExfiltration,
		SurfaceID:      ref.SurfaceID,
		CheckerName:    envCheckerName,
		FilePath:       ref.FilePath,
		Title:          "Script may extract credential to external sink",
		Description:    fmt.Sprintf("Command %q reads a credential variable and pipes or redirects it — potential credential exfiltration", ref.CommandName),
		Recommendation: "Remove the credential piping/redirection; if needed, use a secure credential manager",
	}}
}
