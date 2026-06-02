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
	requireGeneratedRootRoundTrip(t, parsed)
	requireGeneratedDeployRoundTrip(t, parsed.Commands[1])
}

func requireGeneratedRootRoundTrip(t *testing.T, parsed *Invowkfile) {
	t.Helper()

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
}

func requireGeneratedDeployRoundTrip(t *testing.T, deploy Command) {
	t.Helper()

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
	requireGeneratedDeployFlags(t, deploy.Flags)
	requireGeneratedDeployArgs(t, deploy.Args)
	requireGeneratedDeployWatch(t, deploy.Watch)
	requireGeneratedDeployImplementation(t, deploy.Implementations[0])
}

func requireGeneratedDeployFlags(t *testing.T, flags []Flag) {
	t.Helper()

	if len(flags) != 2 {
		t.Fatalf("deploy Flags length = %d, want 2", len(flags))
	}
	if !flags[0].Required || flags[0].Short != "t" || flags[0].Validation != "^(dev|prod)$" {
		t.Fatalf("target flag = %+v, want required short validation preserved", flags[0])
	}
	if flags[1].DefaultValue != "false" || flags[1].GetType() != FlagTypeBool {
		t.Fatalf("dry-run flag = %+v, want false bool default", flags[1])
	}
}

func requireGeneratedDeployArgs(t *testing.T, args []Argument) {
	t.Helper()

	if len(args) != 2 {
		t.Fatalf("deploy Args length = %d, want 2", len(args))
	}
	if !args[0].Required || args[0].GetType() != ArgumentTypeInt || args[0].Validation != "^[0-9]+$" {
		t.Fatalf("count arg = %+v, want required int validation preserved", args[0])
	}
	if args[1].DefaultValue != "all" || !args[1].Variadic {
		t.Fatalf("files arg = %+v, want default variadic preserved", args[1])
	}
}

func requireGeneratedDeployWatch(t *testing.T, watch *WatchConfig) {
	t.Helper()

	if watch == nil || len(watch.Patterns) != 2 || watch.Debounce != "250ms" ||
		!watch.ClearScreen || len(watch.Ignore) != 2 {
		t.Fatalf("Watch = %+v, want full watch config preserved", watch)
	}
}

func requireGeneratedDeployImplementation(t *testing.T, impl Implementation) {
	t.Helper()

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
			name:   "tools",
			deps:   DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"python3", "pypy"}}}},
			assert: assertRuntimeToolsDeps,
		},
		{
			name:   "cmds",
			deps:   DependsOn{Commands: []CommandDependency{{Alternatives: []CommandDependencyRef{"prepare", "compile"}}}},
			assert: assertRuntimeCommandDeps,
		},
		{
			name: "filepaths",
			deps: DependsOn{Filepaths: []FilepathDependency{{
				Alternatives: []FilesystemPath{"./config.json", "./config.yaml"},
				Readable:     true,
				Writable:     true,
				Executable:   true,
			}}},
			assert: assertRuntimeFilepathDeps,
		},
		{
			name:   "capabilities",
			deps:   DependsOn{Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityTTY, CapabilityInternet}}}},
			assert: assertRuntimeCapabilityDeps,
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
			assert: assertRuntimeCustomCheckDeps,
		},
		{
			name: "env vars",
			deps: DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{
				{Name: "API_KEY"},
				{Name: "TOKEN", Validation: "^[A-Z0-9]+$"},
			}}}},
			assert: assertRuntimeEnvVarDeps,
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

func TestGenerateCUE_EnvBlockOmitsEmptySubsections(t *testing.T) {
	t.Parallel()

	var varsOnly strings.Builder
	generateEnvBlock(&varsOnly, &EnvConfig{
		Vars: map[EnvVarName]string{"ONLY_VAR": "1"},
	}, "")
	varsOnlyCUE := varsOnly.String()
	requireGeneratedContains(t, varsOnlyCUE, "env: {")
	requireGeneratedContains(t, varsOnlyCUE, "vars: {")
	requireGeneratedContains(t, varsOnlyCUE, `ONLY_VAR: "1"`)
	requireGeneratedNotContains(t, varsOnlyCUE, "files: [")

	var filesOnly strings.Builder
	generateEnvBlock(&filesOnly, &EnvConfig{
		Files: []DotenvFilePath{".env"},
	}, "")
	filesOnlyCUE := filesOnly.String()
	requireGeneratedContains(t, filesOnlyCUE, "env: {")
	requireGeneratedContains(t, filesOnlyCUE, `files: [".env"]`)
	requireGeneratedNotContains(t, filesOnlyCUE, "vars: {")

	var empty strings.Builder
	generateEnvBlock(&empty, &EnvConfig{}, "")
	if got := empty.String(); got != "" {
		t.Fatalf("empty env block output = %q, want empty", got)
	}
}

func TestGenerateCUE_CommandOmitsEmptyCollectionsAndDefaultTypes(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{{
			Name: "shape",
			Implementations: []Implementation{{
				Script:    ImplementationScript{Content: "echo shape"},
				Runtimes:  []RuntimeConfig{{Name: RuntimeVirtualSh}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			}},
			Flags: []Flag{{
				Name:        "target",
				Description: "Target",
				Type:        FlagTypeString,
			}},
			Watch: &WatchConfig{Patterns: []GlobPattern{"src/**"}},
		}},
	}

	_, generated := parseGeneratedCUEMutationFixture(t, inv)
	requireGeneratedContains(t, generated, "flags: [")
	requireGeneratedContains(t, generated, `name: "target"`)
	requireGeneratedContains(t, generated, "watch: {")
	requireGeneratedNotContains(t, generated, `type: "string"`)
	requireGeneratedNotContains(t, generated, "args: [")
	requireGeneratedNotContains(t, generated, "ignore: [")
}

func TestGeneratePlatformAndVirtualHelpersOmitEmptyConfig(t *testing.T) {
	t.Parallel()

	var plainPlatform strings.Builder
	generatePlatformConfig(&plainPlatform, PlatformConfig{Name: PlatformLinux}, "\t")
	if got, want := plainPlatform.String(), "\t{name: \"linux\"},\n"; got != want {
		t.Fatalf("plain platform output = %q, want %q", got, want)
	}

	var emptyVirtualPlatform strings.Builder
	generatePlatformConfig(&emptyVirtualPlatform, PlatformConfig{
		Name:    PlatformLinux,
		Virtual: &PlatformVirtualConfig{Filesystem: &VirtualFilesystemConfig{}},
	}, "\t")
	if got, want := emptyVirtualPlatform.String(), "\t{name: \"linux\"},\n"; got != want {
		t.Fatalf("empty virtual platform output = %q, want %q", got, want)
	}

	for _, tt := range []struct {
		name string
		run  func(*strings.Builder)
	}{
		{
			name: "nil platform virtual",
			run: func(sb *strings.Builder) {
				generatePlatformVirtualConfig(sb, nil, "\t")
			},
		},
		{
			name: "empty platform virtual",
			run: func(sb *strings.Builder) {
				generatePlatformVirtualConfig(sb, &PlatformVirtualConfig{}, "\t")
			},
		},
		{
			name: "nil virtual filesystem",
			run: func(sb *strings.Builder) {
				generateVirtualFilesystemConfig(sb, nil, "\t")
			},
		},
		{
			name: "empty virtual filesystem",
			run: func(sb *strings.Builder) {
				generateVirtualFilesystemConfig(sb, &VirtualFilesystemConfig{}, "\t")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			tt.run(&sb)
			if got := sb.String(); got != "" {
				t.Fatalf("%s output = %q, want empty", tt.name, got)
			}
		})
	}
}

func TestGenerateRuntimeConfigShapeForEmptyAndSingleItemFields(t *testing.T) {
	t.Parallel()

	exitZero := types.ExitCode(0)
	customCheck := CustomCheckDependency{
		Name:         "direct",
		Script:       CustomCheckScript{Content: "echo ok"},
		ExpectedCode: &exitZero,
	}

	tests := []struct {
		name      string
		runtime   RuntimeConfig
		want      []string
		forbidden []string
	}{
		{
			name: "native ignores runtime depends_on",
			runtime: RuntimeConfig{
				Name:      RuntimeNative,
				DependsOn: &DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"git"}}}},
			},
			want:      []string{`{name: "native"},`},
			forbidden: []string{"depends_on:"},
		},
		{
			name: "container empty depends_on stays compact",
			runtime: RuntimeConfig{
				Name:      RuntimeContainer,
				Image:     "debian:stable-slim",
				DependsOn: &DependsOn{},
			},
			want:      []string{`{name: "container", image: "debian:stable-slim"},`},
			forbidden: []string{"depends_on:", "tools: [", "cmds: [", "filepaths: [", "capabilities: [", "custom_checks: [", "env_vars: ["},
		},
		{
			name: "container single custom check uses multiline depends_on",
			runtime: RuntimeConfig{
				Name:      RuntimeContainer,
				Image:     "debian:stable-slim",
				DependsOn: &DependsOn{CustomChecks: []CustomCheckDependency{customCheck}},
			},
			want: []string{`name: "container"`, "depends_on:", "custom_checks:", `name: "direct"`},
		},
		{
			name: "virtual sh single allow omits empty lists",
			runtime: RuntimeConfig{
				Name:             RuntimeVirtualSh,
				EnvInheritMode:   EnvInheritAllow,
				EnvInheritAllow:  []EnvVarName{"PATH"},
				BinaryLookupMode: BinaryLookupModeStrict,
			},
			want:      []string{`env_inherit_mode: "allow"`, `env_inherit_allow: ["PATH"]`, `binary_lookup_mode: "strict"`},
			forbidden: []string{"env_inherit_deny: []", "allowed_binaries: []"},
		},
		{
			name: "virtual lua cpu limit one is emitted",
			runtime: RuntimeConfig{
				Name:     RuntimeVirtualLua,
				CPULimit: 1,
			},
			want:      []string{"cpu_limit: 1"},
			forbidden: []string{"allowed_binaries: []"},
		},
		{
			name: "container single volume and port are emitted",
			runtime: RuntimeConfig{
				Name:    RuntimeContainer,
				Image:   "debian:stable-slim",
				Volumes: []VolumeMountSpec{"./data:/data"},
				Ports:   []PortMappingSpec{"8080:80"},
			},
			want: []string{`volumes: ["./data:/data"]`, `ports: ["8080:80"]`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			generateRuntimeConfig(&sb, &tt.runtime)
			generated := sb.String()
			for _, want := range tt.want {
				requireGeneratedContains(t, generated, want)
			}
			for _, forbidden := range tt.forbidden {
				requireGeneratedNotContains(t, generated, forbidden)
			}
		})
	}
}

func TestGenerateDependsOnContentOmitsInactiveSections(t *testing.T) {
	t.Parallel()

	exitZero := types.ExitCode(0)
	tests := []struct {
		name  string
		deps  DependsOn
		label string
	}{
		{
			name:  "tools",
			deps:  DependsOn{Tools: []ToolDependency{{Alternatives: []BinaryName{"git"}}}},
			label: "tools: [",
		},
		{
			name:  "cmds",
			deps:  DependsOn{Commands: []CommandDependency{{Alternatives: []CommandDependencyRef{"build"}}}},
			label: "cmds: [",
		},
		{
			name: "filepaths",
			deps: DependsOn{Filepaths: []FilepathDependency{{
				Alternatives: []FilesystemPath{"./config.yaml"},
				Readable:     true,
			}}},
			label: "filepaths: [",
		},
		{
			name:  "capabilities",
			deps:  DependsOn{Capabilities: []CapabilityDependency{{Alternatives: []CapabilityName{CapabilityInternet}}}},
			label: "capabilities: [",
		},
		{
			name: "custom checks",
			deps: DependsOn{CustomChecks: []CustomCheckDependency{{
				Name:         "check",
				Script:       CustomCheckScript{Content: "echo check"},
				ExpectedCode: &exitZero,
			}}},
			label: "custom_checks: [",
		},
		{
			name: "env vars",
			deps: DependsOn{EnvVars: []EnvVarDependency{{Alternatives: []EnvVarCheck{{
				Name:       "TOKEN",
				Validation: "^[A-Z]+$",
			}}}}},
			label: "env_vars: [",
		},
	}
	labels := []string{"tools: [", "cmds: [", "filepaths: [", "capabilities: [", "custom_checks: [", "env_vars: ["}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			generateDependsOnContent(&sb, &tt.deps, "\t")
			generated := sb.String()
			requireGeneratedContains(t, generated, tt.label)
			for _, label := range labels {
				if label != tt.label {
					requireGeneratedNotContains(t, generated, label)
				}
			}
		})
	}

	var empty strings.Builder
	generateDependsOnContent(&empty, &DependsOn{}, "\t")
	if got := empty.String(); got != "" {
		t.Fatalf("empty depends_on content = %q, want empty", got)
	}
}

func assertRuntimeToolsDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.Tools) != 1 || len(deps.Tools[0].Alternatives) != 2 || deps.Tools[0].Alternatives[1] != "pypy" {
		t.Fatalf("Tools = %+v, want python3/pypy alternatives", deps.Tools)
	}
}

func assertRuntimeCommandDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.Commands) != 1 || len(deps.Commands[0].Alternatives) != 2 || deps.Commands[0].Alternatives[0] != "prepare" {
		t.Fatalf("Commands = %+v, want command alternatives", deps.Commands)
	}
}

func assertRuntimeFilepathDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.Filepaths) != 1 || len(deps.Filepaths[0].Alternatives) != 2 ||
		!deps.Filepaths[0].Readable || !deps.Filepaths[0].Writable || !deps.Filepaths[0].Executable {
		t.Fatalf("Filepaths = %+v, want alternatives and permission flags", deps.Filepaths)
	}
}

func assertRuntimeCapabilityDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.Capabilities) != 1 || deps.Capabilities[0].Alternatives[0] != CapabilityTTY ||
		deps.Capabilities[0].Alternatives[1] != CapabilityInternet {
		t.Fatalf("Capabilities = %+v, want tty/internet alternatives", deps.Capabilities)
	}
}

func assertRuntimeCustomCheckDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.CustomChecks) != 2 {
		t.Fatalf("CustomChecks length = %d, want 2", len(deps.CustomChecks))
	}
	assertRuntimeDirectCustomCheck(t, deps.CustomChecks[0])
	assertRuntimeAlternativeCustomChecks(t, deps.CustomChecks[1].Alternatives)
}

func assertRuntimeDirectCustomCheck(t *testing.T, check CustomCheckDependency) {
	t.Helper()

	if check.Name != "direct" || check.Script.Interpreter != "sh" ||
		check.ExpectedCode == nil || *check.ExpectedCode != types.ExitCode(0) ||
		check.ExpectedOutput != "^ok$" {
		t.Fatalf("direct CustomCheck = %+v, want script, code, output preserved", check)
	}
}

func assertRuntimeAlternativeCustomChecks(t *testing.T, alternatives []CustomCheck) {
	t.Helper()

	if len(alternatives) != 2 || alternatives[0].Name != "alt-one" ||
		alternatives[0].ExpectedCode == nil || *alternatives[0].ExpectedCode != types.ExitCode(1) ||
		alternatives[0].ExpectedOutput != "^one$" || alternatives[1].Name != "alt-two" {
		t.Fatalf("alternative CustomChecks = %+v, want both alternatives preserved", alternatives)
	}
}

func assertRuntimeEnvVarDeps(t *testing.T, deps *DependsOn) {
	t.Helper()

	if len(deps.EnvVars) != 1 || len(deps.EnvVars[0].Alternatives) != 2 ||
		deps.EnvVars[0].Alternatives[1].Name != "TOKEN" ||
		deps.EnvVars[0].Alternatives[1].Validation != "^[A-Z0-9]+$" {
		t.Fatalf("EnvVars = %+v, want validated TOKEN alternative", deps.EnvVars)
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
