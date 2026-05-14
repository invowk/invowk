// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/containerargs"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	persistentContainerModeEphemeral  = "ephemeral"
	persistentContainerModePersistent = "persistent"

	persistentContainerNameSourceCLI     = "cli"
	persistentContainerNameSourceConfig  = "config"
	persistentContainerNameSourceDerived = "derived"

	persistentContainerLabelManaged     = "dev.invowk.managed"
	persistentContainerLabelPersistent  = "dev.invowk.persistent"
	persistentContainerLabelNamespace   = "dev.invowk.command.namespace"
	persistentContainerLabelSource      = "dev.invowk.command.source"
	persistentContainerLabelSpecHash    = "dev.invowk.container.spec"
	persistentContainerManagedLabelTrue = "true"

	persistentContainerNamePrefix = "invowk-"
	persistentContainerHashLen    = 12
)

var persistentContainerIdleCommand = []string{
	"/bin/sh",
	"-c",
	"trap 'exit 0' TERM INT; while true; do sleep 3600; done",
}

type (
	// ContainerPersistentPlan contains dry-run facts for persistent container targeting.
	ContainerPersistentPlan struct {
		Mode            string //goplint:ignore -- dry-run render DTO mode label.
		Name            invowkfile.ContainerName
		NameSource      string //goplint:ignore -- dry-run render DTO source label.
		CreateIfMissing bool
	}

	persistentContainerTarget struct {
		name            container.ContainerName
		nameSource      string //goplint:ignore -- internal source classifier label.
		createIfMissing bool
	}

	provisionedImageTagResolver interface {
		GetProvisionedImageTag(context.Context, container.ImageTag) (string, error)
	}
)

// ContainerPersistentDryRunPlan reports what the container runtime would do for
// persistent targeting without inspecting or mutating the container engine.
func ContainerPersistentDryRunPlan(ctx *ExecutionContext) ContainerPersistentPlan {
	if ctx == nil || ctx.SelectedImpl == nil {
		return ContainerPersistentPlan{Mode: persistentContainerModeEphemeral}
	}
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	target, ok := resolvePersistentContainerTarget(ctx, containerConfigFromRuntime(rtConfig))
	if !ok {
		return ContainerPersistentPlan{Mode: persistentContainerModeEphemeral}
	}
	return ContainerPersistentPlan{
		Mode:            persistentContainerModePersistent,
		Name:            target.name,
		NameSource:      target.nameSource,
		CreateIfMissing: target.createIfMissing,
	}
}

// Validate returns nil when the dry-run persistent target plan has valid typed fields.
func (p ContainerPersistentPlan) Validate() error {
	if p.Name != "" {
		return p.Name.Validate()
	}
	return nil
}

func (t persistentContainerTarget) Validate() error {
	return t.name.Validate()
}

func persistentContainerRequested(ctx *ExecutionContext, cfg invowkfileContainerConfig) bool {
	return ctx != nil && (ctx.ContainerNameOverride != "" || cfg.Persistent != nil)
}

func resolvePersistentContainerTarget(ctx *ExecutionContext, cfg invowkfileContainerConfig) (persistentContainerTarget, bool) {
	if !persistentContainerRequested(ctx, cfg) {
		return persistentContainerTarget{}, false
	}

	target := persistentContainerTarget{
		createIfMissing: cfg.Persistent != nil && cfg.Persistent.CreateIfMissing,
	}
	switch {
	case ctx.ContainerNameOverride != "":
		target.name = ctx.ContainerNameOverride
		target.nameSource = persistentContainerNameSourceCLI
	case cfg.Persistent != nil && cfg.Persistent.Name != "":
		target.name = cfg.Persistent.Name
		target.nameSource = persistentContainerNameSourceConfig
	default:
		target.name = derivePersistentContainerName(ctx)
		target.nameSource = persistentContainerNameSourceDerived
	}
	return target, true
}

func derivePersistentContainerName(ctx *ExecutionContext) container.ContainerName {
	namespace := ctx.CommandFullName
	if namespace == "" && ctx.Command != nil {
		namespace = ctx.Command.Name
	}
	source := ""
	if ctx.Invowkfile != nil {
		source = string(ctx.Invowkfile.FilePath)
	}

	sum := sha256.Sum256([]byte(string(namespace) + "\x00" + source))
	hash := hex.EncodeToString(sum[:])[:persistentContainerHashLen]
	slug := containerNameSlug(string(namespace))

	maxSlugLen := max(containerargs.MaxContainerNameLength-len(persistentContainerNamePrefix)-len(hash)-1, 1)
	if len(slug) > maxSlugLen {
		slug = strings.Trim(slug[:maxSlugLen], "-._")
	}
	if slug == "" {
		slug = "cmd"
	}

	//goplint:ignore -- validated immediately below with deterministic fallback.
	name := container.ContainerName(persistentContainerNamePrefix + slug + "-" + hash)
	if err := name.Validate(); err != nil {
		//goplint:ignore -- deterministic hash fallback uses the same portable grammar.
		return container.ContainerName(persistentContainerNamePrefix + hash)
	}
	return name
}

//goplint:ignore -- slug builder operates on command namespace text for container-name derivation.
func containerNameSlug(value string) string {
	var b strings.Builder
	lastSep := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSep = false
		case r == '.', r == '_', r == '-':
			if b.Len() > 0 {
				b.WriteRune(r)
				lastSep = r == '-'
			}
		case !lastSep && b.Len() > 0:
			b.WriteByte('-')
			lastSep = true
		}
	}
	slug := strings.Trim(b.String(), "-._")
	if slug == "" {
		return "cmd"
	}
	return slug
}

func (r *ContainerRuntime) ensurePersistentContainer(ctx *ExecutionContext, prep *containerExecPrep) (container.ContainerID, error) {
	target, ok := resolvePersistentContainerTarget(ctx, prep.containerCfg)
	if !ok {
		return "", errors.New("persistent container target was not requested")
	}

	createOpts := r.persistentCreateOptions(ctx, prep, target)
	return r.withPersistentContainerLock(func() (container.ContainerID, error) {
		info, err := r.engine.InspectContainer(ctx.Context, target.name)
		switch {
		case err == nil:
			return r.reusePersistentContainer(ctx, info, createOpts, target)
		case !errors.Is(err, container.ErrContainerNotFound):
			return "", fmt.Errorf("inspect persistent container %q: %w", target.name, err)
		case !target.createIfMissing:
			return "", missingPersistentContainerError(target.name)
		case !prep.imagePrepared:
			return "", fmt.Errorf(
				"persistent container %q disappeared before execution; retry the command to create it with prepared image state",
				target.name,
			)
		}

		created, createErr := r.engine.Create(ctx.Context, createOpts)
		if createErr != nil {
			if errors.Is(createErr, container.ErrContainerNameConflict) {
				info, inspectErr := r.engine.InspectContainer(ctx.Context, target.name)
				if inspectErr == nil {
					return r.reusePersistentContainer(ctx, info, createOpts, target)
				}
			}
			return "", fmt.Errorf("create persistent container %q: %w", target.name, createErr)
		}
		if err := r.engine.Start(ctx.Context, created.ContainerID); err != nil {
			return "", fmt.Errorf("start persistent container %q: %w", target.name, err)
		}
		return created.ContainerID, nil
	})
}

func (r *ContainerRuntime) reusePersistentContainer(ctx *ExecutionContext, info *container.ContainerInfo, createOpts container.CreateOptions, target persistentContainerTarget) (container.ContainerID, error) {
	if info == nil {
		return "", errors.New("persistent container inspect returned no container info")
	}
	if info.ContainerID == "" {
		return "", errors.New("persistent container inspect returned empty container ID")
	}

	managed := isManagedPersistentContainer(info)
	switch {
	case managed:
		if err := ensureManagedPersistentSpecMatches(info, createOpts); err != nil {
			return "", err
		}
	case target.nameSource != persistentContainerNameSourceCLI:
		return "", fmt.Errorf(
			"persistent container %q already exists but is not managed by invowk; use --ivk-container-name to target an existing external container",
			target.name,
		)
	case !info.Running:
		return "", fmt.Errorf("persistent container %q exists but is not running", target.name)
	}

	if managed && !info.Running {
		if err := r.engine.Start(ctx.Context, info.ContainerID); err != nil {
			return "", fmt.Errorf("start persistent container %q: %w", target.name, err)
		}
	}
	return info.ContainerID, nil
}

func (r *ContainerRuntime) persistentCreateOptions(ctx *ExecutionContext, prep *containerExecPrep, target persistentContainerTarget) container.CreateOptions {
	labels := persistentContainerLabels(ctx, prep, target)
	return container.CreateOptions{
		Image:      prep.image,
		Command:    slices.Clone(persistentContainerIdleCommand),
		Labels:     labels,
		Volumes:    slices.Clone(prep.volumes),
		Ports:      slices.Clone(prep.ports),
		Name:       target.name,
		ExtraHosts: slices.Clone(prep.extraHosts),
	}
}

//goplint:ignore -- Docker/Podman labels are stringly typed engine metadata.
func persistentContainerLabels(ctx *ExecutionContext, prep *containerExecPrep, target persistentContainerTarget) map[string]string {
	labels := map[string]string{
		persistentContainerLabelManaged:    persistentContainerManagedLabelTrue,
		persistentContainerLabelPersistent: persistentContainerManagedLabelTrue,
		persistentContainerLabelSpecHash:   persistentContainerSpecHash(prep),
	}
	if ctx != nil {
		if ctx.CommandFullName != "" {
			labels[persistentContainerLabelNamespace] = string(ctx.CommandFullName)
		} else if ctx.Command != nil {
			labels[persistentContainerLabelNamespace] = string(ctx.Command.Name)
		}
	}
	labels[persistentContainerLabelSource] = target.nameSource
	return labels
}

//goplint:ignore -- SHA-256 hex digest label value.
func persistentContainerSpecHash(prep *containerExecPrep) string {
	// Hash the image the managed container is actually created from. In
	// non-strict provisioning mode, preparation may fall back from the desired
	// provisioned tag to the base image; labeling that degraded container with
	// the desired tag would make later runs silently reuse the wrong target.
	image := prep.image
	parts := []string{
		"image=" + string(image),
		"command=" + strings.Join(persistentContainerIdleCommand, "\x00"),
	}
	for _, volume := range slices.Clone(prep.volumes) {
		parts = append(parts, "volume="+string(volume))
	}
	for _, port := range slices.Clone(prep.ports) {
		parts = append(parts, "port="+string(port))
	}
	for _, host := range slices.Clone(prep.extraHosts) {
		parts = append(parts, "host="+string(host))
	}
	slices.Sort(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func (r *ContainerRuntime) shouldSkipPersistentImagePreparation(ctx *ExecutionContext, cfg invowkfileContainerConfig) (bool, error) {
	target, ok := resolvePersistentContainerTarget(ctx, cfg)
	if !ok {
		return false, nil
	}
	_, err := r.engine.InspectContainer(ctx.Context, target.name)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, container.ErrContainerNotFound):
		if !target.createIfMissing {
			// Report the missing target before image preparation so a typoed
			// persistent name cannot build, provision, or otherwise mutate the
			// engine image cache for a command that must fail.
			return false, missingPersistentContainerError(target.name)
		}
		return false, nil
	default:
		return false, fmt.Errorf("inspect persistent container %q: %w", target.name, err)
	}
}

func missingPersistentContainerError(name container.ContainerName) error {
	return fmt.Errorf(
		"persistent container %q does not exist; create it first or set runtime.persistent.create_if_missing: true",
		name,
	)
}

func (r *ContainerRuntime) persistentSpecImage(ctx *ExecutionContext, cfg invowkfileContainerConfig) (container.ImageTag, error) {
	baseImage, err := r.unpreparedContainerImage(ctx, cfg)
	if err != nil || baseImage == "" {
		return baseImage, err
	}
	if r.provisioner == nil || r.provisionConfig == nil || !r.provisionConfig.Enabled {
		return baseImage, nil
	}
	resolver, ok := r.provisioner.(provisionedImageTagResolver)
	if !ok {
		return baseImage, nil
	}
	tag, err := resolver.GetProvisionedImageTag(ctx.Context, baseImage)
	if err != nil {
		return "", fmt.Errorf("persistent provisioned image identity: %w", err)
	}
	image := container.ImageTag(tag) //goplint:ignore -- validated immediately below.
	if err := image.Validate(); err != nil {
		return "", fmt.Errorf("persistent provisioned image identity: %w", err)
	}
	return image, nil
}

func (r *ContainerRuntime) unpreparedContainerImage(ctx *ExecutionContext, cfg invowkfileContainerConfig) (container.ImageTag, error) {
	if cfg.Image != "" {
		return cfg.Image, nil
	}
	if cfg.Containerfile == "" {
		return "", nil
	}
	if ctx == nil || ctx.Invowkfile == nil {
		return "", errors.New("invowkfile is required to derive container image tag")
	}
	tag, err := r.generateImageTag(string(ctx.Invowkfile.FilePath))
	if err != nil {
		return "", err
	}
	image := container.ImageTag(tag)
	if err := image.Validate(); err != nil {
		return "", err
	}
	return image, nil
}

func isManagedPersistentContainer(info *container.ContainerInfo) bool {
	return info.Labels[persistentContainerLabelManaged] == persistentContainerManagedLabelTrue &&
		info.Labels[persistentContainerLabelPersistent] == persistentContainerManagedLabelTrue
}

func ensureManagedPersistentSpecMatches(info *container.ContainerInfo, createOpts container.CreateOptions) error {
	expected := createOpts.Labels[persistentContainerLabelSpecHash]
	if expected == "" {
		return errors.New("persistent container create options missing spec label")
	}
	got := info.Labels[persistentContainerLabelSpecHash]
	if got == expected {
		return nil
	}
	return fmt.Errorf(
		"persistent container %q was created by invowk with a different runtime configuration; remove or rename it before reusing this target",
		info.Name,
	)
}

func (r *ContainerRuntime) withPersistentContainerLock(fn func() (container.ContainerID, error)) (container.ContainerID, error) {
	lock, lockErr := acquireContainerRunLock()
	if lockErr != nil {
		containerRunFallbackMu.Lock()
		defer containerRunFallbackMu.Unlock()
		return fn()
	}
	defer lock.Release()
	return fn()
}

func execOptionsForPersistent(ctx *ExecutionContext, prep *containerExecPrep, stdout, stderr io.Writer) container.RunOptions {
	return persistentExecOptions(ctx.IO.Stdin, prep, stdout, stderr)
}

func captureExecOptionsForPersistent(prep *containerExecPrep, stdout, stderr io.Writer) container.RunOptions {
	// Capture calls run dependency checks and other non-interactive probes.
	// They must not inherit terminal stdin, otherwise docker/podman exec -i can
	// block waiting for the user's terminal while the caller expects buffers.
	return persistentExecOptions(nil, prep, stdout, stderr)
}

func persistentExecOptions(stdin io.Reader, prep *containerExecPrep, stdout, stderr io.Writer) container.RunOptions {
	return container.RunOptions{
		WorkDir:     prep.workDir,
		Env:         maps.Clone(prep.env),
		Stdin:       stdin,
		Stdout:      stdout,
		Stderr:      stderr,
		Interactive: stdin != nil,
	}
}
