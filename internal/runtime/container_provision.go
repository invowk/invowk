// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// maxBuildRetries is the number of attempts for container image builds.
	// Retries handle transient engine errors (exit code 125, network timeouts,
	// storage driver races) that occur on rootless Podman in CI environments.
	maxBuildRetries = 3

	// baseBuildBackoff is the initial backoff duration between build retries.
	// Builds are heavier than runs, so we use a longer base than the smoke test.
	baseBuildBackoff = 2 * time.Second

	// maxRunRetries is the number of attempts for container run operations.
	// Retries handle transient engine errors (rootless Podman ping_group_range
	// race, exit code 125/126, overlay mount races) that occur when multiple
	// containers start concurrently. Set to 5 (vs 3 for builds) because
	// run races are more frequent under heavy parallelism and runs are fast.
	maxRunRetries = 5

	// baseRunBackoff is the initial backoff duration between run retries.
	// Uses exponential backoff: 1s, 2s, 4s, 8s (total max ~15s).
	baseRunBackoff = 1 * time.Second

	containerWorkspaceRoot   = "/workspace"
	containerWorkspacePrefix = containerWorkspaceRoot + "/"
)

// errStrictModeProvisioning is returned when container provisioning fails
// and strict mode is enabled, preventing fallback to the base image.
var errStrictModeProvisioning = errors.New("container provisioning failed (strict mode enabled)")

//goplint:ignore -- internal container execution prep DTO carrying CLI argv and temp script dir.
type containerInterpreterSpec struct {
	command        []string
	tempScriptPath string
	tempCleanupDir string
	extraVolume    *container.VolumeMountSpec
}

// Validate returns nil if the populated extraVolume is valid.
func (s containerInterpreterSpec) Validate() error {
	if s.extraVolume != nil {
		return s.extraVolume.Validate()
	}
	return nil
}

// ensureProvisionedImage ensures the container image exists and is provisioned
// with invowk resources (binary, modules, etc.). This enables nested invowk commands
// inside containers.
func (r *ContainerRuntime) ensureProvisionedImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (imageName string, envVars map[string]string, cleanup func(), diagnostics []InitDiagnostic, err error) {
	// First, ensure the base image exists
	baseImage, err := r.ensureImage(ctx, cfg, invowkDir)
	if err != nil {
		return "", nil, nil, nil, err
	}

	// If provisioning is disabled, return the base image
	if r.provisioner == nil || r.provisionConfig == nil || !r.provisionConfig.Enabled {
		return baseImage, nil, nil, nil, nil
	}

	// Provision the image with invowk resources
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Provisioning container with invowk resources...\n") // Verbose output; error non-critical
	}

	result, err := r.provisioner.Provision(ctx.Context, provision.Request{
		BaseImage:    container.ImageTag(baseImage),
		ForceRebuild: ctx.ForceRebuild,
		Stdout:       ctx.IO.Stderr,
		Stderr:       ctx.IO.Stderr,
	})
	if err != nil {
		// An unusable configured binary_path is what the user asked for:
		// fail whatever strict says, never fall back to the base image.
		if errors.Is(err, provision.ErrUnusableInvowkBinary) {
			return "", nil, nil, nil, err
		}
		if r.provisionConfig.Strict {
			// Multi-wrap (Go 1.20+): callers can match either sentinel via errors.Is
			return "", nil, nil, nil, fmt.Errorf("%w: %w", errStrictModeProvisioning, err)
		}
		diag := InitDiagnostic{
			Code: CodeContainerProvisioningFailed,
			Message: fmt.Sprintf("Container provisioning failed: %v. Running without invowk resources (binary, modules). "+
				"Nested invowk commands inside the container will not work. "+
				"To fail on provisioning errors, set container.auto_provision.strict = true.", err),
			Cause: err,
		}
		return baseImage, nil, nil, []InitDiagnostic{diag}, nil
	}
	diagnostics = make([]InitDiagnostic, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		diagnostics = append(diagnostics, InitDiagnostic{
			Code:    CodeContainerProvisioningWarning,
			Message: warning.Message.String(),
		})
	}

	return string(result.ImageTag), result.EnvVars, result.Cleanup, diagnostics, nil
}

// ensureImage ensures the container image exists, building if necessary
func (r *ContainerRuntime) ensureImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (string, error) {
	// If an image is specified, use it directly
	if cfg.Image != "" {
		return string(cfg.Image), nil
	}

	// Build from Containerfile/Dockerfile
	containerfile := cfg.Containerfile
	if containerfile == "" {
		return "", fmt.Errorf("%w: container runtime requires either containerfile or image in the runtime config", ErrContainerBuildConfig)
	}

	containerfilePath := filepath.Join(invowkDir, string(containerfile))
	if _, err := os.Stat(containerfilePath); err != nil {
		return "", fmt.Errorf("containerfile not found at %s", containerfilePath)
	}

	// Generate a unique image tag based on invowkfile path
	imageTag, err := r.generateImageTag(string(ctx.Invowkfile.FilePath))
	if err != nil {
		return "", err
	}

	// Check if image already exists (skip if ForceRebuild is set)
	if !ctx.ForceRebuild {
		exists, err := r.engine.ImageExists(ctx.Context, container.ImageTag(imageTag))
		if err != nil {
			return "", fmt.Errorf("failed to check image existence: %w", err)
		}
		if exists {
			return imageTag, nil
		}
	}

	// Build the image
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Building container image from %s...\n", containerfilePath) // Verbose output; error non-critical
	}

	buildOpts := container.BuildOptions{
		ContextDir: container.HostFilesystemPath(invowkDir),
		Dockerfile: containerfile,
		Tag:        container.ImageTag(imageTag),
		NoCache:    ctx.ForceRebuild,
		Stdout:     ctx.IO.Stdout,
		Stderr:     ctx.IO.Stderr,
	}

	// Retry build on transient engine errors (exit code 125, network failures,
	// storage driver races). RetryWithBackoff checks ctx.Err() between retries,
	// preventing wasted build attempts after context cancellation.
	var prevBuildErr error
	retryErr := container.RetryWithBackoff(ctx.Context, maxBuildRetries, baseBuildBackoff,
		func(attempt int) (bool, error) {
			if attempt > 0 && ctx.Verbose {
				_, _ = fmt.Fprintf(ctx.IO.Stdout, "Retrying container build (attempt %d/%d) after transient error: %v\n", attempt+1, maxBuildRetries, prevBuildErr)
			}
			buildErr := r.engine.Build(ctx.Context, buildOpts)
			if buildErr != nil {
				prevBuildErr = buildErr
				return container.IsTransientError(buildErr), buildErr
			}
			return false, nil
		})
	if retryErr != nil {
		return "", retryErr
	}
	return imageTag, nil
}

// generateImageTag generates a unique image tag for an invowkfile
//
//plint:render
func (r *ContainerRuntime) generateImageTag(invowkfilePath string) (string, error) {
	absPath, err := filepath.Abs(invowkfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve invowkfile path: %w", err)
	}
	hash := sha256.Sum256([]byte(absPath))
	shortHash := hex.EncodeToString(hash[:])[:12]
	return fmt.Sprintf("invowk-%s:latest", shortHash), nil
}

// buildProvisionConfig creates a provision.Config from the app config.
func buildProvisionConfig(cfg *config.Config) *provision.Config {
	provisionCfg := provision.DefaultConfig()
	applyHostProvisionDefaults(provisionCfg)

	if cfg == nil {
		return provisionCfg
	}

	// Apply config overrides
	autoProv := cfg.Container.AutoProvision
	provisionCfg.Enabled = autoProv.Enabled
	provisionCfg.Strict = autoProv.Strict

	// InvowkBinaryPath carries only the configured binary_path; the running
	// executable stays in DefaultInvowkBinaryPath, so the provisioner can
	// fail on the first and skip the second.
	if autoProv.BinaryPath != "" {
		provisionCfg.InvowkBinaryPath = types.FilesystemPath(autoProv.BinaryPath)
	}

	// Add modules from auto_provision includes (explicit provisioning paths).
	for _, inc := range autoProv.Includes {
		provisionCfg.ModuleEntries = append(provisionCfg.ModuleEntries, provisionModuleEntryFromInclude(inc))
	}

	// Conditionally inherit root-level includes into provisioning.
	if autoProv.InheritIncludes {
		for _, inc := range cfg.Includes {
			provisionCfg.ModuleEntries = append(provisionCfg.ModuleEntries, provisionModuleEntryFromInclude(inc))
		}
	}

	if autoProv.CacheDir != "" {
		provisionCfg.CacheDir = types.FilesystemPath(autoProv.CacheDir)
	}

	return provisionCfg
}

func provisionModuleEntryFromInclude(inc config.IncludeEntry) provision.ModuleEntry {
	entry := provision.ModuleEntry{Path: types.FilesystemPath(inc.Path)}
	if inc.Alias != "" {
		entry.CommandNamespace = invowkmod.ModuleNamespace(inc.Alias)
	}
	return entry
}

func applyHostProvisionDefaults(provisionCfg *provision.Config) {
	if provisionCfg == nil {
		return
	}

	if provisionCfg.DefaultInvowkBinaryPath == "" {
		if binaryPath, err := os.Executable(); err == nil {
			provisionCfg.DefaultInvowkBinaryPath = types.FilesystemPath(binaryPath) //goplint:ignore -- host path validated by provision.Config.Validate()
		}
	}
	if userDir, err := config.CommandsDir(); err == nil {
		if info, statErr := os.Stat(string(userDir)); statErr == nil && info.IsDir() && !hasGlobalModulePath(provisionCfg, userDir) {
			provisionCfg.GlobalModulesPaths = append(provisionCfg.GlobalModulesPaths, userDir)
		}
	}
	// The default cache dir needs an absolute home, which need not exist
	// (decision 15 of require-absolute-base-dirs): a relative home would put
	// it under the current directory, so the provisioner falls back instead.
	if provisionCfg.CacheDir == "" {
		if home, err := provisionCfg.BaseDirs.Home(); err == nil {
			provisionCfg.CacheDir = types.FilesystemPath(filepath.Join(string(home), ".cache", "invowk", "provision")) //goplint:ignore -- host path validated by provision.Config.Validate()
		}
	}
	if provisionCfg.TagSuffix == "" {
		provisionCfg.TagSuffix = os.Getenv("INVOWK_PROVISION_TAG_SUFFIX")
	}
}

func hasGlobalModulePath(provisionCfg *provision.Config, userDir types.FilesystemPath) bool {
	cleanUserDir := filepath.Clean(string(userDir))
	for _, path := range provisionCfg.GlobalModulesPaths {
		if filepath.Clean(string(path)) == cleanUserDir {
			return true
		}
	}
	for _, entry := range provisionCfg.GlobalModuleEntries {
		if filepath.Clean(string(entry.Path)) == cleanUserDir {
			return true
		}
	}
	return false
}

// containerConfigFromRuntime extracts container config from RuntimeConfig
func containerConfigFromRuntime(rt *invowkfile.RuntimeConfig) invowkfileContainerConfig {
	if rt == nil {
		return invowkfileContainerConfig{}
	}
	return invowkfileContainerConfig{
		Containerfile:  container.HostFilesystemPath(rt.Containerfile),
		Image:          container.ImageTag(rt.Image),
		Volumes:        containerVolumeSpecs(rt.Volumes),
		Ports:          containerPortSpecs(rt.Ports),
		Persistent:     rt.Persistent,
		WorkspaceMount: rt.WorkspaceMount,
	}
}

func containerVolumeSpecs(specs []invowkfile.VolumeMountSpec) []container.VolumeMountSpec {
	volumes := make([]container.VolumeMountSpec, 0, len(specs))
	for _, spec := range specs {
		volumes = append(volumes, container.VolumeMountSpec(spec))
	}
	return volumes
}

func containerPortSpecs(specs []invowkfile.PortMappingSpec) []container.PortMappingSpec {
	ports := make([]container.PortMappingSpec, 0, len(specs))
	for _, spec := range specs {
		ports = append(ports, container.PortMappingSpec(spec))
	}
	return ports
}

// buildInterpreterCommand builds the command array for interpreter-based execution.
// For inline scripts in ephemeral containers, it creates a temp file in an isolated
// host temp directory mounted read-only at /.invowk/tmp rather than inside invowkDir.
// Persistent containers use in-container temp script delivery so reused container specs stay stable.
func (r *ContainerRuntime) buildInterpreterCommand(
	ctx *ExecutionContext,
	script string,
	interp invowkfile.ShebangInfo,
	invowkDir string,
	mountPlan ContainerMountPlan,
	isPersistent bool,
) (containerInterpreterSpec, error) {
	if ctx.SelectedImpl.Script.IsFile() {
		return buildFileInterpreterCommand(ctx, script, interp, invowkDir, mountPlan, isPersistent)
	}
	if isPersistent {
		return containerInterpreterSpec{
			command: buildInContainerTempScriptCommand(script, interp, ctx.PositionalArgs),
		}, nil
	}

	ext := invowkfile.GetExtensionForInterpreter(interp.Interpreter)
	tempDir, err := os.MkdirTemp("", "invowk-container-script-*")
	if err != nil {
		return containerInterpreterSpec{}, fmt.Errorf("failed to create temp script dir: %w", err)
	}
	// Resolve symlinks on tempDir (e.g., /var -> /private/var on macOS) so mount paths stay canonical.
	if resolvedDir, resolveErr := filepath.EvalSymlinks(tempDir); resolveErr == nil {
		tempDir = resolvedDir
	}

	scriptFilePath := filepath.Join(tempDir, "invowk-script"+ext)
	if err := os.WriteFile(scriptFilePath, []byte(script), 0o755); err != nil { //goplint:ignore -- host temp script requires executable bit for interpreter mounts
		_ = os.RemoveAll(tempDir)
		return containerInterpreterSpec{}, fmt.Errorf("failed to write temp script: %w", err)
	}

	containerScriptPath := path.Join(containerInvowkTmpDir, filepath.Base(scriptFilePath))
	vol, volErr := buildValidatedVolumeMountSpec(filepath.ToSlash(tempDir), containerInvowkTmpDir, "ro")
	if volErr != nil {
		_ = os.RemoveAll(tempDir)
		return containerInterpreterSpec{}, fmt.Errorf("temp script volume mount: %w", volErr)
	}
	cmd := make([]string, 0, 1+len(interp.Args)+1+len(ctx.PositionalArgs))
	cmd = append(cmd, interp.Interpreter)
	cmd = append(cmd, interp.Args...)
	cmd = append(cmd, containerScriptPath)
	cmd = append(cmd, ctx.PositionalArgs...)

	return containerInterpreterSpec{
		command:        cmd,
		tempScriptPath: scriptFilePath,
		tempCleanupDir: tempDir,
		extraVolume:    &vol,
	}, nil
}

//goplint:ignore -- internal container interpreter command builder.
func buildFileInterpreterCommand(
	ctx *ExecutionContext,
	script string,
	interp invowkfile.ShebangInfo,
	invowkDir string,
	mountPlan ContainerMountPlan,
	isPersistent bool,
) (containerInterpreterSpec, error) {
	scriptPath := string(ctx.SelectedScriptFilePath())
	physScriptPath, err := normalizeExistingOrParent(scriptPath, invowkDir)
	if err != nil {
		physScriptPath = filepath.Clean(scriptPath)
	}

	containerScriptPath, extraVol, targetErr := resolveScriptFileContainerTarget(physScriptPath, invowkDir, mountPlan, isPersistent)
	if targetErr != nil {
		return containerInterpreterSpec{}, targetErr
	}
	if containerScriptPath == "" && isPersistent {
		return containerInterpreterSpec{
			command: buildInContainerTempScriptCommand(script, interp, ctx.PositionalArgs),
		}, nil
	}

	cmd := make([]string, 0, 1+len(interp.Args)+1+len(ctx.PositionalArgs))
	cmd = append(cmd, interp.Interpreter)
	cmd = append(cmd, interp.Args...)
	cmd = append(cmd, containerScriptPath)
	cmd = append(cmd, ctx.PositionalArgs...)

	return containerInterpreterSpec{
		command:     cmd,
		extraVolume: extraVol,
	}, nil
}

//goplint:ignore -- internal script-file mount mapper returning container target path.
func resolveScriptFileContainerTarget(
	physScriptPath string,
	invowkDir string,
	mountPlan ContainerMountPlan,
	isPersistent bool,
) (string, *container.VolumeMountSpec, error) {
	if mountPlan.WorkspaceEnabled && mountPlan.WorkspaceHostRoot != "" {
		if physRoot, err := normalizeExistingOrParent(string(mountPlan.WorkspaceHostRoot), invowkDir); err == nil {
			if pathWithin(physRoot, physScriptPath) {
				if rel, relErr := filepath.Rel(physRoot, physScriptPath); relErr == nil {
					if rel == "." || rel == "" {
						return containerWorkspaceRoot, nil, nil
					}
					return path.Join(containerWorkspaceRoot, filepath.ToSlash(rel)), nil, nil
				}
			}
		}
	}

	for _, vol := range mountPlan.ResolvedVolumes {
		if vol.HostPath == "" {
			continue
		}
		physVol, err := normalizeExistingOrParent(string(vol.HostPath), invowkDir)
		if err != nil || !pathWithin(physVol, physScriptPath) {
			continue
		}
		if rel, relErr := filepath.Rel(physVol, physScriptPath); relErr == nil {
			if rel == "." || rel == "" {
				return string(vol.ContainerTarget), nil, nil
			}
			return path.Join(string(vol.ContainerTarget), filepath.ToSlash(rel)), nil, nil
		}
	}

	if isPersistent {
		return "", nil, nil
	}

	target := path.Join(containerInvowkScriptsDir, filepath.Base(physScriptPath))
	vol, volErr := buildValidatedVolumeMountSpec(filepath.ToSlash(physScriptPath), target, "ro")
	if volErr != nil {
		return "", nil, fmt.Errorf("decoupled script volume mount: %w", volErr)
	}
	return target, &vol, nil
}

//goplint:ignore -- internal POSIX shell command builder for persistent in-container script delivery.
func buildInContainerTempScriptCommand(script string, interp invowkfile.ShebangInfo, positionalArgs []string) []string {
	var interpCmd strings.Builder
	interpCmd.WriteString(shellSingleQuote(interp.Interpreter))
	for _, arg := range interp.Args {
		interpCmd.WriteByte(' ')
		interpCmd.WriteString(shellSingleQuote(arg))
	}

	// Standard POSIX mktemp supports suffix only with GNU coreutils; keep a plain template.
	shellBody := fmt.Sprintf(
		`f=$(mktemp /tmp/invowk-script-XXXXXX 2>/dev/null || echo "/tmp/invowk-script-$$") && trap 'rm -f "$f"' EXIT && printf '%%s' "$1" > "$f" && chmod 0755 "$f" && shift && %s "$f" "$@"`,
		interpCmd.String(),
	)

	cmd := make([]string, 0, 5+len(positionalArgs))
	cmd = append(cmd, defaultContainerShellPath, "-c", shellBody, "invowk", script)
	cmd = append(cmd, positionalArgs...)
	return cmd
}

//goplint:ignore -- internal POSIX shell argument quoting helper.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func workspaceSELinuxMode(label invowkfile.WorkspaceMountSELinuxLabel) container.VolumeMountSELinuxMode {
	switch label {
	case invowkfile.WorkspaceMountSELinuxShared:
		return container.VolumeMountSELinuxShared
	case invowkfile.WorkspaceMountSELinuxPrivate:
		return container.VolumeMountSELinuxPrivate
	case invowkfile.WorkspaceMountSELinuxNone:
		return container.VolumeMountSELinuxNone
	case invowkfile.WorkspaceMountSELinuxAuto:
		return container.VolumeMountSELinuxAuto
	default:
		return container.VolumeMountSELinuxAuto
	}
}

// getContainerWorkDir determines the working directory for container execution.
// Uses the hierarchical override model (CLI > Implementation > Command > Root > Default)
// mapped strictly onto mounted container paths.
func (r *ContainerRuntime) getContainerWorkDir(ctx *ExecutionContext, invowkDir string) (string, error) {
	if ctx == nil || ctx.Invowkfile == nil {
		return "", errors.New("execution context and invowkfile are required")
	}
	rtConfig := selectedRuntimeConfig(ctx)
	if rtConfig == nil {
		rtConfig = &invowkfile.RuntimeConfig{}
	}
	plan, err := resolveContainerMountPlanForDir(ctx, rtConfig, invowkDir)
	if err != nil {
		return "", err
	}
	return string(plan.WorkDir), nil
}
