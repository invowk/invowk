// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	containerValidationScript string

	dependencyRuntimeProbeFactory struct{}

	dependencyRuntimeProbe struct {
		registry  *runtime.Registry
		parentCtx *runtime.ExecutionContext
	}
)

func (s containerValidationScript) String() string { return string(s) }

func (s containerValidationScript) Validate() error { return nil }

// NewDependencyRuntimeProbeFactory creates runtime probes for dependency validation.
func NewDependencyRuntimeProbeFactory() dependencyRuntimeProbeFactory {
	return dependencyRuntimeProbeFactory{}
}

// NewDependencyRuntimeProbe creates the production runtime probe for dependency checks.
func NewDependencyRuntimeProbe(registry *runtime.Registry, parentCtx *runtime.ExecutionContext) deps.RuntimeDependencyProbe {
	return dependencyRuntimeProbe{registry: registry, parentCtx: parentCtx}
}

// Create adapts a runtime registry into a runtime dependency probe.
func (dependencyRuntimeProbeFactory) Create(registry *runtime.Registry, parentCtx *runtime.ExecutionContext) deps.RuntimeDependencyProbe {
	return NewDependencyRuntimeProbe(registry, parentCtx)
}

// Validate returns nil when the runtime probe has a registry.
func (p dependencyRuntimeProbe) Validate() error {
	if p.registry == nil {
		return deps.ErrContainerRuntimeNotAvailable
	}
	if p.parentCtx == nil {
		return deps.ErrContainerRuntimeNotAvailable
	}
	if err := p.parentCtx.Validate(); err != nil {
		return fmt.Errorf("invalid dependency validation context: %w", err)
	}
	return nil
}

// CheckTool validates a tool dependency against the selected container runtime.
func (p dependencyRuntimeProbe) CheckTool(tool invowkfile.BinaryName) error {
	toolName := string(tool)
	if !deps.ToolNamePattern.MatchString(toolName) {
		return fmt.Errorf("%s - invalid tool name for shell interpolation", tool)
	}

	script := fmt.Sprintf("command -v '%s' || which '%s'", deps.ShellEscapeSingleQuote(toolName), deps.ShellEscapeSingleQuote(toolName))
	result, _, _, err := p.runContainerValidation(script)
	if err != nil {
		return fmt.Errorf("%s - %w", tool, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s - not available in container", tool)
	}
	return nil
}

// CheckFilepath validates a filepath dependency against the selected container runtime.
func (p dependencyRuntimeProbe) CheckFilepath(fp invowkfile.FilepathDependency) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("(no paths specified) - %w", deps.ErrNoPathAlternatives)
	}

	var allErrors []string
	for _, altPath := range fp.Alternatives {
		detail, err := p.checkFilepathAlternative(string(altPath), fp)
		if err != nil {
			return err
		}
		if detail == "" {
			return nil
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, detail))
	}

	if len(allErrors) == 1 {
		return errors.New(allErrors[0])
	}
	return fmt.Errorf("none of the alternatives satisfied the requirements in container: %s", strings.Join(allErrors, "; "))
}

// CheckEnvVar validates an environment variable dependency against the selected container runtime.
func (p dependencyRuntimeProbe) CheckEnvVar(envVar invowkfile.EnvVarCheck) error {
	name := strings.TrimSpace(string(envVar.Name))
	if name == "" {
		return errors.New("(empty) - environment variable name cannot be empty")
	}
	if err := invowkfile.ValidateEnvVarName(name); err != nil {
		return fmt.Errorf("%s - %w", name, err)
	}

	script := fmt.Sprintf("test -n \"${%s+x}\"", name)
	if envVar.Validation != "" {
		escapedValidation := deps.ShellEscapeSingleQuote(string(envVar.Validation))
		script = fmt.Sprintf("test -n \"${%s+x}\" && printf '%%s' \"$%s\" | grep -qE '%s'", name, name, escapedValidation)
	}

	result, _, _, err := p.runContainerValidation(script)
	if err != nil {
		return fmt.Errorf("%w for env var %s", err, name)
	}
	if result.ExitCode == 0 {
		return nil
	}
	if envVar.Validation != "" {
		return fmt.Errorf("%s - not set or value does not match pattern '%s' in container", name, envVar.Validation.String())
	}
	return fmt.Errorf("%s - %w", name, deps.ErrContainerEnvVarNotSet)
}

// CheckCapability validates a capability dependency against the selected container runtime.
func (p dependencyRuntimeProbe) CheckCapability(capability invowkfile.CapabilityName) error {
	script := deps.CapabilityCheckScript(capability)
	if script == "" {
		return fmt.Errorf("%s - unknown capability", capability)
	}

	result, _, _, err := p.runContainerValidation(script)
	if err != nil {
		return fmt.Errorf("%w for capability %s", err, capability)
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("%s - not available in container", capability)
}

// CheckCommand validates command discoverability against the selected container runtime.
func (p dependencyRuntimeProbe) CheckCommand(command invowkfile.CommandName) error {
	script := fmt.Sprintf("invowk internal check-cmd '%s'", deps.ShellEscapeSingleQuote(command.String()))
	result, _, stderr, err := p.runContainerValidation(script)
	if err != nil {
		stderrStr := strings.TrimSpace(stderr)
		if stderrStr != "" {
			return fmt.Errorf("%w for command %s (%s)", err, command, stderrStr)
		}
		return fmt.Errorf("%w for command %s", err, command)
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("command %s %w", command, deps.ErrContainerCommandNotFound)
}

// RunCustomCheck validates a custom check against the selected container runtime.
func (p dependencyRuntimeProbe) RunCustomCheck(check invowkfile.CustomCheck) error {
	result, stdout, stderr, err := p.runContainerValidation(string(check.CheckScript))
	if err != nil {
		return fmt.Errorf("%s - %w", check.Name, err)
	}

	output := strings.TrimSpace(stdout + stderr)
	return deps.ValidateCustomCheckOutput(check, output, result.Error)
}

//goplint:ignore -- runtime adapter translates typed dependency paths into shell probe strings at the container boundary.
func (p dependencyRuntimeProbe) checkFilepathAlternative(altPath string, fp invowkfile.FilepathDependency) (string, error) {
	script := buildContainerFilepathCheckScript(fp, altPath)
	result, _, stderr, err := p.runContainerValidation(script)
	if err != nil {
		return "", fmt.Errorf("%w for path %s", err, altPath)
	}
	if result.ExitCode == 0 {
		return "", nil
	}

	detail := "not found or permission denied in container"
	if stderrStr := strings.TrimSpace(stderr); stderrStr != "" {
		detail = stderrStr
	}
	return detail, nil
}

//goplint:ignore -- runtime adapter captures shell script text and process streams from the container boundary.
func (p dependencyRuntimeProbe) runContainerValidation(script string) (result *runtime.Result, stdout, stderr string, err error) {
	rt, err := p.containerRuntime()
	if err != nil {
		return nil, "", "", err
	}

	validationScript := containerValidationScript(script)
	if err := validationScript.Validate(); err != nil {
		return nil, "", "", err
	}
	validationCtx, stdoutBuf, stderrBuf := p.newContainerValidationContext(validationScript)
	result = rt.Execute(validationCtx)
	if result.Error != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](result.Error); !ok || exitErr == nil {
			return result, stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("%w: %w", deps.ErrContainerValidationFailed, result.Error)
		}
	}
	if err := checkTransientExitCode(result, validationScript); err != nil {
		return result, stdoutBuf.String(), stderrBuf.String(), err
	}
	return result, stdoutBuf.String(), stderrBuf.String(), nil
}

func (p dependencyRuntimeProbe) newContainerValidationContext(script containerValidationScript) (execCtx *runtime.ExecutionContext, stdout, stderr *bytes.Buffer) {
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	selectedImpl := invowkfile.Implementation{
		Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}},
	}
	if p.parentCtx.SelectedImpl != nil {
		selectedImpl = *p.parentCtx.SelectedImpl
	}
	selectedImpl.Script = invowkfile.ScriptContent(script) //goplint:ignore -- inline validation script owned by runtime adapter
	selectedRuntime := p.parentCtx.SelectedRuntime
	if selectedRuntime == "" {
		selectedRuntime = invowkfile.RuntimeContainer
	}
	execCtx = &runtime.ExecutionContext{
		Command:         p.parentCtx.Command,
		Invowkfile:      p.parentCtx.Invowkfile,
		SelectedImpl:    &selectedImpl,
		SelectedRuntime: selectedRuntime,
		Context:         p.parentCtx.Context,
		PositionalArgs:  p.parentCtx.PositionalArgs,
		WorkDir:         p.parentCtx.WorkDir,
		Verbose:         p.parentCtx.Verbose,
		ForceRebuild:    p.parentCtx.ForceRebuild,
		ExecutionID:     p.parentCtx.ExecutionID,
		IO:              runtime.IOContext{Stdout: stdout, Stderr: stderr},
		Env:             p.parentCtx.Env,
		TUI:             p.parentCtx.TUI,
	}
	return execCtx, stdout, stderr
}

func checkTransientExitCode(result *runtime.Result, label containerValidationScript) error {
	if runtime.IsTransientContainerEngineExitCode(result.ExitCode) {
		return fmt.Errorf("%s - %w (exit code %s)", label, deps.ErrContainerEngineFailure, result.ExitCode)
	}
	return nil
}

func (p dependencyRuntimeProbe) containerRuntime() (runtime.Runtime, error) {
	if p.registry == nil {
		return nil, deps.ErrContainerRuntimeNotAvailable
	}
	rt, err := p.registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return nil, fmt.Errorf("%w for dependency validation", deps.ErrContainerRuntimeNotAvailable)
	}
	return rt, nil
}

//goplint:ignore -- runtime adapter builds shell probe text from a typed filepath dependency at the container boundary.
func buildContainerFilepathCheckScript(fp invowkfile.FilepathDependency, altPath string) string {
	escapedPath := deps.ShellEscapeSingleQuote(altPath)
	checks := []string{fmt.Sprintf("test -e '%s'", escapedPath)}
	if fp.Readable {
		checks = append(checks, fmt.Sprintf("test -r '%s'", escapedPath))
	}
	if fp.Writable {
		checks = append(checks, fmt.Sprintf("test -w '%s'", escapedPath))
	}
	if fp.Executable {
		checks = append(checks, fmt.Sprintf("test -x '%s'", escapedPath))
	}
	return strings.Join(checks, " && ")
}
