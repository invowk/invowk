// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

// configDirProvider pins the user config directory of an inner provider so
// tests never read the developer's real user config.
type configDirProvider struct {
	inner config.Provider
	dir   string
}

func (p *configDirProvider) Load(ctx context.Context, opts config.LoadOptions) (*config.Config, error) {
	opts.ConfigDirPath = types.FilesystemPath(p.dir)
	return p.inner.Load(ctx, opts)
}

func (p *configDirProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	opts.ConfigDirPath = types.FilesystemPath(p.dir)
	return p.inner.LoadWithSource(ctx, opts)
}

// envGetter returns an injected getenv backed by values; nil means empty.
func envGetter(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func newNoCwdFlagSet(t *testing.T, rootFlags *rootFlagValues, args ...string) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("root", pflag.ContinueOnError)
	fs.BoolVar(&rootFlags.noCwd, noCwdFlagName, false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return fs
}

func TestResolveNoCwd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		env        string
		wantOn     bool
		wantOrigin noCwdOrigin
		wantErr    bool
	}{
		{name: "unset is off", wantOrigin: noCwdOriginEnv},
		{name: "empty env is off", env: "", wantOrigin: noCwdOriginEnv},
		{name: "env 1 is on", env: "1", wantOn: true, wantOrigin: noCwdOriginEnv},
		{name: "env TRUE is on", env: "TRUE", wantOn: true, wantOrigin: noCwdOriginEnv},
		{name: "env false is off", env: "false", wantOrigin: noCwdOriginEnv},
		{name: "env yes is invalid", env: "yes", wantErr: true, wantOrigin: noCwdOriginEnv},
		{name: "flag on", args: []string{"--ivk-no-cwd"}, wantOn: true, wantOrigin: noCwdOriginFlag},
		{name: "flag false beats env 1", args: []string{"--ivk-no-cwd=false"}, env: "1", wantOrigin: noCwdOriginFlag},
		{name: "flag skips invalid env", args: []string{"--ivk-no-cwd"}, env: "yes", wantOn: true, wantOrigin: noCwdOriginFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rootFlags := &rootFlagValues{}
			fs := newNoCwdFlagSet(t, rootFlags, tt.args...)
			on, origin, err := resolveNoCwd(fs, rootFlags, envGetter(map[string]string{envNoCwd: tt.env}))
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveNoCwd() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidNoCwdEnv) {
					t.Fatalf("error = %v, want ErrInvalidNoCwdEnv", err)
				}
				if !strings.Contains(err.Error(), envNoCwd) || !strings.Contains(err.Error(), "accepted values") {
					t.Fatalf("error %q must name %s and list the accepted values", err, envNoCwd)
				}
			}
			if on != tt.wantOn || origin != tt.wantOrigin {
				t.Fatalf("resolveNoCwd() = (%v, %q), want (%v, %q)", on, origin, tt.wantOn, tt.wantOrigin)
			}
		})
	}
}

func TestExplicitNoCwdFromArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantValue bool
		wantSet   bool
		wantErr   bool
	}{
		{name: "absent", args: []string{"cmd", "build"}},
		{name: "before cmd", args: []string{"--ivk-no-cwd", "cmd", "build"}, wantValue: true, wantSet: true},
		{name: "after cmd", args: []string{"cmd", "--ivk-no-cwd", "build"}, wantValue: true, wantSet: true},
		{name: "inline true", args: []string{"--ivk-no-cwd=true", "cmd"}, wantValue: true, wantSet: true},
		{name: "inline false", args: []string{"cmd", "--ivk-no-cwd=false"}, wantSet: true},
		{name: "after terminator is an argument", args: []string{"cmd", "local", "--", "--ivk-no-cwd"}},
		{name: "last occurrence wins", args: []string{"--ivk-no-cwd", "cmd", "--ivk-no-cwd=false"}, wantSet: true},
		{name: "malformed inline value", args: []string{"--ivk-no-cwd=maybe", "cmd"}, wantSet: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value, set, err := explicitNoCwdFromArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("explicitNoCwdFromArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if value != tt.wantValue || set != tt.wantSet {
				t.Fatalf("explicitNoCwdFromArgs() = (%v, %v), want (%v, %v)", value, set, tt.wantValue, tt.wantSet)
			}
		})
	}
}

func TestNoCwdForRegistration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		env     string
		want    bool
		wantErr bool
	}{
		{name: "env fallback when flag absent", args: []string{"cmd"}, env: "1", want: true},
		{name: "flag beats env", args: []string{"--ivk-no-cwd=false", "cmd"}, env: "1"},
		{name: "flag skips invalid env", args: []string{"cmd", "--ivk-no-cwd"}, env: "yes", want: true},
		{name: "invalid env without flag fails", args: []string{"cmd"}, env: "yes", wantErr: true},
		{name: "malformed flag fails", args: []string{"--ivk-no-cwd=maybe", "cmd"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := noCwdForRegistration(tt.args, envGetter(map[string]string{envNoCwd: tt.env}))
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("noCwdForRegistration() = (%v, %v), want (%v, err=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestFirstTopLevelTokenSkipsNoCwdFlag(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"--ivk-no-cwd", "cmd", "build"},
		{"--ivk-no-cwd=true", "cmd"},
		{"--ivk-no-cwd=false", "--ivk-verbose", "cmd"},
	} {
		if token, _ := firstTopLevelToken(args); token != "cmd" {
			t.Errorf("firstTopLevelToken(%v) = %q, want cmd", args, token)
		}
		if !shouldRegisterDiscoveredCommands(args) {
			t.Errorf("shouldRegisterDiscoveredCommands(%v) = false, want true", args)
		}
	}
	if !shouldRegisterDiscoveredCommands([]string{"__complete", "--ivk-no-cwd", "cmd", ""}) {
		t.Error("completion with --ivk-no-cwd before cmd must still register discovered commands")
	}
}

func TestNoCwdOriginValidate(t *testing.T) {
	t.Parallel()

	for _, origin := range []noCwdOrigin{"", noCwdOriginFlag, noCwdOriginEnv} {
		if err := origin.Validate(); err != nil {
			t.Errorf("%q.Validate() = %v", origin, err)
		}
	}
	if err := noCwdOrigin("cwd").Validate(); !errors.Is(err, ErrInvalidNoCwdOrigin) {
		t.Fatalf("invalid origin error = %v, want ErrInvalidNoCwdOrigin", err)
	}
}
