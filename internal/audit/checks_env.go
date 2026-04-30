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
	// Includes invowk-specific runtime secrets (INVOWK_SSH_TOKEN, INVOWK_TUI_*).
	sensitiveVarPattern = regexp.MustCompile(
		`\$(AWS_SECRET_ACCESS_KEY|AWS_ACCESS_KEY_ID|AWS_SESSION_TOKEN|` +
			`GITHUB_TOKEN|GH_TOKEN|SSH_AUTH_SOCK|GPG_AGENT_INFO|` +
			`DATABASE_URL|REDIS_URL|MONGODB_URI|VAULT_TOKEN|VAULT_ADDR|` +
			`DOCKER_PASSWORD|NPM_TOKEN|PYPI_TOKEN|` +
			`AZURE_CLIENT_SECRET|AZURE_TENANT_ID|` +
			`GOOGLE_APPLICATION_CREDENTIALS|GCP_SERVICE_ACCOUNT_KEY|` +
			`INVOWK_SSH_TOKEN|INVOWK_TUI_TOKEN|INVOWK_TUI_ADDR)`)
	genericSecretPattern = regexp.MustCompile(
		`\$\{?(API_KEY|SECRET_KEY|PRIVATE_KEY|ACCESS_TOKEN|AUTH_TOKEN|PASSWORD)\}?`)

	// Token extraction to files or pipes (High): detects credential-to-sink
	// patterns using both generic and named credential variables. Kept in sync
	// with sensitiveVarPattern for any newly added credential names.
	tokenExtractionPattern = regexp.MustCompile(
		`\$(AWS_SECRET_ACCESS_KEY|AWS_ACCESS_KEY_ID|GITHUB_TOKEN|GH_TOKEN|` +
			`VAULT_TOKEN|DOCKER_PASSWORD|NPM_TOKEN|PYPI_TOKEN|` +
			`AZURE_CLIENT_SECRET|GCP_SERVICE_ACCOUNT_KEY|` +
			`INVOWK_SSH_TOKEN|INVOWK_TUI_TOKEN|` +
			`API_KEY|SECRET_KEY|TOKEN|PASSWORD|ACCESS_TOKEN)[^|>]*[|>]`)
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
		content := allScripts[i].Content()
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
		rt := &ref.Runtimes[i]

		if rt.EnvInheritMode == invowkfile.EnvInheritAll {
			findings = append(findings, Finding{
				Code:           codeEnvInheritAll,
				Severity:       SeverityLow,
				Category:       CategoryExfiltration,
				SurfaceID:      ref.SurfaceID,
				CheckerName:    envCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Command inherits all host environment variables",
				Description:    fmt.Sprintf("Command %q runtime %q uses env_inherit_mode: \"all\" — all host env vars including credentials are visible", ref.CommandName, rt.Name),
				Recommendation: "Use env_inherit_mode: \"allow\" with an explicit allowlist, or env_inherit_mode: \"none\"",
			})
		} else if rt.EnvInheritMode == "" && (rt.Name == invowkfile.RuntimeNative || rt.Name == invowkfile.RuntimeVirtual) {
			// Native and virtual runtimes default to EnvInheritAll when
			// env_inherit_mode is unset — flag the implicit inheritance.
			// Container runtimes are excluded: they follow a different env
			// path through buildContainerEnvMap with INVOWK_* filtering.
			findings = append(findings, Finding{
				Code:           codeEnvInheritDefaultAll,
				Severity:       SeverityInfo,
				Category:       CategoryExfiltration,
				SurfaceID:      ref.SurfaceID,
				CheckerName:    envCheckerName,
				FilePath:       ref.FilePath,
				Title:          "Command uses default env inheritance (all host variables)",
				Description:    fmt.Sprintf("Command %q runtime %q has no explicit env_inherit_mode — defaults to inheriting all host environment variables", ref.CommandName, rt.Name),
				Recommendation: "Set env_inherit_mode explicitly to document the intent: \"all\", \"allow\", or \"none\"",
			})
		}
	}

	return findings
}

func (c *EnvChecker) checkSensitiveVars(ref ScriptRef, content string) []Finding {
	var findings []Finding

	if sensitiveVarPattern.MatchString(content) {
		findings = append(findings, Finding{
			Code:           codeEnvSensitiveVar,
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
			Code:           codeEnvGenericSecret,
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
		Code:           codeEnvTokenExtraction,
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
