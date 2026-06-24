// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImplementationMutationOptionalValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Implementation)
		sentinel  error
		wantValid bool
	}{
		{
			name: "invalid env validates implementation env",
			mutate: func(impl *Implementation) {
				impl.Env = &EnvConfig{Vars: map[EnvVarName]string{"1BAD": "value"}}
			},
			sentinel: ErrInvalidEnvConfig,
		},
		{
			name: "invalid workdir validates non-empty implementation workdir",
			mutate: func(impl *Implementation) {
				impl.WorkDir = "   "
			},
			sentinel: ErrInvalidWorkDir,
		},
		{
			name: "invalid depends_on validates implementation dependencies",
			mutate: func(impl *Implementation) {
				impl.DependsOn = &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{""}}}}
			},
			sentinel: ErrInvalidDependsOn,
		},
		{
			name: "invalid timeout validates implementation timeout",
			mutate: func(impl *Implementation) {
				impl.Timeout = "0s"
			},
			sentinel: ErrInvalidDurationString,
		},
		{
			name: "empty optional fields remain valid",
			mutate: func(impl *Implementation) {
				impl.WorkDir = ""
				impl.Timeout = ""
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := testValidImplementation()
			tt.mutate(&impl)

			err := impl.Validate()
			if tt.wantValid {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.sentinel) {
				t.Fatalf("Validate() error = %v, want sentinel %v", err, tt.sentinel)
			}
		})
	}
}

func TestImplementationMutationScriptValidationContracts(t *testing.T) {
	t.Parallel()

	t.Run("script validates interpreter when present", func(t *testing.T) {
		t.Parallel()

		err := ImplementationScript{
			Content:     "echo hello",
			Interpreter: "not a valid interpreter",
		}.Validate()
		if !errors.Is(err, ErrUnsafeInterpreterSpec) {
			t.Fatalf("ImplementationScript.Validate() error = %v, want ErrUnsafeInterpreterSpec", err)
		}
	})

	t.Run("script validates file path when selected", func(t *testing.T) {
		t.Parallel()

		err := ImplementationScript{File: filesystemPathPtr("   ")}.Validate()
		if !errors.Is(err, ErrInvalidFilesystemPath) {
			t.Fatalf("ImplementationScript.Validate() error = %v, want ErrInvalidFilesystemPath", err)
		}
	})

	t.Run("resolver validates script source before resolving", func(t *testing.T) {
		t.Parallel()

		readCalled := false
		impl := &Implementation{Script: ImplementationScript{}}
		_, err := impl.ResolveScriptWithFSAndModule("invowkfile.cue", "module.invowkmod", func(string) ([]byte, error) {
			readCalled = true
			return []byte("echo should not read"), nil
		})
		if !errors.Is(err, ErrMissingImplementationScriptSource) {
			t.Fatalf("ResolveScriptWithFSAndModule() error = %v, want ErrMissingImplementationScriptSource", err)
		}
		if readCalled {
			t.Fatal("ResolveScriptWithFSAndModule read a file before validating script source")
		}
	})

	t.Run("inline script content is validated during resolve", func(t *testing.T) {
		t.Parallel()

		impl := &Implementation{Script: ImplementationScript{Content: "   \n\t"}}
		_, err := impl.ResolveScript("invowkfile.cue")
		if !errors.Is(err, ErrInvalidScriptContent) {
			t.Fatalf("ResolveScript() error = %v, want ErrInvalidScriptContent", err)
		}
	})

	t.Run("file script without reader returns reader sentinel", func(t *testing.T) {
		t.Parallel()

		moduleDir := t.TempDir()
		impl := &Implementation{Script: ImplementationScript{File: filesystemPathPtr("scripts/build.sh")}}
		_, err := impl.ResolveScriptWithModule(
			FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue")),
			FilesystemPath(moduleDir),
		)
		if !errors.Is(err, ErrScriptReaderRequired) {
			t.Fatalf("ResolveScriptWithModule() error = %v, want ErrScriptReaderRequired", err)
		}
	})
}

func TestImplementationMutationScriptReadErrorContract(t *testing.T) {
	t.Parallel()

	t.Run("relative selected path reports resolved path and wraps read error", func(t *testing.T) {
		t.Parallel()

		moduleDir := t.TempDir()
		resolvedPath := filepath.Join(moduleDir, "scripts", "missing.sh")
		impl := &Implementation{Script: ImplementationScript{File: filesystemPathPtr("scripts/missing.sh")}}
		_, err := impl.ResolveScriptWithFSAndModule(
			FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue")),
			FilesystemPath(moduleDir),
			func(path string) ([]byte, error) {
				if path != resolvedPath {
					t.Fatalf("read path = %q, want %q", path, resolvedPath)
				}
				return nil, os.ErrNotExist
			},
		)
		requireScriptReadError(t, err, os.ErrNotExist, "scripts/missing.sh", resolvedPath)
		if !strings.Contains(err.Error(), "resolved to") {
			t.Fatalf("read error = %q, want resolved path detail", err.Error())
		}
	})

	t.Run("absolute selected path is rejected before file IO", func(t *testing.T) {
		t.Parallel()

		moduleDir := t.TempDir()
		absolutePath := filepath.Join(moduleDir, "missing.sh")
		readCalled := false
		impl := &Implementation{Script: ImplementationScript{File: filesystemPathPtr(absolutePath)}}
		_, err := impl.ResolveScriptWithFSAndModule(
			FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue")),
			FilesystemPath(moduleDir),
			func(string) ([]byte, error) {
				readCalled = true
				return nil, os.ErrNotExist
			},
		)
		if !errors.Is(err, ErrInvalidScriptFilePath) {
			t.Fatalf("ResolveScriptWithFSAndModule() error = %v, want ErrInvalidScriptFilePath", err)
		}
		if readCalled {
			t.Fatal("ResolveScriptWithFSAndModule read an absolute script file")
		}
	})

	t.Run("empty selected path reports resolved path and wraps read error", func(t *testing.T) {
		t.Parallel()

		resolvedPath := FilesystemPath(filepath.Join(t.TempDir(), "missing.sh"))
		err := scriptFileReadError("", resolvedPath, os.ErrNotExist)
		requireScriptReadError(t, err, os.ErrNotExist, resolvedPath.String())
		if strings.Contains(err.Error(), "resolved to") {
			t.Fatalf("read error = %q, want no resolved path detail", err.Error())
		}
	})
}

func TestImplementationMutationHostSSHContracts(t *testing.T) {
	t.Parallel()

	impl := Implementation{
		Script: ImplementationScript{Content: "echo hello"},
		Runtimes: []RuntimeConfig{
			{Name: RuntimeNative, EnableHostSSH: true},
			{Name: RuntimeVirtualSh, EnableHostSSH: true},
		},
		Platforms: []PlatformConfig{{Name: PlatformLinux}},
	}
	if impl.HasHostSSH() {
		t.Fatal("HasHostSSH() = true for non-container runtimes with EnableHostSSH set, want false")
	}
	if impl.GetHostSSHForRuntime(RuntimeNative) {
		t.Fatal("GetHostSSHForRuntime(native) = true, want false")
	}
	if impl.GetHostSSHForRuntime(RuntimeVirtualSh) {
		t.Fatal("GetHostSSHForRuntime(virtual-sh) = true, want false")
	}
}

func TestImplementationMutationDependenciesAndContainment(t *testing.T) {
	t.Parallel()

	t.Run("non-nil empty dependencies are empty", func(t *testing.T) {
		t.Parallel()

		impl := testValidImplementation()
		impl.DependsOn = &DependsOn{}
		if impl.HasDependencies() {
			t.Fatal("HasDependencies() = true for empty depends_on, want false")
		}
	})

	t.Run("exact parent path escapes module", func(t *testing.T) {
		t.Parallel()

		parentDir := t.TempDir()
		moduleDir := filepath.Join(parentDir, "module.invowkmod")
		if err := os.Mkdir(moduleDir, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}
		impl := &Implementation{Script: ImplementationScript{File: filesystemPathPtr("..")}}
		_, err := impl.ResolveScriptWithFSAndModule(
			FilesystemPath(filepath.Join(moduleDir, "invowkfile.cue")),
			FilesystemPath(moduleDir),
			func(string) ([]byte, error) {
				t.Fatal("readFile called for path outside module")
				return nil, nil
			},
		)
		if !errors.Is(err, ErrInvalidScriptFilePath) {
			t.Fatalf("ResolveScriptWithFSAndModule() error = %v, want ErrInvalidScriptFilePath", err)
		}
	})

	t.Run("inline script path lookup returns empty in module context", func(t *testing.T) {
		t.Parallel()

		impl := &Implementation{Script: ImplementationScript{Content: "echo inline"}}
		got := impl.GetScriptFilePathWithModule("invowkfile.cue", FilesystemPath(t.TempDir()))
		if got != "" {
			t.Fatalf("GetScriptFilePathWithModule() = %q for inline script, want empty", got)
		}
	})
}

func testValidImplementation() Implementation {
	return Implementation{
		Script:    ImplementationScript{Content: "echo hello"},
		Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
		Platforms: []PlatformConfig{{Name: PlatformLinux}},
	}
}

func requireScriptReadError(t *testing.T, err, sentinel error, fragments ...string) {
	t.Helper()

	if err == nil {
		t.Fatal("read error = nil, want error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("read error = %v, want sentinel %v", err, sentinel)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("read error = %q, want fragment %q", err.Error(), fragment)
		}
	}
}
