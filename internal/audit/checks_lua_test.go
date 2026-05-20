// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestLuaCheckerFindsVirtualLuaRisks(t *testing.T) {
	t.Parallel()

	sc := newLuaScriptContext(
		`local token = os.getenv("GITHUB_TOKEN")
os.execute("curl https://example.com")`,
		invowkfile.RuntimeConfig{
			Name:            invowkfile.RuntimeVirtualLua,
			AllowedBinaries: []invowkfile.AllowedBinary{"*", "curl"},
		},
		[]invowkfile.PlatformConfig{{
			Name: invowkfile.PlatformLinux,
			Virtual: &invowkfile.PlatformVirtualConfig{Filesystem: &invowkfile.VirtualFilesystemConfig{
				Access: invowkfile.VirtualFilesystemAccessFull,
				Paths:  invowkfile.VirtualFilesystemPaths{"HOME_DIR": "@home"},
			}},
		}},
	)

	findings, err := NewLuaChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	for _, code := range []FindingCode{
		codeLuaDisabledAPI,
		codeLuaSensitiveEnvRead,
		codeLuaHostBinaryWildcard,
		codeLuaNetworkAllowedBinary,
		codeLuaFullFilesystemAccess,
		codeLuaBroadPathMapping,
	} {
		if !hasFindingCode(findings, code) {
			t.Fatalf("LuaChecker findings missing %s: %+v", code, findings)
		}
	}
}

func TestLuaCheckerIgnoresNonLuaRuntime(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext(`os.execute("curl https://example.com")`)
	findings, err := NewLuaChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("LuaChecker() findings = %+v, want none", findings)
	}
}

func TestScanContextIncludesModuleLuaFiles(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "io.example.lua.invowkmod")
	if err := os.MkdirAll(filepath.Join(moduleDir, "helpers"), 0o755); err != nil {
		t.Fatalf("mkdir helpers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "io.example.lua"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(`cmds: [{
	name: "lua"
	implementations: [{
		script: {content: "local h = require(\"helpers.format\")\nprint(h.value)"}
		runtimes: [{name: "virtual-lua"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "helpers", "format.lua"), []byte(`return { value = os.getenv("GITHUB_TOKEN") }`), 0o644); err != nil {
		t.Fatalf("write helper Lua: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "unused.lua"), []byte(`io.popen("curl https://example.com")`), 0o644); err != nil {
		t.Fatalf("write unused Lua: %v", err)
	}

	sc, err := BuildScanContext(t.Context(), types.FilesystemPath(moduleDir), nil, false)
	if err != nil {
		t.Fatalf("BuildScanContext() error = %v", err)
	}

	paths := make(map[string]bool)
	for _, ref := range sc.AllScripts() {
		if ref.ScriptPath != "" {
			paths[filepath.Base(string(ref.ScriptPath))] = true
		}
	}
	if !paths["format.lua"] || !paths["unused.lua"] {
		t.Fatalf("Lua script refs = %v, want format.lua and unused.lua", paths)
	}

	findings, err := NewLuaChecker().Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingCode(findings, codeLuaSensitiveEnvRead) {
		t.Fatalf("LuaChecker findings missing sensitive env read: %+v", findings)
	}
	if !hasFindingCode(findings, codeLuaDisabledAPI) {
		t.Fatalf("LuaChecker findings missing disabled API from unused Lua file: %+v", findings)
	}
}

func newLuaScriptContext(script string, cfg invowkfile.RuntimeConfig, platforms []invowkfile.PlatformConfig) *ScanContext {
	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "lua",
			Implementations: []invowkfile.Implementation{{
				Script:    invowkfile.ImplementationScript{Content: invowkfile.ScriptContent(script)},
				Runtimes:  []invowkfile.RuntimeConfig{cfg},
				Platforms: platforms,
			}},
		}},
	}
	files := []*ScannedInvowkfile{{
		Path:       "test.cue",
		SurfaceID:  "test",
		Invowkfile: inv,
	}}
	return newTestScanContext(files, nil)
}

func hasFindingCode(findings []Finding, code FindingCode) bool {
	for i := range findings {
		finding := findings[i]
		if finding.Code == code {
			return true
		}
	}
	return false
}
