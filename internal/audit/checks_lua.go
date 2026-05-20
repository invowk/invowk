// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const luaCheckerName = "lua"

var (
	luaDisabledAPIPattern  = regexp.MustCompile(`\b(os\.execute|io\.popen|package\.loadlib|debug\b|dofile\s*\(|loadfile\s*\(|golib\b)`)
	luaGetenvPattern       = regexp.MustCompile(`os\.getenv\s*\(\s*["']([^"']+)["']\s*\)`)
	luaEnvIndexPattern     = regexp.MustCompile(`invowk\.env\.([A-Za-z_][A-Za-z0-9_]*)`)
	networkAllowedBinaries = map[string]bool{
		"curl": true, "wget": true, "fetch": true,
		"nc": true, "ncat": true, "netcat": true,
		"ssh": true, "scp": true, "sftp": true, "rsync": true,
	}
)

// LuaChecker analyzes virtual-lua bridge usage and runtime configuration.
//
//goplint:ignore -- stateless checker strategy has no configuration invariants.
type LuaChecker struct{}

// NewLuaChecker creates a LuaChecker.
func NewLuaChecker() *LuaChecker { return &LuaChecker{} }

// Name returns the checker identifier.
func (c *LuaChecker) Name() string { return luaCheckerName }

// Category returns CategoryExecution as the primary category.
func (c *LuaChecker) Category() Category { return CategoryExecution }

// Check analyzes virtual-lua scripts and runtime configuration.
func (c *LuaChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding
	allScripts := sc.AllScripts()
	for i := range allScripts {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("lua checker cancelled: %w", ctx.Err())
		default:
		}

		ref := allScripts[i]
		if !isVirtualLuaScript(ref) {
			continue
		}
		content := ref.Content()
		if content != "" {
			findings = append(findings, c.checkDisabledAPIs(ref, content)...)
			findings = append(findings, c.checkSensitiveEnvReads(ref, content)...)
		}
		findings = append(findings, c.checkRuntimeConfig(ref)...)
		findings = append(findings, c.checkAllowedPaths(ref)...)
	}
	return findings, nil
}

func isVirtualLuaScript(ref ScriptRef) bool {
	for i := range ref.Runtimes {
		if ref.Runtimes[i].Name == invowkfile.RuntimeVirtualLua {
			return true
		}
	}
	return false
}

func (c *LuaChecker) checkDisabledAPIs(ref ScriptRef, content string) []Finding {
	match := luaDisabledAPIPattern.FindString(content)
	if match == "" {
		return nil
	}
	return []Finding{{
		Code:           codeLuaDisabledAPI,
		Severity:       SeverityLow,
		Category:       CategoryExecution,
		SurfaceID:      ref.SurfaceID,
		SurfaceKind:    ref.SurfaceKind,
		CheckerName:    luaCheckerName,
		FilePath:       refFindingPath(ref),
		Line:           lineOf(content, match),
		Title:          "Lua script references disabled Lua API",
		Description:    fmt.Sprintf("Command %q references %q, which virtual-lua disables or replaces with the Invowk bridge", ref.CommandName, match),
		Recommendation: "Use invowk.cmd, invowk.capture, invowk.path, and the path-validated io bridge instead of disabled Lua APIs",
	}}
}

func (c *LuaChecker) checkSensitiveEnvReads(ref ScriptRef, content string) []Finding {
	names := sensitiveLuaEnvNames(content)
	if len(names) == 0 {
		return nil
	}
	return []Finding{{
		Code:           codeLuaSensitiveEnvRead,
		Severity:       SeverityMedium,
		Category:       CategoryExfiltration,
		SurfaceID:      ref.SurfaceID,
		SurfaceKind:    ref.SurfaceKind,
		CheckerName:    luaCheckerName,
		FilePath:       refFindingPath(ref),
		Line:           lineOf(content, names[0]),
		Title:          "Virtual-lua script reads sensitive environment variable",
		Description:    fmt.Sprintf("Command %q reads sensitive environment variable(s): %s", ref.CommandName, strings.Join(names, ", ")),
		Recommendation: "Use env_inherit_mode: \"allow\" with the minimum variable set, or pass a scoped non-secret value explicitly",
	}}
}

func sensitiveLuaEnvNames(content string) []string {
	var names []string
	for _, match := range luaGetenvPattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && isSensitiveEnvName(match[1]) && !containsString(names, match[1]) {
			names = append(names, match[1])
		}
	}
	for _, match := range luaEnvIndexPattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && isSensitiveEnvName(match[1]) && !containsString(names, match[1]) {
			names = append(names, match[1])
		}
	}
	return names
}

func (c *LuaChecker) checkRuntimeConfig(ref ScriptRef) []Finding {
	var findings []Finding
	for i := range ref.Runtimes {
		rt := ref.Runtimes[i]
		if rt.Name != invowkfile.RuntimeVirtualLua {
			continue
		}
		for _, binary := range rt.AllowedBinaries {
			value := binary.String()
			if value == "*" {
				findings = append(findings, Finding{
					Code:           codeLuaHostBinaryWildcard,
					Severity:       SeverityHigh,
					Category:       CategoryExecution,
					SurfaceID:      ref.SurfaceID,
					SurfaceKind:    ref.SurfaceKind,
					CheckerName:    luaCheckerName,
					FilePath:       ref.FilePath,
					Title:          "Virtual-lua allows all host binaries",
					Description:    fmt.Sprintf("Command %q sets allowed_binaries: [\"*\"], so Lua bridge calls can run any host executable as a native process", ref.CommandName),
					Recommendation: "Replace the wildcard with a short explicit allowed_binaries list or move the command to the container runtime for isolation",
				})
				continue
			}
			if networkAllowedBinaries[strings.ToLower(filepath.Base(value))] {
				findings = append(findings, Finding{
					Code:           codeLuaNetworkAllowedBinary,
					Severity:       SeverityMedium,
					Category:       CategoryExfiltration,
					SurfaceID:      ref.SurfaceID,
					SurfaceKind:    ref.SurfaceKind,
					CheckerName:    luaCheckerName,
					FilePath:       ref.FilePath,
					Title:          "Virtual-lua allows network-capable host binary",
					Description:    fmt.Sprintf("Command %q allows host binary %q; Lua bridge code can use it for network access or exfiltration", ref.CommandName, value),
					Recommendation: "Allow only the exact non-network helper required, or constrain network behavior in a container runtime",
				})
			}
		}
	}
	return findings
}

func (c *LuaChecker) checkAllowedPaths(ref ScriptRef) []Finding {
	for name, value := range ref.AllowedPaths {
		for _, raw := range allowedPathRawValues(value) {
			if luaPathMappingIsBroad(raw) {
				return []Finding{{
					Code:           codeLuaBroadPathMapping,
					Severity:       SeverityMedium,
					Category:       CategoryPathTraversal,
					SurfaceID:      ref.SurfaceID,
					SurfaceKind:    ref.SurfaceKind,
					CheckerName:    luaCheckerName,
					FilePath:       ref.FilePath,
					Title:          "Virtual-lua exposes broad allowed path mapping",
					Description:    fmt.Sprintf("Command %q maps allowed_paths.%s to %q, giving Lua file APIs broad host filesystem reach", ref.CommandName, name, raw),
					Recommendation: "Map allowed_paths to a narrow project or module subdirectory instead of home, root, or traversal-capable paths",
				}}
			}
		}
	}
	return nil
}

func allowedPathRawValues(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case map[string]string:
		values := make([]string, 0, len(typed))
		for _, raw := range typed {
			values = append(values, raw)
		}
		return values
	case map[invowkfile.PlatformType]string:
		values := make([]string, 0, len(typed))
		for _, raw := range typed {
			values = append(values, raw)
		}
		return values
	default:
		return nil
	}
}

func luaPathMappingIsBroad(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "/" || trimmed == `\` || trimmed == "." || trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "@home") || strings.Contains(trimmed, "../") || strings.Contains(trimmed, `..\`) {
		return true
	}
	return false
}

func isSensitiveEnvName(name string) bool {
	if sensitiveVarPattern.MatchString("$"+name) || genericSecretPattern.MatchString("$"+name) {
		return true
	}
	return strings.Contains(name, "TOKEN") || strings.Contains(name, "SECRET") || strings.Contains(name, "PASSWORD")
}

func refFindingPath(ref ScriptRef) types.FilesystemPath {
	if ref.ScriptPath != "" {
		return ref.ScriptPath
	}
	return ref.FilePath
}

func lineOf(content, needle string) int {
	before, _, found := strings.Cut(content, needle)
	if !found {
		return 0
	}
	return strings.Count(before, "\n") + 1
}

func containsString(values []string, needle string) bool {
	return slices.Contains(values, needle)
}
