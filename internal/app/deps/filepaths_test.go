// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/platform"
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
		if err := ValidateFilepathAlternatives(fp, invowkDir); err != nil {
			t.Fatalf("ValidateFilepathAlternatives() = %v", err)
		}
	})

	t.Run("missing path returns error", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{"/nonexistent/path"},
		}
		err := ValidateFilepathAlternatives(fp, invowkDir)
		if err == nil {
			t.Fatal("expected error for missing path")
		}
	})

	t.Run("first missing second exists succeeds", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{
			Alternatives: []invowkfile.FilesystemPath{"/nonexistent", invowkfile.FilesystemPath(existingFile)},
		}
		if err := ValidateFilepathAlternatives(fp, invowkDir); err != nil {
			t.Fatalf("ValidateFilepathAlternatives() = %v", err)
		}
	})

	t.Run("empty alternatives returns error", func(t *testing.T) {
		t.Parallel()
		fp := invowkfile.FilepathDependency{}
		err := ValidateFilepathAlternatives(fp, invowkDir)
		if err == nil || !strings.Contains(err.Error(), "at least one path") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestValidateSingleFilepath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	readableFile := filepath.Join(tmpDir, "readable.txt")
	if err := os.WriteFile(readableFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("existing file with no requirements succeeds", func(t *testing.T) {
		t.Parallel()
		err := ValidateSingleFilepath(types.FilesystemPath(readableFile), types.FilesystemPath(readableFile), invowkfile.FilepathDependency{})
		if err != nil {
			t.Fatalf("ValidateSingleFilepath() = %v", err)
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		t.Parallel()
		missing := filepath.Join(tmpDir, "missing.txt")
		err := ValidateSingleFilepath(types.FilesystemPath(missing), types.FilesystemPath(missing), invowkfile.FilepathDependency{})
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("readable check passes for readable file", func(t *testing.T) {
		t.Parallel()
		err := ValidateSingleFilepath(
			types.FilesystemPath(readableFile),
			types.FilesystemPath(readableFile),
			invowkfile.FilepathDependency{Readable: true},
		)
		if err != nil {
			t.Fatalf("ValidateSingleFilepath() = %v", err)
		}
	})

	t.Run("writable check passes for writable dir", func(t *testing.T) {
		t.Parallel()
		writableDir := t.TempDir()
		err := ValidateSingleFilepath(
			types.FilesystemPath(writableDir),
			types.FilesystemPath(writableDir),
			invowkfile.FilepathDependency{Writable: true},
		)
		if err != nil {
			t.Fatalf("ValidateSingleFilepath() = %v", err)
		}
	})
}

func TestIsReadableWritableExecutable(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}

	t.Run("IsReadable file", func(t *testing.T) {
		t.Parallel()
		if !IsReadable(testFile, fileInfo) {
			t.Fatal("IsReadable() = false, want true")
		}
	})

	t.Run("IsReadable dir", func(t *testing.T) {
		t.Parallel()
		if !IsReadable(tmpDir, dirInfo) {
			t.Fatal("IsReadable(dir) = false, want true")
		}
	})

	t.Run("IsWritable dir", func(t *testing.T) {
		t.Parallel()
		if !IsWritable(tmpDir, dirInfo) {
			t.Fatal("IsWritable(dir) = false, want true")
		}
	})

	t.Run("IsWritable file", func(t *testing.T) {
		t.Parallel()
		if !IsWritable(testFile, fileInfo) {
			t.Fatal("IsWritable(file) = false, want true")
		}
	})

	t.Run("IsExecutable on non-executable file", func(t *testing.T) {
		t.Parallel()
		if IsExecutable(testFile, fileInfo) {
			t.Fatal("IsExecutable() = true for non-executable file, want false")
		}
	})

	t.Run("IsExecutable on executable file", func(t *testing.T) {
		t.Parallel()
		// On Windows, executability is determined by file extension (PATHEXT),
		// not Unix permission bits. Use .bat so the test passes cross-platform.
		execName := "exec.sh"
		if goruntime.GOOS == platform.Windows {
			execName = "exec.bat"
		}
		execFile := filepath.Join(t.TempDir(), execName)
		if writeErr := os.WriteFile(execFile, []byte("#!/bin/sh"), 0o755); writeErr != nil {
			t.Fatalf("WriteFile: %v", writeErr)
		}
		execInfo, statErr := os.Stat(execFile)
		if statErr != nil {
			t.Fatalf("Stat: %v", statErr)
		}
		if !IsExecutable(execFile, execInfo) {
			t.Fatal("IsExecutable() = false for executable file, want true")
		}
	})

	t.Run("IsExecutable on dir", func(t *testing.T) {
		t.Parallel()
		if !IsExecutable(tmpDir, dirInfo) {
			t.Fatal("IsExecutable(dir) = false, want true")
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
