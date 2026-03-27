// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestValidateExecutionContextForRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		ctx         *ExecutionContext
		noImplErr   error
		noScriptErr error
		wantErr     error
	}{
		{
			name:        "nil context",
			ctx:         nil,
			noImplErr:   errNativeNoImpl,
			noScriptErr: errNativeNoScript,
			wantErr:     errNilExecutionContext,
		},
		{
			name:        "missing invowkfile",
			ctx:         &ExecutionContext{},
			noImplErr:   errNativeNoImpl,
			noScriptErr: errNativeNoScript,
			wantErr:     errNoInvowkfile,
		},
		{
			name: "missing implementation",
			ctx: &ExecutionContext{
				Invowkfile: &invowkfile.Invowkfile{},
			},
			noImplErr:   errContainerNoImpl,
			noScriptErr: errContainerNoScript,
			wantErr:     errContainerNoImpl,
		},
		{
			name: "missing script content",
			ctx: &ExecutionContext{
				Invowkfile:   &invowkfile.Invowkfile{},
				SelectedImpl: &invowkfile.Implementation{},
			},
			noImplErr:   errNativeNoImpl,
			noScriptErr: errNativeNoScript,
			wantErr:     errNativeNoScript,
		},
		{
			name: "valid context",
			ctx: &ExecutionContext{
				Invowkfile:   &invowkfile.Invowkfile{},
				SelectedImpl: &invowkfile.Implementation{Script: "echo ok"},
			},
			noImplErr:   errNativeNoImpl,
			noScriptErr: errNativeNoScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateExecutionContextForRun(tt.ctx, tt.noImplErr, tt.noScriptErr)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validateExecutionContextForRun() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateExecutionContextForRun() returned nil, want %v", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateExecutionContextForRun() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuntimeExecuteGuards_NoPanics(t *testing.T) {
	t.Parallel()

	t.Run("native execute nil context", func(t *testing.T) {
		t.Parallel()
		rt := NewNativeRuntime()
		result := rt.Execute(nil)
		if result.Error == nil || !errors.Is(result.Error, errNilExecutionContext) {
			t.Fatalf("Execute(nil) error = %v, want %v", result.Error, errNilExecutionContext)
		}
	})

	t.Run("native execute nil implementation", func(t *testing.T) {
		t.Parallel()
		ctx := testExecutionContextForGuard(t, invowkfile.RuntimeNative)
		ctx.SelectedImpl = nil
		rt := NewNativeRuntime()
		result := rt.Execute(ctx)
		if result.Error == nil || !errors.Is(result.Error, errNativeNoImpl) {
			t.Fatalf("Execute() error = %v, want %v", result.Error, errNativeNoImpl)
		}
	})

	t.Run("virtual execute nil implementation", func(t *testing.T) {
		t.Parallel()
		ctx := testExecutionContextForGuard(t, invowkfile.RuntimeVirtual)
		ctx.SelectedImpl = nil
		rt := NewVirtualRuntime(true)
		result := rt.Execute(ctx)
		if result.Error == nil || !errors.Is(result.Error, errVirtualNoImpl) {
			t.Fatalf("Execute() error = %v, want %v", result.Error, errVirtualNoImpl)
		}
	})

	t.Run("container execute nil implementation", func(t *testing.T) {
		t.Parallel()
		ctx := testExecutionContextForGuard(t, invowkfile.RuntimeContainer)
		ctx.SelectedImpl = nil
		rt := &ContainerRuntime{}
		result := rt.Execute(ctx)
		if result.Error == nil || !errors.Is(result.Error, errContainerNoImpl) {
			t.Fatalf("Execute() error = %v, want %v", result.Error, errContainerNoImpl)
		}
	})
}

func testExecutionContextForGuard(t *testing.T, runtimeMode invowkfile.RuntimeMode) *ExecutionContext {
	t.Helper()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		Commands: []invowkfile.Command{
			{
				Name: "guard-test",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: runtimeMode}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("guard-test")
	if cmd == nil {
		t.Fatal("test fixture command not found")
	}

	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.SelectedRuntime = runtimeMode
	ctx.SelectedImpl = cmd.GetImplForPlatformRuntime(invowkfile.CurrentPlatform(), runtimeMode)
	if ctx.SelectedImpl == nil {
		t.Fatalf("no implementation found for runtime %q on platform %q", runtimeMode, invowkfile.CurrentPlatform())
	}
	return ctx
}
