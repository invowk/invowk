// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestFindRuntimeConfig(t *testing.T) {
	t.Parallel()

	runtimes := []RuntimeConfig{
		{Name: RuntimeNative},
		{Name: RuntimeVirtual},
		{Name: RuntimeContainer, Image: "debian:stable-slim"},
	}

	tests := []struct {
		name     string
		mode     RuntimeMode
		wantNil  bool
		wantName RuntimeMode
	}{
		{"find native", RuntimeNative, false, RuntimeNative},
		{"find virtual", RuntimeVirtual, false, RuntimeVirtual},
		{"find container", RuntimeContainer, false, RuntimeContainer},
		{"not found", RuntimeMode("nonexistent"), true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := FindRuntimeConfig(runtimes, tt.mode)
			if tt.wantNil {
				if rc != nil {
					t.Errorf("FindRuntimeConfig(%q) = %v, want nil", tt.mode, rc)
				}
				return
			}
			if rc == nil {
				t.Fatalf("FindRuntimeConfig(%q) = nil, want non-nil", tt.mode)
			}
			if rc.Name != tt.wantName {
				t.Errorf("FindRuntimeConfig(%q).Name = %q, want %q", tt.mode, rc.Name, tt.wantName)
			}
		})
	}
}

func TestFindRuntimeConfig_EmptySlice(t *testing.T) {
	t.Parallel()
	rc := FindRuntimeConfig(nil, RuntimeNative)
	if rc != nil {
		t.Errorf("FindRuntimeConfig(nil, native) = %v, want nil", rc)
	}
}

func TestFindRuntimeConfig_ReturnsMutablePointer(t *testing.T) {
	t.Parallel()

	runtimes := []RuntimeConfig{
		{Name: RuntimeContainer, Image: "debian:stable-slim"},
	}

	rc := FindRuntimeConfig(runtimes, RuntimeContainer)
	if rc == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify the returned pointer points into the original slice
	rc.Image = "modified"
	if runtimes[0].Image != "modified" {
		t.Error("FindRuntimeConfig should return a pointer into the original slice")
	}
}

func TestParseRuntimeDependsOn(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{
			name: "container"
			image: "debian:stable-slim"
			depends_on: {
				tools: [{alternatives: ["python3"]}]
				env_vars: [{alternatives: [{name: "API_KEY"}]}]
			}
		}]
		platforms: [{name: "linux"}]
	}]
}]
`

	inv, err := ParseBytes([]byte(cueContent), "test.cue")
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(inv.Commands))
	}

	impl := &inv.Commands[0].Implementations[0]
	if len(impl.Runtimes) != 1 {
		t.Fatalf("expected 1 runtime, got %d", len(impl.Runtimes))
	}

	rc := &impl.Runtimes[0]
	if rc.DependsOn == nil {
		t.Fatal("expected runtime-level depends_on, got nil")
	}

	if len(rc.DependsOn.Tools) != 1 {
		t.Fatalf("expected 1 tool dep, got %d", len(rc.DependsOn.Tools))
	}
	if rc.DependsOn.Tools[0].Alternatives[0] != "python3" {
		t.Errorf("tool alternative = %q, want %q", rc.DependsOn.Tools[0].Alternatives[0], "python3")
	}

	if len(rc.DependsOn.EnvVars) != 1 {
		t.Fatalf("expected 1 env var dep, got %d", len(rc.DependsOn.EnvVars))
	}
	if rc.DependsOn.EnvVars[0].Alternatives[0].Name != "API_KEY" {
		t.Errorf("env var name = %q, want %q", rc.DependsOn.EnvVars[0].Alternatives[0].Name, "API_KEY")
	}
}

func TestParseRuntimeDependsOn_AllDepTypes(t *testing.T) {
	t.Parallel()

	// depends_on is container-only; use container runtime with all 6 dependency types
	cueContent := `
cmds: [{
	name: "full-deps"
	implementations: [{
		script: "echo test"
		runtimes: [{
			name: "container"
			image: "debian:stable-slim"
			depends_on: {
				tools: [{alternatives: ["curl"]}]
				cmds: [{alternatives: ["build"]}]
				filepaths: [{alternatives: ["/tmp"]}]
				capabilities: [{alternatives: ["internet"]}]
				custom_checks: [{name: "version", check_script: "echo 1"}]
				env_vars: [{alternatives: [{name: "HOME"}]}]
			}
		}]
		platforms: [{name: "linux"}]
	}]
}]
`

	inv, err := ParseBytes([]byte(cueContent), "test.cue")
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	rc := &inv.Commands[0].Implementations[0].Runtimes[0]
	if rc.DependsOn == nil {
		t.Fatal("expected runtime-level depends_on, got nil")
	}

	deps := rc.DependsOn
	if len(deps.Tools) != 1 {
		t.Errorf("tools count = %d, want 1", len(deps.Tools))
	}
	if len(deps.Commands) != 1 {
		t.Errorf("cmds count = %d, want 1", len(deps.Commands))
	}
	if len(deps.Filepaths) != 1 {
		t.Errorf("filepaths count = %d, want 1", len(deps.Filepaths))
	}
	if len(deps.Capabilities) != 1 {
		t.Errorf("capabilities count = %d, want 1", len(deps.Capabilities))
	}
	if len(deps.CustomChecks) != 1 {
		t.Errorf("custom_checks count = %d, want 1", len(deps.CustomChecks))
	}
	if len(deps.EnvVars) != 1 {
		t.Errorf("env_vars count = %d, want 1", len(deps.EnvVars))
	}
}

func TestParseRuntimeDependsOn_RejectsNonContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime string
	}{
		{"virtual rejects depends_on", "virtual"},
		{"native rejects depends_on", "native"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cueContent := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo test"
		runtimes: [{
			name: "` + tt.runtime + `"
			depends_on: {
				tools: [{alternatives: ["curl"]}]
			}
		}]
		platforms: [{name: "linux"}]
	}]
}]
`

			_, err := ParseBytes([]byte(cueContent), "test.cue")
			if err == nil {
				t.Fatalf("expected parse error for %s runtime with depends_on, got nil", tt.runtime)
			}
			// CUE should reject depends_on on non-container runtimes via close()
			if !strings.Contains(err.Error(), "depends_on") {
				t.Errorf("error should mention depends_on, got: %v", err)
			}
		})
	}
}

func TestParseRuntimeDependsOn_NoDeps(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [{
	name: "no-deps"
	implementations: [{
		script: "echo test"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
`

	inv, err := ParseBytes([]byte(cueContent), "test.cue")
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	rc := &inv.Commands[0].Implementations[0].Runtimes[0]
	if rc.DependsOn != nil {
		t.Errorf("expected nil DependsOn for runtime without depends_on block, got %v", rc.DependsOn)
	}
}

func TestGenerateCUE_RuntimeDependsOn(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "test-gen",
			Implementations: []Implementation{{
				Script: "echo test",
				Runtimes: []RuntimeConfig{{
					Name:  RuntimeContainer,
					Image: "debian:stable-slim",
					DependsOn: &DependsOn{
						Tools:    []ToolDependency{{Alternatives: []string{"python3"}}},
						EnvVars:  []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "API_KEY"}}}},
						Commands: []CommandDependency{{Alternatives: []string{"build"}}},
					},
				}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
		}},
	}

	generated := GenerateCUE(inv)

	// Verify the generated CUE contains runtime-level depends_on
	if !strings.Contains(generated, "depends_on:") {
		t.Error("generated CUE should contain depends_on inside runtime config")
	}
	if !strings.Contains(generated, `"python3"`) {
		t.Error("generated CUE should contain python3 tool dependency")
	}
	if !strings.Contains(generated, `"API_KEY"`) {
		t.Error("generated CUE should contain API_KEY env var")
	}

	// Verify roundtrip: parse the generated CUE
	parsed, err := ParseBytes([]byte(generated), "roundtrip.cue")
	if err != nil {
		t.Fatalf("roundtrip ParseBytes failed: %v", err)
	}

	rc := parsed.Commands[0].Implementations[0].Runtimes[0]
	if rc.DependsOn == nil {
		t.Fatal("roundtrip: runtime DependsOn is nil")
	}
	if len(rc.DependsOn.Tools) != 1 || rc.DependsOn.Tools[0].Alternatives[0] != "python3" {
		t.Errorf("roundtrip: tool = %v, want [python3]", rc.DependsOn.Tools)
	}
	if len(rc.DependsOn.EnvVars) != 1 || rc.DependsOn.EnvVars[0].Alternatives[0].Name != "API_KEY" {
		t.Errorf("roundtrip: env var = %v, want [{API_KEY}]", rc.DependsOn.EnvVars)
	}
	if len(rc.DependsOn.Commands) != 1 || rc.DependsOn.Commands[0].Alternatives[0] != "build" {
		t.Errorf("roundtrip: cmd = %v, want [build]", rc.DependsOn.Commands)
	}
}

func TestGenerateCUE_RuntimeNoDeps_CompactFormat(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "compact",
			Implementations: []Implementation{{
				Script: "echo test",
				Runtimes: []RuntimeConfig{{
					Name:  RuntimeContainer,
					Image: "debian:stable-slim",
				}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
		}},
	}

	generated := GenerateCUE(inv)

	// Verify compact format (no multi-line runtime block)
	// The runtime config should be on a single line when no depends_on
	for line := range strings.SplitSeq(generated, "\n") {
		if strings.Contains(line, `name: "container"`) && strings.Contains(line, `image:`) {
			return // Found compact format
		}
	}
	t.Error("expected compact single-line format for runtime without depends_on")
}
