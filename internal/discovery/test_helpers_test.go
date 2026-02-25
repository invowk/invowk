// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

// newTestDiscovery creates a Discovery instance with standard test directories.
// Default baseDir=tmpDir, commandsDir=tmpDir/.invowk/cmds. Extra opts override defaults.
func newTestDiscovery(t *testing.T, cfg *config.Config, tmpDir string, opts ...Option) *Discovery {
	t.Helper()
	defaults := []Option{
		WithBaseDir(types.FilesystemPath(tmpDir)),
		WithCommandsDir(types.FilesystemPath(filepath.Join(tmpDir, ".invowk", "cmds"))),
	}
	return New(cfg, append(defaults, opts...)...)
}

// createTestModule creates a module with the two-file format (invowkmod.cue + invowkfile.cue).
func createTestModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	invowkmodContent := `module: "` + moduleID + `"
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	invowkfileContent := `cmds: [{name: "` + cmdName + `", implementations: [{script: "echo test", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}]}]}]`
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}
}
