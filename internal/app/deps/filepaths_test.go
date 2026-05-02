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
	execFn func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result
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
	script := CapabilityCheckScript(capability)
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
	result, stdout, stderr, err := s.runValidation(string(check.CheckScript))
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
	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	validationCtx := &runtimepkg.ExecutionContext{
		Command:         &invowkfile.Command{Name: "build"},
		SelectedRuntime: invowkfile.RuntimeContainer,
		SelectedImpl: &invowkfile.Implementation{
			Script:   invowkfile.ScriptContent(script), //goplint:ignore -- test runtime probe script
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}},
		},
		Context: context.Background(),
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

	t.Run("missing container runtime", func(t *testing.T) {
		t.Parallel()

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}
		err := CheckFilepathDependenciesInContainer(deps, nil, execCtx)
		if err == nil || !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
			t.Fatalf("err = %v, want wrapping ErrRuntimeDependencyProbeRequired", err)
		}
	})

	t.Run("dependency error aggregates failing paths", func(t *testing.T) {
		t.Parallel()

		probe := &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if _, err := io.WriteString(ctx.IO.Stderr, "permission denied"); err != nil {
					t.Fatalf("WriteString(): %v", err)
				}
				return &runtimepkg.Result{ExitCode: 1}
			},
		}

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}

		err := CheckFilepathDependenciesInContainer(deps, probe, execCtx)
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
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		probe := &filepathStubRuntime{}

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}

		if err := CheckFilepathDependenciesInContainer(deps, probe, execCtx); err != nil {
			t.Fatalf("CheckFilepathDependenciesInContainer() = %v", err)
		}
	})
}

func TestValidateFilepathInContainer(t *testing.T) {
	t.Parallel()

	t.Run("requires at least one alternative", func(t *testing.T) {
		t.Parallel()

		err := (&filepathStubRuntime{}).CheckFilepath(invowkfile.FilepathDependency{})
		if !errors.Is(err, ErrNoPathAlternatives) {
			t.Fatalf("err = %v, want wrapping ErrNoPathAlternatives", err)
		}
	})

	t.Run("succeeds on first passing alternative", func(t *testing.T) {
		t.Parallel()

		err := (&filepathStubRuntime{}).CheckFilepath(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}},
		)
		if err != nil {
			t.Fatalf("ValidateFilepathInContainer() = %v", err)
		}
	})

	t.Run("reports aggregated detail from stderr", func(t *testing.T) {
		t.Parallel()

		err := (&filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if _, writeErr := io.WriteString(ctx.IO.Stderr, "permission denied"); writeErr != nil {
					t.Fatalf("WriteString(): %v", writeErr)
				}
				return &runtimepkg.Result{ExitCode: 1}
			},
		}).CheckFilepath(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}},
		)
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("transient exit code is surfaced", func(t *testing.T) {
		t.Parallel()

		err := (&filepathStubRuntime{
			execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
				return &runtimepkg.Result{ExitCode: 125}
			},
		}).CheckFilepath(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}},
		)
		if !errors.Is(err, ErrContainerEngineFailure) {
			t.Fatalf("err = %v, want wrapping ErrContainerEngineFailure", err)
		}
	})

	t.Run("runtime error is propagated", func(t *testing.T) {
		t.Parallel()

		err := (&filepathStubRuntime{
			execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
				return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
			},
		}).CheckFilepath(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}},
		)
		if !errors.Is(err, ErrContainerValidationFailed) {
			t.Fatalf("err = %v, want wrapping ErrContainerValidationFailed", err)
		}
	})
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

	err := (&filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			_, _ = io.WriteString(ctx.IO.Stderr, "missing permissions")
			return &runtimepkg.Result{ExitCode: 1}
		},
	}).CheckFilepath(invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}})
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

	t.Run("existing path succeeds", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{invowkfile.FilesystemPath(existingFile)},
		}
		probe := &recordingHostProbe{}
		if err := ValidateFilepathAlternativesWithProbe(fp, invowkDir, probe); err != nil {
			t.Fatalf("ValidateFilepathAlternativesWithProbe() = %v", err)
		}
	})

	t.Run("missing path returns error", func(t *testing.T) {
		t.Parallel()
		missingPath := types.FilesystemPath("/nonexistent/path")
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{missingPath},
		}
		err := ValidateFilepathAlternativesWithProbe(fp, invowkDir, &recordingHostProbe{
			filepathErrors: map[types.FilesystemPath]error{
				missingPath: fmt.Errorf("%s: %w", missingPath, ErrPathNotExists),
			},
		})
		if err == nil {
			t.Fatal("expected error for missing path")
		}
	})

	t.Run("first missing second exists succeeds", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{"/nonexistent", invowkfile.FilesystemPath(existingFile)},
		}
		probe := &recordingHostProbe{
			filepathErrors: map[types.FilesystemPath]error{
				"/nonexistent": fmt.Errorf("/nonexistent: %w", ErrPathNotExists),
			},
		}
		if err := ValidateFilepathAlternativesWithProbe(fp, invowkDir, probe); err != nil {
			t.Fatalf("ValidateFilepathAlternativesWithProbe() = %v", err)
		}
	})

	t.Run("empty alternatives returns error", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{}
		err := ValidateFilepathAlternatives(fp, invowkDir)
		if !errors.Is(err, ErrNoPathAlternatives) {
			t.Fatalf("err = %v, want wrapping ErrNoPathAlternatives", err)
		}
	})
}

func TestEvaluateAlternatives(t *testing.T) {
	t.Parallel()

	errFail := errors.New("failed")

	t.Run("empty alternatives", func(t *testing.T) {
		t.Parallel()
		found, lastErr := EvaluateAlternatives([]string{}, func(_ string) error { return nil })
		if found || lastErr != nil {
			t.Fatalf("got (%v, %v), want (false, nil)", found, lastErr)
		}
	})

	t.Run("single passing alternative", func(t *testing.T) {
		t.Parallel()
		found, lastErr := EvaluateAlternatives([]string{"a"}, func(_ string) error { return nil })
		if !found || lastErr != nil {
			t.Fatalf("got (%v, %v), want (true, nil)", found, lastErr)
		}
	})

	t.Run("single failing alternative", func(t *testing.T) {
		t.Parallel()
		found, lastErr := EvaluateAlternatives([]string{"a"}, func(_ string) error { return errFail })
		if found || lastErr == nil {
			t.Fatalf("got (%v, %v), want (false, error)", found, lastErr)
		}
	})

	t.Run("first passes short circuits", func(t *testing.T) {
		t.Parallel()
		callCount := 0
		found, _ := EvaluateAlternatives([]string{"a", "b"}, func(_ string) error {
			callCount++
			return nil
		})
		if !found || callCount != 1 {
			t.Fatalf("got found=%v, callCount=%d, want found=true, callCount=1", found, callCount)
		}
	})

	t.Run("all fail returns last error", func(t *testing.T) {
		t.Parallel()
		found, lastErr := EvaluateAlternatives([]string{"a", "b"}, func(s string) error {
			return fmt.Errorf("failed: %s", s)
		})
		if found {
			t.Fatal("got found=true, want false")
		}
		if lastErr == nil || !strings.Contains(lastErr.Error(), "failed: b") {
			t.Fatalf("lastErr = %v, want error containing 'failed: b'", lastErr)
		}
	})
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
