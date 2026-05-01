// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout DurationString
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "empty string returns zero",
			timeout: "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "30 seconds",
			timeout: "30s",
			want:    30 * time.Second,
			wantErr: false,
		},
		{
			name:    "5 minutes",
			timeout: "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "1 hour 30 minutes",
			timeout: "1h30m",
			want:    90 * time.Minute,
			wantErr: false,
		},
		{
			name:    "500 milliseconds",
			timeout: "500ms",
			want:    500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "invalid string returns error",
			timeout: "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "number without unit returns error",
			timeout: "30",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero duration returns error",
			timeout: "0s",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative duration returns error",
			timeout: "-5m",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &Implementation{Timeout: tt.timeout}
			got, err := impl.ParseTimeout()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformRuntimeKey_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     PlatformRuntimeKey
		want    bool
		wantErr bool
	}{
		{"both valid linux/native", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: RuntimeNative}, true, false},
		{"both valid macos/container", PlatformRuntimeKey{Platform: PlatformMac, Runtime: RuntimeContainer}, true, false},
		{"invalid platform", PlatformRuntimeKey{Platform: "bogus", Runtime: RuntimeNative}, false, true},
		{"invalid runtime", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: "bogus"}, false, true},
		{"both invalid", PlatformRuntimeKey{Platform: "bogus", Runtime: "bogus"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.key.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PlatformRuntimeKey{%q, %q}.Validate() error = %v, want valid=%v",
					tt.key.Platform, tt.key.Runtime, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PlatformRuntimeKey{%q, %q}.Validate() returned nil, want error",
						tt.key.Platform, tt.key.Runtime)
				}
			} else if err != nil {
				t.Errorf("PlatformRuntimeKey{%q, %q}.Validate() returned unexpected error: %v",
					tt.key.Platform, tt.key.Runtime, err)
			}
		})
	}
}

func TestPlatformRuntimeKey_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  PlatformRuntimeKey
		want string
	}{
		{"linux_native", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: RuntimeNative}, "linux/native"},
		{"macos_virtual", PlatformRuntimeKey{Platform: PlatformMac, Runtime: RuntimeVirtual}, "macos/virtual"},
		{"windows_container", PlatformRuntimeKey{Platform: PlatformWindows, Runtime: RuntimeContainer}, "windows/container"},
		{"empty_both", PlatformRuntimeKey{}, "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.key.String()
			if got != tt.want {
				t.Errorf("PlatformRuntimeKey.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestResolveScriptWithModule_PathTraversal verifies that ResolveScriptWithModule
// returns ErrScriptPathTraversal when a module script path escapes the module boundary (SC-01).
func TestResolveScriptWithModule_PathTraversal(t *testing.T) {
	t.Parallel()

	// Create a real module directory with a script file inside it.
	moduleDir := t.TempDir()
	scriptsDir := filepath.Join(moduleDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	scriptFile := filepath.Join(scriptsDir, "build.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/sh\necho ok"), 0o644); err != nil {
		t.Fatalf("failed to write script file: %v", err)
	}

	invowkfilePath := FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue"))
	modulePath := FilesystemPath(moduleDir)

	tests := []struct {
		name          string
		script        ScriptContent
		wantTraversal bool
	}{
		{
			name:          "valid relative path within module",
			script:        "./scripts/build.sh",
			wantTraversal: false,
		},
		{
			name:          "path escaping module with parent traversal",
			script:        "../../etc/passwd",
			wantTraversal: true,
		},
		{
			name:          "path escaping with multiple levels",
			script:        "../../../tmp/evil.sh",
			wantTraversal: true,
		},
		{
			name:          "inline script not affected",
			script:        "echo hello world",
			wantTraversal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &Implementation{
				Script:   tt.script,
				Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
			}

			_, err := impl.ResolveScriptWithModule(invowkfilePath, modulePath)

			if tt.wantTraversal {
				if err == nil {
					t.Fatal("expected ErrScriptPathTraversal, got nil")
				}
				if !errors.Is(err, ErrScriptPathTraversal) {
					t.Errorf("expected error wrapping ErrScriptPathTraversal, got: %v", err)
				}
			} else if err != nil && errors.Is(err, ErrScriptPathTraversal) {
				t.Errorf("unexpected ErrScriptPathTraversal for valid path: %v", err)
			}
		})
	}
}

// TestResolveScriptWithFSAndModule_PathTraversal verifies that
// ResolveScriptWithFSAndModule returns ErrScriptPathTraversal for traversal
// scripts in module context (SC-01). Uses a virtual filesystem to avoid
// needing real files for the traversal cases.
func TestResolveScriptWithFSAndModule_PathTraversal(t *testing.T) {
	t.Parallel()

	// Use platform-native paths for the virtual filesystem.
	var moduleDir string
	if runtime.GOOS == "windows" {
		moduleDir = `C:\modules\mymod.invowkmod`
	} else {
		moduleDir = "/modules/mymod.invowkmod"
	}

	virtualFS := map[string]string{
		filepath.Join(moduleDir, "scripts", "build.sh"): "#!/bin/sh\necho build",
	}

	readFile := func(path string) ([]byte, error) {
		if content, ok := virtualFS[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}

	invowkfilePath := FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue"))
	modulePath := FilesystemPath(moduleDir)

	tests := []struct {
		name          string
		script        ScriptContent
		wantTraversal bool
		wantErr       bool
	}{
		{
			name:          "valid path within module",
			script:        "./scripts/build.sh",
			wantTraversal: false,
			wantErr:       false,
		},
		{
			name:          "path escaping module boundary",
			script:        "../../etc/passwd",
			wantTraversal: true,
			wantErr:       true,
		},
		{
			name:          "deep traversal attack",
			script:        "../../../../../../../tmp/evil.sh",
			wantTraversal: true,
			wantErr:       true,
		},
		{
			name:          "inline script bypasses containment check",
			script:        "echo no file lookup",
			wantTraversal: false,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &Implementation{
				Script:   tt.script,
				Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
			}

			_, err := impl.ResolveScriptWithFSAndModule(invowkfilePath, modulePath, readFile)

			if tt.wantTraversal {
				if err == nil {
					t.Fatal("expected ErrScriptPathTraversal, got nil")
				}
				if !errors.Is(err, ErrScriptPathTraversal) {
					t.Errorf("expected error wrapping ErrScriptPathTraversal, got: %v", err)
				}
				return
			}

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestResolveScriptWithFSAndModule_ValidatesResolvedContent(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(string(filepath.Separator), "modules", "mymod.invowkmod")
	if runtime.GOOS == "windows" {
		moduleDir = `C:\modules\mymod.invowkmod`
	}
	invowkfilePath := FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue"))
	modulePath := FilesystemPath(moduleDir)
	scriptPath := filepath.Join(moduleDir, "scripts", "empty.sh")
	readFile := func(path string) ([]byte, error) {
		if path != scriptPath {
			return nil, os.ErrNotExist
		}
		return []byte("   \n\t"), nil
	}

	impl := &Implementation{
		Script:   "./scripts/empty.sh",
		Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
	}

	_, err := impl.ResolveScriptWithFSAndModule(invowkfilePath, modulePath, readFile)
	if err == nil {
		t.Fatal("ResolveScriptWithFSAndModule() error = nil, want invalid script content")
	}
	if !errors.Is(err, ErrInvalidScriptContent) {
		t.Fatalf("errors.Is(err, ErrInvalidScriptContent) = false for %v", err)
	}
}

// TestResolveScriptWithFSAndModule_NoModulePath_NoContainmentCheck verifies that
// containment checking is NOT applied when modulePath is empty (non-module
// context). This is the backwards-compatibility case.
func TestResolveScriptWithFSAndModule_NoModulePath_NoContainmentCheck(t *testing.T) {
	t.Parallel()

	// Create a script file in one directory and an invowkfile in another,
	// where the script references a parent path. Without module context,
	// this should NOT trigger ErrScriptPathTraversal.
	baseDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho outside"
	if err := os.WriteFile(filepath.Join(baseDir, "outside.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	subDir := filepath.Join(baseDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	invowkfilePath := FilesystemPath(filepath.Join(subDir, "invowkfile.cue"))

	impl := &Implementation{
		Script:   "../outside.sh",
		Runtimes: []RuntimeConfig{{Name: RuntimeNative}},
	}

	// Empty modulePath means no containment check.
	result, err := impl.ResolveScriptWithFSAndModule(invowkfilePath, "", os.ReadFile)
	if errors.Is(err, ErrScriptPathTraversal) {
		t.Fatalf("unexpected ErrScriptPathTraversal without module context: %v", err)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != scriptContent {
		t.Errorf("ResolveScriptWithFSAndModule() = %q, want %q", result, scriptContent)
	}
}

func TestPlatformRuntimeKey_Validate_BothInvalidAggregatesErrors(t *testing.T) {
	t.Parallel()

	key := PlatformRuntimeKey{Platform: "bogus-platform", Runtime: "bogus-runtime"}
	err := key.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The joined error should contain both platform and runtime errors
	if !errors.Is(err, ErrInvalidPlatform) {
		t.Errorf("error should wrap ErrInvalidPlatform, got: %v", err)
	}
	if !errors.Is(err, ErrInvalidRuntimeMode) {
		t.Errorf("error should wrap ErrInvalidRuntimeMode, got: %v", err)
	}
}
