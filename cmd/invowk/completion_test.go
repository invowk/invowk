// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
)

func TestCompletionValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid flags return usage error exit 2", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			args    []string
			wantErr error
		}{
			{
				name:    "empty alias flag",
				args:    []string{"completion", "bash", "--alias", ""},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias with shell injection",
				args:    []string{"completion", "bash", "--alias", "cst; rm -rf /"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias starting with dash",
				args:    []string{"completion", "zsh", "--alias", "-cst"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias starting with dot",
				args:    []string{"completion", "fish", "--alias", ".cst"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias containing slash",
				args:    []string{"completion", "powershell", "--alias", "bin/cst"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias containing newline",
				args:    []string{"completion", "bash", "--alias", "cst\n"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "alias containing NUL byte",
				args:    []string{"completion", "bash", "--alias", "cst\x00"},
				wantErr: errInvalidCompletionAlias,
			},
			{
				name:    "prefix without alias",
				args:    []string{"completion", "bash", "--prefix", "cmd cst"},
				wantErr: errCompletionPrefixWithoutAlias,
			},
			{
				name:    "empty prefix flag without alias",
				args:    []string{"completion", "bash", "--prefix", ""},
				wantErr: errCompletionPrefixWithoutAlias,
			},
			{
				name:    "prefix with NUL byte",
				args:    []string{"completion", "bash", "--alias", "cst", "--prefix", "cmd\x00cst"},
				wantErr: errInvalidCompletionPrefix,
			},
			{
				name:    "prefix with newline",
				args:    []string{"completion", "zsh", "--alias", "cst", "--prefix", "cmd\ncst"},
				wantErr: errInvalidCompletionPrefix,
			},
			{
				name:    "prefix with carriage return",
				args:    []string{"completion", "fish", "--alias", "cst", "--prefix", "cmd\rcst"},
				wantErr: errInvalidCompletionPrefix,
			},
			{
				name:    "prefix with unclosed single quote",
				args:    []string{"completion", "bash", "--alias", "cst", "--prefix", "-c '/etc/cst/config.cue cmd cst"},
				wantErr: errInvalidCompletionPrefix,
			},
			{
				name:    "prefix with unclosed double quote",
				args:    []string{"completion", "powershell", "--alias", "cst", "--prefix", `-c "/etc/cst/config.cue cmd cst`},
				wantErr: errInvalidCompletionPrefix,
			},
			{
				name:    "prefix with trailing backslash",
				args:    []string{"completion", "bash", "--alias", "cst", "--prefix", `cmd cst\`},
				wantErr: errInvalidCompletionPrefix,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var stdout, stderr bytes.Buffer
				app, err := NewApp(Dependencies{
					Config: &staticConfigProvider{cfg: config.DefaultConfig()},
					Stdout: &stdout,
					Stderr: &stderr,
					Getenv: func(string) string { return "" },
				})
				if err != nil {
					t.Fatalf("NewApp() error = %v", err)
				}

				rootCmd := NewRootCommand(app)
				rootCmd.SetOut(&stdout)
				rootCmd.SetErr(&stderr)
				rootCmd.SetArgs(tt.args)

				runErr := rootCmd.ExecuteContext(t.Context())
				if runErr == nil {
					t.Fatalf("ExecuteContext(%v) = nil, want UsageError", tt.args)
				}
				if got := exitCodeForError(runErr); got != ExitCodeUsage {
					t.Errorf("exitCodeForError(%v) = %d, want %d", runErr, got, ExitCodeUsage)
				}
				if !errors.Is(runErr, tt.wantErr) {
					t.Errorf("errors.Is(%v, %v) = false, want true", runErr, tt.wantErr)
				}
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want empty on validation failure", stdout.String())
				}
			})
		}
	})

	t.Run("parseCompletionPrefix handles quoting and spacing", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			raw  string
			want completionPrefixTokens
		}{
			{
				name: "empty",
				raw:  "",
				want: nil,
			},
			{
				name: "whitespace only",
				raw:  "   \t  ",
				want: nil,
			},
			{
				name: "scoped invowk prefix",
				raw:  "-c /etc/cst/config.cue --ivk-no-cwd cmd cst",
				want: completionPrefixTokens{"-c", "/etc/cst/config.cue", "--ivk-no-cwd", "cmd", "cst"},
			},
			{
				name: "single and double quoted tokens",
				raw:  `-c '/tmp/my dir/config.cue' --ivk-from "my mod" cmd`,
				want: completionPrefixTokens{"-c", "/tmp/my dir/config.cue", "--ivk-from", "my mod", "cmd"},
			},
			{
				name: "windows path inside double quotes preserves backslashes",
				raw:  `-c "C:\Program Files\cst\config.cue" cmd cst`,
				want: completionPrefixTokens{"-c", `C:\Program Files\cst\config.cue`, "cmd", "cst"},
			},
			{
				name: "escaped space outside quotes",
				raw:  `-c /tmp/my\ dir/config.cue cmd`,
				want: completionPrefixTokens{"-c", "/tmp/my dir/config.cue", "cmd"},
			},
			{
				name: "empty quoted token is preserved",
				raw:  `cmd "" cst`,
				want: completionPrefixTokens{"cmd", "", "cst"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := parseCompletionPrefix(tt.raw)
				if err != nil {
					t.Fatalf("parseCompletionPrefix(%q) error = %v", tt.raw, err)
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("parseCompletionPrefix(%q) = %#v, want %#v", tt.raw, got, tt.want)
				}
			})
		}
	})
}

func TestCompletionAlias(t *testing.T) {
	t.Parallel()

	runCompletionCLI := func(t *testing.T, args ...string) string {
		t.Helper()

		var stdout, stderr bytes.Buffer
		app, err := NewApp(Dependencies{
			Config: &staticConfigProvider{cfg: config.DefaultConfig()},
			Stdout: &stdout,
			Stderr: &stderr,
			Getenv: func(string) string { return "" },
		})
		if err != nil {
			t.Fatalf("NewApp() error = %v", err)
		}

		rootCmd := NewRootCommand(app)
		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs(append([]string{"completion"}, args...))
		if err := rootCmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("ExecuteContext(%v) error = %v (stderr: %s)", args, err, stderr.String())
		}
		return stdout.String()
	}

	t.Run("all shells rewrite identifiers and prefix cleanly", func(t *testing.T) {
		t.Parallel()

		const prefix = "-c /etc/cst/config.cue --ivk-no-cwd cmd cst"
		tests := []struct {
			shell       string
			mustContain []string
			mustExclude []string
		}{
			{
				shell: "bash",
				mustContain: []string{
					"# bash completion V2 for cst",
					"__cst_debug()",
					"__start_cst()",
					"complete -o default -F __start_cst cst",
					"complete -o default -o nospace -F __start_cst cst",
					`printf -v prefixComp '%q ' '-c' '/etc/cst/config.cue' '--ivk-no-cwd' 'cmd' 'cst'`,
					`requestComp="invowk __complete ${prefixComp}${args[*]}"`,
				},
				mustExclude: []string{
					"__invowk_",
					"__start_invowk",
					"__start_cst invowk",
					`${words[0]} __complete`,
				},
			},
			{
				shell: "zsh",
				mustContain: []string{
					"#compdef cst",
					"compdef _cst cst",
					"# zsh completion for cst",
					"__cst_debug()",
					"_cst()",
					`if [ "$funcstack[1]" = "_cst" ]; then`,
					`prefixArgs=( '-c' '/etc/cst/config.cue' '--ivk-no-cwd' 'cmd' 'cst' )`,
					`requestComp="invowk __complete ${(j: :)${(@q)prefixArgs}} ${words[2,-1]}"`,
				},
				mustExclude: []string{
					"#compdef invowk",
					"compdef _invowk",
					"__invowk_",
					"_invowk()",
					`[ "$funcstack[1]" = "_invowk" ]`,
					`${words[1]} __complete`,
				},
			},
			{
				shell: "fish",
				mustContain: []string{
					"# fish completion for cst",
					"function __cst_debug",
					`if type -q "cst"`,
					`complete --do-complete "cst "`,
					"complete -c cst -e",
					`set -l prefixArgs (string escape -- '-c' '/etc/cst/config.cue' '--ivk-no-cwd' 'cmd' 'cst')`,
					`set -l requestComp "INVOWK_ACTIVE_HELP=0 invowk __complete $prefixArgs $args[2..-1] $lastArg"`,
				},
				mustExclude: []string{
					"__invowk_",
					`type -q "invowk"`,
					"complete -c invowk",
					"complete -k -c invowk",
					"$args[1] __complete",
				},
			},
			{
				shell: "powershell",
				mustContain: []string{
					"# powershell completion for cst",
					"function __cst_debug",
					"filter __cst_escapeStringWithSpecialChars",
					"[scriptblock]${__cstCompleterBlock}",
					"Register-ArgumentCompleter -CommandName 'cst' -ScriptBlock ${__cstCompleterBlock}",
					`${env:INVOWK_ACTIVE_HELP}=0`,
					`$PrefixComp = @('-c', '/etc/cst/config.cue', '--ivk-no-cwd', 'cmd', 'cst')`,
					`$RequestComp="invowk __complete $($PrefixComp -join ' ') $Arguments"`,
				},
				mustExclude: []string{
					"__invowk_",
					"${__invowkCompleterBlock}",
					"Register-ArgumentCompleter -CommandName 'invowk'",
					"$Program __complete",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.shell, func(t *testing.T) {
				t.Parallel()

				out := runCompletionCLI(t, tt.shell, "--alias", "cst", "--prefix", prefix)
				for _, want := range tt.mustContain {
					if !strings.Contains(out, want) {
						t.Errorf("%s completion missing %q\noutput:\n%s", tt.shell, want, out)
					}
				}
				for _, unwanted := range tt.mustExclude {
					if strings.Contains(out, unwanted) {
						t.Errorf("%s completion still contains un-namespaced %q", tt.shell, unwanted)
					}
				}
			})
		}
	})

	t.Run("alias without prefix emits direct invowk probe", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			shell string
			want  string
		}{
			{shell: "bash", want: `requestComp="invowk __complete ${args[*]}"`},
			{shell: "zsh", want: `requestComp="invowk __complete ${words[2,-1]}"`},
			{shell: "fish", want: `set -l requestComp "INVOWK_ACTIVE_HELP=0 invowk __complete $args[2..-1] $lastArg"`},
			{shell: "powershell", want: `$RequestComp="invowk __complete $Arguments"`},
		} {
			out := runCompletionCLI(t, tc.shell, "--alias", "my.cli-2")
			if !strings.Contains(out, tc.want) {
				t.Errorf("%s (--alias only) missing %q", tc.shell, tc.want)
			}
			if !strings.Contains(out, "my_cli_2") {
				t.Errorf("%s (--alias only) did not sanitize identifier to my_cli_2", tc.shell)
			}
		}
	})

	t.Run("special characters in prefix are shell-escaped", func(t *testing.T) {
		t.Parallel()

		const dangerousPrefix = `-c '/tmp/my dir/$FOO;id'\''x/cfg.cue' cmd cst`

		bashOut := runCompletionCLI(t, "bash", "--alias", "cst", "--prefix", dangerousPrefix)
		if !strings.Contains(bashOut, `'/tmp/my dir/$FOO;id'\''x/cfg.cue'`) {
			t.Errorf("bash escaping mismatch:\n%s", bashOut)
		}

		fishOut := runCompletionCLI(t, "fish", "--alias", "cst", "--prefix", `-c 'C:\dir\$FOO'\''x\cfg.cue' cmd`)
		if !strings.Contains(fishOut, `'C:\\dir\\$FOO\'x\\cfg.cue'`) {
			t.Errorf("fish escaping mismatch:\n%s", fishOut)
		}

		psOut := runCompletionCLI(t, "powershell", "--alias", "cst", "--prefix", dangerousPrefix)
		if !strings.Contains(psOut, `'/tmp/my dir/$FOO;id''x/cfg.cue'`) {
			t.Errorf("powershell escaping mismatch:\n%s", psOut)
		}
	})

	t.Run("live bash evaluation preserves prefix tokens and avoids collisions", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("bash"); err != nil {
			t.Skip("bash not available on PATH")
		}

		baseScript := runCompletionCLI(t, "bash")
		aliasScript := runCompletionCLI(t, "bash", "--alias", "my.cli-2", "--prefix", `-c '/tmp/my dir/$UNSET;echo pwned'\''x/cfg.cue' --ivk-no-cwd cmd cst`)

		tmpDir := t.TempDir()
		baseFile := filepath.Join(tmpDir, "invowk.bash")
		aliasFile := filepath.Join(tmpDir, "cst.bash")
		if err := os.WriteFile(baseFile, []byte(baseScript), 0o600); err != nil {
			t.Fatalf("write base bash script: %v", err)
		}
		if err := os.WriteFile(aliasFile, []byte(aliasScript), 0o600); err != nil {
			t.Fatalf("write alias bash script: %v", err)
		}

		syntaxCmd := exec.CommandContext(t.Context(), "bash", "-n", aliasFile)
		if out, err := syntaxCmd.CombinedOutput(); err != nil {
			t.Fatalf("bash -n failed: %v\n%s", err, out)
		}

		harness := `
set -euo pipefail
source "$1"
source "$2"
complete -p invowk | grep -q "__start_invowk"
complete -p my.cli-2 | grep -q "__start_my_cli_2"
invowk() {
    for a in "$@"; do
        printf 'ARG:%s\n' "$a"
    done
    printf ':4\n'
}
words=("my.cli-2" "sub" "")
cur=""
__my_cli_2_get_completion_results
printf 'OUT:%s\n' "$out"
`
		runCmd := exec.CommandContext(t.Context(), "bash", "-c", harness, "bash", baseFile, aliasFile)
		gotBytes, err := runCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bash harness failed: %v\n%s", err, gotBytes)
		}
		got := string(gotBytes)
		for _, want := range []string{
			"ARG:__complete",
			"ARG:-c",
			"ARG:/tmp/my dir/$UNSET;echo pwned'x/cfg.cue",
			"ARG:--ivk-no-cwd",
			"ARG:cmd",
			"ARG:cst",
			"ARG:sub",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("live bash probe output missing %q\ngot:\n%s", want, got)
			}
		}
	})

	t.Run("live powershell syntax and non-colliding registration", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("pwsh"); err != nil {
			t.Skip("pwsh not available on PATH")
		}
		if out, err := exec.CommandContext(t.Context(), "pwsh", "-NoProfile", "-NonInteractive", "-Command", "Write-Output PWSH_READY").CombinedOutput(); err != nil || !strings.Contains(string(out), "PWSH_READY") {
			t.Skipf("pwsh unusable in environment: %v (%s)", err, strings.TrimSpace(string(out)))
		}

		baseScript := runCompletionCLI(t, "powershell")
		aliasScript := runCompletionCLI(t, "powershell", "--alias", "my.cli-2", "--prefix", `-c '/tmp/my dir/$HOME;id'\''x/cfg.cue' --ivk-no-cwd cmd cst`)

		tmpDir := t.TempDir()
		baseFile := filepath.Join(tmpDir, "invowk.ps1")
		aliasFile := filepath.Join(tmpDir, "cst.ps1")
		checkFile := filepath.Join(tmpDir, "check.ps1")
		if err := os.WriteFile(baseFile, []byte(baseScript), 0o600); err != nil {
			t.Fatalf("write base ps1: %v", err)
		}
		if err := os.WriteFile(aliasFile, []byte(aliasScript), 0o600); err != nil {
			t.Fatalf("write alias ps1: %v", err)
		}

		psHarness := `
param([string]$BasePath, [string]$AliasPath)
$errors = $null
$tokens = $null
[void][System.Management.Automation.Language.Parser]::ParseFile($AliasPath, [ref]$tokens, [ref]$errors)
if ($errors.Count -ne 0) {
    $errors | ForEach-Object { Write-Error $_.Message }
    exit 1
}
. $BasePath
. $AliasPath
if (-not (Test-Path function:__invowk_debug)) { Write-Error "missing __invowk_debug"; exit 1 }
if (-not (Test-Path function:__my_cli_2_debug)) { Write-Error "missing __my_cli_2_debug"; exit 1 }
if (-not $__invowkCompleterBlock) { Write-Error "missing __invowkCompleterBlock"; exit 1 }
if (-not $__my_cli_2CompleterBlock) { Write-Error "missing __my_cli_2CompleterBlock"; exit 1 }
Write-Output "PWSH_OK"
`
		if err := os.WriteFile(checkFile, []byte(psHarness), 0o600); err != nil {
			t.Fatalf("write check.ps1: %v", err)
		}
		psCmd := exec.CommandContext(t.Context(), "pwsh", "-NoProfile", "-NonInteractive", "-File", checkFile, baseFile, aliasFile)
		gotBytes, err := psCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pwsh harness failed: %v\n%s", err, gotBytes)
		}
		if !strings.Contains(string(gotBytes), "PWSH_OK") {
			t.Fatalf("pwsh harness did not emit PWSH_OK:\n%s", gotBytes)
		}
	})

	t.Run("scoped __complete probe discovers target module without cwd", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		cwdDir := filepath.Join(root, "cwd")
		if err := os.MkdirAll(cwdDir, 0o755); err != nil {
			t.Fatalf("mkdir cwd: %v", err)
		}
		// Broken cwd invowkfile must be ignored when --ivk-no-cwd is passed.
		writeTestFile(t, filepath.Join(cwdDir, "invowkfile.cue"), `invalid cue syntax {{{`)

		modDir := createInternalCheckCmdTestModule(t, filepath.Join(root, "modules"), "cst.invowkmod", "cst", "build")
		writeTestFile(t, filepath.Join(modDir, "invowkfile.cue"), `cmds: [
	{name: "build", description: "Build cst", implementations: [{script: {content: "echo build"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]},
	{name: "test unit", description: "Unit cst", implementations: [{script: {content: "echo unit"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]},
]`)
		cfgDir := filepath.Join(root, "cfg")
		cfgPath := filepath.Join(cfgDir, "config.cue")
		writeTestFile(t, cfgPath, `includes: [{path: "`+filepath.ToSlash(modDir)+`"}]`)

		env := newConfigSourceTestApp(t, nil, cfgDir, cwdDir, filepath.Join(root, "empty-cmds"))
		if err := env.run(t, "__complete", "-c", cfgPath, "--ivk-no-cwd", "cmd", "cst", ""); err != nil {
			t.Fatalf("__complete failed: %v (stderr: %s)", err, env.stderr.String())
		}

		got := env.stdout.String()
		for _, want := range []string{"build\tBuild cst", "test", ":4"} {
			if !strings.Contains(got, want) {
				t.Errorf("__complete output missing %q\ngot:\n%s", want, got)
			}
		}
	})
}
