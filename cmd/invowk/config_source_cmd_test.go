// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

type configSourceTestEnv struct {
	app    *App
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// newConfigSourceTestApp builds an App whose config provider, env and
// discovery roots are all injected: env replaces the process environment,
// userDir is the user config directory, cwd stands in for the current
// directory and commandsDir for ~/.invowk/cmds ("" disables it).
func newConfigSourceTestApp(t *testing.T, env map[string]string, userDir, cwd, commandsDir string) *configSourceTestEnv {
	t.Helper()
	getenv := envGetter(env)
	provider := &configDirProvider{inner: config.NewProviderWithEnv(getenv), dir: userDir}
	discoverySvc, err := commandadapters.NewDiscoveryServiceWithDirs(provider, types.FilesystemPath(cwd), types.FilesystemPath(commandsDir))
	if err != nil {
		t.Fatalf("NewDiscoveryServiceWithDirs() error = %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app, err := NewApp(Dependencies{
		Config:    provider,
		Discovery: discoverySvc,
		Getenv:    getenv,
		Stdout:    stdout,
		Stderr:    stderr,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	return &configSourceTestEnv{app: app, stdout: stdout, stderr: stderr}
}

func (e *configSourceTestEnv) run(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd := NewRootCommand(e.app)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(e.stdout)
	rootCmd.SetErr(e.stderr)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.ExecuteContext(t.Context()); err != nil {
		return fmt.Errorf("invowk %v: %w", args, err)
	}
	return nil
}

func writeTestFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestConfigShowAndDumpHonourExplicitSources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	flagFile := writeTestFile(t, filepath.Join(dir, "flag.cue"), `container_engine: "docker"`)
	envFile := writeTestFile(t, filepath.Join(dir, "env.cue"), `default_runtime: "virtual-sh"`)
	userDir := t.TempDir()
	writeTestFile(t, filepath.Join(userDir, "config.cue"), `ui: {color_scheme: "light"}`)

	tests := []struct {
		name       string
		env        map[string]string
		args       []string
		wantSource string
		wantValue  string
		wantPath   string
	}{
		{name: "show flag", env: map[string]string{config.EnvConfigPath: envFile}, args: []string{"--ivk-config", flagFile, "config", "show"}, wantSource: "(from --ivk-config)", wantValue: "docker", wantPath: flagFile},
		{name: "show env", env: map[string]string{config.EnvConfigPath: envFile}, args: []string{"config", "show"}, wantSource: "(from INVOWK_CONFIG)", wantValue: "virtual-sh", wantPath: envFile},
		{name: "show user", args: []string{"config", "show"}, wantSource: "(user config)", wantValue: "light"},
		{name: "dump flag", args: []string{"--ivk-config", flagFile, "config", "dump"}, wantValue: `container_engine: "docker"`},
		{name: "dump env", env: map[string]string{config.EnvConfigPath: envFile}, args: []string{"config", "dump"}, wantValue: `default_runtime: "virtual-sh"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newConfigSourceTestApp(t, tt.env, userDir, t.TempDir(), "")
			if err := env.run(t, tt.args...); err != nil {
				t.Fatalf("run %v error = %v\nstderr: %s", tt.args, err, env.stderr)
			}
			out := env.stdout.String()
			for _, want := range []string{tt.wantSource, tt.wantValue, tt.wantPath} {
				if want != "" && !strings.Contains(out, want) {
					t.Fatalf("stdout missing %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestConfigShowFailsOnMissingExplicitFile(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.cue")
	for _, tt := range []struct {
		name    string
		env     map[string]string
		args    []string
		hint    string
		notHint string
	}{
		{name: "flag", args: []string{"--ivk-config", missing, "config", "show"}, hint: "check the --ivk-config path"},
		// A missing absolute file is not an absolute-path problem, so the hint
		// must not blame the path form.
		{name: "env", env: map[string]string{config.EnvConfigPath: missing}, args: []string{"config", "show"}, hint: "fix or unset INVOWK_CONFIG", notHint: "absolute"},
		{name: "env relative dump", env: map[string]string{config.EnvConfigPath: "config.cue"}, args: []string{"config", "dump"}, hint: "fix or unset INVOWK_CONFIG; it must be an absolute path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newConfigSourceTestApp(t, tt.env, t.TempDir(), t.TempDir(), "")
			err := env.run(t, tt.args...)
			if !errors.Is(err, config.ErrExplicitConfig) {
				t.Fatalf("error = %v, want ErrExplicitConfig", err)
			}
			if _, isExit := errors.AsType[*ExitError](err); isExit {
				t.Fatal("explicit config error must be a plain error (exit 1), not an ExitError")
			}
			if strings.Contains(env.stdout.String(), "Current Configuration") || strings.Contains(env.stdout.String(), "container_engine") {
				t.Fatalf("config values printed despite the failure:\n%s", env.stdout)
			}
			hintLine := hintLineOf(env.stderr.String())
			if !strings.Contains(hintLine, tt.hint) {
				t.Fatalf("hint line %q missing %q:\n%s", hintLine, tt.hint, env.stderr)
			}
			if tt.notHint != "" && strings.Contains(hintLine, tt.notHint) {
				t.Fatalf("hint line %q must not mention %q", hintLine, tt.notHint)
			}
		})
	}
}

// TestConfigLoadFailureGuidanceBySource checks the issue guidance per source:
// a failed explicit source must not advise `config init` or deleting the user
// config file (neither fixes it), while a failed user file keeps that advice.
// Both go through the same renderer, so the user-file case also proves the
// needles survive rendering.
func TestConfigLoadFailureGuidanceBySource(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.cue")
	badUserDir := t.TempDir()
	writeTestFile(t, filepath.Join(badUserDir, "config.cue"), `bogus_field: true`)
	for _, tt := range []struct {
		name    string
		env     map[string]string
		userDir string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name: "flag", userDir: t.TempDir(), args: []string{"--ivk-config", missing, "config", "show"},
			want: []string{"--ivk-config", "INVOWK_CONFIG"}, notWant: []string{"config init", "rm ~"},
		},
		{
			name: "env", env: map[string]string{config.EnvConfigPath: missing}, userDir: t.TempDir(), args: []string{"config", "show"},
			want: []string{"unset INVOWK_CONFIG"}, notWant: []string{"config init", "rm ~"},
		},
		{
			name: "user file", userDir: badUserDir, args: []string{"config", "show"},
			want: []string{"config init", "rm ~"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newConfigSourceTestApp(t, tt.env, tt.userDir, t.TempDir(), "")
			if err := env.run(t, tt.args...); err == nil {
				t.Fatal("config show succeeded, want a config load failure")
			}
			stderr := env.stderr.String()
			for _, want := range tt.want {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q:\n%s", want, stderr)
				}
			}
			for _, unwanted := range tt.notWant {
				if strings.Contains(stderr, unwanted) {
					t.Fatalf("stderr contains %q:\n%s", unwanted, stderr)
				}
			}
		})
	}
}

// hintLineOf returns the "Hint:" line that renderExplicitConfigError writes.
func hintLineOf(stderr string) string {
	for line := range strings.SplitSeq(stderr, "\n") {
		if _, hint, ok := strings.Cut(line, "Hint:"); ok {
			return hint
		}
	}
	return ""
}

func TestConfigPathReportsEffectiveSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := writeTestFile(t, filepath.Join(dir, "env.cue"), `default_runtime: "native"`)
	missing := filepath.Join(dir, "missing.cue")

	tests := []struct {
		name     string
		env      map[string]string
		args     []string
		want     string
		wantNone bool
	}{
		{name: "no explicit source", args: []string{"config", "path"}, wantNone: true},
		{name: "env existing", env: map[string]string{config.EnvConfigPath: existing}, args: []string{"config", "path"}, want: "Effective config file: " + existing + " (from INVOWK_CONFIG)\n"},
		{name: "env missing", env: map[string]string{config.EnvConfigPath: missing}, args: []string{"config", "path"}, want: missing + " (from INVOWK_CONFIG) [missing]"},
		{name: "env relative", env: map[string]string{config.EnvConfigPath: "config.cue"}, args: []string{"config", "path"}, want: "config.cue (from INVOWK_CONFIG) [invalid: must be absolute]"},
		{name: "flag missing beats env", env: map[string]string{config.EnvConfigPath: existing}, args: []string{"--ivk-config", missing, "config", "path"}, want: missing + " (from --ivk-config) [missing]"},
		{name: "invalid INVOWK_NO_CWD is ignored", env: map[string]string{envNoCwd: "yes"}, args: []string{"config", "path"}, wantNone: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := newConfigSourceTestApp(t, tt.env, t.TempDir(), t.TempDir(), "")
			if err := env.run(t, tt.args...); err != nil {
				t.Fatalf("config path error = %v", err)
			}
			out := env.stdout.String()
			if !strings.Contains(out, "Config file:") {
				t.Fatalf("stdout missing the user config file line:\n%s", out)
			}
			if tt.wantNone && strings.Contains(out, "Effective config file:") {
				t.Fatalf("unexpected effective config line:\n%s", out)
			}
			if tt.want != "" && !strings.Contains(out, tt.want) {
				t.Fatalf("stdout missing %q:\n%s", tt.want, out)
			}
		})
	}
}

// isolateUserConfigDir points config.ConfigDir() at a temp directory on every
// OS and returns the user config file path. It mutates process env, so
// callers must not be parallel.
func isolateUserConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Cleanup(testutil.SetHomeDir(t, home))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if runtime.GOOS == platform.Windows {
		t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	}
	cfgDir, err := config.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error = %v", err)
	}
	return filepath.Join(string(cfgDir), "config.cue")
}

func TestConfigSetRefusesUnderExplicitSource(t *testing.T) { //nolint:paralleltest // isolateUserConfigDir mutates process env.
	userFile := isolateUserConfigDir(t)
	dir := t.TempDir()
	envFile := writeTestFile(t, filepath.Join(dir, "env.cue"), `container_engine: "docker"`)
	flagFile := filepath.Join(dir, "flag.cue")

	//nolint:paralleltest // subtests share the process env set by isolateUserConfigDir.
	for _, tt := range []struct {
		name  string
		env   map[string]string
		args  []string
		label string
	}{
		{name: "env", env: map[string]string{config.EnvConfigPath: envFile}, args: []string{"config", "set", "default_runtime", "virtual-sh"}, label: "INVOWK_CONFIG"},
		{name: "relative env", env: map[string]string{config.EnvConfigPath: "config.cue"}, args: []string{"config", "set", "default_runtime", "virtual-sh"}, label: "INVOWK_CONFIG"},
		{name: "flag", args: []string{"--ivk-config", flagFile, "config", "set", "default_runtime", "virtual-sh"}, label: "--ivk-config"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := newConfigSourceTestApp(t, tt.env, filepath.Dir(userFile), t.TempDir(), "")
			err := env.run(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.label) || !strings.Contains(err.Error(), userFile) {
				t.Fatalf("error = %v, want a refusal naming %s and %s", err, tt.label, userFile)
			}
			if _, statErr := os.Stat(userFile); !os.IsNotExist(statErr) {
				t.Fatalf("user config file was written: %v", statErr)
			}
			if _, statErr := os.Stat(flagFile); !os.IsNotExist(statErr) {
				t.Fatalf("flag file was written: %v", statErr)
			}
			if data, readErr := os.ReadFile(envFile); readErr != nil || string(data) != `container_engine: "docker"` {
				t.Fatalf("env file changed: %q, %v", data, readErr)
			}
		})
	}
}

func TestConfigSetWritesUserFileWithoutExplicitSource(t *testing.T) { //nolint:paralleltest // isolateUserConfigDir mutates process env.
	userFile := isolateUserConfigDir(t)
	env := newConfigSourceTestApp(t, nil, filepath.Dir(userFile), t.TempDir(), "")
	if err := env.run(t, "config", "set", "default_runtime", "virtual-sh"); err != nil {
		t.Fatalf("config set error = %v", err)
	}
	data, err := os.ReadFile(userFile)
	if err != nil {
		t.Fatalf("read user config: %v", err)
	}
	if !strings.Contains(string(data), `default_runtime: "virtual-sh"`) || !strings.Contains(string(data), `container_engine: "podman"`) {
		t.Fatalf("user config = %s, want default_runtime set and other values at defaults", data)
	}
}

func TestValidateHonoursConfigFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	moduleDir := createInternalCheckCmdTestModule(t, filepath.Join(dir, "mods"), "inc.invowkmod", "inc", "fromcfg")
	cfgFile := writeTestFile(t, filepath.Join(dir, "cfg.cue"), `includes: [{path: "`+filepath.ToSlash(moduleDir)+`"}]`)

	env := newConfigSourceTestApp(t, nil, t.TempDir(), t.TempDir(), "")
	if err := env.run(t, "--ivk-config", cfgFile, "validate"); err != nil {
		t.Fatalf("validate error = %v\nstdout: %s\nstderr: %s", err, env.stdout, env.stderr)
	}
	if !strings.Contains(env.stdout.String(), "1 source(s) discovered, 1 command(s) found") {
		t.Fatalf("validate did not use the --ivk-config includes:\n%s", env.stdout)
	}

	missing := newConfigSourceTestApp(t, nil, t.TempDir(), t.TempDir(), "")
	err := missing.run(t, "--ivk-config", filepath.Join(dir, "missing.cue"), "validate")
	if !errors.Is(err, config.ErrExplicitConfig) {
		t.Fatalf("validate with missing --ivk-config error = %v, want ErrExplicitConfig", err)
	}
	if strings.Contains(missing.stdout.String(), "Workspace Validation") {
		t.Fatalf("validation result printed despite the config failure:\n%s", missing.stdout)
	}
}

// TestCmdFailsClosedOnExplicitConfig runs the full `invowk cmd <name>` path
// with a failed explicit source and checks the exit-1 contract: a plain error
// (Execute maps it to 1, an ExitError would carry its own code) and a script
// that never ran. The positive run proves the marker mechanism works.
func TestCmdFailsClosedOnExplicitConfig(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.cue")
	for _, tt := range []struct {
		name    string
		env     map[string]string
		args    []string
		wantRun bool
	}{
		{name: "no explicit source runs", args: []string{"cmd", "hello"}, wantRun: true},
		{name: "missing flag file", args: []string{"--ivk-config", missing, "cmd", "hello"}},
		{name: "missing env file", env: map[string]string{config.EnvConfigPath: missing}, args: []string{"cmd", "hello"}},
		{name: "relative env", env: map[string]string{config.EnvConfigPath: "config.cue"}, args: []string{"cmd", "hello"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cwd := t.TempDir()
			marker := filepath.Join(t.TempDir(), "ran")
			writeTestFile(t, filepath.Join(cwd, "invowkfile.cue"),
				`cmds: [{name: "hello", implementations: [{script: {content: "echo ran > '`+filepath.ToSlash(marker)+`'"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`)
			// A cwd config.cue with the same name must never rescue the run.
			writeTestFile(t, filepath.Join(cwd, "config.cue"), `container_engine: "docker"`)
			env := newConfigSourceTestApp(t, tt.env, t.TempDir(), cwd, "")

			err := env.run(t, tt.args...)
			_, statErr := os.Stat(marker)
			if tt.wantRun {
				if err != nil || statErr != nil {
					t.Fatalf("run error = %v, marker stat = %v; want the script to run\nstderr: %s", err, statErr, env.stderr)
				}
				return
			}
			if !errors.Is(err, config.ErrExplicitConfig) {
				t.Fatalf("error = %v, want ErrExplicitConfig", err)
			}
			if _, isExit := errors.AsType[*ExitError](err); isExit {
				t.Fatalf("error = %v is an ExitError; the explicit config failure must exit 1", err)
			}
			if !os.IsNotExist(statErr) {
				t.Fatalf("the script ran despite the failed explicit config (marker stat = %v)", statErr)
			}
		})
	}
}
