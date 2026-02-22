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
						Tools:    []ToolDependency{{Alternatives: []BinaryName{"python3"}}},
						EnvVars:  []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "API_KEY"}}}},
						Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}},
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

// T7: Extended GenerateCUE roundtrip with all 6 dependency types
func TestGenerateCUE_RuntimeDependsOn_AllDepTypes(t *testing.T) {
	t.Parallel()

	expectedCode := 0
	inv := &Invowkfile{
		Commands: []Command{{
			Name: "all-deps",
			Implementations: []Implementation{{
				Script: "echo test",
				Runtimes: []RuntimeConfig{{
					Name:  RuntimeContainer,
					Image: "debian:stable-slim",
					DependsOn: &DependsOn{
						Tools:    []ToolDependency{{Alternatives: []BinaryName{"curl"}}},
						Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}},
						Filepaths: []FilepathDependency{{
							Alternatives: []string{"/etc/hosts"},
							Readable:     true,
						}},
						Capabilities: []CapabilityDependency{{
							Alternatives: []CapabilityName{CapabilityInternet},
						}},
						CustomChecks: []CustomCheckDependency{{
							Name:         "version-check",
							CheckScript:  "echo 1",
							ExpectedCode: &expectedCode,
						}},
						EnvVars: []EnvVarDependency{{
							Alternatives: []EnvVarCheck{{Name: "HOME"}},
						}},
					},
				}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
		}},
	}

	generated := GenerateCUE(inv)

	// Verify all 6 dep types are present in generated CUE
	for _, want := range []string{"tools:", "cmds:", "filepaths:", "capabilities:", "custom_checks:", "env_vars:"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated CUE missing %q", want)
		}
	}

	// Roundtrip: parse the generated CUE
	parsed, err := ParseBytes([]byte(generated), "roundtrip-all.cue")
	if err != nil {
		t.Fatalf("roundtrip ParseBytes failed: %v", err)
	}

	rc := parsed.Commands[0].Implementations[0].Runtimes[0]
	if rc.DependsOn == nil {
		t.Fatal("roundtrip: runtime DependsOn is nil")
	}

	deps := rc.DependsOn
	if len(deps.Tools) != 1 || deps.Tools[0].Alternatives[0] != "curl" {
		t.Errorf("roundtrip: tools = %v, want [{curl}]", deps.Tools)
	}
	if len(deps.Commands) != 1 || deps.Commands[0].Alternatives[0] != "build" {
		t.Errorf("roundtrip: cmds = %v, want [{build}]", deps.Commands)
	}
	if len(deps.Filepaths) != 1 || deps.Filepaths[0].Alternatives[0] != "/etc/hosts" {
		t.Errorf("roundtrip: filepaths = %v, want [{/etc/hosts}]", deps.Filepaths)
	}
	if !deps.Filepaths[0].Readable {
		t.Error("roundtrip: filepaths[0].readable should be true")
	}
	if len(deps.Capabilities) != 1 || deps.Capabilities[0].Alternatives[0] != CapabilityInternet {
		t.Errorf("roundtrip: capabilities = %v, want [{internet}]", deps.Capabilities)
	}
	if len(deps.CustomChecks) != 1 || deps.CustomChecks[0].Name != "version-check" {
		t.Errorf("roundtrip: custom_checks = %v, want [{version-check}]", deps.CustomChecks)
	}
	if len(deps.EnvVars) != 1 || deps.EnvVars[0].Alternatives[0].Name != "HOME" {
		t.Errorf("roundtrip: env_vars = %v, want [{HOME}]", deps.EnvVars)
	}
}

// T6: Structural validation defense-in-depth — Go StructureValidator catches
// depends_on on non-container RuntimeConfig even when CUE parsing is bypassed
func TestStructureValidator_RuntimeDependsOn_RejectsNonContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime RuntimeMode
	}{
		{"native rejects depends_on", RuntimeNative},
		{"virtual rejects depends_on", RuntimeVirtual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := &Invowkfile{
				Commands: []Command{{
					Name: "test",
					Implementations: []Implementation{{
						Script: "echo test",
						Runtimes: []RuntimeConfig{{
							Name: tt.runtime,
							DependsOn: &DependsOn{
								Tools: []ToolDependency{{Alternatives: []BinaryName{"curl"}}},
							},
						}},
						Platforms: AllPlatformConfigs(),
					}},
				}},
			}

			validator := NewStructureValidator()
			ctx := &ValidationContext{FilePath: "test.cue"}
			errs := validator.Validate(ctx, inv)

			found := false
			for _, e := range errs {
				if strings.Contains(e.Message, "depends_on is only valid for container runtime") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("StructureValidator should reject depends_on on %s runtime, got errors: %v", tt.runtime, errs)
			}
		})
	}
}

func TestStructureValidator_RuntimeDependsOn_AllowsContainer(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "test",
			Implementations: []Implementation{{
				Script: "echo test",
				Runtimes: []RuntimeConfig{{
					Name:  RuntimeContainer,
					Image: "debian:stable-slim",
					DependsOn: &DependsOn{
						Tools: []ToolDependency{{Alternatives: []BinaryName{"curl"}}},
					},
				}},
				Platforms: AllPlatformConfigs(),
			}},
		}},
	}

	validator := NewStructureValidator()
	ctx := &ValidationContext{FilePath: "test.cue"}
	errs := validator.Validate(ctx, inv)

	for _, e := range errs {
		if strings.Contains(e.Message, "depends_on is only valid for container runtime") {
			t.Errorf("StructureValidator should NOT reject depends_on on container runtime, got: %v", e)
		}
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
