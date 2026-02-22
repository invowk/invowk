// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCommand creates a Command with a single script for testing purposes
func testCommand(name, script string) Command {
	return Command{
		Name: CommandName(name),
		Implementations: []Implementation{
			{Script: ScriptContent(script), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}},
		},
	}
}

// testCommandWithDeps creates a Command with a single script and dependencies for testing
func testCommandWithDeps(name, script string, deps *DependsOn) Command {
	return Command{
		Name:            CommandName(name),
		Implementations: []Implementation{{Script: ScriptContent(script), Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}},
		DependsOn:       deps,
	}
}

func TestDependsOn_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deps     DependsOn
		expected bool
	}{
		{name: "zero value", deps: DependsOn{}, expected: true},
		{name: "only tools", deps: DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"sh"}}}}, expected: false},
		{name: "only commands", deps: DependsOn{Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}}}, expected: false},
		{name: "only filepaths", deps: DependsOn{Filepaths: []FilepathDependency{{Alternatives: []string{"f.txt"}}}}, expected: false},
		{name: "only capabilities", deps: DependsOn{Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}}}, expected: false},
		{name: "only custom_checks", deps: DependsOn{CustomChecks: []CustomCheckDependency{{Name: "c", CheckScript: "true"}}}, expected: false},
		{name: "only env_vars", deps: DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}}}, expected: false},
		{
			name: "all populated",
			deps: DependsOn{
				Tools:        []ToolDependency{{Alternatives: []BinaryName{"sh"}}},
				Commands:     []CommandDependency{{Alternatives: []CommandName{"b"}}},
				Filepaths:    []FilepathDependency{{Alternatives: []string{"f"}}},
				Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}},
				CustomChecks: []CustomCheckDependency{{Name: "c", CheckScript: "true"}},
				EnvVars:      []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "X"}}}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.deps.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCommand_HasDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      Command
		expected bool
	}{
		{
			name:     "nil DependsOn",
			cmd:      testCommand("test", "echo"),
			expected: false,
		},
		{
			name:     "empty DependsOn",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{}),
			expected: false,
		},
		{
			name:     "only tools",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"git"}}}}),
			expected: true,
		},
		{
			name:     "only commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}}}),
			expected: true,
		},
		{
			name: "both tools and commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Tools:    []ToolDependency{{Alternatives: []BinaryName{"git"}}},
				Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}},
			}),
			expected: true,
		},
		{
			name:     "only env_vars",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}}}),
			expected: true,
		},
		{
			name:     "only custom_checks",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{CustomChecks: []CustomCheckDependency{{Name: "c", CheckScript: "true"}}}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.cmd.HasDependencies()
			if result != tt.expected {
				t.Errorf("HasDependencies() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_GetCommandDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      Command
		expected []CommandName
	}{
		{
			name:     "nil DependsOn",
			cmd:      testCommand("test", "echo"),
			expected: nil,
		},
		{
			name:     "empty DependsOn",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{}),
			expected: []CommandName{},
		},
		{
			name:     "single command",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Commands: []CommandDependency{{Alternatives: []CommandName{"build"}}}}),
			expected: []CommandName{"build"},
		},
		{
			name: "multiple commands",
			cmd: testCommandWithDeps("test", "echo", &DependsOn{
				Commands: []CommandDependency{
					{Alternatives: []CommandName{"clean"}},
					{Alternatives: []CommandName{"build"}},
					{Alternatives: []CommandName{"test unit"}},
				},
			}),
			expected: []CommandName{"clean", "build", "test unit"},
		},
		{
			name:     "only tools no commands",
			cmd:      testCommandWithDeps("test", "echo", &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"git"}}}}),
			expected: []CommandName{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.cmd.GetCommandDependencies()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("GetCommandDependencies() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("GetCommandDependencies() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, name := range result {
				if name != tt.expected[i] {
					t.Errorf("GetCommandDependencies()[%d] = %q, want %q", i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestCommand_HasCommandLevelDependencies_AllTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deps     *DependsOn
		expected bool
	}{
		{name: "nil", deps: nil, expected: false},
		{name: "empty", deps: &DependsOn{}, expected: false},
		{name: "tools", deps: &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"sh"}}}}, expected: true},
		{name: "commands", deps: &DependsOn{Commands: []CommandDependency{{Alternatives: []CommandName{"b"}}}}, expected: true},
		{name: "filepaths", deps: &DependsOn{Filepaths: []FilepathDependency{{Alternatives: []string{"f"}}}}, expected: true},
		{name: "capabilities", deps: &DependsOn{Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}}}, expected: true},
		{name: "custom_checks", deps: &DependsOn{CustomChecks: []CustomCheckDependency{{Name: "c", CheckScript: "true"}}}, expected: true},
		{name: "env_vars", deps: &DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{Name: "HOME"}}}}}, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := Command{
				Name:            "test",
				Implementations: []Implementation{{Script: "echo", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: AllPlatformConfigs()}},
				DependsOn:       tt.deps,
			}
			if got := cmd.HasCommandLevelDependencies(); got != tt.expected {
				t.Errorf("HasCommandLevelDependencies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseDependsOn(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "release"
		implementations: [
			{
				script: "echo releasing"

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["git"]},
				{alternatives: ["docker"]},
			]
			cmds: [
				{alternatives: ["build"]},
				{alternatives: ["test unit"]},
			]
			custom_checks: [
				{name: "docker-version", check_script: "docker --version", expected_output: "Docker"},
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

	// Check tools
	if len(cmd.DependsOn.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Tools[0].Alternatives) == 0 || cmd.DependsOn.Tools[0].Alternatives[0] != "git" {
		t.Errorf("First tool alternatives = %v, want [git]", cmd.DependsOn.Tools[0].Alternatives)
	}

	if len(cmd.DependsOn.Tools[1].Alternatives) == 0 || cmd.DependsOn.Tools[1].Alternatives[0] != "docker" {
		t.Errorf("Second tool alternatives = %v, want [docker]", cmd.DependsOn.Tools[1].Alternatives)
	}

	// Check custom_checks
	if len(cmd.DependsOn.CustomChecks) != 1 {
		t.Errorf("Expected 1 custom_check, got %d", len(cmd.DependsOn.CustomChecks))
	}

	checks := cmd.DependsOn.CustomChecks[0].GetChecks()
	if len(checks) == 0 {
		t.Fatal("Expected at least one check from CustomCheckDependency")
	}

	if checks[0].Name != "docker-version" {
		t.Errorf("First custom_check name = %q, want %q", checks[0].Name, "docker-version")
	}

	if checks[0].CheckScript != "docker --version" {
		t.Errorf("First custom_check check_script = %q, want %q", checks[0].CheckScript, "docker --version")
	}

	if checks[0].ExpectedOutput != "Docker" {
		t.Errorf("First custom_check expected_output = %q, want %q", checks[0].ExpectedOutput, "Docker")
	}

	// Check commands
	if len(cmd.DependsOn.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmd.DependsOn.Commands))
	}

	if len(cmd.DependsOn.Commands[0].Alternatives) == 0 || cmd.DependsOn.Commands[0].Alternatives[0] != "build" {
		t.Errorf("First command alternatives = %v, want [build]", cmd.DependsOn.Commands[0].Alternatives)
	}

	if len(cmd.DependsOn.Commands[1].Alternatives) == 0 || cmd.DependsOn.Commands[1].Alternatives[0] != "test unit" {
		t.Errorf("Second command alternatives = %v, want [test unit]", cmd.DependsOn.Commands[1].Alternatives)
	}
}

func TestParseDependsOn_ToolsOnly(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["make"]},
				{alternatives: ["gcc"]},
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

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(cmd.DependsOn.Commands))
	}
}

func TestParseDependsOn_CommandsOnly(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "release"
		implementations: [
			{
				script: "echo release"

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			cmds: [
				{alternatives: ["build"]},
				{alternatives: ["test"]},
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

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(cmd.DependsOn.Tools))
	}

	if len(cmd.DependsOn.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(cmd.DependsOn.Commands))
	}
}

func TestParseDependsOn_WithCustomChecks(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["make"]},
				{alternatives: ["go"]},
			]
			custom_checks: [
				{
					name: "go-version"
					check_script: "go version"
					expected_code: 0
					expected_output: "go1\\."
				},
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

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.Tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(cmd.DependsOn.Tools))
	}

	// First tool - simple
	if len(cmd.DependsOn.Tools[0].Alternatives) == 0 || cmd.DependsOn.Tools[0].Alternatives[0] != "make" {
		t.Errorf("First tool alternatives = %v, want [make]", cmd.DependsOn.Tools[0].Alternatives)
	}

	// Second tool - simple
	if len(cmd.DependsOn.Tools[1].Alternatives) == 0 || cmd.DependsOn.Tools[1].Alternatives[0] != "go" {
		t.Errorf("Second tool alternatives = %v, want [go]", cmd.DependsOn.Tools[1].Alternatives)
	}

	// Custom check with validation
	if len(cmd.DependsOn.CustomChecks) != 1 {
		t.Fatalf("Expected 1 custom_check, got %d", len(cmd.DependsOn.CustomChecks))
	}

	checks := cmd.DependsOn.CustomChecks[0].GetChecks()
	if len(checks) == 0 {
		t.Fatal("Expected at least one check from CustomCheckDependency")
	}
	goCheck := checks[0]
	if goCheck.Name != "go-version" {
		t.Errorf("Custom check name = %q, want %q", goCheck.Name, "go-version")
	}
	if goCheck.CheckScript != "go version" {
		t.Errorf("Custom check check_script = %q, want %q", goCheck.CheckScript, "go version")
	}
	if goCheck.ExpectedCode == nil {
		t.Error("Custom check expected_code should not be nil")
	} else if *goCheck.ExpectedCode != 0 {
		t.Errorf("Custom check expected_code = %d, want 0", *goCheck.ExpectedCode)
	}
	if goCheck.ExpectedOutput != "go1\\." {
		t.Errorf("Custom check expected_output = %q, want %q", goCheck.ExpectedOutput, "go1\\.")
	}
}

func TestParseDependsOn_WithFilepaths(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			filepaths: [
				{alternatives: ["config.yaml"]},
				{alternatives: ["secrets.env"], readable: true},
				{alternatives: ["output"], writable: true},
				{alternatives: ["scripts/deploy.sh"], executable: true},
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

	if len(cmd.DependsOn.Filepaths) != 4 {
		t.Fatalf("Expected 4 filepaths, got %d", len(cmd.DependsOn.Filepaths))
	}

	// First filepath - simple existence check
	fp0 := cmd.DependsOn.Filepaths[0]
	if len(fp0.Alternatives) != 1 || fp0.Alternatives[0] != "config.yaml" {
		t.Errorf("First filepath alternatives = %v, want [config.yaml]", fp0.Alternatives)
	}
	if fp0.Readable || fp0.Writable || fp0.Executable {
		t.Error("First filepath should have no permission checks")
	}

	// Second filepath - readable
	fp1 := cmd.DependsOn.Filepaths[1]
	if len(fp1.Alternatives) != 1 || fp1.Alternatives[0] != "secrets.env" {
		t.Errorf("Second filepath alternatives = %v, want [secrets.env]", fp1.Alternatives)
	}
	if !fp1.Readable {
		t.Error("Second filepath should be readable")
	}

	// Third filepath - writable
	fp2 := cmd.DependsOn.Filepaths[2]
	if len(fp2.Alternatives) != 1 || fp2.Alternatives[0] != "output" {
		t.Errorf("Third filepath alternatives = %v, want [output]", fp2.Alternatives)
	}
	if !fp2.Writable {
		t.Error("Third filepath should be writable")
	}

	// Fourth filepath - executable
	fp3 := cmd.DependsOn.Filepaths[3]
	if len(fp3.Alternatives) != 1 || fp3.Alternatives[0] != "scripts/deploy.sh" {
		t.Errorf("Fourth filepath alternatives = %v, want [scripts/deploy.sh]", fp3.Alternatives)
	}
	if !fp3.Executable {
		t.Error("Fourth filepath should be executable")
	}
}

func TestCommand_HasDependencies_WithFilepaths(t *testing.T) {
	t.Parallel()

	cmd := Command{
		Name:            "test",
		Implementations: []Implementation{{Script: "echo", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}}}},
		DependsOn: &DependsOn{
			Filepaths: []FilepathDependency{{Alternatives: []string{"config.yaml"}}},
		},
	}

	if !cmd.HasDependencies() {
		t.Error("HasDependencies() should return true when filepaths are present")
	}
}

func TestGenerateCUE_WithFilepaths(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name:            "deploy",
				Implementations: []Implementation{{Script: "echo deploy", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}, {Name: PlatformMac}}}},
				DependsOn: &DependsOn{
					Filepaths: []FilepathDependency{
						{Alternatives: []string{"config.yaml"}},
						{Alternatives: []string{"secrets.env"}, Readable: true},
						{Alternatives: []string{"output"}, Writable: true},
						{Alternatives: []string{"deploy.sh"}, Executable: true},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check that filepaths structure is present
	if !strings.Contains(output, "filepaths:") {
		t.Error("GenerateCUE should contain 'filepaths:'")
	}

	if !strings.Contains(output, "alternatives:") {
		t.Error("GenerateCUE should contain 'alternatives:'")
	}

	if !strings.Contains(output, `"config.yaml"`) {
		t.Error("GenerateCUE should contain config.yaml")
	}

	if !strings.Contains(output, "readable: true") {
		t.Error("GenerateCUE should contain readable flag")
	}

	if !strings.Contains(output, "writable: true") {
		t.Error("GenerateCUE should contain writable flag")
	}

	if !strings.Contains(output, "executable: true") {
		t.Error("GenerateCUE should contain executable flag")
	}
}
