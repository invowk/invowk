// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestGenerateCUE_Category(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name:     "build",
				Category: "Build",
				Implementations: []Implementation{
					{
						Script:    "echo building",
						Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
						Platforms: AllPlatformConfigs(),
					},
				},
			},
		},
	}

	got := GenerateCUE(inv)

	if !strings.Contains(got, `category: "Build"`) {
		t.Errorf("expected category field in generated CUE, got:\n%s", got)
	}
}

func TestGenerateCUE_WatchConfig(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "dev",
				Implementations: []Implementation{
					{
						Script:    "echo running",
						Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
						Platforms: AllPlatformConfigs(),
					},
				},
				Watch: &WatchConfig{
					Patterns:    []GlobPattern{"**/*.go", "**/*.cue"},
					Debounce:    "300ms",
					ClearScreen: true,
					Ignore:      []GlobPattern{"**/vendor/**"},
				},
			},
		},
	}

	got := GenerateCUE(inv)

	checks := []string{
		`"**/*.go"`,
		`"**/*.cue"`,
		`debounce: "300ms"`,
		`clear_screen: true`,
		`"**/vendor/**"`,
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("expected %q in generated CUE, got:\n%s", check, got)
		}
	}
}

func TestGenerateCUE_Timeout(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "slow",
				Implementations: []Implementation{
					{
						Script:    "sleep 10",
						Timeout:   "30s",
						Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
						Platforms: AllPlatformConfigs(),
					},
				},
			},
		},
	}

	got := GenerateCUE(inv)

	if !strings.Contains(got, `timeout: "30s"`) {
		t.Errorf("expected timeout field in generated CUE, got:\n%s", got)
	}
}

func TestGenerateCUE_WatchConfigMinimal(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "dev",
				Implementations: []Implementation{
					{
						Script:    "echo running",
						Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
						Platforms: AllPlatformConfigs(),
					},
				},
				Watch: &WatchConfig{
					Patterns: []GlobPattern{"**/*"},
				},
			},
		},
	}

	got := GenerateCUE(inv)

	if !strings.Contains(got, "watch:") {
		t.Errorf("expected watch block in generated CUE, got:\n%s", got)
	}
	// Should NOT contain optional fields when not set.
	if strings.Contains(got, "debounce:") {
		t.Errorf("unexpected debounce field in minimal watch config, got:\n%s", got)
	}
	if strings.Contains(got, "clear_screen:") {
		t.Errorf("unexpected clear_screen field in minimal watch config, got:\n%s", got)
	}
}

func TestGenerateCUE_FlagEnhancedFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "deploy",
			Implementations: []Implementation{{
				Script:    "echo deploy",
				Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
				Platforms: AllPlatformConfigs(),
			}},
			Flags: []Flag{{
				Name:         "environment",
				Description:  "Target environment",
				Type:         FlagTypeString,
				Required:     true,
				Short:        "g",
				Validation:   "^(dev|staging|prod)$",
				DefaultValue: "",
			}, {
				Name:         "dry-run",
				Description:  "Preview changes",
				Type:         FlagTypeBool,
				Short:        "d",
				DefaultValue: "false",
			}},
		}},
	}

	got := GenerateCUE(inv)
	for _, want := range []string{
		`name: "environment"`,
		`required: true`,
		`short: "g"`,
		`validation: "^(dev|staging|prod)$"`,
		`type: "bool"`,
		`short: "d"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated CUE missing %q:\n%s", want, got)
		}
	}

	parsed, err := ParseBytes([]byte(got), "roundtrip.cue")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v\n%s", err, got)
	}
	envFlag := parsed.Commands[0].Flags[0]
	if !envFlag.Required || envFlag.Short != "g" || envFlag.Validation != "^(dev|staging|prod)$" {
		t.Fatalf("roundtrip flag = %+v, want required short validation preserved", envFlag)
	}
	if gotType := parsed.Commands[0].Flags[1].GetType(); gotType != FlagTypeBool {
		t.Fatalf("roundtrip bool flag type = %q, want %q", gotType, FlagTypeBool)
	}
}

func TestGenerateCUE_RuntimeBaseFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "run",
			Implementations: []Implementation{{
				Script: "echo run",
				Runtimes: []RuntimeConfig{{
					Name:            RuntimeNative,
					Interpreter:     "bash",
					EnvInheritMode:  EnvInheritAllow,
					EnvInheritAllow: []EnvVarName{"PATH", "TERM"},
					EnvInheritDeny:  []EnvVarName{"SECRET_TOKEN"},
				}, {
					Name:        RuntimeVirtual,
					Interpreter: "ignored",
				}},
				Platforms: AllPlatformConfigs(),
			}},
		}},
	}

	got := GenerateCUE(inv)
	for _, want := range []string{
		`interpreter: "bash"`,
		`env_inherit_mode: "allow"`,
		`env_inherit_allow: ["PATH", "TERM"]`,
		`env_inherit_deny: ["SECRET_TOKEN"]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated CUE missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `interpreter: "ignored"`) {
		t.Fatalf("virtual runtime interpreter must remain omitted:\n%s", got)
	}

	parsed, err := ParseBytes([]byte(got), "roundtrip.cue")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v\n%s", err, got)
	}
	runtimeCfg := parsed.Commands[0].Implementations[0].Runtimes[0]
	if runtimeCfg.Interpreter != "bash" || runtimeCfg.EnvInheritMode != EnvInheritAllow {
		t.Fatalf("roundtrip runtime = %+v, want interpreter and env inherit mode", runtimeCfg)
	}
	if len(runtimeCfg.EnvInheritAllow) != 2 || runtimeCfg.EnvInheritAllow[0] != "PATH" || runtimeCfg.EnvInheritAllow[1] != "TERM" {
		t.Fatalf("roundtrip env_inherit_allow = %v, want [PATH TERM]", runtimeCfg.EnvInheritAllow)
	}
	if len(runtimeCfg.EnvInheritDeny) != 1 || runtimeCfg.EnvInheritDeny[0] != "SECRET_TOKEN" {
		t.Fatalf("roundtrip env_inherit_deny = %v, want [SECRET_TOKEN]", runtimeCfg.EnvInheritDeny)
	}
}
