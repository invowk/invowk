// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestBuildHostEnv_NestedConfigScopeVars pins decision D8 of the
// harden-config-trust change: INVOWK_CONFIG and INVOWK_NO_CWD are neither
// filtered nor exported. A child sees them only when the caller set them, and
// the normal env_inherit_mode rules decide; the flag forms (--ivk-config,
// --ivk-no-cwd) never turn into child variables.
func TestBuildHostEnv_NestedConfigScopeVars(t *testing.T) {
	t.Parallel()

	callerEnv := func() []string {
		return []string{"INVOWK_CONFIG=/abs/c.cue", "INVOWK_NO_CWD=1", "OTHER=x"}
	}
	noScopeEnv := func() []string { return []string{"OTHER=x"} }

	tests := []struct {
		name     string
		cfg      envInheritConfig
		environ  func() []string
		wantVars map[string]string
		dontWant []string
	}{
		{
			name:     "all passes the caller's values",
			cfg:      envInheritConfig{mode: invowkfile.EnvInheritAll},
			environ:  callerEnv,
			wantVars: map[string]string{"INVOWK_CONFIG": "/abs/c.cue", "INVOWK_NO_CWD": "1"},
		},
		{
			name:     "none drops both",
			cfg:      envInheritConfig{mode: invowkfile.EnvInheritNone},
			environ:  callerEnv,
			dontWant: []string{"INVOWK_CONFIG", "INVOWK_NO_CWD"},
		},
		{
			name:     "allow passes only allowlisted names",
			cfg:      envInheritConfig{mode: invowkfile.EnvInheritAllow, allow: []invowkfile.EnvVarName{"INVOWK_NO_CWD"}},
			environ:  callerEnv,
			wantVars: map[string]string{"INVOWK_NO_CWD": "1"},
			dontWant: []string{"INVOWK_CONFIG"},
		},
		{
			name:     "flag forms are not exported",
			cfg:      envInheritConfig{mode: invowkfile.EnvInheritAll},
			environ:  noScopeEnv,
			dontWant: []string{"INVOWK_CONFIG", "INVOWK_NO_CWD"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := buildHostEnv(tt.cfg, tt.environ)
			for k, v := range tt.wantVars {
				if got := env[k]; got != v {
					t.Errorf("buildHostEnv() %s = %q, want %q", k, got, v)
				}
			}
			for _, k := range tt.dontWant {
				if _, ok := env[k]; ok {
					t.Errorf("buildHostEnv() must not include %s", k)
				}
			}
		})
	}

	for _, name := range []string{"INVOWK_CONFIG", "INVOWK_NO_CWD"} {
		if shouldFilterEnvVar(name) {
			t.Errorf("shouldFilterEnvVar(%q) = true; config-scope variables must pass to nested invocations", name)
		}
	}
}
