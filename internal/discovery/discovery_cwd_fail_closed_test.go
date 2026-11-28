// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestDiscovery_CurrentDirTagsAndNonAborting(t *testing.T) {
	t.Parallel()

	t.Run("broken cwd invowkfile and sibling module are tagged FromCurrentDir while valid sources remain discoverable", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		cwd := filepath.Join(root, "cwd")
		cmdsDir := filepath.Join(root, "cmds")
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatalf("mkdir cwd: %v", err)
		}
		if err := os.WriteFile(filepath.Join(cwd, "invowkfile.cue"), []byte("cmds: [{name: \"local\", bogus: 1, implementations: [{script: {content: \"echo hi\"}, runtimes: [{name: \"virtual-sh\"}], platforms: [{name: \"linux\"}]}]}]"), 0o644); err != nil {
			t.Fatalf("write broken cwd invowkfile: %v", err)
		}
		brokenModDir := filepath.Join(cwd, "badmod.invowkmod")
		if err := os.MkdirAll(brokenModDir, 0o755); err != nil {
			t.Fatalf("mkdir broken cwd module: %v", err)
		}
		if err := os.WriteFile(filepath.Join(brokenModDir, "invowkmod.cue"), []byte("module: \"badmod\"\nversion: \"1.0.0\"\nbogus: true\n"), 0o644); err != nil {
			t.Fatalf("write broken cwd invowkmod: %v", err)
		}
		createTestModule(t, filepath.Join(cmdsDir, "global.invowkmod"), "global", "ok")
		globalBroken := filepath.Join(cmdsDir, "globalbroken.invowkmod")
		if err := os.MkdirAll(globalBroken, 0o755); err != nil {
			t.Fatalf("mkdir broken global module: %v", err)
		}
		if err := os.WriteFile(filepath.Join(globalBroken, "invowkmod.cue"), []byte("module: \"globalbroken\"\nversion: \"1.0.0\"\nbogus: true\n"), 0o644); err != nil {
			t.Fatalf("write broken global invowkmod: %v", err)
		}

		d := New(config.DefaultConfig(), WithBaseDir(types.FilesystemPath(cwd)), WithCommandsDir(types.FilesystemPath(cmdsDir)))
		result, err := d.DiscoverAndValidateCommandSet(t.Context())
		if err != nil {
			t.Fatalf("DiscoverAndValidateCommandSet() error = %v, want non-aborting result", err)
		}
		if result.Set == nil || result.Set.ByName["global ok"] == nil {
			t.Fatalf("expected valid global command to remain discoverable, got %+v", result.Set)
		}

		cwdErr := CurrentDirLoadError(result.Diagnostics)
		if cwdErr == nil {
			t.Fatal("CurrentDirLoadError() = nil, want combined cwd error")
		}
		if !strings.Contains(cwdErr.Error(), "field not allowed") {
			t.Fatalf("CurrentDirLoadError() = %q, want field not allowed", cwdErr)
		}
		for _, diag := range result.Diagnostics {
			if strings.Contains(string(diag.Path()), "globalbroken.invowkmod") && diag.FromCurrentDir() {
				t.Fatalf("non-cwd diagnostic %+v was wrongly tagged FromCurrentDir", diag)
			}
		}
	})

	t.Run("policy and structural skips do not trigger CurrentDirLoadError", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		reservedDir := filepath.Join(root, "invowkfile.invowkmod")
		createTestModule(t, reservedDir, "invowkfile", "ignored")
		mismatchDir := filepath.Join(root, "tools.invowkmod")
		createTestModule(t, mismatchDir, "other", "ignored")

		d := New(config.DefaultConfig(), WithBaseDir(types.FilesystemPath(root)), WithCommandsDir(""))
		result, err := d.DiscoverCommandSet(t.Context())
		if err != nil {
			t.Fatalf("DiscoverCommandSet() error = %v", err)
		}
		if cwdErr := CurrentDirLoadError(result.Diagnostics); cwdErr != nil {
			t.Fatalf("CurrentDirLoadError() = %v, want nil for policy/naming skips", cwdErr)
		}
	})
}

func TestDiscovery_CurrentDirInvowkVersionPreserved(t *testing.T) { //nolint:paralleltest // OverrideBuildVersionForTest mutates process-wide version override.
	defer invowkmod.OverrideBuildVersionForTest("0.18.0")()

	root := t.TempDir()
	modDir := filepath.Join(root, "newer.invowkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir modDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkmod.cue"), []byte("module: \"newer\"\nversion: \"1.0.0\"\ninvowk_version: \">=0.19.0\"\nfuture: true\n"), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}

	d := New(config.DefaultConfig(), WithBaseDir(types.FilesystemPath(root)), WithCommandsDir(""))
	result, err := d.DiscoverCommandSet(t.Context())
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}
	cwdErr := CurrentDirLoadError(result.Diagnostics)
	if !errors.Is(cwdErr, invowkmod.ErrUnsupportedInvowkVersion) {
		t.Fatalf("CurrentDirLoadError() = %v, want ErrUnsupportedInvowkVersion", cwdErr)
	}
}
