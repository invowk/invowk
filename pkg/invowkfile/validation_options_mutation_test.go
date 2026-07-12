// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestValidationOptionsMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "nil invowkfile uses empty path defaults", run: testValidationOptionsNilInvowkfileDefaults},
		{name: "empty invowkfile path does not derive workdir", run: testValidationOptionsEmptyInvowkfilePath},
		{name: "file path derives workdir and default filesystem root", run: testValidationOptionsFilePathDerivesWorkdir},
		{name: "explicit options project into context", run: testValidationOptionsExplicitOptionsProject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testValidationOptionsNilInvowkfileDefaults(t *testing.T) {
	t.Helper()

	opts := defaultValidateOptions(nil)
	if opts.workDir != "" {
		t.Fatalf("default workDir = %q, want empty", opts.workDir)
	}
	requireValidationContextEmptyPaths(t, opts.buildValidationContext(nil))
}

func testValidationOptionsEmptyInvowkfilePath(t *testing.T) {
	t.Helper()

	inv := &Invowkfile{}
	opts := defaultValidateOptions(inv)
	if opts.workDir != "" {
		t.Fatalf("default workDir = %q, want empty", opts.workDir)
	}
	requireValidationContextEmptyPaths(t, opts.buildValidationContext(inv))
}

func testValidationOptionsFilePathDerivesWorkdir(t *testing.T) {
	t.Helper()

	workDir := filepath.Join(t.TempDir(), "nested")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "marker.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	inv := &Invowkfile{FilePath: FilesystemPath(filepath.Join(workDir, "invowkfile.cue"))}
	opts := defaultValidateOptions(inv)
	if opts.workDir != FilesystemPath(workDir) {
		t.Fatalf("default workDir = %q, want %q", opts.workDir, workDir)
	}

	ctx := opts.buildValidationContext(inv)
	if ctx.WorkDir != WorkDir(workDir) {
		t.Fatalf("context WorkDir = %q, want %q", ctx.WorkDir, workDir)
	}
	if ctx.FilePath != inv.FilePath {
		t.Fatalf("context FilePath = %q, want %q", ctx.FilePath, inv.FilePath)
	}
	requireValidationContextFile(t, ctx, "marker.txt")
}

func testValidationOptionsExplicitOptionsProject(t *testing.T) {
	t.Helper()

	inv := &Invowkfile{FilePath: "invowkfile.cue"}
	opts := defaultValidateOptions(inv)
	WithFS(fstest.MapFS{"marker.txt": &fstest.MapFile{Data: []byte("ok")}})(&opts)
	WithPlatform(PlatformLinux)(&opts)
	WithStrictMode(true)(&opts)
	WithWorkDir("workspace")(&opts)

	ctx := opts.buildValidationContext(inv)
	if ctx.WorkDir != "workspace" {
		t.Fatalf("context WorkDir = %q, want workspace", ctx.WorkDir)
	}
	if ctx.Platform != PlatformLinux {
		t.Fatalf("context Platform = %q, want linux", ctx.Platform)
	}
	if !ctx.StrictMode {
		t.Fatal("context StrictMode = false, want true")
	}
	if ctx.FilePath != inv.FilePath {
		t.Fatalf("context FilePath = %q, want %q", ctx.FilePath, inv.FilePath)
	}
	requireValidationContextFile(t, ctx, "marker.txt")
}

func requireValidationContextEmptyPaths(t *testing.T, ctx *ValidationContext) {
	t.Helper()

	if ctx.WorkDir != "" || ctx.FilePath != "" {
		t.Fatalf("context path defaults = %q/%q, want empty/empty", ctx.WorkDir, ctx.FilePath)
	}
	if ctx.FS == nil {
		t.Fatal("context FS = nil, want default filesystem")
	}
}

func requireValidationContextFile(t *testing.T, ctx *ValidationContext, name string) {
	t.Helper()

	if ctx.FS == nil {
		t.Fatal("context FS = nil, want filesystem")
	}
	if _, err := fs.Stat(ctx.FS, name); err != nil {
		t.Fatalf("Stat(%q) = %v, want file in validation context FS", name, err)
	}
}
