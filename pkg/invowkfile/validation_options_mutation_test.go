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

	t.Run("nil invowkfile uses empty path defaults", func(t *testing.T) {
		t.Parallel()
		opts := defaultValidateOptions(nil)
		if opts.workDir != "" {
			t.Fatalf("default workDir = %q, want empty", opts.workDir)
		}

		ctx := opts.buildValidationContext(nil)
		if ctx.WorkDir != "" || ctx.FilePath != "" {
			t.Fatalf("context path defaults = %q/%q, want empty/empty", ctx.WorkDir, ctx.FilePath)
		}
		if ctx.FS == nil {
			t.Fatal("context FS = nil, want default filesystem")
		}
	})

	t.Run("empty invowkfile path does not derive workdir", func(t *testing.T) {
		t.Parallel()
		inv := &Invowkfile{}
		opts := defaultValidateOptions(inv)
		if opts.workDir != "" {
			t.Fatalf("default workDir = %q, want empty", opts.workDir)
		}

		ctx := opts.buildValidationContext(inv)
		if ctx.WorkDir != "" || ctx.FilePath != "" {
			t.Fatalf("context path defaults = %q/%q, want empty/empty", ctx.WorkDir, ctx.FilePath)
		}
	})

	t.Run("file path derives workdir and default filesystem root", func(t *testing.T) {
		t.Parallel()
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
	})

	t.Run("explicit options project into context", func(t *testing.T) {
		t.Parallel()
		testFS := fstest.MapFS{
			"marker.txt": &fstest.MapFile{Data: []byte("ok")},
		}
		inv := &Invowkfile{FilePath: "invowkfile.cue"}
		opts := defaultValidateOptions(inv)
		WithFS(testFS)(&opts)
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
	})
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
