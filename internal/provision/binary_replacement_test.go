// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// seamAfterCheck replaces the binary after the resolution checked it and
	// before the cache key hashes it.
	seamAfterCheck replacementSeam = "after-check"
	// seamBeforeCopy replaces the binary after the cache key hashed it and
	// before the build context copies it.
	seamBeforeCopy replacementSeam = "before-copy"
)

type (
	replacementSeam string

	// binaryRecordingEngine records the invowk binary of every build
	// context, before the provisioner removes it.
	binaryRecordingEngine struct {
		*mockEngine
		binaries [][]byte // nil entry: the build context had no binary
	}

	// binaryReplacement writes the file that replaces the binary at path.
	binaryReplacement struct {
		name  string
		write func(t *testing.T, path string)
	}
)

func (e *binaryRecordingEngine) Build(ctx context.Context, opts container.BuildOptions) error {
	data, err := os.ReadFile(filepath.Join(string(opts.ContextDir), "invowk"))
	if err != nil {
		data = nil
	}
	e.binaries = append(e.binaries, data)
	return e.mockEngine.Build(ctx, opts)
}

// replacementsFor returns the replacements of a usable amd64 binary at a
// seam. Every one makes the bytes the build would install unusable or
// different from the hashed ones. After the check, the replacing Linux build
// is for another architecture (the copy's check rejects it); before the copy,
// it is a different amd64 build (its hash differs from the key's).
func replacementsFor(seam replacementSeam) []binaryReplacement {
	otherELF := binaryReplacement{name: "different Linux build", write: func(t *testing.T, path string) {
		t.Helper()
		if seam == seamAfterCheck {
			testutil.WriteLinuxELFFixture(t, path, "arm64", nil)
			return
		}
		testutil.WriteLinuxELFFixture(t, path, "amd64", []byte("upgraded in place"))
	}}
	return []binaryReplacement{
		{name: "macOS build", write: func(t *testing.T, path string) {
			t.Helper()
			testutil.WriteMachOFixture(t, path)
		}},
		{name: "script", write: func(t *testing.T, path string) {
			t.Helper()
			if err := os.WriteFile(path, []byte("#!/bin/sh\necho replaced\n"), 0o755); err != nil {
				t.Fatalf("write script: %v", err)
			}
		}},
		otherELF,
	}
}

// replaceBinary replaces the file at path the way a package manager does: a
// new file renamed over the old one.
func replaceBinary(t *testing.T, path string, replacement binaryReplacement) {
	t.Helper()
	staged := path + ".new"
	replacement.write(t, staged)
	if err := os.Rename(staged, path); err != nil {
		t.Fatalf("rename replacement over %s: %v", path, err)
	}
}

// installSeam makes the provisioner replace the binary at path once, at seam.
func installSeam(t *testing.T, p *LayerProvisioner, seam replacementSeam, path string, replacement binaryReplacement) {
	t.Helper()
	replace := func() { replaceBinary(t, path, replacement) }
	switch seam {
	case seamAfterCheck:
		p.afterBinaryCheck = replace
	case seamBeforeCopy:
		p.beforeBinaryCopy = replace
	}
}

// binarylessTag is the tag of the image without the binary for p's config.
func binarylessTag(t *testing.T, p *LayerProvisioner) container.ImageTag {
	t.Helper()
	key, _, err := p.calculateCacheKey(t.Context(), testProvisionBaseImage, "")
	if err != nil {
		t.Fatalf("calculateCacheKey(no binary) error = %v", err)
	}
	return container.ImageTag(p.buildProvisionedTag(key[:12]))
}

func TestProvisionFailsWhenConfiguredBinaryChanges(t *testing.T) {
	t.Parallel()

	for _, seam := range []replacementSeam{seamAfterCheck, seamBeforeCopy} {
		for _, replacement := range replacementsFor(seam) {
			t.Run(string(seam)+"/"+replacement.name, func(t *testing.T) {
				t.Parallel()

				path := testutil.WriteLinuxELFFixture(t, filepath.Join(t.TempDir(), "invowk"), "amd64", nil)
				engine := &binaryRecordingEngine{mockEngine: newMockEngine()}
				p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: types.FilesystemPath(path)}, linuxAMD64NoReport())
				installSeam(t, p, seam, path, replacement)

				result, err := provisionBase(t, p)
				if err == nil {
					t.Fatalf("Provision() = %+v, want an error for a binary_path that changed", result)
				}
				if !errors.Is(err, ErrInvowkBinaryChanged) || !errors.Is(err, ErrUnusableInvowkBinary) {
					t.Errorf("Provision() error = %v, want ErrInvowkBinaryChanged and ErrUnusableInvowkBinary", err)
				}
				for _, part := range []string{"container.auto_provision.binary_path", path, "changed during provisioning"} {
					if !strings.Contains(err.Error(), part) {
						t.Errorf("Provision() error %q does not contain %q", err, part)
					}
				}
				if len(engine.binaries) != 0 {
					t.Errorf("builds = %d, want none: no image is tagged for a changed binary_path", len(engine.binaries))
				}
			})
		}
	}
}

func TestProvisionSkipsDefaultBinaryThatChanges(t *testing.T) {
	t.Parallel()

	for _, seam := range []replacementSeam{seamAfterCheck, seamBeforeCopy} {
		for _, replacement := range replacementsFor(seam) {
			for _, strict := range []bool{false, true} {
				name := string(seam) + "/" + replacement.name
				if strict {
					name += "/strict"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					path := testutil.WriteLinuxELFFixture(t, filepath.Join(t.TempDir(), "invowk"), "amd64", nil)
					engine := &binaryRecordingEngine{mockEngine: newMockEngine()}
					cfg := &Config{DefaultInvowkBinaryPath: types.FilesystemPath(path), Strict: strict}
					p := newResolutionProvisioner(t, engine, cfg, linuxAMD64NoReport())
					wantTag := binarylessTag(t, p)
					installSeam(t, p, seam, path, replacement)

					result, err := provisionBase(t, p)
					if err != nil {
						t.Fatalf("Provision() error = %v, want the default binary skipped", err)
					}
					if result.ImageTag != wantTag {
						t.Errorf("ImageTag = %q, want the binary-less tag %q", result.ImageTag, wantTag)
					}
					if len(engine.binaries) != 1 || engine.binaries[0] != nil {
						t.Fatalf("build contexts = %d (binary in first: %t), want one build without the binary",
							len(engine.binaries), len(engine.binaries) > 0 && engine.binaries[0] != nil)
					}
					if _, ok := result.EnvVars["PATH"]; ok {
						t.Errorf("EnvVars = %v, want no PATH entry for the binary", result.EnvVars)
					}
					assertSkipWarning(t, result, "invowk executable changed during provisioning", path)
				})
			}
		}
	}
}

func TestProvisionInstallsUnchangedBinaryCopy(t *testing.T) {
	t.Parallel()

	path := testutil.WriteLinuxELFFixture(t, filepath.Join(t.TempDir(), "invowk"), "amd64", []byte("v1"))
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	engine := &binaryRecordingEngine{mockEngine: newMockEngine()}
	p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: types.FilesystemPath(path)}, linuxAMD64NoReport())
	key, _, err := p.calculateCacheKey(t.Context(), testProvisionBaseImage, types.FilesystemPath(path))
	if err != nil {
		t.Fatalf("calculateCacheKey() error = %v", err)
	}

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if wantTag := container.ImageTag(p.buildProvisionedTag(key[:12])); result.ImageTag != wantTag {
		t.Errorf("ImageTag = %q, want %q", result.ImageTag, wantTag)
	}
	if len(engine.binaries) != 1 || !bytes.Equal(engine.binaries[0], want) {
		t.Fatalf("build context binary differs from the checked binary (builds: %d)", len(engine.binaries))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Warnings = %+v, want none", result.Warnings)
	}
}

func TestProvisionReusesCachedImageWithoutCopyingBinary(t *testing.T) {
	t.Parallel()

	path := testutil.WriteLinuxELFFixture(t, filepath.Join(t.TempDir(), "invowk"), "amd64", nil)
	engine := &binaryRecordingEngine{mockEngine: newMockEngine()}
	engine.imageExistsResult = true
	p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: types.FilesystemPath(path)}, linuxAMD64NoReport())
	copied := false
	p.beforeBinaryCopy = func() { copied = true }

	result, err := provisionBase(t, p)
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if len(engine.binaries) != 0 || copied {
		t.Errorf("builds = %d, copy reached = %t; want a cached image reused with no build context", len(engine.binaries), copied)
	}
	if result.ImageTag == testProvisionBaseImage {
		t.Errorf("ImageTag = %q, want the provisioned tag", result.ImageTag)
	}
}

// TestCopyCheckedBinaryFollowsSymlink installs a symlinked binary_path: the
// copy opens the link's target, as the check and the hash do.
func TestCopyCheckedBinaryFollowsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := testutil.WriteLinuxELFFixture(t, filepath.Join(dir, "invowk-real"), "amd64", nil)
	link := filepath.Join(dir, "invowk")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	engine := &binaryRecordingEngine{mockEngine: newMockEngine()}
	p := newResolutionProvisioner(t, engine, &Config{InvowkBinaryPath: types.FilesystemPath(link)}, linuxAMD64NoReport())

	if _, err := provisionBase(t, p); err != nil {
		t.Fatalf("Provision() error = %v, want a symlinked binary_path installed", err)
	}
	want, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if len(engine.binaries) != 1 || !bytes.Equal(engine.binaries[0], want) {
		t.Fatal("build context does not hold the link target's bytes")
	}
}

// resolvedTestBinary resolves p's binary for debian:stable-slim and records
// its cache-key hash, as Provision does before it prepares a build context.
func resolvedTestBinary(t *testing.T, p *LayerProvisioner) binaryResolution {
	t.Helper()
	binary, err := p.resolveInvowkBinary(t.Context(), "debian:stable-slim", true)
	if err != nil {
		t.Fatalf("resolveInvowkBinary() error = %v", err)
	}
	_, binary.hash, err = p.calculateCacheKey(t.Context(), "debian:stable-slim", binary.path)
	if err != nil {
		t.Fatalf("calculateCacheKey() error = %v", err)
	}
	return binary
}
