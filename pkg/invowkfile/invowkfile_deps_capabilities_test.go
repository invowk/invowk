// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Capability Dependencies
// ============================================================================

func TestParseDependsOn_WithCapabilities(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "rsync -avz ./dist/ user@server:/var/www/"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["local-area-network"]},
				{alternatives: ["internet"]},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(cmd.DependsOn.Capabilities))
	}

	// First capability - local-area-network
	cap0 := cmd.DependsOn.Capabilities[0]
	if len(cap0.Alternatives) == 0 || cap0.Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("First capability alternatives = %v, want [%s]", cap0.Alternatives, CapabilityLocalAreaNetwork)
	}

	// Second capability - internet
	cap1 := cmd.DependsOn.Capabilities[1]
	if len(cap1.Alternatives) == 0 || cap1.Alternatives[0] != CapabilityInternet {
		t.Errorf("Second capability alternatives = %v, want [%s]", cap1.Alternatives, CapabilityInternet)
	}
}

func TestParseDependsOn_WithContainerAndTTYCapabilities(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "interactive-build"
		implementations: [
			{
				script: "echo 'ready'"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			capabilities: [
				{alternatives: ["containers"]},
				{alternatives: ["tty"]},
			]
		}
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(cmd.DependsOn.Capabilities))
	}

	cap0 := cmd.DependsOn.Capabilities[0]
	if len(cap0.Alternatives) == 0 || cap0.Alternatives[0] != CapabilityContainers {
		t.Errorf("First capability alternatives = %v, want [%s]", cap0.Alternatives, CapabilityContainers)
	}

	cap1 := cmd.DependsOn.Capabilities[1]
	if len(cap1.Alternatives) == 0 || cap1.Alternatives[0] != CapabilityTTY {
		t.Errorf("Second capability alternatives = %v, want [%s]", cap1.Alternatives, CapabilityTTY)
	}
}

func TestParseDependsOn_CapabilitiesAtImplementationLevel(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "sync"
		implementations: [
			{
				script: "rsync -avz ./dist/ user@server:/var/www/"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				depends_on: {
					capabilities: [
						{alternatives: ["internet"]},
					]
				}
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 1 {
		t.Fatalf("Expected 1 implementation, got %d", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if impl.DependsOn == nil {
		t.Fatal("Implementation DependsOn should not be nil")
	}

	if len(impl.DependsOn.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(impl.DependsOn.Capabilities))
	}

	if len(impl.DependsOn.Capabilities[0].Alternatives) == 0 || impl.DependsOn.Capabilities[0].Alternatives[0] != CapabilityInternet {
		t.Errorf("Capability alternatives = %v, want [%s]", impl.DependsOn.Capabilities[0].Alternatives, CapabilityInternet)
	}
}

func TestCommand_HasDependencies_WithCapabilities(t *testing.T) {
	t.Parallel()

	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
		},
	}

	if !cmd.HasDependencies() {
		t.Error("HasDependencies() should return true when capabilities are present")
	}
}

func TestCommand_HasCommandLevelDependencies_WithCapabilities(t *testing.T) {
	t.Parallel()

	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
		},
	}

	if !cmd.HasCommandLevelDependencies() {
		t.Error("HasCommandLevelDependencies() should return true when capabilities are present")
	}
}

func TestScript_HasDependencies_WithCapabilities(t *testing.T) {
	t.Parallel()

	impl := Implementation{
		Script:    "echo test",
		Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
		Platforms: []PlatformConfig{{Name: PlatformLinux}},
		DependsOn: &DependsOn{
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
		},
	}

	if !impl.HasDependencies() {
		t.Error("Implementation.HasDependencies() should return true when capabilities are present")
	}
}

func TestMergeDependsOn_WithCapabilities(t *testing.T) {
	t.Parallel()

	cmdDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
	}

	scriptDeps := &DependsOn{
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
	}

	merged := MergeDependsOnAll(nil, cmdDeps, scriptDeps)

	if merged == nil {
		t.Fatal("MergeDependsOnAll should return non-nil result")
	}

	if len(merged.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities after merge, got %d", len(merged.Capabilities))
	}

	// Command-level capabilities should come first
	if len(merged.Capabilities[0].Alternatives) == 0 || merged.Capabilities[0].Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("First capability alternatives = %v, want [%s]", merged.Capabilities[0].Alternatives, CapabilityLocalAreaNetwork)
	}

	if len(merged.Capabilities[1].Alternatives) == 0 || merged.Capabilities[1].Alternatives[0] != CapabilityInternet {
		t.Errorf("Second capability alternatives = %v, want [%s]", merged.Capabilities[1].Alternatives, CapabilityInternet)
	}
}

func TestGenerateCUE_WithCapabilities(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "deploy",
				Implementations: []Implementation{
					{
						Script:    "rsync deploy",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
				DependsOn: &DependsOn{
					Capabilities: []CapabilityDependency{
						{Alternatives: []CapabilityName{CapabilityInternet}},
						{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Check that capabilities section is present
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' section")
	}

	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}

	if !strings.Contains(result, `"local-area-network"`) {
		t.Error("GenerateCUE should include local-area-network capability")
	}
}

func TestGenerateCUE_WithCapabilitiesAtImplementationLevel(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "sync",
				Implementations: []Implementation{
					{
						Script:    "rsync sync",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
						DependsOn: &DependsOn{
							Capabilities: []CapabilityDependency{
								{Alternatives: []CapabilityName{CapabilityInternet}},
							},
						},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Check that capabilities section is present at implementation level
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' section at implementation level")
	}

	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include internet capability")
	}
}

// ============================================================================
// Tests for Root-Level Dependencies
// ============================================================================

// TestParse_RootLevelDependsOn verifies that root-level depends_on is parsed correctly
func TestParse_RootLevelDependsOn(t *testing.T) {
	t.Parallel()

	cueContent := `
depends_on: {
	tools: [{alternatives: ["sh"]}]
	capabilities: [{alternatives: ["internet"]}]
	filepaths: [{alternatives: ["/etc/hosts"], readable: true}]
	env_vars: [{alternatives: [{name: "HOME"}]}]
}

cmds: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	// Verify root-level depends_on was parsed
	if parsed.DependsOn == nil {
		t.Fatal("Invowkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.Tools) != 1 {
		t.Fatalf("Expected 1 tool dependency, got %d", len(parsed.DependsOn.Tools))
	}
	if parsed.DependsOn.Tools[0].Alternatives[0] != "sh" {
		t.Errorf("Tool alternative = %q, want %q", parsed.DependsOn.Tools[0].Alternatives[0], "sh")
	}

	if len(parsed.DependsOn.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability dependency, got %d", len(parsed.DependsOn.Capabilities))
	}
	if parsed.DependsOn.Capabilities[0].Alternatives[0] != CapabilityInternet {
		t.Errorf("Capability alternative = %v, want %v", parsed.DependsOn.Capabilities[0].Alternatives[0], CapabilityInternet)
	}

	if len(parsed.DependsOn.Filepaths) != 1 {
		t.Fatalf("Expected 1 filepath dependency, got %d", len(parsed.DependsOn.Filepaths))
	}
	if parsed.DependsOn.Filepaths[0].Alternatives[0] != "/etc/hosts" {
		t.Errorf("Filepath alternative = %q, want %q", parsed.DependsOn.Filepaths[0].Alternatives[0], "/etc/hosts")
	}
	if !parsed.DependsOn.Filepaths[0].Readable {
		t.Error("Filepath.Readable should be true")
	}

	if len(parsed.DependsOn.EnvVars) != 1 {
		t.Fatalf("Expected 1 env_var dependency, got %d", len(parsed.DependsOn.EnvVars))
	}
	if parsed.DependsOn.EnvVars[0].Alternatives[0].Name != "HOME" {
		t.Errorf("EnvVar alternative name = %q, want %q", parsed.DependsOn.EnvVars[0].Alternatives[0].Name, "HOME")
	}
}

// TestInvowkfile_HasRootLevelDependencies verifies the helper method works correctly
func TestInvowkfile_HasRootLevelDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deps     *DependsOn
		expected bool
	}{
		{
			name:     "nil depends_on",
			deps:     nil,
			expected: false,
		},
		{
			name:     "empty depends_on",
			deps:     &DependsOn{},
			expected: false,
		},
		{
			name: "with tools",
			deps: &DependsOn{
				Tools: []ToolDependency{{Alternatives: []BinaryName{"sh"}}},
			},
			expected: true,
		},
		{
			name: "with capabilities",
			deps: &DependsOn{
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
			},
			expected: true,
		},
		{
			name: "with env_vars",
			deps: &DependsOn{
				EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
			},
			expected: true,
		},
		{
			name: "with commands",
			deps: &DependsOn{
				Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}},
			},
			expected: true,
		},
		{
			name: "with custom_checks",
			deps: &DependsOn{
				CustomChecks: []CustomCheckDependency{{Name: "c", CheckScript: "true"}},
			},
			expected: true,
		},
		{
			name: "with filepaths",
			deps: &DependsOn{
				Filepaths: []FilepathDependency{{Alternatives: []string{"/etc/hosts"}}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inv := &Invowkfile{
				DependsOn: tt.deps,
				Commands:  []Command{{Name: "test", Implementations: []Implementation{{Script: "echo", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}}}},
			}
			if got := inv.HasRootLevelDependencies(); got != tt.expected {
				t.Errorf("HasRootLevelDependencies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Tests for Dependency Merging
// ============================================================================

// TestMergeDependsOnAll verifies three-way merge works correctly
func TestMergeDependsOnAll(t *testing.T) {
	t.Parallel()

	rootDeps := &DependsOn{
		Tools:        []ToolDependency{{Alternatives: []BinaryName{"sh"}}},
		Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityLocalAreaNetwork}}},
		CustomChecks: []CustomCheckDependency{{Name: "root-check", CheckScript: "true"}},
	}
	cmdDeps := &DependsOn{
		Tools:     []ToolDependency{{Alternatives: []BinaryName{"bash"}}},
		Filepaths: []FilepathDependency{{Alternatives: []string{"/etc/hosts"}}},
		Commands:  []CommandDependency{{Alternatives: []CommandName{"build"}}},
	}
	implDeps := &DependsOn{
		Tools:        []ToolDependency{{Alternatives: []BinaryName{"python3"}}},
		EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
		Commands:     []CommandDependency{{Alternatives: []CommandName{"test"}}},
		CustomChecks: []CustomCheckDependency{{Name: "impl-check", CheckScript: "echo ok"}},
	}

	merged := MergeDependsOnAll(rootDeps, cmdDeps, implDeps)

	if merged == nil {
		t.Fatal("MergeDependsOnAll should return non-nil result")
	}

	// Verify tools are merged in order: root -> command -> impl
	if len(merged.Tools) != 3 {
		t.Fatalf("Expected 3 tools after merge, got %d", len(merged.Tools))
	}
	if merged.Tools[0].Alternatives[0] != "sh" {
		t.Errorf("First tool = %q, want %q", merged.Tools[0].Alternatives[0], "sh")
	}
	if merged.Tools[1].Alternatives[0] != "bash" {
		t.Errorf("Second tool = %q, want %q", merged.Tools[1].Alternatives[0], "bash")
	}
	if merged.Tools[2].Alternatives[0] != "python3" {
		t.Errorf("Third tool = %q, want %q", merged.Tools[2].Alternatives[0], "python3")
	}

	// Verify capabilities from root
	if len(merged.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(merged.Capabilities))
	}
	if merged.Capabilities[0].Alternatives[0] != CapabilityLocalAreaNetwork {
		t.Errorf("Capability = %v, want %v", merged.Capabilities[0].Alternatives[0], CapabilityLocalAreaNetwork)
	}

	// Verify filepaths from command
	if len(merged.Filepaths) != 1 {
		t.Fatalf("Expected 1 filepath, got %d", len(merged.Filepaths))
	}
	if merged.Filepaths[0].Alternatives[0] != "/etc/hosts" {
		t.Errorf("Filepath = %q, want %q", merged.Filepaths[0].Alternatives[0], "/etc/hosts")
	}

	// Verify env_vars from impl
	if len(merged.EnvVars) != 1 {
		t.Fatalf("Expected 1 env_var, got %d", len(merged.EnvVars))
	}
	if merged.EnvVars[0].Alternatives[0].Name != "HOME" {
		t.Errorf("EnvVar name = %q, want %q", merged.EnvVars[0].Alternatives[0].Name, "HOME")
	}

	// Verify commands merged: 1 from cmd + 1 from impl = 2
	if len(merged.Commands) != 2 {
		t.Fatalf("Expected 2 commands after merge, got %d", len(merged.Commands))
	}
	if merged.Commands[0].Alternatives[0] != "build" {
		t.Errorf("First command = %q, want %q", merged.Commands[0].Alternatives[0], "build")
	}
	if merged.Commands[1].Alternatives[0] != "test" {
		t.Errorf("Second command = %q, want %q", merged.Commands[1].Alternatives[0], "test")
	}

	// Verify custom_checks merged: 1 from root + 1 from impl = 2
	if len(merged.CustomChecks) != 2 {
		t.Fatalf("Expected 2 custom_checks after merge, got %d", len(merged.CustomChecks))
	}
	if merged.CustomChecks[0].Name != "root-check" {
		t.Errorf("First custom_check = %q, want %q", merged.CustomChecks[0].Name, "root-check")
	}
	if merged.CustomChecks[1].Name != "impl-check" {
		t.Errorf("Second custom_check = %q, want %q", merged.CustomChecks[1].Name, "impl-check")
	}
}

// TestMergeDependsOnAll_NilInputs verifies three-way merge handles nil inputs
func TestMergeDependsOnAll_NilInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rootDeps *DependsOn
		cmdDeps  *DependsOn
		implDeps *DependsOn
		expected bool // true if result should be non-nil
	}{
		{
			name:     "all nil",
			rootDeps: nil,
			cmdDeps:  nil,
			implDeps: nil,
			expected: false,
		},
		{
			name:     "only root",
			rootDeps: &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"sh"}}}},
			cmdDeps:  nil,
			implDeps: nil,
			expected: true,
		},
		{
			name:     "only command",
			rootDeps: nil,
			cmdDeps:  &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"bash"}}}},
			implDeps: nil,
			expected: true,
		},
		{
			name:     "only impl",
			rootDeps: nil,
			cmdDeps:  nil,
			implDeps: &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"python3"}}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := MergeDependsOnAll(tt.rootDeps, tt.cmdDeps, tt.implDeps)
			if tt.expected && result == nil {
				t.Error("MergeDependsOnAll should return non-nil result")
			}
			if !tt.expected && result != nil {
				t.Error("MergeDependsOnAll should return nil")
			}
		})
	}
}

// TestGenerateCUE_WithRootLevelDependsOn verifies GenerateCUE produces valid CUE for root-level depends_on
func TestGenerateCUE_WithRootLevelDependsOn(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		DependsOn: &DependsOn{
			Tools:        []ToolDependency{{Alternatives: []BinaryName{"sh", "bash"}}},
			Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
			Filepaths:    []FilepathDependency{{Alternatives: []string{"/etc/hosts"}, Readable: true}},
			EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}},
		},
		Commands: []Command{
			{
				Name: "hello",
				Implementations: []Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Verify depends_on section is present at root level
	if !strings.Contains(result, "depends_on:") {
		t.Error("GenerateCUE should include 'depends_on:' section at root level")
	}
	if !strings.Contains(result, "tools:") {
		t.Error("GenerateCUE should include 'tools:' in depends_on")
	}
	if !strings.Contains(result, `"sh"`) {
		t.Error("GenerateCUE should include 'sh' tool")
	}
	if !strings.Contains(result, `"bash"`) {
		t.Error("GenerateCUE should include 'bash' tool")
	}
	if !strings.Contains(result, "capabilities:") {
		t.Error("GenerateCUE should include 'capabilities:' in depends_on")
	}
	if !strings.Contains(result, `"internet"`) {
		t.Error("GenerateCUE should include 'internet' capability")
	}
	if !strings.Contains(result, "filepaths:") {
		t.Error("GenerateCUE should include 'filepaths:' in depends_on")
	}
	if !strings.Contains(result, `"/etc/hosts"`) {
		t.Error("GenerateCUE should include filepath")
	}
	if !strings.Contains(result, "readable: true") {
		t.Error("GenerateCUE should include 'readable: true'")
	}
	if !strings.Contains(result, "env_vars:") {
		t.Error("GenerateCUE should include 'env_vars:' in depends_on")
	}
	if !strings.Contains(result, `"HOME"`) {
		t.Error("GenerateCUE should include HOME env var")
	}

	// Verify the generated CUE is parseable
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(result), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed root-level depends_on matches original
	if parsed.DependsOn == nil {
		t.Fatal("Parsed Invowkfile.DependsOn should not be nil")
	}
	if len(parsed.DependsOn.Tools) != 1 {
		t.Errorf("Expected 1 tool dependency, got %d", len(parsed.DependsOn.Tools))
	}
	if len(parsed.DependsOn.Capabilities) != 1 {
		t.Errorf("Expected 1 capability dependency, got %d", len(parsed.DependsOn.Capabilities))
	}
	if len(parsed.DependsOn.Filepaths) != 1 {
		t.Errorf("Expected 1 filepath dependency, got %d", len(parsed.DependsOn.Filepaths))
	}
	if len(parsed.DependsOn.EnvVars) != 1 {
		t.Errorf("Expected 1 env_var dependency, got %d", len(parsed.DependsOn.EnvVars))
	}
}
