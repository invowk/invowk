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
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

func TestDependencyCapabilityCheckerCheck(t *testing.T) {
	t.Parallel()

	checker := NewDependencyCapabilityChecker()
	tests := []struct {
		name       string
		capability invowkfile.CapabilityName
		skipShort  bool
		unknown    bool
	}{
		{name: "local area network", capability: invowkfile.CapabilityLocalAreaNetwork, skipShort: true},
		{name: "internet", capability: invowkfile.CapabilityInternet, skipShort: true},
		{name: "containers", capability: invowkfile.CapabilityContainers, skipShort: true},
		{name: "tty", capability: invowkfile.CapabilityTTY},
		{name: "unknown", capability: invowkfile.CapabilityName("unknown-capability"), unknown: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.skipShort && testing.Short() {
				t.Skip("skipping integration test in short mode")
			}
			err := checker.Check(t.Context(), deps.IOContext{}, tt.capability)
			if !tt.unknown {
				assertCapabilityErrorType(t, err)
				return
			}
			var capErr *invowkfile.CapabilityError
			if !errors.As(err, &capErr) {
				t.Fatalf("errors.As(*CapabilityError) = false for %T", err)
			}
			if capErr.Capability != tt.capability || capErr.Message != "unknown capability" {
				t.Errorf("CapabilityError = %#v, want capability %q and unknown message", capErr, tt.capability)
			}
		})
	}
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

	tests := []struct {
		name       string
		pathKind   string
		dependency invowkfile.FilepathDependency
		wantIs     error
	}{
		{name: "existing file with no requirements succeeds", pathKind: "readable"},
		{name: "nonexistent file returns error", pathKind: "missing", wantIs: deps.ErrPathNotExists},
		{name: "readable check passes for readable file", pathKind: "readable", dependency: invowkfile.FilepathDependency{Readable: true}},
		{name: "writable check passes for writable dir", pathKind: "writable-dir", dependency: invowkfile.FilepathDependency{Writable: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := readableFile
			switch tt.pathKind {
			case "readable":
			case "missing":
				path = filepath.Join(tmpDir, "missing.txt")
			case "writable-dir":
				path = t.TempDir()
			default:
				t.Fatalf("unknown path kind %q", tt.pathKind)
			}
			err := probe.CheckFilepath(types.FilesystemPath(path), types.FilesystemPath(path), tt.dependency)
			if tt.wantIs != nil {
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("error = %v, want wrapping %v", err, tt.wantIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckFilepath() = %v", err)
			}
		})
	}
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

	tests := []struct {
		name       string
		access     string
		targetKind string
		want       bool
	}{
		{name: "readable file", access: "readable", targetKind: "file", want: true},
		{name: "readable dir", access: "readable", targetKind: "dir", want: true},
		{name: "writable dir", access: "writable", targetKind: "dir", want: true},
		{name: "writable file", access: "writable", targetKind: "file", want: true},
		{name: "executable on non-executable file", access: "executable", targetKind: "file"},
		{name: "executable on executable file", access: "executable", targetKind: "exec", want: true},
		{name: "executable on dir", access: "executable", targetKind: "dir", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path, info := testFile, fileInfo
			switch tt.targetKind {
			case "file":
			case "dir":
				path, info = tmpDir, dirInfo
			case "exec":
				execName := "exec.sh"
				if goruntime.GOOS == platform.Windows {
					execName = "exec.bat"
				}
				path = filepath.Join(t.TempDir(), execName)
				if err := os.WriteFile(path, []byte("#!/bin/sh"), 0o755); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				var err error
				info, err = os.Stat(path)
				if err != nil {
					t.Fatalf("Stat: %v", err)
				}
			default:
				t.Fatalf("unknown target kind %q", tt.targetKind)
			}
			var got bool
			switch tt.access {
			case "readable":
				got = isReadable(path, info)
			case "writable":
				got = isWritable(path, info)
			case "executable":
				got = isExecutable(path, info)
			default:
				t.Fatalf("unknown access %q", tt.access)
			}
			if got != tt.want {
				t.Fatalf("%s(%q) = %v, want %v", tt.access, path, got, tt.want)
			}
		})
	}
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
	if _, ok := errors.AsType[*invowkfile.CapabilityError](err); !ok {
		t.Errorf("errors.As(*CapabilityError) = false for %T", err)
	}
}

func TestDependencyLockProvider_LoadCommandScopeLock(t *testing.T) {
	t.Parallel()

	provider := dependencyLockProvider{}
	if lock, err := provider.LoadCommandScopeLock(nil); err != nil || lock == nil {
		t.Fatalf("LoadCommandScopeLock(nil) = (%v, %v), want empty lock", lock, err)
	}

	missingDir := t.TempDir()
	if lock, err := provider.LoadCommandScopeLock(&invowkfile.Invowkfile{ModulePath: types.FilesystemPath(missingDir)}); err != nil || lock == nil {
		t.Fatalf("LoadCommandScopeLock(missing lock) = (%v, %v), want empty lock", lock, err)
	}

	v1Dir := t.TempDir()
	v1Lock := invowkmod.NewLockFile()
	v1Lock.Version = invowkmod.LockFileVersionV1
	if err := v1Lock.Save(filepath.Join(v1Dir, invowkmod.LockFileName)); err != nil {
		t.Fatalf("Save(v1) error = %v", err)
	}
	_, err := provider.LoadCommandScopeLock(&invowkfile.Invowkfile{ModulePath: types.FilesystemPath(v1Dir)})
	if !errors.Is(err, invowkmod.ErrLockFileV1UpgradeRequired) {
		t.Fatalf("LoadCommandScopeLock(v1) error = %v, want ErrLockFileV1UpgradeRequired", err)
	}
	if !strings.Contains(err.Error(), "invowk module sync") {
		t.Fatalf("LoadCommandScopeLock(v1) error = %q, want invowk module sync guidance", err.Error())
	}

	v2Dir := t.TempDir()
	v2Lock := invowkmod.NewLockFile()
	if saveErr := v2Lock.Save(filepath.Join(v2Dir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("Save(v2) error = %v", saveErr)
	}
	got, err := provider.LoadCommandScopeLock(&invowkfile.Invowkfile{ModulePath: types.FilesystemPath(v2Dir)})
	if err != nil || got == nil {
		t.Fatalf("LoadCommandScopeLock(v2) = (%v, %v), want v2 lock", got, err)
	}
}

func TestHostCustomCheckEnvMapStripsProgramResumeToken(t *testing.T) {
	t.Setenv("OSC7577_TOKEN", "secret-host-token")
	t.Setenv("INVOWK_TEST_CUSTOM_CHECK_KEEP", "kept-value")

	env := hostCustomCheckEnvMap()
	if _, exists := env["OSC7577_TOKEN"]; exists {
		t.Fatalf("hostCustomCheckEnvMap() leaked OSC7577_TOKEN: %v", env["OSC7577_TOKEN"])
	}
	if env["INVOWK_TEST_CUSTOM_CHECK_KEEP"] != "kept-value" {
		t.Fatalf("hostCustomCheckEnvMap()[INVOWK_TEST_CUSTOM_CHECK_KEEP] = %q, want kept-value", env["INVOWK_TEST_CUSTOM_CHECK_KEEP"])
	}
}
