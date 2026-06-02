// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"slices"
	"testing"
)

func TestRuntimeBoundaryCommandStringAndPlatformLists(t *testing.T) {
	t.Parallel()

	commandCases := []struct {
		name string
		info ShebangInfo
		want string
	}{
		{name: "not found with interpreter", info: ShebangInfo{Interpreter: "python3"}, want: ""},
		{name: "found with blank interpreter", info: ShebangInfo{Found: true, Interpreter: " \t", Args: []string{"-u"}}, want: ""},
		{name: "found with args", info: ShebangInfo{Found: true, Interpreter: "python3", Args: []string{"-u", "-B"}}, want: "python3 -u -B"},
	}
	for _, tt := range commandCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.info.CommandString(); got != tt.want {
				t.Fatalf("CommandString() = %q, want %q", got, tt.want)
			}
		})
	}

	if got := AllPlatformNames(); !slices.Equal(got, []PlatformType{PlatformLinux, PlatformMac, PlatformWindows}) {
		t.Fatalf("AllPlatformNames() = %v, want linux/macos/windows order", got)
	}

	configs := AllPlatformConfigs()
	if len(configs) != 3 {
		t.Fatalf("AllPlatformConfigs() length = %d, want 3", len(configs))
	}
	for i, want := range []PlatformType{PlatformLinux, PlatformMac, PlatformWindows} {
		if configs[i].Name != want {
			t.Fatalf("AllPlatformConfigs()[%d].Name = %q, want %q", i, configs[i].Name, want)
		}
		if configs[i].Virtual != nil {
			t.Fatalf("AllPlatformConfigs()[%d].Virtual = %+v, want nil", i, configs[i].Virtual)
		}
	}
}

func TestRuntimeBoundaryValidationErrorPayloads(t *testing.T) {
	t.Parallel()

	envErr := EnvInheritMode("bogus").Validate()
	var invalidEnv *InvalidEnvInheritModeError
	if !errors.As(envErr, &invalidEnv) {
		t.Fatalf("EnvInheritMode.Validate() error = %T %v, want *InvalidEnvInheritModeError", envErr, envErr)
	}
	if invalidEnv.Value != "bogus" {
		t.Fatalf("InvalidEnvInheritModeError.Value = %q, want bogus", invalidEnv.Value)
	}
	if got := invalidEnv.Error(); got != `invalid env_inherit_mode "bogus" (valid: none, allow, all)` {
		t.Fatalf("InvalidEnvInheritModeError.Error() = %q", got)
	}

	platformErr := PlatformType("darwin").Validate()
	var invalidPlatform *InvalidPlatformError
	if !errors.As(platformErr, &invalidPlatform) {
		t.Fatalf("PlatformType.Validate() error = %T %v, want *InvalidPlatformError", platformErr, platformErr)
	}
	if invalidPlatform.Value != "darwin" {
		t.Fatalf("InvalidPlatformError.Value = %q, want darwin", invalidPlatform.Value)
	}
	if got := invalidPlatform.Error(); got != `invalid platform type "darwin" (valid: linux, macos, windows)` {
		t.Fatalf("InvalidPlatformError.Error() = %q", got)
	}

	lookupErr := BinaryLookupMode("path").Validate()
	var invalidLookup *InvalidBinaryLookupModeError
	if !errors.As(lookupErr, &invalidLookup) {
		t.Fatalf("BinaryLookupMode.Validate() error = %T %v, want *InvalidBinaryLookupModeError", lookupErr, lookupErr)
	}
	if invalidLookup.Value != "path" {
		t.Fatalf("InvalidBinaryLookupModeError.Value = %q, want path", invalidLookup.Value)
	}
	if got := invalidLookup.Error(); got != `invalid binary_lookup_mode "path" (valid: host, strict)` {
		t.Fatalf("InvalidBinaryLookupModeError.Error() = %q", got)
	}

	accessErr := VirtualFilesystemAccess("wide").Validate()
	var invalidAccess *InvalidVirtualFilesystemAccessError
	if !errors.As(accessErr, &invalidAccess) {
		t.Fatalf("VirtualFilesystemAccess.Validate() error = %T %v, want *InvalidVirtualFilesystemAccessError", accessErr, accessErr)
	}
	if invalidAccess.Value != "wide" {
		t.Fatalf("InvalidVirtualFilesystemAccessError.Value = %q, want wide", invalidAccess.Value)
	}
	if got := invalidAccess.Error(); got != `invalid virtual.filesystem.access "wide" (valid: restricted, full)` {
		t.Fatalf("InvalidVirtualFilesystemAccessError.Error() = %q", got)
	}

	memoryErr := MemoryLimit("64TB").Validate()
	var invalidMemory *InvalidMemoryLimitError
	if !errors.As(memoryErr, &invalidMemory) {
		t.Fatalf("MemoryLimit.Validate() error = %T %v, want *InvalidMemoryLimitError", memoryErr, memoryErr)
	}
	if invalidMemory.Value != "64TB" {
		t.Fatalf("InvalidMemoryLimitError.Value = %q, want 64TB", invalidMemory.Value)
	}
	if got := invalidMemory.Error(); got != `invalid memory limit "64TB": must be a byte count with optional K, M, or G suffix` {
		t.Fatalf("InvalidMemoryLimitError.Error() = %q", got)
	}

	persistentErr := RuntimePersistentConfig{Name: "Uppercase"}.Validate()
	var invalidPersistent *InvalidRuntimePersistentConfigError
	if !errors.As(persistentErr, &invalidPersistent) {
		t.Fatalf("RuntimePersistentConfig.Validate() error = %T %v, want *InvalidRuntimePersistentConfigError", persistentErr, persistentErr)
	}
	if len(invalidPersistent.FieldErrors) != 1 {
		t.Fatalf("RuntimePersistentConfig field errors = %d, want 1", len(invalidPersistent.FieldErrors))
	}
	if !errors.Is(persistentErr, ErrInvalidRuntimePersistentConfig) {
		t.Fatalf("RuntimePersistentConfig error = %v, want ErrInvalidRuntimePersistentConfig", persistentErr)
	}
	if !errors.Is(persistentErr, ErrInvalidContainerName) {
		t.Fatalf("RuntimePersistentConfig error = %v, want ErrInvalidContainerName", persistentErr)
	}
}

func TestPlatformConfigVirtualMutationBoundaries(t *testing.T) {
	t.Parallel()

	err := PlatformConfig{
		Name: PlatformLinux,
		Virtual: &PlatformVirtualConfig{
			Filesystem: &VirtualFilesystemConfig{Access: "wide"},
		},
	}.Validate()
	var invalid *InvalidPlatformConfigError
	if !errors.As(err, &invalid) {
		t.Fatalf("PlatformConfig.Validate() error = %T %v, want *InvalidPlatformConfigError", err, err)
	}
	if !fieldErrorsContain(invalid.FieldErrors, `invalid virtual.filesystem.access "wide"`) {
		t.Fatalf("PlatformConfig field errors = %v, want invalid virtual filesystem access", invalid.FieldErrors)
	}

	empty := PlatformConfig{Name: PlatformLinux}.VirtualFilesystem()
	if empty.EffectiveAccess() != VirtualFilesystemAccessRestricted || len(empty.Paths) != 0 {
		t.Fatalf("missing virtual filesystem = %+v, want restricted with no paths", empty)
	}

	emptyNested := PlatformConfig{Name: PlatformLinux, Virtual: &PlatformVirtualConfig{}}.VirtualFilesystem()
	if emptyNested.EffectiveAccess() != VirtualFilesystemAccessRestricted || len(emptyNested.Paths) != 0 {
		t.Fatalf("missing nested filesystem = %+v, want restricted with no paths", emptyNested)
	}

	configured := PlatformConfig{
		Name: PlatformLinux,
		Virtual: &PlatformVirtualConfig{
			Filesystem: &VirtualFilesystemConfig{
				Access: VirtualFilesystemAccessFull,
				Paths:  VirtualFilesystemPaths{"CACHE": "/tmp/cache"},
			},
		},
	}.VirtualFilesystem()
	if configured.EffectiveAccess() != VirtualFilesystemAccessFull {
		t.Fatalf("configured access = %q, want full", configured.EffectiveAccess())
	}
	if configured.Paths["CACHE"] != "/tmp/cache" {
		t.Fatalf("configured path CACHE = %q, want /tmp/cache", configured.Paths["CACHE"])
	}
}

func TestRuntimeConfigInvariantMutationBoundaries(t *testing.T) {
	t.Parallel()

	nativeErr := RuntimeConfig{
		Name:             RuntimeNative,
		AllowedBinaries:  []AllowedBinary{"git"},
		BinaryLookupMode: BinaryLookupModeStrict,
		CPULimit:         10,
		MemoryLimit:      "64MB",
		DependsOn:        &DependsOn{},
		EnableHostSSH:    true,
		Containerfile:    "Containerfile",
		Image:            "debian:stable-slim",
		Volumes:          []VolumeMountSpec{"./data:/data"},
		Ports:            []PortMappingSpec{"8080:80"},
		Persistent:       &RuntimePersistentConfig{CreateIfMissing: true},
	}.Validate()
	assertRuntimeConfigFieldErrors(t, nativeErr, []string{
		"allowed_binaries is only valid for virtual runtimes",
		"binary_lookup_mode is only valid for virtual runtimes",
		"cpu_limit is only valid for virtual-lua runtime",
		"memory_limit is only valid for virtual-lua runtime",
		"depends_on is only valid for container runtime",
		"enable_host_ssh is only valid for container runtime",
		"containerfile is only valid for container runtime",
		"image is only valid for container runtime",
		"volumes is only valid for container runtime",
		"ports is only valid for container runtime",
		"persistent is only valid for container runtime",
	})

	virtualShErr := RuntimeConfig{
		Name:             RuntimeVirtualSh,
		AllowedBinaries:  []AllowedBinary{"git"},
		BinaryLookupMode: BinaryLookupModeHost,
		CPULimit:         1,
		MemoryLimit:      "1M",
	}.Validate()
	assertRuntimeConfigFieldErrors(t, virtualShErr, []string{
		"cpu_limit is only valid for virtual-lua runtime",
		"memory_limit is only valid for virtual-lua runtime",
	})

	assertRuntimeConfigFieldErrors(t, RuntimeConfig{
		Name:             RuntimeVirtualSh,
		BinaryLookupMode: "path",
	}.Validate(), []string{
		`invalid binary_lookup_mode "path"`,
	})

	assertRuntimeConfigFieldErrors(t, RuntimeConfig{
		Name:        RuntimeVirtualLua,
		MemoryLimit: "64TB",
	}.Validate(), []string{
		`invalid memory limit "64TB"`,
	})

	assertRuntimeConfigFieldErrors(t, RuntimeConfig{
		Name:  RuntimeContainer,
		Image: "debian:stable-slim",
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{""}}},
		},
	}.Validate(), []string{
		"invalid depends_on",
	})

	assertRuntimeConfigFieldErrors(t, RuntimeConfig{
		Name:       RuntimeContainer,
		Image:      "debian:stable-slim",
		Persistent: &RuntimePersistentConfig{Name: "Uppercase"},
	}.Validate(), []string{
		"invalid runtime persistent config",
	})

	virtualLua := RuntimeConfig{
		Name:             RuntimeVirtualLua,
		AllowedBinaries:  []AllowedBinary{"lua"},
		BinaryLookupMode: BinaryLookupModeStrict,
		CPULimit:         1,
		MemoryLimit:      "1M",
	}
	if err := virtualLua.Validate(); err != nil {
		t.Fatalf("RuntimeConfig.Validate() for virtual-lua = %v, want nil", err)
	}
}

func TestInterpreterMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		got         ShebangInfo
		interpreter string
		args        []string
		found       bool
	}{
		{
			name:        "leading whitespace before shebang",
			got:         ParseShebang(" \t#! /bin/sh -eu\n"),
			interpreter: "/bin/sh",
			args:        []string{"-eu"},
			found:       true,
		},
		{
			name:        "env skips flags before interpreter",
			got:         ParseShebang("#!/usr/bin/env -i python3 -u\n"),
			interpreter: "python3",
			args:        []string{"-u"},
			found:       true,
		},
		{
			name:  "env all flags has no interpreter",
			got:   ParseShebang("#!/usr/bin/env -i -u\n"),
			found: false,
		},
		{
			name:        "env -S preserves flag-looking interpreter",
			got:         ParseShebang("#!/usr/bin/env -S --flag value\n"),
			interpreter: "--flag",
			args:        []string{"value"},
			found:       true,
		},
		{
			name:        "env -S with interpreter only",
			got:         ParseShebang("#!/usr/bin/env -S python3\n"),
			interpreter: "python3",
			found:       true,
		},
		{
			name:  "padded auto interpreter stays auto",
			got:   ParseInterpreterString(" \tauto "),
			found: false,
		},
		{
			name:        "explicit bin env skips flags",
			got:         ParseInterpreterString("/bin/env -i python3 -u"),
			interpreter: "python3",
			args:        []string{"-u"},
			found:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got.Found != tt.found {
				t.Fatalf("Found = %v, want %v", tt.got.Found, tt.found)
			}
			if tt.got.Interpreter != tt.interpreter {
				t.Fatalf("Interpreter = %q, want %q", tt.got.Interpreter, tt.interpreter)
			}
			if !slices.Equal(tt.got.Args, tt.args) {
				t.Fatalf("Args = %v, want %v", tt.got.Args, tt.args)
			}
		})
	}

	if !IsLuaInterpreter("/usr/bin/lua5.4.exe") {
		t.Fatal("IsLuaInterpreter(/usr/bin/lua5.4.exe) = false, want true")
	}
	if IsLuaInterpreter("lua5.3") {
		t.Fatal("IsLuaInterpreter(lua5.3) = true, want false")
	}
}

func assertRuntimeConfigFieldErrors(t *testing.T, err error, want []string) {
	t.Helper()

	if err == nil {
		t.Fatal("RuntimeConfig.Validate() error = nil, want field errors")
	}
	var invalid *InvalidRuntimeConfigError
	if !errors.As(err, &invalid) {
		t.Fatalf("RuntimeConfig.Validate() error = %T %v, want *InvalidRuntimeConfigError", err, err)
	}
	for _, message := range want {
		if !fieldErrorsContain(invalid.FieldErrors, message) {
			t.Fatalf("RuntimeConfig field errors = %v, want %q", invalid.FieldErrors, message)
		}
	}
}
