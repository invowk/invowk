// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCwdFailClosed_BlocksGlobalShadowingAndAllowsNoCwdBypass(t *testing.T) {
	t.Parallel()

	t.Run("broken cwd invowkfile blocks global command with exit 1", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		cwd := filepath.Join(root, "cwd")
		commandsDir := filepath.Join(root, "cmds")
		userDir := filepath.Join(root, "config")
		marker := filepath.Join(root, "global-ran")

		writeTestFile(t, filepath.Join(cwd, "invowkfile.cue"), `cmds: [{
	name: "build"
	bogusfield: 1
	implementations: [{script: {content: "echo local"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
}]`)
		globalMod := createInternalCheckCmdTestModule(t, commandsDir, "tools.invowkmod", "tools", "build")
		writeTestFile(t, filepath.Join(globalMod, "invowkfile.cue"),
			`cmds: [{name: "build", implementations: [{script: {content: "echo GLOBAL > '`+filepath.ToSlash(marker)+`'"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)

		env := newConfigSourceTestApp(t, nil, userDir, cwd, commandsDir)
		err := env.run(t, "cmd", "build")
		if err == nil {
			t.Fatal("expected fail-closed error when cwd invowkfile is invalid")
		}
		if got := exitCodeForError(err); got != 1 {
			t.Fatalf("exitCodeForError() = %d, want 1 (err=%v)", got, err)
		}
		combined := err.Error() + env.stderr.String()
		if !strings.Contains(combined, "field not allowed") || !strings.Contains(combined, "hint: this file may require a newer version of invowk") {
			t.Fatalf("expected schema error + upgrade hint, got %q", combined)
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Fatal("global command ran despite broken cwd invowkfile")
		}

		noCwdEnv := newConfigSourceTestApp(t, nil, userDir, cwd, commandsDir)
		if err := noCwdEnv.run(t, "--ivk-no-cwd", "cmd", "build"); err != nil {
			t.Fatalf("--ivk-no-cwd cmd build failed: %v\nstderr: %s", err, noCwdEnv.stderr)
		}
		if _, statErr := os.Stat(marker); statErr != nil {
			t.Fatalf("global command did not run under --ivk-no-cwd: %v", statErr)
		}
	})

	t.Run("broken cwd module invowkmod.cue blocks global command and watch mode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		cwd := filepath.Join(root, "cwd")
		commandsDir := filepath.Join(root, "cmds")
		userDir := filepath.Join(root, "config")
		marker := filepath.Join(root, "global-cst-ran")

		brokenMod := filepath.Join(cwd, "cst.invowkmod")
		writeTestFile(t, filepath.Join(brokenMod, "invowkmod.cue"), "module: \"cst\"\nversion: \"1.0.0\"\nbogusfield: true\n")
		writeTestFile(t, filepath.Join(brokenMod, "invowkfile.cue"),
			`cmds: [{name: "doctor", implementations: [{script: {content: "echo local"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)

		globalMod := createInternalCheckCmdTestModule(t, commandsDir, "cst.invowkmod", "cst", "doctor")
		writeTestFile(t, filepath.Join(globalMod, "invowkfile.cue"),
			`cmds: [{name: "doctor", implementations: [{script: {content: "echo GLOBAL > '`+filepath.ToSlash(marker)+`'"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)

		for _, args := range [][]string{
			{"cmd"},
			{"cmd", "cst", "doctor"},
			{"cmd", "@cst", "doctor"},
			{"cmd", "--ivk-watch", "cst", "doctor"},
		} {
			env := newConfigSourceTestApp(t, nil, userDir, cwd, commandsDir)
			err := env.run(t, args...)
			if err == nil {
				t.Fatalf("args %v succeeded, want fail-closed error", args)
			}
			if got := exitCodeForError(err); got != 1 {
				t.Fatalf("args %v exit code = %d, want 1 (err=%v)", args, got, err)
			}
			if !strings.Contains(err.Error()+env.stderr.String(), "field not allowed") {
				t.Fatalf("args %v error = %q, want field not allowed", args, err.Error()+env.stderr.String())
			}
			if _, statErr := os.Stat(marker); statErr == nil {
				t.Fatalf("args %v executed global command despite broken cwd module", args)
			}
		}

		noCwdEnv := newConfigSourceTestApp(t, nil, userDir, cwd, commandsDir)
		if err := noCwdEnv.run(t, "--ivk-no-cwd", "cmd", "cst", "doctor"); err != nil {
			t.Fatalf("--ivk-no-cwd cmd cst doctor failed: %v\nstderr: %s", err, noCwdEnv.stderr)
		}
		if _, statErr := os.Stat(marker); statErr != nil {
			t.Fatalf("global cst doctor did not run under --ivk-no-cwd: %v", statErr)
		}
	})
}
