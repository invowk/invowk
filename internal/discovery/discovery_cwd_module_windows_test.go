// SPDX-License-Identifier: MPL-2.0

//go:build windows

package discovery

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// TestDiscovery_CurrentDirModule_ShortNameCwd: a current directory named by
// its 8.3 short name is judged by the name the parent directory stores.
func TestDiscovery_CurrentDirModule_ShortNameCwd(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	moduleDir := filepath.Join(parent, "tools.invowkmod")
	createTestModule(t, moduleDir, "tools", "hello")

	short := shortPathName(t, moduleDir)
	if strings.EqualFold(filepath.Base(short), filepath.Base(moduleDir)) {
		t.Skipf("volume of %s has no 8.3 short names (short path %s)", moduleDir, short)
	}
	t.Logf("short name of %s: %s", moduleDir, short)

	out := discoverFromBase(t, nil, short)
	if out.err != nil {
		t.Fatalf("error: %v", out.err)
	}
	if got := out.moduleNames(); !slices.Equal(got, []string{"tools"}) {
		t.Fatalf("modules = %v, want [tools]", got)
	}
	if plain := out.plainFiles(); len(plain) != 0 {
		t.Fatalf("read plain invowkfile %s", plain[0].Path)
	}

	subDir := filepath.Join(moduleDir, "sub")
	writeTreeFile(t, filepath.Join(subDir, "invowkfile.cue"), `cmds: [{name: "subcmd", implementations: [{script: "Write-Output 'SUB'", runtimes: [{name: "native"}], platforms: [{name: "windows"}]}]}]`)
	shortSub := filepath.Join(short, "sub")
	subOut := discoverFromBase(t, nil, shortSub)
	subOut.requireNothingLoaded(t, "short ancestor subdir")
	subOut.requireCode(t, "short ancestor subdir", CodeModuleLoadSkipped)
}

func shortPathName(t *testing.T, path string) string {
	t.Helper()
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString(%q): %v", path, err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatalf("GetShortPathName(%q): %v", path, err)
	}
	return windows.UTF16ToString(buf[:n])
}
