// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestContainerEngine_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		engine  ContainerEngine
		want    bool
		wantErr bool
	}{
		{ContainerEnginePodman, true, false},
		{ContainerEngineDocker, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"PODMAN", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.engine), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.engine.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerEngine(%q).IsValid() = %v, want %v", tt.engine, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerEngine(%q).IsValid() returned no errors, want error", tt.engine)
				}
				if !errors.Is(errs[0], ErrInvalidContainerEngine) {
					t.Errorf("error should wrap ErrInvalidContainerEngine, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerEngine(%q).IsValid() returned unexpected errors: %v", tt.engine, errs)
			}
		})
	}
}

func TestConfigRuntimeMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode    RuntimeMode
		want    bool
		wantErr bool
	}{
		{RuntimeNative, true, false},
		{RuntimeVirtual, true, false},
		{RuntimeContainer, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"NATIVE", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mode.IsValid()
			if isValid != tt.want {
				t.Errorf("RuntimeMode(%q).IsValid() = %v, want %v", tt.mode, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("RuntimeMode(%q).IsValid() returned no errors, want error", tt.mode)
				}
				if !errors.Is(errs[0], ErrInvalidConfigRuntimeMode) {
					t.Errorf("error should wrap ErrInvalidConfigRuntimeMode, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("RuntimeMode(%q).IsValid() returned unexpected errors: %v", tt.mode, errs)
			}
		})
	}
}

func TestColorScheme_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scheme  ColorScheme
		want    bool
		wantErr bool
	}{
		{ColorSchemeAuto, true, false},
		{ColorSchemeDark, true, false},
		{ColorSchemeLight, true, false},
		{"", false, true},
		{"garbage", false, true},
		{"AUTO", false, true},
		{"Dark", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.scheme), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.scheme.IsValid()
			if isValid != tt.want {
				t.Errorf("ColorScheme(%q).IsValid() = %v, want %v", tt.scheme, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ColorScheme(%q).IsValid() returned no errors, want error", tt.scheme)
				}
				if !errors.Is(errs[0], ErrInvalidColorScheme) {
					t.Errorf("error should wrap ErrInvalidColorScheme, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ColorScheme(%q).IsValid() returned unexpected errors: %v", tt.scheme, errs)
			}
		})
	}
}

func TestModuleIncludePath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    ModuleIncludePath
		want    bool
		wantErr bool
	}{
		{"absolute path", ModuleIncludePath("/home/user/modules/my.invowkmod"), true, false},
		{"relative path", ModuleIncludePath("modules/my.invowkmod"), true, false},
		{"single char", ModuleIncludePath("/"), true, false},
		{"empty is invalid", ModuleIncludePath(""), false, true},
		{"whitespace only is invalid", ModuleIncludePath("   "), false, true},
		{"tab only is invalid", ModuleIncludePath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleIncludePath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleIncludePath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidModuleIncludePath) {
					t.Errorf("error should wrap ErrInvalidModuleIncludePath, got: %v", errs[0])
				}
				var mpErr *InvalidModuleIncludePathError
				if !errors.As(errs[0], &mpErr) {
					t.Errorf("error should be *InvalidModuleIncludePathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleIncludePath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestModuleIncludePath_String(t *testing.T) {
	t.Parallel()
	p := ModuleIncludePath("/home/user/modules/my.invowkmod")
	if p.String() != "/home/user/modules/my.invowkmod" {
		t.Errorf("ModuleIncludePath.String() = %q, want %q", p.String(), "/home/user/modules/my.invowkmod")
	}
}

func TestBinaryFilePath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    BinaryFilePath
		want    bool
		wantErr bool
	}{
		{"empty is valid (auto-detect)", BinaryFilePath(""), true, false},
		{"absolute path", BinaryFilePath("/usr/local/bin/invowk"), true, false},
		{"relative path", BinaryFilePath("bin/invowk"), true, false},
		{"single char", BinaryFilePath("/"), true, false},
		{"whitespace only is invalid", BinaryFilePath("   "), false, true},
		{"tab only is invalid", BinaryFilePath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("BinaryFilePath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("BinaryFilePath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidBinaryFilePath) {
					t.Errorf("error should wrap ErrInvalidBinaryFilePath, got: %v", errs[0])
				}
				var bfpErr *InvalidBinaryFilePathError
				if !errors.As(errs[0], &bfpErr) {
					t.Errorf("error should be *InvalidBinaryFilePathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("BinaryFilePath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestBinaryFilePath_String(t *testing.T) {
	t.Parallel()
	p := BinaryFilePath("/usr/local/bin/invowk")
	if p.String() != "/usr/local/bin/invowk" {
		t.Errorf("BinaryFilePath.String() = %q, want %q", p.String(), "/usr/local/bin/invowk")
	}
}

func TestCacheDirPath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    CacheDirPath
		want    bool
		wantErr bool
	}{
		{"empty is valid (default cache)", CacheDirPath(""), true, false},
		{"absolute path", CacheDirPath("/var/cache/invowk"), true, false},
		{"relative path", CacheDirPath(".cache/invowk"), true, false},
		{"single char", CacheDirPath("/"), true, false},
		{"whitespace only is invalid", CacheDirPath("   "), false, true},
		{"tab only is invalid", CacheDirPath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("CacheDirPath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("CacheDirPath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidCacheDirPath) {
					t.Errorf("error should wrap ErrInvalidCacheDirPath, got: %v", errs[0])
				}
				var cdpErr *InvalidCacheDirPathError
				if !errors.As(errs[0], &cdpErr) {
					t.Errorf("error should be *InvalidCacheDirPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("CacheDirPath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestCacheDirPath_String(t *testing.T) {
	t.Parallel()
	p := CacheDirPath("/var/cache/invowk")
	if p.String() != "/var/cache/invowk" {
		t.Errorf("CacheDirPath.String() = %q, want %q", p.String(), "/var/cache/invowk")
	}
}

func TestContainerEngine_String(t *testing.T) {
	t.Parallel()

	if got := ContainerEnginePodman.String(); got != "podman" {
		t.Errorf("ContainerEnginePodman.String() = %q, want %q", got, "podman")
	}
}

func TestConfigRuntimeMode_String(t *testing.T) {
	t.Parallel()

	if got := RuntimeNative.String(); got != "native" {
		t.Errorf("RuntimeNative.String() = %q, want %q", got, "native")
	}
}

func TestColorScheme_String(t *testing.T) {
	t.Parallel()

	if got := ColorSchemeAuto.String(); got != "auto" {
		t.Errorf("ColorSchemeAuto.String() = %q, want %q", got, "auto")
	}
}

func TestIncludeEntry_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entry     IncludeEntry
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors
	}{
		{
			"all valid",
			IncludeEntry{
				Path:  ModuleIncludePath("/home/user/modules/my.invowkmod"),
				Alias: invowkmod.ModuleAlias("mytools"),
			},
			true, false, 0,
		},
		{
			"valid path, empty alias (zero value alias is valid)",
			IncludeEntry{
				Path:  ModuleIncludePath("/home/user/modules/my.invowkmod"),
				Alias: invowkmod.ModuleAlias(""),
			},
			true, false, 0,
		},
		{
			"invalid path (empty)",
			IncludeEntry{
				Path:  ModuleIncludePath(""),
				Alias: invowkmod.ModuleAlias("mytools"),
			},
			false, true, 1,
		},
		{
			"invalid alias (whitespace-only)",
			IncludeEntry{
				Path:  ModuleIncludePath("/home/user/modules/my.invowkmod"),
				Alias: invowkmod.ModuleAlias("   "),
			},
			false, true, 1,
		},
		{
			"both invalid",
			IncludeEntry{
				Path:  ModuleIncludePath(""),
				Alias: invowkmod.ModuleAlias("   "),
			},
			false, true, 2,
		},
		{
			"zero value struct",
			IncludeEntry{},
			false, true, 1, // empty Path is invalid; empty Alias is skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.entry.IsValid()
			if isValid != tt.want {
				t.Errorf("IncludeEntry.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("IncludeEntry.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidIncludeEntry) {
					t.Errorf("error should wrap ErrInvalidIncludeEntry, got: %v", errs[0])
				}
				var entryErr *InvalidIncludeEntryError
				if !errors.As(errs[0], &entryErr) {
					t.Fatalf("error should be *InvalidIncludeEntryError, got: %T", errs[0])
				}
				if len(entryErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(entryErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("IncludeEntry.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestUIConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       UIConfig
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid with auto color scheme",
			UIConfig{ColorScheme: ColorSchemeAuto, Verbose: true, Interactive: false},
			true, false, 0,
		},
		{
			"valid with dark color scheme",
			UIConfig{ColorScheme: ColorSchemeDark},
			true, false, 0,
		},
		{
			"invalid color scheme",
			UIConfig{ColorScheme: "neon"},
			false, true, 1,
		},
		{
			"zero value (empty color scheme is invalid)",
			UIConfig{},
			false, true, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("UIConfig.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("UIConfig.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidUIConfig) {
					t.Errorf("error should wrap ErrInvalidUIConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidUIConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidUIConfigError, got: %T", errs[0])
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("UIConfig.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestAutoProvisionConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       AutoProvisionConfig
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid with defaults",
			AutoProvisionConfig{
				Enabled:         true,
				BinaryPath:      "",
				Includes:        []IncludeEntry{},
				InheritIncludes: true,
				CacheDir:        "",
			},
			true, false, 0,
		},
		{
			"valid with explicit paths",
			AutoProvisionConfig{
				BinaryPath: "/usr/bin/invowk",
				Includes:   []IncludeEntry{{Path: "/home/user/my.invowkmod"}},
				CacheDir:   "/tmp/cache",
			},
			true, false, 0,
		},
		{
			"invalid binary path (whitespace-only)",
			AutoProvisionConfig{BinaryPath: "   "},
			false, true, 1,
		},
		{
			"invalid cache dir (whitespace-only)",
			AutoProvisionConfig{CacheDir: "   "},
			false, true, 1,
		},
		{
			"invalid include entry",
			AutoProvisionConfig{
				Includes: []IncludeEntry{{Path: ""}},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			AutoProvisionConfig{
				BinaryPath: "   ",
				Includes:   []IncludeEntry{{Path: ""}},
				CacheDir:   "   ",
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("AutoProvisionConfig.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("AutoProvisionConfig.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidAutoProvisionConfig) {
					t.Errorf("error should wrap ErrInvalidAutoProvisionConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidAutoProvisionConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidAutoProvisionConfigError, got: %T", errs[0])
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("AutoProvisionConfig.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestContainerConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     ContainerConfig
		want    bool
		wantErr bool
	}{
		{
			"valid with default auto provision",
			ContainerConfig{
				AutoProvision: AutoProvisionConfig{
					Enabled:    true,
					BinaryPath: "",
					Includes:   []IncludeEntry{},
					CacheDir:   "",
				},
			},
			true, false,
		},
		{
			"invalid auto provision (invalid include)",
			ContainerConfig{
				AutoProvision: AutoProvisionConfig{
					Includes: []IncludeEntry{{Path: ""}},
				},
			},
			false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerConfig.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerConfig.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidContainerConfig) {
					t.Errorf("error should wrap ErrInvalidContainerConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidContainerConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidContainerConfigError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerConfig.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestConfig_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       Config
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"DefaultConfig is valid",
			*DefaultConfig(),
			true, false, 0,
		},
		{
			"invalid container engine",
			func() Config {
				c := *DefaultConfig()
				c.ContainerEngine = "unknown"
				return c
			}(),
			false, true, 1,
		},
		{
			"invalid default runtime",
			func() Config {
				c := *DefaultConfig()
				c.DefaultRuntime = "bogus"
				return c
			}(),
			false, true, 1,
		},
		{
			"invalid UI color scheme",
			func() Config {
				c := *DefaultConfig()
				c.UI.ColorScheme = "neon"
				return c
			}(),
			false, true, 1,
		},
		{
			"invalid include entry",
			func() Config {
				c := *DefaultConfig()
				c.Includes = []IncludeEntry{{Path: ""}}
				return c
			}(),
			false, true, 1,
		},
		{
			"invalid container config (invalid auto provision include)",
			func() Config {
				c := *DefaultConfig()
				c.Container.AutoProvision.Includes = []IncludeEntry{{Path: ""}}
				return c
			}(),
			false, true, 1,
		},
		{
			"multiple invalid fields",
			func() Config {
				c := *DefaultConfig()
				c.ContainerEngine = "unknown"
				c.DefaultRuntime = "bogus"
				c.UI.ColorScheme = "neon"
				return c
			}(),
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cfg.IsValid()
			if isValid != tt.want {
				t.Errorf("Config.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Config.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidConfig) {
					t.Errorf("error should wrap ErrInvalidConfig, got: %v", errs[0])
				}
				var cfgErr *InvalidConfigError
				if !errors.As(errs[0], &cfgErr) {
					t.Fatalf("error should be *InvalidConfigError, got: %T", errs[0])
				}
				if len(cfgErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(cfgErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("Config.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}
