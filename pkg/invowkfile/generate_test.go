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
					Patterns:    []string{"**/*.go", "**/*.cue"},
					Debounce:    "300ms",
					ClearScreen: true,
					Ignore:      []string{"**/vendor/**"},
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
					Patterns: []string{"**/*"},
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
