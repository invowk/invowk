// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type filepathStubRuntime struct {
	ctx    context.Context
	execFn func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result
}

func newFilepathStubRuntime(
	t *testing.T,
	execFns ...func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result,
) *filepathStubRuntime {
	t.Helper()

	if len(execFns) > 1 {
		t.Fatalf("newFilepathStubRuntime() received %d execution functions, want at most 1", len(execFns))
	}
	stub := &filepathStubRuntime{ctx: t.Context()}
	if len(execFns) == 1 {
		stub.execFn = execFns[0]
	}
	return stub
}

func (s *filepathStubRuntime) Name() string { return "stub" }

func (s *filepathStubRuntime) Execute(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
	if s.execFn != nil {
		return s.execFn(ctx)
	}
	return &runtimepkg.Result{ExitCode: 0}
}

func (s *filepathStubRuntime) Available() bool { return true }

func (s *filepathStubRuntime) Validate(_ *runtimepkg.ExecutionContext) error { return nil }

func (s *filepathStubRuntime) CheckTool(tool invowkfile.BinaryName) error {
	toolName := string(tool)
	if !ToolNamePattern.MatchString(toolName) {
		return fmt.Errorf("%s - invalid tool name for shell interpolation", tool)
	}
	result, _, _, err := s.runValidation(fmt.Sprintf("command -v '%s' || which '%s'", ShellEscapeSingleQuote(toolName), ShellEscapeSingleQuote(toolName)))
	if err != nil {
		return fmt.Errorf("%s - %w", tool, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s - not available in container", tool)
	}
	return nil
}

func (s *filepathStubRuntime) CheckFilepath(fp invowkfile.FilepathDependency) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("(no paths specified) - %w", ErrNoPathAlternatives)
	}
	var allErrors []string
	for _, altPath := range fp.Alternatives {
		script := testContainerFilepathCheckScript(fp, string(altPath))
		result, _, stderr, err := s.runValidation(script)
		if err != nil {
			return fmt.Errorf("%w for path %s", err, altPath)
		}
		if result.ExitCode == 0 {
			return nil
		}
		detail := "not found or permission denied in container"
		if stderrStr := strings.TrimSpace(stderr); stderrStr != "" {
			detail = stderrStr
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, detail))
	}
	if len(allErrors) == 1 {
		return errors.New(allErrors[0])
	}
	return fmt.Errorf("none of the alternatives satisfied the requirements in container: %s", strings.Join(allErrors, "; "))
}

func (s *filepathStubRuntime) CheckEnvVar(envVar invowkfile.EnvVarCheck) error {
	name := strings.TrimSpace(string(envVar.Name))
	if name == "" {
		return errors.New("(empty) - environment variable name cannot be empty")
	}
	if err := invowkfile.ValidateEnvVarName(name); err != nil {
		return fmt.Errorf("%s - %w", name, err)
	}
	script := fmt.Sprintf("test -n \"${%s+x}\"", name)
	if envVar.Validation != "" {
		script = fmt.Sprintf("test -n \"${%s+x}\" && printf '%%s' \"$%s\" | grep -qE '%s'", name, name, ShellEscapeSingleQuote(string(envVar.Validation)))
	}
	result, _, _, err := s.runValidation(script)
	if err != nil {
		return fmt.Errorf("%w for env var %s", err, name)
	}
	if result.ExitCode == 0 {
		return nil
	}
	if envVar.Validation != "" {
		return fmt.Errorf("%s - not set or value does not match pattern '%s' in container", name, envVar.Validation.String())
	}
	return fmt.Errorf("%s - %w", name, ErrContainerEnvVarNotSet)
}

func (s *filepathStubRuntime) CheckCapability(capability invowkfile.CapabilityName) error {
	script := testCapabilityCheckScript(capability)
	if script == "" {
		return fmt.Errorf("%s - unknown capability", capability)
	}
	result, _, _, err := s.runValidation(script)
	if err != nil {
		return fmt.Errorf("%w for capability %s", err, capability)
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("%s - not available in container", capability)
}

func testCapabilityCheckScript(capName invowkfile.CapabilityName) string {
	switch capName {
	case invowkfile.CapabilityInternet:
		return "ping -c 1 -W 2 8.8.8.8 2>/dev/null || curl -sf --max-time 2 https://google.com >/dev/null 2>&1"
	case invowkfile.CapabilityContainers:
		return "command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1"
	case invowkfile.CapabilityLocalAreaNetwork:
		return "ip route 2>/dev/null | grep -q default || route -n 2>/dev/null | grep -q '^0.0.0.0'"
	case invowkfile.CapabilityTTY:
		return "test -t 0"
	default:
		return ""
	}
}

func (s *filepathStubRuntime) CheckCommand(command invowkfile.CommandName) error {
	result, _, stderr, err := s.runValidation(fmt.Sprintf("invowk internal check-cmd '%s'", ShellEscapeSingleQuote(command.String())))
	if err != nil {
		if stderrStr := strings.TrimSpace(stderr); stderrStr != "" {
			return fmt.Errorf("%w for command %s (%s)", err, command, stderrStr)
		}
		return fmt.Errorf("%w for command %s", err, command)
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("command %s %w", command, ErrContainerCommandNotFound)
}

func (s *filepathStubRuntime) RunCustomCheck(check invowkfile.CustomCheck) (CustomCheckResult, error) {
	result, stdout, stderr, err := s.runValidationScript(invowkfile.ImplementationScript{
		Content:     check.Script.Content,
		Interpreter: check.Script.Interpreter,
	})
	if err != nil {
		return CustomCheckResult{}, fmt.Errorf("%s - %w", check.Name, err)
	}
	output := CustomCheckOutput(strings.TrimSpace(stdout + stderr))
	if validateErr := output.Validate(); validateErr != nil {
		return CustomCheckResult{}, fmt.Errorf("custom check output: %w", validateErr)
	}
	return NewCustomCheckResult(output, result.ExitCode)
}

func (s *filepathStubRuntime) runValidation(script string) (result *runtimepkg.Result, stdout, stderr string, err error) {
	return s.runValidationScript(invowkfile.ImplementationScript{Content: invowkfile.ScriptContent(script)}) //goplint:ignore -- test runtime probe script
}

func (s *filepathStubRuntime) runValidationScript(script invowkfile.ImplementationScript) (result *runtimepkg.Result, stdout, stderr string, err error) {
	if s.ctx == nil {
		panic("filepathStubRuntime must be created with newFilepathStubRuntime")
	}

	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	validationCtx := &runtimepkg.ExecutionContext{
		Command:         &invowkfile.Command{Name: "build"},
		SelectedRuntime: invowkfile.RuntimeContainer,
		SelectedImpl: &invowkfile.Implementation{
			Script:   script,
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}},
		},
		Context: s.ctx,
		IO: runtimepkg.IOContext{
			Stdout: stdoutBuf,
			Stderr: stderrBuf,
		},
	}
	result = s.Execute(validationCtx)
	if result.Error != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](result.Error); !ok || exitErr == nil {
			return result, stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("%w: %w", ErrContainerValidationFailed, result.Error)
		}
	}
	if runtimepkg.IsTransientContainerEngineExitCode(result.ExitCode) {
		err := fmt.Errorf("%s - %w (exit code %s)", script, ErrContainerEngineFailure, result.ExitCode)
		return result, stdoutBuf.String(), stderrBuf.String(), err
	}
	return result, stdoutBuf.String(), stderrBuf.String(), nil
}

func testContainerFilepathCheckScript(fp invowkfile.FilepathDependency, altPath string) string {
	escapedPath := ShellEscapeSingleQuote(altPath)
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

func TestCheckFilepathDependenciesInContainer(t *testing.T) {
	t.Parallel()

	execCtx := newDependencyExecutionContext(t)

	tests := []struct {
		name              string
		deps              *invowkfile.DependsOn
		hasRuntime        bool
		stderr            string
		exitCode          types.ExitCode
		wantIs            error
		wantDependencyErr bool
		wantDetail        string
	}{
		{name: "nil dependencies skip probe"},
		{name: "empty dependencies skip probe", deps: &invowkfile.DependsOn{}},
		{name: "missing container runtime", deps: testContainerFilepathDeps(), wantIs: ErrRuntimeDependencyProbeRequired},
		{name: "dependency error aggregates failing paths", deps: testContainerFilepathDeps(), hasRuntime: true, stderr: "permission denied", exitCode: 1, wantDependencyErr: true, wantDetail: "permission denied"},
		{name: "success", deps: testContainerFilepathDeps(), hasRuntime: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var runtime RuntimeDependencyProbe
			if tt.hasRuntime {
				runtime = newFilepathStubRuntime(t, func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
					if tt.stderr != "" {
						if _, err := io.WriteString(ctx.IO.Stderr, tt.stderr); err != nil {
							t.Errorf("WriteString(): %v", err)
						}
					}
					return &runtimepkg.Result{ExitCode: tt.exitCode}
				})
			}
			err := CheckFilepathDependenciesInContainer(tt.deps, runtime, execCtx)
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("error = %v, want wrapping %v", err, tt.wantIs)
			}
			if tt.wantDependencyErr {
				depErr := requireFilepathContainerDependencyError(t, err, execCtx)
				requireDependencyFailureKinds(t, depErr.StructuredFailures, DependencyFailureFilepath)
				if got := depErr.StructuredFailures[0].Detail().String(); !strings.Contains(got, tt.wantDetail) {
					t.Fatalf("StructuredFailures[0].Detail() = %q, want containing %q", got, tt.wantDetail)
				}
				return
			}
			if tt.wantIs == nil && err != nil {
				t.Fatalf("CheckFilepathDependenciesInContainer() = %v, want nil", err)
			}
		})
	}
}

func testContainerFilepathDeps() *invowkfile.DependsOn {
	return &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
	}
}

func requireFilepathContainerDependencyError(t *testing.T, err error, execCtx ExecutionContext) *DependencyError {
	t.Helper()

	if err == nil {
		t.Fatal("expected error")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
	if len(depErr.MissingFilepaths) != 1 {
		t.Fatalf("len(depErr.MissingFilepaths) = %d, want 1", len(depErr.MissingFilepaths))
	}
	if depErr.CommandName != execCtx.CommandName {
		t.Fatalf("DependencyError.CommandName = %q, want %q", depErr.CommandName, execCtx.CommandName)
	}
	return depErr
}

func TestCheckHostFilepathDependenciesRequiresProbe(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{
			Alternatives: []invowkfile.FilesystemPath{"missing.txt"},
		}},
	}

	err := CheckHostFilepathDependencies(
		deps,
		types.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue")),
		newDependencyExecutionContext(t),
	)
	depErr := requireDependencyError(t, err)
	if depErr.CommandName != "build" {
		t.Fatalf("DependencyError.CommandName = %q, want build", depErr.CommandName)
	}
	if len(depErr.MissingFilepaths) != 1 {
		t.Fatalf("MissingFilepaths = %v, want one missing filepath", depErr.MissingFilepaths)
	}
	if got := depErr.MissingFilepaths[0].String(); !strings.Contains(got, ErrHostProbeRequired.Error()) {
		t.Fatalf("MissingFilepaths[0] = %q, want ErrHostProbeRequired detail", got)
	}
}

func TestCheckHostFilepathDependenciesWithProbeReportsValidationFailure(t *testing.T) {
	t.Parallel()

	invowkDir := t.TempDir()
	invowkfilePath := types.FilesystemPath(filepath.Join(invowkDir, "invowkfile.cue"))
	relativePath := invowkfile.FilesystemPath("missing.txt")
	resolvedPath := types.FilesystemPath(filepath.Join(invowkDir, string(relativePath)))
	probe := &recordingHostProbe{
		filepathErrors: map[types.FilesystemPath]error{
			resolvedPath: errors.New("missing file"),
		},
	}
	deps := &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{
			Alternatives: []invowkfile.FilesystemPath{relativePath},
			Readable:     true,
		}},
	}

	err := CheckHostFilepathDependenciesWithProbe(
		deps,
		invowkfilePath,
		ExecutionContext{CommandName: "build"},
		probe,
	)
	depErr := requireDependencyError(t, err)
	if len(depErr.MissingFilepaths) != 1 {
		t.Fatalf("MissingFilepaths = %v, want one missing filepath", depErr.MissingFilepaths)
	}
	requireDependencyFailureKinds(t, depErr.StructuredFailures, DependencyFailureFilepath)
	if got := depErr.StructuredFailures[0].Detail().String(); !strings.Contains(got, "missing file") {
		t.Fatalf("StructuredFailures[0].Detail() = %q, want containing missing file", got)
	}
	if len(probe.filepaths) != 1 || probe.filepaths[0] != resolvedPath {
		t.Fatalf("probe filepaths = %v, want [%s]", probe.filepaths, resolvedPath)
	}
}

func TestValidateFilepathInContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		alternatives []invowkfile.FilesystemPath
		stderr       string
		exitCode     types.ExitCode
		runtimeErr   error
		wantIs       error
		wantContains string
	}{
		{name: "requires at least one alternative", wantIs: ErrNoPathAlternatives},
		{name: "succeeds on first passing alternative", alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}},
		{name: "reports aggregated detail from stderr", alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}, stderr: "permission denied", exitCode: 1, wantContains: "permission denied"},
		{name: "transient exit code is surfaced", alternatives: []invowkfile.FilesystemPath{"/tmp"}, exitCode: 125, wantIs: ErrContainerEngineFailure},
		{name: "runtime error is propagated", alternatives: []invowkfile.FilesystemPath{"/tmp"}, exitCode: 1, runtimeErr: errors.New("engine down"), wantIs: ErrContainerValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := newFilepathStubRuntime(t, func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if tt.stderr != "" {
					if _, err := io.WriteString(ctx.IO.Stderr, tt.stderr); err != nil {
						t.Errorf("WriteString(): %v", err)
					}
				}
				return &runtimepkg.Result{ExitCode: tt.exitCode, Error: tt.runtimeErr}
			})
			err := runtime.CheckFilepath(invowkfile.FilepathDependency{Alternatives: tt.alternatives})
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("error = %v, want wrapping %v", err, tt.wantIs)
			}
			if tt.wantContains != "" && (err == nil || !strings.Contains(err.Error(), tt.wantContains)) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantContains)
			}
			if tt.wantIs == nil && tt.wantContains == "" && err != nil {
				t.Fatalf("CheckFilepath() = %v, want nil", err)
			}
		})
	}
}

func TestContainerFilepathHelpers(t *testing.T) {
	t.Parallel()

	script := testContainerFilepathCheckScript(
		invowkfile.FilepathDependency{Readable: true, Writable: true, Executable: true},
		"/tmp/that's-it",
	)
	if !strings.Contains(script, "test -e '/tmp/that'\\''s-it'") {
		t.Fatalf("script = %q", script)
	}
	if !strings.Contains(script, "test -r") || !strings.Contains(script, "test -w") || !strings.Contains(script, "test -x") {
		t.Fatalf("script = %q", script)
	}

	err := (newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			_, _ = io.WriteString(ctx.IO.Stderr, "missing permissions")
			return &runtimepkg.Result{ExitCode: 1}
		})).CheckFilepath(invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}})
	if !strings.Contains(err.Error(), "missing permissions") {
		t.Fatalf("err = %q, want containing 'missing permissions'", err.Error())
	}
}

func TestValidateFilepathAlternatives(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	invowkDir := types.FilesystemPath(tmpDir)

	tests := []struct {
		name         string
		alternatives []invowkfile.FilesystemPath
		probeErrors  map[types.FilesystemPath]error
		useWrapper   bool
		nilProbe     bool
		wantErr      bool
		wantIs       error
	}{
		{name: "existing path succeeds", alternatives: []invowkfile.FilesystemPath{invowkfile.FilesystemPath(existingFile)}},
		{name: "missing path returns error", alternatives: []invowkfile.FilesystemPath{"/nonexistent/path"}, probeErrors: map[types.FilesystemPath]error{"/nonexistent/path": fmt.Errorf("/nonexistent/path: %w", ErrPathNotExists)}, wantErr: true},
		{name: "first missing second exists succeeds", alternatives: []invowkfile.FilesystemPath{"/nonexistent", invowkfile.FilesystemPath(existingFile)}, probeErrors: map[types.FilesystemPath]error{"/nonexistent": fmt.Errorf("/nonexistent: %w", ErrPathNotExists)}},
		{name: "empty alternatives returns error", useWrapper: true, wantErr: true, wantIs: ErrNoPathAlternatives},
		{name: "non-empty alternatives require host probe", alternatives: []invowkfile.FilesystemPath{"missing.txt"}, nilProbe: true, wantErr: true, wantIs: ErrHostProbeRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fp := invowkfile.FilepathDependency{Alternatives: tt.alternatives}
			var err error
			if tt.useWrapper {
				err = ValidateFilepathAlternatives(fp, invowkDir)
			} else {
				var probe HostProbe
				if !tt.nilProbe {
					probe = &recordingHostProbe{filepathErrors: tt.probeErrors}
				}
				err = ValidateFilepathAlternativesWithProbe(fp, invowkDir, probe)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("error = %v, want wrapping %v", err, tt.wantIs)
			}
		})
	}
}

func TestHostFilepathMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{name: "absolute slash path is not joined to invowkfile directory", operation: "resolve", want: "/must/stay/absolute"},
		{name: "single alternative keeps raw probe error", operation: "format", want: "missing.txt: not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got string
			switch tt.operation {
			case "resolve":
				got = resolveHostFilepathAlternative(types.FilesystemPath(t.TempDir()), tt.want)
			case "format":
				got = formatHostFilepathError([]invowkfile.FilesystemPath{"missing.txt"}, []string{tt.want}).Error()
			default:
				t.Fatalf("unknown operation %q", tt.operation)
			}
			if got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvaluateAlternatives(t *testing.T) {
	t.Parallel()

	errFail := errors.New("failed")

	tests := []struct {
		name          string
		alternatives  []string
		checkMode     string
		wantFound     bool
		wantErr       bool
		wantErrDetail string
		wantCalls     int
	}{
		{name: "empty alternatives", checkMode: "pass"},
		{name: "single passing alternative", alternatives: []string{"a"}, checkMode: "pass", wantFound: true, wantCalls: 1},
		{name: "single failing alternative", alternatives: []string{"a"}, checkMode: "fixed-fail", wantErr: true, wantCalls: 1},
		{name: "first passes short circuits", alternatives: []string{"a", "b"}, checkMode: "pass", wantFound: true, wantCalls: 1},
		{name: "all fail returns last error", alternatives: []string{"a", "b"}, checkMode: "named-fail", wantErr: true, wantErrDetail: "failed: b", wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			callCount := 0
			found, lastErr := EvaluateAlternatives(tt.alternatives, func(value string) error {
				callCount++
				switch tt.checkMode {
				case "pass":
					return nil
				case "fixed-fail":
					return errFail
				case "named-fail":
					return fmt.Errorf("failed: %s", value)
				default:
					return fmt.Errorf("unknown check mode %q", tt.checkMode)
				}
			})
			if found != tt.wantFound || (lastErr != nil) != tt.wantErr {
				t.Fatalf("got (%v, %v), want found=%v wantErr=%v", found, lastErr, tt.wantFound, tt.wantErr)
			}
			if tt.wantErrDetail != "" && !strings.Contains(lastErr.Error(), tt.wantErrDetail) {
				t.Fatalf("lastErr = %v, want containing %q", lastErr, tt.wantErrDetail)
			}
			if callCount != tt.wantCalls {
				t.Fatalf("check calls = %d, want %d", callCount, tt.wantCalls)
			}
		})
	}
}

// TestValidateFilepathAlternatives_Matrix exercises the canonical
// seven-vector cross-platform path matrix against
// ValidateFilepathAlternativesWithProbe by inspecting what the function
// resolves before passing to the probe. The recording probe always
// returns nil, so the test infers behavior from the captured
// resolvedPath. This is the test that would have caught v0.10.0 bug #1.
//
// Resolver behavior captured here:
//   - UnixAbsolute "/foo" passes through unchanged on every platform
//     (the strings.HasPrefix("/") guard added in v0.10.0).
//   - WindowsDriveAbs "C:\foo" passes through on Windows
//     (filepath.IsAbs true) but is joined to invowkDir on Linux/macOS
//     because the resolver only treats it as absolute when the host
//     filepath package agrees.
//   - WindowsRooted "\foo", UNC "\\server\share", and traversal forms
//     are joined to invowkDir as relative segments.
func TestValidateFilepathAlternatives_Matrix(t *testing.T) {
	t.Parallel()
	invowkDir := types.FilesystemPath(t.TempDir())

	resolveFor := func(input string) (string, error) {
		probe := &recordingHostProbe{}
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{invowkfile.FilesystemPath(input)},
		}
		if err := ValidateFilepathAlternativesWithProbe(fp, invowkDir, probe); err != nil {
			return "", err
		}
		if len(probe.filepaths) == 0 {
			return "", errors.New("probe never called")
		}
		return string(probe.filepaths[0]), nil
	}

	// Platform-divergent vectors use PassHostNativeAbs: pass-through
	// when filepath.IsAbs is true on the running platform, joined with
	// invowkDir otherwise. This matches the resolver's actual contract
	// and produces correct expectations on every platform automatically.
	expect := pathmatrix.Expectations{
		UnixAbsolute:       pathmatrix.Pass(pathmatrix.InputUnixAbsolute),
		WindowsDriveAbs:    pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsDriveAbs),
		WindowsRooted:      pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsRooted),
		UNC:                pathmatrix.PassHostNativeAbs(pathmatrix.InputUNC),
		SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal),
		BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal),
		ValidRelative:      pathmatrix.PassAny(nil),
	}
	pathmatrix.Resolver(t, string(invowkDir), resolveFor, expect)
}
