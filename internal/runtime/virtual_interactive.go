// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type virtualInteractiveSubprocess struct {
	//goplint:ignore -- prepared path is validated before construction; command specs expose it as a pointer.
	scriptFile types.FilesystemPath
	//goplint:ignore -- prepared path is validated before construction; command specs expose it as a pointer.
	workDir types.FilesystemPath
	//goplint:ignore -- prepared path is validated before construction; command specs expose it as a pointer.
	scriptBasePath types.FilesystemPath
	//goplint:ignore -- adapter-specific command specs wrap serialized JSON in runtime-specific value types.
	envJSON []byte
	//goplint:ignore -- runtime config stores allowed binaries as strings after validation.
	allowedBinaries  []string
	binaryLookupMode invowkfile.BinaryLookupMode
	filesystemAccess invowkfile.VirtualFilesystemAccess
	filesystemPaths  invowkfile.VirtualFilesystemPaths
	runtimeCfg       *invowkfile.RuntimeConfig
	cleanup          func()
}

func (p virtualInteractiveSubprocess) Validate() error {
	var runtimeCfgErr error
	if p.runtimeCfg != nil {
		runtimeCfgErr = p.runtimeCfg.Validate()
	}
	return errors.Join(
		p.scriptFile.Validate(),
		p.workDir.Validate(),
		p.scriptBasePath.Validate(),
		p.binaryLookupMode.Validate(),
		p.filesystemAccess.Validate(),
		p.filesystemPaths.Validate(),
		runtimeCfgErr,
	)
}

//goplint:ignore -- internal helper coordinates temp script labels from runtime adapters.
func prepareVirtualInteractiveSubprocess(
	ctx *ExecutionContext,
	script string,
	tempPattern string,
	scriptLabel string,
	interactiveLabel string,
	envBuilder EnvBuilder,
) (*virtualInteractiveSubprocess, error) {
	tempPath, err := writeVirtualInteractiveScript(script, tempPattern, scriptLabel)
	if err != nil {
		return nil, err
	}
	cleanup := func() {
		_ = os.Remove(tempPath)
	}

	prepared, err := buildVirtualInteractiveSubprocess(ctx, envBuilder, tempPath, cleanup, interactiveLabel)
	if err != nil {
		cleanup()
		return nil, err
	}
	return prepared, nil
}

//goplint:ignore -- internal helper receives temp script text and error labels from runtime adapters.
func writeVirtualInteractiveScript(script, tempPattern, scriptLabel string) (string, error) {
	tmpFile, err := os.CreateTemp("", tempPattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp %s file: %w", scriptLabel, err)
	}
	tempPath := tmpFile.Name()
	if _, err = tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to write temp %s: %w", scriptLabel, err)
	}
	if err = tmpFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to close temp %s: %w", scriptLabel, err)
	}
	return tempPath, nil
}

//goplint:ignore -- internal helper carries a validated temp path and error label from the prepare step.
func buildVirtualInteractiveSubprocess(
	ctx *ExecutionContext,
	envBuilder EnvBuilder,
	tempPath string,
	cleanup func(),
	interactiveLabel string,
) (*virtualInteractiveSubprocess, error) {
	workDir := ctx.EffectiveWorkDir()
	workDirPath := types.FilesystemPath(workDir)
	if err := workDirPath.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s workdir: %w", interactiveLabel, err)
	}

	env, err := envBuilder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		return nil, fmt.Errorf(failedBuildEnvironmentFmt, err)
	}
	filesystem := selectedVirtualFilesystem(ctx)
	pathResolver, err := newVirtualPathResolver(ctx)
	if err != nil {
		return nil, err
	}
	addVirtualRuntimeEnv(env, pathResolver)
	ctx.AddTUIEnv(env)

	envJSON, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize environment: %w", err)
	}

	scriptFile := types.FilesystemPath(tempPath)
	if err := scriptFile.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s script file: %w", interactiveLabel, err)
	}
	scriptBasePath := ctx.Invowkfile.GetScriptBasePath()
	if err := scriptBasePath.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s script base path: %w", interactiveLabel, err)
	}

	runtimeCfg := selectedRuntimeConfig(ctx)
	prepared := &virtualInteractiveSubprocess{
		scriptFile:       scriptFile,
		workDir:          workDirPath,
		scriptBasePath:   scriptBasePath,
		envJSON:          envJSON,
		allowedBinaries:  allowedBinaryStrings(runtimeCfg),
		binaryLookupMode: binaryLookupMode(runtimeCfg),
		filesystemAccess: pathResolver.access,
		filesystemPaths:  filesystem.Paths,
		runtimeCfg:       runtimeCfg,
		cleanup:          cleanup,
	}
	if err := prepared.Validate(); err != nil {
		return nil, err
	}
	return prepared, nil
}
