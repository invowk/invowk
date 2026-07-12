// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestLuaRuntimeExecuteBasicOutput(t *testing.T) {
	t.Parallel()

	ctx, stdout, _ := newLuaExecutionContext(t, `print("hello lua")`, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "hello lua\n" {
		t.Fatalf("stdout = %q, want hello lua newline", got)
	}
}

func TestLuaBridgePathEnvAndReadOnlyTables(t *testing.T) {
	t.Parallel()

	script := `
print(invowk.path("@work"))
print(invowk.env.FOO)
print(os.getenv("FOO"))
local invowkWriteOK = pcall(function() invowk.path = nil end)
local stateWriteOK = pcall(function() invowk.state.bin_path = "changed" end)
local cmdWriteOK = pcall(function() invowk.cmd.anything = function() end end)
print(tostring(invowkWriteOK))
print(tostring(stateWriteOK))
print(tostring(cmdWriteOK))
`
	env := map[invowkfile.EnvVarName]string{"FOO": "bar"}
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, env)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("stdout lines = %q, want 6 lines", stdout.String())
	}
	resolver := mustVirtualTestResolver(t, ctx)
	workDir := mustResolveVirtualBridgeTestPath(t, resolver, ctx.EffectiveWorkDir(), "@work")
	if lines[0] != workDir {
		t.Fatalf("invowk.path(@work) = %q, want %q", lines[0], workDir)
	}
	if lines[1] != "bar" || lines[2] != "bar" {
		t.Fatalf("env bridge lines = %q, want bar/bar", lines[1:3])
	}
	for i, line := range lines[3:] {
		if line != "false" {
			t.Fatalf("read-only check %d = %q, want false", i, line)
		}
	}
}

func TestLuaBridgeVirtualFilesystemPathsExposeResolvedPath(t *testing.T) {
	t.Parallel()

	script := `
print(invowk.path("DB_ROOT/file.txt"))
print(os.getenv("INVOWK_PATH_DB_ROOT"))
print(os.getenv("INVOWK_ANCHOR_WORK"))
`
	cfg := invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}
	ctx, stdout, _ := newLuaExecutionContext(t, script, cfg, nil)
	ctx.SelectedImpl.Platforms = testPlatformsWithVirtualFilesystem(
		"",
		invowkfile.VirtualFilesystemPaths{"DB_ROOT": "./db"},
	)
	resolver := mustVirtualTestResolver(t, ctx)
	dbRoot := resolver.paths["DB_ROOT"]
	if err := os.MkdirAll(dbRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(db root) error = %v", err)
	}

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout lines = %q, want 3 lines", stdout.String())
	}
	if want := mustResolveVirtualBridgeTestPath(t, resolver, ctx.EffectiveWorkDir(), "DB_ROOT/file.txt"); lines[0] != want {
		t.Fatalf("invowk.path(DB_ROOT/file.txt) = %q, want %q", lines[0], want)
	}
	if lines[1] != dbRoot {
		t.Fatalf("INVOWK_PATH_DB_ROOT = %q, want %q", lines[1], dbRoot)
	}
	workDir := resolver.anchors["@work"]
	if lines[2] != workDir {
		t.Fatalf("INVOWK_ANCHOR_WORK = %q, want %q", lines[2], workDir)
	}
}

func TestLuaVirtualRuntimeEnvOverridesUserInvowkState(t *testing.T) {
	t.Parallel()

	script := `
print(os.getenv("INVOWK_STATE_BIN_PATH"))
print(os.getenv("INVOWK_PATH_DB_ROOT"))
print(os.getenv("INVOWK_ANCHOR_WORK"))
`
	env := map[invowkfile.EnvVarName]string{
		"INVOWK_STATE_BIN_PATH": "user-bin",
		"INVOWK_PATH_DB_ROOT":   "user-path",
		"INVOWK_ANCHOR_WORK":    "user-work",
	}
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, env)
	ctx.SelectedImpl.Platforms = testPlatformsWithVirtualFilesystem(
		"",
		invowkfile.VirtualFilesystemPaths{"DB_ROOT": "./db"},
	)
	resolver := mustVirtualTestResolver(t, ctx)
	dbRoot := resolver.paths["DB_ROOT"]

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}

	lines := strings.Split(stdout.String(), "\n")
	if len(lines) < 4 {
		t.Fatalf("stdout lines = %q, want at least 3 values", stdout.String())
	}
	if lines[0] != "" {
		t.Fatalf("INVOWK_STATE_BIN_PATH = %q, want runtime-owned empty value", lines[0])
	}
	if lines[1] != dbRoot {
		t.Fatalf("INVOWK_PATH_DB_ROOT = %q, want %q", lines[1], dbRoot)
	}
	workDir := resolver.anchors["@work"]
	if lines[2] != workDir {
		t.Fatalf("INVOWK_ANCHOR_WORK = %q, want %q", lines[2], workDir)
	}
}

func TestLuaBridgePathRejectsUnknownMapping(t *testing.T) {
	t.Parallel()

	script := `
local ok = pcall(function() invowk.path("MISSING_PATH") end)
print(tostring(ok))
	`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "false\n" {
		t.Fatalf("stdout = %q, want false newline", got)
	}
}

func TestLuaFileIOUsesSharedPathValidator(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	outsidePath := filepath.Join(homeDir, ".invowk-denied-test")
	script := fmt.Sprintf(`
local f = assert(io.open("data.txt", "w"))
assert(f:write("inside"))
assert(f:close())
local r = assert(io.open("data.txt", "r"))
print(r:read("*a"))
assert(r:close())
local denied, err = io.open(%q, "r")
print(tostring(denied == nil))
print(tostring(string.find(err, "virtual path denied") ~= nil))
	`, outsidePath)
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	want := "inside\ntrue\ntrue\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestLuaFullFilesystemAccessAllowsHostPath(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	hostFile, err := os.CreateTemp(homeDir, ".invowk-lua-full-access-*")
	if err != nil {
		t.Fatalf("CreateTemp(home) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(hostFile.Name()) })
	if _, err := hostFile.WriteString("lua-full-ok"); err != nil {
		t.Fatalf("WriteString(host file) error = %v", err)
	}
	if err := hostFile.Close(); err != nil {
		t.Fatalf("Close(host file) error = %v", err)
	}

	script := fmt.Sprintf(`
local f = assert(io.open(%q, "r"))
print(f:read("*a"))
assert(f:close())
`, hostFile.Name())
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)
	ctx.SelectedImpl.Platforms = testPlatformsWithVirtualFilesystem(invowkfile.VirtualFilesystemAccessFull, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "lua-full-ok\n" {
		t.Fatalf("stdout = %q, want lua-full-ok newline", got)
	}
}

func TestLuaRequireLoadsModuleLocalFileAndBlocksTraversal(t *testing.T) {
	t.Parallel()

	script := `
local fmt = require("helpers.format")
print(fmt.upper("ok"))
local ok = pcall(function() require("../outside") end)
print(tostring(ok))
	`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)
	helpersDir := filepath.Join(string(ctx.Invowkfile.GetScriptBasePath()), "helpers")
	if err := os.MkdirAll(helpersDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(helpers) error = %v", err)
	}
	helper := `return { upper = function(value) return string.upper(value) end }`
	if err := os.WriteFile(filepath.Join(helpersDir, "format.lua"), []byte(helper), 0o644); err != nil {
		t.Fatalf("WriteFile(format.lua) error = %v", err)
	}

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "OK\nfalse\n" {
		t.Fatalf("stdout = %q, want OK false", got)
	}
}

func TestLuaArgsAvailableAsTableAndVarargs(t *testing.T) {
	t.Parallel()

	script := `
local first, second = ...
print(arg[1] .. ":" .. arg[2] .. ":" .. first .. ":" .. second)
	`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)
	ctx.PositionalArgs = []string{"one", "two"}

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "one:two:one:two\n" {
		t.Fatalf("stdout = %q, want arg and vararg values", got)
	}
}

func TestLuaBridgeVirtualUtilityCommandAndCapture(t *testing.T) {
	t.Parallel()

	script := `
local code = invowk.cmd.basename("a/b")
local out, err, captureCode = invowk.capture.basename("c/d")
out = string.gsub(out, "\n", "")
err = string.gsub(err, "\n", "")
print("cmd-code=" .. tostring(code))
print("cap=" .. out .. ":" .. err .. ":" .. tostring(captureCode))
`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(true).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	want := "b\ncmd-code=0\ncap=d::0\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestLuaBridgeUtilitiesDisabledStillAllowsExplicitHostBinary(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("shell-script executable fixture is POSIX-specific")
	}

	tmpDir := t.TempDir()
	toolPath := filepath.Join(tmpDir, "host-tool.sh")
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nprintf 'host-ok'\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(host tool) error = %v", err)
	}

	script := fmt.Sprintf(`
local out, err, code = invowk.capture(%q)
print(out .. ":" .. tostring(code))
`, toolPath)
	cfg := invowkfile.RuntimeConfig{
		Name:            invowkfile.RuntimeVirtualLua,
		AllowedBinaries: []invowkfile.AllowedBinary{invowkfile.AllowedBinary(toolPath)},
	}
	ctx, stdout, _ := newLuaExecutionContext(t, script, cfg, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "host-ok:0\n" {
		t.Fatalf("stdout = %q, want host-ok:0 newline", got)
	}
}

func TestLuaBridgeUtilitiesDisabledDeniesBuiltinWithoutHostAllow(t *testing.T) {
	t.Parallel()

	script := `
local out, err, code = invowk.capture.basename("a/b")
print(tostring(code))
`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	if got := stdout.String(); got != "126\n" {
		t.Fatalf("stdout = %q, want denied exit status 126", got)
	}
}

func TestLuaStdlibRestrictions(t *testing.T) {
	t.Parallel()

	script := `
print(type(os.getenv))
print(type(os.execute))
print(type(io.open))
print(type(io.popen))
print(type(package))
print(type(debug))
print(type(golib))
print(type(dofile))
print(type(loadfile))
local ok = pcall(function() return require("native.so") end)
print(tostring(ok))
`
	ctx, stdout, _ := newLuaExecutionContext(t, script, invowkfile.RuntimeConfig{Name: invowkfile.RuntimeVirtualLua}, nil)

	result := NewLuaRuntime(false).Execute(ctx)
	if !result.Success() {
		t.Fatalf("Execute() result = %#v, want success", result)
	}
	want := "function\nnil\nfunction\nnil\nnil\nnil\nnil\nnil\nnil\nfalse\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunLuaScriptAttachesStreamsAndVirtualPolicy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	script := `
local line = io.read("*l")
print(invowk.path("DATA/file.txt"))
print("out:" .. line .. ":" .. arg[1])
io.stderr:write("err:" .. line)
`

	err := RunLuaScript(t.Context(), LuaScriptOptions{
		Script:           script,
		ScriptName:       "interactive.lua",
		WorkDir:          tmpDir,
		ScriptBasePath:   tmpDir,
		FilesystemPaths:  invowkfile.VirtualFilesystemPaths{"DATA": invowkfile.VirtualFilesystemPath(tmpDir)},
		Args:             []string{"pos"},
		BinaryLookupMode: invowkfile.BinaryLookupModeHost,
		Stdin:            strings.NewReader("stdin-value\n"),
		Stdout:           &stdout,
		Stderr:           &stderr,
	})
	if err != nil {
		t.Fatalf("RunLuaScript() error = %v", err)
	}
	resolver := mustInteractiveVirtualTestResolver(
		t,
		tmpDir,
		tmpDir,
		invowkfile.VirtualFilesystemPaths{"DATA": invowkfile.VirtualFilesystemPath(tmpDir)},
	)
	resolvedFile := mustResolveVirtualBridgeTestPath(t, resolver, tmpDir, "DATA/file.txt")
	wantOut := resolvedFile + "\nout:stdin-value:pos\n"
	if got := stdout.String(); got != wantOut {
		t.Fatalf("stdout = %q, want %q", got, wantOut)
	}
	if got := stderr.String(); got != "err:stdin-value" {
		t.Fatalf("stderr = %q, want err:stdin-value", got)
	}
}

func newLuaExecutionContext(
	t testing.TB,
	script string,
	cfg invowkfile.RuntimeConfig,
	envVars map[invowkfile.EnvVarName]string,
) (ctx *ExecutionContext, stdout, stderr *bytes.Buffer) {
	t.Helper()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := testCommandWithScript("lua-test", script, invowkfile.RuntimeVirtualLua)
	cmd.Implementations[0].Runtimes = []invowkfile.RuntimeConfig{cfg}
	if envVars != nil {
		cmd.Env = &invowkfile.EnvConfig{Vars: envVars}
	}

	ctx = NewExecutionContext(t.Context(), cmd, inv)
	ctx.SelectedRuntime = invowkfile.RuntimeVirtualLua
	ctx.SelectedImpl = &cmd.Implementations[0]
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	ctx.IO.Stdout = stdout
	ctx.IO.Stderr = stderr
	return ctx, stdout, stderr
}
