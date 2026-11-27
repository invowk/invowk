// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
)

const noCwdTestInvowkfile = `cmds: [{name: "local", implementations: [{script: {content: "echo local"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`

type (
	// countingLoader records every config load, so tests can prove that a flow
	// was rejected before it reached config (and therefore before any LLM call).
	countingLoader struct {
		inner config.Provider
		calls atomic.Int32
	}

	// noCwdFixture is a simulated cwd with an invowkfile (command "local") and a
	// sibling module (command "modcmd"), a user commands dir with module "g"
	// (command "glob"), and a config dir whose config.cue includes module "inc"
	// (command "inc") by absolute path. Running "inc" writes incMarker.
	noCwdFixture struct {
		cwd         string
		commandsDir string
		userDir     string
		incMarker   string
	}
)

func (p *countingLoader) Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error) {
	p.calls.Add(1)
	return p.inner.Load(ctx, opts)
}

func (p *countingLoader) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	p.calls.Add(1)
	return p.inner.LoadWithSource(ctx, opts)
}

func newNoCwdFixture(t *testing.T) noCwdFixture {
	t.Helper()
	root := t.TempDir()
	f := noCwdFixture{
		cwd:         filepath.Join(root, "cwd"),
		commandsDir: filepath.Join(root, "cmds"),
		userDir:     filepath.Join(root, "config"),
		incMarker:   filepath.Join(root, "inc-ran"),
	}
	writeTestFile(t, filepath.Join(f.cwd, "invowkfile.cue"), noCwdTestInvowkfile)
	createInternalCheckCmdTestModule(t, f.cwd, "foo.invowkmod", "foo", "modcmd")
	createInternalCheckCmdTestModule(t, f.commandsDir, "g.invowkmod", "g", "glob")
	incDir := createInternalCheckCmdTestModule(t, filepath.Join(root, "inc"), "bar.invowkmod", "bar", "inc")
	// The include runs on every OS, so "inc" can be executed end to end.
	writeTestFile(t, filepath.Join(incDir, "invowkfile.cue"),
		`cmds: [{name: "inc", implementations: [{script: {content: "echo INC_RAN > '`+filepath.ToSlash(f.incMarker)+`'"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)
	writeTestFile(t, filepath.Join(f.userDir, "config.cue"), `includes: [{path: "`+filepath.ToSlash(incDir)+`"}]`)
	return f
}

func (f noCwdFixture) app(t *testing.T, env map[string]string) *configSourceTestEnv {
	t.Helper()
	return newConfigSourceTestApp(t, env, f.userDir, f.cwd, f.commandsDir)
}

func commandNames(ctx context.Context, t *testing.T, svc *commandadapters.DiscoveryService) []string {
	t.Helper()
	result, err := svc.DiscoverCommandSet(ctx)
	if err != nil {
		t.Fatalf("DiscoverCommandSet() error = %v", err)
	}
	names := make([]string, 0, len(result.Set.Commands))
	for _, c := range result.Set.Commands {
		names = append(names, string(c.Name))
	}
	return names
}

func TestNoCwdKeepsNonCwdSources(t *testing.T) {
	t.Parallel()

	f := newNoCwdFixture(t)
	env := f.app(t, nil)
	svc, ok := env.app.Discovery.(*commandadapters.DiscoveryService)
	if !ok {
		t.Fatalf("Discovery = %T, want *commandadapters.DiscoveryService", env.app.Discovery)
	}

	withCwd := strings.Join(commandNames(contextWithDiscoveryScope(t.Context(), "", false), t, svc), ",")
	for _, want := range []string{"local", "foo modcmd", "bar inc", "g glob"} {
		if !strings.Contains(withCwd, want) {
			t.Fatalf("without --ivk-no-cwd commands = %s, want %q", withCwd, want)
		}
	}

	noCwd := commandNames(contextWithDiscoveryScope(t.Context(), "", true), t, svc)
	joined := strings.Join(noCwd, ",")
	for _, gone := range []string{"local", "foo modcmd"} {
		for _, name := range noCwd {
			if name == gone {
				t.Fatalf("--ivk-no-cwd still discovers cwd command %q: %s", gone, joined)
			}
		}
	}
	for _, want := range []string{"bar inc", "g glob"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("--ivk-no-cwd commands = %s, want %q (includes and user commands dir stay)", joined, want)
		}
	}
}

// TestRescopeRegisteredCommand covers the leaf whose registration-time switch
// differs from the parsed one: under the switch the cwd command is gone (so the
// service reports it as not found), while a user commands dir command is found
// again with its own definitions.
func TestRescopeRegisteredCommand(t *testing.T) {
	t.Parallel()

	env := newNoCwdFixture(t).app(t, nil)
	noCwdCtx := contextWithDiscoveryScope(t.Context(), "", true)
	if got := rescopeRegisteredCommand(noCwdCtx, env.app, "local"); got != nil {
		t.Fatalf("rescope(local) under --ivk-no-cwd = %s from %s, want nil", got.Name, got.FilePath)
	}
	if got := rescopeRegisteredCommand(noCwdCtx, env.app, "g glob"); got == nil || got.Name != "g glob" {
		t.Fatalf("rescope(g glob) under --ivk-no-cwd = %v, want the user commands dir command", got)
	}
	if rescopeRegisteredCommand(contextWithDiscoveryScope(t.Context(), "", false), env.app, "local") == nil {
		t.Fatal("rescope(local) without the switch = nil, want the cwd command")
	}
}

func TestNoCwdCacheIsolation(t *testing.T) {
	t.Parallel()

	f := newNoCwdFixture(t)
	env := f.app(t, nil)
	svc, ok := env.app.Discovery.(*commandadapters.DiscoveryService)
	if !ok {
		t.Fatalf("Discovery = %T", env.app.Discovery)
	}

	// Prime a cwd-scoped request cache, then derive a no-cwd context from it.
	cwdCtx := contextWithDiscoveryScope(t.Context(), "", false)
	if lookup, err := svc.GetCommand(cwdCtx, "local"); err != nil || lookup.Command == nil {
		t.Fatalf("cwd lookup = (%v, %v), want local", lookup.Command, err)
	}
	noCwdCtx := contextWithDiscoveryScope(cwdCtx, "", true)
	lookup, err := svc.GetCommand(noCwdCtx, "local")
	if err != nil {
		t.Fatalf("no-cwd lookup error = %v", err)
	}
	if lookup.Command != nil {
		t.Fatal("no-cwd context was served the cached cwd command")
	}
	for _, name := range commandNames(noCwdCtx, t, svc) {
		if name == "local" {
			t.Fatal("no-cwd command set contains the cached cwd command")
		}
	}
}

func TestNoCwdFlows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        map[string]string
		args       []string
		wantErr    bool
		wantOut    []string
		notWantOut []string
	}{
		{name: "listing hides cwd commands", args: []string{"--ivk-no-cwd", "cmd"}, wantOut: []string{"glob", "inc"}, notWantOut: []string{"local", "modcmd"}},
		{name: "env hides cwd commands", env: map[string]string{envNoCwd: "1"}, args: []string{"cmd"}, wantOut: []string{"glob"}, notWantOut: []string{"local"}},
		{name: "flag false beats env", env: map[string]string{envNoCwd: "1"}, args: []string{"--ivk-no-cwd=false", "cmd"}, wantOut: []string{"local"}},
		{name: "flag skips invalid env", env: map[string]string{envNoCwd: "yes"}, args: []string{"--ivk-no-cwd", "cmd"}, wantOut: []string{"glob"}},
		{name: "run cwd command fails", args: []string{"--ivk-no-cwd", "cmd", "local"}, wantErr: true},
		{name: "dry-run cwd command fails", args: []string{"--ivk-no-cwd", "cmd", "--ivk-dry-run", "local"}, wantErr: true},
		{name: "source invowkfile not found", args: []string{"--ivk-no-cwd", "cmd", "@invowkfile", "local"}, wantErr: true},
		{name: "from invowkfile not found", args: []string{"--ivk-no-cwd", "cmd", "--ivk-from", "invowkfile", "local"}, wantErr: true},
		{name: "configured include dry-run", args: []string{"--ivk-no-cwd", "cmd", "--ivk-dry-run", "bar", "inc"}, wantOut: []string{"INC_RAN"}},
		{name: "watch cwd command fails", args: []string{"--ivk-no-cwd", "cmd", "--ivk-watch", "local"}, wantErr: true},
		{name: "invalid env fails cmd", env: map[string]string{envNoCwd: "yes"}, args: []string{"cmd"}, wantErr: true},
		{name: "invalid env fails validate", env: map[string]string{envNoCwd: "yes"}, args: []string{"validate"}, wantErr: true},
		{name: "invalid env fails module list", env: map[string]string{envNoCwd: "yes"}, args: []string{"module", "list"}, wantErr: true},
		{name: "module list omits cwd module", args: []string{"--ivk-no-cwd", "module", "list"}, wantOut: []string{"g.invowkmod", "bar.invowkmod"}, notWantOut: []string{"foo.invowkmod"}},
		{name: "validate skips cwd sources", args: []string{"--ivk-no-cwd", "validate"}, wantOut: []string{"2 source(s) discovered", "current-directory sources disabled (--ivk-no-cwd)"}},
		{name: "validate names env origin", env: map[string]string{envNoCwd: "1"}, args: []string{"validate"}, wantOut: []string{"current-directory sources disabled (INVOWK_NO_CWD)"}},
		{name: "validate without switch", args: []string{"validate"}, wantOut: []string{"4 source(s) discovered"}, notWantOut: []string{"current-directory sources disabled"}},
		{name: "verbose listing names switch", args: []string{"--ivk-verbose", "--ivk-no-cwd", "cmd"}, wantOut: []string{"current-directory sources disabled (--ivk-no-cwd)"}},
		{name: "non-verbose listing has no switch line", args: []string{"--ivk-no-cwd", "cmd"}, notWantOut: []string{"current-directory sources disabled"}},
		{name: "invalid env ignored by config path", env: map[string]string{envNoCwd: "yes"}, args: []string{"config", "path"}, wantOut: []string{"Config file:"}},
		{name: "internal check-cmd is scoped", args: []string{"--ivk-no-cwd", "internal", "check-cmd", "local"}, wantErr: true},
		{name: "internal check-cmd keeps user dir", args: []string{"--ivk-no-cwd", "internal", "check-cmd", "g glob"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newNoCwdFixture(t).app(t, tt.env)
			err := env.run(t, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run %v error = %v, wantErr %v\nstdout: %s\nstderr: %s", tt.args, err, tt.wantErr, env.stdout, env.stderr)
			}
			out := env.stdout.String()
			for _, want := range tt.wantOut {
				if !strings.Contains(out, want) {
					t.Fatalf("stdout missing %q:\n%s", want, out)
				}
			}
			for _, unwanted := range tt.notWantOut {
				if strings.Contains(out, unwanted) {
					t.Fatalf("stdout contains %q:\n%s", unwanted, out)
				}
			}
		})
	}
}

// TestNoCwdRunsConfiguredInclude executes a configured include under the
// switch (not a dry run): the script's marker file proves it ran.
func TestNoCwdRunsConfiguredInclude(t *testing.T) {
	t.Parallel()

	f := newNoCwdFixture(t)
	env := f.app(t, nil)
	if err := env.run(t, "--ivk-no-cwd", "cmd", "bar", "inc"); err != nil {
		t.Fatalf("run configured include under --ivk-no-cwd: %v\nstderr: %s", err, env.stderr)
	}
	if _, err := os.Stat(f.incMarker); err != nil {
		t.Fatalf("configured include did not run: %v", err)
	}
}

func TestNoCwdCompletionOmitsCwdCommands(t *testing.T) {
	t.Parallel()

	env := newNoCwdFixture(t).app(t, map[string]string{envNoCwd: "1"})
	rootCmd := NewRootCommand(env.app)
	cmdCmd, _, err := rootCmd.Find([]string{"cmd"})
	if err != nil {
		t.Fatalf("Find(cmd) error = %v", err)
	}
	cmdCmd.SetContext(t.Context())
	completions, _ := completeCommands(env.app, &rootFlagValues{})(cmdCmd, nil, "")
	joined := strings.Join(completions, ",")
	if strings.Contains(joined, "local") || strings.Contains(joined, "foo") {
		t.Fatalf("completions = %s, want no cwd commands", joined)
	}
	if !strings.Contains(joined, "g") {
		t.Fatalf("completions = %s, want the user commands dir module", joined)
	}
}

func TestAuditHonoursInjectedConfigEnv(t *testing.T) {
	t.Parallel()

	f := newNoCwdFixture(t)
	missing := filepath.Join(t.TempDir(), "missing.cue")
	env := newConfigSourceTestApp(t, map[string]string{config.EnvConfigPath: missing}, f.userDir, f.cwd, f.commandsDir)
	err := env.run(t, "audit", f.cwd)
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok || exitErr.Code != auditExitError {
		t.Fatalf("audit error = %v, want ExitError code %d (audit error, not findings)", err, auditExitError)
	}
	if !strings.Contains(env.stderr.String()+err.Error(), config.EnvConfigPath) {
		t.Fatalf("audit error does not name %s: %v\n%s", config.EnvConfigPath, err, env.stderr)
	}

	// A includes two modules, while the user config (f.userDir) includes one
	// ("bar") and defaults include none.
	envDir := t.TempDir()
	modA := createInternalCheckCmdTestModule(t, envDir, "auda.invowkmod", "auda", "a")
	modB := createInternalCheckCmdTestModule(t, envDir, "audb.invowkmod", "audb", "b")
	valid := writeTestFile(t, filepath.Join(envDir, "env.cue"),
		`includes: [{path: "`+filepath.ToSlash(modA)+`"}, {path: "`+filepath.ToSlash(modB)+`"}]`)
	target := t.TempDir()
	okEnv := newConfigSourceTestApp(t, map[string]string{config.EnvConfigPath: valid}, f.userDir, f.cwd, f.commandsDir)
	err = okEnv.run(t, "audit", target)
	if exitErr, isExit := errors.AsType[*ExitError](err); isExit && exitErr.Code == auditExitError {
		t.Fatalf("audit with a valid INVOWK_CONFIG failed as a scan error: %v\n%s", err, okEnv.stderr)
	}
	// The near-identical IDs also trigger the typosquatting check, which names
	// both included modules, so the findings show which config audit used.
	out := okEnv.stdout.String()
	for _, want := range []string{"Scanned: 2 module(s)", "auda.invowkmod", `"audb"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit output missing %q, so it did not scan INVOWK_CONFIG's includes:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bar.invowkmod") {
		t.Fatalf("audit scanned the user config's include despite INVOWK_CONFIG:\n%s", out)
	}
}

func TestAgentVerifyRejectedUnderNoCwd(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		env  map[string]string
		args []string
	}{
		{name: "create flag", args: []string{"--ivk-no-cwd", "agent", "cmd", "create", "--verify", "hello", "say hello"}},
		{name: "change env", env: map[string]string{envNoCwd: "1"}, args: []string{"agent", "cmd", "change", "--verify", "hello", "say hi"}},
		{name: "create invalid env", env: map[string]string{envNoCwd: "yes"}, args: []string{"agent", "cmd", "create", "--verify", "hello", "say hello"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newNoCwdFixture(t)
			env := f.app(t, tt.env)
			loader := &countingLoader{inner: env.app.Config}
			env.app.Config = loader
			before, readErr := os.ReadFile(filepath.Join(f.cwd, "invowkfile.cue"))
			if readErr != nil {
				t.Fatalf("read invowkfile: %v", readErr)
			}

			err := env.run(t, tt.args...)
			if err == nil {
				t.Fatal("agent --verify under the switch succeeded, want rejection")
			}
			// The conflict comes from argv and env alone, so it is a usage error.
			if got := exitCodeForError(err); got != ExitCodeUsage {
				t.Fatalf("exit code = %d (error: %v), want %d", got, err, ExitCodeUsage)
			}
			if tt.env[envNoCwd] == "yes" {
				if !errors.Is(err, ErrInvalidNoCwdEnv) {
					t.Fatalf("error = %v, want ErrInvalidNoCwdEnv", err)
				}
			} else if !errors.Is(err, ErrVerifyNeedsCwd) || !strings.Contains(err.Error(), "--verify") || !strings.Contains(err.Error(), "--ivk-no-cwd") {
				t.Fatalf("error = %v, want ErrVerifyNeedsCwd naming --verify and --ivk-no-cwd", err)
			}
			if calls := loader.calls.Load(); calls != 0 {
				t.Fatalf("config loaded %d times; the LLM completer must not be resolved", calls)
			}
			after, readErr := os.ReadFile(filepath.Join(f.cwd, "invowkfile.cue"))
			if readErr != nil || !bytes.Equal(after, before) {
				t.Fatalf("invowkfile changed: %v", readErr)
			}
		})
	}
}

func TestAgentFailsOnMissingExplicitConfig(t *testing.T) {
	t.Parallel()

	f := newNoCwdFixture(t)
	env := f.app(t, nil)
	before, err := os.ReadFile(filepath.Join(f.cwd, "invowkfile.cue"))
	if err != nil {
		t.Fatalf("read invowkfile: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "missing.cue")
	runErr := env.run(t, "--ivk-config", missing, "agent", "cmd", "create", "--file", filepath.Join(f.cwd, "invowkfile.cue"), "hello", "say hello")
	// A failed explicit config source is an internal failure, never usage.
	if got := exitCodeForError(runErr); got != 1 {
		t.Fatalf("agent error = %v, exit code %d, want 1 from LLM config resolution", runErr, got)
	}
	if !errors.Is(runErr, config.ErrExplicitConfig) {
		t.Fatalf("agent error = %v, want ErrExplicitConfig", runErr)
	}
	after, err := os.ReadFile(filepath.Join(f.cwd, "invowkfile.cue"))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("invowkfile changed after a failed explicit config: %v", err)
	}
}
