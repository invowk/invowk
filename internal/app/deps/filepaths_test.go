// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
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

func TestCheckFilepathDependenciesInContainer(t *testing.T) {
	t.Parallel()

	execCtx := &runtimepkg.ExecutionContext{
		Command: &invowkfile.Command{Name: "build"},
		Context: t.Context(),
	}

	t.Run("missing container runtime", func(t *testing.T) {
		t.Parallel()

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}
		err := CheckFilepathDependenciesInContainer(deps, runtimepkg.NewRegistry(), execCtx)
		if err == nil || !errors.Is(err, ErrContainerRuntimeNotAvailable) {
			t.Fatalf("err = %v, want wrapping ErrContainerRuntimeNotAvailable", err)
		}
	})

	t.Run("dependency error aggregates failing paths", func(t *testing.T) {
		t.Parallel()

		registry := runtimepkg.NewRegistry()
		registry.Register(runtimepkg.RuntimeTypeContainer, &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				if _, err := io.WriteString(ctx.IO.Stderr, "permission denied"); err != nil {
					t.Fatalf("WriteString(): %v", err)
				}
				return &runtimepkg.Result{ExitCode: 1}
			},
		})

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}

		err := CheckFilepathDependenciesInContainer(deps, registry, execCtx)
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

		registry := runtimepkg.NewRegistry()
		registry.Register(runtimepkg.RuntimeTypeContainer, &filepathStubRuntime{})

		deps := &invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []invowkfile.FilesystemPath{"/tmp"}}},
		}

		if err := CheckFilepathDependenciesInContainer(deps, registry, execCtx); err != nil {
			t.Fatalf("CheckFilepathDependenciesInContainer() = %v", err)
		}
	})
}

func TestValidateFilepathInContainer(t *testing.T) {
	t.Parallel()

	execCtx := &runtimepkg.ExecutionContext{
		Command: &invowkfile.Command{Name: "build"},
		Context: t.Context(),
	}

	t.Run("requires at least one alternative", func(t *testing.T) {
		t.Parallel()

		err := ValidateFilepathInContainer(invowkfile.FilepathDependency{}, &filepathStubRuntime{}, execCtx)
		if !errors.Is(err, ErrNoPathAlternatives) {
			t.Fatalf("err = %v, want wrapping ErrNoPathAlternatives", err)
		}
	})

	t.Run("succeeds on first passing alternative", func(t *testing.T) {
		t.Parallel()

		err := ValidateFilepathInContainer(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}},
			&filepathStubRuntime{},
			execCtx,
		)
		if err != nil {
			t.Fatalf("ValidateFilepathInContainer() = %v", err)
		}
	})

	t.Run("reports aggregated detail from stderr", func(t *testing.T) {
		t.Parallel()

		err := ValidateFilepathInContainer(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp", "/var/tmp"}},
			&filepathStubRuntime{
				execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
					if _, writeErr := io.WriteString(ctx.IO.Stderr, "permission denied"); writeErr != nil {
						t.Fatalf("WriteString(): %v", writeErr)
					}
					return &runtimepkg.Result{ExitCode: 1}
				},
			},
			execCtx,
		)
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("transient exit code is surfaced", func(t *testing.T) {
		t.Parallel()

		err := ValidateFilepathInContainer(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}},
			&filepathStubRuntime{
				execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
					return &runtimepkg.Result{ExitCode: 125}
				},
			},
			execCtx,
		)
		if !errors.Is(err, ErrContainerEngineFailure) {
			t.Fatalf("err = %v, want wrapping ErrContainerEngineFailure", err)
		}
	})

	t.Run("runtime error is propagated", func(t *testing.T) {
		t.Parallel()

		err := ValidateFilepathInContainer(
			invowkfile.FilepathDependency{Alternatives: []invowkfile.FilesystemPath{"/tmp"}},
			&filepathStubRuntime{
				execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
					return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
				},
			},
			execCtx,
		)
		if !errors.Is(err, ErrContainerValidationFailed) {
			t.Fatalf("err = %v, want wrapping ErrContainerValidationFailed", err)
		}
	})
}

func TestContainerFilepathHelpers(t *testing.T) {
	t.Parallel()

	script := buildContainerFilepathCheckScript(
		invowkfile.FilepathDependency{Readable: true, Writable: true, Executable: true},
		"/tmp/that's-it",
	)
	if !strings.Contains(script, "test -e '/tmp/that'\\''s-it'") {
		t.Fatalf("script = %q", script)
	}
	if !strings.Contains(script, "test -r") || !strings.Contains(script, "test -w") || !strings.Contains(script, "test -x") {
		t.Fatalf("script = %q", script)
	}

	singleErr := formatContainerFilepathError([]invowkfile.FilesystemPath{"/tmp"}, []string{"missing permissions"})
	if !strings.Contains(singleErr.Error(), "missing permissions") {
		t.Fatalf("singleErr = %q, want containing 'missing permissions'", singleErr.Error())
	}

	multiErr := formatContainerFilepathError(
		[]invowkfile.FilesystemPath{"/tmp", "/var/tmp"},
		[]string{"/tmp: missing", "/var/tmp: denied"},
	)
	if !strings.Contains(multiErr.Error(), "none of the alternatives satisfied the requirements in container") {
		t.Fatalf("multiErr = %q", multiErr.Error())
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
