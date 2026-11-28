// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// MountTrustTierContained restricts container workspace/volume mounts to the
	// declaring module's physical root (Layer 1: vendored, remote, ~/.invowk/cmds, provisioned).
	MountTrustTierContained MountTrustTier = "contained"
	// MountTrustTierLocalCwd allows container workspace/volume mounts within the
	// current working directory ceiling (Layer 2: <cwd>/invowkfile.cue and <cwd>/<m>.invowkmod).
	MountTrustTierLocalCwd MountTrustTier = "local-cwd"
	// MountTrustTierInclude allows container workspace/volume mounts within the
	// module root or explicit operator-configured IncludeEntry.workspace_root (Layer 3).
	MountTrustTierInclude MountTrustTier = "include"

	containerInvowkMountRoot    = "/.invowk"
	containerInvowkScriptsDir   = containerInvowkMountRoot + "/scripts"
	containerInvowkTmpDir       = containerInvowkMountRoot + "/tmp"
	containerVolumeKindNamed    = "named"
	containerVolumeKindRelative = "relative"
	containerVolumeKindAbsolute = "absolute"
)

var (
	// ErrInvalidMountTrustTier is returned when a MountTrustTier value is not recognized.
	ErrInvalidMountTrustTier = errors.New("invalid mount trust tier")
	// ErrContainerMountPolicy is the sentinel error wrapped by container mount containment denials.
	ErrContainerMountPolicy = errors.New("container mount policy violation")
	// ErrContainerModuleAbsoluteVolume is returned when a *.invowkmod module declares an absolute host bind-mount.
	ErrContainerModuleAbsoluteVolume = errors.New("module absolute host volume mount is not allowed")
	// ErrContainerWorkDirNotMounted is returned when container workdir maps to an unmounted host path.
	ErrContainerWorkDirNotMounted = errors.New("container workdir is not mounted")

	// namedVolumePattern matches Docker/Podman named volumes (never path separators or leading '.').
	namedVolumePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

type (
	// MountTrustTier classifies the 3-tier containment policy for container mounts.
	MountTrustTier string

	// InvalidMountTrustTierError is returned when a MountTrustTier is unrecognized.
	InvalidMountTrustTierError struct {
		Value MountTrustTier
	}

	// ContainerMountPlan captures resolved workspace and volume mounts for a container execution.
	ContainerMountPlan struct {
		WorkspaceEnabled      bool
		WorkspaceHostRoot     types.FilesystemPath      //goplint:ignore -- optional nonzero path validated when non-empty.
		WorkspaceTarget       container.MountTargetPath //goplint:ignore -- optional nonzero path validated when non-empty.
		WorkspaceReadonly     bool
		WorkspaceSELinuxLabel invowkfile.WorkspaceMountSELinuxLabel
		WorkspaceVolumeSpec   container.VolumeMountSpec //goplint:ignore -- optional nonzero mount spec validated when non-empty.
		ResolvedVolumes       []ResolvedContainerVolume
		WorkDir               container.MountTargetPath //goplint:ignore -- optional nonzero path validated when non-empty.
	}

	// ResolvedContainerVolume describes one resolved volume mount entry from RuntimeConfig.Volumes.
	ResolvedContainerVolume struct {
		RawSpec         container.VolumeMountSpec //goplint:ignore -- validated in Validate().
		ResolvedSpec    container.VolumeMountSpec //goplint:ignore -- validated in Validate().
		SourceKind      string                    //goplint:ignore -- internal volume classifier ("named", "relative", "absolute").
		HostPath        types.FilesystemPath      //goplint:ignore -- optional nonzero path validated when non-empty.
		ContainerTarget container.MountTargetPath //goplint:ignore -- validated in Validate().
		Options         string                    //goplint:ignore -- raw Docker/Podman mount options suffix (e.g., "ro,z").
	}

	//goplint:ignore -- internal parsed Docker/Podman mount components.
	parsedVolumeMount struct {
		sourceKind      string
		source          string
		containerTarget string
		options         string
	}
)

// Error implements the error interface for InvalidMountTrustTierError.
func (e *InvalidMountTrustTierError) Error() string {
	return fmt.Sprintf("invalid mount trust tier %q (valid: contained, local-cwd, include)", e.Value)
}

// Unwrap returns ErrInvalidMountTrustTier for errors.Is() compatibility.
func (e *InvalidMountTrustTierError) Unwrap() error { return ErrInvalidMountTrustTier }

// String returns the string representation of MountTrustTier.
func (t MountTrustTier) String() string { return string(t) }

// Validate returns nil if MountTrustTier is empty (defaults by source kind) or recognized.
func (t MountTrustTier) Validate() error {
	switch t {
	case "", MountTrustTierContained, MountTrustTierLocalCwd, MountTrustTierInclude:
		return nil
	default:
		return &InvalidMountTrustTierError{Value: t}
	}
}

// ResolveContainerMountPlan resolves workspace_mount, volumes, and container workdir
// under the 3-tier containment model for dry-run inspection and container execution.
func ResolveContainerMountPlan(ctx *ExecutionContext) (ContainerMountPlan, error) {
	if ctx == nil || ctx.Invowkfile == nil {
		return ContainerMountPlan{}, errors.New("execution context and invowkfile are required")
	}
	rtConfig := selectedRuntimeConfig(ctx)
	if rtConfig == nil {
		return ContainerMountPlan{}, errors.New("runtime config not found for container runtime")
	}
	invowkDir := filepath.Dir(string(ctx.Invowkfile.FilePath))
	if ctx.Invowkfile.ModulePath != "" {
		invowkDir = string(ctx.Invowkfile.ModulePath)
	}
	return resolveContainerMountPlanForDir(ctx, rtConfig, invowkDir)
}

// Validate returns nil if all populated fields of ResolvedContainerVolume are valid.
func (v ResolvedContainerVolume) Validate() error {
	if err := v.RawSpec.Validate(); err != nil {
		return err
	}
	if err := v.ResolvedSpec.Validate(); err != nil {
		return err
	}
	if v.HostPath != "" {
		if err := v.HostPath.Validate(); err != nil {
			return err
		}
	}
	return v.ContainerTarget.Validate()
}

// Validate returns nil if all populated fields of ContainerMountPlan are valid.
func (p ContainerMountPlan) Validate() error {
	if p.WorkspaceHostRoot != "" {
		if err := p.WorkspaceHostRoot.Validate(); err != nil {
			return err
		}
	}
	if p.WorkspaceTarget != "" {
		if err := p.WorkspaceTarget.Validate(); err != nil {
			return err
		}
	}
	if err := p.WorkspaceSELinuxLabel.Validate(); err != nil {
		return err
	}
	if p.WorkspaceVolumeSpec != "" {
		if err := p.WorkspaceVolumeSpec.Validate(); err != nil {
			return err
		}
	}
	for _, rv := range p.ResolvedVolumes {
		if err := rv.Validate(); err != nil {
			return err
		}
	}
	if p.WorkDir != "" {
		if err := p.WorkDir.Validate(); err != nil {
			return err
		}
	}
	return nil
}

//goplint:ignore -- internal resolver accepting pre-derived invowkfile directory path.
func resolveContainerMountPlanForDir(ctx *ExecutionContext, rtConfig *invowkfile.RuntimeConfig, invowkDir string) (ContainerMountPlan, error) {
	ceilings, err := resolveMountCeilings(ctx, invowkDir)
	if err != nil {
		return ContainerMountPlan{}, err
	}

	wsCfg := rtConfig.WorkspaceMount
	wsEnabled := wsCfg.IsEnabled()
	wsRootMode := wsCfg.EffectiveRoot()
	wsReadonly := wsCfg != nil && wsCfg.Readonly
	wsSELinuxLabel := wsCfg.EffectiveSELinuxLabel()

	effectiveWD := ctx.Invowkfile.ResolveEffectiveWorkDir(ctx.Command, ctx.SelectedImpl, ctx.WorkDir)
	rawWD := rawEffectiveWorkDir(ctx, effectiveWD.Source)

	var wsHostRoot types.FilesystemPath
	var physWorkspaceRoot string
	var wsVolumeSpec container.VolumeMountSpec

	if wsEnabled {
		hostRoot, physRoot, rootErr := resolveWorkspaceMountHostRoot(
			ctx, invowkDir, ceilings, wsRootMode, effectiveWD, rawWD,
		)
		if rootErr != nil {
			return ContainerMountPlan{}, rootErr
		}
		wsHostRoot = hostRoot
		physWorkspaceRoot = physRoot
		wsSpecStr := filepath.ToSlash(string(hostRoot)) + ":" + containerWorkspaceRoot
		if wsReadonly {
			wsSpecStr += ":ro"
		}
		wsVolumeSpec = container.VolumeMountSpec(wsSpecStr) //goplint:ignore -- constructed from resolved host root and constant /workspace target.
	}

	resolvedVolumes, err := resolveContainerVolumes(ctx, rtConfig.Volumes, invowkDir, ceilings)
	if err != nil {
		return ContainerMountPlan{}, err
	}

	containerWorkDir, err := resolveContainerWorkDirTarget(
		invowkDir,
		wsEnabled,
		physWorkspaceRoot,
		resolvedVolumes,
		effectiveWD,
		rawWD,
	)
	if err != nil {
		return ContainerMountPlan{}, err
	}

	var containerWorkDirTarget container.MountTargetPath
	if containerWorkDir != "" {
		candidate := container.MountTargetPath(containerWorkDir)
		if err := candidate.Validate(); err != nil {
			return ContainerMountPlan{}, fmt.Errorf("%w: %w", ErrContainerMountPolicy, err)
		}
		containerWorkDirTarget = candidate
	}

	return ContainerMountPlan{
		WorkspaceEnabled:      wsEnabled,
		WorkspaceHostRoot:     wsHostRoot,
		WorkspaceTarget:       containerWorkspaceRoot,
		WorkspaceReadonly:     wsReadonly,
		WorkspaceSELinuxLabel: wsSELinuxLabel,
		WorkspaceVolumeSpec:   wsVolumeSpec,
		ResolvedVolumes:       resolvedVolumes,
		WorkDir:               containerWorkDirTarget,
	}, nil
}

func (ctx *ExecutionContext) effectiveMountTrustTier() MountTrustTier {
	if ctx.MountTrustTier != "" {
		return ctx.MountTrustTier
	}
	if ctx.Invowkfile != nil && ctx.Invowkfile.IsFromModule() {
		return MountTrustTierContained
	}
	return MountTrustTierLocalCwd
}

//goplint:ignore -- returns normalized physical OS root strings for containment checks.
func resolveMountCeilings(ctx *ExecutionContext, invowkDir string) ([]string, error) {
	physInvowkDir, err := normalizeExistingOrParent(invowkDir, "")
	if err != nil {
		return nil, fmt.Errorf("%w: cannot resolve invowkfile directory %q: %w", ErrContainerMountPolicy, invowkDir, err)
	}

	tier := ctx.effectiveMountTrustTier()
	switch tier {
	case MountTrustTierContained:
		return []string{physInvowkDir}, nil
	case MountTrustTierLocalCwd:
		roots := []string{physInvowkDir}
		if ctx.MountCeiling != "" {
			physCeiling, ceilingErr := normalizeExistingOrParent(string(ctx.MountCeiling), "")
			if ceilingErr != nil {
				return nil, fmt.Errorf("%w: cannot resolve cwd ceiling %q: %w", ErrContainerMountPolicy, ctx.MountCeiling, ceilingErr)
			}
			if physCeiling != physInvowkDir {
				roots = append(roots, physCeiling)
			}
		}
		return roots, nil
	case MountTrustTierInclude:
		roots := []string{physInvowkDir}
		if ctx.MountCeiling != "" {
			physCeiling, ceilingErr := normalizeExistingOrParent(string(ctx.MountCeiling), "")
			if ceilingErr != nil {
				return nil, fmt.Errorf("%w: cannot resolve include workspace_root %q: %w", ErrContainerMountPolicy, ctx.MountCeiling, ceilingErr)
			}
			roots = append(roots, physCeiling)
		}
		return roots, nil
	default:
		return []string{physInvowkDir}, nil
	}
}

//goplint:ignore -- internal physical path boundary helper operating on raw OS paths.
func resolveWorkspaceMountHostRoot(
	ctx *ExecutionContext,
	invowkDir string,
	ceilings []string,
	rootMode invowkfile.WorkspaceMountRoot,
	effectiveWD invowkfile.EffectiveWorkDir,
	rawWD string,
) (types.FilesystemPath, string, error) {
	if rootMode == invowkfile.WorkspaceMountRootInvowkfile {
		physInvowkDir, err := normalizeExistingOrParent(invowkDir, "")
		if err != nil {
			return "", "", fmt.Errorf("%w: cannot resolve workspace root %q: %w", ErrContainerMountPolicy, invowkDir, err)
		}
		return types.FilesystemPath(physInvowkDir), physInvowkDir, nil //goplint:ignore -- physical path resolved from invowkDir
	}

	// rootMode == WorkspaceMountRootWorkdir
	if effectiveWD.Source != invowkfile.WorkDirSourceCLIOverride &&
		isExplicitContainerOnlyRawWorkDir(rawWD) &&
		!osPathExists(string(effectiveWD.Path)) {
		return "", "", fmt.Errorf(
			"%w: workspace_mount.root \"workdir\" cannot mount container-only workdir %q from host",
			ErrContainerMountPolicy, rawWD,
		)
	}

	physWD, err := normalizeExistingOrParent(string(effectiveWD.Path), invowkDir)
	if err != nil {
		return "", "", fmt.Errorf("%w: cannot resolve workdir %q for workspace_mount: %w", ErrContainerMountPolicy, effectiveWD.Path, err)
	}

	if effectiveWD.Source != invowkfile.WorkDirSourceCLIOverride && !pathWithinAny(ceilings, physWD) {
		return "", "", fmt.Errorf(
			"%w: workspace_mount.root \"workdir\" resolved to %q, which escapes the allowed %s ceiling (%s)",
			ErrContainerMountPolicy, physWD, ctx.effectiveMountTrustTier(), strings.Join(ceilings, ", "),
		)
	}

	return types.FilesystemPath(physWD), physWD, nil //goplint:ignore -- physical path resolved from effectiveWD
}

//goplint:ignore -- internal resolver enforcing 3-tier containment on pre-derived host paths.
func resolveContainerVolumes(
	ctx *ExecutionContext,
	specs []invowkfile.VolumeMountSpec,
	invowkDir string,
	ceilings []string,
) ([]ResolvedContainerVolume, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	isModule := ctx.Invowkfile != nil && ctx.Invowkfile.IsFromModule()
	resolved := make([]ResolvedContainerVolume, 0, len(specs))

	for _, rawSpec := range specs {
		parsed, err := parseVolumeMountSpec(string(rawSpec))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrContainerMountPolicy, err)
		}

		rawTyped := container.VolumeMountSpec(rawSpec)
		if valErr := rawTyped.Validate(); valErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrContainerMountPolicy, valErr)
		}
		targetTyped := container.MountTargetPath(parsed.containerTarget)
		if valErr := targetTyped.Validate(); valErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrContainerMountPolicy, valErr)
		}

		switch parsed.sourceKind {
		case containerVolumeKindNamed:
			resolved = append(resolved, ResolvedContainerVolume{
				RawSpec:         rawTyped,
				ResolvedSpec:    rawTyped,
				SourceKind:      containerVolumeKindNamed,
				ContainerTarget: targetTyped,
				Options:         parsed.options,
			})

		case containerVolumeKindAbsolute:
			if isModule {
				return nil, fmt.Errorf(
					"%w: %w: volume %q uses an absolute host path in a *.invowkmod module (use a named volume or a relative path within the allowed workspace)",
					ErrContainerMountPolicy, ErrContainerModuleAbsoluteVolume, rawSpec,
				)
			}
			physHost, normErr := normalizeExistingOrParent(filepath.FromSlash(parsed.source), invowkDir)
			if normErr != nil {
				physHost = filepath.Clean(filepath.FromSlash(parsed.source))
			}
			resolvedTyped, valErr := buildValidatedVolumeMountSpec(filepath.ToSlash(physHost), parsed.containerTarget, parsed.options)
			if valErr != nil {
				return nil, fmt.Errorf("%w: %w", ErrContainerMountPolicy, valErr)
			}
			resolved = append(resolved, ResolvedContainerVolume{
				RawSpec:         rawTyped,
				ResolvedSpec:    resolvedTyped,
				SourceKind:      containerVolumeKindAbsolute,
				HostPath:        types.FilesystemPath(physHost), //goplint:ignore -- validated host path
				ContainerTarget: targetTyped,
				Options:         parsed.options,
			})

		case containerVolumeKindRelative:
			nativeSource := filepath.FromSlash(parsed.source)
			physHost, normErr := normalizeExistingOrParent(nativeSource, invowkDir)
			if normErr != nil {
				return nil, fmt.Errorf("%w: cannot resolve relative volume %q: %w", ErrContainerMountPolicy, rawSpec, normErr)
			}
			if !pathWithinAny(ceilings, physHost) {
				return nil, fmt.Errorf(
					"%w: relative volume %q resolved to %q, which escapes the allowed %s ceiling (%s)",
					ErrContainerMountPolicy, rawSpec, physHost, ctx.effectiveMountTrustTier(), strings.Join(ceilings, ", "),
				)
			}
			resolvedTyped, valErr := buildValidatedVolumeMountSpec(filepath.ToSlash(physHost), parsed.containerTarget, parsed.options)
			if valErr != nil {
				return nil, fmt.Errorf("%w: %w", ErrContainerMountPolicy, valErr)
			}
			resolved = append(resolved, ResolvedContainerVolume{
				RawSpec:         rawTyped,
				ResolvedSpec:    resolvedTyped,
				SourceKind:      containerVolumeKindRelative,
				HostPath:        types.FilesystemPath(physHost), //goplint:ignore -- physical path within ceiling
				ContainerTarget: targetTyped,
				Options:         parsed.options,
			})
		}
	}

	return resolved, nil
}

//goplint:ignore -- internal volume spec parser operates on raw Docker/Podman mount strings.
func parseVolumeMountSpec(spec string) (parsedVolumeMount, error) {
	if strings.TrimSpace(spec) == "" {
		return parsedVolumeMount{}, errors.New("volume mount cannot be empty")
	}

	parts := strings.Split(spec, ":")
	if len(parts) >= 2 && len(parts[0]) == 1 && isASCIILetterByte(parts[0][0]) {
		// Windows drive-letter prefix (e.g., C:\data:/data[:ro] or C:/data:/data[:ro]).
		if len(parts) < 3 {
			return parsedVolumeMount{}, fmt.Errorf("volume mount %q has invalid format (expected host:container)", spec)
		}
		parts = append([]string{parts[0] + ":" + parts[1]}, parts[2:]...)
	}
	if len(parts) < 2 || len(parts) > 3 {
		return parsedVolumeMount{}, fmt.Errorf("volume mount %q has invalid format (expected host:container[:options])", spec)
	}

	source := parts[0]
	target := parts[1]
	var options string
	if len(parts) == 3 {
		options = parts[2]
	}

	if source == "" || target == "" || !strings.HasPrefix(target, "/") {
		return parsedVolumeMount{}, fmt.Errorf("volume mount %q has invalid source or container target", spec)
	}

	kind := classifyVolumeSource(source)
	return parsedVolumeMount{
		sourceKind:      kind,
		source:          source,
		containerTarget: target,
		options:         options,
	}, nil
}

//goplint:ignore -- internal host-path vs named-volume classifier.
func classifyVolumeSource(source string) string {
	// Cross-platform host-absolute checks:
	// - POSIX slash-rooted (/...)
	// - Windows backslash-rooted (\... or \\server\share)
	// - Windows drive prefix (C:\..., C:/..., or C:...)
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, `\`) {
		return containerVolumeKindAbsolute
	}
	if len(source) >= 2 && isASCIILetterByte(source[0]) && source[1] == ':' {
		return containerVolumeKindAbsolute
	}

	// Relative host path markers: '.', '..', './', '../', '.\', '..\', any separator, or leading '.'.
	if source == "." || source == ".." || strings.HasPrefix(source, ".") ||
		strings.Contains(source, "/") || strings.Contains(source, `\`) {
		return containerVolumeKindRelative
	}

	if namedVolumePattern.MatchString(source) {
		return containerVolumeKindNamed
	}

	return containerVolumeKindRelative
}

//goplint:ignore -- raw ASCII byte check for Windows drive letter parsing.
func isASCIILetterByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

//goplint:ignore -- formats and validates Docker/Podman -v mount spec.
func buildValidatedVolumeMountSpec(source, target, options string) (container.VolumeMountSpec, error) {
	formatted := formatVolumeMountSpec(source, target, options)
	vol := container.VolumeMountSpec(formatted)
	if err := vol.Validate(); err != nil {
		return "", err
	}
	return vol, nil
}

//goplint:ignore -- formats raw Docker/Podman -v mount string.
func formatVolumeMountSpec(source, target, options string) string {
	if options != "" {
		return source + ":" + target + ":" + options
	}
	return source + ":" + target
}

//goplint:ignore -- internal helper comparing candidate host path against normalized root paths.
func pathWithinAny(roots []string, candidate string) bool {
	for _, root := range roots {
		if pathWithin(root, candidate) {
			return true
		}
	}
	return false
}

//goplint:ignore -- internal helper extracting raw configured workdir text before path resolution.
func rawEffectiveWorkDir(ctx *ExecutionContext, source invowkfile.WorkDirSource) string {
	if ctx == nil {
		return ""
	}
	switch source {
	case invowkfile.WorkDirSourceCLIOverride:
		return string(ctx.WorkDir)
	case invowkfile.WorkDirSourceImplementation:
		if ctx.SelectedImpl != nil {
			return string(ctx.SelectedImpl.WorkDir)
		}
	case invowkfile.WorkDirSourceCommand:
		if ctx.Command != nil {
			return string(ctx.Command.WorkDir)
		}
	case invowkfile.WorkDirSourceRoot:
		if ctx.Invowkfile != nil {
			return string(ctx.Invowkfile.WorkDir)
		}
	case invowkfile.WorkDirSourceDefault:
		return ""
	}
	return ""
}

//goplint:ignore -- internal container-path syntax classifier.
func isExplicitContainerOnlyRawWorkDir(rawWorkDir string) bool {
	if rawWorkDir == "" {
		return false
	}
	// Windows drive-letter or UNC paths are always host paths.
	if len(rawWorkDir) >= 2 && isASCIILetterByte(rawWorkDir[0]) && rawWorkDir[1] == ':' {
		return false
	}
	if strings.HasPrefix(rawWorkDir, `\`) {
		return false
	}
	return strings.HasPrefix(rawWorkDir, "/") && fspath.IsAbs(types.FilesystemPath(rawWorkDir)) //goplint:ignore -- slash-prefixed syntax check only.
}

//goplint:ignore -- internal Lstat check on resolved host path.
func osPathExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Lstat(p)
	return err == nil
}

//goplint:ignore -- internal container workdir mapper returning container target path or policy error.
func resolveContainerWorkDirTarget(
	invowkDir string,
	wsEnabled bool,
	physWorkspaceRoot string,
	resolvedVolumes []ResolvedContainerVolume,
	effectiveWD invowkfile.EffectiveWorkDir,
	rawWD string,
) (string, error) {
	physWD, physErr := normalizeExistingOrParent(string(effectiveWD.Path), invowkDir)

	// 1. If /workspace is mounted and effectiveWorkDir lies physically inside it, map to /workspace[/rel].
	if wsEnabled && physWorkspaceRoot != "" && physErr == nil && pathWithin(physWorkspaceRoot, physWD) {
		rel, relErr := filepath.Rel(physWorkspaceRoot, physWD)
		if relErr == nil {
			if rel == "." || rel == "" {
				return containerWorkspaceRoot, nil
			}
			return containerWorkspacePrefix + filepath.ToSlash(rel), nil
		}
	}

	// 2. If effectiveWorkDir lies physically inside any host bind-mount volume, map to its container target.
	if physErr == nil {
		for _, vol := range resolvedVolumes {
			if vol.HostPath == "" {
				continue
			}
			physVolHost, volErr := normalizeExistingOrParent(string(vol.HostPath), invowkDir)
			if volErr != nil {
				continue
			}
			if pathWithin(physVolHost, physWD) {
				rel, relErr := filepath.Rel(physVolHost, physWD)
				if relErr == nil {
					if rel == "." || rel == "" {
						return string(vol.ContainerTarget), nil
					}
					return path.Join(string(vol.ContainerTarget), filepath.ToSlash(rel)), nil
				}
			}
		}
	}

	// 3. When workspace_mount is disabled and no workdir was configured, omit -w (use image WORKDIR).
	if effectiveWD.Source == invowkfile.WorkDirSourceDefault {
		if !wsEnabled {
			return "", nil
		}
		return "", fmt.Errorf(
			"%w: %w: default workdir %q is not mounted inside container",
			ErrContainerMountPolicy, ErrContainerWorkDirNotMounted, effectiveWD.Path,
		)
	}

	// 4. Explicit container-absolute path (e.g., "/app" or "/") when not an unmounted host path.
	if isExplicitContainerOnlyRawWorkDir(rawWD) {
		cleanedContainer := path.Clean(rawWD)
		// If a multi-segment path starting with '/' actually exists on the host and was not
		// covered by /workspace or volumes above, treat it as an unmounted host path and fail closed.
		if cleanedContainer != "/" && osPathExists(rawWD) {
			return "", fmt.Errorf(
				"%w: %w: host path %q is not mounted in /workspace or volumes",
				ErrContainerMountPolicy, ErrContainerWorkDirNotMounted, rawWD,
			)
		}
		return cleanedContainer, nil
	}

	// 5. Any remaining relative/Windows/unmounted path must never be forwarded verbatim as -w.
	return "", fmt.Errorf(
		"%w: %w: workdir %q (resolved to %q) is not inside /workspace or any mounted volume",
		ErrContainerMountPolicy, ErrContainerWorkDirNotMounted, rawWD, effectiveWD.Path,
	)
}
