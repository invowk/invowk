// SPDX-License-Identifier: MPL-2.0

package invkfiletest

import (
	"testing"

	"invowk-cli/pkg/invkfile"
)

func TestNewTestCommand_Defaults(t *testing.T) {
	cmd := NewTestCommand("hello")

	if cmd.Name != "hello" {
		t.Errorf("Name = %q, want %q", cmd.Name, "hello")
	}

	if len(cmd.Implementations) != 1 {
		t.Fatalf("len(Implementations) = %d, want 1", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if len(impl.Runtimes) != 1 {
		t.Fatalf("len(Runtimes) = %d, want 1", len(impl.Runtimes))
	}

	if impl.Runtimes[0].Name != invkfile.RuntimeNative {
		t.Errorf("Runtime = %v, want %v", impl.Runtimes[0].Name, invkfile.RuntimeNative)
	}

	if impl.Script != "" {
		t.Errorf("Script = %q, want empty", impl.Script)
	}

	if len(impl.Platforms) != 0 {
		t.Errorf("len(Platforms) = %d, want 0", len(impl.Platforms))
	}
}

func TestNewTestCommand_WithScript(t *testing.T) {
	cmd := NewTestCommand("greet", WithScript("echo hello"))

	if cmd.Implementations[0].Script != "echo hello" {
		t.Errorf("Script = %q, want %q", cmd.Implementations[0].Script, "echo hello")
	}
}

func TestNewTestCommand_WithDescription(t *testing.T) {
	cmd := NewTestCommand("greet", WithDescription("Says hello"))

	if cmd.Description != "Says hello" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Says hello")
	}
}

func TestNewTestCommand_WithRuntime(t *testing.T) {
	cmd := NewTestCommand("test", WithRuntime(invkfile.RuntimeVirtual))

	if len(cmd.Implementations[0].Runtimes) != 1 {
		t.Fatalf("len(Runtimes) = %d, want 1", len(cmd.Implementations[0].Runtimes))
	}

	if cmd.Implementations[0].Runtimes[0].Name != invkfile.RuntimeVirtual {
		t.Errorf("Runtime = %v, want %v", cmd.Implementations[0].Runtimes[0].Name, invkfile.RuntimeVirtual)
	}
}

func TestNewTestCommand_WithRuntimes(t *testing.T) {
	cmd := NewTestCommand("test",
		WithRuntimes(invkfile.RuntimeNative, invkfile.RuntimeVirtual))

	if len(cmd.Implementations[0].Runtimes) != 2 {
		t.Fatalf("len(Runtimes) = %d, want 2", len(cmd.Implementations[0].Runtimes))
	}

	if cmd.Implementations[0].Runtimes[0].Name != invkfile.RuntimeNative {
		t.Errorf("Runtimes[0] = %v, want %v", cmd.Implementations[0].Runtimes[0].Name, invkfile.RuntimeNative)
	}

	if cmd.Implementations[0].Runtimes[1].Name != invkfile.RuntimeVirtual {
		t.Errorf("Runtimes[1] = %v, want %v", cmd.Implementations[0].Runtimes[1].Name, invkfile.RuntimeVirtual)
	}
}

func TestNewTestCommand_WithRuntimeConfig(t *testing.T) {
	rc := invkfile.RuntimeConfig{
		Name:          invkfile.RuntimeContainer,
		Image:         "debian:stable-slim",
		EnableHostSSH: true,
	}
	cmd := NewTestCommand("container-cmd", WithRuntimeConfig(rc))

	if len(cmd.Implementations[0].Runtimes) != 1 {
		t.Fatalf("len(Runtimes) = %d, want 1", len(cmd.Implementations[0].Runtimes))
	}

	gotRC := cmd.Implementations[0].Runtimes[0]
	if gotRC.Name != invkfile.RuntimeContainer {
		t.Errorf("Runtime.Name = %v, want %v", gotRC.Name, invkfile.RuntimeContainer)
	}
	if gotRC.Image != "debian:stable-slim" {
		t.Errorf("Runtime.Image = %q, want %q", gotRC.Image, "debian:stable-slim")
	}
	if !gotRC.EnableHostSSH {
		t.Error("Runtime.EnableHostSSH = false, want true")
	}
}

func TestNewTestCommand_WithPlatform(t *testing.T) {
	cmd := NewTestCommand("test", WithPlatform(invkfile.PlatformLinux))

	if len(cmd.Implementations[0].Platforms) != 1 {
		t.Fatalf("len(Platforms) = %d, want 1", len(cmd.Implementations[0].Platforms))
	}

	if cmd.Implementations[0].Platforms[0].Name != invkfile.PlatformLinux {
		t.Errorf("Platform = %v, want %v", cmd.Implementations[0].Platforms[0].Name, invkfile.PlatformLinux)
	}
}

func TestNewTestCommand_WithPlatforms(t *testing.T) {
	cmd := NewTestCommand("test",
		WithPlatforms(invkfile.PlatformLinux, invkfile.PlatformMac))

	if len(cmd.Implementations[0].Platforms) != 2 {
		t.Fatalf("len(Platforms) = %d, want 2", len(cmd.Implementations[0].Platforms))
	}

	if cmd.Implementations[0].Platforms[0].Name != invkfile.PlatformLinux {
		t.Errorf("Platforms[0] = %v, want %v", cmd.Implementations[0].Platforms[0].Name, invkfile.PlatformLinux)
	}

	if cmd.Implementations[0].Platforms[1].Name != invkfile.PlatformMac {
		t.Errorf("Platforms[1] = %v, want %v", cmd.Implementations[0].Platforms[1].Name, invkfile.PlatformMac)
	}
}

func TestNewTestCommand_WithEnv(t *testing.T) {
	cmd := NewTestCommand("test",
		WithEnv("FOO", "bar"),
		WithEnv("BAZ", "qux"))

	if cmd.Env == nil {
		t.Fatal("Env is nil")
	}

	if cmd.Env.Vars["FOO"] != "bar" {
		t.Errorf("Env.Vars[FOO] = %q, want %q", cmd.Env.Vars["FOO"], "bar")
	}

	if cmd.Env.Vars["BAZ"] != "qux" {
		t.Errorf("Env.Vars[BAZ] = %q, want %q", cmd.Env.Vars["BAZ"], "qux")
	}
}

func TestNewTestCommand_WithWorkDir(t *testing.T) {
	cmd := NewTestCommand("test", WithWorkDir("/tmp/work"))

	if cmd.WorkDir != "/tmp/work" {
		t.Errorf("WorkDir = %q, want %q", cmd.WorkDir, "/tmp/work")
	}
}

func TestNewTestCommand_WithFlag(t *testing.T) {
	cmd := NewTestCommand("deploy",
		WithFlag("env",
			FlagRequired(),
			FlagDefault("staging"),
			FlagShorthand("e"),
			FlagDescription("Target environment"),
			FlagType(invkfile.FlagTypeString),
			FlagValidation("^(dev|staging|prod)$"),
		))

	if len(cmd.Flags) != 1 {
		t.Fatalf("len(Flags) = %d, want 1", len(cmd.Flags))
	}

	flag := cmd.Flags[0]
	if flag.Name != "env" {
		t.Errorf("Flag.Name = %q, want %q", flag.Name, "env")
	}
	if !flag.Required {
		t.Error("Flag.Required = false, want true")
	}
	if flag.DefaultValue != "staging" {
		t.Errorf("Flag.DefaultValue = %q, want %q", flag.DefaultValue, "staging")
	}
	if flag.Short != "e" {
		t.Errorf("Flag.Short = %q, want %q", flag.Short, "e")
	}
	if flag.Description != "Target environment" {
		t.Errorf("Flag.Description = %q, want %q", flag.Description, "Target environment")
	}
	if flag.Type != invkfile.FlagTypeString {
		t.Errorf("Flag.Type = %v, want %v", flag.Type, invkfile.FlagTypeString)
	}
	if flag.Validation != "^(dev|staging|prod)$" {
		t.Errorf("Flag.Validation = %q, want %q", flag.Validation, "^(dev|staging|prod)$")
	}
}

func TestNewTestCommand_WithMultipleFlags(t *testing.T) {
	cmd := NewTestCommand("cmd",
		WithFlag("verbose", FlagType(invkfile.FlagTypeBool)),
		WithFlag("count", FlagType(invkfile.FlagTypeInt), FlagDefault("5")))

	if len(cmd.Flags) != 2 {
		t.Fatalf("len(Flags) = %d, want 2", len(cmd.Flags))
	}

	if cmd.Flags[0].Name != "verbose" {
		t.Errorf("Flags[0].Name = %q, want %q", cmd.Flags[0].Name, "verbose")
	}
	if cmd.Flags[1].Name != "count" {
		t.Errorf("Flags[1].Name = %q, want %q", cmd.Flags[1].Name, "count")
	}
}

func TestNewTestCommand_WithArg(t *testing.T) {
	cmd := NewTestCommand("copy",
		WithArg("source",
			ArgRequired(),
			ArgDescription("Source file"),
			ArgType(invkfile.ArgumentTypeString),
			ArgValidation("^[a-z]+$"),
		),
		WithArg("dest",
			ArgDefault("/tmp"),
			ArgDescription("Destination"),
		))

	if len(cmd.Args) != 2 {
		t.Fatalf("len(Args) = %d, want 2", len(cmd.Args))
	}

	arg0 := cmd.Args[0]
	if arg0.Name != "source" {
		t.Errorf("Args[0].Name = %q, want %q", arg0.Name, "source")
	}
	if !arg0.Required {
		t.Error("Args[0].Required = false, want true")
	}
	if arg0.Description != "Source file" {
		t.Errorf("Args[0].Description = %q, want %q", arg0.Description, "Source file")
	}
	if arg0.Type != invkfile.ArgumentTypeString {
		t.Errorf("Args[0].Type = %v, want %v", arg0.Type, invkfile.ArgumentTypeString)
	}
	if arg0.Validation != "^[a-z]+$" {
		t.Errorf("Args[0].Validation = %q, want %q", arg0.Validation, "^[a-z]+$")
	}

	arg1 := cmd.Args[1]
	if arg1.Name != "dest" {
		t.Errorf("Args[1].Name = %q, want %q", arg1.Name, "dest")
	}
	if arg1.DefaultValue != "/tmp" {
		t.Errorf("Args[1].DefaultValue = %q, want %q", arg1.DefaultValue, "/tmp")
	}
}

func TestNewTestCommand_WithVariadicArg(t *testing.T) {
	cmd := NewTestCommand("echo",
		WithArg("messages", ArgVariadic()))

	if len(cmd.Args) != 1 {
		t.Fatalf("len(Args) = %d, want 1", len(cmd.Args))
	}

	if !cmd.Args[0].Variadic {
		t.Error("Args[0].Variadic = false, want true")
	}
}

func TestNewTestCommand_WithImplementation(t *testing.T) {
	impl := invkfile.Implementation{
		Script: "echo linux",
		Runtimes: []invkfile.RuntimeConfig{
			{Name: invkfile.RuntimeNative},
		},
		Platforms: []invkfile.PlatformConfig{
			{Name: invkfile.PlatformLinux},
		},
	}
	cmd := NewTestCommand("multi", WithImplementation(impl))

	// Should have default + added = 2 implementations
	if len(cmd.Implementations) != 2 {
		t.Fatalf("len(Implementations) = %d, want 2", len(cmd.Implementations))
	}

	if cmd.Implementations[1].Script != "echo linux" {
		t.Errorf("Implementations[1].Script = %q, want %q", cmd.Implementations[1].Script, "echo linux")
	}
}

func TestNewTestCommand_WithImplementations(t *testing.T) {
	impls := []invkfile.Implementation{
		{
			Script:   "echo linux",
			Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
		},
		{
			Script:   "echo mac",
			Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
		},
	}
	cmd := NewTestCommand("multi", WithImplementations(impls...))

	// WithImplementations replaces all, so should have exactly 2
	if len(cmd.Implementations) != 2 {
		t.Fatalf("len(Implementations) = %d, want 2", len(cmd.Implementations))
	}

	if cmd.Implementations[0].Script != "echo linux" {
		t.Errorf("Implementations[0].Script = %q, want %q", cmd.Implementations[0].Script, "echo linux")
	}
	if cmd.Implementations[1].Script != "echo mac" {
		t.Errorf("Implementations[1].Script = %q, want %q", cmd.Implementations[1].Script, "echo mac")
	}
}

func TestNewTestCommand_WithDependsOn(t *testing.T) {
	deps := &invkfile.DependsOn{
		Tools: []invkfile.ToolDependency{
			{Alternatives: []string{"git"}},
		},
		Commands: []invkfile.CommandDependency{
			{Alternatives: []string{"build"}},
		},
	}
	cmd := NewTestCommand("deploy", WithScript("echo deploy"), WithDependsOn(deps))

	if cmd.DependsOn == nil {
		t.Fatal("DependsOn is nil")
	}
	if len(cmd.DependsOn.Tools) != 1 {
		t.Errorf("len(DependsOn.Tools) = %d, want 1", len(cmd.DependsOn.Tools))
	}
	if cmd.DependsOn.Tools[0].Alternatives[0] != "git" {
		t.Errorf("DependsOn.Tools[0].Alternatives[0] = %q, want %q", cmd.DependsOn.Tools[0].Alternatives[0], "git")
	}
	if len(cmd.DependsOn.Commands) != 1 {
		t.Errorf("len(DependsOn.Commands) = %d, want 1", len(cmd.DependsOn.Commands))
	}
	if cmd.DependsOn.Commands[0].Alternatives[0] != "build" {
		t.Errorf("DependsOn.Commands[0].Alternatives[0] = %q, want %q", cmd.DependsOn.Commands[0].Alternatives[0], "build")
	}
}

func TestNewTestCommand_CombinedOptions(t *testing.T) {
	cmd := NewTestCommand("deploy",
		WithDescription("Deploy the app"),
		WithScript("./deploy.sh $ENV"),
		WithRuntime(invkfile.RuntimeNative),
		WithPlatform(invkfile.PlatformLinux),
		WithEnv("APP_NAME", "myapp"),
		WithWorkDir("/app"),
		WithFlag("env", FlagRequired(), FlagShorthand("e")),
		WithArg("version", ArgRequired()),
	)

	if cmd.Name != "deploy" {
		t.Errorf("Name = %q, want %q", cmd.Name, "deploy")
	}
	if cmd.Description != "Deploy the app" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Deploy the app")
	}
	if cmd.Implementations[0].Script != "./deploy.sh $ENV" {
		t.Errorf("Script = %q, want %q", cmd.Implementations[0].Script, "./deploy.sh $ENV")
	}
	if cmd.Implementations[0].Runtimes[0].Name != invkfile.RuntimeNative {
		t.Errorf("Runtime = %v, want %v", cmd.Implementations[0].Runtimes[0].Name, invkfile.RuntimeNative)
	}
	if cmd.Implementations[0].Platforms[0].Name != invkfile.PlatformLinux {
		t.Errorf("Platform = %v, want %v", cmd.Implementations[0].Platforms[0].Name, invkfile.PlatformLinux)
	}
	if cmd.Env.Vars["APP_NAME"] != "myapp" {
		t.Errorf("Env APP_NAME = %q, want %q", cmd.Env.Vars["APP_NAME"], "myapp")
	}
	if cmd.WorkDir != "/app" {
		t.Errorf("WorkDir = %q, want %q", cmd.WorkDir, "/app")
	}
	if len(cmd.Flags) != 1 || cmd.Flags[0].Name != "env" {
		t.Error("Flags not set correctly")
	}
	if len(cmd.Args) != 1 || cmd.Args[0].Name != "version" {
		t.Error("Args not set correctly")
	}
}
