// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

func TestDependencyCapabilityCheckerCheck(t *testing.T) {
	t.Parallel()

	checker := NewDependencyCapabilityChecker()

	t.Run("local area network", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(t.Context(), deps.IOContext{}, invowkfile.CapabilityLocalAreaNetwork))
	})

	t.Run("internet", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(t.Context(), deps.IOContext{}, invowkfile.CapabilityInternet))
	})

	t.Run("containers", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(t.Context(), deps.IOContext{}, invowkfile.CapabilityContainers))
	})

	t.Run("tty", func(t *testing.T) {
		t.Parallel()
		assertCapabilityErrorType(t, checker.Check(t.Context(), deps.IOContext{}, invowkfile.CapabilityTTY))
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		err := checker.Check(t.Context(), deps.IOContext{}, invowkfile.CapabilityName("unknown-capability"))
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Fatalf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr.Capability != "unknown-capability" {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, "unknown-capability")
		}
		if capErr.Message != "unknown capability" {
			t.Errorf("CapabilityError.Message = %q, want %q", capErr.Message, "unknown capability")
		}
	})
}

func TestCheckLocalAreaNetworkReturnsCapabilityError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := checkLocalAreaNetwork()
	if err != nil {
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Errorf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr != nil && capErr.Capability != invowkfile.CapabilityLocalAreaNetwork {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, invowkfile.CapabilityLocalAreaNetwork)
		}
	}
}

func TestCheckInternetReturnsCapabilityError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := checkInternet(t.Context())
	if err != nil {
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Errorf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr != nil && capErr.Capability != invowkfile.CapabilityInternet {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, invowkfile.CapabilityInternet)
		}
	}
}

func TestDependencyHostProbeCheckFilepath(t *testing.T) {
	t.Parallel()

	probe := dependencyHostProbe{}
	tmpDir := t.TempDir()
	readableFile := filepath.Join(tmpDir, "readable.txt")
	if err := os.WriteFile(readableFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("existing file with no requirements succeeds", func(t *testing.T) {
		t.Parallel()
		err := probe.CheckFilepath(types.FilesystemPath(readableFile), types.FilesystemPath(readableFile), invowkfile.FilepathDependency{})
		if err != nil {
			t.Fatalf("CheckFilepath() = %v", err)
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		t.Parallel()
		missing := filepath.Join(tmpDir, "missing.txt")
		err := probe.CheckFilepath(types.FilesystemPath(missing), types.FilesystemPath(missing), invowkfile.FilepathDependency{})
		if err == nil {
			t.Fatal("CheckFilepath() = nil, want error")
		}
		if !errors.Is(err, deps.ErrPathNotExists) {
			t.Fatalf("errors.Is(err, ErrPathNotExists) = false for %v", err)
		}
	})

	t.Run("readable check passes for readable file", func(t *testing.T) {
		t.Parallel()
		err := probe.CheckFilepath(
			types.FilesystemPath(readableFile),
			types.FilesystemPath(readableFile),
			invowkfile.FilepathDependency{Readable: true},
		)
		if err != nil {
			t.Fatalf("CheckFilepath() = %v", err)
		}
	})

	t.Run("writable check passes for writable dir", func(t *testing.T) {
		t.Parallel()
		writableDir := t.TempDir()
		err := probe.CheckFilepath(
			types.FilesystemPath(writableDir),
			types.FilesystemPath(writableDir),
			invowkfile.FilepathDependency{Writable: true},
		)
		if err != nil {
			t.Fatalf("CheckFilepath() = %v", err)
		}
	})
}

func TestDependencyHostProbeRunCustomCheckContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	check := invowkfile.CustomCheck{
		Name:   "slow-check",
		Script: invowkfile.CustomCheckScript{Content: "sleep 60"},
	}

	_, err := dependencyHostProbe{}.RunCustomCheck(ctx, check)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if strings.Contains(err.Error(), "returned exit code 0") {
		t.Error("expected context cancellation error, but check passed with exit code 0")
	}
}

func TestDependencyHostProbeRunCustomCheckWithShellCompatibleInterpreter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script invowkfile.CustomCheckScript
	}{
		{
			name: "explicit shell interpreter",
			script: invowkfile.CustomCheckScript{
				Content:     "echo ok",
				Interpreter: "bash",
			},
		},
		{
			name: "shell shebang",
			script: invowkfile.CustomCheckScript{
				Content: "#!/bin/sh\necho ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := invowkfile.CustomCheck{
				Name:           "shell-check",
				Script:         tt.script,
				ExpectedOutput: "^ok$",
			}

			result, err := dependencyHostProbe{}.RunCustomCheck(t.Context(), check)
			if err != nil {
				t.Fatalf("RunCustomCheck() = %v", err)
			}
			if got := result.Output().String(); got != "ok" {
				t.Fatalf("custom check output = %q, want ok", got)
			}
		})
	}
}

func TestDependencyHostProbeRunCustomCheckWithInterpreter(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}

	tests := []struct {
		name      string
		checkName invowkfile.CheckName
		script    invowkfile.CustomCheckScript
	}{
		{
			name:      "explicit script interpreter",
			checkName: "explicit-python",
			script: invowkfile.CustomCheckScript{
				Content:     "print('ok')",
				Interpreter: "python3",
			},
		},
		{
			name:      "shebang interpreter",
			checkName: "shebang-python",
			script: invowkfile.CustomCheckScript{
				Content: "#!/usr/bin/env python3\nprint('ok')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := invowkfile.CustomCheck{
				Name:           tt.checkName,
				Script:         tt.script,
				ExpectedOutput: "^ok$",
			}

			result, err := dependencyHostProbe{}.RunCustomCheck(t.Context(), check)
			if err != nil {
				t.Fatalf("RunCustomCheck() = %v", err)
			}
			if got := result.Output().String(); got != "ok" {
				t.Fatalf("custom check output = %q, want ok", got)
			}
		})
	}
}

func TestDependencyHostProbeRunCustomCheckReportsMissingInterpreterName(t *testing.T) {
	t.Parallel()

	check := invowkfile.CustomCheck{
		Name: "missing-python",
		Script: invowkfile.CustomCheckScript{
			Content:     "print('no')",
			Interpreter: "/definitely/missing/python3",
		},
	}

	_, err := dependencyHostProbe{}.RunCustomCheck(t.Context(), check)
	if err == nil {
		t.Fatal("RunCustomCheck() error = nil, want missing interpreter error")
	}
	if !strings.Contains(err.Error(), "missing-python") {
		t.Fatalf("RunCustomCheck() error = %v, want check name", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("RunCustomCheck() error = %v, want missing interpreter detail", err)
	}
}

func TestHostPathAccessHelpers(t *testing.T) {
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

	t.Run("readable file", func(t *testing.T) {
		t.Parallel()
		if !isReadable(testFile, fileInfo) {
			t.Fatal("isReadable() = false, want true")
		}
	})

	t.Run("readable dir", func(t *testing.T) {
		t.Parallel()
		if !isReadable(tmpDir, dirInfo) {
			t.Fatal("isReadable(dir) = false, want true")
		}
	})

	t.Run("writable dir", func(t *testing.T) {
		t.Parallel()
		if !isWritable(tmpDir, dirInfo) {
			t.Fatal("isWritable(dir) = false, want true")
		}
	})

	t.Run("writable file", func(t *testing.T) {
		t.Parallel()
		if !isWritable(testFile, fileInfo) {
			t.Fatal("isWritable(file) = false, want true")
		}
	})

	t.Run("executable on non-executable file", func(t *testing.T) {
		t.Parallel()
		if isExecutable(testFile, fileInfo) {
			t.Fatal("isExecutable() = true for non-executable file, want false")
		}
	})

	t.Run("executable on executable file", func(t *testing.T) {
		t.Parallel()
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
		if !isExecutable(execFile, execInfo) {
			t.Fatal("isExecutable() = false for executable file, want true")
		}
	})

	t.Run("executable on dir", func(t *testing.T) {
		t.Parallel()
		if !isExecutable(tmpDir, dirInfo) {
			t.Fatal("isExecutable(dir) = false, want true")
		}
	})
}

func TestIsExecutablePATHEXTFallback(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("skipping: PATHEXT is only consulted on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATHEXT", ".EXE;.PY;.RB")

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !isExecutable(testFile, info) {
		t.Error("isExecutable() should return true for .py file when PATHEXT includes .PY")
	}
}

func TestIsExecutablePATHEXTEmptyEntries(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("skipping: PATHEXT is only consulted on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "noext")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATHEXT", ".EXE;;.BAT;")

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if isExecutable(testFile, info) {
		t.Error("isExecutable() should return false for extensionless file even with empty PATHEXT entries")
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

func assertCapabilityErrorType(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}
	var capErr *invowkfile.CapabilityError
	if !errors.As(err, &capErr) {
		t.Errorf("errors.As(*CapabilityError) = false for %T", err)
	}
}
