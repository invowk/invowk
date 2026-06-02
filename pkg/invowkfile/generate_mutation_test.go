// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestGenerateCUE_RootCommandAndImplementationRoundTrip(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		DefaultShell: "/bin/bash",
		WorkDir:      "workspace",
		Env: &EnvConfig{
			Files: []DotenvFilePath{".env", ".env.local?"},
			Vars: map[EnvVarName]string{
				"A_VAR": "alpha",
				"Z_VAR": "zed",
			},
		},
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{"git", "hub"}}},
		},
		Commands: []Command{
			{
				Name:        "build",
				Description: "Build app",
				Implementations: []Implementation{{
					Script:    ImplementationScript{Content: "echo build"},
					Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
				}},
			},
			{
				Name:        "deploy",
				Description: "Deploy app",
				Category:    "Release",
				Env: &EnvConfig{
					Files: []DotenvFilePath{".deploy.env"},
					Vars:  map[EnvVarName]string{"DEPLOY_ENV": "prod"},
				},
				WorkDir: "cmd/deploy",
				DependsOn: &DependsOn{
					Commands: []CommandDependency{{Alternatives: []CommandDependencyRef{"build"}}},
				},
				Flags: []Flag{
					{
						Name:        "target",
						Description: "Target environment",
						Required:    true,
						Short:       "t",
						Validation:  "^(dev|prod)$",
					},
					{
						Name:         "dry-run",
						Description:  "Preview changes",
						DefaultValue: "false",
						Type:         FlagTypeBool,
					},
				},
				Args: []Argument{
					{
						Name:        "count",
						Description: "Deploy count",
						Required:    true,
						Type:        ArgumentTypeInt,
						Validation:  "^[0-9]+$",
					},
					{
						Name:         "files",
						Description:  "Files to deploy",
						DefaultValue: "all",
						Variadic:     true,
					},
				},
				Watch: &WatchConfig{
					Patterns:    []GlobPattern{"src/**/*.go", "config/**/*.cue"},
					Debounce:    "250ms",
					ClearScreen: true,
					Ignore:      []GlobPattern{"**/vendor/**", "**/.git/**"},
				},
				Implementations: []Implementation{{
					Script:    ImplementationScript{Content: "echo deploy"},
					Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
					Env: &EnvConfig{
						Files: []DotenvFilePath{".impl.env"},
						Vars:  map[EnvVarName]string{"IMPL_ENV": "1"},
					},
					DependsOn: &DependsOn{
						Filepaths: []FilepathDependency{{
							Alternatives: []FilesystemPath{"./config.yaml", "./config.yml"},
							Readable:     true,
							Writable:     true,
							Executable:   true,
						}},
						Capabilities: []CapabilityDependency{{
							Alternatives: []CapabilityName{CapabilityInternet, CapabilityContainers},
						}},
					},
					WorkDir: "impl/deploy",
					Timeout: "30s",
				}},
			},
		},
	}

	parsed, generated := parseGeneratedCUEMutationFixture(t, inv)
	requireGeneratedContains(t, generated, "// Invowkfile - Command definitions for invowk")
	requireGeneratedContains(t, generated, "// See https://github.com/invowk/invowk for documentation")

	if parsed.DefaultShell != "/bin/bash" {
		t.Fatalf("DefaultShell = %q, want /bin/bash", parsed.DefaultShell)
	}
	if parsed.WorkDir != "workspace" {
		t.Fatalf("WorkDir = %q, want workspace", parsed.WorkDir)
	}
	if parsed.Env == nil || len(parsed.Env.Files) != 2 || parsed.Env.Vars["A_VAR"] != "alpha" || parsed.Env.Vars["Z_VAR"] != "zed" {
		t.Fatalf("root Env = %+v, want files and vars preserved", parsed.Env)
	}
	if parsed.DependsOn == nil || len(parsed.DependsOn.Tools) != 1 ||
		len(parsed.DependsOn.Tools[0].Alternatives) != 2 || parsed.DependsOn.Tools[0].Alternatives[1] != "hub" {
		t.Fatalf("root DependsOn = %+v, want git/hub tool alternatives", parsed.DependsOn)
	}

	if len(parsed.Commands) != 2 {
		t.Fatalf("Commands length = %d, want 2", len(parsed.Commands))
	}
	deploy := parsed.Commands[1]
	if deploy.Name != "deploy" || deploy.Description != "Deploy app" || deploy.Category != "Release" {
		t.Fatalf("deploy command fields = %+v", deploy)
	}
	if deploy.Env == nil || len(deploy.Env.Files) != 1 || deploy.Env.Vars["DEPLOY_ENV"] != "prod" {
		t.Fatalf("deploy Env = %+v, want command env preserved", deploy.Env)
	}
	if deploy.WorkDir != "cmd/deploy" {
		t.Fatalf("deploy WorkDir = %q, want cmd/deploy", deploy.WorkDir)
	}
	if deploy.DependsOn == nil || deploy.DependsOn.Commands[0].Alternatives[0] != "build" {
		t.Fatalf("deploy DependsOn = %+v, want build command dependency", deploy.DependsOn)
	}

	if len(deploy.Flags) != 2 {
		t.Fatalf("deploy Flags length = %d, want 2", len(deploy.Flags))
	}
	if !deploy.Flags[0].Required || deploy.Flags[0].Short != "t" || deploy.Flags[0].Validation != "^(dev|prod)$" {
		t.Fatalf("target flag = %+v, want required short validation preserved", deploy.Flags[0])
	}
	if deploy.Flags[1].DefaultValue != "false" || deploy.Flags[1].GetType() != FlagTypeBool {
		t.Fatalf("dry-run flag = %+v, want false bool default", deploy.Flags[1])
	}

	if len(deploy.Args) != 2 {
		t.Fatalf("deploy Args length = %d, want 2", len(deploy.Args))
	}
	if !deploy.Args[0].Required || deploy.Args[0].GetType() != ArgumentTypeInt || deploy.Args[0].Validation != "^[0-9]+$" {
		t.Fatalf("count arg = %+v, want required int validation preserved", deploy.Args[0])
	}
	if deploy.Args[1].DefaultValue != "all" || !deploy.Args[1].Variadic {
		t.Fatalf("files arg = %+v, want default variadic preserved", deploy.Args[1])
	}

	if deploy.Watch == nil || len(deploy.Watch.Patterns) != 2 || deploy.Watch.Debounce != "250ms" ||
		!deploy.Watch.ClearScreen || len(deploy.Watch.Ignore) != 2 {
		t.Fatalf("Watch = %+v, want full watch config preserved", deploy.Watch)
	}

	impl := deploy.Implementations[0]
	if impl.Env == nil || len(impl.Env.Files) != 1 || impl.Env.Vars["IMPL_ENV"] != "1" {
		t.Fatalf("implementation Env = %+v, want env preserved", impl.Env)
	}
	if impl.WorkDir != "impl/deploy" || impl.Timeout != "30s" {
		t.Fatalf("implementation WorkDir/Timeout = %q/%q", impl.WorkDir, impl.Timeout)
	}
	if impl.DependsOn == nil || len(impl.DependsOn.Filepaths) != 1 ||
		!impl.DependsOn.Filepaths[0].Readable || !impl.DependsOn.Filepaths[0].Writable || !impl.DependsOn.Filepaths[0].Executable {
		t.Fatalf("implementation Filepaths = %+v, want permission flags preserved", impl.DependsOn)
	}
	if len(impl.DependsOn.Capabilities) != 1 || impl.DependsOn.Capabilities[0].Alternatives[1] != CapabilityContainers {
		t.Fatalf("implementation Capabilities = %+v, want alternatives preserved", impl.DependsOn.Capabilities)
	}
}

func TestGenerateCUE_OmitsEmptyOptionalBlocks(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Env:       &EnvConfig{},
		DependsOn: &DependsOn{},
		Commands: []Command{{
			Name:      "minimal",
			Env:       &EnvConfig{},
			DependsOn: &DependsOn{},
			Watch:     &WatchConfig{},
			Implementations: []Implementation{{
				Script:    ImplementationScript{Content: "echo minimal"},
				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: []PlatformConfig{{Name: PlatformLinux, Virtual: &PlatformVirtualConfig{Filesystem: &VirtualFilesystemConfig{}}}},
				Env:       &EnvConfig{},
				DependsOn: &DependsOn{},
			}},
		}},
	}

	_, generated := parseGeneratedCUEMutationFixture(t, inv)
	for _, forbidden := range []string{"env:", "depends_on:", "watch:", "virtual:", "filesystem:", "paths:"} {
		requireGeneratedNotContains(t, generated, forbidden)
	}
}

func TestGenerateCUE_RuntimeFieldVariantsRoundTrip(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "runtime-fields",
			Implementations: []Implementation{
				{
					Script: ImplementationScript{Content: "echo virtual sh"},
					Runtimes: []RuntimeConfig{{
						Name:             RuntimeVirtualSh,
						EnvInheritMode:   EnvInheritAllow,
						EnvInheritAllow:  []EnvVarName{"PATH", "TERM"},
						EnvInheritDeny:   []EnvVarName{"SECRET_TOKEN"},
						AllowedBinaries:  []AllowedBinary{"git", "sh"},
						BinaryLookupMode: BinaryLookupModeStrict,
					}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
				},
				{
					Script: ImplementationScript{Content: "print('lua')"},
					Runtimes: []RuntimeConfig{{
						Name:             RuntimeVirtualLua,
						AllowedBinaries:  []AllowedBinary{"node"},
						BinaryLookupMode: BinaryLookupModeHost,
						CPULimit:         LuaCPULimit(1234),
						MemoryLimit:      MemoryLimit("64MB"),
					}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
				},
				{
					Script: ImplementationScript{Content: "echo image"},
					Runtimes: []RuntimeConfig{{
						Name:          RuntimeContainer,
						Image:         "debian:stable-slim",
						EnableHostSSH: true,
						Volumes:       []VolumeMountSpec{"./data:/data", "./cache:/cache:ro"},
						Ports:         []PortMappingSpec{"8080:80", "3000:3000"},
						Persistent:    &RuntimePersistentConfig{Name: "existing_dev"},
					}},
					Platforms: []PlatformConfig{{Name: PlatformLinux}},
				},
				{
					Script: ImplementationScript{Content: "echo containerfile"},
					Runtimes: []RuntimeConfig{{
						Name:          RuntimeContainer,
						Containerfile: "Containerfile",
						Persistent:    &RuntimePersistentConfig{CreateIfMissing: true},
					}},
					Platforms: []PlatformConfig{{Name: PlatformMac}},
				},
			},
		}},
	}

	parsed, generated := parseGeneratedCUEMutationFixture(t, inv)
	for _, want := range []string{
		`env_inherit_mode: "allow"`,
		`env_inherit_allow: ["PATH", "TERM"]`,
		`env_inherit_deny: ["SECRET_TOKEN"]`,
		`allowed_binaries: ["git", "sh"]`,
		`binary_lookup_mode: "strict"`,
		`cpu_limit: 1234`,
		`memory_limit: "64MB"`,
		`enable_host_ssh: true`,
		`volumes: ["./data:/data", "./cache:/cache:ro"]`,
		`ports: ["8080:80", "3000:3000"]`,
		`persistent: {name: "existing_dev"}`,
		`containerfile: "Containerfile"`,
		`persistent: {create_if_missing: true}`,
	} {
		requireGeneratedContains(t, generated, want)
	}

	implementations := parsed.Commands[0].Implementations
	virtualSh := implementations[0].Runtimes[0]
	if virtualSh.EnvInheritMode != EnvInheritAllow || len(virtualSh.EnvInheritAllow) != 2 ||
		len(virtualSh.EnvInheritDeny) != 1 || len(virtualSh.AllowedBinaries) != 2 ||
		virtualSh.BinaryLookupMode != BinaryLookupModeStrict {
		t.Fatalf("virtual-sh runtime = %+v, want env and binary policy preserved", virtualSh)
	}

	virtualLua := implementations[1].Runtimes[0]
	if len(virtualLua.AllowedBinaries) != 1 || virtualLua.BinaryLookupMode != BinaryLookupModeHost ||
		virtualLua.CPULimit != 1234 || virtualLua.MemoryLimit != "64MB" {
		t.Fatalf("virtual-lua runtime = %+v, want lua-specific fields preserved", virtualLua)
	}

	imageRuntime := implementations[2].Runtimes[0]
	if imageRuntime.Image != "debian:stable-slim" || !imageRuntime.EnableHostSSH ||
		len(imageRuntime.Volumes) != 2 || len(imageRuntime.Ports) != 2 ||
		imageRuntime.Persistent == nil || imageRuntime.Persistent.Name != "existing_dev" ||
		imageRuntime.Persistent.CreateIfMissing {
		t.Fatalf("image container runtime = %+v, want image fields preserved", imageRuntime)
	}

	containerfileRuntime := implementations[3].Runtimes[0]
	if containerfileRuntime.Containerfile != "Containerfile" || containerfileRuntime.Persistent == nil ||
		!containerfileRuntime.Persistent.CreateIfMissing || containerfileRuntime.Persistent.Name != "" {
		t.Fatalf("containerfile runtime = %+v, want containerfile fields preserved", containerfileRuntime)
	}
}

func TestGenerateCUE_RuntimeDependsOnEachTypeRoundTrip(t *testing.T) {
	t.Parallel()

	exitZero := types.ExitCode(0)
	exitOne := types.ExitCode(1)
	tests := []struct {
		name   string
		deps   DependsOn
		assert func(*testing.T, *DependsOn)
	}{
		{
			name: "tools",
			deps: DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"python3", "pypy"}}}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.Tools) != 1 || len(deps.Tools[0].Alternatives) != 2 || deps.Tools[0].Alternatives[1] != "pypy" {
					t.Fatalf("Tools = %+v, want python3/pypy alternatives", deps.Tools)
				}
			},
		},
		{
			name: "cmds",
			deps: DependsOn{Commands: []CommandDependency{{Alternatives: []CommandDependencyRef{"prepare", "compile"}}}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.Commands) != 1 || len(deps.Commands[0].Alternatives) != 2 || deps.Commands[0].Alternatives[0] != "prepare" {
					t.Fatalf("Commands = %+v, want command alternatives", deps.Commands)
				}
			},
		},
		{
			name: "filepaths",
			deps: DependsOn{Filepaths: []FilepathDependency{{
				Alternatives: []FilesystemPath{"./config.json", "./config.yaml"},
				Readable:     true,
				Writable:     true,
				Executable:   true,
			}}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.Filepaths) != 1 || len(deps.Filepaths[0].Alternatives) != 2 ||
					!deps.Filepaths[0].Readable || !deps.Filepaths[0].Writable || !deps.Filepaths[0].Executable {
					t.Fatalf("Filepaths = %+v, want alternatives and permission flags", deps.Filepaths)
				}
			},
		},
		{
			name: "capabilities",
			deps: DependsOn{Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityTTY, CapabilityInternet}}}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.Capabilities) != 1 || deps.Capabilities[0].Alternatives[0] != CapabilityTTY ||
					deps.Capabilities[0].Alternatives[1] != CapabilityInternet {
					t.Fatalf("Capabilities = %+v, want tty/internet alternatives", deps.Capabilities)
				}
			},
		},
		{
			name: "custom checks",
			deps: DependsOn{CustomChecks: []CustomCheckDependency{
				{
					Name:           "direct",
					Script:         CustomCheckScript{Content: "echo ok", Interpreter: "sh"},
					ExpectedCode:   &exitZero,
					ExpectedOutput: "^ok$",
				},
				{
					Alternatives: []CustomCheck{
						{
							Name:           "alt-one",
							Script:         CustomCheckScript{Content: "echo one"},
							ExpectedCode:   &exitOne,
							ExpectedOutput: "^one$",
						},
						{Name: "alt-two", Script: CustomCheckScript{Content: "echo two"}},
					},
				},
			}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.CustomChecks) != 2 {
					t.Fatalf("CustomChecks length = %d, want 2", len(deps.CustomChecks))
				}
				if deps.CustomChecks[0].Name != "direct" || deps.CustomChecks[0].Script.Interpreter != "sh" ||
					deps.CustomChecks[0].ExpectedCode == nil || *deps.CustomChecks[0].ExpectedCode != exitZero ||
					deps.CustomChecks[0].ExpectedOutput != "^ok$" {
					t.Fatalf("direct CustomCheck = %+v, want script, code, output preserved", deps.CustomChecks[0])
				}
				alternatives := deps.CustomChecks[1].Alternatives
				if len(alternatives) != 2 || alternatives[0].Name != "alt-one" ||
					alternatives[0].ExpectedCode == nil || *alternatives[0].ExpectedCode != exitOne ||
					alternatives[0].ExpectedOutput != "^one$" || alternatives[1].Name != "alt-two" {
					t.Fatalf("alternative CustomChecks = %+v, want both alternatives preserved", alternatives)
				}
			},
		},
		{
			name: "env vars",
			deps: DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{
				{Name: "API_KEY"},
				{Name: "TOKEN", Validation: "^[A-Z0-9]+$"},
			}}}},
			assert: func(t *testing.T, deps *DependsOn) {
				t.Helper()
				if len(deps.EnvVars) != 1 || len(deps.EnvVars[0].Alternatives) != 2 ||
					deps.EnvVars[0].Alternatives[1].Name != "TOKEN" ||
					deps.EnvVars[0].Alternatives[1].Validation != "^[A-Z0-9]+$" {
					t.Fatalf("EnvVars = %+v, want validated TOKEN alternative", deps.EnvVars)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, generated := parseGeneratedCUEMutationFixture(t, runtimeDependsOnMutationFixture(&tt.deps))
			requireGeneratedContains(t, generated, "depends_on:")
			runtimeDeps := parsed.Commands[0].Implementations[0].Runtimes[0].DependsOn
			if runtimeDeps == nil {
				t.Fatal("runtime DependsOn = nil, want generated dependencies")
			}
			tt.assert(t, runtimeDeps)
		})
	}
}

func TestGenerateCUE_VirtualFilesystemAccessWithoutPathsRoundTrip(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "virtual-fs",
			Implementations: []Implementation{{
				Script:   ImplementationScript{Content: "echo virtual fs"},
				Runtimes: []RuntimeConfig{{Name: RuntimeVirtualSh}},
				Platforms: []PlatformConfig{{
					Name: PlatformLinux,
					Virtual: &PlatformVirtualConfig{Filesystem: &VirtualFilesystemConfig{
						Access: VirtualFilesystemAccessFull,
					}},
				}},
			}},
		}},
	}

	parsed, generated := parseGeneratedCUEMutationFixture(t, inv)
	requireGeneratedContains(t, generated, `virtual: {`)
	requireGeneratedContains(t, generated, `filesystem: {`)
	requireGeneratedContains(t, generated, `access: "full"`)
	requireGeneratedNotContains(t, generated, "paths:")

	filesystem := parsed.Commands[0].Implementations[0].Platforms[0].VirtualFilesystem()
	if filesystem.EffectiveAccess() != VirtualFilesystemAccessFull {
		t.Fatalf("VirtualFilesystem access = %q, want full", filesystem.EffectiveAccess())
	}
	if len(filesystem.Paths) != 0 {
		t.Fatalf("VirtualFilesystem paths = %+v, want empty", filesystem.Paths)
	}
}

func runtimeDependsOnMutationFixture(deps *DependsOn) *Invowkfile {
	return &Invowkfile{
		Commands: []Command{{
			Name: "runtime-deps",
			Implementations: []Implementation{{
				Script: ImplementationScript{Content: "echo deps"},
				Runtimes: []RuntimeConfig{{
					Name:      RuntimeContainer,
					Image:     "debian:stable-slim",
					DependsOn: deps,
				}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
		}},
	}
}

func parseGeneratedCUEMutationFixture(t *testing.T, inv *Invowkfile) (parsed *Invowkfile, generated string) {
	t.Helper()

	generated = GenerateCUE(inv)
	parsed, err := ParseBytes([]byte(generated), "generated.cue")
	if err != nil {
		t.Fatalf("ParseBytes(GenerateCUE()) error = %v\n%s", err, generated)
	}
	return parsed, generated
}

func requireGeneratedContains(t *testing.T, generated, want string) {
	t.Helper()

	if !strings.Contains(generated, want) {
		t.Fatalf("generated CUE missing %q:\n%s", want, generated)
	}
}

func requireGeneratedNotContains(t *testing.T, generated, forbidden string) {
	t.Helper()

	if strings.Contains(generated, forbidden) {
		t.Fatalf("generated CUE unexpectedly contains %q:\n%s", forbidden, generated)
	}
}
