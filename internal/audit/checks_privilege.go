// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	privilegeCheckerName = "privilege"

	worldWriteMask = 0o002
	groupWriteMask = 0o020

	writableRiskNone  writableRisk = 0
	writableRiskGroup writableRisk = 1
	writableRiskWorld writableRisk = 2
)

type (
	writableRisk uint8

	// PrivilegeChecker flags non-owner-writable definitions, script files, and
	// dotenv files referenced by commands declaring the "elevated" capability.
	//
	//goplint:ignore -- checker strategy uses default OS classifier unless overridden in tests.
	PrivilegeChecker struct {
		isSharedGroup func(os.FileInfo) bool
	}

	elevatedFileTarget struct {
		filePath     types.FilesystemPath //goplint:ignore -- resolved target path is always populated.
		kind         string               //goplint:ignore -- human-readable artifact kind label for finding descriptions.
		commandLabel string               //goplint:ignore -- command name or "(root)" fallback for finding descriptions.
	}

	privilegeSurface struct {
		surfaceID   string //goplint:ignore -- scan surface display identity from ScanContext.
		surfaceKey  ScanSurfaceKey
		surfaceKind SurfaceKind
		invPath     types.FilesystemPath //goplint:ignore -- invowkfile path is always populated for scanned surfaces.
		modulePath  types.FilesystemPath //goplint:ignore -- empty for standalone invowkfiles, module dir for modules.
		invowkfile  *invowkfile.Invowkfile
		isModule    bool
	}
)

// NewPrivilegeChecker creates a PrivilegeChecker.
func NewPrivilegeChecker() *PrivilegeChecker {
	return &PrivilegeChecker{
		isSharedGroup: isUnixSharedGroup,
	}
}

func newPrivilegeCheckerWithGroupCheck(fn func(os.FileInfo) bool) *PrivilegeChecker {
	c := NewPrivilegeChecker()
	if fn != nil {
		c.isSharedGroup = fn
	}
	return c
}

func (r writableRisk) String() string {
	switch r {
	case writableRiskNone:
		return "none"
	case writableRiskGroup:
		return "group"
	case writableRiskWorld:
		return "world"
	default:
		return fmt.Sprintf("unknown(%d)", r)
	}
}

func (r writableRisk) Validate() error {
	switch r {
	case writableRiskNone, writableRiskGroup, writableRiskWorld:
		return nil
	default:
		return fmt.Errorf("invalid writable risk %s", r)
	}
}

func (t elevatedFileTarget) Validate() error {
	if strings.TrimSpace(t.commandLabel) == "" {
		return errors.Join(t.filePath.Validate(), errors.New("elevated file target requires a command label"))
	}
	return t.filePath.Validate()
}

func (s privilegeSurface) Validate() error {
	var errs []error
	errs = append(errs, s.surfaceKey.Validate(), s.surfaceKind.Validate(), s.invPath.Validate())
	if s.modulePath != "" {
		errs = append(errs, s.modulePath.Validate())
	}
	return errors.Join(errs...)
}

// Name returns the checker identifier.
func (c *PrivilegeChecker) Name() string { return privilegeCheckerName }

// Category returns CategoryIntegrity.
func (c *PrivilegeChecker) Category() Category { return CategoryIntegrity }

// Check inspects standalone invowkfiles and modules for elevated commands that
// rely on group-writable or world-writable definition, script, or env files.
func (c *PrivilegeChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	for _, sf := range sc.Invowkfiles() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("privilege checker cancelled: %w", ctx.Err())
		default:
		}
		if sf.Invowkfile == nil {
			continue
		}
		surface := privilegeSurface{
			surfaceID:   sf.SurfaceID,
			surfaceKey:  sf.SurfaceKey,
			surfaceKind: sf.SurfaceKind,
			invPath:     sf.Path,
			invowkfile:  sf.Invowkfile,
		}
		findings = append(findings, c.checkSurface(surface)...)
	}

	for _, sm := range sc.Modules() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("privilege checker cancelled: %w", ctx.Err())
		default:
		}
		if sm.Invowkfile == nil {
			continue
		}
		surface := privilegeSurface{
			surfaceID:   sm.SurfaceID,
			surfaceKey:  sm.SurfaceKey,
			surfaceKind: sm.SurfaceKind,
			invPath:     fspath.JoinStr(sm.Path, invowkfileCUEFileName),
			modulePath:  sm.Path,
			invowkfile:  sm.Invowkfile,
			isModule:    true,
		}
		findings = append(findings, c.checkSurface(surface)...)
	}

	return findings, nil
}

func (c *PrivilegeChecker) checkSurface(surface privilegeSurface) []Finding {
	targets := collectElevatedFileTargets(surface)
	if len(targets) == 0 {
		return nil
	}

	var findings []Finding
	for _, target := range targets {
		finding, ok := c.checkTargetPermissions(surface, target)
		if ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func (c *PrivilegeChecker) checkTargetPermissions(surface privilegeSurface, target elevatedFileTarget) (Finding, bool) {
	risk, mode := classifyFileWritableRisk(string(target.filePath), c.isSharedGroup)
	switch risk {
	case writableRiskWorld:
		return Finding{
			Code:           codePrivilegeWorldWritableFile,
			Severity:       SeverityHigh,
			Category:       CategoryIntegrity,
			SurfaceID:      surface.surfaceID,
			SurfaceKey:     surface.surfaceKey,
			SurfaceKind:    surface.surfaceKind,
			CheckerName:    privilegeCheckerName,
			FilePath:       target.filePath,
			Title:          "Elevated command references world-writable file",
			Description:    fmt.Sprintf("File %q (%s, mode %04o) is world-writable and used by elevated command %q — unprivileged users could modify it to tamper with elevated execution", target.filePath, target.kind, mode, target.commandLabel),
			Recommendation: "Remove world and group write permissions (e.g., chmod o-w,g-w) so only the owner can modify files used by elevated commands",
		}, true
	case writableRiskGroup:
		return Finding{
			Code:           codePrivilegeGroupWritableFile,
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      surface.surfaceID,
			SurfaceKey:     surface.surfaceKey,
			SurfaceKind:    surface.surfaceKind,
			CheckerName:    privilegeCheckerName,
			FilePath:       target.filePath,
			Title:          "Elevated command references shared-group-writable file",
			Description:    fmt.Sprintf("File %q (%s, mode %04o) is writable by a shared group and used by elevated command %q — other group members could tamper with elevated execution", target.filePath, target.kind, mode, target.commandLabel),
			Recommendation: "Remove group write permissions (e.g., chmod g-w) or restrict the file to a single-user private group",
		}, true
	case writableRiskNone:
		return Finding{}, false
	default:
		return Finding{}, false
	}
}

func collectElevatedFileTargets(surface privilegeSurface) []elevatedFileTarget {
	inv := surface.invowkfile
	if inv == nil {
		return nil
	}

	rootElevated := dependsOnHasCapability(inv.DependsOn, invowkfile.CapabilityElevated)
	var firstElevatedCmd string
	if rootElevated && len(inv.Commands) > 0 {
		firstElevatedCmd = string(inv.Commands[0].Name)
	}

	var targets []elevatedFileTarget
	seen := make(map[string]bool)
	addTarget := func(rawPath types.FilesystemPath, kind, cmdLabel string) {
		if rawPath == "" {
			return
		}
		cleaned := filepath.Clean(string(rawPath))
		if seen[cleaned] {
			return
		}
		seen[cleaned] = true
		targets = append(targets, elevatedFileTarget{
			filePath:     types.FilesystemPath(cleaned), //goplint:ignore -- cleaned filesystem path from scan context.
			kind:         kind,
			commandLabel: cmdLabel,
		})
	}

	basePath := string(surface.modulePath)
	if basePath == "" {
		basePath = filepath.Dir(string(surface.invPath))
	}
	addEnvFiles := func(env *invowkfile.EnvConfig, cmdLabel string) {
		if env == nil {
			return
		}
		for _, envFile := range env.Files {
			if resolved := resolveAuditEnvFile(string(envFile), basePath, string(surface.modulePath)); resolved != "" {
				addTarget(types.FilesystemPath(resolved), "env.files", cmdLabel) //goplint:ignore -- resolved dotenv path within surface scope.
			}
		}
	}

	anyElevated := rootElevated
	for ci := range inv.Commands {
		cmd := &inv.Commands[ci]
		cmdElevated := rootElevated || dependsOnHasCapability(cmd.DependsOn, invowkfile.CapabilityElevated)
		cmdAnyElevated := cmdElevated

		for ii := range cmd.Implementations {
			impl := &cmd.Implementations[ii]
			implElevated := cmdElevated || dependsOnHasCapability(impl.DependsOn, invowkfile.CapabilityElevated)
			if !implElevated {
				continue
			}
			cmdAnyElevated = true
			anyElevated = true
			if firstElevatedCmd == "" {
				firstElevatedCmd = string(cmd.Name)
			}

			if impl.Script.IsFile() {
				if scriptTarget := resolveAuditScriptFile(impl, surface.invPath, surface.modulePath); scriptTarget != "" {
					addTarget(scriptTarget, "script.file", string(cmd.Name))
				}
			}
			addEnvFiles(impl.Env, string(cmd.Name))
		}

		if cmdAnyElevated {
			anyElevated = true
			if firstElevatedCmd == "" {
				firstElevatedCmd = string(cmd.Name)
			}
			addEnvFiles(cmd.Env, string(cmd.Name))
		}
	}

	if !anyElevated {
		return nil
	}

	if firstElevatedCmd == "" {
		firstElevatedCmd = "(root)"
	}
	addTarget(surface.invPath, invowkfileCUEFileName, firstElevatedCmd)
	if surface.isModule {
		addTarget(fspath.JoinStr(surface.modulePath, invowkmodCueFileName), invowkmodCueFileName, firstElevatedCmd)
	}
	addEnvFiles(inv.Env, firstElevatedCmd)

	return targets
}

func dependsOnHasCapability(deps *invowkfile.DependsOn, target invowkfile.CapabilityName) bool {
	if deps == nil {
		return false
	}
	for _, capDep := range deps.Capabilities {
		if slices.Contains(capDep.Alternatives, target) {
			return true
		}
	}
	return false
}

func resolveAuditScriptFile(impl *invowkfile.Implementation, invPath, modulePath types.FilesystemPath) types.FilesystemPath {
	scriptTarget := impl.GetScriptFilePathWithModule(invPath, modulePath)
	if scriptTarget == "" {
		return ""
	}
	if modulePath != "" && !isContainedInModule(string(scriptTarget), string(modulePath)) {
		return ""
	}
	return scriptTarget
}

//goplint:ignore -- resolves raw dotenv declaration strings into OS-native paths for permission inspection.
func resolveAuditEnvFile(rawEnvFile, basePath, moduleRoot string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(rawEnvFile), "?")
	if trimmed == "" {
		return ""
	}
	if moduleRoot != "" {
		if isAuditAbsoluteOnAnyOS(trimmed) {
			return ""
		}
		joined := filepath.Join(basePath, filepath.FromSlash(trimmed))
		if !isContainedInModule(joined, moduleRoot) {
			return ""
		}
		return joined
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(basePath, filepath.FromSlash(trimmed))
}

//goplint:ignore -- containment helper checks raw OS-native path strings inside module boundaries.
func isContainedInModule(targetPath, moduleRoot string) bool {
	rel, err := filepath.Rel(moduleRoot, targetPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

//goplint:ignore -- inspects raw dotenv path declaration for cross-OS absolute prefixes.
func isAuditAbsoluteOnAnyOS(p string) bool {
	if p == "" {
		return false
	}
	if p[0] == '/' || p[0] == '\\' {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return filepath.IsAbs(p)
}
