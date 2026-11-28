// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const containerMountCheckerName = "container-mount"

// ContainerMountChecker analyzes container workspace_mount, volumes, and SELinux
// settings across discovered invowkfiles and modules.
//
//goplint:ignore -- stateless checker strategy has no configuration invariants.
type ContainerMountChecker struct{}

// NewContainerMountChecker creates a ContainerMountChecker.
func NewContainerMountChecker() *ContainerMountChecker { return &ContainerMountChecker{} }

// Name returns the checker identifier.
func (c *ContainerMountChecker) Name() string { return containerMountCheckerName }

// Category returns CategoryPathTraversal as the primary category.
func (c *ContainerMountChecker) Category() Category { return CategoryPathTraversal }

// Check analyzes container mount configurations in all discovered surfaces.
func (c *ContainerMountChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	for _, sf := range sc.Invowkfiles() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("container-mount checker cancelled: %w", ctx.Err())
		default:
		}
		if sf.Invowkfile == nil {
			continue
		}
		findings = append(findings, c.checkInvowkfile(sf.SurfaceID, sf.SurfaceKind, sf.Path, false, sf.Invowkfile)...)
	}

	for _, sm := range sc.Modules() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("container-mount checker cancelled: %w", ctx.Err())
		default:
		}
		if sm.Invowkfile == nil {
			continue
		}
		filePath := sm.Invowkfile.FilePath
		if filePath == "" {
			filePath = sm.Path
		}
		findings = append(findings, c.checkInvowkfile(sm.SurfaceID, sm.SurfaceKind, filePath, true, sm.Invowkfile)...)
	}

	return findings, nil
}

//goplint:ignore -- audit checker inspects raw container mount specs and surface identifiers.
func (c *ContainerMountChecker) checkInvowkfile(
	surfaceID string,
	surfaceKind SurfaceKind,
	filePath types.FilesystemPath,
	isModule bool,
	inv *invowkfile.Invowkfile,
) []Finding {
	var findings []Finding

	for i := range inv.Commands {
		cmd := &inv.Commands[i]
		for j := range cmd.Implementations {
			impl := &cmd.Implementations[j]
			rawWD := effectiveRawWorkDir(inv, cmd, impl)
			for k := range impl.Runtimes {
				rt := &impl.Runtimes[k]
				if rt.Name != invowkfile.RuntimeContainer {
					continue
				}
				findings = append(findings, checkContainerRuntimeMounts(surfaceID, surfaceKind, filePath, isModule, cmd.Name, rawWD, rt)...)
			}
		}
	}

	return findings
}

//goplint:ignore -- audit checker inspects raw container mount specs and finding identifiers.
func checkContainerRuntimeMounts(
	surfaceID string,
	surfaceKind SurfaceKind,
	filePath types.FilesystemPath,
	isModule bool,
	cmdName invowkfile.CommandName,
	rawWD string,
	rt *invowkfile.RuntimeConfig,
) []Finding {
	var findings []Finding

	for _, vol := range rt.Volumes {
		source := extractContainerVolumeSource(string(vol))
		if isModule && isHostAbsoluteVolumeSource(source) {
			findings = append(findings, Finding{
				Code:           codeContainerModuleAbsoluteVolume,
				Severity:       SeverityHigh,
				Category:       CategoryPathTraversal,
				SurfaceID:      surfaceID,
				SurfaceKind:    surfaceKind,
				CheckerName:    containerMountCheckerName,
				FilePath:       filePath,
				Title:          "Module container declares absolute host bind mount",
				Description:    fmt.Sprintf("Command %q in module %q declares container volume %q using an absolute host path, which is blocked at runtime for modules", cmdName, surfaceID, vol),
				Recommendation: "Use a named container volume or a relative host path within the module's allowed workspace ceiling",
			})
		}
		if !isModule && isStandaloneSensitiveVolumeSource(source) {
			findings = append(findings, Finding{
				Code:           codeContainerStandaloneSensitiveMount,
				Severity:       SeverityHigh,
				Category:       CategoryPathTraversal,
				SurfaceID:      surfaceID,
				SurfaceKind:    surfaceKind,
				CheckerName:    containerMountCheckerName,
				FilePath:       filePath,
				Title:          "Standalone container mounts sensitive host path",
				Description:    fmt.Sprintf("Command %q declares container volume %q targeting sensitive host path %q", cmdName, vol, source),
				Recommendation: "Mount a narrow project directory instead of host system directories, container sockets, or SSH credentials",
			})
		}
		if hasParentTraversalSegment(source) {
			sev := SeverityMedium
			if surfaceKind == SurfaceKindVendoredModule || surfaceKind == SurfaceKindGlobalModule {
				sev = SeverityHigh
			}
			findings = append(findings, Finding{
				Code:           codeContainerMountParentTraversal,
				Severity:       sev,
				Category:       CategoryPathTraversal,
				SurfaceID:      surfaceID,
				SurfaceKind:    surfaceKind,
				CheckerName:    containerMountCheckerName,
				FilePath:       filePath,
				Title:          "Container volume uses parent directory traversal",
				Description:    fmt.Sprintf("Command %q declares container volume %q containing '..' parent directory traversal", cmdName, vol),
				Recommendation: "Keep container bind mounts inside the module directory or configured workspace_root ceiling",
			})
		}
	}

	if rt.WorkspaceMount != nil {
		if rt.WorkspaceMount.EffectiveRoot() == invowkfile.WorkspaceMountRootWorkdir && hasParentTraversalSegment(rawWD) {
			findings = append(findings, Finding{
				Code:           codeContainerWorkdirRootTraversal,
				Severity:       SeverityHigh,
				Category:       CategoryPathTraversal,
				SurfaceID:      surfaceID,
				SurfaceKind:    surfaceKind,
				CheckerName:    containerMountCheckerName,
				FilePath:       filePath,
				Title:          "Container workspace_mount root=workdir uses parent traversal",
				Description:    fmt.Sprintf("Command %q sets workspace_mount.root=\"workdir\" with workdir %q containing '..', attempting to mount a parent directory at /workspace", cmdName, rawWD),
				Recommendation: "Avoid '..' parent traversal in module workdir when workspace_mount.root is \"workdir\"",
			})
		}
		if rt.WorkspaceMount.EffectiveSELinuxLabel() == invowkfile.WorkspaceMountSELinuxNone {
			findings = append(findings, Finding{
				Code:           codeContainerSELinuxLabelNone,
				Severity:       SeverityLow,
				Category:       CategoryExecution,
				SurfaceID:      surfaceID,
				SurfaceKind:    surfaceKind,
				CheckerName:    containerMountCheckerName,
				FilePath:       filePath,
				Title:          "Container workspace_mount disables SELinux labeling",
				Description:    fmt.Sprintf("Command %q sets workspace_mount.selinux_label=\"none\", omitting container MCS labels on /workspace", cmdName),
				Recommendation: "Use \"auto\" (default), \"shared\", or \"private\" so SELinux hosts enforce label confinement",
			})
		}
	}

	return findings
}

//goplint:ignore -- audit helper reads raw invowkfile workdir syntax.
func effectiveRawWorkDir(inv *invowkfile.Invowkfile, cmd *invowkfile.Command, impl *invowkfile.Implementation) string {
	if impl != nil && impl.WorkDir != "" {
		return string(impl.WorkDir)
	}
	if cmd != nil && cmd.WorkDir != "" {
		return string(cmd.WorkDir)
	}
	if inv != nil && inv.WorkDir != "" {
		return string(inv.WorkDir)
	}
	return ""
}

//goplint:ignore -- audit helper parses raw container volume source prefix.
func extractContainerVolumeSource(spec string) string {
	trimmed := strings.TrimSpace(spec)
	if len(trimmed) >= 3 &&
		((trimmed[0] >= 'a' && trimmed[0] <= 'z') || (trimmed[0] >= 'A' && trimmed[0] <= 'Z')) &&
		trimmed[1] == ':' && (trimmed[2] == '/' || trimmed[2] == '\\') {
		if idx := strings.IndexByte(trimmed[2:], ':'); idx >= 0 {
			return trimmed[:2+idx]
		}
		return trimmed
	}
	source, _, _ := strings.Cut(trimmed, ":")
	return source
}

//goplint:ignore -- audit helper classifies raw volume source string.
func isHostAbsoluteVolumeSource(source string) bool {
	if source == "" {
		return false
	}
	if len(source) >= 2 &&
		((source[0] >= 'a' && source[0] <= 'z') || (source[0] >= 'A' && source[0] <= 'Z')) &&
		source[1] == ':' {
		return true
	}
	return strings.HasPrefix(source, "/") || strings.HasPrefix(source, `\`)
}

//goplint:ignore -- audit helper classifies raw volume source string against sensitive host paths.
func isStandaloneSensitiveVolumeSource(source string) bool {
	if source == "" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(source), `\`, "/"))
	if strings.HasPrefix(normalized, "~/") || strings.HasPrefix(normalized, "$home/") || strings.HasPrefix(normalized, "${home}/") {
		if strings.Contains(normalized, "/.ssh") {
			return true
		}
	}
	if !strings.HasPrefix(normalized, "/") {
		return false
	}
	cleaned := path.Clean(normalized)
	if cleaned == "/" {
		return true
	}
	sensitiveRoots := []string{
		"/etc", "/proc", "/sys", "/dev", "/root",
		"/var/run", "/run/podman", "/run/docker.sock",
	}
	for _, root := range sensitiveRoots {
		if cleaned == root || strings.HasPrefix(cleaned, root+"/") {
			return true
		}
	}
	return cleaned == "/.ssh" || strings.HasSuffix(cleaned, "/.ssh") || strings.Contains(cleaned, "/.ssh/")
}

//goplint:ignore -- audit helper scans raw path string for '..' traversal segments.
func hasParentTraversalSegment(raw string) bool {
	normalized := strings.ReplaceAll(raw, `\`, "/")
	return slices.Contains(strings.Split(normalized, "/"), "..")
}
