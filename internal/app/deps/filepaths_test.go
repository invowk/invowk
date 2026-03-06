// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
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
		if err == nil || !strings.Contains(err.Error(), "container runtime not available for filepath validation") {
			t.Fatalf("err = %v", err)
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
		if err == nil || !strings.Contains(err.Error(), "at least one path must be provided in alternatives") {
			t.Fatalf("err = %v", err)
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
		if err == nil || !strings.Contains(err.Error(), "container engine failure") {
			t.Fatalf("err = %v", err)
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
		if err == nil || !strings.Contains(err.Error(), "container validation failed") {
			t.Fatalf("err = %v", err)
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
	if singleErr.Error() != "  • missing permissions" {
		t.Fatalf("singleErr = %q", singleErr.Error())
	}

	multiErr := formatContainerFilepathError(
		[]invowkfile.FilesystemPath{"/tmp", "/var/tmp"},
		[]string{"/tmp: missing", "/var/tmp: denied"},
	)
	if !strings.Contains(multiErr.Error(), "none of the alternatives satisfied the requirements in container") {
		t.Fatalf("multiErr = %q", multiErr.Error())
	}
}

func TestWindowsFilepathHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "tool.exe")
	txtPath := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(exePath, []byte("binary"), 0o600); err != nil {
		t.Fatalf("WriteFile(exe): %v", err)
	}
	if err := os.WriteFile(txtPath, []byte("text"), 0o600); err != nil {
		t.Fatalf("WriteFile(txt): %v", err)
	}

	t.Setenv("PATHEXT", ".EXE;.BAT")

	if !windowsPathHasExecutableExtension(exePath) {
		t.Fatal("windowsPathHasExecutableExtension(exePath) = false, want true")
	}
	if windowsPathHasExecutableExtension(txtPath) {
		t.Fatal("windowsPathHasExecutableExtension(txtPath) = true, want false")
	}
	if !canOpenPath(tmpDir) {
		t.Fatal("canOpenPath(tmpDir) = false, want true")
	}
	if canOpenPath(filepath.Join(tmpDir, "missing")) {
		t.Fatal("canOpenPath(missing) = true, want false")
	}
	if !canOpenReadOnly(exePath) {
		t.Fatal("canOpenReadOnly(exePath) = false, want true")
	}
	if canOpenReadOnly(filepath.Join(tmpDir, "missing.exe")) {
		t.Fatal("canOpenReadOnly(missing) = true, want false")
	}

	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Stat(tmpDir): %v", err)
	}
	if !isExecutableOnWindows(tmpDir, dirInfo) {
		t.Fatal("isExecutableOnWindows(tmpDir) = false, want true")
	}

	fileInfo, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("Stat(exePath): %v", err)
	}
	if !isExecutableOnWindows(exePath, fileInfo) {
		t.Fatal("isExecutableOnWindows(exePath) = false, want true")
	}
	if isExecutableOnWindows(txtPath, fileInfo) {
		t.Fatal("isExecutableOnWindows(txtPath) = true, want false")
	}
}
